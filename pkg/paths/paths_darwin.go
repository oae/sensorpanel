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
// Respects XDG_DATA_HOME if set (for testing).
func DataDir() (string, error) {
	// Check XDG_DATA_HOME first (mainly for testing)
	if xdgData := os.Getenv("XDG_DATA_HOME"); xdgData != "" {
		return filepath.Join(xdgData, appName), nil
	}
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
