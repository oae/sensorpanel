package theme

import (
	"fmt"
	"os/exec"
	"runtime"
)

// OpenBrowser opens a URL in the default browser.
func OpenBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return cmd.Start()
}

// CanOpenBrowser checks if we can open a browser on this system.
func CanOpenBrowser() bool {
	switch runtime.GOOS {
	case "linux":
		_, err := exec.LookPath("xdg-open")
		return err == nil
	case "darwin", "windows":
		return true
	default:
		return false
	}
}
