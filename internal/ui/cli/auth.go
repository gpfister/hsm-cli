package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newAuthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unlock",
		Aliases: []string{"auth"},
		Short: "Authenticate session and unlock the HSM (LED transitions to solid GREEN)",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, cleanup, err := getService()
			if err != nil {
				return err
			}
			defer cleanup()

			if err := svc.Unlock(); err != nil {
				return fmt.Errorf("unlock failed: %w", err)
			}

			fmt.Println("✓ [OK] HSM Session Authenticated & Unlocked! LED should be solid GREEN.")
			return nil
		},
	}
}

func newLockCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lock",
		Short: "Lock the HSM session (LED transitions to solid RED)",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, cleanup, err := getService()
			if err != nil {
				return err
			}
			defer cleanup()

			if err := svc.Lock(); err != nil {
				return fmt.Errorf("lock failed: %w", err)
			}

			fmt.Println("✓ [OK] HSM Locked! LED should be solid RED.")
			return nil
		},
	}
}
