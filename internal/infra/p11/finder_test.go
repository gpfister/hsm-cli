package p11_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gpfister/nrf52840-hsm-cli/internal/infra/p11"
)

func TestFinder_LibraryExtensionAndName(t *testing.T) {
	ext := p11.LibraryExtension()
	switch runtime.GOOS {
	case "darwin":
		if ext != ".dylib" {
			t.Errorf("expected .dylib on darwin, got %s", ext)
		}
	case "windows":
		if ext != ".dll" {
			t.Errorf("expected .dll on windows, got %s", ext)
		}
	default:
		if ext != ".so" {
			t.Errorf("expected .so on unix/linux, got %s", ext)
		}
	}

	name := p11.DefaultLibraryFilename()
	if !strings.HasPrefix(name, "libnrf52840_pkcs11") {
		t.Errorf("expected filename prefix libnrf52840_pkcs11, got %s", name)
	}
}

func TestFinder_FindModule_ExplicitPath(t *testing.T) {
	// Create temporary dummy file
	tmpDir := t.TempDir()
	dummyLib := filepath.Join(tmpDir, p11.DefaultLibraryFilename())
	if err := os.WriteFile(dummyLib, []byte("dummy"), 0644); err != nil {
		t.Fatalf("failed to create dummy lib: %v", err)
	}

	found, err := p11.FindModule(dummyLib)
	if err != nil {
		t.Fatalf("expected explicit path to be found, got error: %v", err)
	}
	if found != dummyLib {
		t.Errorf("expected %s, got %s", dummyLib, found)
	}

	// Non-existent explicit path
	_, err = p11.FindModule(filepath.Join(tmpDir, "nonexistent.dylib"))
	if err == nil {
		t.Errorf("expected error for non-existent path, got nil")
	}
}

func TestFinder_FindModule_EnvVar(t *testing.T) {
	tmpDir := t.TempDir()
	dummyLib := filepath.Join(tmpDir, "custom_p11"+p11.LibraryExtension())
	if err := os.WriteFile(dummyLib, []byte("dummy"), 0644); err != nil {
		t.Fatalf("failed to create dummy lib: %v", err)
	}

	t.Setenv("NRF52840_PKCS11_MODULE", dummyLib)

	found, err := p11.FindModule("")
	if err != nil {
		t.Fatalf("expected env module to be found, got error: %v", err)
	}
	if found != dummyLib {
		t.Errorf("expected %s, got %s", dummyLib, found)
	}
}
