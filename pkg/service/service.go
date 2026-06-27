// Package service provides cross-platform autostart service management.
package service

// ServiceStatus represents the current state of the service.
type ServiceStatus struct {
	Installed bool
	Running   bool
	State     string // "running", "stopped", "not installed"
	Path      string // Path to service file
	PID       int    // Process ID if running
}

// Install installs sensorpanel as an autostart service.
// runArgs are passed to the sensorpanel run command.
func Install(runArgs []string) error {
	return install(runArgs)
}

// Uninstall removes the sensorpanel autostart service.
func Uninstall() error {
	return uninstall()
}

// Start starts the service.
func Start() error {
	return start()
}

// Stop stops the service.
func Stop() error {
	return stop()
}

// Status returns the current service status.
func Status() (*ServiceStatus, error) {
	return status()
}

// Logs shows service logs.
func Logs(follow bool, lines int) error {
	return logs(follow, lines)
}
