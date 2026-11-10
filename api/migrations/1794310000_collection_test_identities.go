package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		users, err := app.FindCollectionByNameOrId("users")
		if err != nil {
			return err
		}
		users.Fields.Add(&core.TextField{Name: "simulationRevision", Max: 48})
		users.RemoveIndex("idx_user_personal_number")
		users.AddIndex("idx_user_personal_number", true, "personalNumber,simulationRevision", "")
		if err := app.Save(users); err != nil {
			return err
		}
		mappings := core.NewBaseCollection("test_identity_mappings")
		mappings.Fields.Add(
			&core.TextField{Name: "realPersonalNumber", Required: true, Pattern: `^[0-9]{12}$`},
			&core.TextField{Name: "testPersonalNumber", Required: true, Pattern: `^[0-9]{12}$`},
			&core.SelectField{Name: "role", Required: true, Values: []string{"participant", "guardian"}, MaxSelect: 1},
			&core.TextField{Name: "revision", Hidden: true},
		)
		mappings.AddIndex("idx_test_identity_number", true, "testPersonalNumber", "")
		mappings.AddIndex("idx_test_participant_actor", true, "realPersonalNumber", "role = 'participant'")
		if err := app.Save(mappings); err != nil {
			return err
		}
		state, err := app.FindCollectionByNameOrId("simulation_state")
		if err != nil {
			return err
		}
		state.Fields.RemoveByName("fingerprint")
		return app.Save(state)
	}, nil)
}
