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
	mirror, ok := OfficialUbuntuMirrors[strings.TrimSpace(release)]
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mirror.PrimaryURL, nil)
	if err != nil {
		return "", err
	}
	if existingOffset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existingOffset))
	}

	resp, err := client.Do(req)
	if err != nil || (resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent) {
		if resp != nil {
			_ = resp.Body.Close()
		}
		reqFallback, fbErr := http.NewRequestWithContext(ctx, http.MethodGet, mirror.MirrorURL, nil)
		if fbErr != nil {
			return "", fmt.Errorf("IS-ISO-106: failed reaching Ubuntu release mirror: %w", err)
		}
		if existingOffset > 0 {
			reqFallback.Header.Set("Range", fmt.Sprintf("bytes=%d-", existingOffset))
		}
		resp, err = client.Do(reqFallback)
		if err != nil {
			return "", fmt.Errorf("IS-ISO-106: Ubuntu mirror download failed: %w", err)
		}
	}
	defer resp.Body.Close()

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
			if onProgress != nil {
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
			return "", fmt.Errorf("IS-ISO-107: stream interrupted at %d bytes (saved to .part for instant resume): %w", downloaded, rErr)
		}
	}

	_ = outFile.Sync()
	_ = outFile.Close()
	if err := os.Rename(partPath, finalPath); err != nil {
		return "", err
	}
	return finalPath, nil
}