package study

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// PublishConsent is shared by the dashboard and optional operator commands.
// The immutable document and its publication pointer commit together.
func PublishConsent(app core.App, cfg Config, version, title, text string) (*core.Record, error) {
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
		_, err := tx.FindFirstRecordByFilter("consent_texts", "study={:study} && version={:version}", dbx.Params{"study": cfg.StudyID, "version": version})
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
		record.Set("study", cfg.StudyID)
		record.Set("version", version)
		record.Set("title", title)
		record.Set("text", text)
		record.Set("documentHash", hash(text))
		current, err := tx.FindRecordsByFilter("consent_texts", "study={:study} && current=true", "", 0, 0, dbx.Params{"study": cfg.StudyID})
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

// IssueInvitation stores only a digest and encrypted delivery code. The dashboard
// can recover the code while it is valid; signer identity is never returned.
func IssueInvitation(app core.App, cfg Config, participant, expectedIdentity string, hours int, now time.Time) (*core.Record, string, error) {
	if err := cfg.ValidateSecrets(); err != nil {
		return nil, "", problem(503, "studySecretsMissing", "Configure the study encryption and identity keys in the server deployment before issuing invitations.")
	}
	if !participantPattern.MatchString(participant) || hours < 1 || hours > 2160 {
		return nil, "", problem(400, "invalidInvitation", "Participant ID must be 4–64 safe characters; validity must be 1–2160 hours.")
	}
	expectedIdentity = strings.TrimSpace(expectedIdentity)
	if expectedIdentity != "" && !personalNumberPattern.MatchString(expectedIdentity) {
		return nil, "", problem(400, "invalidIdentity", "Expected personal number must contain exactly 12 digits.")
	}
	var code string
	var record *core.Record
	err := app.RunInTransaction(func(tx core.App) error {
		var err error
		record, err = tx.FindFirstRecordByData("users", "username", participant)
		if errors.Is(err, sql.ErrNoRows) {
			record, err = newRecord(tx, "users")
			if err != nil {
				return err
			}
			record.Set("username", participant)
			record.SetPassword(randomSecret())
		} else if err != nil {
			return err
		}
		if record.GetBool("active") {
			return problem(409, "alreadyEnrolled", "This participant is already enrolled; use BankID login.")
		}
		if record.GetString("study") != "" && (record.GetString("study") != cfg.StudyID || record.GetString("environment") != cfg.Environment) {
			return problem(409, "participantConflict", "Participant belongs to a different study or environment.")
		}
		code, err = unusedInvitationCode(tx, cfg, rand.Reader)
		if err != nil {
			return err
		}
		record.Set("study", cfg.StudyID)
		record.Set("environment", cfg.Environment)
		record.Set("invitationHash", cfg.digest("invitation", code))
		record.Set("invitationExpiresAt", now.Add(time.Duration(hours)*time.Hour).Unix())
		record.Set("expectedCipher", "")
		record.Set("validityHours", hours)
		if err := tx.Save(record); err != nil {
			return err
		}
		sealed, err := cfg.Seal("invitation-code:"+record.Id, code)
		if err != nil {
			return err
		}
		record.Set("invitationCipher", sealed)
		if expectedIdentity != "" {
			sealed, err := cfg.Seal("expected:"+record.Id, identity{PersonalNumber: expectedIdentity})
			if err != nil {
				return err
			}
			record.Set("expectedCipher", sealed)
		}
		return tx.Save(record)
	})
	return record, code, err
}

// Choose and reserve codes in the issuing transaction. The unique invitationHash
// index is the final guard against assigning the same code to two invitations.
func unusedInvitationCode(app core.App, cfg Config, source io.Reader) (string, error) {
	for attempt := 0; attempt < 32; attempt++ {
		number, err := rand.Int(source, big.NewInt(1_000_000))
		if err != nil {
			return "", err
		}
		n := number.Int64()
		code := fmt.Sprintf("%03d-%03d", n/1000, n%1000)
		_, err = app.FindFirstRecordByData("users", "invitationHash", cfg.digest("invitation", code))
		if errors.Is(err, sql.ErrNoRows) {
			return code, nil
		}
		if err != nil {
			return "", err
		}
	}
	return "", problem(503, "invitationCodesBusy", "Unable to allocate an unused invitation code. Please try again.")
}
