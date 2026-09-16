package study

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase"
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
		record, err := PublishConsent(app, cfg, version, title, string(text))
		if err != nil {
			return err
		}
		cmd.Printf("Published consent %s (%s). Existing participants must sign this version before further uploads.\n", version, record.Id)
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
		var expectedIdentity string
		if expectedFile != "" {
			raw, err := os.ReadFile(expectedFile)
			if err != nil {
				return err
			}
			expectedIdentity = strings.TrimSpace(string(raw))
			if expectedIdentity == "" {
				return errors.New("expected identity file must contain a 12-digit personal number")
			}
		}
		r, code, err := IssueInvitation(app, cfg, participant, expectedIdentity, hours, time.Now())
		if err != nil {
			return err
		}
		cmd.Printf("Participant: %s\nInvitation: %s\nExpires: %s\n", participant, code, time.Unix(int64(r.GetInt("expiresAt")), 0).UTC().Format(time.RFC3339))
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
