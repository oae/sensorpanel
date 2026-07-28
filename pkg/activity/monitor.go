// Package activity reports whether the local desktop session is idle.
package activity

import "time"

// Monitor polls the host desktop's idle state. A missing or unsupported idle
// provider intentionally reports active so dashboards never appear frozen.
type Monitor interface {
	Idle() bool
	Close()
}

// New starts a desktop-activity monitor. pollInterval values below one second
// are clamped because desktop idle state does not need frame-rate polling.
func New(pollInterval time.Duration) Monitor {
	if pollInterval < time.Second {
		pollInterval = time.Second
	}
	return newMonitor(pollInterval)
}
