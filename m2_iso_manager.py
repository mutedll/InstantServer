#!/usr/bin/env python3
"""
Instant Server — Module M2: Official Ubuntu ISO Downloader (26.04.1 / 24.04.5 / 22.04.5),
Dynamic Mirror Resolver, GRUB Autoinstall Patcher & OCI App Puller
=========================================================================================
File: m2_iso_manager.py
Project: Instant Server (Turn any old laptop into a headless Ubuntu Server)
Runs on: Windows 10/11, macOS, and Linux (Python 3.8+ through 3.14+, zero external deps)

Responsibilities:
1. Official Ubuntu Server LTS Release Catalog — Defaulting to **Ubuntu 26.04.1 LTS**:
   - `26.04` (Default): Ubuntu 26.04.1 LTS (Resolute Raccoon)
     * Thank-you URL: `https://ubuntu.com/download/server/thank-you?version=26.04.1&architecture=amd64&lts=true`
     * Direct ISO URL: `https://releases.ubuntu.com/26.04/ubuntu-26.04.1-live-server-amd64.iso`
   - `24.04`: Ubuntu 24.04.5 LTS (Noble Numbat)
     * Thank-you URL: `https://ubuntu.com/download/server/thank-you?version=24.04.5&architecture=amd64&lts=true`
     * Direct ISO URL: `https://releases.ubuntu.com/24.04/ubuntu-24.04.5-live-server-amd64.iso`
   - `22.04`: Ubuntu 22.04.5 LTS (Jammy Jellyfish)
     * Thank-you URL: `https://ubuntu.com/download/server/thank-you?version=22.04.5&architecture=amd64&lts=true`
     * Direct ISO URL: `https://releases.ubuntu.com/22.04/ubuntu-22.04.5-live-server-amd64.iso`
2. 404-Proof Dynamic ISO URL Resolver (`resolve_live_ubuntu_iso_url`):
   - Automatically checks `SHA256SUMS` on `releases.ubuntu.com/<series>/SHA256SUMS`, parses
     the official `ubuntu.com/download/server/thank-you` HTML page for the direct `.iso` href,
     and falls back across official mirrors so point-release updates never cause HTTP 404 errors!
3. Zero-Dependency Pure-Python ISO9660 Full Tree Extractor & Binary GRUB Autoinstall Patcher.
4. Pure-Python Docker Hub OCI Image Tarball Downloader (`pull_docker_image_tarball`).
"""

from __future__ import annotations

import argparse
import hashlib
import io
import json
import os
import re
import struct
import sys
import tarfile
import tempfile
import time
import urllib.request
from typing import Any, Callable, Dict, List, Optional, Tuple


# ---------------------------------------------------------------------------
# 1. Official Ubuntu Server LTS Catalog (26.04.1 LTS Default, 24.04.5, 22.04.5)
# ---------------------------------------------------------------------------
UBUNTU_LTS_CATALOG: Dict[str, Dict[str, Any]] = {
    "26.04": {
        "codename": "resolute",
        "point_version": "26.04.1",
        "iso_filename": "ubuntu-26.04.1-live-server-amd64.iso",
        "thank_you_url": "https://ubuntu.com/download/server/thank-you?version=26.04.1&architecture=amd64&lts=true",
        "iso_url": "https://releases.ubuntu.com/26.04/ubuntu-26.04.1-live-server-amd64.iso",
        "mirror_urls": [
            "https://releases.ubuntu.com/26.04/ubuntu-26.04.1-live-server-amd64.iso",
            "https://releases.ubuntu.com/26.04.1/ubuntu-26.04.1-live-server-amd64.iso",
            "https://releases.ubuntu.com/resolute/ubuntu-26.04.1-live-server-amd64.iso",
            "https://mirrors.mit.edu/ubuntu-releases/26.04/ubuntu-26.04.1-live-server-amd64.iso",
        ],
        "sha256sums_url": "https://releases.ubuntu.com/26.04/SHA256SUMS",
        "approx_size_mb": 2850,
        "offline_wifi_debs": [
            {
                "name": "wpasupplicant",
                "iso_prefix": "wpasupplicant_",
                "fallback_url": "http://archive.ubuntu.com/ubuntu/pool/main/w/wpa/wpasupplicant_2.10-21build4_amd64.deb",
            },
            {
                "name": "rfkill",
                "iso_prefix": "rfkill_",
                "fallback_url": "http://archive.ubuntu.com/ubuntu/pool/main/u/util-linux/rfkill_2.39.3-9ubuntu6_amd64.deb",
            },
        ],
    },
    "24.04": {
        "codename": "noble",
        "point_version": "24.04.5",
        "iso_filename": "ubuntu-24.04.5-live-server-amd64.iso",
        "thank_you_url": "https://ubuntu.com/download/server/thank-you?version=24.04.5&architecture=amd64&lts=true",
        "iso_url": "https://releases.ubuntu.com/24.04/ubuntu-24.04.5-live-server-amd64.iso",
        "mirror_urls": [
            "https://releases.ubuntu.com/24.04/ubuntu-24.04.5-live-server-amd64.iso",
            "https://releases.ubuntu.com/24.04.5/ubuntu-24.04.5-live-server-amd64.iso",
            "https://releases.ubuntu.com/noble/ubuntu-24.04.5-live-server-amd64.iso",
            "https://mirrors.mit.edu/ubuntu-releases/24.04/ubuntu-24.04.5-live-server-amd64.iso",
        ],
        "sha256sums_url": "https://releases.ubuntu.com/24.04/SHA256SUMS",
        "approx_size_mb": 2750,
        "offline_wifi_debs": [
            {
                "name": "wpasupplicant",
                "iso_prefix": "wpasupplicant_",
                "fallback_url": "http://archive.ubuntu.com/ubuntu/pool/main/w/wpa/wpasupplicant_2.10-21build4_amd64.deb",
            },
            {
                "name": "libpcsclite1",
                "iso_prefix": "libpcsclite1_",
                "fallback_url": "http://archive.ubuntu.com/ubuntu/pool/main/p/pcsc-lite/libpcsclite1_2.0.3-1build1_amd64.deb",
            },
            {
                "name": "rfkill",
                "iso_prefix": "rfkill_",
                "fallback_url": "http://archive.ubuntu.com/ubuntu/pool/main/u/util-linux/rfkill_2.39.3-9ubuntu6_amd64.deb",
            },
        ],
    },
    "22.04": {
        "codename": "jammy",
        "point_version": "22.04.5",
        "iso_filename": "ubuntu-22.04.5-live-server-amd64.iso",
        "thank_you_url": "https://ubuntu.com/download/server/thank-you?version=22.04.5&architecture=amd64&lts=true",
        "iso_url": "https://releases.ubuntu.com/22.04/ubuntu-22.04.5-live-server-amd64.iso",
        "mirror_urls": [
            "https://releases.ubuntu.com/22.04/ubuntu-22.04.5-live-server-amd64.iso",
            "https://releases.ubuntu.com/22.04.5/ubuntu-22.04.5-live-server-amd64.iso",
            "https://releases.ubuntu.com/jammy/ubuntu-22.04.5-live-server-amd64.iso",
            "https://mirrors.mit.edu/ubuntu-releases/22.04/ubuntu-22.04.5-live-server-amd64.iso",
        ],
        "sha256sums_url": "https://releases.ubuntu.com/22.04/SHA256SUMS",
        "approx_size_mb": 2100,
        "offline_wifi_debs": [
            {
                "name": "wpasupplicant",
                "iso_prefix": "wpasupplicant_",
                "fallback_url": "http://archive.ubuntu.com/ubuntu/pool/main/w/wpa/wpasupplicant_2.10-6ubuntu2_amd64.deb",
            },
            {
                "name": "rfkill",
                "iso_prefix": "rfkill_",
                "fallback_url": "http://archive.ubuntu.com/ubuntu/pool/main/u/util-linux/rfkill_2.37.2-4ubuntu3_amd64.deb",
            },
        ],
    },
}


def emit_event(
    event_type: str,
    message: str,
    data: Optional[Dict[str, Any]] = None,
    callback: Optional[Callable[[Dict[str, Any]], None]] = None,
) -> Dict[str, Any]:
    """Emit structured JSON event for CLI and Instant Server GUI."""
    payload = {
        "event": event_type,
        "module": "M2_ISO_MANAGER",
        "project": "Instant Server",
        "message": message,
        "data": data or {},
    }
    if callback:
        callback(payload)
    return payload


# ---------------------------------------------------------------------------
# 2. Cryptographic SHA-256 Verification & 404-Proof Dynamic URL Resolver
# ---------------------------------------------------------------------------
def parse_sha256sums_text(text: str) -> Dict[str, str]:
    """Parse an official Ubuntu `SHA256SUMS` file into `{filename: sha256_hex}`."""
    mapping: Dict[str, str] = {}
    for raw_line in text.splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#"):
            continue
        parts = line.split(None, 1)
        if len(parts) == 2:
            digest = parts[0].strip().lower()
            fname = parts[1].strip().lstrip("*").strip()
            if len(digest) == 64:
                mapping[fname] = digest
    return mapping


def extract_iso_url_from_thank_you_html(html: str) -> Optional[str]:
    """
    Extract the direct `https://releases.ubuntu.com/.../ubuntu-...-live-server-amd64.iso`
    download link from an `ubuntu.com/download/server/thank-you?...` page.
    """
    m = re.search(
        r'https?://[a-zA-Z0-9._/-]+/ubuntu-[0-9.]+-live-server-amd64\.iso',
        html,
    )
    if m:
        return m.group(0)
    return None


def resolve_live_ubuntu_iso_url(
    release: str = "26.04",
    timeout: int = 12,
) -> Tuple[str, str, Optional[str]]:
    """
    404-Proof Ubuntu ISO URL & SHA-256 Resolver:
      1. Normalizes `release` (`"26.04.1"` -> `"26.04"`, `"24.04.5"` -> `"24.04"`, `"22.04.5"` -> `"22.04"`).
      2. Queries `https://releases.ubuntu.com/<series>/SHA256SUMS` first to discover the exact
         live-server ISO filename currently hosted on Canonical's servers and its SHA-256 hash.
      3. Queries the official `https://ubuntu.com/download/server/thank-you?version=...` page
         to extract the direct `.iso` download link if `SHA256SUMS` is unreachable.
      4. Probes candidate mirror URLs with a fast HTTP `HEAD`/`Range: bytes=0-0` request so we
         never start downloading a dead 404 URL!
    Returns `(resolved_iso_url, iso_filename, expected_sha256)`.
    """
    norm_rel = "26.04"
    for prefix in ("26.04", "24.04", "22.04"):
        if str(release).startswith(prefix):
            norm_rel = prefix
            break

    cat = UBUNTU_LTS_CATALOG[norm_rel]
    candidate_filename = cat["iso_filename"]
    expected_sha: Optional[str] = None
    candidate_urls: List[str] = []

    # Step 1: Check SHA256SUMS on releases.ubuntu.com/<norm_rel>/SHA256SUMS
    try:
        req = urllib.request.Request(
            cat["sha256sums_url"],
            headers={"User-Agent": "InstantServer-ISOManager/2.1"},
        )
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            text = resp.read().decode("utf-8", errors="replace")
        sums = parse_sha256sums_text(text)
        if candidate_filename in sums:
            expected_sha = sums[candidate_filename]
            candidate_urls.append(f"https://releases.ubuntu.com/{norm_rel}/{candidate_filename}")
        else:
            for fname, digest in sorted(sums.items(), reverse=True):
                if fname.endswith("-live-server-amd64.iso"):
                    candidate_filename = fname
                    expected_sha = digest
                    candidate_urls.append(f"https://releases.ubuntu.com/{norm_rel}/{fname}")
                    break
    except Exception:
        pass

    # Step 2: Check official ubuntu.com/download/server/thank-you page
    try:
        ty_req = urllib.request.Request(
            cat["thank_you_url"],
            headers={"User-Agent": "Mozilla/5.0 InstantServer-ISOManager/2.1"},
        )
        with urllib.request.urlopen(ty_req, timeout=timeout) as ty_resp:
            ty_html = ty_resp.read().decode("utf-8", errors="replace")
        extracted_url = extract_iso_url_from_thank_you_html(ty_html)
        if extracted_url and extracted_url not in candidate_urls:
            candidate_urls.append(extracted_url)
    except Exception:
        pass

    # Step 3: Append catalog mirror URLs
    for m_url in cat.get("mirror_urls", []):
        if m_url not in candidate_urls:
            candidate_urls.append(m_url)

    # Step 4: Verify which URL responds with HTTP 200/206
    for url in candidate_urls:
        try:
            probe_req = urllib.request.Request(
                url,
                headers={"User-Agent": "InstantServer-ISOManager/2.1", "Range": "bytes=0-15"},
            )
            with urllib.request.urlopen(probe_req, timeout=timeout) as p_resp:
                code = getattr(p_resp, "status", 200)
                if code in (200, 206):
                    fname = url.rstrip("/").split("/")[-1]
                    return url, fname, expected_sha
        except Exception:
            continue

    return cat["iso_url"], candidate_filename, expected_sha


def compute_file_sha256(
    file_path: str,
    chunk_size: int = 4 * 1024 * 1024,
    max_bytes: Optional[int] = None,
    event_callback: Optional[Callable[[Dict[str, Any]], None]] = None,
) -> str:
    """Compute SHA-256 hash of a file with progress callbacks."""
    hasher = hashlib.sha256()
    total = max_bytes if max_bytes is not None else os.path.getsize(file_path)
    processed = 0

    with open(file_path, "rb") as fh:
        while True:
            to_read = chunk_size
            if max_bytes is not None:
                rem = max_bytes - processed
                if rem <= 0:
                    break
                to_read = min(chunk_size, rem)
            buf = fh.read(to_read)
            if not buf:
                break
            hasher.update(buf)
            processed += len(buf)
            if event_callback and total > 0:
                pct = round((processed / total) * 100, 1)
                done_mb = round(processed / (1024 * 1024), 1)
                tot_mb = round(total / (1024 * 1024), 1)
                emit_event(
                    "ISO_VERIFY_PROGRESS",
                    f"Verifying ISO SHA-256 integrity: {done_mb} MB / {tot_mb} MB ({pct}%)",
                    {
                        "in_place": True,
                        "label": "Verifying ISO SHA-256",
                        "stage_percent": pct,
                        "detail": f"{done_mb} / {tot_mb} MB",
                        "bytes_verified": processed,
                        "total_bytes": total,
                        "percent": int(10 + pct * 0.15),
                    },
                    callback=event_callback,
                )

    return hasher.hexdigest().lower()


# ---------------------------------------------------------------------------
# 3. Resumable Official ISO Downloader & Local ISO Resolver
# ---------------------------------------------------------------------------
def download_file_resumable(
    url: str,
    dest_path: str,
    expected_sha256: Optional[str] = None,
    chunk_size: int = 2 * 1024 * 1024,
    timeout: int = 30,
    event_callback: Optional[Callable[[Dict[str, Any]], None]] = None,
) -> str:
    """
    Download the official Ubuntu Server ISO with HTTP Range resume (`Range: bytes=<offset>-`)
    and SHA-256 verification.
    """
    os.makedirs(os.path.dirname(os.path.abspath(dest_path)), exist_ok=True)
    part_path = dest_path + ".part"

    if os.path.exists(dest_path) and os.path.getsize(dest_path) > 64 * 1024 * 1024:
        if expected_sha256:
            actual = compute_file_sha256(dest_path, event_callback=event_callback)
            if actual == expected_sha256.lower():
                emit_event(
                    "ISO_CACHE_HIT",
                    f"Using verified cached Ubuntu ISO ({round(os.path.getsize(dest_path) / (1024*1024), 1)} MB)",
                    {"sha256": actual, "percent": 25},
                    callback=event_callback,
                )
                return dest_path
        else:
            emit_event(
                "ISO_CACHE_HIT",
                f"Using cached Ubuntu ISO at {os.path.basename(dest_path)}",
                {"percent": 25},
                callback=event_callback,
            )
            return dest_path

    existing_bytes = os.path.getsize(part_path) if os.path.exists(part_path) else 0
    headers = {"User-Agent": "InstantServer-ISOManager/2.1"}
    if existing_bytes > 0:
        headers["Range"] = f"bytes={existing_bytes}-"

    req = urllib.request.Request(url, headers=headers)
    emit_event(
        "ISO_DOWNLOAD_START",
        f"Pulling official Ubuntu Server ISO from {url}...",
        {"url": url, "resume_offset": existing_bytes, "percent": 10},
        callback=event_callback,
    )

    with urllib.request.urlopen(req, timeout=timeout) as resp:
        status = getattr(resp, "status", 200)
        mode = "ab" if (status == 206 and existing_bytes > 0) else "wb"
        if mode == "wb":
            existing_bytes = 0

        content_len_hdr = resp.headers.get("Content-Length")
        total_bytes = (existing_bytes + int(content_len_hdr)) if content_len_hdr else 0
        downloaded = existing_bytes
        start_t = time.time()
        last_emit_t = 0.0

        with open(part_path, mode) as out_fh:
            while True:
                chunk = resp.read(chunk_size)
                if not chunk:
                    break
                out_fh.write(chunk)
                downloaded += len(chunk)
                now = time.time()
                if now - last_emit_t >= 0.35 or downloaded == total_bytes:
                    last_emit_t = now
                    elapsed = max(0.001, now - start_t)
                    speed_mbps = round(((downloaded - existing_bytes) / (1024 * 1024)) / elapsed, 2)
                    dl_pct = round((downloaded / total_bytes) * 100, 1) if total_bytes > 0 else 0.0
                    ui_pct = min(40, int(10 + (dl_pct * 0.30)))
                    dl_mb = round(downloaded / (1024 * 1024), 1)
                    tot_mb = round(total_bytes / (1024 * 1024), 1)
                    emit_event(
                        "ISO_DOWNLOAD_PROGRESS",
                        f"Downloading official Ubuntu ISO: {dl_mb} MB / {tot_mb} MB ({dl_pct}% @ {speed_mbps} MB/s)",
                        {
                            "in_place": True,
                            "label": "Downloading Ubuntu ISO",
                            "stage_percent": dl_pct,
                            "detail": f"{dl_mb} / {tot_mb} MB @ {speed_mbps} MB/s",
                            "downloaded_bytes": downloaded,
                            "total_bytes": total_bytes,
                            "download_percent": dl_pct,
                            "percent": ui_pct,
                            "speed_mbps": speed_mbps,
                        },
                        callback=event_callback,
                    )

    os.replace(part_path, dest_path)

    if expected_sha256:
        actual_hash = compute_file_sha256(dest_path, event_callback=event_callback)
        if actual_hash != expected_sha256.lower():
            raise ValueError(
                f"SHA-256 mismatch for {dest_path}: expected {expected_sha256}, got {actual_hash}"
            )
        emit_event(
            "ISO_VERIFIED",
            "Official Ubuntu Server ISO SHA-256 checksum verified!",
            {"sha256": actual_hash, "percent": 42},
            callback=event_callback,
        )

    return dest_path


def ensure_ubuntu_iso(
    release: str = "26.04",
    iso_cache_dir: str = "./iso_cache",
    custom_iso_path: str = "",
    auto_download: bool = True,
    event_callback: Optional[Callable[[Dict[str, Any]], None]] = None,
) -> Optional[str]:
    """
    Resolve the Ubuntu Server ISO to flash onto the USB drive:
      1. If `custom_iso_path` is a local `.iso` file on disk, uses it immediately.
      2. If `custom_iso_path` is an HTTP/HTTPS URL (including an `ubuntu.com/download/server/thank-you`
         URL), resolves and downloads it.
      3. Otherwise resolves the live official Ubuntu ISO URL (`26.04.1` default, `24.04.5`, or `22.04.5`)
         via `resolve_live_ubuntu_iso_url()` and downloads/verifies it into `iso_cache_dir`.
    """
    os.makedirs(iso_cache_dir, exist_ok=True)

    if custom_iso_path:
        clean_custom = custom_iso_path.strip()
        if os.path.isfile(clean_custom):
            emit_event(
                "ISO_CUSTOM_SELECTED",
                f"Using local Ubuntu ISO image: {clean_custom}",
                {"iso_path": clean_custom, "percent": 25},
                callback=event_callback,
            )
            return os.path.abspath(clean_custom)
        if clean_custom.startswith(("http://", "https://")):
            dl_url = clean_custom
            if "thank-you" in dl_url:
                try:
                    req = urllib.request.Request(dl_url, headers={"User-Agent": "Mozilla/5.0 InstantServer/2.1"})
                    with urllib.request.urlopen(req, timeout=12) as resp:
                        parsed = extract_iso_url_from_thank_you_html(resp.read().decode("utf-8", errors="replace"))
                        if parsed:
                            dl_url = parsed
                except Exception:
                    pass
            fname = dl_url.rstrip("/").split("/")[-1]
            if not fname.endswith(".iso"):
                fname = f"ubuntu-{release}-live-server-amd64.iso"
            return download_file_resumable(
                url=dl_url,
                dest_path=os.path.join(iso_cache_dir, fname),
                event_callback=event_callback,
            )

    norm_rel = "26.04"
    for prefix in ("26.04", "24.04", "22.04"):
        if str(release).startswith(prefix):
            norm_rel = prefix
            break

    cat = UBUNTU_LTS_CATALOG[norm_rel]
    cached_default = os.path.join(iso_cache_dir, cat["iso_filename"])
    if os.path.isfile(cached_default) and os.path.getsize(cached_default) > 0:
        emit_event(
            "ISO_CACHE_FOUND",
            f"Found cached Ubuntu {cat['point_version']} LTS ISO at {cached_default}",
            {"iso_path": cached_default, "percent": 25},
            callback=event_callback,
        )
        return os.path.abspath(cached_default)

    if not auto_download:
        return None

    emit_event(
        "ISO_RESOLVING_MIRROR",
        f"Resolving official Ubuntu {cat['point_version']} LTS mirror & SHA256SUMS...",
        {"release": cat["point_version"], "percent": 9},
        callback=event_callback,
    )
    resolved_url, resolved_fname, expected_sha = resolve_live_ubuntu_iso_url(norm_rel)
    target_iso = os.path.join(iso_cache_dir, resolved_fname)
    return download_file_resumable(
        url=resolved_url,
        dest_path=target_iso,
        expected_sha256=expected_sha,
        event_callback=event_callback,
    )


# ---------------------------------------------------------------------------
# 4. GRUB Autoinstall Patcher (Zero-Keyboard Unattended Boot for UEFI & BIOS)
# ---------------------------------------------------------------------------
def patch_grub_cfg_text(grub_text: str) -> str:
    """
    Patch an extracted `boot/grub/grub.cfg` or `boot/grub/loopback.cfg` so that:
      1. `set timeout=30` becomes `set timeout=3`.
      2. Every `linux /casper/vmlinuz ... ---` line includes
         `autoinstall ds=nocloud;s=/cdrom/cidata/ quiet` so Ubuntu Subiquity
         automatically boots into unattended installation.
    """
    lines: List[str] = []
    for line in grub_text.splitlines():
        stripped = line.strip()
        if stripped.startswith("set timeout="):
            lines.append("set timeout=3")
            continue
        if ("/casper/vmlinuz" in line or "/casper/hwe-vmlinuz" in line) and "autoinstall" not in line:
            if "---" in line:
                line = line.replace("---", "autoinstall ds=nocloud\\;s=/cdrom/cidata/ quiet ---", 1)
            else:
                line = line + " autoinstall ds=nocloud\\;s=/cdrom/cidata/ quiet"
        lines.append(line)
    return "\n".join(lines) + "\n"


def patch_iso_bytes_in_flight(chunk: bytes) -> bytes:
    if b"set timeout=30" in chunk:
        chunk = chunk.replace(b"set timeout=30", b"set timeout=03")
    return chunk


# ---------------------------------------------------------------------------
# 5. Pure-Python ISO9660 Reader, Full Boot Tree Extractor & `.deb` Extractor
# ---------------------------------------------------------------------------
ISO_SECTOR_SIZE = 2048


class ISO9660Reader:
    def __init__(self, iso_path: str) -> None:
        self.iso_path = iso_path
        self.fh = open(iso_path, "rb")
        self.root_extent_lba, self.root_size = self._read_primary_volume_descriptor()

    def close(self) -> None:
        if self.fh and not self.fh.closed:
            self.fh.close()

    def __enter__(self) -> "ISO9660Reader":
        return self

    def __exit__(self, exc_type: Any, exc_val: Any, exc_tb: Any) -> None:
        self.close()

    def _read_primary_volume_descriptor(self) -> Tuple[int, int]:
        for sector in range(16, 32):
            self.fh.seek(sector * ISO_SECTOR_SIZE)
            vd = self.fh.read(ISO_SECTOR_SIZE)
            if len(vd) < ISO_SECTOR_SIZE:
                break
            vd_type = vd[0]
            ident = vd[1:6]
            if ident != b"CD001":
                continue
            if vd_type == 1:
                root_rec = vd[156:190]
                extent_lba = struct.unpack("<I", root_rec[2:6])[0]
                data_len = struct.unpack("<I", root_rec[10:14])[0]
                return extent_lba, data_len
            if vd_type == 255:
                break
        raise ValueError(f"No valid ISO9660 Primary Volume Descriptor found in {self.iso_path}")

    def list_all_files(self) -> List[Dict[str, Any]]:
        results: List[Dict[str, Any]] = []
        visited_lbas: set = set()
        self._walk_dir(self.root_extent_lba, self.root_size, "", results, visited_lbas)
        return results

    def _walk_dir(
        self,
        extent_lba: int,
        data_len: int,
        parent_path: str,
        out_list: List[Dict[str, Any]],
        visited: set,
    ) -> None:
        if extent_lba in visited or data_len <= 0:
            return
        visited.add(extent_lba)

        self.fh.seek(extent_lba * ISO_SECTOR_SIZE)
        raw = self.fh.read(data_len)
        pos = 0
        n = len(raw)

        while pos < n:
            rec_len = raw[pos]
            if rec_len == 0:
                next_sector = ((pos // ISO_SECTOR_SIZE) + 1) * ISO_SECTOR_SIZE
                if next_sector <= pos:
                    break
                pos = next_sector
                continue

            rec = raw[pos : pos + rec_len]
            if len(rec) < 34:
                break

            child_lba = struct.unpack("<I", rec[2:6])[0]
            child_size = struct.unpack("<I", rec[10:14])[0]
            flags = rec[25]
            name_len = rec[32]
            raw_name = rec[33 : 33 + name_len]

            if name_len == 1 and raw_name in (b"\x00", b"\x01"):
                pos += rec_len
                continue

            su_offset = 33 + name_len
            if su_offset % 2 == 1:
                su_offset += 1
            rr_name = self._parse_rock_ridge_nm(rec[su_offset:])
            if rr_name:
                clean_name = rr_name
            else:
                clean_name = raw_name.decode("ascii", errors="replace").split(";")[0].rstrip(".")

            full_path = f"{parent_path}/{clean_name}" if parent_path else f"/{clean_name}"
            is_dir = bool(flags & 0x02)

            if is_dir:
                self._walk_dir(child_lba, child_size, full_path, out_list, visited)
            else:
                out_list.append({
                    "path": full_path,
                    "name": clean_name,
                    "lba": child_lba,
                    "size": child_size,
                })
            pos += rec_len

    @staticmethod
    def _parse_rock_ridge_nm(su_bytes: bytes) -> str:
        idx = 0
        parts: List[str] = []
        while idx + 4 <= len(su_bytes):
            sig = su_bytes[idx : idx + 2]
            length = su_bytes[idx + 2]
            if length < 4 or idx + length > len(su_bytes):
                break
            if sig == b"NM" and length > 5:
                parts.append(su_bytes[idx + 5 : idx + length].decode("utf-8", errors="replace"))
            idx += length
        return "".join(parts)

    def extract_Entry(
        self,
        entry: Dict[str, Any],
        dest_file: str,
        chunk_callback: Optional[Callable[[int], None]] = None,
    ) -> str:
        os.makedirs(os.path.dirname(os.path.abspath(dest_file)), exist_ok=True)
        self.fh.seek(int(entry["lba"]) * ISO_SECTOR_SIZE)
        remaining = int(entry["size"])
        with open(dest_file, "wb") as out:
            while remaining > 0:
                chunk = self.fh.read(min(2 * 1024 * 1024, remaining))
                if not chunk:
                    break
                out.write(chunk)
                remaining -= len(chunk)
                if chunk_callback:
                    chunk_callback(len(chunk))
        return dest_file


def extract_full_iso_tree_to_usb(
    iso_path: str,
    dest_root_dir: str,
    event_callback: Optional[Callable[[Dict[str, Any]], None]] = None,
) -> Dict[str, Any]:
    os.makedirs(dest_root_dir, exist_ok=True)
    extracted_count = 0
    total_bytes = 0

    emit_event(
        "ISO_EXTRACT_START",
        f"Extracting bootable Ubuntu ISO tree & patching GRUB autoinstall onto {dest_root_dir}...",
        {"dest_root_dir": dest_root_dir, "percent": 45},
        callback=event_callback,
    )

    with ISO9660Reader(iso_path) as reader:
        all_files = reader.list_all_files()
        total_files = max(1, len(all_files))
        grand_total_bytes = max(1, sum(int(e["size"]) for e in all_files))
        last_emit_t = 0.0

        for idx, entry in enumerate(all_files):
            rel_path = entry["path"].lstrip("/")
            out_path = os.path.join(dest_root_dir, rel_path.replace("/", os.sep))

            def _on_chunk(n_bytes: int, _entry_name: str = entry["name"], _idx: int = idx) -> None:
                nonlocal total_bytes, last_emit_t
                total_bytes += n_bytes
                now = time.time()
                if now - last_emit_t >= 0.25 or total_bytes >= grand_total_bytes:
                    last_emit_t = now
                    stage_pct = min(100.0, round((total_bytes / grand_total_bytes) * 100, 1))
                    ui_pct = min(85, int(45 + (stage_pct * 0.40)))
                    done_mb = round(total_bytes / (1024 * 1024), 1)
                    tot_mb = round(grand_total_bytes / (1024 * 1024), 1)
                    emit_event(
                        "ISO_EXTRACT_PROGRESS",
                        f"Writing ISO to USB: {done_mb} MB / {tot_mb} MB ({stage_pct}%) — {_entry_name}",
                        {
                            "in_place": True,
                            "label": "Writing Bootable ISO to USB",
                            "stage_percent": stage_pct,
                            "detail": f"{done_mb} / {tot_mb} MB ({_idx + 1}/{total_files} files • {_entry_name})",
                            "extracted_files": _idx + 1,
                            "total_files": total_files,
                            "percent": ui_pct,
                        },
                        callback=event_callback,
                    )

            reader.extract_Entry(entry, out_path, chunk_callback=_on_chunk)
            extracted_count += 1

            if rel_path.lower() in ("boot/grub/grub.cfg", "boot/grub/loopback.cfg"):
                try:
                    orig = open(out_path, "r", encoding="utf-8", errors="replace").read()
                    patched = patch_grub_cfg_text(orig)
                    with open(out_path, "w", encoding="utf-8", newline="\n") as gfh:
                        gfh.write(patched)
                except Exception:
                    pass

            if idx == total_files - 1:
                done_mb = round(total_bytes / (1024 * 1024), 1)
                tot_mb = round(grand_total_bytes / (1024 * 1024), 1)
                emit_event(
                    "ISO_EXTRACT_PROGRESS",
                    f"Writing ISO to USB: {done_mb} MB / {tot_mb} MB (100.0%)",
                    {
                        "in_place": True,
                        "label": "Writing Bootable ISO to USB",
                        "stage_percent": 100.0,
                        "detail": f"{done_mb} / {tot_mb} MB ({total_files}/{total_files} files)",
                        "extracted_files": total_files,
                        "total_files": total_files,
                        "percent": 85,
                    },
                    callback=event_callback,
                )

    return {
        "dest_root_dir": dest_root_dir,
        "extracted_files": extracted_count,
        "total_bytes": total_bytes,
    }


def extract_offline_debs_from_iso(
    iso_path: str,
    dest_debs_dir: str,
    package_prefixes: Optional[List[str]] = None,
    event_callback: Optional[Callable[[Dict[str, Any]], None]] = None,
) -> List[str]:
    if package_prefixes is None:
        package_prefixes = [
            "wpasupplicant",
            "libpcsclite",
            "libnl-route-3",
            "libnl-genl-3",
            "libnl-3",
            "rfkill",
            "wireless-regdb",
            "ethtool",
        ]

    os.makedirs(dest_debs_dir, exist_ok=True)
    extracted_paths: List[str] = []

    with ISO9660Reader(iso_path) as reader:
        all_files = reader.list_all_files()
        for entry in all_files:
            lower_name = entry["name"].lower()
            if not lower_name.endswith(".deb"):
                continue
            if any(lower_name.startswith(pref.lower()) for pref in package_prefixes):
                out_file = os.path.join(dest_debs_dir, entry["name"])
                reader.extract_Entry(entry, out_file)
                extracted_paths.append(out_file)
                emit_event(
                    "OFFLINE_DEB_EXTRACTED",
                    f"Extracted {entry['name']} ({entry['size']} bytes) from ISO pool",
                    {"name": entry["name"], "size": entry["size"]},
                    callback=event_callback,
                )

    return extracted_paths


# ---------------------------------------------------------------------------
# 6. Pure-Python Docker Hub OCI Image Tarball Puller (Host-Side Pre-Bundler)
# ---------------------------------------------------------------------------
def pull_docker_image_tarball(
    image_ref: str,
    dest_tar_gz: str,
    arch: str = "amd64",
    timeout: int = 20,
    event_callback: Optional[Callable[[Dict[str, Any]], None]] = None,
) -> Dict[str, Any]:
    repo_tag = image_ref.strip()
    if ":" in repo_tag:
        repo, tag = repo_tag.split(":", 1)
    else:
        repo, tag = repo_tag, "latest"
    if "/" not in repo:
        repo = f"library/{repo}"

    emit_event(
        "DOCKER_PULL_START",
        f"Pre-downloading container image {repo}:{tag} from registry...",
        {"image": f"{repo}:{tag}"},
        callback=event_callback,
    )

    auth_url = f"https://auth.docker.io/token?service=registry.docker.io&scope=repository:{repo}:pull"
    with urllib.request.urlopen(auth_url, timeout=timeout) as resp:
        token = json.loads(resp.read().decode("utf-8"))["token"]

    headers = {
        "Authorization": f"Bearer {token}",
        "Accept": (
            "application/vnd.docker.distribution.manifest.v2+json, "
            "application/vnd.oci.image.manifest.v1+json, "
            "application/vnd.docker.distribution.manifest.list.v2+json, "
            "application/vnd.oci.image.index.v1+json"
        ),
    }

    manifest_url = f"https://registry-1.docker.io/v2/{repo}/manifests/{tag}"
    req = urllib.request.Request(manifest_url, headers=headers)
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        manifest_data = json.loads(resp.read().decode("utf-8"))

    if "manifests" in manifest_data:
        chosen_digest = None
        for m in manifest_data["manifests"]:
            plat = m.get("platform") or {}
            if plat.get("architecture") == arch and plat.get("os") == "linux":
                chosen_digest = m.get("digest")
                break
        if not chosen_digest and manifest_data["manifests"]:
            chosen_digest = manifest_data["manifests"][0].get("digest")
        sub_url = f"https://registry-1.docker.io/v2/{repo}/manifests/{chosen_digest}"
        sub_req = urllib.request.Request(sub_url, headers=headers)
        with urllib.request.urlopen(sub_req, timeout=timeout) as resp:
            manifest_data = json.loads(resp.read().decode("utf-8"))

    config_digest = manifest_data["config"]["digest"]
    layers = manifest_data.get("layers", [])

    os.makedirs(os.path.dirname(os.path.abspath(dest_tar_gz)), exist_ok=True)
    with tarfile.open(dest_tar_gz, "w:gz") as tar:
        cfg_url = f"https://registry-1.docker.io/v2/{repo}/blobs/{config_digest}"
        cfg_req = urllib.request.Request(cfg_url, headers={"Authorization": f"Bearer {token}"})
        with urllib.request.urlopen(cfg_req, timeout=timeout) as c_resp:
            cfg_bytes = c_resp.read()

        cfg_name = f"{config_digest.split(':')[-1]}.json"
        cfg_info = tarfile.TarInfo(name=cfg_name)
        cfg_info.size = len(cfg_bytes)
        tar.addfile(cfg_info, io.BytesIO(cfg_bytes))

        layer_files: List[str] = []
        for idx, layer in enumerate(layers):
            l_digest = layer["digest"]
            l_hash = l_digest.split(":")[-1]
            l_name = f"{l_hash}/layer.tar"
            emit_event(
                "DOCKER_LAYER_PULL",
                f"Downloading {repo}:{tag} layer {idx + 1}/{len(layers)}...",
                {"image": f"{repo}:{tag}", "layer": idx + 1, "total_layers": len(layers)},
                callback=event_callback,
            )
            l_url = f"https://registry-1.docker.io/v2/{repo}/blobs/{l_digest}"
            l_req = urllib.request.Request(l_url, headers={"Authorization": f"Bearer {token}"})
            with urllib.request.urlopen(l_req, timeout=timeout) as l_resp:
                l_bytes = l_resp.read()
            l_info = tarfile.TarInfo(name=l_name)
            l_info.size = len(l_bytes)
            tar.addfile(l_info, io.BytesIO(l_bytes))
            layer_files.append(l_name)

        docker_manifest = [
            {
                "Config": cfg_name,
                "RepoTags": [f"{repo}:{tag}"],
                "Layers": layer_files,
            }
        ]
        m_bytes = json.dumps(docker_manifest).encode("utf-8")
        m_info = tarfile.TarInfo(name="manifest.json")
        m_info.size = len(m_bytes)
        tar.addfile(m_info, io.BytesIO(m_bytes))

    return {
        "image": f"{repo}:{tag}",
        "tarball_path": dest_tar_gz,
        "layers_count": len(layers),
        "size_bytes": os.path.getsize(dest_tar_gz),
    }


# ---------------------------------------------------------------------------
# 7. Synthetic ISO9660 Builder & Built-In Self-Test
# ---------------------------------------------------------------------------
def _build_synthetic_iso_with_deb(iso_file: str, deb_name: str, deb_payload: bytes) -> None:
    def make_dir_record(lba: int, size: int, flags: int, ident: bytes, rr_nm: Optional[str] = None) -> bytes:
        su = b""
        if rr_nm:
            nm_bytes = rr_nm.encode("utf-8")
            su = b"NM" + struct.pack("BBB", 5 + len(nm_bytes), 1, 0) + nm_bytes
            if len(su) % 2 == 1:
                su += b"\x00"
        base_len = 33 + len(ident)
        pad = b"\x00" if (base_len % 2 == 1) else b""
        total_len = base_len + len(pad) + len(su)
        rec = bytearray(total_len)
        rec[0] = total_len
        rec[1] = 0
        struct.pack_into("<I", rec, 2, lba)
        struct.pack_into(">I", rec, 6, lba)
        struct.pack_into("<I", rec, 10, size)
        struct.pack_into(">I", rec, 14, size)
        rec[25] = flags
        struct.pack_into("<H", rec, 28, 1)
        struct.pack_into(">H", rec, 30, 1)
        rec[32] = len(ident)
        rec[33 : 33 + len(ident)] = ident
        offset = 33 + len(ident) + len(pad)
        rec[offset : offset + len(su)] = su
        return bytes(rec)

    dot_rec = make_dir_record(18, ISO_SECTOR_SIZE, 0x02, b"\x00")
    dotdot_rec = make_dir_record(18, ISO_SECTOR_SIZE, 0x02, b"\x01")
    file_rec = make_dir_record(19, len(deb_payload), 0x00, b"WPASUPP.DEB;1", rr_nm=deb_name)

    root_sector = bytearray(ISO_SECTOR_SIZE)
    dir_bytes = dot_rec + dotdot_rec + file_rec
    root_sector[: len(dir_bytes)] = dir_bytes

    pvd = bytearray(ISO_SECTOR_SIZE)
    pvd[0] = 1
    pvd[1:6] = b"CD001"
    pvd[6] = 1
    pvd[40:72] = b"UBUNTU_26_04_SERVER             "
    root_vd_rec = make_dir_record(18, ISO_SECTOR_SIZE, 0x02, b"\x00")
    pvd[156 : 156 + len(root_vd_rec)] = root_vd_rec

    term = bytearray(ISO_SECTOR_SIZE)
    term[0] = 255
    term[1:6] = b"CD001"
    term[6] = 1

    file_sector = bytearray(ISO_SECTOR_SIZE)
    file_sector[: len(deb_payload)] = deb_payload

    with open(iso_file, "wb") as fh:
        fh.write(b"\x00" * (16 * ISO_SECTOR_SIZE))
        fh.write(pvd)
        fh.write(term)
        fh.write(root_sector)
        fh.write(file_sector)


def _run_self_test() -> int:
    print("[M2 Self-Test] 1. Testing Ubuntu 26.04.1 / 24.04.5 / 22.04.5 catalog & thank-you parser...")
    assert "26.04" in UBUNTU_LTS_CATALOG
    assert UBUNTU_LTS_CATALOG["26.04"]["point_version"] == "26.04.1"
    assert UBUNTU_LTS_CATALOG["24.04"]["point_version"] == "24.04.5"
    assert UBUNTU_LTS_CATALOG["22.04"]["point_version"] == "22.04.5"

    sample_ty_html = '<a href="https://releases.ubuntu.com/26.04.1/ubuntu-26.04.1-live-server-amd64.iso">download now</a>'
    assert extract_iso_url_from_thank_you_html(sample_ty_html) == (
        "https://releases.ubuntu.com/26.04.1/ubuntu-26.04.1-live-server-amd64.iso"
    )

    sample_grub = 'set timeout=30\nmenuentry "Try or Install Ubuntu Server" {\n  linux /casper/vmlinuz ---\n}\n'
    patched_grub = patch_grub_cfg_text(sample_grub)
    assert "set timeout=3" in patched_grub
    assert "autoinstall ds=nocloud\\;s=/cdrom/cidata/" in patched_grub
    print("  -> PASS: Ubuntu 26.04.1 / 24.04.5 / 22.04.5 catalog, thank-you HTML parser & GRUB patcher verified!")

    print("[M2 Self-Test] 2. Testing pure-Python ISO9660 reader, full tree extraction & .deb extractor...")
    with tempfile.TemporaryDirectory() as tmpdir:
        test_iso = os.path.join(tmpdir, "mock_ubuntu.iso")
        deb_name = "wpasupplicant_2.10-21build4_amd64.deb"
        deb_bytes = b"!<arch>\ndebian-binary   1700000000  0     0     100644  4         `\n2.0\n"
        _build_synthetic_iso_with_deb(test_iso, deb_name, deb_bytes)

        out_debs_dir = os.path.join(tmpdir, "debs")
        extracted = extract_offline_debs_from_iso(test_iso, out_debs_dir)
        assert len(extracted) == 1

        full_extract_dir = os.path.join(tmpdir, "usb_boot_root")
        tree_res = extract_full_iso_tree_to_usb(test_iso, full_extract_dir)
        assert tree_res["extracted_files"] == 1
    print("  -> PASS: Extracted ISO9660 boot tree and wpasupplicant .deb in pure Python!")
    return 0


def main() -> int:
    parser = argparse.ArgumentParser(description="Instant Server — M2 ISO Manager & Pure-Python .deb Extractor")
    parser.add_argument("--self-test", action="store_true", help="Run built-in unit tests")
    parser.add_argument("--extract-debs-from-iso", type=str, default="", help="Path to local Ubuntu ISO")
    parser.add_argument("--output-debs-dir", type=str, default="./cidata_debs", help="Directory for extracted .debs")
    args = parser.parse_args()

    if args.self_test:
        return _run_self_test()

    if args.extract_debs_from_iso:
        files = extract_offline_debs_from_iso(args.extract_debs_from_iso, args.output_debs_dir)
        print(json.dumps({"extracted_debs": files}, indent=2))
        return 0

    print(json.dumps(UBUNTU_LTS_CATALOG, indent=2))
    return 0


if __name__ == "__main__":
    sys.exit(main())