package study

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
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

func TestDashboardPublishConsent(t *testing.T) {
	s, _, h := testService(t)
	token := adminToken(t, s)
	path := "/api/collections/consent_versions/records"
	body := map[string]any{"version": "v2", "title": "Test consent", "text": "I consent.\nÅäö", "study": "forged-study", "documentHash": "forged"}
	if status, _ := request(t, h, "POST", path, "", body); status != http.StatusForbidden {
		t.Fatal("anonymous publication accepted", status)
	}
	status, result := request(t, h, "POST", path, token, body)
	if status != http.StatusOK {
		t.Fatal(status, result)
	}
	doc, err := currentDocument(s.App, s.Config.StudyID)
	if err != nil || doc.Id != result["id"] || doc.GetString("documentHash") != hash(body["text"].(string)) {
		t.Fatal("publication did not set current document and exact hash", err, result)
	}
	if status, _ := request(t, h, "POST", path, token, body); status != 400 {
		t.Fatal("duplicate version accepted", status)
	}
	body["version"], body["text"] = "v3", "Unsupported 😀"
	if status, _ := request(t, h, "POST", path, token, body); status != 400 {
		t.Fatal("invalid consent accepted", status)
	}
	current, _ := currentDocument(s.App, s.Config.StudyID)
	if current.Id != doc.Id {
		t.Fatal("failed publication changed current consent")
	}
	for _, method := range []string{"PATCH", "DELETE"} {
		if status, _ := request(t, h, method, path+"/"+doc.Id, token, map[string]any{"text": "replacement"}); status != 403 {
			t.Fatal("published consent can be changed", method, status)
		}
	}
	settings, _ := s.App.FindFirstRecordByData("study_settings", "study", s.Config.StudyID)
	if status, _ := request(t, h, "PATCH", "/api/collections/study_settings/records/"+settings.Id, token, map[string]any{"currentVersion": "forged"}); status != 403 {
		t.Fatal("publication pointer can be changed directly", status)
	}
}

func TestDashboardInvitationEnrollment(t *testing.T) {
	s, fake, h := testService(t)
	admin := adminToken(t, s)
	path := "/api/collections/study_invitations/records"
	pnr := "200001012384"
	body := map[string]any{"participantId": "TEST-001", "expectedPersonalNumber": pnr, "study": "forged", "tokenHash": "forged", "consumed": true, "claimedFlow": "forged", "expiresAt": 1}
	if status, _ := request(t, h, "POST", path, "", body); status != 403 {
		t.Fatal("anonymous invitation accepted", status)
	}
	status, result := request(t, h, "POST", path, admin, body)
	if status != 200 {
		t.Fatal(status, result)
	}
	code, ok := result["invitationCode"].(string)
	if !ok || !regexp.MustCompile(`^[0-9]{3}-[0-9]{3}$`).MatchString(code) || result["expectedPersonalNumber"] != "" {
		t.Fatal("missing delivery code or exposed personal number", result)
	}
	id := result["id"].(string)
	r, err := s.App.FindRecordById("study_invitations", id)
	if err != nil {
		t.Fatal(err)
	}
	if r.GetString("study") != s.Config.StudyID || r.GetBool("consumed") || r.GetString("claimedFlow") != "" || r.GetInt("expiresAt") != int(s.now().Unix()+168*3600) {
		t.Fatal("server-owned invitation fields not generated correctly")
	}
	raw, _ := json.Marshal(r)
	if strings.Contains(string(raw), pnr) || strings.Contains(string(raw), code) || r.GetString("tokenCipher") == "" || r.GetString("tokenHash") != s.Config.digest("invitation", code) {
		t.Fatal("invitation secrets stored in plaintext or digest incorrect")
	}
	var expected identity
	if err := s.Config.open("invitation:"+id, r.GetString("expectedCipher"), &expected); err != nil || expected.PersonalNumber != pnr {
		t.Fatal("expected identity was not encrypted correctly", err)
	}
	if status, res := request(t, h, "GET", path+"/"+id, admin, nil); status != 200 || res["invitationCode"] != code {
		t.Fatal("dashboard cannot reopen delivery code", status, res)
	}
	if status, _ := request(t, h, "GET", path+"/"+id, "", nil); status != 403 {
		t.Fatal("anonymous invitation read allowed", status)
	}
	if status, _ := request(t, h, "PATCH", path+"/"+id, admin, map[string]any{"participantId": "CHANGED"}); status != 403 {
		t.Fatal("issued invitation editable", status)
	}
	secret := randomSecret()
	status, flow := request(t, h, "POST", "/api/study/enrollments", "", map[string]any{"kind": "enroll", "invitationCode": code, "clientSecret": secret})
	if status != 200 {
		t.Fatal("dashboard invitation cannot enroll", status, flow)
	}
	flowToken := flow["id"].(string) + "." + secret
	order := start(t, s, h, flowToken)
	completed(fake, pnr, "low")
	if err := s.advance(context.Background(), order.Id); err != nil {
		t.Fatal(err)
	}
	if status, res := request(t, h, "POST", "/api/study/enrollments/"+flow["id"].(string)+"/complete", flowToken, nil); status != 200 {
		t.Fatal("dashboard enrollment cannot complete", status, res)
	}
	if status, res := request(t, h, "GET", path+"/"+id, admin, nil); status != 200 || res["invitationCode"] != "" {
		t.Fatal("consumed delivery code remains exposed", status, res)
	}
}

func TestDashboardExpiredInvitationAndPurge(t *testing.T) {
	s, _, h := testService(t)
	token := adminToken(t, s)
	path := "/api/collections/study_invitations/records"
	status, res := request(t, h, "POST", path, token, map[string]any{"participantId": "TEST-001", "validityHours": 1})
	if status != 200 {
		t.Fatal(status, res)
	}
	id := res["id"].(string)
	now := s.now().Add(time.Hour)
	s.now = func() time.Time { return now }
	if status, res := request(t, h, "GET", path+"/"+id, token, nil); status != 200 || res["invitationCode"] != "" {
		t.Fatal("expired invitation code remains exposed", status, res)
	}
	if _, err := PurgeSessions(s.App, s.Config, now.Add(25*time.Hour)); err != nil {
		t.Fatal(err)
	}
	record, err := s.App.FindRecordById("study_invitations", id)
	if err != nil || record.GetString("tokenCipher") != "" || record.GetString("tokenHash") != "expired:"+id {
		t.Fatal("expired invitation credential was not purged", err)
	}
}

func TestDashboardInvitationRequiresServerSecrets(t *testing.T) {
	s, _, h := testService(t)
	token := adminToken(t, s)
	s.Config.EncryptionKeys = nil
	if status, res := request(t, h, "POST", "/api/collections/study_invitations/records", token, map[string]any{"participantId": "TEST-001"}); status != 503 {
		t.Fatal("invitation issued without server secrets", status, res)
	}
}

func TestDashboardInvitationValidation(t *testing.T) {
	s, _, h := testService(t)
	token := adminToken(t, s)
	for _, body := range []map[string]any{
		{"participantId": "bad"},
		{"participantId": "TEST-001", "validityHours": -1},
		{"participantId": "TEST-001", "validityHours": 2161},
		{"participantId": "TEST-001", "validityHours": 1.5},
		{"participantId": "TEST-001", "expectedPersonalNumber": "invalid"},
	} {
		if status, res := request(t, h, "POST", "/api/collections/study_invitations/records", token, body); status != 400 {
			t.Fatal("invalid invitation accepted", status, res)
		}
	}
	records, err := s.App.FindAllRecords("study_invitations")
	if err != nil || len(records) != 0 {
		t.Fatal("failed invitation left records behind", err)
	}
}
