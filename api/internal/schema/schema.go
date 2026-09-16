// Package schema owns the eight application collections.
package schema

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

//go:embed base.json
var base []byte

func Text(name string, max int) *core.TextField    { return &core.TextField{Name: name, Max: max} }
func private(name string, max int) *core.TextField { f := Text(name, max); f.Hidden = true; return f }

// CreateBase creates the retained collections directly, resolving cyclic question relations in a second pass.
func CreateBase(app core.App) error {
	var defs []struct {
		ID      string          `json:"id"`
		Name    string          `json:"name"`
		Type    string          `json:"type"`
		Fields  core.FieldsList `json:"fields"`
		Indexes []string        `json:"indexes"`
	}
	if err := json.Unmarshal(base, &defs); err != nil {
		return err
	}
	for _, d := range defs {
		var c *core.Collection
		if d.Type == "auth" {
			c = core.NewAuthCollection(d.Name, d.ID)
		} else {
			c = core.NewBaseCollection(d.Name, d.ID)
		}
		if existing, err := app.FindCollectionByNameOrId(d.Name); err == nil {
			c = existing
		}
		for _, f := range d.Fields {
			if f.Type() != core.FieldTypeRelation {
				c.Fields.Add(f)
			}
		}
		c.Indexes = d.Indexes
		if c.IsAuth() {
			c.PasswordAuth.Enabled = false
			c.OAuth2.Enabled = false
			c.OTP.Enabled = false
			c.AuthToken.Duration = 7200
			c.Fields.GetByName("email").(*core.EmailField).Required = false
		}
		if err := app.Save(c); err != nil {
			return err
		}
	}
	for _, d := range defs {
		c, err := app.FindCollectionByNameOrId(d.ID)
		if err != nil {
			return err
		}
		for _, f := range d.Fields {
			if f.Type() == core.FieldTypeRelation {
				c.Fields.Add(f)
			}
		}
		if err := app.Save(c); err != nil {
			return err
		}
	}
	return nil
}

func AddStudyFields(app core.App) error {
	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}
	users.Fields.Add(Text("study", 100), Text("environment", 20), &core.BoolField{Name: "active"},
		private("invitationHash", 64), private("invitationCipher", 3000), private("expectedCipher", 3000), &core.NumberField{Name: "invitationExpiresAt", Hidden: true},
		private("identityHash", 64), private("identityCipher", 3000), Text("consentVersion", 15), Text("consentSignature", 15), Text("consentStatus", 30),
		&core.JSONField{Name: "metadata", MaxSize: 2000000, Hidden: true}, &core.NumberField{Name: "withdrawnAt"},
		Text("invitationCode", 100), Text("expectedPersonalNumber", 12), &core.NumberField{Name: "validityHours", OnlyInt: true})
	users.Indexes = append(users.Indexes,
		"CREATE UNIQUE INDEX idx_user_identity ON users (study, environment, identityHash) WHERE identityHash != ''",
		"CREATE UNIQUE INDEX idx_user_invitation ON users (invitationHash) WHERE invitationHash != ''")
	users.Fields.GetByName("username").(*core.TextField).Help = "Participant ID. Creating a record issues an invitation; using an existing inactive participant ID reissues it."
	users.Fields.GetByName("password").(*core.PasswordField).Help = "The PocketBase form requires a value: use Generate and set random password. The backend ignores this input; participants sign in only with BankID."
	users.Fields.GetByName("expectedPersonalNumber").(*core.TextField).Help = "Optional expected signer, 12 digits. Encrypted on save; this input is cleared."
	users.Fields.GetByName("invitationCode").(*core.TextField).Help = "Generated on save. Reopen this record to copy the code while unused and unexpired."
	users.Fields.GetByName("validityHours").(*core.NumberField).Help = "Invitation lifetime, 1–2160 hours. Blank or zero means 168 hours."
	for index, name := range []string{"username", "validityHours", "expectedPersonalNumber", "invitationCode"} {
		users.Fields.AddAt(index+1, users.Fields.GetByName(name))
	}
	users.PasswordAuth.Enabled = false
	users.OAuth2.Enabled = false
	users.OTP.Enabled = false
	users.AuthToken.Duration = 7200
	users.Fields.GetByName("email").(*core.EmailField).Required = false
	if err := app.Save(users); err != nil {
		return err
	}
	defs := []struct {
		name    string
		fields  []core.Field
		indexes []string
	}{
		{"consent_texts", []core.Field{Text("study", 100), Text("version", 100), Text("title", 200), Text("text", 30000), Text("documentHash", 64), &core.BoolField{Name: "current"}}, []string{"CREATE UNIQUE INDEX idx_consent_text_version ON consent_texts (study, version)", "CREATE UNIQUE INDEX idx_consent_text_current ON consent_texts (study) WHERE current = TRUE"}},
		{"signatures", []core.Field{Text("study", 100), Text("environment", 20), Text("attemptId", 100), Text("orderHash", 64), Text("user", 15), Text("version", 15), Text("purpose", 10), Text("providerStatus", 30), Text("outcome", 30), Text("reason", 100), private("evidenceCipher", 4000000), &core.NumberField{Name: "startedAt"}, &core.NumberField{Name: "receivedAt"}, &core.NumberField{Name: "withdrawnAt"}}, []string{"CREATE UNIQUE INDEX idx_signature_attempt ON signatures (attemptId)", "CREATE UNIQUE INDEX idx_signatures_order ON signatures (environment, orderHash) WHERE orderHash != ''"}},
	}
	for _, d := range defs {
		c := core.NewBaseCollection(d.name)
		c.Fields.Add(d.fields...)
		c.Fields.Add(&core.AutodateField{Name: "created", OnCreate: true}, &core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true})
		c.Indexes = d.indexes
		if err := app.Save(c); err != nil {
			return err
		}
	}
	return SetAccessRules(app)
}

func SetAccessRules(app core.App) error {
	cs, err := app.FindAllCollections()
	if err != nil {
		return err
	}
	for _, c := range cs {
		if c.System {
			continue
		}
		c.ListRule = nil
		c.ViewRule = nil
		c.CreateRule = nil
		c.UpdateRule = nil
		c.DeleteRule = nil
		switch c.Name {
		case "questionnaires", "questions", "questionOptions":
			c.ListRule = types.Pointer(`@request.auth.id != ""`)
			c.ViewRule = c.ListRule
		case "answers":
			c.ListRule = types.Pointer(`user = @request.auth.id`)
			c.ViewRule = c.ListRule
			c.UpdateRule = c.ListRule
			c.DeleteRule = c.ListRule
			c.CreateRule = types.Pointer(`@request.auth.id != "" && user = @request.auth.id`)
		}
		if err := app.Save(c); err != nil {
			return err
		}
	}
	return nil
}

// Verify rejects partial conversions and unexpected application collections.
func Verify(app core.App) error {
	expected := map[string]bool{"users": true, "answers": true, "dataUploads": true, "signatures": true, "consent_texts": true, "questionnaires": true, "questions": true, "questionOptions": true}
	cs, err := app.FindAllCollections()
	if err != nil {
		return err
	}
	for _, c := range cs {
		if c.System {
			continue
		}
		if !expected[c.Name] {
			return fmt.Errorf("unexpected application collection %s", c.Name)
		}
		delete(expected, c.Name)
	}
	if len(expected) != 0 {
		return fmt.Errorf("missing application collections: %v", expected)
	}
	return nil
}
