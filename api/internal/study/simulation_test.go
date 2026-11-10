//go:build study_simulation

package study

import (
	"app/internal/bankid"
	"app/internal/testidentity"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

const testActor = "198001012384"
const testOtherActor = "198201012384"

func simulationFile(personal string) testidentity.File {
	return testidentity.File{RunID: "test-run", Personas: []testidentity.Persona{
		{ID: "participant", Role: "participant", PersonalNumber: personal, ActorPersonalNumber: testActor},
		{ID: "guardian-one", Role: "guardian", PersonalNumber: "197001010001", ActorPersonalNumber: testActor},
		{ID: "guardian-two", Role: "guardian", PersonalNumber: "197201010002", ActorPersonalNumber: testActor},
		{ID: "guardian-underage", Role: "guardian", PersonalNumber: "201501010003", ActorPersonalNumber: testActor},
	}}
}

func simulationService(t *testing.T, file testidentity.File) (*Service, *fakeProvider, *core.Record) {
	t.Helper()
	app := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: t.TempDir()})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := app.OnTerminate().Trigger(&core.TerminateEvent{App: app}, func(e *core.TerminateEvent) error { return e.App.ResetBootstrapState() }); err != nil {
			t.Error(err)
		}
	})
	if err := app.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	run, err := testidentity.New(file)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{Simulation: run, Environment: "test", AppID: "example.app", PublicURL: "https://study.example", ReturnURL: "researchsteps://bankid/return", ActiveKey: "one", EncryptionKeys: map[string][]byte{"one": bytes.Repeat([]byte{1}, 32)}}
	f := &fakeProvider{result: guardianResult(testActor)}
	s := &Service{App: app, Config: cfg, simulation: run, provider: f, attempts: map[string]*Attempt{}, rates: map[string]rateEntry{}, now: func() time.Time { return time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC) }}
	s.registerSimulationMappings()
	for _, p := range file.Personas {
		r, err := newRecord(app, "test_identity_mappings")
		if err != nil {
			t.Fatal(err)
		}
		r.Set("realPersonalNumber", p.ActorPersonalNumber)
		r.Set("testPersonalNumber", p.PersonalNumber)
		r.Set("role", p.Role)
		if err = app.Save(r); err != nil {
			t.Fatal(err)
		}
	}
	doc, err := PublishConsent(app, "v1", "Test consent", "I consent.")
	if err != nil {
		t.Fatal(err)
	}
	return s, f, doc
}

func simulationLogin(t *testing.T, s *Service, doc *core.Record) (*Attempt, string) {
	t.Helper()
	secret := randomSecret()
	a, err := s.Start(context.Background(), StartInput{ClientSecret: secret, Mode: "qr", Version: doc.Id, DocumentHash: doc.GetString("documentHash")}, "192.0.2.1")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.advance(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	return a, secret
}

func simulationHandler(t *testing.T, s *Service) http.Handler {
	t.Helper()
	r, err := apis.NewRouter(s.App)
	if err != nil {
		t.Fatal(err)
	}
	s.RegisterRoutes(r)
	h, err := r.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestSimulationPreservesEvidenceAndReusesPersona(t *testing.T) {
	s, f, doc := simulationService(t, simulationFile("201201010004"))
	raw, _ := json.Marshal(f.result)
	f.result.Raw = raw
	a, secret := simulationLogin(t, s, doc)
	if a.Status != "accepted" {
		t.Fatalf("%s: %s", a.Status, a.Hint)
	}
	user, err := s.App.FindRecordById("users", a.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if user.GetString("personalNumber") != "201201010004" || user.GetString("simulationRun") != "test-run" || !s.activeConsent(s.App, user) || s.guardianEligible(s.App, user) {
		t.Fatal("effective identity not used consistently")
	}
	sig, _ := s.App.FindRecordById("signatures", a.SignatureID)
	var evidence Evidence
	if err := s.Config.Open("signature:"+sig.Id, sig.GetString("evidenceCipher"), &evidence); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(evidence.Completion, raw) || evidence.Simulation == nil || evidence.Simulation.PersonalNumber != "201201010004" || evidence.SimulationRun != "test-run" {
		t.Fatal("real evidence changed or mapping lost")
	}
	if f.result.CompletionData.User.PersonalNumber != testActor {
		t.Fatal("provider result mutated")
	}
	h := simulationHandler(t, s)
	r := httptest.NewRequest("GET", "/api/bankid/attempts/"+a.ID, nil)
	r.Header.Set("Authorization", "Bearer "+a.ID+"."+secret)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 || strings.Contains(w.Body.String(), testActor) || strings.Contains(w.Body.String(), "simulationLabel") || w.Header().Get("X-Study-Simulation") != "test-run" {
		t.Fatalf("unsafe or missing session metadata: %d %s", w.Code, w.Body.String())
	}
	second, _ := simulationLogin(t, s, doc)
	if second.UserID != a.UserID {
		t.Fatal("same run lost participant state")
	}
	evidence.Simulation.PersonalNumber = "200001010004"
	cipher, _ := s.Config.Seal("signature:"+sig.Id, evidence)
	sig.Set("evidenceCipher", cipher)
	if err := s.App.Save(sig); err != nil {
		t.Fatal(err)
	}
	// Check the mapping directly: the latest valid signature should remain valid.
	if s.simulation.Check(testActor, "participant", user.GetString("personalNumber"), evidence.Simulation) {
		t.Fatal("altered mapping accepted")
	}
}

func TestSimulationOneActorCanCompleteOneOrTwoGuardians(t *testing.T) {
	for _, count := range []int{1, 2} {
		t.Run(string(rune('0'+count)), func(t *testing.T) {
			s, f, doc := simulationService(t, simulationFile("201201010004"))
			a, _ := simulationLogin(t, s, doc)
			user, _ := s.App.FindRecordById("users", a.UserID)
			if _, err := s.createGuardianRequest(user, "201501010003", count); err == nil {
				t.Fatal("underage guardian allowed")
			}
			for i, number := range []string{"197001010001", "197201010002"} {
				if i == count {
					break
				}
				if _, err := s.createGuardianRequest(user, number, count); err != nil {
					t.Fatal(err)
				}
				if _, err := s.createGuardianRequest(user, number, count); err == nil {
					t.Fatal("duplicate guardian allowed")
				}
				requests, _ := s.guardianRequests(s.App, user.Id)
				g, err := s.startGuardian(context.Background(), requests[i], "192.0.2.2", "test")
				if err != nil {
					t.Fatal(err)
				}
				if f.request.Requirement.PersonalNumber != testActor {
					t.Fatal("fictional identity sent as provider requirement")
				}
				f.result = guardianResult(testOtherActor)
				if err = s.advance(context.Background(), g); err != nil {
					t.Fatal(err)
				}
				if g.Status != "rejected" || s.guardianEligible(s.App, user) {
					t.Fatal("unassigned actor approved guardian")
				}
				g, err = s.startGuardian(context.Background(), requests[i], "192.0.2.2", "test")
				if err != nil {
					t.Fatal(err)
				}
				f.result = guardianResult(testActor)
				if err = s.advance(context.Background(), g); err != nil {
					t.Fatal(err)
				}
				if g.Status != "accepted" {
					t.Fatalf("guardian: %s %s", g.Status, g.Hint)
				}
				if i+1 < count && s.guardianEligible(s.App, user) {
					t.Fatal("partial approval unlocked participant")
				}
			}
			user, _ = s.App.FindRecordById("users", user.Id)
			if !s.guardianEligible(s.App, user) {
				t.Fatal("all guardians did not unlock")
			}
			if err := s.authorizeParticipant(&core.RequestEvent{Auth: user}); err != nil {
				t.Fatal(err)
			}
			mapping, _ := s.App.FindFirstRecordByData("test_identity_mappings", "testPersonalNumber", "197001010001")
			if err := s.App.Save(mapping); err != nil {
				t.Fatal(err)
			}
			if s.guardianEligible(s.App, user) {
				t.Fatal("edited guardian mapping retained approval")
			}
			requests, _ := s.guardianRequests(s.App, user.Id)
			g, err := s.startGuardian(context.Background(), requests[0], "192.0.2.2", "test")
			if err != nil {
				t.Fatal(err)
			}
			if err = s.advance(context.Background(), g); err != nil {
				t.Fatal(err)
			}
			if g.Status != "accepted" || !s.guardianEligible(s.App, user) {
				t.Fatal("new guardian revision did not authorize")
			}
			if _, err := PublishConsent(s.App, "v2", "Changed", "Changed consent"); err != nil {
				t.Fatal(err)
			}
			if s.guardianEligible(s.App, user) || s.activeConsent(s.App, user) {
				t.Fatal("old consent remained authorized")
			}
		})
	}
}

func TestSimulationMappingEditRevokesSessionAndResetsProgress(t *testing.T) {
	s, _, doc := simulationService(t, simulationFile("200101010004"))
	s.now = time.Now
	a, secret := simulationLogin(t, s, doc)
	h := simulationHandler(t, s)
	request := func(path, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	w := request("/api/bankid/attempts/"+a.ID, a.ID+"."+secret)
	var body struct {
		Grant grant `json:"grant"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body.Grant.Token == "" {
		t.Fatal("no session")
	}
	mapping, _ := s.App.FindFirstRecordByData("test_identity_mappings", "role", "participant")
	revision := mapping.GetString("revision")
	mapping.Set("testPersonalNumber", "201201010004")
	if err := s.App.Save(mapping); err != nil {
		t.Fatal(err)
	}
	if mapping.GetString("revision") == revision {
		t.Fatal("revision not rotated")
	}
	if request("/api/me", body.Grant.Token).Code != 401 {
		t.Fatal("old session survived mapping change")
	}
	if request("/api/bankid/attempts/"+a.ID, a.ID+"."+secret).Code != 410 {
		t.Fatal("old attempt still granted session")
	}
	b, _ := simulationLogin(t, s, doc)
	if b.Status != "accepted" || b.UserID == a.UserID {
		t.Fatal("new mapping reused participant")
	}
	mapping.Set("testPersonalNumber", "200101010004")
	if err := s.App.Save(mapping); err != nil {
		t.Fatal(err)
	}
	c, _ := simulationLogin(t, s, doc)
	if c.Status != "accepted" || c.UserID == a.UserID || c.UserID == b.UserID {
		t.Fatal("returning number reused history")
	}
	if err := s.App.Delete(mapping); err != nil {
		t.Fatal(err)
	}
	d, _ := simulationLogin(t, s, doc)
	if d.Status != "rejected" || d.Hint != "testIdentityUnassigned" {
		t.Fatal("deleted mapping passed through real identity")
	}
}

func TestSimulationDoesNotBypassAuthentication(t *testing.T) {
	for _, failure := range []string{"wrong-actor", "failed", "risk", "missing-evidence", "expired", "changed-consent"} {
		t.Run(failure, func(t *testing.T) {
			s, f, doc := simulationService(t, simulationFile("200101010004"))
			a, err := s.Start(context.Background(), StartInput{ClientSecret: randomSecret(), Mode: "qr", Version: doc.Id, DocumentHash: doc.GetString("documentHash")}, "192.0.2.1")
			if err != nil {
				t.Fatal(err)
			}
			switch failure {
			case "wrong-actor":
				f.result = guardianResult(testOtherActor)
			case "failed":
				f.result = bankid.Result{Status: "failed", HintCode: "userCancel"}
			case "risk":
				f.result.CompletionData.Risk = "high"
			case "missing-evidence":
				f.result.CompletionData.Signature = ""
			case "expired":
				a.ExpiresAt = s.now().Add(-time.Second)
			case "changed-consent":
				if _, err := PublishConsent(s.App, "v2", "Changed", "Changed"); err != nil {
					t.Fatal(err)
				}
			}
			if err = s.advance(context.Background(), a); err != nil {
				t.Fatal(err)
			}
			if a.Status == "accepted" || a.UserID != "" || a.TokenKey != "" {
				t.Fatal("failed authentication granted access")
			}
			users, _ := s.App.FindAllRecords("users")
			if len(users) != 0 {
				t.Fatal("failure created participant")
			}
		})
	}
}

func TestSimulationUsesTemplateAgeRules(t *testing.T) {
	for _, tc := range []struct {
		personal string
		guardian bool
	}{
		{"200811110004", true}, // day before 18th birthday
		{"200811100004", false}, {"200101010004", false}, {"199611100004", false},
		{"201111100004", true}, // main retains under-18 rule, not study's under-15 rule
	} {
		t.Run(tc.personal, func(t *testing.T) {
			s, _, doc := simulationService(t, simulationFile(tc.personal))
			a, _ := simulationLogin(t, s, doc)
			user, err := s.App.FindRecordById("users", a.UserID)
			if err != nil {
				t.Fatal(err)
			}
			if s.userDTO(user)["guardianRequired"] != tc.guardian {
				t.Fatal("incorrect age routing")
			}
		})
	}
}

func TestSimulationFreshRun(t *testing.T) {
	s, _, doc := simulationService(t, simulationFile("201201010004"))
	a, secret := simulationLogin(t, s, doc)
	user, _ := s.App.FindRecordById("users", a.UserID)
	if _, err := s.createGuardianRequest(user, "197001010001", 1); err != nil {
		t.Fatal(err)
	}
	requests, _ := s.guardianRequests(s.App, user.Id)
	changed := simulationFile("200101010004")
	h := simulationHandler(t, s)
	r := httptest.NewRequest("GET", "/api/bankid/attempts/"+a.ID, nil)
	r.Header.Set("Authorization", "Bearer "+a.ID+"."+secret)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var response struct {
		Grant grant `json:"grant"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || response.Grant.Token == "" {
		t.Fatal("no session")
	}
	changed.RunID = "fresh-run"
	next, _, _ := simulationService(t, changed)
	nextHandler := simulationHandler(t, next)
	r = httptest.NewRequest("GET", "/api/me", nil)
	r.Header.Set("Authorization", "Bearer "+response.Grant.Token)
	w = httptest.NewRecorder()
	nextHandler.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal("old run session survived")
	}
	r = httptest.NewRequest("GET", "/api/bankid/attempts/"+a.ID, nil)
	r.Header.Set("Authorization", "Bearer "+a.ID+"."+secret)
	w = httptest.NewRecorder()
	nextHandler.ServeHTTP(w, r)
	if w.Code != 410 {
		t.Fatal("old attempt survived")
	}
	r = httptest.NewRequest("GET", "/guardian/"+requests[0].Id, nil)
	w = httptest.NewRecorder()
	nextHandler.ServeHTTP(w, r)
	if w.Code != 404 {
		t.Fatal("old guardian link survived")
	}
}
