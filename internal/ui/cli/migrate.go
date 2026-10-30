package cli

import (
	"fmt"
	"os"

	"github.com/gpfister/nrf52840-hsm-cli/internal/usecases"
	"github.com/spf13/cobra"
)

func newKeyMigrateCmd() *cobra.Command {
	var (
		sourcePort string
		targetPort string
		slotID     int
	)

	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate a cryptographic key slot between two physical HSM dongles",
		Long: `Migrate a private key slot from a source HSM to a target HSM using
end-to-end ephemeral ECDH key wrapping, turn-by-turn custodian verification, and mutual re-locking.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if sourcePort == "" {
				return fmt.Errorf("--source port must be specified")
			}
			if targetPort == "" {
				return fmt.Errorf("--target port must be specified")
			}
			if slotID <= 0 {
				return fmt.Errorf("--slot ID must be specified (>= 1)")
			}

			migrationSvc := usecases.NewMigrationService(nil, os.Stdout)
			if err := migrationSvc.MigratePorts(sourcePort, targetPort, slotID); err != nil {
				return fmt.Errorf("key migration failed: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&sourcePort, "source", "", "Serial port of source HSM dongle")
	cmd.Flags().StringVar(&targetPort, "target", "", "Serial port of target HSM dongle")
	cmd.Flags().IntVar(&slotID, "slot", 0, "Key slot ID to migrate")

	_ = cmd.MarkFlagRequired("source")
	_ = cmd.MarkFlagRequired("target")
	_ = cmd.MarkFlagRequired("slot")

	return cmd
}
