#!/usr/bin/env python3
"""
dashboard_server.py
---------------------------
Standalone HTTP backend daemon for the Instant Server Web Dashboard (default port 8088).

Why this file is separate from laptop_hardening.sh:
- Keeping Python code in its own .py file makes both the shell script and the web server
  easy to read, syntax-check, and study for anyone learning Linux systems engineering.
"""

import http.server
import importlib
import json
import os
import pathlib
import re
import shutil
import subprocess
import threading
import time
import sys
import urllib.parse
sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))
sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent.parent / "os"))
import dashboard_files_api as fm_api
import dashboard_mc_api as mc_api
import dashboard_updates_api as upd_api
import dashboard_system_api as sys_api
try:
    dm_api = importlib.import_module("disk_manager")
except Exception:
    dm_api = None

ROOT_DIR = pathlib.Path("/opt/instant-server")
APPDATA_DIR = pathlib.Path("/DATA/AppData")
WORKFLOW_DIR = pathlib.Path("/DATA/CustomWorkflows")
LOG_DIR = pathlib.Path("/var/log/instant-server")
SETTINGS_FILE = ROOT_DIR / "server_settings.json"
SHORTCUTS_FILE = ROOT_DIR / "home_shortcuts.json"
PORT = int(os.environ.get("INSTANT_DASHBOARD_PORT", "8088"))

def append_cmd_log(entry: str) -> None:
    try:
        LOG_DIR.mkdir(parents=True, exist_ok=True)
        with (LOG_DIR / "commands.log").open("a", encoding="utf-8") as fh:
            fh.write(f"[{time.strftime('%Y-%m-%d %H:%M:%S')}] {entry}\n")
    except Exception:
        pass


def safe_app_dir(app_id: str) -> pathlib.Path:
    """Return a sanitized directory path under /DATA/AppData/<app_id>."""
    clean = "".join(c for c in app_id if c.isalnum() or c in ("_", "-")) or "default"
    target = (APPDATA_DIR / clean).resolve()
    APPDATA_DIR.mkdir(parents=True, exist_ok=True)
    target.mkdir(parents=True, exist_ok=True)
    return target


class ReusableThreadingHTTPServer(http.server.ThreadingHTTPServer):
    allow_reuse_address = True


class DashboardHandler(http.server.BaseHTTPRequestHandler):
    def log_message(self, format: str, *args: object) -> None:
        return

    def _send(self, obj, status=200):
        raw = json.dumps(obj).encode("utf-8")
        try:
            self.send_response(status)
            self.send_header("Content-Type", "application/json; charset=utf-8")
            self.send_header("Cache-Control", "no-store")
            self.send_header("Content-Length", str(len(raw)))
            self.end_headers()
            self.wfile.write(raw)
        except (BrokenPipeError, ConnectionResetError):
            pass

    def do_GET(self):
        parsed_url = urllib.parse.urlsplit(self.path)
        path = parsed_url.path
        params = dict(urllib.parse.parse_qsl(parsed_url.query))

        if path in ("/", "/index.html", "/server_dashboard.html"):
            for cand in (ROOT_DIR / "server_dashboard.html", ROOT_DIR / "ui" / "dashboard" / "server_dashboard.html", ROOT_DIR / "ui" / "server_dashboard.html"):
                if cand.is_file():
                    raw = cand.read_bytes()
                    try:
                        self.send_response(200)
                        self.send_header("Content-Type", "text/html; charset=utf-8")
                        self.send_header("Content-Length", str(len(raw)))
                        self.end_headers()
                        self.wfile.write(raw)
                    except (BrokenPipeError, ConnectionResetError):
                        pass
                    return

        if (path.endswith(".css") or path.endswith(".js")) and "dash_" in pathlib.Path(path).name:
            asset_name = pathlib.Path(path).name
            rel_sub = path.lstrip("/")
            for cand in (
                ROOT_DIR / rel_sub,
                ROOT_DIR / "ui" / "dashboard" / rel_sub,
                ROOT_DIR / asset_name,
                ROOT_DIR / "ui" / asset_name,
                ROOT_DIR / "ui" / "dashboard" / "styles" / asset_name,
                ROOT_DIR / "ui" / "dashboard" / "core" / asset_name,
                ROOT_DIR / "ui" / "dashboard" / "apps" / asset_name,
            ):
                if cand.is_file():
                    raw = cand.read_bytes()
                    ctype = "text/css; charset=utf-8" if asset_name.endswith(".css") else "application/javascript; charset=utf-8"
                    try:
                        self.send_response(200)
                        self.send_header("Content-Type", ctype)
                        self.send_header("Content-Length", str(len(raw)))
                        self.end_headers()
                        self.wfile.write(raw)
                    except (BrokenPipeError, ConnectionResetError):
                        pass
                    return

        if path == "/disk-manager" and dm_api is not None:
            raw = dm_api.DISK_UI_HTML.encode("utf-8")
            self.send_response(200)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.send_header("Content-Length", str(len(raw)))
            self.end_headers()
            self.wfile.write(raw)
            return

        if path == "/api/disk/list" and dm_api is not None:
            self._send({"ok": True, "is_root": dm_api.is_root(), "disks": dm_api.list_disks()})
            return

        if path == "/api/mc/status":
            self._send(mc_api.get_status())
            return

        if path == "/api/mc/console":
            self._send(mc_api.get_console_logs(int(params.get("lines", "120") or "120")))
            return

        if path == "/api/mc/addons":
            self._send(mc_api.list_addons(params.get("kind", "plugins")))
            return

        if path == "/api/mc/backups":
            self._send(mc_api.list_backups())
            return

        if path == "/api/updates/check":
            self._send(upd_api.check_all_updates())
            return

        if path == "/api/fm/disks":
            self._send({"ok": True, "disks": fm_api.get_mounted_disks()})
            return

        if path == "/api/fm/list":
            self._send(fm_api.list_directory(params.get("path", ""), params.get("label", "all"), params.get("q", ""), params.get("sort", "name")))
            return

        if path == "/api/fm/read":
            self._send(fm_api.read_file(params.get("path", "")))
            return

        if path == "/api/fm/download":
            fp = pathlib.Path(params.get("path", "")).resolve()
            if fp.is_file():
                raw = fp.read_bytes()
                self.send_response(200)
                self.send_header("Content-Type", "application/octet-stream")
                self.send_header("Content-Disposition", f'attachment; filename="{fp.name}"')
                self.send_header("Content-Length", str(len(raw)))
                self.end_headers()
                self.wfile.write(raw)
                return
            self._send({"ok": False, "error": "File not found"}, status=404)
            return

        if path == "/api/dashboard/apps/files":
            app_dir = safe_app_dir(params.get("app_id", "minecraft"))
            rel = params.get("file", "")
            if rel:
                fp = (app_dir / pathlib.Path(rel).name).resolve()
                content = fp.read_text(encoding="utf-8", errors="replace") if fp.is_file() else ""
                self._send({"ok": True, "file": fp.name, "content": content})
                return
            files = [
                {"name": p.name, "size": p.stat().st_size, "is_dir": p.is_dir()}
                for p in sorted(app_dir.iterdir())
            ]
            self._send({"ok": True, "path": str(app_dir), "files": files})
            return

        if path == "/api/dashboard/apps/compose":
            app_dir = safe_app_dir(params.get("app_id", "minecraft"))
            comp = app_dir / "docker-compose.yml"
            yaml_str = comp.read_text(encoding="utf-8", errors="replace") if comp.is_file() else ""
            self._send({"ok": True, "app_id": app_dir.name, "compose_yaml": yaml_str})
            return

        if path == "/api/dashboard/settings":
            cur = {}
            if SETTINGS_FILE.is_file():
                try:
                    cur = json.loads(SETTINGS_FILE.read_text(encoding="utf-8"))
                except Exception:
                    cur = {}
            self._send({"ok": True, "settings": cur})
            return

        if path == "/api/dashboard/shortcuts":
            cur = {"shortcuts": ["jellyfin", "nextcloud", "pihole", "immich"], "names": {}}
            if SHORTCUTS_FILE.is_file():
                try:
                    cur = json.loads(SHORTCUTS_FILE.read_text(encoding="utf-8"))
                except Exception:
                    pass
            self._send({"ok": True, **cur})
            return

        if path == "/api/files/disks":
            disks = []
            try:
                out = subprocess.check_output(["df", "-B1", "--output=source,size,used,avail,pcent,target"], text=True, stderr=subprocess.DEVNULL)
                for ln in out.splitlines()[1:]:
                    p = ln.split(None, 5)
                    if len(p) == 6 and (p[0].startswith("/dev/") or p[5] in ("/", "/DATA")):
                        disks.append({"device": p[0], "size": int(p[1]), "used": int(p[2]), "avail": int(p[3]), "pct": p[4], "mount": p[5]})
            except Exception:
                pass
            self._send({"ok": True, "disks": disks})
            return

        self._send({
            "ok": True,
            "status": "online",
            "apps_installing": (ROOT_DIR / ".apps_installing").is_file(),
        })

    def do_POST(self):
        path = urllib.parse.urlsplit(self.path).path
        length = int(self.headers.get("Content-Length", "0") or "0")
        payload = json.loads(self.rfile.read(length).decode("utf-8") or "{}") if length > 0 else {}

        if path.startswith("/api/mc/"):
            op = path[len("/api/mc/"):].strip("/")
            res = mc_api.handle_mc_post(op, payload)
            append_cmd_log(f"MC {op}: {payload.get('action') or payload.get('command', '')}")
            self._send(res, status=200 if res.get("ok") else 400)
            return

        if path == "/api/updates/install":
            res = upd_api.install_selected_updates(list(payload.get("ids") or []))
            append_cmd_log(f"UPDATES installed: {len(payload.get('ids') or [])}")
            self._send(res)
            return

        if path.startswith("/api/fm/"):
            op = path[len("/api/fm/"):].strip("/")
            res = fm_api.handle_fm_post(op, payload)
            append_cmd_log(f"FM {op}: {payload.get('path') or payload.get('src') or payload.get('filename', '')}")
            self._send(res, status=200 if res.get("ok") else 400)
            return

        if path == "/api/disk/action" and dm_api is not None:
            res = dm_api.handle_disk_action(payload)
            append_cmd_log(f"DISK {payload.get('action')}: {payload.get('device')}")
            self._send(res, status=200 if res.get("ok") else 400)
            return


        if path == "/api/dashboard/apps/files":
            app_dir = safe_app_dir(str(payload.get("app_id", "minecraft")))
            fname = pathlib.Path(str(payload.get("file", "server.properties"))).name
            target_file = app_dir / fname
            if target_file.is_file():
                shutil.copy2(target_file, str(target_file) + ".bak")
            target_file.write_text(str(payload.get("content", "")), encoding="utf-8", newline="\n")
            self._send({"ok": True, "saved": str(target_file)})
            return

        if path == "/api/dashboard/apps/rcon":
            cmd = str(payload.get("command", "list")).strip()
            res = subprocess.run(
                ["/usr/local/sbin/instant-mc-rcon"] + cmd.split(),
                capture_output=True,
                text=True,
            )
            self._send({"ok": res.returncode == 0, "output": (res.stdout + res.stderr).strip()})
            return

        if path == "/api/dashboard/apps/compose":
            app_dir = safe_app_dir(str(payload.get("app_id", "minecraft")))
            comp = app_dir / "docker-compose.yml"
            comp.write_text(str(payload.get("compose_yaml", "")), encoding="utf-8", newline="\n")
            subprocess.run(["docker", "compose", "-f", str(comp), "up", "-d"], capture_output=True)
            self._send({"ok": True})
            return

        if path == "/api/dashboard/apps/action":
            app_dir = safe_app_dir(str(payload.get("app_id", "minecraft")))
            op = str(payload.get("action", "restart")).lower()
            comp = app_dir / "docker-compose.yml"
            if comp.is_file():
                if op == "stop":
                    subprocess.run(["docker", "compose", "-f", str(comp), "stop"], capture_output=True)
                elif op in ("start", "install"):
                    subprocess.run(["docker", "compose", "-f", str(comp), "up", "-d"], capture_output=True)
                elif op in ("update", "pull"):
                    subprocess.run(["docker", "compose", "-f", str(comp), "pull"], capture_output=True)
                    subprocess.run(["docker", "compose", "-f", str(comp), "up", "-d"], capture_output=True)
                else:
                    subprocess.run(["docker", "compose", "-f", str(comp), "restart"], capture_output=True)
            self._send({"ok": True, "app_id": app_dir.name, "action": op})
            return

        if path == "/api/dashboard/apps/custom":
            raw = str(payload.get("raw_input", "")).strip()
            img = raw.split()[-1] if raw.startswith("docker run") and raw.split() else raw
            if ":" not in img:
                img += ":latest"
            m_port = re.search(r"(?:-p|--publish)\s+(?:[0-9.]+:)?(\d{2,5}):(\d{2,5})", raw)
            hp = int(m_port.group(1)) if m_port else 8080
            ip = int(m_port.group(2)) if m_port else hp
            slug = re.sub(r"[^a-z0-9_-]+", "-", img.split("/")[-1].split(":")[0].lower()).strip("-_") or "custom-app"
            app_id = "custom_" + slug.replace("-", "_")
            app_dir = safe_app_dir(app_id)
            comp_yaml = (
                f"name: {app_id}\n"
                f"services:\n"
                f"  {app_id}:\n"
                f"    image: {img}\n"
                f"    container_name: {app_id}\n"
                f"    restart: unless-stopped\n"
                f"    ports:\n"
                f"      - \"{hp}:{ip}\"\n"
                f"    volumes:\n"
                f"      - /DATA/AppData/{slug}:/data\n"
            )
            comp_file = app_dir / "docker-compose.yml"
            comp_file.write_text(comp_yaml, encoding="utf-8", newline="\n")
            subprocess.Popen(["docker", "compose", "-f", str(comp_file), "up", "-d"])
            self._send({
                "ok": True,
                "app": {
                    "id": app_id,
                    "title": slug.title(),
                    "image": img,
                    "port": hp,
                    "compose_yaml": comp_yaml,
                },
            })
            return

        if sys_api.handle_system_post(path, payload, self._send):
            return

        self._send({"ok": True})

if __name__ == "__main__":
    if PORT != 80:
        try:
            srv80 = ReusableThreadingHTTPServer(("0.0.0.0", 80), DashboardHandler)
            threading.Thread(target=srv80.serve_forever, daemon=True).start()
        except OSError:
            pass
    srv = ReusableThreadingHTTPServer(("0.0.0.0", PORT), DashboardHandler)
    srv.serve_forever()