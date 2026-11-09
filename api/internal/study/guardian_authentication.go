package study

import (
	"context"
	"database/sql"
	"errors"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// The caller places the returned credential in a request-scoped HttpOnly cookie.
func (s *Service) startGuardian(ctx context.Context, request *core.Record, ip string) (*Attempt, string, error) {
	s.startMu.Lock()
	defer s.startMu.Unlock()
	user, err := s.App.FindRecordById("users", request.GetString("user"))
	if err != nil {
		return nil, "", err
	}
	if !s.activeConsent(s.App, user) || atLeastAge(user.GetString("personalNumber"), s.now(), 15) {
		return nil, "", problem(409, "requestUnavailable", "Vårdnadshavares samtycke behövs inte längre.")
	}
	secret := randomSecret()
	a := &Attempt{ID: attemptID(secret), SecretHash: hash(secret), Purpose: "guardian", Mode: "qr", GuardianRequestID: request.Id, UserID: user.Id, IP: ip, Nonce: randomSecret(), Status: "pending", StartedAt: s.now(), ExpiresAt: s.now().Add(AttemptLifetime)}
	a, err = s.startOrder(ctx, a)
	if err != nil {
		return nil, "", err
	}
	return a, a.ID + "." + secret, nil
}
func (s *Service) approveGuardian(a *Attempt, in AcceptanceInput, consentID string) error {
	if a.Status != "accepted" || a.Purpose != "guardian" || !s.alive(a) {
		return restartRequired()
	}
	return s.App.RunInTransaction(func(tx core.App) error {
		request, err := tx.FindRecordById("guardian_requests", a.GuardianRequestID)
		if err != nil {
			return err
		}
		if a.PersonalNumber != request.GetString("personalNumber") || !adult(a.PersonalNumber, s.now()) {
			return problem(403, "guardianMismatch", "Identifiera dig som den angivna vårdnadshavaren.")
		}
		user, err := tx.FindRecordById("users", request.GetString("user"))
		if err != nil {
			return err
		}
		consent, err := s.activeAcceptance(tx, user)
		if err != nil || consent.Id != consentID || atLeastAge(user.GetString("personalNumber"), s.now(), 15) {
			return problem(409, "consentChanged", "Deltagarens samtycke har ändrats. Ladda om inbjudan.")
		}
		doc, err := currentDocument(tx)
		if err != nil {
			return err
		}
		if doc.Id != in.Version || doc.GetString("documentHash") != in.DocumentHash {
			return problem(409, "consentChanged", "Läs det aktuella samtycket innan du bekräftar.")
		}
		if _, err = tx.FindFirstRecordByFilter("guardian_approvals", "request={:request} && consent={:consent}", dbx.Params{"request": request.Id, "consent": consent.Id}); err == nil {
			return nil
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		r, err := newRecord(tx, "guardian_approvals")
		if err != nil {
			return err
		}
		now := s.now().Unix()
		r.Set("request", request.Id)
		r.Set("consent", consent.Id)
		r.Set("authentication", a.AuthenticationID)
		r.Set("requestKey", a.AuthenticationID+":"+consent.Id)
		r.Set("acceptedAt", now)
		if err = tx.Save(r); err != nil {
			return err
		}
		evidence := ConsentEvidence{Method: "guardianWebApproval", Document: documentDTO(doc), AuthenticationID: a.AuthenticationID, PersonalNumber: a.PersonalNumber, ParticipantID: user.Id, GuardianRequestID: request.Id, ParticipantConsentID: consent.Id, AcceptedAt: now}
		cipher, err := s.Config.Seal("guardian:"+r.Id, evidence)
		if err != nil {
			return err
		}
		r.Set("evidenceCipher", cipher)
		return tx.Save(r)
	})
}
