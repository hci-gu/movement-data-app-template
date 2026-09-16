package study

import (
	"app/internal/bankid"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sync"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

var participantPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{3,63}$`)
var personalNumberPattern = regexp.MustCompile(`^[0-9]{12}$`)

type Service struct {
	App               core.App
	Config            Config
	providers         map[string]bankid.Provider
	activeCertificate string
	locks             [64]sync.Mutex
	rateMu            sync.Mutex
	rates             map[string]rateEntry
	now               func() time.Time
}

func Open(app core.App, cfg Config) (*Service, error) {
	s := &Service{App: app, Config: cfg, providers: map[string]bankid.Provider{}, rates: map[string]rateEntry{}, now: time.Now}
	if cfg.Environment == "disabled" {
		return s, nil
	}
	client, err := bankid.NewClient(cfg.Environment, cfg.CertFile, cfg.KeyFile, cfg.CAFile)
	if err != nil {
		return nil, err
	}
	s.providers[client.Fingerprint] = client
	s.activeCertificate = client.Fingerprint
	app.Logger().Info("BankID configured", "environment", cfg.Environment, "certificateExpires", client.CertificateExpires)
	if cfg.PreviousCertFile != "" || cfg.PreviousKeyFile != "" {
		previous, err := bankid.NewClient(cfg.Environment, cfg.PreviousCertFile, cfg.PreviousKeyFile, cfg.CAFile)
		if err != nil {
			return nil, err
		}
		s.providers[previous.Fingerprint] = previous
	}
	return s, nil
}

func (s *Service) lock(flow string) func() {
	index := []byte(hash(flow))[0] % 64
	s.locks[index].Lock()
	return s.locks[index].Unlock
}

type Document struct {
	ID           string `json:"id"`
	Version      string `json:"version"`
	Title        string `json:"title"`
	Text         string `json:"text"`
	DocumentHash string `json:"documentHash"`
}

func documentDTO(r *core.Record) Document {
	return Document{r.Id, r.GetString("version"), r.GetString("title"), r.GetString("text"), r.GetString("documentHash")}
}

func currentDocument(app core.App, study string) (*core.Record, error) {
	settings, err := app.FindFirstRecordByData("study_settings", "study", study)
	if err != nil {
		return nil, err
	}
	doc, err := app.FindRecordById("consent_versions", settings.GetString("currentVersion"))
	if err != nil {
		return nil, err
	}
	if doc.GetString("study") != study || hash(doc.GetString("text")) != doc.GetString("documentHash") {
		return nil, errors.New("consent document integrity check failed")
	}
	return doc, nil
}

func newRecord(app core.App, collection string) (*core.Record, error) {
	c, err := app.FindCollectionByNameOrId(collection)
	if err != nil {
		return nil, err
	}
	return core.NewRecord(c), nil
}

type orderSecrets struct {
	Request  bankid.Request `json:"request"`
	Order    bankid.Order   `json:"order"`
	Nonce    string         `json:"nonce"`
	Document *Document      `json:"document,omitempty"`
}
type identity struct {
	PersonalNumber string `json:"personalNumber"`
	Name           string `json:"name"`
}

type StartInput struct {
	Version      string `json:"consentVersionId"`
	DocumentHash string `json:"documentHash"`
	Mode         string `json:"mode"`
	RequestKey   string `json:"requestKey"`
}

// StartOrder persists intent before contacting BankID. An ambiguous network
// result stays unresolved and is never retried with the same idempotency key.
// Caller holds the flow lock.
func (s *Service) StartOrder(ctx context.Context, flow *core.Record, input StartInput, ip string) (*core.Record, error) {
	if input.Mode != "sameDevice" && input.Mode != "qr" {
		return nil, problem(400, "invalidMode", "Choose a BankID signing method.")
	}
	if len(input.RequestKey) < 16 || len(input.RequestKey) > 100 {
		return nil, problem(400, "invalidRequestKey", "Invalid request key.")
	}
	existing, err := s.App.FindFirstRecordByFilter("bankid_orders", "flow={:flow} && requestKey={:key}", dbx.Params{"flow": flow.Id, "key": input.RequestKey})
	if err == nil {
		if existing.GetString("mode") != input.Mode || existing.GetString("version") != input.Version {
			return nil, problem(409, "requestChanged", "Use a new request key for a changed signing request.")
		}
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if s.Config.Environment == "disabled" || !s.Config.SigningEnabled {
		return nil, problem(503, "signingUnavailable", "BankID is currently unavailable.")
	}
	if flow.GetString("grantCipher") != "" {
		return nil, problem(409, "alreadyCompleted", "This session has already completed.")
	}
	if oldID := flow.GetString("latestOrder"); oldID != "" {
		old, err := s.App.FindRecordById("bankid_orders", oldID)
		if err != nil {
			return nil, err
		}
		switch old.GetString("status") {
		case "creating", "pending", "collecting", "collected":
			return nil, problem(409, "orderInProgress", "Finish or cancel your current BankID request first.")
		}
	}
	purpose := "sign"
	if flow.GetString("kind") == "login" && flow.GetString("user") == "" {
		purpose = "auth"
	}
	var doc *Document
	if purpose == "sign" {
		r, err := currentDocument(s.App, s.Config.StudyID)
		if err != nil {
			return nil, problem(503, "consentUnavailable", "Study consent is not available yet.")
		}
		if r.Id != input.Version || r.GetString("documentHash") != input.DocumentHash {
			return nil, problem(409, "consentChanged", "The consent has changed. Read the current version before signing.")
		}
		d := documentDTO(r)
		doc = &d
	} else if input.Version != "" {
		return nil, problem(400, "invalidPurpose", "Sign in before reviewing updated consent.")
	}
	order, err := newRecord(s.App, "bankid_orders")
	if err != nil {
		return nil, err
	}
	order.Set("study", s.Config.StudyID)
	order.Set("environment", s.Config.Environment)
	order.Set("flow", flow.Id)
	order.Set("purpose", purpose)
	order.Set("mode", input.Mode)
	order.Set("requestKey", input.RequestKey)
	order.Set("version", input.Version)
	order.Set("status", "creating")
	order.Set("certificate", s.activeCertificate)
	order.Set("startedAt", s.now().Unix())
	order.Set("ipHash", s.Config.digest("ip", ip))
	nonce := randomSecret()
	order.Set("nonceHash", hash(nonce))
	req := bankid.Request{EndUserIP: ip, ReturnRisk: true, App: &bankid.AppInfo{AppIdentifier: s.Config.AppID}}
	if input.Mode == "sameDevice" {
		req.ReturnURL = s.Config.ReturnURL + "#nonce=" + url.QueryEscape(nonce)
	}
	var expected identity
	if flow.GetString("identityCipher") != "" {
		if err := s.Config.open("flow:"+flow.Id, flow.GetString("identityCipher"), &expected); err != nil {
			return nil, err
		}
	} else if id := flow.GetString("invitation"); id != "" {
		invite, err := s.App.FindRecordById("study_invitations", id)
		if err != nil {
			return nil, err
		}
		if invite.GetString("expectedCipher") != "" {
			if err := s.Config.open("invitation:"+invite.Id, invite.GetString("expectedCipher"), &expected); err != nil {
				return nil, err
			}
		}
	}
	if expected.PersonalNumber != "" {
		req.Requirement = &bankid.Requirement{PersonalNumber: expected.PersonalNumber}
	}
	if doc != nil {
		req.UserVisibleData = base64.StdEncoding.EncodeToString([]byte(doc.Text))
	} else {
		req.UserVisibleData = base64.StdEncoding.EncodeToString([]byte("Sign in to " + s.Config.StudyID + " to access your research participation."))
	}
	// Save first to obtain a stable local order ID for the immutable manifest.
	if err := s.App.Save(order); err != nil {
		return nil, err
	}
	manifest, _ := json.Marshal(map[string]any{"schemaVersion": 1, "purpose": purpose, "studyId": s.Config.StudyID, "consent": doc, "participantId": flow.GetString("participantId"), "enrollmentSessionId": flow.Id, "signingSessionId": order.Id, "nonce": nonce})
	// The document text is visible; only identifiers and the document hash need hiding.
	if doc != nil {
		manifest, _ = json.Marshal(map[string]any{"schemaVersion": 1, "purpose": purpose, "studyId": s.Config.StudyID, "consentVersionId": doc.ID, "documentHash": doc.DocumentHash, "participantId": flow.GetString("participantId"), "enrollmentSessionId": flow.Id, "signingSessionId": order.Id, "nonce": nonce})
	}
	req.UserNonVisibleData = base64.StdEncoding.EncodeToString(manifest)
	secrets := orderSecrets{Request: req, Nonce: nonce, Document: doc}
	sealed, err := s.Config.seal("order:"+order.Id, secrets)
	if err != nil {
		return nil, err
	}
	order.Set("requestCipher", sealed)
	flow.Set("latestOrder", order.Id)
	if err := s.App.RunInTransaction(func(tx core.App) error {
		if err := tx.Save(order); err != nil {
			return err
		}
		return tx.Save(flow)
	}); err != nil {
		return nil, err
	}
	remote, err := s.providers[s.activeCertificate].Start(ctx, purpose, req)
	if err != nil {
		order.Set("status", "unresolved")
		order.Set("hintCode", "unknownResult")
		var apiErr *bankid.APIError
		if errors.As(err, &apiErr) {
			order.Set("status", "failed")
			order.Set("hintCode", apiErr.Code)
		}
		if saveErr := s.App.Save(order); saveErr != nil {
			return nil, saveErr
		}
		return order, nil
	}
	secrets.Order = remote
	sealed, err = s.Config.seal("order:"+order.Id, secrets)
	if err != nil {
		return nil, err
	}
	order.Set("requestCipher", sealed)
	order.Set("orderHash", hash(remote.OrderRef))
	order.Set("status", "pending")
	order.Set("hintCode", "outstandingTransaction")
	order.Set("receivedAt", s.now().UnixMilli())
	order.Set("nextCollectAt", s.now().Add(2*time.Second).UnixMilli())
	if err := s.App.Save(order); err != nil {
		return nil, err
	}
	return order, nil
}

func (s *Service) Recover() error {
	// A previous process may have received a terminal result without persisting it.
	// Never recollect such an order and mistake a missing result for failure.
	for _, status := range []string{"creating", "collecting", "cancelling"} {
		records, err := s.App.FindRecordsByFilter("bankid_orders", "study={:study} && environment={:env} && status={:status}", "", 0, 0, dbx.Params{"study": s.Config.StudyID, "env": s.Config.Environment, "status": status})
		if err != nil {
			return errors.New("BankID recovery lookup failed")
		}
		for _, r := range records {
			r.Set("status", "unresolved")
			r.Set("hintCode", "unknownResult")
			if err := s.App.Save(r); err != nil {
				return errors.New("BankID recovery write failed")
			}
		}
	}
	return nil
}

func (s *Service) Run(ctx context.Context) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *Service) tick(ctx context.Context) {
	records, err := s.App.FindRecordsByFilter("bankid_orders", "study={:study} && environment={:env} && (status='pending' || status='collected') && nextCollectAt<={:now}", "nextCollectAt", 100, 0, dbx.Params{"study": s.Config.StudyID, "env": s.Config.Environment, "now": s.now().UnixMilli()})
	if err != nil {
		s.App.Logger().Error("BankID pending order lookup failed")
		return
	}
	// Bounded workers: collection continues independently of app lifecycle.
	var wg sync.WaitGroup
	slots := make(chan struct{}, 8)
	for _, r := range records {
		select {
		case slots <- struct{}{}:
		case <-ctx.Done():
			wg.Wait()
			return
		}
		wg.Add(1)
		go func(id, flow string) {
			defer wg.Done()
			defer func() { <-slots }()
			unlock := s.lock(flow)
			defer unlock()
			if err := s.advance(ctx, id); err != nil {
				s.App.Logger().Error("BankID order persistence failed", "order", id)
			}
		}(r.Id, r.GetString("flow"))
	}
	wg.Wait()
}

func (s *Service) advance(ctx context.Context, id string) error {
	order, err := s.App.FindRecordById("bankid_orders", id)
	if err != nil {
		return err
	}
	if order.GetString("status") == "collected" {
		return s.finalize(order)
	}
	if order.GetString("status") != "pending" {
		return nil
	}
	provider := s.providers[order.GetString("certificate")]
	if provider == nil {
		order.Set("hintCode", "certificateUnavailable")
		order.Set("nextCollectAt", s.now().Add(10*time.Second).UnixMilli())
		return s.App.Save(order)
	}
	var secrets orderSecrets
	if err := s.Config.open("order:"+order.Id, order.GetString("requestCipher"), &secrets); err != nil {
		return err
	}
	if s.now().Unix()-int64(order.GetInt("startedAt")) > int64(FlowLifetime.Seconds()) {
		return s.cancel(ctx, order)
	}
	order.Set("status", "collecting")
	if err := s.App.Save(order); err != nil {
		return err
	}
	result, err := provider.Collect(ctx, secrets.Order.OrderRef)
	if err != nil {
		order.Set("status", "unresolved")
		order.Set("hintCode", "unknownResult")
		var apiErr *bankid.APIError
		if errors.As(err, &apiErr) && (apiErr.Status == 429 || apiErr.Status >= 500) {
			order.Set("status", "pending")
			order.Set("hintCode", "temporarilyUnavailable")
			order.Set("nextCollectAt", s.now().Add(5*time.Second).UnixMilli())
		}
		return s.App.Save(order)
	}
	if result.OrderRef != secrets.Order.OrderRef {
		order.Set("status", "unresolved")
		order.Set("hintCode", "unknownResult")
		return s.App.Save(order)
	}
	switch result.Status {
	case "pending":
		order.Set("status", "pending")
		order.Set("hintCode", result.HintCode)
		order.Set("nextCollectAt", s.now().Add(2*time.Second).UnixMilli())
		if result.HintCode == "userSign" || result.HintCode == "userMrtd" || result.HintCode == "processing" {
			order.Set("pickedUp", true)
		}
		return s.App.Save(order)
	case "failed":
		order.Set("status", "failed")
		order.Set("hintCode", result.HintCode)
		return s.App.Save(order)
	case "complete":
		if len(result.Raw) == 0 {
			result.Raw, _ = json.Marshal(result)
		}
		sealed, err := s.Config.seal("result:"+order.Id, result.Raw)
		if err != nil {
			return err
		}
		order.Set("resultCipher", sealed)
		order.Set("status", "collected")
		if err := s.App.Save(order); err != nil {
			return err
		}
		return s.finalize(order)
	default:
		order.Set("status", "unresolved")
		order.Set("hintCode", "unknownResult")
		return s.App.Save(order)
	}
}

func (s *Service) cancel(ctx context.Context, order *core.Record) error {
	if order.GetString("status") != "pending" {
		return nil
	}
	provider := s.providers[order.GetString("certificate")]
	if provider == nil {
		return problem(503, "certificateUnavailable", "This request cannot be cancelled right now.")
	}
	var secrets orderSecrets
	if err := s.Config.open("order:"+order.Id, order.GetString("requestCipher"), &secrets); err != nil {
		return err
	}
	order.Set("status", "cancelling")
	if err := s.App.Save(order); err != nil {
		return err
	}
	if err := provider.Cancel(ctx, secrets.Order.OrderRef); err != nil {
		order.Set("status", "unresolved")
		order.Set("hintCode", "unknownResult")
	} else {
		order.Set("status", "cancelled")
		order.Set("hintCode", "userCancel")
	}
	return s.App.Save(order)
}

func (s *Service) finalize(order *core.Record) error {
	var raw json.RawMessage
	if err := s.Config.open("result:"+order.Id, order.GetString("resultCipher"), &raw); err != nil {
		return err
	}
	var result bankid.Result
	if err := json.Unmarshal(raw, &result); err != nil {
		return err
	}
	var secrets orderSecrets
	if err := s.Config.open("order:"+order.Id, order.GetString("requestCipher"), &secrets); err != nil {
		return err
	}
	return s.App.RunInTransaction(func(tx core.App) error {
		flow, err := tx.FindRecordById("enrollment_sessions", order.GetString("flow"))
		if err != nil {
			return err
		}
		outcome := "accepted"
		hint := ""
		completion := result.CompletionData
		if result.OrderRef != secrets.Order.OrderRef || completion == nil || !personalNumberPattern.MatchString(completion.User.PersonalNumber) || !validBase64(completion.Signature) || !validBase64(completion.OCSPResponse) {
			outcome = "rejected"
			hint = "invalidEvidence"
		}
		if outcome == "accepted" && completion.Risk != "low" {
			outcome = "rejected"
			hint = "riskRejected"
		}
		if flow.GetInt("expiresAt") <= int(s.now().Unix()) {
			outcome = "rejected"
			hint = "sessionExpired"
		}
		if outcome == "accepted" && secrets.Request.Requirement != nil && secrets.Request.Requirement.PersonalNumber != "" && secrets.Request.Requirement.PersonalNumber != completion.User.PersonalNumber {
			outcome = "rejected"
			hint = "wrongSigner"
		}
		var user *core.Record
		var person identity
		if outcome == "accepted" {
			person = identity{completion.User.PersonalNumber, completion.User.Name}
			identityHash := s.Config.digest("identity:"+s.Config.Environment, person.PersonalNumber)
			mapping, findErr := tx.FindFirstRecordByFilter("participant_identities", "study={:study} && environment={:env} && identityHash={:hash}", dbx.Params{"study": s.Config.StudyID, "env": s.Config.Environment, "hash": identityHash})
			if findErr != nil && !errors.Is(findErr, sql.ErrNoRows) {
				return findErr
			}
			if order.GetString("purpose") == "auth" {
				if mapping == nil {
					outcome = "rejected"
					hint = "participantNotFound"
				} else {
					user, err = tx.FindRecordById("users", mapping.GetString("user"))
					if err != nil {
						return err
					}
				}
			} else {
				doc, err := currentDocument(tx, s.Config.StudyID)
				if err != nil {
					return err
				}
				if doc.Id != order.GetString("version") || secrets.Document == nil || secrets.Document.DocumentHash != doc.GetString("documentHash") {
					outcome = "rejected"
					hint = "consentChanged"
				}
				if outcome == "accepted" {
					if flow.GetString("user") != "" {
						user, err = tx.FindRecordById("users", flow.GetString("user"))
					} else {
						user, err = tx.FindFirstRecordByData("users", "username", flow.GetString("participantId"))
					}
					if err != nil && !errors.Is(err, sql.ErrNoRows) {
						return err
					}
					if mapping != nil && (user == nil || mapping.GetString("user") != user.Id) {
						outcome = "rejected"
						hint = "identityConflict"
					}
					if user != nil {
						bound, boundErr := tx.FindFirstRecordByFilter("participant_identities", "study={:study} && user={:user}", dbx.Params{"study": s.Config.StudyID, "user": user.Id})
						if boundErr != nil && !errors.Is(boundErr, sql.ErrNoRows) {
							return boundErr
						}
						if bound != nil && (bound.GetString("identityHash") != identityHash || bound.GetString("environment") != s.Config.Environment) {
							outcome = "rejected"
							hint = "wrongSigner"
						}
					}
					if outcome == "accepted" {
						if id := flow.GetString("invitation"); id != "" {
							invite, err := tx.FindRecordById("study_invitations", id)
							if err != nil {
								return err
							}
							if invite.GetString("claimedFlow") != flow.Id || (invite.GetBool("consumed") && flow.GetString("user") == "") || (!invite.GetBool("consumed") && invite.GetInt("expiresAt") <= int(s.now().Unix())) {
								outcome = "rejected"
								hint = "invitationUnavailable"
							}
						}
					}
					if outcome == "accepted" {
						if user == nil {
							user, err = newRecord(tx, "users")
							if err != nil {
								return err
							}
							user.Set("username", flow.GetString("participantId"))
							user.SetPassword(randomSecret())
							if err := tx.Save(user); err != nil {
								return err
							}
						}
						if mapping == nil {
							mapping, err = newRecord(tx, "participant_identities")
							if err != nil {
								return err
							}
							mapping.Set("study", s.Config.StudyID)
							mapping.Set("environment", s.Config.Environment)
							mapping.Set("user", user.Id)
							mapping.Set("identityHash", identityHash)
							encrypted, err := s.Config.seal("identity:"+user.Id, person)
							if err != nil {
								return err
							}
							mapping.Set("identityCipher", encrypted)
							if err := tx.Save(mapping); err != nil {
								return err
							}
						}
					}
				}
			}
		}
		if order.GetString("purpose") == "sign" {
			signature, err := newRecord(tx, "consent_signatures")
			if err != nil {
				return err
			}
			signature.Set("study", s.Config.StudyID)
			signature.Set("environment", s.Config.Environment)
			signature.Set("orderHash", order.GetString("orderHash"))
			signature.Set("order", order.Id)
			signature.Set("version", order.GetString("version"))
			signature.Set("outcome", outcome)
			signature.Set("receivedAt", s.now().Unix())
			if user != nil {
				signature.Set("user", user.Id)
			}
			encrypted, err := s.Config.seal("signature:"+order.Id, map[string]any{"request": secrets.Request, "document": secrets.Document, "completion": raw, "receivedAt": s.now().UTC().Format(time.RFC3339Nano)})
			if err != nil {
				return err
			}
			signature.Set("evidenceCipher", encrypted)
			if err := tx.Save(signature); err != nil {
				return err
			}
			order.Set("signatureId", signature.Id)
			if outcome == "accepted" {
				if user.GetString("consentSignature") != "" {
					if err := consentEvent(tx, s.Config.StudyID, user.Id, user.GetString("consentSignature"), "superseded", s.now()); err != nil {
						return err
					}
				}
				user.Set("consentStatus", "accepted")
				user.Set("consentVersion", order.GetString("version"))
				user.Set("consentSignature", signature.Id)
				if err := tx.Save(user); err != nil {
					return err
				}
				if err := consentEvent(tx, s.Config.StudyID, user.Id, signature.Id, "accepted", s.now()); err != nil {
					return err
				}
				if id := flow.GetString("invitation"); id != "" {
					invite, err := tx.FindRecordById("study_invitations", id)
					if err != nil {
						return err
					}
					invite.Set("consumed", true)
					if err := tx.Save(invite); err != nil {
						return err
					}
				}
			}
		}
		if outcome == "accepted" {
			flow.Set("user", user.Id)
			flow.Set("participantId", user.GetString("username"))
			flow.Set("authenticatedAt", s.now().Unix())
			encrypted, err := s.Config.seal("flow:"+flow.Id, person)
			if err != nil {
				return err
			}
			flow.Set("identityCipher", encrypted)
			if err := tx.Save(flow); err != nil {
				return err
			}
		}
		order.Set("status", outcome)
		order.Set("hintCode", hint)
		return tx.Save(order)
	})
}

func validBase64(value string) bool {
	raw, err := base64.StdEncoding.DecodeString(value)
	return err == nil && len(raw) > 0
}
func consentEvent(app core.App, study, user, signature, kind string, now time.Time) error {
	r, err := newRecord(app, "consent_events")
	if err != nil {
		return err
	}
	r.Set("study", study)
	r.Set("user", user)
	r.Set("signature", signature)
	r.Set("kind", kind)
	r.Set("occurredAt", now.Unix())
	return app.Save(r)
}

func (s *Service) activeConsent(app core.App, user *core.Record) bool {
	doc, err := currentDocument(app, s.Config.StudyID)
	if err != nil {
		return false
	}
	if user.GetString("consentStatus") != "accepted" || user.GetString("consentVersion") != doc.Id {
		return false
	}
	signature, err := app.FindRecordById("consent_signatures", user.GetString("consentSignature"))
	if err != nil || signature.GetString("outcome") != "accepted" || signature.GetString("environment") != s.Config.Environment || signature.GetString("study") != s.Config.StudyID || signature.GetString("user") != user.Id || signature.GetString("version") != doc.Id {
		return false
	}
	var evidence struct {
		Document   Document       `json:"document"`
		Request    bankid.Request `json:"request"`
		Completion bankid.Result  `json:"completion"`
	}
	if err := s.Config.open("signature:"+signature.GetString("order"), signature.GetString("evidenceCipher"), &evidence); err != nil {
		return false
	}
	completion := evidence.Completion.CompletionData
	return evidence.Document.ID == doc.Id && evidence.Document.DocumentHash == doc.GetString("documentHash") && evidence.Document.Text == doc.GetString("text") && evidence.Request.UserVisibleData == base64.StdEncoding.EncodeToString([]byte(evidence.Document.Text)) && completion != nil && completion.Risk == "low" && validBase64(completion.Signature) && validBase64(completion.OCSPResponse)
}

func (s *Service) String() string {
	return fmt.Sprintf("BankID %s / %s", s.Config.Environment, s.Config.StudyID)
}
