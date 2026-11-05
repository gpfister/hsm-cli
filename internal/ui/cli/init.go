package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/gpfister/nrf52840-hsm-cli/internal/domain"
	"github.com/gpfister/nrf52840-hsm-cli/internal/usecases"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	var (
		modeFlag      string
		pinFlag       string
		thresholdMode bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize token in PIN or Shamir Secret Sharing authentication mode",
		Long: `Initialize the Hardware Security Module (HSM) in one of two mutually exclusive
authentication modes:
  1. PIN Mode: Protected by a high-complexity passphrase conforming to Yocto pam_pwquality policy.
  2. Shamir Secret Sharing: Master secret split across 5 custodians requiring a 3-of-5 threshold.`,
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
			if status.Configured {
				return fmt.Errorf("%w: device is already configured with method %s. Run 'gphsm device reset' first to reconfigure", domain.ErrAlreadyConfigured, status.Method)
			}

			mode := strings.ToLower(strings.TrimSpace(modeFlag))
			if thresholdMode {
				mode = "shamir"
			}

			// If mode not specified, check if PIN was supplied via flag or env
			if mode == "" {
				envPin := os.Getenv("GPHSM_PIN")
				if envPin == "" {
					envPin = os.Getenv("HSM_PIN")
				}
				if pinFlag != "" || envPin != "" {
					mode = "pin"
				}
			}

			// Interactive mode selection if still undetermined
			if mode == "" {
				if isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd()) {
					fmt.Fprintln(cmd.OutOrStdout(), "Select Authentication Mode:")
					fmt.Fprintln(cmd.OutOrStdout(), "  [1] PIN Mode (Password-Protected)")
					fmt.Fprintln(cmd.OutOrStdout(), "  [2] Shamir Secret Sharing (3-of-5 Threshold)")
					fmt.Fprint(cmd.OutOrStdout(), "Enter choice [1 or 2]: ")

					reader := bufio.NewReader(cmd.InOrStdin())
					line, _ := reader.ReadString('\n')
					choice := strings.TrimSpace(line)

					switch choice {
					case "1", "pin", "PIN":
						mode = "pin"
					case "2", "shamir", "SHAMIR", "threshold":
						mode = "shamir"
					default:
						return fmt.Errorf("invalid mode selection: %s", choice)
					}
				} else {
					return fmt.Errorf("initialization mode required: specify --mode=pin (--pin=<pin>) or --mode=shamir (or --threshold)")
				}
			}

			switch mode {
			case "pin":
				pin := pinFlag
				if pin == "" {
					pin = os.Getenv("GPHSM_PIN")
					if pin == "" {
						pin = os.Getenv("HSM_PIN")
					}
				}

				if pin == "" {
					if isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd()) {
						p, err := promptPINBubbleTeaWithTitle(
							"Enter New HSM PIN (min 10 chars, uppercase, lowercase, digit, special):",
							"Enter New HSM PIN",
						)
						if err != nil {
							return err
						}
						if err := domain.ValidatePIN(p); err != nil {
							return fmt.Errorf("PIN complexity validation failed: %w", err)
						}

						confirm, err := promptPINBubbleTeaWithTitle(
							"Confirm HSM PIN:",
							"Confirm HSM PIN",
						)
						if err != nil {
							return err
						}
						if confirm != p {
							return errors.New("PIN confirmation does not match")
						}
						pin = p
					} else {
						reader := bufio.NewReader(cmd.InOrStdin())
						line, err := reader.ReadString('\n')
						if err != nil && len(line) == 0 {
							return fmt.Errorf("PIN is required: supply via --pin, set GPHSM_PIN, or pipe to stdin")
						}
						pin = strings.TrimSpace(line)
					}
				}

				// Strictly validate PIN complexity against Yocto pam_pwquality policy
				if err := domain.ValidatePIN(pin); err != nil {
					return fmt.Errorf("PIN complexity validation failed: %w", err)
				}

				if err := client.InitPin(pin); err != nil {
					return fmt.Errorf("PIN initialization failed on HSM: %w", err)
				}

				fmt.Fprintln(cmd.OutOrStdout(), "✓ [OK] HSM Initialized in PIN mode!")
				fmt.Fprintln(cmd.OutOrStdout(), "Device is now locked (Solid RED LED). Run 'gphsm unlock' to authenticate.")
				return nil

			case "shamir", "threshold":
				ceremony := usecases.NewCeremonyServiceWithIO(cmd.InOrStdin(), cmd.OutOrStdout())
				if _, err := ceremony.RunTurnByTurnInit(client); err != nil {
					return fmt.Errorf("threshold ceremony failed: %w", err)
				}

				fmt.Fprintln(cmd.OutOrStdout(), "✓ [OK] HSM Initialized in Shamir Secret Sharing mode!")
				fmt.Fprintln(cmd.OutOrStdout(), "Device is now locked (Solid RED LED). Run 'gphsm unlock' to authenticate with 3 custodian shares.")
				return nil

			default:
				return fmt.Errorf("%w: got %q (must be 'pin' or 'shamir')", domain.ErrInvalidAuthMode, mode)
			}
		},
	}

	cmd.Flags().StringVarP(&modeFlag, "mode", "m", "", "Authentication mode: 'pin' or 'shamir'")
	cmd.Flags().StringVar(&pinFlag, "pin", "", "Initial User PIN for PIN mode (10..64 characters, pam_pwquality enforced)")
	cmd.Flags().BoolVarP(&thresholdMode, "threshold", "t", false, "Execute turn-by-turn 3-of-5 custodian initialization ceremony (alias for --mode=shamir)")

	return cmd
}
