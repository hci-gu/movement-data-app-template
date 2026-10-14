package migrations

import (
	"fmt"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// Seed the questionnaire from context/scolios_formulär.docx. Settings and option
// values follow the contract documented in docs/questionnaire-data.md.
func init() {
	m.Register(seedScoliosisQuestionnaire, nil)
}

func seedScoliosisQuestionnaire(app core.App) error {
	optionsCollection, err := app.FindCollectionByNameOrId("questionOptions")
	if err != nil {
		return err
	}
	questionsCollection, err := app.FindCollectionByNameOrId("questions")
	if err != nil {
		return err
	}
	questionnairesCollection, err := app.FindCollectionByNameOrId("questionnaires")
	if err != nil {
		return err
	}

	optionSets := []struct {
		id, name string
		values   []map[string]string
	}{
		{"scoliosisopt001", "Skoliosbehandling", []map[string]string{
			{"value": "surgery", "label": "Kirurgi"},
			{"value": "fullTimeBrace", "label": "Fulltidskorsett (Boston, Chêneau-Gensingen)"},
			{"value": "nightBrace", "label": "Nattkorsett (Providence)"},
		}},
		{"scoliosisopt002", "Träningslängd utan mobiltelefon", []map[string]string{
			{"value": "lessThan1Hour", "label": "Mindre än 1 timme"},
			{"value": "about1Hour", "label": "Ca 1 timme"},
			{"value": "about1Point5Hours", "label": "Ca 1,5 timmar"},
			{"value": "about2Hours", "label": "Ca 2 timmar"},
			{"value": "moreThan2Hours", "label": "Mer än 2 timmar"},
		}},
	}
	for _, set := range optionSets {
		r := core.NewRecord(optionsCollection)
		r.Id = set.id
		r.Set("name", set.name)
		r.Set("value", set.values)
		if err := app.Save(r); err != nil {
			return fmt.Errorf("create option set %s: %w", set.id, err)
		}
	}

	const activityIntro = "Vi använder data från din mobiltelefon för att mäta din fysiska aktivitet. Därför vill vi veta hur ofta du tränar utan att ha mobilen med dig."
	const adherenceIntro = "Tänk tillbaka på hela den tid du har behandlats med korsett hittills. Försök att göra en så bra uppskattning som möjligt."
	defs := []struct {
		text, kind, section, intro, placeholder, options, precision, unit string
		max                                                               int
		treatments                                                        []string
	}{
		{text: "När fick du din första menstruation? Om du minns, ange månad och år.", kind: "date", section: "Datum för första menstruation", placeholder: "ÅÅÅÅ-MM", precision: "month"},
		{text: "Vilken var den senaste behandlingen du fick för din skolios?", kind: "singleChoice", section: "Typ av behandling", options: "scoliosisopt001"},
		{text: "När opererades du?", kind: "date", section: "Datum för behandlingsstart", precision: "day", treatments: []string{"surgery"}},
		{text: "När provades din korsett ut?", kind: "date", section: "Datum för behandlingsstart", precision: "day", treatments: []string{"fullTimeBrace", "nightBrace"}},
		{text: "Innan du opererades, ungefär hur många gånger per vecka tränade du utan att ha mobilen med dig?", kind: "integer", section: "Fysisk aktivitet utan mobiltelefon", intro: activityIntro, unit: "gånger per vecka", max: 21, treatments: []string{"surgery"}},
		{text: "Innan du började använda korsetten, ungefär hur många gånger per vecka tränade du utan att ha mobilen med dig?", kind: "integer", section: "Fysisk aktivitet utan mobiltelefon", intro: activityIntro, unit: "gånger per vecka", max: 21, treatments: []string{"fullTimeBrace", "nightBrace"}},
		{text: "Under de senaste 6 månaderna, ungefär hur många gånger per vecka har du tränat utan att mobilen med dig?", kind: "integer", section: "Fysisk aktivitet utan mobiltelefon", unit: "gånger per vecka", max: 21},
		{text: "När du tränar utan mobilen, ungefär hur länge brukar du träna varje gång?", kind: "singleChoice", section: "Fysisk aktivitet utan mobiltelefon", options: "scoliosisopt002"},
		{text: "Under en vanlig vecka, hur många dagar använde du korsetten enligt rekommendationen?", kind: "integer", section: "Följsamhet till korsettbehandling", intro: adherenceIntro, unit: "dagar per vecka", max: 7, treatments: []string{"fullTimeBrace"}},
		{text: "De dagar du använde korsetten, ungefär hur många timmar per dygn hade du den på dig?", kind: "integer", section: "Följsamhet till korsettbehandling", unit: "timmar per dygn", max: 24, treatments: []string{"fullTimeBrace"}},
		{text: "Under en vanlig vecka, hur många nätter använde du korsetten?", kind: "integer", section: "Följsamhet till korsettbehandling", intro: adherenceIntro, unit: "nätter per vecka", max: 7, treatments: []string{"nightBrace"}},
		{text: "De dagar du använde korsetten, ungefär hur många timmar per dygn hade du den på dig?", kind: "integer", section: "Följsamhet till korsettbehandling", unit: "timmar per natt", max: 24, treatments: []string{"nightBrace"}},
	}
	ids := make([]string, 0, len(defs))
	for i, d := range defs {
		r := core.NewRecord(questionsCollection)
		r.Id = fmt.Sprintf("scoliosisq%05d", i+1)
		r.Set("text", d.text)
		r.Set("type", d.kind)
		r.Set("introduction", d.intro)
		r.Set("options", d.options)
		r.Set("placeholder", d.placeholder)
		settings := map[string]any{
			"version": 1, "order": i + 1, "section": d.section, "required": i != 0,
		}
		if len(d.treatments) > 0 {
			settings["visibleWhen"] = map[string]any{
				"questionId": "scoliosisq00002", "operator": "in", "values": d.treatments,
			}
		}
		if d.kind == "integer" {
			settings["interval"] = map[string]int{"min": 0, "max": d.max, "step": 1}
			settings["unit"] = d.unit
			r.Set("placeholder", d.unit)
		}
		if d.kind == "date" {
			settings["precision"] = d.precision
			if d.precision == "month" {
				settings["format"] = "YYYY-MM"
			} else {
				settings["format"] = "YYYY-MM-DD"
			}
		}
		r.Set("settings", settings)
		if err := app.Save(r); err != nil {
			return fmt.Errorf("create question %s: %w", r.Id, err)
		}
		ids = append(ids, r.Id)
	}
	questionnaire := core.NewRecord(questionnairesCollection)
	questionnaire.Id = "scoliosisform01"
	questionnaire.Set("name", "Skoliosformulär")
	questionnaire.Set("description", "Frågor om din skoliosbehandling, fysisk aktivitet utan mobiltelefon och användning av korsett.")
	questionnaire.Set("enabled", true)
	questionnaire.Set("occurance", "once")
	questionnaire.Set("questions", ids)
	return app.Save(questionnaire)
}
