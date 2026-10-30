package p11

import (
	"fmt"
	"strings"
)

// Standard Cryptoki return codes
const (
	CKR_OK                           uintptr = 0x00000000
	CKR_CANCEL                       uintptr = 0x00000001
	CKR_HOST_MEMORY                  uintptr = 0x00000002
	CKR_SLOT_ID_INVALID              uintptr = 0x00000003
	CKR_GENERAL_ERROR                uintptr = 0x00000005
	CKR_FUNCTION_FAILED              uintptr = 0x00000006
	CKR_ARGUMENTS_BAD                uintptr = 0x00000007
	CKR_NO_EVENT                     uintptr = 0x00000008
	CKR_NEED_TO_CREATE_THREADS       uintptr = 0x00000009
	CKR_CANT_LOCK                    uintptr = 0x0000000A
	CKR_DEVICE_ERROR                 uintptr = 0x00000030
	CKR_DEVICE_MEMORY                uintptr = 0x00000031
	CKR_DEVICE_REMOVED               uintptr = 0x00000032
	CKR_PIN_INCORRECT                uintptr = 0x000000A0
	CKR_PIN_INVALID                  uintptr = 0x000000A1
	CKR_PIN_LEN_RANGE                uintptr = 0x000000A2
	CKR_PIN_EXPIRED                  uintptr = 0x000000A3
	CKR_PIN_LOCKED                   uintptr = 0x000000A4
	CKR_SESSION_CLOSED               uintptr = 0x000000B0
	CKR_SESSION_COUNT                uintptr = 0x000000B1
	CKR_SESSION_HANDLE_INVALID       uintptr = 0x000000B3
	CKR_USER_NOT_LOGGED_IN           uintptr = 0x00000101
	CKR_USER_ALREADY_LOGGED_IN       uintptr = 0x00000100
	CKR_BUFFER_TOO_SMALL             uintptr = 0x00000150
	CKR_CRYPTOKI_ALREADY_INITIALIZED uintptr = 0x00000191
	CKR_CRYPTOKI_NOT_INITIALIZED     uintptr = 0x00000190
)

// Cryptoki Flags & Enums
const (
	CKF_TOKEN_PRESENT  uint64 = 0x00000001
	CKF_REMOVABLE_DEVICE uint64 = 0x00000002
	CKF_HW_SLOT        uint64 = 0x00000004

	CKF_RW_SESSION     uint64 = 0x00000002
	CKF_SERIAL_SESSION uint64 = 0x00000004

	CKU_SO   uintptr = 0
	CKU_USER uintptr = 1

	CKM_RSA_PKCS_KEY_PAIR_GEN uint64 = 0x00000000
	CKM_RSA_PKCS             uint64 = 0x00000001
	CKM_EC_KEY_PAIR_GEN      uint64 = 0x00001040
	CKM_ECDSA                uint64 = 0x00001041
	CKM_ECDSA_SHA256         uint64 = 0x00001043
	CKM_SHA256               uint64 = 0x00000250
)

// CK_VERSION represents Cryptoki major/minor versions.
type CK_VERSION struct {
	Major byte
	Minor byte
}

func (v CK_VERSION) String() string {
	return fmt.Sprintf("%d.%d", v.Major, v.Minor)
}

// CK_INFO holds general information about Cryptoki.
type CK_INFO struct {
	CryptokiVersion    CK_VERSION
	ManufacturerID     [32]byte
	_pad               [6]byte // 64-bit alignment padding
	Flags              uint64
	LibraryDescription [32]byte
	LibraryVersion     CK_VERSION
	_pad2              [6]byte
}

func (i CK_INFO) CleanManufacturer() string {
	return strings.TrimSpace(string(i.ManufacturerID[:]))
}

func (i CK_INFO) CleanLibraryDesc() string {
	return strings.TrimSpace(string(i.LibraryDescription[:]))
}

// CK_SLOT_INFO provides information about a slot.
type CK_SLOT_INFO struct {
	SlotDescription [64]byte
	ManufacturerID  [32]byte
	Flags           uint64
	HardwareVersion CK_VERSION
	FirmwareVersion CK_VERSION
	_pad            [4]byte
}

func (s CK_SLOT_INFO) CleanDescription() string {
	return strings.TrimSpace(string(s.SlotDescription[:]))
}

func (s CK_SLOT_INFO) CleanManufacturer() string {
	return strings.TrimSpace(string(s.ManufacturerID[:]))
}

// CK_TOKEN_INFO provides information about a token.
type CK_TOKEN_INFO struct {
	Label              [32]byte
	ManufacturerID     [32]byte
	Model              [16]byte
	SerialNumber       [16]byte
	Flags              uint64
	MaxSessionCount    uint64
	SessionCount       uint64
	MaxRwSessionCount  uint64
	RwSessionCount     uint64
	MaxPinLen          uint64
	MinPinLen          uint64
	TotalPublicMemory  uint64
	FreePublicMemory   uint64
	TotalPrivateMemory uint64
	FreePrivateMemory  uint64
	HardwareVersion    CK_VERSION
	FirmwareVersion    CK_VERSION
	UtcTime            [16]byte
	_pad               [4]byte
}

func (t CK_TOKEN_INFO) CleanLabel() string {
	return strings.TrimSpace(string(t.Label[:]))
}

func (t CK_TOKEN_INFO) CleanManufacturer() string {
	return strings.TrimSpace(string(t.ManufacturerID[:]))
}

func (t CK_TOKEN_INFO) CleanModel() string {
	return strings.TrimSpace(string(t.Model[:]))
}

func (t CK_TOKEN_INFO) CleanSerial() string {
	return strings.TrimSpace(string(t.SerialNumber[:]))
}

// MechanismName maps a mechanism code to its symbolic name.
func MechanismName(m uint64) string {
	switch m {
	case CKM_EC_KEY_PAIR_GEN:
		return "CKM_EC_KEY_PAIR_GEN"
	case CKM_ECDSA:
		return "CKM_ECDSA"
	case CKM_ECDSA_SHA256:
		return "CKM_ECDSA_SHA256"
	case CKM_SHA256:
		return "CKM_SHA256"
	case CKM_RSA_PKCS:
		return "CKM_RSA_PKCS"
	case CKM_RSA_PKCS_KEY_PAIR_GEN:
		return "CKM_RSA_PKCS_KEY_PAIR_GEN"
	default:
		return fmt.Sprintf("0x%08X", m)
	}
}

// RVError converts a non-zero CK_RV into an informative Go error.
func RVError(funcName string, rv uintptr) error {
	if rv == CKR_OK {
		return nil
	}
	switch rv {
	case CKR_PIN_INCORRECT:
		return fmt.Errorf("%s failed: CKR_PIN_INCORRECT (0x%08X) - Incorrect PIN", funcName, rv)
	case CKR_PIN_LOCKED:
		return fmt.Errorf("%s failed: CKR_PIN_LOCKED (0x%08X) - Token PIN is locked", funcName, rv)
	case CKR_USER_NOT_LOGGED_IN:
		return fmt.Errorf("%s failed: CKR_USER_NOT_LOGGED_IN (0x%08X) - Authentication required", funcName, rv)
	case CKR_DEVICE_ERROR:
		return fmt.Errorf("%s failed: CKR_DEVICE_ERROR (0x%08X) - Hardware communication or execution error", funcName, rv)
	case CKR_SLOT_ID_INVALID:
		return fmt.Errorf("%s failed: CKR_SLOT_ID_INVALID (0x%08X) - Specified slot is invalid", funcName, rv)
	case CKR_GENERAL_ERROR:
		return fmt.Errorf("%s failed: CKR_GENERAL_ERROR (0x%08X)", funcName, rv)
	case CKR_FUNCTION_FAILED:
		return fmt.Errorf("%s failed: CKR_FUNCTION_FAILED (0x%08X)", funcName, rv)
	case CKR_ARGUMENTS_BAD:
		return fmt.Errorf("%s failed: CKR_ARGUMENTS_BAD (0x%08X)", funcName, rv)
	default:
		return fmt.Errorf("%s failed with CK_RV 0x%08X", funcName, rv)
	}
}
