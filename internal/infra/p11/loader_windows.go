//go:build windows

package p11

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// DynamicLibrary wraps a Windows DLL handle.
type DynamicLibrary struct {
	handle windows.Handle
	path   string
}

// OpenLibrary loads a Windows PKCS#11 DLL.
func OpenLibrary(path string) (*DynamicLibrary, error) {
	h, err := windows.LoadLibrary(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load DLL %q: %w", path, err)
	}
	return &DynamicLibrary{
		handle: h,
		path:   path,
	}, nil
}

// FindSymbol locates a symbol in the loaded DLL.
func (l *DynamicLibrary) FindSymbol(name string) (uintptr, error) {
	sym, err := windows.GetProcAddress(l.handle, name)
	if err != nil {
		return 0, fmt.Errorf("symbol %q not found in %q: %w", name, l.path, err)
	}
	return sym, nil
}

// Close releases the loaded DLL handle.
func (l *DynamicLibrary) Close() error {
	if l.handle != 0 {
		err := windows.FreeLibrary(l.handle)
		l.handle = 0
		return err
	}
	return nil
}
