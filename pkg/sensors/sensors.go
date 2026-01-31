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
	// EnabledSensors controls which sensors are enabled.
	// Key is the sensor ID (e.g., "cpu", "memory", "nvidia_gpu").
	// If nil, all available sensors are enabled.
	// If empty map, no sensors are enabled.
	EnabledSensors map[string]bool

	// DisabledSensors is a list of sensor IDs to disable.
	// This is applied after EnabledSensors, allowing selective disabling.
	DisabledSensors []string

	// Options contains provider-specific configuration.
	// Keys are in the format "provider_id.option_name" (e.g., "disk.mounts", "network.interface").
	// Values can be strings, []string, or other types depending on the option.
	Options map[string]interface{}
}

// DefaultConfig returns a default configuration.
func DefaultConfig() *Config {
	return &Config{
		EnabledSensors:  nil, // nil means all sensors enabled
		DisabledSensors: nil,
		Options:         nil, // nil means providers use their defaults
	}
}

// GetOption retrieves a typed option value from the config.
func GetOption[T any](c *Config, key string) (T, bool) {
	var zero T
	if c.Options == nil {
		return zero, false
	}
	v, ok := c.Options[key]
	if !ok {
		return zero, false
	}
	typed, ok := v.(T)
	return typed, ok
}

// GetStringSliceOption retrieves a string slice option, handling both []string and []interface{}.
func (c *Config) GetStringSliceOption(key string) ([]string, bool) {
	if c.Options == nil {
		return nil, false
	}
	v, ok := c.Options[key]
	if !ok {
		return nil, false
	}

	// Direct []string
	if ss, ok := v.([]string); ok {
		return ss, true
	}

	// []interface{} from JSON unmarshaling
	if ii, ok := v.([]interface{}); ok {
		result := make([]string, 0, len(ii))
		for _, i := range ii {
			if s, ok := i.(string); ok {
				result = append(result, s)
			}
		}
		return result, len(result) > 0
	}

	return nil, false
}

// GetStringOption retrieves a string option.
func (c *Config) GetStringOption(key string) (string, bool) {
	return GetOption[string](c, key)
}

// GetStringMapOption retrieves a map[string]string option, handling JSON unmarshaling.
func (c *Config) GetStringMapOption(key string) (map[string]string, bool) {
	if c.Options == nil {
		return nil, false
	}
	v, ok := c.Options[key]
	if !ok {
		return nil, false
	}

	// Direct map[string]string
	if m, ok := v.(map[string]string); ok {
		return m, true
	}

	// map[string]interface{} from JSON unmarshaling
	if mi, ok := v.(map[string]interface{}); ok {
		result := make(map[string]string, len(mi))
		for k, val := range mi {
			if s, ok := val.(string); ok {
				result[k] = s
			}
		}
		return result, len(result) > 0
	}

	return nil, false
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
