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

type DeviceInfo struct {
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
	return fmt.Sprintf("%d.%d.%d-build%d", d.Major, d.Minor, d.Patch, d.Build)
}
