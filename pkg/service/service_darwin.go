//go:build darwin

package service

import (
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	serviceName = "com.sensorpanel.agent"
	plistName   = serviceName + ".plist"
	labelName   = serviceName
)

func serviceDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents"), nil
}

func servicePath() (string, error) {
	dir, err := serviceDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, plistName), nil
}

func logPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "Logs", "sensorpanel.log"), nil
}

func executablePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Abs(exe)
}

func generatePlist(runArgs []string) (string, error) {
	exePath, err := executablePath()
	if err != nil {
		return "", err
	}

	logFile, err := logPath()
	if err != nil {
		return "", err
	}

	// Build program arguments
	args := fmt.Sprintf(`    <string>%s</string>
    <string>run</string>`, html.EscapeString(exePath))

	for _, arg := range runArgs {
		args += fmt.Sprintf("\n    <string>%s</string>", html.EscapeString(arg))
	}

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>%s</string>
  <key>ProgramArguments</key>
  <array>
%s
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <dict>
    <key>SuccessfulExit</key>
    <false/>
  </dict>
  <key>StandardOutPath</key>
  <string>%s</string>
  <key>StandardErrorPath</key>
  <string>%s</string>
</dict>
</plist>
`, labelName, args, html.EscapeString(logFile), html.EscapeString(logFile)), nil
}

func install(runArgs []string) error {
	dir, err := serviceDir()
	if err != nil {
		return err
	}

	// Create directory if needed
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create LaunchAgents directory: %w", err)
	}

	// Generate plist
	content, err := generatePlist(runArgs)
	if err != nil {
		return err
	}

	// Write plist file
	path, err := servicePath()
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write plist file: %w", err)
	}

	return nil
}

func uninstall() error {
	// Unload service first
	path, err := servicePath()
	if err != nil {
		return err
	}

	exec.Command("launchctl", "unload", path).Run()

	// Remove plist file
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove plist file: %w", err)
	}

	return nil
}

func start() error {
	path, err := servicePath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("service not installed, run 'sensorpanel service install' first")
	}

	if err := exec.Command("launchctl", "load", path).Run(); err != nil {
		return fmt.Errorf("failed to load service: %w", err)
	}
	return nil
}

func stop() error {
	path, err := servicePath()
	if err != nil {
		return err
	}

	if err := exec.Command("launchctl", "unload", path).Run(); err != nil {
		return fmt.Errorf("failed to unload service: %w", err)
	}
	return nil
}

func status() (*ServiceStatus, error) {
	path, err := servicePath()
	if err != nil {
		return nil, err
	}

	s := &ServiceStatus{
		Path: path,
	}

	// Check if installed
	if _, err := os.Stat(path); os.IsNotExist(err) {
		s.State = "not installed"
		return s, nil
	}
	s.Installed = true

	// Check if running using launchctl list
	out, err := exec.Command("launchctl", "list", labelName).Output()
	if err != nil {
		s.State = "stopped"
		return s, nil
	}

	// Parse output for PID
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.Contains(line, "PID") {
			parts := strings.Split(line, "=")
			if len(parts) == 2 {
				if pid, err := strconv.Atoi(strings.TrimSpace(strings.TrimSuffix(parts[1], ";"))); err == nil && pid > 0 {
					s.PID = pid
					s.Running = true
					s.State = "running"
				}
			}
		}
	}

	if !s.Running {
		s.State = "stopped"
	}

	return s, nil
}

func logs(follow bool, lines int) error {
	logFile, err := logPath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		fmt.Println("No logs found yet.")
		return nil
	}

	var cmd *exec.Cmd
	if follow {
		cmd = exec.Command("tail", "-f", "-n", strconv.Itoa(lines), logFile)
	} else {
		cmd = exec.Command("tail", "-n", strconv.Itoa(lines), logFile)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}
