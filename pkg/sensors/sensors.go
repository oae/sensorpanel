// Package sensors collects system metrics from various sources.
//
// Supported metrics:
//   - CPU: temperature, load, frequency
//   - GPU: temperature, load, memory (NVIDIA via nvidia-smi, AMD via sysfs)
//   - RAM: usage statistics
//   - Disk: usage per mount point
//   - Network: throughput per interface
//
// This package uses a modular provider system. Use Collector for collecting
// sensor data. The provider implementations are platform-specific and located
// in *_linux.go, *_darwin.go, and *_windows.go files.
//
// Collector returns sensor data as map[string]interface{} which can be
// directly serialized to JSON and passed to renderers and themes.
package sensors

import "fmt"

// Config configures the sensor collector.
type Config struct {
	// Which sensors to collect
	ShowCPU     bool
	ShowGPU     bool
	ShowRAM     bool
	ShowDisk    bool
	ShowNetwork bool

	// Disk mount points to monitor
	DiskMounts []string

	// Network interface pattern (e.g., "eth*", "enp*", "*" for all)
	NetworkInterface string

	// GPU collection method: "auto", "nvidia", "amd"
	GPUMethod string

	// Path to nvidia-smi (auto-detected if empty)
	NvidiaSMIPath string
}

// DefaultConfig returns a default configuration.
func DefaultConfig() *Config {
	return &Config{
		ShowCPU:          true,
		ShowGPU:          true,
		ShowRAM:          true,
		ShowDisk:         true,
		ShowNetwork:      true,
		DiskMounts:       []string{"/"},
		NetworkInterface: "*",
		GPUMethod:        "auto",
	}
}

// FormatBytes formats a byte count as a human-readable string.
func FormatBytes(bytes float64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	for _, unit := range units {
		if bytes < 1024 {
			return fmt.Sprintf("%.1f%s", bytes, unit)
		}
		bytes /= 1024
	}
	return fmt.Sprintf("%.1fPB", bytes)
}

// FormatBytesPerSec formats bytes per second as a human-readable string.
func FormatBytesPerSec(bps float64) string {
	return FormatBytes(bps) + "/s"
}
