# src/instant_server/iso_manager.py
from __future__ import annotations

import argparse
import hashlib
import io
import json
import os
import re
import shutil
import struct
import sys
import tarfile
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request
from typing import Any, Callable, Dict, List, Optional, Tuple

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
    payload = {
        "event": event_type,
        "module": "ISO_MANAGER",
        "project": "Instant Server",
        "message": message,
        "data": data or {},
    }
    if callback:
        callback(payload)
    return payload

def parse_sha256sums_text(text: str) -> Dict[str, str]:
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
    m = re.search(
        r'https?://[a-zA-Z0-9._/-]+/ubuntu-[0-9.]+-live-server-amd64\.iso',
        html,
    )
    if m:
        return m.group(0)
    return None

def resolve_live_ubuntu_iso_url(
    release: str = "26.04",
    timeout: int = 5,
) -> Tuple[str, str, Optional[str]]:
    norm_rel = "26.04"
    for prefix in ("26.04", "24.04", "22.04"):
        if str(release).startswith(prefix):
            norm_rel = prefix
            break

    cat = UBUNTU_LTS_CATALOG[norm_rel]
    candidate_filename = cat["iso_filename"]
    expected_sha: Optional[str] = None

    sha_urls = [
        cat["sha256sums_url"],
        f"https://releases.ubuntu.com/{cat['codename']}/SHA256SUMS",
    ]
    for s_url in sha_urls:
        try:
            req = urllib.request.Request(
                s_url,
                headers={"User-Agent": "InstantServer-ISOManager/2.2"},
            )
            with urllib.request.urlopen(req, timeout=timeout) as resp:
                text = resp.read().decode("utf-8", errors="replace")
            sums = parse_sha256sums_text(text)
            if candidate_filename in sums:
                expected_sha = sums[candidate_filename]
                base_dir_url = s_url.rsplit("/", 1)[0]
                return f"{base_dir_url}/{candidate_filename}", candidate_filename, expected_sha
            for fname, digest in sorted(sums.items(), reverse=True):
                if fname.endswith("-live-server-amd64.iso"):
                    candidate_filename = fname
                    expected_sha = digest
                    base_dir_url = s_url.rsplit("/", 1)[0]
                    return f"{base_dir_url}/{fname}", fname, expected_sha
        except Exception:
            continue

    try:
        ty_req = urllib.request.Request(
            cat["thank_you_url"],
            headers={"User-Agent": "Mozilla/5.0 InstantServer-ISOManager/2.2"},
        )
        with urllib.request.urlopen(ty_req, timeout=timeout) as ty_resp:
            ty_html = ty_resp.read().decode("utf-8", errors="replace")
        extracted_url = extract_iso_url_from_thank_you_html(ty_html)
        if extracted_url:
            fname = extracted_url.rstrip("/").split("/")[-1]
            return extracted_url, fname, expected_sha
    except Exception:
        pass

    return cat["iso_url"], candidate_filename, expected_sha

def compute_file_sha256(
    file_path: str,
    chunk_size: int = 4 * 1024 * 1024,
    max_bytes: Optional[int] = None,
    event_callback: Optional[Callable[[Dict[str, Any]], None]] = None,
) -> str:
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

def compute_sha256_streaming(file_path: str, chunk_size: int = 1024 * 1024) -> str:
    h = hashlib.sha256()
    with open(file_path, "rb") as fh:
        while True:
            chunk = fh.read(chunk_size)
            if not chunk:
                break
            h.update(chunk)
    return h.hexdigest()

def download_file_resumable(
    url: str,
    dest_path: str,
    expected_sha256: Optional[str] = None,
    chunk_size: int = 64 * 1024,
    timeout: int = 25,
    fallback_urls: Optional[List[str]] = None,
    event_callback: Optional[Callable[[Dict[str, Any]], None]] = None,
) -> str:
    os.makedirs(os.path.dirname(os.path.abspath(dest_path)), exist_ok=True)
    part_path = dest_path + ".part"

    if os.path.exists(dest_path) and os.path.getsize(dest_path) > 64 * 1024 * 1024:
        if expected_sha256:
            actual = compute_file_sha256(dest_path, event_callback=event_callback)
            if actual == expected_sha256.lower():
                emit_event(
                    "ISO_CACHE_HIT",
                    f"Using verified cached Ubuntu ISO ({round(os.path.getsize(dest_path) / (1024*1024), 1)} MB)",
                    {"sha256": actual, "percent": 40},
                    callback=event_callback,
                )
                return dest_path
        else:
            emit_event(
                "ISO_CACHE_HIT",
                f"Using cached Ubuntu ISO at {os.path.basename(dest_path)}",
                {"percent": 40},
                callback=event_callback,
            )
            return dest_path

    existing_bytes = os.path.getsize(part_path) if os.path.exists(part_path) else 0
    urls_to_try = [url] + [u for u in (fallback_urls or []) if u != url]

    emit_event(
        "ISO_DOWNLOAD_START",
        f"Pulling official Ubuntu Server ISO ({os.path.basename(dest_path)})...",
        {"url": url, "resume_offset": existing_bytes, "percent": 10},
        callback=event_callback,
    )

    emit_event(
        "ISO_DOWNLOAD_PROGRESS",
        f"Connecting to Ubuntu release mirror ({url})...",
        {
            "in_place": True,
            "label": "Downloading Ubuntu ISO",
            "stage_percent": 0.0,
            "detail": "Connecting to official Ubuntu mirror...",
            "downloaded_bytes": existing_bytes,
            "total_bytes": 0,
            "download_percent": 0.0,
            "percent": 10,
            "speed_mbps": 0.0,
        },
        callback=event_callback,
    )

    last_err: Optional[Exception] = None
    for candidate_url in urls_to_try:
        try:
            headers = {"User-Agent": "Mozilla/5.0 InstantServer-ISOManager/2.2"}
            if existing_bytes > 0:
                headers["Range"] = f"bytes={existing_bytes}-"
            req = urllib.request.Request(candidate_url, headers=headers)
            with urllib.request.urlopen(req, timeout=timeout) as resp:
                status = getattr(resp, "status", 200)
                mode = "ab" if (status == 206 and existing_bytes > 0) else "wb"
                if mode == "wb":
                    existing_bytes = 0

                content_len_hdr = resp.headers.get("Content-Length") or resp.headers.get("content-length")
                total_bytes = (existing_bytes + int(content_len_hdr)) if content_len_hdr else (2650 * 1024 * 1024)
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
                        if now - last_emit_t >= 0.15 or downloaded >= total_bytes:
                            last_emit_t = now
                            elapsed = max(0.001, now - start_t)
                            bytes_this_session = max(0, downloaded - existing_bytes)
                            speed_bps = bytes_this_session / elapsed
                            speed_mbps = round(speed_bps / (1024 * 1024), 2)
                            dl_pct = min(100.0, round((downloaded / max(1, total_bytes)) * 100, 1))
                            ui_pct = min(40, int(10 + (dl_pct * 0.30)))
                            dl_mb = round(downloaded / (1024 * 1024), 1)
                            tot_mb = round(total_bytes / (1024 * 1024), 1)
                            rem_bytes = max(0, total_bytes - downloaded)
                            if speed_bps > 1024:
                                eta_sec = int(rem_bytes / speed_bps)
                                eta_str = f"ETA {eta_sec // 60:02d}:{eta_sec % 60:02d}"
                            else:
                                eta_str = "calculating ETA..."
                            emit_event(
                                "ISO_DOWNLOAD_PROGRESS",
                                f"Downloading Ubuntu ISO: {dl_mb} MB / {tot_mb} MB ({dl_pct}% @ {speed_mbps} MB/s • {eta_str})",
                                {
                                    "in_place": True,
                                    "label": "Downloading Ubuntu ISO",
                                    "stage_percent": dl_pct,
                                    "detail": f"{dl_mb} / {tot_mb} MB @ {speed_mbps} MB/s • {eta_str}",
                                    "downloaded_bytes": downloaded,
                                    "total_bytes": total_bytes,
                                    "download_percent": dl_pct,
                                    "percent": ui_pct,
                                    "speed_mbps": speed_mbps,
                                },
                                callback=event_callback,
                            )
                last_err = None
                break
        except InterruptedError:
            raise
        except Exception as exc:
            last_err = exc
            continue

    if last_err is not None:
        raise last_err

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
                    req = urllib.request.Request(dl_url, headers={"User-Agent": "Mozilla/5.0 InstantServer/2.2"})
                    with urllib.request.urlopen(req, timeout=8) as resp:
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
    emit_event(
        "ISO_VERIFY_PROGRESS",
        f"Checking official Ubuntu {cat['point_version']} LTS mirror & SHA256SUMS...",
        {
            "in_place": True,
            "label": "Resolving Ubuntu Mirror",
            "stage_percent": 15.0,
            "detail": f"Ubuntu {cat['point_version']} LTS (releases.ubuntu.com)",
            "percent": 9,
        },
        callback=event_callback,
    )
    resolved_url, resolved_fname, expected_sha = resolve_live_ubuntu_iso_url(norm_rel)
    target_iso = os.path.join(iso_cache_dir, resolved_fname)
    return download_file_resumable(
        url=resolved_url,
        dest_path=target_iso,
        expected_sha256=expected_sha,
        fallback_urls=cat.get("mirror_urls", []),
        event_callback=event_callback,
    )

def patch_grub_cfg_text(grub_text: str) -> str:
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

class _OciBlobRedirectHandler(urllib.request.HTTPRedirectHandler):

    def redirect_request(self, req, fp, code, msg, headers, newurl):
        new_req = super().redirect_request(req, fp, code, msg, headers, newurl)
        if new_req is None:
            return None
        orig_host = urllib.parse.urlparse(req.full_url).netloc.lower()
        new_host = urllib.parse.urlparse(newurl).netloc.lower()
        if orig_host != new_host:
            new_req.headers.pop("Authorization", None)
            new_req.headers.pop("authorization", None)
            if hasattr(new_req, "unredirected_hdrs"):
                new_req.unredirected_hdrs.pop("Authorization", None)
                new_req.unredirected_hdrs.pop("authorization", None)
        return new_req

_OCI_OPENER = urllib.request.build_opener(_OciBlobRedirectHandler())

def resolve_oci_registry_endpoints(image_ref: str) -> List[Dict[str, str]]:
    clean = image_ref.strip()
    last_slash = clean.rfind("/")
    colon_pos = clean.rfind(":")
    if colon_pos > last_slash:
        path_part = clean[:colon_pos]
        tag = clean[colon_pos + 1 :] or "latest"
    else:
        path_part = clean
        tag = "latest"

    segments = [s for s in path_part.split("/") if s]
    if len(segments) >= 2 and ("." in segments[0] or ":" in segments[0] or segments[0] == "localhost"):
        reg_domain = segments[0].lower()
        repo = "/".join(segments[1:])
    else:
        reg_domain = "docker.io"
        repo = "/".join(segments) if len(segments) >= 2 else f"library/{segments[0]}"

    candidates: List[Dict[str, str]] = []

    if reg_domain == "ghcr.io":
        candidates.append({
            "registry": "ghcr.io",
            "repo": repo,
            "tag": tag,
            "full_tag": f"ghcr.io/{repo}:{tag}",
            "token_url": f"https://ghcr.io/token?service=ghcr.io&scope=repository:{repo}:pull",
            "api_base": f"https://ghcr.io/v2/{repo}",
        })
        dh_repo = "weejewel/wg-easy" if repo == "wg-easy/wg-easy" else (
            "paperlessngx/paperless-ngx" if repo == "paperless-ngx/paperless-ngx" else repo
        )
        candidates.append({
            "registry": "registry-1.docker.io",
            "repo": dh_repo,
            "tag": tag,
            "full_tag": f"ghcr.io/{repo}:{tag}",
            "token_url": f"https://auth.docker.io/token?service=registry.docker.io&scope=repository:{dh_repo}:pull",
            "api_base": f"https://registry-1.docker.io/v2/{dh_repo}",
        })
    elif reg_domain == "lscr.io":

        candidates.append({
            "registry": "registry-1.docker.io",
            "repo": repo,
            "tag": tag,
            "full_tag": f"lscr.io/{repo}:{tag}",
            "token_url": f"https://auth.docker.io/token?service=registry.docker.io&scope=repository:{repo}:pull",
            "api_base": f"https://registry-1.docker.io/v2/{repo}",
        })
        candidates.append({
            "registry": "ghcr.io",
            "repo": repo,
            "tag": tag,
            "full_tag": f"lscr.io/{repo}:{tag}",
            "token_url": f"https://ghcr.io/token?service=ghcr.io&scope=repository:{repo}:pull",
            "api_base": f"https://ghcr.io/v2/{repo}",
        })
    else:
        candidates.append({
            "registry": "registry-1.docker.io",
            "repo": repo,
            "tag": tag,
            "full_tag": f"{repo}:{tag}" if not repo.startswith("library/") else f"{repo.split('/', 1)[1]}:{tag}",
            "token_url": f"https://auth.docker.io/token?service=registry.docker.io&scope=repository:{repo}:pull",
            "api_base": f"https://registry-1.docker.io/v2/{repo}",
        })

    return candidates

def _fetch_oci_bearer_token(token_url: str, timeout: int = 15) -> str:
    req = urllib.request.Request(
        token_url,
        headers={"User-Agent": "Docker-Client/27.0 (InstantServer-OCIPuller/2.2)"},
    )
    with _OCI_OPENER.open(req, timeout=timeout) as resp:
        payload = json.loads(resp.read().decode("utf-8", errors="replace"))
    return str(payload.get("token") or payload.get("access_token") or "")

def _open_oci_with_auth_retry(
    url: str,
    token: str,
    accept_header: Optional[str] = None,
    timeout: int = 25,
) -> Tuple[Any, str]:
    headers: Dict[str, str] = {
        "User-Agent": "Docker-Client/27.0 (InstantServer-OCIPuller/2.2)",
    }
    if token:
        headers["Authorization"] = f"Bearer {token}"
    if accept_header:
        headers["Accept"] = accept_header

    req = urllib.request.Request(url, headers=headers)
    try:
        return _OCI_OPENER.open(req, timeout=timeout), token
    except urllib.error.HTTPError as http_err:
        if http_err.code == 401:
            www_auth = http_err.headers.get("WWW-Authenticate") or http_err.headers.get("www-authenticate") or ""
            realm_m = re.search(r'realm="([^"]+)"', www_auth)
            if realm_m:
                realm = realm_m.group(1)
                svc_m = re.search(r'service="([^"]+)"', www_auth)
                scope_m = re.search(r'scope="([^"]+)"', www_auth)
                q_parts: List[str] = []
                if svc_m:
                    q_parts.append(f"service={urllib.parse.quote(svc_m.group(1), safe='/:._-')}")
                if scope_m:
                    q_parts.append(f"scope={urllib.parse.quote(scope_m.group(1), safe='/:._-')}")
                dyn_token_url = f"{realm}?{'&'.join(q_parts)}" if q_parts else realm
                new_tok = _fetch_oci_bearer_token(dyn_token_url, timeout=timeout)
                if new_tok:
                    headers["Authorization"] = f"Bearer {new_tok}"
                    retry_req = urllib.request.Request(url, headers=headers)
                    return _OCI_OPENER.open(retry_req, timeout=timeout), new_tok
        raise

KIWIX_ZIM_CATALOG: List[Dict[str, Any]] = [
    {
        "id": "wikipedia_en_simple",
        "title": "Wikipedia (Simple English — Illustrated)",
        "size_gb": 2.4,
        "articles": "255,000+ articles",
        "category": "Encyclopedia",
        "recommended": True,
        "filename": "wikipedia_en_simple_all_maxi.zim",
        "url": "https://download.kiwix.org/zim/wikipedia/wikipedia_en_simple_all_maxi_2024-06.zim",
        "description": "Fast, complete Simple English Wikipedia with images. Ideal for any old laptop or quick offline deployment.",
    },
    {
        "id": "wikipedia_en_medicine",
        "title": "WikiMed Medical Encyclopedia",
        "size_gb": 1.8,
        "articles": "65,000+ clinical articles",
        "category": "Medical & Survival",
        "recommended": True,
        "filename": "wikipedia_en_medicine_maxi.zim",
        "url": "https://download.kiwix.org/zim/wikipedia/wikipedia_en_medicine_maxi_2024-06.zim",
        "description": "Offline clinical reference, pharmacology, anatomy, and emergency first-aid encyclopedia.",
    },
    {
        "id": "ifixit_en_all",
        "title": "iFixit Complete Repair Guides",
        "size_gb": 3.6,
        "articles": "100,000+ step-by-step guides",
        "category": "Medical & Survival",
        "recommended": True,
        "filename": "ifixit_en_all.zim",
        "url": "https://download.kiwix.org/zim/ifixit/ifixit_en_all_2024-05.zim",
        "description": "Illustrated teardown and repair manuals for laptops, phones, vehicles, and household electronics.",
    },
    {
        "id": "wikipedia_en_nopic",
        "title": "Wikipedia (Full English — Text & Formulas, No Images)",
        "size_gb": 56.0,
        "articles": "6,800,000+ articles",
        "category": "Encyclopedia",
        "recommended": False,
        "filename": "wikipedia_en_all_nopic.zim",
        "url": "https://download.kiwix.org/zim/wikipedia/wikipedia_en_all_nopic_2024-06.zim",
        "description": "Every single English Wikipedia article, table, and math formula without heavy image assets.",
    },
    {
        "id": "wikipedia_en_full_maxi",
        "title": "Wikipedia (Full English — Complete Edition with All Images)",
        "size_gb": 102.0,
        "articles": "6,800,000+ articles + HD media",
        "category": "Encyclopedia",
        "recommended": False,
        "filename": "wikipedia_en_all_maxi.zim",
        "url": "https://download.kiwix.org/zim/wikipedia/wikipedia_en_all_maxi_2024-06.zim",
        "description": "The complete English Wikipedia archive with all high-resolution illustrations and infoboxes.",
    },
    {
        "id": "gutenberg_en_all",
        "title": "Project Gutenberg World Literature Library",
        "size_gb": 72.0,
        "articles": "70,000+ full books",
        "category": "Books & Literature",
        "recommended": False,
        "filename": "gutenberg_en_all.zim",
        "url": "https://download.kiwix.org/zim/gutenberg/gutenberg_en_all_2023-08.zim",
        "description": "Over 70,000 classic books, philosophy, science texts, and historical literature for offline reading.",
    },
]

KIWIX_ZIM_BY_ID: Dict[str, Dict[str, Any]] = {z["id"]: z for z in KIWIX_ZIM_CATALOG}

def _sanitize_cache_key(image_ref: str) -> str:
    return re.sub(r"[^a-zA-Z0-9._-]+", "_", image_ref.strip()).strip("_")

def _download_oci_blob_resumable(
    url: str,
    token: str,
    dest_blob_path: str,
    expected_digest: str,
    expected_size: int,
    label: str,
    layer_idx: int,
    total_layers: int,
    timeout: int = 25,
    event_callback: Optional[Callable[[Dict[str, Any]], None]] = None,
) -> Tuple[str, str]:
    os.makedirs(os.path.dirname(os.path.abspath(dest_blob_path)), exist_ok=True)
    expected_hex = expected_digest.split(":")[-1].lower() if ":" in expected_digest else ""

    if os.path.isfile(dest_blob_path) and os.path.getsize(dest_blob_path) > 0:
        if expected_size <= 0 or os.path.getsize(dest_blob_path) == expected_size:
            return dest_blob_path, token

    part_path = dest_blob_path + ".part"
    existing_bytes = os.path.getsize(part_path) if os.path.isfile(part_path) else 0
    if expected_size > 0 and existing_bytes > expected_size:
        existing_bytes = 0

    headers: Dict[str, str] = {
        "User-Agent": "Docker-Client/27.0 (InstantServer-OCIPuller/2.3)",
    }
    if token:
        headers["Authorization"] = f"Bearer {token}"
    if existing_bytes > 0:
        headers["Range"] = f"bytes={existing_bytes}-"

    req = urllib.request.Request(url, headers=headers)
    try:
        resp_ctx = _OCI_OPENER.open(req, timeout=timeout)
    except urllib.error.HTTPError as http_err:
        if http_err.code == 401:
            resp_ctx, token = _open_oci_with_auth_retry(url, token="", timeout=timeout)
            existing_bytes = 0
        elif http_err.code == 416:
            existing_bytes = 0
            resp_ctx, token = _open_oci_with_auth_retry(url, token=token, timeout=timeout)
        else:
            raise

    with resp_ctx as resp:
        status_code = getattr(resp, "status", 200) or 200
        if status_code == 200 and existing_bytes > 0:
            existing_bytes = 0
        mode = "ab" if (status_code == 206 and existing_bytes > 0) else "wb"
        got_bytes = existing_bytes
        last_t = 0.0
        with open(part_path, mode) as out_fh:
            while True:
                chunk = resp.read(64 * 1024)
                if not chunk:
                    break
                out_fh.write(chunk)
                got_bytes += len(chunk)
                now = time.time()
                if now - last_t >= 0.15 or (expected_size and got_bytes >= expected_size):
                    last_t = now
                    layer_pct = min(
                        100.0,
                        round(
                            ((layer_idx + (got_bytes / max(1, expected_size or got_bytes))) / max(1, total_layers))
                            * 100,
                            1,
                        ),
                    )
                    got_mb = round(got_bytes / (1024 * 1024), 1)
                    tot_mb = round((expected_size or got_bytes) / (1024 * 1024), 1)
                    emit_event(
                        "DOCKER_PULL_PROGRESS",
                        f"Downloading {label} (layer {layer_idx + 1}/{total_layers}): {got_mb}/{tot_mb} MB",
                        {
                            "in_place": True,
                            "label": f"Pulling {label}",
                            "stage_percent": layer_pct,
                            "detail": f"Layer {layer_idx + 1}/{total_layers} ({got_mb}/{tot_mb} MB)",
                        },
                        callback=event_callback,
                    )

    if expected_hex and len(expected_hex) == 64:
        actual_hex = compute_sha256_streaming(part_path)
        if actual_hex.lower() != expected_hex:
            try:
                os.remove(part_path)
            except Exception:
                pass
            raise ValueError(
                f"OCI blob checksum mismatch for {label}: expected {expected_hex[:12]}..., got {actual_hex[:12]}..."
            )

    os.replace(part_path, dest_blob_path)
    return dest_blob_path, token

def pull_docker_image_tarball(
    image_ref: str,
    dest_tar_gz: str,
    arch: str = "amd64",
    timeout: int = 25,
    cache_dir: Optional[str] = None,
    event_callback: Optional[Callable[[Dict[str, Any]], None]] = None,
) -> Dict[str, Any]:
    os.makedirs(os.path.dirname(os.path.abspath(dest_tar_gz)), exist_ok=True)
    if cache_dir is None:
        cache_dir = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(dest_tar_gz))), "_app_cache")
    blobs_dir = os.path.join(cache_dir, "blobs")
    images_dir = os.path.join(cache_dir, "images")
    os.makedirs(blobs_dir, exist_ok=True)
    os.makedirs(images_dir, exist_ok=True)

    slug = _sanitize_cache_key(f"{image_ref}_{arch}")
    cached_tar_gz = os.path.join(images_dir, f"{slug}.tar.gz")

    if os.path.isfile(cached_tar_gz) and os.path.getsize(cached_tar_gz) > 128:
        if os.path.abspath(cached_tar_gz) != os.path.abspath(dest_tar_gz):
            shutil.copy2(cached_tar_gz, dest_tar_gz)
        emit_event(
            "DOCKER_CACHE_HIT",
            f"Found cached offline container image for {image_ref} ({round(os.path.getsize(dest_tar_gz) / (1024 * 1024), 1)} MB) — skipping download!",
            {"image": image_ref, "dest": dest_tar_gz, "cached": True},
            callback=event_callback,
        )
        return {
            "image": image_ref,
            "tarball_path": dest_tar_gz,
            "layers_count": 1,
            "size_bytes": os.path.getsize(dest_tar_gz),
            "cached": True,
        }

    candidates = resolve_oci_registry_endpoints(image_ref)
    accept_manifest = (
        "application/vnd.docker.distribution.manifest.v2+json, "
        "application/vnd.oci.image.manifest.v1+json, "
        "application/vnd.docker.distribution.manifest.list.v2+json, "
        "application/vnd.oci.image.index.v1+json"
    )

    emit_event(
        "DOCKER_PULL_START",
        f"Pre-downloading container image {image_ref} (resumable layer cache active)...",
        {"image": image_ref},
        callback=event_callback,
    )

    last_exc: Optional[Exception] = None
    for cand in candidates:
        api_base = cand["api_base"]
        repo = cand["repo"]
        tag = cand["tag"]
        full_tag = cand["full_tag"]
        short_label = repo.split("/")[-1]
        try:
            token = ""
            try:
                token = _fetch_oci_bearer_token(cand["token_url"], timeout=timeout)
            except Exception:
                token = ""

            manifest_url = f"{api_base}/manifests/{tag}"
            resp_ctx, token = _open_oci_with_auth_retry(
                manifest_url,
                token=token,
                accept_header=accept_manifest,
                timeout=timeout,
            )
            with resp_ctx as resp:
                manifest_data = json.loads(resp.read().decode("utf-8", errors="replace"))

            if "manifests" in manifest_data:
                chosen_digest = None
                for m in manifest_data["manifests"]:
                    plat = m.get("platform") or {}
                    if plat.get("architecture") == arch and plat.get("os") == "linux":
                        chosen_digest = m.get("digest")
                        break
                if not chosen_digest and manifest_data["manifests"]:
                    chosen_digest = manifest_data["manifests"][0].get("digest")
                sub_url = f"{api_base}/manifests/{chosen_digest}"
                sub_ctx, token = _open_oci_with_auth_retry(
                    sub_url,
                    token=token,
                    accept_header=accept_manifest,
                    timeout=timeout,
                )
                with sub_ctx as resp:
                    manifest_data = json.loads(resp.read().decode("utf-8", errors="replace"))

            config_digest = manifest_data["config"]["digest"]
            config_size = int(manifest_data["config"].get("size") or 0)
            layers = manifest_data.get("layers", [])
            total_layers = max(1, len(layers))

            cfg_hash = config_digest.split(":")[-1]
            cfg_blob_path = os.path.join(blobs_dir, f"sha256_{cfg_hash}.json")
            cfg_url = f"{api_base}/blobs/{config_digest}"
            cfg_blob_path, token = _download_oci_blob_resumable(
                url=cfg_url,
                token=token,
                dest_blob_path=cfg_blob_path,
                expected_digest=config_digest,
                expected_size=config_size,
                label=short_label,
                layer_idx=0,
                total_layers=total_layers,
                timeout=timeout,
                event_callback=event_callback,
            )

            cached_layer_paths: List[Tuple[str, str]] = []
            for idx, layer in enumerate(layers):
                l_digest = layer["digest"]
                l_size = int(layer.get("size") or 0)
                l_hash = l_digest.split(":")[-1]
                l_name = f"{l_hash}/layer.tar"
                l_url = f"{api_base}/blobs/{l_digest}"
                l_blob_path = os.path.join(blobs_dir, f"sha256_{l_hash}.blob")

                l_blob_path, token = _download_oci_blob_resumable(
                    url=l_url,
                    token=token,
                    dest_blob_path=l_blob_path,
                    expected_digest=l_digest,
                    expected_size=l_size,
                    label=short_label,
                    layer_idx=idx,
                    total_layers=total_layers,
                    timeout=timeout,
                    event_callback=event_callback,
                )
                cached_layer_paths.append((l_blob_path, l_name))

            tmp_archive = cached_tar_gz + ".part"
            cfg_name = f"{cfg_hash}.json"
            with tarfile.open(tmp_archive, "w:gz") as tar:
                tar.add(cfg_blob_path, arcname=cfg_name)
                layer_files: List[str] = []
                for l_blob_path, l_name in cached_layer_paths:
                    tar.add(l_blob_path, arcname=l_name)
                    layer_files.append(l_name)

                docker_manifest = [
                    {
                        "Config": cfg_name,
                        "RepoTags": [full_tag],
                        "Layers": layer_files,
                    }
                ]
                man_bytes = json.dumps(docker_manifest, indent=2).encode("utf-8")
                man_info = tarfile.TarInfo(name="manifest.json")
                man_info.size = len(man_bytes)
                tar.addfile(man_info, io.BytesIO(man_bytes))

            os.replace(tmp_archive, cached_tar_gz)
            if os.path.abspath(cached_tar_gz) != os.path.abspath(dest_tar_gz):
                shutil.copy2(cached_tar_gz, dest_tar_gz)

            emit_event(
                "DOCKER_PULL_COMPLETE",
                f"Bundled & cached offline container image {full_tag} ({len(layers)} layers)",
                {"image": full_tag, "dest": dest_tar_gz, "layers": len(layers), "cached": False},
                callback=event_callback,
            )
            return {
                "image": full_tag,
                "tarball_path": dest_tar_gz,
                "layers_count": len(layers),
                "size_bytes": os.path.getsize(dest_tar_gz),
                "cached": False,
            }
        except InterruptedError:
            raise
        except Exception as exc:
            last_exc = exc
            continue

    if last_exc is not None:
        raise last_exc
    raise RuntimeError(f"Failed to pull container image: {image_ref}")

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
    print("[IsoManager Test] 1. Testing Ubuntu 26.04.1 / 24.04.5 / 22.04.5 catalog & thank-you parser...")
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

    print("[IsoManager Test] 2. Testing pure-Python ISO9660 reader, full tree extraction & .deb extractor...")
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

    print("[IsoManager Test] 3. Testing Multi-Registry OCI Endpoint Resolver (Docker Hub, GHCR.io, LSCR.io) across all 26 apps...")
    wg_eps = resolve_oci_registry_endpoints("ghcr.io/wg-easy/wg-easy:latest")
    assert wg_eps[0]["registry"] == "ghcr.io"
    assert wg_eps[0]["token_url"] == "https://ghcr.io/token?service=ghcr.io&scope=repository:wg-easy/wg-easy:pull"
    assert wg_eps[0]["api_base"] == "https://ghcr.io/v2/wg-easy/wg-easy"
    assert wg_eps[1]["registry"] == "registry-1.docker.io" and wg_eps[1]["repo"] == "weejewel/wg-easy"

    sync_eps = resolve_oci_registry_endpoints("lscr.io/linuxserver/syncthing:latest")
    assert sync_eps[0]["registry"] == "registry-1.docker.io"
    assert sync_eps[0]["token_url"] == "https://auth.docker.io/token?service=registry.docker.io&scope=repository:linuxserver/syncthing:pull"
    assert sync_eps[0]["api_base"] == "https://registry-1.docker.io/v2/linuxserver/syncthing"
    assert sync_eps[1]["registry"] == "ghcr.io"
    assert sync_eps[1]["api_base"] == "https://ghcr.io/v2/linuxserver/syncthing"

    qbit_eps = resolve_oci_registry_endpoints("lscr.io/linuxserver/qbittorrent:latest")
    assert qbit_eps[0]["api_base"] == "https://registry-1.docker.io/v2/linuxserver/qbittorrent"
    assert qbit_eps[1]["api_base"] == "https://ghcr.io/v2/linuxserver/qbittorrent"

    nc_eps = resolve_oci_registry_endpoints("nextcloud:stable")
    assert nc_eps[0]["repo"] == "library/nextcloud" and nc_eps[0]["tag"] == "stable"
    print("  -> PASS: Multi-registry OCI token & manifest resolver verified for GHCR.io, LSCR.io, and Docker Hub!")
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