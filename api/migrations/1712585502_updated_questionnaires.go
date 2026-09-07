package migrations

import (
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

		collection.ListRule = types.Pointer("user = @request.auth.id")

		collection.ViewRule = types.Pointer("user = @request.auth.id")

		collection.CreateRule = types.Pointer("")

		collection.UpdateRule = types.Pointer("user = @request.auth.id")

		collection.DeleteRule = types.Pointer("user = @request.auth.id")

		return app.Save(collection)
	}, func(app core.App) error {

		collection, err := app.FindCollectionByNameOrId("r8x60e97o694ihv")
		if err != nil {
			return err
		}

		collection.ListRule = nil

		collection.ViewRule = nil

		collection.CreateRule = nil

		collection.UpdateRule = nil

		collection.DeleteRule = nil

		return app.Save(collection)
	})
}
