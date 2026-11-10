package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		for _, name := range []string{"users", "authentications", "consent_acceptances", "guardian_requests", "guardian_approvals", "dataUploads"} {
			c, err := app.FindCollectionByNameOrId(name)
			if err != nil {
				return err
			}
			c.Fields.Add(&core.TextField{Name: "simulationRun", Max: 48})
			if name == "users" {
				c.Fields.Add(&core.TextField{Name: "simulationPersona", Max: 48})
				c.Fields.Add(&core.TextField{Name: "simulationRevision", Max: 48})
				c.RemoveIndex("idx_user_personal_number")
				c.AddIndex("idx_user_personal_number", true, "personalNumber,simulationRevision", "")
			}
			if name == "guardian_approvals" {
				c.Fields.Add(&core.TextField{Name: "simulationRevision", Max: 48})
				c.RemoveIndex("idx_guardian_consent")
				c.AddIndex("idx_guardian_consent", true, "request,consent,simulationRevision", "")
			}
			if err := app.Save(c); err != nil {
				return err
			}
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
		return app.Save(mappings)
	}, nil)
}
