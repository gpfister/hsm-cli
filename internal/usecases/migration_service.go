package usecases

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

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

	// Pre-flight: verify homogeneous authentication mode
	sourceStatus, err := sourceClient.Status()
	if err != nil {
		return fmt.Errorf("failed to query source HSM status: %w", err)
	}
	targetStatus, err := targetClient.Status()
	if err != nil {
		return fmt.Errorf("failed to query target HSM status: %w", err)
	}

	if !targetStatus.Configured {
		return fmt.Errorf("target HSM is not configured (method NONE). Initialize target HSM with method %s before migrating keys", sourceStatus.Method)
	}
	if !sourceStatus.Configured {
		return fmt.Errorf("source HSM is not configured (method NONE). Initialize source HSM before migrating keys")
	}
	if sourceStatus.Method != targetStatus.Method {
		return fmt.Errorf("%w: source device is using %s authentication, but target device is using %s authentication (both devices must use the same authentication method)",
			domain.ErrMismatchedAuthMethod, sourceStatus.Method, targetStatus.Method)
	}

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
	sourceStatus, err = sourceClient.Status()
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

// MigrateDevicePorts connects to the source and target serial ports and executes all-keys device migration.
func (s *MigrationService) MigrateDevicePorts(sourcePort, targetPort string, zeroizeSource bool) error {
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

	return s.ExecuteDeviceMigration(sourceClient, targetClient, zeroizeSource)
}

// ExecuteDeviceMigration coordinates full device migration across all occupied key slots.
// On any failure, fail-fast lockdown is triggered immediately (AT+LOCK sent to both devices).
func (s *MigrationService) ExecuteDeviceMigration(sourceClient, targetClient *domain.ATClient, zeroizeSource bool) (err error) {
	if sourceClient == nil || targetClient == nil {
		return errors.New("source and target clients must be non-nil")
	}

	// Fail-fast lockdown on error
	defer func() {
		if err != nil {
			fmt.Fprintln(s.out, "\n[FAIL-FAST LOCKDOWN] Migration error encountered. Transmitting AT+LOCK to source and target devices...")
			_ = sourceClient.Lock()
			_ = targetClient.Lock()
			fmt.Fprintln(s.out, "[FAIL-FAST LOCKDOWN] Both devices locked.")
		}
	}()

	fmt.Fprintln(s.out, "================================================================================")
	fmt.Fprintln(s.out, "                 FULL DEVICE MIGRATION & CLONING CEREMONY                       ")
	fmt.Fprintln(s.out, "================================================================================")

	// Pre-flight: verify homogeneous authentication mode
	sourceStatus, err := sourceClient.Status()
	if err != nil {
		return fmt.Errorf("failed to query source HSM status: %w", err)
	}
	targetStatus, err := targetClient.Status()
	if err != nil {
		return fmt.Errorf("failed to query target HSM status: %w", err)
	}

	if !targetStatus.Configured {
		return fmt.Errorf("target HSM is not configured (method NONE). Initialize target HSM with method %s before migrating keys", sourceStatus.Method)
	}
	if !sourceStatus.Configured {
		return fmt.Errorf("source HSM is not configured (method NONE). Initialize source HSM before migrating keys")
	}
	if sourceStatus.Method != targetStatus.Method {
		return fmt.Errorf("%w: source device is using %s authentication, but target device is using %s authentication (both devices must use the same authentication method)",
			domain.ErrMismatchedAuthMethod, sourceStatus.Method, targetStatus.Method)
	}

	// Step 1: Verify source HSM authentication
	fmt.Fprintln(s.out, "[Step 1/5] Verifying source HSM authentication status...")
	sourceStatus, err = sourceClient.Status()
	if err != nil {
		return fmt.Errorf("failed to query source HSM status: %w", err)
	}
	if !sourceStatus.Authenticated {
		fmt.Fprintln(s.out, "  Source HSM is locked. Initiating turn-by-turn custodian ceremony...")
		if _, err = s.ceremony.RunTurnByTurnAuth(sourceClient); err != nil {
			return fmt.Errorf("source HSM threshold ceremony authentication failed: %w", err)
		}
	} else {
		fmt.Fprintln(s.out, "  ✓ Source HSM is already authenticated and unlocked (solid GREEN).")
	}

	// Step 2: Verify target HSM authentication
	fmt.Fprintln(s.out, "[Step 2/5] Verifying target HSM authentication status...")
	targetStatus, err = targetClient.Status()
	if err != nil {
		return fmt.Errorf("failed to query target HSM status: %w", err)
	}
	if !targetStatus.Authenticated {
		fmt.Fprintln(s.out, "  Target HSM is locked. Initiating turn-by-turn custodian ceremony...")
		if _, err = s.ceremony.RunTurnByTurnAuth(targetClient); err != nil {
			return fmt.Errorf("target HSM threshold ceremony authentication failed: %w", err)
		}
	} else {
		fmt.Fprintln(s.out, "  ✓ Target HSM is already authenticated and unlocked (solid GREEN).")
	}

	// Step 3: Discover all occupied key slots on source HSM
	fmt.Fprintln(s.out, "[Step 3/5] Querying occupied key slots on source HSM...")
	slots, err := sourceClient.Slots()
	if err != nil {
		return fmt.Errorf("failed to query source key slots: %w", err)
	}
	if len(slots) == 0 {
		fmt.Fprintln(s.out, "  Notice: No occupied key slots found on source HSM.")
	} else {
		fmt.Fprintf(s.out, "  ✓ Discovered %d key slot(s) to migrate: ", len(slots))
		slotDescs := make([]string, len(slots))
		for i, sl := range slots {
			slotDescs[i] = fmt.Sprintf("Slot %d (%s)", sl.ID, sl.Algorithm)
		}
		fmt.Fprintln(s.out, strings.Join(slotDescs, ", "))
	}

	// Step 4: Atomic migration loop across all discovered slots
	fmt.Fprintln(s.out, "[Step 4/5] Executing slot-by-slot ephemeral ECDH wrapped migration...")
	for idx, sl := range slots {
		fmt.Fprintf(s.out, "  --> [%d/%d] Migrating Slot %d (%s, %d bits)...\n", idx+1, len(slots), sl.ID, sl.Algorithm, sl.Size)

		targetEphemeralPub, err := targetClient.TargetInit()
		if err != nil {
			return fmt.Errorf("target HSM failed ephemeral ECDH initialization for slot %d: %w", sl.ID, err)
		}

		envelope, err := sourceClient.MigrateWrap(sl.ID, targetEphemeralPub)
		if err != nil {
			return fmt.Errorf("source HSM failed to wrap key slot %d: %w", sl.ID, err)
		}

		if err := targetClient.MigrateUnwrap(envelope); err != nil {
			return fmt.Errorf("target HSM failed to unwrap key slot %d: %w", sl.ID, err)
		}

		// Verify target slot public key matches source
		srcPub, err := sourceClient.PubKey(sl.ID)
		if err != nil {
			return fmt.Errorf("failed to query source public key for slot %d: %w", sl.ID, err)
		}
		tgtPub, err := targetClient.PubKey(sl.ID)
		if err != nil {
			return fmt.Errorf("failed to query target public key for slot %d: %w", sl.ID, err)
		}
		if strings.TrimSpace(srcPub) != strings.TrimSpace(tgtPub) {
			return fmt.Errorf("public key verification mismatch on slot %d (source: %s, target: %s)", sl.ID, srcPub, tgtPub)
		}

		fmt.Fprintf(s.out, "      ✓ Slot %d verified successfully on target device.\n", sl.ID)
	}

	// Step 5: Finalization & mutual lockdown
	fmt.Fprintln(s.out, "[Step 5/5] Finalizing device migration ceremony...")
	if zeroizeSource {
		fmt.Fprintln(s.out, "  --> Zeroizing source HSM (decommissioning)...")
		if err := sourceClient.Reset(); err != nil {
			return fmt.Errorf("failed to zeroize source HSM: %w", err)
		}
		fmt.Fprintln(s.out, "  ✓ Source HSM zeroized successfully (transitioned to pulsating BLUE).")
	}

	fmt.Fprintln(s.out, "  --> Locking target device...")
	_ = targetClient.Lock()
	if !zeroizeSource {
		fmt.Fprintln(s.out, "  --> Locking source device...")
		_ = sourceClient.Lock()
	}

	fmt.Fprintln(s.out, "================================================================================")
	fmt.Fprintln(s.out, "✓ SUCCESS: All keys migrated and verified successfully!")
	fmt.Fprintln(s.out, "================================================================================")
	return nil
}
