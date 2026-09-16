package migrations

import (
	"encoding/json"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {

		collection, err := app.FindCollectionByNameOrId("b2mshgf2i5wivh1")
		if err != nil {
			return err
		}

		// add
		new_dataFrom := &core.DateField{}
		if err := json.Unmarshal([]byte(`{
  "system": false,
  "id": "ww5tq4wd",
  "name": "dataFrom",
  "type": "date",
  "required": false,
  "presentable": false,
  "min": "",
  "max": ""
}`), new_dataFrom); err != nil {
			return err
		}
		collection.Fields.Add(new_dataFrom)

		// add
		new_dataTo := &core.DateField{}
		if err := json.Unmarshal([]byte(`{
  "system": false,
  "id": "kkixhcs3",
  "name": "dataTo",
  "type": "date",
  "required": false,
  "presentable": false,
  "min": "",
  "max": ""
}`), new_dataTo); err != nil {
			return err
		}
		collection.Fields.Add(new_dataTo)

		return app.Save(collection)
	}, func(app core.App) error {

		collection, err := app.FindCollectionByNameOrId("b2mshgf2i5wivh1")
		if err != nil {
			return err
		}

		// remove
		collection.Fields.RemoveById("ww5tq4wd")

		// remove
		collection.Fields.RemoveById("kkixhcs3")

		return app.Save(collection)
	})
}
