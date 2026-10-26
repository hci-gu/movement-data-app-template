// demo-export reads definition migrations in a disposable database. It never
// loads .env, connects to BankID, or opens a participant database.
package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"app/internal/study"
	_ "app/migrations"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func fields(r *core.Record, names ...string) map[string]any {
	out := map[string]any{"id": r.Id}
	for _, name := range names {
		out[name] = r.Get(name)
	}
	return out
}

func export(app core.App) ([]map[string]any, error) {
	forms, err := app.FindRecordsByFilter("questionnaires", "enabled=true", "id", 0, 0)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(forms))
	for _, form := range forms {
		f := fields(form, "name", "description", "enabled", "occurance", "questions")
		questions := []map[string]any{}
		for _, id := range form.GetStringSlice("questions") {
			q, err := app.FindRecordById("questions", id)
			if err != nil {
				return nil, err
			}
			item := fields(q, "text", "type", "introduction", "placeholder", "settings", "dependency", "dependencyValue", "audience", "options", "valueFromQuestion")
			if id := q.GetString("options"); id != "" {
				o, err := app.FindRecordById("questionOptions", id)
				if err != nil {
					return nil, err
				}
				item["expand"] = map[string]any{"options": fields(o, "name", "value")}
			}
			questions = append(questions, item)
		}
		f["expand"] = map[string]any{"questions": questions}
		out = append(out, f)
	}
	return out, nil
}

func run() error {
	output := flag.String("out", "", "Output directory")
	consent := flag.String("consent", "", "Agreed UTF-8 consent source (required for a study without seeded consent)")
	title := flag.String("title", "Study consent", "Consent source title")
	template := flag.String("template-fixture", "", "Synthetic fixture, used only when migrations contain no enabled forms")
	flag.Parse()
	if *output == "" {
		return fmt.Errorf("--out is required")
	}
	dir, err := os.MkdirTemp("", "study-demo-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	app := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: dir})
	if err := app.Bootstrap(); err != nil {
		return err
	}
	defer app.ResetBootstrapState()
	if err := app.RunAllMigrations(); err != nil {
		return err
	}
	forms, err := export(app)
	if err != nil {
		return err
	}
	kind := "study"
	_, consentError := app.FindFirstRecordByFilter("consent_texts", "current=true")
	if consentError != nil && !errors.Is(consentError, sql.ErrNoRows) {
		return consentError
	}
	if len(forms) == 0 && *template != "" && *consent == "" && errors.Is(consentError, sql.ErrNoRows) {
		kind = "template"
		data, err := os.ReadFile(*template)
		if err != nil {
			return err
		}
		var fixtures []struct {
			Collection string         `json:"collection"`
			Data       map[string]any `json:"data"`
		}
		if err := json.Unmarshal(data, &fixtures); err != nil {
			return err
		}
		for _, fixture := range fixtures {
			if fixture.Collection != "questions" && fixture.Collection != "questionOptions" && fixture.Collection != "questionnaires" {
				return fmt.Errorf("unexpected fixture collection")
			}
			collection, err := app.FindCollectionByNameOrId(fixture.Collection)
			if err != nil {
				return err
			}
			r := core.NewRecord(collection)
			for key, value := range fixture.Data {
				r.Set(key, value)
			}
			if err := app.Save(r); err != nil {
				return err
			}
		}
		forms, err = export(app)
		if err != nil {
			return err
		}
	}
	var doc *core.Record
	if *consent != "" {
		data, err := os.ReadFile(*consent)
		if err != nil {
			return err
		}
		doc, err = study.PublishConsent(app, "demo-source", *title, string(data))
		if err != nil {
			return err
		}
	} else {
		doc, err = app.FindFirstRecordByFilter("consent_texts", "current=true")
		if err != nil {
			if kind != "template" {
				return fmt.Errorf("study consent missing: supply --consent; no template consent will be substituted")
			}
			doc, err = study.PublishConsent(app, "template-demo", "Demonstration consent", "DEMONSTRATION ONLY — not research consent.\n\nThis preview illustrates reviewing consent, answering a questionnaire and sharing sample movement data. No real signatures or participant data are used.")
			if err != nil {
				return err
			}
		}
	}
	if err := os.MkdirAll(*output, 0755); err != nil {
		return err
	}
	consentData := fields(doc, "version", "title", "text", "documentHash")
	// No production record ID is needed by an in-memory demo gateway.
	consentData["id"] = "preview-consent"
	data, err := json.MarshalIndent(map[string]any{
		"schemaVersion": 1, "sourceKind": kind, "questionnaires": forms,
		"consent": consentData,
	}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(*output, "study.json"), data, 0644); err != nil {
		return err
	}
	return study.WriteGuardianPreviews(filepath.Join(*output, "guardian"))
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
