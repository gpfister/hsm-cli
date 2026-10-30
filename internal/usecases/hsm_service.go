package usecases

import (
	"encoding/binary"
	"fmt"
	"os"
	"time"

	"github.com/gpfister/nrf52840-hsm-cli/internal/domain"
	"github.com/gpfister/nrf52840-hsm-cli/internal/infra/serial"
)

type HSMService struct {
	transport *serial.Transport
	atClient  *domain.ATClient
}

func NewHSMService(t *serial.Transport) *HSMService {
	return &HSMService{transport: t}
}

func NewHSMServiceWithAT(at *domain.ATClient) *HSMService {
	return &HSMService{atClient: at}
}

func (s *HSMService) ATClient() *domain.ATClient {
	return s.atClient
}

func (s *HSMService) Echo(msg string) (string, time.Duration, error) {
	if s.atClient != nil {
		start := time.Now()
		err := s.atClient.Ping()
		return "OK", time.Since(start), err
	}

	start := time.Now()
	respCmd, payload, err := s.transport.SendAndReceive(domain.CmdEcho, []byte(msg), 3*time.Second)
	latency := time.Since(start)

	if err != nil {
		return "", latency, err
	}

	expectedCmd := domain.CmdEcho | domain.RspSuccessBit
	if respCmd != expectedCmd {
		return "", latency, fmt.Errorf("unexpected response opcode: 0x%02X", respCmd)
	}

	return string(payload), latency, nil
}

func (s *HSMService) GetDeviceInfo() (*domain.DeviceInfo, error) {
	if s.atClient != nil {
		return s.atClient.Info()
	}

	respCmd, payload, err := s.transport.SendAndReceive(domain.CmdDeviceInfo, nil, 3*time.Second)
	if err != nil {
		return nil, err
	}

	if respCmd != (domain.CmdDeviceInfo | domain.RspSuccessBit) {
		return nil, fmt.Errorf("unexpected response opcode 0x%02X", respCmd)
	}

	if len(payload) < 8 {
		return nil, fmt.Errorf("device info payload too short: %d bytes", len(payload))
	}

	info := &domain.DeviceInfo{
		Major:    payload[0],
		Minor:    payload[1],
		Patch:    payload[2],
		Build:    payload[3],
		Serial:   binary.LittleEndian.Uint32(payload[4:8]),
		LogTotal: 0,
		LogUsed:  0,
	}

	if len(payload) >= 10 {
		info.LogTotal = payload[8]
		info.LogUsed = payload[9]
	}

	if len(payload) > 10 {
		info.Algorithms = payload[10:]
	}

	return info, nil
}

func (s *HSMService) GetFirmwareInfo() (*domain.FirmwareInfo, error) {
	if s.transport == nil {
		// Mock/AT fallback
		return &domain.FirmwareInfo{
			Major:      1,
			Minor:      0,
			Patch:      0,
			ActiveSlot: 0,
			ImageSize:  0,
		}, nil
	}

	respCmd, payload, err := s.transport.SendAndReceive(domain.CmdDfuInfo, nil, 3*time.Second)
	if err != nil {
		return nil, err
	}

	if respCmd != (domain.CmdDfuInfo | domain.RspSuccessBit) {
		return nil, fmt.Errorf("unexpected response opcode 0x%02X", respCmd)
	}

	if len(payload) < 11 {
		return nil, fmt.Errorf("firmware info payload too short: %d bytes", len(payload))
	}

	info := &domain.FirmwareInfo{
		Major:      payload[0],
		Minor:      payload[1],
		Patch:      payload[2],
		ActiveSlot: binary.LittleEndian.Uint32(payload[3:7]),
		ImageSize:  binary.LittleEndian.Uint32(payload[7:11]),
	}

	if len(payload) >= 43 {
		info.ImageSHA256 = payload[11:43]
	}

	return info, nil
}

func (s *HSMService) Unlock() error {
	if s.atClient != nil {
		return s.atClient.Auth("123456")
	}

	respCmd, _, err := s.transport.SendAndReceive(domain.CmdAuthSession, []byte{0x00}, 3*time.Second)
	if err != nil {
		return err
	}
	if respCmd != (domain.CmdAuthSession | domain.RspSuccessBit) {
		return fmt.Errorf("auth failed: 0x%02X", respCmd)
	}
	return nil
}

func (s *HSMService) Lock() error {
	if s.atClient != nil {
		return s.atClient.Lock()
	}

	respCmd, _, err := s.transport.SendAndReceive(domain.CmdCloseSession, nil, 3*time.Second)
	if err != nil {
		return err
	}
	if respCmd != (domain.CmdCloseSession | domain.RspSuccessBit) {
		return fmt.Errorf("close session failed: 0x%02X", respCmd)
	}
	return nil
}

func (s *HSMService) Reset() error {
	if s.atClient != nil {
		return s.atClient.Reset()
	}

	respCmd, _, err := s.transport.SendAndReceive(domain.CmdResetDevice, nil, 3*time.Second)
	if err != nil {
		return err
	}
	if respCmd != (domain.CmdResetDevice | domain.RspSuccessBit) {
		return fmt.Errorf("reset failed: 0x%02X", respCmd)
	}
	return nil
}

func (s *HSMService) UpdateFirmware(filePath string, onProgress func(sent, total int, pct float64)) error {
	if s.transport == nil {
		return fmt.Errorf("DFU update requires legacy binary serial connection")
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read firmware file: %w", err)
	}

	total := len(data)
	if total == 0 {
		return fmt.Errorf("firmware file is empty")
	}

	chunkSize := 256
	offset := 0

	for offset < total {
		end := offset + chunkSize
		if end > total {
			end = total
		}

		chunk := data[offset:end]
		payload := make([]byte, 4+len(chunk))
		binary.LittleEndian.PutUint32(payload[0:4], uint32(offset))
		copy(payload[4:], chunk)

		respCmd, _, err := s.transport.SendAndReceive(domain.CmdDfuChunk, payload, 5*time.Second)
		if err != nil {
			return fmt.Errorf("chunk write failed at offset %d: %w", offset, err)
		}
		if respCmd != (domain.CmdDfuChunk | domain.RspSuccessBit) {
			return fmt.Errorf("device rejected chunk at offset %d: 0x%02X", offset, respCmd)
		}

		offset = end
		if onProgress != nil {
			onProgress(offset, total, float64(offset)/float64(total)*100.0)
		}
	}

	respCmd, _, err := s.transport.SendAndReceive(domain.CmdDfuApply, nil, 10*time.Second)
	if err != nil {
		return fmt.Errorf("failed to apply firmware update: %w", err)
	}
	if respCmd != (domain.CmdDfuApply | domain.RspSuccessBit) {
		return fmt.Errorf("firmware apply rejected by device: 0x%02X", respCmd)
	}

	return nil
}
