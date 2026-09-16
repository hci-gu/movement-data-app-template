package study

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/spf13/cobra"
)

// Commands operate locally with the same database and key configuration as the
// service. They never print encryption keys or personal numbers.
func RegisterCommands(app *pocketbase.PocketBase) {
	root := &cobra.Command{Use: "study", Short: "Publish consent, issue invitations and export signing evidence"}
	prepare := func(cmd *cobra.Command, args []string) error {
		if err := app.Bootstrap(); err != nil {
			return err
		}
		return app.RunAllMigrations()
	}
	root.PersistentPreRunE = prepare
	var file, version, title string
	publish := &cobra.Command{Use: "publish-consent", Short: "Publish an immutable UTF-8 consent version", RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := LoadConfig()
		if err != nil {
			return err
		}
		text, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if !utf8.Valid(text) || len(text) == 0 || len(text) > 30000 || strings.TrimSpace(version) == "" || strings.TrimSpace(title) == "" {
			return errors.New("provide a version, title and nonempty UTF-8 consent file of at most 30000 bytes")
		}
		// BankID supports a subset of Unicode; avoid unsupported controls/emoji.
		for _, r := range string(text) {
			if !(r == '\n' || r == '\r' || r == '\t' || r >= 0x20 && r <= 0x7e || r >= 0xa0 && r <= 0xffef) {
				return fmt.Errorf("consent contains unsupported character U+%04X", r)
			}
		}
		var id string
		err = app.RunInTransaction(func(tx core.App) error {
			existing, err := tx.FindFirstRecordByFilter("consent_versions", "study={:study} && version={:version}", dbx.Params{"study": cfg.StudyID, "version": version})
			if err == nil {
				return fmt.Errorf("version already exists (%s); use a new version", existing.Id)
			}
			if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			r, err := newRecord(tx, "consent_versions")
			if err != nil {
				return err
			}
			r.Set("study", cfg.StudyID)
			r.Set("version", version)
			r.Set("title", title)
			r.Set("text", string(text))
			r.Set("documentHash", hash(string(text)))
			if err := tx.Save(r); err != nil {
				return err
			}
			id = r.Id
			settings, err := tx.FindFirstRecordByData("study_settings", "study", cfg.StudyID)
			if errors.Is(err, sql.ErrNoRows) {
				settings, err = newRecord(tx, "study_settings")
			}
			if err != nil {
				return err
			}
			settings.Set("study", cfg.StudyID)
			settings.Set("currentVersion", r.Id)
			return tx.Save(settings)
		})
		if err != nil {
			return err
		}
		cmd.Printf("Published consent %s (%s). Existing participants must sign this version before further uploads.\n", version, id)
		return nil
	}}
	publish.Flags().StringVar(&file, "file", "", "Path to approved consent text")
	publish.Flags().StringVar(&version, "version", "", "Unique version label")
	publish.Flags().StringVar(&title, "title", "", "Consent title")
	var participant, expectedFile string
	var hours int
	invite := &cobra.Command{Use: "invite", Short: "Issue a one-use invitation for a study participant", RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := LoadConfig()
		if err != nil {
			return err
		}
		if err := cfg.ValidateSecrets(); err != nil {
			return err
		}
		if !participantPattern.MatchString(participant) || hours < 1 || hours > 2160 {
			return errors.New("participant must be 4–64 safe characters; validity must be 1–2160 hours")
		}
		code := randomSecret()
		r, err := newRecord(app, "study_invitations")
		if err != nil {
			return err
		}
		r.Set("study", cfg.StudyID)
		r.Set("participantId", participant)
		r.Set("tokenHash", cfg.digest("invitation", code))
		r.Set("expiresAt", time.Now().Add(time.Duration(hours)*time.Hour).Unix())
		// Save to allocate the ID used in authenticated encryption, in one transaction.
		err = app.RunInTransaction(func(tx core.App) error {
			if err := tx.Save(r); err != nil {
				return err
			}
			if expectedFile != "" {
				raw, err := os.ReadFile(expectedFile)
				if err != nil {
					return err
				}
				pnr := strings.TrimSpace(string(raw))
				if !personalNumberPattern.MatchString(pnr) {
					return errors.New("expected identity file must contain a 12-digit personal number")
				}
				sealed, err := cfg.seal("invitation:"+r.Id, identity{PersonalNumber: pnr})
				if err != nil {
					return err
				}
				r.Set("expectedCipher", sealed)
				return tx.Save(r)
			}
			return nil
		})
		if err != nil {
			return err
		}
		cmd.Printf("Participant: %s\nInvitation (shown once): %s\nExpires: %s\n", participant, code, time.Unix(int64(r.GetInt("expiresAt")), 0).UTC().Format(time.RFC3339))
		return nil
	}}
	invite.Flags().StringVar(&participant, "participant", "", "Study participant identifier")
	invite.Flags().StringVar(&expectedFile, "expected-identity-file", "", "File with expected signer personal number (optional)")
	invite.Flags().IntVar(&hours, "hours", 168, "Invitation lifetime in hours")
	var signatureID, output string
	export := &cobra.Command{Use: "export-evidence", Short: "Decrypt a signature evidence bundle into a private local file", RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := LoadConfig()
		if err != nil {
			return err
		}
		if err := cfg.ValidateSecrets(); err != nil {
			return err
		}
		r, err := app.FindRecordById("consent_signatures", signatureID)
		if err != nil {
			return err
		}
		if r.GetString("study") != cfg.StudyID {
			return errors.New("signature belongs to another study")
		}
		var raw json.RawMessage
		if err := cfg.open("signature:"+r.GetString("order"), r.GetString("evidenceCipher"), &raw); err != nil {
			return err
		}
		f, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := f.Write(raw); err != nil {
			return err
		}
		if err := f.Sync(); err != nil {
			return err
		}
		cmd.Println("Evidence exported to", output)
		return nil
	}}
	export.Flags().StringVar(&signatureID, "signature", "", "Signature record ID")
	export.Flags().StringVar(&output, "out", "", "New private output file (contains personal information)")
	purge := &cobra.Command{Use: "purge-sessions", Short: "Remove expired operational secrets after a 24-hour grace period", RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := LoadConfig()
		if err != nil {
			return err
		}
		count, err := PurgeSessions(app, cfg, time.Now())
		if err != nil {
			return err
		}
		cmd.Printf("Purged %d operational records; consent evidence retained.\n", count)
		return nil
	}}
	root.AddCommand(publish, invite, export, purge)
	app.RootCmd.AddCommand(root)
}
