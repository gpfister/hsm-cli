package domain

import "fmt"

type DeviceState int

const (
	StateUnconfigured DeviceState = iota
	StateLocked
	StateUnlocked
)

func (s DeviceState) String() string {
	switch s {
	case StateUnconfigured:
		return "Unconfigured (Pulsating Blue)"
	case StateLocked:
		return "Locked (Solid Red)"
	case StateUnlocked:
		return "Unlocked (Solid Green)"
	default:
		return "Unknown"
	}
}

// DeviceInfo represents hardware and firmware telemetry returned by AT+INFO? or legacy queries.
type DeviceInfo struct {
	Firmware   string
	Hardware   string
	UID        string
	Boot       string

	// Legacy binary packet compatibility fields
	Major      uint8
	Minor      uint8
	Patch      uint8
	Build      uint8
	Serial     uint32
	LogTotal   uint8
	LogUsed    uint8
	Algorithms []byte
}

func (d DeviceInfo) VersionString() string {
	if d.Firmware != "" {
		return d.Firmware
	}
	return fmt.Sprintf("%d.%d.%d-build%d", d.Major, d.Minor, d.Patch, d.Build)
}

// DiscoveredDongle represents a physical nRF52840 HSM dongle detected via USB/serial.
type DiscoveredDongle struct {
	Serial       string `json:"serial"`                 // 12-digit hex BLE address or USB serial number (e.g. F4CE368F12AB)
	DataPort     string `json:"data_port"`              // AT command/data serial port (prioritizing suffix 3)
	ConsolePort  string `json:"console_port,omitempty"` // Console logging port (suffix 1), if detected
	VID          string `json:"vid,omitempty"`          // USB Vendor ID (e.g. "1209")
	PID          string `json:"pid,omitempty"`          // USB Product ID (e.g. "0001")
	Product      string `json:"product,omitempty"`      // USB Product String
	Manufacturer string `json:"manufacturer,omitempty"` // USB Manufacturer String
}
