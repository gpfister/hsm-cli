package serial

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gpfister/nrf52840-hsm-cli/internal/domain"
)

func TestPortPriority(t *testing.T) {
	ttyData := "/dev/tty.usbmodemF4CE368F12AB3"
	cuData := "/dev/cu.usbmodemF4CE368F12AB3"
	ttyConsole := "/dev/tty.usbmodemF4CE368F12AB1"
	cuConsole := "/dev/cu.usbmodemF4CE368F12AB1"

	scoreTTYData := PortPriority(ttyData)
	scoreCUData := PortPriority(cuData)
	scoreTTYConsole := PortPriority(ttyConsole)
	scoreCUConsole := PortPriority(cuConsole)

	if scoreTTYData <= scoreTTYConsole {
		t.Errorf("expected suffix 3 priority (%d) > suffix 1 priority (%d)", scoreTTYData, scoreTTYConsole)
	}

	if scoreTTYData <= scoreCUData {
		t.Errorf("expected /dev/tty priority (%d) > /dev/cu priority (%d)", scoreTTYData, scoreCUData)
	}

	if scoreCUData <= scoreTTYConsole {
		t.Errorf("expected cuData (%d) > ttyConsole (%d)", scoreCUData, scoreTTYConsole)
	}

	if scoreTTYConsole <= scoreCUConsole {
		t.Errorf("expected ttyConsole (%d) > cuConsole (%d)", scoreTTYConsole, scoreCUConsole)
	}
}

func TestSortPortsByPriority(t *testing.T) {
	ports := []string{
		"/dev/cu.usbmodem31424101",
		"/dev/cu.usbmodem31424103",
		"/dev/tty.usbmodem31424101",
		"/dev/tty.usbmodem31424103",
	}

	SortPortsByPriority(ports)

	if ports[0] != "/dev/tty.usbmodem31424103" {
		t.Errorf("expected first port to be /dev/tty.usbmodem31424103, got %s", ports[0])
	}

	if ports[1] != "/dev/cu.usbmodem31424103" {
		t.Errorf("expected second port to be /dev/cu.usbmodem31424103, got %s", ports[1])
	}

	// Verify suffix 1 ports are ranked lower
	if !strings.HasSuffix(ports[2], "1") || !strings.HasSuffix(ports[3], "1") {
		t.Errorf("expected last two ports to have suffix 1, got %v", ports[2:])
	}
}

func TestNormalizeSerial(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"F4:CE:36:8F:12:AB", "F4CE368F12AB"},
		{"f4-ce-36-8f-12-ab", "F4CE368F12AB"},
		{"  f4ce368f12ab  ", "F4CE368F12AB"},
		{"FD54E793A75540C9", "FD54E793A75540C9"},
		{"12:34:56:78:90:ab", "1234567890AB"},
	}

	for _, tc := range cases {
		got := NormalizeSerial(tc.input)
		if got != tc.expected {
			t.Errorf("NormalizeSerial(%q) = %q, expected %q", tc.input, got, tc.expected)
		}
	}
}

func TestExtractBLEAddressFromPort(t *testing.T) {
	cases := []struct {
		path     string
		expected string
	}{
		{"/dev/tty.usbmodemF4CE368F12AB3", "F4CE368F12AB"},
		{"/dev/tty.usbmodemF4CE368F12AB1", "F4CE368F12AB"},
		{"/dev/cu.usbmodemF4CE368F12AB3", "F4CE368F12AB"},
		{"/dev/cu.usbmodemF4CE368F12AB1", "F4CE368F12AB"},
		{"/dev/tty.usbmodemFD54E793A75540C93", "FD54E793A75540C9"},
		{"/dev/tty.usbmodemFD54E793A75540C91", "FD54E793A75540C9"},
		{"/dev/tty.hsmF4CE368F12AB0", "F4CE368F12AB"},
		{"/dev/tty.hsmF4CE368F12AB1", "F4CE368F12AB"},
		{"/dev/tty.hsmFD54E793A75540C90", "FD54E793A75540C9"},
		{"/dev/tty.usbmodem31424103", ""}, // Location ID, not 12/16 hex chars
		{"/dev/ttyUSB0", ""},
		{"COM3", ""},
	}

	for _, tc := range cases {
		got := ExtractBLEAddressFromPort(tc.path)
		if got != tc.expected {
			t.Errorf("ExtractBLEAddressFromPort(%q) = %q, expected %q", tc.path, got, tc.expected)
		}
	}
}

func TestStripTrailingDigit(t *testing.T) {
	p1 := "/dev/tty.usbmodemF4CE368F12AB1"
	p3 := "/dev/tty.usbmodemF4CE368F12AB3"

	if stripTrailingDigit(p1) != stripTrailingDigit(p3) {
		t.Errorf("expected both ports to reduce to same base: %q vs %q", stripTrailingDigit(p1), stripTrailingDigit(p3))
	}
	expected := "/dev/tty.usbmodemF4CE368F12AB"
	if stripTrailingDigit(p1) != expected {
		t.Errorf("expected %q, got %q", expected, stripTrailingDigit(p1))
	}
}

func TestAutoDiscoverPortTargeting_NotFound(t *testing.T) {
	_, err := AutoDiscoverPortTargeting("NONEXISTENT_SERIAL_999999")
	if err == nil {
		t.Fatalf("expected error for non-existent serial, got nil")
	}
	if !strings.Contains(err.Error(), "no nRF52840 HSM Dongle found matching") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestEnumerateDongles_Format(t *testing.T) {
	dongles, err := EnumerateDongles()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// If physical dongles are present in this test environment, verify their structure
	for _, d := range dongles {
		if d.DataPort == "" {
			t.Errorf("dongle missing DataPort: %+v", d)
		}
		if d.Serial == "" {
			t.Errorf("dongle missing Serial: %+v", d)
		}
	}
}

func TestAutoDiscover_SingleDevice(t *testing.T) {
	orig := enumerateDonglesFn
	defer func() { enumerateDonglesFn = orig }()

	t.Setenv("NRF52840_HSM_PORT", "")
	t.Setenv("NRF52840_HSM_SERIAL", "")

	expectedPort := "/dev/tty.usbmodemF4CE368F12AB3"
	enumerateDonglesFn = func() ([]domain.DiscoveredDongle, error) {
		return []domain.DiscoveredDongle{
			{
				Serial:   "F4CE368F12AB",
				DataPort: expectedPort,
				Product:  "nRF52840 HSM Dongle",
			},
		}, nil
	}

	port, err := AutoDiscoverPortTargeting("")
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if port != expectedPort {
		t.Errorf("expected port %q, got %q", expectedPort, port)
	}
}

func TestAutoDiscover_MultipleDevices(t *testing.T) {
	orig := enumerateDonglesFn
	defer func() { enumerateDonglesFn = orig }()

	t.Setenv("NRF52840_HSM_PORT", "")
	t.Setenv("NRF52840_HSM_SERIAL", "")

	dongles := []domain.DiscoveredDongle{
		{
			Serial:   "F4CE368F12AB",
			DataPort: "/dev/tty.usbmodemF4CE368F12AB3",
			Product:  "nRF52840 HSM Dongle",
		},
		{
			Serial:   "FD54E793A75540C9",
			DataPort: "/dev/tty.usbmodemFD54E793A75540C93",
			Product:  "nRF52840 HSM Dongle",
		},
	}

	enumerateDonglesFn = func() ([]domain.DiscoveredDongle, error) {
		return dongles, nil
	}

	port, err := AutoDiscoverPortTargeting("")
	if err == nil {
		t.Fatalf("expected ErrMultipleDevicesDetected, got port: %q", port)
	}

	var multErr *domain.ErrMultipleDevicesDetected
	if !errors.As(err, &multErr) {
		t.Fatalf("expected error of type *domain.ErrMultipleDevicesDetected, got %T: %v", err, err)
	}

	if len(multErr.Devices) != 2 {
		t.Errorf("expected 2 devices in error, got %d", len(multErr.Devices))
	}

	errMsg := multErr.Error()
	if !strings.Contains(errMsg, "multiple HSM USB devices detected") {
		t.Errorf("error message missing header: %s", errMsg)
	}
	if !strings.Contains(errMsg, "F4CE368F12AB") || !strings.Contains(errMsg, "FD54E793A75540C9") {
		t.Errorf("error message missing device serials: %s", errMsg)
	}
	if !strings.Contains(errMsg, "--serial (-s)") || !strings.Contains(errMsg, "--port (-p)") {
		t.Errorf("error message missing disambiguation instructions: %s", errMsg)
	}
}

func TestAutoDiscover_TargetSerial_Disambiguation(t *testing.T) {
	orig := enumerateDonglesFn
	defer func() { enumerateDonglesFn = orig }()

	t.Setenv("NRF52840_HSM_PORT", "")
	t.Setenv("NRF52840_HSM_SERIAL", "")

	port1 := "/dev/tty.usbmodemF4CE368F12AB3"
	port2 := "/dev/tty.usbmodemFD54E793A75540C93"

	enumerateDonglesFn = func() ([]domain.DiscoveredDongle, error) {
		return []domain.DiscoveredDongle{
			{
				Serial:   "F4CE368F12AB",
				DataPort: port1,
				Product:  "nRF52840 HSM Dongle",
			},
			{
				Serial:   "FD54E793A75540C9",
				DataPort: port2,
				Product:  "nRF52840 HSM Dongle",
			},
		}, nil
	}

	// Exact match
	p, err := AutoDiscoverPortTargeting("F4CE368F12AB")
	if err != nil || p != port1 {
		t.Errorf("target F4CE368F12AB: got (%q, %v), expected (%q, nil)", p, err, port1)
	}

	// Colon-separated BLE address
	p, err = AutoDiscoverPortTargeting("F4:CE:36:8F:12:AB")
	if err != nil || p != port1 {
		t.Errorf("target F4:CE:36:8F:12:AB: got (%q, %v), expected (%q, nil)", p, err, port1)
	}

	// Prefix match
	p, err = AutoDiscoverPortTargeting("FD54")
	if err != nil || p != port2 {
		t.Errorf("target FD54: got (%q, %v), expected (%q, nil)", p, err, port2)
	}

	// Non-matching serial
	_, err = AutoDiscoverPortTargeting("999999999999")
	if err == nil {
		t.Errorf("expected error for non-matching serial")
	}
	if !errors.Is(err, domain.ErrDeviceNotFound) {
		t.Errorf("expected ErrDeviceNotFound, got %v", err)
	}
}

func TestAutoDiscover_AmbiguousSerial(t *testing.T) {
	orig := enumerateDonglesFn
	defer func() { enumerateDonglesFn = orig }()

	t.Setenv("NRF52840_HSM_PORT", "")
	t.Setenv("NRF52840_HSM_SERIAL", "")

	enumerateDonglesFn = func() ([]domain.DiscoveredDongle, error) {
		return []domain.DiscoveredDongle{
			{
				Serial:   "F4CE11111111",
				DataPort: "/dev/tty.usbmodemF4CE111111113",
				Product:  "nRF52840 HSM Dongle",
			},
			{
				Serial:   "F4CE22222222",
				DataPort: "/dev/tty.usbmodemF4CE222222223",
				Product:  "nRF52840 HSM Dongle",
			},
		}, nil
	}

	// Ambiguous prefix matches both devices
	_, err := AutoDiscoverPortTargeting("F4CE")
	if err == nil {
		t.Fatalf("expected error for ambiguous serial, got nil")
	}

	var multErr *domain.ErrMultipleDevicesDetected
	if !errors.As(err, &multErr) {
		t.Fatalf("expected *domain.ErrMultipleDevicesDetected, got %T: %v", err, err)
	}
	if len(multErr.Devices) != 2 {
		t.Errorf("expected 2 devices in error, got %d", len(multErr.Devices))
	}
}

func TestAutoDiscover_NoDevices(t *testing.T) {
	orig := enumerateDonglesFn
	defer func() { enumerateDonglesFn = orig }()

	t.Setenv("NRF52840_HSM_PORT", "")
	t.Setenv("NRF52840_HSM_SERIAL", "")

	enumerateDonglesFn = func() ([]domain.DiscoveredDongle, error) {
		return []domain.DiscoveredDongle{}, nil
	}

	_, err := AutoDiscoverPortTargeting("")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, domain.ErrDeviceNotFound) {
		t.Errorf("expected ErrDeviceNotFound, got %v", err)
	}
}

func TestResolvePortOrSerial(t *testing.T) {
	orig := enumerateDonglesFn
	defer func() { enumerateDonglesFn = orig }()

	t.Setenv("NRF52840_HSM_PORT", "")
	t.Setenv("NRF52840_HSM_SERIAL", "")

	enumerateDonglesFn = func() ([]domain.DiscoveredDongle, error) {
		return []domain.DiscoveredDongle{
			{
				Serial:   "F4CE368F12AB",
				DataPort: "/dev/tty.usbmodemF4CE368F12AB3",
				Product:  "nRF52840 HSM Dongle",
			},
		}, nil
	}

	// 1. /dev/ path
	p, err := ResolvePortOrSerial("/dev/tty.usbmodemCustomPort3")
	if err != nil || p != "/dev/tty.usbmodemCustomPort3" {
		t.Errorf("ResolvePortOrSerial(/dev/...) = (%q, %v)", p, err)
	}

	// 2. COM port
	p, err = ResolvePortOrSerial("COM4")
	if err != nil || p != "COM4" {
		t.Errorf("ResolvePortOrSerial(COM4) = (%q, %v)", p, err)
	}

	// 3. Existing file path
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "mock-pty")
	if err := os.WriteFile(tmpFile, []byte("pty"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	p, err = ResolvePortOrSerial(tmpFile)
	if err != nil || p != tmpFile {
		t.Errorf("ResolvePortOrSerial(tmpFile) = (%q, %v)", p, err)
	}

	// 4. Serial / BLE address
	p, err = ResolvePortOrSerial("F4CE368F12AB")
	if err != nil || p != "/dev/tty.usbmodemF4CE368F12AB3" {
		t.Errorf("ResolvePortOrSerial(serial) = (%q, %v)", p, err)
	}
}
