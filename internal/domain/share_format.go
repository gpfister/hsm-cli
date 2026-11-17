package domain

import (
	"encoding/hex"
	"errors"
	"fmt"
	"hash/crc32"
	"strconv"
	"strings"
)

var (
	ErrInvalidShareFormat   = errors.New("invalid custodian share format: expected '<index>-<64-hex-share>-<8-hex-crc32>'")
	ErrInvalidShareIndex    = errors.New("custodian index out of range: must be between 1 and 5")
	ErrInvalidShareHex      = errors.New("invalid share payload: must be exactly 64 hexadecimal characters (32 bytes)")
	ErrInvalidShareChecksum = errors.New("custodian share checksum mismatch: potential transcription typo")
	ErrRawShareDetected     = errors.New("raw 64-hex share detected: formatted share '<index>-<share>-<crc32>' is required to prevent hardware lockout")
)

// CalculateShareCRC32 calculates IEEE 802.3 CRC-32 over [uint8(index) || 32-byte share binary].
func CalculateShareCRC32(index int, shareBytes []byte) uint32 {
	buf := make([]byte, 1+len(shareBytes))
	buf[0] = byte(index)
	copy(buf[1:], shareBytes)
	return crc32.ChecksumIEEE(buf)
}

// FormatCustodianShare packages an index and 64-hex share into '<index>-<64_hex>-<8_hex_crc32>'.
func FormatCustodianShare(index int, shareHex string) (string, error) {
	if index < 1 || index > 5 {
		return "", fmt.Errorf("%w: got %d", ErrInvalidShareIndex, index)
	}

	cleanHex := strings.TrimSpace(shareHex)
	cleanHex = strings.TrimPrefix(cleanHex, "0x")
	cleanHex = strings.TrimPrefix(cleanHex, "0X")

	if len(cleanHex) != 64 {
		return "", fmt.Errorf("%w: length is %d", ErrInvalidShareHex, len(cleanHex))
	}

	shareBytes, err := hex.DecodeString(cleanHex)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidShareHex, err)
	}

	crc := CalculateShareCRC32(index, shareBytes)
	return fmt.Sprintf("%d-%s-%08X", index, strings.ToUpper(cleanHex), crc), nil
}

// ParseCustodianShare parses and verifies '<index>-<64_hex>-<8_hex_crc32>', returning (index, 64_hex, nil).
func ParseCustodianShare(input string) (int, string, error) {
	clean := strings.TrimSpace(input)

	// Check if user submitted a legacy raw 64-hex share (or 0x-prefixed 66-char string)
	unprefixed := strings.TrimPrefix(strings.TrimPrefix(clean, "0x"), "0X")
	if len(unprefixed) == 64 && !strings.Contains(clean, "-") {
		return 0, "", ErrRawShareDetected
	}

	parts := strings.Split(clean, "-")
	if len(parts) != 3 {
		return 0, "", ErrInvalidShareFormat
	}

	idxStr := strings.TrimSpace(parts[0])
	shareHex := strings.TrimSpace(parts[1])
	crcHex := strings.TrimSpace(parts[2])

	if len(idxStr) != 1 || idxStr[0] < '1' || idxStr[0] > '5' {
		return 0, "", fmt.Errorf("%w: got %q", ErrInvalidShareIndex, idxStr)
	}
	index := int(idxStr[0] - '0')

	if len(shareHex) != 64 {
		return 0, "", fmt.Errorf("%w: got %d characters", ErrInvalidShareHex, len(shareHex))
	}

	shareBytes, err := hex.DecodeString(shareHex)
	if err != nil {
		return 0, "", fmt.Errorf("%w: invalid hex: %v", ErrInvalidShareHex, err)
	}

	if len(crcHex) != 8 {
		return 0, "", fmt.Errorf("%w: expected 8 hex characters, got %q", ErrInvalidShareFormat, crcHex)
	}

	expectedCRC := CalculateShareCRC32(index, shareBytes)
	actualCRC, err := strconv.ParseUint(crcHex, 16, 32)
	if err != nil {
		return 0, "", fmt.Errorf("%w: invalid crc hex %q", ErrInvalidShareFormat, crcHex)
	}

	if uint32(actualCRC) != expectedCRC {
		return 0, "", fmt.Errorf("%w: expected %08X, got %08X", ErrInvalidShareChecksum, expectedCRC, uint32(actualCRC))
	}

	return index, strings.ToUpper(shareHex), nil
}
