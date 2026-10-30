package p11

import (
	"fmt"
	"unsafe"

	"github.com/ebitengine/purego"
)

// P11Client provides an idiomatic Go interface to a PKCS#11 Cryptoki shared library.
type P11Client struct {
	lib        *DynamicLibrary
	modulePath string

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
	cInitToken        func(slotID uintptr, pPin *byte, ulPinLen uintptr, pLabel *byte) uintptr
	cGenerateKeyPair  func(hSession uintptr, pMechanism *CK_MECHANISM, pPublicKeyTemplate *CK_ATTRIBUTE, ulPublicKeyAttributeCount uintptr, pPrivateKeyTemplate *CK_ATTRIBUTE, ulPrivateKeyAttributeCount uintptr, phPublicKey *uintptr, phPrivateKey *uintptr) uintptr
	cSignInit         func(hSession uintptr, pMechanism *CK_MECHANISM, hKey uintptr) uintptr
	cSign             func(hSession uintptr, pData *byte, ulDataLen uintptr, pSignature *byte, pulSignatureLen *uintptr) uintptr
}

// NewP11Client loads the specified dynamic library and registers function bindings.
// If modulePath is empty, it uses FindModule to auto-discover the library.
func NewP11Client(modulePath string) (*P11Client, error) {
	if modulePath == "" {
		resolved, err := FindModule("")
		if err != nil {
			return nil, err
		}
		modulePath = resolved
	}

	lib, err := OpenLibrary(modulePath)
	if err != nil {
		return nil, err
	}

	client := &P11Client{lib: lib, modulePath: modulePath}

	// Register core symbols
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

	// Optional extended cryptographic functions
	_ = client.bind("C_InitToken", &client.cInitToken)
	_ = client.bind("C_GenerateKeyPair", &client.cGenerateKeyPair)
	_ = client.bind("C_SignInit", &client.cSignInit)
	_ = client.bind("C_Sign", &client.cSign)

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

// ModulePath returns the resolved file path of the loaded PKCS#11 module.
func (c *P11Client) ModulePath() string {
	return c.modulePath
}

// InitToken initializes a token on the specified slot via C_InitToken.
func (c *P11Client) InitToken(slotID uintptr, pin string, label string) error {
	if c.cInitToken == nil {
		return fmt.Errorf("C_InitToken symbol not available in loaded PKCS#11 provider")
	}

	pinBytes := []byte(pin)
	var pinPtr *byte
	if len(pinBytes) > 0 {
		pinPtr = &pinBytes[0]
	}

	labelBytes := make([]byte, 32)
	copy(labelBytes, []byte(label))
	for i := len(label); i < 32; i++ {
		labelBytes[i] = ' '
	}

	rv := c.cInitToken(slotID, pinPtr, uintptr(len(pinBytes)), &labelBytes[0])
	return RVError(fmt.Sprintf("C_InitToken(slot=%d)", slotID), rv)
}

// GenerateKeyPair generates an ECDSA keypair on the active session via C_GenerateKeyPair.
func (c *P11Client) GenerateKeyPair(hSession uintptr, keyID int, label string) (uintptr, uintptr, error) {
	if c.cGenerateKeyPair == nil {
		return 0, 0, fmt.Errorf("C_GenerateKeyPair symbol not available in loaded PKCS#11 provider")
	}

	mech := CK_MECHANISM{
		Mechanism: CKM_EC_KEY_PAIR_GEN,
	}

	var pubTemplates []CK_ATTRIBUTE
	var privTemplates []CK_ATTRIBUTE

	var keyIDByte byte
	if keyID > 0 {
		keyIDByte = byte(keyID)
		pubTemplates = append(pubTemplates, CK_ATTRIBUTE{
			Type:     CKA_ID,
			Value:    uintptr(unsafe.Pointer(&keyIDByte)),
			ValueLen: 1,
		})
		privTemplates = append(privTemplates, CK_ATTRIBUTE{
			Type:     CKA_ID,
			Value:    uintptr(unsafe.Pointer(&keyIDByte)),
			ValueLen: 1,
		})
	}

	var labelBytes []byte
	if label != "" {
		labelBytes = []byte(label)
		pubTemplates = append(pubTemplates, CK_ATTRIBUTE{
			Type:     CKA_LABEL,
			Value:    uintptr(unsafe.Pointer(&labelBytes[0])),
			ValueLen: uint64(len(labelBytes)),
		})
		privTemplates = append(privTemplates, CK_ATTRIBUTE{
			Type:     CKA_LABEL,
			Value:    uintptr(unsafe.Pointer(&labelBytes[0])),
			ValueLen: uint64(len(labelBytes)),
		})
	}

	var pubPtr *CK_ATTRIBUTE
	if len(pubTemplates) > 0 {
		pubPtr = &pubTemplates[0]
	}
	var privPtr *CK_ATTRIBUTE
	if len(privTemplates) > 0 {
		privPtr = &privTemplates[0]
	}

	var hPub, hPriv uintptr
	rv := c.cGenerateKeyPair(hSession, &mech, pubPtr, uintptr(len(pubTemplates)), privPtr, uintptr(len(privTemplates)), &hPub, &hPriv)
	if rv != CKR_OK {
		return 0, 0, RVError("C_GenerateKeyPair", rv)
	}

	return hPub, hPriv, nil
}

// Sign initializes and computes an ECDSA signature over data using C_SignInit and C_Sign.
func (c *P11Client) Sign(hSession uintptr, hKey uintptr, mechanism uint64, data []byte) ([]byte, error) {
	if c.cSignInit == nil || c.cSign == nil {
		return nil, fmt.Errorf("C_SignInit/C_Sign symbols not available in loaded PKCS#11 provider")
	}

	mech := CK_MECHANISM{
		Mechanism: mechanism,
	}

	rv := c.cSignInit(hSession, &mech, hKey)
	if rv != CKR_OK {
		return nil, RVError(fmt.Sprintf("C_SignInit(hKey=%d)", hKey), rv)
	}

	var dataPtr *byte
	if len(data) > 0 {
		dataPtr = &data[0]
	}

	var sigLen uintptr
	rv = c.cSign(hSession, dataPtr, uintptr(len(data)), nil, &sigLen)
	if rv != CKR_OK {
		return nil, RVError("C_Sign (size query)", rv)
	}

	if sigLen == 0 {
		sigLen = 64
	}

	sig := make([]byte, sigLen)
	rv = c.cSign(hSession, dataPtr, uintptr(len(data)), &sig[0], &sigLen)
	if rv != CKR_OK {
		return nil, RVError("C_Sign", rv)
	}

	return sig[:sigLen], nil
}
