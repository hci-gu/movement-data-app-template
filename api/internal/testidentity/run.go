// Package testidentity resolves explicitly assigned test personas only after a
// provider has authenticated a real actor. It has no provider or database dependency.
package testidentity

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

type Persona struct {
	Revision            string `json:"revision"`
	ID                  string `json:"id"`
	Role                string `json:"role"`
	PersonalNumber      string `json:"personalNumber"`
	ActorPersonalNumber string `json:"actorPersonalNumber"`
}

type File struct {
	RunID    string    `json:"runId"`
	Personas []Persona `json:"personas"`
}

// Run is an immutable snapshot of the current deployment's mappings.
type Run struct {
	file File
}

// Mapping accompanies the original, unmodified provider evidence.
type Mapping struct {
	Revision       string `json:"revision"`
	RunID          string `json:"runId"`
	PersonaID      string `json:"personaId"`
	Role           string `json:"role"`
	PersonalNumber string `json:"personalNumber"`
}

var idPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,48}$`)
var personalPattern = regexp.MustCompile(`^[0-9]{12}$`)
var ErrUnassigned = errors.New("authenticated identity has no assignment for this test role")

func ValidPersonal(value string, now time.Time) bool {
	if !personalPattern.MatchString(value) {
		return false
	}
	birth, err := time.Parse("20060102", value[:8])
	return err == nil && !birth.After(now.UTC())
}

func New(file File) (*Run, error) {
	if !Enabled {
		return nil, errors.New("this binary has no test identity support; build with -tags study_simulation")
	}
	if !idPattern.MatchString(file.RunID) || len(file.Personas) > 50 {
		return nil, errors.New("simulation requires a runId and at most 50 mappings")
	}
	ids, numbers, actors := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, p := range file.Personas {
		if !idPattern.MatchString(p.ID) || (p.Role != "participant" && p.Role != "guardian") || !ValidPersonal(p.PersonalNumber, time.Now()) || !ValidPersonal(p.ActorPersonalNumber, time.Now()) {
			return nil, errors.New("invalid persona: use an ID, participant/guardian role and two valid 12-digit identities")
		}
		if ids[p.ID] || numbers[p.PersonalNumber] || (p.Role == "participant" && actors[p.ActorPersonalNumber]) {
			return nil, errors.New("persona IDs and effective identities must be unique; each actor can have one participant persona per run")
		}
		ids[p.ID], numbers[p.PersonalNumber] = true, true
		if p.Role == "participant" {
			actors[p.ActorPersonalNumber] = true
		}
	}
	for _, p := range file.Personas {
		if numbers[p.ActorPersonalNumber] {
			return nil, errors.New("test identities must differ from every real actor identity")
		}
	}
	file.Personas = append([]Persona(nil), file.Personas...)
	return &Run{file: file}, nil
}

func Load(id, deployment string) (*Run, error) {
	if id == "" && deployment != "simulation" {
		return nil, nil
	}
	if !Enabled {
		return nil, errors.New("simulation configuration is forbidden in this binary")
	}
	if id == "" || deployment != "simulation" {
		return nil, errors.New("set both STUDY_DEPLOYMENT=simulation and STUDY_SIMULATION_ID")
	}
	return New(File{RunID: id})
}

func (r *Run) ID() string {
	if r == nil {
		return ""
	}
	return r.file.RunID
}
func (r *Run) Label() string {
	if r == nil {
		return ""
	}
	return "TEST SIMULATION"
}
func (r *Run) Personas() []Persona { return append([]Persona(nil), r.file.Personas...) }
func (m *Mapping) Label() string {
	if m == nil {
		return ""
	}
	return "TEST SIMULATION · " + m.PersonaID
}

// Check binds stored mapping metadata back to the actual authenticated actor.
func (r *Run) Check(actor, role, expected string, m *Mapping) bool {
	if r == nil {
		return m == nil && actor == expected
	}
	personal, resolved, err := r.Resolve(actor, role, expected)
	return err == nil && personal == expected && m != nil && resolved != nil && *m == *resolved
}

func Role(purpose string) string {
	if strings.EqualFold(purpose, "guardian") {
		return "guardian"
	}
	return "participant"
}
