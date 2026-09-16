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
		new_questions := &core.RelationField{}
		if err := json.Unmarshal([]byte(`{
  "system": false,
  "id": "q5jzzexa",
  "name": "questions",
  "type": "relation",
  "required": false,
  "presentable": false,
  "collectionId": "revsudry2wqi0dp",
  "cascadeDelete": false,
  "maxSelect": 999
}`), new_questions); err != nil {
			return err
		}
		collection.Fields.Add(new_questions)

		return app.Save(collection)
	}, func(app core.App) error {

		collection, err := app.FindCollectionByNameOrId("r8x60e97o694ihv")
		if err != nil {
			return err
		}

		// remove
		collection.Fields.RemoveById("q5jzzexa")

		return app.Save(collection)
	})
}
