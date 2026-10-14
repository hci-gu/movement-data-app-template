package questionnaires

import (
	"testing"
	"time"
)

func TestAudienceChainAndConfiguration(t *testing.T) {
	d := Definition{Questions: []Question{{ID: "a", Type: "text", Audience: "female"}, {ID: "b", Type: "text", Dependency: "a", DependencyValue: "yes"}, {ID: "c", Type: "text", Dependency: "b", DependencyValue: "yes"}}}
	if err := d.Check(); err != nil {
		t.Fatal(err)
	}
	for _, gender := range []string{"male", "unknown"} {
		for id, visible := range d.Visible(map[string]any{"a": "yes", "b": "yes", "c": "x"}, gender) {
			if visible {
				t.Fatal(gender, id)
			}
		}
	}
	if len(d.Validate(map[string]any{"c": "x"}, "female", time.Now())) != 1 {
		t.Fatal("hidden parent chain accepted")
	}
	d.Questions[0].Dependency = "missing"
	d.Questions[0].DependencyValue = "yes"
	if d.Check() == nil {
		t.Fatal("missing dependency accepted")
	}
	d.Questions[0].Dependency = "c"
	if d.Check() == nil {
		t.Fatal("cycle accepted")
	}
}
func TestDateValidationAndJSONTypes(t *testing.T) {
	no := false
	q := Question{Type: "date", Settings: Settings{Precision: "month", AllowFuture: &no}}
	now := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	for _, v := range []any{"2026-13", "2026-11", "2026-01-01", "0000-01", false, map[string]any{"year": 2026}} {
		if q.Validate(v, now) == "" {
			t.Fatal("accepted", v)
		}
	}
	if q.Validate("2026-10", now) != "" {
		t.Fatal("current month rejected")
	}
	q.Settings.Precision = "day"
	for _, v := range []string{"2026-02-29", "2026-10-15", "2026-1-01"} {
		if q.Validate(v, now) == "" {
			t.Fatal("accepted", v)
		}
	}
	if q.Validate("2024-02-29", now) != "" {
		t.Fatal("leap day rejected")
	}
}
