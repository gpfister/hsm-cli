package p11

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// DefaultLibraryBasename is the standard library name across platforms without extension.
const DefaultLibraryBasename = "libnrf52840_pkcs11"

// LibraryExtension returns the platform-specific shared library file extension.
func LibraryExtension() string {
	switch runtime.GOOS {
	case "darwin":
		return ".dylib"
	case "windows":
		return ".dll"
	default:
		return ".so"
	}
}

// DefaultLibraryFilename returns the platform-specific library file name.
func DefaultLibraryFilename() string {
	return DefaultLibraryBasename + LibraryExtension()
}

// FindModule resolves the PKCS#11 module path using the following precedence:
// 1. Explicit path (CLI flag --module)
// 2. NRF52840_PKCS11_MODULE environment variable
// 3. Workspace build paths (../nrf52840-hsm-libpkcs11/build/, ./build/)
// 4. Standard platform system paths
func FindModule(explicitPath string) (string, error) {
	// 1. Explicit path
	if strings.TrimSpace(explicitPath) != "" {
		p := filepath.Clean(strings.TrimSpace(explicitPath))
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			abs, err := filepath.Abs(p)
			if err == nil {
				return abs, nil
			}
			return p, nil
		}
		return "", fmt.Errorf("specified PKCS#11 module not found: %s", explicitPath)
	}

	// 2. Environment variable
	if envPath := strings.TrimSpace(os.Getenv("NRF52840_PKCS11_MODULE")); envPath != "" {
		p := filepath.Clean(envPath)
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			abs, err := filepath.Abs(p)
			if err == nil {
				return abs, nil
			}
			return p, nil
		}
		return "", fmt.Errorf("NRF52840_PKCS11_MODULE path not found: %s", envPath)
	}

	// 3. Candidate discovery locations
	filename := DefaultLibraryFilename()
	var candidates []string

	// Check relative to cwd
	cwdCandidates := []string{
		filepath.Join("build", filename),
		filepath.Join(".", "build", filename),
		filepath.Join("..", "build", filename),
		filepath.Join("..", "hsm-libpkcs11", "build", filename),
		filepath.Join("..", "..", "hsm-libpkcs11", "build", filename),
		filepath.Join("hsm-libpkcs11", "build", filename),
		filepath.Join("..", "nrf52840-hsm-libpkcs11", "build", filename),
		filepath.Join("..", "..", "nrf52840-hsm-libpkcs11", "build", filename),
		filepath.Join("nrf52840-hsm-libpkcs11", "build", filename),
	}
	candidates = append(candidates, cwdCandidates...)

	// Check relative to executable directory
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		candidates = append(candidates,
			filepath.Join(execDir, filename),
			filepath.Join(execDir, "build", filename),
			filepath.Join(execDir, "..", "build", filename),
			filepath.Join(execDir, "..", "hsm-libpkcs11", "build", filename),
			filepath.Join(execDir, "..", "..", "hsm-libpkcs11", "build", filename),
			filepath.Join(execDir, "..", "nrf52840-hsm-libpkcs11", "build", filename),
			filepath.Join(execDir, "..", "..", "nrf52840-hsm-libpkcs11", "build", filename),
		)
	}

	// 4. Standard platform system locations
	switch runtime.GOOS {
	case "darwin":
		candidates = append(candidates,
			"/usr/local/lib/"+filename,
			"/opt/homebrew/lib/"+filename,
			"/Library/OpenSC/lib/"+filename,
			"/usr/lib/"+filename,
		)
	case "windows":
		candidates = append(candidates,
			`C:\Windows\System32\`+filename,
			`C:\Program Files\nRF52840-HSM\`+filename,
			`C:\Program Files (x86)\nRF52840-HSM\`+filename,
		)
	default: // Linux / Unix
		candidates = append(candidates,
			"/usr/local/lib/"+filename,
			"/usr/lib/"+filename,
			"/usr/lib/x86_64-linux-gnu/"+filename,
			"/usr/lib/aarch64-linux-gnu/"+filename,
			"/usr/lib64/"+filename,
		)
	}

	// Iterate and find first valid file
	var checked []string
	for _, c := range candidates {
		cleaned := filepath.Clean(c)
		checked = append(checked, cleaned)
		if info, err := os.Stat(cleaned); err == nil && !info.IsDir() {
			abs, err := filepath.Abs(cleaned)
			if err == nil {
				return abs, nil
			}
			return cleaned, nil
		}
	}

	return "", fmt.Errorf("PKCS#11 provider library (%s) not found. Checked:\n  - %s\nSpecify module via --module flag or NRF52840_PKCS11_MODULE environment variable",
		filename, strings.Join(checked[:min(len(checked), 6)], "\n  - "))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
