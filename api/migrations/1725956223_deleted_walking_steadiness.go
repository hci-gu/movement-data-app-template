package migrations

import (
	"encoding/json"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {

		collection, err := app.FindCollectionByNameOrId("hmz4hww9sjhkxh4")
		if err != nil {
			return err
		}

		return app.Delete(collection)
	}, func(app core.App) error {
		jsonData := `{
  "id": "hmz4hww9sjhkxh4",
  "created": "2024-03-27 10:48:34.481Z",
  "updated": "2024-03-27 13:26:46.450Z",
  "name": "walking_steadiness",
  "type": "base",
  "system": false,
  "indexes": [
    "CREATE INDEX \u0060idx_vcpdkhH\u0060 ON \u0060walking_steadiness\u0060 (\u0060user\u0060)"
  ],
  "listRule": "user = @request.auth.id",
  "viewRule": "user = @request.auth.id",
  "createRule": "",
  "updateRule": "user = @request.auth.id",
  "deleteRule": "user = @request.auth.id",
  "fields": [
    {
      "system": false,
      "id": "n02aye2o",
      "name": "user",
      "type": "relation",
      "required": false,
      "presentable": false,
      "collectionId": "_pb_users_auth_",
      "cascadeDelete": true,
      "maxSelect": 1
    },
    {
      "system": false,
      "id": "6u62dehp",
      "name": "value",
      "type": "number",
      "required": false,
      "presentable": false,
      "noDecimal": false
    },
    {
      "system": false,
      "id": "6qtszo9h",
      "name": "date_from",
      "type": "date",
      "required": false,
      "presentable": false,
      "min": "",
      "max": ""
    },
    {
      "system": false,
      "id": "vc4irxv1",
      "name": "date_to",
      "type": "date",
      "required": false,
      "presentable": false,
      "min": "",
      "max": ""
    },
    {
      "system": false,
      "id": "hlkwt5oh",
      "name": "device_id",
      "type": "text",
      "required": false,
      "presentable": false,
      "pattern": ""
    },
    {
      "system": false,
      "id": "byszjney",
      "name": "source_id",
      "type": "text",
      "required": false,
      "presentable": false,
      "pattern": ""
    },
    {
      "system": false,
      "id": "thltw07g",
      "name": "source_name",
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
	})
}
