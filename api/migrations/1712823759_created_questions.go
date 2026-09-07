package migrations

import (
	"encoding/json"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		jsonData := `{
  "id": "revsudry2wqi0dp",
  "created": "2024-04-11 08:22:39.529Z",
  "updated": "2024-04-11 08:22:39.529Z",
  "name": "questions",
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
      "id": "erndbv8k",
      "name": "text",
      "type": "text",
      "required": false,
      "presentable": false,
      "pattern": ""
    },
    {
      "system": false,
      "id": "evuw4smv",
      "name": "type",
      "type": "select",
      "required": false,
      "presentable": false,
      "maxSelect": 1,
      "values": [
        "text",
        "singleChoice",
        "segmentControl",
        "painMedication",
        "painScale",
        "date"
      ]
    },
    {
      "system": false,
      "id": "exei5pwy",
      "name": "introduction",
      "type": "editor",
      "required": false,
      "presentable": false,
      "convertUrls": false
    },
    {
      "system": false,
      "id": "ltcb0lal",
      "name": "placeholder",
      "type": "text",
      "required": false,
      "presentable": false,
      "pattern": ""
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

		collection, err := app.FindCollectionByNameOrId("revsudry2wqi0dp")
		if err != nil {
			return err
		}

		return app.Delete(collection)
	})
}
