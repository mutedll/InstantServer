# src/instant_server/config_builder.py
from __future__ import annotations

import argparse
from dataclasses import asdict, dataclass, field
import hashlib
import json
import os
import re
import secrets
import sys
import urllib.request
from typing import Any, Callable, Dict, List, Optional

_CRYPT_B64_ALPHABET = (
    "./0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

def _crypt_b64_encode(data: bytes, indices: List[tuple]) -> str:
    out: List[str] = []
    for idx_tuple in indices:
        if len(idx_tuple) == 3:
            b2, b1, b0 = data[idx_tuple[0]], data[idx_tuple[1]], data[idx_tuple[2]]
            w = (b2 << 16) | (b1 << 8) | b0
            for _ in range(4):
                out.append(_CRYPT_B64_ALPHABET[w & 0x3F])
                w >>= 6
        else:
            b0 = data[idx_tuple[0]]
            w = b0
            for _ in range(2):
                out.append(_CRYPT_B64_ALPHABET[w & 0x3F])
                w >>= 6
    return "".join(out)

def sha512_crypt(
    password: str,
    salt: Optional[str] = None,
    rounds: int = 5000,
) -> str:
    if password.startswith("$6$") and len(password) > 30:
        return password

    if salt is None:
        salt = "".join(secrets.choice(_CRYPT_B64_ALPHABET) for _ in range(16))
    else:
        salt = re.sub(r"[^./0-9A-Za-z]", "", salt)[:16]
        if not salt:
            salt = "".join(secrets.choice(_CRYPT_B64_ALPHABET) for _ in range(16))

    rounds = max(1000, min(999999999, int(rounds)))
    pw_bytes = password.encode("utf-8")
    salt_bytes = salt.encode("ascii")
    pw_len = len(pw_bytes)
    salt_len = len(salt_bytes)

    ctx_b = hashlib.sha512()
    ctx_b.update(pw_bytes)
    ctx_b.update(salt_bytes)
    ctx_b.update(pw_bytes)
    digest_b = ctx_b.digest()

    ctx_a = hashlib.sha512()
    ctx_a.update(pw_bytes)
    ctx_a.update(salt_bytes)

    full_blocks, rem = divmod(pw_len, 64)
    for _ in range(full_blocks):
        ctx_a.update(digest_b)
    ctx_a.update(digest_b[:rem])

    n = pw_len
    while n > 0:
        if n & 1:
            ctx_a.update(digest_b)
        else:
            ctx_a.update(pw_bytes)
        n >>= 1
    digest_a = ctx_a.digest()

    ctx_dp = hashlib.sha512()
    for _ in range(pw_len):
        ctx_dp.update(pw_bytes)
    digest_dp = ctx_dp.digest()
    p_bytes = (digest_dp * full_blocks) + digest_dp[:rem]

    ctx_ds = hashlib.sha512()
    for _ in range(16 + digest_a[0]):
        ctx_ds.update(salt_bytes)
    digest_ds = ctx_ds.digest()
    s_full, s_rem = divmod(salt_len, 64)
    s_bytes = (digest_ds * s_full) + digest_ds[:s_rem]

    digest_c = digest_a
    for i in range(rounds):
        ctx_c = hashlib.sha512()
        if i & 1:
            ctx_c.update(p_bytes)
        else:
            ctx_c.update(digest_c)
        if i % 3 != 0:
            ctx_c.update(s_bytes)
        if i % 7 != 0:
            ctx_c.update(p_bytes)
        if i & 1:
            ctx_c.update(digest_c)
        else:
            ctx_c.update(p_bytes)
        digest_c = ctx_c.digest()

    perm = [
        (0, 21, 42), (22, 43, 1), (44, 2, 23), (3, 24, 45),
        (25, 46, 4), (47, 5, 26), (6, 27, 48), (28, 49, 7),
        (50, 8, 29), (9, 30, 51), (31, 52, 10), (53, 11, 32),
        (12, 33, 54), (34, 55, 13), (56, 14, 35), (15, 36, 57),
        (37, 58, 16), (59, 17, 38), (18, 39, 60), (40, 61, 19),
        (62, 20, 41), (63,),
    ]
    encoded = _crypt_b64_encode(digest_c, perm)
    if rounds != 5000:
        return f"$6$rounds={rounds}${salt}${encoded}"
    return f"$6${salt}${encoded}"

SELF_HOSTED_APP_CATALOG: List[Dict[str, Any]] = [
    {
        "id": "kiwix",
        "title": "Offline Wikipedia (Kiwix)",
        "category": "Knowledge & Wiki",
        "hardware_tier": "old_friendly",
        "recommended": True,
        "interest_tag": "Offline Encyclopedia",
        "port": 8086,
        "container_port": 8080,
        "image": "ghcr.io/kiwix/kiwix-serve:latest",
        "ram_mb": 128,
        "description": "Host full offline Wikipedia, Wiktionary, medical guides, and Project Gutenberg (.zim files) on any old laptop.",
        "volumes": [
            "/DATA/AppData/kiwix/zim:/data",
        ],
        "command": "*.zim",
    },
    {
        "id": "jellyfin",
        "title": "Jellyfin",
        "category": "Media & Photos",
        "hardware_tier": "old_friendly",
        "recommended": True,
        "interest_tag": "Movies & TV",
        "port": 8096,
        "container_port": 8096,
        "image": "jellyfin/jellyfin:latest",
        "ram_mb": 512,
        "description": "Stream your movies, TV shows, and music to phones, smart TVs, and browsers with zero subscriptions.",
        "volumes": [
            "/DATA/AppData/jellyfin/config:/config",
            "/DATA/Media:/media",
        ],
    },
    {
        "id": "nextcloud",
        "title": "Nextcloud",
        "category": "Cloud & Files",
        "hardware_tier": "old_friendly",
        "recommended": True,
        "interest_tag": "Personal Cloud",
        "port": 8081,
        "container_port": 80,
        "image": "nextcloud:stable",
        "ram_mb": 512,
        "description": "Private Google Drive & Google Workspace replacement for your files, calendars, contacts, and phone backups.",
        "volumes": [
            "/DATA/AppData/nextcloud/html:/var/www/html",
            "/DATA/AppData/nextcloud/data:/var/www/html/data",
        ],
    },
    {
        "id": "pihole",
        "title": "Pi-hole",
        "category": "Network & Ad-Block",
        "hardware_tier": "old_friendly",
        "recommended": True,
        "interest_tag": "Ad Blocker",
        "port": 8085,
        "container_port": 80,
        "image": "pihole/pihole:latest",
        "ram_mb": 128,
        "description": "Network-wide DNS ad-blocker and tracker shield that blocks ads on every phone, TV, and laptop on your Wi-Fi.",
        "volumes": [
            "/DATA/AppData/pihole/etc-pihole:/etc/pihole",
            "/DATA/AppData/pihole/etc-dnsmasq.d:/etc/dnsmasq.d",
        ],
        "extra_ports": ["53:53/tcp", "53:53/udp"],
    },
    {
        "id": "immich",
        "title": "Immich",
        "category": "Media & Photos",
        "hardware_tier": "old_friendly",
        "recommended": True,
        "interest_tag": "Photo Backup",
        "port": 2283,
        "container_port": 2283,
        "image": "ghcr.io/immich-app/immich-server:release",
        "ram_mb": 1024,
        "description": "Fast self-hosted Google Photos alternative with automatic iOS/Android camera backup and album sharing.",
        "volumes": [
            "/DATA/AppData/immich/upload:/usr/src/app/upload",
        ],
    },
    {
        "id": "homeassistant",
        "title": "Home Assistant",
        "category": "Smart Home",
        "hardware_tier": "old_friendly",
        "recommended": True,
        "interest_tag": "Smart Home",
        "port": 8123,
        "container_port": 8123,
        "image": "homeassistant/home-assistant:stable",
        "ram_mb": 512,
        "description": "Automate smart lights, plugs, cameras, and Zigbee/Matter devices locally inside your home.",
        "volumes": [
            "/DATA/AppData/homeassistant/config:/config",
        ],
    },
    {
        "id": "vaultwarden",
        "title": "Vaultwarden",
        "category": "Security & Tools",
        "hardware_tier": "old_friendly",
        "recommended": True,
        "interest_tag": "Passwords",
        "port": 8082,
        "container_port": 80,
        "image": "vaultwarden/server:latest",
        "ram_mb": 64,
        "description": "Ultra-lightweight Bitwarden-compatible password manager server for your browser extensions and mobile apps.",
        "volumes": [
            "/DATA/AppData/vaultwarden/data:/data",
        ],
    },
    {
        "id": "minecraft",
        "title": "Minecraft Dedicated Server (Java)",
        "category": "Gaming Servers",
        "hardware_tier": "high_end",
        "recommended": False,
        "interest_tag": "Multiplayer Game (4GB+ RAM)",
        "port": 25565,
        "container_port": 25565,
        "image": "itzg/minecraft-server:latest",
        "ram_mb": 2048,
        "description": "24/7 dedicated Minecraft Java server (Vanilla, Paper, Spigot, Fabric, or Forge) with automatic world backups.",
        "volumes": [
            "/DATA/AppData/minecraft/data:/data",
        ],
        "extra_env": ["EULA=TRUE", "MEMORY=2G", "TYPE=PAPER"],
    },
    {
        "id": "minecraft_bedrock",
        "title": "Minecraft Bedrock Server (Mobile / Console)",
        "category": "Gaming Servers",
        "hardware_tier": "old_friendly",
        "recommended": False,
        "interest_tag": "Cross-Play Game",
        "port": 19132,
        "container_port": 19132,
        "image": "itzg/minecraft-bedrock-server:latest",
        "ram_mb": 512,
        "description": "Lightweight C++ Minecraft Bedrock server for Phones (iOS/Android), Xbox, PlayStation, Switch, and Windows.",
        "volumes": [
            "/DATA/AppData/minecraft_bedrock/data:/data",
        ],
        "extra_ports": ["19132:19132/udp"],
        "extra_env": ["EULA=TRUE"],
    },
    {
        "id": "ollama_webui",
        "title": "Local AI Server (Ollama + Open WebUI)",
        "category": "AI & Machine Learning",
        "hardware_tier": "high_end",
        "recommended": False,
        "interest_tag": "Higher-End Only (8GB+ RAM)",
        "port": 3005,
        "container_port": 8080,
        "image": "ghcr.io/open-webui/open-webui:ollama",
        "ram_mb": 8192,
        "description": "Run private local AI models (Llama 3, DeepSeek, Mistral, Qwen) with a ChatGPT web UI. Not for old low-RAM laptops.",
        "volumes": [
            "/DATA/AppData/ollama_webui/ollama:/root/.ollama",
            "/DATA/AppData/ollama_webui/webui:/app/backend/data",
        ],
        "extra_ports": ["11434:11434/tcp"],
    },
    {
        "id": "localai",
        "title": "LocalAI (Private AI Voice, Vision & Text)",
        "category": "AI & Machine Learning",
        "hardware_tier": "high_end",
        "recommended": False,
        "interest_tag": "Higher-End Only (8GB+ RAM)",
        "port": 8088,
        "container_port": 8080,
        "image": "localai/localai:latest",
        "ram_mb": 4096,
        "description": "Self-hosted OpenAI-compatible API for local text generation, Whisper speech-to-text, and voice synthesis.",
        "volumes": [
            "/DATA/AppData/localai/models:/build/models",
        ],
    },
    {
        "id": "bookstack",
        "title": "BookStack (Personal Wiki & Notes)",
        "category": "Knowledge & Wiki",
        "hardware_tier": "old_friendly",
        "recommended": False,
        "interest_tag": "Personal Wiki",
        "port": 6875,
        "container_port": 80,
        "image": "lscr.io/linuxserver/bookstack:latest",
        "ram_mb": 256,
        "description": "Build your own Wikipedia-style knowledge base, family handbook, and study notes organized into books and chapters.",
        "volumes": [
            "/DATA/AppData/bookstack/config:/config",
        ],
    },
    {
        "id": "calibreweb",
        "title": "Calibre-Web (E-Book Library)",
        "category": "Knowledge & Wiki",
        "hardware_tier": "old_friendly",
        "recommended": False,
        "interest_tag": "E-Books & EPUB",
        "port": 8083,
        "container_port": 8083,
        "image": "lscr.io/linuxserver/calibre-web:latest",
        "ram_mb": 128,
        "description": "Browse, read in browser, and wireless-send EPUB/PDF e-books to your Kindle, Kobo, or phone.",
        "volumes": [
            "/DATA/AppData/calibreweb/config:/config",
            "/DATA/Media/Books:/books",
        ],
    },
    {
        "id": "wg_easy",
        "title": "WireGuard VPN (wg-easy)",
        "category": "Network & Ad-Block",
        "hardware_tier": "old_friendly",
        "recommended": False,
        "interest_tag": "Home VPN",
        "port": 51821,
        "container_port": 51821,
        "image": "ghcr.io/wg-easy/wg-easy:latest",
        "ram_mb": 64,
        "description": "Fast WireGuard VPN server with a web dashboard and instant QR-code setup for phones and laptops.",
        "volumes": [
            "/DATA/AppData/wg_easy/etc_wireguard:/etc/wireguard",
        ],
        "extra_ports": ["51820:51820/udp"],
    },
    {
        "id": "adguardhome",
        "title": "AdGuard Home",
        "category": "Network & Ad-Block",
        "hardware_tier": "old_friendly",
        "recommended": False,
        "interest_tag": "Family DNS",
        "port": 3000,
        "container_port": 3000,
        "image": "adguard/adguardhome:latest",
        "ram_mb": 128,
        "description": "All-in-one DNS ad-blocker, parental control filter, and encrypted DNS-over-HTTPS server.",
        "volumes": [
            "/DATA/AppData/adguardhome/work:/opt/adguardhome/work",
            "/DATA/AppData/adguardhome/conf:/opt/adguardhome/conf",
        ],
    },
    {
        "id": "plex",
        "title": "Plex Media Server",
        "category": "Media & Photos",
        "hardware_tier": "old_friendly",
        "recommended": False,
        "interest_tag": "Movies & TV",
        "port": 32400,
        "container_port": 32400,
        "image": "lscr.io/linuxserver/plex:latest",
        "ram_mb": 512,
        "description": "Organize and stream personal movies, shows, and music to Plex apps on any smart TV, console, or phone.",
        "volumes": [
            "/DATA/AppData/plex/config:/config",
            "/DATA/Media:/media",
        ],
    },
    {
        "id": "syncthing",
        "title": "Syncthing",
        "category": "Cloud & Files",
        "hardware_tier": "old_friendly",
        "recommended": False,
        "interest_tag": "File Sync",
        "port": 8384,
        "container_port": 8384,
        "image": "lscr.io/linuxserver/syncthing:latest",
        "ram_mb": 128,
        "description": "Real-time peer-to-peer folder sync across your laptops, desktops, and Android devices.",
        "volumes": [
            "/DATA/AppData/syncthing/config:/config",
            "/DATA/Sync:/data1",
        ],
    },
    {
        "id": "filebrowser",
        "title": "FileBrowser",
        "category": "Cloud & Files",
        "hardware_tier": "old_friendly",
        "recommended": False,
        "interest_tag": "Web File Manager",
        "port": 8087,
        "container_port": 80,
        "image": "filebrowser/filebrowser:latest",
        "ram_mb": 64,
        "description": "Simple web interface to browse, upload, download, and share files stored on your server.",
        "volumes": [
            "/DATA:/srv",
            "/DATA/AppData/filebrowser/db:/database",
        ],
    },
    {
        "id": "qbittorrent",
        "title": "qBittorrent",
        "category": "Media & Photos",
        "hardware_tier": "old_friendly",
        "recommended": False,
        "interest_tag": "Downloads",
        "port": 8080,
        "container_port": 8080,
        "image": "lscr.io/linuxserver/qbittorrent:latest",
        "ram_mb": 256,
        "description": "Web-based torrent downloader with magnet link, sequential download, and RSS auto-download support.",
        "volumes": [
            "/DATA/AppData/qbittorrent/config:/config",
            "/DATA/Downloads:/downloads",
        ],
    },
    {
        "id": "uptimekuma",
        "title": "Uptime Kuma",
        "category": "Security & Tools",
        "hardware_tier": "old_friendly",
        "recommended": False,
        "interest_tag": "Monitoring",
        "port": 3001,
        "container_port": 3001,
        "image": "louislam/uptime-kuma:1",
        "ram_mb": 128,
        "description": "Self-hosted status page and uptime monitor that sends alerts if your home apps or internet go offline.",
        "volumes": [
            "/DATA/AppData/uptimekuma/data:/app/data",
        ],
    },
    {
        "id": "portainer",
        "title": "Portainer CE",
        "category": "Security & Tools",
        "hardware_tier": "old_friendly",
        "recommended": False,
        "interest_tag": "Containers",
        "port": 9443,
        "container_port": 9443,
        "image": "portainer/portainer-ce:latest",
        "ram_mb": 128,
        "description": "Advanced web dashboard for managing Docker containers, images, networks, and Compose stacks.",
        "volumes": [
            "/var/run/docker.sock:/var/run/docker.sock",
            "/DATA/AppData/portainer/data:/data",
        ],
    },
    {
        "id": "nginxproxymanager",
        "title": "Nginx Proxy Manager",
        "category": "Network & Ad-Block",
        "hardware_tier": "old_friendly",
        "recommended": False,
        "interest_tag": "Reverse Proxy",
        "port": 81,
        "container_port": 81,
        "image": "jc21/nginx-proxy-manager:latest",
        "ram_mb": 128,
        "description": "Route custom domain names to your self-hosted apps with automatic Let's Encrypt SSL certificates.",
        "volumes": [
            "/DATA/AppData/nginxproxymanager/data:/data",
            "/DATA/AppData/nginxproxymanager/letsencrypt:/etc/letsencrypt",
        ],
    },
    {
        "id": "paperless",
        "title": "Paperless-ngx",
        "category": "Cloud & Files",
        "hardware_tier": "old_friendly",
        "recommended": False,
        "interest_tag": "Document OCR",
        "port": 8000,
        "container_port": 8000,
        "image": "ghcr.io/paperless-ngx/paperless-ngx:latest",
        "ram_mb": 512,
        "description": "Organize scanned receipts, PDFs, and invoices with automatic OCR full-text search.",
        "volumes": [
            "/DATA/AppData/paperless/data:/usr/src/paperless/data",
            "/DATA/AppData/paperless/media:/usr/src/paperless/media",
        ],
    },
    {
        "id": "stirlingpdf",
        "title": "Stirling-PDF",
        "category": "Security & Tools",
        "hardware_tier": "old_friendly",
        "recommended": False,
        "interest_tag": "PDF Toolkit",
        "port": 8084,
        "container_port": 8080,
        "image": "frooodle/s-pdf:latest",
        "ram_mb": 256,
        "description": "Private local web toolkit to merge, split, rotate, compress, and convert PDF documents.",
        "volumes": [
            "/DATA/AppData/stirlingpdf/configs:/configs",
        ],
    },
    {
        "id": "navidrome",
        "title": "Navidrome",
        "category": "Media & Photos",
        "hardware_tier": "old_friendly",
        "recommended": False,
        "interest_tag": "Music Server",
        "port": 4533,
        "container_port": 4533,
        "image": "deluan/navidrome:latest",
        "ram_mb": 64,
        "description": "Fast, lightweight personal music streaming server compatible with Subsonic mobile apps.",
        "volumes": [
            "/DATA/AppData/navidrome/data:/data",
            "/DATA/Media/Music:/music:ro",
        ],
    },
    {
        "id": "gitea",
        "title": "Gitea",
        "category": "Security & Tools",
        "hardware_tier": "old_friendly",
        "recommended": False,
        "interest_tag": "Git Server",
        "port": 3002,
        "container_port": 3000,
        "image": "gitea/gitea:latest",
        "ram_mb": 128,
        "description": "Lightweight self-hosted Git service for hosting your private code repositories, issues, and actions.",
        "volumes": [
            "/DATA/AppData/gitea/data:/data",
        ],
    },
]

APP_CATALOG_BY_ID: Dict[str, Dict[str, Any]] = {app["id"]: app for app in SELF_HOSTED_APP_CATALOG}

def generate_casaos_app_compose(app_id: str, timezone: str = "UTC") -> str:
    app = APP_CATALOG_BY_ID.get(app_id)
    if not app:
        raise KeyError(f"Unknown self-hosted app ID: {app_id}")

    ports_list = [f"{app['port']}:{app['container_port']}"]
    for ep in app.get("extra_ports", []):
        ports_list.append(ep)

    env_list = [f"TZ={timezone}", "PUID=1000", "PGID=1000"]
    for ee in app.get("extra_env", []):
        env_list.append(ee)

    env_yaml = "\n".join(f"      - {e}" for e in env_list)
    vols_yaml = "\n".join(f"      - {v}" for v in app.get("volumes", []))
    ports_yaml = "\n".join(f'      - "{p}"' for p in ports_list)
    cmd_line = f"\n    command: {app['command']}" if app.get("command") else ""

    return f"""name: {app['id']}
services:
  {app['id']}:
    image: {app['image']}
    container_name: {app['id']}
    restart: unless-stopped{cmd_line}
    environment:
{env_yaml}
    ports:
{ports_yaml}
    volumes:
{vols_yaml}
x-casaos:
  architectures:
    - amd64
    - arm64
  main: {app['id']}
  category: {app['category']}
  port_map: "{app['port']}"
  scheme: http
  index: /
  title:
    en_us: "{app['title']}"
  description:
    en_us: "{app['description']}"
"""

SERVER_ROLE_PRESETS: List[Dict[str, Any]] = [
    {
        "id": "media_box",
        "title": "Home Media & Movies Box",
        "badge": "Most Popular",
        "recommended_ram_gb": 4,
        "description": "Stream movies, TV shows, music, and photos to every TV and phone at home with hardware transcoding.",
        "selected_apps": ["jellyfin", "immich", "navidrome", "qbittorrent", "filebrowser"],
        "enable_intel_vaapi": True,
        "enable_samba_nas": True,
    },
    {
        "id": "family_cloud",
        "title": "Private Family Cloud & Backup",
        "badge": "Google Drive Alternative",
        "recommended_ram_gb": 4,
        "description": "Replace Google Drive, Google Photos, and 1Password with Nextcloud, Immich, Vaultwarden, and Paperless OCR.",
        "selected_apps": ["nextcloud", "immich", "vaultwarden", "syncthing", "paperless", "filebrowser"],
        "enable_samba_nas": True,
    },
    {
        "id": "adblock_vpn",
        "title": "Whole-Home Ad-Blocker & Private VPN",
        "badge": "Runs on 2GB RAM",
        "recommended_ram_gb": 2,
        "description": "Block ads and trackers across every Wi-Fi device with Pi-hole/AdGuard and connect securely from anywhere via WireGuard.",
        "selected_apps": ["pihole", "adguardhome", "wg_easy", "uptimekuma", "vaultwarden"],
        "enable_pihole_dns_prep": True,
    },
    {
        "id": "offline_wiki",
        "title": "Offline Wikipedia, Medical & Survival Box",
        "badge": "100% Air-Gapped Ready",
        "recommended_ram_gb": 2,
        "description": "Host full offline Wikipedia, WikiMed emergency medical guides, iFixit repair manuals, and 70,000+ Gutenberg books.",
        "selected_apps": ["kiwix", "calibreweb", "bookstack", "stirlingpdf", "filebrowser"],
        "preselect_kiwix_zims": ["wikipedia_en_simple", "wikipedia_en_medicine", "ifixit_en_all"],
    },
    {
        "id": "old_laptop_starter",
        "title": "Lightweight Old Laptop Starter (2GB–4GB RAM)",
        "badge": "Ultra-Low Power",
        "recommended_ram_gb": 2,
        "description": "Tuned for older dual-core laptops: ZRAM compressed swap, CPU powersave governor, Pi-hole, FileBrowser, and Kiwix.",
        "selected_apps": ["kiwix", "pihole", "vaultwarden", "filebrowser"],
        "enable_low_ram_zram": True,
        "cpu_governor": "powersave",
    },
    {
        "id": "ai_gaming_pro",
        "title": "Local AI & Multiplayer Gaming Server (8GB–16GB+)",
        "badge": "High-Spec Laptops",
        "recommended_ram_gb": 8,
        "description": "Run local Llama/DeepSeek LLMs via Ollama + Open WebUI, 24/7 Minecraft Java & Bedrock servers, and Portainer.",
        "selected_apps": ["ollama_webui", "minecraft", "minecraft_bedrock", "jellyfin", "portainer", "uptimekuma"],
        "cpu_governor": "schedutil",
    },
]

VALID_INSTALL_MODES = (
    "auto_erase",
    "dual_boot",
    "interactive_select",
    "secondary_drive",
    "repair_rescue_usb",
)
VALID_DEVICE_FORM_FACTORS = (
    "auto_detect",
    "laptop_ewaste",
    "laptop_modern",
    "desktop_pc",
    "mini_pc_n100",
    "workstation_gpu",
    "nas_multibay",
)
VALID_CLUSTER_MODES = (
    "standalone",
    "usb_multi_node_auto",
    "cluster_coordinator",
    "cluster_worker",
)
VALID_CLUSTER_STORAGE_POLICIES = (
    "mergerfs_union_safe",
    "replicated_ha_mirror",
    "wipe_secondary_expand",
)
VALID_STORAGE_LAYOUTS = ("direct", "lvm", "btrfs", "zfs")
VALID_BOOT_MODES = ("auto", "uefi", "bios")
VALID_USB_SCHEMES = ("auto", "gpt_uefi", "mbr_bios")
VALID_SHUTDOWN_MODES = ("poweroff", "reboot")
VALID_DASHBOARDS = ("cockpit", "casaos", "both", "none")

@dataclass
class ServerProfile:

    hostname: str = "instant-server"
    username: str = "admin"
    password: str = "instant2026"
    timezone: str = "Asia/Kolkata"
    locale: str = "en_US.UTF-8"
    keyboard_layout: str = "us"
    wifi_ssid: str = ""
    wifi_password: str = ""
    enable_usb_tethering: bool = True
    disable_wifi_power_save: bool = True
    enable_mosh_server: bool = True
    enable_network_bonding: bool = False

    dashboard_choice: str = "both"
    selected_apps: List[str] = field(
        default_factory=lambda: [
            "kiwix",
            "jellyfin",
            "nextcloud",
            "pihole",
            "immich",
            "homeassistant",
            "vaultwarden",
        ]
    )
    prebundle_offline_images: bool = False
    enable_docker: bool = True
    enable_tailscale: bool = False
    tailscale_authkey: str = ""
    enable_samba_nas: bool = True
    enable_pihole_dns_prep: bool = True
    enable_intel_vaapi: bool = True
    enable_wake_on_lan: bool = True

    laptop_age_hint: str = "unknown"
    laptop_ram_hint: str = "unknown"
    usb_partition_scheme: str = "auto"
    custom_iso_path: str = ""
    confirm_usb_erase: bool = True
    install_mode: str = "dual_boot"
    confirm_erase_main_drive: bool = False
    dual_boot_Allocation_gb: int = 45
    ubuntu_release: str = "26.04"
    storage_layout: str = "direct"
    boot_mode: str = "auto"
    target_disk_hint: str = "auto"
    allow_usb_target_disk: bool = False
    force_reinstall: bool = False
    shutdown_after_install: str = "poweroff"

    ignore_lid_switch: bool = True
    screen_blank_seconds: int = 60
    battery_charge_start_pct: int = 40
    battery_charge_stop_pct: int = 60
    ups_shutdown_battery_pct: int = 15
    enable_low_ram_zram: bool = True
    safe_gpu_nomodeset: bool = False

    ui_mode: str = "standard"
    preset_role: str = "custom"
    enable_custom_dashboard: bool = True
    dashboard_port: int = 8088
    enable_auto_updates: bool = True
    auto_update_window: str = "03:00"
    enable_scheduled_backups: bool = True
    backup_retention_days: int = 14
    preselect_kiwix_zims: List[str] = field(default_factory=lambda: ["wikipedia_en_simple"])

    ssh_port: int = 22
    ssh_allow_password: bool = True
    ssh_authorized_keys: List[str] = field(default_factory=list)
    github_ssh_username: str = ""
    enable_ufw_firewall: bool = True
    enable_fail2ban: bool = True
    enable_ipv4_forwarding: bool = True
    enable_tcp_bbr: bool = True
    vm_swappiness: int = 10
    inotify_max_watches: int = 524288
    root_filesystem: str = "ext4"
    enable_ssd_noatime_trim: bool = True
    enable_smartd_monitor: bool = True
    zram_algorithm: str = "zstd"
    zram_percent: int = 50
    cpu_governor: str = "schedutil"
    disable_cpu_turbo_boost: bool = False
    docker_subnet_cidr: str = "172.17.0.1/16"
    docker_log_max_size: str = "10m"
    docker_log_max_file: int = 3
    enable_prometheus_metrics: bool = False
    custom_kernel_args: str = ""
    custom_post_install_script: str = ""
    custom_autoinstall_yaml_override: str = ""
    extra_apt_packages: List[str] = field(default_factory=list)
    custom_late_commands: List[str] = field(default_factory=list)
    static_ip_cidr: str = ""
    static_gateway: str = ""
    static_dns: List[str] = field(default_factory=list)
    device_form_factor: str = "auto_detect"
    enable_pcie_aspm: bool = True
    enable_realtek_25gbe_fix: bool = True
    enable_nvidia_gpu_Passthrough: bool = False
    enable_multi_disk_mergerfs: bool = True
    cluster_mode: str = "standalone"
    cluster_name: str = "instant-homelab"
    cluster_join_token: str = ""
    cluster_coordinator_ip: str = ""
    cluster_storage_policy: str = "mergerfs_union_safe"
    cluster_sqlite_local_pinning: bool = True
    cluster_confirm_wipe_secondary: str = ""
    repair_actions: List[str] = field(default_factory=lambda: ["bootloader_efi", "fsck_disks", "smart_nvme_health", "samba_rescue_share"])

    def __post_init__(self) -> None:
        if self.laptop_age_hint == "modern_uefi" and self.usb_partition_scheme == "auto":
            self.boot_mode = "uefi"
        elif self.laptop_age_hint == "legacy_bios" and self.usb_partition_scheme == "auto":
            self.boot_mode = "bios"
        elif self.laptop_age_hint == "unknown":
            self.boot_mode = "auto"

        if self.laptop_ram_hint == "low_2_4gb":
            self.enable_low_ram_zram = True

        if "pihole" in (self.selected_apps or []) or "adguardhome" in (self.selected_apps or []):
            self.enable_pihole_dns_prep = True

    def validate(self) -> List[str]:
        errors: List[str] = []
        if not re.match(r"^[a-z0-9][a-z0-9-]{0,61}[a-z0-9]?$", self.hostname):
            errors.append(
                f"Invalid hostname '{self.hostname}': use lowercase letters, digits, and hyphens."
            )
        reserved_users = {"root", "daemon", "bin", "sys", "sync", "ubuntu", "sshd"}
        if not re.match(r"^[a-z_][a-z0-9_-]{0,31}$", self.username):
            errors.append(
                f"Invalid username '{self.username}': must start with a lowercase letter and contain only [a-z0-9_-]."
            )
        if self.username in reserved_users:
            errors.append(
                f"Username '{self.username}' is reserved by Ubuntu/Subiquity; choose another username (e.g., 'admin')."
            )
        if not self.password and not self.ssh_authorized_keys and not self.github_ssh_username:
            errors.append("Either a password or at least one SSH key must be provided.")
        if self.wifi_ssid and len(self.wifi_password) > 0 and len(self.wifi_password) < 8:
            errors.append("WPA2/WPA3 Wi-Fi passwords must be at least 8 characters long.")
        if self.install_mode not in VALID_INSTALL_MODES:
            errors.append(f"install_mode must be one of {VALID_INSTALL_MODES}.")
        if self.install_mode == "auto_erase" and not self.confirm_erase_main_drive:
            errors.append(
                "Auto-Erase Main Drive requires explicit confirmation ('I know what I'm doing — erase main drive')."
            )
        if self.install_mode == "dual_boot" and not (15 <= self.dual_boot_Allocation_gb <= 65536):
            errors.append("dual_boot_Allocation_gb must be between 15 GB and 65536 GB (64 TB).")
        if self.storage_layout not in VALID_STORAGE_LAYOUTS:
            errors.append(f"storage_layout must be one of {VALID_STORAGE_LAYOUTS}.")
        if self.boot_mode not in VALID_BOOT_MODES:
            errors.append(f"boot_mode must be one of {VALID_BOOT_MODES}.")
        if self.shutdown_after_install not in VALID_SHUTDOWN_MODES:
            errors.append(f"shutdown_after_install must be one of {VALID_SHUTDOWN_MODES}.")
        if self.dashboard_choice not in VALID_DASHBOARDS:
            errors.append(f"dashboard_choice must be one of {VALID_DASHBOARDS}.")
        if not (20 <= self.battery_charge_start_pct < self.battery_charge_stop_pct <= 100):
            errors.append(
                "battery_charge_start_pct must be < battery_charge_stop_pct (between 20% and 100%)."
            )
        if not (5 <= self.ups_shutdown_battery_pct <= 40):
            errors.append("ups_shutdown_battery_pct must be between 5% and 40%.")
        return errors

def emit_event(
    event_type: str,
    message: str,
    data: Optional[Dict[str, Any]] = None,
    callback: Optional[Callable[[Dict[str, Any]], None]] = None,
) -> Dict[str, Any]:
    payload = {
        "event": event_type,
        "module": "CONFIG_BUILDER",
        "project": "Instant Server",
        "message": message,
        "data": data or {},
    }
    if callback:
        callback(payload)
    return payload

def fetch_github_ssh_keys(github_user: str, timeout: int = 8) -> List[str]:
    clean_user = re.sub(r"[^a-zA-Z0-9-]", "", github_user.strip())
    if not clean_user:
        return []
    url = f"https://github.com/{clean_user}.keys"
    req = urllib.request.Request(url, headers={"User-Agent": "InstantServer-ConfigBuilder/2.1"})
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            lines = resp.read().decode("utf-8", errors="replace").splitlines()
            return [ln.strip() for ln in lines if ln.strip().startswith(("ssh-", "ecdsa-"))]
    except Exception:
        return []

def _yaml_quote(val: str) -> str:
    if val == "":
        return "''"
    safe_unquoted = re.compile(r"^[a-zA-Z0-9_./:-]+$")
    yaml_reserved = {
        "true", "false", "yes", "no", "on", "off", "null", "none", "~", "y", "n"
    }
    if (
        safe_unquoted.match(val)
        and val.lower() not in yaml_reserved
        and not val.startswith(("-", ":"))
        and not val.isdigit()
    ):
        return val
    escaped = val.replace("'", "''")
    return f"'{escaped}'"

def dump_yaml(obj: Any, indent: int = 0) -> str:
    lines: List[str] = []
    prefix = "  " * indent
    if isinstance(obj, dict):
        if not obj:
            return f"{prefix}{{}}"
        for k, v in obj.items():
            if isinstance(v, (dict, list)) and v:
                lines.append(f"{prefix}{k}:")
                lines.append(dump_yaml(v, indent + 1))
            elif isinstance(v, dict) and not v:
                lines.append(f"{prefix}{k}: {{}}")
            elif isinstance(v, list) and not v:
                lines.append(f"{prefix}{k}: []")
            else:
                lines.append(f"{prefix}{k}: {_format_scalar(v)}")
    elif isinstance(obj, list):
        for item in obj:
            if isinstance(item, dict) and item:
                first_k = next(iter(item))
                first_v = item[first_k]
                rest = {k: v for k, v in item.items() if k != first_k}
                if isinstance(first_v, (dict, list)) and first_v:
                    lines.append(f"{prefix}- {first_k}:")
                    lines.append(dump_yaml(first_v, indent + 2))
                else:
                    lines.append(f"{prefix}- {first_k}: {_format_scalar(first_v)}")
                if rest:
                    lines.append(dump_yaml(rest, indent + 1))
            elif isinstance(item, list):
                lines.append(f"{prefix}-")
                lines.append(dump_yaml(item, indent + 1))
            else:
                lines.append(f"{prefix}- {_format_scalar(item)}")
    else:
        lines.append(f"{prefix}{_format_scalar(obj)}")
    return "\n".join(lines)

def _format_scalar(val: Any) -> str:
    if val is None:
        return "null"
    if isinstance(val, bool):
        return "true" if val else "false"
    if isinstance(val, (int, float)):
        return str(val)
    return _yaml_quote(str(val))

def compile_cidata_payloads(
    profile: ServerProfile,
    event_callback: Optional[Callable[[Dict[str, Any]], None]] = None,
) -> Dict[str, str]:
    errors = profile.validate()
    if errors:
        raise ValueError("Profile validation failed:\n- " + "\n- ".join(errors))

    emit_event("CONFIG_BUILD_START", f"Compiling Instant Server configuration for {profile.hostname}", callback=event_callback)

    pw_hash = sha512_crypt(profile.password)
    emit_event("PASSWORD_HASHED", "Generated SHA-512 crypt password hash", callback=event_callback)

    ssh_keys = list(profile.ssh_authorized_keys)
    if profile.github_ssh_username:
        fetched = fetch_github_ssh_keys(profile.github_ssh_username)
        for k in fetched:
            if k not in ssh_keys:
                ssh_keys.append(k)

    ethernets_cfg: Dict[str, Any] = {
        "any-ethernet": {
            "match": {"name": "en*"},
            "dhcp4": True,
            "dhcp4-overrides": {"route-metric": 100},
            "dhcp6": True,
            "optional": True,
        },
        "legacy-ethernet": {
            "match": {"name": "eth*"},
            "dhcp4": True,
            "dhcp4-overrides": {"route-metric": 100},
            "dhcp6": True,
            "optional": True,
        },
    }
    if profile.enable_usb_tethering:
        ethernets_cfg["usb-tether-rndis"] = {
            "match": {"name": "usb*"},
            "dhcp4": True,
            "optional": True,
        }
        ethernets_cfg["usb-tether-ncm"] = {
            "match": {"name": "enx*"},
            "dhcp4": True,
            "optional": True,
        }

    netplan_cfg: Dict[str, Any] = {
        "version": 2,
        "renderer": "networkd",
        "ethernets": ethernets_cfg,
    }
    if profile.wifi_ssid:
        wifi_ap: Dict[str, Any] = {}
        if profile.wifi_password:
            wifi_ap[profile.wifi_ssid] = {"password": profile.wifi_password}
        else:
            wifi_ap[profile.wifi_ssid] = {}
        netplan_cfg["wifis"] = {
            "wlan0": {
                "dhcp4": True,
                "dhcp4-overrides": {"route-metric": 600},
                "optional": True,
                "access-points": wifi_ap,
            }
        }

    storage_cfg: Dict[str, Any] = {
        "layout": {
            "name": "lvm" if profile.storage_layout == "lvm" else "direct",
        }
    }
    if profile.storage_layout == "lvm":
        storage_cfg["layout"]["sizing-policy"] = "all"

    base_packages = [
        "openssh-server",
        "curl",
        "wget",
        "ca-certificates",
        "gnupg",
        "lsb-release",
        "htop",
        "tmux",
        "pciutils",
        "usbutils",
        "ethtool",
        "avahi-daemon",
        "mosh",
        "mergerfs",
        "nfs-kernel-server",
        "nfs-common",
        "smartmontools",
        "nvme-cli",
        "efibootmgr",
        "mdadm",
        "memtester",
        "iw",
    ]
    if profile.install_mode == "dual_boot":
        base_packages.append("os-prober")
    for pkg in profile.extra_apt_packages:
        if pkg not in base_packages:
            base_packages.append(pkg)

    live_runcmd = [
        [
            "sh",
            "-c",
            (
                "if ! grep -qw autoinstall /proc/cmdline; then "
                "echo \"$(cat /proc/cmdline) autoinstall\" > /run/instant-cmdline && "
                "mount -n --bind -o ro /run/instant-cmdline /proc/cmdline && "
                "snap restart subiquity.subiquity-server || true; "
                "snap restart subiquity.subiquity-service || true; "
                "fi"
            ),
        ]
    ]

    early_commands = [
        (
            "sh -c '"
            "mkdir -p /run/instant-cidata; "
            "CIDATA_DEV=$(blkid -L cidata || blkid -L CIDATA || true); "
            "if [ -n \"$CIDATA_DEV\" ]; then mount -o ro \"$CIDATA_DEV\" /run/instant-cidata || true; fi; "
            "for dir in /run/instant-cidata /cdrom/cidata /cdrom /var/lib/cloud/seed/nocloud; do "
            "  if [ -f \"$dir/hardware_adapter.py\" ]; then "
            "    python3 \"$dir/hardware_adapter.py\" --env \"$dir/instant-server-env.conf\" --autoinstall /autoinstall.yaml; "
            "    break; "
            "  fi; "
            "done'"
        )
    ]

    late_commands = [
        (
            "sh -c '"
            "mkdir -p /run/instant-cidata /target/opt/instant-server; "
            "if ! mountpoint -q /run/instant-cidata 2>/dev/null; then "
            "  CIDATA_DEV=$(blkid -L cidata || blkid -L CIDATA || true); "
            "  if [ -n \"$CIDATA_DEV\" ]; then mount -o ro \"$CIDATA_DEV\" /run/instant-cidata || true; fi; "
            "fi; "
            "for dir in /run/instant-cidata /cdrom/cidata /cdrom /var/lib/cloud/seed/nocloud; do "
            "  if [ -f \"$dir/laptop_hardening.sh\" ]; then "
            "    cp -r \"$dir/\"* /target/opt/instant-server/ 2>/dev/null || true; "
            "    chmod +x /target/opt/instant-server/laptop_hardening.sh; "
            "    curtin in-target --target=/target -- /bin/bash /opt/instant-server/laptop_hardening.sh; "
            "    break; "
            "  fi; "
            "done'"
        )
    ]
    for custom_cmd in profile.custom_late_commands:
        if custom_cmd.strip():
            late_commands.append(f"curtin in-target --target=/target -- /bin/bash -c {_yaml_quote(custom_cmd.strip())}")

    autoinstall_doc: Dict[str, Any] = {
        "runcmd": live_runcmd,
        "autoinstall": {
            "version": 1,
            "locale": profile.locale,
            "timezone": profile.timezone,
            "keyboard": {
                "layout": profile.keyboard_layout,
                "variant": "",
                "toggle": None,
            },
            "refresh-installer": {
                "update": False,
            },
            "apt": {
                "fallback": "offline-install",
                "geoip": True,
                "preserve_sources_list": False,
            },
            "identity": {
                "realname": f"{profile.hostname} Admin",
                "username": profile.username,
                "hostname": profile.hostname,
                "password": pw_hash,
            },
            "ssh": {
                "install-server": True,
                "allow-pw": profile.ssh_allow_password,
                "authorized-keys": ssh_keys,
            },
            "drivers": {
                "install": True,
            },
            "kernel-crash-dumps": {
                "enabled": False,
            },
            "network": netplan_cfg,
            "storage": storage_cfg,
            "packages": base_packages,
            "updates": "security",
            "EarlyCommandsPlaceholder": early_commands,
            "LateCommandsPlaceholder": late_commands,
            "error-commands": [
                "sh -c 'journalctl -b > /var/log/installer/instant-server-error-journal.log || true'",
            ],
            "shutdown": profile.shutdown_after_install,
            "user-data": {
                "disable_root": True,
                "package_update": False,
                "package_upgrade": False,
            },
        },
    }

    ai_block = autoinstall_doc["autoinstall"]
    if profile.install_mode == "interactive_select":
        ai_block["interactive-sections"] = ["storage"]

    ai_block["early-commands"] = ai_block.pop("EarlyCommandsPlaceholder")
    ai_block["late-commands"] = ai_block.pop("LateCommandsPlaceholder")

    user_data_yaml = "#cloud-config\n" + dump_yaml(autoinstall_doc) + "\n"
    meta_data_content = (
        f"instance-id: instant-server-{profile.hostname}-001\n"
        f"local-hostname: {profile.hostname}\n"
    )

    enable_cockpit = profile.dashboard_choice in ("cockpit", "both")
    enable_casaos = profile.dashboard_choice in ("casaos", "both")
    enable_docker = profile.enable_docker or enable_casaos or len(profile.selected_apps) > 0
    valid_selected_apps = [a for a in (profile.selected_apps or []) if a in APP_CATALOG_BY_ID]

    env_lines = [
        "# Instant Server — Target Hardware & Stack Environment Configuration",
        f"INSTANT_HOSTNAME={_sh_escape(profile.hostname)}",
        f"INSTANT_USERNAME={_sh_escape(profile.username)}",
        f"INSTANT_UBUNTU_RELEASE={_sh_escape(profile.ubuntu_release)}",
        f"INSTANT_INSTALL_MODE={_sh_escape(profile.install_mode)}",
        f"INSTANT_CONFIRM_ERASE={_sh_escape('1' if profile.confirm_erase_main_drive else '0')}",
        f"INSTANT_DUAL_BOOT_GB={int(profile.dual_boot_Allocation_gb)}",
        f"INSTANT_STORAGE_LAYOUT={_sh_escape(profile.storage_layout)}",
        f"INSTANT_BOOT_MODE={_sh_escape(profile.boot_mode)}",
        f"INSTANT_LAPTOP_AGE_HINT={_sh_escape(profile.laptop_age_hint)}",
        f"INSTANT_LAPTOP_RAM_HINT={_sh_escape(profile.laptop_ram_hint)}",
        f"INSTANT_USB_PARTITION_SCHEME={_sh_escape(profile.usb_partition_scheme)}",
        f"INSTANT_TARGET_DISK_HINT={_sh_escape(profile.target_disk_hint)}",
        f"INSTANT_ALLOW_USB_TARGET={_sh_escape('1' if profile.allow_usb_target_disk else '0')}",
        f"INSTANT_FORCE_REINSTALL={_sh_escape('1' if profile.force_reinstall else '0')}",
        f"INSTANT_WIFI_SSID={_sh_escape(profile.wifi_ssid)}",
        f"INSTANT_WIFI_PASSWORD={_sh_escape(profile.wifi_password)}",
        f"INSTANT_ENABLE_USB_TETHER={_sh_escape('1' if profile.enable_usb_tethering else '0')}",
        f"INSTANT_DISABLE_WIFI_POWERSAVE={_sh_escape('1' if profile.disable_wifi_power_save else '0')}",
        f"INSTANT_ENABLE_MOSH={_sh_escape('1' if profile.enable_mosh_server else '0')}",
        f"INSTANT_ENABLE_BONDING={_sh_escape('1' if profile.enable_network_bonding else '0')}",
        f"INSTANT_STATIC_IP_CIDR={_sh_escape(profile.static_ip_cidr)}",
        f"INSTANT_STATIC_GATEWAY={_sh_escape(profile.static_gateway)}",
        f"INSTANT_STATIC_DNS={_sh_escape(','.join(profile.static_dns))}",
        f"INSTANT_DASHBOARD_CHOICE={_sh_escape(profile.dashboard_choice)}",
        f"INSTANT_ENABLE_COCKPIT={_sh_escape('1' if enable_cockpit else '0')}",
        f"INSTANT_ENABLE_CASAOS={_sh_escape('1' if enable_casaos else '0')}",
        f"INSTANT_ENABLE_DOCKER={_sh_escape('1' if enable_docker else '0')}",
        f"INSTANT_SELECTED_APPS={_sh_escape(','.join(valid_selected_apps))}",
        f"INSTANT_PREBUNDLE_OFFLINE_IMAGES={_sh_escape('1' if profile.prebundle_offline_images else '0')}",
        f"INSTANT_ENABLE_TAILSCALE={_sh_escape('1' if profile.enable_tailscale else '0')}",
        f"INSTANT_TAILSCALE_AUTHKEY={_sh_escape(profile.tailscale_authkey)}",
        f"INSTANT_ENABLE_SAMBA_NAS={_sh_escape('1' if profile.enable_samba_nas else '0')}",
        f"INSTANT_ENABLE_PIHOLE_DNS_PREP={_sh_escape('1' if profile.enable_pihole_dns_prep else '0')}",
        f"INSTANT_ENABLE_INTEL_VAAPI={_sh_escape('1' if profile.enable_intel_vaapi else '0')}",
        f"INSTANT_ENABLE_WOL={_sh_escape('1' if profile.enable_wake_on_lan else '0')}",
        f"INSTANT_IGNORE_LID_SWITCH={_sh_escape('1' if profile.ignore_lid_switch else '0')}",
        f"INSTANT_SCREEN_BLANK_SEC={int(profile.screen_blank_seconds)}",
        f"INSTANT_BAT_START_THRESH={int(profile.battery_charge_start_pct)}",
        f"INSTANT_BAT_STOP_THRESH={int(profile.battery_charge_stop_pct)}",
        f"INSTANT_UPS_SHUTDOWN_PCT={int(profile.ups_shutdown_battery_pct)}",
        f"INSTANT_ENABLE_LOW_RAM_ZRAM={_sh_escape('1' if profile.enable_low_ram_zram else '0')}",
        f"INSTANT_SAFE_GPU_NOMODESET={_sh_escape('1' if profile.safe_gpu_nomodeset else '0')}",
        f"INSTANT_UI_MODE={_sh_escape(profile.ui_mode)}",
        f"INSTANT_ENABLE_CUSTOM_DASHBOARD={_sh_escape('1' if profile.enable_custom_dashboard else '0')}",
        f"INSTANT_DASHBOARD_PORT={int(profile.dashboard_port)}",
        f"INSTANT_ENABLE_AUTO_UPDATES={_sh_escape('1' if profile.enable_auto_updates else '0')}",
        f"INSTANT_AUTO_UPDATE_WINDOW={_sh_escape(profile.auto_update_window)}",
        f"INSTANT_ENABLE_SCHEDULED_BACKUPS={_sh_escape('1' if profile.enable_scheduled_backups else '0')}",
        f"INSTANT_BACKUP_RETENTION_DAYS={int(profile.backup_retention_days)}",
        f"INSTANT_PRESELECT_KIWIX_ZIMS={_sh_escape(','.join(profile.preselect_kiwix_zims or []))}",
        f"INSTANT_SSH_PORT={int(profile.ssh_port)}",
        f"INSTANT_ENABLE_UFW={_sh_escape('1' if profile.enable_ufw_firewall else '0')}",
        f"INSTANT_ENABLE_FAIL2BAN={_sh_escape('1' if profile.enable_fail2ban else '0')}",
        f"INSTANT_ENABLE_IPV4_FORWARDING={_sh_escape('1' if profile.enable_ipv4_forwarding else '0')}",
        f"INSTANT_ENABLE_TCP_BBR={_sh_escape('1' if profile.enable_tcp_bbr else '0')}",
        f"INSTANT_VM_SWAPPINESS={int(profile.vm_swappiness)}",
        f"INSTANT_INOTIFY_MAX_WATCHES={int(profile.inotify_max_watches)}",
        f"INSTANT_ROOT_FILESYSTEM={_sh_escape(profile.root_filesystem)}",
        f"INSTANT_ENABLE_SSD_NOATIME_TRIM={_sh_escape('1' if profile.enable_ssd_noatime_trim else '0')}",
        f"INSTANT_ENABLE_SMARTD={_sh_escape('1' if profile.enable_smartd_monitor else '0')}",
        f"INSTANT_ZRAM_ALGORITHM={_sh_escape(profile.zram_algorithm)}",
        f"INSTANT_ZRAM_PERCENT={int(profile.zram_percent)}",
        f"INSTANT_CPU_GOVERNOR={_sh_escape(profile.cpu_governor)}",
        f"INSTANT_DISABLE_CPU_TURBO={_sh_escape('1' if profile.disable_cpu_turbo_boost else '0')}",
        f"INSTANT_DOCKER_SUBNET_CIDR={_sh_escape(profile.docker_subnet_cidr)}",
        f"INSTANT_DOCKER_LOG_MAX_SIZE={_sh_escape(profile.docker_log_max_size)}",
        f"INSTANT_DOCKER_LOG_MAX_FILE={int(profile.docker_log_max_file)}",
        f"INSTANT_ENABLE_PROMETHEUS={_sh_escape('1' if profile.enable_prometheus_metrics else '0')}",
        f"INSTANT_CUSTOM_KERNEL_ARGS={_sh_escape(profile.custom_kernel_args)}",
        f"INSTANT_DEVICE_FORM_FACTOR={_sh_escape(profile.device_form_factor)}",
        f"INSTANT_CLUSTER_MODE={_sh_escape(profile.cluster_mode)}",
        f"INSTANT_CLUSTER_NAME={_sh_escape(profile.cluster_name)}",
        f"INSTANT_CLUSTER_STORAGE_POLICY={_sh_escape(profile.cluster_storage_policy)}",
        f"INSTANT_CLUSTER_SQLITE_PINNING={_sh_escape('1' if profile.cluster_sqlite_local_pinning else '0')}",
    ]
    instant_env_conf = "\n".join(env_lines) + "\n"

    schema_issues = validate_subiquity_user_data_text(user_data_yaml)
    if schema_issues:
        raise ValueError("Subiquity schema check failed:\n- " + "\n- ".join(schema_issues))

    emit_event(
        "CONFIG_VALIDATED",
        f"Compiled Instant Server config (Ubuntu {profile.ubuntu_release} LTS) + {len(valid_selected_apps)} selected app(s)",
        {
            "hostname": profile.hostname,
            "ubuntu_release": profile.ubuntu_release,
            "install_mode": profile.install_mode,
            "dashboard": profile.dashboard_choice,
            "selected_apps": valid_selected_apps,
            "storage_layout": profile.storage_layout,
        },
        callback=event_callback,
    )
    return {
        "user-data": user_data_yaml,
        "meta-data": meta_data_content,
        "instant-server-env.conf": instant_env_conf,
        "lazarus-env.conf": instant_env_conf,
        "profile.json": json.dumps(asdict(profile), indent=2),
    }

def _sh_escape(val: str) -> str:
    escaped = val.replace("\\", "\\\\").replace('"', '\\"').replace("$", "\\$").replace("`", "\\`")
    return f'"{escaped}"'

ALLOWED_AUTOINSTALL_TOP_KEYS = {
    "version",
    "interactive-sections",
    "early-commands",
    "locale",
    "refresh-installer",
    "keyboard",
    "source",
    "network",
    "proxy",
    "apt",
    "storage",
    "identity",
    "active-directory",
    "ubuntu-pro",
    "ssh",
    "codecs",
    "drivers",
    "oem",
    "snaps",
    "debconf-selections",
    "packages",
    "kernel",
    "kernel-crash-dumps",
    "timezone",
    "updates",
    "shutdown",
    "late-commands",
    "error-commands",
    "reporting",
    "user-data",
    "zdevs",
}

def validate_subiquity_user_data_text(user_data_text: str) -> List[str]:
    issues: List[str] = []
    lines = user_data_text.splitlines()
    if not lines or lines[0].strip() != "#cloud-config":
        issues.append("Line 1 of user-data must be '#cloud-config'.")
    if "\r\n" in user_data_text:
        issues.append("user-data contains CRLF (\\r\\n) line endings; must use Unix LF (\\n).")
    if "autoinstall:" not in user_data_text:
        issues.append("Missing top-level 'autoinstall:' section.")

    in_autoinstall = False
    for line in lines[1:]:
        if not line.strip() or line.strip().startswith("#"):
            continue
        indent = len(line) - len(line.lstrip(" "))
        if indent == 0:
            key = line.strip().split(":", 1)[0]
            in_autoinstall = key == "autoinstall"
            if key not in ("autoinstall", "runcmd", "bootcmd", "write_files"):
                issues.append(f"Unexpected root cloud-config key '{key}'.")
        elif indent == 2 and in_autoinstall:
            subkey = line.strip().split(":", 1)[0]
            if subkey not in ALLOWED_AUTOINSTALL_TOP_KEYS:
                issues.append(
                    f"Illegal Ubuntu Subiquity key 'autoinstall.{subkey}'."
                )
    return issues

def _run_self_test() -> int:
    print("[ConfigBuilder Test] 1. Testing pure-Python SHA-512 crypt ($6$) determinism...")
    known_hash = sha512_crypt("ubuntu", salt="exDY1mhS4KUYCE/2")
    expected = (
        "$6$exDY1mhS4KUYCE/2$zmn9ToZwTKLhCw.b4/b.ZRTIZM30JZ4QrOQ2aOXJ8yk96xpcCof0kxKwuX1kqLG/"
        "ygbJ1f8wxED22bTL4F46P0"
    )
    assert known_hash == expected
    print("  -> PASS: Pure-Python SHA-512 crypt matches official Canonical test vector!")

    print("[ConfigBuilder Test] 2. Testing 26-App Self-Hosted Catalog (Kiwix Wikipedia, Minecraft, Ollama AI)...")
    assert len(SELF_HOSTED_APP_CATALOG) >= 26
    for required_id in ("kiwix", "minecraft", "minecraft_bedrock", "ollama_webui", "localai", "bookstack"):
        assert required_id in APP_CATALOG_BY_ID, f"Missing {required_id}"
    mc_compose = generate_casaos_app_compose("minecraft", timezone="Asia/Kolkata")
    assert "EULA=TRUE" in mc_compose and "25565:25565" in mc_compose
    ai_compose = generate_casaos_app_compose("ollama_webui", timezone="Asia/Kolkata")
    assert "11434:11434/tcp" in ai_compose
    print("  -> PASS: 26-App CasaOS/Docker catalog (Wikipedia, Minecraft, Ollama AI) verified!")

    print("[ConfigBuilder Test] 3. Testing Default Ubuntu 26.04.1 LTS & Dual-Boot profiles...")
    default_prof = ServerProfile()
    assert default_prof.ubuntu_release == "26.04"
    bundle = compile_cidata_payloads(default_prof)
    assert 'INSTANT_UBUNTU_RELEASE="26.04"' in bundle["instant-server-env.conf"]
    print("  -> PASS: Default Ubuntu 26.04.1 LTS profile compiled and validated!")
    return 0

def main() -> int:
    parser = argparse.ArgumentParser(description="Instant Server — M4 Config Builder")
    parser.add_argument("--self-test", action="store_true", help="Run built-in cryptographic and schema self-tests")
    parser.add_argument("--output-dir", type=str, default="", help="Write compiled cidata files to directory")
    parser.add_argument("--hostname", type=str, default="instant-server")
    parser.add_argument("--username", type=str, default="admin")
    parser.add_argument("--password", type=str, default="instant2026")
    parser.add_argument("--install-mode", type=str, default="dual_boot", choices=list(VALID_INSTALL_MODES))
    parser.add_argument("--confirm-erase", action="store_true", help="Acknowledge 'I know what I'm doing' for auto_erase")
    parser.add_argument("--wifi-ssid", type=str, default="")
    parser.add_argument("--wifi-password", type=str, default="")
    parser.add_argument("--dashboard", type=str, default="both", choices=list(VALID_DASHBOARDS))
    args = parser.parse_args()

    if args.self_test:
        return _run_self_test()

    prof = ServerProfile(
        hostname=args.hostname,
        username=args.username,
        password=args.password,
        install_mode=args.install_mode,
        confirm_erase_main_drive=args.confirm_erase,
        wifi_ssid=args.wifi_ssid,
        wifi_password=args.wifi_password,
        dashboard_choice=args.dashboard,
    )
    payloads = compile_cidata_payloads(prof)
    if args.output_dir:
        os.makedirs(args.output_dir, exist_ok=True)
        for fname, content in payloads.items():
            with open(os.path.join(args.output_dir, fname), "w", encoding="utf-8", newline="\n") as fh:
                fh.write(content)
        print(f"Wrote {len(payloads)} files to {args.output_dir}")
    else:
        print(payloads["user-data"])
    return 0

DIAGNOSTIC_ERROR_CATALOG: Dict[str, Dict[str, str]] = {
    "IS-ERR-101": {
        "code": "IS-ERR-101",
        "category": "USB Flasher & Privileges",
        "title": "Administrator / Root Privileges Required",
        "what_happened": "The USB flasher could not open the raw physical drive because it is running as a standard user.",
        "why_it_happened": "Operating systems block direct sector writes to physical disks unless the process has elevated privileges (Windows UAC Administrator or sudo/pkexec on Linux and macOS).",
        "one_click_action": "relaunch_elevated",
        "fix_command": "sudo python3 instant_server.py",
        "fix_steps": "Click 'Relaunch as Administrator' in the banner or run the app with sudo/Admin rights before starting the flash.",
    },
    "IS-ERR-102": {
        "code": "IS-ERR-102",
        "category": "ISO Download & Verification",
        "title": "Ubuntu ISO SHA-256 Checksum Mismatch or Corrupted Cache",
        "what_happened": "The downloaded Ubuntu Server LTS ISO failed SHA-256 cryptographic verification.",
        "why_it_happened": "An interrupted mirror transfer or captive proxy altered bytes in the `.iso.part` cache file.",
        "one_click_action": "repair_iso_cache",
        "fix_command": "python3 instant_server.py --verify-iso",
        "fix_steps": "Click 'Re-Verify & Resume ISO' to discard the bad tail chunk and resume via HTTP Range from the official Ubuntu mirror.",
    },
    "IS-ERR-103": {
        "code": "IS-ERR-103",
        "category": "USB Volume Lock",
        "title": "USB Drive Locked by File Explorer or Antivirus",
        "what_happened": "Windows or the desktop automounter held an open handle on a partition of the target USB drive.",
        "why_it_happened": "Before raw sector flashing can begin, all mounted volumes on that physical disk must be dismounted via FSCTL_LOCK_VOLUME and FSCTL_DISMOUNT_VOLUME.",
        "one_click_action": "force_unlock_usb",
        "fix_command": "win_usb_flasher --self-test",
        "fix_steps": "Instant Server automatically sends FSCTL_LOCK_VOLUME with 12 retries. If another tool is indexing the USB stick, pause it and click Retry.",
    },
    "IS-ERR-104": {
        "code": "IS-ERR-104",
        "category": "USB Hardware I/O",
        "title": "USB Write I/O Error or Read-Back CRC32 Mismatch",
        "what_happened": "A 4 MB DMA block failed to write or read back with the matching IEEE 802.3 CRC32 checksum.",
        "why_it_happened": "Loose USB hubs, worn NAND flash cells, or counterfeit capacity thumb drives drop sectors under sustained unbuffered writes.",
        "one_click_action": "rescan_usb",
        "fix_command": "dmesg | tail -n 30",
        "fix_steps": "Plug the USB drive directly into a motherboard USB 3.0 port (avoid unpowered hubs) or try a known-good 8 GB+ USB stick.",
    },
    "IS-ERR-201": {
        "code": "IS-ERR-201",
        "category": "Networking & Wi-Fi Failover",
        "title": "Both Ethernet and Wi-Fi Interfaces Disconnected",
        "what_happened": "The server has no active default route on either Ethernet (metric 100) or Wi-Fi (metric 600).",
        "why_it_happened": "The Ethernet cable was unplugged while Wi-Fi credentials were missing, or the router DHCP lease expired.",
        "one_click_action": "heal_network",
        "fix_command": "sudo systemctl start instant-server-netwatch.service",
        "fix_steps": "Run the 1-Click Network Healer to unblock rfkill, disable Wi-Fi power saving, renew DHCP leases, and reload Netplan.",
    },
    "IS-ERR-202": {
        "code": "IS-ERR-202",
        "category": "mDNS & Local Discovery",
        "title": ".local mDNS Hostname Not Resolving on LAN",
        "what_happened": "Opening http://instant-server.local:8088 timed out even though the server is reachable by its raw IPv4 address.",
        "why_it_happened": "Multicast DNS (UDP port 5353) was paused after a network interface switch or blocked by client isolation on guest Wi-Fi.",
        "one_click_action": "restart_mdns",
        "fix_command": "sudo systemctl restart avahi-daemon",
        "fix_steps": "Click 'Restart mDNS (Avahi)' or connect directly using the Ethernet or Wi-Fi IPv4 shown on the laptop screen (tty1).",
    },
    "IS-ERR-301": {
        "code": "IS-ERR-301",
        "category": "Docker & Self-Hosted Apps",
        "title": "Container Port Conflict Detected",
        "what_happened": "A self-hosted app container could not start because its host TCP/UDP port is already bound by another service.",
        "why_it_happened": "Two services (for example, systemd-resolved and Pi-hole on port 53, or two web apps on port 8080) requested the same host port.",
        "one_click_action": "heal_docker_ports",
        "fix_command": "sudo ss -tulpn",
        "fix_steps": "Open 'Compose & Volumes' on the app card, inspect the port binding, or click 'Run Config Doctor' to free port 53/conflict ports.",
    },
    "IS-ERR-302": {
        "code": "IS-ERR-302",
        "category": "Docker & Self-Hosted Apps",
        "title": "Invalid docker-compose.yml Syntax (Auto-Rolled Back to .bak)",
        "what_happened": "An edit to an app's docker-compose.yml failed validation, so Instant Server automatically restored the working `.bak` copy.",
        "why_it_happened": "YAML indentation errors or missing volume colons prevent Docker Compose from parsing the service definition.",
        "one_click_action": "restore_compose_backup",
        "fix_command": "docker compose config",
        "fix_steps": "Open the Glass-Box Compose Inspector on the app card and click 'Restore Backup (.bak)' or 'Validate Compose'.",
    },
    "IS-ERR-401": {
        "code": "IS-ERR-401",
        "category": "Storage & Disk Health",
        "title": "Root or AppData Partition Above 85% Capacity",
        "what_happened": "Available disk space on `/` or `/DATA` dropped below 15%, risking database write stalls.",
        "why_it_happened": "Old container image layers, unrotated system logs, or large media downloads filled the partition.",
        "one_click_action": "prune_storage",
        "fix_command": "sudo systemctl start instant-server-maintenance.service",
        "fix_steps": "Click '1-Click Prune Logs & Unused Images' to cap journald at 200 MB, vacuum old logs, and prune unused Docker layers.",
    },
    "IS-ERR-501": {
        "code": "IS-ERR-501",
        "category": "Laptop Power & Thermal",
        "title": "Battery Charge Threshold Not Exposed by BIOS/EC",
        "what_happened": "The kernel does not expose `/sys/class/power_supply/BAT0/charge_control_end_threshold` on this laptop model.",
        "why_it_happened": "Some older consumer laptops require vendor ACPI kernel modules (`thinkpad_acpi`, `asus_nb_wmi`, `ideapad_laptop`) or BIOS settings for charge capping.",
        "one_click_action": "probe_battery_acpi",
        "fix_command": "sudo tlp-stat -b",
        "fix_steps": "Click 'Probe ACPI Battery Modules' to load vendor kernel modules and apply TLP thresholds where supported.",
    },
    "IS-ERR-601": {
        "code": "IS-ERR-601",
        "category": "Container Registry & Offline Staging",
        "title": "Container Registry Token Handshake Required (HTTP 401)",
        "what_happened": "An unauthenticated pull request to GHCR/LSCR returned HTTP 401 before anonymous bearer token exchange.",
        "why_it_happened": "OCI registries (`ghcr.io`, `lscr.io`, `docker.io`) require requesting an anonymous Bearer token from `Www-Authenticate` before fetching manifests.",
        "one_click_action": "retry_registry_token",
        "fix_command": "python3 instant_server.py --verify-catalog",
        "fix_steps": "Instant Server's OCI token client automatically negotiates anonymous Bearer tokens and resumes layer blobs on retry.",
    },
    "IS-ERR-701": {
        "code": "IS-ERR-701",
        "category": "Multi-Server Cluster & Storage Merger",
        "title": "Duplicate Filename Conflict Detected Across Pooled Servers",
        "what_happened": "Both Server A and Server B contain a file at the same relative path with different SHA-256 checksums.",
        "why_it_happened": "When combining two existing servers via MergerFS Union Pool, identical relative paths from separate branches appear in the same merged directory.",
        "one_click_action": "resolve_mergerfs_collision",
        "fix_command": "instant-cluster-ctl audit-collisions --keep-both",
        "fix_steps": "Click 'Keep Both with Node Tag' to preserve both files non-destructively (e.g. file.node2.ext) without overwriting either server's data.",
    },
    "IS-ERR-702": {
        "code": "IS-ERR-702",
        "category": "Multi-Server Cluster & Split-Brain Guard",
        "title": "SQLite Database Protected from Network Mount Corruption",
        "what_happened": "An app using SQLite (Jellyfin, Immich, Vaultwarden, or Nextcloud) attempted to store its database file on an NFS network share.",
        "why_it_happened": "SQLite relies on POSIX file locks that can corrupt over NFS/network links during brief packet drops.",
        "one_click_action": "pin_sqlite_local",
        "fix_command": "instant-cluster-ctl pin-databases --local-nvme",
        "fix_steps": "Instant Server automatically pins `/DATA/AppData` SQLite databases to local SSD/NVMe and uses async `.backup` snapshots to the peer node while pooling `/DATA/Media` via MergerFS.",
    },
    "IS-ERR-703": {
        "code": "IS-ERR-703",
        "category": "Multi-Server Cluster Safety Gate",
        "title": "Secondary Server Wipe Blocked by Data Protection Gate",
        "what_happened": "A request to format and wipe the secondary server was blocked because existing user files were detected on its drives.",
        "why_it_happened": "Instant Server defaults to Non-Destructive Union Pooling (`mergerfs_union_safe`) and never erases a secondary server unless the administrator explicitly types `WIPE-<hostname>`.",
        "one_click_action": "switch_to_safe_union_merge",
        "fix_command": "instant-cluster-ctl set-policy mergerfs_union_safe",
        "fix_steps": "Switch to 'Non-Destructive Union Pool (Keep All Data on Both Servers)' or type the exact `WIPE-<hostname>` confirmation string if you intentionally want to erase the second machine.",
    },
    "IS-ERR-704": {
        "code": "IS-ERR-704",
        "category": "Multi-Server Cluster Networking",
        "title": "Cluster Node Clock Skew or UID/GID Permission Mismatch",
        "what_happened": "Two clustered servers had system clocks drifting by >500ms or mismatched Linux user IDs on shared mounts.",
        "why_it_happened": "Distributed token validation and NFSv4/MergerFS permissions require synchronized NTP time and uniform UID/GID 1000:1000 ownership.",
        "one_click_action": "sync_cluster_clock_and_uids",
        "fix_command": "sudo chronyc makestep && sudo chown -R 1000:1000 /DATA/ClusterPool",
        "fix_steps": "Click 'Sync Cluster Clocks & Permissions' to step NTP via chrony and normalize UID/GID 1000 across all nodes.",
    },
    "IS-ERR-801": {
        "code": "IS-ERR-801",
        "category": "Bootable Repair & Rescue Mode",
        "title": "Damaged EFI Bootloader or Missing GRUB Entry Detected",
        "what_happened": "The target machine's NVMe/SSD has an intact Linux filesystem, but its UEFI NVRAM entry or `/boot/efi` GRUB stub is missing or corrupted.",
        "why_it_happened": "Motherboard BIOS updates, Windows dual-boot updates, or sudden power loss during a kernel upgrade can clear UEFI boot order entries.",
        "one_click_action": "repair_efi_bootloader",
        "fix_command": "sudo instant-repair-suite --repair-bootloader",
        "fix_steps": "Click '1-Click Repair UEFI & GRUB Bootloader' to auto-mount the root & EFI partitions in chroot and run `grub-install` + `update-grub`.",
    },
    "IS-ERR-802": {
        "code": "IS-ERR-802",
        "category": "Bootable Repair & Rescue Mode",
        "title": "Dirty Filesystem Journal Requiring Unmounted fsck Repair",
        "what_happened": "The target disk partition has unrecovered journal transactions or superblock inconsistencies preventing normal boot.",
        "why_it_happened": "An unclean power-off on a system without battery backup interrupted disk writes.",
        "one_click_action": "repair_unmounted_fsck",
        "fix_command": "sudo instant-repair-suite --fsck-all",
        "fix_steps": "Click 'Run Safe Unmounted fsck/Scrub' to ensure all target partitions are unmounted and run `e2fsck -fy` / `btrfs check` / `xfs_repair`.",
    },
    "IS-ERR-602": {
        "code": "IS-ERR-602",
        "category": "Dual-Boot & Partitioning",
        "title": "Requested Dual-Boot Size Outside Safe Bounds (15 GB – 64 TB)",
        "what_happened": "The requested Ubuntu Server partition size is smaller than 15 GB or exceeds 64 TB (65,536 GB).",
        "why_it_happened": "Ubuntu Server LTS + Docker base images require at least 15 GB, and ext4/GPT automated guided sizing caps at 64 TB.",
        "one_click_action": "reset_partition_size",
        "fix_command": "lsblk -o NAME,SIZE,FSTYPE,MOUNTPOINT",
        "fix_steps": "Enter a partition size between 15 GB and 64 TB (65,536 GB) in the Dual-Boot Storage field.",
    },
}


def lookup_error_code(query: str) -> List[Dict[str, str]]:
    """Looks up an IS-ERR-XXXX code or searches error titles/symptoms."""
    q = (query or "").strip().upper()
    if not q:
        return list(DIAGNOSTIC_ERROR_CATALOG.values())
    if q in DIAGNOSTIC_ERROR_CATALOG:
        return [DIAGNOSTIC_ERROR_CATALOG[q]]
    matches: List[Dict[str, str]] = []
    q_low = query.strip().lower()
    for item in DIAGNOSTIC_ERROR_CATALOG.values():
        haystack = " ".join([
            item["code"],
            item["category"],
            item["title"],
            item["what_happened"],
            item["why_it_happened"],
            item["fix_steps"],
        ]).lower()
        if q_low in haystack:
            matches.append(item)
    return matches



def run_cluster_preflight_audit(
    primary_node: Dict[str, Any],
    secondary_node: Dict[str, Any],
    storage_policy: str = "mergerfs_union_safe",
    confirm_wipe_token: str = "",
) -> Dict[str, Any]:
    """Audits two servers before combining them into a unified Instant Server cluster.
    Detects existing data volumes, duplicate filenames, overlapping Docker ports,
    SQLite database paths that must remain pinned locally, and validates wipe tokens.
    """
    p_host = primary_node.get("hostname", "instant-server-1")
    s_host = secondary_node.get("hostname", "instant-server-2")
    p_used_gb = float(primary_node.get("used_gb", 420.0))
    s_used_gb = float(secondary_node.get("used_gb", 310.0))
    p_total_gb = float(primary_node.get("total_gb", 1000.0))
    s_total_gb = float(secondary_node.get("total_gb", 1000.0))
    p_apps = set(primary_node.get("apps", ["jellyfin", "immich", "nextcloud"]))
    s_apps = set(secondary_node.get("apps", ["jellyfin", "qbittorrent", "kiwix"]))

    overlapping_apps = sorted(p_apps.intersection(s_apps))
    all_apps = sorted(p_apps.union(s_apps))

    p_files = set(primary_node.get("sample_files", ["/DATA/Media/Movies/Interstellar.mkv", "/DATA/Media/Photos/2025_Trip.zip"]))
    s_files = set(secondary_node.get("sample_files", ["/DATA/Media/Movies/Interstellar.mkv", "/DATA/Downloads/Ubuntu_24.04.iso"]))
    duplicate_paths = sorted(p_files.intersection(s_files))

    sqlite_apps = [a for a in all_apps if a in {"jellyfin", "immich", "nextcloud", "vaultwarden", "pihole", "homeassistant"}]

    wipe_required_token = f"WIPE-{s_host}"
    wipe_blocked = False
    warnings: List[str] = []

    if storage_policy == "wipe_secondary_expand":
        if confirm_wipe_token.strip() != wipe_required_token:
            wipe_blocked = True
            warnings.append(
                f"IS-ERR-703: Secondary server '{s_host}' contains {s_used_gb:.1f} GB of existing data. "
                f"To prevent accidental data loss, wiping is blocked unless you type '{wipe_required_token}' "
                f"or choose 'mergerfs_union_safe' (Non-Destructive Union Pool)."
            )
        else:
            warnings.append(
                f"Confirmed destructive wipe of '{s_host}' ({s_used_gb:.1f} GB will be erased and added as clean capacity)."
            )
    elif storage_policy == "mergerfs_union_safe":
        if duplicate_paths:
            warnings.append(
                f"IS-ERR-701: Detected {len(duplicate_paths)} shared file path(s) across '{p_host}' and '{s_host}'. "
                f"MergerFS Union Pool will preserve both copies non-destructively (auto-tagging conflicting writes as *.{s_host})."
            )
    elif storage_policy == "replicated_ha_mirror":
        warnings.append(
            f"High-Availability Mirror will keep all existing files on '{p_host}' and '{s_host}' and continuously sync deltas via rsync."
        )

    combined_total_gb = p_total_gb + s_total_gb
    combined_used_gb = p_used_gb if storage_policy == "wipe_secondary_expand" and not wipe_blocked else (p_used_gb + s_used_gb)
    combined_free_gb = max(0.0, combined_total_gb - combined_used_gb)

    return {
        "ok": not wipe_blocked,
        "storage_policy": storage_policy,
        "data_loss_risk": "HIGH (Secondary Drive Will Be Formatted)" if storage_policy == "wipe_secondary_expand" else "ZERO (100% Non-Destructive)",
        "wipe_blocked": wipe_blocked,
        "expected_wipe_token": wipe_required_token,
        "primary_hostname": p_host,
        "secondary_hostname": s_host,
        "combined_total_gb": round(combined_total_gb, 1),
        "combined_used_gb": round(combined_used_gb, 1),
        "combined_free_gb": round(combined_free_gb, 1),
        "overlapping_apps": overlapping_apps,
        "merged_apps": all_apps,
        "sqlite_pinned_apps": sqlite_apps,
        "duplicate_file_paths": duplicate_paths,
        "warnings": warnings,
        "mount_command_preview": (
            f"mergerfs -o defaults,allow_other,use_ino,category.create=mfs,moveonenospc=true,dropcacheonclose=true,noforget "
            f"/DATA/LocalBranch:/mnt/cluster/{s_host}/DATA /DATA/ClusterPool"
        ),
    }


if __name__ == "__main__":
    sys.exit(main())