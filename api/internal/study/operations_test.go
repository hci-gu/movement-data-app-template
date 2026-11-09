package study

import "testing"

func TestDeploymentConsentInitialization(t *testing.T) {
	s, _, _ := testService(t)
	first, err := publishConsent(s.App, "technical-v1", "Technical test", "Synthetic consent.", true)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := publishConsent(s.App, "technical-v1", "Technical test", "Synthetic consent.", true)
	if err != nil || repeated.Id != first.Id {
		t.Fatalf("restart must reuse the same document: %v", err)
	}
	for _, changed := range []struct{ title, text string }{
		{"Changed title", "Synthetic consent."},
		{"Technical test", "Changed text."},
	} {
		if _, err := publishConsent(s.App, "technical-v1", changed.title, changed.text, true); err == nil {
			t.Fatal("initialization must reject changed content for an existing version")
		}
	}
	if _, err := PublishConsent(s.App, "technical-v1", "Technical test", "Synthetic consent."); err == nil {
		t.Fatal("normal publication must still reject duplicate versions")
	}
	newer, err := PublishConsent(s.App, "technical-v2", "Updated test", "New synthetic consent.")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := publishConsent(s.App, "technical-v1", "Technical test", "Synthetic consent.", true); err != nil {
		t.Fatal(err)
	}
	current, err := currentDocument(s.App)
	if err != nil || current.Id != newer.Id {
		t.Fatalf("restart must preserve the newer current document: %v", err)
	}
	all, err := s.App.FindAllRecords("consent_texts")
	if err != nil || len(all) != 3 { // The initial fixture plus two technical versions.
		t.Fatalf("unexpected consent count after retries: %d, %v", len(all), err)
	}
}
