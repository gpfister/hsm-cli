//go:build darwin && !cgo

package serial

import "github.com/gpfister/nrf52840-hsm-cli/internal/domain"

func getDetailedMatchingPorts(targetVID, targetPID string) ([]string, error) {
	return nil, nil
}

func getDetailedDongles() ([]domain.DiscoveredDongle, error) {
	return nil, nil
}
