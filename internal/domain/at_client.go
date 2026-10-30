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

// ThresholdInit executes threshold initialization (e.g. 3-of-5) via AT+INIT=THRESHOLD,<k>,<n>.
func (c *ATClient) ThresholdInit(threshold, total int) (*ThresholdInitResult, error) {
	if threshold < 2 || total < threshold {
		return nil, fmt.Errorf("invalid threshold parameters: threshold=%d, total=%d (must be 2 <= k <= n)", threshold, total)
	}

	lines, err := c.transport.SendCommand(fmt.Sprintf("AT+INIT=THRESHOLD,%d,%d", threshold, total), 10*time.Second)
	if err != nil {
		return nil, err
	}

	var shares []CustodianShare
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "+SHARE:") {
			share, err := parseShareLine(trimmed)
			if err != nil {
				return nil, err
			}
			shares = append(shares, *share)
		}
	}

	if len(shares) == 0 {
		return nil, errors.New("no custodian shares returned by HSM")
	}

	return &ThresholdInitResult{
		Threshold: threshold,
		Total:     total,
		Shares:    shares,
	}, nil
}

// SubmitShare submits a single custodian share during threshold authentication via AT+SHARE=<index>,<shareHex>.
func (c *ATClient) SubmitShare(index int, shareHex string) (*WordCommitment, error) {
	if index <= 0 {
		return nil, errors.New("custodian index must be >= 1")
	}
	if strings.TrimSpace(shareHex) == "" {
		return nil, errors.New("share cannot be empty")
	}

	lines, err := c.transport.SendCommand(fmt.Sprintf("AT+SHARE=%d,%s", index, strings.TrimSpace(shareHex)), 5*time.Second)
	if err != nil {
		return nil, err
	}

	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "+COMMIT:") || strings.HasPrefix(trimmed, "+WORD:") {
			return parseCommitLine(trimmed, index)
		}
	}

	return nil, errors.New("missing +COMMIT: line in device response")
}

// TargetInit generates an ephemeral ECDH public key on the target HSM for blind migration via AT+MIGRATE=INIT_TARGET.
func (c *ATClient) TargetInit() (string, error) {
	lines, err := c.transport.SendCommand("AT+MIGRATE=INIT_TARGET", 5*time.Second)
	if err != nil {
		return "", err
	}

	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "+TARGET:") || strings.HasPrefix(trimmed, "+MIGRATE:") || strings.HasPrefix(trimmed, "+PUBKEY:") {
			return parseTargetInitLine(trimmed)
		}
	}

	return "", errors.New("missing +TARGET: line in target device response")
}

// MigrateWrap wraps (encrypts) the key in slotID using targetPubKeyHex via AT+MIGRATE=WRAP,<slotID>,<targetPubKeyHex>.
func (c *ATClient) MigrateWrap(slotID int, targetPubKeyHex string) (*MigrationEnvelope, error) {
	if slotID <= 0 {
		return nil, errors.New("slot ID must be >= 1")
	}
	if strings.TrimSpace(targetPubKeyHex) == "" {
		return nil, errors.New("target public key cannot be empty")
	}

	lines, err := c.transport.SendCommand(fmt.Sprintf("AT+MIGRATE=WRAP,%d,%s", slotID, strings.TrimSpace(targetPubKeyHex)), 10*time.Second)
	if err != nil {
		return nil, err
	}

	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "+WRAP:") {
			env, err := parseWrapLine(trimmed, slotID, targetPubKeyHex)
			if err != nil {
				return nil, err
			}
			if err := env.Validate(); err != nil {
				return nil, fmt.Errorf("invalid envelope returned: %w", err)
			}
			return env, nil
		}
	}

	return nil, errors.New("missing +WRAP: line in source device response")
}

// MigrateUnwrap imports and decrypts the encrypted key envelope on the target HSM via AT+MIGRATE=UNWRAP,...
func (c *ATClient) MigrateUnwrap(env *MigrationEnvelope) error {
	if env == nil {
		return errors.New("migration envelope cannot be nil")
	}
	if err := env.Validate(); err != nil {
		return fmt.Errorf("invalid migration envelope: %w", err)
	}

	cmd := fmt.Sprintf("AT+MIGRATE=UNWRAP,%d,%s,%s,%s,%s,%s",
		env.SlotID, env.Algorithm, env.EphemeralPub, env.EncryptedKey, env.AuthTag, env.Nonce)
	_, err := c.transport.SendCommand(cmd, 10*time.Second)
	return err
}

func parseShareLine(line string) (*CustodianShare, error) {
	content := strings.TrimSpace(strings.TrimPrefix(line, "+SHARE:"))
	if strings.Contains(content, "=") {
		parts := strings.Split(content, ",")
		var share CustodianShare
		for _, p := range parts {
			kv := strings.SplitN(strings.TrimSpace(p), "=", 2)
			if len(kv) == 2 {
				k := strings.ToUpper(strings.TrimSpace(kv[0]))
				v := strings.Trim(strings.TrimSpace(kv[1]), "\"")
				switch k {
				case "INDEX", "ID":
					id, _ := strconv.Atoi(v)
					share.Index = id
				case "SHARE", "KEY":
					share.Share = v
				case "WORD", "COMMIT", "COMMITMENT":
					share.CommitmentWord = v
				}
			}
		}
		if share.Index > 0 && share.Share != "" {
			return &share, nil
		}
	}

	parts := strings.Split(content, ",")
	if len(parts) < 2 {
		return nil, fmt.Errorf("malformed +SHARE line: %s", line)
	}
	idx, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return nil, fmt.Errorf("invalid share index in '%s': %w", line, err)
	}
	shareHex := strings.Trim(strings.TrimSpace(parts[1]), "\"")
	var commitmentWord string
	if len(parts) >= 3 {
		commitmentWord = strings.Trim(strings.TrimSpace(parts[2]), "\"")
	}

	return &CustodianShare{
		Index:          idx,
		Share:          shareHex,
		CommitmentWord: commitmentWord,
	}, nil
}

func parseCommitLine(line string, fallbackIndex int) (*WordCommitment, error) {
	content := strings.TrimSpace(line)
	if strings.HasPrefix(content, "+COMMIT:") {
		content = strings.TrimSpace(strings.TrimPrefix(content, "+COMMIT:"))
	} else if strings.HasPrefix(content, "+WORD:") {
		content = strings.TrimSpace(strings.TrimPrefix(content, "+WORD:"))
	}

	if strings.Contains(content, "=") {
		parts := strings.Split(content, ",")
		idx := fallbackIndex
		word := ""
		for _, p := range parts {
			kv := strings.SplitN(strings.TrimSpace(p), "=", 2)
			if len(kv) == 2 {
				k := strings.ToUpper(strings.TrimSpace(kv[0]))
				v := strings.Trim(strings.TrimSpace(kv[1]), "\"")
				if k == "INDEX" || k == "ID" {
					if id, err := strconv.Atoi(v); err == nil {
						idx = id
					}
				} else if k == "WORD" || k == "COMMIT" {
					word = v
				}
			}
		}
		if word != "" {
			return &WordCommitment{Index: idx, Word: word}, nil
		}
	}

	parts := strings.Split(content, ",")
	if len(parts) == 1 {
		return &WordCommitment{
			Index: fallbackIndex,
			Word:  strings.Trim(strings.TrimSpace(parts[0]), "\""),
		}, nil
	}
	idx, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		idx = fallbackIndex
	}
	return &WordCommitment{
		Index: idx,
		Word:  strings.Trim(strings.TrimSpace(parts[1]), "\""),
	}, nil
}

func parseTargetInitLine(line string) (string, error) {
	content := strings.TrimSpace(line)
	for _, prefix := range []string{"+TARGET:", "+MIGRATE:", "+PUBKEY:"} {
		if strings.HasPrefix(content, prefix) {
			content = strings.TrimSpace(strings.TrimPrefix(content, prefix))
			break
		}
	}

	if strings.Contains(content, "=") {
		parts := strings.Split(content, ",")
		for _, p := range parts {
			kv := strings.SplitN(strings.TrimSpace(p), "=", 2)
			if len(kv) == 2 {
				k := strings.ToUpper(strings.TrimSpace(kv[0]))
				v := strings.Trim(strings.TrimSpace(kv[1]), "\"")
				if k == "PUB" || k == "PUBKEY" || k == "TARGET" || k == "KEY" {
					return v, nil
				}
			}
		}
	}

	pub := strings.Trim(strings.TrimSpace(content), "\"")
	if pub == "" {
		return "", fmt.Errorf("empty public key in line: %s", line)
	}
	return pub, nil
}

func parseWrapLine(line string, defaultSlot int, defaultTargetPub string) (*MigrationEnvelope, error) {
	content := strings.TrimSpace(strings.TrimPrefix(line, "+WRAP:"))
	env := &MigrationEnvelope{
		SlotID:       defaultSlot,
		EphemeralPub: defaultTargetPub,
		Algorithm:    "EC_P256",
	}

	if strings.Contains(content, "=") {
		parts := strings.Split(content, ",")
		for _, p := range parts {
			kv := strings.SplitN(strings.TrimSpace(p), "=", 2)
			if len(kv) == 2 {
				k := strings.ToUpper(strings.TrimSpace(kv[0]))
				v := strings.Trim(strings.TrimSpace(kv[1]), "\"")
				switch k {
				case "SLOT", "SLOT_ID", "ID":
					if id, err := strconv.Atoi(v); err == nil {
						env.SlotID = id
					}
				case "ALGO", "ALGORITHM":
					env.Algorithm = v
				case "EPHEM", "EPHEMERAL", "PUB", "PUBKEY":
					env.EphemeralPub = v
				case "CIPHER", "ENCRYPTED", "KEY":
					env.EncryptedKey = v
				case "TAG", "AUTHTAG", "MAC":
					env.AuthTag = v
				case "NONCE", "IV":
					env.Nonce = v
				}
			}
		}
		return env, nil
	}

	parts := strings.Split(content, ",")
	if len(parts) >= 6 {
		if id, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil {
			env.SlotID = id
		}
		env.Algorithm = strings.Trim(strings.TrimSpace(parts[1]), "\"")
		env.EphemeralPub = strings.Trim(strings.TrimSpace(parts[2]), "\"")
		env.EncryptedKey = strings.Trim(strings.TrimSpace(parts[3]), "\"")
		env.AuthTag = strings.Trim(strings.TrimSpace(parts[4]), "\"")
		env.Nonce = strings.Trim(strings.TrimSpace(parts[5]), "\"")
		return env, nil
	} else if len(parts) >= 4 {
		// slot, cipher, tag, nonce (using defaults for algo and ephem)
		if id, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil {
			env.SlotID = id
		}
		env.EncryptedKey = strings.Trim(strings.TrimSpace(parts[1]), "\"")
		env.AuthTag = strings.Trim(strings.TrimSpace(parts[2]), "\"")
		env.Nonce = strings.Trim(strings.TrimSpace(parts[3]), "\"")
		return env, nil
	}

	return nil, fmt.Errorf("malformed +WRAP line: %s", line)
}
