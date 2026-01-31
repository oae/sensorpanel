//go:build darwin

package paths

import (
	"os"
	"path/filepath"
)

// ConfigDir returns the Application Support directory for sensorpanel on macOS.
// Default: ~/Library/Application Support/sensorpanel/
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, "Library", "Application Support", appName), nil
}

// DataDir returns the Application Support directory for sensorpanel on macOS.
// Default: ~/Library/Application Support/sensorpanel/
func DataDir() (string, error) {
	// On macOS, data is typically stored with config
	return ConfigDir()
}

// CacheDir returns the Caches directory for sensorpanel on macOS.
// Default: ~/Library/Caches/sensorpanel/
func CacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, "Library", "Caches", appName), nil
}

// DefaultDiskMounts returns the default disk mount points for macOS.
func DefaultDiskMounts() []string {
	return []string{"/"}
}
