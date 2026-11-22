#!/usr/bin/env python3
"""
disk_manager.py
Windows Disk Management–grade Partition & Disk Selector for Instant Server.
Runs as root (with sudo/root privilege verification) during Live USB boot
(port 8088) and from the running Server Dashboard.

Features:
- Visual disk & partition map (proportional allocated + unallocated bars)
- Initialize / Convert Disk (GPT or MBR)
- Clean / Wipe Disk (wipefs + sgdisk --zap-all)
- Create New Simple Volume from unallocated space (ext4/ntfs/fat32/exfat/btrfs/xfs + label)
- Format Partition, Shrink / Split Partition, Extend Partition, Delete Partition, Relabel
- Validation-gated "Select as Install Target" button (greyed out with exact reason
  if <15 GB, protected installer USB, read-only, or invalid)
"""

import http.server
import json
import os
import pathlib
import re
import shutil
import subprocess
import sys
import urllib.parse
import http.client
import socket
import threading
import time


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
    """If Subiquity native Disk Selector is currently WAITING, apply DIRECT full-disk layout & start install."""
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


import sys
sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))
from disk_ops import (
    PORT, MIN_TARGET_BYTES, USB_EXCLUDE, PICK_OUT, INSTALLER_MODE,
    is_root, run_cmd, valid_dev_path, is_protected_usb, get_free_gaps,
    evaluate_target, list_disks, unmount_device_tree, refresh_kernel_pt,
    format_partition_fs, handle_disk_action,
)
from disk_manager_ui import DISK_UI_HTML

class DiskManagerHandler(http.server.BaseHTTPRequestHandler):
    def log_message(self, format: str, *args: object) -> None:
        return

    def _json(self, obj: dict, status: int = 200) -> None:
        raw = json.dumps(obj).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Cache-Control", "no-store")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def do_GET(self) -> None:
        path = urllib.parse.urlsplit(self.path).path
        if path == "/api/disk/list":
            self._json({"ok": True, "is_root": is_root(), "disks": list_disks()})
            return
        raw = DISK_UI_HTML.encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "text/html; charset=utf-8")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def do_POST(self) -> None:
        path = urllib.parse.urlsplit(self.path).path
        ln = int(self.headers.get("Content-Length", "0") or "0")
        body = json.loads(self.rfile.read(ln).decode("utf-8") or "{}") if ln > 0 else {}
        if path == "/api/disk/action":
            self._json(handle_disk_action(body))
            return
        self._json({"ok": False, "error": "Not found"}, status=404)


if __name__ == "__main__":
    srv = http.server.ThreadingHTTPServer(("0.0.0.0", PORT), DiskManagerHandler)
    def _watch_subiquity_running():
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