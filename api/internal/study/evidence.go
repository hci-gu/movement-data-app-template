package study

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"app/internal/bankid"
	"app/internal/testidentity"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

type Evidence struct {
	SimulationRun string                `json:"simulationRun,omitempty"`
	Simulation    *testidentity.Mapping `json:"simulation,omitempty"`
	Request       bankid.Request        `json:"request"`
	Document      *Document             `json:"document,omitempty"`
	Completion    json.RawMessage       `json:"completion,omitempty"`
	ErrorResponse json.RawMessage       `json:"errorResponse,omitempty"`
	LocalError    string                `json:"localError,omitempty"`
	ReceivedAt    string                `json:"receivedAt"`
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
			var completion *bankid.Completion
			if a.Result != nil {
				completion = a.Result.CompletionData
			}
			if completion == nil || !personalNumberPattern.MatchString(completion.User.PersonalNumber) || !validBase64(completion.Signature) || !validBase64(completion.OCSPResponse) {
				outcome, hint = "rejected", "invalidEvidence"
			} else if completion.Risk != "low" {
				outcome, hint = "rejected", "riskRejected"
			} else if !s.now().Before(a.ExpiresAt) {
				outcome, hint = "rejected", "sessionExpired"
			}
			personal := ""
			if outcome == "accepted" {
				run, snapshotErr := s.mappingSnapshot(tx)
				if snapshotErr != nil {
					return snapshotErr
				}
				personal, a.Simulation, err = run.Resolve(completion.User.PersonalNumber, testidentity.Role(a.Purpose), a.ExpectedGuardian)
				if err != nil {
					outcome, hint = "rejected", "testIdentityUnassigned"
				}
			}
			if outcome == "accepted" && a.Purpose == "guardian" {
				outcome, hint, user, err = s.guardianCompletion(tx, a, personal)
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
					revision := ""
					if a.Simulation != nil {
						revision = a.Simulation.Revision
					}
					user, err = tx.FindFirstRecordByFilter("users", "personalNumber={:personal} && simulationRevision={:revision}", dbx.Params{"personal": personal, "revision": revision})
					if errors.Is(err, sql.ErrNoRows) {
						user, err = newRecord(tx, "users")
						if err != nil {
							return err
						}
						user.Set("personalNumber", personal)
						if a.Simulation != nil {
							user.Set("simulationRun", a.Simulation.RunID)
							user.Set("simulationPersona", a.Simulation.PersonaID)
							user.Set("simulationRevision", a.Simulation.Revision)
						}
						user.SetPassword(randomSecret()) // PocketBase requires an internal password; password login stays disabled.
						if err := tx.Save(user); err != nil {
							return err
						}
					} else if err != nil {
						return err
					}
					if !s.validSimulationUser(user) {
						return errors.New("identity belongs to a different test run")
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
		r.Set("simulationRun", s.simulation.ID())
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
		evidence := Evidence{SimulationRun: s.simulation.ID(), Simulation: a.Simulation, Request: a.Request, Document: a.Document, ErrorResponse: a.ErrorResponse, LocalError: a.LocalError, ReceivedAt: finished.UTC().Format(time.RFC3339Nano)}
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
	if !s.validSimulationUser(user) {
		return false
	}
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
	run, err := s.mappingSnapshot(app)
	return err == nil && run.Check(result.CompletionData.User.PersonalNumber, "participant", user.GetString("personalNumber"), evidence.Simulation) && evidence.Document.ID == doc.Id && evidence.Document.DocumentHash == doc.GetString("documentHash") && evidence.Document.Text == doc.GetString("text") && evidence.Request.UserVisibleData == base64.StdEncoding.EncodeToString([]byte(doc.GetString("text"))) && result.CompletionData.Risk == "low" && validBase64(result.CompletionData.Signature) && validBase64(result.CompletionData.OCSPResponse)
}
