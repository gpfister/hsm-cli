package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func newKeyCmd() *cobra.Command {
	keyCmd := &cobra.Command{
		Use:   "key",
		Short: "Manage hardware cryptographic keys and execute signing operations",
	}

	keyCmd.AddCommand(newKeyListCmd())
	keyCmd.AddCommand(newKeyGenerateCmd())
	keyCmd.AddCommand(newKeyPubkeyCmd())
	keyCmd.AddCommand(newKeySignCmd())
	keyCmd.AddCommand(newKeyMigrateCmd())

	return keyCmd
}

func newKeyListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List stored private/public key slots",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, cleanup, err := getATClient()
			if err != nil {
				return err
			}
			defer cleanup()

			slots, err := client.Slots()
			if err != nil {
				return fmt.Errorf("failed to query key slots: %w", err)
			}

			if len(slots) == 0 {
				fmt.Println("No stored keys found in HSM token.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "SLOT\tALGORITHM\tBITS\tLABEL")
			fmt.Fprintln(w, "----\t---------\t----\t-----")
			for _, s := range slots {
				label := s.Label
				if label == "" {
					label = "(unlabeled)"
				}
				fmt.Fprintf(w, "%d\t%s\t%d\t%s\n", s.ID, s.Algorithm, s.Size, label)
			}
			return w.Flush()
		},
	}
}

func newKeyGenerateCmd() *cobra.Command {
	var (
		slot int
		algo string
	)

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate secp256r1 keypair using hardware TRNG",
		RunE: func(cmd *cobra.Command, args []string) error {
			if slot < 1 || slot > 16 {
				return fmt.Errorf("--slot must be between 1 and 16 (got %d)", slot)
			}
			if algo == "" {
				algo = "EC_P256"
			}

			client, cleanup, err := getATClient()
			if err != nil {
				return err
			}
			defer cleanup()

			res, err := client.KeyGen(slot, algo)
			if err != nil {
				return fmt.Errorf("key generation failed: %w", err)
			}

			fmt.Println("✓ [OK] Keypair generated successfully in hardware!")
			fmt.Printf("  Slot:       %d\n", res.ID)
			fmt.Printf("  Algorithm:  %s\n", algo)
			fmt.Printf("  Public Key: %s\n", res.PublicKeyHex)
			return nil
		},
	}

	cmd.Flags().IntVar(&slot, "slot", 0, "Target key slot identifier (1..16)")
	cmd.Flags().StringVar(&algo, "algo", "EC_P256", "Key algorithm (default: EC_P256)")
	_ = cmd.MarkFlagRequired("slot")

	return cmd
}

func newKeyPubkeyCmd() *cobra.Command {
	var slot int

	cmd := &cobra.Command{
		Use:   "pubkey",
		Short: "Display uncompressed public key point from specified slot",
		RunE: func(cmd *cobra.Command, args []string) error {
			if slot < 1 || slot > 16 {
				return fmt.Errorf("--slot must be between 1 and 16 (got %d)", slot)
			}

			client, cleanup, err := getATClient()
			if err != nil {
				return err
			}
			defer cleanup()

			pubKey, err := client.PubKey(slot)
			if err != nil {
				return fmt.Errorf("failed to retrieve public key: %w", err)
			}

			fmt.Printf("Slot %d Public Key (65B uncompressed):\n", slot)
			fmt.Printf("%s\n", pubKey)
			return nil
		},
	}

	cmd.Flags().IntVar(&slot, "slot", 0, "Source key slot identifier (1..16)")
	_ = cmd.MarkFlagRequired("slot")

	return cmd
}

func newKeySignCmd() *cobra.Command {
	var (
		slot int
		hash string
	)

	cmd := &cobra.Command{
		Use:   "sign",
		Short: "Sign a 32-byte SHA-256 digest with specified key using hardware accelerator",
		RunE: func(cmd *cobra.Command, args []string) error {
			if slot < 1 || slot > 16 {
				return fmt.Errorf("--slot must be between 1 and 16 (got %d)", slot)
			}
			if hash == "" {
				return fmt.Errorf("--hash must be provided (64 hex characters / 32 bytes)")
			}

			client, cleanup, err := getATClient()
			if err != nil {
				return err
			}
			defer cleanup()

			sig, err := client.Sign(slot, hash)
			if err != nil {
				return fmt.Errorf("signing operation failed: %w", err)
			}

			fmt.Println("✓ [OK] ECDSA signature generated successfully!")
			fmt.Printf("  Slot:      %d\n", slot)
			fmt.Printf("  Signature: %s\n", sig)
			return nil
		},
	}

	cmd.Flags().IntVar(&slot, "slot", 0, "Key slot identifier for signing (1..16)")
	cmd.Flags().StringVar(&hash, "hash", "", "SHA-256 digest in hex (64 hex characters)")
	_ = cmd.MarkFlagRequired("slot")
	_ = cmd.MarkFlagRequired("hash")

	return cmd
}
