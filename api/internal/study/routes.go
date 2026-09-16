package study

import (
	"app/internal/bankid"
	"crypto/subtle"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	validation "github.com/pocketbase/ozzo-validation/v4"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
	"github.com/pocketbase/pocketbase/tools/security"
)

func problem(status int, reason, message string) error {
	return apis.NewApiError(status, message, map[string]any{"reason": validation.NewError(reason, message)})
}
func restartRequired() error {
	return problem(410, "attemptExpired", "This BankID request has expired or the server restarted. Start a new request.")
}
func bearer(e *core.RequestEvent) string {
	return strings.TrimPrefix(e.Request.Header.Get("Authorization"), "Bearer ")
}
func splitCredential(value string) (string, string, bool) {
	id, secret, ok := strings.Cut(value, ".")
	return id, secret, ok && len(id) == 32 && secretPattern.MatchString(secret)
}

type rateEntry struct {
	count int
	reset time.Time
}

func (s *Service) throttle(e *core.RequestEvent) error {
	e.Response.Header().Set("Cache-Control", "no-store")
	e.Response.Header().Set("Referrer-Policy", "no-referrer")
	ip, err := s.Config.endUserIP(e.Request)
	if err != nil {
		return problem(400, "invalidIP", "Unable to establish your connection.")
	}
	s.rateMu.Lock()

	key := hash(ip + ":" + e.Request.Method)
	now := s.now()
	if len(s.rates) > 2048 {
		for k, v := range s.rates {
			if !now.Before(v.reset) {
				delete(s.rates, k)
			}
		}
	}
	v, ok := s.rates[key]
	if !ok && len(s.rates) >= 4096 {
		s.rateMu.Unlock()
		return problem(429, "rateLimited", "Please wait and try again.")
	}
	if !now.Before(v.reset) {
		v = rateEntry{reset: now.Add(time.Minute)}
	}
	v.count++
	s.rates[key] = v
	s.rateMu.Unlock()
	limit := 120
	if e.Request.Method == http.MethodPost {
		limit = 20
	}
	if v.count > limit {
		e.Response.Header().Set("Retry-After", "60")
		return problem(429, "rateLimited", "Please wait and try again.")
	}
	return e.Next()
}

func (s *Service) RegisterRoutes(r *router.Router[*core.RequestEvent]) {
	s.registerLinks(r)
	s.registerQuestionnaireAccess()
	g := r.Group("/api/study")
	g.BindFunc(s.throttle)
	g.BindFunc(func(e *core.RequestEvent) error {
		e.Request.Body = http.MaxBytesReader(e.Response, e.Request.Body, 16384)
		return e.Next()
	})
	g.GET("/consent/current", func(e *core.RequestEvent) error {
		doc, err := currentDocument(s.App, s.Config.StudyID)
		if err != nil {
			return problem(503, "consentUnavailable", "Study consent is not available yet.")
		}
		return e.JSON(200, map[string]any{"document": documentDTO(doc), "bankidAvailable": s.provider != nil && s.Config.SigningEnabled})
	})
	g.POST("/bankid/attempts", func(e *core.RequestEvent) error {
		var in StartInput
		if err := e.BindBody(&in); err != nil {
			return problem(400, "invalidRequest", "Invalid request.")
		}
		ip, err := s.Config.endUserIP(e.Request)
		if err != nil {
			return err
		}
		a, err := s.Start(e.Request.Context(), in, ip)
		if err != nil {
			return err
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		return s.attemptResponse(e, a)
	})
	g.GET("/bankid/attempts/{id}", s.withAttempt(s.attemptResponse))
	g.POST("/bankid/attempts/{id}/cancel", s.withAttempt(func(e *core.RequestEvent, a *Attempt) error {
		if err := s.cancel(e.Request.Context(), a); err != nil {
			return err
		}
		return s.attemptResponse(e, a)
	}))
	g.POST("/bankid/attempts/{id}/return", s.withAttempt(func(e *core.RequestEvent, a *Attempt) error {
		var in struct {
			Nonce string `json:"nonce"`
		}
		if err := e.BindBody(&in); err != nil {
			return err
		}
		if a.Mode != "sameDevice" || subtle.ConstantTimeCompare([]byte(in.Nonce), []byte(a.Nonce)) != 1 {
			return problem(400, "invalidReturn", "This BankID return does not match your request.")
		}
		return s.attemptResponse(e, a)
	}))
	g.GET("/me", func(e *core.RequestEvent) error { return e.JSON(200, s.userDTO(e.Auth)) }).BindFunc(s.RequireSession)
	g.POST("/logout", func(e *core.RequestEvent) error {
		e.Auth.RefreshTokenKey()
		if err := s.App.Save(e.Auth); err != nil {
			return err
		}
		return e.NoContent(204)
	}).BindFunc(s.RequireSession)
	g.POST("/consent/withdraw", func(e *core.RequestEvent) error {
		err := s.App.RunInTransaction(func(tx core.App) error {
			user, err := tx.FindRecordById("users", e.Auth.Id)
			if err != nil {
				return err
			}
			if user.GetString("consentStatus") == "withdrawn" {
				return nil
			}
			sig, err := tx.FindRecordById("signatures", user.GetString("consentSignature"))
			if err != nil {
				return err
			}
			sig.Set("withdrawnAt", s.now().Unix())
			if err := tx.Save(sig); err != nil {
				return err
			}
			user.Set("withdrawnAt", s.now().Unix())
			user.Set("consentStatus", "withdrawn")
			return tx.Save(user)
		})
		if err != nil {
			return err
		}
		return e.NoContent(204)
	}).BindFunc(s.RequireSession)
	g.GET("/consent/receipt", func(e *core.RequestEvent) error {
		sig, err := s.App.FindRecordById("signatures", e.Auth.GetString("consentSignature"))
		if err != nil || sig.GetString("user") != e.Auth.Id {
			return problem(404, "receiptUnavailable", "No signed consent receipt is available.")
		}
		doc, err := s.App.FindRecordById("consent_texts", sig.GetString("version"))
		if err != nil {
			return err
		}
		return e.JSON(200, map[string]any{"signatureId": sig.Id, "receivedAt": sig.GetInt("receivedAt"), "status": e.Auth.GetString("consentStatus"), "document": documentDTO(doc)})
	}).BindFunc(s.RequireSession)
}
func (s *Service) withAttempt(next func(*core.RequestEvent, *Attempt) error) func(*core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		a, err := s.owned(bearer(e))
		if err != nil {
			return err
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.ID != e.Request.PathValue("id") || !s.alive(a) {
			return restartRequired()
		}
		if a.Mode == "sameDevice" {
			ip, err := s.Config.endUserIP(e.Request)
			if err != nil {
				return err
			}
			if ip != a.IP {
				return problem(409, "connectionChanged", "Return to the original connection or start again.")
			}
		}
		return next(e, a)
	}
}
func (s *Service) attemptResponse(e *core.RequestEvent, a *Attempt) error {
	response := map[string]any{"id": a.ID, "status": a.Status, "hintCode": a.Hint, "purpose": a.Purpose, "mode": a.Mode, "signatureId": a.SignatureID, "pickedUp": a.PickedUp, "expiresAt": a.ExpiresAt.Add(ResultLifetime).Unix(), "nonce": a.Nonce}
	if a.Document != nil {
		response["document"] = a.Document
	}
	if a.Status == "pending" && a.Terminal == "" {
		if a.Mode == "sameDevice" {
			response["launchUrl"] = "https://app.bankid.com/?autostarttoken=" + url.QueryEscape(a.Order.AutoStartToken)
		}
		if a.Mode == "qr" && !a.PickedUp {
			remaining := 30 - int(s.now().Sub(a.ReceivedAt).Seconds())
			if remaining < 0 {
				remaining = 0
			}
			response["qrSecondsRemaining"] = remaining
			if remaining > 0 {
				response["qrData"] = bankid.QRPayload(a.Order, a.ReceivedAt, s.now())
			}
		}
	}
	if a.Status == "accepted" {
		user, err := s.App.FindRecordById("users", a.UserID)
		if err != nil {
			return err
		}
		if !user.GetBool("active") || user.TokenKey() != a.TokenKey {
			return restartRequired()
		}
		if !s.activeConsent(s.App, user) {
			response["consentRequired"] = true
		} else {
			if a.Grant == nil {
				expires := a.FinishedAt.Add(SessionLifetime)
				token, err := security.NewJWT(jwt.MapClaims{core.TokenClaimType: core.TokenTypeAuth, core.TokenClaimId: user.Id, core.TokenClaimCollectionId: user.Collection().Id, core.TokenClaimRefreshable: false, "study": s.Config.StudyID, "environment": s.Config.Environment, "exp": expires.Unix()}, user.TokenKey()+user.Collection().AuthToken.Secret, expires.Sub(s.now()))
				if err != nil {
					return err
				}
				a.Grant = &grant{Token: token, ExpiresAt: expires.Unix(), Record: s.userDTO(user)}
			}
			if a.Grant.ExpiresAt <= s.now().Unix() {
				return restartRequired()
			}
			response["grant"] = a.Grant
		}
	}
	return e.JSON(200, response)
}

type grant struct {
	Token     string         `json:"token"`
	ExpiresAt int64          `json:"expiresAt"`
	Record    map[string]any `json:"record"`
}

func (s *Service) userDTO(user *core.Record) map[string]any {
	return map[string]any{"id": user.Id, "collectionId": user.Collection().Id, "collectionName": "users", "username": user.GetString("username"), "consentStatus": user.GetString("consentStatus"), "consentVersion": user.GetString("consentVersion"), "consentSignature": user.GetString("consentSignature"), "consentRequired": !s.activeConsent(s.App, user)}
}
func (s *Service) validateSession(e *core.RequestEvent) error {
	token := bearer(e)
	user, err := s.App.FindAuthRecordByToken(token, core.TokenTypeAuth)
	if err != nil || user.Collection().Name != "users" || !user.GetBool("active") || user.GetString("study") != s.Config.StudyID || user.GetString("environment") != s.Config.Environment {
		return problem(401, "sessionExpired", "Sign in again with BankID.")
	}
	// The token was verified above; inspect the application scope claims.
	claims, err := security.ParseUnverifiedJWT(token)
	if err != nil || claims["study"] != s.Config.StudyID || claims["environment"] != s.Config.Environment || claims[core.TokenClaimRefreshable] != false {
		return problem(401, "sessionExpired", "Sign in again with BankID.")
	}
	e.Auth = user
	return nil
}
func (s *Service) RequireSession(e *core.RequestEvent) error {
	if err := s.validateSession(e); err != nil {
		return err
	}
	return e.Next()
}
func (s *Service) RequireConsent(e *core.RequestEvent) error {
	if e.Auth == nil || !s.activeConsent(s.App, e.Auth) {
		return problem(403, "consentRequired", "Sign the current study consent before continuing.")
	}
	return e.Next()
}
func ProtectRecords(app core.App) {
	managed := []string{"consent_texts", "signatures", "users"}
	app.OnRecordCreateRequest(managed...).BindFunc(func(e *core.RecordRequestEvent) error {
		return problem(403, "managedRecord", "Use the study administration or BankID flow.")
	})
	app.OnRecordUpdateRequest(managed...).BindFunc(func(e *core.RecordRequestEvent) error {
		return problem(403, "managedRecord", "Use the study administration or BankID flow.")
	})
	app.OnRecordDeleteRequest(managed...).BindFunc(func(e *core.RecordRequestEvent) error {
		return problem(403, "managedRecord", "Study evidence and participants cannot be deleted here.")
	})
	app.OnRecordAuthRequest("users").BindFunc(func(e *core.RecordAuthRequestEvent) error {
		return problem(403, "bankidRequired", "Use BankID to sign in.")
	})
}

// Questionnaire definitions are shared; answers remain owned by their participant.
// Rules handle record selection, while these hooks enforce BankID scope and current consent.
func (s *Service) registerQuestionnaireAccess() {
	names := []string{"questionnaires", "questions", "questionOptions", "answers"}
	guard := func(e *core.RequestEvent) error {
		if err := s.validateSession(e); err != nil {
			return err
		}
		if !s.activeConsent(s.App, e.Auth) {
			return problem(403, "consentRequired", "Sign the current study consent before continuing.")
		}
		return nil
	}
	s.App.OnRecordsListRequest(names...).BindFunc(func(e *core.RecordsListRequestEvent) error {
		if !e.HasSuperuserAuth() {
			if err := guard(e.RequestEvent); err != nil {
				return err
			}
		}
		return e.Next()
	})
	s.App.OnRecordViewRequest(names...).BindFunc(func(e *core.RecordRequestEvent) error {
		if !e.HasSuperuserAuth() {
			if err := guard(e.RequestEvent); err != nil {
				return err
			}
		}
		return e.Next()
	})
	write := func(e *core.RecordRequestEvent) error {
		if !e.HasSuperuserAuth() {
			if err := guard(e.RequestEvent); err != nil {
				return err
			}
			if e.Record.GetString("user") != e.Auth.Id {
				return problem(403, "wrongParticipant", "Answers must belong to the authenticated participant.")
			}
		}
		return e.Next()
	}
	s.App.OnRecordCreateRequest("answers").BindFunc(write)
	s.App.OnRecordUpdateRequest("answers").BindFunc(write)
	s.App.OnRecordDeleteRequest("answers").BindFunc(write)
}
