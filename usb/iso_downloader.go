// usb/iso_downloader.go
package usb

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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
	var existingOffset int64
	if st, err := os.Stat(partPath); err == nil {
		existingOffset = st.Size()
	}

	client := &http.Client{Timeout: 0}
	tryOpenURL := func(url string) (*http.Response, error) {
		req, rErr := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if rErr != nil {
			return nil, rErr
		}
		if existingOffset > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existingOffset))
		}
		return client.Do(req)
	}

	resp, err := tryOpenURL(mirror.PrimaryURL)
	if err != nil || (resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent) {
		if resp != nil {
			_ = resp.Body.Close()
		}
		if dynFile, dynURL := discoverLiveISOFromReleaseIndex(ctx, cleanRel); dynURL != "" {
			mirror.Filename = dynFile
			finalPath = filepath.Join(cacheDir, dynFile)
			partPath = finalPath + ".part"
			if st, sErr := os.Stat(partPath); sErr == nil {
				existingOffset = st.Size()
			} else {
				existingOffset = 0
			}
			resp, err = tryOpenURL(dynURL)
		}
	}
	if err != nil || resp == nil || (resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent) {
		if resp != nil {
			_ = resp.Body.Close()
		}
		resp, err = tryOpenURL(mirror.MirrorURL)
		if err != nil {
			return "", fmt.Errorf("IS-ISO-106: Ubuntu mirror download failed: %w", err)
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return "", fmt.Errorf("IS-ISO-106: Ubuntu release mirror returned HTTP %d", resp.StatusCode)
	}

	flags := os.O_CREATE | os.O_WRONLY
	if resp.StatusCode == http.StatusPartialContent && existingOffset > 0 {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
		existingOffset = 0
	}

	outFile, err := os.OpenFile(partPath, flags, 0644)
	if err != nil {
		return "", err
	}
	defer outFile.Close()

	totalSize := existingOffset + resp.ContentLength
	if totalSize <= 0 {
		totalSize = 2640 * 1024 * 1024
	}

	buf := make([]byte, 512*1024)
	downloaded := existingOffset
	startTime := time.Now()
	var lastProgressEmit time.Time

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

		n, rErr := resp.Body.Read(buf)
		if n > 0 {
			if _, wErr := outFile.Write(buf[:n]); wErr != nil {
				return "", wErr
			}
			downloaded += int64(n)
			elapsed := time.Since(startTime).Seconds()
			if elapsed <= 0.001 {
				elapsed = 0.001
			}
			speedMB := (float64(downloaded-existingOffset) / 1048576.0) / elapsed
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
		if errors.Is(rErr, io.EOF) {
			break
		}
		if rErr != nil {
			return "", fmt.Errorf("IS-ISO-107: download interrupted at %d bytes (saved as .part): %w", downloaded, rErr)
		}
	}

	_ = outFile.Sync()
	_ = outFile.Close()
	if err := os.Rename(partPath, finalPath); err != nil {
		return "", err
	}
	return finalPath, nil
}
