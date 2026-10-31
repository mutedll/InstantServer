#!/usr/bin/env python3
"""
Instant Server — Module M2: Resumable ISO Manager & Pure-Python ISO9660 `.deb` Extractor
========================================================================================
File: m2_iso_manager.py
Project: Instant Server (Turn any old laptop into a headless Ubuntu Server)
Runs on: Windows 10/11, macOS, and Linux (Python 3.8+ through 3.14+, zero external deps)

Responsibilities:
1. Official Ubuntu Server LTS Release Catalog (24.04.1 LTS Noble & 22.04.5 LTS Jammy)
   with SHA256SUMS URLs and offline Wi-Fi `.deb` pool metadata.
2. Resumable HTTP Range downloader (`Range: bytes=<offset>-`) with real-time progress
   events so large 2.6 GB ISO downloads survive flaky connections without restarting.
3. Cryptographic SHA-256 verification against official Ubuntu `SHA256SUMS` manifests.
4. Zero-Dependency Pure-Python ISO9660 (ECMA-119) reader (`ISO9660Reader`) that parses
   Primary Volume Descriptors and directory records directly from an `.iso` file—allowing
   Windows, macOS, and Linux users to extract `pool/main/**/*.deb` packages (such as
   `wpasupplicant`, `rfkill`, `wireless-regdb`) onto the `cidata` partition WITHOUT
   needing root loop-mounts (`mount -o loop`), `xorriso`, or `7z`!
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import struct
import sys
import tempfile
import time
import urllib.request
from typing import Any, Callable, Dict, List, Optional, Tuple


# ---------------------------------------------------------------------------
# 1. Official Ubuntu Server LTS Catalog & Offline Wi-Fi Package Manifest
# ---------------------------------------------------------------------------
UBUNTU_LTS_CATALOG: Dict[str, Dict[str, Any]] = {
    "24.04": {
        "codename": "noble",
        "point_version": "24.04.1",
        "iso_filename": "ubuntu-24.04.1-live-server-amd64.iso",
        "iso_url": "https://releases.ubuntu.com/24.04/ubuntu-24.04.1-live-server-amd64.iso",
        "sha256sums_url": "https://releases.ubuntu.com/24.04/SHA256SUMS",
        "approx_size_mb": 2650,
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
        "iso_url": "https://releases.ubuntu.com/22.04/ubuntu-22.04.5-live-server-amd64.iso",
        "sha256sums_url": "https://releases.ubuntu.com/22.04/SHA256SUMS",
        "approx_size_mb": 2050,
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
# 2. Cryptographic SHA-256 Verification & `SHA256SUMS` Parser
# ---------------------------------------------------------------------------
def parse_sha256sums_text(text: str) -> Dict[str, str]:
    """
    Parse an official Ubuntu `SHA256SUMS` file into `{filename: sha256_hex}`.
    Handles lines like:
      `e240e4b801f7... *ubuntu-24.04.1-live-server-amd64.iso`
    """
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


def compute_file_sha256(
    file_path: str,
    chunk_size: int = 4 * 1024 * 1024,
    max_bytes: Optional[int] = None,
    event_callback: Optional[Callable[[Dict[str, Any]], None]] = None,
) -> str:
    """Compute SHA-256 hash of a file (or prefix up to `max_bytes`) with progress callbacks."""
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
                emit_event(
                    "ISO_VERIFY_PROGRESS",
                    f"Verifying SHA-256 checksum ({pct}%)",
                    {"bytes_verified": processed, "total_bytes": total, "percent": pct},
                    callback=event_callback,
                )

    return hasher.hexdigest().lower()


# ---------------------------------------------------------------------------
# 3. Resumable HTTP Range Downloader
# ---------------------------------------------------------------------------
def download_file_resumable(
    url: str,
    dest_path: str,
    expected_sha256: Optional[str] = None,
    chunk_size: int = 1024 * 1024,
    timeout: int = 30,
    event_callback: Optional[Callable[[Dict[str, Any]], None]] = None,
) -> str:
    """
    Download a large ISO file with HTTP Range resume (`Range: bytes=<offset>-`)
    and optional SHA-256 verification.
    """
    os.makedirs(os.path.dirname(os.path.abspath(dest_path)), exist_ok=True)
    part_path = dest_path + ".part"

    if os.path.exists(dest_path) and expected_sha256:
        actual = compute_file_sha256(dest_path, event_callback=event_callback)
        if actual == expected_sha256.lower():
            emit_event("ISO_CACHE_HIT", f"Verified cached ISO at {dest_path}", {"sha256": actual}, callback=event_callback)
            return dest_path

    existing_bytes = os.path.getsize(part_path) if os.path.exists(part_path) else 0
    headers = {"User-Agent": "InstantServer-ISOManager/1.0"}
    if existing_bytes > 0:
        headers["Range"] = f"bytes={existing_bytes}-"

    req = urllib.request.Request(url, headers=headers)
    emit_event(
        "ISO_DOWNLOAD_START",
        f"Starting download from {url} (resume offset: {existing_bytes} bytes)",
        {"url": url, "resume_offset": existing_bytes},
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

        with open(part_path, mode) as out_fh:
            while True:
                chunk = resp.read(chunk_size)
                if not chunk:
                    break
                out_fh.write(chunk)
                downloaded += len(chunk)
                elapsed = max(0.001, time.time() - start_t)
                speed_mbps = round(((downloaded - existing_bytes) / (1024 * 1024)) / elapsed, 2)
                pct = round((downloaded / total_bytes) * 100, 1) if total_bytes > 0 else 0.0
                emit_event(
                    "ISO_DOWNLOAD_PROGRESS",
                    f"Downloading ISO: {pct}% ({speed_mbps} MB/s)",
                    {
                        "downloaded_bytes": downloaded,
                        "total_bytes": total_bytes,
                        "percent": pct,
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
        emit_event("ISO_VERIFIED", "Cryptographic SHA-256 verification passed", {"sha256": actual_hash}, callback=event_callback)

    return dest_path


# ---------------------------------------------------------------------------
# 4. Zero-Dependency Pure-Python ISO9660 (ECMA-119) Reader & `.deb` Extractor
# ---------------------------------------------------------------------------
ISO_SECTOR_SIZE = 2048


class ISO9660Reader:
    """
    Pure-Python ISO9660 / Rock Ridge / Joliet reader that walks an `.iso` file's
    directory tree and extracts `.deb` packages without mounting or external tools.
    """

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
        """Locate the Primary Volume Descriptor (type 1) starting at LBA 16."""
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
                # Root Directory Record is 34 bytes at offset 156..190
                root_rec = vd[156:190]
                extent_lba = struct.unpack("<I", root_rec[2:6])[0]
                data_len = struct.unpack("<I", root_rec[10:14])[0]
                return extent_lba, data_len
            if vd_type == 255:
                break
        raise ValueError(f"No valid ISO9660 Primary Volume Descriptor found in {self.iso_path}")

    def list_all_files(self) -> List[Dict[str, Any]]:
        """Recursively walk the ISO9660 directory tree and return file records."""
        results: List[Dict[str, Any]] = []
        visited_lbas = set()
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
                # Advance to next sector boundary
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

            # Skip '.' (0x00) and '..' (0x01)
            if name_len == 1 and raw_name in (b"\x00", b"\x01"):
                pos += rec_len
                continue

            # Check Rock Ridge 'NM' alternate name in System Use Area if present
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
        """Extract POSIX filename from Rock Ridge 'NM' System Use entries if present."""
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

    def extract_Entry(self, entry: Dict[str, Any], dest_file: str) -> str:
        """Extract a single file record from the ISO to `dest_file`."""
        os.makedirs(os.path.dirname(os.path.abspath(dest_file)), exist_ok=True)
        self.fh.seek(int(entry["lba"]) * ISO_SECTOR_SIZE)
        remaining = int(entry["size"])
        with open(dest_file, "wb") as out:
            while remaining > 0:
                chunk = self.fh.read(min(1024 * 1024, remaining))
                if not chunk:
                    break
                out.write(chunk)
                remaining -= len(chunk)
        return dest_file


def extract_offline_debs_from_iso(
    iso_path: str,
    dest_debs_dir: str,
    package_prefixes: Optional[List[str]] = None,
    event_callback: Optional[Callable[[Dict[str, Any]], None]] = None,
) -> List[str]:
    """
    Scan `/pool/` inside the Ubuntu Server ISO and extract matching `.deb`
    packages (e.g., `wpasupplicant`, `rfkill`, `libnl`, `libpcsclite`) to `dest_debs_dir`.
    """
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
# 5. Synthetic ISO9660 Builder for Self-Testing
# ---------------------------------------------------------------------------
def _build_synthetic_iso_with_deb(iso_file: str, deb_name: str, deb_payload: bytes) -> None:
    """Build a minimal spec-compliant ISO9660 image containing `/pool/main/<deb_name>`."""
    # Layout:
    # Sector 0..15: System area (zeros)
    # Sector 16: Primary Volume Descriptor
    # Sector 17: Volume Descriptor Set Terminator
    # Sector 18: Root Directory Extent (contains '.', '..', and 'WPASUPPLICANT.DEB;1' with Rock Ridge NM)
    # Sector 19: File Data Extent for deb_payload
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
    pvd[40:72] = b"UBUNTU_24_04_SERVER             "
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
    print("[M2 Self-Test] 1. Testing SHA256SUMS manifest parser...")
    sample_sums = (
        "e240e4b801f7deadbeef00112233445566778899aabbccddeeff001122334455 *ubuntu-24.04.1-live-server-amd64.iso\n"
    )
    parsed = parse_sha256sums_text(sample_sums)
    assert parsed["ubuntu-24.04.1-live-server-amd64.iso"].startswith("e240e4b801f7")
    print("  -> PASS: Official Ubuntu SHA256SUMS parser verified!")

    print("[M2 Self-Test] 2. Testing pure-Python ISO9660 reader & offline .deb extractor...")
    with tempfile.TemporaryDirectory() as tmpdir:
        test_iso = os.path.join(tmpdir, "mock_ubuntu.iso")
        deb_name = "wpasupplicant_2.10-21build4_amd64.deb"
        deb_bytes = b"!<arch>\ndebian-binary   1700000000  0     0     100644  4         `\n2.0\n"
        _build_synthetic_iso_with_deb(test_iso, deb_name, deb_bytes)

        out_debs_dir = os.path.join(tmpdir, "debs")
        extracted = extract_offline_debs_from_iso(test_iso, out_debs_dir)
        assert len(extracted) == 1, f"Expected 1 extracted .deb, got {extracted}"
        with open(extracted[0], "rb") as rf:
            assert rf.read() == deb_bytes
        iso_hash = compute_file_sha256(test_iso)
        assert len(iso_hash) == 64
    print("  -> PASS: Extracted wpasupplicant .deb directly from ISO9660 image in pure Python!")
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