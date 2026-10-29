package cli

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newFirmwareCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "firmware",
		Short: "Manage dongle firmware (info, update)",
	}

	cmd.AddCommand(newFirmwareInfoCmd())
	cmd.AddCommand(newFirmwareUpdateCmd())
	return cmd
}

func newFirmwareInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Query active firmware version and MCUboot partition status",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, cleanup, err := getService()
			if err != nil {
				return err
			}
			defer cleanup()

			info, err := svc.GetFirmwareInfo()
			if err != nil {
				return fmt.Errorf("failed to get firmware info: %w", err)
			}

			fmt.Println("==================================================")
			fmt.Println("              Dongle Firmware Info                ")
			fmt.Println("==================================================")
			fmt.Printf(" Firmware Version: %s\n", info.VersionString())
			fmt.Printf(" Active Partition: %s\n", info.SlotString())
			fmt.Printf(" Image Size:       %d bytes (0x%X)\n", info.ImageSize, info.ImageSize)
			if len(info.ImageSHA256) > 0 {
				fmt.Printf(" Image SHA-256:    %s\n", hex.EncodeToString(info.ImageSHA256))
			}
			fmt.Println("==================================================")
			return nil
		},
	}
}

func newFirmwareUpdateCmd() *cobra.Command {
	var filePath string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update firmware from a signed binary file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if filePath == "" {
				return fmt.Errorf("--file flag is required")
			}

			svc, cleanup, err := getService()
			if err != nil {
				return err
			}
			defer cleanup()

			fmt.Printf("[INFO] Starting firmware streaming from: %s\n", filePath)

			progressFn := func(sent, total int, pct float64) {
				barWidth := 30
				filled := int((pct / 100.0) * float64(barWidth))
				bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
				fmt.Printf("\r[%s] %5.1f%% (%d / %d bytes)", bar, pct, sent, total)
			}

			err = svc.UpdateFirmware(filePath, progressFn)
			fmt.Println()

			if err != nil {
				return fmt.Errorf("firmware update failed: %w", err)
			}

			fmt.Println("✓ [SUCCESS] Firmware update streamed and candidate slot validated.")
			fmt.Println("✓ [INFO] Device has been requested to swap into MCUboot.")
			return nil
		},
	}

	cmd.Flags().StringVarP(&filePath, "file", "f", "", "Path to signed firmware binary (.bin)")
	_ = cmd.MarkFlagRequired("file")

	return cmd
}
