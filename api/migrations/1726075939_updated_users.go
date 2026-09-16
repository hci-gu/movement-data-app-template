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
		new_app_type := &core.TextField{}
		if err := json.Unmarshal([]byte(`{
  "system": false,
  "id": "bd8dc0s0",
  "name": "app_type",
  "type": "text",
  "required": false,
  "presentable": false,
  "pattern": ""
}`), new_app_type); err != nil {
			return err
		}
		collection.Fields.Add(new_app_type)

		return app.Save(collection)
	}, func(app core.App) error {

		collection, err := app.FindCollectionByNameOrId("_pb_users_auth_")
		if err != nil {
			return err
		}

		// remove
		collection.Fields.RemoveById("bd8dc0s0")

		return app.Save(collection)
	})
}
