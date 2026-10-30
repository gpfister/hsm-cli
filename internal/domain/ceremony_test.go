package domain_test

import (
	"strings"
	"testing"

	"github.com/gpfister/nrf52840-hsm-cli/internal/domain"
)

func TestBIP39WordListIntegrity(t *testing.T) {
	if len(domain.BIP39WordList) != 2048 {
		t.Fatalf("expected 2048 BIP-39 words, got %d", len(domain.BIP39WordList))
	}

	if domain.BIP39WordList[0] != "abandon" {
		t.Errorf("expected first BIP-39 word to be 'abandon', got '%s'", domain.BIP39WordList[0])
	}

	if domain.BIP39WordList[2047] != "zoo" {
		t.Errorf("expected last BIP-39 word to be 'zoo', got '%s'", domain.BIP39WordList[2047])
	}

	seen := make(map[string]bool, 2048)
	for i, w := range domain.BIP39WordList {
		if w == "" {
			t.Errorf("empty word found at index %d", i)
		}
		if seen[w] {
			t.Errorf("duplicate word '%s' at index %d", w, i)
		}
		seen[w] = true
	}
}

func TestBIP39LookupAndValidation(t *testing.T) {
	// Valid word
	if !domain.IsValidBIP39Word("ability") {
		t.Errorf("expected 'ability' to be valid BIP-39 word")
	}
	if !domain.IsValidBIP39Word("  ABILITY  ") {
		t.Errorf("expected trimmed/lowercase 'ABILITY' to be valid")
	}

	// Invalid word
	if domain.IsValidBIP39Word("notabipword123") {
		t.Errorf("expected invalid word to return false")
	}

	// WordForIndex
	w0, err := domain.WordForIndex(0)
	if err != nil || w0 != "abandon" {
		t.Errorf("expected index 0 to return 'abandon', got '%s', err=%v", w0, err)
	}

	wLast, err := domain.WordForIndex(2047)
	if err != nil || wLast != "zoo" {
		t.Errorf("expected index 2047 to return 'zoo', got '%s', err=%v", wLast, err)
	}

	if _, err := domain.WordForIndex(-1); err == nil {
		t.Errorf("expected error for negative index")
	}
	if _, err := domain.WordForIndex(2048); err == nil {
		t.Errorf("expected error for index >= 2048")
	}

	// IndexForWord
	idx, err := domain.IndexForWord("abandon")
	if err != nil || idx != 0 {
		t.Errorf("expected 'abandon' to return index 0, got %d, err=%v", idx, err)
	}

	idxLast, err := domain.IndexForWord("ZOO")
	if err != nil || idxLast != 2047 {
		t.Errorf("expected 'ZOO' to return index 2047, got %d, err=%v", idxLast, err)
	}

	if _, err := domain.IndexForWord("unknownword"); err == nil {
		t.Errorf("expected error for unknown word")
	}
}

func TestVerifyCommitmentSequence(t *testing.T) {
	expected := []string{"abandon", "ability", "able"}

	// Exact match
	if err := domain.VerifyCommitmentSequence([]string{"abandon", "ability", "able"}, expected); err != nil {
		t.Errorf("expected valid sequence to pass, got err: %v", err)
	}

	// Case and whitespace insensitive
	if err := domain.VerifyCommitmentSequence([]string{" ABANDON ", "Ability", "able "}, expected); err != nil {
		t.Errorf("expected trimmed case-insensitive match to pass, got err: %v", err)
	}

	// Mismatched word
	err := domain.VerifyCommitmentSequence([]string{"abandon", "wrong", "able"}, expected)
	if err == nil || !strings.Contains(err.Error(), "mismatch at index 2") {
		t.Errorf("expected mismatch error at index 2, got: %v", err)
	}

	// Length mismatch
	err = domain.VerifyCommitmentSequence([]string{"abandon", "ability"}, expected)
	if err == nil || !strings.Contains(err.Error(), "count mismatch") {
		t.Errorf("expected count mismatch error, got: %v", err)
	}
}

func TestMigrationEnvelopeValidation(t *testing.T) {
	env := &domain.MigrationEnvelope{
		SlotID:       1,
		Algorithm:    "EC_P256",
		EphemeralPub: "040102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20",
		EncryptedKey: "aabbccddeeff00112233",
		AuthTag:      "11223344556677889900aabbccddeeff",
		Nonce:        "0102030405060708090a0b0c",
	}

	if err := env.Validate(); err != nil {
		t.Errorf("expected valid envelope to pass validation, got: %v", err)
	}

	if !strings.Contains(env.String(), "MigrationEnvelope") {
		t.Errorf("expected string representation to contain MigrationEnvelope")
	}

	// Missing fields
	invalid := *env
	invalid.SlotID = 0
	if err := invalid.Validate(); err == nil {
		t.Errorf("expected validation failure on invalid slot ID")
	}

	invalid = *env
	invalid.EphemeralPub = ""
	if err := invalid.Validate(); err == nil {
		t.Errorf("expected validation failure on empty ephemeral pub")
	}

	invalid = *env
	invalid.EncryptedKey = ""
	if err := invalid.Validate(); err == nil {
		t.Errorf("expected validation failure on empty encrypted key")
	}

	invalid = *env
	invalid.AuthTag = ""
	if err := invalid.Validate(); err == nil {
		t.Errorf("expected validation failure on empty auth tag")
	}

	invalid = *env
	invalid.Nonce = ""
	if err := invalid.Validate(); err == nil {
		t.Errorf("expected validation failure on empty nonce")
	}
}
