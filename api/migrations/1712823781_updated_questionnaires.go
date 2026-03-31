package migrations

import (
	"encoding/json"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/daos"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/models/schema"
)

func init() {
	m.Register(func(db dbx.Builder) error {
		dao := daos.New(db);

		collection, err := dao.FindCollectionByNameOrId("r8x60e97o694ihv")
		if err != nil {
			return err
		}

		// add
		new_questions := &schema.SchemaField{}
		if err := json.Unmarshal([]byte(`{
			"system": false,
			"id": "q5jzzexa",
			"name": "questions",
			"type": "relation",
			"required": false,
			"presentable": false,
			"unique": false,
			"options": {
				"collectionId": "revsudry2wqi0dp",
				"cascadeDelete": false,
				"minSelect": null,
				"maxSelect": null,
				"displayFields": null
			}
		}`), new_questions); err != nil {
			return err
		}
		collection.Schema.AddField(new_questions)

		return dao.SaveCollection(collection)
	}, func(db dbx.Builder) error {
		dao := daos.New(db);

		collection, err := dao.FindCollectionByNameOrId("r8x60e97o694ihv")
		if err != nil {
			return err
		}

		// remove
		collection.Schema.RemoveField("q5jzzexa")

		return dao.SaveCollection(collection)
	})
}
