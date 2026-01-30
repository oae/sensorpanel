//go:build windows

package sensors

import (
	"syscall"
	"unsafe"
)

// syscallStatfs wraps the statfs result for cross-platform compatibility.
type syscallStatfs struct {
	Bsize  int64
	Blocks uint64
	Bfree  uint64
	Bavail uint64
}

var (
	kernel32            = syscall.NewLazyDLL("kernel32.dll")
	getDiskFreeSpaceExW = kernel32.NewProc("GetDiskFreeSpaceExW")
)

// statfs gets disk statistics on Windows using GetDiskFreeSpaceExW.
func statfs(path string, stat *syscallStatfs) error {
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}

	var freeBytesAvailable, totalBytes, totalFreeBytes uint64

	ret, _, err := getDiskFreeSpaceExW.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFreeBytes)),
	)

	if ret == 0 {
		return err
	}

	// Windows doesn't have block size concept like Unix, use 1 byte blocks
	stat.Bsize = 1
	stat.Blocks = totalBytes
	stat.Bfree = totalFreeBytes
	stat.Bavail = freeBytesAvailable

	return nil
}
