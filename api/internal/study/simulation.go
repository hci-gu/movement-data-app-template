package study

import (
	"app/internal/testidentity"
	"errors"
	"github.com/pocketbase/pocketbase/core"
)

// BindSimulationDatabase isolates test data from ordinary deployments.
func BindSimulationDatabase(app core.App, run *testidentity.Run) error {
	return app.RunInTransaction(func(tx core.App) error {
		rows, err := tx.FindAllRecords("simulation_state")
		if err != nil {
			return err
		}
		if len(rows) != 0 {
			if len(rows) != 1 || run == nil || rows[0].GetString("runId") != run.ID() {
				return errors.New("database belongs to a different simulation deployment")
			}
			return nil
		}
		if run == nil {
			mappings, err := tx.FindAllRecords("test_identity_mappings")
			if err != nil {
				return err
			}
			if len(mappings) != 0 {
				return errors.New("test identity mappings require a simulation deployment")
			}
			for _, name := range []string{"users", "authentications", "consent_acceptances", "guardian_approvals", "dataUploads"} {
				rows, err := tx.FindRecordsByFilter(name, "simulationRun != ''", "", 1, 0)
				if err != nil {
					return err
				}
				if len(rows) != 0 {
					return errors.New("simulation records cannot be served without their test configuration")
				}
			}
			return nil
		}
		for _, name := range []string{"users", "authentications", "consent_acceptances", "guardian_requests", "guardian_approvals", "answers", "dataUploads"} {
			rows, err := tx.FindRecordsByFilter(name, "id != ''", "", 1, 0)
			if err != nil {
				return err
			}
			if len(rows) != 0 {
				return errors.New("simulation requires a fresh database without participants, evidence, answers or uploads")
			}
		}
		r, err := newRecord(tx, "simulation_state")
		if err != nil {
			return err
		}
		r.Set("runId", run.ID())
		return tx.Save(r)
	})
}

func (s *Service) simulationHeaders(e *core.RequestEvent) error {
	if s.simulation != nil {
		e.Response.Header().Set("X-Study-Simulation", s.simulation.ID())
	}
	return e.Next()
}

func (s *Service) validSimulationUser(user *core.Record) bool {
	return user.GetString("simulationRun") == s.simulation.ID()
}

// Recheck the persisted mapping when granting sessions or accepting/verifying
// consent. The provider result is always the original authenticated identity.
func (s *Service) validSimulationAuthentication(app core.App, id, role, personal, userID string) bool {
	if s.simulation == nil {
		return true
	}
	run, err := s.mappingSnapshot(app)
	if err != nil {
		return false
	}
	r, err := app.FindRecordById("authentications", id)
	if err != nil || r.GetString("simulationRun") != s.simulation.ID() || r.GetString("outcome") != "accepted" || r.GetString("purpose") != role || r.GetString("user") != userID {
		return false
	}
	var evidence AuthenticationEvidence
	return s.Config.Open("authentication:"+id, r.GetString("evidenceCipher"), &evidence) == nil && evidence.Provider == "kontoapi" && evidence.SimulationRun == s.simulation.ID() && evidence.Result != nil && evidence.Result.Status == "complete" && run.Check(evidence.Result.PersonalNumber, role, personal, evidence.Simulation)
}

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

func (s *Service) guardianRevision(app core.App, personal string) (string, error) {
	if s.simulation == nil {
		return "", nil
	}
	run, err := s.mappingSnapshot(app)
	if err != nil {
		return "", err
	}
	for _, p := range run.Personas() {
		if p.Role == "guardian" && p.PersonalNumber == personal {
			return p.Revision, nil
		}
	}
	return "", testidentity.ErrUnassigned
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
