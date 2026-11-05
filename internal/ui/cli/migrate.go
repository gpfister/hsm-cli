package cli

import (
	"fmt"
	"os"

	"github.com/gpfister/nrf52840-hsm-cli/internal/domain"
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

			// Pre-flight check: homogeneous authentication validation
			srcTransport, err := serial.OpenAT(srcResolved)
			if err != nil {
				return fmt.Errorf("connecting to source HSM: %w", err)
			}
			srcClient := domain.NewATClient(srcTransport)
			srcStatus, err := srcClient.Status()
			_ = srcTransport.Close()
			if err != nil {
				return fmt.Errorf("failed to query source HSM status: %w", err)
			}

			tgtTransport, err := serial.OpenAT(tgtResolved)
			if err != nil {
				return fmt.Errorf("connecting to target HSM: %w", err)
			}
			tgtClient := domain.NewATClient(tgtTransport)
			tgtStatus, err := tgtClient.Status()
			_ = tgtTransport.Close()
			if err != nil {
				return fmt.Errorf("failed to query target HSM status: %w", err)
			}

			if err := verifyHomogeneousMigration(srcStatus, tgtStatus); err != nil {
				return err
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

// verifyHomogeneousMigration ensures both source and target devices are configured
// with identical authentication methods (PIN == PIN, SHAMIR == SHAMIR).
func verifyHomogeneousMigration(sourceStatus, targetStatus *domain.DeviceStatus) error {
	if !targetStatus.Configured {
		return fmt.Errorf("target HSM is not configured (method NONE). Initialize target HSM with method %s before migrating keys", sourceStatus.Method)
	}
	if !sourceStatus.Configured {
		return fmt.Errorf("source HSM is not configured (method NONE). Initialize source HSM before migrating keys")
	}
	if sourceStatus.Method != targetStatus.Method {
		return fmt.Errorf("%w: source device is using %s authentication, but target device is using %s authentication (both devices must use the same authentication method)",
			domain.ErrMismatchedAuthMethod, sourceStatus.Method, targetStatus.Method)
	}
	return nil
}
