package migrations

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// Convert the previous invitation-based schema in place. Any missing key or
// damaged evidence aborts the migration before its fields are removed.
func init() {
	m.Register(migrateSingleStudy, nil)
}

func migrateSingleStudy(app core.App) error {
	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}
	if users.Fields.GetByName("identityCipher") == nil {
		return nil
	}
	cfg, err := loadKeyring()
	if err != nil {
		return err
	}
	return app.RunInTransaction(func(tx core.App) error {
		users, err := tx.FindCollectionByNameOrId("users")
		if err != nil {
			return err
		}
		users.Fields.Add(&core.TextField{Name: "personalNumber", Min: 12, Max: 12})
		if err := tx.Save(users); err != nil {
			return err
		}
		allUsers, err := tx.FindAllRecords("users")
		if err != nil {
			return err
		}
		for _, user := range allUsers {
			if user.GetString("identityCipher") == "" {
				if user.GetBool("active") {
					return fmt.Errorf("active user %s has no BankID identity", user.Id)
				}
				if err := tx.Delete(user); err != nil {
					return err
				}
				continue
			}
			var identity struct {
				PersonalNumber string `json:"personalNumber"`
			}
			if err := openLegacy(cfg, user.GetString("study"), "identity:"+user.Id, user.GetString("identityCipher"), &identity); err != nil {
				return fmt.Errorf("user %s: %w", user.Id, err)
			}
			if len(identity.PersonalNumber) != 12 {
				return fmt.Errorf("user %s has invalid personal number", user.Id)
			}
			if err := migrateUploadDir(tx, user); err != nil {
				return err
			}
			user.Set("personalNumber", identity.PersonalNumber)
			user.RefreshTokenKey()
			if err := tx.Save(user); err != nil {
				return err
			}
		}
		signatures, err := tx.FindAllRecords("signatures")
		if err != nil {
			return err
		}
		for _, sig := range signatures {
			if sig.GetString("evidenceCipher") == "" {
				continue
			}
			var evidence json.RawMessage
			if err := openLegacy(cfg, sig.GetString("study"), "signature:"+sig.Id, sig.GetString("evidenceCipher"), &evidence); err != nil {
				return fmt.Errorf("signature %s: %w", sig.Id, err)
			}
			ciphertext, err := cfg.seal("signature:"+sig.Id, evidence)
			if err != nil {
				return err
			}
			sig.Set("evidenceCipher", ciphertext)
			if err := tx.Save(sig); err != nil {
				return err
			}
		}
		users, err = tx.FindCollectionByNameOrId("users")
		if err != nil {
			return err
		}
		for _, name := range []string{"study", "environment", "active", "invitationHash", "invitationCipher", "expectedCipher", "invitationExpiresAt", "identityHash", "identityCipher", "consentVersion", "consentSignature", "consentStatus", "metadata", "withdrawnAt", "invitationCode", "expectedPersonalNumber", "validityHours", "username", "name", "avatar", "event_date", "app_type", "device_token", "created", "updated"} {
			users.Fields.RemoveByName(name)
		}
		users.Fields.GetByName("personalNumber").(*core.TextField).Required = true
		users.Indexes = keepSystemIndexes(users.Indexes)
		users.Indexes = append(users.Indexes, "CREATE UNIQUE INDEX idx_user_personal_number ON users (personalNumber)")
		if err := tx.Save(users); err != nil {
			return err
		}
		consent, err := tx.FindCollectionByNameOrId("consent_texts")
		if err != nil {
			return err
		}
		consent.Fields.RemoveByName("study")
		consent.Indexes = []string{"CREATE UNIQUE INDEX idx_consent_text_version ON consent_texts (version)", "CREATE UNIQUE INDEX idx_consent_text_current ON consent_texts (current) WHERE current = TRUE"}
		if err := tx.Save(consent); err != nil {
			return err
		}
		sigs, err := tx.FindCollectionByNameOrId("signatures")
		if err != nil {
			return err
		}
		sigs.Fields.RemoveByName("study")
		sigs.Fields.RemoveByName("environment")
		sigs.Fields.RemoveByName("user")
		sigs.Fields.Add(&core.RelationField{Name: "user", CollectionId: users.Id, MaxSelect: 1})
		sigs.Indexes = []string{"CREATE UNIQUE INDEX idx_signature_attempt ON signatures (attemptId)", "CREATE UNIQUE INDEX idx_signatures_order ON signatures (orderHash) WHERE orderHash != ''"}
		return tx.Save(sigs)
	})
}

func keepSystemIndexes(indexes []string) []string {
	kept := make([]string, 0, len(indexes))
	for _, index := range indexes {
		if strings.Contains(index, "idx_tokenKey_") || strings.Contains(index, "idx_email_") {
			kept = append(kept, index)
		}
	}
	return kept
}

func migrateUploadDir(app core.App, user *core.Record) error {
	oldID := user.GetString("username")
	if oldID == "" || oldID == user.Id {
		return nil
	}
	if filepath.Base(oldID) != oldID || oldID == "." || oldID == ".." {
		return fmt.Errorf("invalid legacy upload directory for user %s", user.Id)
	}
	root := filepath.Join(app.DataDir(), "raw")
	oldPath, newPath := filepath.Join(root, oldID), filepath.Join(root, user.Id)
	if _, err := os.Stat(oldPath); err == nil {
		if _, err := os.Stat(newPath); err == nil {
			return fmt.Errorf("both legacy and new upload directories exist for user %s", user.Id)
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := os.Rename(oldPath, newPath); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	uploads, err := app.FindRecordsByFilter("dataUploads", "user={:user}", "", 0, 0, dbx.Params{"user": user.Id})
	if err != nil {
		return err
	}
	for _, upload := range uploads {
		path := upload.GetString("filePath")
		if !strings.HasPrefix(path, oldPath+string(filepath.Separator)) {
			continue
		}
		upload.Set("filePath", newPath+strings.TrimPrefix(path, oldPath))
		if err := app.Save(upload); err != nil {
			return err
		}
	}
	return nil
}

type keyring struct {
	ActiveKey      string
	EncryptionKeys map[string][]byte
}

func loadKeyring() (keyring, error) {
	var cfg keyring
	var keys map[string]string
	if file := os.Getenv("STUDY_SECRETS_FILE"); file != "" {
		raw, err := os.ReadFile(file)
		if err != nil {
			return cfg, err
		}
		var source struct {
			ActiveKey string            `json:"activeKey"`
			Keys      map[string]string `json:"encryptionKeys"`
		}
		if err := json.Unmarshal(raw, &source); err != nil {
			return cfg, err
		}
		cfg.ActiveKey, keys = source.ActiveKey, source.Keys
	}
	if raw := os.Getenv("STUDY_ENCRYPTION_KEYS"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &keys); err != nil {
			return cfg, err
		}
	}
	if active := os.Getenv("STUDY_ACTIVE_ENCRYPTION_KEY"); active != "" {
		cfg.ActiveKey = active
	}
	cfg.EncryptionKeys = map[string][]byte{}
	for id, encoded := range keys {
		key, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil || len(key) != 32 {
			return cfg, fmt.Errorf("invalid encryption key %s", id)
		}
		cfg.EncryptionKeys[id] = key
	}
	if len(cfg.EncryptionKeys[cfg.ActiveKey]) != 32 {
		return cfg, errors.New("configure the existing study encryption keys before migration")
	}
	return cfg, nil
}

func (cfg keyring) seal(purpose string, value any) (string, error) {
	block, err := aes.NewCipher(cfg.EncryptionKeys[cfg.ActiveKey])
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	plain, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return cfg.ActiveKey + ":" + base64.RawStdEncoding.EncodeToString(aead.Seal(nonce, nonce, plain, []byte(purpose))), nil
}

func openLegacy(cfg keyring, namespace, purpose, value string, target any) error {
	id, encoded, ok := strings.Cut(value, ":")
	if !ok {
		return errors.New("invalid ciphertext")
	}
	key := cfg.EncryptionKeys[id]
	if len(key) != 32 {
		return errors.New("required encryption key unavailable")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	raw, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil || len(raw) < aead.NonceSize() {
		return errors.New("invalid ciphertext")
	}
	plain, err := aead.Open(nil, raw[:aead.NonceSize()], raw[aead.NonceSize():], []byte(namespace+":"+purpose))
	if err != nil {
		return err
	}
	return json.Unmarshal(plain, target)
}
