package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/gpfister/nrf52840-hsm-cli/internal/infra/p11"
	"github.com/gpfister/nrf52840-hsm-cli/internal/usecases"
	"github.com/spf13/cobra"
)

func newP11Cmd() *cobra.Command {
	p11Cmd := &cobra.Command{
		Use:   "p11",
		Short: "PKCS#11 provider diagnostic, inspection, and compliance tools",
	}

	p11Cmd.AddCommand(newP11InfoCmd())
	p11Cmd.AddCommand(newP11SlotsCmd())
	p11Cmd.AddCommand(newP11TestCmd())

	return p11Cmd
}

func newP11InfoCmd() *cobra.Command {
	var modulePath string

	cmd := &cobra.Command{
		Use:   "info",
		Short: "Display PKCS#11 Cryptoki provider library metadata and versions",
		RunE: func(cmd *cobra.Command, args []string) error {
			if modulePath == "" {
				return fmt.Errorf("--module must be specified")
			}

			diagSvc := usecases.NewP11DiagService()
			info, err := diagSvc.ModuleInfo(modulePath)
			if err != nil {
				return fmt.Errorf("failed to query PKCS#11 module info: %w", err)
			}

			fmt.Println("==================================================")
			fmt.Println("               PKCS#11 Provider Info              ")
			fmt.Println("==================================================")
			fmt.Printf(" Module Path:         %s\n", modulePath)
			fmt.Printf(" Cryptoki Version:    %s\n", info.CryptokiVersion)
			fmt.Printf(" Manufacturer:        %s\n", info.CleanManufacturer())
			fmt.Printf(" Library Description: %s\n", info.CleanLibraryDesc())
			fmt.Printf(" Library Version:     %s\n", info.LibraryVersion)
			fmt.Println("==================================================")
			return nil
		},
	}

	cmd.Flags().StringVarP(&modulePath, "module", "m", "", "Path to PKCS#11 provider shared library (.dylib/.so/.dll)")
	_ = cmd.MarkFlagRequired("module")

	return cmd
}

func newP11SlotsCmd() *cobra.Command {
	var modulePath string

	cmd := &cobra.Command{
		Use:   "slots",
		Short: "Enumerate PKCS#11 slots, token descriptors, and supported mechanisms",
		RunE: func(cmd *cobra.Command, args []string) error {
			if modulePath == "" {
				return fmt.Errorf("--module must be specified")
			}

			diagSvc := usecases.NewP11DiagService()
			slots, err := diagSvc.SlotList(modulePath)
			if err != nil {
				return fmt.Errorf("failed to enumerate slots: %w", err)
			}

			if len(slots) == 0 {
				fmt.Println("No PKCS#11 slots discovered.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "SLOT ID\tDESCRIPTION\tTOKEN LABEL\tSERIAL NUMBER\tFLAGS")
			fmt.Fprintln(w, "-------\t-----------\t-----------\t-------------\t-----")
			for _, s := range slots {
				slotDesc := ""
				if s.SlotInfo != nil {
					slotDesc = s.SlotInfo.CleanDescription()
				}
				tokenLabel := "(no token)"
				tokenSerial := "-"
				flags := uint64(0)
				if s.TokenInfo != nil {
					tokenLabel = s.TokenInfo.CleanLabel()
					tokenSerial = s.TokenInfo.CleanSerial()
					flags = s.TokenInfo.Flags
				}
				fmt.Fprintf(w, "%d\t%s\t%s\t%s\t0x%08X\n", s.SlotID, slotDesc, tokenLabel, tokenSerial, flags)
			}
			_ = w.Flush()

			fmt.Println()
			for _, s := range slots {
				if len(s.Mechanisms) > 0 {
					fmt.Printf("Slot %d Supported Mechanisms (%d):\n", s.SlotID, len(s.Mechanisms))
					for _, m := range s.Mechanisms {
						fmt.Printf("  - %s\n", p11.MechanismName(m))
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&modulePath, "module", "m", "", "Path to PKCS#11 provider shared library (.dylib/.so/.dll)")
	_ = cmd.MarkFlagRequired("module")

	return cmd
}

func newP11TestCmd() *cobra.Command {
	var (
		modulePath string
		pin        string
	)

	cmd := &cobra.Command{
		Use:   "test",
		Short: "Run automated end-to-end PKCS#11 provider diagnostics against the module",
		RunE: func(cmd *cobra.Command, args []string) error {
			if modulePath == "" {
				return fmt.Errorf("--module must be specified")
			}
			if pin == "" {
				pin = "123456"
			}

			fmt.Println("==================================================")
			fmt.Println("       Running PKCS#11 Provider Diagnostics       ")
			fmt.Println("==================================================")
			fmt.Printf("Target Module: %s\n\n", modulePath)

			diagSvc := usecases.NewP11DiagService()
			steps, runErr := diagSvc.RunDiagnostics(modulePath, pin)

			allPassed := true
			for _, step := range steps {
				if step.Passed {
					fmt.Printf("[PASS] %s\n       -> %s\n", step.Name, step.Details)
				} else {
					allPassed = false
					fmt.Printf("[FAIL] %s\n       -> %s\n", step.Name, step.Details)
					if step.Error != nil {
						fmt.Printf("       -> Error: %v\n", step.Error)
					}
				}
			}

			fmt.Println("==================================================")
			if !allPassed || runErr != nil {
				fmt.Println(" ❌ PKCS#11 diagnostics encountered failures.")
				fmt.Println("==================================================")
				return fmt.Errorf("PKCS#11 diagnostics failed")
			}

			fmt.Println(" ✓ All subtests passed (PASS)")
			fmt.Println("==================================================")
			return nil
		},
	}

	cmd.Flags().StringVarP(&modulePath, "module", "m", "", "Path to PKCS#11 provider shared library (.dylib/.so/.dll)")
	cmd.Flags().StringVar(&pin, "pin", "123456", "Token User PIN for authentication test")
	_ = cmd.MarkFlagRequired("module")

	return cmd
}
