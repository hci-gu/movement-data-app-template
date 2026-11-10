package study

import (
	"app/internal/testidentity"
	"errors"
	"github.com/pocketbase/pocketbase/core"
)

// BindSimulationDatabase isolates simulation records from ordinary deployments.
func BindSimulationDatabase(app core.App, run *testidentity.Run) error {
	return app.RunInTransaction(func(tx core.App) error {
		rows, err := tx.FindAllRecords("simulation_state")
		if err != nil {
			return err
		}
		if len(rows) != 0 {
			if len(rows) != 1 || run == nil || rows[0].GetString("runId") != run.ID() {
				return errors.New("database belongs to a different simulation configuration; start a fresh test run")
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
			for _, name := range []string{"users", "signatures", "dataUploads"} {
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
		for _, name := range []string{"users", "signatures", "signingRequests", "answers", "dataUploads"} {
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
	if user.GetString("simulationRun") != s.simulation.ID() {
		return false
	}
	if s.simulation == nil {
		return true
	}
	r, err := s.App.FindRecordById("test_identity_mappings", user.GetString("simulationPersona"))
	return err == nil && r.GetString("revision") == user.GetString("simulationRevision") && r.GetString("role") == "participant" && r.GetString("testPersonalNumber") == user.GetString("personalNumber")
}

func (s *Service) participantVisibleText(text string) string {
	if s.simulation == nil {
		return text
	}
	return s.simulation.Label() + "\nThis is a workflow test using a fictional study identity. Your real BankID identity is retained in the authentication evidence.\n\n" + text
}

func (s *Service) guardianText(user, doc *core.Record) string {
	return s.participantVisibleText(guardianVisibleText(user, doc))
}
