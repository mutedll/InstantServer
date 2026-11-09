//go:build windows

// usb/flasher_windows.go
package usb

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	winFileFlagNoBuffering   = 0x20000000
	winFileFlagWriteThrough  = 0x80000000
	fsctlLockVolume          = 0x00090018
	fsctlDismountVolume      = 0x00090020
	fsctlUnlockVolume        = 0x0009001C
	fsctlAllowExtendedDasdIO = 0x00090083
	ioctlDeleteDriveLayout   = 0x0007C040
	ioctlDiskUpdateProps     = 0x00070140

	esContinuous       = 0x80000000
	esSystemRequired   = 0x00000001
	esDisplayRequired  = 0x00000002
	esAwayModeRequired = 0x00000040
)

// AcquireHostWakeLock prevents Windows from entering sleep, standby, or idle suspend
// while an ISO download or raw USB DMA flash operation is running.
func AcquireHostWakeLock() func() {
	k32 := syscall.NewLazyDLL("kernel32.dll")
	proc := k32.NewProc("SetThreadExecutionState")
	_, _, _ = proc.Call(uintptr(esContinuous | esSystemRequired | esDisplayRequired | esAwayModeRequired))
	return func() {
		_, _, _ = proc.Call(uintptr(esContinuous))
	}
}

type windowsRawDisk struct {
	file          *os.File
	handle        syscall.Handle
	devicePath    string
	lockedVolumes []syscall.Handle
	unbuffered    bool
}

func getDriveLettersForPhysicalDisk(physicalDrivePath string) []string {
	idx := strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(physicalDrivePath)), `\\.\PHYSICALDRIVE`)
	if idx == "" {
		return nil
	}
	for _, ch := range idx {
		if ch < '0' || ch > '9' {
			return nil
		}
	}
	ps := fmt.Sprintf("Get-Partition -DiskNumber %s -ErrorAction SilentlyContinue | Where-Object { $_.DriveLetter } | ForEach-Object { $_.DriveLetter }", idx)
	out, err := exec.Command("powershell.exe", "-NoProfile", "-WindowStyle", "Hidden", "-Command", ps).Output()
	if err != nil {
		return nil
	}
	var letters []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		l := strings.TrimSpace(strings.Trim(line, "\r\n:"))
		if len(l) == 1 && ((l[0] >= 'A' && l[0] <= 'Z') || (l[0] >= 'a' && l[0] <= 'z')) {
			letters = append(letters, strings.ToUpper(l)+":")
		}
	}
	return letters
}

func lockAndDismountWindowsVolume(driveLetter string) (syscall.Handle, error) {
	volPath := `\\.\` + strings.TrimSuffix(strings.TrimSpace(driveLetter), `\`)
	pathPtr, err := syscall.UTF16PtrFromString(volPath)
	if err != nil {
		return syscall.InvalidHandle, err
	}
	hVol, err := syscall.CreateFile(
		pathPtr,
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE,
		nil,
		syscall.OPEN_EXISTING,
		0,
		0,
	)
	if err != nil {
		return syscall.InvalidHandle, err
	}

	var bytesReturned uint32
	_ = syscall.DeviceIoControl(hVol, fsctlAllowExtendedDasdIO, nil, 0, nil, 0, &bytesReturned, nil)

	locked := false
	for attempt := 0; attempt < 20; attempt++ {
		err = syscall.DeviceIoControl(hVol, fsctlLockVolume, nil, 0, nil, 0, &bytesReturned, nil)
		if err == nil {
			locked = true
			break
		}
		time.Sleep(time.Duration(60*(attempt+1)) * time.Millisecond)
	}
	if !locked {
		_ = syscall.CloseHandle(hVol)
		return syscall.InvalidHandle, fmt.Errorf("IS-USB-308: FSCTL_LOCK_VOLUME timed out on %s (close open File Explorer windows)", volPath)
	}

	if err := syscall.DeviceIoControl(hVol, fsctlDismountVolume, nil, 0, nil, 0, &bytesReturned, nil); err != nil {
		_ = syscall.CloseHandle(hVol)
		return syscall.InvalidHandle, fmt.Errorf("IS-USB-309: FSCTL_DISMOUNT_VOLUME failed on %s: %w", volPath, err)
	}
	return hVol, nil
}

func OpenRawDisk(devicePath string) (RawDiskHandle, error) {
	cleanPath := strings.TrimSpace(devicePath)
	upperPath := strings.ToUpper(cleanPath)
	if !strings.HasPrefix(upperPath, `\\.\PHYSICALDRIVE`) {
		f, err := os.OpenFile(cleanPath, os.O_RDWR, 0600)
		if err != nil {
			return nil, fmt.Errorf("IS-USB-301: no physical USB drive found at %s (plug in a removable USB flash drive and click Refresh Drives): %w", cleanPath, err)
		}
		return &windowsRawDisk{
			file:       f,
			handle:     syscall.Handle(f.Fd()),
			devicePath: cleanPath,
			unbuffered: false,
		}, nil
	}

	// Lock and dismount all mounted volume letters on this PhysicalDrive first
	// so Windows Kernel PnP Manager permits raw sector writes across partition boundaries.
	var lockedVols []syscall.Handle
	for _, letter := range getDriveLettersForPhysicalDisk(cleanPath) {
		if hVol, lErr := lockAndDismountWindowsVolume(letter); lErr == nil && hVol != syscall.InvalidHandle {
			lockedVols = append(lockedVols, hVol)
		}
	}

	pathPtr, err := syscall.UTF16PtrFromString(cleanPath)
	if err != nil {
		for _, h := range lockedVols {
			_ = syscall.CloseHandle(h)
		}
		return nil, err
	}

	hDisk, err := syscall.CreateFile(
		pathPtr,
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE,
		nil,
		syscall.OPEN_EXISTING,
		winFileFlagNoBuffering|winFileFlagWriteThrough,
		0,
	)
	if err != nil {
		for _, h := range lockedVols {
			_ = syscall.CloseHandle(h)
		}
		return nil, fmt.Errorf("IS-USB-301: CreateFile failed on %s (ensure UAC Administrator elevation): %w", cleanPath, err)
	}

	var bytesReturned uint32
	_ = syscall.DeviceIoControl(hDisk, fsctlAllowExtendedDasdIO, nil, 0, nil, 0, &bytesReturned, nil)
	_ = syscall.DeviceIoControl(hDisk, ioctlDeleteDriveLayout, nil, 0, nil, 0, &bytesReturned, nil)
	_ = syscall.DeviceIoControl(hDisk, ioctlDiskUpdateProps, nil, 0, nil, 0, &bytesReturned, nil)

	f := os.NewFile(uintptr(hDisk), cleanPath)
	return &windowsRawDisk{
		file:          f,
		handle:        hDisk,
		devicePath:    cleanPath,
		lockedVolumes: lockedVols,
		unbuffered:    true,
	}, nil
}

func (d *windowsRawDisk) Read(p []byte) (int, error) {
	return d.file.Read(p)
}

func (d *windowsRawDisk) Write(p []byte) (int, error) {
	return d.file.Write(p)
}

func (d *windowsRawDisk) ReadAt(p []byte, off int64) (int, error) {
	return d.file.ReadAt(p, off)
}

func (d *windowsRawDisk) WriteAt(p []byte, off int64) (int, error) {
	if _, err := d.file.Seek(off, 0); err != nil {
		return 0, err
	}
	var written uint32
	err := syscall.WriteFile(d.handle, p, &written, (*syscall.Overlapped)(unsafe.Pointer(nil)))
	return int(written), err
}

func (d *windowsRawDisk) Seek(offset int64, whence int) (int64, error) {
	return d.file.Seek(offset, whence)
}

func (d *windowsRawDisk) SyncAndFlush() error {
	var bytesReturned uint32
	if d.unbuffered {
		_ = syscall.DeviceIoControl(d.handle, ioctlDiskUpdateProps, nil, 0, nil, 0, &bytesReturned, nil)
	}
	return syscall.FlushFileBuffers(d.handle)
}

func (d *windowsRawDisk) Close() error {
	_ = d.SyncAndFlush()
	var bytesReturned uint32
	for _, hVol := range d.lockedVolumes {
		_ = syscall.DeviceIoControl(hVol, fsctlUnlockVolume, nil, 0, nil, 0, &bytesReturned, nil)
		_ = syscall.CloseHandle(hVol)
	}
	d.lockedVolumes = nil
	return d.file.Close()
}

func (d *windowsRawDisk) DevicePath() string {
	return d.devicePath
}

func (d *windowsRawDisk) RequiresPageAlignedTail() bool {
	return d.unbuffered
}
