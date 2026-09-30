package study

import (
	"crypto/subtle"
	"net/http"
	"net/url"
	"time"

	"app/internal/bankid"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
	qrcode "github.com/skip2/go-qrcode"
)

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
	for _, candidate := range s.snapshotAttempts() {
		candidate.mu.Lock()
		matches := candidate.GuardianRequestID == requestID && candidate.IP == ip && subtle.ConstantTimeCompare([]byte(candidate.Nonce), []byte(nonce)) == 1
		candidate.mu.Unlock()
		if matches {
			return candidate
		}
	}
	return nil
}
