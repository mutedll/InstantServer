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
import webbrowser
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


def _open_standalone_desktop_window(url: str, profile_dir: str) -> Optional[subprocess.Popen]:
    candidates: List[str] = []
    if sys.platform.startswith("win"):
        prog_files = [
            os.environ.get("ProgramFiles(x86)", "C:\\Program Files (x86)"),
            os.environ.get("ProgramFiles", "C:\\Program Files"),
            os.environ.get("LOCALAPPDATA", ""),
        ]
        rel_bins = [
            "Microsoft\\Edge\\Application\\msedge.exe",
            "Google\\Chrome\\Application\\chrome.exe",
            "BraveSoftware\\Brave-Browser\\Application\\brave.exe",
        ]
        for root in prog_files:
            if not root:
                continue
            for rel in rel_bins:
                full = os.path.join(root, rel)
                if os.path.isfile(full):
                    candidates.append(full)
    elif sys.platform == "darwin":
        mac_apps = [
            "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
            "/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
            "/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
        ]
        for app_bin in mac_apps:
            if os.path.isfile(app_bin):
                candidates.append(app_bin)
    else:
        for bin_name in ("google-chrome", "microsoft-edge", "chromium", "chromium-browser", "brave-browser"):
            found = shutil.which(bin_name)
            if found:
                candidates.append(found)

    os.makedirs(profile_dir, exist_ok=True)
    for exe in candidates:
        try:
            proc = subprocess.Popen(
                [
                    exe,
                    f"--app={url}",
                    "--window-size=1220,820",
                    f"--user-data-dir={profile_dir}",
                    "--no-first-run",
                    "--no-default-browser-check",
                ],
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
                **_win_no_window_kwargs(),
            )
            return proc
        except Exception:
            continue

    webbrowser.open(url)
    return None


def _render_gui_html_with_boot_config(gui_html_path: str, base_dir: Optional[str] = None) -> bytes:
    with open(gui_html_path, "r", encoding="utf-8") as fh:
        html = fh.read()
    boot_state = load_saved_config(base_dir)
    boot_script = f"<script>window.__INSTANT_SERVER_BOOT_CONFIG__ = {json.dumps(boot_state)};</script>\n</head>"
    if "</head>" in html:
        html = html.replace("</head>", boot_script, 1)
    return html.encode("utf-8")


class ReusableThreadingHTTPServer(http.server.ThreadingHTTPServer):
    allow_reuse_address = True
    daemon_threads = True


def launch_desktop_or_browser_gui(port: int = 8765, open_browser: bool = True) -> None:
    _set_windows_taskbar_app_id()
    paths = ensure_workspace_folder()
    _cleanup_previous_instance(port, paths["root_dir"])
    gui_html_path = os.path.join(_BUNDLED_DIR, "lazarus_gui.html")
    api = InstantServerJsApi(base_dir=paths["root_dir"])

    class GuiHandler(http.server.SimpleHTTPRequestHandler):
        def __init__(self, *args: Any, **kwargs: Any) -> None:
            super().__init__(*args, directory=_BUNDLED_DIR, **kwargs)

        def do_GET(self) -> None:
            api.last_heartbeat = time.time()
            path = self.path.split("?", 1)[0]
            if path in ("/", "/index.html", "/lazarus_gui.html"):
                api.client_connected = True
                body = _render_gui_html_with_boot_config(gui_html_path, paths["root_dir"])
                self.send_response(200)
                self.send_header("Content-Type", "text/html; charset=utf-8")
                self.send_header("Cache-Control", "no-store")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)
                return
            if path == "/favicon.ico":
                ico_bytes = open(paths["icon_ico_path"], "rb").read()
                self.send_response(200)
                self.send_header("Content-Type", "image/x-icon")
                self.send_header("Content-Length", str(len(ico_bytes)))
                self.end_headers()
                self.wfile.write(ico_bytes)
                return
            if path == "/api/host":
                self._send_json(api.probe_host())
                return
            if path == "/api/usb":
                self._send_json(api.scan_usb_devices())
                return
            if path == "/api/apps":
                self._send_json(api.get_app_catalog())
                return
            if path == "/api/load-config":
                self._send_json(api.load_config())
                return
            if path == "/api/discover":
                self._send_json(api.discover_servers())
                return
            if path == "/api/status":
                self._send_json(api.poll_build_status())
                return
            return super().do_GET()

        def do_POST(self) -> None:
            api.last_heartbeat = time.time()
            api.client_connected = True
            path = self.path.split("?", 1)[0]
            length = int(self.headers.get("Content-Length", "0") or "0")
            raw_body = self.rfile.read(length).decode("utf-8") if length > 0 else "{}"
            try:
                payload = json.loads(raw_body or "{}")
            except Exception:
                payload = {}

            if path == "/api/heartbeat":
                self._send_json(api.heartbeat())
                return
            if path == "/api/cancel":
                self._send_json(api.cancel_build())
                return
            if path == "/api/shutdown":
                self._send_json(api.shutdown_app(force=bool(payload.get("force", True))))
                return
            if path == "/api/save-config":
                self._send_json(api.save_config(payload))
                return
            if path == "/api/compile-preview":
                self._send_json(api.compile_preview(payload))
                return
            if path in ("/api/flash", "/api/start-build"):
                target_dev = payload.get("selected_usb_device")
                out_dir = payload.get("output_dir") or paths["cidata_dir"]
                self._send_json(api.start_build(payload, output_dir=out_dir, target_device=target_dev))
                return
            self._send_json({"error": f"Unknown endpoint {path}"}, status=404)

        def _send_json(self, obj: Any, status: int = 200) -> None:
            body = json.dumps(obj, indent=2).encode("utf-8")
            self.send_response(status)
            self.send_header("Content-Type", "application/json")
            self.send_header("Cache-Control", "no-store")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, format: str, *args: Any) -> None:
            pass

    server = ReusableThreadingHTTPServer(("127.0.0.1", port), GuiHandler)
    url = f"http://127.0.0.1:{port}/"

    try:
        import webview  # type: ignore

        srv_thread = threading.Thread(target=server.serve_forever, daemon=True)
        srv_thread.start()
        win = webview.create_window(
            "Instant Server",
            url,
            js_api=api,
            width=1220,
            height=820,
            min_size=(940, 660),
        )
        api._webview_window = win

        def _on_closing() -> Optional[bool]:
            if api.jobs.status == "RUNNING" and not api.force_close_confirmed:
                try:
                    win.evaluate_js("window.__showCloseWhileFlashingWarning && window.__showCloseWhileFlashingWarning()")
                except Exception:
                    pass
                return False
            return True

        def _on_closed() -> None:
            api.jobs.cancel_job()
            _remove_pid_file(paths["root_dir"])
            try:
                server.server_close()
            except Exception:
                pass
            os._exit(0)

        try:
            win.events.closing += _on_closing
            win.events.closed += _on_closed
        except Exception:
            pass

        try:
            webview.start(icon=paths["icon_ico_path"])
        except TypeError:
            webview.start()
        _on_closed()
        return
    except ImportError:
        pass

    print(f"[Instant Server] Workspace folder: {paths['workspace_dir']}")
    print(f"[Instant Server] Desktop UI running at {url}")
    browser_proc: Optional[subprocess.Popen] = None
    if open_browser:
        profile_dir = os.path.join(paths["workspace_dir"], "window_profile")
        browser_proc = _open_standalone_desktop_window(url, profile_dir=profile_dir)

    def _window_lifecycle_watchdog() -> None:
        """Automatically exit InstantServer.exe when the desktop window is closed."""
        launch_t = time.time()
        while True:
            time.sleep(1.0)
            # If the dedicated browser process exited and the UI isn't actively sending heartbeats, exit cleanly
            if browser_proc is not None and browser_proc.poll() is not None:
                if time.time() - api.last_heartbeat > 3.0:
                    api.jobs.cancel_job()
                    _remove_pid_file(paths["root_dir"])
                    os._exit(0)
            # If the client connected and then closed the window (no heartbeat for 6.5s), exit cleanly
            if api.client_connected and (time.time() - api.last_heartbeat > 6.5):
                api.jobs.cancel_job()
                _remove_pid_file(paths["root_dir"])
                os._exit(0)
            # Safety fallback if no window ever connected within 60s
            if not api.client_connected and (time.time() - launch_t > 60.0):
                _remove_pid_file(paths["root_dir"])
                os._exit(0)

    if open_browser:
        threading.Thread(target=_window_lifecycle_watchdog, daemon=True).start()

    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        _remove_pid_file(paths["root_dir"])
        server.server_close()
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