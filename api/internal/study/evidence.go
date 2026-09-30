package study

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"app/internal/bankid"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

type Evidence struct {
	Request       bankid.Request  `json:"request"`
	Document      *Document       `json:"document,omitempty"`
	Completion    json.RawMessage `json:"completion,omitempty"`
	ErrorResponse json.RawMessage `json:"errorResponse,omitempty"`
	LocalError    string          `json:"localError,omitempty"`
	ReceivedAt    string          `json:"receivedAt"`
}

func (s *Service) finalize(a *Attempt) error {
	outcome, hint := a.Terminal, a.Hint
	userID, tokenKey, signatureID := a.UserID, "", ""
	if a.ResultAt.IsZero() {
		a.ResultAt = s.now()
	}
	finished := a.ResultAt
	err := s.App.RunInTransaction(func(tx core.App) error {
		var user *core.Record
		var err error
		if outcome == "accepted" {
			completion := a.Result.CompletionData
			if completion == nil || !personalNumberPattern.MatchString(completion.User.PersonalNumber) || !validBase64(completion.Signature) || !validBase64(completion.OCSPResponse) {
				outcome, hint = "rejected", "invalidEvidence"
			} else if completion.Risk != "low" {
				outcome, hint = "rejected", "riskRejected"
			} else if !s.now().Before(a.ExpiresAt) {
				outcome, hint = "rejected", "sessionExpired"
			}
			if outcome == "accepted" && a.Purpose == "guardian" {
				outcome, hint, user, err = s.guardianCompletion(tx, a, completion)
				if err != nil {
					return err
				}
			} else if outcome == "accepted" {
				doc, err := currentDocument(tx)
				if err != nil {
					return err
				}
				if a.Document == nil || doc.Id != a.Document.ID || doc.GetString("documentHash") != a.Document.DocumentHash {
					outcome, hint = "rejected", "consentChanged"
				} else {
					user, err = tx.FindFirstRecordByData("users", "personalNumber", completion.User.PersonalNumber)
					if errors.Is(err, sql.ErrNoRows) {
						user, err = newRecord(tx, "users")
						if err != nil {
							return err
						}
						user.Set("personalNumber", completion.User.PersonalNumber)
						user.SetPassword(randomSecret()) // PocketBase requires an internal password; password login stays disabled.
						if err := tx.Save(user); err != nil {
							return err
						}
					} else if err != nil {
						return err
					}
				}
			}
		}
		r, err := newRecord(tx, "signatures")
		if err != nil {
			return err
		}
		r.Set("attemptId", a.ID)
		r.Set("purpose", a.Purpose)
		if a.Order.OrderRef != "" {
			r.Set("orderHash", hash(a.Order.OrderRef))
		}
		r.Set("outcome", outcome)
		r.Set("reason", hint)
		r.Set("startedAt", a.StartedAt.Unix())
		r.Set("receivedAt", finished.Unix())
		if a.Document != nil {
			r.Set("version", a.Document.ID)
		}
		if user != nil {
			r.Set("user", user.Id)
		}
		evidence := Evidence{Request: a.Request, Document: a.Document, ErrorResponse: a.ErrorResponse, LocalError: a.LocalError, ReceivedAt: finished.UTC().Format(time.RFC3339Nano)}
		if a.Result != nil {
			r.Set("providerStatus", a.Result.Status)
			evidence.Completion = a.Result.Raw
			if len(evidence.Completion) == 0 {
				evidence.Completion, err = json.Marshal(a.Result)
				if err != nil {
					return err
				}
			}
		}
		// Allocate the evidence record's ID inside the same transaction.
		if err := tx.Save(r); err != nil {
			return err
		}
		cipher, err := s.Config.Seal("signature:"+r.Id, evidence)
		if err != nil {
			return err
		}
		r.Set("evidenceCipher", cipher)
		if err := tx.Save(r); err != nil {
			return err
		}
		if outcome == "accepted" {
			if a.Purpose == "guardian" {
				request, err := tx.FindRecordById("signingRequests", a.GuardianRequestID)
				if err != nil {
					return err
				}
				request.Set("signature", r.Id)
				if err := tx.Save(request); err != nil {
					return err
				}
			}
			userID = user.Id
			tokenKey = user.TokenKey()
		}
		signatureID = r.Id
		return nil
	})
	if err != nil {
		return err
	}
	a.Status = outcome
	a.Hint = hint
	a.UserID = userID
	a.SignatureID = signatureID
	a.TokenKey = tokenKey
	a.FinishedAt = finished
	return nil
}
func validBase64(value string) bool {
	raw, err := base64.StdEncoding.DecodeString(value)
	return err == nil && len(raw) > 0
}
func latestSignature(app core.App, userID string) (*core.Record, error) {
	records, err := app.FindRecordsByFilter("signatures", "user={:user} && purpose='sign' && outcome='accepted'", "-created,-id", 1, 0, dbx.Params{"user": userID})
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, sql.ErrNoRows
	}
	return records[0], nil
}
func (s *Service) activeConsent(app core.App, user *core.Record) bool {
	doc, err := currentDocument(app)
	if err != nil {
		return false
	}
	r, err := latestSignature(app, user.Id)
	valid := err == nil && r.GetString("purpose") == "sign" && r.GetString("outcome") == "accepted" && r.GetString("user") == user.Id && r.GetString("version") == doc.Id && r.GetInt("withdrawnAt") == 0 && r.GetString("evidenceCipher") != ""
	if !valid {
		return false
	}
	var evidence Evidence
	if err := s.Config.Open("signature:"+r.Id, r.GetString("evidenceCipher"), &evidence); err != nil {
		return false
	}
	var result bankid.Result
	if json.Unmarshal(evidence.Completion, &result) != nil || result.CompletionData == nil || evidence.Document == nil {
		return false
	}
	return evidence.Document.ID == doc.Id && evidence.Document.DocumentHash == doc.GetString("documentHash") && evidence.Document.Text == doc.GetString("text") && evidence.Request.UserVisibleData == base64.StdEncoding.EncodeToString([]byte(doc.GetString("text"))) && result.CompletionData.Risk == "low" && validBase64(result.CompletionData.Signature) && validBase64(result.CompletionData.OCSPResponse)
}
