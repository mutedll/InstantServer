#!/usr/bin/env python3
"""
Instant Server — Phase 3: Unified Cross-Platform Desktop App Launcher, Local Config Store & CLI
===============================================================================================
File: instant_server_cli.py
Project: Instant Server (Turn any old laptop into a headless Ubuntu Server)
Runs on: Windows 10/11, macOS, and Linux (Python 3.8+ through 3.14+, zero external deps)

Responsibilities:
1. Automatic Workspace Folder Next to the Executable (`Instant_Server_Data/`):
   - Detects the folder where the `.exe` or script was launched from (`sys.executable` when
     frozen by PyInstaller, or script directory when run via Python) and automatically creates
     `Instant_Server_Data/` containing `instant-server-config.json`, `cidata/`, `iso_cache/`,
     `logs/`, and the taskbar icons (`instant_server_icon.ico`, `.png`, `.icns`).
2. True Bootable ISO Pulling + USB Flashing Pipeline (`AsyncPipelineManager`):
   - Automatically pulls the official Ubuntu Server LTS ISO (`ubuntu-26.04.1-live-server-amd64.iso`
     or `22.04.5`) into `Instant_Server_Data/iso_cache/` (or uses a user-selected local `.iso` file),
     stages `cidata` + all pre-unpacked Self-Hosted App Store bundles (`apps/<app_id>/`),
     formats the USB drive (`GPT` for UEFI, `MBR` for Legacy BIOS, or `Auto` Universal),
     and writes the full bootable Ubuntu Server ISO + patched GRUB `autoinstall` + `CIDATA`!
3. Guaranteed Automatic Config Read on Next Launch & Zero PowerShell Popup on Windows.
"""

from __future__ import annotations

import argparse
from concurrent.futures import ThreadPoolExecutor, as_completed
from dataclasses import asdict
import http.server
import ipaddress
import json
import math
import os
import re
import shutil
import socket
import struct
import subprocess
import sys
import tempfile
import threading
import time
from typing import Any, Dict, List, Optional, Tuple
import zlib

# ---------------------------------------------------------------------------
# 0. Frozen PyInstaller (`sys._MEIPASS`) & Launch Folder Resolution
# ---------------------------------------------------------------------------
_BUNDLED_DIR = getattr(sys, "_MEIPASS", os.path.dirname(os.path.abspath(__file__)))
if _BUNDLED_DIR not in sys.path:
    sys.path.insert(0, _BUNDLED_DIR)


def _win_no_window_kwargs() -> Dict[str, Any]:
    """Return `creationflags=CREATE_NO_WINDOW` on Windows to suppress console popups."""
    if sys.platform.startswith("win"):
        return {"creationflags": getattr(subprocess, "CREATE_NO_WINDOW", 0x08000000)}
    return {}


def get_launch_root_dir() -> str:
    """
    Return the directory where the user launched the `.exe` or Python script from.
    """
    if getattr(sys, "frozen", False):
        exe_dir = os.path.dirname(os.path.abspath(sys.executable))
        if os.access(exe_dir, os.W_OK):
            return exe_dir
    script_dir = os.path.dirname(os.path.abspath(__file__))
    if os.access(script_dir, os.W_OK):
        return script_dir
    return os.getcwd()


import m1_host_probe  # noqa: E402
import m2_iso_manager  # noqa: E402
import m3_usb_engine  # noqa: E402
import m4_config_builder  # noqa: E402
import m5_payload_packager  # noqa: E402

WORKSPACE_FOLDER_NAME = "Instant_Server_Data"
CONFIG_FILE_NAME = "instant-server-config.json"


def ensure_workspace_folder(base_dir: Optional[str] = None) -> Dict[str, str]:
    """
    Automatically create the `Instant_Server_Data/` folder (and its subfolders
    `cidata/`, `iso_cache/`, `logs/`) inside the folder where the `.exe` or script
    was opened from.
    """
    root = base_dir or get_launch_root_dir()
    workspace_dir = os.path.join(root, WORKSPACE_FOLDER_NAME)
    cidata_dir = os.path.join(workspace_dir, "cidata")
    iso_dir = os.path.join(workspace_dir, "iso_cache")
    logs_dir = os.path.join(workspace_dir, "logs")
    for d in (workspace_dir, cidata_dir, iso_dir, logs_dir):
        os.makedirs(d, exist_ok=True)

    icon_ico_path = os.path.join(workspace_dir, "instant_server_icon.ico")
    icon_png_path = os.path.join(workspace_dir, "instant_server_icon.png")
    icon_icns_path = os.path.join(workspace_dir, "instant_server_icon.icns")
    if not (os.path.isfile(icon_ico_path) and os.path.isfile(icon_png_path) and os.path.isfile(icon_icns_path)):
        try:
            generate_photoshop_style_is_icon(icon_ico_path, icon_png_path, icon_icns_path)
        except Exception:
            pass

    return {
        "root_dir": root,
        "workspace_dir": workspace_dir,
        "config_path": os.path.join(workspace_dir, CONFIG_FILE_NAME),
        "legacy_config_path": os.path.join(root, CONFIG_FILE_NAME),
        "cidata_dir": cidata_dir,
        "iso_dir": iso_dir,
        "logs_dir": logs_dir,
        "icon_ico_path": icon_ico_path,
        "icon_png_path": icon_png_path,
        "icon_icns_path": icon_icns_path,
    }


def get_config_file_path(base_dir: Optional[str] = None) -> str:
    paths = ensure_workspace_folder(base_dir)
    return paths["config_path"]


def load_saved_config(base_dir: Optional[str] = None) -> Dict[str, Any]:
    paths = ensure_workspace_folder(base_dir)
    candidates = [paths["config_path"], paths["legacy_config_path"]]
    for cfg_path in candidates:
        if os.path.isfile(cfg_path):
            try:
                with open(cfg_path, "r", encoding="utf-8") as fh:
                    data = json.load(fh)
                if isinstance(data, dict) and data:
                    if cfg_path != paths["config_path"]:
                        try:
                            with open(paths["config_path"], "w", encoding="utf-8") as out_fh:
                                json.dump(data, out_fh, indent=2)
                        except Exception:
                            pass
                    return {
                        "exists": True,
                        "path": paths["config_path"],
                        "workspace_dir": paths["workspace_dir"],
                        "config": data,
                    }
            except Exception as exc:
                return {
                    "exists": False,
                    "path": paths["config_path"],
                    "workspace_dir": paths["workspace_dir"],
                    "config": None,
                    "error": str(exc),
                }
    return {
        "exists": False,
        "path": paths["config_path"],
        "workspace_dir": paths["workspace_dir"],
        "config": None,
    }


def save_local_config(config_data: Dict[str, Any], base_dir: Optional[str] = None) -> Dict[str, Any]:
    paths = ensure_workspace_folder(base_dir)
    cfg_path = paths["config_path"]
    with open(cfg_path, "w", encoding="utf-8") as fh:
        json.dump(config_data, fh, indent=2)
    return {
        "saved": True,
        "path": cfg_path,
        "workspace_dir": paths["workspace_dir"],
        "config": config_data,
    }


def _filter_server_profile_kwargs(raw: Dict[str, Any]) -> Dict[str, Any]:
    allowed = set(m4_config_builder.ServerProfile.__dataclass_fields__.keys())
    filtered: Dict[str, Any] = {}
    for k, v in raw.items():
        if k in allowed:
            filtered[k] = v
    if not filtered.get("password") and not filtered.get("ssh_authorized_keys") and not filtered.get("github_ssh_username"):
        filtered["password"] = "instant2026"
    return filtered


# ---------------------------------------------------------------------------
# 0b. Pure-Python Photoshop-Style `IS` Icon Generator (.ico, .png, .icns)
# ---------------------------------------------------------------------------
def _render_photoshop_is_rgba(size: int = 128) -> bytes:
    bg_rgb = (0, 30, 54)
    border_rgb = (49, 168, 255)
    fg_rgb = (102, 196, 255)

    rows = bytearray()
    radius = size * 0.21
    border_w = max(2.0, size * 0.045)

    for y in range(size):
        rows.append(0)
        ny = y / float(size)
        for x in range(size):
            nx = x / float(size)

            dx = max(abs(x - (size - 1) / 2.0) - (size / 2.0 - radius), 0.0)
            dy = max(abs(y - (size - 1) / 2.0) - (size / 2.0 - radius), 0.0)
            dist = math.hypot(dx, dy) - radius

            if dist > 0.8:
                rows.extend((0, 0, 0, 0))
                continue

            alpha = 255 if dist < -0.8 else int(255 * (0.8 - dist) / 1.6)
            is_border = dist >= -border_w

            in_i = False
            if 0.24 <= ny <= 0.76:
                if (0.24 <= ny <= 0.315 or 0.685 <= ny <= 0.76) and (0.18 <= nx <= 0.42):
                    in_i = True
                elif 0.265 <= nx <= 0.335:
                    in_i = True

            in_s = False
            if 0.24 <= ny <= 0.76 and 0.47 <= nx <= 0.81:
                if 0.24 <= ny <= 0.315 and 0.51 <= nx <= 0.79:
                    in_s = True
                elif 0.685 <= ny <= 0.76 and 0.48 <= nx <= 0.76:
                    in_s = True
                elif 0.465 <= ny <= 0.535 and 0.51 <= nx <= 0.76:
                    in_s = True
                elif 0.28 <= ny <= 0.50 and 0.48 <= nx <= 0.555:
                    in_s = True
                elif 0.50 <= ny <= 0.72 and 0.725 <= nx <= 0.80:
                    in_s = True
                elif 0.26 <= ny <= 0.35 and 0.725 <= nx <= 0.795:
                    in_s = True
                elif 0.65 <= ny <= 0.74 and 0.48 <= nx <= 0.55:
                    in_s = True

            if in_i or in_s:
                r, g, b = fg_rgb
            elif is_border:
                r, g, b = border_rgb
            else:
                r, g, b = bg_rgb

            rows.extend((r, g, b, alpha))

    return bytes(rows)


def build_png_bytes(size: int = 128) -> bytes:
    raw_scanlines = _render_photoshop_is_rgba(size=size)

    def chunk(tag: bytes, data: bytes) -> bytes:
        crc = zlib.crc32(tag + data) & 0xFFFFFFFF
        return struct.pack(">I", len(data)) + tag + data + struct.pack(">I", crc)

    ihdr = struct.pack(">IIBBBBB", size, size, 8, 6, 0, 0, 0)
    idat = zlib.compress(raw_scanlines, level=9)
    return b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", ihdr) + chunk(b"IDAT", idat) + chunk(b"IEND", b"")


def generate_photoshop_style_is_icon(
    ico_path: str,
    png_path: Optional[str] = None,
    icns_path: Optional[str] = None,
) -> None:
    png_data = build_png_bytes(size=128)
    if png_path:
        with open(png_path, "wb") as pfh:
            pfh.write(png_data)

    ico_header = struct.pack("<HHH", 0, 1, 1)
    ico_entry = struct.pack("<BBBBHHII", 128, 128, 0, 0, 1, 32, len(png_data), 22)
    with open(ico_path, "wb") as ifh:
        ifh.write(ico_header + ico_entry + png_data)

    if icns_path:
        chunk_len = 8 + len(png_data)
        total_len = 8 + chunk_len
        icns_bytes = b"icns" + struct.pack(">I", total_len) + b"ic07" + struct.pack(">I", chunk_len) + png_data
        with open(icns_path, "wb") as mfh:
            mfh.write(icns_bytes)


# ---------------------------------------------------------------------------
# 1. Pure-Python RFC 6762 mDNS Packet Builder & Dual-Mode LAN Discovery Engine
# ---------------------------------------------------------------------------
MDNS_MULTICAST_IP = "224.0.0.251"
MDNS_PORT = 5353


def build_mdns_query_packet(qname: str, qtype: int = 1) -> bytes:
    header = struct.pack(">HHHHHH", 0, 0, 1, 0, 0, 0)
    labels: List[bytes] = []
    for part in qname.strip(".").split("."):
        encoded = part.encode("utf-8")
        if len(encoded) > 63:
            raise ValueError(f"DNS label too long: {part}")
        labels.append(struct.pack("B", len(encoded)) + encoded)
    labels.append(b"\x00")
    question = b"".join(labels) + struct.pack(">HH", qtype, 0x0001)
    return header + question


def parse_mdns_a_record_response(packet: bytes) -> Optional[str]:
    if len(packet) < 12:
        return None
    _tid, flags, qdcount, ancount, _nscount, _arcount = struct.unpack(">HHHHHH", packet[:12])
    if ancount == 0 or not (flags & 0x8000):
        return None

    pos = 12
    n = len(packet)

    def skip_name(idx: int) -> int:
        while idx < n:
            length = packet[idx]
            if length == 0:
                return idx + 1
            if (length & 0xC0) == 0xC0:
                return idx + 2
            idx += 1 + length
        return idx

    for _ in range(qdcount):
        pos = skip_name(pos) + 4

    for _ in range(ancount):
        if pos >= n:
            break
        pos = skip_name(pos)
        if pos + 10 > n:
            break
        rtype, _rclass, _ttl, rdlen = struct.unpack(">HHIH", packet[pos : pos + 10])
        pos += 10
        if pos + rdlen > n:
            break
        if rtype == 1 and rdlen == 4:
            return socket.inet_ntoa(packet[pos : pos + 4])
        pos += rdlen
    return None


def query_mdns_hostname(hostname: str, timeout: float = 0.6) -> Optional[str]:
    fqdn = hostname if hostname.endswith(".local") else f"{hostname}.local"
    try:
        ip = socket.gethostbyname(fqdn)
        if ip and not ip.startswith("127."):
            return ip
    except Exception:
        pass

    try:
        pkt = build_mdns_query_packet(fqdn, qtype=1)
        sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM, socket.IPPROTO_UDP)
        sock.settimeout(timeout)
        try:
            sock.sendto(pkt, (MDNS_MULTICAST_IP, MDNS_PORT))
            data, addr = sock.recvfrom(1024)
            parsed_ip = parse_mdns_a_record_response(data)
            return parsed_ip or addr[0]
        finally:
            sock.close()
    except Exception:
        return None


def probe_tcp_service(
    ip: str,
    ports: Tuple[int, ...] = (9090, 80, 22),
    timeout: float = 0.25,
) -> Dict[str, Any]:
    open_ports: List[int] = []
    ssh_banner = ""

    for port in ports:
        s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        s.settimeout(timeout)
        try:
            if s.connect_ex((ip, port)) == 0:
                open_ports.append(port)
                if port == 22:
                    try:
                        raw = s.recv(128)
                        ssh_banner = raw.decode("utf-8", errors="replace").strip()
                    except Exception:
                        pass
        except Exception:
            pass
        finally:
            s.close()

    is_instant_candidate = (9090 in open_ports) or (22 in open_ports and "Ubuntu" in ssh_banner)
    return {
        "ip": ip,
        "open_ports": open_ports,
        "ssh_banner": ssh_banner,
        "is_instant_candidate": is_instant_candidate,
        "cockpit_url": f"https://{ip}:9090" if 9090 in open_ports else None,
        "casaos_url": f"http://{ip}" if 80 in open_ports else None,
        "ssh_cmd": f"ssh admin@{ip}" if 22 in open_ports else None,
    }


def get_local_ipv4_and_arp_neighbors() -> Tuple[str, List[str]]:
    local_ip = "127.0.0.1"
    try:
        s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        s.connect(("1.1.1.1", 80))
        local_ip = s.getsockname()[0]
        s.close()
    except Exception:
        pass

    neighbors: List[str] = []
    if os.path.exists("/proc/net/arp"):
        try:
            with open("/proc/net/arp", "r", encoding="utf-8") as fh:
                for line in fh.read().splitlines()[1:]:
                    parts = line.split()
                    if parts and re.match(r"^\d+\.\d+\.\d+\.\d+$", parts[0]):
                        neighbors.append(parts[0])
        except Exception:
            pass
    else:
        try:
            res = subprocess.run(
                ["arp", "-a"],
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                text=True,
                check=False,
                **_win_no_window_kwargs(),
            )
            for match in re.findall(r"\b(?:\d{1,3}\.){3}\d{1,3}\b", res.stdout or ""):
                if not match.startswith(("224.", "239.", "255.")):
                    neighbors.append(match)
        except Exception:
            pass

    return local_ip, sorted(set(neighbors))


def discover_instant_servers_on_lan(
    hostname: str = "instant-server",
    candidate_ips: Optional[List[str]] = None,
    ports: Tuple[int, ...] = (9090, 80, 22),
    timeout: float = 0.25,
) -> Dict[str, Any]:
    mdns_ip = query_mdns_hostname(hostname, timeout=timeout)
    local_ip, arp_ips = get_local_ipv4_and_arp_neighbors()

    if candidate_ips is None:
        pool: List[str] = []
        if mdns_ip:
            pool.append(mdns_ip)
        for aip in arp_ips:
            if aip not in pool:
                pool.append(aip)
        if not pool and local_ip != "127.0.0.1":
            net = ipaddress.ip_network(f"{local_ip}/24", strict=False)
            pool = [str(h) for h in list(net.hosts())[:64]]
        candidate_ips = pool

    discovered: List[Dict[str, Any]] = []
    if candidate_ips:
        with ThreadPoolExecutor(max_workers=min(32, max(1, len(candidate_ips)))) as ex:
            futures = {ex.submit(probe_tcp_service, ip, ports, timeout): ip for ip in candidate_ips}
            for fut in as_completed(futures):
                info = fut.result()
                if info["open_ports"]:
                    discovered.append(info)

    discovered.sort(key=lambda d: (0 if 9090 in d["open_ports"] else 1, d["ip"]))
    return {
        "queried_mdns": f"{hostname}.local",
        "mdns_resolved_ip": mdns_ip,
        "host_local_ip": local_ip,
        "scanned_count": len(candidate_ips),
        "servers_found": discovered,
    }


# ---------------------------------------------------------------------------
# 2. Asynchronous Job Runner, Cancellation, & Local HTTP / `pywebview` GUI Bridge
# ---------------------------------------------------------------------------
IN_PLACE_PROGRESS_EVENTS = {
    "ISO_DOWNLOAD_PROGRESS",
    "ISO_VERIFY_PROGRESS",
    "ISO_EXTRACT_PROGRESS",
    "USB_RAW_FLASH_PROGRESS",
    "DOCKER_PULL_PROGRESS",
}


def format_terminal_progress_bar(
    stage_percent: float,
    label: str = "Progress",
    detail: str = "",
    width: int = 22,
) -> str:
    """Build a clean terminal-style single-line progress bar (`[████████░░░░] 62.4% │ ...`)."""
    clamped = max(0.0, min(100.0, float(stage_percent)))
    filled = int(round((clamped / 100.0) * width))
    bar = ("█" * filled) + ("░" * max(0, width - filled))
    pct_str = f"{clamped:5.1f}%"
    if detail:
        return f"⟳ {label}: [{bar}] {pct_str} │ {detail}"
    return f"⟳ {label}: [{bar}] {pct_str}"


class AsyncPipelineManager:
    def __init__(self, base_dir: Optional[str] = None) -> None:
        self.base_dir = base_dir
        self._lock = threading.Lock()
        self._cancel_requested = threading.Event()
        self.status: str = "IDLE"
        self.percent: int = 0
        self.job_start_time: float = 0.0
        self.events: List[Dict[str, Any]] = []
        self.active_progress: Optional[Dict[str, Any]] = None
        self.last_result: Optional[Dict[str, Any]] = None
        self.error: Optional[str] = None

    def _elapsed_tag(self) -> str:
        elapsed = max(0, int(time.time() - (self.job_start_time or time.time())))
        mins = elapsed // 60
        secs = elapsed % 60
        return f"[{mins:02d}:{secs:02d}]"

    def _record_event(self, payload: Dict[str, Any]) -> None:
        ev = payload.get("event", "")
        data = payload.get("data") or {}

        # Honor user cancellation immediately between any chunk/file step
        if self._cancel_requested.is_set() and ev not in ("JOB_CANCELLED", "JOB_FAILED"):
            raise InterruptedError("Flashing cancelled by user.")

        with self._lock:
            if "percent" in data:
                self.percent = max(self.percent, int(data["percent"]))
            elif ev == "M4_COMPILE_COMPLETE":
                self.percent = max(self.percent, 45)
            elif ev == "M5_STAGING_COMPLETE":
                self.percent = max(self.percent, 60)
            elif ev == "USB_COPY_CIDATA":
                self.percent = max(self.percent, 88)
            elif ev == "USB_PROVISION_COMPLETE":
                self.percent = 100

            # In-place terminal progress updates overwrite active_progress instead of spamming new lines!
            if ev in IN_PLACE_PROGRESS_EVENTS or data.get("in_place") is True:
                stage_pct = float(
                    data.get(
                        "stage_percent",
                        data.get("download_percent", data.get("percent", self.percent)),
                    )
                )
                label = str(data.get("label") or "Progress")
                detail = str(data.get("detail") or payload.get("message") or "")
                self.active_progress = {
                    "event": ev,
                    "label": label,
                    "stage_percent": round(stage_pct, 1),
                    "detail": detail,
                    "message": payload.get("message", ""),
                    "terminal_line": format_terminal_progress_bar(stage_pct, label=label, detail=detail),
                }
                return

            # Clear in-place bar when a transfer phase completes
            if ev in (
                "ISO_VERIFIED",
                "ISO_CACHE_HIT",
                "ISO_CACHE_FOUND",
                "USB_COPY_CIDATA",
                "USB_PROVISION_COMPLETE",
                "JOB_FAILED",
                "JOB_CANCELLED",
            ):
                self.active_progress = None

            entry = dict(payload)
            entry["seq"] = len(self.events) + 1
            entry["elapsed"] = self._elapsed_tag()
            entry["formatted_line"] = f"{entry['elapsed']} {entry.get('message', '')}"
            self.events.append(entry)

    def cancel_job(self) -> Dict[str, Any]:
        with self._lock:
            if self.status != "RUNNING":
                return {"cancelled": False, "status": self.status}
            self._cancel_requested.set()
        return {"cancelled": True, "status": "CANCELLING"}

    def start_stage_job(
        self,
        profile_dict: Dict[str, Any],
        output_dir: Optional[str] = None,
        target_device: Optional[str] = None,
        iso_path: Optional[str] = None,
        auto_download_iso: Optional[bool] = None,
    ) -> Dict[str, Any]:
        paths = ensure_workspace_folder(self.base_dir)
        resolved_out_dir = output_dir or paths["cidata_dir"]
        with self._lock:
            if self.status == "RUNNING":
                return {"started": False, "reason": "Job already running"}
            self._cancel_requested.clear()
            self.status = "RUNNING"
            self.percent = 5
            self.job_start_time = time.time()
            self.events = []
            self.active_progress = None
            self.last_result = None
            self.error = None

        def worker() -> None:
            try:
                self._record_event(
                    {
                        "event": "JOB_STARTED",
                        "module": "CLI_BRIDGE",
                        "message": f"Preparing bootable Instant Server USB for '{profile_dict.get('hostname', 'instant-server')}'...",
                        "data": {"percent": 8},
                    }
                )
                kwargs = _filter_server_profile_kwargs(profile_dict)
                prof = m4_config_builder.ServerProfile(**kwargs)

                # Step 1: Resolve or Download Official Ubuntu Server ISO (unless disabled in unit test)
                should_pull_iso = auto_download_iso if auto_download_iso is not None else bool(target_device)
                resolved_iso = iso_path or prof.custom_iso_path
                if not resolved_iso or not os.path.isfile(str(resolved_iso)):
                    resolved_iso = m2_iso_manager.ensure_ubuntu_iso(
                        release=prof.ubuntu_release,
                        iso_cache_dir=paths["iso_dir"],
                        custom_iso_path=prof.custom_iso_path,
                        auto_download=should_pull_iso,
                        event_callback=self._record_event,
                    )

                # Step 2: Stage cidata + Pre-Unpack Selected Self-Hosted Apps
                summary = m5_payload_packager.stage_cidata_directory(
                    profile=prof,
                    staging_dir=resolved_out_dir,
                    iso_path_for_debs=resolved_iso,
                    event_callback=self._record_event,
                )

                # Step 3: Format USB (GPT/MBR) + Extract/Flash Bootable ISO + Write cidata & Apps
                usb_res = m3_usb_engine.provision_usb_drive(
                    staged_cidata_dir=resolved_out_dir,
                    target_device=target_device,
                    iso_path=resolved_iso,
                    usb_partition_scheme=prof.usb_partition_scheme,
                    format_usb=bool(profile_dict.get("format_usb", True)),
                    event_callback=self._record_event,
                )
                summary["usb_provision"] = usb_res
                with self._lock:
                    self.active_progress = None
                    self.last_result = summary
                    self.percent = 100
                    self.status = "COMPLETED"
            except InterruptedError as cancel_exc:
                self._record_event(
                    {
                        "event": "JOB_CANCELLED",
                        "module": "CLI_BRIDGE",
                        "message": f"Cancelled: {cancel_exc}",
                        "data": {},
                    }
                )
                with self._lock:
                    self.active_progress = None
                    self.error = str(cancel_exc)
                    self.status = "CANCELLED"
            except Exception as exc:
                self._record_event(
                    {
                        "event": "JOB_FAILED",
                        "module": "CLI_BRIDGE",
                        "message": f"Error: {exc}",
                        "data": {},
                    }
                )
                with self._lock:
                    self.active_progress = None
                    self.error = str(exc)
                    self.status = "FAILED"

        t = threading.Thread(target=worker, daemon=True)
        t.start()
        return {"started": True, "status": "RUNNING"}

    def get_snapshot(self) -> Dict[str, Any]:
        with self._lock:
            return {
                "status": self.status,
                "percent": self.percent,
                "event_count": len(self.events),
                "events": list(self.events[-120:]),
                "active_progress": dict(self.active_progress) if self.active_progress else None,
                "result": self.last_result,
                "error": self.error,
            }


class InstantServerJsApi:
    def __init__(self, base_dir: Optional[str] = None) -> None:
        self.base_dir = base_dir or get_launch_root_dir()
        ensure_workspace_folder(self.base_dir)
        self.jobs = AsyncPipelineManager(base_dir=self.base_dir)
        self.force_close_confirmed: bool = False
        self.last_heartbeat: float = time.time()
        self.client_connected: bool = False
        self._webview_window: Any = None

    def probe_host(self) -> Dict[str, Any]:
        return asdict(m1_host_probe.probe_host_environment())

    def scan_usb_devices(self) -> List[Dict[str, Any]]:
        return [asdict(d) for d in m3_usb_engine.scan_host_usb_devices()]

    def get_app_catalog(self) -> List[Dict[str, Any]]:
        return list(m4_config_builder.SELF_HOSTED_APP_CATALOG)

    def load_config(self) -> Dict[str, Any]:
        return load_saved_config(self.base_dir)

    def save_config(self, profile_dict: Dict[str, Any]) -> Dict[str, Any]:
        return save_local_config(profile_dict, self.base_dir)

    def compile_preview(self, profile_dict: Dict[str, Any]) -> Dict[str, Any]:
        kwargs = _filter_server_profile_kwargs(profile_dict)
        prof = m4_config_builder.ServerProfile(**kwargs)
        return m4_config_builder.compile_cidata_payloads(prof)

    def start_build(
        self,
        profile_dict: Dict[str, Any],
        output_dir: Optional[str] = None,
        target_device: Optional[str] = None,
        auto_download_iso: Optional[bool] = None,
    ) -> Dict[str, Any]:
        paths = ensure_workspace_folder(self.base_dir)
        out_dir = output_dir or paths["cidata_dir"]
        target = target_device or profile_dict.get("selected_usb_device")
        save_local_config(profile_dict, self.base_dir)
        return self.jobs.start_stage_job(
            profile_dict,
            out_dir,
            target_device=target,
            auto_download_iso=auto_download_iso,
        )

    def cancel_build(self) -> Dict[str, Any]:
        return self.jobs.cancel_job()

    def poll_build_status(self) -> Dict[str, Any]:
        self.last_heartbeat = time.time()
        self.client_connected = True
        return self.jobs.get_snapshot()

    def heartbeat(self) -> Dict[str, Any]:
        self.last_heartbeat = time.time()
        self.client_connected = True
        return {"ok": True, "status": self.jobs.status}

    def shutdown_app(self, force: bool = True) -> Dict[str, Any]:
        self.force_close_confirmed = True
        self.jobs.cancel_job()
        _remove_pid_file(self.base_dir)

        def _hard_exit() -> None:
            time.sleep(0.12)
            try:
                if self._webview_window is not None:
                    if hasattr(self._webview_window, 'close'):
                        self._webview_window.close()
                    elif hasattr(self._webview_window, 'destroy'):
                        self._webview_window.destroy()
            except Exception:
                pass
            os._exit(0)

        threading.Thread(target=_hard_exit, daemon=True).start()
        return {"shutting_down": True}

    def discover_servers(self, hostname: str = "instant-server") -> Dict[str, Any]:
        return discover_instant_servers_on_lan(hostname=hostname)


def _get_pid_file_path(base_dir: Optional[str] = None) -> str:
    paths = ensure_workspace_folder(base_dir)
    return os.path.join(paths["workspace_dir"], "instant-server.pid")


def _remove_pid_file(base_dir: Optional[str] = None) -> None:
    try:
        pid_path = _get_pid_file_path(base_dir)
        if os.path.isfile(pid_path):
            os.remove(pid_path)
    except Exception:
        pass


def _cleanup_previous_instance(port: int, base_dir: Optional[str] = None) -> None:
    """Ensure any previously running InstantServer.exe background process is terminated before starting."""
    try:
        import urllib.request
        req = urllib.request.Request(
            f"http://127.0.0.1:{port}/api/shutdown",
            data=b'{"force": true}',
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        with urllib.request.urlopen(req, timeout=0.6) as _:
            pass
        time.sleep(0.25)
    except Exception:
        pass

    pid_path = _get_pid_file_path(base_dir)
    if os.path.isfile(pid_path):
        try:
            with open(pid_path, "r", encoding="utf-8") as fh:
                old_pid = int(fh.read().strip())
            if old_pid > 0 and old_pid != os.getpid():
                if sys.platform.startswith("win"):
                    subprocess.run(
                        ["taskkill", "/F", "/T", "/PID", str(old_pid)],
                        stdout=subprocess.DEVNULL,
                        stderr=subprocess.DEVNULL,
                        check=False,
                        **_win_no_window_kwargs(),
                    )
                else:
                    import signal
                    os.kill(old_pid, signal.SIGTERM)
                time.sleep(0.2)
        except Exception:
            pass

    try:
        with open(pid_path, "w", encoding="utf-8") as fh:
            fh.write(str(os.getpid()))
    except Exception:
        pass


def _set_windows_taskbar_app_id() -> None:
    if sys.platform.startswith("win"):
        try:
            import ctypes
            ctypes.windll.shell32.SetCurrentProcessExplicitAppUserModelID("io.instantserver.desktop.app")
        except Exception:
            pass


# ---------------------------------------------------------------------------
# 2B. Pure Native PyQt6 (QtWidgets) Windows 11 Fluent Desktop GUI
#     - Zero Browser / Zero Microsoft Edge / Zero QtWebEngine (Saves ~165 MB!)
#     - Opens Instantly (<0.3s), Single Process, Clean Exit via os._exit(0)
# ---------------------------------------------------------------------------
def build_fluent_stylesheet(theme: str = "dark") -> str:
    if theme == "light":
        bg_main = "#f0f3f9"
        bg_sidebar = "#e6ebf4"
        bg_card = "#ffffff"
        bg_card_sec = "#f6f8fc"
        bg_input = "#ffffff"
        bg_popup = "#ffffff"
        border_card = "#dfe5ef"
        border_ctrl = "#c4cddb"
        text_pri = "#1b1f26"
        text_sec = "#586274"
        accent = "#0067c0"
        accent_hover = "#005ba8"
        accent_text = "#ffffff"
        danger = "#c42b1c"
        danger_bg = "#fde7e9"
        scroll_idle = "rgba(90, 102, 122, 0.25)"
        scroll_hover = "rgba(90, 102, 122, 0.55)"
    else:
        bg_main = "#191d24"
        bg_sidebar = "#14181f"
        bg_card = "#232832"
        bg_card_sec = "#1d222b"
        bg_input = "#2a303c"
        bg_popup = "#282e3a"
        border_card = "#303746"
        border_ctrl = "#434c5e"
        text_pri = "#f3f6fa"
        text_sec = "#9ba6b8"
        accent = "#60cdff"
        accent_hover = "#7ad6ff"
        accent_text = "#09131c"
        danger = "#ff99a4"
        danger_bg = "#442226"
        scroll_idle = "rgba(155, 166, 184, 0.22)"
        scroll_hover = "rgba(155, 166, 184, 0.55)"

    return f"""
    QMainWindow, QWidget#RootCentralWidget {{
        background-color: {bg_main};
        color: {text_pri};
        font-family: "Segoe UI Variable Text", "Segoe UI", -apple-system, sans-serif;
        font-size: 13px;
    }}
    QLabel {{
        color: {text_pri};
        background: transparent;
    }}
    QLabel[role="secondary"] {{
        color: {text_sec};
        font-size: 12px;
    }}
    QLabel[role="title"] {{
        font-size: 22px;
        font-weight: 600;
    }}
    QLabel[role="section"] {{
        font-size: 13px;
        font-weight: 600;
        padding-top: 4px;
    }}
    QFrame[card="true"] {{
        background-color: {bg_card};
        border: 1px solid {border_card};
        border-radius: 8px;
    }}
    QFrame[card="secondary"] {{
        background-color: {bg_card_sec};
        border: 1px solid {border_card};
        border-radius: 8px;
    }}
    QFrame[card="selected"] {{
        background-color: {bg_card};
        border: 1.5px solid {accent};
        border-radius: 8px;
    }}
    QFrame#SidebarFrame {{
        background-color: {bg_sidebar};
        border-right: 1px solid {border_card};
    }}
    QFrame#TitleBarFrame {{
        background-color: {bg_sidebar};
        border-bottom: 1px solid {border_card};
    }}
    QPushButton {{
        background-color: {bg_input};
        color: {text_pri};
        border: 1px solid {border_ctrl};
        border-radius: 6px;
        padding: 6px 14px;
        font-weight: 500;
    }}
    QPushButton:hover {{
        border-color: {accent};
    }}
    QPushButton:disabled {{
        opacity: 0.45;
        color: {text_sec};
    }}
    QPushButton[accent="true"] {{
        background-color: {accent};
        color: {accent_text};
        border: 1px solid {accent};
        font-weight: 600;
    }}
    QPushButton[accent="true"]:hover {{
        background-color: {accent_hover};
    }}
    QPushButton[danger="true"] {{
        background-color: {danger_bg};
        color: {danger};
        border: 1px solid {danger};
        font-weight: 600;
    }}
    QPushButton[nav="true"] {{
        background-color: transparent;
        border: none;
        border-radius: 6px;
        text-align: left;
        padding: 9px 12px;
        color: {text_pri};
        font-size: 13px;
    }}
    QPushButton[nav="true"]:hover {{
        background-color: {bg_card};
    }}
    QPushButton[nav="active"] {{
        background-color: {bg_card};
        border-left: 3px solid {accent};
        border-radius: 6px;
        text-align: left;
        padding: 9px 12px;
        color: {text_pri};
        font-weight: 600;
    }}
    QLineEdit {{
        background-color: {bg_input};
        color: {text_pri};
        border: 1px solid {border_ctrl};
        border-radius: 6px;
        padding: 6px 10px;
        selection-background-color: {accent};
        selection-color: {accent_text};
    }}
    QLineEdit:focus {{
        border: 1.5px solid {accent};
    }}
    /* Windows 11 Fluent Dark/Light ComboBox Dropdown */
    QComboBox {{
        background-color: {bg_input};
        color: {text_pri};
        border: 1px solid {border_ctrl};
        border-radius: 6px;
        padding: 6px 12px;
        min-width: 210px;
    }}
    QComboBox:hover {{
        border-color: {accent};
    }}
    QComboBox::drop-down {{
        border: none;
        width: 24px;
    }}
    QComboBox QAbstractItemView {{
        background-color: {bg_popup};
        color: {text_pri};
        border: 1px solid {border_ctrl};
        border-radius: 6px;
        padding: 4px;
        selection-background-color: {accent};
        selection-color: {accent_text};
        outline: 0px;
    }}
    QCheckBox {{
        color: {text_pri};
        spacing: 8px;
    }}
    QProgressBar {{
        background-color: {bg_card_sec};
        border: 1px solid {border_card};
        border-radius: 5px;
        height: 10px;
        text-align: center;
        color: transparent;
    }}
    QProgressBar::chunk {{
        background-color: {accent};
        border-radius: 4px;
    }}
    QPlainTextEdit {{
        background-color: #10141b;
        color: #d8e1ee;
        border: 1px solid {border_card};
        border-radius: 8px;
        padding: 8px;
        font-family: "Cascadia Mono", "Consolas", monospace;
        font-size: 12px;
    }}
    /* Sleek Blended ScrollBar */
    QScrollArea {{
        border: none;
        background: transparent;
    }}
    QScrollBar:vertical {{
        background: transparent;
        width: 8px;
        margin: 2px 1px 2px 1px;
    }}
    QScrollBar::handle:vertical {{
        background: {scroll_idle};
        min-height: 28px;
        border-radius: 4px;
    }}
    QScrollBar::handle:vertical:hover, QScrollBar::handle:vertical:pressed {{
        background: {scroll_hover};
    }}
    QScrollBar::add-line:vertical, QScrollBar::sub-line:vertical,
    QScrollBar::add-page:vertical, QScrollBar::sub-page:vertical {{
        background: none;
        height: 0px;
    }}
    """


def launch_desktop_or_browser_gui(port: int = 8765, open_browser: bool = True) -> None:
    """
    Launch the standalone native PyQt6 (`QtWidgets`) desktop application.
      - Zero Microsoft Edge (`msedge.exe`), zero `QtWebEngineProcess`, zero browser popups.
      - Single lightweight process (~22 MB packaged binary).
      - Intercepts window close (`X`) during active USB flashing with a confirmation dialog.
      - Terminates 100% cleanly via `os._exit(0)` on window close.
    """
    _set_windows_taskbar_app_id()
    paths = ensure_workspace_folder()
    api = InstantServerJsApi(base_dir=paths["root_dir"])
    icon_for_os = paths["icon_ico_path"] if sys.platform.startswith("win") else paths["icon_png_path"]

    try:
        from PyQt6.QtCore import Qt, QTimer  # type: ignore
        from PyQt6.QtGui import QCloseEvent, QIcon  # type: ignore
        from PyQt6.QtWidgets import (  # type: ignore
            QApplication,
            QCheckBox,
            QComboBox,
            QFileDialog,
            QFrame,
            QHBoxLayout,
            QLabel,
            QLineEdit,
            QMainWindow,
            QMessageBox,
            QPlainTextEdit,
            QProgressBar,
            QPushButton,
            QScrollArea,
            QSlider,
            QStackedWidget,
            QVBoxLayout,
            QWidget,
        )
    except ImportError as exc:
        msg = (
            "Instant Server requires PyQt6 for its native desktop window.\n\n"
            "Install it with:  pip install PyQt6\n"
            f"Details: {exc}"
        )
        print(msg, file=sys.stderr)
        if sys.platform.startswith("win"):
            try:
                import ctypes
                ctypes.windll.user32.MessageBoxW(0, msg, "Instant Server — Missing PyQt6", 0x10)
            except Exception:
                pass
        os._exit(1)

    qt_app = QApplication.instance() or QApplication(sys.argv)
    qt_app.setApplicationName("Instant Server")
    if os.path.isfile(icon_for_os):
        qt_app.setWindowIcon(QIcon(icon_for_os))

    class CollapsibleExpander(QFrame):
        """Windows 11 Settings style expandable 'Advanced options' card that never overlaps neighbors."""

        def __init__(self, title: str = "Advanced options", parent: Optional[QWidget] = None) -> None:
            super().__init__(parent)
            self.setProperty("card", "true")
            self._expanded = False
            outer = QVBoxLayout(self)
            outer.setContentsMargins(14, 10, 14, 10)
            outer.setSpacing(8)

            self.toggle_btn = QPushButton(f"▸  {title}")
            self.toggle_btn.setStyleSheet("border: none; text-align: left; font-weight: 600; padding: 4px 2px; background: transparent;")
            self.toggle_btn.setCursor(Qt.CursorShape.PointingHandCursor)
            outer.addWidget(self.toggle_btn)

            self.body_widget = QWidget()
            self.body_layout = QVBoxLayout(self.body_widget)
            self.body_layout.setContentsMargins(0, 6, 0, 2)
            self.body_layout.setSpacing(8)
            self.body_widget.setVisible(False)
            outer.addWidget(self.body_widget)

            self._title = title
            self.toggle_btn.clicked.connect(self._on_toggle)

        def _on_toggle(self) -> None:
            self._expanded = not self._expanded
            self.body_widget.setVisible(self._expanded)
            arrow = "▾" if self._expanded else "▸"
            self.toggle_btn.setText(f"{arrow}  {self._title}")

    class InstantServerNativeWindow(QMainWindow):
        def __init__(self) -> None:
            super().__init__()
            self.setWindowTitle("Instant Server")
            self.resize(1180, 800)
            self.setMinimumSize(900, 620)
            if os.path.isfile(icon_for_os):
                self.setWindowIcon(QIcon(icon_for_os))

            # State dictionary synced with ServerProfile & saved config
            self.theme = "dark"
            self.view_mode = "setup"
            self.current_tab_index = 0
            self.app_category_filter = "All"
            self.app_search_query = ""
            self.rendered_log_count = 0
            self.log_auto_scroll = True

            self.cfg: Dict[str, Any] = {
                "hostname": "instant-server",
                "username": "admin",
                "password": "",
                "wifi_ssid": "",
                "wifi_password": "",
                "enable_usb_tethering": True,
                "static_ip_cidr": "",
                "static_gateway": "",
                "static_dns": "",
                "ssh_authorized_key": "",
                "dashboard_choice": "both",
                "selected_apps": ["jellyfin", "nextcloud", "pihole", "kiwix", "immich", "homeassistant", "vaultwarden"],
                "prebundle_offline_images": False,
                "enable_pihole_dns_prep": True,
                "enable_samba_nas": True,
                "enable_intel_vaapi": True,
                "enable_tailscale": False,
                "tailscale_authkey": "",
                "laptop_age_hint": "unknown",
                "laptop_ram_hint": "unknown",
                "usb_partition_scheme": "auto",
                "iso_source_mode": "official_dl",
                "custom_iso_path": "",
                "confirm_usb_erase": True,
                "install_mode": "dual_boot",
                "dual_boot_Allocation_gb": 45,
                "confirm_erase_main_drive": False,
                "ubuntu_release": "26.04",
                "storage_layout": "direct",
                "ignore_lid_switch": True,
                "screen_blank_seconds": "60",
                "battery_charge_start_pct": 40,
                "battery_charge_stop_pct": 60,
                "ups_shutdown_battery_pct": 15,
                "enable_low_ram_zram": True,
                "safe_gpu_nomodeset": False,
                "selected_usb_device": "",
            }

            boot_cfg = api.load_config()
            if boot_cfg.get("exists") and isinstance(boot_cfg.get("config"), dict):
                for k, v in boot_cfg["config"].items():
                    if k in self.cfg:
                        self.cfg[k] = v
                self.view_mode = "settings"

            self.usb_devices: List[Dict[str, Any]] = []
            self.app_checkboxes: Dict[str, QCheckBox] = {}
            self.app_card_frames: Dict[str, QFrame] = {}

            self._build_ui()
            self._apply_theme()
            self._refresh_usb_list()

            # Poll timer for flashing progress & terminal updates (350ms)
            self.poll_timer = QTimer(self)
            self.poll_timer.setInterval(350)
            self.poll_timer.timeout.connect(self._on_poll_flash_status)

        def _apply_theme(self) -> None:
            qt_app.setStyleSheet(build_fluent_stylesheet(self.theme))
            self.btn_theme.setText("☀ Light" if self.theme == "dark" else "🌙 Dark")

        def _toggle_theme(self) -> None:
            self.theme = "light" if self.theme == "dark" else "dark"
            self._apply_theme()

        def _toggle_view_mode(self) -> None:
            self.view_mode = "settings" if self.view_mode == "setup" else "setup"
            self.sidebar_frame.setVisible(self.view_mode == "settings")
            self.setup_tabs_bar.setVisible(self.view_mode == "setup")
            self.btn_mode_switch.setText("Setup Wizard" if self.view_mode == "settings" else "Home Settings")

        def _make_card_row(self, icon_txt: str, title: str, desc: str, right_widget: Optional[QWidget] = None) -> QFrame:
            card = QFrame()
            card.setProperty("card", "true")
            row = QHBoxLayout(card)
            row.setContentsMargins(16, 12, 16, 12)
            row.setSpacing(14)

            ico = QLabel(icon_txt)
            ico.setStyleSheet("font-size: 18px;")
            ico.setFixedWidth(26)
            row.addWidget(ico)

            txt_col = QVBoxLayout()
            txt_col.setSpacing(2)
            lbl_t = QLabel(title)
            lbl_t.setStyleSheet("font-weight: 600; font-size: 13.5px;")
            lbl_d = QLabel(desc)
            lbl_d.setProperty("role", "secondary")
            lbl_d.setWordWrap(True)
            txt_col.addWidget(lbl_t)
            txt_col.addWidget(lbl_d)
            row.addLayout(txt_col, 1)

            if right_widget is not None:
                row.addWidget(right_widget)
            return card

        def _build_ui(self) -> None:
            root = QWidget()
            root.setObjectName("RootCentralWidget")
            self.setCentralWidget(root)
            root_layout = QVBoxLayout(root)
            root_layout.setContentsMargins(0, 0, 0, 0)
            root_layout.setSpacing(0)

            # 1. Unified Top Titlebar (Back + Title + Search + Mode Switch + Theme Toggle)
            titlebar = QFrame()
            titlebar.setObjectName("TitleBarFrame")
            tb_layout = QHBoxLayout(titlebar)
            tb_layout.setContentsMargins(16, 10, 16, 10)
            tb_layout.setSpacing(12)

            self.btn_back = QPushButton("←")
            self.btn_back.setFixedWidth(36)
            self.btn_back.clicked.connect(self._on_prev_tab)
            tb_layout.addWidget(self.btn_back)

            lbl_brand = QLabel("Instant Server")
            lbl_brand.setStyleSheet("font-weight: 600; font-size: 14px;")
            tb_layout.addWidget(lbl_brand)
            tb_layout.addStretch(1)

            self.search_box = QLineEdit()
            self.search_box.setPlaceholderText("Find a setting")
            self.search_box.setFixedWidth(280)
            self.search_box.textChanged.connect(self._on_global_search)
            tb_layout.addWidget(self.search_box)
            tb_layout.addStretch(1)

            self.btn_mode_switch = QPushButton("Setup Wizard" if self.view_mode == "settings" else "Home Settings")
            self.btn_mode_switch.clicked.connect(self._toggle_view_mode)
            tb_layout.addWidget(self.btn_mode_switch)

            self.btn_theme = QPushButton("☀ Light")
            self.btn_theme.clicked.connect(self._toggle_theme)
            tb_layout.addWidget(self.btn_theme)

            root_layout.addWidget(titlebar)

            # 2. Setup Top Step Bar (Visible in Setup Wizard mode)
            self.setup_tabs_bar = QFrame()
            st_layout = QHBoxLayout(self.setup_tabs_bar)
            st_layout.setContentsMargins(20, 10, 20, 6)
            st_layout.setSpacing(8)
            self.setup_tab_btns: List[QPushButton] = []
            self.tab_titles = [
                ("🌐  Network", "Server Name, Admin Login & Wi-Fi"),
                ("🧩  Apps & Store", "Cockpit, CasaOS & 26 Self-Hosted Apps"),
                ("🖴  Laptop & USB", "Target Laptop Hardware, Ubuntu 26.04.1 ISO & USB Flash"),
                ("🔋  Power & Battery", "24/7 Lid-Close Operation & Battery Health Limiter"),
                ("🚀  Finish & Flash", "Review Summary, Flash Bootable USB & Live Terminal"),
            ]
            for idx, (short_lbl, _) in enumerate(self.tab_titles):
                b = QPushButton(short_lbl)
                b.clicked.connect(lambda _checked=False, i=idx: self._switch_tab(i))
                self.setup_tab_btns.append(b)
                st_layout.addWidget(b)
            st_layout.addStretch(1)
            root_layout.addWidget(self.setup_tabs_bar)

            # 3. Main Body Split: Left Sidebar (Settings mode) + Right Scrollable Content Stack
            body_split = QHBoxLayout()
            body_split.setContentsMargins(0, 0, 0, 0)
            body_split.setSpacing(0)

            self.sidebar_frame = QFrame()
            self.sidebar_frame.setObjectName("SidebarFrame")
            self.sidebar_frame.setFixedWidth(235)
            sb_layout = QVBoxLayout(self.sidebar_frame)
            sb_layout.setContentsMargins(10, 14, 10, 14)
            sb_layout.setSpacing(4)
            self.sidebar_btns: List[QPushButton] = []
            for idx, (short_lbl, _) in enumerate(self.tab_titles):
                sb_btn = QPushButton(short_lbl)
                sb_btn.setProperty("nav", "true")
                sb_btn.clicked.connect(lambda _checked=False, i=idx: self._switch_tab(i))
                self.sidebar_btns.append(sb_btn)
                sb_layout.addWidget(sb_btn)
            sb_layout.addStretch(1)
            body_split.addWidget(self.sidebar_frame)

            # Right Scroll Area
            self.scroll_area = QScrollArea()
            self.scroll_area.setWidgetResizable(True)
            scroll_content = QWidget()
            self.scroll_vbox = QVBoxLayout(scroll_content)
            self.scroll_vbox.setContentsMargins(28, 20, 28, 20)
            self.scroll_vbox.setSpacing(14)

            self.lbl_page_header = QLabel()
            self.lbl_page_header.setProperty("role", "title")
            self.scroll_vbox.addWidget(self.lbl_page_header)

            self.pages_stack = QStackedWidget()
            self.pages_stack.addWidget(self._build_network_page())
            self.pages_stack.addWidget(self._build_apps_page())
            self.pages_stack.addWidget(self._build_storage_page())
            self.pages_stack.addWidget(self._build_power_page())
            self.pages_stack.addWidget(self._build_finish_page())
            self.scroll_vbox.addWidget(self.pages_stack)
            self.scroll_vbox.addStretch(1)

            self.scroll_area.setWidget(scroll_content)
            body_split.addWidget(self.scroll_area, 1)
            root_layout.addLayout(body_split, 1)

            # 4. Bottom Navigation & Flash Action Bar
            footer = QFrame()
            footer.setObjectName("TitleBarFrame")
            f_layout = QHBoxLayout(footer)
            f_layout.setContentsMargins(20, 10, 20, 10)
            f_layout.setSpacing(10)

            self.btn_prev = QPushButton("← Previous")
            self.btn_prev.clicked.connect(self._on_prev_tab)
            f_layout.addWidget(self.btn_prev)

            f_layout.addStretch(1)

            self.btn_save_cfg = QPushButton("Save configuration")
            self.btn_save_cfg.clicked.connect(self._on_save_config)
            f_layout.addWidget(self.btn_save_cfg)

            self.btn_cancel_flash_footer = QPushButton("✕ Cancel flashing")
            self.btn_cancel_flash_footer.setProperty("danger", "true")
            self.btn_cancel_flash_footer.setVisible(False)
            self.btn_cancel_flash_footer.clicked.connect(self._on_cancel_flash)
            f_layout.addWidget(self.btn_cancel_flash_footer)

            self.btn_next_or_flash = QPushButton("Proceed →")
            self.btn_next_or_flash.setProperty("accent", "true")
            self.btn_next_or_flash.clicked.connect(self._on_next_or_flash)
            f_layout.addWidget(self.btn_next_or_flash)

            root_layout.addWidget(footer)

            self.sidebar_frame.setVisible(self.view_mode == "settings")
            self.setup_tabs_bar.setVisible(self.view_mode == "setup")
            self._switch_tab(0)

        def _build_network_page(self) -> QWidget:
            page = QWidget()
            vbox = QVBoxLayout(page)
            vbox.setContentsMargins(0, 0, 0, 0)
            vbox.setSpacing(10)

            self.in_hostname = QLineEdit(str(self.cfg["hostname"]))
            self.in_hostname.textChanged.connect(lambda v: self.cfg.__setitem__("hostname", v.strip() or "instant-server"))
            vbox.addWidget(self._make_card_row("🏷", "Server name (Hostname)", "Reach your server on your home Wi-Fi at <name>.local", self.in_hostname))

            self.in_username = QLineEdit(str(self.cfg["username"]))
            self.in_username.textChanged.connect(lambda v: self.cfg.__setitem__("username", v.strip() or "admin"))
            vbox.addWidget(self._make_card_row("👤", "Admin username", "Administrator account for SSH, Cockpit, and CasaOS", self.in_username))

            self.in_password = QLineEdit(str(self.cfg["password"]))
            self.in_password.setEchoMode(QLineEdit.EchoMode.Password)
            self.in_password.setPlaceholderText("Enter admin password (required)")
            self.in_password.textChanged.connect(lambda v: self.cfg.__setitem__("password", v))
            vbox.addWidget(self._make_card_row("🔑", "Admin password", "Encrypted with SHA-512 crypt ($6$) before writing to USB", self.in_password))

            self.in_wifi_ssid = QLineEdit(str(self.cfg["wifi_ssid"]))
            self.in_wifi_ssid.setPlaceholderText("Home Wi-Fi name (leave blank for Ethernet)")
            self.in_wifi_ssid.textChanged.connect(lambda v: self.cfg.__setitem__("wifi_ssid", v))
            vbox.addWidget(self._make_card_row("📶", "Wi-Fi network name (SSID)", "Bundles offline wpasupplicant .debs from the Ubuntu ISO so Wi-Fi works out of the box", self.in_wifi_ssid))

            self.in_wifi_pw = QLineEdit(str(self.cfg["wifi_password"]))
            self.in_wifi_pw.setEchoMode(QLineEdit.EchoMode.Password)
            self.in_wifi_pw.setPlaceholderText("Wi-Fi password")
            self.in_wifi_pw.textChanged.connect(lambda v: self.cfg.__setitem__("wifi_password", v))
            vbox.addWidget(self._make_card_row("🔒", "Wi-Fi password", "WPA2/WPA3 wireless password", self.in_wifi_pw))

            adv = CollapsibleExpander("Advanced options")
            self.chk_tethering = QCheckBox("Enable Android / iPhone USB tethering fallback")
            self.chk_tethering.setChecked(bool(self.cfg["enable_usb_tethering"]))
            self.chk_tethering.toggled.connect(lambda c: self.cfg.__setitem__("enable_usb_tethering", c))
            adv.body_layout.addWidget(self.chk_tethering)

            self.in_static_ip = QLineEdit(str(self.cfg["static_ip_cidr"]))
            self.in_static_ip.setPlaceholderText("Optional Static IP CIDR (e.g. 192.168.1.50/24)")
            self.in_static_ip.textChanged.connect(lambda v: self.cfg.__setitem__("static_ip_cidr", v))
            adv.body_layout.addWidget(self.in_static_ip)

            self.in_ssh_key = QLineEdit(str(self.cfg["ssh_authorized_key"]))
            self.in_ssh_key.setPlaceholderText("Optional SSH Public Key (ssh-ed25519 / ssh-rsa)")
            self.in_ssh_key.textChanged.connect(lambda v: self.cfg.__setitem__("ssh_authorized_key", v))
            adv.body_layout.addWidget(self.in_ssh_key)

            vbox.addWidget(adv)
            return page

        def _build_apps_page(self) -> QWidget:
            page = QWidget()
            vbox = QVBoxLayout(page)
            vbox.setContentsMargins(0, 0, 0, 0)
            vbox.setSpacing(10)

            cb_dash = QComboBox()
            dash_items = [
                ("both", "Cockpit + CasaOS (Recommended — System Console + 1-Click App Store)"),
                ("cockpit", "Cockpit Only (Lightweight System Manager)"),
                ("casaos", "CasaOS Only (Docker Home Cloud & App Store)"),
                ("none", "SSH Only (Minimal Headless Terminal)"),
            ]
            for val, lbl in dash_items:
                cb_dash.addItem(lbl, val)
            idx_d = max(0, next((i for i, (v, _) in enumerate(dash_items) if v == self.cfg["dashboard_choice"]), 0))
            cb_dash.setCurrentIndex(idx_d)
            cb_dash.currentIndexChanged.connect(lambda i: self.cfg.__setitem__("dashboard_choice", cb_dash.itemData(i)))
            vbox.addWidget(self._make_card_row("✨", "Primary web dashboard", "CasaOS lets you install even more apps anytime with 1 click", cb_dash))

            # App Store Header & Preset Buttons
            preset_row = QHBoxLayout()
            self.lbl_app_count = QLabel(f"Pre-select self-hosted apps to bundle onto USB ({len(self.cfg['selected_apps'])} selected)")
            self.lbl_app_count.setProperty("role", "section")
            preset_row.addWidget(self.lbl_app_count)
            preset_row.addStretch(1)

            btn_rec = QPushButton("Recommended")
            btn_rec.clicked.connect(lambda: self._apply_app_preset([a["id"] for a in m4_config_builder.SELF_HOSTED_APP_CATALOG if a.get("recommended")]))
            preset_row.addWidget(btn_rec)

            btn_old = QPushButton("Old Laptop Pack (Low RAM)")
            btn_old.clicked.connect(lambda: self._apply_app_preset(["jellyfin", "pihole", "kiwix", "vaultwarden", "filebrowser", "syncthing", "navidrome"]))
            preset_row.addWidget(btn_old)

            btn_high = QPushButton("Higher-End + AI & Minecraft")
            btn_high.clicked.connect(lambda: self._apply_app_preset(["jellyfin", "nextcloud", "pihole", "kiwix", "immich", "homeassistant", "vaultwarden", "minecraft", "ollama_webui"]))
            preset_row.addWidget(btn_high)

            btn_all = QPushButton("Select all")
            btn_all.clicked.connect(lambda: self._apply_app_preset([a["id"] for a in m4_config_builder.SELF_HOSTED_APP_CATALOG]))
            preset_row.addWidget(btn_all)

            btn_none = QPushButton("Unselect all")
            btn_none.clicked.connect(lambda: self._apply_app_preset([]))
            preset_row.addWidget(btn_none)
            vbox.addLayout(preset_row)

            # Search & Category Filter Row
            filter_row = QHBoxLayout()
            self.in_app_search = QLineEdit()
            self.in_app_search.setPlaceholderText("Search 26 self-hosted apps (Wikipedia, Minecraft, Ollama AI, Jellyfin, Nextcloud, Pi-hole)...")
            self.in_app_search.textChanged.connect(self._filter_app_cards)
            filter_row.addWidget(self.in_app_search, 1)

            self.cb_app_cat = QComboBox()
            cats = [
                "All", "Recommended", "Old Laptop Friendly", "Higher-End / Gaming & AI",
                "Knowledge & Wiki", "Gaming Servers", "AI & Machine Learning",
                "Media & Photos", "Cloud & Files", "Network & Ad-Block", "Smart Home", "Security & Tools"
            ]
            for c in cats:
                self.cb_app_cat.addItem(c, c)
            self.cb_app_cat.currentIndexChanged.connect(self._filter_app_cards)
            filter_row.addWidget(self.cb_app_cat)
            vbox.addLayout(filter_row)

            # App Cards Container
            self.apps_list_vbox = QVBoxLayout()
            self.apps_list_vbox.setSpacing(6)
            for app in m4_config_builder.SELF_HOSTED_APP_CATALOG:
                app_id = app["id"]
                card = QFrame()
                card.setProperty("card", "true")
                row = QHBoxLayout(card)
                row.setContentsMargins(14, 10, 14, 10)
                row.setSpacing(12)

                chk = QCheckBox()
                chk.setChecked(app_id in self.cfg["selected_apps"])
                chk.toggled.connect(lambda checked, aid=app_id: self._on_toggle_single_app(aid, checked))
                self.app_checkboxes[app_id] = chk
                row.addWidget(chk)

                col = QVBoxLayout()
                col.setSpacing(2)
                tier_tag = ""
                if app.get("hardware_tier") == "high_end":
                    tier_tag = f"  •  ⚠️ {app.get('hardware_note', 'Higher-End Only • Not for old laptops')}"
                elif app.get("recommended"):
                    tier_tag = "  •  ★ Recommended"
                title_lbl = QLabel(f"{app['title']}   [{app['category']} • RAM: {app.get('min_ram_mb', 128)} MB]{tier_tag}")
                title_lbl.setStyleSheet("font-weight: 600; font-size: 13px;")
                desc_lbl = QLabel(app["description"])
                desc_lbl.setProperty("role", "secondary")
                desc_lbl.setWordWrap(True)
                col.addWidget(title_lbl)
                col.addWidget(desc_lbl)
                row.addLayout(col, 1)

                self.app_card_frames[app_id] = card
                self.apps_list_vbox.addWidget(card)

            vbox.addLayout(self.apps_list_vbox)

            adv = CollapsibleExpander("Advanced options")
            chk_offline = QCheckBox("Pre-download container images on this PC (Offline USB bundle)")
            chk_offline.setChecked(bool(self.cfg["prebundle_offline_images"]))
            chk_offline.toggled.connect(lambda c: self.cfg.__setitem__("prebundle_offline_images", c))
            adv.body_layout.addWidget(chk_offline)

            chk_samba = QCheckBox("Enable Samba LAN File Share (Windows / macOS / Linux network drive)")
            chk_samba.setChecked(bool(self.cfg["enable_samba_nas"]))
            chk_samba.toggled.connect(lambda c: self.cfg.__setitem__("enable_samba_nas", c))
            adv.body_layout.addWidget(chk_samba)

            chk_vaapi = QCheckBox("Enable Intel Hardware Video Transcoding (VA-API for Jellyfin & Plex)")
            chk_vaapi.setChecked(bool(self.cfg["enable_intel_vaapi"]))
            chk_vaapi.toggled.connect(lambda c: self.cfg.__setitem__("enable_intel_vaapi", c))
            adv.body_layout.addWidget(chk_vaapi)

            vbox.addWidget(adv)
            return page

        def _on_toggle_single_app(self, app_id: str, checked: bool) -> None:
            sel = list(self.cfg["selected_apps"])
            if checked and app_id not in sel:
                sel.append(app_id)
            elif not checked and app_id in sel:
                sel.remove(app_id)
            self.cfg["selected_apps"] = sel
            self.lbl_app_count.setText(f"Pre-select self-hosted apps to bundle onto USB ({len(sel)} selected)")

        def _apply_app_preset(self, app_ids: List[str]) -> None:
            self.cfg["selected_apps"] = list(app_ids)
            for aid, chk in self.app_checkboxes.items():
                chk.blockSignals(True)
                chk.setChecked(aid in app_ids)
                chk.blockSignals(False)
            self.lbl_app_count.setText(f"Pre-select self-hosted apps to bundle onto USB ({len(app_ids)} selected)")

        def _filter_app_cards(self) -> None:
            q = self.in_app_search.text().strip().lower()
            cat = self.cb_app_cat.currentData() or "All"
            for app in m4_config_builder.SELF_HOSTED_APP_CATALOG:
                aid = app["id"]
                vis = True
                if cat == "Recommended" and not app.get("recommended"):
                    vis = False
                elif cat == "Old Laptop Friendly" and app.get("hardware_tier") != "old_friendly":
                    vis = False
                elif cat == "Higher-End / Gaming & AI" and app.get("hardware_tier") != "high_end":
                    vis = False
                elif cat not in ("All", "Recommended", "Old Laptop Friendly", "Higher-End / Gaming & AI") and app.get("category") != cat:
                    vis = False
                if vis and q:
                    hay = f"{app['title']} {app['description']} {app['category']} {app.get('hardware_note', '')}".lower()
                    if q not in hay:
                        vis = False
                self.app_card_frames[aid].setVisible(vis)

        def _build_storage_page(self) -> QWidget:
            page = QWidget()
            vbox = QVBoxLayout(page)
            vbox.setContentsMargins(0, 0, 0, 0)
            vbox.setSpacing(10)

            # 1. Laptop Age & UEFI/BIOS Questionnaire
            cb_age = QComboBox()
            age_opts = [
                ("unknown", "I don't know (Universal Hybrid UEFI + Legacy BIOS Boot)"),
                ("modern_uefi", "2013 or newer (UEFI + GPT)"),
                ("legacy_bios", "2012 or older (Legacy BIOS + MBR)"),
            ]
            for val, lbl in age_opts:
                cb_age.addItem(lbl, val)
            cb_age.currentIndexChanged.connect(lambda i: self._on_age_changed(cb_age.itemData(i)))
            vbox.addWidget(self._make_card_row("🕰", "How old is the target laptop?", "Automatically configures GPT vs MBR boot records (leave on 'I don't know' for universal boot)", cb_age))

            cb_ram = QComboBox()
            ram_opts = [
                ("unknown", "I don't know (Auto-tune memory)"),
                ("low_2_4gb", "2 GB – 4 GB RAM (Old Laptop — Low-RAM zram boost)"),
                ("standard_8gb_plus", "8 GB RAM (Standard Homelab & Minecraft)"),
                ("high_end_16gb_gpu", "16 GB+ RAM / GPU (Higher-End — Local AI & Gaming)"),
            ]
            for val, lbl in ram_opts:
                cb_ram.addItem(lbl, val)
            cb_ram.currentIndexChanged.connect(lambda i: self.cfg.__setitem__("laptop_ram_hint", cb_ram.itemData(i)))
            vbox.addWidget(self._make_card_row("🧠", "How much RAM does the laptop have?", "Tunes memory compression (zram) so even 2 GB – 4 GB laptops run containers smoothly", cb_ram))

            # 2. Target Laptop Installation Mode
            cb_mode = QComboBox()
            mode_opts = [
                ("dual_boot", "Dual-Boot alongside existing OS (Safe Default — keeps Windows/Linux intact)"),
                ("auto_erase", "Automatically erase main drive (Dedicates 100% of laptop disk to server)"),
                ("interactive_select", "Select after plugging in (Choose partition on laptop screen)"),
                ("secondary_drive", "Secondary drive only (Leave primary drive untouched)"),
            ]
            for val, lbl in mode_opts:
                cb_mode.addItem(lbl, val)
            cb_mode.currentIndexChanged.connect(lambda i: self._on_install_mode_changed(cb_mode.itemData(i)))
            vbox.addWidget(self._make_card_row("⚖", "Target laptop hard drive installation mode", "Choose how Ubuntu Server should be installed on the target laptop", cb_mode))

            self.chk_confirm_erase_laptop = QCheckBox("I know what I'm doing — erase all files on the target laptop's main internal drive")
            self.chk_confirm_erase_laptop.setVisible(self.cfg["install_mode"] == "auto_erase")
            self.chk_confirm_erase_laptop.toggled.connect(lambda c: self.cfg.__setitem__("confirm_erase_main_drive", c))
            vbox.addWidget(self.chk_confirm_erase_laptop)

            # 3. USB Drive Selector
            usb_hdr = QHBoxLayout()
            lbl_u = QLabel("Select removable USB drive to format & make bootable")
            lbl_u.setProperty("role", "section")
            usb_hdr.addWidget(lbl_u)
            usb_hdr.addStretch(1)
            btn_refresh_usb = QPushButton("↻ Refresh USB drives")
            btn_refresh_usb.clicked.connect(self._refresh_usb_list)
            usb_hdr.addWidget(btn_refresh_usb)
            vbox.addLayout(usb_hdr)

            self.cb_usb_drive = QComboBox()
            self.cb_usb_drive.currentIndexChanged.connect(self._on_usb_combo_changed)
            vbox.addWidget(self._make_card_row("🔌", "Target USB Flash Drive", "Protected by Triple-Lock safety guard (internal OS drives are hard-blocked)", self.cb_usb_drive))

            self.chk_confirm_usb_format = QCheckBox("Format the selected USB drive (all existing files on the USB stick will be erased)")
            self.chk_confirm_usb_format.setChecked(True)
            self.chk_confirm_usb_format.toggled.connect(lambda c: self.cfg.__setitem__("confirm_usb_erase", c))
            vbox.addWidget(self.chk_confirm_usb_format)

            # 4. Advanced Options (Ubuntu 26.04.1 LTS Default, Local ISO Browse, Rufus Scheme)
            adv = CollapsibleExpander("Advanced options")

            self.cb_ubuntu_rel = QComboBox()
            rel_opts = [
                ("26.04", "Ubuntu Server 26.04.1 LTS (Default — Latest LTS)"),
                ("24.04", "Ubuntu Server 24.04.5 LTS"),
                ("22.04", "Ubuntu Server 22.04.5 LTS"),
            ]
            for val, lbl in rel_opts:
                self.cb_ubuntu_rel.addItem(lbl, val)
            self.cb_ubuntu_rel.currentIndexChanged.connect(lambda i: self.cfg.__setitem__("ubuntu_release", self.cb_ubuntu_rel.itemData(i)))
            adv.body_layout.addWidget(self._make_card_row("🏷", "Ubuntu Server LTS release", "Pulls live official ISO & verifies SHA256SUMS", self.cb_ubuntu_rel))

            self.cb_part_scheme = QComboBox()
            scheme_opts = [
                ("auto", "Universal Hybrid (UEFI + Legacy BIOS)"),
                ("gpt_uefi", "GPT Partition Scheme for UEFI"),
                ("mbr_bios", "MBR Partition Scheme for BIOS / CSM"),
            ]
            for val, lbl in scheme_opts:
                self.cb_part_scheme.addItem(lbl, val)
            self.cb_part_scheme.currentIndexChanged.connect(lambda i: self.cfg.__setitem__("usb_partition_scheme", self.cb_part_scheme.itemData(i)))
            adv.body_layout.addWidget(self._make_card_row("🧭", "USB partition scheme (Rufus mode)", "Synced automatically with laptop age or override manually", self.cb_part_scheme))

            iso_row_w = QWidget()
            iso_row = QHBoxLayout(iso_row_w)
            iso_row.setContentsMargins(0, 0, 0, 0)
            self.in_custom_iso = QLineEdit(str(self.cfg["custom_iso_path"]))
            self.in_custom_iso.setPlaceholderText("Leave blank to auto-download official Ubuntu ISO, or pick local .iso")
            self.in_custom_iso.textChanged.connect(lambda v: self.cfg.__setitem__("custom_iso_path", v.strip()))
            btn_browse_iso = QPushButton("Browse .iso...")
            btn_browse_iso.clicked.connect(self._browse_local_iso)
            iso_row.addWidget(self.in_custom_iso, 1)
            iso_row.addWidget(btn_browse_iso)
            adv.body_layout.addWidget(self._make_card_row("💿", "Custom local Ubuntu .iso file (optional)", "Use a pre-downloaded Ubuntu Server ISO from your computer", iso_row_w))

            vbox.addWidget(adv)
            return page

        def _on_age_changed(self, age_val: str) -> None:
            self.cfg["laptop_age_hint"] = age_val
            if age_val == "modern_uefi":
                self.cfg["usb_partition_scheme"] = "gpt_uefi"
                self.cb_part_scheme.setCurrentIndex(1)
            elif age_val == "legacy_bios":
                self.cfg["usb_partition_scheme"] = "mbr_bios"
                self.cb_part_scheme.setCurrentIndex(2)
            else:
                self.cfg["usb_partition_scheme"] = "auto"
                self.cb_part_scheme.setCurrentIndex(0)

        def _on_install_mode_changed(self, mode_val: str) -> None:
            self.cfg["install_mode"] = mode_val
            self.chk_confirm_erase_laptop.setVisible(mode_val == "auto_erase")

        def _browse_local_iso(self) -> None:
            path, _ = QFileDialog.getOpenFileName(self, "Select Ubuntu Server ISO", "", "ISO Images (*.iso);;All Files (*)")
            if path:
                self.in_custom_iso.setText(path)
                self.cfg["custom_iso_path"] = path

        def _refresh_usb_list(self) -> None:
            self.usb_devices = api.scan_usb_devices()
            self.cb_usb_drive.blockSignals(True)
            self.cb_usb_drive.clear()
            approved = [d for d in self.usb_devices if d.get("triple_lock_passed")]
            if not approved:
                self.cb_usb_drive.addItem("No removable USB drive detected (plug in a USB drive and click Refresh)", "")
                self.cfg["selected_usb_device"] = ""
            else:
                for d in approved:
                    lbl = f"{d.get('model', 'USB Drive')} ({d.get('size_gb', 0)} GB) — {d.get('device_node', '')}"
                    self.cb_usb_drive.addItem(lbl, d.get("device_node") or d.get("device_id"))
                self.cfg["selected_usb_device"] = approved[0].get("device_node") or approved[0].get("device_id")
            self.cb_usb_drive.blockSignals(False)

        def _on_usb_combo_changed(self, idx: int) -> None:
            val = self.cb_usb_drive.itemData(idx)
            self.cfg["selected_usb_device"] = val or ""

        def _build_power_page(self) -> QWidget:
            page = QWidget()
            vbox = QVBoxLayout(page)
            vbox.setContentsMargins(0, 0, 0, 0)
            vbox.setSpacing(10)

            chk_lid = QCheckBox("Keep running 24/7 when the laptop lid is closed")
            chk_lid.setChecked(bool(self.cfg["ignore_lid_switch"]))
            chk_lid.toggled.connect(lambda c: self.cfg.__setitem__("ignore_lid_switch", c))
            vbox.addWidget(self._make_card_row("💻", "Clamshell lid-close sleep guard", "Configures systemd-logind HandleLidSwitch=ignore so closing the screen never puts the server to sleep", chk_lid))

            sl_bat = QSlider(Qt.Orientation.Horizontal)
            sl_bat.setRange(50, 85)
            sl_bat.setValue(int(self.cfg["battery_charge_stop_pct"]))
            lbl_bat_val = QLabel(f"Cap charge at {self.cfg['battery_charge_stop_pct']}%")
            sl_bat.valueChanged.connect(lambda v: (self.cfg.__setitem__("battery_charge_stop_pct", v), lbl_bat_val.setText(f"Cap charge at {v}%")))
            bat_w = QWidget()
            bat_r = QHBoxLayout(bat_w)
            bat_r.setContentsMargins(0, 0, 0, 0)
            bat_r.addWidget(sl_bat)
            bat_r.addWidget(lbl_bat_val)
            vbox.addWidget(self._make_card_row("🔋", "Battery health charge limiter", "Prevents battery swelling while plugged into wall power 24/7", bat_w))

            adv = CollapsibleExpander("Advanced options")
            chk_zram = QCheckBox("Enable Low-RAM zram memory compression (Recommended for 2 GB – 4 GB laptops)")
            chk_zram.setChecked(bool(self.cfg["enable_low_ram_zram"]))
            chk_zram.toggled.connect(lambda c: self.cfg.__setitem__("enable_low_ram_zram", c))
            adv.body_layout.addWidget(chk_zram)

            chk_gpu = QCheckBox("Safe graphics mode (nomodeset) for old laptops with broken NVIDIA/AMD GPUs")
            chk_gpu.setChecked(bool(self.cfg["safe_gpu_nomodeset"]))
            chk_gpu.toggled.connect(lambda c: self.cfg.__setitem__("safe_gpu_nomodeset", c))
            adv.body_layout.addWidget(chk_gpu)
            vbox.addWidget(adv)
            return page

        def _build_finish_page(self) -> QWidget:
            page = QWidget()
            vbox = QVBoxLayout(page)
            vbox.setContentsMargins(0, 0, 0, 0)
            vbox.setSpacing(12)

            self.lbl_summary_box = QLabel()
            self.lbl_summary_box.setWordWrap(True)
            vbox.addWidget(self._make_card_row("📋", "Instant Server Configuration Summary", "Ready to pull official Ubuntu Server LTS ISO, format USB, and pre-unpack selected CasaOS apps", self.lbl_summary_box))

            # Flashing Progress & Smart Terminal Card
            self.flash_card = QFrame()
            self.flash_card.setProperty("card", "true")
            fc_vbox = QVBoxLayout(self.flash_card)
            fc_vbox.setContentsMargins(16, 14, 16, 14)
            fc_vbox.setSpacing(10)

            top_r = QHBoxLayout()
            self.lbl_flash_status = QLabel("Ready to flash bootable USB drive.")
            self.lbl_flash_status.setStyleSheet("font-weight: 600; font-size: 13.5px;")
            top_r.addWidget(self.lbl_flash_status, 1)

            self.btn_cancel_flash_inline = QPushButton("✕ Cancel flashing")
            self.btn_cancel_flash_inline.setProperty("danger", "true")
            self.btn_cancel_flash_inline.setVisible(False)
            self.btn_cancel_flash_inline.clicked.connect(self._on_cancel_flash)
            top_r.addWidget(self.btn_cancel_flash_inline)

            self.btn_start_flash_inline = QPushButton("Confirm & Flash Bootable USB")
            self.btn_start_flash_inline.setProperty("accent", "true")
            self.btn_start_flash_inline.clicked.connect(self._start_flashing)
            top_r.addWidget(self.btn_start_flash_inline)
            fc_vbox.addLayout(top_r)

            self.progress_bar = QProgressBar()
            self.progress_bar.setRange(0, 100)
            self.progress_bar.setValue(0)
            fc_vbox.addWidget(self.progress_bar)

            # In-Place Terminal Live Progress Bar Row (never spams new lines!)
            self.lbl_terminal_live_bar = QLabel("")
            self.lbl_terminal_live_bar.setStyleSheet(
                "background-color: #142234; color: #60cdff; border: 1px solid #26466d; "
                "border-radius: 6px; padding: 6px 10px; font-family: 'Cascadia Mono', 'Consolas', monospace; "
                "font-size: 12px; font-weight: 600;"
            )
            self.lbl_terminal_live_bar.setVisible(False)
            fc_vbox.addWidget(self.lbl_terminal_live_bar)

            # Terminal Log Console with Smart Sticky Auto-Scroll
            self.log_console = QPlainTextEdit()
            self.log_console.setReadOnly(True)
            self.log_console.setMinimumHeight(195)
            self.log_console.verticalScrollBar().valueChanged.connect(self._on_log_scroll_changed)
            fc_vbox.addWidget(self.log_console)

            self.btn_resume_autoscroll = QPushButton("↓ Auto-scroll paused — Jump to bottom")
            self.btn_resume_autoscroll.setProperty("accent", "true")
            self.btn_resume_autoscroll.setVisible(False)
            self.btn_resume_autoscroll.clicked.connect(self._resume_log_autoscroll)
            fc_vbox.addWidget(self.btn_resume_autoscroll, 0, Qt.AlignmentFlag.AlignRight)

            vbox.addWidget(self.flash_card)
            return page

        def _on_log_scroll_changed(self, val: int) -> None:
            vbar = self.log_console.verticalScrollBar()
            at_bottom = val >= (vbar.maximum() - 15)
            self.log_auto_scroll = at_bottom
            self.btn_resume_autoscroll.setVisible(not at_bottom)

        def _resume_log_autoscroll(self) -> None:
            self.log_auto_scroll = True
            vbar = self.log_console.verticalScrollBar()
            vbar.setValue(vbar.maximum())
            self.btn_resume_autoscroll.setVisible(False)

        def _append_log_line(self, line: str) -> None:
            vbar = self.log_console.verticalScrollBar()
            prev_val = vbar.value()
            was_sticky = self.log_auto_scroll
            self.log_console.appendPlainText(line)
            if was_sticky:
                vbar.setValue(vbar.maximum())
            else:
                vbar.setValue(prev_val)

        def _switch_tab(self, idx: int) -> None:
            self.current_tab_index = max(0, min(len(self.tab_titles) - 1, idx))
            self.pages_stack.setCurrentIndex(self.current_tab_index)
            _, subtitle = self.tab_titles[self.current_tab_index]
            self.lbl_page_header.setText(subtitle)

            for i, b in enumerate(self.setup_tab_btns):
                b.setProperty("accent", "true" if i == self.current_tab_index else "false")
                b.style().unpolish(b)
                b.style().polish(b)
            for i, b in enumerate(self.sidebar_btns):
                b.setProperty("nav", "active" if i == self.current_tab_index else "true")
                b.style().unpolish(b)
                b.style().polish(b)

            self.btn_prev.setEnabled(self.current_tab_index > 0)
            is_last = self.current_tab_index == len(self.tab_titles) - 1
            if is_last:
                self._refresh_summary_text()
                self.btn_next_or_flash.setText("Confirm & Flash Bootable USB")
            else:
                self.btn_next_or_flash.setText("Proceed →")

        def _refresh_summary_text(self) -> None:
            apps_cnt = len(self.cfg["selected_apps"])
            usb_dev = self.cfg["selected_usb_device"] or "None selected"
            rel = self.cfg["ubuntu_release"]
            pt = "26.04.1" if rel == "26.04" else ("24.04.5" if rel == "24.04" else "22.04.5")
            self.lbl_summary_box.setText(
                f"Host: {self.cfg['hostname']}.local  •  OS: Ubuntu Server {pt} LTS  •  "
                f"Apps bundled: {apps_cnt}  •  USB: {usb_dev}"
            )

        def _on_prev_tab(self) -> None:
            if self.current_tab_index > 0:
                self._switch_tab(self.current_tab_index - 1)

        def _on_next_or_flash(self) -> None:
            if self.current_tab_index < len(self.tab_titles) - 1:
                self._switch_tab(self.current_tab_index + 1)
            else:
                self._start_flashing()

        def _on_global_search(self, text: str) -> None:
            q = text.strip().lower()
            if not q:
                return
            if any(k in q for k in ("wifi", "ssid", "password", "hostname", "ssh", "ip")):
                self._switch_tab(0)
            elif any(k in q for k in ("app", "casaos", "cockpit", "wikipedia", "minecraft", "ai", "ollama", "jellyfin", "pihole")):
                self._switch_tab(1)
            elif any(k in q for k in ("usb", "iso", "ubuntu", "26.04", "dual", "partition", "gpt", "mbr")):
                self._switch_tab(2)
            elif any(k in q for k in ("power", "battery", "lid", "zram")):
                self._switch_tab(3)

        def _on_save_config(self) -> None:
            api.save_config(self.cfg)
            self.lbl_flash_status.setText("Saved configuration to Instant_Server_Data/instant-server-config.json")

        def _start_flashing(self) -> None:
            if api.jobs.status == "RUNNING":
                return
            if not self.cfg.get("password") and not self.cfg.get("ssh_authorized_key"):
                QMessageBox.warning(self, "Admin Password Required", "Please set an Admin Password on the Network tab before flashing.")
                self._switch_tab(0)
                return
            if not self.cfg.get("selected_usb_device"):
                QMessageBox.warning(self, "Select a USB Drive", "Please select an approved removable USB flash drive on the Laptop & USB tab.")
                self._switch_tab(2)
                return
            if not self.cfg.get("confirm_usb_erase", True):
                QMessageBox.warning(self, "Confirm USB Formatting", "Please check the box confirming that existing files on the USB drive will be erased.")
                self._switch_tab(2)
                return

            self._switch_tab(4)
            self.log_console.clear()
            self.rendered_log_count = 0
            self.log_auto_scroll = True
            self.progress_bar.setValue(6)
            self._append_log_line("[00:00] Saved config to Instant_Server_Data/instant-server-config.json")

            self.btn_start_flash_inline.setEnabled(False)
            self.btn_next_or_flash.setEnabled(False)
            self.btn_cancel_flash_inline.setVisible(True)
            self.btn_cancel_flash_footer.setVisible(True)

            api.start_build(self.cfg)
            self.poll_timer.start()

        def _on_cancel_flash(self) -> None:
            if api.jobs.status != "RUNNING":
                return
            ans = QMessageBox.question(
                self,
                "Cancel USB Flashing?",
                "Are you sure you want to stop the current ISO download / USB flashing process?",
                QMessageBox.StandardButton.Yes | QMessageBox.StandardButton.No,
                QMessageBox.StandardButton.No,
            )
            if ans == QMessageBox.StandardButton.Yes:
                api.cancel_build()

        def _on_poll_flash_status(self) -> None:
            snap = api.poll_build_status()
            self.progress_bar.setValue(int(snap.get("percent", 0)))

            events = snap.get("events") or []
            for ev in events[self.rendered_log_count :]:
                line = ev.get("formatted_line") or f"[{ev.get('seq', 0):02d}] {ev.get('message', '')}"
                self._append_log_line(line)
            self.rendered_log_count = len(events)

            active_prog = snap.get("active_progress")
            if active_prog and active_prog.get("terminal_line"):
                self.lbl_terminal_live_bar.setText(active_prog["terminal_line"])
                self.lbl_terminal_live_bar.setVisible(True)
                self.lbl_flash_status.setText(active_prog.get("message") or "Flashing in progress...")
            else:
                self.lbl_terminal_live_bar.setVisible(False)
                if events:
                    self.lbl_flash_status.setText(events[-1].get("message", ""))

            status = snap.get("status")
            if status in ("COMPLETED", "FAILED", "CANCELLED"):
                self.poll_timer.stop()
                self.lbl_terminal_live_bar.setVisible(False)
                self.btn_start_flash_inline.setEnabled(True)
                self.btn_next_or_flash.setEnabled(True)
                self.btn_cancel_flash_inline.setVisible(False)
                self.btn_cancel_flash_footer.setVisible(False)
                if status == "COMPLETED":
                    self.progress_bar.setValue(100)
                    self.lbl_flash_status.setText("Bootable Instant Server USB is ready! Plug it into your old laptop and power it on.")

        def closeEvent(self, event: QCloseEvent) -> None:
            if api.jobs.status == "RUNNING" and not api.force_close_confirmed:
                ans = QMessageBox.warning(
                    self,
                    "Flashing is Still in Progress",
                    "Closing Instant Server right now will interrupt the Ubuntu Server ISO download or leave your USB drive incomplete.\n\n"
                    "Do you want to cancel flashing and close the app?",
                    QMessageBox.StandardButton.Yes | QMessageBox.StandardButton.No,
                    QMessageBox.StandardButton.No,
                )
                if ans != QMessageBox.StandardButton.Yes:
                    event.ignore()
                    return
                api.force_close_confirmed = True
                api.cancel_build()

            api.jobs.cancel_job()
            _remove_pid_file(paths["root_dir"])
            event.accept()
            os._exit(0)

    win = InstantServerNativeWindow()
    win.show()
    qt_app.exec()
    api.jobs.cancel_job()
    _remove_pid_file(paths["root_dir"])
    os._exit(0)


# ---------------------------------------------------------------------------
# 3. Interactive 3-Question Terminal Wizard (`--wizard`)
# ---------------------------------------------------------------------------
def run_interactive_cli_wizard(output_dir: str) -> Dict[str, Any]:
    print("=" * 76)
    print("  INSTANT SERVER — 3-Question Headless Laptop Server Wizard")
    print("=" * 76)

    print("\n[Question 1/3] Server Identity & Wi-Fi")
    hostname = input("  Server hostname [instant-server]: ").strip() or "instant-server"
    username = input("  Admin username [admin]: ").strip() or "admin"
    password = input("  Admin password: ").strip() or "instant2026"
    wifi_ssid = input("  Wi-Fi SSID (leave blank if using Ethernet only): ").strip()
    wifi_pw = ""
    if wifi_ssid:
        wifi_pw = input("  Wi-Fi Password: ").strip()

    print("\n[Question 2/3] Web Dashboards & Homelab Apps")
    print("  1) Both Cockpit + CasaOS [Recommended]")
    print("  2) Cockpit only")
    print("  3) CasaOS only")
    print("  4) Minimal SSH Server only (No Web UI)")
    dash_sel = input("  Choose dashboard [1]: ").strip() or "1"
    dash_map = {"1": "both", "2": "cockpit", "3": "casaos", "4": "none"}
    dashboard_choice = dash_map.get(dash_sel, "both")

    print("\n[Question 3/3] Target Laptop Hard Drive Installation Mode")
    print("  1) Dual-Boot (Keep existing Windows/Linux OS untouched) [Safe Default]")
    print("  2) Auto-Erase Main Drive (Dedicated server — requires confirmation)")
    print("  3) Select After Plugging In (Interactive partition chooser on laptop screen)")
    print("  4) Secondary Drive Only (Leave primary Disk 0 untouched)")
    mode_sel = input("  Choose installation mode [1]: ").strip() or "1"
    mode_map = {
        "1": "dual_boot",
        "2": "auto_erase",
        "3": "interactive_select",
        "4": "secondary_drive",
    }
    install_mode = mode_map.get(mode_sel, "dual_boot")

    confirm_erase = False
    dual_boot_gb = 45
    if install_mode == "dual_boot":
        gb_in = input("  GB to allocate for Instant Server alongside existing OS [45]: ").strip()
        if gb_in.isdigit():
            dual_boot_gb = int(gb_in)
    elif install_mode == "auto_erase":
        print("\n  [!] WARNING: Auto-Erase will format the target laptop's main internal drive.")
        ack = input("  Type 'I KNOW WHAT I AM DOING' to confirm: ").strip()
        confirm_erase = ack.upper() == "I KNOW WHAT I AM DOING"
        if not confirm_erase:
            raise ValueError("Auto-Erase aborted because confirmation phrase did not match.")

    prof = m4_config_builder.ServerProfile(
        hostname=hostname,
        username=username,
        password=password,
        wifi_ssid=wifi_ssid,
        wifi_password=wifi_pw,
        dashboard_choice=dashboard_choice,
        install_mode=install_mode,
        confirm_erase_main_drive=confirm_erase,
        dual_boot_Allocation_gb=dual_boot_gb,
    )
    return m5_payload_packager.stage_cidata_directory(prof, output_dir)


# ---------------------------------------------------------------------------
# 4. Built-In Self-Test (`--self-test`)
# ---------------------------------------------------------------------------
def _run_self_test() -> int:
    print("[CLI Self-Test] 1. Testing RFC 6762 mDNS query packet builder & parser...")
    pkt = build_mdns_query_packet("instant-server.local", qtype=1)
    assert b"\x0einstant-server\x05local\x00" in pkt
    resp_hdr = struct.pack(">HHHHHH", 0, 0x8400, 0, 1, 0, 0)
    resp_ans = (
        b"\x0einstant-server\x05local\x00"
        + struct.pack(">HHIH", 1, 1, 120, 4)
        + socket.inet_aton("192.168.1.142")
    )
    parsed_ip = parse_mdns_a_record_response(resp_hdr + resp_ans)
    assert parsed_ip == "192.168.1.142", f"Expected 192.168.1.142, got {parsed_ip}"
    print("  -> PASS: RFC 6762 mDNS packet builder and A-record response parser verified!")

    print("[CLI Self-Test] 2. Testing Zero-Config LAN TCP Service Discovery Scanner...")
    srv = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    srv.bind(("127.0.0.1", 0))
    srv.listen(1)
    ephemeral_port = srv.getsockname()[1]
    try:
        scan_res = discover_instant_servers_on_lan(
            hostname="instant-server",
            candidate_ips=["127.0.0.1"],
            ports=(ephemeral_port,),
            timeout=0.2,
        )
        assert len(scan_res["servers_found"]) == 1
        assert ephemeral_port in scan_res["servers_found"][0]["open_ports"]
    finally:
        srv.close()
    print("  -> PASS: Multi-threaded LAN TCP service discovery scanner verified!")

    print("[CLI Self-Test] 3. Testing Automatic `Instant_Server_Data/` Folder, Icons, App Catalog & Build...")
    with tempfile.TemporaryDirectory() as tmpdir:
        api = InstantServerJsApi(base_dir=tmpdir)
        ws_dir = os.path.join(tmpdir, WORKSPACE_FOLDER_NAME)
        assert os.path.isdir(ws_dir)
        assert os.path.isfile(os.path.join(ws_dir, "instant_server_icon.ico"))
        assert os.path.isfile(os.path.join(ws_dir, "instant_server_icon.icns"))
        assert len(api.get_app_catalog()) >= 18
        assert api.load_config()["exists"] is False

        save_res = api.save_config({"hostname": "instant-box", "username": "admin"})
        assert save_res["saved"] is True
        loaded = api.load_config()
        assert loaded["exists"] is True and loaded["config"]["hostname"] == "instant-box"

        job_start = api.start_build(
            {
                "hostname": "instant-box",
                "username": "admin",
                "password": "InstantPassword99!",
                "install_mode": "dual_boot",
                "dual_boot_Allocation_gb": 40,
                "selected_apps": ["jellyfin", "pihole"],
            },
            output_dir=os.path.join(ws_dir, "cidata"),
            auto_download_iso=False,
        )
        assert job_start["started"] is True
        for _ in range(40):
            snap = api.poll_build_status()
            if snap["status"] in ("COMPLETED", "FAILED"):
                break
            time.sleep(0.05)
        assert snap["status"] == "COMPLETED", f"Async build failed: {snap}"
        assert snap["result"]["hostname"] == "instant-box"
        assert snap["result"]["staged_apps"] == ["jellyfin", "pihole"]
    print("  -> PASS: Automatic workspace folder, 26-app catalog, and async build verified!")

    print("[CLI Self-Test] 4. Testing In-Place Terminal Progress Bar & Job Cancellation...")
    bar_str = format_terminal_progress_bar(50.0, label="Downloading ISO", detail="1320 / 2640 MB", width=20)
    assert "[██████████░░░░░░░░░░]" in bar_str and "50.0%" in bar_str
    pm = AsyncPipelineManager()
    pm.status = "RUNNING"
    pm.job_start_time = time.time()
    pm._record_event({"event": "JOB_STARTED", "message": "Starting job", "data": {"percent": 8}})
    for p in (15.0, 35.0, 65.0):
        pm._record_event({
            "event": "ISO_DOWNLOAD_PROGRESS",
            "message": f"Downloading {p}%",
            "data": {"in_place": True, "label": "Downloading Ubuntu ISO", "stage_percent": p, "detail": f"{p}% @ 20 MB/s", "percent": int(p)},
        })
    snap_prog = pm.get_snapshot()
    # Milestone events count remains 1 while active_progress reflects latest 65.0% in-place!
    assert snap_prog["event_count"] == 1, f"Expected 1 milestone log line, got {snap_prog['event_count']}"
    assert snap_prog["active_progress"] is not None
    assert snap_prog["active_progress"]["stage_percent"] == 65.0
    cancel_res = pm.cancel_job()
    assert cancel_res["cancelled"] is True
    try:
        pm._record_event({"event": "ISO_DOWNLOAD_PROGRESS", "message": "Next chunk", "data": {"in_place": True}})
        raise AssertionError("Expected InterruptedError on cancelled job")
    except InterruptedError:
        pass
    print("  -> PASS: In-place terminal progress bar (no log spam) & instant job cancellation verified!")
    return 0


def main() -> int:
    parser = argparse.ArgumentParser(description="Instant Server — Desktop App & CLI Launcher")
    parser.add_argument("--self-test", action="store_true", help="Run built-in unit and integration tests")
    parser.add_argument("--wizard", action="store_true", help="Run interactive 3-Question Terminal Wizard")
    parser.add_argument("--discover", action="store_true", help="Scan LAN & mDNS for running Instant Servers")
    parser.add_argument("--hostname", type=str, default="instant-server", help="Server hostname for discovery/build")
    parser.add_argument("--gui", action="store_true", help="Launch the Instant Server desktop app (default)")
    parser.add_argument("--port", type=int, default=8765, help="Local port for GUI HTTP bridge")
    parser.add_argument("--profile", type=str, default="", help="Path to JSON profile for non-interactive build")
    parser.add_argument("--output-dir", type=str, default="", help="Output directory for cidata")
    args = parser.parse_args()

    if args.self_test:
        return _run_self_test()

    paths = ensure_workspace_folder()
    out_dir = args.output_dir or paths["cidata_dir"]

    if args.discover:
        res = discover_instant_servers_on_lan(hostname=args.hostname)
        print(json.dumps(res, indent=2))
        return 0

    if args.wizard:
        res = run_interactive_cli_wizard(out_dir)
        print(json.dumps(res, indent=2))
        return 0

    if args.profile:
        with open(args.profile, "r", encoding="utf-8") as fh:
            data = json.load(fh)
        prof = m4_config_builder.ServerProfile(**_filter_server_profile_kwargs(data))
        res = m5_payload_packager.stage_cidata_directory(prof, out_dir)
        print(json.dumps(res, indent=2))
        return 0

    launch_desktop_or_browser_gui(port=args.port)
    return 0


if __name__ == "__main__":
    sys.exit(main())