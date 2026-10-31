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
     `logs/`, and the taskbar icon `instant_server_icon.ico`.
2. Guaranteed Automatic Config Read on Next Launch:
   - Reads `Instant_Server_Data/instant-server-config.json` (or `./instant-server-config.json`)
     on startup and injects it directly into the HTML payload (`window.__INSTANT_SERVER_SAVED_CONFIG__`)
     as well as serving `/api/load-config` so the app always opens straight to the Home Settings
     view whenever a saved configuration exists.
3. Photoshop-Style Taskbar & Executable Icon (`IS` with Serif `I` Top & Bottom Bars):
   - Pure-Python generator creates a crisp 256x256 `.ico` and `.png` with a deep navy blue
     background (`#001E36`), sky-blue border (`#31A8FF`), and light-blue `IS` monogram where
     `I` has top and bottom horizontal bars, setting the Windows taskbar AppUserModelID.
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


def get_launch_root_dir() -> str:
    """
    Return the directory where the user launched the `.exe` or Python script from.
    When running as a PyInstaller frozen executable (`sys.frozen`), `sys.executable`
    points to the `.exe` file on disk rather than the temporary `_MEIPASS` folder.
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
    if not os.path.isfile(icon_ico_path) or not os.path.isfile(icon_png_path):
        try:
            generate_photoshop_style_is_icon(icon_ico_path, icon_png_path)
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
    }


def get_config_file_path(base_dir: Optional[str] = None) -> str:
    """Return the canonical path to `Instant_Server_Data/instant-server-config.json`."""
    paths = ensure_workspace_folder(base_dir)
    return paths["config_path"]


def load_saved_config(base_dir: Optional[str] = None) -> Dict[str, Any]:
    """
    Load `instant-server-config.json` from `Instant_Server_Data/instant-server-config.json`
    (or migrate from `./instant-server-config.json` if present).
    """
    paths = ensure_workspace_folder(base_dir)
    candidates = [paths["config_path"], paths["legacy_config_path"]]
    for cfg_path in candidates:
        if os.path.isfile(cfg_path):
            try:
                with open(cfg_path, "r", encoding="utf-8") as fh:
                    data = json.load(fh)
                if isinstance(data, dict) and data:
                    # Ensure a copy is also in the canonical workspace folder
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
    """Save the user's configuration to `Instant_Server_Data/instant-server-config.json`."""
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
    """Filter a dictionary to valid `m4_config_builder.ServerProfile` dataclass fields."""
    allowed = set(m4_config_builder.ServerProfile.__dataclass_fields__.keys())
    filtered: Dict[str, Any] = {}
    for k, v in raw.items():
        if k in allowed:
            filtered[k] = v
    if not filtered.get("password") and not filtered.get("ssh_authorized_keys") and not filtered.get("github_ssh_username"):
        filtered["password"] = "instant2026"
    return filtered


# ---------------------------------------------------------------------------
# 0b. Pure-Python Photoshop-Style `IS` Taskbar Icon Generator (.ico & .png)
# ---------------------------------------------------------------------------
def _render_photoshop_is_rgba(size: int = 128) -> bytes:
    """
    Render a Photoshop-inspired dark blue rounded-square icon (`#001E36`) with a
    sky-blue border (`#31A8FF`) and light-blue `IS` letters (`#31A8FF`) where the
    capital `I` has distinct top and bottom horizontal serif bars.
    Returns raw RGBA scanlines with leading filter byte `0x00` per row for PNG encoding.
    """
    bg_rgb = (0, 30, 54)         # Deep Photoshop navy blue (#001E36)
    border_rgb = (49, 168, 255)  # Photoshop sky blue (#31A8FF)
    fg_rgb = (102, 196, 255)     # Bright light blue for 'IS' (#66C4FF)

    rows = bytearray()
    radius = size * 0.21
    border_w = max(2.0, size * 0.045)

    for y in range(size):
        rows.append(0)  # PNG filter type 0 (None)
        ny = y / float(size)
        for x in range(size):
            nx = x / float(size)

            # Rounded rectangle signed distance check
            dx = max(abs(x - (size - 1) / 2.0) - (size / 2.0 - radius), 0.0)
            dy = max(abs(y - (size - 1) / 2.0) - (size / 2.0 - radius), 0.0)
            dist = math.hypot(dx, dy) - radius

            if dist > 0.8:
                rows.extend((0, 0, 0, 0))
                continue

            alpha = 255 if dist < -0.8 else int(255 * (0.8 - dist) / 1.6)
            is_border = dist >= -border_w

            # Normalized coordinates for 'I' (with top & bottom horizontal bars) and 'S'
            # Vertical bounds for both letters: ny in [0.24, 0.76]
            in_i = False
            if 0.24 <= ny <= 0.76:
                # Top bar of 'I' (ny in [0.24, 0.315]) or Bottom bar of 'I' (ny in [0.685, 0.76])
                if (0.24 <= ny <= 0.315 or 0.685 <= ny <= 0.76) and (0.18 <= nx <= 0.42):
                    in_i = True
                # Vertical stem of 'I'
                elif 0.265 <= nx <= 0.335:
                    in_i = True

            # Geometric 'S' in nx in [0.48, 0.80], ny in [0.24, 0.76]
            in_s = False
            if 0.24 <= ny <= 0.76 and 0.47 <= nx <= 0.81:
                # Top arc / bar
                if 0.24 <= ny <= 0.315 and 0.51 <= nx <= 0.79:
                    in_s = True
                # Bottom arc / bar
                elif 0.685 <= ny <= 0.76 and 0.48 <= nx <= 0.76:
                    in_s = True
                # Middle bar
                elif 0.465 <= ny <= 0.535 and 0.51 <= nx <= 0.76:
                    in_s = True
                # Upper-left curve
                elif 0.28 <= ny <= 0.50 and 0.48 <= nx <= 0.555:
                    in_s = True
                # Lower-right curve
                elif 0.50 <= ny <= 0.72 and 0.725 <= nx <= 0.80:
                    in_s = True
                # Top-right serif tip
                elif 0.26 <= ny <= 0.35 and 0.725 <= nx <= 0.795:
                    in_s = True
                # Bottom-left serif tip
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
    """Encode the Photoshop-style `IS` icon as a spec-compliant PNG image."""
    raw_scanlines = _render_photoshop_is_rgba(size=size)

    def chunk(tag: bytes, data: bytes) -> bytes:
        crc = zlib.crc32(tag + data) & 0xFFFFFFFF
        return struct.pack(">I", len(data)) + tag + data + struct.pack(">I", crc)

    ihdr = struct.pack(">IIBBBBB", size, size, 8, 6, 0, 0, 0)
    idat = zlib.compress(raw_scanlines, level=9)
    return b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", ihdr) + chunk(b"IDAT", idat) + chunk(b"IEND", b"")


def generate_photoshop_style_is_icon(ico_path: str, png_path: Optional[str] = None) -> None:
    """
    Write a Windows `.ico` file (wrapping a 256x256/128x128 PNG) and optional `.png`
    file for the taskbar and PyInstaller executable icon.
    """
    png_data = build_png_bytes(size=128)
    if png_path:
        with open(png_path, "wb") as pfh:
            pfh.write(png_data)

    # ICO header (6 bytes): Reserved=0, Type=1 (ICO), Count=1
    ico_header = struct.pack("<HHH", 0, 1, 1)
    # ICO directory entry (16 bytes): Width=128, Height=128, ColorCount=0, Reserved=0,
    # Planes=1, BitCount=32, BytesInRes=len(png_data), ImageOffset=22
    ico_entry = struct.pack("<BBBBHHII", 128, 128, 0, 0, 1, 32, len(png_data), 22)
    with open(ico_path, "wb") as ifh:
        ifh.write(ico_header + ico_entry + png_data)


# ---------------------------------------------------------------------------
# 1. Pure-Python RFC 6762 mDNS Packet Builder & Dual-Mode LAN Discovery Engine
# ---------------------------------------------------------------------------
MDNS_MULTICAST_IP = "224.0.0.251"
MDNS_PORT = 5353


def build_mdns_query_packet(qname: str, qtype: int = 1) -> bytes:
    """Build a minimal RFC 6762 / RFC 1035 DNS query packet for `qname`."""
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
    """Extract the first IPv4 A-record address from a raw DNS/mDNS response packet."""
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
    """Resolve `<hostname>.local` via OS resolver first, then UDP multicast query."""
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
    """Probe a single IPv4 host for Instant Server ports (9090, 80, 22)."""
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
    """Return the primary local LAN IPv4 address and known ARP neighbor IPs."""
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
    """Discover Instant Server nodes across the user's LAN."""
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
# 2. Asynchronous Job Runner & Local HTTP / `pywebview` GUI Bridge
# ---------------------------------------------------------------------------
class AsyncPipelineManager:
    """Runs M1-M5 tasks on a background daemon thread."""

    def __init__(self) -> None:
        self._lock = threading.Lock()
        self.status: str = "IDLE"
        self.percent: int = 0
        self.events: List[Dict[str, Any]] = []
        self.last_result: Optional[Dict[str, Any]] = None
        self.error: Optional[str] = None

    def _record_event(self, payload: Dict[str, Any]) -> None:
        with self._lock:
            self.events.append(payload)
            ev = payload.get("event", "")
            data = payload.get("data") or {}
            if "percent" in data:
                self.percent = int(data["percent"])
            elif ev == "M4_COMPILE_COMPLETE":
                self.percent = max(self.percent, 35)
            elif ev == "M5_STAGING_COMPLETE":
                self.percent = max(self.percent, 75)
            elif ev == "USB_COPY_CIDATA":
                self.percent = max(self.percent, 88)
            elif ev == "USB_PROVISION_COMPLETE":
                self.percent = 100

    def start_stage_job(
        self,
        profile_dict: Dict[str, Any],
        output_dir: Optional[str] = None,
        target_device: Optional[str] = None,
        iso_path: Optional[str] = None,
    ) -> Dict[str, Any]:
        paths = ensure_workspace_folder()
        resolved_out_dir = output_dir or paths["cidata_dir"]
        with self._lock:
            if self.status == "RUNNING":
                return {"started": False, "reason": "Job already running"}
            self.status = "RUNNING"
            self.percent = 10
            self.events = []
            self.last_result = None
            self.error = None

        def worker() -> None:
            try:
                self._record_event(
                    {
                        "event": "JOB_STARTED",
                        "module": "CLI_BRIDGE",
                        "message": f"Validating configuration for '{profile_dict.get('hostname', 'instant-server')}'...",
                        "data": {"percent": 15},
                    }
                )
                kwargs = _filter_server_profile_kwargs(profile_dict)
                prof = m4_config_builder.ServerProfile(**kwargs)
                summary = m5_payload_packager.stage_cidata_directory(
                    profile=prof,
                    staging_dir=resolved_out_dir,
                    iso_path_for_debs=iso_path,
                    event_callback=self._record_event,
                )
                usb_res = m3_usb_engine.provision_usb_drive(
                    staged_cidata_dir=resolved_out_dir,
                    target_device=target_device,
                    iso_path=iso_path,
                    event_callback=self._record_event,
                )
                summary["usb_provision"] = usb_res
                with self._lock:
                    self.last_result = summary
                    self.percent = 100
                    self.status = "COMPLETED"
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
                "events": list(self.events[-40:]),
                "result": self.last_result,
                "error": self.error,
            }


class InstantServerJsApi:
    """
    JavaScript-callable API exposed to `pywebview.create_window(..., js_api=...)`
    and mirrored by the local HTTP server endpoints.
    """

    def __init__(self, base_dir: Optional[str] = None) -> None:
        self.base_dir = base_dir or get_launch_root_dir()
        ensure_workspace_folder(self.base_dir)
        self.jobs = AsyncPipelineManager()

    def probe_host(self) -> Dict[str, Any]:
        return asdict(m1_host_probe.probe_host_environment())

    def scan_usb_devices(self) -> List[Dict[str, Any]]:
        return [asdict(d) for d in m3_usb_engine.scan_host_usb_devices()]

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
    ) -> Dict[str, Any]:
        paths = ensure_workspace_folder(self.base_dir)
        out_dir = output_dir or paths["cidata_dir"]
        target = target_device or profile_dict.get("selected_usb_device")
        save_local_config(profile_dict, self.base_dir)
        return self.jobs.start_stage_job(profile_dict, out_dir, target_device=target)

    def poll_build_status(self) -> Dict[str, Any]:
        return self.jobs.get_snapshot()

    def discover_servers(self, hostname: str = "instant-server") -> Dict[str, Any]:
        return discover_instant_servers_on_lan(hostname=hostname)


def _set_windows_taskbar_app_id() -> None:
    """Set Windows AppUserModelID so the taskbar displays the custom Photoshop-style `IS` icon."""
    if sys.platform.startswith("win"):
        try:
            import ctypes
            ctypes.windll.shell32.SetCurrentProcessExplicitAppUserModelID("io.instantserver.desktop.app")
        except Exception:
            pass


def _open_standalone_desktop_window(url: str) -> None:
    """
    Launch `url` in a standalone frameless desktop app window (`--app=url`)
    using Microsoft Edge, Google Chrome, Brave, or Chromium.
    """
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

    for exe in candidates:
        try:
            subprocess.Popen(
                [exe, f"--app={url}", "--window-size=1220,820"],
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
            )
            return
        except Exception:
            continue

    webbrowser.open(url)


def _render_gui_html_with_boot_config(gui_html_path: str, base_dir: Optional[str] = None) -> bytes:
    """
    Read `lazarus_gui.html` and inject `window.__INSTANT_SERVER_BOOT_CONFIG__` with the
    current saved config from `Instant_Server_Data/instant-server-config.json` so the UI
    immediately boots into the Home Settings screen on the very first frame if a saved
    config exists on disk.
    """
    with open(gui_html_path, "r", encoding="utf-8") as fh:
        html = fh.read()
    boot_state = load_saved_config(base_dir)
    boot_script = f"<script>window.__INSTANT_SERVER_BOOT_CONFIG__ = {json.dumps(boot_state)};</script>\n</head>"
    if "</head>" in html:
        html = html.replace("</head>", boot_script, 1)
    return html.encode("utf-8")


def launch_desktop_or_browser_gui(port: int = 8765, open_browser: bool = True) -> None:
    """
    Launch the Instant Server desktop application (`lazarus_gui.html`).
    Automatically creates `Instant_Server_Data/` in the folder where the `.exe` or script
    was opened from, generates the Photoshop-style `IS` taskbar icon, and loads any saved
    configuration automatically.
    """
    _set_windows_taskbar_app_id()
    paths = ensure_workspace_folder()
    gui_html_path = os.path.join(_BUNDLED_DIR, "lazarus_gui.html")
    api = InstantServerJsApi(base_dir=paths["root_dir"])

    class GuiHandler(http.server.SimpleHTTPRequestHandler):
        def __init__(self, *args: Any, **kwargs: Any) -> None:
            super().__init__(*args, directory=_BUNDLED_DIR, **kwargs)

        def do_GET(self) -> None:
            path = self.path.split("?", 1)[0]
            if path in ("/", "/index.html", "/lazarus_gui.html"):
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
            path = self.path.split("?", 1)[0]
            length = int(self.headers.get("Content-Length", "0") or "0")
            raw_body = self.rfile.read(length).decode("utf-8") if length > 0 else "{}"
            try:
                payload = json.loads(raw_body or "{}")
            except Exception:
                payload = {}

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

    server = http.server.ThreadingHTTPServer(("127.0.0.1", port), GuiHandler)
    url = f"http://127.0.0.1:{port}/"

    # Try native pywebview pointed at our local HTTP server so both window.pywebview.api
    # AND synchronous boot-config injection work seamlessly
    try:
        import webview  # type: ignore

        srv_thread = threading.Thread(target=server.serve_forever, daemon=True)
        srv_thread.start()
        webview.create_window(
            "Instant Server",
            url,
            js_api=api,
            width=1220,
            height=820,
            min_size=(940, 660),
        )
        try:
            webview.start(icon=paths["icon_ico_path"])
        except TypeError:
            webview.start()
        server.server_close()
        return
    except ImportError:
        pass

    print(f"[Instant Server] Workspace folder: {paths['workspace_dir']}")
    print(f"[Instant Server] Desktop UI running at {url}")
    if open_browser:
        _open_standalone_desktop_window(url)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        server.server_close()


# ---------------------------------------------------------------------------
# 3. Interactive 3-Question Terminal Wizard (`--wizard`)
# ---------------------------------------------------------------------------
def run_interactive_cli_wizard(output_dir: str) -> Dict[str, Any]:
    """Run the 3-Question Beginner Wizard interactively in the terminal."""
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
    if install_boot_mode := install_mode == "dual_boot":
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

    print("[CLI Self-Test] 3. Testing Automatic `Instant_Server_Data/` Folder, Icon & Config Persistence...")
    with tempfile.TemporaryDirectory() as tmpdir:
        api = InstantServerJsApi(base_dir=tmpdir)
        ws_dir = os.path.join(tmpdir, WORKSPACE_FOLDER_NAME)
        assert os.path.isdir(ws_dir), "Expected Instant_Server_Data folder to be created automatically"
        assert os.path.isfile(os.path.join(ws_dir, "instant_server_icon.ico")), "Expected Photoshop-style IS .ico"
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
            },
            output_dir=os.path.join(ws_dir, "cidata"),
        )
        assert job_start["started"] is True
        for _ in range(40):
            snap = api.poll_build_status()
            if snap["status"] in ("COMPLETED", "FAILED"):
                break
            time.sleep(0.05)
        assert snap["status"] == "COMPLETED", f"Async build failed: {snap}"
        assert snap["result"]["hostname"] == "instant-box"
    print("  -> PASS: Automatic workspace folder, Photoshop-style IS icon & config auto-load verified!")
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

    # Default behavior: launch the desktop GUI directly!
    launch_desktop_or_browser_gui(port=args.port)
    return 0


if __name__ == "__main__":
    sys.exit(main())