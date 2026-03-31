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

		collection, err := dao.FindCollectionByNameOrId("revsudry2wqi0dp")
		if err != nil {
			return err
		}

		// add
		new_options := &schema.SchemaField{}
		if err := json.Unmarshal([]byte(`{
			"system": false,
			"id": "sa5gw6r2",
			"name": "options",
			"type": "relation",
			"required": false,
			"presentable": false,
			"unique": false,
			"options": {
				"collectionId": "7eppoiupbbau54z",
				"cascadeDelete": false,
				"minSelect": null,
				"maxSelect": 1,
				"displayFields": null
			}
		}`), new_options); err != nil {
			return err
		}
		collection.Schema.AddField(new_options)

		return dao.SaveCollection(collection)
	}, func(db dbx.Builder) error {
		dao := daos.New(db);

		collection, err := dao.FindCollectionByNameOrId("revsudry2wqi0dp")
		if err != nil {
			return err
		}

		// remove
		collection.Schema.RemoveField("sa5gw6r2")

		return dao.SaveCollection(collection)
	})
}
