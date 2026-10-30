package domain

import (
	"fmt"
)

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
