package migrations

import (
	"fmt"
	"regexp"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

var guardianIDPattern = regexp.MustCompile(`^[0-9]{3}-[0-9]{3}$`)

func init() {
	m.Register(migrateNumericGuardianIDs, nil)
}

func migrateNumericGuardianIDs(app core.App) error {
	return app.RunInTransaction(func(tx core.App) error {
		collection, err := tx.FindCollectionByNameOrId("signingRequests")
		if err != nil {
			return err
		}
		records, err := tx.FindAllRecords("signingRequests")
		if err != nil {
			return err
		}
		used := make(map[string]bool, len(records))
		for _, record := range records {
			used[record.Id] = true
		}
		next := 0
		for _, record := range records {
			if guardianIDPattern.MatchString(record.Id) {
				continue
			}
			var newID string
			for next < 1000000 {
				candidate := fmt.Sprintf("%03d-%03d", next/1000, next%1000)
				next++
				if !used[candidate] {
					newID = candidate
					break
				}
			}
			if newID == "" {
				return fmt.Errorf("no numeric signing request IDs remain")
			}
			if _, err := tx.DB().NewQuery("UPDATE signingRequests SET id = {:new} WHERE id = {:old}").Bind(dbx.Params{"new": newID, "old": record.Id}).Execute(); err != nil {
				return err
			}
			used[newID] = true
		}
		id := collection.Fields.GetByName("id").(*core.TextField)
		id.Min, id.Max = 7, 7
		id.Pattern = `^[0-9]{3}-[0-9]{3}$`
		id.AutogeneratePattern = ""
		collection.Fields.RemoveByName("tokenHash")
		collection.Fields.RemoveByName("tokenCipher")
		collection.Indexes = []string{
			"CREATE UNIQUE INDEX idx_signing_request_user_person ON signingRequests (user, personalNumber)",
			"CREATE UNIQUE INDEX idx_signing_request_user_slot ON signingRequests (user, slot)",
			"CREATE UNIQUE INDEX idx_signing_request_signature ON signingRequests (signature) WHERE signature != ''",
		}
		return tx.Save(collection)
	})
}
