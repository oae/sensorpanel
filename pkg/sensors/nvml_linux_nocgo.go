//go:build linux && !cgo

package sensors

import "fmt"

func nvmlAvailable() bool { return false }

func queryNVML() (map[string]interface{}, error) {
	return nil, fmt.Errorf("NVML is unavailable without cgo")
}
