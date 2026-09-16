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
		new_dependency := &core.RelationField{}
		if err := json.Unmarshal([]byte(`{
  "system": false,
  "id": "aqh1wouk",
  "name": "dependency",
  "type": "relation",
  "required": false,
  "presentable": false,
  "collectionId": "revsudry2wqi0dp",
  "cascadeDelete": false,
  "maxSelect": 1
}`), new_dependency); err != nil {
			return err
		}
		collection.Fields.Add(new_dependency)

		// add
		new_dependencyValue := &core.JSONField{}
		if err := json.Unmarshal([]byte(`{
  "system": false,
  "id": "wjdpapo1",
  "name": "dependencyValue",
  "type": "json",
  "required": false,
  "presentable": false,
  "maxSize": 2000000
}`), new_dependencyValue); err != nil {
			return err
		}
		collection.Fields.Add(new_dependencyValue)

		return app.Save(collection)
	}, func(app core.App) error {

		collection, err := app.FindCollectionByNameOrId("revsudry2wqi0dp")
		if err != nil {
			return err
		}

		// remove
		collection.Fields.RemoveById("aqh1wouk")

		// remove
		collection.Fields.RemoveById("wjdpapo1")

		return app.Save(collection)
	})
}
