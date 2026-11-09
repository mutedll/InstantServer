//go:build darwin

// usb/flasher_darwin.go
package usb

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// AcquireHostWakeLock runs caffeinate -dims bound to the current process PID so macOS
// never sleeps display, system, or disk during ISO downloading or USB flashing.
func AcquireHostWakeLock() func() {
	cmd := exec.Command("caffeinate", "-dims", "-w", fmt.Sprintf("%d", os.Getpid()))
	if err := cmd.Start(); err == nil && cmd.Process != nil {
		return func() {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
	}
	return func() {}
}

type darwinRawDisk struct {
	file        *os.File
	rawCharPath string
}

func toMacOSRawCharacterDevice(devicePath string) string {
	clean := strings.TrimSpace(devicePath)
	if strings.HasPrefix(clean, "/dev/disk") && !strings.HasPrefix(clean, "/dev/rdisk") {
		return strings.Replace(clean, "/dev/disk", "/dev/rdisk", 1)
	}
	return clean
}

func unlockMacOSDiskArbitration(devicePath string) error {
	baseDisk := strings.Replace(strings.TrimSpace(devicePath), "/dev/rdisk", "/dev/disk", 1)
	if strings.HasPrefix(baseDisk, "/dev/disk") {
		cmd := exec.Command("diskutil", "unmountDisk", "force", baseDisk)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("IS-USB-306: diskutil unmountDisk force failed on %s (%s): %w", baseDisk, strings.TrimSpace(string(out)), err)
		}
		time.Sleep(150 * time.Millisecond)
	}
	return nil
}

func OpenRawDisk(devicePath string) (RawDiskHandle, error) {
	if err := unlockMacOSDiskArbitration(devicePath); err != nil {
		return nil, err
	}

	rawPath := toMacOSRawCharacterDevice(devicePath)
	flags := os.O_RDWR
	if strings.HasPrefix(rawPath, "/dev/rdisk") {
		flags |= syscall.O_EXCL
	}

	var f *os.File
	var err error
	for attempt := 0; attempt < 5; attempt++ {
		f, err = os.OpenFile(rawPath, flags, 0600)
		if err == nil {
			break
		}
		if strings.HasPrefix(rawPath, "/dev/rdisk") {
			_ = unlockMacOSDiskArbitration(devicePath)
			time.Sleep(time.Duration(120*(attempt+1)) * time.Millisecond)
		} else {
			break
		}
	}
	if err != nil && strings.HasPrefix(rawPath, "/dev/rdisk") {
		f, err = os.OpenFile(rawPath, os.O_RDWR, 0600)
	}
	if err != nil {
		return nil, fmt.Errorf("IS-USB-301: failed to open macOS raw character device %s: %w", rawPath, err)
	}

	// Bypass XNU unified buffer cache via F_NOCACHE on the raw character descriptor
	_, _, errno := syscall.Syscall(syscall.SYS_FCNTL, f.Fd(), uintptr(syscall.F_NOCACHE), 1)
	if errno != 0 && strings.HasPrefix(rawPath, "/dev/rdisk") {
		_ = f.Close()
		return nil, fmt.Errorf("IS-USB-307: F_NOCACHE fcntl failed on %s: %v", rawPath, errno)
	}

	return &darwinRawDisk{
		file:        f,
		rawCharPath: rawPath,
	}, nil
}

func (d *darwinRawDisk) Read(p []byte) (int, error) {
	return d.file.Read(p)
}

func (d *darwinRawDisk) Write(p []byte) (int, error) {
	return d.file.Write(p)
}

func (d *darwinRawDisk) ReadAt(p []byte, off int64) (int, error) {
	return d.file.ReadAt(p, off)
}

func (d *darwinRawDisk) WriteAt(p []byte, off int64) (int, error) {
	return d.file.WriteAt(p, off)
}

func (d *darwinRawDisk) Seek(offset int64, whence int) (int64, error) {
	return d.file.Seek(offset, whence)
}

func (d *darwinRawDisk) SyncAndFlush() error {
	return syscall.Fsync(int(d.file.Fd()))
}

func (d *darwinRawDisk) Close() error {
	_ = d.SyncAndFlush()
	return d.file.Close()
}

func (d *darwinRawDisk) DevicePath() string {
	return d.rawCharPath
}

func (d *darwinRawDisk) RequiresPageAlignedTail() bool {
	return strings.HasPrefix(d.rawCharPath, "/dev/rdisk")
}
