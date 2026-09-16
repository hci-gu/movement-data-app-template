package study

import (
	"app/internal/bankid"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
	"github.com/pocketbase/pocketbase/tools/security"
)

func problem(status int, reason, message string) error {
	return apis.NewApiError(status, message, map[string]any{"reason": reason})
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
		return e.JSON(200, map[string]any{"document": documentDTO(doc), "bankidAvailable": s.Config.Environment != "disabled" && s.Config.SigningEnabled})
	})
	g.POST("/enrollments", s.createFlow)
	g.POST("/bankid-orders", s.withFlow(func(e *core.RequestEvent, flow *core.Record) error {
		var input StartInput
		if err := e.BindBody(&input); err != nil {
			return problem(400, "invalidRequest", "Invalid signing request.")
		}
		ip, err := s.Config.endUserIP(e.Request)
		if err != nil {
			return problem(400, "invalidIP", "Unable to establish your connection.")
		}
		order, err := s.StartOrder(e.Request.Context(), flow, input, ip)
		if err != nil {
			return err
		}
		return s.orderResponse(e, order, true)
	}))
	g.GET("/bankid-orders/{id}", s.withFlow(func(e *core.RequestEvent, flow *core.Record) error {
		order, err := s.ownedOrder(e, flow)
		if err != nil {
			return err
		}
		return s.orderResponse(e, order, false)
	}))
	g.POST("/bankid-orders/{id}/return", s.withFlow(func(e *core.RequestEvent, flow *core.Record) error {
		order, err := s.ownedOrder(e, flow)
		if err != nil {
			return err
		}
		var body struct {
			Nonce string `json:"nonce"`
		}
		if err := e.BindBody(&body); err != nil {
			return problem(400, "invalidReturn", "Invalid BankID return.")
		}
		if order.GetString("mode") != "sameDevice" || subtle.ConstantTimeCompare([]byte(hash(body.Nonce)), []byte(order.GetString("nonceHash"))) != 1 {
			return problem(400, "invalidReturn", "This BankID return does not match your session.")
		}
		return s.orderResponse(e, order, false)
	}))
	g.POST("/bankid-orders/{id}/cancel", s.withFlow(func(e *core.RequestEvent, flow *core.Record) error {
		order, err := s.ownedOrder(e, flow)
		if err != nil {
			return err
		}
		// First collect the latest state. A completed signature is not a cancelled order.
		if order.GetString("status") == "pending" && int64(order.GetInt("nextCollectAt")) <= s.now().UnixMilli() {
			if err := s.advance(e.Request.Context(), order.Id); err != nil {
				return err
			}
			order, err = s.App.FindRecordById("bankid_orders", order.Id)
			if err != nil {
				return err
			}
		}
		if err := s.cancel(e.Request.Context(), order); err != nil {
			return err
		}
		return s.orderResponse(e, order, false)
	}))
	g.POST("/enrollments/{id}/complete", s.withFlow(s.completeFlow))
	g.POST("/enrollments/{id}/abandon", s.withFlow(func(e *core.RequestEvent, flow *core.Record) error {
		if flow.Id != e.Request.PathValue("id") {
			return problem(404, "sessionNotFound", "Session not found.")
		}
		if id := flow.GetString("latestOrder"); id != "" {
			order, err := s.App.FindRecordById("bankid_orders", id)
			if err != nil {
				return err
			}
			if err := s.cancel(e.Request.Context(), order); err != nil {
				return err
			}
		}
		err := s.App.RunInTransaction(func(tx core.App) error {
			flow.Set("expiresAt", s.now().Unix())
			if err := tx.Save(flow); err != nil {
				return err
			}
			if id := flow.GetString("invitation"); id != "" {
				invite, err := tx.FindRecordById("study_invitations", id)
				if err != nil {
					return err
				}
				if !invite.GetBool("consumed") && invite.GetString("claimedFlow") == flow.Id {
					invite.Set("claimedFlow", "")
					if err := tx.Save(invite); err != nil {
						return err
					}
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		return e.NoContent(204)
	}))
	g.GET("/me", func(e *core.RequestEvent) error { return e.JSON(200, s.userDTO(e.Auth)) }).BindFunc(s.RequireSession)
	g.POST("/logout", func(e *core.RequestEvent) error {
		session := e.Get("studySession").(*core.Record)
		session.Set("revoked", true)
		if err := s.App.Save(session); err != nil {
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
			user.Set("consentStatus", "withdrawn")
			if err := tx.Save(user); err != nil {
				return err
			}
			return consentEvent(tx, s.Config.StudyID, user.Id, user.GetString("consentSignature"), "withdrawn", s.now())
		})
		if err != nil {
			return err
		}
		return e.NoContent(204)
	}).BindFunc(s.RequireSession)
	g.GET("/consent/receipt", func(e *core.RequestEvent) error {
		r, err := s.App.FindRecordById("consent_signatures", e.Auth.GetString("consentSignature"))
		if err != nil || r.GetString("user") != e.Auth.Id {
			return problem(404, "receiptUnavailable", "No signed consent receipt is available.")
		}
		doc, err := s.App.FindRecordById("consent_versions", r.GetString("version"))
		if err != nil {
			return err
		}
		return e.JSON(200, map[string]any{"signatureId": r.Id, "receivedAt": r.GetInt("receivedAt"), "status": e.Auth.GetString("consentStatus"), "document": documentDTO(doc)})
	}).BindFunc(s.RequireSession)
}

func bearer(e *core.RequestEvent) string {
	h := e.Request.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return h
}

func (s *Service) createFlow(e *core.RequestEvent) error {
	if s.Config.Environment == "disabled" || !s.Config.SigningEnabled {
		return problem(503, "signingUnavailable", "BankID is currently unavailable.")
	}
	var body struct {
		Kind       string `json:"kind"`
		Invitation string `json:"invitationCode"`
		Secret     string `json:"clientSecret"`
	}
	if err := e.BindBody(&body); err != nil {
		return problem(400, "invalidRequest", "Invalid enrollment request.")
	}
	secret, err := base64.RawURLEncoding.DecodeString(body.Secret)
	if err != nil || len(secret) != 32 || (body.Kind != "enroll" && body.Kind != "login") {
		return problem(400, "invalidRequest", "Invalid enrollment request.")
	}
	tokenHash := s.Config.digest("flow", body.Secret)
	unlock := s.lock("create:" + s.Config.digest("invitation", body.Invitation))
	defer unlock()
	var flow *core.Record
	err = s.App.RunInTransaction(func(tx core.App) error {
		var err error
		flow, err = tx.FindFirstRecordByData("enrollment_sessions", "tokenHash", tokenHash)
		if err == nil {
			if flow.GetString("kind") != body.Kind || flow.GetString("environment") != s.Config.Environment || flow.GetInt("expiresAt") <= int(s.now().Unix()) {
				return problem(409, "sessionExpired", "Start a new enrollment session.")
			}
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		var invite *core.Record
		if body.Kind == "enroll" {
			invite, err = tx.FindFirstRecordByData("study_invitations", "tokenHash", s.Config.digest("invitation", strings.TrimSpace(body.Invitation)))
			if err != nil || invite.GetString("study") != s.Config.StudyID || invite.GetBool("consumed") || invite.GetInt("expiresAt") <= int(s.now().Unix()) {
				return problem(400, "invitationUnavailable", "This invitation is invalid, used or expired.")
			}
			if claim := invite.GetString("claimedFlow"); claim != "" {
				old, err := tx.FindRecordById("enrollment_sessions", claim)
				if err == nil && old.GetInt("expiresAt") > int(s.now().Unix()) {
					return problem(409, "invitationInUse", "This invitation is already open on another session. Resume that session or wait for it to expire.")
				}
				if err != nil && !errors.Is(err, sql.ErrNoRows) {
					return err
				}
			}
		}
		flow, err = newRecord(tx, "enrollment_sessions")
		if err != nil {
			return err
		}
		flow.Set("study", s.Config.StudyID)
		flow.Set("environment", s.Config.Environment)
		flow.Set("kind", body.Kind)
		flow.Set("tokenHash", tokenHash)
		flow.Set("expiresAt", s.now().Add(FlowLifetime).Unix())
		if invite != nil {
			flow.Set("invitation", invite.Id)
			flow.Set("participantId", invite.GetString("participantId"))
		}
		if err := tx.Save(flow); err != nil {
			return err
		}
		if invite != nil {
			invite.Set("claimedFlow", flow.Id)
			if err := tx.Save(invite); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return e.JSON(200, map[string]any{"id": flow.Id, "expiresAt": flow.GetInt("expiresAt"), "latestOrderId": flow.GetString("latestOrder")})
}

func (s *Service) withFlow(next func(*core.RequestEvent, *core.Record) error) func(*core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		id, secret, ok := strings.Cut(bearer(e), ".")
		if !ok || len(id) != 15 || len(secret) != 43 {
			return problem(401, "sessionExpired", "Your enrollment session has expired. Start again.")
		}
		unlock := s.lock(id)
		defer unlock()
		flow, err := s.App.FindRecordById("enrollment_sessions", id)
		if err != nil || flow.GetString("study") != s.Config.StudyID || flow.GetString("environment") != s.Config.Environment || flow.GetInt("expiresAt") <= int(s.now().Unix()) || subtle.ConstantTimeCompare([]byte(flow.GetString("tokenHash")), []byte(s.Config.digest("flow", secret))) != 1 {
			return problem(401, "sessionExpired", "Your enrollment session has expired. Start again.")
		}
		return next(e, flow)
	}
}

func (s *Service) ownedOrder(e *core.RequestEvent, flow *core.Record) (*core.Record, error) {
	order, err := s.App.FindRecordById("bankid_orders", e.Request.PathValue("id"))
	if err != nil || order.GetString("flow") != flow.Id {
		return nil, problem(404, "orderNotFound", "BankID request not found.")
	}
	return order, nil
}

func (s *Service) orderResponse(e *core.RequestEvent, order *core.Record, includeLaunch bool) error {
	if order.GetString("mode") == "sameDevice" {
		ip, err := s.Config.endUserIP(e.Request)
		if err != nil {
			return err
		}
		if subtle.ConstantTimeCompare([]byte(order.GetString("ipHash")), []byte(s.Config.digest("ip", ip))) != 1 {
			return problem(409, "connectionChanged", "Your network connection changed. Return to the original connection or start a new BankID session.")
		}
	}
	status := order.GetString("status")
	response := map[string]any{"id": order.Id, "status": status, "hintCode": order.GetString("hintCode"), "purpose": order.GetString("purpose"), "mode": order.GetString("mode"), "signatureId": order.GetString("signatureId"), "pickedUp": order.GetBool("pickedUp")}
	if status == "pending" {
		var secrets orderSecrets
		if err := s.Config.open("order:"+order.Id, order.GetString("requestCipher"), &secrets); err != nil {
			return err
		}
		if includeLaunch && order.GetString("mode") == "sameDevice" {
			response["launchUrl"] = "https://app.bankid.com/?autostarttoken=" + url.QueryEscape(secrets.Order.AutoStartToken)
			response["nonce"] = secrets.Nonce
		}
		if order.GetString("mode") == "qr" && !order.GetBool("pickedUp") {
			receivedAt := time.UnixMilli(int64(order.GetInt("receivedAt")))
			remaining := 30 - int(s.now().Sub(receivedAt).Seconds())
			if remaining < 0 {
				remaining = 0
			}
			response["qrSecondsRemaining"] = remaining
			if remaining > 0 {
				response["qrData"] = bankid.QRPayload(secrets.Order, receivedAt, s.now())
			}
		}
	}
	return e.JSON(200, response)
}

type grant struct {
	Token           string         `json:"token"`
	ExpiresAt       int64          `json:"expiresAt"`
	Record          map[string]any `json:"record"`
	ConsentRequired bool           `json:"consentRequired"`
}

func (s *Service) userDTO(user *core.Record) map[string]any {
	return map[string]any{"id": user.Id, "collectionId": user.Collection().Id, "collectionName": "users", "username": user.GetString("username"), "consentStatus": user.GetString("consentStatus"), "consentVersion": user.GetString("consentVersion"), "consentSignature": user.GetString("consentSignature"), "consentRequired": !s.activeConsent(s.App, user)}
}

func (s *Service) completeFlow(e *core.RequestEvent, flow *core.Record) error {
	if flow.Id != e.Request.PathValue("id") {
		return problem(404, "sessionNotFound", "Session not found.")
	}
	order, err := s.App.FindRecordById("bankid_orders", flow.GetString("latestOrder"))
	if err != nil || order.GetString("status") != "accepted" {
		return problem(409, "notCompleted", "Complete your BankID request first.")
	}
	if order.GetString("mode") == "sameDevice" {
		ip, err := s.Config.endUserIP(e.Request)
		if err != nil {
			return err
		}
		if s.Config.digest("ip", ip) != order.GetString("ipHash") {
			return problem(409, "connectionChanged", "Return to the original network to complete this session.")
		}
	}
	user, err := s.App.FindRecordById("users", flow.GetString("user"))
	if err != nil {
		return err
	}
	if !s.activeConsent(s.App, user) {
		return e.JSON(200, grant{ConsentRequired: true})
	}
	var response grant
	if encrypted := flow.GetString("grantCipher"); encrypted != "" {
		if err := s.Config.open("grant:"+flow.Id, encrypted, &response); err != nil {
			return err
		}
		if response.ExpiresAt <= s.now().Unix() {
			return problem(401, "sessionExpired", "Sign in again with BankID.")
		}
		return e.JSON(200, response)
	}
	expires := time.Unix(int64(flow.GetInt("authenticatedAt")), 0).Add(SessionLifetime)
	if !s.now().Before(expires) {
		return problem(401, "sessionExpired", "Sign in again with BankID.")
	}
	sessionID := randomSecret()
	token, err := security.NewJWT(jwt.MapClaims{
		core.TokenClaimType:         core.TokenTypeAuth,
		core.TokenClaimId:           user.Id,
		core.TokenClaimCollectionId: user.Collection().Id,
		core.TokenClaimRefreshable:  false,
		"studySession":              sessionID,
	}, user.TokenKey()+user.Collection().AuthToken.Secret, expires.Sub(s.now()))
	if err != nil {
		return err
	}
	response = grant{Token: token, ExpiresAt: expires.Unix(), Record: s.userDTO(user)}
	sealed, err := s.Config.seal("grant:"+flow.Id, response)
	if err != nil {
		return err
	}
	err = s.App.RunInTransaction(func(tx core.App) error {
		flow.Set("grantCipher", sealed)
		if err := tx.Save(flow); err != nil {
			return err
		}
		r, err := newRecord(tx, "app_sessions")
		if err != nil {
			return err
		}
		r.Set("study", s.Config.StudyID)
		r.Set("environment", s.Config.Environment)
		r.Set("user", user.Id)
		r.Set("tokenHash", hash(token))
		r.Set("expiresAt", expires.Unix())
		return tx.Save(r)
	})
	if err != nil {
		return err
	}
	return e.JSON(200, response)
}

func (s *Service) RequireSession(e *core.RequestEvent) error {
	token := bearer(e)
	user, err := s.App.FindAuthRecordByToken(token, core.TokenTypeAuth)
	if err != nil || user.Collection().Name != "users" {
		return problem(401, "sessionExpired", "Sign in again with BankID.")
	}
	session, err := s.App.FindFirstRecordByData("app_sessions", "tokenHash", hash(token))
	if err != nil || session.GetString("user") != user.Id || session.GetString("study") != s.Config.StudyID || session.GetString("environment") != s.Config.Environment || session.GetBool("revoked") || session.GetInt("expiresAt") <= int(s.now().Unix()) {
		return problem(401, "sessionExpired", "Sign in again with BankID.")
	}
	e.Auth = user
	e.Set("studySession", session)
	return e.Next()
}

func (s *Service) RequireConsent(e *core.RequestEvent) error {
	if e.Auth == nil || !s.activeConsent(s.App, e.Auth) {
		return problem(403, "consentRequired", "Sign the current study consent before uploading.")
	}
	return e.Next()
}

// Even superuser API writes must use the study operations, which preserve
// immutable documents/evidence. Ordinary collection rules alone exclude no superusers.
func ProtectRecords(app core.App) {
	immutable := []string{"consent_versions", "consent_signatures", "participant_identities", "consent_events", "study_settings"}
	app.OnRecordCreateRequest(immutable...).BindFunc(func(e *core.RecordRequestEvent) error {
		return problem(403, "managedRecord", "Use the enrollment administration or signing flow to create this record.")
	})
	app.OnRecordUpdateRequest(immutable...).BindFunc(func(e *core.RecordRequestEvent) error {
		return problem(403, "immutableRecord", "This study record is immutable.")
	})
	app.OnRecordDeleteRequest(immutable...).BindFunc(func(e *core.RecordRequestEvent) error {
		return problem(403, "immutableRecord", "Use the approved retention procedure for study evidence.")
	})
	app.OnRecordAuthRequest("users").BindFunc(func(e *core.RecordAuthRequestEvent) error {
		return problem(403, "bankidRequired", "Use BankID to sign in.")
	})
}
