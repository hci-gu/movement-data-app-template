package study

import (
	"app/internal/questionnaires"
	"testing"
	"time"
)

func TestStudyAgeThresholds(t *testing.T) {
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		id    string
		years int
		want  bool
	}{
		{"201610260020", 10, true}, {"201610270020", 10, false},
		{"201110260020", 15, true}, {"201110270020", 15, false},
		{"200810260020", 18, true}, {"200810270020", 18, false},
		{"199510260020", 31, true}, {"199510270020", 31, false},
		{"201113010020", 15, false}, {"invalid", 15, false},
	} {
		if got := atLeastAge(tc.id, now, tc.years); got != tc.want {
			t.Errorf("age %s threshold %d: %v", tc.id, tc.years, got)
		}
	}
}

func TestScoliosisSubmissionAndMovementWindow(t *testing.T) {
	app, _, handler := questionnaireApp(t)
	user, token := questionnaireUser(t, app, "200501010020", true)
	path := "/api/collections/answers/records"
	for _, answers := range []map[string]any{
		{}, {questionnaires.TreatmentID: "surgery"},
		{questionnaires.TreatmentID: "surgery", questionnaires.BraceDateID: "2025-01-15"},
		{questionnaires.TreatmentID: "surgery", questionnaires.SurgeryDateID: "2019-12-31"},
		{questionnaires.TreatmentID: "surgery", questionnaires.SurgeryDateID: "2025-02-30"},
	} {
		w := questionnaireRequest(handler, "POST", path, token, map[string]any{"user": user.Id, "questionnaire": questionnaires.ScoliosisID, "answers": answers})
		if w.Code != 400 {
			t.Fatalf("invalid study response accepted: %d %s", w.Code, w.Body.String())
		}
	}
	if w := questionnaireRequest(handler, "GET", "/api/movement/window", token, nil); w.Code != 409 {
		t.Fatalf("window without treatment: %d", w.Code)
	}
	answers := map[string]any{questionnaires.TreatmentID: "surgery", questionnaires.SurgeryDateID: "2025-01-15"}
	w := questionnaireRequest(handler, "POST", path, token, map[string]any{"user": user.Id, "questionnaire": questionnaires.ScoliosisID, "answers": answers})
	if w.Code != 200 {
		t.Fatalf("valid response: %d %s", w.Code, w.Body.String())
	}
	w = questionnaireRequest(handler, "GET", "/api/movement/window", token, nil)
	if w.Code != 200 {
		t.Fatalf("window: %d %s", w.Code, w.Body.String())
	}
	_, child := questionnaireUser(t, app, "201301010020", true)
	if w := questionnaireRequest(handler, "GET", "/api/movement/window", child, nil); w.Code != 403 {
		t.Fatalf("child bypassed guardians: %d", w.Code)
	}
	_, unsigned := questionnaireUser(t, app, "200501010022", false)
	if w := questionnaireRequest(handler, "GET", "/api/movement/window", unsigned, nil); w.Code != 403 {
		t.Fatalf("window without consent: %d", w.Code)
	}
}
