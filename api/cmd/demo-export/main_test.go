package main

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func TestExportUsesSeededDefinitionsAndOrder(t *testing.T) {
	app := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: t.TempDir()})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	defer app.ResetBootstrapState()
	if err := app.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../../internal/questionnaires/testdata/records.json")
	if err != nil {
		t.Fatal(err)
	}
	var records []struct {
		Collection string         `json:"collection"`
		Data       map[string]any `json:"data"`
	}
	if err := json.Unmarshal(data, &records); err != nil {
		t.Fatal(err)
	}
	for _, item := range records {
		collection, err := app.FindCollectionByNameOrId(item.Collection)
		if err != nil {
			t.Fatal(err)
		}
		record := core.NewRecord(collection)
		for k, v := range item.Data {
			record.Set(k, v)
		}
		if err := app.Save(record); err != nil {
			t.Fatal(err)
		}
	}
	form, err := app.FindRecordById("questionnaires", "testfrm00000001")
	if err != nil {
		t.Fatal(err)
	}
	form.Set("name", "Actual adapted study")
	form.Set("questions", []string{"testqst00000008", "testqst00000002"})
	if err := app.Save(form); err != nil {
		t.Fatal(err)
	}
	output, err := export(app)
	if err != nil {
		t.Fatal(err)
	}
	var exported map[string]any
	for _, item := range output {
		if item["id"] == form.Id {
			exported = item
		}
	}
	if exported == nil || exported["name"] != "Actual adapted study" {
		t.Fatal("export did not use study records")
	}
	questions := exported["expand"].(map[string]any)["questions"].([]map[string]any)
	if len(questions) != 2 || questions[0]["id"] != "testqst00000008" || questions[1]["id"] != "testqst00000002" {
		t.Fatal("question relation order was lost")
	}
	option := questions[0]["expand"].(map[string]any)["options"].(map[string]any)
	if option["id"] != "testopt00000002" || option["value"] == nil {
		t.Fatal("option expansion missing")
	}
	for _, forbidden := range []string{"collectionId", "created", "updated", "user", "answers", "personalNumber"} {
		if _, exists := exported[forbidden]; exists {
			t.Fatalf("unexpected export field %s", forbidden)
		}
	}
	form.Set("enabled", false)
	if err := app.Save(form); err != nil {
		t.Fatal(err)
	}
	output, err = export(app)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range output {
		if item["id"] == form.Id {
			t.Fatal("disabled form should not be previewed")
		}
	}
}
