#!/usr/bin/env python3
"""
disk_manager.py
Windows Disk Management–grade Partition & Disk Selector for Instant Server.
Runs as root during Live USB boot (port 8088) and from the Server Dashboard.
Serves static HTML/CSS/JS assets (disk_manager.html/.css/.js) and bridges
live target selection to Subiquity's Unix socket (/run/subiquity/socket).
"""

import http.client
import http.server
import json
import os
import pathlib
import socket
import sys
import threading
import time
import urllib.parse

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))
from disk_ops import (
    INSTALLER_MODE,
    PORT,
    handle_disk_action,
    is_root,
    list_disks,
)

STATIC_SEARCH_DIRS = [
    pathlib.Path(__file__).resolve().parent,
    pathlib.Path(__file__).resolve().parent.parent.parent / "ui" / "disk_manager",
    pathlib.Path("/tmp/cidata"),
    pathlib.Path("/opt/instant-server"),
]

STATIC_MIME = {
    "disk_manager.html": "text/html; charset=utf-8",
    "disk_manager.css": "text/css; charset=utf-8",
    "disk_manager.js": "application/javascript; charset=utf-8",
}

FALLBACK_HTML = (
    "<!DOCTYPE html><html><head><meta charset='utf-8'>"
    "<meta name='viewport' content='width=device-width,initial-scale=1'>"
    "<title>Disk Management</title></head>"
    "<body style='background:#0b0e14;color:#f1f5f9;font-family:sans-serif;padding:24px'>"
    "<h2>Disk Management</h2>"
    "<p>Static UI assets (disk_manager.html) not found alongside disk_manager.py.</p>"
    "</body></html>"
)


class _UnixHTTPConnection(http.client.HTTPConnection):
    def __init__(self, sock_path: str, timeout: float = 10.0):
        super().__init__("localhost", timeout=timeout)
        self._sock_path = sock_path

    def connect(self) -> None:
        af_unix = getattr(socket, "AF_UNIX", None)
        if af_unix is None:
            raise OSError("Unix domain sockets are only supported on Linux/POSIX hosts")
        s = socket.socket(af_unix, socket.SOCK_STREAM)
        s.settimeout(self.timeout)
        s.connect(self._sock_path)
        self.sock = s


def _subiquity_req(method: str, path: str, body: dict | None = None) -> tuple[int, dict | list | str | None]:
    sock_path = "/run/subiquity/socket"
    if not os.path.exists(sock_path):
        return 0, None
    try:
        conn = _UnixHTTPConnection(sock_path, timeout=15.0)
        headers = {"Content-Type": "application/json"} if body is not None else {}
        payload = json.dumps(body).encode("utf-8") if body is not None else None
        conn.request(method, path, body=payload, headers=headers)
        resp = conn.getresponse()
        raw = resp.read().decode("utf-8", errors="replace")
        conn.close()
        try:
            return resp.status, json.loads(raw) if raw.strip() else None
        except Exception:
            return resp.status, raw
    except Exception as exc:
        return 0, str(exc)


def drive_subiquity_live_selection(target_disk: str) -> tuple[bool, str]:
    """If Subiquity native Disk Selector is WAITING, apply DIRECT full-disk layout & start install."""
    st_code, st_data = _subiquity_req("GET", "/meta/status")
    if st_code != 200 or not isinstance(st_data, dict):
        return False, ""
    if st_data.get("state") not in ("WAITING", "NEEDS_CONFIRMATION"):
        return False, ""

    disk_id = f"disk-{os.path.basename(target_disk)}"
    g_code, g_data = _subiquity_req("GET", "/storage/v2/guided")
    if g_code != 200 or not isinstance(g_data, dict):
        return False, "Subiquity guided storage not ready yet"

    chosen_target = None
    for t in g_data.get("targets", []) or []:
        if isinstance(t, dict) and t.get("disk_id") == disk_id:
            chosen_target = t
            break
    if not chosen_target:
        return False, f"Disk {disk_id} not found in Subiquity guided targets"

    choice_body = {
        "target": chosen_target,
        "capability": "DIRECT",
        "sizing_policy": "ALL",
        "reset_partition": False,
    }
    p_code, _ = _subiquity_req("POST", "/storage/v2/guided", choice_body)
    if p_code not in (200, 201, 204):
        return False, f"Subiquity guided POST failed ({p_code})"

    s_code, s_data = _subiquity_req("GET", "/storage")
    if s_code == 200 and isinstance(s_data, dict) and s_data.get("config"):
        _subiquity_req("POST", "/storage", s_data.get("config"))
    _subiquity_req("POST", "/meta/mark_configured", {})
    _subiquity_req("POST", "/meta/confirm", {"tty": "/dev/tty1"})
    return True, f"Applied {target_disk} (Direct full-disk, no LVM) directly to Subiquity — installation started automatically!"


def load_static_asset(filename: str) -> bytes | None:
    for base in STATIC_SEARCH_DIRS:
        cand = base / filename
        if cand.is_file():
            try:
                return cand.read_bytes()
            except Exception:
                continue
    return None


class DiskManagerHandler(http.server.BaseHTTPRequestHandler):
    def log_message(self, format: str, *args: object) -> None:
        return

    def _send_bytes(self, raw: bytes, content_type: str, status: int = 200) -> None:
        self.send_response(status)
        self.send_header("Content-Type", content_type)
        self.send_header("Cache-Control", "no-store")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def _json(self, obj: dict, status: int = 200) -> None:
        self._send_bytes(json.dumps(obj).encode("utf-8"), "application/json; charset=utf-8", status)

    def do_GET(self) -> None:
        path = urllib.parse.urlsplit(self.path).path
        if path == "/api/disk/list":
            self._json({"ok": True, "is_root": is_root(), "disks": list_disks()})
            return
        if path in ("/disk_manager.css", "/disk_manager.js"):
            fname = path.lstrip("/")
            raw = load_static_asset(fname)
            if raw is not None:
                self._send_bytes(raw, STATIC_MIME[fname])
                return
            self._json({"ok": False, "error": "Asset not found"}, status=404)
            return
        raw_html = load_static_asset("disk_manager.html") or FALLBACK_HTML.encode("utf-8")
        self._send_bytes(raw_html, STATIC_MIME["disk_manager.html"])

    def do_POST(self) -> None:
        path = urllib.parse.urlsplit(self.path).path
        ln = int(self.headers.get("Content-Length", "0") or "0")
        body = json.loads(self.rfile.read(ln).decode("utf-8") or "{}") if ln > 0 else {}
        if path == "/api/disk/action":
            res = handle_disk_action(body, subiquity_driver=drive_subiquity_live_selection)
            self._json(res)
            if INSTALLER_MODE and body.get("action") == "select_target" and res.get("ok"):
                srv_ref = self.server
                threading.Timer(2.5, srv_ref.shutdown).start()
            return
        self._json({"ok": False, "error": "Not found"}, status=404)


class ReusableThreadingHTTPServer(http.server.ThreadingHTTPServer):
    allow_reuse_address = True


if __name__ == "__main__":
    srv = ReusableThreadingHTTPServer(("0.0.0.0", PORT), DiskManagerHandler)

    def _watch_subiquity_running() -> None:
        while True:
            time.sleep(3)
            code, st = _subiquity_req("GET", "/meta/status")
            if code == 200 and isinstance(st, dict) and st.get("state") in ("RUNNING", "POST_RUNNING", "DONE"):
                try:
                    srv.shutdown()
                except Exception:
                    pass
                break

    if INSTALLER_MODE:
        threading.Thread(target=_watch_subiquity_running, daemon=True).start()
    srv.serve_forever()
