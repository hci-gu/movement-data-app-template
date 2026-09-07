package migrations

import (
	"encoding/json"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {

		collection, err := app.FindCollectionByNameOrId("1fy9xtsufxz9e4f")
		if err != nil {
			return err
		}

		// update
		edit_started := &core.DateField{}
		if err := json.Unmarshal([]byte(`{
  "system": false,
  "id": "3w7eidkr",
  "name": "started",
  "type": "date",
  "required": false,
  "presentable": false,
  "min": "",
  "max": ""
}`), edit_started); err != nil {
			return err
		}
		collection.Fields.Add(edit_started)

		return app.Save(collection)
	}, func(app core.App) error {

		collection, err := app.FindCollectionByNameOrId("1fy9xtsufxz9e4f")
		if err != nil {
			return err
		}

		// update
		edit_started := &core.DateField{}
		if err := json.Unmarshal([]byte(`{
  "system": false,
  "id": "3w7eidkr",
  "name": "startDate",
  "type": "date",
  "required": false,
  "presentable": false,
  "min": "",
  "max": ""
}`), edit_started); err != nil {
			return err
		}
		collection.Fields.Add(edit_started)

		return app.Save(collection)
	})
}
