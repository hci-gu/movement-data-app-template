package migrations

import (
	"encoding/json"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {

		collection, err := app.FindCollectionByNameOrId("r8x60e97o694ihv")
		if err != nil {
			return err
		}

		// update
		edit_occurance := &core.SelectField{}
		if err := json.Unmarshal([]byte(`{
  "system": false,
  "id": "t2uadbki",
  "name": "occurance",
  "type": "select",
  "required": true,
  "presentable": false,
  "maxSelect": 1,
  "values": [
    "daily",
    "weekly",
    "monthly",
    "once"
  ]
}`), edit_occurance); err != nil {
			return err
		}
		collection.Fields.Add(edit_occurance)

		return app.Save(collection)
	}, func(app core.App) error {

		collection, err := app.FindCollectionByNameOrId("r8x60e97o694ihv")
		if err != nil {
			return err
		}

		// update
		edit_occurance := &core.SelectField{}
		if err := json.Unmarshal([]byte(`{
  "system": false,
  "id": "t2uadbki",
  "name": "occurance",
  "type": "select",
  "required": false,
  "presentable": false,
  "maxSelect": 1,
  "values": [
    "daily",
    "weekly",
    "monthly",
    "once"
  ]
}`), edit_occurance); err != nil {
			return err
		}
		collection.Fields.Add(edit_occurance)

		return app.Save(collection)
	})
}
