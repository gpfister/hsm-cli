package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

type mockTransport struct {
	responses   map[string][]string
	errors      map[string]error
	lastCommand string
}

func newMockTransport() *mockTransport {
	return &mockTransport{
		responses: make(map[string][]string),
		errors:    make(map[string]error),
	}
}

func (m *mockTransport) SendCommand(cmd string, timeout time.Duration) ([]string, error) {
	m.lastCommand = cmd
	if err, exists := m.errors[cmd]; exists {
		return nil, err
	}
	if resp, exists := m.responses[cmd]; exists {
		return resp, nil
	}
	return nil, errors.New("unexpected command: " + cmd)
}

func (m *mockTransport) Close() error     { return nil }
func (m *mockTransport) PortName() string { return "mock" }

func TestATClient_Ping(t *testing.T) {
	mt := newMockTransport()
	mt.responses["AT"] = []string{}

	client := NewATClient(mt)
	if err := client.Ping(); err != nil {
		t.Fatalf("Ping failed: %v", err)
	}

	if mt.lastCommand != "AT" {
		t.Errorf("expected command 'AT', got %s", mt.lastCommand)
	}
}

func TestATClient_Info(t *testing.T) {
	mt := newMockTransport()
	mt.responses["AT+INFO?"] = []string{
		`+INFO: FW="1.0.0-mock",HW="PCA10059",UID="0102030405060708",BOOT="B0+MCUboot"`,
	}

	client := NewATClient(mt)
	info, err := client.Info()
	if err != nil {
		t.Fatalf("Info failed: %v", err)
	}

	if info.Firmware != "1.0.0-mock" {
		t.Errorf("expected FW '1.0.0-mock', got %s", info.Firmware)
	}
	if info.Hardware != "PCA10059" {
		t.Errorf("expected HW 'PCA10059', got %s", info.Hardware)
	}
	if info.UID != "0102030405060708" {
		t.Errorf("expected UID '0102030405060708', got %s", info.UID)
	}
	if info.Boot != "B0+MCUboot" {
		t.Errorf("expected Boot 'B0+MCUboot', got %s", info.Boot)
	}
	if info.VersionString() != "1.0.0-mock" {
		t.Errorf("expected VersionString '1.0.0-mock', got %s", info.VersionString())
	}
}

func TestATClient_Status(t *testing.T) {
	mt := newMockTransport()
	mt.responses["AT+STATUS?"] = []string{
		`+STATUS: AUTH=1,CONFIGURED=1,METHOD=SHAMIR,RETRIES=3,KEYCOUNT=4`,
	}

	client := NewATClient(mt)
	status, err := client.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	if !status.Authenticated {
		t.Errorf("expected Authenticated=true")
	}
	if !status.Configured {
		t.Errorf("expected Configured=true")
	}
	if status.Method != "SHAMIR" {
		t.Errorf("expected Method=SHAMIR, got %s", status.Method)
	}
	if status.RetriesRemaining != 3 {
		t.Errorf("expected Retries=3, got %d", status.RetriesRemaining)
	}
	if status.KeyCount != 4 {
		t.Errorf("expected KeyCount=4, got %d", status.KeyCount)
	}

	// Test unconfigured status
	mt.responses["AT+STATUS?"] = []string{
		`+STATUS: AUTH=0,CONFIGURED=0,METHOD=NONE,RETRIES=3,KEYCOUNT=0`,
	}
	status, err = client.Status()
	if err != nil {
		t.Fatalf("Status unconfigured failed: %v", err)
	}
	if status.Configured {
		t.Errorf("expected Configured=false")
	}
	if status.Method != "NONE" {
		t.Errorf("expected Method=NONE, got %s", status.Method)
	}
}

func TestATClient_InitPin(t *testing.T) {
	mt := newMockTransport()
	mt.responses["AT+INIT=PIN,Tr0ub4dor&3"] = []string{"OK"}

	client := NewATClient(mt)
	if err := client.InitPin("Tr0ub4dor&3"); err != nil {
		t.Fatalf("InitPin failed: %v", err)
	}
	if mt.lastCommand != "AT+INIT=PIN,Tr0ub4dor&3" {
		t.Errorf("expected command 'AT+INIT=PIN,Tr0ub4dor&3', got %s", mt.lastCommand)
	}

	// Empty PIN check
	if err := client.InitPin(""); err == nil {
		t.Errorf("expected error on empty PIN")
	}
}

func TestATClient_InitShamir(t *testing.T) {
	mt := newMockTransport()
	mt.responses["AT+INIT=SHAMIR"] = []string{
		`+SHARE: 1,0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20,abandon`,
		`+SHARE: 2,0202030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20,ability`,
		`+SHARE: 3,0302030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20,able`,
		`+SHARE: 4,0402030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20,about`,
		`+SHARE: 5,0502030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20,above`,
	}

	client := NewATClient(mt)
	res, err := client.InitShamir()
	if err != nil {
		t.Fatalf("InitShamir failed: %v", err)
	}

	if res.Threshold != 3 || res.Total != 5 {
		t.Errorf("unexpected threshold parameters: %d/%d", res.Threshold, res.Total)
	}
	if len(res.Shares) != 5 {
		t.Fatalf("expected 5 shares, got %d", len(res.Shares))
	}
}

func TestATClient_Auth_SuccessAndFailure(t *testing.T) {
	mt := newMockTransport()
	mt.responses["AT+AUTH=123456"] = []string{}
	mt.errors["AT+AUTH=badpin"] = ErrInvalidPIN

	client := NewATClient(mt)

	// Success
	if err := client.Auth("123456"); err != nil {
		t.Fatalf("Auth failed: %v", err)
	}

	// Failure
	err := client.Auth("badpin")
	if !errors.Is(err, ErrInvalidPIN) {
		t.Errorf("expected ErrInvalidPIN, got: %v", err)
	}

	// Empty PIN check
	if err := client.Auth(""); err == nil {
		t.Errorf("expected error on empty PIN, got nil")
	}
}

func TestATClient_LockAndReset(t *testing.T) {
	mt := newMockTransport()
	mt.responses["AT+LOCK"] = []string{}
	mt.responses["AT+RESET"] = []string{}

	client := NewATClient(mt)

	if err := client.Lock(); err != nil {
		t.Fatalf("Lock failed: %v", err)
	}
	if err := client.Reset(); err != nil {
		t.Fatalf("Reset failed: %v", err)
	}
}

func TestATClient_Slots(t *testing.T) {
	mt := newMockTransport()
	mt.responses["AT+SLOTS?"] = []string{
		`+SLOTS: 1,EC_P256,256,"KeyOne"`,
		`+SLOTS: 2,EC_P256,256,"KeyTwo"`,
	}

	client := NewATClient(mt)
	slots, err := client.Slots()
	if err != nil {
		t.Fatalf("Slots failed: %v", err)
	}

	if len(slots) != 2 {
		t.Fatalf("expected 2 slots, got %d", len(slots))
	}

	if slots[0].ID != 1 || slots[0].Algorithm != "EC_P256" || slots[0].Size != 256 || slots[0].Label != "KeyOne" {
		t.Errorf("slot 0 mismatch: %+v", slots[0])
	}
	if slots[1].ID != 2 || slots[1].Algorithm != "EC_P256" || slots[1].Size != 256 || slots[1].Label != "KeyTwo" {
		t.Errorf("slot 1 mismatch: %+v", slots[1])
	}
}

func TestATClient_KeyGen(t *testing.T) {
	mt := newMockTransport()
	mockPub := "04A1B2C3D4E5F6010203040506070809A1B2C3D4E5F601020304050607080901A1B2C3D4E5F6010203040506070809A1B2C3D4E5F601020304050607080902"
	mt.responses["AT+KEYGEN=1,EC_P256"] = []string{
		`+KEYGEN: 1,PUB="` + mockPub + `"`,
	}

	client := NewATClient(mt)
	res, err := client.KeyGen(1, "EC_P256")
	if err != nil {
		t.Fatalf("KeyGen failed: %v", err)
	}

	if res.ID != 1 {
		t.Errorf("expected slot 1, got %d", res.ID)
	}
	if res.PublicKeyHex != mockPub {
		t.Errorf("expected pubkey %s, got %s", mockPub, res.PublicKeyHex)
	}

	// Slot out of range
	if _, err := client.KeyGen(0, "EC_P256"); err == nil {
		t.Errorf("expected error on slot 0, got nil")
	}
	if _, err := client.KeyGen(17, "EC_P256"); err == nil {
		t.Errorf("expected error on slot 17, got nil")
	}
}

func TestATClient_PubKey(t *testing.T) {
	mt := newMockTransport()
	mockPub := "04A1B2C3D4E5F6010203040506070809A1B2C3D4E5F601020304050607080901A1B2C3D4E5F6010203040506070809A1B2C3D4E5F601020304050607080902"
	mt.responses["AT+PUBKEY=2"] = []string{
		`+PUBKEY: 2,PUB="` + mockPub + `"`,
	}

	client := NewATClient(mt)
	pub, err := client.PubKey(2)
	if err != nil {
		t.Fatalf("PubKey failed: %v", err)
	}

	if pub != mockPub {
		t.Errorf("expected pub %s, got %s", mockPub, pub)
	}
}

func TestATClient_Sign(t *testing.T) {
	mt := newMockTransport()
	validHash := strings.Repeat("A1", 32) // 64 hex chars
	mockSig := strings.Repeat("B2", 64)   // 128 hex chars
	mt.responses["AT+SIGN=1,"+validHash] = []string{
		`+SIGN: ` + mockSig,
	}

	client := NewATClient(mt)
	sig, err := client.Sign(1, validHash)
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}

	if sig != mockSig {
		t.Errorf("expected sig %s, got %s", mockSig, sig)
	}

	// Invalid hash length
	if _, err := client.Sign(1, "DEADBEEF"); err == nil {
		t.Errorf("expected error on short hash, got nil")
	}

	// Non-hex hash
	if _, err := client.Sign(1, strings.Repeat("ZZ", 32)); err == nil {
		t.Errorf("expected error on non-hex hash, got nil")
	}
}

func TestATClient_Random(t *testing.T) {
	mt := newMockTransport()
	mt.responses["AT+RANDOM=4"] = []string{
		`+RANDOM: 01020304`,
	}

	client := NewATClient(mt)
	bytes, err := client.Random(4)
	if err != nil {
		t.Fatalf("Random failed: %v", err)
	}

	if len(bytes) != 4 || bytes[0] != 1 || bytes[1] != 2 || bytes[2] != 3 || bytes[3] != 4 {
		t.Errorf("unexpected random bytes: %v", bytes)
	}

	// Length boundaries
	if _, err := client.Random(0); err == nil {
		t.Errorf("expected error for len 0, got nil")
	}
	if _, err := client.Random(129); err == nil {
		t.Errorf("expected error for len 129, got nil")
	}
}

func TestCMEError_ParseAndFormatting(t *testing.T) {
	err := ParseCMEError(2)
	if !errors.Is(err, ErrInvalidPIN) {
		t.Errorf("expected ErrInvalidPIN, got %v", err)
	}

	expectedStr := "+CME ERROR: 2 (Invalid PIN)"
	if err.Error() != expectedStr {
		t.Errorf("expected %q, got %q", expectedStr, err.Error())
	}

	unknownErr := ParseCMEError(99)
	if !strings.Contains(unknownErr.Error(), "+CME ERROR: 99") {
		t.Errorf("expected unknown code in error string, got %s", unknownErr.Error())
	}
}

func TestATClient_ThresholdInit(t *testing.T) {
	mt := newMockTransport()
	mt.responses["AT+INIT=THRESHOLD,3,5"] = []string{
		`+SHARE: 1,0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20,abandon`,
		`+SHARE: 2,0202030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20,ability`,
		`+SHARE: 3,0302030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20,able`,
		`+SHARE: 4,0402030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20,about`,
		`+SHARE: 5,0502030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20,above`,
	}

	client := NewATClient(mt)
	res, err := client.ThresholdInit(3, 5)
	if err != nil {
		t.Fatalf("ThresholdInit failed: %v", err)
	}

	if res.Threshold != 3 || res.Total != 5 {
		t.Errorf("unexpected threshold parameters: %d/%d", res.Threshold, res.Total)
	}
	if len(res.Shares) != 5 {
		t.Fatalf("expected 5 shares, got %d", len(res.Shares))
	}
	if res.Shares[0].Index != 1 || res.Shares[0].CommitmentWord != "abandon" {
		t.Errorf("unexpected share 1: %+v", res.Shares[0])
	}

	// Boundary validations
	if _, err := client.ThresholdInit(1, 5); err == nil {
		t.Errorf("expected error for threshold < 2")
	}
	if _, err := client.ThresholdInit(4, 3); err == nil {
		t.Errorf("expected error for threshold > total")
	}
}

func TestATClient_SubmitShare(t *testing.T) {
	mt := newMockTransport()
	mt.responses["AT+SHARE=1,aabbccddeeff"] = []string{
		`+COMMIT: 1,abandon`,
	}

	client := NewATClient(mt)
	commit, err := client.SubmitShare(1, "aabbccddeeff")
	if err != nil {
		t.Fatalf("SubmitShare failed: %v", err)
	}

	if commit.Index != 1 || commit.Word != "abandon" {
		t.Errorf("unexpected commitment: %+v", commit)
	}

	// Validation
	if _, err := client.SubmitShare(0, "hex"); err == nil {
		t.Errorf("expected error on invalid index")
	}
	if _, err := client.SubmitShare(1, ""); err == nil {
		t.Errorf("expected error on empty share")
	}
}

func TestATClient_TargetInit(t *testing.T) {
	mt := newMockTransport()
	mt.responses["AT+MIGRATE=INIT_TARGET"] = []string{
		`+TARGET: 040102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20`,
	}

	client := NewATClient(mt)
	pub, err := client.TargetInit()
	if err != nil {
		t.Fatalf("TargetInit failed: %v", err)
	}

	if pub != "040102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20" {
		t.Errorf("unexpected target pubkey: %s", pub)
	}
}

func TestATClient_MigrateWrapAndUnwrap(t *testing.T) {
	mt := newMockTransport()
	targetPub := "040102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
	mt.responses["AT+MIGRATE=WRAP,1,"+targetPub] = []string{
		`+WRAP: 1,EC_P256,` + targetPub + `,cipher123,tag456,nonce789`,
	}

	client := NewATClient(mt)
	env, err := client.MigrateWrap(1, targetPub)
	if err != nil {
		t.Fatalf("MigrateWrap failed: %v", err)
	}

	if env.SlotID != 1 || env.Algorithm != "EC_P256" || env.EncryptedKey != "cipher123" {
		t.Errorf("unexpected envelope: %+v", env)
	}

	// Test Unwrap
	mt.responses["AT+MIGRATE=UNWRAP,1,EC_P256,"+targetPub+",cipher123,tag456,nonce789"] = []string{}
	if err := client.MigrateUnwrap(env); err != nil {
		t.Fatalf("MigrateUnwrap failed: %v", err)
	}
}

func TestATClient_DFUInfo(t *testing.T) {
	mt := newMockTransport()
	shaHex := strings.Repeat("AA", 32)
	mt.responses["AT+DFU_INFO?"] = []string{
		`+DFU_INFO: VER="1.2.3",SLOT=0,SIZE=262144,SHA256="` + shaHex + `"`,
	}

	client := NewATClient(mt)
	info, err := client.DFUInfo()
	if err != nil {
		t.Fatalf("DFUInfo failed: %v", err)
	}

	if info.Major != 1 || info.Minor != 2 || info.Patch != 3 {
		t.Errorf("expected version 1.2.3, got %d.%d.%d", info.Major, info.Minor, info.Patch)
	}
	if info.ActiveSlot != 0 {
		t.Errorf("expected ActiveSlot=0, got %d", info.ActiveSlot)
	}
	if info.ImageSize != 262144 {
		t.Errorf("expected ImageSize=262144, got %d", info.ImageSize)
	}
	if len(info.ImageSHA256) != 32 || info.ImageSHA256[0] != 0xAA {
		t.Errorf("unexpected ImageSHA256: %x", info.ImageSHA256)
	}
}

func TestATClient_DFUInfo_FallbackOnUnsupported(t *testing.T) {
	mt := newMockTransport()
	mt.errors["AT+DFU_INFO?"] = ErrCommandUnsupported
	mt.responses["AT+INFO?"] = []string{
		`+INFO: FW="2.1.4-mock",HW="PCA10059",UID="0102030405060708",BOOT="B0+MCUboot"`,
	}

	client := NewATClient(mt)
	info, err := client.DFUInfo()
	if err != nil {
		t.Fatalf("DFUInfo fallback failed: %v", err)
	}

	if info.Major != 2 || info.Minor != 1 || info.Patch != 4 {
		t.Errorf("expected fallback version 2.1.4, got %d.%d.%d", info.Major, info.Minor, info.Patch)
	}
	if info.ActiveSlot != 0 || info.ImageSize != 0 {
		t.Errorf("expected slot 0 and size 0 on fallback, got slot=%d size=%d", info.ActiveSlot, info.ImageSize)
	}
}

func TestATClient_DFUChunk(t *testing.T) {
	mt := newMockTransport()
	chunk := []byte{0xde, 0xad, 0xbe, 0xef}
	mt.responses["AT+DFU_CHUNK=0,DEADBEEF"] = []string{}

	client := NewATClient(mt)
	if err := client.DFUChunk(0, chunk); err != nil {
		t.Fatalf("DFUChunk failed: %v", err)
	}

	if mt.lastCommand != "AT+DFU_CHUNK=0,DEADBEEF" {
		t.Errorf("expected command 'AT+DFU_CHUNK=0,DEADBEEF', got %q", mt.lastCommand)
	}

	// Empty chunk rejection
	if err := client.DFUChunk(0, nil); err == nil {
		t.Errorf("expected error for empty chunk, got nil")
	}

	// Oversized (>256B) chunk rejection
	oversized := make([]byte, 257)
	if err := client.DFUChunk(0, oversized); err == nil {
		t.Errorf("expected error for 257-byte chunk, got nil")
	}

	// Device locked error propagation
	mt.errors["AT+DFU_CHUNK=256,DEADBEEF"] = ErrDeviceLocked
	if err := client.DFUChunk(256, chunk); !errors.Is(err, ErrDeviceLocked) {
		t.Errorf("expected ErrDeviceLocked, got %v", err)
	}
}

func TestATClient_DFUApply(t *testing.T) {
	mt := newMockTransport()
	mt.responses["AT+DFU_APPLY"] = []string{}

	client := NewATClient(mt)
	if err := client.DFUApply(); err != nil {
		t.Fatalf("DFUApply failed: %v", err)
	}
	if mt.lastCommand != "AT+DFU_APPLY" {
		t.Errorf("expected command 'AT+DFU_APPLY', got %q", mt.lastCommand)
	}

	// Error propagation
	mt.errors["AT+DFU_APPLY"] = ErrDeviceLocked
	if err := client.DFUApply(); !errors.Is(err, ErrDeviceLocked) {
		t.Errorf("expected ErrDeviceLocked, got %v", err)
	}
}
