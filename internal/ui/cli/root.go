package cli

import (
	"fmt"
	"os"

	"github.com/gpfister/nrf52840-hms-cli/internal/infra/serial"
	"github.com/gpfister/nrf52840-hms-cli/internal/usecases"
	"github.com/spf13/cobra"
)

var (
	portFlag string
	rootCmd  = &cobra.Command{
		Use:   "hmsutil",
		Short: "hmsutil — CLI & TUI tool for nRF52840 Hardware Security Module Dongle",
		Long: `hmsutil is the command-line interface for managing and testing
the nRF52840-based Hardware Security Module (HMS) USB dongle, supporting
YubiHSM 2 API commands, dual-stage firmware updates, and interactive TUI.`,
	}
)

func init() {
	rootCmd.PersistentFlags().StringVarP(&portFlag, "port", "p", "", "Serial port of HSM data interface (auto-detected if omitted)")

	rootCmd.AddCommand(newFirmwareCmd())
	rootCmd.AddCommand(newInfoCmd())
	rootCmd.AddCommand(newEchoCmd())
	rootCmd.AddCommand(newAuthCmd())
	rootCmd.AddCommand(newLockCmd())
	rootCmd.AddCommand(newResetCmd())
	rootCmd.AddCommand(newTestCmd())
	rootCmd.AddCommand(newTUICmd())
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func getService() (*usecases.HSMService, func(), error) {
	portPath := portFlag
	if portPath == "" {
		ports, err := serial.FindDonglePorts()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to scan serial ports: %w", err)
		}
		if len(ports) == 0 {
			return nil, nil, fmt.Errorf("no nRF52840 HSM Dongle detected. Please plug in the device or specify --port")
		}
		// In dual CDC-ACM composite device, typically the second port is UART1 (HSM data)
		portPath = ports[len(ports)-1]
		fmt.Printf("[INFO] Auto-selected dongle port: %s\n", portPath)
	}

	transport, err := serial.Open(portPath)
	if err != nil {
		return nil, nil, err
	}

	cleanup := func() {
		_ = transport.Close()
	}

	return usecases.NewHSMService(transport), cleanup, nil
}
