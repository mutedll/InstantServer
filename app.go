// ============================================================================
// FILE: app.go
// PURPOSE: Wails Backend Controller — bridges the HTML/JS UI (`app_studio.js`)
//
//	with the Go systems packages (`usb`, `network`, `privilege`).
//
// HOW WAILS IPC WORKS (For Beginners):
//   - Every exported method on `func (a *App) MethodName(...)` automatically
//     becomes callable from JavaScript as `window.go.main.App.MethodName(...)`
//     AND over HTTP POST at `/api/<kebab-case-route>` in `AssetAndAPIRouter()`.
//
// KEY METHODS IN THIS FILE:
//   - `ScanUSBDevices()`     -> Lists safe USB drives and mounted VHD/VHDX disks.
//   - `FlashUSBDevice()`     ->Runs the 5-stage pipeline:
//     1. Validate config & Triple-Lock safety
//     2. Stage CIDATA autoinstall payload
//     3. Download/verify Ubuntu Live Server ISO
//     4. Stream ISO + CIDATA + INSTANTUSB to raw USB
//     5. Sync hardware buffers & emit 100% Ready
//   - `ResetUSBDrive()`      -> Formats a flashed USB stick back to clean FAT32.
//   - `BootUSBNextRestart()` -> Configures UEFI BootNext / BCD so the PC boots
//     straight into the USB on next restart.
//
// HOW TO ADD A NEW BACKEND ACTION FOR THE UI:
//  1. Write `func (a *App) MyNewAction(...) map[string]interface{}` below.
//  2. Call `invokeBackend("MyNewAction", ...)` from `app_studio.js`.
//
// ============================================================================
// app.go
package main

import (
	"context"
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/mutedll/InstantServer/privilege"
	"github.com/mutedll/InstantServer/usb"
	"github.com/mutedll/InstantServer/usb/payload"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx            context.Context
	assets         embed.FS
	mu             sync.Mutex
	flashing       bool
	forceClosing   bool
	flashStartTime time.Time
	cancelFunc     context.CancelFunc
	pauseGate      *usb.PauseGate
	lastProfile    usb.ServerProfileConfig
	lastISOPath    string
	lastStatus     map[string]interface{}
}

func NewApp(assets embed.FS) *App {
	payload.SetEmbeddedAssets(assets)
	return &App{
		assets:    assets,
		pauseGate: usb.NewPauseGate(),
		lastStatus: map[string]interface{}{
			"status":      "IDLE",
			"percent":     0,
			"stage_index": 0,
			"paused":      false,
			"events":      []map[string]interface{}{},
		},
	}
}

func resolveWritableDataDir() string {
	candidates := []string{
		filepath.Join(".", "Instant_Server_Data"),
	}
	if exePath, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exePath), "Instant_Server_Data"))
	}
	if cfgDir, err := os.UserConfigDir(); err == nil {
		candidates = append(candidates, filepath.Join(cfgDir, "InstantServer", "Instant_Server_Data"))
	}
	candidates = append(candidates, filepath.Join(os.TempDir(), "Instant_Server_Data"))

	for _, dir := range candidates {
		if err := os.MkdirAll(dir, 0755); err != nil {
			continue
		}
		probe := filepath.Join(dir, ".write_probe")
		if err := os.WriteFile(probe, []byte("ok"), 0600); err == nil {
			_ = os.Remove(probe)
			return dir
		}
	}
	return filepath.Join(os.TempDir(), "Instant_Server_Data")
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	wailsRuntime.WindowCenter(ctx)
}

func (a *App) FitWindowToScreen(availW int, availH int) map[string]interface{} {
	if a.ctx == nil || availW <= 0 || availH <= 0 {
		return map[string]interface{}{"ok": false}
	}
	targetW := int(float64(availW) * 0.86)
	if targetW > 1120 {
		targetW = 1120
	}
	if targetW > availW-40 {
		targetW = availW - 40
	}
	if targetW < 720 {
		targetW = 720
	}

	targetH := int(float64(availH) * 0.84)
	if targetH > 720 {
		targetH = 720
	}
	if targetH > availH-52 {
		targetH = availH - 52
	}
	if targetH < 460 {
		targetH = 460
	}

	wailsRuntime.WindowSetSize(a.ctx, targetW, targetH)
	wailsRuntime.WindowCenter(a.ctx)
	return map[string]interface{}{
		"ok":     true,
		"width":  targetW,
		"height": targetH,
	}
}

func (a *App) ToggleMaximizeWindow() map[string]interface{} {
	if a.ctx != nil {
		wailsRuntime.WindowToggleMaximise(a.ctx)
	}
	return map[string]interface{}{"ok": true}
}

func (a *App) IsFlashing() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.flashing
}

func (a *App) IsForceClosing() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.forceClosing
}

func (a *App) GracefulShutdownSupervisor() {
	a.mu.Lock()
	a.forceClosing = true
	cancel := a.cancelFunc
	a.pauseGate.Resume()
	a.mu.Unlock()
	if cancel != nil {
		cancel()
		time.Sleep(120 * time.Millisecond)
	}
}

func (a *App) ProbeHost() map[string]interface{} {
	return map[string]interface{}{
		"host_os":          runtime.GOOS,
		"arch":             runtime.GOARCH,
		"is_admin_or_root": privilege.IsProcessElevated(),
		"engine":           "wails_go_native_dma",
	}
}

func (a *App) ElevateAdmin() privilege.EscalationResult {
	res := privilege.RequestNativePrivilegeEscalation(nil)
	if res.ElevatedSpawned {
		a.mu.Lock()
		a.forceClosing = true
		a.mu.Unlock()
		go func() {
			time.Sleep(250 * time.Millisecond)
			if a.ctx != nil {
				wailsRuntime.Quit(a.ctx)
			}
			os.Exit(0)
		}()
	}
	return res
}

func (a *App) ScanUSBDevices() []usb.USBDeviceCandidate {
	return usb.ScanRemovableUSBDrives()
}

// ResetUSBDrive clears write-protection / hybrid ISO9660 partition locks on a USB drive
// and formats 100% of its capacity as a clean FAT32 volume.
func (a *App) ResetUSBDrive(devicePath string) map[string]interface{} {
	cleanDev := strings.TrimSpace(devicePath)
	if cleanDev == "" {
		return map[string]interface{}{
			"ok":    false,
			"error": "Select a removable USB drive first.",
		}
	}

	a.mu.Lock()
	if a.flashing {
		a.mu.Unlock()
		return map[string]interface{}{
			"ok":    false,
			"error": "Cannot reset USB drive while flashing is in progress.",
		}
	}
	a.mu.Unlock()

	passed, reasons := usb.EvaluateTripleLockSafety("usb", true, 32.0, nil, cleanDev)
	if !passed {
		return map[string]interface{}{
			"ok":    false,
			"error": fmt.Sprintf("Safety lock blocked %s: %s", cleanDev, strings.Join(reasons, "; ")),
		}
	}

	if !privilege.IsProcessElevated() {
		esc := privilege.RequestNativePrivilegeEscalation(nil)
		if esc.ElevatedSpawned {
			a.mu.Lock()
			a.forceClosing = true
			a.mu.Unlock()
			go func() {
				time.Sleep(250 * time.Millisecond)
				if a.ctx != nil {
					wailsRuntime.Quit(a.ctx)
				}
				os.Exit(0)
			}()
			return map[string]interface{}{
				"ok":                false,
				"elevated_relaunch": true,
				"error":             "Launched Administrator window to reset the USB drive.",
			}
		}
		if !esc.AlreadyElevated {
			return map[string]interface{}{
				"ok":    false,
				"error": esc.Message,
			}
		}
	}

	if err := usb.ResetUSBToCleanFAT32(cleanDev); err != nil {
		return map[string]interface{}{
			"ok":    false,
			"error": err.Error(),
		}
	}

	return map[string]interface{}{
		"ok":      true,
		"device":  cleanDev,
		"message": fmt.Sprintf("Cleared write-protection and formatted %s as a full-capacity FAT32 USB drive.", cleanDev),
	}
}

// VerifyUSBDrive inspects the GPT partition table, boot sectors, FAT clean bits,
// and UBUNTU-ISO / CIDATA / INSTANTUSB partitions on a USB drive without modifying it.
func (a *App) VerifyUSBDrive(devicePath string) map[string]interface{} {
	cleanDev := strings.TrimSpace(devicePath)
	if cleanDev == "" {
		candidates := usb.ScanRemovableUSBDrives()
		for _, c := range candidates {
			if c.TripleLockPassed {
				cleanDev = c.DeviceNode
				break
			}
		}
	}
	if cleanDev == "" {
		return map[string]interface{}{
			"ok":    false,
			"error": "Select a target USB drive first to verify its bootable partitions.",
		}
	}

	startTime := time.Now()
	a.appendEventLog(startTime, "VERIFY_USB", fmt.Sprintf("Inspecting GPT partition table and boot sectors on %s...", cleanDev))
	report, err := usb.VerifyBootableUSB(cleanDev)
	if err != nil {
		a.appendEventLog(startTime, "VERIFY_ERR", err.Error())
		return map[string]interface{}{
			"ok":    false,
			"error": err.Error(),
		}
	}

	for _, p := range report.Partitions {
		a.appendEventLog(startTime, "PARTITION", p)
	}
	for _, chk := range report.Checks {
		a.appendEventLog(startTime, "CHECK", chk)
	}
	a.appendEventLog(startTime, "VERIFY_DONE", report.Summary)

	return map[string]interface{}{
		"ok":         report.Passed,
		"passed":     report.Passed,
		"device":     report.Device,
		"partitions": report.Partitions,
		"checks":     report.Checks,
		"message":    report.Summary,
	}
}

// BootUSBNextRestart configures the current Windows or Linux machine to boot directly
// into the flashed Instant Server USB drive on next restart.
// mode can be "stage_only" (configure UEFI BootNext without rebooting immediately) or "restart_now".
func (a *App) BootUSBNextRestart(mode string) map[string]interface{} {
	cleanMode := strings.ToLower(strings.TrimSpace(mode))
	if cleanMode == "" {
		cleanMode = "restart_now"
	}
	restartNow := cleanMode == "restart_now"

	if !privilege.IsProcessElevated() {
		esc := privilege.RequestNativePrivilegeEscalation(nil)
		if esc.ElevatedSpawned {
			a.mu.Lock()
			a.forceClosing = true
			a.mu.Unlock()
			go func() {
				time.Sleep(250 * time.Millisecond)
				if a.ctx != nil {
					wailsRuntime.Quit(a.ctx)
				}
				os.Exit(0)
			}()
			return map[string]interface{}{
				"ok":                false,
				"elevated_relaunch": true,
				"error":             "Launched Administrator window to configure next-boot USB target.",
			}
		}
		if !esc.AlreadyElevated {
			return map[string]interface{}{
				"ok":    false,
				"error": esc.Message,
			}
		}
	}

	switch runtime.GOOS {
	case "windows":
		rebootCmd := ""
		if restartNow {
			rebootCmd = `if ($usbGuid) { shutdown /r /t 3 /c "Instant Server: Rebooting directly into USB installer" } else { shutdown /r /fw /t 2 }`
		}
		ps := `$out = bcdedit /enum firmware 2>$null | Out-String; ` +
			`$blocks = $out -split '(?=identifier\s+\{)'; ` +
			`$usbGuid = ''; ` +
			`foreach ($b in $blocks) { if ($b -match 'identifier\s+(\{[0-9a-fA-F-]+\})' -and $b -match '(?i)(USB|Ubuntu|Removable|Flash|Instant|Linpus|UEFI:)') { $usbGuid = $Matches[1]; break } }; ` +
			`if (-not $usbGuid) { ` +
			`  $efiDrv = Get-PSDrive -PSProvider FileSystem | Where-Object { Test-Path ($_.Root + 'EFI\BOOTootx64.efi') } | Select-Object -First 1; ` +
			`  if ($efiDrv) { ` +
			`    $part = $efiDrv.Name + ':'; ` +
			`    $cp = bcdedit /copy '{bootmgr}' /d 'Instant Server USB' 2>$null | Out-String; ` +
			`    if ($cp -match '(\{[0-9a-fA-F-]+\})') { ` +
			`      $usbGuid = $Matches[1]; ` +
			`      bcdedit /set $usbGuid device partition=$part | Out-Null; ` +
			`      bcdedit /set $usbGuid path '\EFI\BOOTootx64.efi' | Out-Null ` +
			`    } ` +
			`  } ` +
			`}; ` +
			`if ($usbGuid) { bcdedit /set '{fwbootmgr}' bootsequence $usbGuid | Out-Null; Write-Output ("UEFI_USB:" + $usbGuid) } else { Write-Output "UEFI_FW_MENU" }; ` +
			rebootCmd
		out, err := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", ps).CombinedOutput()
		if err != nil {
			return map[string]interface{}{
				"ok":    false,
				"error": fmt.Sprintf("Failed configuring Windows UEFI next-boot target: %v (%s)", err, strings.TrimSpace(string(out))),
			}
		}
		outStr := strings.TrimSpace(string(out))
		if !restartNow {
			if strings.Contains(outStr, "UEFI_USB:") {
				return map[string]interface{}{
					"ok":      true,
					"message": "Queued Instant Server USB in Windows UEFI bootsequence. Your PC will boot into the USB the next time you restart.",
				}
			}
			return map[string]interface{}{
				"ok":      true,
				"message": "Your UEFI BIOS enumerates USB drives at POST. Click 'Confirm & Restart Now' when ready to boot into UEFI/USB.",
			}
		}
		return map[string]interface{}{
			"ok":      true,
			"message": "Configured UEFI one-time USB boot sequence. Restarting into Instant Server USB in 3 seconds...",
		}

	case "linux":
		rebootTail := ""
		if restartNow {
			rebootTail = " && (sleep 2 && systemctl reboot) &"
		}
		sh := fmt.Sprintf(`if [ -d /sys/firmware/efi ] && command -v efibootmgr >/dev/null 2>&1; then `+
			`bnum=$(efibootmgr 2>/dev/null | grep -iE 'USB|Ubuntu|Linpus|Removable|UEFI:' | head -n1 | sed -n 's/^Boot\([0-9A-Fa-f]\{4\}\).*//p'); `+
			`if [ -n "$bnum" ]; then efibootmgr -n "$bnum"%s; exit 0; fi; `+
			`fi; `+
			`iso_part=$(blkid -L UBUNTU-ISO 2>/dev/null || blkid -t TYPE=iso9660 -o device 2>/dev/null | head -n1 || true); `+
			`if [ -n "$iso_part" ] && command -v kexec >/dev/null 2>&1; then `+
			`mkdir -p /mnt/instant_usb_iso && mount -o ro "$iso_part" /mnt/instant_usb_iso 2>/dev/null || true; `+
			`if [ -f /mnt/instant_usb_iso/casper/vmlinuz ]; then kexec -l /mnt/instant_usb_iso/casper/vmlinuz --initrd=/mnt/instant_usb_iso/casper/initrd --command-line="BOOT_IMAGE=/casper/vmlinuz autoinstall noprompt ---"%s; exit 0; fi; `+
			`fi`, rebootTail, strings.ReplaceAll(rebootTail, "systemctl reboot", "systemctl kexec"))
		if out, err := exec.Command("bash", "-c", sh).CombinedOutput(); err != nil {
			return map[string]interface{}{
				"ok":    false,
				"error": fmt.Sprintf("Failed configuring Linux next-boot USB target: %v (%s)", err, strings.TrimSpace(string(out))),
			}
		}
		if !restartNow {
			return map[string]interface{}{
				"ok":      true,
				"message": "Staged Instant Server USB for next boot via efibootmgr/kexec. Restart whenever you are ready.",
			}
		}
		return map[string]interface{}{
			"ok":      true,
			"message": "Configured Linux next-boot USB target. Restarting into Instant Server USB in 2 seconds...",
		}

	default:
		return map[string]interface{}{
			"ok":    false,
			"error": "One-click next-boot USB restart is supported on Windows and Linux.",
		}
	}
}
