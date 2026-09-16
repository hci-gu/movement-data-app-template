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

		// add
		new_options := &core.RelationField{}
		if err := json.Unmarshal([]byte(`{
  "system": false,
  "id": "sa5gw6r2",
  "name": "options",
  "type": "relation",
  "required": false,
  "presentable": false,
  "collectionId": "7eppoiupbbau54z",
  "cascadeDelete": false,
  "maxSelect": 1
}`), new_options); err != nil {
			return err
		}
		collection.Fields.Add(new_options)

		return app.Save(collection)
	}, func(app core.App) error {

		collection, err := app.FindCollectionByNameOrId("revsudry2wqi0dp")
		if err != nil {
			return err
		}

		// remove
		collection.Fields.RemoveById("sa5gw6r2")

		return app.Save(collection)
	})
}
