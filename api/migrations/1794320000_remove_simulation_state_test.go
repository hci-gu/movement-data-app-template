package migrations

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func TestRemoveSimulationStatePreservesMappings(t *testing.T) {
	app := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: t.TempDir()})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	defer app.ResetBootstrapState()
	if err := app.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	if _, err := app.FindCollectionByNameOrId("simulation_state"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("fresh database still contains simulation_state")
	}
	// Recreate the deployed schema, with a saved mapping, before upgrading it.
	state := core.NewBaseCollection("simulation_state")
	state.Fields.Add(&core.TextField{Name: "runId", Required: true})
	if err := app.Save(state); err != nil {
		t.Fatal(err)
	}
	marker := core.NewRecord(state)
	marker.Set("runId", "existing-test")
	if err := app.Save(marker); err != nil {
		t.Fatal(err)
	}
	collection, err := app.FindCollectionByNameOrId("test_identity_mappings")
	if err != nil {
		t.Fatal(err)
	}
	mapping := core.NewRecord(collection)
	mapping.Set("realPersonalNumber", "198001012384")
	mapping.Set("testPersonalNumber", "201101011003")
	mapping.Set("role", "participant")
	mapping.Set("revision", "existing-revision")
	if err := app.Save(mapping); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := app.RunInTransaction(removeSimulationState); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := app.FindCollectionByNameOrId("simulation_state"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("migration did not remove simulation_state")
	}
	saved, err := app.FindRecordById("test_identity_mappings", mapping.Id)
	if err != nil || saved.GetString("realPersonalNumber") != mapping.GetString("realPersonalNumber") || saved.GetString("testPersonalNumber") != mapping.GetString("testPersonalNumber") || saved.GetString("role") != "participant" || saved.GetString("revision") != "existing-revision" {
		t.Fatal("migration changed the saved mapping")
	}
}
