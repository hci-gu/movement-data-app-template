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
		new_device_token := &core.TextField{}
		if err := json.Unmarshal([]byte(`{
  "system": false,
  "id": "orm1wbwg",
  "name": "device_token",
  "type": "text",
  "required": false,
  "presentable": false,
  "pattern": ""
}`), new_device_token); err != nil {
			return err
		}
		collection.Fields.Add(new_device_token)

		return app.Save(collection)
	}, func(app core.App) error {

		collection, err := app.FindCollectionByNameOrId("_pb_users_auth_")
		if err != nil {
			return err
		}

		// remove
		collection.Fields.RemoveById("orm1wbwg")

		return app.Save(collection)
	})
}
