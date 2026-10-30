package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newAuthCmd() *cobra.Command {
	var pin string

	cmd := &cobra.Command{
		Use:     "auth",
		Aliases: []string{"unlock"},
		Short:   "Authenticate User PIN and unlock the HSM (AT+AUTH)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if pin == "" && len(args) > 0 {
				pin = args[0]
			}
			if pin == "" {
				return fmt.Errorf("PIN is required: supply via --pin or positional argument")
			}

			client, cleanup, err := getATClient()
			if err != nil {
				return err
			}
			defer cleanup()

			if err := client.Auth(pin); err != nil {
				return fmt.Errorf("authentication failed: %w", err)
			}

			fmt.Println("✓ [OK] HSM Session Authenticated & Unlocked! LED transitioned to solid GREEN.")
			return nil
		},
	}

	cmd.Flags().StringVar(&pin, "pin", "", "User PIN (4..16 characters)")
	return cmd
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
	return &cobra.Command{
		Use:   "reset",
		Short: "Reset active session state (AT+RESET)",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, cleanup, err := getATClient()
			if err != nil {
				return err
			}
			defer cleanup()

			if err := client.Reset(); err != nil {
				return fmt.Errorf("reset failed: %w", err)
			}

			fmt.Println("✓ [OK] HSM Session Reset! Volatile context cleared.")
			return nil
		},
	}
}
