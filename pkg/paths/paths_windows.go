//go:build windows

package paths

import (
	"os"
	"path/filepath"
)

// ConfigDir returns the AppData Roaming directory for sensorpanel on Windows.
// Default: %APPDATA%\sensorpanel\
func ConfigDir() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		appData = filepath.Join(home, "AppData", "Roaming")
	}

	return filepath.Join(appData, appName), nil
}

// DataDir returns the LocalAppData directory for sensorpanel on Windows.
// Default: %LOCALAPPDATA%\sensorpanel\
// Respects XDG_DATA_HOME if set (for testing).
func DataDir() (string, error) {
	// Check XDG_DATA_HOME first (mainly for testing)
	if xdgData := os.Getenv("XDG_DATA_HOME"); xdgData != "" {
		return filepath.Join(xdgData, appName), nil
	}

	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		localAppData = filepath.Join(home, "AppData", "Local")
	}

	return filepath.Join(localAppData, appName), nil
}

// CacheDir returns the LocalAppData cache directory for sensorpanel on Windows.
// Default: %LOCALAPPDATA%\sensorpanel\cache\
func CacheDir() (string, error) {
	dataDir, err := DataDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dataDir, "cache"), nil
}
