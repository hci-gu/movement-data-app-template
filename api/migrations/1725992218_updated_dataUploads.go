package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	m.Register(func(app core.App) error {

		collection, err := app.FindCollectionByNameOrId("b2mshgf2i5wivh1")
		if err != nil {
			return err
		}

		collection.ListRule = types.Pointer("user = @request.auth.id")

		collection.ViewRule = types.Pointer("user = @request.auth.id")

		return app.Save(collection)
	}, func(app core.App) error {

		collection, err := app.FindCollectionByNameOrId("b2mshgf2i5wivh1")
		if err != nil {
			return err
		}

		collection.ListRule = nil

		collection.ViewRule = nil

		return app.Save(collection)
	})
}
