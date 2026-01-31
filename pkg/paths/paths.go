// Package paths provides platform-specific directory paths for sensorpanel.
//
// Directory structure varies by platform:
//
// Linux:
//   - Config:  $XDG_CONFIG_HOME/sensorpanel/ (default: ~/.config/sensorpanel/)
//   - Data:    $XDG_DATA_HOME/sensorpanel/   (default: ~/.local/share/sensorpanel/)
//   - Cache:   $XDG_CACHE_HOME/sensorpanel/  (default: ~/.cache/sensorpanel/)
//
// macOS:
//   - Config:  ~/Library/Application Support/sensorpanel/
//   - Data:    ~/Library/Application Support/sensorpanel/
//   - Cache:   ~/Library/Caches/sensorpanel/
//
// Windows:
//   - Config:  %APPDATA%\sensorpanel\
//   - Data:    %LOCALAPPDATA%\sensorpanel\
//   - Cache:   %LOCALAPPDATA%\sensorpanel\cache\
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

// ThemesDir returns the directory where themes are stored.
func ThemesDir() (string, error) {
	dataDir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "themes"), nil
}

// BrowserDir returns the directory where the headless browser is stored.
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
