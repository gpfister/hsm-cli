package domain

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Transport represents the underlying command/response transport for the AT protocol.
type Transport interface {
	SendCommand(cmd string, timeout time.Duration) ([]string, error)
	Close() error
	PortName() string
}

// ATClient provides high-level domain operations for the nRF52840 HSM AT protocol.
type ATClient struct {
	transport Transport
}

// NewATClient constructs an ATClient with the given transport.
func NewATClient(t Transport) *ATClient {
	return &ATClient{transport: t}
}

// Transport returns the underlying transport.
func (c *ATClient) Transport() Transport {
	return c.transport
}

// Ping verifies serial communication and responsiveness via AT.
func (c *ATClient) Ping() error {
	_, err := c.transport.SendCommand("AT", 3*time.Second)
	return err
}

// Info queries device build information, hardware model, UID, and bootloader status via AT+INFO?.
func (c *ATClient) Info() (*DeviceInfo, error) {
	lines, err := c.transport.SendCommand("AT+INFO?", 3*time.Second)
	if err != nil {
		return nil, err
	}

	var infoLine string
	for _, l := range lines {
		if strings.HasPrefix(l, "+INFO:") {
			infoLine = l
			break
		}
	}

	if infoLine == "" {
		return nil, errors.New("missing +INFO: line in device response")
	}

	return parseInfoLine(infoLine)
}

// Status queries authentication state, retries remaining, and key count via AT+STATUS?.
func (c *ATClient) Status() (*DeviceStatus, error) {
	lines, err := c.transport.SendCommand("AT+STATUS?", 3*time.Second)
	if err != nil {
		return nil, err
	}

	var statusLine string
	for _, l := range lines {
		if strings.HasPrefix(l, "+STATUS:") {
			statusLine = l
			break
		}
	}

	if statusLine == "" {
		return nil, errors.New("missing +STATUS: line in device response")
	}

	return parseStatusLine(statusLine)
}

// Auth verifies User PIN and transitions HSM to UNLOCKED state via AT+AUTH=<pin>.
func (c *ATClient) Auth(pin string) error {
	if pin == "" {
		return errors.New("PIN cannot be empty")
	}
	_, err := c.transport.SendCommand(fmt.Sprintf("AT+AUTH=%s", pin), 3*time.Second)
	return err
}

// Lock invalidates the active authentication session via AT+LOCK.
func (c *ATClient) Lock() error {
	_, err := c.transport.SendCommand("AT+LOCK", 3*time.Second)
	return err
}

// Reset clears volatile session tokens and triggers internal re-initialization via AT+RESET.
func (c *ATClient) Reset() error {
	_, err := c.transport.SendCommand("AT+RESET", 3*time.Second)
	return err
}

// Slots enumerates all stored keys via AT+SLOTS?.
func (c *ATClient) Slots() ([]KeySlot, error) {
	lines, err := c.transport.SendCommand("AT+SLOTS?", 3*time.Second)
	if err != nil {
		return nil, err
	}

	var slots []KeySlot
	for _, l := range lines {
		if strings.HasPrefix(l, "+SLOTS:") {
			slot, err := parseSlotLine(l)
			if err == nil {
				slots = append(slots, *slot)
			}
		}
	}
	return slots, nil
}

// KeyGen generates an EC keypair in the specified slot via AT+KEYGEN=<id>,<algo>.
func (c *ATClient) KeyGen(slot int, algo string) (*KeyGenResult, error) {
	if slot < 1 || slot > 16 {
		return nil, fmt.Errorf("slot ID %d out of range (1..16)", slot)
	}
	if algo == "" {
		algo = "EC_P256"
	}

	cmd := fmt.Sprintf("AT+KEYGEN=%d,%s", slot, algo)
	lines, err := c.transport.SendCommand(cmd, 5*time.Second)
	if err != nil {
		return nil, err
	}

	for _, l := range lines {
		if strings.HasPrefix(l, "+KEYGEN:") {
			return parseKeyGenLine(l)
		}
	}

	return nil, errors.New("missing +KEYGEN: response line")
}

// PubKey reads the uncompressed public key point (65 bytes hex) from the given slot via AT+PUBKEY=<id>.
func (c *ATClient) PubKey(slot int) (string, error) {
	if slot < 1 || slot > 16 {
		return "", fmt.Errorf("slot ID %d out of range (1..16)", slot)
	}

	cmd := fmt.Sprintf("AT+PUBKEY=%d", slot)
	lines, err := c.transport.SendCommand(cmd, 3*time.Second)
	if err != nil {
		return "", err
	}

	for _, l := range lines {
		if strings.HasPrefix(l, "+PUBKEY:") {
			res, err := parsePubKeyLine(l)
			if err != nil {
				return "", err
			}
			return res.PublicKeyHex, nil
		}
	}

	return "", errors.New("missing +PUBKEY: response line")
}

// Sign signs a 32-byte SHA-256 digest using the specified slot via AT+SIGN=<id>,<hash>.
func (c *ATClient) Sign(slot int, hashHex string) (string, error) {
	if slot < 1 || slot > 16 {
		return "", fmt.Errorf("slot ID %d out of range (1..16)", slot)
	}

	cleanHash := strings.TrimPrefix(strings.TrimSpace(hashHex), "0x")
	cleanHash = strings.ToUpper(cleanHash)

	if len(cleanHash) != 64 {
		return "", fmt.Errorf("invalid hash length (%d hex chars, expected 64 chars / 32 bytes)", len(cleanHash))
	}

	if _, err := hex.DecodeString(cleanHash); err != nil {
		return "", fmt.Errorf("invalid hex in hash: %w", err)
	}

	cmd := fmt.Sprintf("AT+SIGN=%d,%s", slot, cleanHash)
	lines, err := c.transport.SendCommand(cmd, 5*time.Second)
	if err != nil {
		return "", err
	}

	for _, l := range lines {
		if strings.HasPrefix(l, "+SIGN:") {
			sig := strings.TrimSpace(strings.TrimPrefix(l, "+SIGN:"))
			return sig, nil
		}
	}

	return "", errors.New("missing +SIGN: response line")
}

// Random requests len random bytes from the hardware TRNG via AT+RANDOM=<len>.
func (c *ATClient) Random(length int) ([]byte, error) {
	if length < 1 || length > 128 {
		return nil, fmt.Errorf("random length %d out of range (1..128)", length)
	}

	cmd := fmt.Sprintf("AT+RANDOM=%d", length)
	lines, err := c.transport.SendCommand(cmd, 3*time.Second)
	if err != nil {
		return nil, err
	}

	for _, l := range lines {
		if strings.HasPrefix(l, "+RANDOM:") {
			hexStr := strings.TrimSpace(strings.TrimPrefix(l, "+RANDOM:"))
			bytes, err := hex.DecodeString(hexStr)
			if err != nil {
				return nil, fmt.Errorf("failed to decode random hex response: %w", err)
			}
			return bytes, nil
		}
	}

	return nil, errors.New("missing +RANDOM: response line")
}

// Helper parsing routines

func parseInfoLine(line string) (*DeviceInfo, error) {
	content := strings.TrimSpace(strings.TrimPrefix(line, "+INFO:"))
	info := &DeviceInfo{}

	params := splitQuotedCsv(content)
	for _, p := range params {
		parts := strings.SplitN(p, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToUpper(strings.TrimSpace(parts[0]))
		val := strings.Trim(strings.TrimSpace(parts[1]), "\"")

		switch key {
		case "FW":
			info.Firmware = val
		case "HW":
			info.Hardware = val
		case "UID":
			info.UID = val
		case "BOOT":
			info.Boot = val
		}
	}

	return info, nil
}

func parseStatusLine(line string) (*DeviceStatus, error) {
	content := strings.TrimSpace(strings.TrimPrefix(line, "+STATUS:"))
	status := &DeviceStatus{
		RetriesRemaining: 3,
	}

	params := splitQuotedCsv(content)
	for _, p := range params {
		parts := strings.SplitN(p, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToUpper(strings.TrimSpace(parts[0]))
		val := strings.Trim(strings.TrimSpace(parts[1]), "\"")

		switch key {
		case "AUTH":
			status.Authenticated = (val == "1" || strings.EqualFold(val, "true"))
		case "RETRIES":
			if r, err := strconv.Atoi(val); err == nil {
				status.RetriesRemaining = r
			}
		case "KEYCOUNT":
			if k, err := strconv.Atoi(val); err == nil {
				status.KeyCount = k
			}
		}
	}

	return status, nil
}

func parseSlotLine(line string) (*KeySlot, error) {
	content := strings.TrimSpace(strings.TrimPrefix(line, "+SLOTS:"))
	parts := strings.Split(content, ",")
	if len(parts) < 3 {
		return nil, fmt.Errorf("malformed +SLOTS line: %s", line)
	}

	id, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return nil, fmt.Errorf("invalid slot id: %w", err)
	}

	algo := strings.TrimSpace(parts[1])
	size, err := strconv.Atoi(strings.TrimSpace(parts[2]))
	if err != nil {
		size = 256
	}

	label := ""
	if len(parts) >= 4 {
		label = strings.Trim(strings.TrimSpace(parts[3]), "\"")
	}

	return &KeySlot{
		ID:        id,
		Algorithm: algo,
		Size:      size,
		Label:     label,
	}, nil
}

func parseKeyGenLine(line string) (*KeyGenResult, error) {
	content := strings.TrimSpace(strings.TrimPrefix(line, "+KEYGEN:"))
	parts := strings.SplitN(content, ",", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("malformed +KEYGEN line: %s", line)
	}

	id, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return nil, fmt.Errorf("invalid slot id: %w", err)
	}

	pubPart := strings.TrimSpace(parts[1])
	kv := strings.SplitN(pubPart, "=", 2)
	pubHex := ""
	if len(kv) == 2 {
		pubHex = strings.Trim(strings.TrimSpace(kv[1]), "\"")
	} else {
		pubHex = strings.Trim(pubPart, "\"")
	}

	return &KeyGenResult{
		ID:           id,
		PublicKeyHex: pubHex,
	}, nil
}

func parsePubKeyLine(line string) (*KeyGenResult, error) {
	content := strings.TrimSpace(strings.TrimPrefix(line, "+PUBKEY:"))
	parts := strings.SplitN(content, ",", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("malformed +PUBKEY line: %s", line)
	}

	id, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return nil, fmt.Errorf("invalid slot id: %w", err)
	}

	pubPart := strings.TrimSpace(parts[1])
	kv := strings.SplitN(pubPart, "=", 2)
	pubHex := ""
	if len(kv) == 2 {
		pubHex = strings.Trim(strings.TrimSpace(kv[1]), "\"")
	} else {
		pubHex = strings.Trim(pubPart, "\"")
	}

	return &KeyGenResult{
		ID:           id,
		PublicKeyHex: pubHex,
	}, nil
}

// splitQuotedCsv splits comma-separated values while respecting quotes.
func splitQuotedCsv(s string) []string {
	var entries []string
	var current strings.Builder
	inQuotes := false

	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch ch {
		case '"':
			inQuotes = !inQuotes
			current.WriteByte(ch)
		case ',':
			if inQuotes {
				current.WriteByte(ch)
			} else {
				entries = append(entries, strings.TrimSpace(current.String()))
				current.Reset()
			}
		default:
			current.WriteByte(ch)
		}
	}

	if current.Len() > 0 {
		entries = append(entries, strings.TrimSpace(current.String()))
	}

	return entries
}
