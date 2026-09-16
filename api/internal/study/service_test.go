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
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/security"
)

type fakeBankID struct {
	mu                       sync.Mutex
	calls, collects, cancels int
	req                      bankid.Request
	result                   bankid.Result
	err                      error
}

func (f *fakeBankID) Start(_ context.Context, purpose string, req bankid.Request) (bankid.Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.req = req
	return bankid.Order{OrderRef: fmt.Sprintf("order-%d", f.calls), AutoStartToken: "auto", QRStartToken: "qr", QRStartSecret: "secret"}, f.err
}
func (f *fakeBankID) Collect(_ context.Context, ref string) (bankid.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.collects++
	r := f.result
	r.OrderRef = ref
	return r, f.err
}
func (f *fakeBankID) Cancel(context.Context, string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cancels++
	return f.err
}
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
	f := &fakeBankID{result: bankid.Result{Status: "pending", HintCode: "userSign"}}
	s := &Service{App: app, Config: cfg, provider: f, attempts: map[string]*Attempt{}, rates: map[string]rateEntry{}, now: func() time.Time { return now }}
	if _, err := PublishConsent(app, cfg, "v1", "Study consent", "I consent to sharing step data for this study."); err != nil {
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
	return s, f, mux
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

func start(t *testing.T, s *Service, h http.Handler, invitation, parent string) (*Attempt, string, StartInput) {
	t.Helper()
	in := StartInput{ClientSecret: randomSecret(), InvitationCode: invitation, AuthAttempt: parent, Mode: "sameDevice"}
	if invitation != "" || parent != "" {
		d, err := currentDocument(s.App, s.Config.StudyID)
		if err != nil {
			t.Fatal(err)
		}
		in.Version = d.Id
		in.DocumentHash = d.GetString("documentHash")
	}
	status, result := request(t, h, "POST", "/api/study/bankid/attempts", "", in)
	if status != 200 {
		t.Fatal(status, result)
	}
	id := result["id"].(string)
	return s.lookup(id), id + "." + in.ClientSecret, in
}
func invite(t *testing.T, s *Service, participant, expected string) string {
	t.Helper()
	_, code, err := IssueInvitation(s.App, s.Config, participant, expected, 24, s.now())
	if err != nil {
		t.Fatal(err)
	}
	return code
}
func completed(f *fakeBankID, pnr, risk string) {
	f.result = bankid.Result{Status: "complete", CompletionData: &bankid.Completion{User: bankid.User{PersonalNumber: pnr, Name: "Test Signer"}, Signature: base64.StdEncoding.EncodeToString([]byte("<signature/>")), OCSPResponse: base64.StdEncoding.EncodeToString([]byte("ocsp")), Risk: risk}}
}
func advance(t *testing.T, s *Service, a *Attempt) {
	t.Helper()
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := s.advance(context.Background(), a); err != nil {
		t.Fatal(err)
	}
}
func status(t *testing.T, h http.Handler, a *Attempt, credential string) map[string]any {
	t.Helper()
	code, res := request(t, h, "GET", "/api/study/bankid/attempts/"+a.ID, credential, nil)
	if code != 200 {
		t.Fatal(code, res)
	}
	return res
}
func session(t *testing.T, h http.Handler, a *Attempt, credential string) string {
	t.Helper()
	r := status(t, h, a, credential)
	g, ok := r["grant"].(map[string]any)
	if !ok {
		t.Fatal("missing grant", r)
	}
	return g["token"].(string)
}
func enroll(t *testing.T, s *Service, f *fakeBankID, h http.Handler) (*Attempt, string, string) {
	t.Helper()
	a, c, _ := start(t, s, h, invite(t, s, "PART-001", ""), "")
	completed(f, "200001012384", "low")
	advance(t, s, a)
	return a, c, session(t, h, a, c)
}

func TestEightCollections(t *testing.T) {
	s, _, _ := testService(t)
	cs, err := s.App.FindAllCollections()
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]bool{"users": true, "answers": true, "dataUploads": true, "signatures": true, "consent_texts": true, "questionnaires": true, "questions": true, "questionOptions": true}
	for _, c := range cs {
		if c.System {
			continue
		}
		if !expected[c.Name] {
			t.Fatal("unexpected collection", c.Name)
		}
		delete(expected, c.Name)
	}
	if len(expected) != 0 {
		t.Fatal(expected)
	}
}
func TestSigningEvidenceLoginWithdrawalAndLogout(t *testing.T) {
	s, f, h := testService(t)
	a, credential, token := enroll(t, s, f, h)
	if a.Status != "accepted" {
		t.Fatal(a.Status, a.Hint)
	}
	if code, res := request(t, h, "POST", "/upload-test", token, nil); code != 204 {
		t.Fatal(code, res)
	}
	if code, res := request(t, h, "GET", "/api/study/consent/receipt", token, nil); code != 200 || res["signatureId"] != a.SignatureID {
		t.Fatal(code, res)
	}
	sig, err := s.App.FindRecordById("signatures", a.SignatureID)
	if err != nil {
		t.Fatal(err)
	}
	var ev Evidence
	if err := s.Config.Open("signature:"+sig.Id, sig.GetString("evidenceCipher"), &ev); err != nil {
		t.Fatal(err)
	}
	if ev.Document.Text != "I consent to sharing step data for this study." || ev.Request.UserVisibleData != base64.StdEncoding.EncodeToString([]byte(ev.Document.Text)) || len(ev.Completion) == 0 {
		t.Fatal("incomplete evidence")
	}
	if token != session(t, h, a, credential) {
		t.Fatal("session delivery changed token")
	}
	login, lc, _ := start(t, s, h, "", "")
	advance(t, s, login)
	second := session(t, h, login, lc)
	if code, _ := request(t, h, "POST", "/api/collections/users/auth-refresh", token, nil); code == 200 {
		t.Fatal("token refreshed")
	}
	if code, _ := request(t, h, "GET", "/api/collections/users/records/"+a.UserID, token, nil); code == 200 {
		t.Fatal("private user exposed")
	}
	if code, _ := request(t, h, "POST", "/api/study/consent/withdraw", token, nil); code != 204 {
		t.Fatal(code)
	}
	if code, _ := request(t, h, "POST", "/upload-test", second, nil); code != 403 {
		t.Fatal("withdrawal did not block upload", code)
	}
	sig, _ = s.App.FindRecordById("signatures", a.SignatureID)
	if sig.GetInt("withdrawnAt") == 0 {
		t.Fatal("withdrawal missing")
	}
	if code, _ := request(t, h, "POST", "/api/study/logout", token, nil); code != 204 {
		t.Fatal(code)
	}
	for _, tok := range []string{token, second} {
		if code, _ := request(t, h, "GET", "/api/study/me", tok, nil); code != 401 {
			t.Fatal("session survived logout", code)
		}
	}
	if code, _ := request(t, h, "GET", "/api/study/bankid/attempts/"+a.ID, credential, nil); code != 410 {
		t.Fatal("attempt reissued revoked session", code)
	}
}
func TestReturningLoginRequiresNewConsent(t *testing.T) {
	s, f, h := testService(t)
	old, _, token := enroll(t, s, f, h)
	if _, err := PublishConsent(s.App, s.Config, "v2", "New consent", "Updated consent."); err != nil {
		t.Fatal(err)
	}
	if code, _ := request(t, h, "POST", "/upload-test", token, nil); code != 403 {
		t.Fatal(code)
	}
	auth, credential, _ := start(t, s, h, "", "")
	advance(t, s, auth)
	if status(t, h, auth, credential)["consentRequired"] != true {
		t.Fatal("missing review")
	}
	next, nc, _ := start(t, s, h, "", credential)
	advance(t, s, next)
	if code, _ := request(t, h, "POST", "/upload-test", session(t, h, next, nc), nil); code != 204 {
		t.Fatal(code)
	}
	if next.UserID != old.UserID {
		t.Fatal("created another participant")
	}
}
func TestRejectedAndFailedAttemptsPersist(t *testing.T) {
	for _, tc := range []struct{ name, pnr, risk, expected, hint string }{
		{"wrong signer", "200001012384", "low", "199001012384", "wrongSigner"},
		{"risk", "200001012384", "high", "", "riskRejected"},
		{"invalid evidence", "bad", "low", "", "invalidEvidence"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, f, h := testService(t)
			a, c, _ := start(t, s, h, invite(t, s, "PART-001", tc.expected), "")
			completed(f, tc.pnr, tc.risk)
			advance(t, s, a)
			if a.Status != "rejected" || a.Hint != tc.hint {
				t.Fatal(a.Status, a.Hint)
			}
			if status(t, h, a, c)["grant"] != nil {
				t.Fatal("rejected grant")
			}
			r, _ := s.App.FindRecordById("signatures", a.SignatureID)
			if r == nil || r.GetString("outcome") != "rejected" {
				t.Fatal("missing evidence")
			}
		})
	}
	for _, kind := range []string{"failed", "cancelled", "unknown", "start-error"} {
		t.Run(kind, func(t *testing.T) {
			s, f, h := testService(t)
			code := invite(t, s, "PART-001", "")
			if kind == "start-error" {
				f.err = &bankid.APIError{Status: 400, Code: "alreadyInProgress", Raw: json.RawMessage(`{"errorCode":"alreadyInProgress"}`)}
			}
			a, c, _ := start(t, s, h, code, "")
			switch kind {
			case "failed":
				f.result = bankid.Result{Status: "failed", HintCode: "userCancel"}
				advance(t, s, a)
			case "cancelled":
				if status, _ := request(t, h, "POST", "/api/study/bankid/attempts/"+a.ID+"/cancel", c, nil); status != 200 {
					t.Fatal(status)
				}
			case "unknown":
				f.err = errors.New("connection lost")
				advance(t, s, a)
			}
			if a.SignatureID == "" || status(t, h, a, c)["grant"] != nil {
				t.Fatal("missing failure record or unexpected grant")
			}
		})
	}
}
func TestDuplicateOwnershipExpiryAndRestart(t *testing.T) {
	s, f, h := testService(t)
	a, c, in := start(t, s, h, invite(t, s, "PART-001", ""), "")
	for i := 0; i < 3; i++ {
		if code, _ := request(t, h, "POST", "/api/study/bankid/attempts", "", in); code != 200 {
			t.Fatal(code)
		}
	}
	if f.calls != 1 {
		t.Fatal("duplicate provider start")
	}
	if code, _ := request(t, h, "GET", "/api/study/bankid/attempts/"+a.ID, a.ID+"."+randomSecret(), nil); code != 410 {
		t.Fatal("unowned status")
	}
	completed(f, "200001012384", "low")
	advance(t, s, a)
	advance(t, s, a)
	rs, _ := s.App.FindAllRecords("signatures")
	if len(rs) != 1 {
		t.Fatal("duplicate result")
	}
	s.mu.Lock()
	s.attempts = map[string]*Attempt{}
	s.mu.Unlock()
	if code, _ := request(t, h, "POST", "/api/study/bankid/attempts", "", in); code != 410 {
		t.Fatal("completed request restarted after process reset", code)
	}
	if f.calls != 1 {
		t.Fatal("replayed completed attempt contacted provider")
	}

	if code, _ := request(t, h, "GET", "/api/study/bankid/attempts/"+a.ID, c, nil); code != 410 {
		t.Fatal("restart was not explicit")
	}
	auth, ac, _ := start(t, s, h, "", "")
	advance(t, s, auth)
	if session(t, h, auth, ac) == "" {
		t.Fatal("committed user cannot return")
	}
	now := s.now().Add(AttemptLifetime + ResultLifetime + time.Second)
	s.now = func() time.Time { return now }
	s.tick(context.Background())
	if s.lookup(auth.ID) != nil {
		t.Fatal("expired attempt retained")
	}
}
func TestPersistenceRetryDoesNotRecollect(t *testing.T) {
	s, f, h := testService(t)
	a, c, _ := start(t, s, h, invite(t, s, "PART-001", ""), "")
	completed(f, "200001012384", "low")
	s.App.OnRecordCreate("signatures").BindFunc(func(e *core.RecordEvent) error { return errors.New("disk unavailable") })
	if err := s.advance(context.Background(), a); err == nil {
		t.Fatal("expected storage failure")
	}
	if a.Status != "pending" || a.Result == nil {
		t.Fatal("reported unsaved acceptance")
	}
	if status(t, h, a, c)["grant"] != nil {
		t.Fatal("unsaved grant")
	}
	receivedAt := s.now()
	later := receivedAt.Add(time.Minute)
	s.now = func() time.Time { return later }
	s.App.OnRecordCreate("signatures").UnbindAll()
	advance(t, s, a)
	if !a.FinishedAt.Equal(receivedAt) {
		t.Fatal("persistence retry shifted authentication time")
	}
	if f.collects != 1 || a.Status != "accepted" {
		t.Fatal("recollected terminal result", f.collects, a.Status)
	}
}
func TestInvitationChangeAndConsentChangeRejectPending(t *testing.T) {
	for _, change := range []string{"reissue", "expired", "consent"} {
		t.Run(change, func(t *testing.T) {
			s, f, h := testService(t)
			a, _, _ := start(t, s, h, invite(t, s, "PART-001", ""), "")
			switch change {
			case "reissue":
				invite(t, s, "PART-001", "")
			case "expired":
				u, _ := s.App.FindRecordById("users", a.UserID)
				u.Set("invitationExpiresAt", 1)
				if err := s.App.Save(u); err != nil {
					t.Fatal(err)
				}
			case "consent":
				if _, err := PublishConsent(s.App, s.Config, "v2", "New", "New text"); err != nil {
					t.Fatal(err)
				}
			}
			completed(f, "200001012384", "low")
			advance(t, s, a)
			if a.Status != "rejected" {
				t.Fatal(a.Status)
			}
		})
	}
}
func TestIdentityConflictAndCancelCompletionRace(t *testing.T) {
	s, f, h := testService(t)
	enroll(t, s, f, h)
	a, c, _ := start(t, s, h, invite(t, s, "PART-002", ""), "")
	advance(t, s, a)
	if a.Hint != "identityConflict" {
		t.Fatal(a.Hint)
	}
	auth, ac, _ := start(t, s, h, "", "")
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		auth.mu.Lock()
		defer auth.mu.Unlock()
		_ = s.advance(context.Background(), auth)
	}()
	go func() {
		defer wg.Done()
		request(t, h, "POST", "/api/study/bankid/attempts/"+auth.ID+"/cancel", ac, nil)
	}()
	wg.Wait()
	if auth.Status != "accepted" || f.cancels != 0 {
		t.Fatal("completion lost to cancellation")
	}
	_ = c
}

func TestAttemptScopeAndChangedStart(t *testing.T) {
	s, f, h := testService(t)
	a, credential, in := start(t, s, h, invite(t, s, "PART-001", ""), "")
	in.Mode = "qr"
	if code, _ := request(t, h, "POST", "/api/study/bankid/attempts", "", in); code != 409 {
		t.Fatal("changed idempotent request", code)
	}
	if code, _ := request(t, h, "POST", "/api/study/bankid/attempts/"+a.ID+"/return", credential, map[string]any{"nonce": "wrong"}); code != 400 {
		t.Fatal("wrong callback accepted", code)
	}
	if code, _ := request(t, h, "POST", "/api/study/bankid/attempts/"+a.ID+"/return", credential, map[string]any{"nonce": a.Nonce}); code != 200 {
		t.Fatal(code)
	}
	if f.collects != 0 {
		t.Fatal("mobile poll called provider")
	}
	r := httptest.NewRequest("GET", "/api/study/bankid/attempts/"+a.ID, nil)
	r.RemoteAddr = "192.0.2.2:12345"
	r.Header.Set("Authorization", "Bearer "+credential)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 409 {
		t.Fatal("connection changed", w.Code)
	}
	if _, err := PublishConsent(s.App, s.Config, "v2", "Changed", "Changed"); err != nil {
		t.Fatal(err)
	}
	in.ClientSecret = randomSecret()
	code, res := request(t, h, "POST", "/api/study/bankid/attempts", "", in)
	if code != 409 {
		t.Fatal(code, res)
	}
	reason := res["data"].(map[string]any)["reason"].(map[string]any)["code"]
	if reason != "consentChanged" {
		t.Fatal("unusable reason", reason)
	}
}
func TestWorkerCollectsWithoutAppAndPersistsExpiry(t *testing.T) {
	s, f, h := testService(t)
	a, _, _ := start(t, s, h, invite(t, s, "PART-001", ""), "")
	completed(f, "200001012384", "low")
	now := s.now().Add(3 * time.Second)
	s.now = func() time.Time { return now }
	s.tick(context.Background())
	if a.Status != "accepted" {
		t.Fatal("worker needs app poll")
	}
	b, _, _ := start(t, s, h, invite(t, s, "PART-002", ""), "")
	f.result = bankid.Result{Status: "pending"}
	now = now.Add(AttemptLifetime + time.Second)
	s.tick(context.Background())
	if b.Status != "cancelled" || b.SignatureID == "" {
		t.Fatal("expired attempt missing outcome", b.Status)
	}
}
func TestTokensHaveAbsoluteExpiryAndEnvironmentScope(t *testing.T) {
	s, f, h := testService(t)
	a, credential, token := enroll(t, s, f, h)
	claims, err := security.ParseUnverifiedJWT(token)
	if err != nil {
		t.Fatal(err)
	}
	if int64(claims["exp"].(float64)) != a.FinishedAt.Add(SessionLifetime).Unix() {
		t.Fatal("wrong expiry")
	}
	original := a.Grant.ExpiresAt
	now := s.now().Add(5 * time.Minute)
	s.now = func() time.Time { return now }
	if session(t, h, a, credential) != token || a.Grant.ExpiresAt != original {
		t.Fatal("retry extended session")
	}
	s.Config.Environment = "production"
	if code, _ := request(t, h, "POST", "/upload-test", token, nil); code != 401 {
		t.Fatal("cross environment token accepted")
	}
	s.Config.Environment = "test"
	user, _ := s.App.FindRecordById("users", a.UserID)
	expired, err := security.NewJWT(jwt.MapClaims{core.TokenClaimType: core.TokenTypeAuth, core.TokenClaimId: a.UserID, core.TokenClaimCollectionId: "_pb_users_auth_", core.TokenClaimRefreshable: false, "study": "study", "environment": "test", "exp": time.Now().Add(-time.Second).Unix()}, a.TokenKey+user.Collection().AuthToken.Secret, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if code, _ := request(t, h, "GET", "/api/study/me", expired, nil); code != 401 {
		t.Fatal("expired token accepted")
	}
}

func TestTamperedEvidenceCannotAuthorizeWrites(t *testing.T) {
	s, f, h := testService(t)
	a, _, token := enroll(t, s, f, h)
	sig, _ := s.App.FindRecordById("signatures", a.SignatureID)
	sig.Set("evidenceCipher", "invalid")
	if err := s.App.Save(sig); err != nil {
		t.Fatal(err)
	}
	if code, _ := request(t, h, "POST", "/upload-test", token, nil); code != 403 {
		t.Fatal("tampered evidence accepted", code)
	}
}

func TestQuestionnairesAndAnswersRequireConsentAndOwnership(t *testing.T) {
	s, f, h := testService(t)
	a, _, token := enroll(t, s, f, h)
	q, _ := newRecord(s.App, "questionnaires")
	q.Set("name", "Baseline")
	q.Set("occurance", "once")
	if err := s.App.Save(q); err != nil {
		t.Fatal(err)
	}
	path := "/api/collections/answers/records"
	body := map[string]any{"user": a.UserID, "questionnaire": q.Id, "answers": map[string]any{"example": true}}
	code, result := request(t, h, "POST", path, token, body)
	if code != 200 {
		t.Fatal("consenting participant cannot answer", code, result)
	}
	answerID := result["id"].(string)
	if code, _ := request(t, h, "GET", "/api/collections/questionnaires/records", token, nil); code != 200 {
		t.Fatal(code)
	}
	f.result = bankid.Result{Status: "pending"}
	other, oc, _ := start(t, s, h, invite(t, s, "PART-002", ""), "")
	completed(f, "199001012384", "low")
	advance(t, s, other)
	otherToken := session(t, h, other, oc)
	if code, _ := request(t, h, "GET", path+"/"+answerID, otherToken, nil); code == 200 {
		t.Fatal("another participant can read answer")
	}
	if code, _ := request(t, h, "POST", path, otherToken, body); code == 200 {
		t.Fatal("another participant can write answer")
	}
	if code, _ := request(t, h, "PATCH", path+"/"+answerID, token, map[string]any{"user": other.UserID}); code == 200 {
		t.Fatal("answer ownership can change")
	}
	if code, _ := request(t, h, "POST", "/api/study/consent/withdraw", token, nil); code != 204 {
		t.Fatal(code)
	}
	for _, method := range []string{"POST", "GET"} {
		if code, _ := request(t, h, method, path, token, body); code != 403 {
			t.Fatal("withdrawn participant answer access", method, code)
		}
	}
	if code, _ := request(t, h, "GET", "/api/collections/questionnaires/records", token, nil); code != 403 {
		t.Fatal("withdrawn participant questionnaire access", code)
	}
}
