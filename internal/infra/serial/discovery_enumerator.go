//go:build darwin || linux || windows

package serial

import (
	"os"
	"strings"

	"github.com/gpfister/nrf52840-hsm-cli/internal/domain"
	"go.bug.st/serial/enumerator"
)

func getDetailedMatchingPorts(targetVID, targetPID string) ([]string, error) {
	dongles, err := getDetailedDongles()
	if err != nil {
		return nil, err
	}
	var ports []string
	for _, d := range dongles {
		if d.DataPort != "" {
			ports = append(ports, d.DataPort)
		}
		if d.ConsolePort != "" {
			ports = append(ports, d.ConsolePort)
		}
	}
	SortPortsByPriority(ports)
	return ports, nil
}

func getDetailedDongles() ([]domain.DiscoveredDongle, error) {
	detailed, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return nil, err
	}

	type portInfo struct {
		name         string
		serialNumber string
		vid          string
		pid          string
		product      string
		manufacturer string
	}

	var matching []portInfo
	for _, p := range detailed {
		if p.IsUSB && strings.EqualFold(p.VID, "1209") && strings.EqualFold(p.PID, "0001") {
			name := p.Name
			// On macOS, if /dev/cu.usbmodem* is reported, prefer /dev/tty.usbmodem* if present on disk
			if strings.HasPrefix(name, "/dev/cu.") {
				ttyName := "/dev/tty." + strings.TrimPrefix(name, "/dev/cu.")
				if _, err := os.Stat(ttyName); err == nil {
					name = ttyName
				}
			}
			matching = append(matching, portInfo{
				name:         name,
				serialNumber: strings.TrimSpace(p.SerialNumber),
				vid:          p.VID,
				pid:          p.PID,
				product:      p.Product,
				manufacturer: p.Manufacturer,
			})
		}
	}

	if len(matching) == 0 {
		return nil, nil
	}

	// Group ports by device: serial number if available, else BLE address, else node prefix
	groups := make(map[string][]portInfo)
	var groupOrder []string

	for _, p := range matching {
		key := p.serialNumber
		if key == "" {
			key = ExtractBLEAddressFromPort(p.name)
		}
		if key == "" {
			key = stripTrailingDigit(p.name)
		}

		if _, exists := groups[key]; !exists {
			groupOrder = append(groupOrder, key)
		}
		groups[key] = append(groups[key], p)
	}

	var results []domain.DiscoveredDongle
	for _, key := range groupOrder {
		ports := groups[key]
		var dataPort, consolePort string
		var vid, pid, product, mfg, serialNum string

		for _, p := range ports {
			if vid == "" {
				vid = p.vid
				pid = p.pid
				product = p.product
				mfg = p.manufacturer
			}
			if serialNum == "" && p.serialNumber != "" {
				serialNum = p.serialNumber
			}

			// Prioritize suffix 3 as AT data port, suffix 1 as Console port
			if strings.HasSuffix(p.name, "3") {
				dataPort = p.name
			} else if strings.HasSuffix(p.name, "1") {
				consolePort = p.name
			} else if dataPort == "" {
				dataPort = p.name
			} else if consolePort == "" {
				consolePort = p.name
			}
		}

		if dataPort == "" && len(ports) > 0 {
			dataPort = ports[0].name
		}

		if serialNum == "" {
			serialNum = ExtractBLEAddressFromPort(dataPort)
		}
		if serialNum == "" && consolePort != "" {
			serialNum = ExtractBLEAddressFromPort(consolePort)
		}
		if serialNum == "" {
			serialNum = key
		}

		results = append(results, domain.DiscoveredDongle{
			Serial:       NormalizeSerial(serialNum),
			DataPort:     dataPort,
			ConsolePort:  consolePort,
			VID:          vid,
			PID:          pid,
			Product:      product,
			Manufacturer: mfg,
		})
	}

	return results, nil
}
