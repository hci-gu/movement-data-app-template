package study

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// PublishConsent is shared by the dashboard and optional operator commands.
// The immutable document and its publication pointer commit together.
func PublishConsent(app core.App, _ Config, version, title, text string) (*core.Record, error) {
	if !utf8.ValidString(text) || len(text) == 0 || len(text) > 30000 || strings.TrimSpace(version) == "" || strings.TrimSpace(title) == "" {
		return nil, problem(400, "invalidConsent", "Provide a version, title and nonempty UTF-8 consent text of at most 30000 bytes.")
	}
	for _, r := range text {
		if !(r == '\n' || r == '\r' || r == '\t' || r >= 0x20 && r <= 0x7e || r >= 0xa0 && r <= 0xffef) {
			return nil, problem(400, "invalidConsent", fmt.Sprintf("Consent contains unsupported character U+%04X.", r))
		}
	}
	var record *core.Record
	err := app.RunInTransaction(func(tx core.App) error {
		_, err := tx.FindFirstRecordByFilter("consent_texts", "version={:version}", dbx.Params{"version": version})
		if err == nil {
			return problem(400, "duplicateVersion", "This consent version already exists; use a new version label.")
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		record, err = newRecord(tx, "consent_texts")
		if err != nil {
			return err
		}
		record.Set("version", version)
		record.Set("title", title)
		record.Set("text", text)
		record.Set("documentHash", hash(text))
		current, err := tx.FindRecordsByFilter("consent_texts", "current=true", "", 0, 0)
		if err != nil {
			return err
		}
		for _, old := range current {
			old.Set("current", false)
			if err := tx.Save(old); err != nil {
				return err
			}
		}
		record.Set("current", true)
		return tx.Save(record)
	})
	return record, err
}
