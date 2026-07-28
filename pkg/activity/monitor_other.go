//go:build !linux

package activity

import "time"

type activeMonitor struct{}

func newMonitor(_ time.Duration) Monitor      { return activeMonitor{} }
func (activeMonitor) Idle() bool              { return false }
func (activeMonitor) Events() <-chan struct{} { return nil }
func (activeMonitor) Close()                  {}
