//go:build linux

package activity

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// evdevMonitor watches all selected input nodes through one epoll goroutine.
// It works on Wayland without compositor-specific IPC and requires read access
// to /dev/input.
type evdevMonitor struct {
	lastInput atomic.Int64
	files     []*os.File
	done      chan struct{}
	events    chan struct{}
	epollFD   int
	once      sync.Once
}

func newMonitor(_ time.Duration) Monitor {
	epollFD, err := unix.EpollCreate1(unix.EPOLL_CLOEXEC)
	if err != nil {
		return activeMonitor{}
	}
	m := &evdevMonitor{
		done:    make(chan struct{}),
		events:  make(chan struct{}, 1),
		epollFD: epollFD,
	}
	m.lastInput.Store(time.Now().UnixNano())
	for _, path := range inputEventPaths() {
		fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
		if err != nil {
			continue
		}
		file := os.NewFile(uintptr(fd), path)
		if err := unix.EpollCtl(epollFD, unix.EPOLL_CTL_ADD, fd, &unix.EpollEvent{
			Events: unix.EPOLLIN,
			Fd:     int32(fd),
		}); err != nil {
			_ = file.Close()
			continue
		}
		m.files = append(m.files, file)
	}
	// Without input permission, report active rather than unexpectedly dropping
	// a dashboard into low-power mode.
	if len(m.files) == 0 {
		_ = unix.Close(epollFD)
		return activeMonitor{}
	}
	go m.watch()
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

func (m *evdevMonitor) watch() {
	epollEvents := make([]unix.EpollEvent, 16)
	timevalSize := int(unsafe.Sizeof(unix.Timeval{}))
	eventSize := timevalSize + 8
	buffer := make([]byte, eventSize*32)
	for {
		count, err := unix.EpollWait(m.epollFD, epollEvents, 1000)
		if err != nil {
			if err == unix.EINTR {
				continue
			}
			return
		}
		for i := 0; i < count; i++ {
			n, readErr := unix.Read(int(epollEvents[i].Fd), buffer)
			if readErr != nil && readErr != unix.EAGAIN {
				continue
			}
			meaningful := false
			for offset := 0; offset+eventSize <= n; offset += eventSize {
				eventType := uint16(buffer[offset+timevalSize]) |
					uint16(buffer[offset+timevalSize+1])<<8
				if eventType == unix.EV_KEY || eventType == unix.EV_REL || eventType == unix.EV_ABS {
					meaningful = true
					break
				}
			}
			if meaningful {
				m.lastInput.Store(time.Now().UnixNano())
				select {
				case m.events <- struct{}{}:
				default:
				}
			}
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

func (m *evdevMonitor) Events() <-chan struct{} { return m.events }

func (m *evdevMonitor) Close() {
	m.once.Do(func() {
		close(m.done)
		if m.epollFD >= 0 {
			_ = unix.Close(m.epollFD)
			m.epollFD = -1
		}
		for _, file := range m.files {
			_ = file.Close()
		}
	})
}

type activeMonitor struct{}

func (activeMonitor) Idle() bool              { return false }
func (activeMonitor) Events() <-chan struct{} { return nil }
func (activeMonitor) Close()                  {}
