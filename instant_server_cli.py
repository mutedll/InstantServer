#!/usr/bin/env python3
"""
Instant Server — Phase 3: Unified Cross-Platform Desktop App Launcher, Local Config Store & CLI
===============================================================================================
File: instant_server_cli.py
Project: Instant Server (Turn any old laptop into a headless Ubuntu Server)
Runs on: Windows 10/11, macOS, and Linux (Python 3.8+ through 3.14+, zero external deps)

Responsibilities:
1. Default Desktop App Experience:
   - Running `python instant_server_cli.py` (with or without `--gui`) launches the native
     desktop window immediately via `pywebview` (if installed) or a standalone frameless
     desktop app window (`--app=http://127.0.0.1:<port>`) using Edge/Chrome/Chromium.
2. Persistent Local Configuration (`instant-server-config.json`):
   - Saves and loads the user's configuration directly in the application folder so if a saved
     config already exists, the app opens straight to the Main Settings Home page on next launch.
3. Real Connected Hardware Execution (`InstantServerJsApi` & HTTP `/api/*` Endpoints):
   - Connects the UI directly to `m1_host_probe`, `m2_iso_manager`, `m3_usb_engine`,
     `m4_config_builder`, and `m5_payload_packager` to scan real USB drives, compile cloud-init
     YAML payloads, stage `cidata`, and provision USB drives with live progress logs.
4. Zero-Config LAN & mDNS Server Discovery Scanner (`--discover`):
   - Finds headless Instant Server laptops on the local network via mDNS and TCP port probing.
"""

from __future__ import annotations

import argparse
from concurrent.futures import ThreadPoolExecutor, as_completed
from dataclasses import asdict
import http.server
import ipaddress
import json
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

# Ensure sibling modules can be imported
_CURRENT_DIR = os.path.dirname(os.path.abspath(__file__))
if _CURRENT_DIR not in sys.path:
    sys.path.insert(0, _CURRENT_DIR)

import m1_host_probe  # noqa: E402
import m2_iso_manager  # noqa: E402
import m3_usb_engine  # noqa: E402
import m4_config_builder  # noqa: E402
import m5_payload_packager  # noqa: E402

CONFIG_FILE_NAME = "instant-server-config.json"
DEFAULT_CIDATA_DIR = os.path.join(_CURRENT_DIR, "instant_cidata_out")


def get_config_file_path(base_dir: Optional[str] = None) -> str:
    """Return the path to `instant-server-config.json` in the application folder."""
    folder = base_dir or _CURRENT_DIR
    return os.path.join(folder, CONFIG_FILE_NAME)


def load_saved_config(base_dir: Optional[str] = None) -> Dict[str, Any]:
    """Load `instant-server-config.json` from the app folder if it exists."""
    cfg_path = get_config_file_path(base_dir)
    if not os.path.isfile(cfg_path):
        return {"exists": False, "path": cfg_path, "config": None}
    try:
        with open(cfg_path, "r", encoding="utf-8") as fh:
            data = json.load(fh)
        return {"exists": True, "path": cfg_path, "config": data}
    except Exception as exc:
        return {"exists": False, "path": cfg_path, "config": None, "error": str(exc)}


def save_local_config(config_data: Dict[str, Any], base_dir: Optional[str] = None) -> Dict[str, Any]:
    """Save the user's configuration to `instant-server-config.json` in the app folder."""
    cfg_path = get_config_file_path(base_dir)
    os.makedirs(os.path.dirname(os.path.abspath(cfg_path)), exist_ok=True)
    with open(cfg_path, "w", encoding="utf-8") as fh:
        json.dump(config_data, fh, indent=2)
    return {"saved": True, "path": cfg_path, "config": config_data}


def _filter_server_profile_kwargs(raw: Dict[str, Any]) -> Dict[str, Any]:
    """Filter a dictionary to valid `m4_config_builder.ServerProfile` dataclass fields."""
    allowed = set(m4_config_builder.ServerProfile.__dataclass_fields__.keys())
    filtered: Dict[str, Any] = {}
    for k, v in raw.items():
        if k in allowed:
            filtered[k] = v
    # Ensure a valid fallback password if the user left password blank during preview/skip
    if not filtered.get("password") and not filtered.get("ssh_authorized_keys") and not filtered.get("github_ssh_username"):
        filtered["password"] = "instant2026"
    return filtered


# ---------------------------------------------------------------------------
# 1. Pure-Python RFC 6762 mDNS Packet Builder & Dual-Mode LAN Discovery Engine
# ---------------------------------------------------------------------------
MDNS_MULTICAST_IP = "224.0.0.251"
MDNS_PORT = 5353


def build_mdns_query_packet(qname: str, qtype: int = 1) -> bytes:
    """
    Build a minimal RFC 6762 / RFC 1035 DNS query packet for `qname`
    (e.g., `instant-server.local`, qtype=1 for A record, qclass=1 for IN).
    """
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
    """
    Resolve `<hostname>.local` via OS resolver first, then fallback to a direct
    UDP multicast query to `224.0.0.251:5353`.
    """
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
    """
    Probe a single IPv4 host for Instant Server signatures:
      - Port 9090 (Cockpit Web Console)
      - Port 80   (CasaOS Web Dashboard)
      - Port 22   (OpenSSH Server + banner fingerprint)
    """
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
    """
    Discover Instant Server nodes across the user's LAN even when the target laptop
    has no screen and the user's router/OS blocks `.local` mDNS.
    """
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
    """
    Runs long-running M1-M5 tasks on a background daemon thread so neither
    `pywebview`'s `js_api` nor the local HTTP bridge ever blocks the UI thread.
    """

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
        output_dir: str = DEFAULT_CIDATA_DIR,
        target_device: Optional[str] = None,
        iso_path: Optional[str] = None,
    ) -> Dict[str, Any]:
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
                    staging_dir=output_dir,
                    iso_path_for_debs=iso_path,
                    event_callback=self._record_event,
                )
                usb_res = m3_usb_engine.provision_usb_drive(
                    staged_cidata_dir=output_dir,
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
        self.base_dir = base_dir or _CURRENT_DIR
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
        out_dir = output_dir or os.path.join(self.base_dir, "instant_cidata_out")
        target = target_device or profile_dict.get("selected_usb_device")
        save_local_config(profile_dict, self.base_dir)
        return self.jobs.start_stage_job(profile_dict, out_dir, target_device=target)

    def poll_build_status(self) -> Dict[str, Any]:
        return self.jobs.get_snapshot()

    def discover_servers(self, hostname: str = "instant-server") -> Dict[str, Any]:
        return discover_instant_servers_on_lan(hostname=hostname)


def _open_standalone_desktop_window(url: str) -> None:
    """
    Try to launch `url` in a native-looking frameless desktop app window (`--app=url`)
    using Microsoft Edge, Google Chrome, Brave, or Chromium before falling back to the
    default system browser.
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
        mac_Apps = [
            "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
            "/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
            "/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
        ]
        for app_bin in mac_Apps:
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


def launch_desktop_or_browser_gui(port: int = 8765, open_browser: bool = True) -> None:
    """
    Launch the Instant Server desktop application (`lazarus_gui.html`).
    1. Uses native `pywebview` window if installed.
    2. Otherwise starts a zero-dependency local HTTP + JSON IPC server on `127.0.0.1:<port>`
       and launches a standalone `--app=` window.
    """
    gui_html_path = os.path.join(_CURRENT_DIR, "lazarus_gui.html")
    api = InstantServerJsApi()

    try:
        import webview  # type: ignore

        webview.create_window(
            "Instant Server",
            gui_html_path,
            js_api=api,
            width=1220,
            height=820,
            min_size=(940, 660),
        )
        webview.start()
        return
    except ImportError:
        pass

    class GuiHandler(http.server.SimpleHTTPRequestHandler):
        def __init__(self, *args: Any, **kwargs: Any) -> None:
            super().__init__(*args, directory=_CURRENT_DIR, **kwargs)

        def do_GET(self) -> None:
            path = self.path.split("?", 1)[0]
            if path in ("/", "/index.html"):
                self.path = "/lazarus_gui.html"
                return super().do_GET()
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
                out_dir = payload.get("output_dir") or DEFAULT_CIDATA_DIR
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

    print("[CLI Self-Test] 3. Testing Local Config Persistence & Async GUI Bridge (`InstantServerJsApi`)...")
    with tempfile.TemporaryDirectory() as tmpdir:
        api = InstantServerJsApi(base_dir=tmpdir)
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
            output_dir=os.path.join(tmpdir, "cidata_out"),
        )
        assert job_start["started"] is True
        for _ in range(40):
            snap = api.poll_build_status()
            if snap["status"] in ("COMPLETED", "FAILED"):
                break
            time.sleep(0.05)
        assert snap["status"] == "COMPLETED", f"Async build failed: {snap}"
        assert snap["result"]["hostname"] == "instant-box"
    print("  -> PASS: Local config save/load & asynchronous background job worker verified!")
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
    parser.add_argument("--output-dir", type=str, default="./instant_cidata_out", help="Output directory for cidata")
    args = parser.parse_args()

    if args.self_test:
        return _run_self_test()

    if args.discover:
        res = discover_instant_servers_on_lan(hostname=args.hostname)
        print(json.dumps(res, indent=2))
        return 0

    if args.wizard:
        res = run_interactive_cli_wizard(args.output_dir)
        print(json.dumps(res, indent=2))
        return 0

    if args.profile:
        with open(args.profile, "r", encoding="utf-8") as fh:
            data = json.load(fh)
        prof = m4_config_builder.ServerProfile(**_filter_server_profile_kwargs(data))
        res = m5_payload_packager.stage_cidata_directory(prof, args.output_dir)
        print(json.dumps(res, indent=2))
        return 0

    # Default behavior: launch the desktop GUI directly!
    launch_desktop_or_browser_gui(port=args.port)
    return 0


if __name__ == "__main__":
    sys.exit(main())