package study

import (
	"bytes"
	"regexp"
	"testing"

	"github.com/pocketbase/pocketbase/core"
)

func TestNumericInvitationEnrollment(t *testing.T) {
	s, _, h := testService(t)
	token := adminToken(t, s)
	status, invitation := request(t, h, "POST", "/api/collections/study_invitations/records", token, map[string]any{"participantId": "NUMERIC-001"})
	if status != 200 {
		t.Fatal(status, invitation)
	}
	code, ok := invitation["invitationCode"].(string)
	if !ok || !regexp.MustCompile(`^[0-9]{3}-[0-9]{3}$`).MatchString(code) {
		t.Fatal("invitation did not use XXX-XXX digits", invitation)
	}
	status, result := request(t, h, "POST", "/api/study/enrollments", "", map[string]any{"kind": "enroll", "invitationCode": code, "clientSecret": randomSecret()})
	if status != 200 {
		t.Fatal("numeric invitation could not enroll", status, result)
	}
	flow, err := s.App.FindRecordById("enrollment_sessions", result["id"].(string))
	if err != nil || flow.GetString("participantId") != "NUMERIC-001" || flow.GetString("invitation") != invitation["id"] {
		t.Fatal("numeric invitation enrolled the wrong participant", err)
	}
}

func TestInvitationCodeRetriesCollision(t *testing.T) {
	s, _, _ := testService(t)
	existing, err := newRecord(s.App, "study_invitations")
	if err != nil {
		t.Fatal(err)
	}
	existing.Set("study", s.Config.StudyID)
	existing.Set("tokenHash", s.Config.digest("invitation", "000-000"))
	if err := s.App.Save(existing); err != nil {
		t.Fatal(err)
	}
	err = s.App.RunInTransaction(func(tx core.App) error {
		// crypto/rand.Int reads three bytes for this range: first an occupied
		// code, then an available code that must preserve its leading zeros.
		code, err := unusedInvitationCode(tx, s.Config, bytes.NewReader([]byte{0, 0, 0, 0, 0, 1}))
		if err != nil || code != "000-001" {
			t.Fatalf("got code %q, error %v", code, err)
		}
		if code, err := unusedInvitationCode(tx, s.Config, bytes.NewReader(make([]byte, 32*3))); err == nil || code != "" {
			t.Fatal("repeated collisions did not fail within the retry limit")
		}
		if code, err := unusedInvitationCode(tx, s.Config, bytes.NewReader(nil)); err == nil || code != "" {
			t.Fatal("random source failure did not stop code generation")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
