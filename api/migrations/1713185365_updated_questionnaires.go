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

		// update
		edit_occurance := &schema.SchemaField{}
		if err := json.Unmarshal([]byte(`{
			"system": false,
			"id": "t2uadbki",
			"name": "occurance",
			"type": "select",
			"required": false,
			"presentable": false,
			"unique": false,
			"options": {
				"maxSelect": 1,
				"values": [
					"daily",
					"weekly",
					"monthly"
				]
			}
		}`), edit_occurance); err != nil {
			return err
		}
		collection.Schema.AddField(edit_occurance)

		return dao.SaveCollection(collection)
	}, func(db dbx.Builder) error {
		dao := daos.New(db);

		collection, err := dao.FindCollectionByNameOrId("r8x60e97o694ihv")
		if err != nil {
			return err
		}

		// update
		edit_occurance := &schema.SchemaField{}
		if err := json.Unmarshal([]byte(`{
			"system": false,
			"id": "t2uadbki",
			"name": "field",
			"type": "select",
			"required": false,
			"presentable": false,
			"unique": false,
			"options": {
				"maxSelect": 1,
				"values": [
					"daily",
					"weekly",
					"monthly"
				]
			}
		}`), edit_occurance); err != nil {
			return err
		}
		collection.Schema.AddField(edit_occurance)

		return dao.SaveCollection(collection)
	})
}
