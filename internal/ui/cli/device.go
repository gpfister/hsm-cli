package cli

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

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
