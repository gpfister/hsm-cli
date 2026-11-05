package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/gpfister/nrf52840-hsm-cli/internal/usecases"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

type pinModel struct {
	textInput textinput.Model
	err       error
	value     string
	aborted   bool
}

func initialPinModel() pinModel {
	ti := textinput.New()
	ti.Placeholder = "Enter HSM PIN"
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '•'
	ti.Focus()
	ti.CharLimit = 64
	ti.Width = 30

	return pinModel{
		textInput: ti,
	}
}

func (m pinModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m pinModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			m.value = m.textInput.Value()
			return m, tea.Quit
		case tea.KeyCtrlC, tea.KeyEsc:
			m.aborted = true
			return m, tea.Quit
		}
	case error:
		m.err = msg
		return m, nil
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m pinModel) View() string {
	return fmt.Sprintf(
		"Enter HSM PIN:\n\n%s\n\n(press Enter to submit, Esc to cancel)\n",
		m.textInput.View(),
	)
}

func promptPINBubbleTea() (string, error) {
	p := tea.NewProgram(initialPinModel())
	m, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("interactive PIN prompt failed: %w", err)
	}
	finalModel, ok := m.(pinModel)
	if !ok || finalModel.aborted {
		return "", errors.New("PIN entry cancelled")
	}
	if finalModel.err != nil {
		return "", finalModel.err
	}
	return finalModel.value, nil
}

func newUnlockCmd() *cobra.Command {
	var pin string
	var thresholdMode bool

	cmd := &cobra.Command{
		Use:     "unlock",
		Aliases: []string{"auth"},
		Short:   "Authenticate User PIN or execute threshold ceremony to unlock the HSM",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, cleanup, err := getATClient()
			if err != nil {
				return err
			}
			defer cleanup()

			if thresholdMode {
				ceremony := usecases.NewCeremonyService()
				if _, err := ceremony.RunTurnByTurnAuth(client); err != nil {
					return fmt.Errorf("threshold authentication ceremony failed: %w", err)
				}
				return nil
			}

			if pin == "" && len(args) > 0 {
				pin = args[0]
			}
			if pin == "" {
				// Interactive TTY detection
				if isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd()) {
					p, err := promptPINBubbleTea()
					if err != nil {
						return err
					}
					pin = p
				} else {
					// Non-interactive / headless fallback: read single line from stdin
					reader := bufio.NewReader(cmd.InOrStdin())
					line, err := reader.ReadString('\n')
					if err == nil || len(line) > 0 {
						pin = strings.TrimSpace(line)
					}
				}
			}
			if pin == "" {
				return fmt.Errorf("PIN is required: supply via --pin, pipe to stdin, or use --threshold")
			}

			if err := client.Auth(pin); err != nil {
				return fmt.Errorf("authentication failed: %w", err)
			}

			fmt.Println("✓ [OK] HSM Session Authenticated & Unlocked! LED transitioned to solid GREEN.")
			return nil
		},
	}

	cmd.Flags().StringVar(&pin, "pin", "", "User PIN (4..16 characters)")
	cmd.Flags().BoolVarP(&thresholdMode, "threshold", "t", false, "Execute turn-by-turn custodian threshold unlock ceremony")
	return cmd
}

// newAuthCmd returns an unlock command for backward compatibility.
func newAuthCmd() *cobra.Command {
	return newUnlockCmd()
}

func newLockCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lock",
		Short: "Invalidate authentication session and lock the HSM (AT+LOCK)",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, cleanup, err := getATClient()
			if err != nil {
				return err
			}
			defer cleanup()

			if err := client.Lock(); err != nil {
				return fmt.Errorf("lock failed: %w", err)
			}

			fmt.Println("✓ [OK] HSM Locked! LED transitioned to solid RED.")
			return nil
		},
	}
}

func newResetCmd() *cobra.Command {
	var yesFlag bool

	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Factory reset device, zeroizing all persistent keys and Shamir secrets (AT+RESET)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yesFlag {
				fmt.Fprintln(cmd.OutOrStdout(), "WARNING: This operation will perform a destructive factory reset on the HSM.")
				fmt.Fprintln(cmd.OutOrStdout(), "All persistent private keys, cryptographic material, and Shamir secrets will be permanently destroyed.")
				fmt.Fprintln(cmd.OutOrStdout(), "The device will transition to an unconfigured state (pulsating blue LED).")
				fmt.Fprint(cmd.OutOrStdout(), "\nAre you sure you want to proceed? (yes/no): ")

				reader := bufio.NewReader(cmd.InOrStdin())
				line, _ := reader.ReadString('\n')
				ans := strings.ToLower(strings.TrimSpace(line))
				if ans != "y" && ans != "yes" {
					fmt.Fprintln(cmd.OutOrStdout(), "Reset cancelled.")
					return nil
				}
			}

			client, cleanup, err := getATClient()
			if err != nil {
				return err
			}
			defer cleanup()

			if err := client.Reset(); err != nil {
				return fmt.Errorf("reset failed: %w", err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), "✓ [OK] HSM Factory Reset Complete!")
			fmt.Fprintln(cmd.OutOrStdout(), "All persistent keys and Shamir secrets have been wiped.")
			fmt.Fprintln(cmd.OutOrStdout(), "The device is now in an unconfigured state. Run 'hsmutil device init --threshold' to reconfigure.")
			return nil
		},
	}

	cmd.Flags().BoolVarP(&yesFlag, "yes", "y", false, "Confirm destructive factory reset without interactive prompt")
	return cmd
}
