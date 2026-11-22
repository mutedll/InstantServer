#!/usr/bin/env python3
"""
dashboard_updates_api.py
Official-Repository Update Center backend API (/api/updates/*).
Checks & installs updates selectively from official sources:
  1. Ubuntu OS Security & System Packages (official Ubuntu APT repositories)
  2. Instant Server Dashboard & Scripts (https://github.com/mutedll/InstantServer)
  3. Installed Docker Apps (official container registries via docker compose pull)
"""

import json
import pathlib
import subprocess
import time
import urllib.request

GITHUB_REPO = "mutedll/InstantServer"
GITHUB_RELEASE_API = f"https://api.github.com/repos/{GITHUB_REPO}/releases/latest"
GITHUB_RAW_BASE = f"https://raw.githubusercontent.com/{GITHUB_REPO}/main"
ROOT_DIR = pathlib.Path("/opt/instant-server")
APPDATA_DIR = pathlib.Path("/DATA/AppData")
VERSION_FILE = ROOT_DIR / ".dashboard_version"

DASHBOARD_SYNC_FILES = [
    ("src/instant_server/scripts/dashboard/dashboard_server.py", ROOT_DIR / "dashboard_server.py"),
    ("src/instant_server/scripts/dashboard/dashboard_system_api.py", ROOT_DIR / "dashboard_system_api.py"),
    ("src/instant_server/scripts/dashboard/dashboard_files_api.py", ROOT_DIR / "dashboard_files_api.py"),
    ("src/instant_server/scripts/dashboard/dashboard_mc_api.py", ROOT_DIR / "dashboard_mc_api.py"),
    ("src/instant_server/scripts/dashboard/dashboard_updates_api.py", ROOT_DIR / "dashboard_updates_api.py"),
    ("src/instant_server/scripts/os/disk_manager.py", ROOT_DIR / "disk_manager.py"),
    ("src/instant_server/scripts/os/disk_ops.py", ROOT_DIR / "disk_ops.py"),
    ("src/instant_server/ui/disk_manager/disk_manager.html", ROOT_DIR / "disk_manager.html"),
    ("src/instant_server/ui/disk_manager/disk_manager.css", ROOT_DIR / "disk_manager.css"),
    ("src/instant_server/ui/disk_manager/disk_manager.js", ROOT_DIR / "disk_manager.js"),
    ("src/instant_server/ui/dashboard/server_dashboard.html", ROOT_DIR / "server_dashboard.html"),
    ("src/instant_server/ui/dashboard/styles/dash_file_manager.css", ROOT_DIR / "styles" / "dash_file_manager.css"),
    ("src/instant_server/ui/dashboard/styles/dash_responsive.css", ROOT_DIR / "styles" / "dash_responsive.css"),
    ("src/instant_server/ui/dashboard/apps/dash_file_manager.js", ROOT_DIR / "apps" / "dash_file_manager.js"),
    ("src/instant_server/ui/dashboard/apps/dash_fm_events.js", ROOT_DIR / "apps" / "dash_fm_events.js"),
    ("src/instant_server/ui/dashboard/apps/dash_settings_app.js", ROOT_DIR / "apps" / "dash_settings_app.js"),
    ("src/instant_server/ui/dashboard/apps/dash_minecraft_app.js", ROOT_DIR / "apps" / "dash_minecraft_app.js"),
    ("src/instant_server/ui/dashboard/apps/dash_mc_events.js", ROOT_DIR / "apps" / "dash_mc_events.js"),
    ("src/instant_server/ui/dashboard/apps/dash_updates_center.js", ROOT_DIR / "apps" / "dash_updates_center.js"),
]


def _run(cmd: list[str], timeout: int = 90) -> tuple[bool, str]:
    try:
        res = subprocess.run(cmd, capture_output=True, text=True, timeout=timeout)
        return res.returncode == 0, (res.stdout + "\n" + res.stderr).strip()
    except Exception as exc:
        return False, str(exc)


def _check_github_dashboard() -> dict | None:
    local_ver = VERSION_FILE.read_text(encoding="utf-8").strip() if VERSION_FILE.is_file() else "installed"
    try:
        req = urllib.request.Request(GITHUB_RELEASE_API, headers={"User-Agent": "InstantServer-Updater"})
        with urllib.request.urlopen(req, timeout=8) as resp:
            data = json.loads(resp.read().decode("utf-8"))
            tag = str(data.get("tag_name") or "main")
            if tag != local_ver:
                return {
                    "id": "dashboard:instant_server",
                    "category": "dashboard",
                    "name": "Instant Server Dashboard & System Scripts",
                    "current": local_ver,
                    "available": tag,
                    "source": f"https://github.com/{GITHUB_REPO}",
                    "notes": str(data.get("name") or f"Official release {tag}"),
                }
    except Exception:
        return {
            "id": "dashboard:instant_server",
            "category": "dashboard",
            "name": "Instant Server Dashboard & System Scripts",
            "current": local_ver,
            "available": "latest (main)",
            "source": f"https://github.com/{GITHUB_REPO}",
            "notes": "Sync latest Dashboard & scripts from official mutedll/InstantServer repo",
        }
    return None


def _check_apt_updates() -> list[dict]:
    items = []
    _run(["apt-get", "update", "-qq"], timeout=60)
    ok, out = _run(["apt", "list", "--upgradable"], timeout=30)
    if not ok:
        return items
    for ln in out.splitlines():
        if "/" not in ln or ln.startswith("Listing"):
            continue
        pkg = ln.split("/", 1)[0].strip()
        parts = ln.split()
        avail = parts[1] if len(parts) > 1 else "latest"
        items.append({
            "id": f"os:{pkg}",
            "category": "os",
            "name": pkg,
            "current": "installed",
            "available": avail,
            "source": "Official Ubuntu APT Repo",
            "notes": "Ubuntu OS system/security package",
        })
        if len(items) >= 35:
            break
    return items


def _check_docker_apps() -> list[dict]:
    items = []
    if not APPDATA_DIR.is_dir():
        return items
    for comp in sorted(APPDATA_DIR.glob("*/docker-compose.yml")):
        app_id = comp.parent.name
        img = "official registry image"
        for ln in comp.read_text(encoding="utf-8", errors="replace").splitlines():
            if ln.strip().startswith("image:"):
                img = ln.split("image:", 1)[1].strip().strip("\"'")
                break
        items.append({
            "id": f"app:{app_id}",
            "category": "app",
            "name": app_id.replace("_", " ").title(),
            "current": "local image",
            "available": "latest registry digest",
            "source": img,
            "notes": "Pull latest official container image & recreate",
        })
    return items


def check_all_updates() -> dict:
    updates = []
    dash_upd = _check_github_dashboard()
    if dash_upd:
        updates.append(dash_upd)
    updates.extend(_check_docker_apps())
    updates.extend(_check_apt_updates())
    return {
        "ok": True,
        "checked_at": time.strftime("%Y-%m-%d %H:%M:%S"),
        "repo": f"https://github.com/{GITHUB_REPO}",
        "updates": updates,
    }


def install_selected_updates(selected_ids: list[str]) -> dict:
    results = []
    os_pkgs = [x[3:] for x in selected_ids if x.startswith("os:")]
    app_ids = [x[4:] for x in selected_ids if x.startswith("app:")]
    do_dash = "dashboard:instant_server" in selected_ids

    if do_dash:
        synced = 0
        for rel_src, dst_path in DASHBOARD_SYNC_FILES:
            url = f"{GITHUB_RAW_BASE}/{rel_src}"
            try:
                req = urllib.request.Request(url, headers={"User-Agent": "InstantServer-Updater"})
                with urllib.request.urlopen(req, timeout=12) as resp:
                    content = resp.read()
                dst_path.parent.mkdir(parents=True, exist_ok=True)
                dst_path.write_bytes(content)
                synced += 1
            except Exception:
                continue
        if synced > 0:
            VERSION_FILE.write_text(time.strftime("main-%Y%m%d"), encoding="utf-8")
        results.append(f"Dashboard: synced {synced} file(s) from github.com/{GITHUB_REPO}")

    for aid in app_ids:
        comp = APPDATA_DIR / aid / "docker-compose.yml"
        if comp.is_file():
            ok1, _ = _run(["docker", "compose", "-f", str(comp), "pull"], timeout=180)
            ok2, _ = _run(["docker", "compose", "-f", str(comp), "up", "-d"], timeout=120)
            results.append(f"App {aid}: {'updated & restarted' if (ok1 and ok2) else 'pull/up attempted'}")

    if os_pkgs:
        env_cmd = ["env", "DEBIAN_FRONTEND=noninteractive", "apt-get", "install", "-y", "--only-upgrade"] + os_pkgs
        ok, out = _run(env_cmd, timeout=300)
        results.append(f"Ubuntu OS ({len(os_pkgs)} pkg): {'upgraded' if ok else out[:160]}")

    return {"ok": True, "installed_count": len(selected_ids), "results": results}