//go:build linux

package sensors

import (
	"syscall"
)

// syscallStatfs wraps the statfs result for cross-platform compatibility.
type syscallStatfs struct {
	Bsize  int64
	Blocks uint64
	Bfree  uint64
	Bavail uint64
}

// statfs wraps syscall.Statfs for disk statistics.
func statfs(path string, stat *syscallStatfs) error {
	var sysstat syscall.Statfs_t
	if err := syscall.Statfs(path, &sysstat); err != nil {
		return err
	}

	stat.Bsize = sysstat.Bsize
	stat.Blocks = sysstat.Blocks
	stat.Bfree = sysstat.Bfree
	stat.Bavail = sysstat.Bavail

	return nil
}
