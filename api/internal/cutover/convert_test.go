package cutover

import (
	"app/internal/study"
	"bytes"
	_ "embed"
	"encoding/json"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"os"
	"path/filepath"
	"testing"
	"time"
)

//go:embed testdata/source-schema.json
var sourceSchema []byte

func sourceApp(t *testing.T) (core.App, study.Config) {
	t.Helper()
	app := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: t.TempDir()})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { app.ResetBootstrapState() })
	var defs []struct {
		ID, Name, Type string
		Fields         core.FieldsList
		Indexes        []string
	}
	if err := json.Unmarshal(sourceSchema, &defs); err != nil {
		t.Fatal(err)
	}
	for _, d := range defs {
		c := core.NewBaseCollection(d.Name, d.ID)
		if d.Type == "auth" {
			c = core.NewAuthCollection(d.Name, d.ID)
		}
		if existing, err := app.FindCollectionByNameOrId(d.Name); err == nil {
			c = existing
		}
		for _, f := range d.Fields {
			if f.Type() != core.FieldTypeRelation {
				c.Fields.Add(f)
			}
		}
		c.Indexes = d.Indexes
		if c.IsAuth() {
			c.Fields.GetByName("email").(*core.EmailField).Required = false
		}
		if err := app.Save(c); err != nil {
			t.Fatal(err)
		}
	}
	for _, d := range defs {
		c, _ := app.FindCollectionByNameOrId(d.ID)
		for _, f := range d.Fields {
			if f.Type() == core.FieldTypeRelation {
				c.Fields.Add(f)
			}
		}
		if err := app.Save(c); err != nil {
			t.Fatal(err)
		}
	}
	cfg := study.Config{StudyID: "test", Environment: "test", ActiveKey: "key", EncryptionKeys: map[string][]byte{"key": bytes.Repeat([]byte{1}, 32)}, IdentityKey: bytes.Repeat([]byte{2}, 32)}
	return app, cfg
}
func record(t *testing.T, app core.App, name string, fields map[string]any) *core.Record {
	t.Helper()
	c, err := app.FindCollectionByNameOrId(name)
	if err != nil {
		t.Fatal(err)
	}
	r := core.NewRecord(c)
	for k, v := range fields {
		r.Set(k, v)
	}
	if c.IsAuth() {
		r.SetPassword("test-password-123456")
	}
	if err := app.Save(r); err != nil {
		t.Fatal(name, err)
	}
	return r
}
func seal(t *testing.T, cfg study.Config, context string, value any) string {
	t.Helper()
	cipher, err := cfg.Seal(context, value)
	if err != nil {
		t.Fatal(err)
	}
	return cipher
}
func TestConversionPreservesDataRelationsAndEvidence(t *testing.T) {
	app, cfg := sourceApp(t)
	u := record(t, app, "users", map[string]any{"username": "PART-001", "consentStatus": "withdrawn"})
	tokenKey := u.TokenKey()
	identity := record(t, app, "participant_identities", map[string]any{"study": "test", "environment": "test", "user": u.Id, "identityHash": "identity-digest", "identityCipher": seal(t, cfg, "identity:"+u.Id, map[string]any{"personalNumber": "200001012384"})})
	_ = identity
	d := record(t, app, "consent_versions", map[string]any{"study": "test", "version": "v1", "title": "Consent", "text": "Consent text", "documentHash": "hash"})
	record(t, app, "study_settings", map[string]any{"study": "test", "currentVersion": d.Id})
	order := record(t, app, "bankid_orders", map[string]any{"study": "test", "environment": "test", "purpose": "sign", "status": "accepted", "orderHash": "order-digest", "version": d.Id})
	evidence := study.Evidence{Document: &study.Document{ID: d.Id, Text: "Consent text"}, Completion: json.RawMessage(`{"status":"complete","unknownField":"retained"}`)}
	sig := record(t, app, "consent_signatures", map[string]any{"study": "test", "environment": "test", "order": order.Id, "orderHash": "order-digest", "user": u.Id, "version": d.Id, "outcome": "accepted", "evidenceCipher": seal(t, cfg, "signature:"+order.Id, evidence)})
	u.Set("consentSignature", sig.Id)
	u.Set("consentVersion", d.Id)
	if err := app.Save(u); err != nil {
		t.Fatal(err)
	}
	record(t, app, "consent_events", map[string]any{"study": "test", "user": u.Id, "signature": sig.Id, "kind": "withdrawn", "occurredAt": 1234})
	invitation := record(t, app, "study_invitations", map[string]any{"study": "test", "participantId": "PART-002", "tokenHash": "invitation-digest", "expiresAt": time.Now().Add(time.Hour).Unix()})
	invitation.Set("tokenCipher", seal(t, cfg, "invitation-code:"+invitation.Id, "001-234"))
	invitation.Set("expectedCipher", seal(t, cfg, "invitation:"+invitation.Id, map[string]any{"personalNumber": "199001012384"}))
	if err := app.Save(invitation); err != nil {
		t.Fatal(err)
	}
	record(t, app, "bankid_orders", map[string]any{"study": "test", "environment": "test", "purpose": "auth", "status": "failed", "requestKey": "failed", "hintCode": "userCancel", "orderHash": "failed-digest"})
	record(t, app, "bankid_orders", map[string]any{"study": "test", "environment": "test", "purpose": "sign", "status": "collecting", "requestKey": "unknown", "orderHash": "unknown-digest"})
	info := record(t, app, "info", map[string]any{"user": u.Id, "data": map[string]any{"example": "value"}})
	option := record(t, app, "questionOptions", map[string]any{"name": "Yes", "value": true})
	question := record(t, app, "questions", map[string]any{"text": "Example?", "options": []string{option.Id}})
	questionnaire := record(t, app, "questionnaires", map[string]any{"name": "Baseline", "occurance": "once", "questions": []string{question.Id}})
	answer := record(t, app, "answers", map[string]any{"user": u.Id, "questionnaire": questionnaire.Id, "answers": map[string]any{question.Id: true}})
	upload := record(t, app, "dataUploads", map[string]any{"user": u.Id, "filePath": "/pb/pb_data/raw/PART-001/old.gz"})
	folder := filepath.Join(app.DataDir(), "raw", "PART-001")
	if err := os.MkdirAll(folder, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "old.gz"), []byte("exact-file-bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Convert(app, cfg); err != nil {
		t.Fatal(err)
	}
	cs, _ := app.FindAllCollections()
	count := 0
	for _, c := range cs {
		if !c.System {
			count++
		}
	}
	if count != 8 {
		t.Fatal("not eight collections", count)
	}
	user, err := app.FindRecordById("users", u.Id)
	if err != nil || !user.GetBool("active") || user.GetString("consentStatus") != "withdrawn" || user.TokenKey() == tokenKey {
		t.Fatal("participant state or token invalidation", err)
	}
	r, err := app.FindRecordById("signatures", sig.Id)
	if err != nil || r.GetInt("withdrawnAt") != 1234 {
		t.Fatal("withdrawal history", err)
	}
	var restored study.Evidence
	if err := cfg.Open("signature:"+sig.Id, r.GetString("evidenceCipher"), &restored); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(restored.Completion, evidence.Completion) {
		t.Fatal("provider response changed")
	}
	invite, err := app.FindFirstRecordByData("users", "username", "PART-002")
	if err != nil {
		t.Fatal(err)
	}
	var code string
	if err := cfg.Open("invitation-code:"+invite.Id, invite.GetString("invitationCipher"), &code); err != nil || code != "001-234" {
		t.Fatal("invitation not reencrypted", err)
	}
	var metadata []map[string]any
	if err := user.UnmarshalJSONField("metadata", &metadata); err != nil || len(metadata) != 1 {
		t.Fatal("metadata missing", err)
	}
	if metadata[0]["record"].(map[string]any)["id"] != info.Id {
		t.Fatal("metadata history changed")
	}
	answerAfter, err := app.FindRecordById("answers", answer.Id)
	if err != nil || answerAfter.GetString("questionnaire") != questionnaire.Id || answerAfter.GetString("user") != u.Id {
		t.Fatal("answer relation changed", err)
	}
	questionAfter, _ := app.FindRecordById("questions", question.Id)
	if questionAfter.GetStringSlice("options")[0] != option.Id {
		t.Fatal("question options relation changed")
	}
	uploadAfter, _ := app.FindRecordById("dataUploads", upload.Id)
	raw, err := os.ReadFile(uploadAfter.GetString("filePath"))
	if err != nil || string(raw) != "exact-file-bytes" {
		t.Fatal("upload not preserved", err)
	}
	attempts, _ := app.FindAllRecords("signatures")
	if len(attempts) != 3 {
		t.Fatal("missing failed/interrupted attempt", len(attempts))
	}
}
func TestConflictingInvitationsRollBackDatabase(t *testing.T) {
	app, cfg := sourceApp(t)
	for i := 0; i < 2; i++ {
		record(t, app, "study_invitations", map[string]any{"study": "test", "participantId": "PART-001", "tokenHash": string(rune('a' + i)), "expiresAt": time.Now().Add(time.Hour).Unix()})
	}
	if err := Convert(app, cfg); err == nil {
		t.Fatal("silently chose invitation")
	}
	if _, err := app.FindCollectionByNameOrId("signatures"); err == nil {
		t.Fatal("partial conversion committed")
	}
	if _, err := app.FindCollectionByNameOrId("study_invitations"); err != nil {
		t.Fatal("source records removed", err)
	}
}
