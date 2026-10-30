package protocol

import (
	"encoding/binary"
	"fmt"
	"github.com/gpfister/nrf52840-hsm-cli/internal/domain"
)

const HeaderSize = 3

// EncodePacket prepares a YubiHSM 2 frame: [1 byte cmd] [2 bytes length BE] [payload]
func EncodePacket(cmd byte, payload []byte) []byte {
	length := uint16(len(payload))
	pkt := make([]byte, HeaderSize+len(payload))
	pkt[0] = cmd
	binary.BigEndian.PutUint16(pkt[1:3], length)
	if len(payload) > 0 {
		copy(pkt[HeaderSize:], payload)
	}
	return pkt
}

// DecodePacket decodes a YubiHSM 2 frame
func DecodePacket(data []byte) (byte, []byte, error) {
	if len(data) < HeaderSize {
		return 0, nil, domain.ErrInvalidPacket
	}

	cmd := data[0]
	length := binary.BigEndian.Uint16(data[1:3])

	if len(data) < int(HeaderSize+length) {
		return 0, nil, domain.ErrInvalidPacket
	}

	payload := data[HeaderSize : HeaderSize+length]

	if cmd == domain.RspError {
		errCode := byte(0xFF)
		if len(payload) > 0 {
			errCode = payload[0]
		}
		return cmd, payload, fmt.Errorf("%w: code 0x%02X", domain.ErrDeviceError, errCode)
	}

	return cmd, payload, nil
}
