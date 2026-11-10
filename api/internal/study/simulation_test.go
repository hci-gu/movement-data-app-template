//go:build study_simulation

package study

import (
	"app/internal/kontoapi"
	"app/internal/testidentity"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

const testActor = "198001012384"
const testOtherActor = "198201012384"

func TestSuperuserCanManageMappingsThroughPocketBase(t *testing.T) {
	s, f, h, mapping := simulationService(t, "200001010004")
	admin, err := newRecord(s.App, "_superusers")
	if err != nil {
		t.Fatal(err)
	}
	admin.Set("email", "simulation-admin@example.org")
	admin.SetPassword(randomSecret())
	if err = s.App.Save(admin); err != nil {
		t.Fatal(err)
	}
	token, err := admin.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
	base := "/api/collections/test_identity_mappings/records"
	t.Run("formatted personal numbers", func(t *testing.T) {
		w := questionnaireRequest(h, "POST", base, token, map[string]any{
			"id": "", "realPersonalNumber": " 19820101-2384 ",
			"testPersonalNumber": "20110101-1003", "role": "participant",
		})
		if w.Code != 200 {
			t.Fatalf("formatted create: %d %s", w.Code, w.Body)
		}
		var created struct {
			ID   string `json:"id"`
			Real string `json:"realPersonalNumber"`
			Test string `json:"testPersonalNumber"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
			t.Fatal(err)
		}
		if created.Real != testOtherActor || created.Test != "201101011003" {
			t.Fatal("formatted identities were not normalized")
		}
		_, _, user := authenticate(t, s, f, h, testOtherActor)
		if user.GetString("personalNumber") != created.Test {
			t.Fatal("normalized mapping was not applied to authentication")
		}
		requireCode(t, h, "PATCH", base+"/"+created.ID, token, map[string]any{"testPersonalNumber": "20120101-1003"}, 200)
		saved, err := s.App.FindRecordById("test_identity_mappings", created.ID)
		if err != nil || saved.GetString("testPersonalNumber") != "201201011003" {
			t.Fatal("update not normalized")
		}
		requireCode(t, h, "DELETE", base+"/"+created.ID, token, nil, 204)
	})
	for _, invalid := range []struct{ field, value string }{
		{"testPersonalNumber", "20110230-1003"},
		{"testPersonalNumber", "29990101-1003"},
		{"testPersonalNumber", "1101011003"},
		{"realPersonalNumber", "1982010-12384"},
		{"role", ""},
	} {
		input := map[string]any{"realPersonalNumber": testOtherActor, "testPersonalNumber": "201101011003", "role": "participant"}
		input[invalid.field] = invalid.value
		w := questionnaireRequest(h, "POST", base, token, input)
		var response struct {
			Data map[string]struct{ Message string } `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if w.Code != 400 || response.Data[invalid.field].Message == "" {
			t.Fatalf("missing field error for %s: %d %s", invalid.field, w.Code, w.Body)
		}
	}
	// PocketBase's dashboard sends an empty id and other unset fields.
	for _, role := range []any{"guardian", []string{"guardian"}} {
		w := questionnaireRequest(h, "POST", base, token, map[string]any{"id": "", "realPersonalNumber": testOtherActor, "testPersonalNumber": "197301010001", "role": role, "revision": ""})
		if w.Code != 200 {
			t.Fatalf("dashboard create (%T): %d %s", role, w.Code, w.Body)
		}
		var created struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
			t.Fatal(err)
		}
		requireCode(t, h, "DELETE", base+"/"+created.ID, token, nil, 204)
	}
	requireCode(t, h, "GET", base, token, nil, 200)
	oldRevision := mapping.GetString("revision")
	requireCode(t, h, "PATCH", base+"/"+mapping.Id, token, map[string]any{"testPersonalNumber": "201201010004", "revision": oldRevision}, 200)
	changed, _ := s.App.FindRecordById("test_identity_mappings", mapping.Id)
	if changed.GetString("revision") == oldRevision {
		t.Fatal("client preserved revision")
	}
	w := questionnaireRequest(h, "POST", base, token, map[string]any{"realPersonalNumber": testActor, "testPersonalNumber": "197001010001", "role": "guardian"})
	if w.Code != 200 {
		t.Fatalf("create: %d %s", w.Code, w.Body)
	}
	var record struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	requireCode(t, h, "POST", base, token, map[string]any{"realPersonalNumber": testActor, "testPersonalNumber": "200101010004", "role": "participant"}, 400)
	requireCode(t, h, "DELETE", base+"/"+record.ID, token, nil, 204)
}

func simulationService(t *testing.T, personal string) (*Service, *fakeProvider, http.Handler, *core.Record) {
	t.Helper()
	s, f, h := testService(t)
	run, err := testidentity.New(testidentity.File{RunID: "test-run"})
	if err != nil {
		t.Fatal(err)
	}
	s.simulation, s.Config.Simulation = run, run
	s.registerSimulationMappings()
	return s, f, h, addMapping(t, s, testActor, personal, "participant")
}

func addMapping(t *testing.T, s *Service, real, fake, role string) *core.Record {
	t.Helper()
	r, err := newRecord(s.App, "test_identity_mappings")
	if err != nil {
		t.Fatal(err)
	}
	r.Set("realPersonalNumber", real)
	r.Set("testPersonalNumber", fake)
	r.Set("role", role)
	if err = s.App.Save(r); err != nil {
		t.Fatal(err)
	}
	if r.GetString("revision") == "" {
		t.Fatal("missing server revision")
	}
	return r
}

func TestSimulationEvidenceConsentAndPrivateMapping(t *testing.T) {
	s, f, h, _ := simulationService(t, "200001010004")
	a, token, user := authenticate(t, s, f, h, testActor)
	if user.GetString("personalNumber") != "200001010004" || s.activeConsent(s.App, user) {
		t.Fatal("wrong identity or implicit consent")
	}
	requireCode(t, h, "GET", "/protected-test", token, nil, 403)
	requireCode(t, h, "POST", "/api/consent/accept", token, acceptInput(t, s), 200)
	requireCode(t, h, "GET", "/protected-test", token, nil, 204)
	r, _ := s.App.FindRecordById("authentications", a.AuthenticationID)
	var evidence AuthenticationEvidence
	if err := s.Config.Open("authentication:"+r.Id, r.GetString("evidenceCipher"), &evidence); err != nil {
		t.Fatal(err)
	}
	if evidence.Result.Raw != testActor+":Test:Person" || evidence.Result.PersonalNumber != testActor || evidence.Simulation.PersonalNumber != user.GetString("personalNumber") || evidence.SimulationRun != "test-run" {
		t.Fatal("evidence not preserved")
	}
	w := questionnaireRequest(h, "GET", "/api/me", token, nil)
	if strings.Contains(w.Body.String(), testActor) || strings.Contains(w.Body.String(), "simulationLabel") || w.Header().Get("X-Study-Simulation") != "test-run" {
		t.Fatal("unsafe session metadata")
	}
	for _, credential := range []string{"", token} {
		requireCode(t, h, "GET", "/api/collections/test_identity_mappings/records", credential, nil, 403)
		requireCode(t, h, "POST", "/api/collections/test_identity_mappings/records", credential, map[string]any{"realPersonalNumber": testActor, "testPersonalNumber": "201201010004", "role": "participant"}, 403)
	}
	_, _, again := authenticate(t, s, f, h, testActor)
	if again.Id != user.Id {
		t.Fatal("unchanged mapping lost progress")
	}
	evidence.Simulation.PersonalNumber = "201201010004"
	cipher, _ := s.Config.Seal("authentication:"+r.Id, evidence)
	r.Set("evidenceCipher", cipher)
	if err := s.App.Save(r); err != nil {
		t.Fatal(err)
	}
	requireCode(t, h, "GET", "/api/me", token, nil, 401)
}

func TestSimulationMappingEditsResetIdentityAndRevokeOldSessions(t *testing.T) {
	s, f, h, mapping := simulationService(t, "200001010004")
	a, token, user := authenticate(t, s, f, h, testActor)
	requireCode(t, h, "POST", "/api/consent/accept", token, acceptInput(t, s), 200)
	originalRevision := mapping.GetString("revision")
	mapping.Set("testPersonalNumber", "201201010004")
	if err := s.App.Save(mapping); err != nil {
		t.Fatal(err)
	}
	if originalRevision == mapping.GetString("revision") {
		t.Fatal("edit did not rotate revision")
	}
	requireCode(t, h, "GET", "/api/me", token, nil, 401)
	if s.activeConsent(s.App, user) || s.validSimulationAuthentication(s.App, a.AuthenticationID, "participant", user.GetString("personalNumber"), user.Id) {
		t.Fatal("old mapping still valid")
	}
	_, minorToken, minor := authenticate(t, s, f, h, testActor)
	if minor.Id == user.Id || !s.userDTO(minor)["guardianRequired"].(bool) || !s.userDTO(minor)["consentRequired"].(bool) {
		t.Fatal("new age reused progress")
	}
	mapping.Set("testPersonalNumber", "200001010004")
	if err := s.App.Save(mapping); err != nil {
		t.Fatal(err)
	}
	_, _, returned := authenticate(t, s, f, h, testActor)
	if returned.Id == user.Id || s.activeConsent(s.App, returned) {
		t.Fatal("returning to old number restored previous scenario")
	}
	requireCode(t, h, "GET", "/api/me", token, nil, 401)
	requireCode(t, h, "GET", "/api/me", minorToken, nil, 401)
	if err := s.App.Delete(mapping); err != nil {
		t.Fatal(err)
	}
	secret := randomSecret()
	next, err := s.Start(context.Background(), StartInput{ClientSecret: secret, Mode: "qr"}, "192.0.2.1")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.advance(context.Background(), next); err != nil {
		t.Fatal(err)
	}
	if next.Status != "rejected" || next.Hint != "testIdentityUnassigned" {
		t.Fatal("deleted mapping fell through to real identity")
	}
}

func TestSimulationOneActorAndExplicitGuardianApprovals(t *testing.T) {
	for _, count := range []int{1, 2} {
		t.Run(string(rune('0'+count)), func(t *testing.T) {
			s, f, h, _ := simulationService(t, "201201010004")
			g1 := addMapping(t, s, testActor, "197001010001", "guardian")
			addMapping(t, s, testActor, "197201010002", "guardian")
			addMapping(t, s, testActor, "201501010003", "guardian")
			_, token, user := authenticate(t, s, f, h, testActor)
			requireCode(t, h, "POST", "/api/consent/accept", token, acceptInput(t, s), 200)
			if _, err := s.createGuardianRequest(user, "201501010003", count); err == nil {
				t.Fatal("minor guardian accepted")
			}
			var first *core.Record
			for i, pn := range []string{"197001010001", "197201010002"} {
				if i == count {
					break
				}
				if _, err := s.createGuardianRequest(user, pn, count); err != nil {
					t.Fatal(err)
				}
				if _, err := s.createGuardianRequest(user, pn, count); err == nil {
					t.Fatal("duplicate guardian")
				}
				requests, _ := s.guardianRequests(s.App, user.Id)
				if i == 0 {
					first = requests[0]
				}
				g, _, err := s.startGuardian(context.Background(), requests[i], "192.0.2.2")
				if err != nil {
					t.Fatal(err)
				}
				f.result = kontoapi.Result{Status: "complete", PersonalNumber: testOtherActor}
				if err = s.advance(context.Background(), g); err != nil {
					t.Fatal(err)
				}
				if g.Status != "rejected" {
					t.Fatal("wrong actor accepted")
				}
				g, _, err = s.startGuardian(context.Background(), requests[i], "192.0.2.2")
				if err != nil {
					t.Fatal(err)
				}
				f.result = kontoapi.Result{Status: "complete", PersonalNumber: testActor, Raw: testActor + ":Test:Person"}
				if err = s.advance(context.Background(), g); err != nil {
					t.Fatal(err)
				}
				if g.Status != "accepted" || s.guardianEligible(s.App, user) {
					t.Fatal("authentication implicitly approved guardian")
				}
				consent, err := s.activeAcceptance(s.App, user)
				if err != nil {
					t.Fatal(err)
				}
				if err = s.approveGuardian(g, acceptInput(t, s), consent.Id); err != nil {
					t.Fatal(err)
				}
				if i+1 < count {
					requireCode(t, h, "GET", "/protected-test", token, nil, 403)
				}
			}
			requireCode(t, h, "GET", "/protected-test", token, nil, 204)
			// Even editing back to the same number requires a fresh explicit approval.
			if err := s.App.Save(g1); err != nil {
				t.Fatal(err)
			}
			requireCode(t, h, "GET", "/protected-test", token, nil, 403)
			g, _, err := s.startGuardian(context.Background(), first, "192.0.2.2")
			if err != nil {
				t.Fatal(err)
			}
			if err = s.advance(context.Background(), g); err != nil {
				t.Fatal(err)
			}
			consent, _ := s.activeAcceptance(s.App, user)
			if err = s.approveGuardian(g, acceptInput(t, s), consent.Id); err != nil {
				t.Fatal(err)
			}
			requireCode(t, h, "GET", "/protected-test", token, nil, 204)
		})
	}
}

func TestSimulationAgeBoundariesAndProviderFailures(t *testing.T) {
	for _, tc := range []struct {
		pn                 string
		accepted, guardian bool
	}{
		{"201611110004", false, false}, {"201611100004", true, true},
		{"201111110004", true, true}, {"201111100004", true, false},
		{"200811100004", true, false}, {"199611100004", true, false},
		{"199511110004", true, false}, {"199511100004", false, false},
	} {
		t.Run(tc.pn, func(t *testing.T) {
			s, f, _, _ := simulationService(t, tc.pn)
			s.now = func() time.Time { return time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC) }
			a, err := s.Start(context.Background(), StartInput{ClientSecret: randomSecret(), Mode: "qr"}, "192.0.2.1")
			if err != nil {
				t.Fatal(err)
			}
			f.result = kontoapi.Result{Status: "complete", PersonalNumber: testActor}
			if err = s.advance(context.Background(), a); err != nil {
				t.Fatal(err)
			}
			if (a.Status == "accepted") != tc.accepted {
				t.Fatalf("%s %s", a.Status, a.Hint)
			}
			if tc.accepted {
				u, _ := s.App.FindRecordById("users", a.UserID)
				if s.userDTO(u)["guardianRequired"] != tc.guardian {
					t.Fatal("wrong guardian threshold")
				}
			}
		})
	}
	for _, failure := range []string{"unassigned", "invalid", "failed", "expired", "cancelled"} {
		t.Run(failure, func(t *testing.T) {
			s, f, _, _ := simulationService(t, "200001010004")
			a, err := s.Start(context.Background(), StartInput{ClientSecret: randomSecret(), Mode: "qr"}, "192.0.2.1")
			if err != nil {
				t.Fatal(err)
			}
			f.result = kontoapi.Result{Status: "complete", PersonalNumber: testActor}
			switch failure {
			case "unassigned":
				f.result.PersonalNumber = testOtherActor
			case "invalid":
				f.result.PersonalNumber = "invalid"
			case "failed":
				f.result.Status = "failed"
			case "expired":
				a.ExpiresAt = s.now().Add(-time.Second)
			case "cancelled":
				if err = s.cancel(context.Background(), a); err != nil {
					t.Fatal(err)
				}
			}
			if err = s.advance(context.Background(), a); err != nil {
				t.Fatal(err)
			}
			if a.Status == "accepted" || a.UserID != "" {
				t.Fatal("failed authentication granted access")
			}
		})
	}
}

func TestSimulationMappingValidation(t *testing.T) {
	s, _, h, mapping := simulationService(t, "200001010004")
	for _, change := range []struct{ field, value string }{{"testPersonalNumber", testActor}, {"testPersonalNumber", "200102300004"}, {"role", "admin"}} {
		copy, _ := s.App.FindRecordById("test_identity_mappings", mapping.Id)
		copy.Set(change.field, change.value)
		if err := s.App.Save(copy); err == nil {
			t.Fatal("invalid mapping accepted")
		}
	}
	// Invalid rows cannot weaken closed collection API rules.
	requireCode(t, h, "GET", "/api/collections/test_identity_mappings/records/"+mapping.Id, "", nil, 403)
	normal, _, _ := testService(t)
	normal.registerSimulationMappings()
	r, _ := newRecord(normal.App, "test_identity_mappings")
	r.Set("realPersonalNumber", testActor)
	r.Set("testPersonalNumber", "200001010004")
	r.Set("role", "participant")
	if err := normal.App.Save(r); err == nil {
		t.Fatal("mapping saved in ordinary deployment")
	}
}
