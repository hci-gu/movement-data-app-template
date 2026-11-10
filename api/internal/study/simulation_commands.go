//go:build study_simulation

package study

import (
	"app/internal/testidentity"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/spf13/cobra"
)

func registerSimulationCommands(app *pocketbase.PocketBase) {
	root := &cobra.Command{Use: "simulation", Short: "Manage isolated test identity runs (never changes the active server)"}
	var source, output, consentFile string
	create := &cobra.Command{Use: "new-run", Args: cobra.NoArgs, Short: "Create a fresh database, immutable persona configuration and evidence keyring", RunE: func(cmd *cobra.Command, args []string) error {
		file, err := testidentity.ReadFile(source)
		if err != nil {
			return err
		}
		file.RunID = randomSecret()
		run, err := testidentity.New(file)
		if err != nil {
			return err
		}
		consent, err := os.ReadFile(consentFile)
		if err != nil {
			return errors.New("cannot read the test consent file")
		}
		if output == "" {
			return errors.New("a new output directory is required")
		}
		output, err = filepath.Abs(output)
		if err != nil {
			return err
		}
		if err = os.MkdirAll(filepath.Dir(output), 0700); err != nil {
			return err
		}
		if err = os.Mkdir(output, 0700); err != nil {
			return errors.New("output directory must not already exist")
		}
		ok := false
		defer func() {
			if !ok {
				os.RemoveAll(output)
			}
		}()
		key := make([]byte, 32)
		if _, err = rand.Read(key); err != nil {
			return err
		}
		if err = writePrivateJSON(filepath.Join(output, "personas.json"), file); err != nil {
			return err
		}
		if err = writePrivateJSON(filepath.Join(output, "study-keys.json"), map[string]any{"activeKey": "simulation", "encryptionKeys": map[string]string{"simulation": base64.StdEncoding.EncodeToString(key)}}); err != nil {
			return err
		}
		db := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: filepath.Join(output, "pb_data")})
		if err = db.Bootstrap(); err != nil {
			return err
		}
		defer db.OnTerminate().Trigger(&core.TerminateEvent{App: db}, func(e *core.TerminateEvent) error { return e.App.ResetBootstrapState() })
		if err = db.RunAllMigrations(); err != nil {
			return err
		}
		if err = BindSimulationDatabase(db, run); err != nil {
			return err
		}
		if _, err = PublishConsent(db, "simulation-v1", "Test workflow consent", string(consent)); err != nil {
			return err
		}
		ok = true
		cmd.Println("Created isolated simulation run:", run.ID())
		cmd.Println("Directory:", output)
		cmd.Println("Stop the previous test server, then serve this run's pb_data with STUDY_DEPLOYMENT=simulation, STUDY_SIMULATION_FILE=personas.json and STUDY_SECRETS_FILE=study-keys.json (use absolute paths).")
		return nil
	}}
	create.Flags().StringVar(&source, "file", "", "Private persona assignment JSON")
	create.Flags().StringVar(&output, "out", "", "New run directory (must not exist)")
	create.Flags().StringVar(&consentFile, "consent", "", "Test consent text")
	for _, name := range []string{"file", "out", "consent"} {
		_ = create.MarkFlagRequired(name)
	}
	var inspectFile string
	show := &cobra.Command{Use: "show", Args: cobra.NoArgs, Short: "Validate a run and show only fictional identities", RunE: func(cmd *cobra.Command, args []string) error {
		file, err := testidentity.ReadFile(inspectFile)
		if err != nil {
			return err
		}
		run, err := testidentity.New(file)
		if err != nil {
			return err
		}
		cmd.Println(run.Label())
		for _, p := range run.Personas() {
			cmd.Printf("%s (%s): %s\n", p.ID, p.Role, p.PersonalNumber)
		}
		return nil
	}}
	show.Flags().StringVar(&inspectFile, "file", "", "Private run configuration")
	_ = show.MarkFlagRequired("file")
	var birthDate, serial string
	identity := &cobra.Command{Use: "identity", Args: cobra.NoArgs, Short: "Generate a fictional personal-number fixture from a birth date", RunE: func(cmd *cobra.Command, args []string) error {
		birth, err := time.Parse("2006-01-02", birthDate)
		if err != nil || birth.After(time.Now()) || !regexp.MustCompile(`^[0-9]{3}$`).MatchString(serial) {
			return errors.New("use a past YYYY-MM-DD birth date and a three-digit serial")
		}
		base := birth.Format("20060102") + serial
		sum := 0
		for i, ch := range base[2:] {
			n := int(ch - '0')
			if i%2 == 0 {
				n *= 2
			}
			sum += n/10 + n%10
		}
		cmd.Println(fmt.Sprintf("%s%d", base, (10-sum%10)%10))
		return nil
	}}
	identity.Flags().StringVar(&birthDate, "birth-date", "", "Fictional YYYY-MM-DD birth date")
	identity.Flags().StringVar(&serial, "serial", "001", "Three digits; last digit controls registered gender parity")
	_ = identity.MarkFlagRequired("birth-date")
	root.AddCommand(create, show, identity)
	app.RootCmd.AddCommand(root)
}

func writePrivateJSON(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0600)
}
