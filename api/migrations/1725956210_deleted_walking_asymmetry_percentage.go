package migrations

import (
	"encoding/json"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {

		collection, err := app.FindCollectionByNameOrId("r7s6uvdofnd6qd3")
		if err != nil {
			return err
		}

		return app.Delete(collection)
	}, func(app core.App) error {
		jsonData := `{
  "id": "r7s6uvdofnd6qd3",
  "created": "2024-03-27 10:44:47.750Z",
  "updated": "2024-03-27 13:26:46.450Z",
  "name": "walking_asymmetry_percentage",
  "type": "base",
  "system": false,
  "indexes": [
    "CREATE INDEX \u0060idx_V9s5YVj\u0060 ON \u0060walking_asymmetry_percentage\u0060 (\u0060user\u0060)"
  ],
  "listRule": "user = @request.auth.id",
  "viewRule": "user = @request.auth.id",
  "createRule": "",
  "updateRule": "user = @request.auth.id",
  "deleteRule": "user = @request.auth.id",
  "fields": [
    {
      "system": false,
      "id": "3ylt63ye",
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
      "id": "k6zkzg2y",
      "name": "value",
      "type": "number",
      "required": false,
      "presentable": false,
      "noDecimal": false
    },
    {
      "system": false,
      "id": "g2hv0frp",
      "name": "date_from",
      "type": "date",
      "required": false,
      "presentable": false,
      "min": "",
      "max": ""
    },
    {
      "system": false,
      "id": "pmstcaeh",
      "name": "date_to",
      "type": "date",
      "required": false,
      "presentable": false,
      "min": "",
      "max": ""
    },
    {
      "system": false,
      "id": "zunmab1z",
      "name": "device_id",
      "type": "text",
      "required": false,
      "presentable": false,
      "pattern": ""
    },
    {
      "system": false,
      "id": "ou47bm61",
      "name": "source_id",
      "type": "text",
      "required": false,
      "presentable": false,
      "pattern": ""
    },
    {
      "system": false,
      "id": "st7kqjyc",
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
