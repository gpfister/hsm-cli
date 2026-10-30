package usecases

import (
	"strings"
	"testing"

	"github.com/gpfister/nrf52840-hsm-cli/internal/infra/p11"
)

func TestP11Diag_MechanismName(t *testing.T) {
	testCases := []struct {
		code     uint64
		expected string
	}{
		{p11.CKM_EC_KEY_PAIR_GEN, "CKM_EC_KEY_PAIR_GEN"},
		{p11.CKM_ECDSA, "CKM_ECDSA"},
		{p11.CKM_ECDSA_SHA256, "CKM_ECDSA_SHA256"},
		{p11.CKM_SHA256, "CKM_SHA256"},
		{p11.CKM_RSA_PKCS, "CKM_RSA_PKCS"},
		{0x99999999, "0x99999999"},
	}

	for _, tc := range testCases {
		res := p11.MechanismName(tc.code)
		if res != tc.expected {
			t.Errorf("for code 0x%X expected %q, got %q", tc.code, tc.expected, res)
		}
	}
}

func TestP11Diag_RVError(t *testing.T) {
	if err := p11.RVError("test", p11.CKR_OK); err != nil {
		t.Errorf("expected nil error for CKR_OK, got: %v", err)
	}

	err := p11.RVError("C_Login", p11.CKR_PIN_INCORRECT)
	if err == nil || !strings.Contains(err.Error(), "CKR_PIN_INCORRECT") {
		t.Errorf("expected CKR_PIN_INCORRECT error, got: %v", err)
	}

	errLocked := p11.RVError("C_Login", p11.CKR_PIN_LOCKED)
	if errLocked == nil || !strings.Contains(errLocked.Error(), "CKR_PIN_LOCKED") {
		t.Errorf("expected CKR_PIN_LOCKED error, got: %v", errLocked)
	}
}

func TestP11Diag_NonExistentModule(t *testing.T) {
	diag := NewP11DiagService()

	_, err := diag.ModuleInfo("/path/to/non/existent/module.dylib")
	if err == nil {
		t.Errorf("expected error for non-existent module, got nil")
	}

	_, err = diag.SlotList("/path/to/non/existent/module.dylib")
	if err == nil {
		t.Errorf("expected error for non-existent module, got nil")
	}

	steps, err := diag.RunDiagnostics("/path/to/non/existent/module.dylib", "123456")
	if err == nil {
		t.Errorf("expected error for non-existent module diagnostics, got nil")
	}
	if len(steps) != 1 || steps[0].Passed {
		t.Errorf("expected 1 failed step, got %v", steps)
	}
}
