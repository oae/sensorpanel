//go:build windows

package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	serviceName = "SensorPanel"
	registryKey = `Software\Microsoft\Windows\CurrentVersion\Run`
)

func startupDir() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		appData = filepath.Join(home, "AppData", "Roaming")
	}
	return filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs", "Startup"), nil
}

func shortcutPath() (string, error) {
	dir, err := startupDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, serviceName+".bat"), nil
}

func logPath() (string, error) {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		localAppData = filepath.Join(home, "AppData", "Local")
	}
	return filepath.Join(localAppData, "sensorpanel", "sensorpanel.log"), nil
}

func executablePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Abs(exe)
}

func generateBatchFile(opts []string) (string, error) {
	exePath, err := executablePath()
	if err != nil {
		return "", err
	}

	logFile, err := logPath()
	if err != nil {
		return "", err
	}

	// Ensure log directory exists
	logDir := filepath.Dir(logFile)

	cmd := fmt.Sprintf(`@echo off
if not exist "%s" mkdir "%s"
"%s" run`, logDir, logDir, exePath)

	for _, opt := range opts {
		cmd += fmt.Sprintf(` --opt "%s"`, opt)
	}

	cmd += fmt.Sprintf(` >> "%s" 2>&1`, logFile)

	return cmd, nil
}

func install(opts []string) error {
	// Generate batch file
	content, err := generateBatchFile(opts)
	if err != nil {
		return err
	}

	// Write batch file to startup folder
	path, err := shortcutPath()
	if err != nil {
		return err
	}

	// Ensure startup directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create startup directory: %w", err)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write batch file: %w", err)
	}

	// Also add to registry for more reliable startup
	key, _, err := registry.CreateKey(registry.CURRENT_USER, registryKey, registry.SET_VALUE)
	if err != nil {
		// Non-fatal, batch file in startup folder should still work
		fmt.Printf("Warning: could not add registry entry: %v\n", err)
		return nil
	}
	defer key.Close()

	exePath, _ := executablePath()
	runCmd := fmt.Sprintf(`"%s" run`, exePath)
	for _, opt := range opts {
		runCmd += fmt.Sprintf(` --opt "%s"`, opt)
	}

	if err := key.SetStringValue(serviceName, runCmd); err != nil {
		fmt.Printf("Warning: could not set registry value: %v\n", err)
	}

	return nil
}

func uninstall() error {
	// Remove batch file
	path, err := shortcutPath()
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove batch file: %w", err)
	}

	// Remove registry entry
	key, err := registry.OpenKey(registry.CURRENT_USER, registryKey, registry.SET_VALUE)
	if err == nil {
		defer key.Close()
		key.DeleteValue(serviceName)
	}

	return nil
}

func start() error {
	path, err := shortcutPath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("service not installed, run 'sensorpanel service install' first")
	}

	// Start the batch file in background
	cmd := exec.Command("cmd", "/c", "start", "/b", "", path)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	return nil
}

func stop() error {
	// Find and kill sensorpanel.exe process
	cmd := exec.Command("taskkill", "/IM", "sensorpanel.exe", "/F")
	if err := cmd.Run(); err != nil {
		// Check if it's because process wasn't found
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 128 {
			return fmt.Errorf("service is not running")
		}
		return fmt.Errorf("failed to stop service: %w", err)
	}
	return nil
}

func status() (*ServiceStatus, error) {
	path, err := shortcutPath()
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

	// Check if running using tasklist
	out, err := exec.Command("tasklist", "/FI", "IMAGENAME eq sensorpanel.exe", "/FO", "CSV", "/NH").Output()
	if err != nil {
		s.State = "stopped"
		return s, nil
	}

	output := string(out)
	if strings.Contains(output, "sensorpanel.exe") {
		s.Running = true
		s.State = "running"
		// Try to extract PID from CSV output
		// Format: "sensorpanel.exe","1234","Console","1","12,345 K"
		parts := strings.Split(output, ",")
		if len(parts) >= 2 {
			pidStr := strings.Trim(parts[1], "\" \r\n")
			if pid, err := parseInt(pidStr); err == nil {
				s.PID = pid
			}
		}
	} else {
		s.State = "stopped"
	}

	return s, nil
}

func parseInt(s string) (int, error) {
	var result int
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}

func logs(follow bool, lines int) error {
	logFile, err := logPath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		fmt.Println("No logs found yet.")
		fmt.Printf("Log file location: %s\n", logFile)
		return nil
	}

	if follow {
		// Windows doesn't have tail -f, use PowerShell Get-Content -Wait
		cmd := exec.Command("powershell", "-Command",
			fmt.Sprintf("Get-Content -Path '%s' -Tail %d -Wait", logFile, lines))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		return cmd.Run()
	}

	// Just read last N lines
	cmd := exec.Command("powershell", "-Command",
		fmt.Sprintf("Get-Content -Path '%s' -Tail %d", logFile, lines))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
