package serial

import (
	"encoding/binary"
	"fmt"
	"strings"
	"time"

	"github.com/gpfister/nrf52840-hms-cli/internal/domain"
	"github.com/gpfister/nrf52840-hms-cli/internal/infra/protocol"
	"go.bug.st/serial"
)

type Transport struct {
	port serial.Port
	path string
}

// FindDonglePorts finds potential serial ports for the dongle
func FindDonglePorts() ([]string, error) {
	list, err := serial.GetPortsList()
	if err != nil {
		return nil, err
	}

	var matches []string
	for _, name := range list {
		lower := strings.ToLower(name)
		if strings.Contains(lower, "usbmodem") || strings.Contains(lower, "ttyacm") || strings.Contains(lower, "com") {
			matches = append(matches, name)
		}
	}

	if len(matches) == 0 {
		return list, nil
	}

	return matches, nil
}

func Open(portName string) (*Transport, error) {
	mode := &serial.Mode{
		BaudRate: 115200,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}

	p, err := serial.Open(portName, mode)
	if err != nil {
		return nil, fmt.Errorf("failed to open port %s: %w", portName, err)
	}

	return &Transport{
		port: p,
		path: portName,
	}, nil
}

func (t *Transport) Close() error {
	if t.port != nil {
		return t.port.Close()
	}
	return nil
}

func (t *Transport) PortName() string {
	return t.path
}

// SendAndReceive transmits a packet and waits for the matching response
func (t *Transport) SendAndReceive(cmd byte, payload []byte, timeout time.Duration) (byte, []byte, error) {
	if t.port == nil {
		return 0, nil, fmt.Errorf("transport closed")
	}

	req := protocol.EncodePacket(cmd, payload)
	if _, err := t.port.Write(req); err != nil {
		return 0, nil, fmt.Errorf("failed to write to device: %w", err)
	}

	// Read response header (3 bytes)
	deadline := time.Now().Add(timeout)
	hdr := make([]byte, protocol.HeaderSize)
	hdrRead := 0

	for hdrRead < protocol.HeaderSize {
		if time.Now().After(deadline) {
			return 0, nil, domain.ErrDeviceTimeout
		}
		buf := make([]byte, protocol.HeaderSize-hdrRead)
		_ = t.port.SetReadTimeout(200 * time.Millisecond)
		n, err := t.port.Read(buf)
		if err != nil {
			return 0, nil, fmt.Errorf("read header error: %w", err)
		}
		if n > 0 {
			copy(hdr[hdrRead:], buf[:n])
			hdrRead += n
		}
	}

	respLen := binary.BigEndian.Uint16(hdr[1:3])

	// Read payload
	payloadBuf := make([]byte, respLen)
	payloadRead := 0

	for payloadRead < int(respLen) {
		if time.Now().After(deadline) {
			return 0, nil, domain.ErrDeviceTimeout
		}
		buf := make([]byte, int(respLen)-payloadRead)
		_ = t.port.SetReadTimeout(200 * time.Millisecond)
		n, err := t.port.Read(buf)
		if err != nil {
			return 0, nil, fmt.Errorf("read payload error: %w", err)
		}
		if n > 0 {
			copy(payloadBuf[payloadRead:], buf[:n])
			payloadRead += n
		}
	}

	fullPkt := append(hdr, payloadBuf...)
	return protocol.DecodePacket(fullPkt)
}
