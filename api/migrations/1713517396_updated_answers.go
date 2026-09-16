package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	m.Register(func(app core.App) error {

		collection, err := app.FindCollectionByNameOrId("1fy9xtsufxz9e4f")
		if err != nil {
			return err
		}

		collection.ListRule = types.Pointer("")

		return app.Save(collection)
	}, func(app core.App) error {

		collection, err := app.FindCollectionByNameOrId("1fy9xtsufxz9e4f")
		if err != nil {
			return err
		}

		collection.ListRule = types.Pointer("user = @request.auth.id")

		return app.Save(collection)
	})
}
