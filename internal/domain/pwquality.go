package domain

import (
	"errors"
	"unicode"
)

var (
	// ErrPINTooShort is returned when PIN length is less than 10 characters.
	ErrPINTooShort = errors.New("PIN must be at least 10 characters long")

	// ErrPINTooLong is returned when PIN length exceeds 64 characters.
	ErrPINTooLong = errors.New("PIN exceeds maximum length of 64 characters")

	// ErrPINNoUpper is returned when PIN lacks an uppercase letter.
	ErrPINNoUpper = errors.New("PIN must contain at least one uppercase letter")

	// ErrPINNoLower is returned when PIN lacks a lowercase letter.
	ErrPINNoLower = errors.New("PIN must contain at least one lowercase letter")

	// ErrPINNoDigit is returned when PIN lacks a numeric digit.
	ErrPINNoDigit = errors.New("PIN must contain at least one digit")

	// ErrPINNoSpecial is returned when PIN lacks a special/symbol character.
	ErrPINNoSpecial = errors.New("PIN must contain at least one special character")

	// ErrPINConsecutive is returned when PIN contains consecutive identical characters (max allowed is 1).
	ErrPINConsecutive = errors.New("PIN contains consecutive identical characters (maximum allowed is 1)")

	// ErrPINSequence is returned when PIN contains a monotonic sequence of 3 or more characters.
	ErrPINSequence = errors.New("PIN contains monotonic character sequence longer than 2 characters")
)

// ValidatePIN validates that a PIN satisfies Yocto pam_pwquality policy constraints:
// - Minimum length 10 characters, maximum 64 characters
// - At least 1 uppercase letter
// - At least 1 lowercase letter
// - At least 1 digit
// - At least 1 special character
// - Maximum 1 consecutive identical character (no repeated adjacent characters)
// - Maximum 2 sequential monotonic characters (no sequences of 3+ like "123" or "cba")
func ValidatePIN(pin string) error {
	runes := []rune(pin)

	if len(runes) < 10 {
		return ErrPINTooShort
	}
	if len(runes) > 64 {
		return ErrPINTooLong
	}

	var (
		hasUpper   bool
		hasLower   bool
		hasDigit   bool
		hasSpecial bool
	)

	for i, r := range runes {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r) || (!unicode.IsLetter(r) && !unicode.IsDigit(r)):
			hasSpecial = true
		}

		// Check consecutive identical characters (maxrepeat = 1)
		if i > 0 && r == runes[i-1] {
			return ErrPINConsecutive
		}

		// Check monotonic sequences of 3+ characters (maxsequence = 2)
		if i >= 2 {
			r0 := runes[i-2]
			r1 := runes[i-1]
			r2 := r

			// Direct integer delta monotonic check
			if (r1 == r0+1 && r2 == r1+1) || (r1 == r0-1 && r2 == r1-1) {
				return ErrPINSequence
			}

			// Case-insensitive letter sequence check
			if unicode.IsLetter(r0) && unicode.IsLetter(r1) && unicode.IsLetter(r2) {
				l0 := unicode.ToLower(r0)
				l1 := unicode.ToLower(r1)
				l2 := unicode.ToLower(r2)
				if (l1 == l0+1 && l2 == l1+1) || (l1 == l0-1 && l2 == l1-1) {
					return ErrPINSequence
				}
			}
		}
	}

	if !hasUpper {
		return ErrPINNoUpper
	}
	if !hasLower {
		return ErrPINNoLower
	}
	if !hasDigit {
		return ErrPINNoDigit
	}
	if !hasSpecial {
		return ErrPINNoSpecial
	}

	return nil
}
