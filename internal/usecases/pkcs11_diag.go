package usecases

import (
	"encoding/hex"
	"fmt"

	"github.com/gpfister/nrf52840-hsm-cli/internal/infra/p11"
)

// P11DiagService coordinates diagnostic and inspection operations on PKCS#11 providers.
type P11DiagService struct{}

func NewP11DiagService() *P11DiagService {
	return &P11DiagService{}
}

// SlotDetail encapsulates detailed information for a single PKCS#11 slot.
type SlotDetail struct {
	SlotID     uintptr
	SlotInfo   *p11.CK_SLOT_INFO
	TokenInfo  *p11.CK_TOKEN_INFO
	Mechanisms []uint64
}

// ModuleInfo retrieves general Cryptoki module information.
func (s *P11DiagService) ModuleInfo(modulePath string) (*p11.CK_INFO, error) {
	client, err := p11.NewP11Client(modulePath)
	if err != nil {
		return nil, fmt.Errorf("module load failed: %w", err)
	}
	defer client.Close()

	if err := client.Initialize(); err != nil {
		return nil, fmt.Errorf("initialization failed: %w", err)
	}
	defer func() { _ = client.Finalize() }()

	return client.GetInfo()
}

// SlotList queries all available slots and their token descriptors.
func (s *P11DiagService) SlotList(modulePath string) ([]SlotDetail, error) {
	client, err := p11.NewP11Client(modulePath)
	if err != nil {
		return nil, fmt.Errorf("module load failed: %w", err)
	}
	defer client.Close()

	if err := client.Initialize(); err != nil {
		return nil, fmt.Errorf("initialization failed: %w", err)
	}
	defer func() { _ = client.Finalize() }()

	slots, err := client.GetSlotList(false)
	if err != nil {
		return nil, fmt.Errorf("slot list query failed: %w", err)
	}

	var details []SlotDetail
	for _, slotID := range slots {
		slotInfo, _ := client.GetSlotInfo(slotID)
		tokenInfo, _ := client.GetTokenInfo(slotID)
		mechs, _ := client.GetMechanismList(slotID)

		details = append(details, SlotDetail{
			SlotID:     slotID,
			SlotInfo:   slotInfo,
			TokenInfo:  tokenInfo,
			Mechanisms: mechs,
		})
	}

	return details, nil
}

// DiagStep reports the execution state of an individual diagnostic check.
type DiagStep struct {
	Name    string
	Passed  bool
	Details string
	Error   error
}

// RunDiagnostics performs automated PKCS#11 provider sanity checks against the specified module.
func (s *P11DiagService) RunDiagnostics(modulePath string, pin string) ([]DiagStep, error) {
	var steps []DiagStep

	recordStep := func(name string, passed bool, details string, err error) {
		steps = append(steps, DiagStep{
			Name:    name,
			Passed:  passed,
			Details: details,
			Error:   err,
		})
	}

	// Step 1: Module loading and symbol resolution
	client, err := p11.NewP11Client(modulePath)
	if err != nil {
		recordStep("Module Load & Symbol Binding", false, fmt.Sprintf("Failed to load %s", modulePath), err)
		return steps, err
	}
	defer client.Close()
	recordStep("Module Load & Symbol Binding", true, fmt.Sprintf("Loaded %s and verified Cryptoki symbols", modulePath), nil)

	// Step 2: C_Initialize
	if err := client.Initialize(); err != nil {
		recordStep("C_Initialize", false, "Cryptoki initialization returned non-zero code", err)
		return steps, err
	}
	defer func() { _ = client.Finalize() }()
	recordStep("C_Initialize", true, "Cryptoki framework initialized cleanly", nil)

	// Step 3: C_GetInfo
	info, err := client.GetInfo()
	if err != nil {
		recordStep("C_GetInfo", false, "Failed to query library metadata", err)
		return steps, err
	}
	recordStep("C_GetInfo", true, fmt.Sprintf("Cryptoki v%s, Manufacturer: %q, Desc: %q",
		info.CryptokiVersion, info.CleanManufacturer(), info.CleanLibraryDesc()), nil)

	// Step 4: C_GetSlotList
	slots, err := client.GetSlotList(true)
	if err != nil {
		recordStep("C_GetSlotList", false, "Failed to query slots with token present", err)
		return steps, err
	}
	if len(slots) == 0 {
		err := fmt.Errorf("no slots with token present detected")
		recordStep("C_GetSlotList", false, "Device token not detected in any slot", err)
		return steps, err
	}
	recordStep("C_GetSlotList", true, fmt.Sprintf("Detected %d active slot(s) with token present", len(slots)), nil)

	targetSlot := slots[0]

	// Step 5: C_GetTokenInfo
	tokenInfo, err := client.GetTokenInfo(targetSlot)
	if err != nil {
		recordStep("C_GetTokenInfo", false, fmt.Sprintf("Failed to query token info on slot %d", targetSlot), err)
		return steps, err
	}
	recordStep("C_GetTokenInfo", true, fmt.Sprintf("Label: %q, Model: %q, Serial: %q",
		tokenInfo.CleanLabel(), tokenInfo.CleanModel(), tokenInfo.CleanSerial()), nil)

	// Step 6: C_OpenSession
	hSession, err := client.OpenSession(targetSlot, p11.CKF_SERIAL_SESSION|p11.CKF_RW_SESSION)
	if err != nil {
		recordStep("C_OpenSession", false, "Failed to open read-write serial session", err)
		return steps, err
	}
	defer func() { _ = client.CloseSession(hSession) }()
	recordStep("C_OpenSession", true, fmt.Sprintf("Opened session handle: 0x%08X", hSession), nil)

	// Step 7: C_Login
	if pin == "" {
		pin = "123456"
	}
	if err := client.Login(hSession, p11.CKU_USER, pin); err != nil {
		recordStep("C_Login", false, fmt.Sprintf("Failed to authenticate with user PIN (length %d)", len(pin)), err)
		return steps, err
	}
	defer func() { _ = client.Logout(hSession) }()
	recordStep("C_Login", true, "User PIN authentication successful (session state unlocked)", nil)

	// Step 8: C_GenerateRandom
	randBytes, err := client.GenerateRandom(hSession, 32)
	if err != nil {
		recordStep("C_GenerateRandom", false, "Hardware TRNG byte generation failed", err)
		return steps, err
	}
	recordStep("C_GenerateRandom", true, fmt.Sprintf("Generated 32 random bytes: %s...", hex.EncodeToString(randBytes[:8])), nil)

	// Step 9: C_GetMechanismList
	mechs, err := client.GetMechanismList(targetSlot)
	if err != nil {
		recordStep("C_GetMechanismList", false, "Failed to query token mechanism list", err)
		return steps, err
	}
	var mechNames []string
	for _, m := range mechs {
		mechNames = append(mechNames, p11.MechanismName(m))
	}
	recordStep("C_GetMechanismList", true, fmt.Sprintf("Token supports %d mechanisms: %v", len(mechs), mechNames), nil)

	// Step 10: C_Logout
	if err := client.Logout(hSession); err != nil {
		recordStep("C_Logout", false, "Failed to logout", err)
		return steps, err
	}
	recordStep("C_Logout", true, "User session logged out successfully", nil)

	// Step 11: C_CloseSession
	if err := client.CloseSession(hSession); err != nil {
		recordStep("C_CloseSession", false, "Failed to close session", err)
		return steps, err
	}
	recordStep("C_CloseSession", true, "Session closed cleanly", nil)

	// Step 12: C_Finalize
	if err := client.Finalize(); err != nil {
		recordStep("C_Finalize", false, "Cryptoki finalization returned error", err)
		return steps, err
	}
	recordStep("C_Finalize", true, "Cryptoki finalized cleanly", nil)

	return steps, nil
}
