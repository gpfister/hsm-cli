package cli

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gpfister/nrf52840-hsm-cli/internal/domain"
	"github.com/gpfister/nrf52840-hsm-cli/internal/infra/serial"
	"github.com/spf13/cobra"
)

func newDevicesCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:     "devices",
		Aliases: []string{"list"},
		Short:   "List connected nRF52840 HSM dongles with detected BLE addresses and ports",
		RunE: func(cmd *cobra.Command, args []string) error {
			dongles, err := serial.EnumerateDongles()
			if err != nil {
				return fmt.Errorf("failed to enumerate dongles: %w", err)
			}

			target := serialFlag
			if target == "" {
				target = os.Getenv("NRF52840_HSM_SERIAL")
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
					fmt.Printf("No nRF52840 HSM dongles detected matching %q.\n", target)
				} else {
					fmt.Println("No nRF52840 HSM dongles detected.")
				}
				return nil
			}

			fmt.Printf("Connected nRF52840 HSM Dongles (%d detected):\n\n", len(dongles))
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
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output discovered devices in JSON format")
	return cmd
}

func newInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Query nRF52840 HSM device build, hardware, and bootloader status",
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
			fmt.Println("              nRF52840 HSM Device Info            ")
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

func newStatusCmd() *cobra.Command {
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
			fmt.Println("               nRF52840 HSM Status                ")
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

func newEchoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "echo [message]",
		Short: "Send ping packet to dongle to verify link responsiveness and latency",
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
		Short: "Generate hardware random bytes using CC310 TRNG (AT+RANDOM)",
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
