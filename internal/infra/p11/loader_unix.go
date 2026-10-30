//go:build !windows

package p11

import (
	"fmt"

	"github.com/ebitengine/purego"
)

// DynamicLibrary wraps an OS shared library handle.
type DynamicLibrary struct {
	handle uintptr
	path   string
}

// OpenLibrary loads a PKCS#11 shared library (.dylib or .so).
func OpenLibrary(path string) (*DynamicLibrary, error) {
	h, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return nil, fmt.Errorf("failed to load shared library %q: %w", path, err)
	}
	return &DynamicLibrary{
		handle: h,
		path:   path,
	}, nil
}

// FindSymbol locates a symbol in the loaded dynamic library.
func (l *DynamicLibrary) FindSymbol(name string) (uintptr, error) {
	sym, err := purego.Dlsym(l.handle, name)
	if err != nil {
		return 0, fmt.Errorf("symbol %q not found in %q: %w", name, l.path, err)
	}
	return sym, nil
}

// Close releases the loaded dynamic library handle.
func (l *DynamicLibrary) Close() error {
	if l.handle != 0 {
		err := purego.Dlclose(l.handle)
		l.handle = 0
		return err
	}
	return nil
}
