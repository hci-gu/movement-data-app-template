package migrations

import (
	"encoding/json"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {

		collection, err := app.FindCollectionByNameOrId("revsudry2wqi0dp")
		if err != nil {
			return err
		}

		// update
		edit_type := &core.SelectField{}
		if err := json.Unmarshal([]byte(`{
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
    "date",
    "stepDataAccess"
  ]
}`), edit_type); err != nil {
			return err
		}
		collection.Fields.Add(edit_type)

		return app.Save(collection)
	}, func(app core.App) error {

		collection, err := app.FindCollectionByNameOrId("revsudry2wqi0dp")
		if err != nil {
			return err
		}

		// update
		edit_type := &core.SelectField{}
		if err := json.Unmarshal([]byte(`{
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
}`), edit_type); err != nil {
			return err
		}
		collection.Fields.Add(edit_type)

		return app.Save(collection)
	})
}
