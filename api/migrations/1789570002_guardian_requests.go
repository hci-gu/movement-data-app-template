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
		users.Fields.Add(&core.NumberField{Name: "guardianCount"})
		if err := app.Save(users); err != nil {
			return err
		}
		signatures, err := app.FindCollectionByNameOrId("signatures")
		if err != nil {
			return err
		}
		requests := core.NewBaseCollection("signingRequests")
		id := requests.Fields.GetByName("id").(*core.TextField)
		id.Min, id.Max = 7, 7
		id.Pattern = `^[0-9]{3}-[0-9]{3}$`
		id.AutogeneratePattern = ""
		requests.Fields.Add(
			&core.RelationField{Name: "user", CollectionId: users.Id, MaxSelect: 1, Required: true},
			&core.RelationField{Name: "signature", CollectionId: signatures.Id, MaxSelect: 1},
			&core.NumberField{Name: "slot", Required: true},
			&core.TextField{Name: "personalNumber", Min: 12, Max: 12, Required: true, Hidden: true},
			&core.AutodateField{Name: "created", OnCreate: true},
		)
		requests.Indexes = []string{
			"CREATE UNIQUE INDEX idx_signing_request_user_person ON signingRequests (user, personalNumber)",
			"CREATE UNIQUE INDEX idx_signing_request_user_slot ON signingRequests (user, slot)",
			"CREATE UNIQUE INDEX idx_signing_request_signature ON signingRequests (signature) WHERE signature != ''",
		}
		return app.Save(requests)
	}, nil)
}
