package protocol

import (
	"bytes"
	"testing"
	"github.com/gpfister/nrf52840-hms-cli/internal/domain"
)

func TestEncodeDecodeEcho(t *testing.T) {
	testPayload := []byte("Hello HSM Dongle")
	encoded := EncodePacket(domain.CmdEcho, testPayload)

	if len(encoded) != HeaderSize+len(testPayload) {
		t.Fatalf("expected packet length %d, got %d", HeaderSize+len(testPayload), len(encoded))
	}
	if encoded[0] != domain.CmdEcho {
		t.Fatalf("expected command 0x%02X, got 0x%02X", domain.CmdEcho, encoded[0])
	}

	cmd, payload, err := DecodePacket(encoded)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if cmd != domain.CmdEcho {
		t.Fatalf("expected cmd 0x%02X, got 0x%02X", domain.CmdEcho, cmd)
	}
	if !bytes.Equal(payload, testPayload) {
		t.Fatalf("expected payload %q, got %q", testPayload, payload)
	}
}

func TestDecodeErrorPacket(t *testing.T) {
	errPacket := []byte{domain.RspError, 0x00, 0x01, 0x08}
	_, _, err := DecodePacket(errPacket)
	if err == nil {
		t.Fatalf("expected error for error packet, got nil")
	}
}
