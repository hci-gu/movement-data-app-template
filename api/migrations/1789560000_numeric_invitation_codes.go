package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		invitations, err := app.FindCollectionByNameOrId("study_invitations")
		if err != nil {
			return err
		}
		invitations.Fields.GetByName("invitationCode").(*core.TextField).Help = "Generated on save as six digits, for example 123-456. Reopen the saved record and copy this code to the participant. Visible to superusers only while unused and unexpired. Leave blank when creating."
		return app.Save(invitations)
	}, nil)
}
