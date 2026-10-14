package study

import (
	"app/internal/bankid"
	"app/internal/questionnaires"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/security"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"
)

func questionnaireApp(t *testing.T) (*pocketbase.PocketBase, *Service, http.Handler) {
	t.Helper()
	app := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: t.TempDir()})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { app.ResetBootstrapState() })
	if err := app.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../questionnaires/testdata/records.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Collection string         `json:"collection"`
		Data       map[string]any `json:"data"`
	}
	if err := json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, item := range fixtures {
		c, err := app.FindCollectionByNameOrId(item.Collection)
		if err != nil {
			t.Fatal(err)
		}
		r := core.NewRecord(c)
		r.Load(item.Data)
		if err := app.Save(r); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := PublishConsent(app, "v1", "Consent", "I consent."); err != nil {
		t.Fatal(err)
	}
	s := &Service{App: app, Config: questionnaireConfig(), now: time.Now, rates: map[string]rateEntry{}}
	router, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	s.RegisterRoutes(router)
	handler, err := router.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	return app, s, handler
}
func questionnaireConfig() Config {
	return Config{ActiveKey: "test", EncryptionKeys: map[string][]byte{"test": bytes.Repeat([]byte{1}, 32)}}
}

func questionnaireUser(t *testing.T, app core.App, personal string, consent bool) (*core.Record, string) {
	t.Helper()
	c, _ := app.FindCollectionByNameOrId("users")
	u := core.NewRecord(c)
	u.Set("personalNumber", personal)
	u.SetPassword("test-password-123456")
	if err := app.Save(u); err != nil {
		t.Fatal(err)
	}
	if consent {
		doc, err := currentDocument(app)
		if err != nil {
			t.Fatal(err)
		}
		c, _ := app.FindCollectionByNameOrId("signatures")
		sig := core.NewRecord(c)
		sig.Id = u.Id
		sig.Set("user", u.Id)
		sig.Set("version", doc.Id)
		sig.Set("purpose", "sign")
		sig.Set("outcome", "accepted")
		sig.Set("attemptId", u.Id)
		sig.Set("receivedAt", time.Now().Unix())
		document := documentDTO(doc)
		completion, _ := json.Marshal(bankid.Result{Status: "complete", CompletionData: &bankid.Completion{User: bankid.User{PersonalNumber: personal}, Risk: "low", Signature: "c2ln", OCSPResponse: "b2NzcA=="}})
		cipher, err := questionnaireConfig().Seal("signature:"+sig.Id, Evidence{Document: &document, Request: bankid.Request{UserVisibleData: base64.StdEncoding.EncodeToString([]byte(document.Text))}, Completion: completion})
		if err != nil {
			t.Fatal(err)
		}
		sig.Set("evidenceCipher", cipher)
		if err := app.Save(sig); err != nil {
			t.Fatal(err)
		}
	}
	token, err := security.NewJWT(jwt.MapClaims{core.TokenClaimType: core.TokenTypeAuth, core.TokenClaimId: u.Id, core.TokenClaimCollectionId: u.Collection().Id, core.TokenClaimRefreshable: false}, u.TokenKey()+u.Collection().AuthToken.Secret, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return u, token
}
func questionnaireRequest(handler http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	data, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, bytes.NewReader(data))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}
func TestQuestionnaireBranchPathsAndIdempotentHTTPSubmission(t *testing.T) {
	app, _, handler := questionnaireApp(t)
	q := func(n int) string { return fmt.Sprintf("testqst%08d", n) }
	for _, gender := range []string{"female", "male"} {
		for i, branch := range []string{"branch_a", "branch_b", "branch_c"} {
			digit := 2
			if gender == "male" {
				digit = 3
			}
			u, token := questionnaireUser(t, app, fmt.Sprintf("19900101%02d%d0", i, digit), true)
			d, err := questionnaires.Load(app, "testfrm00000001")
			if err != nil {
				t.Fatal(err)
			}
			answers := map[string]any{q(2): branch, q(7): float64(0), q(8): "value_1"}
			if gender == "female" {
				answers[q(1)] = "2003-06"
			}
			if branch == "branch_a" {
				answers[q(3)] = "2024-03-10"
				answers[q(5)] = float64(2)
			} else {
				answers[q(4)] = "2024-03-10"
				answers[q(6)] = float64(2)
			}
			if branch == "branch_b" {
				answers[q(9)] = float64(7)
				answers[q(10)] = float64(23)
			}
			if branch == "branch_c" {
				answers[q(11)] = float64(5)
				answers[q(12)] = float64(8)
			}
			visible := d.Visible(answers, gender)
			count := 0
			for _, v := range visible {
				if v {
					count++
				}
			}
			if count != len(answers) {
				t.Fatal(gender, branch, count, len(answers))
			}
			body := map[string]any{"user": u.Id, "questionnaire": "testfrm00000001", "answers": answers, "started": "2026-01-01T00:00:00Z", "submissionKey": "client-forgery"}
			var wg sync.WaitGroup
			results := make(chan *httptest.ResponseRecorder, 2)
			for range 2 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					results <- questionnaireRequest(handler, "POST", "/api/collections/answers/records", token, body)
				}()
			}
			wg.Wait()
			close(results)
			ids := map[string]bool{}
			for w := range results {
				if w.Code != 200 {
					t.Fatalf("%s %s: %d %s", gender, branch, w.Code, w.Body.String())
				}
				var response map[string]any
				json.Unmarshal(w.Body.Bytes(), &response)
				ids[response["id"].(string)] = true
				if _, ok := response["submissionKey"]; ok {
					t.Fatal("private key exposed")
				}
			}
			if len(ids) != 1 {
				t.Fatal("duplicate submissions")
			}
			saved, err := app.FindFirstRecordByData("answers", "user", u.Id)
			if err != nil {
				t.Fatal(err)
			}
			var got map[string]any
			saved.UnmarshalJSONField("answers", &got)
			if len(got) != len(answers) || got[q(7)] != float64(0) || saved.GetString("submissionKey") != u.Id+":testfrm00000001:once" {
				t.Fatal("incorrect stored answers")
			}
			for _, method := range []string{"PATCH", "DELETE"} {
				w := questionnaireRequest(handler, method, "/api/collections/answers/records/"+saved.Id, token, body)
				if w.Code != 403 {
					t.Fatal(method, w.Code, w.Body.String())
				}
			}
		}
	}
}
func TestQuestionnaireRejectsInvalidAndUnauthorizedWrites(t *testing.T) {
	app, s, h := questionnaireApp(t)
	u, token := questionnaireUser(t, app, "199001010030", true)
	cases := []map[string]any{
		{"not_a_question": "x"}, {"testqst00000001": "2004-01"},
		{"testqst00000002": "branch_a", "testqst00000011": float64(4)},
		{"testqst00000002": "bad-option"}, {"testqst00000007": "2"},
		{"testqst00000007": 1.5}, {"testqst00000007": -1}, {"testqst00000007": []any{1}},
		{"testqst00000002": "branch_c", "testqst00000011": 8},
		{"testqst00000002": "branch_a", "testqst00000003": "2026-02-30"},
	}
	for _, answers := range cases {
		w := questionnaireRequest(h, "POST", "/api/collections/answers/records", token, map[string]any{"user": u.Id, "questionnaire": "testfrm00000001", "answers": answers})
		if w.Code != 400 || !bytes.Contains(w.Body.Bytes(), []byte("invalidAnswer")) {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	for _, identity := range []struct {
		personal string
		consent  bool
		reason   string
	}{{"199001010040", false, "consentRequired"}, {"201501010040", true, "guardianRequired"}} {
		other, otherToken := questionnaireUser(t, app, identity.personal, identity.consent)
		w := questionnaireRequest(h, "POST", "/api/collections/answers/records", otherToken, map[string]any{"user": other.Id, "questionnaire": "testfrm00000001", "answers": map[string]any{}})
		if w.Code != 403 || !bytes.Contains(w.Body.Bytes(), []byte(identity.reason)) {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	other, _ := questionnaireUser(t, app, "199001010050", true)
	w := questionnaireRequest(h, "POST", "/api/collections/answers/records", token, map[string]any{"user": other.Id, "questionnaire": "testfrm00000001", "answers": map[string]any{}})
	if w.Code < 400 {
		t.Fatal("foreign user accepted")
	}
	if s.userDTO(u)["gender"] != "male" {
		t.Fatal("gender absent from participant")
	}
	w = questionnaireRequest(h, "POST", "/api/collections/answers/records", token, map[string]any{"user": u.Id, "questionnaire": "testfrm00000001", "answers": map[string]any{}})
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	form, _ := app.FindRecordById("questionnaires", "testfrm00000001")
	form.Set("enabled", false)
	app.Save(form)
	w = questionnaireRequest(h, "POST", "/api/collections/answers/records", token, map[string]any{"user": u.Id, "questionnaire": form.Id, "answers": map[string]any{}})
	if w.Code != 400 {
		t.Fatal("disabled form accepted")
	}
}
func TestQuestionnaireDefinitionValidation(t *testing.T) {
	app, _, _ := questionnaireApp(t)
	question, _ := app.FindRecordById("questions", "testqst00000007")
	for _, settings := range []any{map[string]any{"required": true}, map[string]any{"min": 8, "max": 2}, map[string]any{"min": "0"}} {
		question.Set("settings", settings)
		if err := app.Save(question); err != nil {
			t.Fatal(err)
		}
		if _, err := questionnaires.Load(app, "testfrm00000001"); err == nil {
			t.Fatal("invalid settings accepted")
		}
	}
	question.Set("settings", map[string]any{})
	question.Set("dependency", question.Id)
	question.Set("dependencyValue", 1)
	app.Save(question)
	if _, err := questionnaires.Load(app, "testfrm00000001"); err == nil {
		t.Fatal("cycle accepted")
	}
	if questionnaires.Gender("") != "unknown" {
		t.Fatal("unknown gender")
	}
}
