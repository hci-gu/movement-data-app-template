package study

import (
	"app/internal/testidentity"
	"strings"
	"time"

	validation "github.com/pocketbase/ozzo-validation/v4"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func mappingPersona(r *core.Record) testidentity.Persona {
	return testidentity.Persona{ID: r.Id, Role: r.GetString("role"), ActorPersonalNumber: r.GetString("realPersonalNumber"), PersonalNumber: r.GetString("testPersonalNumber"), Revision: r.GetString("revision")}
}

func (s *Service) mappingSnapshot(app core.App) (*testidentity.Run, error) {
	if s.simulation == nil {
		return nil, nil
	}
	rows, err := app.FindAllRecords("test_identity_mappings")
	if err != nil {
		return nil, err
	}
	file := testidentity.File{RunID: s.simulation.ID()}
	for _, r := range rows {
		file.Personas = append(file.Personas, mappingPersona(r))
	}
	return testidentity.New(file)
}

func (s *Service) assignedGuardian(app core.App, personal string) error {
	run, err := s.mappingSnapshot(app)
	if err != nil {
		return err
	}
	_, err = run.GuardianActor(personal)
	return err
}

// Collection rules restrict all reads/writes to PocketBase superusers. Revisions
// are server-owned: edits or deletion immediately revoke the previous mapping.
func (s *Service) registerSimulationMappings() {
	validate := func(e *core.RecordEvent) error {
		if s.simulation == nil {
			return apis.NewBadRequestError("Test mappings are disabled in this deployment.", nil)
		}
		fieldErrors := validation.Errors{}
		for _, field := range []string{"realPersonalNumber", "testPersonalNumber"} {
			value := strings.TrimSpace(e.Record.GetString(field))
			if len(value) == 13 && value[8] == '-' {
				value = value[:8] + value[9:]
			}
			e.Record.Set(field, value)
			if !testidentity.ValidPersonal(value, time.Now()) {
				fieldErrors[field] = validation.NewError("validation_personal_number", "Use YYYYMMDDXXXX or YYYYMMDD-XXXX with a valid birth date that is not in the future.")
			}
		}
		if role := e.Record.GetString("role"); role != "participant" && role != "guardian" {
			fieldErrors["role"] = validation.NewError("validation_role", "Select participant or guardian.")
		}
		if len(fieldErrors) != 0 {
			return apis.NewBadRequestError("Check the highlighted mapping fields.", fieldErrors)
		}
		e.Record.Set("revision", randomSecret())
		rows, err := e.App.FindAllRecords("test_identity_mappings")
		if err != nil {
			return err
		}
		file := testidentity.File{RunID: s.simulation.ID()}
		for _, r := range rows {
			if r.Id != e.Record.Id {
				file.Personas = append(file.Personas, mappingPersona(r))
			}
		}
		file.Personas = append(file.Personas, mappingPersona(e.Record))
		if _, err = testidentity.New(file); err != nil {
			return apis.NewBadRequestError("Cannot save test identity mapping: "+err.Error(), nil)
		}
		return e.Next()
	}
	s.App.OnRecordCreate("test_identity_mappings").BindFunc(validate)
	s.App.OnRecordUpdate("test_identity_mappings").BindFunc(validate)
}
