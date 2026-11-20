// ============================================================================
// FILE: app_flash.go
// PURPOSE: Orchestrates the 5-stage USB provisioning pipeline, config saving,
//
//	pause/resume/cancel gates, and Wails progress event emission.
//
// ============================================================================
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mutedll/InstantServer/privilege"
	"github.com/mutedll/InstantServer/usb"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

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
		hardeningBytes, errH := a.assets.ReadFile("src/instant_server/scripts/os/laptop_hardening.sh")
		if errH != nil {
			hardeningBytes, _ = a.assets.ReadFile("src/instant_server/scripts/laptop_hardening.sh")
		}
		dashBytes, errD := a.assets.ReadFile("src/instant_server/ui/dashboard/server_dashboard.html")
		if errD != nil {
			dashBytes, _ = a.assets.ReadFile("src/instant_server/ui/server_dashboard.html")
		}
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
					stPct := ev.StagePercent
					if stPct <= 0 && ev.Percent > 0 {
						stPct = float64(ev.Percent)
					}
					overallPct := 10 + int((stPct*35.0)/100.0)
					a.lastStatus["percent"] = overallPct
					a.lastStatus["stage_index"] = 3
					a.lastStatus["active_progress"] = map[string]interface{}{
						"stage_percent": stPct,
						"stage_index":   3,
						"eta_sec":       ev.ETASeconds,
						"label":         "Downloading Ubuntu ISO",
						"message":       ev.Message,
						"terminal_line": fmt.Sprintf("[ISO] Downloading Ubuntu ISO: %.1f%% | %s", stPct, ev.Message),
					}
					a.mu.Unlock()
					if a.ctx != nil {
						wailsRuntime.EventsEmit(a.ctx, "flash:progress", map[string]interface{}{
							"percent":       overallPct,
							"stage_percent": stPct,
							"stage_index":   3,
							"eta_sec":       ev.ETASeconds,
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
				stPct := ev.StagePercent
				if stPct <= 0 && ev.Percent > 0 {
					stPct = float64(ev.Percent)
				}
				overallPct := 48 + int((stPct*50.0)/100.0)
				a.lastStatus["percent"] = overallPct
				a.lastStatus["stage_index"] = curStageIdx
				a.lastStatus["active_progress"] = map[string]interface{}{
					"stage_percent": stPct,
					"stage_index":   curStageIdx,
					"eta_sec":       ev.ETASeconds,
					"label":         "Writing Bootable USB",
					"message":       ev.Message,
					"terminal_line": fmt.Sprintf("[USB] Writing Bootable USB: %.1f%% | %s (CRC32 %s)", stPct, ev.Message, ev.CRC32Hex),
				}
				a.mu.Unlock()
				if a.ctx != nil {
					wailsRuntime.EventsEmit(a.ctx, "flash:progress", map[string]interface{}{
						"percent":       overallPct,
						"stage_percent": stPct,
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
