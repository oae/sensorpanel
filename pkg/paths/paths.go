// Package paths provides XDG-compliant directory paths for sensorpanel.
//
// Directory structure:
//   - Config:  $XDG_CONFIG_HOME/sensorpanel/ (default: ~/.config/sensorpanel/)
//   - Data:    $XDG_DATA_HOME/sensorpanel/   (default: ~/.local/share/sensorpanel/)
//   - Cache:   $XDG_CACHE_HOME/sensorpanel/  (default: ~/.cache/sensorpanel/)
//
// Subdirectories:
//   - themes/  -> in Data directory (user-installed themes)
//   - browser/ -> in Cache directory (headless browser binary)
package paths

import (
	"os"
	"path/filepath"
)

const appName = "sensorpanel"

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

// ThemesDir returns the directory where themes are stored.
// Default: ~/.local/share/sensorpanel/themes/
func ThemesDir() (string, error) {
	dataDir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "themes"), nil
}

// BrowserDir returns the directory where the headless browser is stored.
// Default: ~/.cache/sensorpanel/browser/
func BrowserDir() (string, error) {
	cacheDir, err := CacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, "browser"), nil
}

// ThemeDir returns the directory for a specific theme.
func ThemeDir(themeName string) (string, error) {
	themesDir, err := ThemesDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(themesDir, themeName), nil
}

// EnsureDir creates a directory if it doesn't exist.
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// EnsureThemesDir creates the themes directory if it doesn't exist.
func EnsureThemesDir() (string, error) {
	dir, err := ThemesDir()
	if err != nil {
		return "", err
	}
	if err := EnsureDir(dir); err != nil {
		return "", err
	}
	return dir, nil
}

// EnsureBrowserDir creates the browser cache directory if it doesn't exist.
func EnsureBrowserDir() (string, error) {
	dir, err := BrowserDir()
	if err != nil {
		return "", err
	}
	if err := EnsureDir(dir); err != nil {
		return "", err
	}
	return dir, nil
}
