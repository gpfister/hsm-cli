package cli

import (
	"fmt"

	"github.com/gpfister/nrf52840-hsm-cli/internal/usecases"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	var thresholdMode bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize token or execute custodian threshold ceremony",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !thresholdMode {
				return fmt.Errorf("initialization mode required (specify --threshold for 3-of-5 custodian ceremony)")
			}

			client, cleanup, err := getATClient()
			if err != nil {
				return err
			}
			defer cleanup()

			ceremony := usecases.NewCeremonyService()
			_, err = ceremony.RunTurnByTurnInit(client)
			if err != nil {
				return fmt.Errorf("threshold ceremony failed: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&thresholdMode, "threshold", "t", false, "Execute turn-by-turn 3-of-5 custodian initialization ceremony")

	return cmd
}
