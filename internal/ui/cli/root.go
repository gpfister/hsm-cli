package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/gpfister/nrf52840-hsm-cli/internal/domain"
	"github.com/gpfister/nrf52840-hsm-cli/internal/infra/serial"
	"github.com/gpfister/nrf52840-hsm-cli/internal/usecases"
	"github.com/spf13/cobra"
)

var (
	portFlag   string
	serialFlag string
	rootCmd    = &cobra.Command{
		Use:   "hsmutil",
		Short: "hsmutil — CLI & TUI tool for nRF52840 Hardware Security Module Dongle",
		Long: `hsmutil is the command-line interface for managing and testing
the nRF52840-based Hardware Security Module (HSM) USB dongle, supporting
AT-style ASCII serial protocol, PKCS#11 diagnostics, and dual-stage secure boot.`,
	}
)

func init() {
	rootCmd.PersistentFlags().StringVarP(&portFlag, "port", "p", "", "Serial port of HSM data interface (auto-detected if omitted or from NRF52840_HSM_PORT)")
	rootCmd.PersistentFlags().StringVarP(&serialFlag, "serial", "s", "", "Target specific HSM dongle by 12-digit hex BLE address or serial number (or from NRF52840_HSM_SERIAL)")

	// AT protocol device commands
	rootCmd.AddCommand(newDevicesCmd())
	rootCmd.AddCommand(newInfoCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newAuthCmd())
	rootCmd.AddCommand(newInitCmd())
	rootCmd.AddCommand(newLockCmd())
	rootCmd.AddCommand(newResetCmd())
	rootCmd.AddCommand(newKeyCmd())
	rootCmd.AddCommand(newRandomCmd())
	rootCmd.AddCommand(newEchoCmd())

	// PKCS#11 diagnostics
	rootCmd.AddCommand(newP11Cmd())

	// Firmware DFU, integration test, and interactive TUI
	rootCmd.AddCommand(newFirmwareCmd())
	rootCmd.AddCommand(newTestCmd())
	rootCmd.AddCommand(newTUICmd())
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func getATClient() (*domain.ATClient, func(), error) {
	portPath := portFlag
	if portPath == "" {
		targetSerial := serialFlag
		if targetSerial == "" {
			targetSerial = os.Getenv("NRF52840_HSM_SERIAL")
		}
		discovered, err := serial.AutoDiscoverPortTargeting(targetSerial)
		if err != nil {
			var multErr *domain.ErrMultipleDevicesDetected
			if errors.As(err, &multErr) {
				return nil, nil, multErr
			}
			return nil, nil, fmt.Errorf("no nRF52840 HSM Dongle detected: %w (plug in device, specify --port or --serial, or set NRF52840_HSM_PORT/NRF52840_HSM_SERIAL)", err)
		}
		portPath = discovered
	}

	transport, err := serial.OpenAT(portPath)
	if err != nil {
		return nil, nil, err
	}

	cleanup := func() {
		_ = transport.Close()
	}

	return domain.NewATClient(transport), cleanup, nil
}

func getService() (*usecases.HSMService, func(), error) {
	// First attempt AT client connection
	atClient, atCleanup, atErr := getATClient()
	if atErr == nil {
		return usecases.NewHSMServiceWithAT(atClient), atCleanup, nil
	}

	var multErr *domain.ErrMultipleDevicesDetected
	if errors.As(atErr, &multErr) {
		return nil, nil, multErr
	}

	// Fallback to legacy binary serial transport if AT fails
	portPath := portFlag
	if portPath == "" {
		targetSerial := serialFlag
		if targetSerial == "" {
			targetSerial = os.Getenv("NRF52840_HSM_SERIAL")
		}
		ports, err := serial.FindDonglePortsTargeting(targetSerial)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to scan serial ports: %w", err)
		}
		if len(ports) == 0 {
			return nil, nil, fmt.Errorf("no nRF52840 HSM Dongle detected. Please plug in the device or specify --port / --serial")
		}
		portPath = ports[0]
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
