package cli

import (
	"fmt"
	"os"

	"github.com/gpfister/nrf52840-hsm-cli/internal/infra/serial"
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
		Short: "Migrate a cryptographic key slot between two physical HSM keys",
		Long: `Migrate a private key slot from a source HSM to a target HSM using
end-to-end ephemeral ECDH key wrapping, turn-by-turn custodian verification, and mutual re-locking.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if targetPort == "" {
				return fmt.Errorf("--target port or serial must be specified")
			}
			if slotID <= 0 {
				return fmt.Errorf("--slot ID must be specified (>= 1)")
			}

			if sourcePort == "" {
				sourcePort = serialFlag
				if sourcePort == "" {
					sourcePort = portFlag
				}
				if sourcePort == "" {
					sourcePort = os.Getenv("HSM_SERIAL")
					if sourcePort == "" {
						sourcePort = os.Getenv("NRF52840_HSM_SERIAL")
					}
				}
			}
			if sourcePort == "" {
				discovered, err := serial.AutoDiscoverPortTargeting("")
				if err != nil {
					return fmt.Errorf("source device auto-discovery failed: %w (specify --source)", err)
				}
				sourcePort = discovered
			}

			srcResolved, err := serial.ResolvePortOrSerial(sourcePort)
			if err != nil {
				return fmt.Errorf("resolving --source: %w", err)
			}
			tgtResolved, err := serial.ResolvePortOrSerial(targetPort)
			if err != nil {
				return fmt.Errorf("resolving --target: %w", err)
			}

			migrationSvc := usecases.NewMigrationService(nil, cmd.OutOrStdout())
			if err := migrationSvc.MigratePorts(srcResolved, tgtResolved, slotID); err != nil {
				return fmt.Errorf("key migration failed: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&sourcePort, "source", "", "Serial port or serial/BLE address of source HSM (auto-detected if omitted)")
	cmd.Flags().StringVar(&targetPort, "target", "", "Serial port or serial/BLE address of target HSM")
	cmd.Flags().IntVar(&slotID, "slot", 0, "Key slot ID to migrate")

	_ = cmd.MarkFlagRequired("target")
	_ = cmd.MarkFlagRequired("slot")

	return cmd
}
