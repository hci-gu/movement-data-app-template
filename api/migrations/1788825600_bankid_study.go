package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		definitions := []struct {
			name    string
			fields  []core.Field
			indexes []string
		}{
			{"consent_versions", []core.Field{
				textField("study", 100), textField("version", 100), textField("title", 200), textField("text", 30000), textField("documentHash", 64),
			}, []string{"CREATE UNIQUE INDEX idx_consent_version ON consent_versions (study, version)"}},
			{"study_settings", []core.Field{textField("study", 100), textField("currentVersion", 15)}, []string{"CREATE UNIQUE INDEX idx_study_settings ON study_settings (study)"}},
			{"study_invitations", []core.Field{
				textField("study", 100), textField("tokenHash", 64), textField("participantId", 100), textField("expectedCipher", 3000),
				textField("claimedFlow", 15), &core.NumberField{Name: "expiresAt"}, &core.BoolField{Name: "consumed"},
			}, []string{"CREATE UNIQUE INDEX idx_invitation_token ON study_invitations (tokenHash)"}},
			{"enrollment_sessions", []core.Field{
				textField("study", 100), textField("environment", 20), textField("tokenHash", 64), textField("kind", 20), textField("invitation", 15),
				textField("participantId", 100), textField("user", 15), textField("identityCipher", 3000), textField("latestOrder", 15),
				textField("grantCipher", 10000), &core.NumberField{Name: "expiresAt"}, &core.NumberField{Name: "authenticatedAt"},
			}, []string{"CREATE UNIQUE INDEX idx_enrollment_secret ON enrollment_sessions (tokenHash)"}},
			{"bankid_orders", []core.Field{
				textField("study", 100), textField("environment", 20), textField("flow", 15), textField("requestKey", 100),
				textField("purpose", 10), textField("mode", 10), textField("status", 30), textField("hintCode", 100), textField("version", 15),
				textField("certificate", 64), textField("orderHash", 64), textField("requestCipher", 500000), textField("resultCipher", 3000000),
				textField("nonceHash", 64), textField("ipHash", 64), textField("signatureId", 15),
				&core.NumberField{Name: "startedAt"}, &core.NumberField{Name: "receivedAt"}, &core.NumberField{Name: "nextCollectAt"},
				&core.BoolField{Name: "pickedUp"},
			}, []string{
				"CREATE UNIQUE INDEX idx_bankid_request ON bankid_orders (flow, requestKey)",
				"CREATE UNIQUE INDEX idx_bankid_order ON bankid_orders (environment, orderHash) WHERE orderHash != ''",
				"CREATE INDEX idx_bankid_pending ON bankid_orders (status, nextCollectAt)",
			}},
			{"consent_signatures", []core.Field{
				textField("study", 100), textField("environment", 20), textField("orderHash", 64), textField("order", 15), textField("user", 15),
				textField("version", 15), textField("evidenceCipher", 4000000), textField("outcome", 30), &core.NumberField{Name: "receivedAt"},
			}, []string{"CREATE UNIQUE INDEX idx_signature_order ON consent_signatures (environment, orderHash)"}},
			{"participant_identities", []core.Field{
				textField("study", 100), textField("environment", 20), textField("user", 15), textField("identityHash", 64), textField("identityCipher", 3000),
			}, []string{
				"CREATE UNIQUE INDEX idx_identity_person ON participant_identities (study, environment, identityHash)",
				"CREATE UNIQUE INDEX idx_identity_user ON participant_identities (study, user)",
			}},
			{"consent_events", []core.Field{
				textField("study", 100), textField("user", 15), textField("signature", 15), textField("kind", 30), &core.NumberField{Name: "occurredAt"},
			}, nil},
			{"app_sessions", []core.Field{
				textField("study", 100), textField("environment", 20), textField("user", 15), textField("tokenHash", 64), &core.NumberField{Name: "expiresAt"}, &core.BoolField{Name: "revoked"},
			}, []string{"CREATE UNIQUE INDEX idx_app_session_token ON app_sessions (tokenHash)"}},
		}
		for _, def := range definitions {
			c := core.NewBaseCollection(def.name)
			c.Fields.Add(def.fields...)
			c.Fields.Add(&core.AutodateField{Name: "created", OnCreate: true}, &core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true})
			c.Indexes = def.indexes
			if err := app.Save(c); err != nil {
				return err
			}
		}
		users, err := app.FindCollectionByNameOrId("users")
		if err != nil {
			return err
		}
		users.Fields.Add(textField("consentVersion", 15), textField("consentSignature", 15), textField("consentStatus", 30))
		// Legacy consent remains historical data; it never grants signed-consent access.
		users.Fields.GetByName("email").(*core.EmailField).Required = false
		users.PasswordAuth.Enabled = false
		users.OAuth2.Enabled = false
		users.OTP.Enabled = false
		users.AuthToken.Duration = 7200
		if err := app.Save(users); err != nil {
			return err
		}
		// This app accesses participant data through authenticated custom endpoints.
		// Lock old public APIs as well as new evidence collections against bypasses.
		collections, err := app.FindAllCollections()
		if err != nil {
			return err
		}
		for _, c := range collections {
			if c.System {
				continue
			}
			c.ListRule, c.ViewRule, c.CreateRule, c.UpdateRule, c.DeleteRule = nil, nil, nil, nil, nil
			if err := app.Save(c); err != nil {
				return err
			}
		}
		return nil
	}, nil)
}

func textField(name string, max int) *core.TextField { return &core.TextField{Name: name, Max: max} }
