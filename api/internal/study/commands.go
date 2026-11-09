package study

import (
	"app/internal/kontoapi"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/spf13/cobra"
)

// Commands operate locally with the same database and key configuration as the
// service. They never print encryption keys or personal numbers.
func RegisterCommands(app *pocketbase.PocketBase) {
	root := &cobra.Command{Use: "bankid", Short: "KontoAPI authentication and study consent administration"}
	prepare := func(cmd *cobra.Command, args []string) error {
		if err := app.Bootstrap(); err != nil {
			return err
		}
		return app.RunAllMigrations()
	}
	root.PersistentPreRunE = prepare
	var file, version, title string
	var ifMissing bool
	publish := &cobra.Command{Use: "publish-consent", Short: "Publish an immutable UTF-8 consent version", RunE: func(cmd *cobra.Command, args []string) error {
		text, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		record, err := publishConsent(app, version, title, string(text), ifMissing)
		if err != nil {
			return err
		}
		cmd.Printf("Consent %s (%s) available; current=%t.\n", version, record.Id, record.GetBool("current"))
		return nil
	}}
	publish.Flags().StringVar(&file, "file", "", "Path to approved consent text")
	publish.Flags().StringVar(&version, "version", "", "Unique version label")
	publish.Flags().StringVar(&title, "title", "", "Consent title")
	publish.Flags().BoolVar(&ifMissing, "if-missing", false, "Allow an existing version only when its title and text match; do not reactivate retired versions")
	var recordID, kind, output string
	export := &cobra.Command{Use: "export-evidence", Short: "Decrypt authentication or consent evidence into a private local file", RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := LoadConfig()
		if err != nil {
			return err
		}
		if err := cfg.ValidateSecrets(); err != nil {
			return err
		}
		collections := map[string]string{"authentication": "authentications", "consent": "consent_acceptances", "guardian": "guardian_approvals"}
		collection, ok := collections[kind]
		if !ok {
			return fmt.Errorf("kind must be authentication, consent or guardian")
		}
		r, err := app.FindRecordById(collection, recordID)
		if err != nil {
			return err
		}
		var raw json.RawMessage
		if err := cfg.Open(kind+":"+r.Id, r.GetString("evidenceCipher"), &raw); err != nil {
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
	export.Flags().StringVar(&recordID, "record", "", "Evidence record ID")
	export.Flags().StringVar(&kind, "kind", "consent", "authentication, consent or guardian")
	export.Flags().StringVar(&output, "out", "", "New private output file (contains personal information)")
	var ip string
	check := &cobra.Command{Use: "check-kontoapi", Short: "Create a diagnostic authentication order and check QR/status (requires VPN)", RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := LoadConfig()
		if err != nil {
			return err
		}
		client, err := kontoapi.NewClient(cfg.KontoAPI)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		order, err := client.Start(ctx, ip)
		if err != nil {
			return err
		}
		cmd.Println("KontoAPI authentication order created.")
		if _, err = client.QRImage(ctx, order); err != nil {
			return err
		}
		cmd.Println("Animated QR image retrieved.")
		result, err := client.Collect(ctx, order.OrderRef)
		if err != nil {
			return err
		}
		cmd.Println("Order status:", result.Status, "(no study session or consent created)")
		return nil
	}}
	check.Flags().StringVar(&ip, "ip", "127.0.0.1", "IP of the local diagnostic client")
	root.AddCommand(publish, export, check)
	app.RootCmd.AddCommand(root)
}
