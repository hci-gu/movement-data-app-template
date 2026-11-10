package migrations

import (
	"database/sql"
	"errors"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(removeSimulationState, nil)
}

func removeSimulationState(app core.App) error {
	collection, err := app.FindCollectionByNameOrId("simulation_state")
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return app.Delete(collection)
}
