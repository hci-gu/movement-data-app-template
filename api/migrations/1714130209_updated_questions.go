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
		new_valueFromQuestion := &core.RelationField{}
		if err := json.Unmarshal([]byte(`{
  "system": false,
  "id": "yqp6lqi4",
  "name": "valueFromQuestion",
  "type": "relation",
  "required": false,
  "presentable": false,
  "collectionId": "revsudry2wqi0dp",
  "cascadeDelete": false,
  "maxSelect": 1
}`), new_valueFromQuestion); err != nil {
			return err
		}
		collection.Fields.Add(new_valueFromQuestion)

		return app.Save(collection)
	}, func(app core.App) error {

		collection, err := app.FindCollectionByNameOrId("revsudry2wqi0dp")
		if err != nil {
			return err
		}

		// remove
		collection.Fields.RemoveById("yqp6lqi4")

		return app.Save(collection)
	})
}
