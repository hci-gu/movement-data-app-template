package study

import (
	"app/internal/kontoapi"
	_ "app/migrations"
	"bytes"
	"context"
	"encoding/json"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"net/http"
	"testing"
	"time"
)

type fakeProvider struct {
	result          kontoapi.Result
	calls, collects int
	startErr        error
}

func (f *fakeProvider) Start(context.Context, string) (kontoapi.Order, error) {
	f.calls++
	if f.startErr != nil {
		return kontoapi.Order{}, f.startErr
	}
	return kontoapi.Order{OrderRef: randomSecret(), AutoStartToken: "auto"}, nil
}
func (f *fakeProvider) Collect(context.Context, string) (kontoapi.Result, error) {
	f.collects++
	return f.result, nil
}
func (f *fakeProvider) QRImage(context.Context, kontoapi.Order) (string, error) {
	return "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII=", nil
}
func testService(t *testing.T) (*Service, *fakeProvider, http.Handler) {
	t.Helper()
	app := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: t.TempDir()})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { app.ResetBootstrapState() })
	if err := app.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	if _, err := PublishConsent(app, "v1", "Consent", "I agree to participate."); err != nil {
		t.Fatal(err)
	}
	cfg := Config{PublicURL: "https://study.example", ActiveKey: "test", EncryptionKeys: map[string][]byte{"test": bytes.Repeat([]byte{1}, 32)}}
	f := &fakeProvider{result: kontoapi.Result{Status: "pending"}}
	s := &Service{App: app, Config: cfg, provider: f, attempts: map[string]*Attempt{}, rates: map[string]rateEntry{}, now: time.Now}
	ProtectRecords(app)
	router, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	s.RegisterRoutes(router)
	router.GET("/protected-test", func(e *core.RequestEvent) error { return e.NoContent(204) }).BindFunc(s.RequireSession).BindFunc(s.RequireConsent)
	handler, err := router.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	return s, f, handler
}
func authenticate(t *testing.T, s *Service, f *fakeProvider, h http.Handler, pn string) (*Attempt, string, *core.Record) {
	t.Helper()
	secret := randomSecret()
	a, err := s.Start(context.Background(), StartInput{ClientSecret: secret, Mode: "qr"}, "192.0.2.1")
	if err != nil {
		t.Fatal(err)
	}
	f.result = kontoapi.Result{Status: "complete", PersonalNumber: pn, Raw: pn + ":Test:Person"}
	if err = s.advance(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	if a.Status != "accepted" {
		t.Fatalf("auth: %s %s", a.Status, a.Hint)
	}
	w := questionnaireRequest(h, "GET", "/api/bankid/attempts/"+a.ID, a.ID+"."+secret, nil)
	if w.Code != 200 {
		t.Fatalf("grant: %d %s", w.Code, w.Body)
	}
	var result struct {
		Grant grant `json:"grant"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Grant.Token == "" {
		t.Fatal("missing auth session")
	}
	user, err := s.App.FindRecordById("users", a.UserID)
	if err != nil {
		t.Fatal(err)
	}
	return a, result.Grant.Token, user
}
func acceptInput(t *testing.T, s *Service) AcceptanceInput {
	t.Helper()
	doc, err := currentDocument(s.App)
	if err != nil {
		t.Fatal(err)
	}
	return AcceptanceInput{Version: doc.Id, DocumentHash: doc.GetString("documentHash"), RequestID: randomSecret()}
}
func requireCode(t *testing.T, h http.Handler, method, path, token string, body any, code int) {
	t.Helper()
	w := questionnaireRequest(h, method, path, token, body)
	if w.Code != code {
		t.Fatalf("%s %s: got %d want %d: %s", method, path, w.Code, code, w.Body)
	}
}
func TestAuthenticationRequiresExplicitConsentAndWithdrawalRevokes(t *testing.T) {
	s, f, h := testService(t)
	a, token, user := authenticate(t, s, f, h, "200001012384")
	if s.activeConsent(s.App, user) {
		t.Fatal("authentication accepted consent")
	}
	requireCode(t, h, "GET", "/api/me", token, nil, 200)
	requireCode(t, h, "GET", "/protected-test", token, nil, 403)
	requireCode(t, h, "GET", "/api/collections/questionnaires/records", token, nil, 403)
	requireCode(t, h, "GET", "/api/consent/current", "", nil, 401)
	in := acceptInput(t, s)
	requireCode(t, h, "POST", "/api/consent/accept", token, in, 200)
	requireCode(t, h, "POST", "/api/consent/accept", token, in, 200)
	records, _ := s.App.FindAllRecords("consent_acceptances")
	if len(records) != 1 || records[0].GetString("authentication") != a.AuthenticationID {
		t.Fatal("acceptance not linked or not idempotent")
	}
	requireCode(t, h, "GET", "/protected-test", token, nil, 204)
	requireCode(t, h, "GET", "/api/consent/receipt", token, nil, 200)
	requireCode(t, h, "POST", "/consent/accept", token, in, 404)
	requireCode(t, h, "POST", "/api/consent/withdraw", token, nil, 204)
	requireCode(t, h, "GET", "/api/me", token, nil, 401)
	requireCode(t, h, "GET", "/protected-test", token, nil, 401)
	if s.activeConsent(s.App, user) {
		t.Fatal("withdrawal left consent active")
	}
	_, token2, user2 := authenticate(t, s, f, h, "200001012384")
	if user2.Id != user.Id {
		t.Fatal("returning identity duplicated user")
	}
	requireCode(t, h, "GET", "/protected-test", token2, nil, 403)
	requireCode(t, h, "POST", "/api/consent/accept", token2, in, 409)
	requireCode(t, h, "POST", "/api/consent/accept", token2, acceptInput(t, s), 200)
	requireCode(t, h, "GET", "/protected-test", token2, nil, 204)
}
func TestChangedConsentAndIdempotencyConflict(t *testing.T) {
	s, f, h := testService(t)
	_, token, _ := authenticate(t, s, f, h, "200001012384")
	old := acceptInput(t, s)
	if _, err := PublishConsent(s.App, "v2", "Consent", "Updated information."); err != nil {
		t.Fatal(err)
	}
	requireCode(t, h, "POST", "/api/consent/accept", token, old, 409)
	current := acceptInput(t, s)
	requireCode(t, h, "POST", "/api/consent/accept", token, current, 200)
	current.DocumentHash = "wrong"
	requireCode(t, h, "POST", "/api/consent/accept", token, current, 409)
	if _, err := PublishConsent(s.App, "v3", "Consent", "Third version."); err != nil {
		t.Fatal(err)
	}
	requireCode(t, h, "GET", "/protected-test", token, nil, 403)
	requireCode(t, h, "GET", "/api/me", token, nil, 200)
}
func TestAttemptsAreOwnedIdempotentAndLocalCancellationIsTerminal(t *testing.T) {
	s, f, h := testService(t)
	secret := randomSecret()
	in := StartInput{ClientSecret: secret, Mode: "qr"}
	a, err := s.Start(context.Background(), in, "192.0.2.1")
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Start(context.Background(), in, "192.0.2.1")
	if err != nil || a != b || f.calls != 1 {
		t.Fatal("duplicate upstream start")
	}
	requireCode(t, h, "GET", "/api/bankid/attempts/"+a.ID, a.ID+"."+randomSecret(), nil, 410)
	in.Mode = "sameDevice"
	if _, err = s.Start(context.Background(), in, "192.0.2.1"); err == nil {
		t.Fatal("changed attempt accepted")
	}
	if err = s.cancel(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	f.result = kontoapi.Result{Status: "complete", PersonalNumber: "200001012384"}
	if err = s.advance(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	if f.collects != 0 || a.Status != "cancelled" {
		t.Fatal("late result used after local stop")
	}
	users, _ := s.App.FindAllRecords("users")
	if len(users) != 0 {
		t.Fatal("cancel created user")
	}
}
func TestInvalidOrExpiredIdentityNeverGrantsSession(t *testing.T) {
	for _, pn := range []string{"", "200013992384", "209901012384"} {
		t.Run(pn, func(t *testing.T) {
			s, f, _ := testService(t)
			a, err := s.Start(context.Background(), StartInput{ClientSecret: randomSecret(), Mode: "qr"}, "192.0.2.1")
			if err != nil {
				t.Fatal(err)
			}
			f.result = kontoapi.Result{Status: "complete", PersonalNumber: pn}
			if err = s.advance(context.Background(), a); err != nil {
				t.Fatal(err)
			}
			if a.Status == "accepted" || a.UserID != "" {
				t.Fatal("invalid identity granted access")
			}
		})
	}
	s, f, _ := testService(t)
	a, _ := s.Start(context.Background(), StartInput{ClientSecret: randomSecret(), Mode: "qr"}, "192.0.2.1")
	s.now = func() time.Time { return a.ExpiresAt }
	if err := s.advance(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	if a.Status != "expired" || f.collects != 0 {
		t.Fatal("expired attempt collected")
	}
}
func TestConsentEvidenceTamperingDeniesAccess(t *testing.T) {
	s, f, h := testService(t)
	_, token, user := authenticate(t, s, f, h, "200001012384")
	requireCode(t, h, "POST", "/api/consent/accept", token, acceptInput(t, s), 200)
	r, err := s.activeAcceptance(s.App, user)
	if err != nil {
		t.Fatal(err)
	}
	r.Set("evidenceCipher", "damaged")
	if err = s.App.Save(r); err != nil {
		t.Fatal(err)
	}
	requireCode(t, h, "GET", "/protected-test", token, nil, 403)
	requireCode(t, h, "GET", "/api/consent/receipt", token, nil, 503)
}

func TestReceiptUsesAcceptedDocumentSnapshot(t *testing.T) {
	s, f, h := testService(t)
	_, token, _ := authenticate(t, s, f, h, "200001012384")
	requireCode(t, h, "POST", "/api/consent/accept", token, acceptInput(t, s), 200)
	doc, err := currentDocument(s.App)
	if err != nil {
		t.Fatal(err)
	}
	original := documentDTO(doc)
	doc.Set("text", "An administrator changed this text.")
	doc.Set("documentHash", hash(doc.GetString("text")))
	if err = s.App.Save(doc); err != nil {
		t.Fatal(err)
	}
	w := questionnaireRequest(h, "GET", "/api/consent/receipt", token, nil)
	var receipt struct {
		Document Document `json:"document"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &receipt) != nil || receipt.Document != original {
		t.Fatalf("receipt did not preserve accepted text: %d %s", w.Code, w.Body)
	}
}
