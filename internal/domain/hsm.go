package domain

import (
	"errors"
	"fmt"
)

// ThresholdInitResult contains the outcome of a threshold initialization ceremony.
type ThresholdInitResult struct {
	Threshold int              `json:"threshold"`
	Total     int              `json:"total"`
	Shares    []CustodianShare `json:"shares"`
}

// MigrationEnvelope represents an encrypted key container for secure blind migration between HSMs.
type MigrationEnvelope struct {
	SlotID       int    `json:"slot_id"`
	Algorithm    string `json:"algorithm"`
	EphemeralPub string `json:"ephemeral_pub"` // Target or ephemeral ECDH public key (hex)
	EncryptedKey string `json:"encrypted_key"` // AES-GCM encrypted key material (hex)
	AuthTag      string `json:"auth_tag"`      // AES-GCM authentication tag (hex)
	Nonce        string `json:"nonce"`         // Initialization vector / nonce (hex)
}

// Validate checks that all required migration envelope fields are present.
func (m *MigrationEnvelope) Validate() error {
	if m.SlotID <= 0 {
		return errors.New("invalid slot id in migration envelope")
	}
	if m.Algorithm == "" {
		return errors.New("missing algorithm in migration envelope")
	}
	if m.EphemeralPub == "" {
		return errors.New("missing ephemeral public key in migration envelope")
	}
	if m.EncryptedKey == "" {
		return errors.New("missing encrypted key data in migration envelope")
	}
	if m.AuthTag == "" {
		return errors.New("missing authentication tag in migration envelope")
	}
	if m.Nonce == "" {
		return errors.New("missing nonce in migration envelope")
	}
	return nil
}

func (m *MigrationEnvelope) String() string {
	return fmt.Sprintf("MigrationEnvelope[Slot=%d, Algo=%s, Ephem=%s..., Cipher=%d bytes]",
		m.SlotID, m.Algorithm, truncateString(m.EphemeralPub, 16), len(m.EncryptedKey)/2)
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
