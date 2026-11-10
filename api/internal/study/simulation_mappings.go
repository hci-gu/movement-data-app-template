package study

import (
	"app/internal/testidentity"
	"errors"
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
			return errors.New("test mappings are disabled in this deployment")
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
			return err
		}
		return e.Next()
	}
	s.App.OnRecordCreate("test_identity_mappings").BindFunc(validate)
	s.App.OnRecordUpdate("test_identity_mappings").BindFunc(validate)
}
