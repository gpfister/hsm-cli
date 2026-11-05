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
	cmdErrors   map[string]error
	callHistory []string
}

func newMockMigrationTransport() *mockMigrationTransport {
	return &mockMigrationTransport{
		responses: make(map[string][]string),
		cmdErrors: make(map[string]error),
	}
}

func (m *mockMigrationTransport) SendCommand(cmd string, timeout time.Duration) ([]string, error) {
	m.callHistory = append(m.callHistory, cmd)
	if cmd == "AT+LOCK" {
		m.responses["AT+STATUS?"] = []string{`+STATUS: AUTH=0,RETRIES=3,KEYCOUNT=1`}
	}
	if err, exists := m.cmdErrors[cmd]; exists {
		return nil, err
	}
	if resp, exists := m.responses[cmd]; exists {
		for _, l := range resp {
			if strings.HasPrefix(l, "+CME ERROR:") {
				return resp, domain.ErrCryptoFailure
			}
		}
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

func TestMigrationService_ExecuteDeviceMigration_Success(t *testing.T) {
	sourceTransport := newMockMigrationTransport()
	targetTransport := newMockMigrationTransport()

	targetPub := "040102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
	slotPub := "04aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"

	// Source setup: authenticated, has slot 1
	sourceTransport.responses["AT+STATUS?"] = []string{`+STATUS: AUTH=1,RETRIES=3,KEYCOUNT=1`}
	sourceTransport.responses["AT+SLOTS?"] = []string{
		`+SLOTS: 1, EC_P256, 256, "Key1"`,
		`OK`,
	}
	sourceTransport.responses["AT+MIGRATE=WRAP,1,"+targetPub] = []string{
		`+WRAP: 1,EC_P256,` + targetPub + `,cipher123,tag456,nonce789`,
	}
	sourceTransport.responses["AT+PUBKEY=1"] = []string{
		`+PUBKEY: 1,` + slotPub,
		`OK`,
	}
	sourceTransport.responses["AT+LOCK"] = []string{`OK`}

	// Target setup: authenticated
	targetTransport.responses["AT+STATUS?"] = []string{`+STATUS: AUTH=1,RETRIES=3,KEYCOUNT=0`}
	targetTransport.responses["AT+MIGRATE=INIT_TARGET"] = []string{`+TARGET: ` + targetPub}
	targetTransport.responses["AT+MIGRATE=UNWRAP,1,EC_P256,"+targetPub+",cipher123,tag456,nonce789"] = []string{`OK`}
	targetTransport.responses["AT+PUBKEY=1"] = []string{
		`+PUBKEY: 1,` + slotPub,
		`OK`,
	}
	targetTransport.responses["AT+LOCK"] = []string{`OK`}

	sourceClient := domain.NewATClient(sourceTransport)
	targetClient := domain.NewATClient(targetTransport)

	var out bytes.Buffer
	svc := usecases.NewMigrationService(nil, &out)

	err := svc.ExecuteDeviceMigration(sourceClient, targetClient, false)
	if err != nil {
		t.Fatalf("ExecuteDeviceMigration failed: %v", err)
	}

	if !strings.Contains(out.String(), "SUCCESS: All keys migrated and verified successfully!") {
		t.Errorf("expected success banner, got: %s", out.String())
	}
}

func TestMigrationService_ExecuteDeviceMigration_FailFastLockdown(t *testing.T) {
	sourceTransport := newMockMigrationTransport()
	targetTransport := newMockMigrationTransport()

	targetPub := "040102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"

	// Source setup: authenticated, has slot 1
	sourceTransport.responses["AT+STATUS?"] = []string{`+STATUS: AUTH=1,RETRIES=3,KEYCOUNT=1`}
	sourceTransport.responses["AT+SLOTS?"] = []string{
		`+SLOTS: 1, EC_P256, 256, "Key1"`,
		`OK`,
	}
	sourceTransport.responses["AT+MIGRATE=WRAP,1,"+targetPub] = []string{
		`+WRAP: 1,EC_P256,` + targetPub + `,cipher123,tag456,nonce789`,
	}
	sourceTransport.responses["AT+LOCK"] = []string{`OK`}

	// Target setup: unwrap fails
	targetTransport.responses["AT+STATUS?"] = []string{`+STATUS: AUTH=1,RETRIES=3,KEYCOUNT=0`}
	targetTransport.responses["AT+MIGRATE=INIT_TARGET"] = []string{`+TARGET: ` + targetPub}
	targetTransport.responses["AT+MIGRATE=UNWRAP,1,EC_P256,"+targetPub+",cipher123,tag456,nonce789"] = []string{
		`+CME ERROR: 6 (Cryptographic accelerator failure)`,
	}
	targetTransport.responses["AT+LOCK"] = []string{`OK`}

	sourceClient := domain.NewATClient(sourceTransport)
	targetClient := domain.NewATClient(targetTransport)

	var out bytes.Buffer
	svc := usecases.NewMigrationService(nil, &out)

	err := svc.ExecuteDeviceMigration(sourceClient, targetClient, false)
	if err == nil {
		t.Fatalf("expected error from failed unwrap, got nil")
	}

	// Verify fail-fast lockdown occurred on both devices
	var sourceHadLock, targetHadLock bool
	for _, cmd := range sourceTransport.callHistory {
		if cmd == "AT+LOCK" {
			sourceHadLock = true
		}
	}
	for _, cmd := range targetTransport.callHistory {
		if cmd == "AT+LOCK" {
			targetHadLock = true
		}
	}

	if !sourceHadLock {
		t.Errorf("expected source device to receive AT+LOCK on failure")
	}
	if !targetHadLock {
		t.Errorf("expected target device to receive AT+LOCK on failure")
	}
	if !strings.Contains(out.String(), "[FAIL-FAST LOCKDOWN]") {
		t.Errorf("expected [FAIL-FAST LOCKDOWN] banner in output, got: %s", out.String())
	}
}
