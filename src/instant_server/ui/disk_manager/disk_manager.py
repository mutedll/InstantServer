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
sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))
from disk_ops import (
    PORT, MIN_TARGET_BYTES, USB_EXCLUDE, PICK_OUT, INSTALLER_MODE,
    is_root, run_cmd, valid_dev_path, is_protected_usb, get_free_gaps,
    evaluate_target, list_disks, unmount_device_tree, refresh_kernel_pt,
    format_partition_fs, handle_disk_action,
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
    "<!DOCTYPE html><html><head><meta charset='utf-8'><meta name='viewport' content='width=device-width,initial-scale=1'>"
    "<title>Disk Management</title></head><body style='background:#0b0e14;color:#f1f5f9;font-family:sans-serif;padding:24px'>"
    "<h2>Disk Management</h2><p>Static UI assets (disk_manager.html) not found alongside disk_manager.py.</p></body></html>"
)


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
            # After confirming target disk in installer mode, wait 2.5s so the browser
            # receives the 200 OK response and renders the "Installation Started" handoff screen.
            if INSTALLER_MODE and body.get("action") == "select_target" and res.get("ok"):
                srv_ref = self.server
                threading.Timer(2.5, srv_ref.shutdown).start()
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