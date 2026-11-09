// main.go
package main

import (
	"bytes"
	"context"
	"embed"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:src/instant_server/ui all:src/instant_server/scripts
var embeddedAssets embed.FS

func inRoundedRect(x, y, x0, y0, x1, y1, r int) bool {
	if x < x0 || x >= x1 || y < y0 || y >= y1 {
		return false
	}
	if (x >= x0+r && x < x1-r) || (y >= y0+r && y < y1-r) {
		return true
	}
	cx := x0 + r
	if x >= x1-r {
		cx = x1 - r - 1
	}
	cy := y0 + r
	if y >= y1-r {
		cy = y1 - r - 1
	}
	dx := x - cx
	dy := y - cy
	return dx*dx+dy*dy <= r*r
}

func isISGlyphPixel(nx, ny float64) bool {
	// Letter "I"
	if ny >= 0.26 && ny <= 0.335 && nx >= 0.20 && nx <= 0.44 {
		return true
	}
	if ny >= 0.665 && ny <= 0.74 && nx >= 0.20 && nx <= 0.44 {
		return true
	}
	if ny >= 0.26 && ny <= 0.74 && nx >= 0.285 && nx <= 0.355 {
		return true
	}
	// Letter "S"
	if ny >= 0.26 && ny <= 0.335 && nx >= 0.50 && nx <= 0.80 {
		return true
	}
	if ny >= 0.26 && ny <= 0.538 && nx >= 0.50 && nx <= 0.575 {
		return true
	}
	if ny >= 0.462 && ny <= 0.538 && nx >= 0.50 && nx <= 0.80 {
		return true
	}
	if ny >= 0.462 && ny <= 0.74 && nx >= 0.725 && nx <= 0.80 {
		return true
	}
	if ny >= 0.665 && ny <= 0.74 && nx >= 0.50 && nx <= 0.80 {
		return true
	}
	return false
}

// buildInstantServerIconPNG renders the "IS" (Instant Server) emblem as an RGBA PNG
// for the native window titlebar, dock/panel, and About dialog.
func buildInstantServerIconPNG(size int) []byte {
	if size <= 0 {
		size = 256
	}
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	radius := int(float64(size) * 0.21)
	border := int(float64(size) * 0.045)
	if border < 2 {
		border = 2
	}
	margin := int(float64(size) * 0.03)
	if margin < 1 {
		margin = 1
	}

	x0, y0, x1, y1 := margin, margin, size-margin, size-margin
	ix0, iy0, ix1, iy1 := x0+border, y0+border, x1-border, y1-border
	ir := radius - border
	if ir < 2 {
		ir = 2
	}

	colBorder := color.RGBA{R: 0x31, G: 0xA8, B: 0xFF, A: 255}
	colGlyph := color.RGBA{R: 0x66, G: 0xC4, B: 0xFF, A: 255}

	for y := 0; y < size; y++ {
		ny := float64(y) / float64(size)
		bVal := uint8(0x36 + int((1.0-ny)*14.0))
		colBg := color.RGBA{R: 0x00, G: 0x1E, B: bVal, A: 255}
		for x := 0; x < size; x++ {
			nx := float64(x) / float64(size)
			if !inRoundedRect(x, y, x0, y0, x1, y1, radius) {
				img.SetRGBA(x, y, color.RGBA{0, 0, 0, 0})
			} else if !inRoundedRect(x, y, ix0, iy0, ix1, iy1, ir) {
				img.SetRGBA(x, y, colBorder)
			} else if isISGlyphPixel(nx, ny) {
				img.SetRGBA(x, y, colGlyph)
			} else {
				img.SetRGBA(x, y, colBg)
			}
		}
	}

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

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
	iconBytes := buildInstantServerIconPNG(256)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		app.GracefulShutdownSupervisor()
		os.Exit(0)
	}()

	err := wails.Run(&options.App{
		Title:     "Instant Server",
		Width:     960,
		Height:    600,
		MinWidth:  720,
		MinHeight: 460,
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
				Icon:    iconBytes,
			},
		},
		Linux: &linux.Options{
			Icon:                iconBytes,
			WindowIsTranslucent: false,
			WebviewGpuPolicy:    linux.WebviewGpuPolicyAlways,
			ProgramName:         "InstantServer",
		},
	})
	if err != nil {
		os.Exit(1)
	}
}
