package study

import (
	"app/internal/bankid"
	_ "app/migrations"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
)

type fakeProvider struct {
	result   bankid.Result
	calls    int
	purpose  string
	request  bankid.Request
	startErr error
}

func (f *fakeProvider) Start(_ context.Context, purpose string, request bankid.Request) (bankid.Order, error) {
	f.calls++
	f.purpose = purpose
	f.request = request
	if f.startErr != nil {
		return bankid.Order{}, f.startErr
	}
	return bankid.Order{OrderRef: randomSecret(), AutoStartToken: "auto", QRStartToken: "qr-token", QRStartSecret: "qr-secret"}, nil
}
func (f *fakeProvider) Collect(_ context.Context, ref string) (bankid.Result, error) {
	r := f.result
	r.OrderRef = ref
	return r, nil
}
func (f *fakeProvider) Cancel(context.Context, string) error { return nil }

func TestBankIDCreatesUserAndRelatesSignature(t *testing.T) {
	app := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: t.TempDir()})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	defer app.ResetBootstrapState()
	if err := app.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	userCollection, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"study", "environment", "active", "invitationCode", "validityHours", "identityCipher", "consentStatus", "consentSignature", "withdrawnAt", "username", "name", "avatar", "event_date", "app_type", "created", "updated"} {
		if userCollection.Fields.GetByName(name) != nil {
			t.Fatalf("unwanted user field %s", name)
		}
	}
	cfg := Config{Environment: "test", AppID: "example.app", ReturnURL: "researchsteps://bankid/return", ActiveKey: "one", EncryptionKeys: map[string][]byte{"one": bytes.Repeat([]byte{1}, 32)}}
	if _, err := PublishConsent(app, "v1", "Consent", "I consent."); err != nil {
		t.Fatal(err)
	}
	f := &fakeProvider{result: bankid.Result{Status: "complete", CompletionData: &bankid.Completion{User: bankid.User{PersonalNumber: "200001012384"}, Signature: base64.StdEncoding.EncodeToString([]byte("signature")), OCSPResponse: base64.StdEncoding.EncodeToString([]byte("ocsp")), Risk: "low"}}}
	s := &Service{App: app, Config: cfg, provider: f, attempts: map[string]*Attempt{}, rates: map[string]rateEntry{}, now: time.Now}
	signDoc, err := currentDocument(app)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Start(context.Background(), StartInput{ClientSecret: randomSecret(), Mode: "qr"}, "192.0.2.1"); err == nil {
		t.Fatal("attempt without reviewed consent was accepted")
	}
	secret := randomSecret()
	auth, err := s.Start(context.Background(), StartInput{ClientSecret: secret, Mode: "qr", Version: signDoc.Id, DocumentHash: signDoc.GetString("documentHash")}, "192.0.2.1")
	if err != nil {
		t.Fatal(err)
	}
	if f.calls != 1 || f.purpose != "sign" || f.request.UserVisibleData != base64.StdEncoding.EncodeToString([]byte(signDoc.GetString("text"))) {
		t.Fatal("consent was not the single BankID request")
	}
	if err := s.advance(context.Background(), auth); err != nil {
		t.Fatal(err)
	}
	if auth.Status != "accepted" {
		t.Fatalf("sign: %s %s", auth.Status, auth.Hint)
	}
	users, err := app.FindAllRecords("users")
	if err != nil || len(users) != 1 {
		t.Fatalf("users: %d %v", len(users), err)
	}
	user := users[0]
	if user.GetString("personalNumber") != "200001012384" {
		t.Fatal("personal number missing")
	}
	sig, err := app.FindRecordById("signatures", auth.SignatureID)
	if err != nil || sig.GetString("user") != user.Id {
		t.Fatalf("signature user relation: %v", err)
	}
	if !s.activeConsent(app, user) {
		t.Fatal("signature did not activate consent")
	}
	router, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	s.RegisterRoutes(router)
	handler, err := router.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("GET", "/api/bankid/attempts/"+auth.ID, nil)
	request.Header.Set("Authorization", "Bearer "+auth.ID+"."+secret)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	var status map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if response.Code != 200 || status["consentRequired"] == true || status["grant"] == nil {
		t.Fatalf("single signature did not grant a session: %d %v", response.Code, status)
	}
	second, err := s.Start(context.Background(), StartInput{ClientSecret: randomSecret(), Mode: "qr", Version: signDoc.Id, DocumentHash: signDoc.GetString("documentHash")}, "192.0.2.1")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.advance(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	users, err = app.FindAllRecords("users")
	if err != nil || len(users) != 1 || second.UserID != user.Id {
		t.Fatal("returning signer did not reuse user")
	}
}

func TestChangedConsentDoesNotCreateUser(t *testing.T) {
	app := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: t.TempDir()})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	defer app.ResetBootstrapState()
	if err := app.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	cfg := Config{Environment: "test", AppID: "example.app", ReturnURL: "researchsteps://bankid/return", ActiveKey: "one", EncryptionKeys: map[string][]byte{"one": bytes.Repeat([]byte{1}, 32)}}
	doc, err := PublishConsent(app, "v1", "Consent", "First version")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeProvider{result: bankid.Result{Status: "complete", CompletionData: &bankid.Completion{User: bankid.User{PersonalNumber: "200001012384"}, Signature: base64.StdEncoding.EncodeToString([]byte("signature")), OCSPResponse: base64.StdEncoding.EncodeToString([]byte("ocsp")), Risk: "low"}}}
	s := &Service{App: app, Config: cfg, provider: f, attempts: map[string]*Attempt{}, rates: map[string]rateEntry{}, now: time.Now}
	a, err := s.Start(context.Background(), StartInput{ClientSecret: randomSecret(), Mode: "qr", Version: doc.Id, DocumentHash: doc.GetString("documentHash")}, "192.0.2.1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PublishConsent(app, "v2", "Consent", "Updated version"); err != nil {
		t.Fatal(err)
	}
	if err := s.advance(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	if a.Status != "rejected" || a.Hint != "consentChanged" {
		t.Fatalf("unexpected result: %s %s", a.Status, a.Hint)
	}
	users, err := app.FindAllRecords("users")
	if err != nil || len(users) != 0 {
		t.Fatalf("user created without current consent: %v", err)
	}
}
