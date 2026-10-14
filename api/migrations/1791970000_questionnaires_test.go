package migrations

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"testing"
)

func TestTemplateMigrationsLeaveQuestionnaireCollectionsEmpty(t *testing.T) {
	app := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: t.TempDir()})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	defer app.ResetBootstrapState()
	if err := app.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	for _, collection := range []string{"questionnaires", "questions", "questionOptions", "answers"} {
		records, err := app.FindRecordsByFilter(collection, "", "", 1, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(records) != 0 {
			t.Fatalf("template migration unexpectedly populated %s", collection)
		}
	}
}

func TestQuestionnaireMigrationPreservesDefinitionsAndResponses(t *testing.T) {
	app := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: t.TempDir()})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	defer app.ResetBootstrapState()
	if err := app.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	c, _ := app.FindCollectionByNameOrId("questions")
	q := core.NewRecord(c)
	q.Id = "testqst00000001"
	q.Set("type", "date")
	q.Set("text", "Administrator wording")
	q.Set("audience", "male")
	q.Set("settings", map[string]any{"precision": "month", "customPresentation": "preserve"})
	if err := app.Save(q); err != nil {
		t.Fatal(err)
	}
	other := core.NewRecord(c)
	other.Set("type", "text")
	other.Set("settings", map[string]any{})
	if err := app.Save(other); err != nil {
		t.Fatal(err)
	}
	c, _ = app.FindCollectionByNameOrId("questionnaires")
	f := core.NewRecord(c)
	f.Set("occurance", "once")
	f.Set("questions", []string{q.Id})
	if err := app.Save(f); err != nil {
		t.Fatal(err)
	}
	c, _ = app.FindCollectionByNameOrId("users")
	u := core.NewRecord(c)
	u.Set("personalNumber", "199001010020")
	u.SetPassword("test-password-123")
	if err := app.Save(u); err != nil {
		t.Fatal(err)
	}
	c, _ = app.FindCollectionByNameOrId("answers")
	a := core.NewRecord(c)
	a.Set("user", u.Id)
	a.Set("questionnaire", f.Id)
	a.Set("answers", map[string]any{q.Id: "2003-05"})
	if err := app.Save(a); err != nil {
		t.Fatal(err)
	}
	if err := app.RunInTransaction(migrateQuestionnaires); err != nil {
		t.Fatal(err)
	}
	migrated, _ := app.FindRecordById("questions", q.Id)
	if migrated.GetString("text") != "Administrator wording" || migrated.GetString("audience") != "male" || migrated.GetString("settings") != q.GetString("settings") {
		t.Fatal("lost field data")
	}
	untouched, _ := app.FindRecordById("questions", other.Id)
	if untouched.GetString("settings") != other.GetString("settings") || untouched.GetString("audience") != "" {
		t.Fatal("unrelated question changed")
	}
	saved, _ := app.FindRecordById("answers", a.Id)
	if saved.GetString("answers") != a.GetString("answers") || saved.GetString("submissionKey") != u.Id+":"+f.Id+":once" {
		t.Fatal("response not preserved")
	}
}
