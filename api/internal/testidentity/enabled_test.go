//go:build study_simulation

package testidentity

import "testing"

func TestAssignmentsAreImmutableAndRoleBound(t *testing.T) {
	file := File{RunID: "run", Personas: []Persona{
		{ID: "child", Role: "participant", PersonalNumber: "201201010004", ActorPersonalNumber: "198001012384"},
		{ID: "parent", Role: "guardian", PersonalNumber: "197001010001", ActorPersonalNumber: "198201012384"},
	}}
	run, err := New(file)
	if err != nil {
		t.Fatal(err)
	}
	file.Personas[0].PersonalNumber = "200101010004"
	number, _, err := run.Resolve("198001012384", "participant", "")
	if err != nil || number != "201201010004" {
		t.Fatal("configuration was not snapshotted")
	}
	if _, _, err := run.Resolve("198001012384", "guardian", "197001010001"); err == nil {
		t.Fatal("allowlisted participant became another tester's guardian")
	}
	if _, _, err := run.Resolve("198201012384", "guardian", "197201010002"); err == nil {
		t.Fatal("guardian role crossed invitations")
	}
	if _, _, err := run.Resolve("198201012384", "participant", ""); err == nil {
		t.Fatal("guardian acquired participant role")
	}
}

func TestInvalidAssignmentsFailClosed(t *testing.T) {
	for _, kind := range []string{"duplicate-id", "duplicate-number", "duplicate-actor", "real-equals-fake", "invalid-date", "future-date", "wrong-role"} {
		t.Run(kind, func(t *testing.T) {
			file := File{RunID: "run", Personas: []Persona{
				{ID: "one", Role: "participant", PersonalNumber: "200101010004", ActorPersonalNumber: "198001012384"},
				{ID: "two", Role: "guardian", PersonalNumber: "197001010001", ActorPersonalNumber: "198201012384"},
			}}
			switch kind {
			case "duplicate-id":
				file.Personas[1].ID = "one"
			case "duplicate-number":
				file.Personas[1].PersonalNumber = file.Personas[0].PersonalNumber
			case "duplicate-actor":
				file.Personas[1].Role = "participant"
				file.Personas[1].ActorPersonalNumber = file.Personas[0].ActorPersonalNumber
			case "real-equals-fake":
				file.Personas[1].PersonalNumber = file.Personas[0].ActorPersonalNumber
			case "invalid-date":
				file.Personas[0].PersonalNumber = "200102300004"
			case "future-date":
				file.Personas[0].PersonalNumber = "299901010004"
			case "wrong-role":
				file.Personas[0].Role = "admin"
			}
			if _, err := New(file); err == nil {
				t.Fatal("invalid assignment accepted")
			}
		})
	}
}
