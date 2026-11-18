//go:build windows

// ============================================================================
// FILE: usb/icon_windows.go
// PURPOSE: Sets the native Win32 titlebar and taskbar icon (WM_SETICON) at runtime.
// ============================================================================
package usb

import (
	"syscall"
	"time"
	"unsafe"
)

// ApplyNativeWindowIcon sets the native Win32 window titlebar icon (ICON_SMALL)
// and taskbar icon (ICON_BIG) for all top-level windows belonging to this process.
func ApplyNativeWindowIcon(bigPNG []byte, smallPNG []byte) {
	if len(bigPNG) == 0 {
		return
	}
	user32 := syscall.NewLazyDLL("user32.dll")
	procCreateIconFromResourceEx := user32.NewProc("CreateIconFromResourceEx")
	procSendMessageW := user32.NewProc("SendMessageW")
	procGetForegroundWindow := user32.NewProc("GetForegroundWindow")
	procFindWindowW := user32.NewProc("FindWindowW")

	hIconBig, _, _ := procCreateIconFromResourceEx.Call(
		uintptr(unsafe.Pointer(&bigPNG[0])),
		uintptr(len(bigPNG)),
		1,
		0x00030000,
		64,
		64,
		0,
	)
	if len(smallPNG) == 0 {
		smallPNG = bigPNG
	}
	hIconSmall, _, _ := procCreateIconFromResourceEx.Call(
		uintptr(unsafe.Pointer(&smallPNG[0])),
		uintptr(len(smallPNG)),
		1,
		0x00030000,
		32,
		32,
		0,
	)
	if hIconBig == 0 && hIconSmall == 0 {
		return
	}

	const wmSetIcon = 0x0080
	titlePtr, _ := syscall.UTF16PtrFromString("Instant Server")
	for attempt := 0; attempt < 12; attempt++ {
		hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(titlePtr)))
		if hwnd == 0 {
			hwnd, _, _ = procGetForegroundWindow.Call()
		}
		if hwnd != 0 {
			if hIconSmall != 0 {
				_, _, _ = procSendMessageW.Call(hwnd, wmSetIcon, 0, hIconSmall)
			}
			if hIconBig != 0 {
				_, _, _ = procSendMessageW.Call(hwnd, wmSetIcon, 1, hIconBig)
			}
		}
		time.Sleep(120 * time.Millisecond)
	}
}
