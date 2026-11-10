//go:build study_simulation

package testidentity

const Enabled = true

func (r *Run) Resolve(actor, role, expected string) (string, *Mapping, error) {
	if r == nil {
		return actor, nil, nil
	}
	for _, p := range r.file.Personas {
		if p.Role == role && p.ActorPersonalNumber == actor && (role != "guardian" || p.PersonalNumber == expected) {
			return p.PersonalNumber, &Mapping{r.ID(), p.ID, role, p.PersonalNumber}, nil
		}
	}
	return "", nil, ErrUnassigned
}

func (r *Run) GuardianActor(personal string) (string, error) {
	if r == nil {
		return personal, nil
	}
	for _, p := range r.file.Personas {
		if p.Role == "guardian" && p.PersonalNumber == personal {
			return p.ActorPersonalNumber, nil
		}
	}
	return "", ErrUnassigned
}
