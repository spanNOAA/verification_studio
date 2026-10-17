//go:build windows

package hpc

import (
	"os"
	"time"
)

// drainPTY flushes any pending bytes from the Windows pseudo-terminal / pipe non-blockingly.
func drainPTY(f *os.File) {
	if f == nil {
		return
	}
	_ = f.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
	buf := make([]byte, 4096)
	for {
		n, err := f.Read(buf)
		if n <= 0 || err != nil {
			break
		}
	}
	_ = f.SetReadDeadline(time.Time{})
}
