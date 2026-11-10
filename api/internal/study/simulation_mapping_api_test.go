//go:build study_simulation

package study

import (
	"app/internal/testidentity"
	"encoding/json"
	"net/http"
	"testing"
)

func TestSuperuserCanManageMappingsThroughPocketBase(t *testing.T) {
	s, _, _ := simulationService(t, testidentity.File{RunID: "test-run", Personas: simulationFile("200001010004").Personas[:1]})
	h := simulationHandler(t, s)
	mapping, _ := s.App.FindFirstRecordByFilter("test_identity_mappings", "role = 'participant'")
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

func requireCode(t *testing.T, h http.Handler, method, path, token string, body any, code int) {
	t.Helper()
	w := questionnaireRequest(h, method, path, token, body)
	if w.Code != code {
		t.Fatalf("%s %s: got %d, want %d: %s", method, path, w.Code, code, w.Body)
	}
}
