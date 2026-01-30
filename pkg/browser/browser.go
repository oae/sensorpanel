// Package browser manages headless Chrome/Chromium for rendering themes.
package browser

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/alperen/sensorpanel/pkg/paths"
)

// Chrome for Testing download URLs (stable versions)
// See: https://googlechromelabs.github.io/chrome-for-testing/
const (
	chromeVersion = "131.0.6778.85" // Stable version
	baseURL       = "https://storage.googleapis.com/chrome-for-testing-public"
)

var (
	// ErrUnsupportedPlatform is returned for unsupported OS/arch combinations.
	ErrUnsupportedPlatform = errors.New("unsupported platform")
	// ErrBrowserNotFound is returned when the browser binary is not found.
	ErrBrowserNotFound = errors.New("browser not found")
	// ErrDownloadFailed is returned when browser download fails.
	ErrDownloadFailed = errors.New("browser download failed")
)

// platformKey returns the Chrome for Testing platform identifier.
func platformKey() (string, error) {
	switch runtime.GOOS {
	case "linux":
		if runtime.GOARCH == "amd64" {
			return "linux64", nil
		}
	case "darwin":
		if runtime.GOARCH == "amd64" {
			return "mac-x64", nil
		}
		if runtime.GOARCH == "arm64" {
			return "mac-arm64", nil
		}
	case "windows":
		if runtime.GOARCH == "amd64" {
			return "win64", nil
		}
		if runtime.GOARCH == "386" {
			return "win32", nil
		}
	}
	return "", ErrUnsupportedPlatform
}

// downloadURL returns the Chrome for Testing download URL.
func downloadURL() (string, error) {
	platform, err := platformKey()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%s/%s/chrome-%s.zip", baseURL, chromeVersion, platform, platform), nil
}

// binaryName returns the expected Chrome binary name for the current platform.
func binaryName() string {
	switch runtime.GOOS {
	case "windows":
		return "chrome.exe"
	case "darwin":
		return "Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing"
	default:
		return "chrome"
	}
}

// BrowserPath returns the path to the Chrome binary if installed.
func BrowserPath() (string, error) {
	browserDir, err := paths.BrowserDir()
	if err != nil {
		return "", err
	}

	// Look for Chrome binary in the browser directory
	platform, err := platformKey()
	if err != nil {
		return "", err
	}

	binaryPath := filepath.Join(browserDir, fmt.Sprintf("chrome-%s", platform), binaryName())

	if _, err := os.Stat(binaryPath); err != nil {
		if os.IsNotExist(err) {
			return "", ErrBrowserNotFound
		}
		return "", err
	}

	return binaryPath, nil
}

// IsInstalled checks if Chrome is installed in the cache directory.
func IsInstalled() bool {
	_, err := BrowserPath()
	return err == nil
}

// SystemChromePath tries to find a system-installed Chrome/Chromium.
func SystemChromePath() (string, error) {
	// Common Chrome/Chromium paths
	candidates := []string{
		// Linux
		"chromium",
		"chromium-browser",
		"google-chrome",
		"google-chrome-stable",
		// macOS
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
		// Windows paths are handled by PATH lookup
		"chrome",
	}

	for _, candidate := range candidates {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}

	return "", ErrBrowserNotFound
}

// GetChromePath returns the path to Chrome, preferring cached version.
// Falls back to system Chrome if cached version is not available.
func GetChromePath() (string, error) {
	// First try cached version
	if path, err := BrowserPath(); err == nil {
		return path, nil
	}

	// Fall back to system Chrome
	return SystemChromePath()
}

// Download downloads and installs Chrome for Testing.
func Download(ctx context.Context, progress func(downloaded, total int64)) error {
	url, err := downloadURL()
	if err != nil {
		return err
	}

	browserDir, err := paths.EnsureBrowserDir()
	if err != nil {
		return err
	}

	// Create temp file for download
	tmpFile, err := os.CreateTemp(browserDir, "chrome-download-*.zip")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// Download
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		tmpFile.Close()
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("%w: %v", ErrDownloadFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		tmpFile.Close()
		return fmt.Errorf("%w: HTTP %d", ErrDownloadFailed, resp.StatusCode)
	}

	// Copy with progress
	var downloaded int64
	total := resp.ContentLength

	buf := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := tmpFile.Write(buf[:n]); writeErr != nil {
				tmpFile.Close()
				return fmt.Errorf("failed to write: %w", writeErr)
			}
			downloaded += int64(n)
			if progress != nil {
				progress(downloaded, total)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			tmpFile.Close()
			return fmt.Errorf("download error: %w", err)
		}
	}
	tmpFile.Close()

	// Extract zip
	if err := extractZip(tmpPath, browserDir); err != nil {
		return fmt.Errorf("failed to extract: %w", err)
	}

	// Verify binary exists
	if _, err := BrowserPath(); err != nil {
		return fmt.Errorf("extraction succeeded but binary not found: %w", err)
	}

	return nil
}

// extractZip extracts a zip file to a directory.
func extractZip(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		// Security: prevent zip slip
		path := filepath.Join(destDir, f.Name)
		if !strings.HasPrefix(path, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(path, f.Mode())
			continue
		}

		// Create parent directories
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}

		// Extract file
		outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

// Remove removes the cached browser.
func Remove() error {
	browserDir, err := paths.BrowserDir()
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(browserDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "chrome-") {
			if err := os.RemoveAll(filepath.Join(browserDir, entry.Name())); err != nil {
				return err
			}
		}
	}

	return nil
}

// Version returns the version of Chrome for Testing being used.
func Version() string {
	return chromeVersion
}
