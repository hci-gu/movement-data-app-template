package study

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"sync"
	"time"

	"app/internal/bankid"

	"github.com/pocketbase/pocketbase/core"
)

var personalNumberPattern = regexp.MustCompile(`^[0-9]{12}$`)
var secretPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)

const AttemptLifetime = 20 * time.Minute
const ResultLifetime = 10 * time.Minute
const MaxAttempts = 1024

type Service struct {
	App    core.App
	Config Config

	provider bankid.Provider
	startMu  sync.Mutex
	mu       sync.Mutex
	attempts map[string]*Attempt
	rateMu   sync.Mutex
	rates    map[string]rateEntry
	now      func() time.Time
}

type Document struct {
	ID           string `json:"id"`
	Version      string `json:"version"`
	Title        string `json:"title"`
	Text         string `json:"text"`
	DocumentHash string `json:"documentHash"`
}
type StartInput struct {
	ClientSecret string `json:"clientSecret"`
	Version      string `json:"consentTextId"`
	DocumentHash string `json:"documentHash"`
	Mode         string `json:"mode"`
}

// Attempt is transient. Only its terminal outcome and evidence enter the database.
// Its mutex serializes collection, cancellation and response delivery.
type Attempt struct {
	mu sync.Mutex

	ID, SecretHash, InputHash string
	Purpose, Mode, UserID     string
	GuardianRequestID         string
	Nonce, IP                 string
	Status, Hint, Terminal    string
	SignatureID, TokenKey     string

	StartedAt, ReceivedAt, NextCollect time.Time
	ExpiresAt, FinishedAt, ResultAt    time.Time

	Request       bankid.Request
	Order         bankid.Order
	Document      *Document
	Result        *bankid.Result
	ErrorResponse json.RawMessage
	LocalError    string
	PickedUp      bool
	Grant         *grant
}

func Open(app core.App, cfg Config) (*Service, error) {
	s := &Service{App: app, Config: cfg, attempts: map[string]*Attempt{}, rates: map[string]rateEntry{}, now: time.Now}
	if cfg.Environment == "disabled" {
		return s, nil
	}
	client, err := bankid.NewClient(cfg.Environment, cfg.CertFile, cfg.KeyFile, cfg.CAFile)
	if err != nil {
		return nil, err
	}
	s.provider = client
	app.Logger().Info("BankID configured", "environment", cfg.Environment, "certificateExpires", client.CertificateExpires)
	return s, nil
}
func documentDTO(r *core.Record) Document {
	return Document{r.Id, r.GetString("version"), r.GetString("title"), r.GetString("text"), r.GetString("documentHash")}
}
func currentDocument(app core.App) (*core.Record, error) {
	r, err := app.FindFirstRecordByFilter("consent_texts", "current=true")
	if err != nil {
		return nil, err
	}
	if hash(r.GetString("text")) != r.GetString("documentHash") {
		return nil, errors.New("consent document integrity check failed")
	}
	return r, nil
}
func newRecord(app core.App, collection string) (*core.Record, error) {
	c, err := app.FindCollectionByNameOrId(collection)
	if err != nil {
		return nil, err
	}
	return core.NewRecord(c), nil
}
func (s *Service) lookup(id string) *Attempt { s.mu.Lock(); defer s.mu.Unlock(); return s.attempts[id] }
func attemptID(secret string) string         { return hash(secret)[:32] }

func (s *Service) snapshotAttempts() []*Attempt {
	s.mu.Lock()
	defer s.mu.Unlock()
	attempts := make([]*Attempt, 0, len(s.attempts))
	for _, attempt := range s.attempts {
		attempts = append(attempts, attempt)
	}
	return attempts
}

func (s *Service) owned(credential string) (*Attempt, error) {
	id, secret, ok := splitCredential(credential)
	if !ok {
		return nil, restartRequired()
	}
	a := s.lookup(id)
	if a == nil || subtle.ConstantTimeCompare([]byte(a.SecretHash), []byte(hash(secret))) != 1 {
		return nil, restartRequired()
	}
	return a, nil
}
func (s *Service) alive(a *Attempt) bool { return s.now().Before(a.ExpiresAt.Add(ResultLifetime)) }

func (s *Service) Start(ctx context.Context, in StartInput, ip string) (*Attempt, error) {
	if !secretPattern.MatchString(in.ClientSecret) || (in.Mode != "sameDevice" && in.Mode != "qr") {
		return nil, problem(400, "invalidRequest", "Invalid BankID request.")
	}
	s.startMu.Lock()
	defer s.startMu.Unlock()
	id := attemptID(in.ClientSecret)
	encoded, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	fingerprint := hash(string(encoded))
	if a := s.lookup(id); a != nil {
		a.mu.Lock()
		defer a.mu.Unlock()
		if !s.alive(a) {
			return nil, restartRequired()
		}
		if a.InputHash != fingerprint {
			return nil, problem(409, "requestChanged", "Start a new attempt for a changed request.")
		}
		if a.Mode == "sameDevice" && a.IP != ip {
			return nil, problem(409, "connectionChanged", "Return to the original connection or start again.")
		}
		return a, nil
	}
	if _, err := s.App.FindFirstRecordByData("signatures", "attemptId", id); err == nil {
		return nil, restartRequired()
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if s.provider == nil {
		return nil, problem(503, "signingUnavailable", "BankID is currently unavailable.")
	}
	a := &Attempt{ID: id, SecretHash: hash(in.ClientSecret), InputHash: fingerprint, Purpose: "sign", Mode: in.Mode, IP: ip, Nonce: randomSecret(), Status: "pending", StartedAt: s.now(), ExpiresAt: s.now().Add(AttemptLifetime)}
	doc, err := currentDocument(s.App)
	if err != nil {
		return nil, problem(503, "consentUnavailable", "Study consent is not available yet.")
	}
	if in.Version != doc.Id || in.DocumentHash != doc.GetString("documentHash") {
		return nil, problem(409, "consentChanged", "The consent has changed. Read the current version before signing.")
	}
	d := documentDTO(doc)
	a.Document = &d
	a.Request = bankid.Request{EndUserIP: ip, ReturnRisk: true, App: &bankid.AppInfo{AppIdentifier: s.Config.AppID}}
	if a.Mode == "sameDevice" {
		a.Request.ReturnURL = s.Config.ReturnURL + "#nonce=" + url.QueryEscape(a.Nonce)
	}
	a.Request.UserVisibleData = base64.StdEncoding.EncodeToString([]byte(a.Document.Text))
	manifest, err := json.Marshal(map[string]any{"schemaVersion": 2, "purpose": a.Purpose, "attemptId": id, "consentTextId": in.Version, "documentHash": in.DocumentHash, "nonce": a.Nonce})
	if err != nil {
		return nil, err
	}
	a.Request.UserNonVisibleData = base64.StdEncoding.EncodeToString(manifest)
	a.mu.Lock()
	defer a.mu.Unlock()
	s.mu.Lock()
	if len(s.attempts) >= MaxAttempts {
		s.mu.Unlock()
		return nil, problem(503, "busy", "Please wait before starting another BankID request.")
	}
	s.attempts[id] = a
	s.mu.Unlock()
	a.Order, err = s.provider.Start(ctx, a.Purpose, a.Request)
	if err != nil {
		a.captureError(err)
		a.Terminal = "unknown"
		a.Hint = "unknownResult"
		var e *bankid.APIError
		if errors.As(err, &e) {
			a.Terminal = "failed"
			a.Hint = e.Code
		}
		return a, s.finalize(a)
	}
	a.ReceivedAt = s.now()
	a.NextCollect = s.now().Add(2 * time.Second)
	a.Hint = "outstandingTransaction"
	return a, nil
}
func (a *Attempt) captureError(err error) {
	a.LocalError = err.Error()
	var apiErr *bankid.APIError
	if errors.As(err, &apiErr) {
		a.ErrorResponse = apiErr.Raw
	}
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
	all := s.snapshotAttempts()
	var wg sync.WaitGroup
	slots := make(chan struct{}, 8)
	for _, a := range all {
		select {
		case slots <- struct{}{}:
		case <-ctx.Done():
			wg.Wait()
			return
		}
		wg.Add(1)
		go func(a *Attempt) {
			defer wg.Done()
			defer func() { <-slots }()
			a.mu.Lock()
			defer a.mu.Unlock()
			if a.Status == "pending" && !s.now().Before(a.NextCollect) {
				if err := s.advance(ctx, a); err != nil {
					s.App.Logger().Error("BankID result could not be saved", "attempt", a.ID)
				}
			}
			if a.Status != "pending" && !s.alive(a) {
				s.mu.Lock()
				delete(s.attempts, a.ID)
				s.mu.Unlock()
			}
		}(a)
	}
	wg.Wait()
}

// Caller holds the attempt lock. A received terminal result is retained until its database transaction succeeds.
func (s *Service) advance(ctx context.Context, a *Attempt) error {
	if a.Status != "pending" {
		return nil
	}
	if a.Terminal != "" {
		return s.finalize(a)
	}
	if !s.now().Before(a.ExpiresAt) {
		return s.cancel(ctx, a)
	}
	result, err := s.provider.Collect(ctx, a.Order.OrderRef)
	a.NextCollect = s.now().Add(2 * time.Second)
	if err != nil {
		var apiErr *bankid.APIError
		if errors.As(err, &apiErr) && (apiErr.Status == 429 || apiErr.Status >= 500) {
			a.Hint = "temporarilyUnavailable"
			a.NextCollect = s.now().Add(5 * time.Second)
			return nil
		}
		a.captureError(err)
		a.Terminal = "unknown"
		a.Hint = "unknownResult"
		return s.finalize(a)
	}
	if result.OrderRef != a.Order.OrderRef {
		a.Result = &result
		a.Terminal = "unknown"
		a.Hint = "unknownResult"
		return s.finalize(a)
	}
	switch result.Status {
	case "pending":
		a.Hint = result.HintCode
		if result.HintCode == "userSign" || result.HintCode == "userMrtd" || result.HintCode == "processing" {
			a.PickedUp = true
		}
		return nil
	case "failed":
		a.Terminal = "failed"
		a.Hint = result.HintCode
	case "complete":
		a.Terminal = "accepted"
		a.Hint = ""
	default:
		a.Terminal = "unknown"
		a.Hint = "unknownResult"
	}
	a.Result = &result
	return s.finalize(a)
}
func (s *Service) cancel(ctx context.Context, a *Attempt) error {
	if a.Status != "pending" {
		return nil
	}
	if a.Terminal != "" {
		return s.finalize(a)
	}
	// Collect before cancelling, without recursive expiry handling.
	result, err := s.provider.Collect(ctx, a.Order.OrderRef)
	if err == nil && result.OrderRef == a.Order.OrderRef && (result.Status == "complete" || result.Status == "failed") {
		a.Result = &result
		a.Hint = result.HintCode
		a.Terminal = "failed"
		if result.Status == "complete" {
			a.Terminal = "accepted"
		}
		return s.finalize(a)
	}
	if err := s.provider.Cancel(ctx, a.Order.OrderRef); err != nil {
		a.captureError(err)
		a.Terminal = "unknown"
		a.Hint = "unknownResult"
	} else {
		a.Terminal = "cancelled"
		a.Hint = "userCancel"
	}
	return s.finalize(a)
}
