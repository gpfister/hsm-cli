//go:build darwin && !cgo

package serial

func getDetailedMatchingPorts(targetVID, targetPID string) ([]string, error) {
	return nil, nil
}
