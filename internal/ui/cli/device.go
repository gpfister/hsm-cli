package cli

import (
	"bufio"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gpfister/nrf52840-hsm-cli/internal/domain"
	"github.com/gpfister/nrf52840-hsm-cli/internal/infra/serial"
	"github.com/gpfister/nrf52840-hsm-cli/internal/usecases"
	"github.com/spf13/cobra"
)

func newDeviceCmd() *cobra.Command {
	deviceCmd := &cobra.Command{
		Use:   "device",
		Short: "Manage HSM device lifecycle, configuration, firmware, and cloning",
	}

	deviceCmd.AddCommand(newDeviceListCmd())
	deviceCmd.AddCommand(newDeviceInfoCmd())
	deviceCmd.AddCommand(newInitCmd())
	deviceCmd.AddCommand(newLockCmd())
	deviceCmd.AddCommand(newResetCmd())
	deviceCmd.AddCommand(newDeviceStatusCmd())
	deviceCmd.AddCommand(newDeviceMigrateCmd())
	deviceCmd.AddCommand(newFirmwareCmd())

	return deviceCmd
}

func runDeviceList(jsonOutput bool) error {
	dongles, err := serial.EnumerateDongles()
	if err != nil {
		return fmt.Errorf("failed to enumerate devices: %w", err)
	}

	target := serialFlag
	if target == "" {
		target = os.Getenv("HSM_SERIAL")
		if target == "" {
			target = os.Getenv("NRF52840_HSM_SERIAL")
		}
	}
	if target != "" {
		var filtered []domain.DiscoveredDongle
		normTarget := serial.NormalizeSerial(target)
		for _, d := range dongles {
			if serial.NormalizeSerial(d.Serial) == normTarget ||
				strings.HasSuffix(serial.NormalizeSerial(d.Serial), normTarget) ||
				strings.Contains(strings.ToUpper(d.DataPort), normTarget) {
				filtered = append(filtered, d)
			}
		}
		dongles = filtered
	}

	if jsonOutput {
		data, err := json.MarshalIndent(dongles, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	if len(dongles) == 0 {
		if target != "" {
			fmt.Printf("No HSM USB keys detected matching %q.\n", target)
		} else {
			fmt.Println("No HSM USB keys detected.")
		}
		return nil
	}

	fmt.Printf("Connected HSM Keys (%d detected):\n\n", len(dongles))
	for i, d := range dongles {
		fmt.Printf("[%d] Serial / BLE:  %s\n", i+1, d.Serial)
		fmt.Printf("    AT Data Port:  %s\n", d.DataPort)
		if d.ConsolePort != "" {
			fmt.Printf("    Console Port:  %s\n", d.ConsolePort)
		}
		if d.VID != "" && d.PID != "" {
			fmt.Printf("    USB ID:        %s:%s", d.VID, d.PID)
			if d.Product != "" {
				fmt.Printf(" (%s)", d.Product)
			}
			fmt.Println()
		}
		if i < len(dongles)-1 {
			fmt.Println()
		}
	}
	return nil
}

func newDeviceListCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"devices", "ls"},
		Short:   "List connected HSM keys with detected BLE addresses and ports",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDeviceList(jsonOutput)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output discovered devices in JSON format")
	return cmd
}

func newDevicesCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:     "devices",
		Aliases: []string{"list"},
		Short:   "List connected HSM keys with detected BLE addresses and ports",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDeviceList(jsonOutput)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output discovered devices in JSON format")
	return cmd
}

func newDeviceInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Query HSM device build, hardware, and bootloader status",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, cleanup, err := getATClient()
			if err != nil {
				return err
			}
			defer cleanup()

			info, err := client.Info()
			if err != nil {
				return fmt.Errorf("failed to query device info: %w", err)
			}

			fmt.Println("==================================================")
			fmt.Println("                 HSM Device Info                  ")
			fmt.Println("==================================================")
			fmt.Printf(" Port:           %s\n", client.Transport().PortName())
			if info.Firmware != "" {
				fmt.Printf(" Firmware:       %s\n", info.Firmware)
			} else {
				fmt.Printf(" Version:        %s\n", info.VersionString())
			}
			if info.Hardware != "" {
				fmt.Printf(" Hardware:       %s\n", info.Hardware)
			}
			if info.UID != "" {
				fmt.Printf(" Device UID:     %s\n", info.UID)
			}
			if info.Boot != "" {
				fmt.Printf(" Bootloader:     %s\n", info.Boot)
			}
			if info.Serial != 0 {
				fmt.Printf(" Serial:         %d (0x%08X)\n", info.Serial, info.Serial)
			}
			fmt.Println("==================================================")
			return nil
		},
	}
}

func newInfoCmd() *cobra.Command {
	return newDeviceInfoCmd()
}

func newDeviceStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Query authentication state, remaining PIN retries, and stored key count",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, cleanup, err := getATClient()
			if err != nil {
				return err
			}
			defer cleanup()

			status, err := client.Status()
			if err != nil {
				return fmt.Errorf("failed to query device status: %w", err)
			}

			fmt.Println("==================================================")
			fmt.Println("                    HSM Status                    ")
			fmt.Println("==================================================")
			if status.Authenticated {
				fmt.Println(" Authentication: Unlocked (Authenticated)")
			} else {
				fmt.Println(" Authentication: Locked (Authentication Required)")
			}
			fmt.Printf(" PIN Retries:    %d remaining\n", status.RetriesRemaining)
			fmt.Printf(" Stored Keys:    %d\n", status.KeyCount)
			fmt.Println("==================================================")
			return nil
		},
	}
}

func newStatusCmd() *cobra.Command {
	return newDeviceStatusCmd()
}

func newDeviceMigrateCmd() *cobra.Command {
	var (
		sourcePort    string
		targetPort    string
		zeroizeSource bool
		yesFlag       bool
	)

	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate all cryptographic key slots from source HSM to target HSM",
		Long: `Full device migration: discovers all occupied slots on the source HSM,
encrypts and transfers each slot using target ephemeral ECDH wrapping, verifies target digests,
and executes fail-fast lockdown on any error.
Optionally zeroizes the source device upon completion (--zeroize-source).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if targetPort == "" {
				return fmt.Errorf("--target port or serial must be specified")
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

			if zeroizeSource && !yesFlag {
				fmt.Fprintln(cmd.OutOrStdout(), "WARNING: You have selected --zeroize-source.")
				fmt.Fprintln(cmd.OutOrStdout(), "Upon successful migration and verification of all keys, the source HSM will be permanently wiped (factory reset).")
				fmt.Fprint(cmd.OutOrStdout(), "\nAre you sure you want to proceed? (yes/no): ")

				reader := bufio.NewReader(cmd.InOrStdin())
				line, _ := reader.ReadString('\n')
				ans := strings.ToLower(strings.TrimSpace(line))
				if ans != "y" && ans != "yes" {
					fmt.Fprintln(cmd.OutOrStdout(), "Device migration cancelled.")
					return nil
				}
			}

			migrationSvc := usecases.NewMigrationService(nil, cmd.OutOrStdout())
			if err := migrationSvc.MigrateDevicePorts(srcResolved, tgtResolved, zeroizeSource); err != nil {
				return fmt.Errorf("device migration failed: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&targetPort, "target", "t", "", "Serial port or serial/BLE address of target HSM")
	cmd.Flags().StringVarP(&sourcePort, "source", "s", "", "Serial port or serial/BLE address of source HSM (auto-detected if omitted)")
	cmd.Flags().BoolVar(&zeroizeSource, "zeroize-source", false, "Destructively wipe and decommission source HSM after successful migration")
	cmd.Flags().BoolVarP(&yesFlag, "yes", "y", false, "Confirm device migration and source zeroization without interactive prompt")

	_ = cmd.MarkFlagRequired("target")

	return cmd
}

func newEchoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "echo [message]",
		Short: "Send ping packet to verify link responsiveness and latency",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, cleanup, err := getATClient()
			if err != nil {
				return err
			}
			defer cleanup()

			start := time.Now()
			if err := client.Ping(); err != nil {
				return fmt.Errorf("ping failed: %w", err)
			}
			latency := time.Since(start)

			fmt.Printf("✓ HSM ping response OK (round-trip latency: %v)\n", latency)
			return nil
		},
	}
}

func newRandomCmd() *cobra.Command {
	var length int

	cmd := &cobra.Command{
		Use:   "random",
		Short: "Generate hardware random bytes using TRNG",
		RunE: func(cmd *cobra.Command, args []string) error {
			if length < 1 || length > 128 {
				return fmt.Errorf("--len must be between 1 and 128 bytes (got %d)", length)
			}

			client, cleanup, err := getATClient()
			if err != nil {
				return err
			}
			defer cleanup()

			bytes, err := client.Random(length)
			if err != nil {
				return fmt.Errorf("random byte generation failed: %w", err)
			}

			fmt.Printf("Random Bytes (%d bytes):\n", len(bytes))
			fmt.Printf("  HEX:    %s\n", hex.EncodeToString(bytes))
			fmt.Printf("  BASE64: %s\n", base64.StdEncoding.EncodeToString(bytes))
			return nil
		},
	}

	cmd.Flags().IntVarP(&length, "len", "n", 32, "Number of random bytes to generate (1..128)")
	return cmd
}
