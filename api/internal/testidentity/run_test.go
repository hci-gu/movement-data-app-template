package testidentity

import "testing"

func TestDefaultIsPassThroughAndConfigurationIsExplicit(t *testing.T) {
	var run *Run
	identity, mapping, err := run.Resolve("198001012384", "participant", "")
	if err != nil || identity != "198001012384" || mapping != nil {
		t.Fatal("normal identity changed")
	}
	if _, err := Load("missing", "production"); err == nil {
		t.Fatal("production enabled simulation")
	}
	if _, err := Load("", "simulation"); err == nil {
		t.Fatal("missing configuration accepted")
	}
	if !Enabled {
		if _, err := New(File{RunID: "run"}); err == nil {
			t.Fatal("normal binary constructed simulation")
		}
		if _, _, err := (&Run{}).Resolve("198001012384", "participant", ""); err == nil {
			t.Fatal("normal binary resolved simulation")
		}
	}
}
