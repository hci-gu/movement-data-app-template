//go:build !study_simulation

package testidentity

const Enabled = false

func (r *Run) Resolve(actor, role, expected string) (string, *Mapping, error) {
	if r != nil {
		return "", nil, ErrUnassigned
	}
	return actor, nil, nil
}
func (r *Run) GuardianActor(personal string) (string, error) {
	if r != nil {
		return "", ErrUnassigned
	}
	return personal, nil
}
