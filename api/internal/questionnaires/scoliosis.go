package questionnaires

import (
	"errors"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// Stable identifiers for the instrument in context/scolios_formulär.docx.
const ScoliosisID = "scoliosis000001"
const TreatmentID = "scoliquestion02"
const SurgeryDateID = "scoliquestion03"
const BraceDateID = "scoliquestion04"

// Treatment is required to restrict Health collection to the protocol window.
// This is a study rule, not a widget setting. Other questions remain optional.
func ValidateTreatment(answers map[string]any, now time.Time) map[string]string {
	errors := map[string]string{}
	var dateID string
	switch answers[TreatmentID] {
	case "surgery":
		dateID = SurgeryDateID
	case "full_time", "night":
		dateID = BraceDateID
	default:
		errors[TreatmentID] = "Välj din behandling för att bestämma perioden för rörelsedata."
		return errors
	}
	value, _ := answers[dateID].(string)
	date, err := time.Parse("2006-01-02", value)
	if err != nil || value > now.Format("2006-01-02") || date.Year() < 2020 || date.Year() > 2027 {
		errors[dateID] = "Ange behandlingsdatum mellan 2020 och 2027, senast i dag."
	}
	return errors
}

func TreatmentDate(app core.App, userID string, now time.Time) (time.Time, error) {
	r, err := app.FindFirstRecordByFilter("answers", "user={:user} && questionnaire={:form}", dbx.Params{"user": userID, "form": ScoliosisID})
	if err != nil {
		return time.Time{}, err
	}
	answers := map[string]any{}
	if err := r.UnmarshalJSONField("answers", &answers); err != nil {
		return time.Time{}, err
	}
	if len(ValidateTreatment(answers, now)) != 0 {
		return time.Time{}, errors.New("treatment date missing or invalid")
	}
	key := BraceDateID
	if answers[TreatmentID] == "surgery" {
		key = SurgeryDateID
	}
	return time.Parse("2006-01-02", answers[key].(string))
}
