//go:build linux

// usb/flasher_linux.go
package usb

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

const (
	linuxBLKFLSBUF = 0x1261
	linuxBLKRRPART = 0x125f
)

// AcquireHostWakeLock inhibits systemd sleep/idle/lid-switch while an ISO download
// or raw USB DMA flash operation is running.
func AcquireHostWakeLock() func() {
	cmd := exec.Command("systemd-inhibit", "--what=idle:sleep:handle-lid-switch", "--who=InstantServer", "--why=Flashing bootable USB", "sleep", "7200")
	if err := cmd.Start(); err == nil && cmd.Process != nil {
		return func() {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
	}
	return func() {}
}

type linuxRawDisk struct {
	file       *os.File
	devicePath string
	directIO   bool
}

func unmountLinuxPartitions(devicePath string) {
	matches, err := filepath.Glob(devicePath + "*")
	if err != nil {
		return
	}
	for _, part := range matches {
		if part == devicePath {
			continue
		}
		_ = exec.Command("udisksctl", "unmount", "-b", part, "--no-user-interaction").Run()
		_ = syscall.Unmount(part, syscall.MNT_DETACH)
		_ = exec.Command("umount", "-l", part).Run()
	}
}

func OpenRawDisk(devicePath string) (RawDiskHandle, error) {
	cleanPath := strings.TrimSpace(devicePath)
	if strings.HasPrefix(cleanPath, "/dev/") {
		unmountLinuxPartitions(cleanPath)
	}

	// Open with O_DIRECT | O_EXCL and WITHOUT O_SYNC so 4MB page-aligned DMA chunks
	// bypass the kernel page cache without stalling the USB flash microcontroller on every write.
	flags := os.O_RDWR | syscall.O_DIRECT
	if strings.HasPrefix(cleanPath, "/dev/") {
		flags |= syscall.O_EXCL
	}

	f, err := os.OpenFile(cleanPath, flags, 0600)
	if err != nil {
		// Retry O_DIRECT without O_EXCL in case udisks2/udev briefly holds a probe reference
		if fDirect, dErr := os.OpenFile(cleanPath, os.O_RDWR|syscall.O_DIRECT, 0600); dErr == nil {
			return &linuxRawDisk{file: fDirect, devicePath: cleanPath, directIO: true}, nil
		}
		// Fallback for tmpfs/loop test filesystems that do not support O_DIRECT
		fFallback, fbErr := os.OpenFile(cleanPath, os.O_RDWR, 0600)
		if fbErr != nil {
			return nil, fmt.Errorf("IS-USB-301: failed to open Linux block device %s (check Polkit/root privileges): %w", cleanPath, err)
		}
		return &linuxRawDisk{file: fFallback, devicePath: cleanPath, directIO: false}, nil
	}
	return &linuxRawDisk{file: f, devicePath: cleanPath, directIO: true}, nil
}

func (d *linuxRawDisk) Read(p []byte) (int, error) {
	return d.file.Read(p)
}

func (d *linuxRawDisk) Write(p []byte) (int, error) {
	return d.file.Write(p)
}

func (d *linuxRawDisk) ReadAt(p []byte, off int64) (int, error) {
	return d.file.ReadAt(p, off)
}

func (d *linuxRawDisk) WriteAt(p []byte, off int64) (int, error) {
	return d.file.WriteAt(p, off)
}

func (d *linuxRawDisk) Seek(offset int64, whence int) (int64, error) {
	return d.file.Seek(offset, whence)
}

func (d *linuxRawDisk) SyncAndFlush() error {
	if err := syscall.Fsync(int(d.file.Fd())); err != nil {
		return err
	}
	if strings.HasPrefix(d.devicePath, "/dev/") {
		_, _, _ = syscall.Syscall(syscall.SYS_IOCTL, d.file.Fd(), uintptr(linuxBLKFLSBUF), 0)
		_, _, _ = syscall.Syscall(syscall.SYS_IOCTL, d.file.Fd(), uintptr(linuxBLKRRPART), 0)
	}
	return nil
}

func (d *linuxRawDisk) Close() error {
	_ = d.SyncAndFlush()
	return d.file.Close()
}

func (d *linuxRawDisk) DevicePath() string {
	return d.devicePath
}

func (d *linuxRawDisk) RequiresPageAlignedTail() bool {
	return d.directIO
}
