package migrations

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func oldSeal(t *testing.T, key []byte, namespace, purpose string, value any) string {
	t.Helper()
	block, _ := aes.NewCipher(key)
	aead, _ := cipher.NewGCM(block)
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	plain, _ := json.Marshal(value)
	return "old:" + base64.RawStdEncoding.EncodeToString(aead.Seal(nonce, nonce, plain, []byte(namespace+":"+purpose)))
}

func TestExistingRecordsMigrateToSingleStudy(t *testing.T) {
	key := bytes.Repeat([]byte{1}, 32)
	t.Setenv("STUDY_ACTIVE_ENCRYPTION_KEY", "old")
	t.Setenv("STUDY_ENCRYPTION_KEYS", `{"old":"`+base64.StdEncoding.EncodeToString(key)+`"}`)
	app := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: t.TempDir()})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	defer app.ResetBootstrapState()
	if err := app.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	users, _ := app.FindCollectionByNameOrId("users")
	users.Fields.Add(&core.TextField{Name: "study"}, &core.BoolField{Name: "active"}, &core.TextField{Name: "identityCipher", Max: 3000}, &core.TextField{Name: "username"})
	if err := app.Save(users); err != nil {
		t.Fatal(err)
	}
	u := core.NewRecord(users)
	u.Set("personalNumber", "200001012384")
	u.Set("study", "legacy")
	u.Set("username", "OLD-001")
	u.Set("active", true)
	u.SetPassword("internal-password")
	if err := app.Save(u); err != nil {
		t.Fatal(err)
	}
	oldDir := filepath.Join(app.DataDir(), "raw", "OLD-001")
	if err := os.MkdirAll(oldDir, 0o700); err != nil {
		t.Fatal(err)
	}
	oldFile := filepath.Join(oldDir, "upload.gz")
	if err := os.WriteFile(oldFile, []byte("saved"), 0o600); err != nil {
		t.Fatal(err)
	}
	uploads, _ := app.FindCollectionByNameOrId("dataUploads")
	upload := core.NewRecord(uploads)
	upload.Set("user", u.Id)
	upload.Set("filePath", oldFile)
	if err := app.Save(upload); err != nil {
		t.Fatal(err)
	}
	u.Set("identityCipher", oldSeal(t, key, "legacy", "identity:"+u.Id, map[string]any{"personalNumber": "200001012384"}))
	if err := app.Save(u); err != nil {
		t.Fatal(err)
	}
	sigs, _ := app.FindCollectionByNameOrId("signatures")
	sigs.Fields.Add(&core.TextField{Name: "study"}, &core.TextField{Name: "environment"})
	if err := app.Save(sigs); err != nil {
		t.Fatal(err)
	}
	sig := core.NewRecord(sigs)
	sig.Set("study", "legacy")
	sig.Set("user", u.Id)
	sig.Set("attemptId", "attempt")
	if err := app.Save(sig); err != nil {
		t.Fatal(err)
	}
	sig.Set("evidenceCipher", oldSeal(t, key, "legacy", "signature:"+sig.Id, map[string]any{"sample": true}))
	if err := app.Save(sig); err != nil {
		t.Fatal(err)
	}
	consent, _ := app.FindCollectionByNameOrId("consent_texts")
	consent.Fields.Add(&core.TextField{Name: "study"})
	if err := app.Save(consent); err != nil {
		t.Fatal(err)
	}
	if err := migrateSingleStudy(app); err != nil {
		t.Fatal(err)
	}
	users, _ = app.FindCollectionByNameOrId("users")
	if users.Fields.GetByName("study") != nil || users.Fields.GetByName("identityCipher") != nil {
		t.Fatal("legacy user fields remain")
	}
	newFile := filepath.Join(app.DataDir(), "raw", u.Id, "upload.gz")
	if _, err := os.Stat(newFile); err != nil {
		t.Fatalf("upload was not moved: %v", err)
	}
	upload, err := app.FindRecordById("dataUploads", upload.Id)
	if err != nil || upload.GetString("filePath") != newFile {
		t.Fatalf("upload path was not updated: %v", err)
	}
	sigs, _ = app.FindCollectionByNameOrId("signatures")
	if sigs.Fields.GetByName("user").Type() != core.FieldTypeRelation {
		t.Fatal("signature user is not a relation")
	}
	migrated, err := app.FindRecordById("signatures", sig.Id)
	if err != nil || migrated.GetString("user") != u.Id {
		t.Fatalf("signature relation: %v", err)
	}
	_, encoded, ok := bytes.Cut([]byte(migrated.GetString("evidenceCipher")), []byte(":"))
	if !ok {
		t.Fatal("missing migrated evidence")
	}
	raw, err := base64.RawStdEncoding.DecodeString(string(encoded))
	if err != nil {
		t.Fatal(err)
	}
	block, _ := aes.NewCipher(key)
	aead, _ := cipher.NewGCM(block)
	plain, err := aead.Open(nil, raw[:aead.NonceSize()], raw[aead.NonceSize():], []byte("signature:"+sig.Id))
	if err != nil || string(plain) != `{"sample":true}` {
		t.Fatalf("evidence was not re-encrypted: %v", err)
	}
}
