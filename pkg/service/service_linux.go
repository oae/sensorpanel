//go:build linux

package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const serviceName = "sensorpanel.service"

func serviceDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "systemd", "user"), nil
}

func servicePath() (string, error) {
	dir, err := serviceDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, serviceName), nil
}

func executablePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Abs(exe)
}

func generateServiceFile(runArgs []string) (string, error) {
	exePath, err := executablePath()
	if err != nil {
		return "", err
	}

	execStart := exePath + " run"
	for _, arg := range runArgs {
		execStart += fmt.Sprintf(" %q", arg)
	}

	return fmt.Sprintf(`[Unit]
Description=SensorPanel USB LCD Display
PartOf=graphical-session.target
After=graphical-session.target

[Service]
Type=simple
ExecStart=%s
Restart=on-failure
RestartSec=5

[Install]
WantedBy=graphical-session.target
`, execStart), nil
}

func install(runArgs []string) error {
	dir, err := serviceDir()
	if err != nil {
		return err
	}

	// Create directory if needed
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create service directory: %w", err)
	}

	// Generate and write service file
	content, err := generateServiceFile(runArgs)
	if err != nil {
		return err
	}

	path, err := servicePath()
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write service file: %w", err)
	}

	// Reload systemd
	if err := exec.Command("systemctl", "--user", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	// Enable service
	if err := exec.Command("systemctl", "--user", "enable", serviceName).Run(); err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}

	return nil
}

func uninstall() error {
	// Stop service first
	exec.Command("systemctl", "--user", "stop", serviceName).Run()

	// Disable service
	exec.Command("systemctl", "--user", "disable", serviceName).Run()

	// Remove service file
	path, err := servicePath()
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove service file: %w", err)
	}

	// Reload systemd
	exec.Command("systemctl", "--user", "daemon-reload").Run()

	return nil
}

func start() error {
	if err := exec.Command("systemctl", "--user", "start", serviceName).Run(); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}
	return nil
}

func stop() error {
	if err := exec.Command("systemctl", "--user", "stop", serviceName).Run(); err != nil {
		return fmt.Errorf("failed to stop service: %w", err)
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

	// Check if running
	out, err := exec.Command("systemctl", "--user", "is-active", serviceName).Output()
	if err == nil && strings.TrimSpace(string(out)) == "active" {
		s.Running = true
		s.State = "running"

		// Get PID
		pidOut, err := exec.Command("systemctl", "--user", "show", serviceName, "--property=MainPID", "--value").Output()
		if err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(pidOut))); err == nil && pid > 0 {
				s.PID = pid
			}
		}
	} else {
		s.State = "stopped"
	}

	return s, nil
}

func logs(follow bool, lines int) error {
	args := []string{"--user", "-u", serviceName, "-n", strconv.Itoa(lines)}
	if follow {
		args = append(args, "-f")
	}

	cmd := exec.Command("journalctl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	// Handle interrupt gracefully
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	return cmd.Run()
}
