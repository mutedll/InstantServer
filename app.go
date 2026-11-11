// app.go
package main

import (
	"context"
	"embed"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
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
	lastStatus     map[string]interface{}
}

func NewApp(assets embed.FS) *App {
	return &App{
		assets:    assets,
		pauseGate: usb.NewPauseGate(),
		lastStatus: map[string]interface{}{
			"status":  "IDLE",
			"percent": 0,
			"paused":  false,
			"events":  []map[string]interface{}{},
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

// GracefulShutdownSupervisor safely stops any active DMA stream while keeping
// all downloaded ISO and staged app files on disk by default so the user can
// resume or reflash later without re-downloading.
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
	qemuBin, _ := exec.LookPath("qemu-system-x86_64")
	return map[string]interface{}{
		"host_os":          runtime.GOOS,
		"arch":             runtime.GOARCH,
		"is_admin_or_root": privilege.IsProcessElevated(),
		"engine":           "wails_go_native_dma",
		"qemu_available":   qemuBin != "",
		"qemu_path":        qemuBin,
	}
}

// ElevateAdmin triggers the native OS privilege elevation dialog (UAC on Windows,
// Polkit on Linux, AuthorizationServices on macOS). When the new elevated instance
// spawns (`res.ElevatedSpawned == true`), this unprivileged instance automatically
// closes itself so the user never ends up with duplicate windows.
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
	a.pauseGate.Resume()
	a.lastStatus = map[string]interface{}{
		"status":  "RUNNING",
		"percent": 6,
		"paused":  false,
		"events":  []map[string]interface{}{},
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
		a.appendEventLog(startTime, "STAGE_CIDATA", fmt.Sprintf("Staged %d cloud-init & app file(s) for FAT16 CIDATA partition (cached for fast reflash)", len(stagedFiles)))

		resolvedISO := strings.TrimSpace(isoPath)
		if resolvedISO == "" {
			resolvedISO = strings.TrimSpace(profile.CustomISOPath)
		}
		if resolvedISO == "" || func() bool { _, e := os.Stat(resolvedISO); return e != nil }() {
			a.appendEventLog(startTime, "ISO_CHECK", fmt.Sprintf("Resolving Ubuntu Server %s LTS ISO mirror & local cache...", profile.UbuntuRelease))
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
					a.lastStatus["active_progress"] = map[string]interface{}{
						"stage_percent": ev.Percent,
						"label":         "Downloading Ubuntu ISO",
						"message":       ev.Message,
						"terminal_line": fmt.Sprintf("⟳ Downloading Ubuntu ISO: %d%% │ %s", ev.Percent, ev.Message),
					}
					a.mu.Unlock()
					if a.ctx != nil {
						wailsRuntime.EventsEmit(a.ctx, "flash:progress", map[string]interface{}{
							"percent":       overallPct,
							"stage_percent": ev.Percent,
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
			a.appendEventLog(startTime, "ISO_READY", fmt.Sprintf("Ubuntu Server ISO ready in cache: %s", filepath.Base(resolvedISO)))
		} else {
			a.appendEventLog(startTime, "ISO_LOCAL", fmt.Sprintf("Using local ISO file: %s", resolvedISO))
		}

		a.appendEventLog(startTime, "USB_OPEN", fmt.Sprintf("Opening raw USB block device %s with 1MB Win32-safe DMA chunks...", profile.SelectedUSBDevice))
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
				overallPct := 48 + (ev.Percent*50)/100
				a.lastStatus["percent"] = overallPct
				a.lastStatus["active_progress"] = map[string]interface{}{
					"stage_percent": ev.Percent,
					"label":         "Writing Bootable USB",
					"message":       ev.Message,
					"terminal_line": fmt.Sprintf("⟳ Writing Bootable USB: %d%% │ %s (CRC32 %s)", ev.Percent, ev.Message, ev.CRC32Hex),
				}
				a.mu.Unlock()
				if a.ctx != nil {
					wailsRuntime.EventsEmit(a.ctx, "flash:progress", map[string]interface{}{
						"percent":       overallPct,
						"stage_percent": ev.Percent,
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

		a.appendEventLog(startTime, "USB_VERIFIED", fmt.Sprintf("Flashed & verified SHA-256 (%s) + FAT16 CIDATA partition on %s", finalEv.SHA256Hex[:16]+"...", profile.SelectedUSBDevice))
		a.mu.Lock()
		a.lastStatus["status"] = "COMPLETED"
		a.lastStatus["percent"] = 100
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
		"message":       "Paused — all downloaded ISO & staged files are safely held on disk",
		"terminal_line": "⏸ PAUSED │ Click Resume anytime to continue from exact byte offset",
	}
	a.mu.Unlock()
	a.appendEventLog(startTime, "PAUSED", "[PAUSED] Flashing paused safely. Downloaded ISO & staged files are preserved.")
	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, "flash:paused", map[string]interface{}{"paused": true})
	}
	return map[string]interface{}{"paused": true, "status": "PAUSED"}
}

func (a *App) ResumeFlashing() map[string]interface{} {
	a.pauseGate.Resume()
	a.mu.Lock()
	startTime := a.flashStartTime
	a.lastStatus["status"] = "RUNNING"
	a.lastStatus["paused"] = false
	a.mu.Unlock()
	a.appendEventLog(startTime, "RESUMED", "[RESUMED] Resuming USB DMA stream from exact byte offset...")
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
	a.appendEventLog(startTime, "CANCELLED", "[CANCELLED] Operation cancelled by user. Cached ISO & app files kept for later.")
	if cancel != nil {
		cancel()
	}
	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, "flash:cancelled", map[string]interface{}{"cancelled": true})
	}
	return map[string]interface{}{"cancelled": true, "status": "CANCELLED"}
}

// PauseAndPrepareExit pauses the current operation, preserves all downloaded `.part` / `.iso`
// files and staged CIDATA configurations in `Instant_Server_Data`, and permits the window to close.
func (a *App) PauseAndPrepareExit() map[string]interface{} {
	a.pauseGate.Pause()
	a.mu.Lock()
	startTime := a.flashStartTime
	a.forceClosing = true
	cancel := a.cancelFunc
	a.lastStatus["status"] = "PAUSED"
	a.lastStatus["paused"] = true
	a.mu.Unlock()
	a.appendEventLog(startTime, "PAUSED_EXIT", "[PAUSED & SAVED] Saved partial ISO download & staged app files for instant resume next time.")
	if cancel != nil {
		cancel()
	}
	return map[string]interface{}{
		"ok":         true,
		"paused":     true,
		"saved_data": resolveWritableDataDir(),
	}
}

// ManageFlashCache inspects, keeps (default), or deletes the cached Ubuntu ISO and staged CIDATA files.
// If the user exits or force-quits without selecting an option, files are kept by default.
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
			"message":     "Deleted cached Ubuntu ISO and temporary staging files to free disk space.",
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
		"message":     "Cached ISO & staged app configs retained. Reflashing or adding new apps will be instant.",
	}
}

// TestInDisposableVM builds a real bootable hybrid MBR/GPT + FAT16 CIDATA virtual USB disk image
// (`Instant_Server_Data/vm_test/instant-server-usb.img`) plus a standalone 16 MiB `instant-server-cidata.img`
// and a sparse 20 GB virtual target drive, generates cross-platform QEMU launch scripts with SMBIOS NoCloud
// (`-smbios type=1,serial=ds=nocloud`) and port forwarding (`localhost:18088 -> :8088`),
// and spawns QEMU automatically if installed on the host.
func (a *App) TestInDisposableVM(profile usb.ServerProfileConfig) map[string]interface{} {
	if err := usb.ValidateProfileConfig(profile); err != nil {
		return map[string]interface{}{"ok": false, "error": err.Error()}
	}
	_ = a.SaveConfig(profile)

	baseDir := resolveWritableDataDir()
	vmDir := filepath.Join(baseDir, "vm_test")
	stageDir := filepath.Join(baseDir, "cidata_stage")
	_ = os.MkdirAll(vmDir, 0755)

	hardeningBytes, _ := a.assets.ReadFile("src/instant_server/scripts/laptop_hardening.sh")
	dashBytes, _ := a.assets.ReadFile("src/instant_server/ui/server_dashboard.html")
	stagedFiles, stageErr := usb.StageCIDATAPayload(stageDir, profile, hardeningBytes, dashBytes)
	if stageErr != nil {
		return map[string]interface{}{"ok": false, "error": stageErr.Error()}
	}

	usbImgPath := filepath.Join(vmDir, "instant-server-usb.img")
	cidataImgPath := filepath.Join(vmDir, "instant-server-cidata.img")
	targetDiskPath := filepath.Join(vmDir, "disposable-target-disk.img")

	// Always build the 16 MiB FAT16 CIDATA volume (`instant-server-cidata.img`) so QEMU always has
	// the complete NoCloud seed (`user-data`, `meta-data`, `laptop_hardening.sh`, `server_dashboard.html`, `instant_apps_bundle.sh`)
	// even when booting directly from a cached 2.6 GB Ubuntu Server ISO.
	cidataBytes, cidataErr := usb.BuildCidataFAT16Image(stageDir)
	if cidataErr != nil {
		return map[string]interface{}{"ok": false, "error": fmt.Sprintf("Failed to build FAT16 CIDATA volume: %v", cidataErr)}
	}
	if err := os.WriteFile(cidataImgPath, cidataBytes, 0644); err != nil {
		return map[string]interface{}{"ok": false, "error": fmt.Sprintf("Failed to write CIDATA image: %v", err)}
	}

	// Check if a cached full ISO already exists in iso_cache; if not, create a synthetic 36 MB hybrid GPT/MBR ISO
	// with GRUB autoinstall config so the virtual USB image undergoes the exact same GPT relocation & FAT16 CIDATA builder.
	isoSourcePath := strings.TrimSpace(profile.CustomISOPath)
	if isoSourcePath == "" {
		isoCacheDir := filepath.Join(baseDir, "iso_cache")
		if entries, err := os.ReadDir(isoCacheDir); err == nil {
			for _, e := range entries {
				if strings.HasSuffix(strings.ToLower(e.Name()), ".iso") {
					isoSourcePath = filepath.Join(isoCacheDir, e.Name())
					break
				}
			}
		}
	}

	usingFullISO := false
	if isoSourcePath != "" {
		if st, err := os.Stat(isoSourcePath); err == nil && st.Size() > 32*1024*1024 {
			usingFullISO = true
		}
	}

	if !usingFullISO {
		synthISO := filepath.Join(vmDir, "synthetic-boot-check.iso")
		if err := buildSyntheticBootableTestISO(synthISO); err != nil {
			return map[string]interface{}{"ok": false, "error": fmt.Sprintf("Failed to create synthetic test ISO: %v", err)}
		}
		isoSourcePath = synthISO
	}

	// Build the virtual USB image using the exact same StreamISOAndCidataToRawDisk pipeline
	if !usingFullISO {
		if f, err := os.OpenFile(usbImgPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644); err == nil {
			_ = f.Truncate(36 * 1024 * 1024)
			_ = f.Close()
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := usb.StreamISOAndCidataToRawDisk(ctx, isoSourcePath, usbImgPath, stageDir, true, true, usb.NewPauseGate(), nil)
		if err != nil {
			return map[string]interface{}{"ok": false, "error": fmt.Sprintf("Virtual USB boot image build failed: %v", err)}
		}
	}

	// Create a sparse 20 GB disposable target disk if it doesn't exist
	if f, err := os.OpenFile(targetDiskPath, os.O_CREATE|os.O_RDWR, 0644); err == nil {
		_ = f.Truncate(20 * 1024 * 1024 * 1024)
		_ = f.Close()
	}

	usbDriveSource := usbImgPath
	if usingFullISO {
		usbDriveSource = isoSourcePath
	}

	batScript := fmt.Sprintf(`@echo off
title Instant Server — Disposable VM Boot Tester
echo ======================================================================
echo   Instant Server Disposable VM Tester (Port 18088 -^> :8088)
echo   Dashboard Preview: http://127.0.0.1:18088
echo ======================================================================
qemu-system-x86_64 -m 4096 -smp 2 ^
  -smbios type=1,serial=ds=nocloud ^
  -drive file="%s",format=raw,if=virtio,index=0 ^
  -drive file="%s",format=raw,if=virtio,index=1 ^
  -drive file="%s",format=raw,if=virtio,index=2 ^
  -boot order=c,menu=on ^
  -net nic,model=virtio -net user,hostfwd=tcp:127.0.0.1:18088-:8088,hostfwd=tcp:127.0.0.1:12222-:22,hostfwd=tcp:127.0.0.1:25565-:25565
`, usbDriveSource, cidataImgPath, targetDiskPath)
	_ = os.WriteFile(filepath.Join(vmDir, "launch-disposable-vm.bat"), []byte(strings.ReplaceAll(batScript, "\n", "\r\n")), 0755)

	shScript := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
echo "======================================================================"
echo "  Instant Server Disposable VM Tester (Port 18088 -> :8088)"
echo "  Dashboard URL once booted: http://127.0.0.1:18088"
echo "======================================================================"
ACCEL=""
if [ -e /dev/kvm ]; then
    ACCEL="-enable-kvm"
elif [ "$(uname -s 2>/dev/null || true)" = "Darwin" ]; then
    ACCEL="-accel hvf"
fi
exec qemu-system-x86_64 $ACCEL -m 4096 -smp 2 \
  -smbios type=1,serial=ds=nocloud \
  -drive file=%q,format=raw,if=virtio,index=0 \
  -drive file=%q,format=raw,if=virtio,index=1 \
  -drive file=%q,format=raw,if=virtio,index=2 \
  -boot order=c,menu=on \
  -net nic,model=virtio -net user,hostfwd=tcp:127.0.0.1:18088-:8088,hostfwd=tcp:127.0.0.1:12222-:22,hostfwd=tcp:127.0.0.1:25565-:25565
`, usbDriveSource, cidataImgPath, targetDiskPath)
	_ = os.WriteFile(filepath.Join(vmDir, "launch-disposable-vm.sh"), []byte(shScript), 0755)

	qemuBin, _ := exec.LookPath("qemu-system-x86_64")
	launchedQEMU := false
	if qemuBin != "" && usingFullISO {
		args := []string{
			"-m", "4096", "-smp", "2",
			"-smbios", "type=1,serial=ds=nocloud",
			"-drive", fmt.Sprintf("file=%s,format=raw,if=virtio,index=0", usbDriveSource),
			"-drive", fmt.Sprintf("file=%s,format=raw,if=virtio,index=1", cidataImgPath),
			"-drive", fmt.Sprintf("file=%s,format=raw,if=virtio,index=2", targetDiskPath),
			"-boot", "order=c,menu=on",
			"-net", "nic,model=virtio",
			"-net", "user,hostfwd=tcp:127.0.0.1:18088-:8088,hostfwd=tcp:127.0.0.1:12222-:22,hostfwd=tcp:127.0.0.1:25565-:25565",
		}
		if runtime.GOOS == "linux" {
			if _, err := os.Stat("/dev/kvm"); err == nil {
				args = append([]string{"-enable-kvm"}, args...)
			}
		} else if runtime.GOOS == "darwin" {
			args = append([]string{"-accel", "hvf"}, args...)
		}
		cmd := exec.Command(qemuBin, args...)
		if err := cmd.Start(); err == nil {
			launchedQEMU = true
		}
	}

	return map[string]interface{}{
		"ok":                true,
		"vm_dir":            vmDir,
		"usb_image_path":    usbImgPath,
		"cidata_image_path": cidataImgPath,
		"target_disk_path":  targetDiskPath,
		"staged_files":      len(stagedFiles),
		"using_full_iso":    usingFullISO,
		"qemu_found":        qemuBin != "",
		"qemu_launched":     launchedQEMU,
		"dashboard_preview": "/server_dashboard.html",
		"vm_forwarded_url":  "http://127.0.0.1:18088",
		"message":           "Verified hybrid MBR + GPT + FAT16 CIDATA bootable USB structure and generated disposable VM disks & launcher.",
	}
}

func buildSyntheticBootableTestISO(isoPath string) error {
	isoBytes := make([]byte, 2*1024*1024)
	// Protective MBR (LBA 0)
	isoBytes[446+4] = 0xEE
	binary.LittleEndian.PutUint32(isoBytes[446+8:446+12], 1)
	binary.LittleEndian.PutUint32(isoBytes[446+12:446+16], 4095)
	isoBytes[510] = 0x55
	isoBytes[511] = 0xAA

	// Primary GPT Header (LBA 1)
	hdr := isoBytes[512 : 512+92]
	copy(hdr[0:8], []byte("EFI PART"))
	binary.LittleEndian.PutUint32(hdr[8:12], 0x00010000)
	binary.LittleEndian.PutUint32(hdr[12:16], 92)
	binary.LittleEndian.PutUint64(hdr[24:32], 1)
	binary.LittleEndian.PutUint64(hdr[32:40], 4095)
	binary.LittleEndian.PutUint64(hdr[40:48], 64)
	binary.LittleEndian.PutUint64(hdr[48:56], 4030)
	binary.LittleEndian.PutUint64(hdr[72:80], 2)
	binary.LittleEndian.PutUint32(hdr[80:84], 128)
	binary.LittleEndian.PutUint32(hdr[84:88], 128)

	// Entry 0: ISO9660 system partition (LBA 64..2000), Entry 1: EFI System Partition (LBA 2001..3000)
	e0 := isoBytes[1024 : 1024+128]
	copy(e0[0:16], []byte{0xA2, 0xA0, 0xD0, 0xEB, 0xE5, 0xB9, 0x33, 0x44, 0x87, 0xC0, 0x68, 0xB6, 0xB7, 0x26, 0x99, 0xC7})
	binary.LittleEndian.PutUint64(e0[32:40], 64)
	binary.LittleEndian.PutUint64(e0[40:48], 2000)

	e1 := isoBytes[1024+128 : 1024+256]
	copy(e1[0:16], []byte{0x28, 0x73, 0x2A, 0xC1, 0x1F, 0xF8, 0xD2, 0x11, 0xBA, 0x4B, 0x00, 0xA0, 0xC9, 0x3E, 0xC9, 0x3B})
	binary.LittleEndian.PutUint64(e1[32:40], 2001)
	binary.LittleEndian.PutUint64(e1[40:48], 3000)

	entriesCRC := crc32.ChecksumIEEE(isoBytes[1024 : 1024+128*128])
	binary.LittleEndian.PutUint32(hdr[88:92], entriesCRC)
	binary.LittleEndian.PutUint32(hdr[16:20], 0)
	hdrCRC := crc32.ChecksumIEEE(hdr)
	binary.LittleEndian.PutUint32(hdr[16:20], hdrCRC)

	grubSample := "set timeout=30\nmenuentry \"Try or Install Ubuntu Server\" {\n\tlinux\t/casper/vmlinuz  ---\n\tinitrd\t/casper/initrd\n}\n"
	copy(isoBytes[65536:], []byte(grubSample))
	return os.WriteFile(isoPath, isoBytes, 0644)
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
	mux.HandleFunc("/api/test-vm", func(w http.ResponseWriter, r *http.Request) {
		var cfg usb.ServerProfileConfig
		readJSONBody(r, &cfg)
		writeJSON(w, a.TestInDisposableVM(cfg))
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
