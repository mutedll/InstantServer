// ============================================================================
// FILE: app_router.go
// PURPOSE: HTTP AssetServer router serving the modular UI (.html, .css, .js),
//
//	mDNS LAN server discovery, and ISO cache management endpoints.
//
// ============================================================================
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mutedll/InstantServer/network"
	"github.com/mutedll/InstantServer/usb"
)

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

	readUIAsset := func(baseName string) ([]byte, error) {
		subdirs := []string{"", "styles", "core", "views", "dashboard"}
		for _, sub := range subdirs {
			embedPath := "src/instant_server/ui/" + baseName
			diskPath := filepath.Join("src", "instant_server", "ui", baseName)
			if sub != "" {
				embedPath = "src/instant_server/ui/" + sub + "/" + baseName
				diskPath = filepath.Join("src", "instant_server", "ui", sub, baseName)
			}
			if data, err := a.assets.ReadFile(embedPath); err == nil {
				return data, nil
			}
			if data, err := os.ReadFile(diskPath); err == nil {
				return data, nil
			}
		}
		return nil, os.ErrNotExist
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		reqPath := strings.TrimPrefix(r.URL.Path, "/")
		if reqPath == "" || reqPath == "index.html" || reqPath == "app_ui.html" {
			if data, err := readUIAsset("app_ui.html"); err == nil {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = w.Write(data)
				return
			}
		}
		if filepath.Base(reqPath) == "server_dashboard.html" {
			if data, err := readUIAsset("server_dashboard.html"); err == nil {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = w.Write(data)
				return
			}
		}
		if strings.HasSuffix(reqPath, ".js") || strings.HasSuffix(reqPath, ".css") {
			cleanBase := filepath.Base(reqPath)
			if data, err := readUIAsset(cleanBase); err == nil {
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
