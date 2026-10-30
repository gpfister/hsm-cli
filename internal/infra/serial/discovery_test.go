package serial

import (
	"strings"
	"testing"
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
