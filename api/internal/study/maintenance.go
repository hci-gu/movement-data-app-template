package study

import (
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"time"
)

// PurgeSessions removes expired operational secrets after a 24-hour recovery
// grace period. Immutable consent evidence and identity mappings remain intact.
// Run locally while the server is stopped, against the same --dir.
func PurgeSessions(app core.App, cfg Config, now time.Time) (int, error) {
	cutoff := now.Add(-24 * time.Hour).Unix()
	count := 0
	err := app.RunInTransaction(func(tx core.App) error {
		specs := []struct {
			collection, filter string
			fields             []string
		}{
			{"enrollment_sessions", "expiresAt < {:cutoff}", []string{"identityCipher", "grantCipher", "tokenHash"}},
			{"study_invitations", "expiresAt < {:cutoff}", []string{"expectedCipher", "tokenHash"}},
			{"bankid_orders", "startedAt < {:cutoff} && (status='accepted' || status='rejected' || status='failed' || status='cancelled' || status='unresolved')", []string{"requestCipher", "resultCipher", "nonceHash", "ipHash"}},
		}
		for _, spec := range specs {
			records, err := tx.FindRecordsByFilter(spec.collection, "study={:study} && ("+spec.filter+")", "", 0, 0, dbx.Params{"study": cfg.StudyID, "cutoff": cutoff})
			if err != nil {
				return err
			}
			for _, r := range records {
				changed := false
				for _, field := range spec.fields {
					if r.GetString(field) != "" && r.GetString(field) != "expired:"+r.Id {
						// Unique token hashes need a distinct tombstone per record.
						if field == "tokenHash" {
							r.Set(field, "expired:"+r.Id)
						} else {
							r.Set(field, "")
						}
						changed = true
					}
				}
				if changed {
					if err := tx.Save(r); err != nil {
						return err
					}
					count++
				}
			}
		}
		sessions, err := tx.FindRecordsByFilter("app_sessions", "study={:study} && expiresAt < {:cutoff}", "", 0, 0, dbx.Params{"study": cfg.StudyID, "cutoff": cutoff})
		if err != nil {
			return err
		}
		for _, r := range sessions {
			if err := tx.Delete(r); err != nil {
				return err
			}
			count++
		}
		return nil
	})
	return count, err
}
