//go:build (darwin && cgo) || linux || windows

package serial

import (
	"strings"

	"go.bug.st/serial/enumerator"
)

func getDetailedMatchingPorts(targetVID, targetPID string) ([]string, error) {
	detailed, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return nil, err
	}

	var matches []string
	for _, p := range detailed {
		if p.IsUSB && strings.EqualFold(p.VID, targetVID) && strings.EqualFold(p.PID, targetPID) {
			matches = append(matches, p.Name)
		}
	}
	return matches, nil
}
