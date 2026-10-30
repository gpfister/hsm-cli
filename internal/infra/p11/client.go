package p11

import (
	"fmt"
	"unsafe"

	"github.com/ebitengine/purego"
)

// P11Client provides an idiomatic Go interface to a PKCS#11 Cryptoki shared library.
type P11Client struct {
	lib *DynamicLibrary

	cInitialize       func(pInitArgs uintptr) uintptr
	cFinalize         func(pReserved uintptr) uintptr
	cGetInfo          func(pInfo *CK_INFO) uintptr
	cGetSlotList      func(tokenPresent byte, pSlotList *uintptr, pulCount *uintptr) uintptr
	cGetSlotInfo      func(slotID uintptr, pInfo *CK_SLOT_INFO) uintptr
	cGetTokenInfo     func(slotID uintptr, pInfo *CK_TOKEN_INFO) uintptr
	cGetMechanismList func(slotID uintptr, pMechanismList *uint64, pulCount *uintptr) uintptr
	cOpenSession      func(slotID uintptr, flags uint64, pApp uintptr, notify uintptr, phSession *uintptr) uintptr
	cCloseSession     func(hSession uintptr) uintptr
	cLogin            func(hSession uintptr, userType uintptr, pPin *byte, ulPinLen uintptr) uintptr
	cLogout           func(hSession uintptr) uintptr
	cGenerateRandom   func(hSession uintptr, pRandomData *byte, ulRandomLen uintptr) uintptr
}

// NewP11Client loads the specified dynamic library and registers function bindings.
func NewP11Client(modulePath string) (*P11Client, error) {
	lib, err := OpenLibrary(modulePath)
	if err != nil {
		return nil, err
	}

	client := &P11Client{lib: lib}

	// Register symbols
	if err := client.bind("C_Initialize", &client.cInitialize); err != nil {
		_ = lib.Close()
		return nil, err
	}
	if err := client.bind("C_Finalize", &client.cFinalize); err != nil {
		_ = lib.Close()
		return nil, err
	}
	if err := client.bind("C_GetInfo", &client.cGetInfo); err != nil {
		_ = lib.Close()
		return nil, err
	}
	if err := client.bind("C_GetSlotList", &client.cGetSlotList); err != nil {
		_ = lib.Close()
		return nil, err
	}
	if err := client.bind("C_GetSlotInfo", &client.cGetSlotInfo); err != nil {
		_ = lib.Close()
		return nil, err
	}
	if err := client.bind("C_GetTokenInfo", &client.cGetTokenInfo); err != nil {
		_ = lib.Close()
		return nil, err
	}
	if err := client.bind("C_GetMechanismList", &client.cGetMechanismList); err != nil {
		_ = lib.Close()
		return nil, err
	}
	if err := client.bind("C_OpenSession", &client.cOpenSession); err != nil {
		_ = lib.Close()
		return nil, err
	}
	if err := client.bind("C_CloseSession", &client.cCloseSession); err != nil {
		_ = lib.Close()
		return nil, err
	}
	if err := client.bind("C_Login", &client.cLogin); err != nil {
		_ = lib.Close()
		return nil, err
	}
	if err := client.bind("C_Logout", &client.cLogout); err != nil {
		_ = lib.Close()
		return nil, err
	}
	if err := client.bind("C_GenerateRandom", &client.cGenerateRandom); err != nil {
		_ = lib.Close()
		return nil, err
	}

	return client, nil
}

func (c *P11Client) bind(symbol string, fptr any) error {
	sym, err := c.lib.FindSymbol(symbol)
	if err != nil {
		return err
	}
	purego.RegisterFunc(fptr, sym)
	return nil
}

// Close releases the underlying dynamic library.
func (c *P11Client) Close() error {
	return c.lib.Close()
}

// Initialize initializes Cryptoki.
func (c *P11Client) Initialize() error {
	rv := c.cInitialize(0)
	if rv != CKR_OK && rv != CKR_CRYPTOKI_ALREADY_INITIALIZED {
		return RVError("C_Initialize", rv)
	}
	return nil
}

// Finalize finalizes Cryptoki.
func (c *P11Client) Finalize() error {
	rv := c.cFinalize(0)
	return RVError("C_Finalize", rv)
}

// GetInfo retrieves general library and Cryptoki version information.
func (c *P11Client) GetInfo() (*CK_INFO, error) {
	var info CK_INFO
	rv := c.cGetInfo(&info)
	if rv != CKR_OK {
		return nil, RVError("C_GetInfo", rv)
	}
	return &info, nil
}

// GetSlotList retrieves the list of slot IDs.
func (c *P11Client) GetSlotList(tokenPresent bool) ([]uintptr, error) {
	var tokenPresentByte byte
	if tokenPresent {
		tokenPresentByte = 1
	}

	var count uintptr
	rv := c.cGetSlotList(tokenPresentByte, nil, &count)
	if rv != CKR_OK {
		return nil, RVError("C_GetSlotList (count query)", rv)
	}

	if count == 0 {
		return nil, nil
	}

	slots := make([]uintptr, count)
	rv = c.cGetSlotList(tokenPresentByte, &slots[0], &count)
	if rv != CKR_OK {
		return nil, RVError("C_GetSlotList", rv)
	}

	return slots[:count], nil
}

// GetSlotInfo retrieves information about a specific slot.
func (c *P11Client) GetSlotInfo(slotID uintptr) (*CK_SLOT_INFO, error) {
	var info CK_SLOT_INFO
	rv := c.cGetSlotInfo(slotID, &info)
	if rv != CKR_OK {
		return nil, RVError(fmt.Sprintf("C_GetSlotInfo(slot=%d)", slotID), rv)
	}
	return &info, nil
}

// GetTokenInfo retrieves information about the token in a slot.
func (c *P11Client) GetTokenInfo(slotID uintptr) (*CK_TOKEN_INFO, error) {
	var info CK_TOKEN_INFO
	rv := c.cGetTokenInfo(slotID, &info)
	if rv != CKR_OK {
		return nil, RVError(fmt.Sprintf("C_GetTokenInfo(slot=%d)", slotID), rv)
	}
	return &info, nil
}

// GetMechanismList retrieves supported cryptographic mechanisms for a slot.
func (c *P11Client) GetMechanismList(slotID uintptr) ([]uint64, error) {
	var count uintptr
	rv := c.cGetMechanismList(slotID, nil, &count)
	if rv != CKR_OK {
		return nil, RVError(fmt.Sprintf("C_GetMechanismList (count query, slot=%d)", slotID), rv)
	}

	if count == 0 {
		return nil, nil
	}

	mechs := make([]uint64, count)
	rv = c.cGetMechanismList(slotID, &mechs[0], &count)
	if rv != CKR_OK {
		return nil, RVError(fmt.Sprintf("C_GetMechanismList(slot=%d)", slotID), rv)
	}

	return mechs[:count], nil
}

// OpenSession opens a session on the given slot.
func (c *P11Client) OpenSession(slotID uintptr, flags uint64) (uintptr, error) {
	var hSession uintptr
	rv := c.cOpenSession(slotID, flags, 0, 0, &hSession)
	if rv != CKR_OK {
		return 0, RVError(fmt.Sprintf("C_OpenSession(slot=%d)", slotID), rv)
	}
	return hSession, nil
}

// CloseSession closes an active session.
func (c *P11Client) CloseSession(hSession uintptr) error {
	rv := c.cCloseSession(hSession)
	return RVError(fmt.Sprintf("C_CloseSession(hSession=%d)", hSession), rv)
}

// Login authenticates to the token in an active session.
func (c *P11Client) Login(hSession uintptr, userType uintptr, pin string) error {
	pinBytes := []byte(pin)
	var pinPtr *byte
	if len(pinBytes) > 0 {
		pinPtr = (*byte)(unsafe.Pointer(&pinBytes[0]))
	}

	rv := c.cLogin(hSession, userType, pinPtr, uintptr(len(pinBytes)))
	return RVError("C_Login", rv)
}

// Logout terminates the authenticated session.
func (c *P11Client) Logout(hSession uintptr) error {
	rv := c.cLogout(hSession)
	return RVError("C_Logout", rv)
}

// GenerateRandom obtains random bytes from the token.
func (c *P11Client) GenerateRandom(hSession uintptr, length int) ([]byte, error) {
	if length <= 0 {
		return nil, nil
	}

	buf := make([]byte, length)
	rv := c.cGenerateRandom(hSession, &buf[0], uintptr(length))
	if rv != CKR_OK {
		return nil, RVError("C_GenerateRandom", rv)
	}

	return buf, nil
}
