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

		// add
		new_enabled := &core.BoolField{}
		if err := json.Unmarshal([]byte(`{
  "system": false,
  "id": "hq0i0oxy",
  "name": "enabled",
  "type": "bool",
  "required": false,
  "presentable": false
}`), new_enabled); err != nil {
			return err
		}
		collection.Fields.Add(new_enabled)

		// add
		new_description := &core.EditorField{}
		if err := json.Unmarshal([]byte(`{
  "system": false,
  "id": "xo7zswls",
  "name": "description",
  "type": "editor",
  "required": false,
  "presentable": false,
  "convertUrls": false
}`), new_description); err != nil {
			return err
		}
		collection.Fields.Add(new_description)

		// add
		new_field := &core.SelectField{}
		if err := json.Unmarshal([]byte(`{
  "system": false,
  "id": "t2uadbki",
  "name": "field",
  "type": "select",
  "required": false,
  "presentable": false,
  "maxSelect": 1,
  "values": [
    "daily",
    "weekly",
    "monthly"
  ]
}`), new_field); err != nil {
			return err
		}
		collection.Fields.Add(new_field)

		return app.Save(collection)
	}, func(app core.App) error {

		collection, err := app.FindCollectionByNameOrId("r8x60e97o694ihv")
		if err != nil {
			return err
		}

		// remove
		collection.Fields.RemoveById("hq0i0oxy")

		// remove
		collection.Fields.RemoveById("xo7zswls")

		// remove
		collection.Fields.RemoveById("t2uadbki")

		return app.Save(collection)
	})
}
