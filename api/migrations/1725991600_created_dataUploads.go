package migrations

import (
	"encoding/json"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		jsonData := `{
  "id": "b2mshgf2i5wivh1",
  "created": "2024-09-10 18:06:40.737Z",
  "updated": "2024-09-10 18:06:40.737Z",
  "name": "dataUploads",
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
      "id": "va43tsb2",
      "name": "timestamp",
      "type": "date",
      "required": false,
      "presentable": false,
      "min": "",
      "max": ""
    },
    {
      "system": false,
      "id": "zyfncesz",
      "name": "filepath",
      "type": "text",
      "required": false,
      "presentable": false,
      "pattern": ""
    },
    {
      "system": false,
      "id": "kwvtw5gb",
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

		collection, err := app.FindCollectionByNameOrId("b2mshgf2i5wivh1")
		if err != nil {
			return err
		}

		return app.Delete(collection)
	})
}
