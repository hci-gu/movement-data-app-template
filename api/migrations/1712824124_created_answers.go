package migrations

import (
	"encoding/json"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		jsonData := `{
  "id": "1fy9xtsufxz9e4f",
  "created": "2024-04-11 08:28:44.748Z",
  "updated": "2024-04-11 08:28:44.748Z",
  "name": "answers",
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
      "id": "8ttfnsrp",
      "name": "user",
      "type": "relation",
      "required": false,
      "presentable": false,
      "collectionId": "_pb_users_auth_",
      "cascadeDelete": false,
      "maxSelect": 1
    },
    {
      "system": false,
      "id": "tfgfucvi",
      "name": "questionnaire",
      "type": "relation",
      "required": false,
      "presentable": false,
      "collectionId": "r8x60e97o694ihv",
      "cascadeDelete": false,
      "maxSelect": 1
    },
    {
      "system": false,
      "id": "ro4ojd2x",
      "name": "answers",
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

		collection, err := app.FindCollectionByNameOrId("1fy9xtsufxz9e4f")
		if err != nil {
			return err
		}

		return app.Delete(collection)
	})
}
