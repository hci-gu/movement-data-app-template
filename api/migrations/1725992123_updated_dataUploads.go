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

		// update
		edit_filePath := &core.TextField{}
		if err := json.Unmarshal([]byte(`{
  "system": false,
  "id": "zyfncesz",
  "name": "filePath",
  "type": "text",
  "required": false,
  "presentable": false,
  "pattern": ""
}`), edit_filePath); err != nil {
			return err
		}
		collection.Fields.Add(edit_filePath)

		return app.Save(collection)
	}, func(app core.App) error {

		collection, err := app.FindCollectionByNameOrId("b2mshgf2i5wivh1")
		if err != nil {
			return err
		}

		// update
		edit_filePath := &core.TextField{}
		if err := json.Unmarshal([]byte(`{
  "system": false,
  "id": "zyfncesz",
  "name": "filepath",
  "type": "text",
  "required": false,
  "presentable": false,
  "pattern": ""
}`), edit_filePath); err != nil {
			return err
		}
		collection.Fields.Add(edit_filePath)

		return app.Save(collection)
	})
}
