package domain

// DeviceStatus represents the authentication and key store status returned by AT+STATUS?.
type DeviceStatus struct {
	Authenticated    bool
	RetriesRemaining int
	KeyCount         int
}

// KeySlot represents a cryptographic key slot returned by AT+SLOTS?.
type KeySlot struct {
	ID        int
	Algorithm string
	Size      int
	Label     string
}

// KeyGenResult represents the output of AT+KEYGEN=.
type KeyGenResult struct {
	ID           int
	PublicKeyHex string
}
