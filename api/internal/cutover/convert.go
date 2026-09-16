// Package cutover performs the one-time, offline conversion. The server does not import it.
package cutover

import (
	"app/internal/schema"
	"app/internal/study"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

func Convert(app core.App, cfg study.Config) error {
	if err := cfg.ValidateSecrets(); err != nil {
		return err
	}
	if _, err := app.FindCollectionByNameOrId("signatures"); err == nil {
		return errors.New("database is already converted")
	}
	// File conversion only touches the copy, before changing database references.
	paths, err := convertUploads(app.DataDir())
	if err != nil {
		return err
	}
	return app.RunInTransaction(func(tx core.App) error {
		if err := schema.AddStudyFields(tx); err != nil {
			return err
		}
		all := func(name string) ([]*core.Record, error) {
			if _, err := tx.FindCollectionByNameOrId(name); err != nil {
				return nil, nil
			}
			return tx.FindAllRecords(name)
		}
		create := func(name string) (*core.Record, error) {
			c, err := tx.FindCollectionByNameOrId(name)
			if err != nil {
				return nil, err
			}
			return core.NewRecord(c), nil
		}
		users, err := all("users")
		if err != nil {
			return err
		}
		for _, u := range users {
			u.RefreshTokenKey()
			u.Set("study", cfg.StudyID)
			u.Set("environment", cfg.Environment)
			u.Set("active", false)
			if err := tx.Save(u); err != nil {
				return err
			}
		}
		appendMetadata := func(u *core.Record, entry any) error {
			var entries []any
			if err := u.UnmarshalJSONField("metadata", &entries); err != nil {
				return err
			}
			u.Set("metadata", append(entries, entry))
			return tx.Save(u)
		}
		participant := func(id string) (*core.Record, error) {
			u, err := tx.FindFirstRecordByData("users", "username", id)
			if errors.Is(err, sql.ErrNoRows) {
				u, err = create("users")
				if err != nil {
					return nil, err
				}
				u.Set("username", id)
				u.SetPassword(strings.Repeat("x", 32) + id)
				u.Set("study", cfg.StudyID)
				u.Set("environment", cfg.Environment)
				err = tx.Save(u)
			}
			return u, err
		}
		checkStudy := func(r *core.Record) error {
			if r.GetString("study") != "" && r.GetString("study") != cfg.StudyID {
				return fmt.Errorf("record %s belongs to a different study; conversion requires its own configuration", r.Id)
			}
			return nil
		}
		docs, err := all("consent_versions")
		if err != nil {
			return err
		}
		settings, err := all("study_settings")
		if err != nil {
			return err
		}
		current := map[string]string{}
		for _, r := range settings {
			if err := checkStudy(r); err != nil {
				return err
			}
			current[r.GetString("study")] = r.GetString("currentVersion")
		}
		for _, old := range docs {
			if err := checkStudy(old); err != nil {
				return err
			}
			r, err := create("consent_texts")
			if err != nil {
				return err
			}
			r.Id = old.Id
			for _, key := range []string{"study", "version", "title", "text", "documentHash", "created", "updated"} {
				r.Set(key, old.Get(key))
			}
			r.Set("current", current[old.GetString("study")] == old.Id)
			if err := tx.Save(r); err != nil {
				return err
			}
		}
		identities, err := all("participant_identities")
		if err != nil {
			return err
		}
		for _, r := range identities {
			if err := checkStudy(r); err != nil {
				return err
			}
			u, err := tx.FindRecordById("users", r.GetString("user"))
			if err != nil {
				return err
			}
			if u.GetString("identityHash") != "" {
				return fmt.Errorf("conflicting identity mappings for user %s", u.Id)
			}
			// Identity ciphertext already binds to the retained user ID; authenticate it before copying.
			var person json.RawMessage
			if err := cfg.Open("identity:"+u.Id, r.GetString("identityCipher"), &person); err != nil {
				return err
			}
			u.Set("identityHash", r.GetString("identityHash"))
			u.Set("identityCipher", r.GetString("identityCipher"))
			u.Set("environment", r.GetString("environment"))
			u.Set("active", true)
			if err := tx.Save(u); err != nil {
				return err
			}
		}
		invites, err := all("study_invitations")
		if err != nil {
			return err
		}
		live := map[string]bool{}
		sort.Slice(invites, func(i, j int) bool { return invites[i].GetInt("expiresAt") < invites[j].GetInt("expiresAt") })
		for _, r := range invites {
			if err := checkStudy(r); err != nil {
				return err
			}
			u, err := participant(r.GetString("participantId"))
			if err != nil {
				return err
			}
			if r.GetBool("consumed") || u.GetBool("active") {
				continue
			}
			if r.GetInt("expiresAt") > int(time.Now().Unix()) {
				if live[u.Id] {
					return fmt.Errorf("multiple live invitations for participant %s; reconcile the source copy first", u.Id)
				}
				live[u.Id] = true
			}
			u.Set("invitationHash", r.GetString("tokenHash"))
			u.Set("invitationExpiresAt", r.GetInt("expiresAt"))
			u.Set("validityHours", r.GetInt("validityHours"))
			u.Set("expectedCipher", "")
			u.Set("invitationCipher", "")
			for _, pair := range [][3]string{{"tokenCipher", "invitation-code:", "invitationCipher"}, {"expectedCipher", "invitation:", "expectedCipher"}} {
				if r.GetString(pair[0]) == "" {
					continue
				}
				var raw json.RawMessage
				if err := cfg.Open(pair[1]+r.Id, r.GetString(pair[0]), &raw); err != nil {
					return err
				}
				purpose := "invitation-code:"
				if pair[2] == "expectedCipher" {
					purpose = "expected:"
				}
				cipher, err := cfg.Seal(purpose+u.Id, raw)
				if err != nil {
					return err
				}
				u.Set(pair[2], cipher)
			}
			if err := tx.Save(u); err != nil {
				return err
			}
		}
		sigs, err := all("consent_signatures")
		if err != nil {
			return err
		}
		convertedOrders := map[string]bool{}
		for _, old := range sigs {
			if err := checkStudy(old); err != nil {
				return err
			}
			r, err := create("signatures")
			if err != nil {
				return err
			}
			r.Id = old.Id
			for _, key := range []string{"study", "environment", "orderHash", "user", "version", "outcome", "receivedAt", "created", "updated"} {
				r.Set(key, old.Get(key))
			}
			r.Set("attemptId", old.GetString("order"))
			r.Set("purpose", "sign")
			r.Set("providerStatus", "complete")
			if order, err := tx.FindRecordById("bankid_orders", old.GetString("order")); err == nil {
				r.Set("startedAt", order.GetInt("startedAt"))
				r.Set("reason", order.GetString("hintCode"))
			}
			var evidence json.RawMessage
			if err := cfg.Open("signature:"+old.GetString("order"), old.GetString("evidenceCipher"), &evidence); err != nil {
				return err
			}
			cipher, err := cfg.Seal("signature:"+r.Id, evidence)
			if err != nil {
				return err
			}
			r.Set("evidenceCipher", cipher)
			if err := tx.Save(r); err != nil {
				return err
			}
			convertedOrders[old.GetString("order")] = true
		}
		orders, err := all("bankid_orders")
		if err != nil {
			return err
		}
		for _, old := range orders {
			if err := checkStudy(old); err != nil {
				return err
			}
			if convertedOrders[old.Id] {
				continue
			}
			outcome := old.GetString("status")
			switch outcome {
			case "accepted", "rejected", "failed", "cancelled":
			default:
				outcome = "unknown"
			}
			r, err := create("signatures")
			if err != nil {
				return err
			}
			for _, key := range []string{"study", "environment", "orderHash", "version", "purpose", "startedAt", "created", "updated"} {
				r.Set(key, old.Get(key))
			}
			r.Set("attemptId", old.Id)
			r.Set("outcome", outcome)
			r.Set("reason", old.GetString("hintCode"))
			r.Set("receivedAt", old.GetDateTime("updated").Time().Unix())
			if flow, err := tx.FindRecordById("enrollment_sessions", old.GetString("flow")); err == nil {
				r.Set("user", flow.GetString("user"))
			}
			ev := study.Evidence{LocalError: "Converted stored attempt; unavailable provider evidence remains unavailable.", ReceivedAt: old.GetString("updated")}
			if old.GetString("requestCipher") != "" {
				if err := cfg.Open("order:"+old.Id, old.GetString("requestCipher"), &ev); err != nil {
					return err
				}
			}
			if old.GetString("resultCipher") != "" {
				if err := cfg.Open("result:"+old.Id, old.GetString("resultCipher"), &ev.Completion); err != nil {
					return err
				}
				var result struct {
					Status string `json:"status"`
				}
				if err := json.Unmarshal(ev.Completion, &result); err != nil {
					return err
				}
				r.Set("providerStatus", result.Status)
			}
			if err := tx.Save(r); err != nil {
				return err
			}
			cipher, err := cfg.Seal("signature:"+r.Id, ev)
			if err != nil {
				return err
			}
			r.Set("evidenceCipher", cipher)
			if err := tx.Save(r); err != nil {
				return err
			}
		}
		events, err := all("consent_events")
		if err != nil {
			return err
		}
		for _, e := range events {
			if err := checkStudy(e); err != nil {
				return err
			}
			if e.GetString("kind") != "withdrawn" {
				continue
			}
			sig, err := tx.FindRecordById("signatures", e.GetString("signature"))
			if err != nil {
				return err
			}
			sig.Set("withdrawnAt", e.GetInt("occurredAt"))
			if err := tx.Save(sig); err != nil {
				return err
			}
			u, err := tx.FindRecordById("users", e.GetString("user"))
			if err != nil {
				return err
			}
			if u.GetString("consentSignature") == sig.Id {
				u.Set("withdrawnAt", e.GetInt("occurredAt"))
				if err := tx.Save(u); err != nil {
					return err
				}
			}
		}
		for _, name := range []string{"info", "consent"} {
			records, err := all(name)
			if err != nil {
				return err
			}
			for _, r := range records {
				key := "user"
				if name == "consent" {
					key = "field"
				}
				u, err := tx.FindRecordById("users", r.GetString(key))
				if err != nil {
					return fmt.Errorf("orphan %s record %s: %w", name, r.Id, err)
				}
				if err := appendMetadata(u, map[string]any{"source": name, "record": r.FieldsData()}); err != nil {
					return err
				}
			}
		}
		uploads, err := all("dataUploads")
		if err != nil {
			return err
		}
		for _, r := range uploads {
			oldPath := filepath.ToSlash(r.GetString("filePath"))
			pos := strings.Index(oldPath, "/raw/")
			relative := ""
			if pos >= 0 {
				relative = oldPath[pos+1:]
			} else if strings.HasPrefix(oldPath, "raw/") {
				relative = oldPath
			}
			if relative != "" {
				destination := filepath.Join(app.DataDir(), filepath.FromSlash(relative))
				if converted, ok := paths[destination]; ok {
					destination = converted
				}
				r.Set("filePath", destination)
				if err := tx.Save(r); err != nil {
					return err
				}
			}
		}

		users, err = all("users")
		if err != nil {
			return err
		}
		for _, u := range users {
			if u.GetBool("consent") {
				if err := appendMetadata(u, map[string]any{"source": "priorConsentFlag", "value": true}); err != nil {
					return err
				}
			}
			if u.GetString("consentSignature") == "" {
				u.Set("consentStatus", "")
				u.Set("consentVersion", "")
				if err := tx.Save(u); err != nil {
					return err
				}
			}
		}
		usersCollection, err := tx.FindCollectionByNameOrId("users")
		if err != nil {
			return err
		}
		usersCollection.Fields.RemoveByName("consent")
		if err := tx.Save(usersCollection); err != nil {
			return err
		}
		for _, name := range []string{"consent_signatures", "consent_versions", "study_settings", "study_invitations", "participant_identities", "consent_events", "app_sessions", "enrollment_sessions", "bankid_orders", "info", "consent"} {
			if c, err := tx.FindCollectionByNameOrId(name); err == nil {
				if err := tx.Delete(c); err != nil {
					return err
				}
			}
		}
		if err := schema.SetAccessRules(tx); err != nil {
			return err
		}
		return schema.Verify(tx)
	})
}

// Convert the old flat gzip layout once. The runtime reads only upload sessions.
func convertUploads(dir string) (map[string]string, error) {
	paths := map[string]string{}
	root := filepath.Join(dir, "raw")
	participants, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return paths, nil
	}
	if err != nil {
		return nil, err
	}
	for _, p := range participants {
		if !p.IsDir() {
			continue
		}
		folder := filepath.Join(root, p.Name())
		files, err := os.ReadDir(folder)
		if err != nil {
			return nil, err
		}
		index := 0
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".gz") {
				continue
			}
			dest := filepath.Join(folder, "_uploads", "00000000T000000.000000000Z-converted", fmt.Sprintf("chunk-%05d.json.gz", index))
			index++
			if err := os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
				return nil, err
			}
			source := filepath.Join(folder, f.Name())
			if _, err := os.Stat(dest); !os.IsNotExist(err) {
				return nil, fmt.Errorf("converted upload destination already exists: %s", dest)
			}
			if err := os.Rename(source, dest); err != nil {
				return nil, err
			}
			paths[source] = dest
			// Historical deployments used the container's absolute data directory.
			relative, _ := filepath.Rel(dir, source)
			paths[filepath.Join("/pb/pb_data", relative)] = dest
		}
	}
	return paths, nil
}
