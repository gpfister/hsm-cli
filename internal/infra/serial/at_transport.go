package serial

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gpfister/nrf52840-hsm-cli/internal/domain"
	"go.bug.st/serial"
)

// ATTransport abstracts the serial command/response transport for the AT ASCII protocol.
type ATTransport interface {
	SendCommand(cmd string, timeout time.Duration) ([]string, error)
	Close() error
	PortName() string
}

// SerialATTransport implements ATTransport over a physical USB CDC-ACM or virtual PTY serial port.
type SerialATTransport struct {
	port serial.Port
	path string
	mu   sync.Mutex
}

// FindDonglePorts scans for available dongle serial ports, prioritizing AT data channels (suffix 3).
func FindDonglePorts() ([]string, error) {
	return FindDonglePortsTargeting("")
}

// AutoDiscoverPort locates and selects the HSM AT command serial port.
// Prioritizes suffix 3 (AT data channel) over suffix 1 (Console logging).
func AutoDiscoverPort() (string, error) {
	return AutoDiscoverPortTargeting("")
}

// OpenAT opens a serial connection to the specified port path at 115200 8N1.
func OpenAT(portPath string) (*SerialATTransport, error) {
	mode := &serial.Mode{
		BaudRate: 115200,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}

	p, err := serial.Open(portPath, mode)
	if err != nil {
		return nil, fmt.Errorf("failed to open serial port %s: %w", portPath, err)
	}

	return &SerialATTransport{
		port: p,
		path: portPath,
	}, nil
}

// Close closes the underlying serial connection.
func (t *SerialATTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.port != nil {
		err := t.port.Close()
		t.port = nil
		return err
	}
	return nil
}

// PortName returns the configured serial port path.
func (t *SerialATTransport) PortName() string {
	return t.path
}

// SendCommand writes an AT command and reads lines until OK or +CME ERROR: <code> is received.
func (t *SerialATTransport) SendCommand(cmd string, timeout time.Duration) ([]string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.port == nil {
		return nil, errors.New("serial transport is closed")
	}

	// Drain any residual bytes in the input queue
	_ = t.port.ResetInputBuffer()
	_ = t.port.SetReadTimeout(10 * time.Millisecond)
	drainBuf := make([]byte, 256)
	for {
		n, _ := t.port.Read(drainBuf)
		if n <= 0 {
			break
		}
	}

	// Format command with trailing \r\n
	formatted := strings.TrimRight(cmd, "\r\n") + "\r\n"
	if _, err := t.port.Write([]byte(formatted)); err != nil {
		return nil, fmt.Errorf("failed to write command to serial port: %w", err)
	}

	// Line-buffered reader accumulating response lines until terminal delimiter
	deadline := time.Now().Add(timeout)
	readChunk := make([]byte, 256)
	var rawAccum []byte
	var lines []string

	_ = t.port.SetReadTimeout(50 * time.Millisecond)

	for {
		if time.Now().After(deadline) {
			return lines, domain.ErrDeviceTimeout
		}

		n, err := t.port.Read(readChunk)
		if err != nil {
			return lines, fmt.Errorf("read error: %w", err)
		}

		if n > 0 {
			rawAccum = append(rawAccum, readChunk[:n]...)

			// Process complete lines separated by \n
			for {
				idx := bytes.IndexByte(rawAccum, '\n')
				if idx < 0 {
					break
				}

				lineBytes := rawAccum[:idx]
				rawAccum = rawAccum[idx+1:]

				line := strings.TrimRight(string(lineBytes), "\r\n")
				line = strings.TrimSpace(line)

				if line == "" {
					continue
				}

				// Check terminal status conditions
				if line == "OK" {
					return lines, nil
				}

				if strings.HasPrefix(line, "+CME ERROR:") {
					codeStr := strings.TrimSpace(strings.TrimPrefix(line, "+CME ERROR:"))
					code, _ := strconv.Atoi(codeStr)
					return lines, domain.ParseCMEError(code)
				}

				if line == "ERROR" {
					return lines, errors.New("device returned ERROR")
				}

				lines = append(lines, line)
			}
		}
	}
}
