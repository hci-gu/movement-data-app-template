package study

import (
	"crypto/subtle"
	"encoding/base64"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
	"net/http"
	"strings"
	"time"
)

func guardianCookie(id string) string { return "guardian_" + id }
func (s *Service) registerGuardianRoutes(r *router.Router[*core.RequestEvent], g *router.RouterGroup[*core.RequestEvent]) {
	g.GET("/guardians", func(e *core.RequestEvent) error {
		status, err := s.guardianStatus(e.Auth)
		if err != nil {
			return err
		}
		return e.JSON(200, status)
	}).BindFunc(s.RequireSession)
	g.POST("/guardians", func(e *core.RequestEvent) error {
		var in struct {
			PersonalNumber string `json:"personalNumber"`
			GuardianCount  int    `json:"guardianCount"`
		}
		if err := e.BindBody(&in); err != nil {
			return err
		}
		if s.Config.PublicURL == "" {
			return problem(503, "publicURLMissing", "Länkar till vårdnadshavare är inte konfigurerade.")
		}
		status, err := s.createGuardianRequest(e.Auth, in.PersonalNumber, in.GuardianCount)
		if err != nil {
			return err
		}
		return e.JSON(201, status)
	}).BindFunc(s.RequireSession)
	r.GET("/guardian/{id}", func(e *core.RequestEvent) error {
		s.guardianHeaders(e)
		id := e.Request.PathValue("id")
		if !guardianRequestIDPattern.MatchString(id) {
			return e.NoContent(404)
		}
		request, err := s.App.FindRecordById("guardian_requests", id)
		if err != nil {
			return e.NoContent(404)
		}
		a, err := s.guardianPageAttempt(e)
		if err != nil {
			ip, err := s.Config.endUserIP(e.Request)
			if err != nil {
				return problem(400, "invalidIP", "Anslutningen kunde inte verifieras.")
			}
			var credential string
			a, credential, err = s.startGuardian(e.Request.Context(), request, ip)
			if err != nil {
				return err
			}
			http.SetCookie(e.Response, &http.Cookie{Name: guardianCookie(id), Value: credential, Path: "/guardian/" + id, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: int((AttemptLifetime + ResultLifetime) / time.Second)})
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		return s.guardianView(e, a)
	}).BindFunc(s.throttle)
	r.GET("/guardian/{id}/state", func(e *core.RequestEvent) error {
		s.guardianHeaders(e)
		a, err := s.guardianPageAttempt(e)
		if err != nil {
			return err
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		return e.JSON(200, map[string]any{"status": a.Status, "hintCode": a.Hint, "qrAvailable": a.Status == "pending" && s.now().Before(a.ExpiresAt)})
	}).BindFunc(s.throttle)
	r.GET("/guardian/{id}/qr", func(e *core.RequestEvent) error {
		s.guardianHeaders(e)
		a, err := s.guardianPageAttempt(e)
		if err != nil {
			return err
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		value, err := s.qrImage(e.Request.Context(), a)
		if err != nil {
			return err
		}
		encoded, ok := strings.CutPrefix(value, "data:image/png;base64,")
		if !ok {
			return problem(502, "invalidQR", "QR-koden är ogiltig.")
		}
		raw, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return err
		}
		e.Response.Header().Set("Content-Type", "image/png")
		e.Response.WriteHeader(200)
		_, err = e.Response.Write(raw)
		return err
	}).BindFunc(s.throttle)
	r.POST("/guardian/{id}/restart", func(e *core.RequestEvent) error {
		s.guardianHeaders(e)
		a, err := s.guardianFormAttempt(e)
		if err != nil {
			return err
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		if err = s.cancel(e.Request.Context(), a); err != nil {
			return err
		}
		http.SetCookie(e.Response, &http.Cookie{Name: guardianCookie(a.GuardianRequestID), Value: "", Path: "/guardian/" + a.GuardianRequestID, MaxAge: -1, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
		return e.Redirect(http.StatusSeeOther, "/guardian/"+a.GuardianRequestID)
	}).BindFunc(s.throttle)
	r.GET("/guardian/{id}/return", func(e *core.RequestEvent) error {
		s.guardianHeaders(e)
		a, err := s.guardianPageAttempt(e)
		if err != nil {
			return err
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		return s.guardianView(e, a)
	}).BindFunc(s.throttle)
	r.POST("/guardian/{id}/approve", func(e *core.RequestEvent) error {
		s.guardianHeaders(e)
		a, err := s.guardianFormAttempt(e)
		if err != nil {
			return err
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		if e.Request.PostForm.Get("accept") != "yes" {
			return problem(400, "acceptanceRequired", "Du behöver bekräfta ditt samtycke.")
		}
		in := AcceptanceInput{Version: e.Request.PostForm.Get("consentTextId"), DocumentHash: e.Request.PostForm.Get("documentHash")}
		if err = s.approveGuardian(a, in, e.Request.PostForm.Get("participantConsentId")); err != nil {
			return renderGuardianPage(e, guardianPageView{Mode: "error", Message: "Samtycket sparades inte. Det kan ha ändrats eller inbjudan kan ha gått ut. Öppna inbjudan och läs samtycket igen.", DoneURL: "/guardian/" + a.GuardianRequestID})
		}
		return guardianApprovedPage(e)
	}).BindFunc(s.throttle)
}
func (s *Service) guardianPageAttempt(e *core.RequestEvent) (*Attempt, error) {
	id := e.Request.PathValue("id")
	if !guardianRequestIDPattern.MatchString(id) {
		return nil, restartRequired()
	}
	cookie, err := e.Request.Cookie(guardianCookie(id))
	if err != nil {
		return nil, restartRequired()
	}
	a, err := s.owned(cookie.Value)
	if err != nil {
		return nil, err
	}
	if a.Purpose != "guardian" || a.GuardianRequestID != id || !s.alive(a) {
		return nil, restartRequired()
	}
	ip, err := s.Config.endUserIP(e.Request)
	if err != nil || ip != a.IP {
		return nil, problem(409, "connectionChanged", "Öppna inbjudan igen med den ursprungliga anslutningen.")
	}
	return a, nil
}
func (s *Service) guardianFormAttempt(e *core.RequestEvent) (*Attempt, error) {
	a, err := s.guardianPageAttempt(e)
	if err != nil {
		return nil, err
	}
	e.Request.Body = http.MaxBytesReader(e.Response, e.Request.Body, 8192)
	if err = e.Request.ParseForm(); err != nil {
		return nil, problem(400, "invalidRequest", "Ogiltig samtyckesförfrågan.")
	}
	if subtle.ConstantTimeCompare([]byte(e.Request.PostForm.Get("csrf")), []byte(a.Nonce)) != 1 {
		return nil, problem(403, "invalidRequest", "Ladda om inbjudan innan du fortsätter.")
	}
	return a, nil
}
func (s *Service) guardianHeaders(e *core.RequestEvent) {
	e.Response.Header().Set("Cache-Control", "no-store")
	e.Response.Header().Set("Referrer-Policy", "no-referrer")
	e.Response.Header().Set("X-Content-Type-Options", "nosniff")
	e.Response.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
}
func (s *Service) guardianView(e *core.RequestEvent, a *Attempt) error {
	if a.Status == "pending" {
		return guardianLaunchPage(e, a)
	}
	if a.Status != "accepted" {
		return renderGuardianPage(e, guardianPageView{Mode: "error", Message: "Identifieringen slutfördes inte. Inget samtycke registrerades.", DoneURL: "/guardian/" + a.GuardianRequestID, RestartURL: "/guardian/" + a.GuardianRequestID + "/restart", CSRF: a.Nonce})
	}
	request, err := s.App.FindRecordById("guardian_requests", a.GuardianRequestID)
	if err != nil {
		return err
	}
	user, err := s.App.FindRecordById("users", request.GetString("user"))
	if err != nil {
		return err
	}
	consent, err := s.activeAcceptance(s.App, user)
	if err != nil {
		return renderGuardianPage(e, guardianPageView{Mode: "error", Message: "Deltagaren behöver bekräfta det aktuella samtycket innan du kan fortsätta."})
	}
	doc, err := currentDocument(s.App)
	if err != nil {
		return err
	}
	if s.guardianApproved(s.App, request, doc.Id) {
		return guardianApprovedPage(e)
	}
	d := documentDTO(doc)
	return renderGuardianPage(e, guardianPageView{Mode: "consent", Document: &d, ParticipantSuffix: user.GetString("personalNumber")[8:], ConsentID: consent.Id, CSRF: a.Nonce, ApproveURL: "/guardian/" + request.Id + "/approve"})
}
