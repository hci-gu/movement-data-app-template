package migrations

import (
	"app/internal/schema"
	"fmt"
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		if err := schema.Verify(app); err != nil {
			return fmt.Errorf("KontoAPI requires a fresh database; use the new test data directory or reset disposable data: %w", err)
		}
		return nil
	}, nil)
}
