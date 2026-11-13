//go:build windows

// usb/flasher_windows.go
package usb

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

const (
	winFileFlagNoBuffering   = 0x20000000
	winFileFlagWriteThrough  = 0x80000000
	winCreateNoWindow        = 0x08000000
	fsctlLockVolume          = 0x00090018
	fsctlDismountVolume      = 0x00090020
	fsctlUnlockVolume        = 0x0009001C
	fsctlAllowExtendedDasdIO = 0x00090083
	ioctlDiskUpdateProps     = 0x00070140
	ioctlDiskGetLengthInfo   = 0x0007405C
	ioctlDiskCreateDisk      = 0x0007C058
	ioctlDiskSetAttributes   = 0x0007C0F4

	winMaxUSBTransferChunk = 1024 * 1024

	esContinuous       = 0x80000000
	esSystemRequired   = 0x00000001
	esDisplayRequired  = 0x00000002
	esAwayModeRequired = 0x00000040
)

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

func parsePhysicalDriveNumber(physicalDrivePath string) string {
	idx := strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(physicalDrivePath)), `\\.\PHYSICALDRIVE`)
	if idx == "" {
		return ""
	}
	for _, ch := range idx {
		if ch < '0' || ch > '9' {
			return ""
		}
	}
	// Never allow disk 0 (primary boot drive) in automated diskpart/raw reset helpers
	if idx == "0" {
		return ""
	}
	return idx
}

func getDriveLettersForPhysicalDisk(physicalDrivePath string) []string {
	idx := parsePhysicalDriveNumber(physicalDrivePath)
	if idx == "" {
		return nil
	}
	ps := fmt.Sprintf("Get-Partition -DiskNumber %s -ErrorAction SilentlyContinue | Where-Object { $_.DriveLetter } | ForEach-Object { $_.DriveLetter }", idx)
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", ps)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: winCreateNoWindow,
	}
	out, err := cmd.Output()
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

func clearReadOnlyAndResetRawDiskIOCTL(hDisk syscall.Handle) {
	var bytesReturned uint32
	// SET_DISK_ATTRIBUTES (40 bytes):
	// Offset 0:  Version = 40
	// Offset 4:  Persist = 1
	// Offset 8:  Attributes = 0 (clear OFFLINE=0x01 and READ_ONLY=0x02)
	// Offset 16: AttributesMask = 0x03 (DISK_ATTRIBUTE_OFFLINE | DISK_ATTRIBUTE_READ_ONLY)
	var setAttr [40]byte
	binary.LittleEndian.PutUint32(setAttr[0:4], 40)
	setAttr[4] = 1
	binary.LittleEndian.PutUint64(setAttr[8:16], 0)
	binary.LittleEndian.PutUint64(setAttr[16:24], 0x03)
	_ = syscall.DeviceIoControl(
		hDisk,
		ioctlDiskSetAttributes,
		&setAttr[0],
		uint32(len(setAttr)),
		nil,
		0,
		&bytesReturned,
		nil,
	)

	// CREATE_DISK (24 bytes) with PARTITION_STYLE_RAW (0) drops read-only hybrid ISO9660/GPT locks in partmgr.sys
	var createRaw [24]byte
	_ = syscall.DeviceIoControl(
		hDisk,
		ioctlDiskCreateDisk,
		&createRaw[0],
		uint32(len(createRaw)),
		nil,
		0,
		&bytesReturned,
		nil,
	)
	_ = syscall.DeviceIoControl(hDisk, ioctlDiskUpdateProps, nil, 0, nil, 0, &bytesReturned, nil)
}

// cleanAndUnlockWindowsPhysicalDisk clears the Windows read-only disk flag and removes
// stale hybrid ISO9660/GPT partition locks (equivalent to running diskpart -> select disk N ->
// attributes disk clear readonly -> clean) without flashing any console window.
func cleanAndUnlockWindowsPhysicalDisk(physicalDrivePath string) {
	idx := parsePhysicalDriveNumber(physicalDrivePath)
	if idx == "" {
		return
	}

	if pathPtr, err := syscall.UTF16PtrFromString(physicalDrivePath); err == nil {
		if hPre, oErr := syscall.CreateFile(
			pathPtr,
			syscall.GENERIC_READ|syscall.GENERIC_WRITE,
			syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE,
			nil,
			syscall.OPEN_EXISTING,
			0,
			0,
		); oErr == nil {
			clearReadOnlyAndResetRawDiskIOCTL(hPre)
			_ = syscall.CloseHandle(hPre)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	script := fmt.Sprintf("select disk %s\r\nattributes disk clear readonly noerr\r\nonline disk noerr\r\nclean\r\nrescan\r\nexit\r\n", idx)
	cmd := exec.CommandContext(ctx, "diskpart.exe")
	cmd.Stdin = bytes.NewBufferString(script)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: winCreateNoWindow,
	}
	_ = cmd.Run()
	time.Sleep(150 * time.Millisecond)
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
			return nil, fmt.Errorf("IS-USB-301: no physical USB drive found at %s (plug in a removable USB flash drive and click Refresh): %w", cleanPath, err)
		}
		return &windowsRawDisk{
			file:       f,
			handle:     syscall.Handle(f.Fd()),
			devicePath: cleanPath,
			unbuffered: false,
		}, nil
	}

	var lockedVols []syscall.Handle
	for _, letter := range getDriveLettersForPhysicalDisk(cleanPath) {
		if hVol, lErr := lockAndDismountWindowsVolume(letter); lErr == nil && hVol != syscall.InvalidHandle {
			lockedVols = append(lockedVols, hVol)
		}
	}

	// Automatically clear read-only flags and stale hybrid ISO9660/GPT partition locks before opening
	cleanAndUnlockWindowsPhysicalDisk(cleanPath)

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
		return nil, fmt.Errorf("IS-USB-301: CreateFile failed on %s (ensure Administrator elevation): %w", cleanPath, err)
	}

	clearReadOnlyAndResetRawDiskIOCTL(hDisk)
	var bytesReturned uint32
	_ = syscall.DeviceIoControl(hDisk, fsctlAllowExtendedDasdIO, nil, 0, nil, 0, &bytesReturned, nil)

	// Zero out the first 64 KB (including LBA 0 MBR, LBA 1 GPT, and offset 32768 ISO9660 CD001 superblock)
	// so Windows MountMgr never treats the open drive as a write-protected CD-ROM.
	zero64K := AllocPageAlignedBuffer(64*1024, SectorAlignmentBytes)
	ovZero := syscall.Overlapped{}
	var nZero uint32
	_ = syscall.WriteFile(hDisk, zero64K, &nZero, &ovZero)
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
	if !d.unbuffered {
		return d.file.ReadAt(p, off)
	}
	totalRead := 0
	for totalRead < len(p) {
		rem := len(p) - totalRead
		chunkLen := rem
		if chunkLen > winMaxUSBTransferChunk {
			chunkLen = winMaxUSBTransferChunk
		}
		curOff := uint64(off) + uint64(totalRead)
		ov := syscall.Overlapped{
			Offset:     uint32(curOff & 0xFFFFFFFF),
			OffsetHigh: uint32(curOff >> 32),
		}
		var n uint32
		err := syscall.ReadFile(d.handle, p[totalRead:totalRead+chunkLen], &n, &ov)
		totalRead += int(n)
		if err != nil {
			return totalRead, err
		}
		if n == 0 {
			break
		}
	}
	return totalRead, nil
}

func (d *windowsRawDisk) WriteAt(p []byte, off int64) (int, error) {
	if !d.unbuffered {
		return d.file.WriteAt(p, off)
	}
	totalWritten := 0
	for totalWritten < len(p) {
		rem := len(p) - totalWritten
		chunkLen := rem
		if chunkLen > winMaxUSBTransferChunk {
			chunkLen = winMaxUSBTransferChunk
		}
		curOff := uint64(off) + uint64(totalWritten)
		ov := syscall.Overlapped{
			Offset:     uint32(curOff & 0xFFFFFFFF),
			OffsetHigh: uint32(curOff >> 32),
		}
		var n uint32
		err := syscall.WriteFile(d.handle, p[totalWritten:totalWritten+chunkLen], &n, &ov)
		if err != nil {
			if pathPtr, pErr := syscall.UTF16PtrFromString(d.devicePath); pErr == nil {
				if hBuf, oErr := syscall.CreateFile(pathPtr, syscall.GENERIC_READ|syscall.GENERIC_WRITE, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE, nil, syscall.OPEN_EXISTING, winFileFlagWriteThrough, 0); oErr == nil {
					var br uint32
					_ = syscall.DeviceIoControl(hBuf, fsctlAllowExtendedDasdIO, nil, 0, nil, 0, &br, nil)
					err2 := syscall.WriteFile(hBuf, p[totalWritten:totalWritten+chunkLen], &n, &ov)
					_ = syscall.FlushFileBuffers(hBuf)
					_ = syscall.CloseHandle(hBuf)
					if err2 == nil {
						err = nil
					}
				}
			}
		}
		totalWritten += int(n)
		if err != nil {
			return totalWritten, err
		}
		if n == 0 {
			break
		}
	}
	return totalWritten, nil
}

func (d *windowsRawDisk) Seek(offset int64, whence int) (int64, error) {
	if d.unbuffered && whence == io.SeekEnd {
		var lengthBuf [8]byte
		var bytesReturned uint32
		err := syscall.DeviceIoControl(
			d.handle,
			ioctlDiskGetLengthInfo,
			nil,
			0,
			&lengthBuf[0],
			uint32(len(lengthBuf)),
			&bytesReturned,
			nil,
		)
		if err == nil {
			diskBytes := int64(binary.LittleEndian.Uint64(lengthBuf[:]))
			return diskBytes + offset, nil
		}
	}
	return d.file.Seek(offset, whence)
}

func (d *windowsRawDisk) SyncAndFlush() error {
	return syscall.FlushFileBuffers(d.handle)
}

func (d *windowsRawDisk) Close() error {
	_ = d.SyncAndFlush()
	var bytesReturned uint32
	_ = syscall.DeviceIoControl(d.handle, ioctlDiskUpdateProps, nil, 0, nil, 0, &bytesReturned, nil)
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
