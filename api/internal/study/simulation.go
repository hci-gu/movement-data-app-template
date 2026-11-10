package study

import (
	"github.com/pocketbase/pocketbase/core"
)

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

func (s *Service) guardianText(user, doc *core.Record) string {
	return guardianVisibleText(user, doc)
}
