package migrations

import (
	"encoding/json"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	m.Register(func(app core.App) error {

		collection, err := app.FindCollectionByNameOrId("r8x60e97o694ihv")
		if err != nil {
			return err
		}

		collection.ListRule = types.Pointer("")

		collection.ViewRule = types.Pointer("")

		collection.CreateRule = nil

		collection.UpdateRule = nil

		collection.DeleteRule = nil

		if err := json.Unmarshal([]byte(`[]`), &collection.Indexes); err != nil {
			return err
		}

		// remove
		collection.Fields.RemoveById("tmen8iab")

		return app.Save(collection)
	}, func(app core.App) error {

		collection, err := app.FindCollectionByNameOrId("r8x60e97o694ihv")
		if err != nil {
			return err
		}

		collection.ListRule = types.Pointer("user = @request.auth.id")

		collection.ViewRule = types.Pointer("user = @request.auth.id")

		collection.CreateRule = types.Pointer("")

		collection.UpdateRule = types.Pointer("user = @request.auth.id")

		collection.DeleteRule = types.Pointer("user = @request.auth.id")

		if err := json.Unmarshal([]byte(`[
  "CREATE INDEX \u0060idx_KRbmwPl\u0060 ON \u0060questionnaires\u0060 (\u0060user\u0060)"
]`), &collection.Indexes); err != nil {
			return err
		}

		// add
		del_user := &core.RelationField{}
		if err := json.Unmarshal([]byte(`{
  "system": false,
  "id": "tmen8iab",
  "name": "user",
  "type": "relation",
  "required": false,
  "presentable": false,
  "collectionId": "_pb_users_auth_",
  "cascadeDelete": true,
  "maxSelect": 1
}`), del_user); err != nil {
			return err
		}
		collection.Fields.Add(del_user)

		return app.Save(collection)
	})
}
