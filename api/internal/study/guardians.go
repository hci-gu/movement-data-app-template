package study

import (
	"app/internal/bankid"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
	qrcode "github.com/skip2/go-qrcode"
)

var signingRequestIDPattern = regexp.MustCompile(`^[0-9]{3}-[0-9]{3}$`)

func generateSigningRequestID(app core.App) (string, error) {
	for range 100 {
		n, err := rand.Int(rand.Reader, big.NewInt(1000000))
		if err != nil {
			return "", err
		}
		id := fmt.Sprintf("%03d-%03d", n.Int64()/1000, n.Int64()%1000)
		if _, err := app.FindRecordById("signingRequests", id); errors.Is(err, sql.ErrNoRows) {
			return id, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", problem(503, "busy", "No signing request ID is available. Please try again.")
}

func adult(personalNumber string, now time.Time) bool {
	if !personalNumberPattern.MatchString(personalNumber) {
		return false
	}
	year, _ := strconv.Atoi(personalNumber[:4])
	month, _ := strconv.Atoi(personalNumber[4:6])
	day, _ := strconv.Atoi(personalNumber[6:8])
	birth := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if birth.Year() != year || int(birth.Month()) != month || birth.Day() != day || birth.After(now.UTC()) {
		return false
	}
	return !now.UTC().Before(birth.AddDate(18, 0, 0))
}

func (s *Service) guardianRequests(app core.App, userID string) ([]*core.Record, error) {
	return app.FindRecordsByFilter("signingRequests", "user={:user}", "slot", 3, 0, dbx.Params{"user": userID})
}

func (s *Service) guardianSigned(app core.App, request *core.Record, version string) bool {
	id := request.GetString("signature")
	if id == "" {
		return false
	}
	sig, err := app.FindRecordById("signatures", id)
	if err != nil || sig.GetString("user") != request.GetString("user") || sig.GetString("purpose") != "guardian" || sig.GetString("outcome") != "accepted" || sig.GetString("version") != version || sig.GetString("evidenceCipher") == "" {
		return false
	}
	var evidence Evidence
	if s.Config.Open("signature:"+sig.Id, sig.GetString("evidenceCipher"), &evidence) != nil {
		return false
	}
	doc, err := app.FindRecordById("consent_texts", version)
	if err != nil || evidence.Document == nil || evidence.Document.ID != doc.Id || evidence.Document.DocumentHash != doc.GetString("documentHash") || evidence.Document.Text != doc.GetString("text") {
		return false
	}
	user, err := app.FindRecordById("users", request.GetString("user"))
	if err != nil || evidence.Request.UserVisibleData != base64.StdEncoding.EncodeToString([]byte(guardianVisibleText(user, doc))) {
		return false
	}
	var result bankid.Result
	return json.Unmarshal(evidence.Completion, &result) == nil && result.CompletionData != nil && result.CompletionData.User.PersonalNumber == request.GetString("personalNumber") && adult(result.CompletionData.User.PersonalNumber, time.Unix(int64(sig.GetInt("receivedAt")), 0)) && result.CompletionData.Risk == "low" && validBase64(result.CompletionData.Signature) && validBase64(result.CompletionData.OCSPResponse)
}

func guardianVisibleText(user, doc *core.Record) string {
	return fmt.Sprintf("I approve participation for the person with personal number %s in %s (consent version %s).\n\n%s", user.GetString("personalNumber"), doc.GetString("title"), doc.GetString("version"), doc.GetString("text"))
}

func (s *Service) guardianEligible(app core.App, user *core.Record) bool {
	if adult(user.GetString("personalNumber"), s.now()) {
		return true
	}
	count := user.GetInt("guardianCount")
	if count < 1 || count > 2 {
		return false
	}
	doc, err := currentDocument(app)
	if err != nil {
		return false
	}
	requests, err := s.guardianRequests(app, user.Id)
	if err != nil || len(requests) != count {
		return false
	}
	for _, request := range requests {
		if !s.guardianSigned(app, request, doc.Id) {
			return false
		}
	}
	return true
}

func (s *Service) guardianStatus(user *core.Record) (map[string]any, error) {
	user, err := s.App.FindRecordById("users", user.Id)
	if err != nil {
		return nil, err
	}
	requests, err := s.guardianRequests(s.App, user.Id)
	if err != nil {
		return nil, err
	}
	doc, err := currentDocument(s.App)
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(requests))
	for _, request := range requests {
		items = append(items, map[string]any{"id": request.Id, "personalNumber": request.GetString("personalNumber"), "signed": s.guardianSigned(s.App, request, doc.Id), "link": s.Config.PublicURL + "/guardian/" + request.Id})
	}
	return map[string]any{"eligible": s.activeConsent(s.App, user) && s.guardianEligible(s.App, user), "adult": adult(user.GetString("personalNumber"), s.now()), "guardianCount": user.GetInt("guardianCount"), "requests": items}, nil
}

func (s *Service) createGuardianRequest(user *core.Record, personalNumber string, count int) (map[string]any, error) {
	if !personalNumberPattern.MatchString(personalNumber) || !adult(personalNumber, s.now()) || personalNumber == user.GetString("personalNumber") || count < 1 || count > 2 {
		return nil, problem(400, "invalidGuardian", "Enter an adult guardian's 12-digit personal number and choose one or two guardians.")
	}
	if adult(user.GetString("personalNumber"), s.now()) {
		return nil, problem(409, "alreadyEligible", "Guardian permission is not needed.")
	}
	var created *core.Record
	err := s.App.RunInTransaction(func(tx core.App) error {
		fresh, err := tx.FindRecordById("users", user.Id)
		if err != nil {
			return err
		}
		if !s.activeConsent(tx, fresh) {
			return problem(403, "consentRequired", "Sign the current study consent first.")
		}
		requests, err := s.guardianRequests(tx, fresh.Id)
		if err != nil {
			return err
		}
		configured := fresh.GetInt("guardianCount")
		if configured != 0 && configured != count {
			return problem(409, "guardianCountChanged", "The guardian count is already set for this participant.")
		}
		if len(requests) >= count {
			return problem(409, "guardianLimit", "All guardian links have already been created.")
		}
		for _, request := range requests {
			if request.GetString("personalNumber") == personalNumber {
				return problem(409, "duplicateGuardian", "This guardian already has a link.")
			}
		}
		if configured == 0 {
			fresh.Set("guardianCount", count)
			if err := tx.Save(fresh); err != nil {
				return err
			}
		}
		created, err = newRecord(tx, "signingRequests")
		if err != nil {
			return err
		}
		id, err := generateSigningRequestID(tx)
		if err != nil {
			return err
		}
		created.Set("id", id)
		created.Set("user", fresh.Id)
		created.Set("slot", len(requests)+1)
		created.Set("personalNumber", personalNumber)
		return tx.Save(created)
	})
	if err != nil {
		return nil, err
	}
	return s.guardianStatus(user)
}

func (s *Service) registerGuardianRoutes(r *router.Router[*core.RequestEvent], g *router.RouterGroup[*core.RequestEvent]) {
	g.GET("/guardians", func(e *core.RequestEvent) error {
		status, err := s.guardianStatus(e.Auth)
		if err != nil {
			return err
		}
		return e.JSON(200, status)
	}).BindFunc(s.RequireSession)
	g.POST("/guardians", func(e *core.RequestEvent) error {
		var input struct {
			PersonalNumber string `json:"personalNumber"`
			GuardianCount  int    `json:"guardianCount"`
		}
		if err := e.BindBody(&input); err != nil {
			return problem(400, "invalidRequest", "Invalid guardian request.")
		}
		status, err := s.createGuardianRequest(e.Auth, input.PersonalNumber, input.GuardianCount)
		if err != nil {
			return err
		}
		return e.JSON(201, status)
	}).BindFunc(s.RequireSession)
	r.GET("/guardian/{id}", func(e *core.RequestEvent) error {
		s.guardianHeaders(e)
		id := e.Request.PathValue("id")
		if !signingRequestIDPattern.MatchString(id) {
			return e.NoContent(404)
		}
		request, err := s.App.FindRecordById("signingRequests", id)
		if err != nil {
			return e.NoContent(404)
		}
		doc, err := currentDocument(s.App)
		if err != nil {
			return problem(503, "consentUnavailable", "Study consent is not available yet.")
		}
		if s.guardianSigned(s.App, request, doc.Id) {
			return guardianApprovedPage(e)
		}
		ip, err := s.Config.endUserIP(e.Request)
		if err != nil {
			return problem(400, "invalidIP", "Unable to establish your connection.")
		}
		a, err := s.startGuardian(e.Request.Context(), request, ip, e.Request.UserAgent())
		if err != nil {
			return err
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.Status != "pending" {
			return e.Redirect(http.StatusFound, "/guardian/"+id+"/return?nonce="+url.QueryEscape(a.Nonce))
		}
		return guardianLaunchPage(e, a)
	}).BindFunc(s.throttle)
	r.GET("/guardian/{id}/state", func(e *core.RequestEvent) error {
		s.guardianHeaders(e)
		a, err := s.guardianPageAttempt(e)
		if err != nil {
			return err
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		return e.JSON(200, map[string]any{"status": a.Status, "hintCode": a.Hint, "pickedUp": a.PickedUp, "qrAvailable": guardianQRAvailable(a, s.now())})
	}).BindFunc(s.throttle)
	r.GET("/guardian/{id}/qr", func(e *core.RequestEvent) error {
		s.guardianHeaders(e)
		a, err := s.guardianPageAttempt(e)
		if err != nil {
			return err
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		if !guardianQRAvailable(a, s.now()) {
			return e.NoContent(404)
		}
		png, err := qrcode.Encode(bankid.QRPayload(a.Order, a.ReceivedAt, s.now()), qrcode.Medium, 320)
		if err != nil {
			return err
		}
		e.Response.Header().Set("Content-Type", "image/png")
		e.Response.WriteHeader(http.StatusOK)
		_, err = e.Response.Write(png)
		return err
	}).BindFunc(s.throttle)
	r.POST("/guardian/{id}/restart", func(e *core.RequestEvent) error {
		s.guardianHeaders(e)
		a, err := s.guardianPageAttempt(e)
		if err != nil {
			return err
		}
		a.mu.Lock()
		err = s.cancel(e.Request.Context(), a)
		a.mu.Unlock()
		if err != nil {
			return err
		}
		return e.Redirect(http.StatusSeeOther, "/guardian/"+e.Request.PathValue("id"))
	}).BindFunc(s.throttle)
	r.GET("/guardian/{id}/return", func(e *core.RequestEvent) error {
		s.guardianHeaders(e)
		id := e.Request.PathValue("id")
		if !signingRequestIDPattern.MatchString(id) {
			return e.NoContent(404)
		}
		request, err := s.App.FindRecordById("signingRequests", id)
		if err != nil {
			return e.NoContent(404)
		}
		ip, err := s.Config.endUserIP(e.Request)
		if err != nil || !s.guardianReturnMatches(request.Id, e.Request.URL.Query().Get("nonce"), ip) {
			return problem(400, "invalidReturn", "This BankID return does not match the signing request.")
		}
		doc, err := currentDocument(s.App)
		if err != nil {
			return problem(503, "consentUnavailable", "Study consent is not available yet.")
		}
		if s.guardianSigned(s.App, request, doc.Id) {
			return guardianApprovedPage(e)
		}
		return guardianCheckingPage(e)
	}).BindFunc(s.throttle)
}

func guardianQRAvailable(a *Attempt, now time.Time) bool {
	return a.Status == "pending" && !a.PickedUp && !a.ReceivedAt.IsZero() && now.Sub(a.ReceivedAt) < 30*time.Second
}

func (s *Service) guardianPageAttempt(e *core.RequestEvent) (*Attempt, error) {
	id := e.Request.PathValue("id")
	if !signingRequestIDPattern.MatchString(id) {
		return nil, problem(404, "invalidRequest", "Signing request not found.")
	}
	ip, err := s.Config.endUserIP(e.Request)
	if err != nil {
		return nil, problem(400, "invalidIP", "Unable to establish your connection.")
	}
	a := s.guardianAttempt(id, e.Request.URL.Query().Get("nonce"), ip)
	if a == nil {
		return nil, problem(404, "invalidRequest", "Signing request not found or expired.")
	}
	return a, nil
}

func (s *Service) guardianHeaders(e *core.RequestEvent) {
	e.Response.Header().Set("Cache-Control", "no-store")
	e.Response.Header().Set("Referrer-Policy", "no-referrer")
	e.Response.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
}

func (s *Service) guardianReturnMatches(requestID, nonce, ip string) bool {
	return s.guardianAttempt(requestID, nonce, ip) != nil
}

func (s *Service) guardianAttempt(requestID, nonce, ip string) *Attempt {
	if nonce == "" || ip == "" {
		return nil
	}
	s.mu.Lock()
	attempts := make([]*Attempt, 0, len(s.attempts))
	for _, candidate := range s.attempts {
		attempts = append(attempts, candidate)
	}
	s.mu.Unlock()
	for _, candidate := range attempts {
		candidate.mu.Lock()
		matches := candidate.GuardianRequestID == requestID && candidate.IP == ip && subtle.ConstantTimeCompare([]byte(candidate.Nonce), []byte(nonce)) == 1
		candidate.mu.Unlock()
		if matches {
			return candidate
		}
	}
	return nil
}

func (s *Service) startGuardian(ctx context.Context, request *core.Record, ip, userAgent string) (*Attempt, error) {
	if s.provider == nil {
		return nil, problem(503, "signingUnavailable", "BankID is currently unavailable.")
	}
	if s.Config.PublicURL == "" {
		return nil, problem(503, "publicURLMissing", "Set BANKID_PUBLIC_URL to the API's public HTTPS origin before opening guardian links.")
	}
	s.startMu.Lock()
	defer s.startMu.Unlock()
	s.mu.Lock()
	attempts := make([]*Attempt, 0, len(s.attempts))
	for _, candidate := range s.attempts {
		attempts = append(attempts, candidate)
	}
	s.mu.Unlock()
	for _, candidate := range attempts {
		candidate.mu.Lock()
		reusable := candidate.GuardianRequestID == request.Id && candidate.Status == "pending" && s.now().Before(candidate.ExpiresAt)
		matchesIP := candidate.IP == ip
		candidate.mu.Unlock()
		if reusable && !matchesIP {
			return nil, problem(409, "requestInProgress", "This signing link is already open on another device. Try again later.")
		}
		if reusable {
			return candidate, nil
		}
	}
	if len(attempts) >= MaxAttempts {
		return nil, problem(503, "busy", "Please wait before starting another BankID request.")
	}
	user, err := s.App.FindRecordById("users", request.GetString("user"))
	if err != nil {
		return nil, err
	}
	if !s.activeConsent(s.App, user) || adult(user.GetString("personalNumber"), s.now()) {
		return nil, problem(409, "requestUnavailable", "Guardian permission is no longer needed.")
	}
	doc, err := currentDocument(s.App)
	if err != nil {
		return nil, err
	}
	secret := randomSecret()
	a := &Attempt{ID: attemptID(secret), SecretHash: hash(secret), Purpose: "guardian", Mode: "sameDevice", GuardianRequestID: request.Id, UserID: user.Id, IP: ip, Nonce: randomSecret(), Status: "pending", StartedAt: s.now(), ExpiresAt: s.now().Add(AttemptLifetime)}
	d := documentDTO(doc)
	a.Document = &d
	public, _ := url.Parse(s.Config.PublicURL)
	if len(userAgent) > 512 {
		userAgent = userAgent[:512]
	}
	a.Request = bankid.Request{EndUserIP: ip, ReturnRisk: true, Web: &bankid.WebInfo{ReferringDomain: strings.ToLower(public.Hostname()), UserAgent: userAgent}, ReturnURL: s.Config.PublicURL + "/guardian/" + request.Id + "/return?nonce=" + url.QueryEscape(a.Nonce), Requirement: &bankid.Requirement{PersonalNumber: request.GetString("personalNumber")}}
	a.Request.UserVisibleData = base64.StdEncoding.EncodeToString([]byte(guardianVisibleText(user, doc)))
	if len(a.Request.UserVisibleData) > 40000 {
		return nil, problem(503, "consentUnavailable", "The consent text is too long for a guardian BankID signature.")
	}
	manifest, _ := json.Marshal(map[string]any{"schemaVersion": 2, "purpose": "guardian", "requestId": request.Id, "userId": user.Id, "consentTextId": doc.Id, "documentHash": doc.GetString("documentHash")})
	a.Request.UserNonVisibleData = base64.StdEncoding.EncodeToString(manifest)
	order, err := s.provider.Start(ctx, "sign", a.Request)
	if err != nil {
		var apiErr *bankid.APIError
		if errors.As(err, &apiErr) {
			return nil, problem(502, "bankIDRejected", "BankID rejected the guardian request ("+apiErr.Code+").")
		}
		return nil, problem(503, "signingUnavailable", "BankID could not start. Please try again.")
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
