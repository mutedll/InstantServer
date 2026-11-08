// main.go
package main

import (
	"context"
	"embed"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:src/instant_server/ui all:src/instant_server/scripts
var embeddedAssets embed.FS

func isDirWritable(dir string) bool {
	if err := os.MkdirAll(dir, 0777); err != nil {
		return false
	}
	probe := filepath.Join(dir, ".write_probe")
	if err := os.WriteFile(probe, []byte("ok"), 0666); err != nil {
		return false
	}
	_ = os.Remove(probe)
	return true
}

func resolveIsolatedWebviewDataPath() string {
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		portableCache := filepath.Join(exeDir, "Instant_Server_Data", "webview2_runtime_cache")
		if isDirWritable(portableCache) {
			_ = os.Chmod(portableCache, 0777)
			return portableCache
		}
	}
	if cfgDir, err := os.UserConfigDir(); err == nil {
		userCache := filepath.Join(cfgDir, "InstantServer", "webview2_runtime_cache")
		if isDirWritable(userCache) {
			_ = os.Chmod(userCache, 0777)
			return userCache
		}
	}
	fallbackTmp := filepath.Join(os.TempDir(), "InstantServer_WebView2_SharedCache")
	_ = os.MkdirAll(fallbackTmp, 0777)
	_ = os.Chmod(fallbackTmp, 0777)
	return fallbackTmp
}

func main() {
	app := NewApp(embeddedAssets)
	isolatedWebViewDir := resolveIsolatedWebviewDataPath()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		app.GracefulShutdownSupervisor()
		os.Exit(0)
	}()

	err := wails.Run(&options.App{
		Title:     "Instant Server",
		Width:     1180,
		Height:    820,
		MinWidth:  900,
		MinHeight: 640,
		AssetServer: &assetserver.Options{
			Handler: app.AssetAndAPIRouter(),
		},
		BackgroundColour: &options.RGBA{R: 11, G: 12, B: 14, A: 255},
		OnStartup:        app.startup,
		OnBeforeClose: func(ctx context.Context) (prevent bool) {
			if app.IsFlashing() && !app.IsForceClosing() {
				runtime.EventsEmit(ctx, "flash:close-warning", map[string]interface{}{
					"close_warning_requested": true,
					"message":                 "USB flashing is currently active. Closing now will interrupt writing unless cancelled cleanly.",
				})
				return true
			}
			app.GracefulShutdownSupervisor()
			return false
		},
		OnShutdown: func(ctx context.Context) {
			app.GracefulShutdownSupervisor()
		},
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			WebviewIsTransparent:              false,
			WindowIsTranslucent:               false,
			DisableWindowIcon:                 false,
			DisableFramelessWindowDecorations: false,
			WebviewUserDataPath:               isolatedWebViewDir,
			Theme:                             windows.Dark,
		},
		Mac: &mac.Options{
			TitleBar:             mac.TitleBarDefault(),
			Appearance:           mac.NSAppearanceNameDarkAqua,
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			About: &mac.AboutInfo{
				Title:   "Instant Server",
				Message: "Zero-Touch Bootable Ubuntu LTS Homelab & USB Provisioning Engine",
			},
		},
	})
	if err != nil {
		os.Exit(1)
	}
}
