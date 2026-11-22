#!/usr/bin/env python3
"""
dashboard_system_api.py
-----------------------
System, Settings, Tailscale, Self-Heal, Kiwix, Cluster, Terminal, System Files,
and Custom Workflow POST handlers for dashboard_server.py.
"""

import json
import os
import pathlib
import re
import shutil
import subprocess
import time
from typing import Any, Callable

ROOT_DIR = pathlib.Path("/opt/instant-server")
APPDATA_DIR = pathlib.Path("/DATA/AppData")
WORKFLOW_DIR = pathlib.Path("/DATA/CustomWorkflows")
LOG_DIR = pathlib.Path("/var/log/instant-server")
SETTINGS_FILE = ROOT_DIR / "server_settings.json"
SHORTCUTS_FILE = ROOT_DIR / "home_shortcuts.json"


def append_cmd_log(entry: str) -> None:
    try:
        LOG_DIR.mkdir(parents=True, exist_ok=True)
        with (LOG_DIR / "commands.log").open("a", encoding="utf-8") as fh:
            fh.write(f"[{time.strftime('%Y-%m-%d %H:%M:%S')}] {entry}\n")
    except Exception:
        pass


def handle_system_post(path: str, payload: dict[str, Any], send_fn: Callable[..., None]) -> bool:
    if path == "/api/dashboard/shortcuts":
        ROOT_DIR.mkdir(parents=True, exist_ok=True)
        SHORTCUTS_FILE.write_text(json.dumps(payload, indent=2), encoding="utf-8", newline="\n")
        append_cmd_log("Updated Home Launcher app shortcuts")
        send_fn({"ok": True})
        return True

    if path == "/api/dashboard/settings":
        settings_file = SETTINGS_FILE
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
        if "log_retention_days" in payload:
            days = max(1, min(365, int(payload.get("log_retention_days") or 14)))
            lr = pathlib.Path("/etc/logrotate.d/instant-server")
            try:
                lr.write_text(f"/var/log/instant-server/*.log {{\n    daily\n    rotate {days}\n    missingok\n    notifempty\n    compress\n    delaycompress\n    copytruncate\n    dateext\n}}\n", encoding="utf-8", newline="\n")
            except Exception:
                pass
        if "address_host" in payload and str(payload.get("address_host", "")).strip():
            nh = re.sub(r"[^a-zA-Z0-9-]", "", str(payload["address_host"]).strip())
            if nh:
                subprocess.run(["hostnamectl", "set-hostname", nh], capture_output=True)
        if "wifi_ssid" in payload and str(payload.get("wifi_ssid", "")).strip():
            ssid = str(payload["wifi_ssid"]).strip()
            wpass = str(payload.get("wifi_password", ""))
            np_file = pathlib.Path("/etc/netplan/60-instant-wifi.yaml")
            try:
                auth = f'\n          password: "{wpass}"' if wpass else " {}"
                np_file.write_text(f'network:\n  version: 2\n  wifis:\n    wlan0:\n      dhcp4: true\n      optional: true\n      access-points:\n        "{ssid}":{auth}\n', encoding="utf-8", newline="\n")
                os.chmod(np_file, 0o600)
            except Exception:
                pass
        append_cmd_log(f"Settings updated: {list(payload.keys())}")
        subprocess.run(["/usr/local/sbin/instant-server-banner-refresh"], capture_output=True)
        send_fn({"ok": True, "settings": cur})
        return True

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
        send_fn({"ok": True, "tailscale_ip": ts_ip})
        return True

    if path in ("/api/dashboard/updates/apply", "/api/system/update"):
        mode = str(payload.get("mode", "auto"))
        res = subprocess.run(
            ["/usr/local/sbin/instant-server-updater", mode],
            capture_output=True,
            text=True,
        )
        send_fn({"ok": res.returncode == 0, "output": (res.stdout + res.stderr).strip()})
        return True

    if path == "/api/self-heal/run":
        subprocess.run(["/usr/local/sbin/instant-server-maintenance"], capture_output=True)
        subprocess.run(["/usr/local/sbin/instant-server-netwatch"], capture_output=True)
        send_fn({"ok": True, "healed": True})
        return True

    if path == "/api/dashboard/kiwix":
        zim_dir = APPDATA_DIR / "kiwix" / "zim"
        zim_dir.mkdir(parents=True, exist_ok=True)
        send_fn({"ok": True, "zim_dir": str(zim_dir)})
        return True

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
        send_fn({"ok": res.returncode == 0, "output": res.stdout.strip()})
        return True

    if path == "/api/terminal/exec":
        cmd_str = str(payload.get("command", "uptime")).strip()
        append_cmd_log(f"TERMINAL: {cmd_str}")
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
        send_fn({
            "ok": res.returncode == 0,
            "exit_code": res.returncode,
            "output": (res.stdout + res.stderr).strip(),
            "cwd": cwd,
        })
        return True

    if path == "/api/system/files":
        action = str(payload.get("action", "list"))
        target_path = pathlib.Path(str(payload.get("path", "/DATA"))).resolve()
        is_sys = not str(target_path).startswith("/DATA")
        if action == "read" and target_path.is_file():
            content = target_path.read_text(encoding="utf-8", errors="replace")
            send_fn({"ok": True, "path": str(target_path), "is_system_critical": is_sys, "content": content})
            return True
        if action == "restore_bak":
            bak = pathlib.Path(str(target_path) + ".bak")
            if bak.is_file():
                shutil.copy2(bak, target_path)
                send_fn({"ok": True, "restored": str(target_path)})
                return True
            send_fn({"ok": False, "error": "No .bak file found"}, status=404)
            return True
        if action == "write":
            if is_sys and not (payload.get("confirm_system_write") or payload.get("confirmed_system_edit")):
                send_fn({"ok": False, "error": "System file modification requires explicit confirmation"}, status=400)
                return True
            if target_path.is_file():
                shutil.copy2(target_path, str(target_path) + ".bak")
            target_path.parent.mkdir(parents=True, exist_ok=True)
            target_path.write_text(str(payload.get("content", "")), encoding="utf-8", newline="\n")
            send_fn({"ok": True, "saved": str(target_path), "backup": str(target_path) + ".bak"})
            return True
        entries = []
        if target_path.is_dir():
            for p in sorted(target_path.iterdir()):
                entries.append({
                    "name": p.name,
                    "path": str(p),
                    "is_dir": p.is_dir(),
                    "size": p.stat().st_size if p.is_file() else 0,
                })
        send_fn({"ok": True, "path": str(target_path), "is_system_critical": is_sys, "entries": entries})
        return True

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
        send_fn({"ok": True, "path": str(wf_file), "output": output})
        return True

    return False