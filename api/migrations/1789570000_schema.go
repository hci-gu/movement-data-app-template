package migrations

import (
	"app/internal/schema"
	"fmt"
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		if _, err := app.FindCollectionByNameOrId("signatures"); err == nil {
			return schema.Verify(app)
		}
		collections, err := app.FindAllCollections()
		if err != nil {
			return err
		}
		for _, c := range collections {
			if !c.System && c.Name != "users" {
				return fmt.Errorf("run the offline cutover command on a database copy before starting this release")
			}
		}
		if err := schema.CreateBase(app); err != nil {
			return err
		}
		return schema.AddStudyFields(app)
	}, nil)
}
