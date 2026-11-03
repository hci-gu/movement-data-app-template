package questionnaires

import (
	"time"
	_ "time/tzdata"
)

// The study dates are Swedish calendar dates. Clamp leap-day anniversaries
// rather than letting AddDate roll February 29 into March.
func Anniversary(date time.Time, years int) time.Time {
	year, month, day := date.Date()
	last := time.Date(year+years, month+1, 0, 0, 0, 0, 0, date.Location()).Day()
	if day > last {
		day = last
	}
	return time.Date(year+years, month, day, 0, 0, 0, 0, date.Location())
}

func MovementWindow(treatment, now time.Time) (time.Time, time.Time) {
	location, _ := time.LoadLocation("Europe/Stockholm")
	treatment = time.Date(treatment.Year(), treatment.Month(), treatment.Day(), 0, 0, 0, 0, location)
	from, to := Anniversary(treatment, -1), Anniversary(treatment, 5).AddDate(0, 0, 1)
	if now.Before(to) {
		to = now
	}
	return from.UTC(), to.UTC()
}
