package migrations

import (
	"encoding/json"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		jsonData := `{
  "id": "w9724r5hvmhn56r",
  "created": "2025-01-28 08:28:22.262Z",
  "updated": "2025-01-28 08:28:22.262Z",
  "name": "info",
  "type": "base",
  "system": false,
  "indexes": [],
  "listRule": null,
  "viewRule": null,
  "createRule": null,
  "updateRule": null,
  "deleteRule": null,
  "fields": [
    {
      "system": false,
      "id": "1j5156u6",
      "name": "data",
      "type": "json",
      "required": false,
      "presentable": false,
      "maxSize": 2000000
    },
    {
      "system": false,
      "id": "sqjzecyq",
      "name": "user",
      "type": "relation",
      "required": false,
      "presentable": false,
      "collectionId": "_pb_users_auth_",
      "cascadeDelete": false,
      "maxSelect": 1
    },
    {
      "type": "autodate",
      "name": "created",
      "onCreate": true
    },
    {
      "type": "autodate",
      "name": "updated",
      "onCreate": true,
      "onUpdate": true
    }
  ]
}`

		collection := &core.Collection{}
		if err := json.Unmarshal([]byte(jsonData), &collection); err != nil {
			return err
		}

		return app.Save(collection)
	}, func(app core.App) error {

		collection, err := app.FindCollectionByNameOrId("w9724r5hvmhn56r")
		if err != nil {
			return err
		}

		return app.Delete(collection)
	})
}
