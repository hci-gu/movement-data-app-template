// Package schema owns the application collections.
package schema

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

//go:embed base.json
var base []byte

func textField(name string, max int) *core.TextField {
	return &core.TextField{Name: name, Max: max}
}

func privateTextField(name string, max int) *core.TextField {
	field := textField(name, max)
	field.Hidden = true
	return field
}

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
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		for _, f := range d.Fields {
			if f.Type() != core.FieldTypeRelation {
				c.Fields.Add(f)
			}
		}
		c.Indexes = d.Indexes
		if c.IsAuth() {
			for _, name := range []string{"name", "avatar", "username", "created", "updated"} {
				c.Fields.RemoveByName(name)
			}
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
	users.Fields.Add(&core.TextField{Name: "personalNumber", Min: 12, Max: 12, Required: true})
	users.Fields.Add(&core.NumberField{Name: "guardianCount"})
	users.Indexes = append(users.Indexes, "CREATE UNIQUE INDEX idx_user_personal_number ON users (personalNumber)")
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
		{
			name: "consent_texts",
			fields: []core.Field{
				textField("version", 100), textField("title", 200), textField("text", 30000),
				textField("documentHash", 64), &core.BoolField{Name: "current"},
			},
			indexes: []string{
				"CREATE UNIQUE INDEX idx_consent_text_version ON consent_texts (version)",
				"CREATE UNIQUE INDEX idx_consent_text_current ON consent_texts (current) WHERE current = TRUE",
			},
		},
		{
			name: "authentications",
			fields: []core.Field{
				textField("attemptId", 100), textField("orderHash", 64),
				&core.RelationField{Name: "user", CollectionId: users.Id, MaxSelect: 1},
				textField("purpose", 20), textField("providerStatus", 30),
				textField("outcome", 30), textField("reason", 100),
				privateTextField("evidenceCipher", 4000000),
				&core.NumberField{Name: "startedAt"}, &core.NumberField{Name: "receivedAt"},
			},
			indexes: []string{
				"CREATE UNIQUE INDEX idx_auth_attempt ON authentications (attemptId)",
				"CREATE UNIQUE INDEX idx_auth_order ON authentications (orderHash) WHERE orderHash != ''",
			},
		},
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
	if err := addConsentCollections(app, users.Id); err != nil {
		return err
	}
	return SetAccessRules(app)
}

func addConsentCollections(app core.App, usersID string) error {
	auth, err := app.FindCollectionByNameOrId("authentications")
	if err != nil {
		return err
	}
	consents, err := app.FindCollectionByNameOrId("consent_texts")
	if err != nil {
		return err
	}
	acceptances := core.NewBaseCollection("consent_acceptances")
	acceptances.Fields.Add(
		&core.RelationField{Name: "user", CollectionId: usersID, MaxSelect: 1, Required: true},
		&core.RelationField{Name: "authentication", CollectionId: auth.Id, MaxSelect: 1, Required: true},
		&core.RelationField{Name: "version", CollectionId: consents.Id, MaxSelect: 1, Required: true},
		textField("documentHash", 64), textField("requestKey", 100), privateTextField("evidenceCipher", 4000000),
		&core.NumberField{Name: "acceptedAt", Required: true}, &core.NumberField{Name: "withdrawnAt"},
		&core.AutodateField{Name: "created", OnCreate: true},
	)
	acceptances.Indexes = []string{"CREATE UNIQUE INDEX idx_acceptance_request ON consent_acceptances (requestKey)"}
	if err := app.Save(acceptances); err != nil {
		return err
	}
	requests := core.NewBaseCollection("guardian_requests")
	id := requests.Fields.GetByName("id").(*core.TextField)
	id.Min, id.Max, id.Pattern, id.AutogeneratePattern = 7, 7, `^[0-9]{3}-[0-9]{3}$`, ""
	requests.Fields.Add(&core.RelationField{Name: "user", CollectionId: usersID, MaxSelect: 1, Required: true}, &core.NumberField{Name: "slot", Required: true}, privateTextField("personalNumber", 12))
	requests.Indexes = []string{"CREATE UNIQUE INDEX idx_guardian_person ON guardian_requests (user,personalNumber)", "CREATE UNIQUE INDEX idx_guardian_slot ON guardian_requests (user,slot)"}
	if err := app.Save(requests); err != nil {
		return err
	}
	approvals := core.NewBaseCollection("guardian_approvals")
	approvals.Fields.Add(
		&core.RelationField{Name: "request", CollectionId: requests.Id, MaxSelect: 1, Required: true},
		&core.RelationField{Name: "consent", CollectionId: acceptances.Id, MaxSelect: 1, Required: true},
		&core.RelationField{Name: "authentication", CollectionId: auth.Id, MaxSelect: 1, Required: true},
		textField("requestKey", 100), privateTextField("evidenceCipher", 4000000), &core.NumberField{Name: "acceptedAt", Required: true},
	)
	approvals.Indexes = []string{"CREATE UNIQUE INDEX idx_guardian_approval_request ON guardian_approvals (requestKey)", "CREATE UNIQUE INDEX idx_guardian_consent ON guardian_approvals (request,consent)"}
	return app.Save(approvals)
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
	expected := map[string]bool{"users": true, "answers": true, "dataUploads": true, "authentications": true, "consent_acceptances": true, "guardian_requests": true, "guardian_approvals": true, "consent_texts": true, "questionnaires": true, "questions": true, "questionOptions": true}
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
