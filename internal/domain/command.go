package domain

import "errors"

const (
	RspSuccessBit byte = 0x80
	RspError      byte = 0x7f

	CmdEcho            byte = 0x01
	CmdCreateSession   byte = 0x03
	CmdAuthSession     byte = 0x04
	CmdSessionMsg      byte = 0x05
	CmdDeviceInfo      byte = 0x06
	CmdResetDevice     byte = 0x07
	CmdCloseSession    byte = 0x08
	CmdGetStorageInfo  byte = 0x09
	CmdGetPseudoRandom byte = 0x1f

	// DFU firmware extensions
	CmdDfuInfo  byte = 0x70
	CmdDfuChunk byte = 0x71
	CmdDfuApply byte = 0x72
)

var (
	ErrInvalidPacket  = errors.New("invalid packet format or length")
	ErrDeviceError    = errors.New("device returned error status")
	ErrDeviceTimeout  = errors.New("timeout waiting for device response")
	ErrDeviceNotFound = errors.New("no compatible nRF52840 HSM Dongle detected")
)
