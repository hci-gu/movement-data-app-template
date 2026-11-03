package migrations

import (
	"app/internal/questionnaires"
	"fmt"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() { m.Register(seedScoliosis, nil) }

// Source: context/scolios_formulär.docx. No questionnaire counts or invented
// numeric limits are exposed to participants. Definitions are immutable seeds.
func seedScoliosis(app core.App) error {
	create := func(collection, id string, fields map[string]any) error {
		c, err := app.FindCollectionByNameOrId(collection)
		if err != nil {
			return err
		}
		r := core.NewRecord(c)
		r.Id = id
		for key, value := range fields {
			r.Set(key, value)
		}
		return app.Save(r)
	}
	if err := create("questionOptions", "scolioptions001", map[string]any{
		"name": "Skoliosbehandling", "value": []map[string]string{
			{"value": "surgery", "label": "Kirurgi"},
			{"value": "full_time", "label": "Fulltidskorsett (Boston, Chêneau-Gensingen)"},
			{"value": "night", "label": "Nattkorsett (Providence)"},
		},
	}); err != nil {
		return err
	}
	if err := create("questionOptions", "scolioptions002", map[string]any{
		"name": "Träningens längd", "value": []map[string]string{
			{"value": "under_1h", "label": "Mindre än 1 timme"},
			{"value": "about_1h", "label": "Ca 1 timme"},
			{"value": "about_1_5h", "label": "Ca 1,5 timmar"},
			{"value": "about_2h", "label": "Ca 2 timmar"},
			{"value": "over_2h", "label": "Mer än 2 timmar"},
		},
	}); err != nil {
		return err
	}
	activity := "Vi använder data från din mobiltelefon för att mäta din fysiska aktivitet. Därför vill vi veta hur ofta du tränar utan att ha mobilen med dig."
	brace := "Tänk tillbaka på hela den tid du har behandlats med korsett hittills. Försök att göra en så bra uppskattning som möjligt."
	rows := []map[string]any{
		{"text": "När fick du din första menstruation? Om du minns, ange månad och år.", "type": "date", "audience": "female", "settings": map[string]any{"precision": "month", "allowFuture": false}},
		{"text": "Vilken var den senaste behandlingen du fick för din skolios?", "type": "singleChoice", "options": "scolioptions001"},
		{"text": "När opererades du?", "type": "date", "dependencyValue": "surgery", "settings": map[string]any{"precision": "day", "allowFuture": false}},
		{"text": "När provades din korsett ut?", "type": "date", "dependencyValue": []string{"full_time", "night"}, "settings": map[string]any{"precision": "day", "allowFuture": false}},
		{"text": "Innan du opererades, ungefär hur många gånger per vecka tränade du utan att ha mobilen med dig?", "type": "integer", "introduction": activity, "dependencyValue": "surgery", "settings": map[string]any{"unit": "gånger per vecka"}},
		{"text": "Under de senaste 6 månaderna, ungefär hur många gånger per vecka har du tränat utan att mobilen med dig?", "type": "integer", "dependencyValue": "surgery", "settings": map[string]any{"unit": "gånger per vecka"}},
		{"text": "Innan du började använda korsetten, ungefär hur många gånger per vecka tränade du utan att ha mobilen med dig?", "type": "integer", "introduction": activity, "dependencyValue": []string{"full_time", "night"}, "settings": map[string]any{"unit": "gånger per vecka"}},
		{"text": "Under de senaste 6 månaderna, ungefär hur många gånger per vecka har du tränat utan att ha mobilen med dig?", "type": "integer", "dependencyValue": []string{"full_time", "night"}, "settings": map[string]any{"unit": "gånger per vecka"}},
		{"text": "När du tränar utan mobilen, ungefär hur länge brukar du träna varje gång?", "type": "singleChoice", "options": "scolioptions002"},
		{"text": "Under en vanlig vecka, hur många dagar använde du korsetten enligt rekommendationen?", "type": "integer", "introduction": brace, "dependencyValue": "full_time", "settings": map[string]any{"unit": "dagar per vecka"}},
		{"text": "De dagar du använde korsetten, ungefär hur många timmar per dygn hade du den på dig?", "type": "integer", "dependencyValue": "full_time", "settings": map[string]any{"unit": "timmar per dygn"}},
		{"text": "Under en vanlig vecka, hur många nätter använde du korsetten?", "type": "integer", "introduction": brace, "dependencyValue": "night", "settings": map[string]any{"unit": "nätter per vecka"}},
		{"text": "De dagar du använde korsetten, ungefär hur många timmar per dygn hade du den på dig?", "type": "integer", "dependencyValue": "night", "settings": map[string]any{"unit": "timmar per natt"}},
	}
	ids := []string{}
	for i, row := range rows {
		id := fmt.Sprintf("scoliquestion%02d", i+1)
		if row["audience"] == nil {
			row["audience"] = "all"
		}
		if row["dependencyValue"] != nil {
			row["dependency"] = questionnaires.TreatmentID
		}
		if err := create("questions", id, row); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	return create("questionnaires", questionnaires.ScoliosisID, map[string]any{
		"name": "Behandling och fysisk aktivitet", "description": "Frågor om din skoliosbehandling, träning utan mobiltelefon och användning av korsett.",
		"occurance": "once", "enabled": true, "questions": ids,
	})
}
