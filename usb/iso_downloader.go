// usb/iso_downloader.go
package usb

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
)

type UbuntuReleaseMirror struct {
	Release    string
	Point      string
	Filename   string
	PrimaryURL string
	MirrorURL  string
	SumsURL    string
}

var OfficialUbuntuMirrors = map[string]UbuntuReleaseMirror{
	"26.04": {
		Release:    "26.04",
		Point:      "26.04.1",
		Filename:   "ubuntu-26.04.1-live-server-amd64.iso",
		PrimaryURL: "https://releases.ubuntu.com/26.04/ubuntu-26.04.1-live-server-amd64.iso",
		MirrorURL:  "https://releases.ubuntu.com/24.04/ubuntu-24.04.5-live-server-amd64.iso",
		SumsURL:    "https://releases.ubuntu.com/26.04/SHA256SUMS",
	},
	"24.04": {
		Release:    "24.04",
		Point:      "24.04.5",
		Filename:   "ubuntu-24.04.5-live-server-amd64.iso",
		PrimaryURL: "https://releases.ubuntu.com/24.04/ubuntu-24.04.5-live-server-amd64.iso",
		MirrorURL:  "https://old-releases.ubuntu.com/releases/24.04/ubuntu-24.04.5-live-server-amd64.iso",
		SumsURL:    "https://releases.ubuntu.com/24.04/SHA256SUMS",
	},
	"22.04": {
		Release:    "22.04",
		Point:      "22.04.5",
		Filename:   "ubuntu-22.04.5-live-server-amd64.iso",
		PrimaryURL: "https://releases.ubuntu.com/22.04/ubuntu-22.04.5-live-server-amd64.iso",
		MirrorURL:  "https://old-releases.ubuntu.com/releases/22.04/ubuntu-22.04.5-live-server-amd64.iso",
		SumsURL:    "https://releases.ubuntu.com/22.04/SHA256SUMS",
	},
}

var isoHrefRegex = regexp.MustCompile(`href="(ubuntu-[0-9.]+(?:-beta)?-live-server-amd64\.iso)"`)

func discoverLiveISOFromReleaseIndex(ctx context.Context, release string) (string, string) {
	for _, rel := range []string{release, "24.04", "22.04"} {
		if rel == "" {
			continue
		}
		baseURL := "https://releases.ubuntu.com/" + rel + "/"
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL, nil)
		if err != nil {
			continue
		}
		client := &http.Client{Timeout: 8 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			matches := isoHrefRegex.FindStringSubmatch(string(body))
			if len(matches) >= 2 {
				fn := matches[1]
				return fn, baseURL + fn
			}
		}
	}
	return "", ""
}

func ComputeFileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	buf := make([]byte, 1024*1024)
	for {
		n, rErr := f.Read(buf)
		if n > 0 {
			_, _ = h.Write(buf[:n])
		}
		if errors.Is(rErr, io.EOF) {
			break
		}
		if rErr != nil {
			return "", rErr
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func newResilientHTTPClient() *http.Client {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 15 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		ExpectContinueTimeout: 2 * time.Second,
		IdleConnTimeout:       30 * time.Second,
		DisableKeepAlives:     true, // Always use a fresh socket on resume/reconnect after Wi-Fi change
	}
	return &http.Client{
		Transport: transport,
		Timeout:   0,
	}
}

func DownloadISOWithResume(
	ctx context.Context,
	release string,
	cacheDir string,
	pauseGate *PauseGate,
	onProgress func(FlashProgressEvent),
) (string, error) {
	releaseWakeLock := AcquireHostWakeLock()
	defer releaseWakeLock()

	cleanRel := strings.TrimSpace(release)
	mirror, ok := OfficialUbuntuMirrors[cleanRel]
	if !ok {
		mirror = OfficialUbuntuMirrors["26.04"]
	}

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", fmt.Errorf("IS-ISO-105: cannot create ISO cache directory %s: %w", cacheDir, err)
	}

	finalPath := filepath.Join(cacheDir, mirror.Filename)
	if st, err := os.Stat(finalPath); err == nil && st.Size() > 100*1024*1024 {
		if onProgress != nil {
			onProgress(FlashProgressEvent{
				Stage:        "ISO_CACHE_HIT",
				Percent:      100,
				WrittenBytes: st.Size(),
				TotalBytes:   st.Size(),
				Message:      fmt.Sprintf("Using cached %s (%.1f MB)", mirror.Filename, float64(st.Size())/1048576.0),
			})
		}
		return finalPath, nil
	}

	partPath := finalPath + ".part"
	var downloaded int64
	if st, err := os.Stat(partPath); err == nil {
		downloaded = st.Size()
	}

	var totalSize int64 = 2640 * 1024 * 1024
	activeURL := mirror.PrimaryURL
	buf := make([]byte, 512*1024)
	var lastProgressEmit time.Time

	const maxConsecutiveNetRetries = 60
	retryCount := 0

	for {
		if pauseGate != nil {
			if err := pauseGate.WaitIfPaused(ctx); err != nil {
				return "", err
			}
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		if st, err := os.Stat(partPath); err == nil {
			downloaded = st.Size()
		}

		client := newResilientHTTPClient()
		attemptCtx, cancelAttempt := context.WithCancel(ctx)
		var lastActivityNano int64
		atomic.StoreInt64(&lastActivityNano, time.Now().UnixNano())

		// Watchdog closes stalled TCP sockets within 12 seconds if Wi-Fi drops silently
		watchdogDone := make(chan struct{})
		go func() {
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-watchdogDone:
					return
				case <-attemptCtx.Done():
					return
				case <-ticker.C:
					if pauseGate != nil && pauseGate.IsPaused() {
						// Close the open HTTP stream immediately on Pause so Resume always opens a fresh socket
						cancelAttempt()
						return
					}
					last := time.Unix(0, atomic.LoadInt64(&lastActivityNano))
					if time.Since(last) > 12*time.Second {
						cancelAttempt()
						return
					}
				}
			}
		}()

		tryOpen := func(u string) (*http.Response, error) {
			req, rErr := http.NewRequestWithContext(attemptCtx, http.MethodGet, u, nil)
			if rErr != nil {
				return nil, rErr
			}
			if downloaded > 0 {
				req.Header.Set("Range", fmt.Sprintf("bytes=%d-", downloaded))
			}
			return client.Do(req)
		}

		resp, err := tryOpen(activeURL)
		if err != nil || (resp != nil && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent) {
			if resp != nil {
				_ = resp.Body.Close()
			}
			if dynFile, dynURL := discoverLiveISOFromReleaseIndex(attemptCtx, cleanRel); dynURL != "" {
				mirror.Filename = dynFile
				finalPath = filepath.Join(cacheDir, dynFile)
				partPath = finalPath + ".part"
				if st, sErr := os.Stat(partPath); sErr == nil {
					downloaded = st.Size()
				}
				activeURL = dynURL
				resp, err = tryOpen(activeURL)
			}
		}
		if err != nil || resp == nil || (resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent) {
			if resp != nil {
				_ = resp.Body.Close()
			}
			activeURL = mirror.MirrorURL
			resp, err = tryOpen(activeURL)
		}

		if err != nil || resp == nil || (resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent) {
			if resp != nil {
				_ = resp.Body.Close()
			}
			close(watchdogDone)
			cancelAttempt()

			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			if pauseGate != nil && pauseGate.IsPaused() {
				continue
			}

			retryCount++
			if retryCount > maxConsecutiveNetRetries {
				if err != nil {
					return "", fmt.Errorf("IS-ISO-106: Ubuntu mirror download interrupted at %d bytes (saved as .part): %w", downloaded, err)
				}
				return "", fmt.Errorf("IS-ISO-106: Ubuntu release mirror unreachable at %d bytes (saved as .part)", downloaded)
			}
			if onProgress != nil {
				pct := int((downloaded * 100) / totalSize)
				if pct > 99 {
					pct = 99
				}
				onProgress(FlashProgressEvent{
					Stage:        "DOWNLOADING_ISO",
					Percent:      pct,
					WrittenBytes: downloaded,
					TotalBytes:   totalSize,
					SpeedMBps:    0,
					Message:      fmt.Sprintf("Waiting for Wi-Fi / network to resume from %.1f MB (saved as .part)...", float64(downloaded)/1048576.0),
				})
			}
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(2 * time.Second):
			}
			continue
		}

		flags := os.O_CREATE | os.O_WRONLY
		if resp.StatusCode == http.StatusPartialContent && downloaded > 0 {
			flags |= os.O_APPEND
		} else if resp.StatusCode == http.StatusOK {
			flags |= os.O_TRUNC
			downloaded = 0
		}

		if resp.ContentLength > 0 {
			totalSize = downloaded + resp.ContentLength
		}

		outFile, fErr := os.OpenFile(partPath, flags, 0644)
		if fErr != nil {
			_ = resp.Body.Close()
			close(watchdogDone)
			cancelAttempt()
			return "", fErr
		}

		streamStartTime := time.Now()
		streamStartBytes := downloaded
		var readErr error

		for {
			if pauseGate != nil && pauseGate.IsPaused() {
				readErr = errors.New("paused")
				break
			}
			n, rErr := resp.Body.Read(buf)
			if n > 0 {
				atomic.StoreInt64(&lastActivityNano, time.Now().UnixNano())
				retryCount = 0 // Reset network retry counter whenever bytes arrive
				if _, wErr := outFile.Write(buf[:n]); wErr != nil {
					_ = outFile.Close()
					_ = resp.Body.Close()
					close(watchdogDone)
					cancelAttempt()
					return "", wErr
				}
				downloaded += int64(n)
				elapsed := time.Since(streamStartTime).Seconds()
				if elapsed <= 0.001 {
					elapsed = 0.001
				}
				speedMB := (float64(downloaded-streamStartBytes) / 1048576.0) / elapsed
				pct := int((downloaded * 100) / totalSize)
				if pct > 100 {
					pct = 100
				}
				if onProgress != nil && (time.Since(lastProgressEmit) >= 160*time.Millisecond || pct >= 100 || errors.Is(rErr, io.EOF)) {
					lastProgressEmit = time.Now()
					onProgress(FlashProgressEvent{
						Stage:        "DOWNLOADING_ISO",
						Percent:      pct,
						WrittenBytes: downloaded,
						TotalBytes:   totalSize,
						SpeedMBps:    speedMB,
						Message:      fmt.Sprintf("Downloading %s: %.1f / %.1f MB @ %.1f MB/s", mirror.Filename, float64(downloaded)/1048576.0, float64(totalSize)/1048576.0, speedMB),
					})
				}
			}
			if rErr != nil {
				readErr = rErr
				break
			}
		}

		_ = outFile.Sync()
		_ = outFile.Close()
		_ = resp.Body.Close()
		close(watchdogDone)
		cancelAttempt()

		if errors.Is(readErr, io.EOF) && downloaded >= totalSize {
			break
		}
		if ctx.Err() != nil {
			return "", fmt.Errorf("IS-ISO-107: download interrupted at %d bytes (saved as .part): %w", downloaded, ctx.Err())
		}
		if pauseGate != nil && pauseGate.IsPaused() {
			// Wait cleanly while paused, then loop around to open a fresh HTTP Range connection on Resume
			continue
		}

		retryCount++
		if retryCount > maxConsecutiveNetRetries {
			return "", fmt.Errorf("IS-ISO-107: download interrupted at %d bytes (saved as .part): %w", downloaded, readErr)
		}
		if onProgress != nil {
			pct := int((downloaded * 100) / totalSize)
			if pct > 99 {
				pct = 99
			}
			onProgress(FlashProgressEvent{
				Stage:        "DOWNLOADING_ISO",
				Percent:      pct,
				WrittenBytes: downloaded,
				TotalBytes:   totalSize,
				SpeedMBps:    0,
				Message:      fmt.Sprintf("Reconnecting & resuming download from %.1f MB (saved as .part)...", float64(downloaded)/1048576.0),
			})
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(1500 * time.Millisecond):
		}
	}

	if err := os.Rename(partPath, finalPath); err != nil {
		return "", err
	}
	return finalPath, nil
}
