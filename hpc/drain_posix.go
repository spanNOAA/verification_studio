//go:build !windows

package hpc

import (
	"os"
	"syscall"
)

// drainPTY flushes any pending bytes from the PTY non-blockingly without hanging on POSIX systems (macOS, Linux).
func drainPTY(f *os.File) {
	if f == nil {
		return
	}
	fd := int(f.Fd())
	_ = syscall.SetNonblock(fd, true)
	buf := make([]byte, 4096)
	for {
		n, err := syscall.Read(fd, buf)
		if n <= 0 || err != nil {
			break
		}
	}
	_ = syscall.SetNonblock(fd, false)
}
