package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		consent, err := app.FindCollectionByNameOrId("consent_versions")
		if err != nil {
			return err
		}
		for name, help := range map[string]string{
			"version":      "Enter a new, unique version label, for example 2026-01. Saving publishes this version immediately and requires participants to sign it before further uploads.",
			"title":        "Title displayed to participants, for example Study participation consent.",
			"text":         "Paste the approved plain-text consent (at most 30,000 UTF-8 bytes). Published consent cannot be edited or deleted; create a new version to change it.",
			"study":        "Assigned automatically from the server STUDY_ID. Leave blank when creating.",
			"documentHash": "Calculated automatically from the exact consent text. Leave blank when creating.",
		} {
			consent.Fields.GetByName(name).(*core.TextField).Help = help
		}
		for index, name := range []string{"version", "title", "text"} {
			consent.Fields.AddAt(index+1, consent.Fields.GetByName(name))
		}
		if err := app.Save(consent); err != nil {
			return err
		}
		invitations, err := app.FindCollectionByNameOrId("study_invitations")
		if err != nil {
			return err
		}
		invitations.Fields.Add(
			&core.NumberField{Name: "validityHours", OnlyInt: true, Help: "Invitation lifetime in hours (1–2160). Leave blank or 0 for 168 hours (7 days)."},
			&core.TextField{Name: "expectedPersonalNumber", Max: 12, Help: "Optional: bind to the intended signer's 12-digit personal number. Encrypted on save; this input is cleared. Use a synthetic test identity in test environments."},
			&core.TextField{Name: "invitationCode", Max: 100, Help: "Generated on save. Reopen the saved record and copy this code to the participant. Visible to superusers only while unused and unexpired. Leave blank when creating."},
			&core.TextField{Name: "tokenCipher", Max: 3000, Hidden: true, Help: "Server-managed encrypted invitation code. Leave blank."},
		)
		for name, help := range map[string]string{
			"participantId":  "Enter the participant's study identifier, for example TEST-001 (4–64 letters, digits, underscores or hyphens, beginning with a letter or digit). Saving issues an invitation; it does not enroll the participant until they sign in the app.",
			"study":          "Assigned automatically from the server STUDY_ID. Leave blank when creating.",
			"tokenHash":      "Server-managed verification digest. Leave blank when creating.",
			"expectedCipher": "Server-managed encrypted signer identity. Use expectedPersonalNumber when creating; leave this blank.",
			"claimedFlow":    "Server-managed enrollment session. Leave blank when creating.",
		} {
			invitations.Fields.GetByName(name).(*core.TextField).Help = help
		}
		invitations.Fields.GetByName("expiresAt").(*core.NumberField).Help = "Server-generated expiry as Unix seconds. Set validityHours when creating instead."
		invitations.Fields.GetByName("consumed").(*core.BoolField).Help = "Set automatically after successful enrollment. Issued invitations cannot be edited."
		for index, name := range []string{"participantId", "validityHours", "expectedPersonalNumber", "invitationCode"} {
			invitations.Fields.AddAt(index+1, invitations.Fields.GetByName(name))
		}
		return app.Save(invitations)
	}, nil)
}
