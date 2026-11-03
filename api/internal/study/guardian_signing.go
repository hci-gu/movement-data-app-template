package study

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"app/internal/bankid"

	"github.com/pocketbase/pocketbase/core"
)

func (s *Service) startGuardian(ctx context.Context, request *core.Record, ip, userAgent string) (*Attempt, error) {
	if s.provider == nil {
		return nil, problem(503, "signingUnavailable", "BankID är inte tillgängligt just nu.")
	}
	if s.Config.PublicURL == "" {
		return nil, problem(503, "publicURLMissing", "Signeringslänken kunde inte öppnas. Kontakta forskargruppen.")
	}
	s.startMu.Lock()
	defer s.startMu.Unlock()
	attempts := s.snapshotAttempts()
	for _, candidate := range attempts {
		candidate.mu.Lock()
		reusable := candidate.GuardianRequestID == request.Id && candidate.Status == "pending" && s.now().Before(candidate.ExpiresAt)
		matchesIP := candidate.IP == ip
		candidate.mu.Unlock()
		if reusable && !matchesIP {
			return nil, problem(409, "requestInProgress", "Signeringslänken är redan öppen på en annan enhet. Försök igen senare.")
		}
		if reusable {
			return candidate, nil
		}
	}
	if len(attempts) >= MaxAttempts {
		return nil, problem(503, "busy", "Vänta en stund innan du startar en ny BankID-förfrågan.")
	}
	user, err := s.App.FindRecordById("users", request.GetString("user"))
	if err != nil {
		return nil, err
	}
	if !s.activeConsent(s.App, user) || atLeastAge(user.GetString("personalNumber"), s.now(), 15) {
		return nil, problem(409, "requestUnavailable", "Vårdnadshavares samtycke behövs inte längre.")
	}
	doc, err := currentDocument(s.App)
	if err != nil {
		return nil, err
	}
	secret := randomSecret()
	a := &Attempt{ID: attemptID(secret), SecretHash: hash(secret), Purpose: "guardian", Mode: "sameDevice", GuardianRequestID: request.Id, UserID: user.Id, IP: ip, Nonce: randomSecret(), Status: "pending", StartedAt: s.now(), ExpiresAt: s.now().Add(AttemptLifetime)}
	d := documentDTO(doc)
	a.Document = &d
	public, err := url.Parse(s.Config.PublicURL)
	if err != nil || public.Scheme != "https" || public.Hostname() == "" {
		return nil, problem(503, "publicURLInvalid", "Signeringslänkens adress kunde inte verifieras. Kontakta forskargruppen.")
	}
	if len(userAgent) > 512 {
		userAgent = userAgent[:512]
	}
	a.Request = bankid.Request{EndUserIP: ip, ReturnRisk: true, Web: &bankid.WebInfo{ReferringDomain: strings.ToLower(public.Hostname()), UserAgent: userAgent}, ReturnURL: s.Config.PublicURL + "/guardian/" + request.Id + "/return?nonce=" + url.QueryEscape(a.Nonce), Requirement: &bankid.Requirement{PersonalNumber: request.GetString("personalNumber")}}
	a.Request.UserVisibleData = base64.StdEncoding.EncodeToString([]byte(guardianVisibleText(user, doc)))
	if len(a.Request.UserVisibleData) > 40000 {
		return nil, problem(503, "consentUnavailable", "Samtycket kunde inte förberedas för BankID. Kontakta forskargruppen.")
	}
	manifest, err := json.Marshal(map[string]any{"schemaVersion": 2, "purpose": "guardian", "requestId": request.Id, "userId": user.Id, "consentTextId": doc.Id, "documentHash": doc.GetString("documentHash")})
	if err != nil {
		return nil, err
	}
	a.Request.UserNonVisibleData = base64.StdEncoding.EncodeToString(manifest)
	order, err := s.provider.Start(ctx, "sign", a.Request)
	if err != nil {
		var apiErr *bankid.APIError
		if errors.As(err, &apiErr) {
			return nil, problem(502, "bankIDRejected", "BankID rejected the guardian request ("+apiErr.Code+").")
		}
		return nil, problem(503, "signingUnavailable", "BankID kunde inte startas. Försök igen.")
	}
	a.Order = order
	a.ReceivedAt = s.now()
	a.NextCollect = s.now().Add(2 * time.Second)
	a.Hint = "outstandingTransaction"
	s.mu.Lock()
	s.attempts[a.ID] = a
	s.mu.Unlock()
	return a, nil
}

func (s *Service) guardianCompletion(tx core.App, a *Attempt, completion *bankid.Completion) (string, string, *core.Record, error) {
	request, err := tx.FindRecordById("signingRequests", a.GuardianRequestID)
	if err != nil {
		return "rejected", "requestUnavailable", nil, err
	}
	user, err := tx.FindRecordById("users", request.GetString("user"))
	if err != nil {
		return "rejected", "requestUnavailable", nil, err
	}
	if !s.activeConsent(tx, user) {
		return "rejected", "requestUnavailable", user, nil
	}
	if completion == nil || subtle.ConstantTimeCompare([]byte(completion.User.PersonalNumber), []byte(request.GetString("personalNumber"))) != 1 {
		return "rejected", "guardianMismatch", user, nil
	}
	if !adult(completion.User.PersonalNumber, s.now()) {
		return "rejected", "guardianUnderage", user, nil
	}
	doc, err := currentDocument(tx)
	if err != nil {
		return "rejected", "consentChanged", user, nil
	}
	if a.Document == nil || a.Document.ID != doc.Id {
		return "rejected", "consentChanged", user, nil
	}
	if s.guardianSigned(tx, request, doc.Id) {
		return "rejected", "alreadySigned", user, nil
	}
	return "accepted", "", user, nil
}
