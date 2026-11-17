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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/InstantServer/InstantServer/network"
	"github.com/InstantServer/InstantServer/privilege"
	"github.com/InstantServer/InstantServer/usb"
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

func (a *App) AddCustomApp(payload map[string]interface{}) map[string]interface{} {
	rawInput, _ := payload["raw_input"].(string)
	title, _ := payload["title"].(string)
	port := 0
	if pVal, ok := payload["port"].(float64); ok {
		port = int(pVal)
	}
	if strings.TrimSpace(rawInput) == "" {
		return map[string]interface{}{
			"ok":    false,
			"error": "Paste a valid Docker image, docker run command, or GitHub/Docker Hub URL",
		}
	}
	entry := usb.ParseCustomAppInput(rawInput, title, port)
	return map[string]interface{}{
		"ok":  true,
		"app": entry,
	}
}

func (a *App) SaveConfig(profile usb.ServerProfileConfig) map[string]interface{} {
	dataDir := resolveWritableDataDir()
	cfgPath := filepath.Join(dataDir, "instant-server-config.json")
	raw, _ := json.MarshalIndent(profile, "", "  ")
	_ = os.WriteFile(cfgPath, raw, 0600)
	return map[string]interface{}{
		"saved": true,
		"path":  cfgPath,
	}
}

func (a *App) LoadConfig() map[string]interface{} {
	dataDir := resolveWritableDataDir()
	cfgPath := filepath.Join(dataDir, "instant-server-config.json")
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		return map[string]interface{}{"exists": false, "path": cfgPath}
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return map[string]interface{}{"exists": false, "path": cfgPath}
	}
	return map[string]interface{}{
		"exists": true,
		"path":   cfgPath,
		"config": parsed,
	}
}

func (a *App) PollBuildStatus() map[string]interface{} {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make(map[string]interface{}, len(a.lastStatus))
	for k, v := range a.lastStatus {
		out[k] = v
	}
	return out
}

func (a *App) appendEventLog(startTime time.Time, eventName string, msg string) {
	if startTime.IsZero() {
		startTime = time.Now()
	}
	elapsed := int(time.Since(startTime).Seconds())
	if elapsed < 0 {
		elapsed = 0
	}
	ts := fmt.Sprintf("[%02d:%02d] %s", elapsed/60, elapsed%60, msg)
	a.mu.Lock()
	evs, _ := a.lastStatus["events"].([]map[string]interface{})
	evs = append(evs, map[string]interface{}{
		"event":          eventName,
		"message":        msg,
		"formatted_line": ts,
	})
	a.lastStatus["events"] = evs
	a.mu.Unlock()
}

func (a *App) FlashUSBDevice(profile usb.ServerProfileConfig, isoPath string) map[string]interface{} {
	if err := usb.ValidateProfileConfig(profile); err != nil {
		return map[string]interface{}{"started": false, "reason": err.Error()}
	}

	_ = a.SaveConfig(profile)

	a.mu.Lock()
	if a.flashing {
		a.mu.Unlock()
		return map[string]interface{}{"started": false, "reason": "Flashing is already in progress"}
	}
	if !privilege.IsProcessElevated() {
		esc := privilege.RequestNativePrivilegeEscalation(nil)
		if esc.ElevatedSpawned {
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
				"started":           false,
				"elevated_relaunch": true,
				"reason":            "Launched Administrator window with your saved configuration.",
			}
		}
		if !esc.AlreadyElevated {
			a.mu.Unlock()
			return map[string]interface{}{"started": false, "reason": esc.Message}
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.flashing = true
	a.flashStartTime = time.Now()
	a.cancelFunc = cancel
	a.lastProfile = profile
	a.lastISOPath = isoPath
	a.pauseGate.Resume()
	initPct := 6
	if prevPct, ok := a.lastStatus["percent"].(int); ok && prevPct > 6 && prevPct < 45 {
		initPct = prevPct
	}
	a.lastStatus = map[string]interface{}{
		"status":      "RUNNING",
		"percent":     initPct,
		"stage_index": 1,
		"paused":      false,
		"events":      []map[string]interface{}{},
	}
	a.mu.Unlock()

	go func() {
		startTime := a.flashStartTime
		defer func() {
			a.mu.Lock()
			a.flashing = false
			a.cancelFunc = nil
			a.mu.Unlock()
		}()

		// Stage 2: Stage cloud-init & app files
		a.mu.Lock()
		a.lastStatus["stage_index"] = 2
		a.mu.Unlock()

		baseDataDir := resolveWritableDataDir()
		hardeningBytes, _ := a.assets.ReadFile("src/instant_server/scripts/laptop_hardening.sh")
		dashBytes, _ := a.assets.ReadFile("src/instant_server/ui/server_dashboard.html")
		stagingDir := filepath.Join(baseDataDir, "cidata_stage")
		stagedFiles, stageErr := usb.StageCIDATAPayload(stagingDir, profile, hardeningBytes, dashBytes)
		if stageErr != nil {
			a.mu.Lock()
			a.lastStatus["status"] = "FAILED"
			a.lastStatus["error"] = stageErr.Error()
			a.mu.Unlock()
			return
		}
		a.appendEventLog(startTime, "STAGE_CIDATA", fmt.Sprintf("Staged %d cloud-init & app file(s) for FAT16 CIDATA partition", len(stagedFiles)))

		// Stage 3: Check / download Ubuntu Server LTS ISO
		a.mu.Lock()
		a.lastStatus["stage_index"] = 3
		a.mu.Unlock()

		resolvedISO := strings.TrimSpace(isoPath)
		if resolvedISO == "" {
			resolvedISO = strings.TrimSpace(profile.CustomISOPath)
		}
		if resolvedISO == "" || func() bool { _, e := os.Stat(resolvedISO); return e != nil }() {
			a.appendEventLog(startTime, "ISO_CHECK", fmt.Sprintf("Checking Ubuntu Server %s LTS ISO...", profile.UbuntuRelease))
			cacheDir := filepath.Join(baseDataDir, "iso_cache")
			dlPath, dlErr := usb.DownloadISOWithResume(
				ctx,
				profile.UbuntuRelease,
				cacheDir,
				a.pauseGate,
				func(ev usb.FlashProgressEvent) {
					a.mu.Lock()
					if a.lastStatus["status"] == "CANCELLED" {
						a.mu.Unlock()
						return
					}
					if a.pauseGate.IsPaused() {
						a.lastStatus["status"] = "PAUSED"
						a.lastStatus["paused"] = true
					} else {
						a.lastStatus["status"] = "RUNNING"
						a.lastStatus["paused"] = false
					}
					overallPct := 10 + (ev.Percent*35)/100
					a.lastStatus["percent"] = overallPct
					a.lastStatus["stage_index"] = 3
					a.lastStatus["active_progress"] = map[string]interface{}{
						"stage_percent": ev.Percent,
						"stage_index":   3,
						"label":         "Downloading Ubuntu ISO",
						"message":       ev.Message,
						"terminal_line": fmt.Sprintf("[ISO] Downloading Ubuntu ISO: %d%% | %s", ev.Percent, ev.Message),
					}
					a.mu.Unlock()
					if a.ctx != nil {
						wailsRuntime.EventsEmit(a.ctx, "flash:progress", map[string]interface{}{
							"percent":       overallPct,
							"stage_percent": ev.Percent,
							"stage_index":   3,
							"stage":         "Downloading Ubuntu ISO",
							"message":       ev.Message,
						})
					}
				},
			)
			if dlErr != nil {
				a.mu.Lock()
				if a.lastStatus["status"] != "CANCELLED" {
					a.lastStatus["status"] = "FAILED"
					a.lastStatus["error"] = dlErr.Error()
				}
				a.mu.Unlock()
				if a.ctx != nil {
					wailsRuntime.EventsEmit(a.ctx, "flash:error", map[string]interface{}{"error": dlErr.Error()})
				}
				return
			}
			resolvedISO = dlPath
			a.appendEventLog(startTime, "ISO_READY", fmt.Sprintf("Ubuntu Server ISO ready: %s", filepath.Base(resolvedISO)))
		} else {
			a.appendEventLog(startTime, "ISO_LOCAL", fmt.Sprintf("Using local ISO file: %s", resolvedISO))
		}

		// Stage 4: Unlock & write bootable ISO + Stage 5: Partition & verify
		a.mu.Lock()
		a.lastStatus["stage_index"] = 4
		a.mu.Unlock()
		a.appendEventLog(startTime, "USB_OPEN", fmt.Sprintf("Preparing and unlocking USB device %s...", profile.SelectedUSBDevice))
		finalEv, err := usb.StreamISOAndCidataToRawDisk(
			ctx,
			resolvedISO,
			profile.SelectedUSBDevice,
			stagingDir,
			true,
			true,
			a.pauseGate,
			func(ev usb.FlashProgressEvent) {
				a.mu.Lock()
				if a.lastStatus["status"] == "CANCELLED" {
					a.mu.Unlock()
					return
				}
				if a.pauseGate.IsPaused() {
					a.lastStatus["status"] = "PAUSED"
					a.lastStatus["paused"] = true
				} else {
					a.lastStatus["status"] = "RUNNING"
					a.lastStatus["paused"] = false
				}
				curStageIdx := 4
				if ev.Stage == "VERIFIED" || ev.Percent >= 99 {
					curStageIdx = 5
				}
				overallPct := 48 + (ev.Percent*50)/100
				a.lastStatus["percent"] = overallPct
				a.lastStatus["stage_index"] = curStageIdx
				a.lastStatus["active_progress"] = map[string]interface{}{
					"stage_percent": ev.Percent,
					"stage_index":   curStageIdx,
					"label":         "Writing Bootable USB",
					"message":       ev.Message,
					"terminal_line": fmt.Sprintf("[USB] Writing Bootable USB: %d%% | %s (CRC32 %s)", ev.Percent, ev.Message, ev.CRC32Hex),
				}
				a.mu.Unlock()
				if a.ctx != nil {
					wailsRuntime.EventsEmit(a.ctx, "flash:progress", map[string]interface{}{
						"percent":       overallPct,
						"stage_percent": ev.Percent,
						"stage_index":   curStageIdx,
						"stage":         "Writing Bootable USB",
						"eta_sec":       ev.ETASeconds,
						"message":       ev.Message,
					})
				}
			},
		)

		if err != nil {
			a.mu.Lock()
			if a.lastStatus["status"] != "CANCELLED" {
				a.lastStatus["status"] = "FAILED"
				a.lastStatus["error"] = err.Error()
			}
			a.mu.Unlock()
			if a.ctx != nil {
				wailsRuntime.EventsEmit(a.ctx, "flash:error", map[string]interface{}{
					"error": err.Error(),
				})
			}
			return
		}

		a.appendEventLog(startTime, "USB_VERIFIED", fmt.Sprintf("Flashed & verified SHA-256 (%s) + FAT16 CIDATA + FAT32 INSTANTUSB on %s", finalEv.SHA256Hex[:16]+"...", profile.SelectedUSBDevice))
		a.mu.Lock()
		a.lastStatus["status"] = "COMPLETED"
		a.lastStatus["percent"] = 100
		a.lastStatus["stage_index"] = 6
		a.lastStatus["paused"] = false
		a.lastStatus["active_progress"] = nil
		a.lastStatus["cache_kept_by_default"] = true
		a.lastStatus["result"] = map[string]interface{}{
			"usb_provision": map[string]interface{}{
				"target_device": profile.SelectedUSBDevice,
				"copied_files":  stagedFiles,
				"sha256":        finalEv.SHA256Hex,
				"crc32":         finalEv.CRC32Hex,
			},
		}
		a.mu.Unlock()
		if a.ctx != nil && finalEv != nil {
			wailsRuntime.EventsEmit(a.ctx, "flash:complete", finalEv)
		}
	}()

	return map[string]interface{}{"started": true}
}

func (a *App) PauseFlashing() map[string]interface{} {
	a.pauseGate.Pause()
	a.mu.Lock()
	startTime := a.flashStartTime
	a.lastStatus["status"] = "PAUSED"
	a.lastStatus["paused"] = true
	a.lastStatus["active_progress"] = map[string]interface{}{
		"message":       "Paused — partial progress saved as .part",
		"terminal_line": "[PAUSED] Click Resume to continue",
	}
	a.mu.Unlock()
	a.appendEventLog(startTime, "PAUSED", "[PAUSED] Flashing paused (saved as .part).")
	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, "flash:paused", map[string]interface{}{"paused": true})
	}
	return map[string]interface{}{"paused": true, "status": "PAUSED"}
}

func (a *App) ResumeFlashing() map[string]interface{} {
	a.pauseGate.Resume()
	a.mu.Lock()
	isRunning := a.flashing
	savedProfile := a.lastProfile
	savedISO := a.lastISOPath
	startTime := a.flashStartTime
	if isRunning {
		a.lastStatus["status"] = "RUNNING"
		a.lastStatus["paused"] = false
	}
	a.mu.Unlock()

	if !isRunning && savedProfile.Hostname != "" {
		res := a.FlashUSBDevice(savedProfile, savedISO)
		if started, _ := res["started"].(bool); started {
			if a.ctx != nil {
				wailsRuntime.EventsEmit(a.ctx, "flash:resumed", map[string]interface{}{"paused": false})
			}
			return map[string]interface{}{"resumed": true, "status": "RUNNING", "restarted_from_part": true}
		}
	}

	a.appendEventLog(startTime, "RESUMED", "[RESUMED] Resuming from saved .part progress...")
	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, "flash:resumed", map[string]interface{}{"paused": false})
	}
	return map[string]interface{}{"resumed": true, "status": "RUNNING"}
}

func (a *App) CancelFlashing() map[string]interface{} {
	a.mu.Lock()
	startTime := a.flashStartTime
	cancel := a.cancelFunc
	a.flashing = false
	a.pauseGate.Resume()
	a.lastStatus["status"] = "CANCELLED"
	a.lastStatus["paused"] = false
	a.lastStatus["active_progress"] = nil
	a.mu.Unlock()
	a.appendEventLog(startTime, "CANCELLED", "[CANCELLED] Operation cancelled (saved as .part).")
	if cancel != nil {
		cancel()
	}
	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, "flash:cancelled", map[string]interface{}{"cancelled": true})
	}
	return map[string]interface{}{"cancelled": true, "status": "CANCELLED"}
}

func (a *App) PauseAndPrepareExit() map[string]interface{} {
	a.pauseGate.Pause()
	a.mu.Lock()
	startTime := a.flashStartTime
	a.forceClosing = true
	cancel := a.cancelFunc
	a.lastStatus["status"] = "PAUSED"
	a.lastStatus["paused"] = true
	a.mu.Unlock()
	a.appendEventLog(startTime, "PAUSED_EXIT", "[PAUSED] Saved as .part.")
	if cancel != nil {
		cancel()
	}
	return map[string]interface{}{
		"ok":         true,
		"paused":     true,
		"saved_data": resolveWritableDataDir(),
	}
}

func (a *App) ManageFlashCache(action string) map[string]interface{} {
	baseDir := resolveWritableDataDir()
	isoDir := filepath.Join(baseDir, "iso_cache")
	stageDir := filepath.Join(baseDir, "cidata_stage")

	act := strings.ToLower(strings.TrimSpace(action))
	if act == "delete" || act == "clear" {
		_ = os.RemoveAll(isoDir)
		_ = os.RemoveAll(stageDir)
		_ = os.MkdirAll(isoDir, 0755)
		_ = os.MkdirAll(stageDir, 0755)
		return map[string]interface{}{
			"ok":          true,
			"action":      "deleted",
			"kept":        false,
			"total_bytes": 0,
			"total_mb":    "0.0 MB",
			"message":     "Deleted cached ISO and staging files.",
		}
	}

	var totalBytes int64
	var fileCount int
	for _, d := range []string{isoDir, stageDir} {
		_ = filepath.Walk(d, func(path string, info os.FileInfo, err error) error {
			if err == nil && info != nil && !info.IsDir() {
				totalBytes += info.Size()
				fileCount++
			}
			return nil
		})
	}
	return map[string]interface{}{
		"ok":          true,
		"action":      "kept",
		"kept":        true,
		"file_count":  fileCount,
		"total_bytes": totalBytes,
		"total_mb":    fmt.Sprintf("%.1f MB", float64(totalBytes)/(1024.0*1024.0)),
		"data_dir":    baseDir,
		"message":     "Cached ISO and app files kept.",
	}
}

func (a *App) DiscoverServers(hostname string) map[string]interface{} {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	servers := network.DiscoverInstantServersOnLAN(ctx, hostname)
	return map[string]interface{}{
		"servers_found": servers,
	}
}

func (a *App) AssetAndAPIRouter() http.Handler {
	mux := http.NewServeMux()

	writeJSON := func(w http.ResponseWriter, data interface{}) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(data)
	}

	readJSONBody := func(r *http.Request, dst interface{}) {
		defer r.Body.Close()
		raw, _ := io.ReadAll(r.Body)
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, dst)
		}
	}

	mux.HandleFunc("/api/host", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, a.ProbeHost())
	})
	mux.HandleFunc("/api/fit-window", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			AvailWidth  int `json:"avail_width"`
			AvailHeight int `json:"avail_height"`
		}
		readJSONBody(r, &body)
		writeJSON(w, a.FitWindowToScreen(body.AvailWidth, body.AvailHeight))
	})
	mux.HandleFunc("/api/toggle-maximize", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, a.ToggleMaximizeWindow())
	})
	mux.HandleFunc("/api/elevate", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, a.ElevateAdmin())
	})
	mux.HandleFunc("/api/usb", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, a.ScanUSBDevices())
	})
	mux.HandleFunc("/api/reset-usb", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Device string `json:"device"`
		}
		readJSONBody(r, &body)
		if body.Device == "" {
			body.Device = r.URL.Query().Get("device")
		}
		writeJSON(w, a.ResetUSBDrive(body.Device))
	})
	mux.HandleFunc("/api/verify-usb", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Device string `json:"device"`
		}
		readJSONBody(r, &body)
		if body.Device == "" {
			body.Device = r.URL.Query().Get("device")
		}
		writeJSON(w, a.VerifyUSBDrive(body.Device))
	})
	mux.HandleFunc("/api/boot-usb-next", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Mode string `json:"mode"`
		}
		readJSONBody(r, &body)
		writeJSON(w, a.BootUSBNextRestart(body.Mode))
	})
	mux.HandleFunc("/api/load-config", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, a.LoadConfig())
	})
	mux.HandleFunc("/api/save-config", func(w http.ResponseWriter, r *http.Request) {
		var cfg usb.ServerProfileConfig
		readJSONBody(r, &cfg)
		writeJSON(w, a.SaveConfig(cfg))
	})
	mux.HandleFunc("/api/custom-app", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		readJSONBody(r, &body)
		writeJSON(w, a.AddCustomApp(body))
	})
	mux.HandleFunc("/api/flash", func(w http.ResponseWriter, r *http.Request) {
		var cfg usb.ServerProfileConfig
		readJSONBody(r, &cfg)
		writeJSON(w, a.FlashUSBDevice(cfg, cfg.CustomISOPath))
	})
	mux.HandleFunc("/api/pause", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, a.PauseFlashing())
	})
	mux.HandleFunc("/api/resume", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, a.ResumeFlashing())
	})
	mux.HandleFunc("/api/cancel", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, a.CancelFlashing())
	})
	mux.HandleFunc("/api/pause-and-exit", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, a.PauseAndPrepareExit())
	})
	mux.HandleFunc("/api/cache", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Action string `json:"action"`
		}
		readJSONBody(r, &body)
		if body.Action == "" {
			body.Action = r.URL.Query().Get("action")
		}
		writeJSON(w, a.ManageFlashCache(body.Action))
	})
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, a.PollBuildStatus())
	})
	mux.HandleFunc("/api/discover", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, a.DiscoverServers("instant-server"))
	})
	mux.HandleFunc("/api/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"alive": true, "close_warning_requested": false})
	})
	mux.HandleFunc("/api/shutdown", func(w http.ResponseWriter, r *http.Request) {
		a.GracefulShutdownSupervisor()
		writeJSON(w, map[string]interface{}{"shutdown": true})
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		reqPath := strings.TrimPrefix(r.URL.Path, "/")
		if reqPath == "" || reqPath == "index.html" || reqPath == "app_ui.html" {
			data, err := a.assets.ReadFile("src/instant_server/ui/app_ui.html")
			if err != nil {
				data, err = os.ReadFile(filepath.Join("src", "instant_server", "ui", "app_ui.html"))
			}
			if err == nil {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = w.Write(data)
				return
			}
		}
		if reqPath == "server_dashboard.html" {
			data, err := a.assets.ReadFile("src/instant_server/ui/server_dashboard.html")
			if err != nil {
				data, err = os.ReadFile(filepath.Join("src", "instant_server", "ui", "server_dashboard.html"))
			}
			if err == nil {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = w.Write(data)
				return
			}
		}
		if strings.HasSuffix(reqPath, ".js") || strings.HasSuffix(reqPath, ".css") {
			cleanBase := filepath.Base(reqPath)
			data, err := a.assets.ReadFile("src/instant_server/ui/" + cleanBase)
			if err != nil {
				data, err = os.ReadFile(filepath.Join("src", "instant_server", "ui", cleanBase))
			}
			if err == nil {
				if strings.HasSuffix(cleanBase, ".js") {
					w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
				} else {
					w.Header().Set("Content-Type", "text/css; charset=utf-8")
				}
				_, _ = w.Write(data)
				return
			}
		}
		http.NotFound(w, r)
	})

	return mux
}
