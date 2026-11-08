// app.go
package main

import (
	"context"
	"embed"
	"encoding/json"
	"io"
	"net/http"
	"os"
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
	ctx          context.Context
	assets       embed.FS
	mu           sync.Mutex
	flashing     bool
	forceClosing bool
	cancelFunc   context.CancelFunc
	pauseGate    *usb.PauseGate
	lastStatus   map[string]interface{}
}

func NewApp(assets embed.FS) *App {
	return &App{
		assets:    assets,
		pauseGate: usb.NewPauseGate(),
		lastStatus: map[string]interface{}{
			"status":  "IDLE",
			"percent": 0,
			"paused":  false,
			"events":  []interface{}{},
		},
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
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
	return privilege.RequestNativePrivilegeEscalation(nil)
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
	dataDir := filepath.Join(".", "Instant_Server_Data")
	_ = os.MkdirAll(dataDir, 0755)
	cfgPath := filepath.Join(dataDir, "instant-server-config.json")
	raw, _ := json.MarshalIndent(profile, "", "  ")
	_ = os.WriteFile(cfgPath, raw, 0600)
	return map[string]interface{}{
		"saved": true,
		"path":  cfgPath,
	}
}

func (a *App) LoadConfig() map[string]interface{} {
	cfgPath := filepath.Join(".", "Instant_Server_Data", "instant-server-config.json")
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

func (a *App) FlashUSBDevice(profile usb.ServerProfileConfig, isoPath string) map[string]interface{} {
	if err := usb.ValidateProfileConfig(profile); err != nil {
		return map[string]interface{}{"started": false, "reason": err.Error()}
	}

	a.mu.Lock()
	if a.flashing {
		a.mu.Unlock()
		return map[string]interface{}{"started": false, "reason": "Flashing is already in progress"}
	}
	if !privilege.IsProcessElevated() {
		esc := privilege.RequestNativePrivilegeEscalation(nil)
		if !esc.AlreadyElevated && !esc.ElevatedSpawned {
			a.mu.Unlock()
			return map[string]interface{}{"started": false, "reason": esc.Message}
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.flashing = true
	a.cancelFunc = cancel
	a.pauseGate.Resume()
	a.lastStatus = map[string]interface{}{
		"status":  "RUNNING",
		"percent": 6,
		"paused":  false,
		"events":  []interface{}{},
	}
	a.mu.Unlock()

	_ = a.SaveConfig(profile)

	go func() {
		defer func() {
			a.mu.Lock()
			a.flashing = false
			a.cancelFunc = nil
			a.mu.Unlock()
		}()

		hardeningBytes, _ := a.assets.ReadFile("src/instant_server/scripts/laptop_hardening.sh")
		dashBytes, _ := a.assets.ReadFile("src/instant_server/ui/server_dashboard.html")
		stagingDir := filepath.Join(".", "Instant_Server_Data", "cidata_stage")
		_, _ = usb.StageCIDATAPayload(stagingDir, profile, hardeningBytes, dashBytes)

		resolvedISO := strings.TrimSpace(isoPath)
		if resolvedISO == "" || func() bool { _, e := os.Stat(resolvedISO); return e != nil }() {
			cacheDir := filepath.Join(".", "Instant_Server_Data", "iso_cache")
			dlPath, dlErr := usb.DownloadISOWithResume(
				ctx,
				profile.UbuntuRelease,
				cacheDir,
				a.pauseGate,
				func(ev usb.FlashProgressEvent) {
					a.mu.Lock()
					a.lastStatus["status"] = "RUNNING"
					a.lastStatus["percent"] = 10 + (ev.Percent*30)/100
					a.lastStatus["paused"] = a.pauseGate.IsPaused()
					a.mu.Unlock()
					if a.ctx != nil {
						wailsRuntime.EventsEmit(a.ctx, "flash:progress", ev)
					}
				},
			)
			if dlErr != nil {
				a.mu.Lock()
				a.lastStatus["status"] = "FAILED"
				a.lastStatus["error"] = dlErr.Error()
				a.mu.Unlock()
				if a.ctx != nil {
					wailsRuntime.EventsEmit(a.ctx, "flash:error", map[string]interface{}{"error": dlErr.Error()})
				}
				return
			}
			resolvedISO = dlPath
		}

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
				a.lastStatus["status"] = "RUNNING"
				a.lastStatus["percent"] = ev.Percent
				a.lastStatus["paused"] = a.pauseGate.IsPaused()
				a.mu.Unlock()
				if a.ctx != nil {
					wailsRuntime.EventsEmit(a.ctx, "flash:progress", ev)
				}
			},
		)

		if err != nil {
			a.mu.Lock()
			a.lastStatus["status"] = "FAILED"
			a.lastStatus["error"] = err.Error()
			a.mu.Unlock()
			if a.ctx != nil {
				wailsRuntime.EventsEmit(a.ctx, "flash:error", map[string]interface{}{
					"error": err.Error(),
				})
			}
			return
		}

		a.mu.Lock()
		a.lastStatus["status"] = "COMPLETED"
		a.lastStatus["percent"] = 100
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
	a.lastStatus["status"] = "PAUSED"
	a.lastStatus["paused"] = true
	a.mu.Unlock()
	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, "flash:paused", map[string]interface{}{"paused": true})
	}
	return map[string]interface{}{"paused": true, "status": "PAUSED"}
}

func (a *App) ResumeFlashing() map[string]interface{} {
	a.pauseGate.Resume()
	a.mu.Lock()
	a.lastStatus["status"] = "RUNNING"
	a.lastStatus["paused"] = false
	a.mu.Unlock()
	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, "flash:resumed", map[string]interface{}{"paused": false})
	}
	return map[string]interface{}{"resumed": true, "status": "RUNNING"}
}

func (a *App) CancelFlashing() map[string]interface{} {
	a.mu.Lock()
	cancel := a.cancelFunc
	a.pauseGate.Resume()
	a.lastStatus["status"] = "CANCELLED"
	a.lastStatus["paused"] = false
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return map[string]interface{}{"cancelled": true, "status": "CANCELLED"}
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
		writeJSON(w, a.FlashUSBDevice(cfg, ""))
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