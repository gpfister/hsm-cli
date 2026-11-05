package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/gpfister/nrf52840-hsm-cli/internal/domain"
)

func TestValidatePIN_ValidPINs(t *testing.T) {
	validPINs := []string{
		"Tr0ub4dor&3",
		"Secr3t!P@s1",
		"C0mplex#Key9",
		"Vault_2026!k",
		"GpHsm-2026*ok",
		"P@sw0rd!Secure",
		"Nrf52-Key*2026",
	}

	for _, pin := range validPINs {
		if err := domain.ValidatePIN(pin); err != nil {
			t.Errorf("expected valid PIN %q to pass validation, got: %v", pin, err)
		}
	}
}

func TestValidatePIN_TooShort(t *testing.T) {
	shortPINs := []string{
		"",
		"123",
		"P@ss1",
		"Ab1!Cd2",
		"Ab1!Cd2#",  // 8 chars
		"Ab1!Cd2#E", // 9 chars
	}

	for _, pin := range shortPINs {
		err := domain.ValidatePIN(pin)
		if !errors.Is(err, domain.ErrPINTooShort) {
			t.Errorf("expected ErrPINTooShort for %q, got: %v", pin, err)
		}
	}
}

func TestValidatePIN_TooLong(t *testing.T) {
	// 65 chars
	longPIN := strings.Repeat("A1!b", 16) + "X"
	if len(longPIN) != 65 {
		t.Fatalf("expected length 65, got %d", len(longPIN))
	}

	err := domain.ValidatePIN(longPIN)
	if !errors.Is(err, domain.ErrPINTooLong) {
		t.Errorf("expected ErrPINTooLong, got: %v", err)
	}
}

func TestValidatePIN_MissingCharacterClasses(t *testing.T) {
	tests := []struct {
		name        string
		pin         string
		expectedErr error
	}{
		{
			name:        "missing uppercase",
			pin:         "tr0ub4dor&3long",
			expectedErr: domain.ErrPINNoUpper,
		},
		{
			name:        "missing lowercase",
			pin:         "TR0UB4DOR&3LONG",
			expectedErr: domain.ErrPINNoLower,
		},
		{
			name:        "missing digit",
			pin:         "Troubador&Xlong",
			expectedErr: domain.ErrPINNoDigit,
		},
		{
			name:        "missing special char",
			pin:         "Tr0ub4dor3LongX",
			expectedErr: domain.ErrPINNoSpecial,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := domain.ValidatePIN(tc.pin)
			if !errors.Is(err, tc.expectedErr) {
				t.Errorf("expected %v for %q, got: %v", tc.expectedErr, tc.pin, err)
			}
		})
	}
}

func TestValidatePIN_ConsecutiveIdenticalCharacters(t *testing.T) {
	repeats := []string{
		"Tr0ub4ddor&3",  // dd
		"Tr00ub4dor&3",  // 00
		"Tr0ub4dor&&3",  // &&
		"TTr0ub4dor&3",  // TT
		"Tr0ub4dor&33",  // 33
	}

	for _, pin := range repeats {
		err := domain.ValidatePIN(pin)
		if !errors.Is(err, domain.ErrPINConsecutive) {
			t.Errorf("expected ErrPINConsecutive for %q, got: %v", pin, err)
		}
	}
}

func TestValidatePIN_MonotonicSequences(t *testing.T) {
	sequences := []struct {
		name string
		pin  string
	}{
		{"ascending digits 123", "Tr0ub4123&Xk"},
		{"descending digits 321", "Tr0ub4321&Xk"},
		{"ascending letters abc", "Tr0ub4abc&Xk"},
		{"descending letters cba", "Tr0ub4cba&Xk"},
		{"ascending mixed-case AbC", "Tr0ub4AbC&Xk"},
		{"descending mixed-case CbA", "Tr0ub4CbA&Xk"},
		{"ascending xyz", "Tr0ub4xyz&1!"},
		{"descending zyx", "Tr0ub4zyx&1!"},
	}

	for _, tc := range sequences {
		t.Run(tc.name, func(t *testing.T) {
			err := domain.ValidatePIN(tc.pin)
			if !errors.Is(err, domain.ErrPINSequence) {
				t.Errorf("expected ErrPINSequence for %q, got: %v", tc.pin, err)
			}
		})
	}
}

func TestValidatePIN_ShortSequenceAllowed(t *testing.T) {
	// Sequences of 2 are allowed (maxsequence = 2)
	allowed := []string{
		"Tr0ub12&Xk9P", // "12" is sequence of 2
		"Tr0ub21&Xk9P", // "21" is sequence of 2
		"Tr0ubab&Xk9P", // "ab" is sequence of 2
		"Tr0ubcb&Xk9P", // "cb" is sequence of 2
	}

	for _, pin := range allowed {
		if err := domain.ValidatePIN(pin); err != nil {
			t.Errorf("expected sequence of 2 to be allowed in %q, got: %v", pin, err)
		}
	}
}
