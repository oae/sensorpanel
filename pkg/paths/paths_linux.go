//go:build linux

package paths

import (
	"os"
	"path/filepath"
)

// ConfigDir returns the XDG config directory for sensorpanel.
// Default: ~/.config/sensorpanel/
func ConfigDir() (string, error) {
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, appName), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".config", appName), nil
}

// DataDir returns the XDG data directory for sensorpanel.
// Default: ~/.local/share/sensorpanel/
func DataDir() (string, error) {
	if xdgData := os.Getenv("XDG_DATA_HOME"); xdgData != "" {
		return filepath.Join(xdgData, appName), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".local", "share", appName), nil
}

// CacheDir returns the XDG cache directory for sensorpanel.
// Default: ~/.cache/sensorpanel/
func CacheDir() (string, error) {
	if xdgCache := os.Getenv("XDG_CACHE_HOME"); xdgCache != "" {
		return filepath.Join(xdgCache, appName), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".cache", appName), nil
}
