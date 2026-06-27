//go:build !linux

package music

import "context"

type LyricLine struct {
	Time float64 `json:"time"`
	Text string  `json:"text"`
}

type Snapshot struct {
	Available bool `json:"available"`
}

type Monitor struct{}

func NewMonitor() *Monitor               { return &Monitor{} }
func (m *Monitor) Start(context.Context) {}
func (m *Monitor) Snapshot() Snapshot    { return Snapshot{} }
func ParseLRC(string) []LyricLine        { return nil }
