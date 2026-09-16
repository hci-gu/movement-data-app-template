package migrations

import (
	"encoding/json"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {

		collection, err := app.FindCollectionByNameOrId("_pb_users_auth_")
		if err != nil {
			return err
		}

		// add
		new_consent := &core.BoolField{}
		if err := json.Unmarshal([]byte(`{
  "system": false,
  "id": "consent01",
  "name": "consent",
  "type": "bool",
  "required": false,
  "presentable": false
}`), new_consent); err != nil {
			return err
		}
		collection.Fields.Add(new_consent)

		return app.Save(collection)
	}, func(app core.App) error {

		collection, err := app.FindCollectionByNameOrId("_pb_users_auth_")
		if err != nil {
			return err
		}

		// remove
		collection.Fields.RemoveById("consent01")

		return app.Save(collection)
	})
}
