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

		// remove
		collection.Fields.RemoveById("xtjmexnj")

		return app.Save(collection)
	}, func(app core.App) error {

		collection, err := app.FindCollectionByNameOrId("r8x60e97o694ihv")
		if err != nil {
			return err
		}

		// add
		del_answers := &core.JSONField{}
		if err := json.Unmarshal([]byte(`{
  "system": false,
  "id": "xtjmexnj",
  "name": "answers",
  "type": "json",
  "required": false,
  "presentable": false,
  "maxSize": 2000000
}`), del_answers); err != nil {
			return err
		}
		collection.Fields.Add(del_answers)

		return app.Save(collection)
	})
}
