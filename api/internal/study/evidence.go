package study

import (
	"app/internal/kontoapi"
	"database/sql"
	"errors"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"time"
)

type AuthenticationEvidence struct {
	Provider   string           `json:"provider"`
	OrderRef   string           `json:"orderRef,omitempty"`
	Result     *kontoapi.Result `json:"result,omitempty"`
	Error      string           `json:"error,omitempty"`
	ReceivedAt string           `json:"receivedAt"`
}
type ConsentEvidence struct {
	Method               string   `json:"method"`
	Document             Document `json:"document"`
	AuthenticationID     string   `json:"authenticationId"`
	PersonalNumber       string   `json:"personalNumber"`
	ParticipantID        string   `json:"participantId"`
	GuardianRequestID    string   `json:"guardianRequestId,omitempty"`
	ParticipantConsentID string   `json:"participantConsentId,omitempty"`
	AcceptedAt           int64    `json:"acceptedAt"`
}

func validIdentity(value string, now time.Time) bool {
	if !personalNumberPattern.MatchString(value) {
		return false
	}
	birth, err := time.Parse("20060102", value[:8])
	return err == nil && !birth.After(now.UTC())
}
func (s *Service) finalize(a *Attempt) error {
	status, hint := a.Terminal, a.Hint
	if a.FinishedAt.IsZero() {
		a.FinishedAt = s.now()
	}
	userID, tokenKey, authID, personal := a.UserID, "", "", ""
	err := s.App.RunInTransaction(func(tx core.App) error {
		if status == "accepted" {
			if a.Result == nil || !validIdentity(a.Result.PersonalNumber, s.now()) {
				status, hint = "rejected", "invalidIdentity"
			} else if !s.now().Before(a.ExpiresAt) {
				status, hint = "expired", "expiredTransaction"
			} else {
				personal = a.Result.PersonalNumber
				if a.Purpose == "guardian" {
					request, err := tx.FindRecordById("guardian_requests", a.GuardianRequestID)
					if err != nil {
						return err
					}
					user, err := tx.FindRecordById("users", request.GetString("user"))
					if err != nil {
						return err
					}
					if personal != request.GetString("personalNumber") || !adult(personal, s.now()) {
						status, hint = "rejected", "guardianMismatch"
					} else if !s.activeConsent(tx, user) || atLeastAge(user.GetString("personalNumber"), s.now(), 15) {
						status, hint = "rejected", "requestUnavailable"
					}
				} else if !atLeastAge(personal, s.now(), 10) || atLeastAge(personal, s.now(), 31) {
					status, hint = "rejected", "ageIneligible"
				} else {
					user, err := tx.FindFirstRecordByData("users", "personalNumber", personal)
					if errors.Is(err, sql.ErrNoRows) {
						user, err = newRecord(tx, "users")
						if err != nil {
							return err
						}
						user.Set("personalNumber", personal)
						user.SetPassword(randomSecret())
						if err = tx.Save(user); err != nil {
							return err
						}
					} else if err != nil {
						return err
					}
					userID, tokenKey = user.Id, user.TokenKey()
				}
			}
		}
		r, err := newRecord(tx, "authentications")
		if err != nil {
			return err
		}
		r.Set("attemptId", a.ID)
		r.Set("purpose", a.Purpose)
		r.Set("user", userID)
		r.Set("outcome", status)
		r.Set("reason", hint)
		r.Set("startedAt", a.StartedAt.Unix())
		r.Set("receivedAt", a.FinishedAt.Unix())
		if a.Order.OrderRef != "" {
			r.Set("orderHash", hash(a.Order.OrderRef))
		}
		if a.Result != nil {
			r.Set("providerStatus", a.Result.Status)
		}
		if err = tx.Save(r); err != nil {
			return err
		}
		evidence := AuthenticationEvidence{Provider: "kontoapi", OrderRef: a.Order.OrderRef, Result: a.Result, Error: a.LocalError, ReceivedAt: a.FinishedAt.UTC().Format(time.RFC3339Nano)}
		cipher, err := s.Config.Seal("authentication:"+r.Id, evidence)
		if err != nil {
			return err
		}
		r.Set("evidenceCipher", cipher)
		if err = tx.Save(r); err != nil {
			return err
		}
		authID = r.Id
		return nil
	})
	if err != nil {
		return err
	}
	a.Status, a.Hint, a.UserID, a.TokenKey, a.AuthenticationID, a.PersonalNumber = status, hint, userID, tokenKey, authID, personal
	return nil
}
func latestConsent(app core.App, userID string) (*core.Record, error) {
	records, err := app.FindRecordsByFilter("consent_acceptances", "user={:user}", "-acceptedAt,-created,-id", 1, 0, dbx.Params{"user": userID})
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, sql.ErrNoRows
	}
	return records[0], nil
}
func (s *Service) activeAcceptance(app core.App, user *core.Record) (*core.Record, error) {
	doc, err := currentDocument(app)
	if err != nil {
		return nil, err
	}
	records, err := app.FindRecordsByFilter("consent_acceptances", "user={:user} && version={:version} && withdrawnAt=0", "-acceptedAt,-created,-id", 1, 0, dbx.Params{"user": user.Id, "version": doc.Id})
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, sql.ErrNoRows
	}
	r := records[0]
	var evidence ConsentEvidence
	if s.Config.Open("consent:"+r.Id, r.GetString("evidenceCipher"), &evidence) != nil || evidence.Method != "inAppAcceptance" || evidence.ParticipantID != user.Id || evidence.PersonalNumber != user.GetString("personalNumber") || evidence.Document != documentDTO(doc) || evidence.AuthenticationID != r.GetString("authentication") || evidence.AcceptedAt != int64(r.GetInt("acceptedAt")) {
		return nil, errors.New("invalid consent evidence")
	}
	return r, nil
}
func (s *Service) activeConsent(app core.App, user *core.Record) bool {
	_, err := s.activeAcceptance(app, user)
	return err == nil
}

type AcceptanceInput struct {
	Version      string `json:"consentTextId"`
	DocumentHash string `json:"documentHash"`
	RequestID    string `json:"acceptanceRequestId"`
}

func (s *Service) acceptConsent(user *core.Record, authentication string, in AcceptanceInput) (*core.Record, error) {
	if !secretPattern.MatchString(in.RequestID) {
		return nil, problem(400, "invalidRequest", "A consent request ID is required.")
	}
	var acceptance *core.Record
	err := s.App.RunInTransaction(func(tx core.App) error {
		// Token revocation may have raced with middleware verification.
		fresh, err := tx.FindRecordById("users", user.Id)
		if err != nil {
			return err
		}
		if fresh.TokenKey() != user.TokenKey() {
			return problem(401, "sessionExpired", "Authenticate again before accepting consent.")
		}
		doc, err := currentDocument(tx)
		if err != nil {
			return err
		}
		if doc.Id != in.Version || doc.GetString("documentHash") != in.DocumentHash {
			return problem(409, "consentChanged", "The consent changed. Read the current version before accepting.")
		}
		auth, err := tx.FindRecordById("authentications", authentication)
		if err != nil || auth.GetString("user") != user.Id || auth.GetString("purpose") != "participant" || auth.GetString("outcome") != "accepted" || !s.now().Before(time.Unix(int64(auth.GetInt("receivedAt")), 0).Add(SessionLifetime)) {
			return problem(401, "sessionExpired", "Authenticate again before accepting consent.")
		}
		key := user.Id + ":" + in.RequestID
		existing, err := tx.FindFirstRecordByData("consent_acceptances", "requestKey", key)
		if err == nil {
			if existing.GetString("version") != in.Version || existing.GetString("documentHash") != in.DocumentHash || existing.GetInt("withdrawnAt") != 0 {
				return problem(409, "requestChanged", "Start a new consent acceptance.")
			}
			acceptance = existing
			return nil
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if current, err := s.activeAcceptance(tx, user); err == nil {
			acceptance = current
			return nil
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		r, err := newRecord(tx, "consent_acceptances")
		if err != nil {
			return err
		}
		now := s.now().Unix()
		r.Set("user", user.Id)
		r.Set("authentication", authentication)
		r.Set("version", doc.Id)
		r.Set("documentHash", in.DocumentHash)
		r.Set("requestKey", key)
		r.Set("acceptedAt", now)
		if err = tx.Save(r); err != nil {
			return err
		}
		evidence := ConsentEvidence{Method: "inAppAcceptance", Document: documentDTO(doc), AuthenticationID: authentication, PersonalNumber: user.GetString("personalNumber"), ParticipantID: user.Id, AcceptedAt: now}
		cipher, err := s.Config.Seal("consent:"+r.Id, evidence)
		if err != nil {
			return err
		}
		r.Set("evidenceCipher", cipher)
		if err = tx.Save(r); err != nil {
			return err
		}
		acceptance = r
		return nil
	})
	return acceptance, err
}
