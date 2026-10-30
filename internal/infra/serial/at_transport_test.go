package serial

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/gpfister/nrf52840-hsm-cli/internal/domain"
	"go.bug.st/serial"
)

type mockPort struct {
	readBuf     *bytes.Buffer
	writeBuf    *bytes.Buffer
	replyOnCmd  string
	closed      bool
}

func newMockPort(reply string) *mockPort {
	return &mockPort{
		readBuf:    &bytes.Buffer{},
		writeBuf:   &bytes.Buffer{},
		replyOnCmd: reply,
	}
}

func (m *mockPort) Read(p []byte) (int, error) {
	if m.readBuf.Len() == 0 {
		return 0, nil
	}
	return m.readBuf.Read(p)
}

func (m *mockPort) Write(p []byte) (int, error) {
	n, err := m.writeBuf.Write(p)
	if m.replyOnCmd != "" {
		m.readBuf.WriteString(m.replyOnCmd)
		m.replyOnCmd = ""
	}
	return n, err
}

func (m *mockPort) Close() error {
	m.closed = true
	return nil
}

func (m *mockPort) ResetInputBuffer() error                              { return nil }
func (m *mockPort) ResetOutputBuffer() error                             { return nil }
func (m *mockPort) SetMode(mode *serial.Mode) error                      { return nil }
func (m *mockPort) Drain() error                                         { return nil }
func (m *mockPort) SetDTR(dtr bool) error                                { return nil }
func (m *mockPort) SetRTS(rts bool) error                                { return nil }
func (m *mockPort) GetModemStatusBits() (*serial.ModemStatusBits, error) { return nil, nil }
func (m *mockPort) SetReadTimeout(t time.Duration) error                 { return nil }
func (m *mockPort) Break(d time.Duration) error                          { return nil }

func TestTransport_SendCommand_Success(t *testing.T) {
	mock := newMockPort("\r\n+INFO: FW=\"1.0.0-mock\",HW=\"PCA10059\"\r\nOK\r\n")
	tr := &SerialATTransport{
		port: mock,
		path: "test-port",
	}

	lines, err := tr.SendCommand("AT+INFO?", 500*time.Millisecond)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	expectedLine := "+INFO: FW=\"1.0.0-mock\",HW=\"PCA10059\""
	if lines[0] != expectedLine {
		t.Errorf("expected line %q, got %q", expectedLine, lines[0])
	}

	if mock.writeBuf.String() != "AT+INFO?\r\n" {
		t.Errorf("expected written cmd %q, got %q", "AT+INFO?\r\n", mock.writeBuf.String())
	}
}

func TestTransport_SendCommand_CMEErrors(t *testing.T) {
	testCases := []struct {
		name        string
		deviceReply string
		expectedErr error
	}{
		{"Locked", "+CME ERROR: 1\r\n", domain.ErrDeviceLocked},
		{"InvalidPIN", "+CME ERROR: 2\r\n", domain.ErrInvalidPIN},
		{"PINBlocked", "+CME ERROR: 3\r\n", domain.ErrPINBlocked},
		{"KeyNotFound", "+CME ERROR: 4\r\n", domain.ErrKeyNotFound},
		{"InvalidParam", "+CME ERROR: 5\r\n", domain.ErrInvalidParameter},
		{"CryptoFailure", "+CME ERROR: 6\r\n", domain.ErrCryptoFailure},
		{"BufferOverflow", "+CME ERROR: 7\r\n", domain.ErrBufferOverflow},
		{"Unsupported", "+CME ERROR: 8\r\n", domain.ErrCommandUnsupported},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mock := newMockPort(tc.deviceReply)
			tr := &SerialATTransport{
				port: mock,
				path: "test-port",
			}

			_, err := tr.SendCommand("AT+TEST", 500*time.Millisecond)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}

			if !errors.Is(err, tc.expectedErr) {
				t.Errorf("expected error %v, got %v", tc.expectedErr, err)
			}
		})
	}
}

func TestTransport_AutoDiscoverPort_EnvOverride(t *testing.T) {
	customPort := "/tmp/mock-virtual-pty-test"
	t.Setenv("NRF52840_HSM_PORT", customPort)

	port, err := AutoDiscoverPort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if port != customPort {
		t.Errorf("expected port %q, got %q", customPort, port)
	}

	ports, err := FindDonglePorts()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ports) != 1 || ports[0] != customPort {
		t.Errorf("expected [%q], got %v", customPort, ports)
	}
}

func TestTransport_Close(t *testing.T) {
	mock := newMockPort("")
	tr := &SerialATTransport{
		port: mock,
		path: "test-port",
	}

	if err := tr.Close(); err != nil {
		t.Fatalf("failed to close: %v", err)
	}

	if !mock.closed {
		t.Errorf("expected mock port to be closed")
	}

	if tr.PortName() != "test-port" {
		t.Errorf("expected PortName 'test-port', got %s", tr.PortName())
	}
}
