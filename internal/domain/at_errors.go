package domain

import (
	"errors"
	"fmt"
	"strings"
)

// Authentication and configuration errors.
var (
	ErrAlreadyConfigured    = errors.New("device is already configured")
	ErrInvalidAuthMode      = errors.New("invalid authentication mode: must be PIN or SHAMIR")
	ErrMismatchedAuthMethod = errors.New("source and target devices have mismatched authentication methods")
)

// ErrMultipleDevicesDetected is returned when multiple HSM dongles are detected during auto-discovery
// and disambiguation via --serial (-s) or --port (-p) is required.
type ErrMultipleDevicesDetected struct {
	Devices []DiscoveredDongle
}

func (e *ErrMultipleDevicesDetected) Error() string {
	var sb strings.Builder
	sb.WriteString("multiple HSM USB devices detected:\n")
	for i, d := range e.Devices {
		sb.WriteString(fmt.Sprintf("  [%d] Serial/BLE: %-16s Product: %-22s Port: %s\n",
			i+1, d.Serial, d.Product, d.DataPort))
	}
	sb.WriteString("\nPlease specify target device using --port (-p) or --serial (-s), or set HSM_SERIAL / NRF52840_HSM_SERIAL.")
	return sb.String()
}

// CMEError represents a standard +CME ERROR: <code> response from the HSM.
type CMEError struct {
	Code    int
	Message string
}

func (e *CMEError) Error() string {
	return fmt.Sprintf("+CME ERROR: %d (%s)", e.Code, e.Message)
}

func (e *CMEError) Is(target error) bool {
	t, ok := target.(*CMEError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

// Well-known CME error codes defined in the specification.
var (
	ErrDeviceLocked       = &CMEError{Code: 1, Message: "Device locked (authentication required)"}
	ErrInvalidPIN         = &CMEError{Code: 2, Message: "Invalid PIN"}
	ErrPINBlocked         = &CMEError{Code: 3, Message: "PIN blocked (maximum attempts exceeded)"}
	ErrKeyNotFound        = &CMEError{Code: 4, Message: "Key not found in specified slot"}
	ErrInvalidParameter   = &CMEError{Code: 5, Message: "Invalid parameter"}
	ErrCryptoFailure      = &CMEError{Code: 6, Message: "Cryptographic accelerator failure"}
	ErrBufferOverflow     = &CMEError{Code: 7, Message: "Buffer overflow"}
	ErrCommandUnsupported = &CMEError{Code: 8, Message: "Command unsupported"}
)

// ParseCMEError converts an integer CME error code into a structured CMEError.
func ParseCMEError(code int) error {
	switch code {
	case 1:
		return ErrDeviceLocked
	case 2:
		return ErrInvalidPIN
	case 3:
		return ErrPINBlocked
	case 4:
		return ErrKeyNotFound
	case 5:
		return ErrInvalidParameter
	case 6:
		return ErrCryptoFailure
	case 7:
		return ErrBufferOverflow
	case 8:
		return ErrCommandUnsupported
	default:
		return &CMEError{Code: code, Message: "Unknown hardware error"}
	}
}
