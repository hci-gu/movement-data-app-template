package study

import (
	"app/internal/bankid"
	_ "app/migrations"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

type fakeBankID struct {
	calls     int
	req       bankid.Request
	result    bankid.Result
	err       error
	cancelled bool
}

func (f *fakeBankID) Start(_ context.Context, purpose string, req bankid.Request) (bankid.Order, error) {
	f.calls++
	f.req = req
	return bankid.Order{OrderRef: fmt.Sprintf("order-%d", f.calls), AutoStartToken: "auto", QRStartToken: "qr", QRStartSecret: "secret"}, f.err
}
func (f *fakeBankID) Collect(_ context.Context, ref string) (bankid.Result, error) {
	r := f.result
	r.OrderRef = ref
	return r, f.err
}
func (f *fakeBankID) Cancel(context.Context, string) error { f.cancelled = true; return f.err }

func testService(t *testing.T) (*Service, *fakeBankID, http.Handler) {
	t.Helper()
	app := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: t.TempDir()})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	if err := app.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.ResetBootstrapState() })
	now := time.Now().Truncate(time.Second)
	cfg := Config{Environment: "test", StudyID: "study", AppID: "org.example.study", ReturnURL: "researchsteps://bankid/return", ActiveKey: "v1", EncryptionKeys: map[string][]byte{"v1": bytes.Repeat([]byte{1}, 32)}, IdentityKey: bytes.Repeat([]byte{2}, 32), SigningEnabled: true}
	fake := &fakeBankID{result: bankid.Result{Status: "pending", HintCode: "userSign"}}
	s := &Service{App: app, Config: cfg, providers: map[string]bankid.Provider{"test-cert": fake}, activeCertificate: "test-cert", now: func() time.Time { return now }, rates: map[string]rateEntry{}}
	doc, _ := newRecord(app, "consent_versions")
	doc.Set("study", "study")
	doc.Set("version", "v1")
	doc.Set("title", "Study consent")
	doc.Set("text", "I consent to sharing step data for this study.")
	doc.Set("documentHash", hash(doc.GetString("text")))
	if err := app.Save(doc); err != nil {
		t.Fatal(err)
	}
	settings, _ := newRecord(app, "study_settings")
	settings.Set("study", "study")
	settings.Set("currentVersion", doc.Id)
	if err := app.Save(settings); err != nil {
		t.Fatal(err)
	}
	ProtectRecords(app)
	s.RegisterAdmin()
	r, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	s.RegisterRoutes(r)
	r.POST("/upload-test", func(e *core.RequestEvent) error { return e.NoContent(204) }).BindFunc(s.RequireSession, s.RequireConsent)
	mux, err := r.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	return s, fake, mux
}

func request(t *testing.T, h http.Handler, method, path, token string, body any) (int, map[string]any) {
	t.Helper()
	raw, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, bytes.NewReader(raw))
	r.RemoteAddr = "192.0.2.1:12345"
	r.Header.Set("Content-Type", "application/json")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	result := map[string]any{}
	_ = json.Unmarshal(w.Body.Bytes(), &result)
	return w.Code, result
}

func enroll(t *testing.T, s *Service, h http.Handler, expected string) (string, *core.Record) {
	t.Helper()
	invite, _ := newRecord(s.App, "study_invitations")
	invite.Set("study", "study")
	invite.Set("participantId", "PART-001")
	invite.Set("tokenHash", s.Config.digest("invitation", "invite-code"))
	invite.Set("expiresAt", s.now().Add(time.Hour).Unix())
	if err := s.App.Save(invite); err != nil {
		t.Fatal(err)
	}
	if expected != "" {
		sealed, _ := s.Config.seal("invitation:"+invite.Id, identity{PersonalNumber: expected})
		invite.Set("expectedCipher", sealed)
		if err := s.App.Save(invite); err != nil {
			t.Fatal(err)
		}
	}
	secret := randomSecret()
	code, res := request(t, h, "POST", "/api/study/enrollments", "", map[string]any{"kind": "enroll", "invitationCode": "invite-code", "clientSecret": secret})
	if code != 200 {
		t.Fatal(code, res)
	}
	id := res["id"].(string)
	flow, _ := s.App.FindRecordById("enrollment_sessions", id)
	return id + "." + secret, flow
}

func start(t *testing.T, s *Service, h http.Handler, token string) *core.Record {
	t.Helper()
	doc, _ := currentDocument(s.App, "study")
	code, res := request(t, h, "POST", "/api/study/bankid-orders", token, StartInput{Version: doc.Id, DocumentHash: doc.GetString("documentHash"), Mode: "sameDevice", RequestKey: "idempotency-key-123"})
	if code != 200 {
		t.Fatal(code, res)
	}
	order, err := s.App.FindRecordById("bankid_orders", res["id"].(string))
	if err != nil {
		t.Fatal(err)
	}
	return order
}

func completed(f *fakeBankID, pnr, risk string) {
	f.result = bankid.Result{Status: "complete", CompletionData: &bankid.Completion{User: bankid.User{PersonalNumber: pnr, Name: "Test Signer"}, Signature: base64.StdEncoding.EncodeToString([]byte("<signed-evidence/>")), OCSPResponse: base64.StdEncoding.EncodeToString([]byte("ocsp")), Risk: risk}}
}

func TestSigningEvidenceSessionAndUploadGate(t *testing.T) {
	s, f, h := testService(t)
	token, flow := enroll(t, s, h, "")
	order := start(t, s, h, token)
	if code, _ := request(t, h, "POST", "/upload-test", "", nil); code != 401 {
		t.Fatal("unsigned upload accepted", code)
	}
	if code, _ := request(t, h, "POST", "/api/study/enrollments/"+flow.Id+"/complete", token, nil); code != 409 {
		t.Fatal("pending order exchanged", code)
	}
	completed(f, "200001012384", "low")
	if err := s.advance(context.Background(), order.Id); err != nil {
		t.Fatal(err)
	}
	if err := s.advance(context.Background(), order.Id); err != nil {
		t.Fatal(err)
	}
	signatures, err := s.App.FindAllRecords("consent_signatures")
	if err != nil || len(signatures) != 1 {
		t.Fatal("completion not idempotent", err, len(signatures))
	}
	if strings.Contains(signatures[0].GetString("evidenceCipher"), "200001012384") {
		t.Fatal("plaintext identity in evidence")
	}
	var evidence map[string]any
	if err := s.Config.open("signature:"+order.Id, signatures[0].GetString("evidenceCipher"), &evidence); err != nil {
		t.Fatal(err)
	}
	if evidence["request"].(map[string]any)["userVisibleData"] != f.req.UserVisibleData {
		t.Fatal("signed bytes lost")
	}
	code, grant := request(t, h, "POST", "/api/study/enrollments/"+flow.Id+"/complete", token, nil)
	if code != 200 {
		t.Fatal(code, grant)
	}
	auth := grant["token"].(string)
	s.Config.SigningEnabled = false
	_ = start(t, s, h, token) // Recover a saved grant even while new signing is disabled.
	s.Config.SigningEnabled = true
	if f.calls != 1 {
		t.Fatal("grant recovery created another BankID order")
	}
	_, again := request(t, h, "POST", "/api/study/enrollments/"+flow.Id+"/complete", token, nil)
	if again["token"] != auth {
		t.Fatal("grant retry extended the session")
	}
	if code, res := request(t, h, "POST", "/upload-test", auth, nil); code != 204 {
		t.Fatal("valid signed upload blocked", code, res)
	}
	if code, _ := request(t, h, "POST", "/api/collections/users/auth-refresh", auth, nil); code == 200 {
		t.Fatal("session refresh bypass")
	}
	if code, _ := request(t, h, "PATCH", "/api/collections/users/"+signatures[0].GetString("user"), auth, map[string]any{"consentStatus": "accepted"}); code < 400 {
		t.Fatal("direct user mutation allowed")
	}
	if code, _ := request(t, h, "POST", "/api/study/consent/withdraw", auth, nil); code != 204 {
		t.Fatal("withdrawal failed", code)
	}
	if code, _ := request(t, h, "POST", "/upload-test", auth, nil); code != 403 {
		t.Fatal("withdrawn upload accepted", code)
	}
	if code, _ := request(t, h, "POST", "/api/study/logout", auth, nil); code != 204 {
		t.Fatal("logout failed", code)
	}
	if code, _ := request(t, h, "GET", "/api/study/me", auth, nil); code != 401 {
		t.Fatal("revoked session accepted", code)
	}
}

func TestWrongSignerRiskAndMissingEvidenceNeverActivate(t *testing.T) {
	for _, tc := range []struct {
		name, pnr, risk, expected string
		missing                   bool
	}{{"wrongSigner", "200001012384", "low", "199001012384", false}, {"highRisk", "200001012384", "high", "", false}, {"moderateRisk", "200001012384", "moderate", "", false}, {"missingRisk", "200001012384", "", "", false}, {"missingEvidence", "200001012384", "low", "", true}} {
		t.Run(tc.name, func(t *testing.T) {
			s, f, h := testService(t)
			token, flow := enroll(t, s, h, tc.expected)
			order := start(t, s, h, token)
			completed(f, tc.pnr, tc.risk)
			if tc.missing {
				f.result.CompletionData.Signature = ""
			}
			if err := s.advance(context.Background(), order.Id); err != nil {
				t.Fatal(err)
			}
			r, _ := s.App.FindRecordById("bankid_orders", order.Id)
			if r.GetString("status") != "rejected" {
				t.Fatal(r.GetString("status"))
			}
			if code, _ := request(t, h, "POST", "/api/study/enrollments/"+flow.Id+"/complete", token, nil); code != 409 {
				t.Fatal("rejected order exchanged", code)
			}
		})
	}
}

func TestOrderOwnershipNonceAndIdempotency(t *testing.T) {
	s, f, h := testService(t)
	token, flow := enroll(t, s, h, "")
	order := start(t, s, h, token)
	_ = start(t, s, h, token)
	if f.calls != 1 {
		t.Fatal("duplicate provider order")
	}
	if code, _ := request(t, h, "GET", "/api/study/bankid-orders/"+order.Id, flow.Id+"."+randomSecret(), nil); code != 401 {
		t.Fatal("another secret can read order", code)
	}
	if code, _ := request(t, h, "POST", "/api/study/bankid-orders/"+order.Id+"/return", token, map[string]string{"nonce": "wrong"}); code != 400 {
		t.Fatal("wrong nonce accepted", code)
	}
	var secrets orderSecrets
	_ = s.Config.open("order:"+order.Id, order.GetString("requestCipher"), &secrets)
	code, res := request(t, h, "POST", "/api/study/bankid-orders/"+order.Id+"/return", token, map[string]string{"nonce": secrets.Nonce})
	if code != 200 || res["status"] != "pending" {
		t.Fatal("callback manufactured completion", code, res)
	}
	if _, exists := res["qrStartSecret"]; exists {
		t.Fatal("secret exposed")
	}
}

func TestAmbiguousCollectCannotGrantAccess(t *testing.T) {
	s, f, h := testService(t)
	token, _ := enroll(t, s, h, "")
	order := start(t, s, h, token)
	f.err = errors.New("connection reset")
	if err := s.advance(context.Background(), order.Id); err != nil {
		t.Fatal(err)
	}
	r, _ := s.App.FindRecordById("bankid_orders", order.Id)
	if r.GetString("status") != "unresolved" {
		t.Fatal("ambiguous response treated as conclusive")
	}
}

func TestEncryptedValuesBoundToRecordAndOldKeys(t *testing.T) {
	s, _, _ := testService(t)
	sealed, err := s.Config.seal("record:a", identity{PersonalNumber: "200001012384"})
	if err != nil {
		t.Fatal(err)
	}
	var value identity
	if err := s.Config.open("record:b", sealed, &value); err == nil {
		t.Fatal("ciphertext can be substituted")
	}
	s.Config.EncryptionKeys["v2"] = bytes.Repeat([]byte{3}, 32)
	s.Config.ActiveKey = "v2"
	if err := s.Config.open("record:a", sealed, &value); err != nil || value.PersonalNumber != "200001012384" {
		t.Fatal("rotation lost old evidence", err)
	}
}

func TestTrustedProxyChain(t *testing.T) {
	c := Config{TrustedProxies: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}}
	for _, tc := range []struct{ peer, forwarded, want string }{{"192.0.2.1:1", "198.51.100.1", "192.0.2.1"}, {"10.0.0.1:1", "203.0.113.9, 198.51.100.1, 10.0.0.2", "198.51.100.1"}, {"[2001:db8::1]:1", "", "2001:db8::1"}} {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = tc.peer
		r.Header.Set("X-Forwarded-For", tc.forwarded)
		got, err := c.endUserIP(r)
		if err != nil || got != tc.want {
			t.Fatal(got, err, tc.want)
		}
	}
}

func TestReturningAuthenticationRequiresCurrentConsentAndRejectsTamperedEvidence(t *testing.T) {
	s, f, h := testService(t)
	enrollment, flow := enroll(t, s, h, "")
	order := start(t, s, h, enrollment)
	completed(f, "200001012384", "low")
	if err := s.advance(context.Background(), order.Id); err != nil {
		t.Fatal(err)
	}
	_, original := request(t, h, "POST", "/api/study/enrollments/"+flow.Id+"/complete", enrollment, nil)
	originalToken := original["token"].(string)
	doc, _ := currentDocument(s.App, "study")
	next, _ := newRecord(s.App, "consent_versions")
	next.Set("study", "study")
	next.Set("version", "v2")
	next.Set("title", "Updated consent")
	next.Set("text", "I agree to the updated study.")
	next.Set("documentHash", hash(next.GetString("text")))
	if err := s.App.Save(next); err != nil {
		t.Fatal(err)
	}
	settings, _ := s.App.FindFirstRecordByData("study_settings", "study", "study")
	settings.Set("currentVersion", next.Id)
	if err := s.App.Save(settings); err != nil {
		t.Fatal(err)
	}
	if code, _ := request(t, h, "POST", "/upload-test", originalToken, nil); code != 403 {
		t.Fatal("old consent still uploads", code)
	}
	secret := randomSecret()
	code, res := request(t, h, "POST", "/api/study/enrollments", "", map[string]string{"kind": "login", "clientSecret": secret})
	if code != 200 {
		t.Fatal(code, res)
	}
	token := res["id"].(string) + "." + secret
	path := "/api/study/enrollments/" + res["id"].(string) + "/complete"
	code, res = request(t, h, "POST", "/api/study/bankid-orders", token, StartInput{Mode: "qr", RequestKey: "login-request-12345"})
	if code != 200 || res["purpose"] != "auth" {
		t.Fatal(code, res)
	}
	if err := s.advance(context.Background(), res["id"].(string)); err != nil {
		t.Fatal(err)
	}
	code, res = request(t, h, "POST", path, token, nil)
	if code != 200 || res["consentRequired"] != true || res["token"] != "" {
		t.Fatal("login bypassed reconsent", code, res)
	}
	signed := start(t, s, h, token)
	if f.req.Requirement == nil || f.req.Requirement.PersonalNumber != "200001012384" {
		t.Fatal("reconsent lost expected identity")
	}
	if err := s.advance(context.Background(), signed.Id); err != nil {
		t.Fatal(err)
	}
	code, res = request(t, h, "POST", path, token, nil)
	if code != 200 {
		t.Fatal(code, res)
	}
	auth := res["token"].(string)
	if auth == originalToken {
		t.Fatal("separate sessions reused a token")
	}
	if code, res := request(t, h, "POST", "/upload-test", auth, nil); code != 204 {
		t.Fatal(code, res)
	}
	signatures, _ := s.App.FindAllRecords("consent_signatures")
	if len(signatures) != 2 {
		t.Fatal("immutable evidence history lost")
	}
	if old, err := s.App.FindRecordById("consent_versions", doc.Id); err != nil || old.GetString("version") != "v1" {
		t.Fatal("old document changed")
	}
	current, _ := s.App.FindRecordById("bankid_orders", signed.Id)
	signature, _ := s.App.FindRecordById("consent_signatures", current.GetString("signatureId"))
	signature.Set("evidenceCipher", "v1:corrupted")
	if err := s.App.Save(signature); err != nil {
		t.Fatal(err)
	}
	if code, _ := request(t, h, "POST", "/upload-test", auth, nil); code != 403 {
		t.Fatal("tampered evidence still enables uploads", code)
	}
}

func TestCancelCompletionRaceAndExpiredSessions(t *testing.T) {
	s, f, h := testService(t)
	token, flow := enroll(t, s, h, "")
	order := start(t, s, h, token)
	completed(f, "200001012384", "low")
	now := s.now().Add(3 * time.Second)
	s.now = func() time.Time { return now }
	code, res := request(t, h, "POST", "/api/study/bankid-orders/"+order.Id+"/cancel", token, nil)
	if code != 200 || res["status"] != "accepted" || f.cancelled {
		t.Fatal("cancel hid a completed signature", code, res)
	}
	_, grant := request(t, h, "POST", "/api/study/enrollments/"+flow.Id+"/complete", token, nil)
	auth := grant["token"].(string)
	now = now.Add(SessionLifetime + time.Second)
	if code, _ := request(t, h, "GET", "/api/study/me", auth, nil); code != 401 {
		t.Fatal("expired session accepted", code)
	}
	if code, _ := request(t, h, "GET", "/api/study/bankid-orders/"+order.Id, token, nil); code != 401 {
		t.Fatal("expired flow accepted", code)
	}
}

func TestRecoveryAndOperationalPurgePreserveConsent(t *testing.T) {
	s, f, h := testService(t)
	token, flow := enroll(t, s, h, "")
	order := start(t, s, h, token)
	// A response persisted before a process crash can be finalized without collecting again.
	completed(f, "200001012384", "low")
	result := f.result
	var secrets orderSecrets
	_ = s.Config.open("order:"+order.Id, order.GetString("requestCipher"), &secrets)
	result.OrderRef = secrets.Order.OrderRef
	raw, _ := json.Marshal(result)
	sealed, _ := s.Config.seal("result:"+order.Id, json.RawMessage(raw))
	order.Set("resultCipher", sealed)
	order.Set("status", "collected")
	if err := s.App.Save(order); err != nil {
		t.Fatal(err)
	}
	f.err = errors.New("provider unavailable")
	if err := s.Recover(); err != nil {
		t.Fatal(err)
	}
	if err := s.advance(context.Background(), order.Id); err != nil {
		t.Fatal(err)
	}
	flow, _ = s.App.FindRecordById("enrollment_sessions", flow.Id)
	user, _ := s.App.FindRecordById("users", flow.GetString("user"))
	if !s.activeConsent(s.App, user) {
		t.Fatal("durably collected evidence was not recovered")
	}
	if _, err := PurgeSessions(s.App, s.Config, s.now().Add(48*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if !s.activeConsent(s.App, user) {
		t.Fatal("purge deleted signing evidence")
	}
	order, _ = s.App.FindRecordById("bankid_orders", order.Id)
	if order.GetString("requestCipher") != "" || order.GetString("resultCipher") != "" {
		t.Fatal("operational secrets retained")
	}
	// An in-flight collect at a crash boundary must remain inconclusive.
	order.Set("status", "collecting")
	if err := s.App.Save(order); err != nil {
		t.Fatal(err)
	}
	if err := s.Recover(); err != nil {
		t.Fatal(err)
	}
	order, _ = s.App.FindRecordById("bankid_orders", order.Id)
	if order.GetString("status") != "unresolved" {
		t.Fatal("ambiguous crash result accepted")
	}
}
