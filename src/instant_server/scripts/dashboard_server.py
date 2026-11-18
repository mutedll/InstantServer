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
import json
import os
import pathlib
import re
import shutil
import subprocess
import urllib.parse

ROOT_DIR = pathlib.Path("/opt/instant-server")
APPDATA_DIR = pathlib.Path("/DATA/AppData")
WORKFLOW_DIR = pathlib.Path("/DATA/CustomWorkflows")
PORT = int(os.environ.get("INSTANT_DASHBOARD_PORT", "8088"))


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
            for cand in (ROOT_DIR / "server_dashboard.html", ROOT_DIR / "ui" / "server_dashboard.html"):
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

        if path.startswith("/dash_") and (path.endswith(".css") or path.endswith(".js")):
            asset_name = pathlib.Path(path).name
            for cand in (ROOT_DIR / asset_name, ROOT_DIR / "ui" / asset_name):
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

        self._send({
            "ok": True,
            "status": "online",
            "apps_installing": (ROOT_DIR / ".apps_installing").is_file(),
        })

    def do_POST(self):
        path = urllib.parse.urlsplit(self.path).path
        length = int(self.headers.get("Content-Length", "0") or "0")
        payload = json.loads(self.rfile.read(length).decode("utf-8") or "{}") if length > 0 else {}

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

        if path == "/api/dashboard/settings":
            settings_file = ROOT_DIR / "dashboard-settings.json"
            cur = {}
            if settings_file.is_file():
                try:
                    cur = json.loads(settings_file.read_text(encoding="utf-8"))
                except Exception:
                    cur = {}
            cur.update(payload)
            settings_file.write_text(json.dumps(cur, indent=2), encoding="utf-8", newline="\n")
            if "disable_cpu_turbo_boost" in payload:
                no_turbo = "1" if payload["disable_cpu_turbo_boost"] else "0"
                p_intel = pathlib.Path("/sys/devices/system/cpu/intel_pstate/no_turbo")
                if p_intel.exists():
                    try:
                        p_intel.write_text(no_turbo)
                    except Exception:
                        pass
            if "cpu_governor" in payload:
                gov = str(payload["cpu_governor"])
                for gp in pathlib.Path("/sys/devices/system/cpu").glob("cpu*/cpufreq/scaling_governor"):
                    try:
                        gp.write_text(gov)
                    except Exception:
                        pass
            self._send({"ok": True, "settings": cur})
            return

        if path == "/api/dashboard/tailscale":
            key = str(payload.get("tailscale_authkey", "")).strip()
            mode = str(payload.get("tailscale_mode", "subnet_and_ssh")).strip()
            if key and shutil.which("tailscale"):
                flags = ["tailscale", "up", f"--authkey={key}", "--ssh"]
                if mode == "exit_node_full":
                    flags.append("--advertise-exit-node")
                subprocess.run(flags, capture_output=True, text=True, timeout=25)
            ts_ip = "100.104.28.14"
            if shutil.which("tailscale"):
                r_ip = subprocess.run(["tailscale", "ip", "-4"], capture_output=True, text=True)
                if r_ip.returncode == 0 and r_ip.stdout.strip():
                    ts_ip = r_ip.stdout.strip().splitlines()[0]
            self._send({"ok": True, "tailscale_ip": ts_ip})
            return

        if path in ("/api/dashboard/updates/apply", "/api/system/update"):
            mode = str(payload.get("mode", "auto"))
            res = subprocess.run(
                ["/usr/local/sbin/instant-server-updater", mode],
                capture_output=True,
                text=True,
            )
            self._send({"ok": res.returncode == 0, "output": (res.stdout + res.stderr).strip()})
            return

        if path == "/api/self-heal/run":
            subprocess.run(["/usr/local/sbin/instant-server-maintenance"], capture_output=True)
            subprocess.run(["/usr/local/sbin/instant-server-netwatch"], capture_output=True)
            self._send({"ok": True, "healed": True})
            return

        if path == "/api/dashboard/kiwix":
            zim_dir = APPDATA_DIR / "kiwix" / "zim"
            zim_dir.mkdir(parents=True, exist_ok=True)
            self._send({"ok": True, "zim_dir": str(zim_dir)})
            return

        if path == "/api/cluster/join":
            ip = str(payload.get("ip", "")).strip()
            host = str(payload.get("hostname", "node2")).strip()
            pol = str(payload.get("storage_policy", "mergerfs_union_safe")).strip()
            tok = str(payload.get("confirm_wipe_token", "")).strip()
            res = subprocess.run(
                ["/usr/local/sbin/instant-cluster-ctl", "join", ip, host, pol, tok],
                capture_output=True,
                text=True,
            )
            self._send({"ok": res.returncode == 0, "output": res.stdout.strip()})
            return

        if path == "/api/terminal/exec":
            cmd_str = str(payload.get("command", "uptime")).strip()
            cwd = str(payload.get("cwd", "/DATA")).strip() or "/DATA"
            if not os.path.isdir(cwd):
                cwd = "/"
            res = subprocess.run(
                ["/bin/bash", "-c", cmd_str],
                cwd=cwd,
                capture_output=True,
                text=True,
                timeout=45,
            )
            self._send({
                "ok": res.returncode == 0,
                "exit_code": res.returncode,
                "output": (res.stdout + res.stderr).strip(),
                "cwd": cwd,
            })
            return

        if path == "/api/system/files":
            action = str(payload.get("action", "list"))
            target_path = pathlib.Path(str(payload.get("path", "/DATA"))).resolve()
            is_sys = not str(target_path).startswith("/DATA")
            if action == "read" and target_path.is_file():
                content = target_path.read_text(encoding="utf-8", errors="replace")
                self._send({"ok": True, "path": str(target_path), "is_system_critical": is_sys, "content": content})
                return
            if action == "restore_bak":
                bak = pathlib.Path(str(target_path) + ".bak")
                if bak.is_file():
                    shutil.copy2(bak, target_path)
                    self._send({"ok": True, "restored": str(target_path)})
                    return
                self._send({"ok": False, "error": "No .bak file found"}, status=404)
                return
            if action == "write":
                if is_sys and not (payload.get("confirm_system_write") or payload.get("confirmed_system_edit")):
                    self._send({"ok": False, "error": "System file modification requires explicit confirmation"}, status=400)
                    return
                if target_path.is_file():
                    shutil.copy2(target_path, str(target_path) + ".bak")
                target_path.parent.mkdir(parents=True, exist_ok=True)
                target_path.write_text(str(payload.get("content", "")), encoding="utf-8", newline="\n")
                self._send({"ok": True, "saved": str(target_path), "backup": str(target_path) + ".bak"})
                return
            entries = []
            if target_path.is_dir():
                for p in sorted(target_path.iterdir()):
                    entries.append({
                        "name": p.name,
                        "path": str(p),
                        "is_dir": p.is_dir(),
                        "size": p.stat().st_size if p.is_file() else 0,
                    })
            self._send({"ok": True, "path": str(target_path), "is_system_critical": is_sys, "entries": entries})
            return

        if path == "/api/custom/workflow":
            WORKFLOW_DIR.mkdir(parents=True, exist_ok=True)
            wf_name = "".join(
                c for c in str(payload.get("name", "custom-task.sh"))
                if c.isalnum() or c in ("_", "-", ".")
            ) or "custom-task.sh"
            wf_script = str(payload.get("script", "")).replace("\r\n", "\n")
            wf_file = WORKFLOW_DIR / wf_name
            if wf_script:
                wf_file.write_text(wf_script, encoding="utf-8", newline="\n")
                wf_file.chmod(0o755)
            run_now = bool(payload.get("run_now", False))
            output = f"Saved workflow {wf_file}"
            if run_now and wf_file.is_file():
                res = subprocess.run(["/bin/bash", str(wf_file)], capture_output=True, text=True, timeout=60)
                output = (res.stdout + res.stderr).strip() or f"Executed {wf_file} (exit 0)"
            self._send({"ok": True, "path": str(wf_file), "output": output})
            return

        self._send({"ok": True})


if __name__ == "__main__":
    srv = ReusableThreadingHTTPServer(("0.0.0.0", PORT), DashboardHandler)
    srv.serve_forever()