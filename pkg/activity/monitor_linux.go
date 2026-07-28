//go:build linux

package activity

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// evdevMonitor listens to keyboard and mouse event nodes. It works on Wayland
// without compositor-specific IPC and requires read access to /dev/input.
type evdevMonitor struct {
	lastInput atomic.Int64
	files     []*os.File
	done      chan struct{}
	once      sync.Once
}

func newMonitor(_ time.Duration) Monitor {
	paths := inputEventPaths()
	m := &evdevMonitor{done: make(chan struct{})}
	m.lastInput.Store(time.Now().UnixNano())
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		m.files = append(m.files, file)
		go m.watch(file)
	}
	// Without input permission, report active rather than unexpectedly dropping
	// a dashboard into low-power mode.
	if len(m.files) == 0 {
		return activeMonitor{}
	}
	return m
}

func inputEventPaths() []string {
	patterns := []string{
		"/dev/input/by-id/*-event-kbd",
		"/dev/input/by-id/*-event-mouse",
	}
	seen := make(map[string]bool)
	paths := make([]string, 0)
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		for _, path := range matches {
			realPath, err := filepath.EvalSymlinks(path)
			if err != nil || seen[realPath] {
				continue
			}
			seen[realPath] = true
			paths = append(paths, realPath)
		}
	}
	return paths
}

func (m *evdevMonitor) watch(file *os.File) {
	buffer := make([]byte, 24*16) // Linux input_event is 24 bytes on 64-bit.
	for {
		n, err := file.Read(buffer)
		if n > 0 {
			// Input nodes selected above are keyboard/mouse only; any event is
			// meaningful desktop activity. Event data is intentionally discarded.
			m.lastInput.Store(time.Now().UnixNano())
		}
		if err != nil {
			return
		}
		select {
		case <-m.done:
			return
		default:
		}
	}
}

func (m *evdevMonitor) Idle() bool {
	return time.Since(time.Unix(0, m.lastInput.Load())) >= time.Second
}

func (m *evdevMonitor) Close() {
	m.once.Do(func() {
		close(m.done)
		for _, file := range m.files {
			_ = file.Close()
		}
	})
}

type activeMonitor struct{}

func (activeMonitor) Idle() bool { return false }
func (activeMonitor) Close()     {}
