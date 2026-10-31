package serial

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gpfister/nrf52840-hsm-cli/internal/domain"
	"go.bug.st/serial"
)

// NormalizeSerial strips delimiters (colons, hyphens, spaces) and uppercases a serial or BLE address.
func NormalizeSerial(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ":", "")
	s = strings.ReplaceAll(s, "-", "")
	return strings.ToUpper(s)
}

func isHex(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// ExtractBLEAddressFromPort attempts to parse a 12-digit hex BLE address (or 16-digit hex UID)
// from a USB CDC-ACM device path like /dev/tty.usbmodemF4CE368F12AB3 or /dev/cu.usbmodemF4CE368F12AB1.
func ExtractBLEAddressFromPort(portPath string) string {
	base := filepath.Base(portPath)
	lower := strings.ToLower(base)

	// Remove common serial prefix keywords
	var rem string
	for _, prefix := range []string{"tty.usbmodem", "cu.usbmodem", "usbmodem", "ttyacm"} {
		if strings.HasPrefix(lower, prefix) {
			rem = base[len(prefix):]
			break
		}
	}
	if rem == "" {
		rem = base
	}

	// Remove trailing interface digit if present (e.g. 1 or 3)
	if len(rem) > 1 && (rem[len(rem)-1] >= '0' && rem[len(rem)-1] <= '9') {
		candidate := rem[:len(rem)-1]
		cleaned := NormalizeSerial(candidate)
		if (len(cleaned) == 12 || len(cleaned) == 16) && isHex(cleaned) {
			return cleaned
		}
	}

	// Try complete remaining string
	cleaned := NormalizeSerial(rem)
	if (len(cleaned) == 12 || len(cleaned) == 16) && isHex(cleaned) {
		return cleaned
	}

	return ""
}

// stripTrailingDigit strips a single trailing interface index (e.g. '1' or '3') to group dual CDC ports.
func stripTrailingDigit(p string) string {
	if len(p) > 1 && p[len(p)-1] >= '0' && p[len(p)-1] <= '9' {
		return p[:len(p)-1]
	}
	return p
}

// PortPriority evaluates a port path's suitability as an HSM AT data communication channel.
// Suffix 3 (AT channel) receives high precedence over suffix 1 (Console logging).
// /dev/tty.* nodes receive precedence over /dev/cu.* nodes on macOS.
func PortPriority(name string) int {
	lower := strings.ToLower(name)
	score := 0

	// Prefer call-in /dev/tty.* over call-out /dev/cu.* on macOS
	if strings.HasPrefix(name, "/dev/tty.") {
		score += 15
	}

	// Prioritize suffix 3 (dedicated AT data channel)
	if strings.HasSuffix(lower, "3") {
		score += 100
	} else if strings.HasSuffix(lower, "1") {
		// Demote suffix 1 (firmware console logging channel)
		score -= 50
	}

	// General USB CDC-ACM affinity
	if strings.Contains(lower, "usbmodem") || strings.Contains(lower, "ttyacm") {
		score += 20
	}

	return score
}

// SortPortsByPriority sorts a slice of port names in place in descending priority order.
func SortPortsByPriority(ports []string) {
	sort.SliceStable(ports, func(i, j int) bool {
		return PortPriority(ports[i]) > PortPriority(ports[j])
	})
}

// fallbackEnumerateDongles inspects serial.GetPortsList() when detailed USB enumeration is not active.
func fallbackEnumerateDongles() ([]domain.DiscoveredDongle, error) {
	list, err := serial.GetPortsList()
	if err != nil {
		return nil, err
	}

	// Filter for USB CDC-ACM candidates
	var candidates []string
	hasTTY := make(map[string]bool)
	for _, p := range list {
		if strings.HasPrefix(p, "/dev/tty.usbmodem") {
			hasTTY[strings.TrimPrefix(p, "/dev/tty.")] = true
		}
	}

	for _, p := range list {
		lower := strings.ToLower(p)
		if strings.Contains(lower, "usbmodem") || strings.Contains(lower, "ttyacm") || strings.Contains(lower, "com") {
			// Skip cu.usbmodem if corresponding tty.usbmodem is present
			if strings.HasPrefix(p, "/dev/cu.") {
				baseSuffix := strings.TrimPrefix(p, "/dev/cu.")
				if hasTTY[baseSuffix] {
					continue
				}
			}
			candidates = append(candidates, p)
		}
	}

	if len(candidates) == 0 {
		return nil, nil
	}

	// Group by prefix without trailing interface digit
	groups := make(map[string][]string)
	var groupOrder []string

	for _, c := range candidates {
		key := stripTrailingDigit(c)
		if _, exists := groups[key]; !exists {
			groupOrder = append(groupOrder, key)
		}
		groups[key] = append(groups[key], c)
	}

	var results []domain.DiscoveredDongle
	for _, key := range groupOrder {
		ports := groups[key]
		var dataPort, consolePort string

		for _, p := range ports {
			if strings.HasSuffix(p, "3") {
				dataPort = p
			} else if strings.HasSuffix(p, "1") {
				consolePort = p
			} else if dataPort == "" {
				dataPort = p
			} else if consolePort == "" {
				consolePort = p
			}
		}

		if dataPort == "" && len(ports) > 0 {
			dataPort = ports[0]
		}

		serialNum := ExtractBLEAddressFromPort(dataPort)
		if serialNum == "" && consolePort != "" {
			serialNum = ExtractBLEAddressFromPort(consolePort)
		}
		if serialNum == "" {
			serialNum = filepath.Base(key)
		}

		results = append(results, domain.DiscoveredDongle{
			Serial:       NormalizeSerial(serialNum),
			DataPort:     dataPort,
			ConsolePort:  consolePort,
			VID:          "1209",
			PID:          "0001",
			Product:      "nRF52840 HSM Dongle",
			Manufacturer: "pid.codes",
		})
	}

	return results, nil
}

var enumerateDonglesFn = defaultEnumerateDongles

// defaultEnumerateDongles inspects USB enumerators and fallback port scans for nRF52840 dongles.
func defaultEnumerateDongles() ([]domain.DiscoveredDongle, error) {
	detailed, err := getDetailedDongles()
	if err == nil && len(detailed) > 0 {
		return detailed, nil
	}

	fallback, err := fallbackEnumerateDongles()
	if err == nil && len(fallback) > 0 {
		return fallback, nil
	}

	var dongles []domain.DiscoveredDongle
	if envPort := os.Getenv("NRF52840_HSM_PORT"); envPort != "" {
		dongles = append(dongles, domain.DiscoveredDongle{
			Serial:   "MOCK-HSM",
			DataPort: envPort,
			Product:  "Virtual HSM Mock PTY",
		})
	}

	return dongles, nil
}

// EnumerateDongles lists all detected nRF52840 HSM dongles attached to the system.
func EnumerateDongles() ([]domain.DiscoveredDongle, error) {
	return enumerateDonglesFn()
}

// FindDonglePortsTargeting scans for available dongle serial ports, optionally filtered by serial/BLE address.
func FindDonglePortsTargeting(targetSerial string) ([]string, error) {
	if targetSerial == "" {
		targetSerial = os.Getenv("NRF52840_HSM_SERIAL")
	}

	// Environment variable port override takes precedence if no serial target is specified
	if envPort := os.Getenv("NRF52840_HSM_PORT"); envPort != "" && targetSerial == "" {
		return []string{envPort}, nil
	}

	dongles, err := EnumerateDongles()
	if err != nil {
		return nil, err
	}

	var matchingDongles []domain.DiscoveredDongle
	if targetSerial != "" {
		norm := NormalizeSerial(targetSerial)
		for _, d := range dongles {
			dNorm := NormalizeSerial(d.Serial)
			if dNorm == norm ||
				strings.HasPrefix(dNorm, norm) ||
				strings.HasSuffix(dNorm, norm) ||
				strings.Contains(strings.ToUpper(d.DataPort), norm) {
				matchingDongles = append(matchingDongles, d)
			}
		}
	} else {
		matchingDongles = dongles
	}

	var ports []string
	for _, d := range matchingDongles {
		if d.DataPort != "" {
			ports = append(ports, d.DataPort)
		}
		if d.ConsolePort != "" {
			ports = append(ports, d.ConsolePort)
		}
	}

	SortPortsByPriority(ports)

	if len(ports) > 0 {
		return ports, nil
	}

	// Direct port scan fallback
	list, err := serial.GetPortsList()
	if err != nil {
		return nil, err
	}

	var fallbackMatches []string
	for _, name := range list {
		lower := strings.ToLower(name)
		if strings.Contains(lower, "usbmodem") || strings.Contains(lower, "ttyacm") || strings.Contains(lower, "com") {
			if targetSerial == "" || strings.Contains(strings.ToUpper(name), NormalizeSerial(targetSerial)) {
				fallbackMatches = append(fallbackMatches, name)
			}
		}
	}

	SortPortsByPriority(fallbackMatches)
	if len(fallbackMatches) > 0 {
		return fallbackMatches, nil
	}

	if targetSerial != "" {
		return nil, nil
	}

	return list, nil
}

// AutoDiscoverPortTargeting locates and selects the HSM AT command serial port for a specific serial / BLE target.
// When targetSerial is empty, it returns the single detected dongle port, or ErrMultipleDevicesDetected if
// more than one dongle is present.
func AutoDiscoverPortTargeting(targetSerial string) (string, error) {
	if targetSerial == "" {
		targetSerial = os.Getenv("NRF52840_HSM_SERIAL")
	}

	if envPort := os.Getenv("NRF52840_HSM_PORT"); envPort != "" && targetSerial == "" {
		return envPort, nil
	}

	dongles, err := EnumerateDongles()
	if err != nil {
		return "", fmt.Errorf("failed to scan serial ports: %w", err)
	}

	if targetSerial == "" {
		if len(dongles) == 0 {
			return "", domain.ErrDeviceNotFound
		}
		if len(dongles) == 1 {
			return dongles[0].DataPort, nil
		}
		return "", &domain.ErrMultipleDevicesDetected{Devices: dongles}
	}

	// Filter dongles by matching normalized serial, BLE address, or prefix
	norm := NormalizeSerial(targetSerial)
	var matches []domain.DiscoveredDongle
	for _, d := range dongles {
		dNorm := NormalizeSerial(d.Serial)
		if dNorm == norm ||
			strings.HasPrefix(dNorm, norm) ||
			strings.HasSuffix(dNorm, norm) ||
			strings.Contains(strings.ToUpper(d.DataPort), norm) {
			matches = append(matches, d)
		}
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("no nRF52840 HSM Dongle found matching %q: %w", targetSerial, domain.ErrDeviceNotFound)
	}
	if len(matches) > 1 {
		return "", &domain.ErrMultipleDevicesDetected{Devices: matches}
	}

	return matches[0].DataPort, nil
}

// ResolvePortOrSerial resolves an input target which may be a direct port path (/dev/..., COM..., or file path)
// or a serial/BLE address, returning the AT data port path.
func ResolvePortOrSerial(target string) (string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return AutoDiscoverPortTargeting("")
	}

	// If target points to an existing file/dev path or starts with /dev/ or COM, return directly
	if strings.HasPrefix(target, "/dev/") ||
		strings.HasPrefix(strings.ToUpper(target), "COM") ||
		strings.Contains(target, string(os.PathSeparator)) {
		return target, nil
	}

	if _, err := os.Stat(target); err == nil {
		return target, nil
	}

	return AutoDiscoverPortTargeting(target)
}
