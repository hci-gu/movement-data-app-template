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
		edit_text := &core.TextField{}
		if err := json.Unmarshal([]byte(`{
  "system": false,
  "id": "erndbv8k",
  "name": "text",
  "type": "text",
  "required": false,
  "presentable": true,
  "pattern": ""
}`), edit_text); err != nil {
			return err
		}
		collection.Fields.Add(edit_text)

		return app.Save(collection)
	}, func(app core.App) error {

		collection, err := app.FindCollectionByNameOrId("revsudry2wqi0dp")
		if err != nil {
			return err
		}

		// update
		edit_text := &core.TextField{}
		if err := json.Unmarshal([]byte(`{
  "system": false,
  "id": "erndbv8k",
  "name": "text",
  "type": "text",
  "required": false,
  "presentable": false,
  "pattern": ""
}`), edit_text); err != nil {
			return err
		}
		collection.Fields.Add(edit_text)

		return app.Save(collection)
	})
}
