//go:build !windows

package main

import "unsafe"

// setWindowIcon is a no-op on platforms where window icons are managed natively by the OS (macOS/Linux).
func setWindowIcon(windowHandle unsafe.Pointer) {
}
