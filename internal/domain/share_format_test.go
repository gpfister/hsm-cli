package domain_test

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/gpfister/nrf52840-hsm-cli/internal/domain"
)

func TestCanonicalWorkedVector(t *testing.T) {
	idx := 1
	rawHex := "000102030405060708090A0B0C0D0E0F101112131415161718191A1B1C1D1E1F"
	expectedCRC := uint32(0xC4A28159)
	expectedFormatted := "1-000102030405060708090A0B0C0D0E0F101112131415161718191A1B1C1D1E1F-C4A28159"

	shareBytes, err := hex.DecodeString(rawHex)
	if err != nil {
		t.Fatalf("failed decoding test share hex: %v", err)
	}

	crc := domain.CalculateShareCRC32(idx, shareBytes)
	if crc != expectedCRC {
		t.Fatalf("CRC32 mismatch: expected 0x%08X, got 0x%08X", expectedCRC, crc)
	}

	formatted, err := domain.FormatCustodianShare(idx, rawHex)
	if err != nil {
		t.Fatalf("FormatCustodianShare failed: %v", err)
	}
	if formatted != expectedFormatted {
		t.Fatalf("Format mismatch: expected %q, got %q", expectedFormatted, formatted)
	}

	parsedIdx, parsedHex, err := domain.ParseCustodianShare(formatted)
	if err != nil {
		t.Fatalf("ParseCustodianShare failed: %v", err)
	}
	if parsedIdx != idx {
		t.Errorf("Parsed index mismatch: expected %d, got %d", idx, parsedIdx)
	}
	if parsedHex != rawHex {
		t.Errorf("Parsed hex mismatch: expected %s, got %s", rawHex, parsedHex)
	}
}

func TestRoundTripRandomShares(t *testing.T) {
	for i := 0; i < 100; i++ {
		idx := (i % 5) + 1
		raw := make([]byte, 32)
		if _, err := rand.Read(raw); err != nil {
			t.Fatalf("rand.Read failed: %v", err)
		}
		rawHex := hex.EncodeToString(raw)

		formatted, err := domain.FormatCustodianShare(idx, rawHex)
		if err != nil {
			t.Fatalf("turn %d: FormatCustodianShare failed: %v", i, err)
		}

		if len(formatted) != 75 {
			t.Fatalf("turn %d: expected formatted length 75, got %d (%s)", i, len(formatted), formatted)
		}

		parsedIdx, parsedHex, err := domain.ParseCustodianShare(formatted)
		if err != nil {
			t.Fatalf("turn %d: ParseCustodianShare failed: %v", i, err)
		}
		if parsedIdx != idx {
			t.Errorf("turn %d: index mismatch: expected %d, got %d", i, idx, parsedIdx)
		}
		if parsedHex != strings.ToUpper(rawHex) {
			t.Errorf("turn %d: hex mismatch: expected %s, got %s", i, strings.ToUpper(rawHex), parsedHex)
		}
	}
}

func TestTypoAndMutationDetection(t *testing.T) {
	idx := 2
	rawHex := "A0A1A2A3A4A5A6A7A8A9AAABACADAEAFB0B1B2B3B4B5B6B7B8B9BABBBCBDBEBF"
	formatted, err := domain.FormatCustodianShare(idx, rawHex)
	if err != nil {
		t.Fatalf("FormatCustodianShare failed: %v", err)
	}

	// 1. Mutate index character
	mutatedIdx := "3" + formatted[1:]
	if _, _, err := domain.ParseCustodianShare(mutatedIdx); !errors.Is(err, domain.ErrInvalidShareChecksum) {
		t.Errorf("expected ErrInvalidShareChecksum when mutating index, got: %v", err)
	}

	// 2. Mutate one hex character in the share
	shareStart := 2
	shareEnd := 2 + 64
	for pos := shareStart; pos < shareEnd; pos += 7 {
		origChar := formatted[pos]
		replacement := '0'
		if origChar == '0' {
			replacement = '1'
		}
		mutatedShare := formatted[:pos] + string(replacement) + formatted[pos+1:]
		if _, _, err := domain.ParseCustodianShare(mutatedShare); !errors.Is(err, domain.ErrInvalidShareChecksum) {
			t.Errorf("pos %d: expected ErrInvalidShareChecksum when mutating share byte, got: %v", pos, err)
		}
	}

	// 3. Mutate one character in CRC suffix
	crcPos := len(formatted) - 4
	origCRCChar := formatted[crcPos]
	crcReplacement := 'F'
	if origCRCChar == 'F' {
		crcReplacement = 'E'
	}
	mutatedCRC := formatted[:crcPos] + string(crcReplacement) + formatted[crcPos+1:]
	if _, _, err := domain.ParseCustodianShare(mutatedCRC); !errors.Is(err, domain.ErrInvalidShareChecksum) {
		t.Errorf("expected ErrInvalidShareChecksum when mutating CRC, got: %v", err)
	}
}

func TestFormatCustodianShare_InvalidInputs(t *testing.T) {
	validHex := strings.Repeat("A", 64)

	// Invalid indices
	for _, invalidIdx := range []int{0, 6, -1, 99} {
		if _, err := domain.FormatCustodianShare(invalidIdx, validHex); !errors.Is(err, domain.ErrInvalidShareIndex) {
			t.Errorf("expected ErrInvalidShareIndex for idx %d, got: %v", invalidIdx, err)
		}
	}

	// Invalid hex length
	for _, invalidHex := range []string{
		strings.Repeat("A", 62),
		strings.Repeat("A", 66),
		"",
	} {
		if _, err := domain.FormatCustodianShare(1, invalidHex); !errors.Is(err, domain.ErrInvalidShareHex) {
			t.Errorf("expected ErrInvalidShareHex for len %d, got: %v", len(invalidHex), err)
		}
	}

	// Non-hex chars
	badHex := strings.Repeat("A", 63) + "Z"
	if _, err := domain.FormatCustodianShare(1, badHex); !errors.Is(err, domain.ErrInvalidShareHex) {
		t.Errorf("expected ErrInvalidShareHex for non-hex, got: %v", err)
	}

	// 0x prefix is accepted in FormatCustodianShare
	with0x := "0x" + validHex
	formatted, err := domain.FormatCustodianShare(1, with0x)
	if err != nil {
		t.Fatalf("expected success with 0x prefix, got: %v", err)
	}
	if !strings.HasPrefix(formatted, "1-AAAA") {
		t.Errorf("unexpected formatted string: %s", formatted)
	}
}

func TestParseCustodianShare_InvalidInputs(t *testing.T) {
	validRawHex := strings.Repeat("B", 64)
	formatted, _ := domain.FormatCustodianShare(3, validRawHex)

	tests := []struct {
		name        string
		input       string
		expectedErr error
	}{
		{
			name:        "Raw 64-hex share without dashes",
			input:       validRawHex,
			expectedErr: domain.ErrRawShareDetected,
		},
		{
			name:        "Raw 64-hex share with 0x prefix",
			input:       "0x" + validRawHex,
			expectedErr: domain.ErrRawShareDetected,
		},
		{
			name:        "Empty string",
			input:       "",
			expectedErr: domain.ErrInvalidShareFormat,
		},
		{
			name:        "Missing hyphens",
			input:       "1" + validRawHex + "12345678",
			expectedErr: domain.ErrInvalidShareFormat,
		},
		{
			name:        "Extra hyphens",
			input:       "1-" + validRawHex + "-12345678-extra",
			expectedErr: domain.ErrInvalidShareFormat,
		},
		{
			name:        "Double hyphen",
			input:       "1--" + validRawHex + "-12345678",
			expectedErr: domain.ErrInvalidShareFormat,
		},
		{
			name:        "Index 0",
			input:       "0-" + validRawHex + "-12345678",
			expectedErr: domain.ErrInvalidShareIndex,
		},
		{
			name:        "Index 6",
			input:       "6-" + validRawHex + "-12345678",
			expectedErr: domain.ErrInvalidShareIndex,
		},
		{
			name:        "Non-numeric index",
			input:       "X-" + validRawHex + "-12345678",
			expectedErr: domain.ErrInvalidShareIndex,
		},
		{
			name:        "Two-digit index 01",
			input:       "01-" + validRawHex + "-12345678",
			expectedErr: domain.ErrInvalidShareIndex,
		},
		{
			name:        "Short share payload",
			input:       "1-" + strings.Repeat("A", 62) + "-12345678",
			expectedErr: domain.ErrInvalidShareHex,
		},
		{
			name:        "Long share payload",
			input:       "1-" + strings.Repeat("A", 66) + "-12345678",
			expectedErr: domain.ErrInvalidShareHex,
		},
		{
			name:        "Non-hex share payload",
			input:       "1-" + strings.Repeat("A", 63) + "Z-12345678",
			expectedErr: domain.ErrInvalidShareHex,
		},
		{
			name:        "Short CRC suffix",
			input:       "1-" + validRawHex + "-1234567",
			expectedErr: domain.ErrInvalidShareFormat,
		},
		{
			name:        "Long CRC suffix",
			input:       "1-" + validRawHex + "-123456789",
			expectedErr: domain.ErrInvalidShareFormat,
		},
		{
			name:        "Non-hex CRC suffix",
			input:       "1-" + validRawHex + "-1234567Z",
			expectedErr: domain.ErrInvalidShareFormat,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := domain.ParseCustodianShare(tc.input)
			if err == nil {
				t.Fatalf("expected error %v, got nil", tc.expectedErr)
			}
			if !errors.Is(err, tc.expectedErr) {
				t.Fatalf("expected error wrapping %v, got %v", tc.expectedErr, err)
			}
		})
	}

	// Verify whitespace tolerance and case insensitivity
	t.Run("Whitespace and lowercase normalization", func(t *testing.T) {
		paddedLower := "  " + strings.ToLower(formatted) + " \t\n"
		idx, hexStr, err := domain.ParseCustodianShare(paddedLower)
		if err != nil {
			t.Fatalf("failed parsing lowercase/whitespace padded share: %v", err)
		}
		if idx != 3 {
			t.Errorf("expected idx 3, got %d", idx)
		}
		if hexStr != validRawHex {
			t.Errorf("expected %s, got %s", validRawHex, hexStr)
		}
	})
}

func BenchmarkCalculateShareCRC32(b *testing.B) {
	raw := make([]byte, 32)
	_, _ = rand.Read(raw)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = domain.CalculateShareCRC32(1, raw)
	}
}

func BenchmarkFormatCustodianShare(b *testing.B) {
	rawHex := strings.Repeat("A", 64)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = domain.FormatCustodianShare(1, rawHex)
	}
}

func BenchmarkParseCustodianShare(b *testing.B) {
	rawHex := strings.Repeat("A", 64)
	formatted, _ := domain.FormatCustodianShare(1, rawHex)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = domain.ParseCustodianShare(formatted)
	}
}
