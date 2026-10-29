package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Query YubiHSM 2 device information and status",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, cleanup, err := getService()
			if err != nil {
				return err
			}
			defer cleanup()

			info, err := svc.GetDeviceInfo()
			if err != nil {
				return fmt.Errorf("failed to get device info: %w", err)
			}

			fmt.Println("==================================================")
			fmt.Println("             YubiHSM 2 Device Info                ")
			fmt.Println("==================================================")
			fmt.Printf(" Version:       %s\n", info.VersionString())
			fmt.Printf(" Serial Number: %d (0x%08X)\n", info.Serial, info.Serial)
			fmt.Printf(" Log Store:     %d / %d entries used\n", info.LogUsed, info.LogTotal)
			if len(info.Algorithms) > 0 {
				fmt.Printf(" Algorithms:    %d supported\n", len(info.Algorithms))
			}
			fmt.Println("==================================================")
			return nil
		},
	}
}

func newEchoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "echo [message]",
		Short: "Send echo packet to dongle to verify round-trip latency",
		RunE: func(cmd *cobra.Command, args []string) error {
			msg := "Ping from hmsutil"
			if len(args) > 0 {
				msg = args[0]
			}

			svc, cleanup, err := getService()
			if err != nil {
				return err
			}
			defer cleanup()

			resp, latency, err := svc.Echo(msg)
			if err != nil {
				return fmt.Errorf("echo failed: %w", err)
			}

			fmt.Printf("✓ [OK] Echo reply received in %v: %q\n", latency, resp)
			return nil
		},
	}
}

func newResetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reset",
		Short: "Reset device state back to unconfigured (LED transitions to pulsating blue)",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, cleanup, err := getService()
			if err != nil {
				return err
			}
			defer cleanup()

			if err := svc.Reset(); err != nil {
				return fmt.Errorf("reset failed: %w", err)
			}

			fmt.Println("✓ [OK] Device state reset to Unconfigured. LED should be slowly pulsating BLUE.")
			return nil
		},
	}
}
