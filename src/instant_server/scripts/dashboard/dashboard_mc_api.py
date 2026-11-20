#!/usr/bin/env python3
"""
dashboard_mc_api.py
Dedicated Pro-Level Minecraft Server Control Panel Backend API (/api/mc/*).
Matches paid Minecraft hosting panels (Pterodactyl, Apex, BisectHosting, Shockbyte):
- Power controls (start, stop, restart, kill, save-all) & live console/RCON
- Full 60+ key server.properties parser & saver with .bak safety
- Server software (Paper/Purpur/Spigot/Vanilla/Fabric/Forge/NeoForge/Bedrock),
  version, RAM heap, Aikar/ZGC JVM flags, and GeyserMC crossplay configurator
- Players manager: OPs (ops.json), Whitelist (whitelist.json), Bans & IP Bans
- Plugins, Mods & Datapacks manager: upload .jar/.zip, enable/disable, delete, URL install
- Worlds & RCON-coordinated backups: create, restore, delete, upload world .zip, reset
- Scheduled tasks persistence (auto-backup, auto-restart, RCON broadcasts)
"""

import base64
import json
import pathlib
import re
import shutil
import subprocess
import time
import urllib.request
import zipfile

MC_DIR = pathlib.Path("/DATA/AppData/minecraft")
MC_DATA = MC_DIR / "data"
MC_COMPOSE = MC_DIR / "docker-compose.yml"
MC_BACKUPS = pathlib.Path("/DATA/Backups/minecraft")
MC_SCHEDULES = MC_DIR / "schedules.json"


def _run(cmd: list[str], timeout: int = 45) -> tuple[bool, str]:
    try:
        res = subprocess.run(cmd, capture_output=True, text=True, timeout=timeout)
        return res.returncode == 0, (res.stdout + "\n" + res.stderr).strip()
    except Exception as exc:
        return False, str(exc)


def rcon_exec(command: str) -> tuple[bool, str]:
    cmd = command.strip()
    if not cmd:
        return False, "Empty command"
    if pathlib.Path("/usr/local/sbin/instant-mc-rcon").is_file():
        return _run(["/usr/local/sbin/instant-mc-rcon"] + cmd.split(), timeout=20)
    return _run(["docker", "exec", "minecraft", "rcon-cli", cmd], timeout=20)


def parse_properties() -> dict[str, str]:
    props_file = MC_DATA / "server.properties"
    out: dict[str, str] = {}
    if not props_file.is_file():
        return out
    for line in props_file.read_text(encoding="utf-8", errors="replace").splitlines():
        s = line.strip()
        if not s or s.startswith("#") or "=" not in s:
            continue
        k, v = s.split("=", 1)
        out[k.strip()] = v.strip()
    return out


def save_properties(updates: dict, raw_text: str | None = None, restart: bool = False) -> dict:
    MC_DATA.mkdir(parents=True, exist_ok=True)
    props_file = MC_DATA / "server.properties"
    if props_file.is_file():
        shutil.copy2(props_file, str(props_file) + ".bak")
    if raw_text is not None:
        props_file.write_text(raw_text.replace("\r\n", "\n"), encoding="utf-8", newline="\n")
    else:
        cur = parse_properties()
        for k, v in updates.items():
            clean_k = re.sub(r"[^a-zA-Z0-9._-]", "", str(k))
            if clean_k:
                cur[clean_k] = str(v)
        lines = ["# Managed by Instant Server Minecraft Pro Control Panel"]
        for k in sorted(cur.keys()):
            lines.append(f"{k}={cur[k]}")
        props_file.write_text("\n".join(lines) + "\n", encoding="utf-8", newline="\n")
    if restart and MC_COMPOSE.is_file():
        _run(["docker", "compose", "-f", str(MC_COMPOSE), "restart"], timeout=60)
    return {"ok": True, "saved": str(props_file), "restarted": restart}


def read_json_list(name: str) -> list:
    fp = MC_DATA / name
    if not fp.is_file():
        return []
    try:
        data = json.loads(fp.read_text(encoding="utf-8"))
        return data if isinstance(data, list) else []
    except Exception:
        return []


def write_json_list(name: str, items: list) -> None:
    MC_DATA.mkdir(parents=True, exist_ok=True)
    (MC_DATA / name).write_text(json.dumps(items, indent=2), encoding="utf-8", newline="\n")


def get_status() -> dict:
    ok_ps, ps_out = _run(["docker", "inspect", "-f", "{{.State.Status}}", "minecraft"], timeout=10)
    status = ps_out.strip() if ok_ps and ps_out.strip() else "stopped"
    ok_stats, st_out = _run(["docker", "stats", "--no-stream", "--format", "{{.CPUPerc}}|{{.MemUsage}}", "minecraft"], timeout=10)
    cpu, mem = ("0%", "0 MiB")
    if ok_stats and "|" in st_out:
        cpu, mem = st_out.split("|", 1)
    online_str = ""
    if status == "running":
        _, online_str = rcon_exec("list")
    return {
        "ok": True,
        "status": status,
        "cpu": cpu.strip(),
        "memory": mem.strip(),
        "online_info": online_str.strip(),
        "properties": parse_properties(),
        "ops": read_json_list("ops.json"),
        "whitelist": read_json_list("whitelist.json"),
        "banned_players": read_json_list("banned-players.json"),
        "banned_ips": read_json_list("banned-ips.json"),
        "schedules": read_json_list("../schedules.json") if MC_SCHEDULES.is_file() else [],
    }


def get_console_logs(lines: int = 120) -> dict:
    ok, out = _run(["docker", "logs", "--tail", str(min(500, max(20, lines))), "minecraft"], timeout=15)
    return {"ok": ok, "logs": out}


def list_addons(kind: str) -> dict:
    folder_map = {
        "plugins": MC_DATA / "plugins",
        "mods": MC_DATA / "mods",
        "datapacks": MC_DATA / (parse_properties().get("level-name", "world")) / "datapacks",
    }
    target = folder_map.get(kind, MC_DATA / "plugins")
    target.mkdir(parents=True, exist_ok=True)
    items = []
    for p in sorted(target.iterdir()):
        if p.is_file():
            items.append({
                "name": p.name,
                "size": p.stat().st_size,
                "enabled": not p.name.endswith(".disabled"),
                "mtime": time.strftime("%Y-%m-%d %H:%M", time.localtime(p.stat().st_mtime)),
            })
    return {"ok": True, "kind": kind, "path": str(target), "items": items}


def list_backups() -> dict:
    MC_BACKUPS.mkdir(parents=True, exist_ok=True)
    items = []
    for p in sorted(MC_BACKUPS.iterdir(), key=lambda x: x.stat().st_mtime, reverse=True):
        if p.is_file():
            items.append({
                "name": p.name,
                "size": p.stat().st_size,
                "mtime": time.strftime("%Y-%m-%d %H:%M", time.localtime(p.stat().st_mtime)),
            })
    return {"ok": True, "path": str(MC_BACKUPS), "backups": items}


def handle_mc_post(op: str, payload: dict) -> dict:
    if op == "power":
        act = str(payload.get("action", "restart")).lower()
        if act == "save-all":
            ok, out = rcon_exec("save-all flush")
            return {"ok": ok, "output": out or "World saved."}
        if act == "kill":
            ok, out = _run(["docker", "kill", "minecraft"], timeout=20)
            return {"ok": ok, "output": out}
        if not MC_COMPOSE.is_file():
            return {"ok": False, "error": "Minecraft docker-compose.yml not found yet."}
        cmd_map = {"start": ["up", "-d"], "stop": ["stop"], "restart": ["restart"]}
        ok, out = _run(["docker", "compose", "-f", str(MC_COMPOSE)] + cmd_map.get(act, ["restart"]), timeout=90)
        return {"ok": ok, "output": out or f"Minecraft {act} complete."}

    if op == "rcon":
        ok, out = rcon_exec(str(payload.get("command", "")))
        return {"ok": ok, "output": out}

    if op == "properties":
        return save_properties(
            payload.get("properties") or {},
            raw_text=payload.get("raw_text"),
            restart=bool(payload.get("restart", False)),
        )

    if op == "software":
        sw = re.sub(r"[^A-Z0-9_]", "", str(payload.get("type", "PAPER")).upper()) or "PAPER"
        ver = re.sub(r"[^A-Za-z0-9._-]", "", str(payload.get("version", "LATEST"))) or "LATEST"
        mem = re.sub(r"[^0-9GMgm]", "", str(payload.get("memory", "4G")).upper()) or "4G"
        jvm = str(payload.get("jvm_flags", "aikar")).lower()
        geyser = bool(payload.get("geyser_bedrock", False))
        port = int(payload.get("port", 25565) or 25565)
        eula = "TRUE" if payload.get("eula", True) else "FALSE"
        MC_DIR.mkdir(parents=True, exist_ok=True)
        MC_DATA.mkdir(parents=True, exist_ok=True)
        flags_line = "      USE_AIKAR_FLAGS: \"true\"\n" if jvm == "aikar" else ("      USE_MEOWICE_FLAGS: \"true\"\n" if jvm == "zgc" else "")
        geyser_port = "      - \"19132:19132/udp\"\n" if geyser else ""
        geyser_mod = "      MODRINTH_PROJECTS: \"geyser,floodgate\"\n" if geyser else ""
        yml = (
            "name: minecraft\nservices:\n  minecraft:\n"
            "    image: itzg/minecraft-server:latest\n    container_name: minecraft\n"
            "    restart: unless-stopped\n    tty: true\n    stdin_open: true\n"
            f"    ports:\n      - \"{port}:25565\"\n      - \"25575:25575\"\n{geyser_port}"
            f"    environment:\n      EULA: \"{eula}\"\n      TYPE: \"{sw}\"\n"
            f"      VERSION: \"{ver}\"\n      MEMORY: \"{mem}\"\n      ENABLE_RCON: \"true\"\n"
            f"{flags_line}{geyser_mod}"
            "    volumes:\n      - /DATA/AppData/minecraft/data:/data\n"
        )
        MC_COMPOSE.write_text(yml, encoding="utf-8", newline="\n")
        ok, out = _run(["docker", "compose", "-f", str(MC_COMPOSE), "up", "-d"], timeout=120)
        return {"ok": ok, "output": out or f"Configured {sw} {ver} ({mem}) and restarted container."}

    if op == "players":
        act = str(payload.get("action", ""))
        name = re.sub(r"[^A-Za-z0-9_.-]", "", str(payload.get("player", "")).strip())
        if not name and act not in ("whitelist_on", "whitelist_off"):
            return {"ok": False, "error": "Enter a valid Minecraft username or IP."}
        rcon_map = {
            "op": f"op {name}",
            "deop": f"deop {name}",
            "whitelist_add": f"whitelist add {name}",
            "whitelist_remove": f"whitelist remove {name}",
            "whitelist_on": "whitelist on",
            "whitelist_off": "whitelist off",
            "ban": f"ban {name} {payload.get('reason', 'Banned by admin')}",
            "pardon": f"pardon {name}",
            "ban_ip": f"ban-ip {name}",
            "pardon_ip": f"pardon-ip {name}",
            "kick": f"kick {name} {payload.get('reason', 'Kicked by admin')}",
        }
        cmd = rcon_map.get(act)
        if not cmd:
            return {"ok": False, "error": f"Unknown player action: {act}"}
        ok, out = rcon_exec(cmd)
        return {"ok": ok, "output": out or f"Executed: {cmd}"}

    if op == "addons":
        act = str(payload.get("action", ""))
        kind = str(payload.get("kind", "plugins"))
        folder = pathlib.Path(list_addons(kind)["path"])
        if act == "upload":
            fname = pathlib.Path(str(payload.get("filename", ""))).name
            b64 = str(payload.get("data_base64", ""))
            if not fname or not b64:
                return {"ok": False, "error": "Missing file name or data."}
            (folder / fname).write_bytes(base64.b64decode(b64))
            return {"ok": True, "output": f"Uploaded {fname} to {kind}/"}
        if act == "url_install":
            url = str(payload.get("url", "")).strip()
            if not url.startswith(("https://", "http://")):
                return {"ok": False, "error": "Provide a valid http(s) direct .jar/.zip URL."}
            fname = pathlib.Path(url.split("?")[0]).name or f"addon_{int(time.time())}.jar"
            urllib.request.urlretrieve(url, str(folder / fname))
            return {"ok": True, "output": f"Downloaded {fname} into {kind}/"}
        fname = pathlib.Path(str(payload.get("name", ""))).name
        fp = folder / fname
        if not fp.is_file():
            return {"ok": False, "error": f"File not found: {fname}"}
        if act == "toggle":
            dst = folder / (fname[:-9] if fname.endswith(".disabled") else fname + ".disabled")
            fp.rename(dst)
            return {"ok": True, "output": f"Toggled {dst.name}"}
        if act == "delete":
            fp.unlink()
            return {"ok": True, "output": f"Deleted {fname}"}
        return {"ok": False, "error": f"Unknown addon action: {act}"}

    if op == "backups":
        act = str(payload.get("action", ""))
        MC_BACKUPS.mkdir(parents=True, exist_ok=True)
        if act == "create":
            rcon_exec("save-off")
            rcon_exec("save-all flush")
            out_tgz = MC_BACKUPS / f"mc_world_{time.strftime('%Y%m%d_%H%M%S')}.tar.gz"
            ok, out = _run(["tar", "-czf", str(out_tgz), "-C", str(MC_DATA), "."], timeout=180)
            rcon_exec("save-on")
            return {"ok": ok, "backup": out_tgz.name, "output": out or f"Created backup {out_tgz.name}"}
        if act == "restore":
            bname = pathlib.Path(str(payload.get("name", ""))).name
            bf = MC_BACKUPS / bname
            if not bf.is_file():
                return {"ok": False, "error": "Backup file not found."}
            if MC_COMPOSE.is_file():
                _run(["docker", "compose", "-f", str(MC_COMPOSE), "stop"], timeout=45)
            ok, out = _run(["tar", "-xzf", str(bf), "-C", str(MC_DATA)], timeout=180)
            if MC_COMPOSE.is_file():
                _run(["docker", "compose", "-f", str(MC_COMPOSE), "up", "-d"], timeout=60)
            return {"ok": ok, "output": out or f"Restored {bname} and restarted server."}
        if act == "delete":
            bname = pathlib.Path(str(payload.get("name", ""))).name
            bf = MC_BACKUPS / bname
            if bf.is_file():
                bf.unlink()
            return {"ok": True, "output": f"Deleted backup {bname}"}
        if act == "upload_world":
            b64 = str(payload.get("data_base64", ""))
            wname = re.sub(r"[^A-Za-z0-9_-]", "", str(payload.get("world_name", "custom_world"))) or "custom_world"
            tmp_zip = MC_BACKUPS / f"_tmp_{wname}.zip"
            tmp_zip.write_bytes(base64.b64decode(b64))
            dst_dir = MC_DATA / wname
            dst_dir.mkdir(parents=True, exist_ok=True)
            with zipfile.ZipFile(tmp_zip, "r") as zf:
                zf.extractall(dst_dir)
            tmp_zip.unlink(missing_ok=True)
            save_properties({"level-name": wname})
            return {"ok": True, "output": f"Extracted world '{wname}' and set level-name={wname}."}
        return {"ok": False, "error": f"Unknown backup action: {act}"}

    if op == "schedules":
        MC_DIR.mkdir(parents=True, exist_ok=True)
        MC_SCHEDULES.write_text(json.dumps(payload.get("schedules") or [], indent=2), encoding="utf-8", newline="\n")
        return {"ok": True, "output": "Saved Minecraft scheduled tasks."}

    return {"ok": False, "error": f"Unknown Minecraft endpoint: {op}"}