package study

import (
	"app/internal/kontoapi"
	"app/internal/testidentity"
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/pocketbase/pocketbase/core"
	"regexp"
	"sync"
	"time"
)

var personalNumberPattern = regexp.MustCompile("^[0-9]{12}$")
var secretPattern = regexp.MustCompile("^[A-Za-z0-9_-]{43}$")

const AttemptLifetime = 3 * time.Minute
const ResultLifetime = 10 * time.Minute
const MaxAttempts = 1024

type Service struct {
	simulation *testidentity.Run
	App        core.App
	Config     Config
	provider   kontoapi.Provider
	startMu    sync.Mutex
	mu         sync.Mutex
	attempts   map[string]*Attempt
	rateMu     sync.Mutex
	rates      map[string]rateEntry
	now        func() time.Time
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
	Mode         string `json:"mode"`
}
type Attempt struct {
	Simulation                                    *testidentity.Mapping
	ExpectedGuardian                              string
	mu                                            sync.Mutex
	ID, SecretHash, InputHash                     string
	Purpose, Mode, UserID, GuardianRequestID      string
	Nonce, IP, Status, Hint, Terminal             string
	AuthenticationID, TokenKey, PersonalNumber    string
	StartedAt, ExpiresAt, FinishedAt, NextCollect time.Time
	Order                                         kontoapi.Order
	Result                                        *kontoapi.Result
	LocalError                                    string
	Grant                                         *grant
	QR                                            string
	QRAt                                          time.Time
}

func Open(app core.App, cfg Config) (*Service, error) {
	s := &Service{App: app, Config: cfg, simulation: cfg.Simulation, attempts: map[string]*Attempt{}, rates: map[string]rateEntry{}, now: time.Now}
	s.registerSimulationMappings()
	if cfg.Environment == "disabled" {
		return s, nil
	}
	client, err := kontoapi.NewClient(cfg.KontoAPI)
	if err != nil {
		return nil, err
	}
	s.provider = client
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
func attemptID(secret string) string         { return hash(secret)[:32] }
func (s *Service) lookup(id string) *Attempt { s.mu.Lock(); defer s.mu.Unlock(); return s.attempts[id] }
func (s *Service) snapshotAttempts() []*Attempt {
	s.mu.Lock()
	defer s.mu.Unlock()
	all := make([]*Attempt, 0, len(s.attempts))
	for _, a := range s.attempts {
		all = append(all, a)
	}
	return all
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
		return nil, problem(400, "invalidRequest", "Invalid authentication request.")
	}
	s.startMu.Lock()
	defer s.startMu.Unlock()
	encoded, _ := json.Marshal(in)
	id, fingerprint := attemptID(in.ClientSecret), hash(string(encoded))
	if a := s.lookup(id); a != nil {
		a.mu.Lock()
		defer a.mu.Unlock()
		if !s.alive(a) {
			return nil, restartRequired()
		}
		if a.InputHash != fingerprint || a.Purpose != "participant" {
			return nil, problem(409, "requestChanged", "Start a new authentication request.")
		}
		if a.Mode == "sameDevice" && a.IP != ip {
			return nil, problem(409, "connectionChanged", "Return to the original connection or start again.")
		}
		return a, nil
	}
	if _, err := s.App.FindFirstRecordByData("authentications", "attemptId", id); err == nil {
		return nil, restartRequired()
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	a := &Attempt{ID: id, SecretHash: hash(in.ClientSecret), InputHash: fingerprint, Purpose: "participant", Mode: in.Mode, IP: ip, Nonce: randomSecret(), Status: "pending", StartedAt: s.now(), ExpiresAt: s.now().Add(AttemptLifetime)}
	return s.startOrder(ctx, a)
}

// startMu must be held. Register before sending to avoid duplicate upstream
// orders after an ambiguous response. No consent is accepted here.
func (s *Service) startOrder(ctx context.Context, a *Attempt) (*Attempt, error) {
	if s.provider == nil {
		return nil, problem(503, "authenticationUnavailable", "BankID is currently unavailable.")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	s.mu.Lock()
	if len(s.attempts) >= MaxAttempts {
		s.mu.Unlock()
		return nil, problem(503, "busy", "Please wait before starting another request.")
	}
	s.attempts[a.ID] = a
	s.mu.Unlock()
	order, err := s.provider.Start(ctx, a.IP)
	if err != nil {
		a.LocalError = err.Error()
		a.Terminal = "unknown"
		a.Hint = "unknownResult"
		var apiErr *kontoapi.APIError
		if errors.As(err, &apiErr) {
			a.Terminal = "failed"
			a.Hint = "temporarilyUnavailable"
		}
		return a, s.finalize(a)
	}
	a.Order = order
	a.NextCollect = s.now().Add(2 * time.Second)
	a.Hint = "outstandingTransaction"
	return a, nil
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
	var wg sync.WaitGroup
	slots := make(chan struct{}, 8)
	for _, a := range s.snapshotAttempts() {
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
					s.App.Logger().Error("Authentication result could not be saved", "attempt", a.ID)
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
func (s *Service) advance(ctx context.Context, a *Attempt) error {
	if a.Status != "pending" {
		return nil
	}
	if a.Terminal != "" {
		return s.finalize(a)
	}
	if !s.now().Before(a.ExpiresAt) {
		a.Terminal = "expired"
		a.Hint = "expiredTransaction"
		return s.finalize(a)
	}
	result, err := s.provider.Collect(ctx, a.Order.OrderRef)
	a.NextCollect = s.now().Add(2 * time.Second)
	if err != nil {
		var apiErr *kontoapi.APIError
		if errors.As(err, &apiErr) && (apiErr.Status == 429 || apiErr.Status >= 500) {
			a.Hint = "temporarilyUnavailable"
			a.NextCollect = s.now().Add(5 * time.Second)
			return nil
		}
		a.LocalError = err.Error()
		a.Terminal = "unknown"
		a.Hint = "unknownResult"
		return s.finalize(a)
	}
	if result.Status == "pending" {
		return nil
	}
	a.Result = &result
	a.Terminal = "rejected"
	a.Hint = "invalidIdentity"
	if result.Status == "complete" {
		a.Terminal = "accepted"
		a.Hint = ""
	}
	return s.finalize(a)
}

// KontoAPI documents no remote cancellation. A local stop is terminal: late
// upstream success cannot grant access, and this attempt is never polled again.
func (s *Service) cancel(_ context.Context, a *Attempt) error {
	if a.Status != "pending" {
		return nil
	}
	if a.Terminal != "" {
		return s.finalize(a)
	}
	a.Terminal = "cancelled"
	a.Hint = "userCancel"
	return s.finalize(a)
}
func (s *Service) qrImage(ctx context.Context, a *Attempt) (string, error) {
	if a.Status != "pending" || a.Terminal != "" || !s.now().Before(a.ExpiresAt) {
		return "", restartRequired()
	}
	if a.QR != "" && s.now().Sub(a.QRAt) < time.Second {
		return a.QR, nil
	}
	value, err := s.provider.QRImage(ctx, a.Order)
	if err != nil {
		return "", problem(503, "qrUnavailable", "The QR code could not be refreshed. Try again.")
	}
	a.QR = value
	a.QRAt = s.now()
	return value, nil
}
