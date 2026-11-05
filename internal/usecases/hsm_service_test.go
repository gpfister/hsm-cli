package usecases

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gpfister/nrf52840-hsm-cli/internal/domain"
)

type dfuMockTransport struct {
	commands []string
	chunks   bytes.Buffer
	applied  bool
	locked   bool
}

func (m *dfuMockTransport) SendCommand(cmd string, timeout time.Duration) ([]string, error) {
	m.commands = append(m.commands, cmd)

	switch {
	case cmd == "AT+DFU_INFO?":
		sha := strings.Repeat("BB", 32)
		return []string{fmt.Sprintf(`+DFU_INFO: VER="1.0.0",SLOT=0,SIZE=%d,SHA256="%s"`, m.chunks.Len(), sha)}, nil
	case strings.HasPrefix(cmd, "AT+DFU_CHUNK="):
		if m.locked {
			return nil, domain.ErrDeviceLocked
		}
		rest := strings.TrimPrefix(cmd, "AT+DFU_CHUNK=")
		parts := strings.SplitN(rest, ",", 2)
		if len(parts) != 2 {
			return nil, domain.ErrInvalidParameter
		}
		b, err := hex.DecodeString(parts[1])
		if err != nil {
			return nil, domain.ErrInvalidParameter
		}
		m.chunks.Write(b)
		return []string{}, nil
	case cmd == "AT+DFU_APPLY":
		if m.locked {
			return nil, domain.ErrDeviceLocked
		}
		m.applied = true
		return []string{}, nil
	default:
		return nil, fmt.Errorf("unexpected command: %s", cmd)
	}
}

func (m *dfuMockTransport) Close() error     { return nil }
func (m *dfuMockTransport) PortName() string { return "mock-dfu" }

func TestHSMService_GetFirmwareInfo_WithATClient(t *testing.T) {
	mt := &dfuMockTransport{}
	mt.chunks.Write(make([]byte, 1024))
	svc := NewHSMServiceWithAT(domain.NewATClient(mt))

	info, err := svc.GetFirmwareInfo()
	if err != nil {
		t.Fatalf("GetFirmwareInfo failed: %v", err)
	}

	if info.VersionString() != "v1.0.0" {
		t.Errorf("expected v1.0.0, got %s", info.VersionString())
	}
	if info.ImageSize != 1024 {
		t.Errorf("expected ImageSize=1024, got %d", info.ImageSize)
	}
}

func TestHSMService_UpdateFirmware_WithATClient(t *testing.T) {
	tmpDir := t.TempDir()
	fwPath := filepath.Join(tmpDir, "firmware.bin")

	// Create 600-byte payload (256 + 256 + 88 bytes = 3 chunks)
	payload := make([]byte, 600)
	for i := range payload {
		payload[i] = byte(i & 0xFF)
	}
	if err := os.WriteFile(fwPath, payload, 0600); err != nil {
		t.Fatalf("failed to write temp firmware file: %v", err)
	}

	mt := &dfuMockTransport{}
	svc := NewHSMServiceWithAT(domain.NewATClient(mt))

	var progressCalls int
	var lastPct float64
	err := svc.UpdateFirmware(fwPath, func(sent, total int, pct float64) {
		progressCalls++
		lastPct = pct
	})
	if err != nil {
		t.Fatalf("UpdateFirmware failed: %v", err)
	}

	if progressCalls != 3 {
		t.Errorf("expected 3 progress callbacks, got %d", progressCalls)
	}
	if lastPct != 100.0 {
		t.Errorf("expected final progress 100.0%%, got %f", lastPct)
	}
	if !mt.applied {
		t.Errorf("expected AT+DFU_APPLY to be called")
	}
	if !bytes.Equal(mt.chunks.Bytes(), payload) {
		t.Errorf("streamed bytes do not match original payload")
	}
}

func TestHSMService_UpdateFirmware_LockedDeviceError(t *testing.T) {
	tmpDir := t.TempDir()
	fwPath := filepath.Join(tmpDir, "firmware.bin")
	if err := os.WriteFile(fwPath, []byte{0x01, 0x02, 0x03, 0x04}, 0600); err != nil {
		t.Fatalf("failed to write temp firmware file: %v", err)
	}

	mt := &dfuMockTransport{locked: true}
	svc := NewHSMServiceWithAT(domain.NewATClient(mt))

	err := svc.UpdateFirmware(fwPath, nil)
	if !errors.Is(err, domain.ErrDeviceLocked) {
		t.Errorf("expected ErrDeviceLocked when device is locked, got %v", err)
	}
}
