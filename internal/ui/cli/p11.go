package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/gpfister/nrf52840-hsm-cli/internal/infra/p11"
	"github.com/gpfister/nrf52840-hsm-cli/internal/usecases"
	"github.com/spf13/cobra"
)

func newP11Cmd() *cobra.Command {
	pkcs11Cmd := &cobra.Command{
		Use:     "pkcs11",
		Aliases: []string{"p11"},
		Short:   "PKCS#11 provider diagnostic, inspection, and token management tools",
	}

	pkcs11Cmd.AddCommand(newP11InfoCmd())
	pkcs11Cmd.AddCommand(newP11SlotsCmd())
	pkcs11Cmd.AddCommand(newP11InitTokenCmd())
	pkcs11Cmd.AddCommand(newP11KeygenCmd())
	pkcs11Cmd.AddCommand(newP11SignCmd())
	pkcs11Cmd.AddCommand(newP11TestCmd())

	return pkcs11Cmd
}

func resolveModule(explicit string) (string, error) {
	return p11.FindModule(explicit)
}

func newP11InfoCmd() *cobra.Command {
	var modulePath string

	cmd := &cobra.Command{
		Use:   "info",
		Short: "Display PKCS#11 Cryptoki provider library metadata and versions",
		RunE: func(cmd *cobra.Command, args []string) error {
			mod, err := resolveModule(modulePath)
			if err != nil {
				return err
			}

			diagSvc := usecases.NewP11DiagService()
			info, err := diagSvc.ModuleInfo(mod)
			if err != nil {
				return fmt.Errorf("failed to query PKCS#11 module info: %w", err)
			}

			fmt.Println("==================================================")
			fmt.Println("               PKCS#11 Provider Info              ")
			fmt.Println("==================================================")
			fmt.Printf(" Module Path:         %s\n", mod)
			fmt.Printf(" Cryptoki Version:    %s\n", info.CryptokiVersion)
			fmt.Printf(" Manufacturer:        %s\n", info.CleanManufacturer())
			fmt.Printf(" Library Description: %s\n", info.CleanLibraryDesc())
			fmt.Printf(" Library Version:     %s\n", info.LibraryVersion)
			fmt.Println("==================================================")
			return nil
		},
	}

	cmd.Flags().StringVarP(&modulePath, "module", "m", "", "Path to PKCS#11 provider shared library (.dylib/.so/.dll)")
	return cmd
}

func newP11SlotsCmd() *cobra.Command {
	var modulePath string

	cmd := &cobra.Command{
		Use:   "slots",
		Short: "Enumerate PKCS#11 slots, token descriptors, and supported mechanisms",
		RunE: func(cmd *cobra.Command, args []string) error {
			mod, err := resolveModule(modulePath)
			if err != nil {
				return err
			}

			diagSvc := usecases.NewP11DiagService()
			slots, err := diagSvc.SlotList(mod)
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
	return cmd
}

func newP11InitTokenCmd() *cobra.Command {
	var (
		modulePath string
		slotID     uint64
		pin        string
		label      string
	)

	cmd := &cobra.Command{
		Use:   "init-token",
		Short: "Initialize token on specified slot (C_InitToken)",
		RunE: func(cmd *cobra.Command, args []string) error {
			mod, err := resolveModule(modulePath)
			if err != nil {
				return err
			}

			diagSvc := usecases.NewP11DiagService()
			if err := diagSvc.InitToken(mod, uintptr(slotID), pin, label); err != nil {
				return fmt.Errorf("token initialization failed: %w", err)
			}

			fmt.Printf("✓ [OK] Token on slot %d successfully initialized (Label: %q).\n", slotID, label)
			return nil
		},
	}

	cmd.Flags().StringVarP(&modulePath, "module", "m", "", "Path to PKCS#11 provider shared library (.dylib/.so/.dll)")
	cmd.Flags().Uint64VarP(&slotID, "slot", "s", 1, "Slot ID to initialize")
	cmd.Flags().StringVar(&pin, "pin", "123456", "Token SO or User PIN")
	cmd.Flags().StringVarP(&label, "label", "l", "nRF52840-HSM", "Token label (up to 32 characters)")

	return cmd
}

func newP11KeygenCmd() *cobra.Command {
	var (
		modulePath string
		slotID     uint64
		pin        string
		keyID      int
		label      string
	)

	cmd := &cobra.Command{
		Use:   "keygen",
		Short: "Generate EC SECP256R1 keypair via PKCS#11 (C_GenerateKeyPair)",
		RunE: func(cmd *cobra.Command, args []string) error {
			mod, err := resolveModule(modulePath)
			if err != nil {
				return err
			}

			diagSvc := usecases.NewP11DiagService()
			hPub, hPriv, err := diagSvc.GenerateKeyPair(mod, uintptr(slotID), pin, keyID, label)
			if err != nil {
				return fmt.Errorf("PKCS#11 keypair generation failed: %w", err)
			}

			fmt.Printf("✓ [OK] EC P-256 Keypair generated successfully!\n")
			fmt.Printf("  Slot ID:        %d\n", slotID)
			fmt.Printf("  Target Key ID:  %d\n", keyID)
			fmt.Printf("  Key Label:      %s\n", label)
			fmt.Printf("  Public Handle:  0x%08X\n", hPub)
			fmt.Printf("  Private Handle: 0x%08X\n", hPriv)
			return nil
		},
	}

	cmd.Flags().StringVarP(&modulePath, "module", "m", "", "Path to PKCS#11 provider shared library (.dylib/.so/.dll)")
	cmd.Flags().Uint64VarP(&slotID, "slot", "s", 1, "Slot ID")
	cmd.Flags().StringVar(&pin, "pin", "123456", "User PIN for authentication")
	cmd.Flags().IntVarP(&keyID, "id", "i", 1, "Key slot ID (1..8)")
	cmd.Flags().StringVarP(&label, "label", "l", "EC-Key", "Key label")

	return cmd
}

func newP11SignCmd() *cobra.Command {
	var (
		modulePath string
		slotID     uint64
		pin        string
		keyID      int
		dataStr    string
	)

	cmd := &cobra.Command{
		Use:   "sign",
		Short: "Compute ECDSA signature via PKCS#11 (C_Sign)",
		RunE: func(cmd *cobra.Command, args []string) error {
			mod, err := resolveModule(modulePath)
			if err != nil {
				return err
			}

			var digest []byte
			cleanData := strings.TrimSpace(dataStr)
			if len(cleanData) == 64 {
				if d, err := hex.DecodeString(cleanData); err == nil {
					digest = d
				}
			}

			if len(digest) == 0 {
				if cleanData == "" {
					cleanData = "nrf52840-hsm-pkcs11-signature-payload"
				}
				h := sha256.Sum256([]byte(cleanData))
				digest = h[:]
			}

			diagSvc := usecases.NewP11DiagService()
			sig, err := diagSvc.SignData(mod, uintptr(slotID), pin, keyID, digest)
			if err != nil {
				return fmt.Errorf("PKCS#11 signing failed: %w", err)
			}

			fmt.Println("==================================================")
			fmt.Println("            PKCS#11 ECDSA Signature              ")
			fmt.Println("==================================================")
			fmt.Printf(" Key ID:    %d (Slot %d)\n", keyID, slotID)
			fmt.Printf(" Digest:    %s\n", hex.EncodeToString(digest))
			fmt.Printf(" Signature: %s\n", hex.EncodeToString(sig))
			fmt.Printf(" Length:    %d bytes (R || S raw)\n", len(sig))
			fmt.Println("==================================================")
			return nil
		},
	}

	cmd.Flags().StringVarP(&modulePath, "module", "m", "", "Path to PKCS#11 provider shared library (.dylib/.so/.dll)")
	cmd.Flags().Uint64VarP(&slotID, "slot", "s", 1, "Slot ID")
	cmd.Flags().StringVar(&pin, "pin", "123456", "User PIN for authentication")
	cmd.Flags().IntVarP(&keyID, "id", "i", 1, "Key slot ID (1..8)")
	cmd.Flags().StringVarP(&dataStr, "data", "d", "", "32-byte hex digest or text to hash and sign")

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
			mod, err := resolveModule(modulePath)
			if err != nil {
				return err
			}

			if pin == "" {
				pin = "123456"
			}

			fmt.Println("==================================================")
			fmt.Println("       Running PKCS#11 Provider Diagnostics       ")
			fmt.Println("==================================================")
			fmt.Printf("Target Module: %s\n\n", mod)

			diagSvc := usecases.NewP11DiagService()
			steps, runErr := diagSvc.RunDiagnostics(mod, pin)

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
				fmt.Printf(" ❌ PKCS#11 diagnostics encountered failures (%s)\n", diagSvc.DiagnosticsSummary(steps))
				fmt.Println("==================================================")
				return fmt.Errorf("PKCS#11 diagnostics failed")
			}

			fmt.Printf(" ✓ All checks passed: %s\n", diagSvc.DiagnosticsSummary(steps))
			fmt.Println("==================================================")
			return nil
		},
	}

	cmd.Flags().StringVarP(&modulePath, "module", "m", "", "Path to PKCS#11 provider shared library (.dylib/.so/.dll)")
	cmd.Flags().StringVar(&pin, "pin", "123456", "Token User PIN for authentication test")

	return cmd
}
