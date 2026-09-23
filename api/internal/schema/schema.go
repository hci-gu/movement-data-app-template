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
			name: "signatures",
			fields: []core.Field{
				textField("attemptId", 100), textField("orderHash", 64),
				&core.RelationField{Name: "user", CollectionId: users.Id, MaxSelect: 1},
				textField("version", 15), textField("purpose", 10), textField("providerStatus", 30),
				textField("outcome", 30), textField("reason", 100),
				privateTextField("evidenceCipher", 4000000),
				&core.NumberField{Name: "startedAt"}, &core.NumberField{Name: "receivedAt"},
				&core.NumberField{Name: "withdrawnAt"},
			},
			indexes: []string{
				"CREATE UNIQUE INDEX idx_signature_attempt ON signatures (attemptId)",
				"CREATE UNIQUE INDEX idx_signatures_order ON signatures (orderHash) WHERE orderHash != ''",
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
		if c.Name == "signingRequests" {
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
