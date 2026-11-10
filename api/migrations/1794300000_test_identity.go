package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		for _, name := range []string{"users", "signatures", "dataUploads"} {
			c, err := app.FindCollectionByNameOrId(name)
			if err != nil {
				return err
			}
			c.Fields.Add(&core.TextField{Name: "simulationRun", Max: 48})
			if name == "users" {
				c.Fields.Add(&core.TextField{Name: "simulationPersona", Max: 48})
			}
			if err := app.Save(c); err != nil {
				return err
			}
		}
		return nil
	}, nil)
}
