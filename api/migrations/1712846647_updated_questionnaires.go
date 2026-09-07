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

		collection.ListRule = types.Pointer("enabled = true")

		collection.ViewRule = types.Pointer("enabled = true")

		return app.Save(collection)
	}, func(app core.App) error {

		collection, err := app.FindCollectionByNameOrId("r8x60e97o694ihv")
		if err != nil {
			return err
		}

		collection.ListRule = types.Pointer("")

		collection.ViewRule = types.Pointer("")

		return app.Save(collection)
	})
}
