package migrations

import (
	"testing"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func TestOldGuardianRequestsGetNumericIDs(t *testing.T) {
	app := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: t.TempDir()})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	defer app.ResetBootstrapState()
	if err := app.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	collection, err := app.FindCollectionByNameOrId("signingRequests")
	if err != nil {
		t.Fatal(err)
	}
	idField := collection.Fields.GetByName("id").(*core.TextField)
	idField.Min, idField.Max = 15, 15
	idField.Pattern = `^[a-z0-9]+$`
	idField.AutogeneratePattern = `[a-z0-9]{15}`
	collection.Fields.Add(&core.TextField{Name: "tokenHash", Max: 64, Required: true, Hidden: true}, &core.TextField{Name: "tokenCipher", Max: 1000, Hidden: true})
	collection.Indexes = append(collection.Indexes, "CREATE UNIQUE INDEX idx_signing_request_token ON signingRequests (tokenHash)")
	if err := app.Save(collection); err != nil {
		t.Fatal(err)
	}
	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		t.Fatal(err)
	}
	user := core.NewRecord(users)
	user.Set("personalNumber", "201001012384")
	user.SetPassword("internal-password")
	if err := app.Save(user); err != nil {
		t.Fatal(err)
	}
	oldIDs := make([]string, 2)
	for i, personal := range []string{"198001012384", "197501012384"} {
		request := core.NewRecord(collection)
		request.Set("user", user.Id)
		request.Set("slot", i+1)
		request.Set("personalNumber", personal)
		request.Set("tokenHash", personal)
		request.Set("tokenCipher", "old-encrypted-token")
		if err := app.Save(request); err != nil {
			t.Fatal(err)
		}
		oldIDs[i] = request.Id
	}
	if err := migrateNumericGuardianIDs(app); err != nil {
		t.Fatal(err)
	}
	requests, err := app.FindAllRecords("signingRequests")
	if err != nil || len(requests) != 2 {
		t.Fatalf("migrated requests: %v", err)
	}
	for _, request := range requests {
		if !guardianIDPattern.MatchString(request.Id) || request.GetString("user") != user.Id {
			t.Fatalf("invalid migrated request %s", request.Id)
		}
	}
	collection, err = app.FindCollectionByNameOrId("signingRequests")
	if err != nil {
		t.Fatal(err)
	}
	if collection.Fields.GetByName("tokenHash") != nil || collection.Fields.GetByName("tokenCipher") != nil {
		t.Fatal("obsolete link token fields remain")
	}
	for _, oldID := range oldIDs {
		if _, err := app.FindRecordById("signingRequests", oldID); err == nil {
			t.Fatal("old signing request ID remains active")
		}
	}
}
