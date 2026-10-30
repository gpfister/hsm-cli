package usecases_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gpfister/nrf52840-hsm-cli/internal/domain"
	"github.com/gpfister/nrf52840-hsm-cli/internal/usecases"
)

type mockMigrationTransport struct {
	responses   map[string][]string
	callHistory []string
}

func newMockMigrationTransport() *mockMigrationTransport {
	return &mockMigrationTransport{
		responses: make(map[string][]string),
	}
}

func (m *mockMigrationTransport) SendCommand(cmd string, timeout time.Duration) ([]string, error) {
	m.callHistory = append(m.callHistory, cmd)
	if cmd == "AT+LOCK" {
		m.responses["AT+STATUS?"] = []string{`+STATUS: AUTH=0,RETRIES=3,KEYCOUNT=1`}
	}
	if resp, exists := m.responses[cmd]; exists {
		return resp, nil
	}
	return nil, errors.New("unexpected command: " + cmd)
}

func (m *mockMigrationTransport) Close() error     { return nil }
func (m *mockMigrationTransport) PortName() string { return "mock" }

func TestMigrationService_ExecuteMigration_Success(t *testing.T) {
	sourceTransport := newMockMigrationTransport()
	targetTransport := newMockMigrationTransport()

	targetPub := "040102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"

	// Target expectations
	targetTransport.responses["AT+MIGRATE=INIT_TARGET"] = []string{
		`+TARGET: ` + targetPub,
	}
	targetTransport.responses["AT+MIGRATE=UNWRAP,1,EC_P256,"+targetPub+",cipher123,tag456,nonce789"] = []string{
		`OK`,
	}
	targetTransport.responses["AT+LOCK"] = []string{`OK`}
	targetTransport.responses["AT+STATUS?"] = []string{
		`+STATUS: AUTH=0,RETRIES=3,KEYCOUNT=1`,
	}

	// Source expectations (already unlocked)
	sourceTransport.responses["AT+STATUS?"] = []string{
		`+STATUS: AUTH=1,RETRIES=3,KEYCOUNT=1`,
	}
	sourceTransport.responses["AT+MIGRATE=WRAP,1,"+targetPub] = []string{
		`+WRAP: 1,EC_P256,` + targetPub + `,cipher123,tag456,nonce789`,
	}
	sourceTransport.responses["AT+LOCK"] = []string{`OK`}

	sourceClient := domain.NewATClient(sourceTransport)
	targetClient := domain.NewATClient(targetTransport)

	var out bytes.Buffer
	svc := usecases.NewMigrationService(nil, &out)
	err := svc.ExecuteMigration(sourceClient, targetClient, 1)
	if err != nil {
		t.Fatalf("ExecuteMigration failed: %v", err)
	}

	if !strings.Contains(out.String(), "SUCCESS: Key slot 1 securely migrated") {
		t.Errorf("expected success banner in output, got: %s", out.String())
	}
	if !strings.Contains(out.String(), "Both devices confirmed locked (Solid RED LED)") {
		t.Errorf("expected locked confirmation in output, got: %s", out.String())
	}
}

func TestMigrationService_Validation(t *testing.T) {
	svc := usecases.NewMigrationService(nil, nil)

	// Same port
	err := svc.MigratePorts("/dev/cu.modem1", "/dev/cu.modem1", 1)
	if err == nil || !strings.Contains(err.Error(), "cannot be the same") {
		t.Errorf("expected same port error, got: %v", err)
	}

	// Invalid slot
	sourceTransport := newMockMigrationTransport()
	targetTransport := newMockMigrationTransport()
	sourceClient := domain.NewATClient(sourceTransport)
	targetClient := domain.NewATClient(targetTransport)

	err = svc.ExecuteMigration(sourceClient, targetClient, 0)
	if err == nil || !strings.Contains(err.Error(), "slot ID must be >= 1") {
		t.Errorf("expected slot ID error, got: %v", err)
	}
}
