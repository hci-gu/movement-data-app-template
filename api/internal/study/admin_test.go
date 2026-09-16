package study

import (
	"github.com/pocketbase/pocketbase/core"
	"regexp"
	"testing"
)

func adminToken(t *testing.T, s *Service) string {
	t.Helper()
	c, err := s.App.FindCollectionByNameOrId(core.CollectionNameSuperusers)
	if err != nil {
		t.Fatal(err)
	}
	r := core.NewRecord(c)
	r.SetEmail("operator@example.org")
	r.SetPassword("test-password-123456")
	if err := s.App.Save(r); err != nil {
		t.Fatal(err)
	}
	token, err := r.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
	return token
}
func TestAdminPublicationAndInvitations(t *testing.T) {
	s, _, h := testService(t)
	token := adminToken(t, s)
	path := "/api/collections/consent_texts/records"
	body := map[string]any{"version": "v2", "title": "Consent", "text": "I consent. Åäö"}
	if code, _ := request(t, h, "POST", path, "", body); code != 403 {
		t.Fatal(code)
	}
	code, r := request(t, h, "POST", path, token, body)
	if code != 200 {
		t.Fatal(code, r)
	}
	if code, _ := request(t, h, "PATCH", path+"/"+r["id"].(string), token, map[string]any{"text": "changed"}); code != 403 {
		t.Fatal(code)
	}
	body = map[string]any{"username": "PART-001", "expectedPersonalNumber": "200001012384"}
	path = "/api/collections/users/records"
	code, r = request(t, h, "POST", path, token, body)
	if code != 200 {
		t.Fatal(code, r)
	}
	invitation, ok := r["invitationCode"].(string)
	if !ok || !regexp.MustCompile(`^[0-9]{3}-[0-9]{3}$`).MatchString(invitation) || r["expectedPersonalNumber"] != "" {
		t.Fatal("invalid admin response", r)
	}
	id := r["id"].(string)
	code, r = request(t, h, "POST", path, token, body)
	if code != 200 || r["id"] != id || r["invitationCode"] == invitation {
		t.Fatal("reissue must retain user and change code", code, r)
	}
	u, err := s.App.FindRecordById("users", id)
	if err != nil {
		t.Fatal(err)
	}
	if u.GetString("invitationCode") != "" || u.GetString("expectedPersonalNumber") != "" {
		t.Fatal("stored plaintext")
	}
	if code, _ := request(t, h, "PATCH", path+"/"+id, token, map[string]any{"active": true}); code != 403 {
		t.Fatal("admin bypass")
	}
}
