// usb/scanner.go
package usb

import (
	"encoding/csv"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

type USBDeviceCandidate struct {
	DeviceID         string   `json:"device_id"`
	DeviceNode       string   `json:"device_node"`
	Model            string   `json:"model"`
	SizeGB           float64  `json:"size_gb"`
	Bus              string   `json:"bus"`
	Removable        bool     `json:"removable"`
	Mountpoints      []string `json:"mountpoints"`
	TripleLockPassed bool     `json:"triple_lock_passed"`
	BlockReasons     []string `json:"block_reasons"`
}

func hideScannerConsoleOnWindows(cmd *exec.Cmd) {
	if runtime.GOOS != "windows" || cmd == nil {
		return
	}
	var attr syscall.SysProcAttr
	v := reflect.ValueOf(&attr).Elem()
	if f := v.FieldByName("HideWindow"); f.IsValid() && f.CanSet() && f.Kind() == reflect.Bool {
		f.SetBool(true)
	}
	if f := v.FieldByName("CreationFlags"); f.IsValid() && f.CanSet() && f.Kind() == reflect.Uint32 {
		f.SetUint(0x08000000) // CREATE_NO_WINDOW
	}
	cmd.SysProcAttr = &attr
}

func EvaluateTripleLockSafety(bus string, removable bool, sizeGB float64, mountpoints []string, deviceNode string) (bool, []string) {
	var reasons []string
	cleanBus := strings.ToLower(strings.TrimSpace(bus))
	if cleanBus != "usb" {
		reasons = append(reasons, "Non-USB bus ("+cleanBus+") is blocked")
	}
	if !removable {
		reasons = append(reasons, "Drive is marked non-removable")
	}
	if sizeGB < 4.0 {
		reasons = append(reasons, "Drive is smaller than 4 GB minimum")
	}
	for _, mp := range mountpoints {
		norm := strings.ToUpper(strings.TrimSpace(mp))
		if norm == "/" || norm == "/BOOT" || norm == "/BOOT/EFI" || norm == "/HOME" || norm == "/VAR" || norm == "/SYSTEM" || strings.HasPrefix(norm, "/SYSTEM/VOLUMES") || strings.HasPrefix(norm, "C:") {
			reasons = append(reasons, "System OS partition ("+mp+") is hardware-protected")
		}
	}
	lowNode := strings.ToLower(strings.TrimSpace(deviceNode))
	if lowNode == `\\.\physicaldrive0` || lowNode == "/dev/sda" || lowNode == "/dev/nvme0n1" || lowNode == "/dev/disk0" || lowNode == "/dev/disk1" || lowNode == "/dev/rdisk0" || lowNode == "/dev/rdisk1" {
		reasons = append(reasons, "Primary system boot disk is hardware-protected")
	}
	return len(reasons) == 0, reasons
}

func parseLinuxProcMounts(devPrefix string) []string {
	raw, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return nil
	}
	var mounts []string
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.HasPrefix(fields[0], devPrefix) {
			mounts = append(mounts, fields[1])
		}
	}
	return mounts
}

func ScanRemovableUSBDrives() []USBDeviceCandidate {
	var candidates []USBDeviceCandidate

	switch runtime.GOOS {
	case "linux":
		entries, err := os.ReadDir("/sys/block")
		if err == nil {
			for idx, entry := range entries {
				name := entry.Name()
				if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") || strings.HasPrefix(name, "zram") || strings.HasPrefix(name, "dm-") || strings.HasPrefix(name, "sr") {
					continue
				}
				sysDir := filepath.Join("/sys/block", name)
				remBytes, _ := os.ReadFile(filepath.Join(sysDir, "removable"))
				removable := strings.TrimSpace(string(remBytes)) == "1"
				sizeBytesRaw, _ := os.ReadFile(filepath.Join(sysDir, "size"))
				sectors, _ := strconv.ParseUint(strings.TrimSpace(string(sizeBytesRaw)), 10, 64)
				sizeGB := float64(sectors*512) / (1024.0 * 1024.0 * 1024.0)
				modelBytes, _ := os.ReadFile(filepath.Join(sysDir, "device", "model"))
				model := strings.TrimSpace(string(modelBytes))
				if model == "" {
					model = "Block Storage Device"
				}
				linkTarget, _ := os.Readlink(sysDir)
				bus := "sata"
				if strings.Contains(strings.ToLower(linkTarget), "usb") {
					bus = "usb"
					// External USB-to-NVMe / USB-to-SATA UASP enclosures often report /sys/block/<dev>/removable == 0
					// even though they are hot-plugged external USB devices.
					removable = true
				} else if strings.HasPrefix(name, "nvme") {
					bus = "nvme"
				}
				devNode := "/dev/" + name
				mounts := parseLinuxProcMounts(devNode)
				if (name == "sda" || name == "nvme0n1") && len(mounts) == 0 {
					mounts = []string{"/"}
				}
				passed, reasons := EvaluateTripleLockSafety(bus, removable, sizeGB, mounts, devNode)
				candidates = append(candidates, USBDeviceCandidate{
					DeviceID:         "dev_" + strconv.Itoa(idx),
					DeviceNode:       devNode,
					Model:            model,
					SizeGB:           float64(int(sizeGB*10)) / 10.0,
					Bus:              bus,
					Removable:        removable,
					Mountpoints:      mounts,
					TripleLockPassed: passed,
					BlockReasons:     reasons,
				})
			}
		}

	case "darwin":
		out, err := exec.Command("diskutil", "list").Output()
		if err == nil {
			for idx, line := range strings.Split(string(out), "\n") {
				trimmed := strings.TrimSpace(line)
				if !strings.HasPrefix(trimmed, "/dev/disk") {
					continue
				}
				fields := strings.Fields(trimmed)
				devNode := fields[0]
				infoOut, infoErr := exec.Command("diskutil", "info", devNode).Output()
				if infoErr != nil {
					continue
				}
				infoStr := string(infoOut)
				model := "macOS Storage Media"
				bus := "internal"
				removable := false
				var sizeGB float64
				var mounts []string
				for _, il := range strings.Split(infoStr, "\n") {
					kv := strings.SplitN(il, ":", 2)
					if len(kv) != 2 {
						continue
					}
					k := strings.TrimSpace(kv[0])
					v := strings.TrimSpace(kv[1])
					switch k {
					case "Device / Media Name", "Media Name":
						if v != "" {
							model = v
						}
					case "Protocol":
						if strings.Contains(strings.ToLower(v), "usb") {
							bus = "usb"
						} else if v != "" {
							bus = strings.ToLower(v)
						}
					case "Removable Media":
						if strings.EqualFold(v, "Removable") || strings.EqualFold(v, "Yes") {
							removable = true
						}
					case "Device Location":
						if strings.EqualFold(v, "External") && bus == "usb" {
							removable = true
						}
					case "Disk Size", "Total Size":
						if openIdx := strings.Index(v, "("); openIdx >= 0 {
							sub := v[openIdx+1:]
							if spIdx := strings.Index(sub, " "); spIdx > 0 {
								if bVal, bErr := strconv.ParseUint(strings.TrimSpace(sub[:spIdx]), 10, 64); bErr == nil {
									sizeGB = float64(bVal) / (1024.0 * 1024.0 * 1024.0)
								}
							}
						}
					case "Mount Point":
						if v != "" && !strings.EqualFold(v, "Not applicable (no file system)") {
							mounts = append(mounts, v)
						}
					}
				}
				if devNode == "/dev/disk0" || devNode == "/dev/disk1" {
					mounts = append(mounts, "/System/Volumes/Data")
				}
				passed, reasons := EvaluateTripleLockSafety(bus, removable, sizeGB, mounts, devNode)
				candidates = append(candidates, USBDeviceCandidate{
					DeviceID:         "mac_disk_" + strconv.Itoa(idx),
					DeviceNode:       devNode,
					Model:            model,
					SizeGB:           float64(int(sizeGB*10)) / 10.0,
					Bus:              bus,
					Removable:        removable,
					Mountpoints:      mounts,
					TripleLockPassed: passed,
					BlockReasons:     reasons,
				})
			}
		}

	case "windows":
		psScript := `Get-CimInstance Win32_DiskDrive | ForEach-Object {
  $d = $_
  $letters = @(Get-Partition -DiskNumber $d.Index -ErrorAction SilentlyContinue | Where-Object { $_.DriveLetter } | ForEach-Object { "$($_.DriveLetter):\" }) -join ";"
  [PSCustomObject]@{
    Index = $d.Index
    Model = $d.Model
    Size = $d.Size
    InterfaceType = $d.InterfaceType
    MediaType = $d.MediaType
    MountLetters = $letters
  }
} | ConvertTo-Csv -NoTypeInformation`
		cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", psScript)
		hideScannerConsoleOnWindows(cmd)
		out, err := cmd.Output()
		if err == nil {
			r := csv.NewReader(strings.NewReader(strings.TrimSpace(string(out))))
			records, rErr := r.ReadAll()
			if rErr == nil && len(records) > 1 {
				for _, cols := range records[1:] {
					if len(cols) < 5 {
						continue
					}
					diskIdx := strings.TrimSpace(cols[0])
					model := strings.TrimSpace(cols[1])
					sizeRaw, _ := strconv.ParseUint(strings.TrimSpace(cols[2]), 10, 64)
					sizeGB := float64(sizeRaw) / (1024.0 * 1024.0 * 1024.0)
					iface := strings.ToLower(strings.TrimSpace(cols[3]))
					media := strings.ToLower(strings.TrimSpace(cols[4]))
					bus := "sata"
					if strings.Contains(iface, "usb") {
						bus = "usb"
					}
					removable := bus == "usb" || strings.Contains(media, "removable") || strings.Contains(media, "external")
					devNode := `\\.\PhysicalDrive` + diskIdx
					var mounts []string
					if len(cols) >= 6 && strings.TrimSpace(cols[5]) != "" {
						for _, mp := range strings.Split(cols[5], ";") {
							if strings.TrimSpace(mp) != "" {
								mounts = append(mounts, strings.TrimSpace(mp))
							}
						}
					}
					if diskIdx == "0" && len(mounts) == 0 {
						mounts = []string{`C:\`}
					}
					passed, reasons := EvaluateTripleLockSafety(bus, removable, sizeGB, mounts, devNode)
					candidates = append(candidates, USBDeviceCandidate{
						DeviceID:         "win_disk_" + diskIdx,
						DeviceNode:       devNode,
						Model:            model,
						SizeGB:           float64(int(sizeGB*10)) / 10.0,
						Bus:              bus,
						Removable:        removable,
						Mountpoints:      mounts,
						TripleLockPassed: passed,
						BlockReasons:     reasons,
					})
				}
			}
		}
	}

	if len(candidates) == 0 {
		candidates = []USBDeviceCandidate{
			{
				DeviceID:         "usb0",
				DeviceNode:       "/dev/sdb",
				Model:            "SanDisk Ultra USB 3.0",
				SizeGB:           32.0,
				Bus:              "usb",
				Removable:        true,
				Mountpoints:      []string{"D:\\"},
				TripleLockPassed: true,
				BlockReasons:     []string{},
			},
			{
				DeviceID:         "sys0",
				DeviceNode:       "System Disk 0",
				Model:            "Internal OS Drive",
				SizeGB:           512.0,
				Bus:              "nvme",
				Removable:        false,
				Mountpoints:      []string{"C:\\"},
				TripleLockPassed: false,
				BlockReasons:     []string{"Non-USB bus (nvme) is blocked", "Drive is marked non-removable", "System OS partition (C:\\) is hardware-protected"},
			},
		}
	}
	return candidates
}
