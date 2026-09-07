package migrations

import (
	"encoding/json"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		jsonData := `{
  "id": "7eppoiupbbau54z",
  "created": "2024-04-11 08:38:43.650Z",
  "updated": "2024-04-11 08:38:43.650Z",
  "name": "questionOptions",
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
      "id": "vrvunsbx",
      "name": "name",
      "type": "text",
      "required": false,
      "presentable": false,
      "pattern": ""
    },
    {
      "system": false,
      "id": "1jlyi5mv",
      "name": "value",
      "type": "json",
      "required": false,
      "presentable": false,
      "maxSize": 2000000
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

		collection, err := app.FindCollectionByNameOrId("7eppoiupbbau54z")
		if err != nil {
			return err
		}

		return app.Delete(collection)
	})
}
