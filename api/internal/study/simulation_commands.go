//go:build study_simulation

package study

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/spf13/cobra"
)

func registerSimulationCommands(app *pocketbase.PocketBase) {
	root := &cobra.Command{Use: "simulation", Short: "Test identity helpers (mappings are managed in PocketBase)"}
	var birthDate, serial string
	identity := &cobra.Command{Use: "identity", Args: cobra.NoArgs, Short: "Generate a personal-number fixture from a birth date", RunE: func(cmd *cobra.Command, args []string) error {
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
	root.AddCommand(identity)
	app.RootCmd.AddCommand(root)
}
