//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	procSendMessageW     = user32.NewProc("SendMessageW")
	procLoadImageW       = user32.NewProc("LoadImageW")
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
)

const (
	wmSetIcon      = 0x0080
	iconSmall      = 0
	iconBig        = 1
	imageIcon      = 1
	lrDefaultColor = 0x0000
)

// setWindowIcon applies the embedded 1 ICON resource to the window titlebar and taskbar.
func setWindowIcon(windowHandle unsafe.Pointer) {
	if windowHandle == nil {
		return
	}
	hwnd := uintptr(windowHandle)
	hinst, _, _ := procGetModuleHandleW.Call(0)

	// Load small icon (16x16) for window title bar (Resource ID 1)
	hSmallIcon, _, _ := procLoadImageW.Call(
		hinst,
		uintptr(1),
		imageIcon,
		16,
		16,
		lrDefaultColor,
	)
	if hSmallIcon != 0 {
		procSendMessageW.Call(hwnd, wmSetIcon, iconSmall, hSmallIcon)
	}

	// Load big icon (32x32) for taskbar and Alt+Tab
	hBigIcon, _, _ := procLoadImageW.Call(
		hinst,
		uintptr(1),
		imageIcon,
		32,
		32,
		lrDefaultColor,
	)
	if hBigIcon != 0 {
		procSendMessageW.Call(hwnd, wmSetIcon, iconBig, hBigIcon)
	}
}
