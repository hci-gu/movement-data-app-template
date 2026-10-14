package migrations

import (
	"database/sql"
	"errors"
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"slices"
)

func init() { m.Register(migrateQuestionnaires, nil) }

func migrateQuestionnaires(app core.App) error {
	questions, err := app.FindCollectionByNameOrId("questions")
	if err != nil {
		return err
	}
	questions.Fields.Add(&core.SelectField{Name: "audience", Values: []string{"all", "female", "male"}, MaxSelect: 1})
	if err := app.Save(questions); err != nil {
		return err
	}
	answers, err := app.FindCollectionByNameOrId("answers")
	if err != nil {
		return err
	}
	answers.Fields.Add(&core.TextField{Name: "submissionKey", Max: 100, Hidden: true})
	index := "CREATE UNIQUE INDEX idx_answers_submission ON answers (submissionKey) WHERE submissionKey != ''"
	if !slices.Contains(answers.Indexes, index) {
		answers.Indexes = append(answers.Indexes, index)
	}
	answers.UpdateRule = nil
	answers.DeleteRule = nil
	if err := app.Save(answers); err != nil {
		return err
	}
	// Keep historical responses intact. The first response reserves its once-key;
	// any legacy duplicates remain available for researcher reconciliation.
	existing, err := app.FindRecordsByFilter("answers", "", "created,id", 0, 0)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, r := range existing {
		form, err := app.FindRecordById("questionnaires", r.GetString("questionnaire"))
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		if form.GetString("occurance") != "once" || r.GetString("user") == "" {
			continue
		}
		key := r.GetString("user") + ":" + form.Id + ":once"
		if seen[key] {
			continue
		}
		seen[key] = true
		r.Set("submissionKey", key)
		if err := app.Save(r); err != nil {
			return err
		}
	}
	return nil
}
