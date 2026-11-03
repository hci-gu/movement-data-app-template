package main

import (
	"app/internal/questionnaires"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

var movementUnits = map[string]string{
	"STEPS": "COUNT", "WALKING_STEP_LENGTH": "METER",
	"DISTANCE_WALKING_RUNNING": "METER", "WALKING_SPEED": "METERS_PER_SECOND",
	"WALKING_ASYMMETRY_PERCENTAGE": "PERCENT", "WALKING_DOUBLE_SUPPORT_PERCENTAGE": "PERCENT",
}

func validateMovement(app core.App, user string, samples []DataItem, now time.Time) error {
	date, err := questionnaires.TreatmentDate(app, user, now)
	if err != nil {
		return httpError(409, "Svara på frågorna om behandling och behandlingsdatum först.")
	}
	from, to := questionnaires.MovementWindow(date, now)
	for i, sample := range samples {
		unit, allowed := movementUnits[sample.DataType]
		value, valueErr := strconv.ParseFloat(sample.Value.NumericValue, 64)
		start, startErr := time.Parse(time.RFC3339Nano, sample.DateFrom)
		end, endErr := time.Parse(time.RFC3339Nano, sample.DateTo)
		if !allowed || sample.Unit != unit || sample.PlatformType != "ios" || valueErr != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || startErr != nil || endErr != nil || end.Before(start) || start.Before(from) || end.After(to) {
			return httpError(400, fmt.Sprintf("Ogiltig rörelsedata i mätning %d. Kontrollera datatyp, enhet och insamlingsperiod.", i+1))
		}
	}
	return nil
}
