package main

import (
	"app/internal/questionnaires"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

func TestMovementWindowAndSampleValidation(t *testing.T) {
	app := uploadApp(t)
	c, _ := app.FindCollectionByNameOrId("users")
	u := core.NewRecord(c)
	u.Set("personalNumber", "200501010020")
	u.SetPassword("fixture-password-123456")
	if err := app.Save(u); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	valid := DataItem{Value: Value{"0.25"}, DataType: "WALKING_DOUBLE_SUPPORT_PERCENTAGE", Unit: "PERCENT", PlatformType: "ios", DateFrom: "2025-01-15T12:00:00Z", DateTo: "2025-01-15T12:00:10Z"}
	if validateMovement(app, u.Id, []DataItem{valid}, now) == nil {
		t.Fatal("accepted without treatment")
	}
	seedTreatment(t, app, u.Id)
	if err := validateMovement(app, u.Id, []DataItem{valid}, now); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*DataItem){
		"unapproved metric":  func(p *DataItem) { p.DataType = "WALKING_STEADINESS" },
		"wrong unit":         func(p *DataItem) { p.Unit = "METER" },
		"wrong platform":     func(p *DataItem) { p.PlatformType = "android" },
		"nonfinite":          func(p *DataItem) { p.Value.NumericValue = "NaN" },
		"negative":           func(p *DataItem) { p.Value.NumericValue = "-1" },
		"old":                func(p *DataItem) { p.DateFrom = "2024-01-14T22:59:59Z" },
		"future":             func(p *DataItem) { p.DateTo = "2026-10-27T00:00:00Z" },
		"ambiguous timezone": func(p *DataItem) { p.DateFrom = "2025-01-15T12:00:00" },
		"reversed":           func(p *DataItem) { p.DateTo = "2025-01-14T00:00:00Z" },
	} {
		t.Run(name, func(t *testing.T) {
			sample := valid
			change(&sample)
			if validateMovement(app, u.Id, []DataItem{sample}, now) == nil {
				t.Fatal("accepted invalid sample")
			}
		})
	}
	from, to := questionnaires.MovementWindow(time.Date(2020, 2, 29, 0, 0, 0, 0, time.UTC), now)
	if from.Format(time.RFC3339) != "2019-02-27T23:00:00Z" || to.Format(time.RFC3339) != "2025-02-28T23:00:00Z" {
		t.Fatalf("leap window %s–%s", from, to)
	}
}
