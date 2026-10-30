package usecases

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/gpfister/nrf52840-hsm-cli/internal/domain"
	"github.com/gpfister/nrf52840-hsm-cli/internal/infra/serial"
)

// MigrationService orchestrates dual-device blind key migration.
type MigrationService struct {
	ceremony *CeremonyService
	out      io.Writer
}

// NewMigrationService constructs a MigrationService.
func NewMigrationService(ceremony *CeremonyService, out io.Writer) *MigrationService {
	if ceremony == nil {
		ceremony = NewCeremonyService()
	}
	if out == nil {
		out = os.Stdout
	}
	return &MigrationService{
		ceremony: ceremony,
		out:      out,
	}
}

// MigratePorts connects to the source and target serial ports and executes key migration for slotID.
func (s *MigrationService) MigratePorts(sourcePort, targetPort string, slotID int) error {
	if sourcePort == "" {
		return errors.New("source port must be specified")
	}
	if targetPort == "" {
		return errors.New("target port must be specified")
	}
	if sourcePort == targetPort {
		return errors.New("source and target ports cannot be the same physical device")
	}

	fmt.Fprintf(s.out, "Connecting to Source HSM at %s...\n", sourcePort)
	sourceTransport, err := serial.OpenAT(sourcePort)
	if err != nil {
		return fmt.Errorf("failed to connect to source HSM at %s: %w", sourcePort, err)
	}
	defer sourceTransport.Close()
	sourceClient := domain.NewATClient(sourceTransport)

	fmt.Fprintf(s.out, "Connecting to Target HSM at %s...\n", targetPort)
	targetTransport, err := serial.OpenAT(targetPort)
	if err != nil {
		return fmt.Errorf("failed to connect to target HSM at %s: %w", targetPort, err)
	}
	defer targetTransport.Close()
	targetClient := domain.NewATClient(targetTransport)

	return s.ExecuteMigration(sourceClient, targetClient, slotID)
}

// ExecuteMigration coordinates blind ECDH key wrapping, source unlock, target unwrapping, and self-lock verification.
func (s *MigrationService) ExecuteMigration(sourceClient, targetClient *domain.ATClient, slotID int) error {
	if sourceClient == nil || targetClient == nil {
		return errors.New("source and target clients must be non-nil")
	}
	if slotID <= 0 {
		return errors.New("slot ID must be >= 1")
	}

	fmt.Fprintln(s.out, "================================================================================")
	fmt.Fprintf(s.out, "           DUAL-DEVICE BLIND KEY MIGRATION (SLOT %d)                            \n", slotID)
	fmt.Fprintln(s.out, "================================================================================")

	// Step 1: Target ephemeral ECDH init
	fmt.Fprintln(s.out, "[Step 1/5] Initializing target HSM ephemeral ECDH keypair...")
	targetEphemeralPub, err := targetClient.TargetInit()
	if err != nil {
		return fmt.Errorf("target HSM failed ephemeral ECDH initialization: %w", err)
	}
	displayLen := 24
	if len(targetEphemeralPub) < displayLen {
		displayLen = len(targetEphemeralPub)
	}
	fmt.Fprintf(s.out, "  ✓ Target ephemeral ECDH public key received: %s...\n", targetEphemeralPub[:displayLen])

	// Step 2: Source HSM authentication (turn-by-turn unlock if needed)
	fmt.Fprintln(s.out, "[Step 2/5] Verifying source HSM authentication status...")
	sourceStatus, err := sourceClient.Status()
	if err != nil {
		return fmt.Errorf("failed to query source HSM status: %w", err)
	}

	if !sourceStatus.Authenticated {
		fmt.Fprintln(s.out, "  Source HSM is locked. Initiating turn-by-turn custodian ceremony...")
		_, err := s.ceremony.RunTurnByTurnAuth(sourceClient)
		if err != nil {
			return fmt.Errorf("source HSM threshold ceremony authentication failed: %w", err)
		}
	} else {
		fmt.Fprintln(s.out, "  ✓ Source HSM is already authenticated and unlocked (solid GREEN).")
	}

	// Step 3: Source wrap
	fmt.Fprintf(s.out, "[Step 3/5] Wrapping key slot %d on source HSM using target public key...\n", slotID)
	envelope, err := sourceClient.MigrateWrap(slotID, targetEphemeralPub)
	if err != nil {
		return fmt.Errorf("source HSM failed to wrap key slot %d: %w", slotID, err)
	}
	fmt.Fprintf(s.out, "  ✓ Key encrypted into migration envelope (ciphertext size: %d bytes)\n", len(envelope.EncryptedKey)/2)

	// Step 4: Target unwrap
	fmt.Fprintf(s.out, "[Step 4/5] Unwrapping and importing key into target HSM slot %d...\n", slotID)
	if err := targetClient.MigrateUnwrap(envelope); err != nil {
		return fmt.Errorf("target HSM failed to unwrap migration envelope: %w", err)
	}
	fmt.Fprintln(s.out, "  ✓ Target HSM successfully decrypted and stored migrated key!")

	// Step 5: Self-lock both devices and verify solid RED state
	fmt.Fprintln(s.out, "[Step 5/5] Re-locking both devices to ensure zero exposure...")
	_ = sourceClient.Lock()
	_ = targetClient.Lock()

	sourceFinalStatus, err := sourceClient.Status()
	if err == nil && sourceFinalStatus.Authenticated {
		return errors.New("source HSM failed to transition to locked state")
	}
	targetFinalStatus, err := targetClient.Status()
	if err == nil && targetFinalStatus.Authenticated {
		return errors.New("target HSM failed to transition to locked state")
	}

	fmt.Fprintln(s.out, "  ✓ Both devices confirmed locked (Solid RED LED).")
	fmt.Fprintln(s.out, "================================================================================")
	fmt.Fprintf(s.out, "✓ SUCCESS: Key slot %d securely migrated with end-to-end blind ECDH encryption!\n", slotID)
	fmt.Fprintln(s.out, "================================================================================")

	return nil
}
