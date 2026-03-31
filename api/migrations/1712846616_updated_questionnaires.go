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
		new_enabled := &schema.SchemaField{}
		if err := json.Unmarshal([]byte(`{
			"system": false,
			"id": "hq0i0oxy",
			"name": "enabled",
			"type": "bool",
			"required": false,
			"presentable": false,
			"unique": false,
			"options": {}
		}`), new_enabled); err != nil {
			return err
		}
		collection.Schema.AddField(new_enabled)

		// add
		new_description := &schema.SchemaField{}
		if err := json.Unmarshal([]byte(`{
			"system": false,
			"id": "xo7zswls",
			"name": "description",
			"type": "editor",
			"required": false,
			"presentable": false,
			"unique": false,
			"options": {
				"convertUrls": false
			}
		}`), new_description); err != nil {
			return err
		}
		collection.Schema.AddField(new_description)

		// add
		new_field := &schema.SchemaField{}
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
		}`), new_field); err != nil {
			return err
		}
		collection.Schema.AddField(new_field)

		return dao.SaveCollection(collection)
	}, func(db dbx.Builder) error {
		dao := daos.New(db);

		collection, err := dao.FindCollectionByNameOrId("r8x60e97o694ihv")
		if err != nil {
			return err
		}

		// remove
		collection.Schema.RemoveField("hq0i0oxy")

		// remove
		collection.Schema.RemoveField("xo7zswls")

		// remove
		collection.Schema.RemoveField("t2uadbki")

		return dao.SaveCollection(collection)
	})
}
