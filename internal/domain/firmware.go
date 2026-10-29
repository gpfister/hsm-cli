package domain

import "fmt"

type FirmwareInfo struct {
	Major       uint8
	Minor       uint8
	Patch       uint8
	ActiveSlot  uint32
	ImageSize   uint32
	ImageSHA256 []byte
}

func (f FirmwareInfo) VersionString() string {
	return fmt.Sprintf("v%d.%d.%d", f.Major, f.Minor, f.Patch)
}

func (f FirmwareInfo) SlotString() string {
	if f.ActiveSlot == 0 {
		return "Slot 0 (Active Application)"
	}
	return fmt.Sprintf("Slot %d", f.ActiveSlot)
}
