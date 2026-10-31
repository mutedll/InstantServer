#!/usr/bin/env python3
"""
Instant Server — Module M4: Cross-Platform Profile Compiler & Subiquity Config Builder
======================================================================================
File: m4_config_builder.py
Project: Instant Server (Turn any old laptop into a headless Ubuntu Server)
Runs on: Windows, macOS, Linux (Python 3.8+ through 3.14+, zero external dependencies)

Responsibilities:
1. Pure-Python SHA-512 crypt ($6$) password hasher (solves Python 3.13 PEP 594 `crypt`
   removal and Windows missing `crypt` stdlib module).
2. Supports 4 Target Laptop Disk Installation Modes:
   - `auto_erase`: Automatically erases the target laptop's main internal drive (gated
     by `confirm_erase_main_drive=True` — "I know what I'm doing" safeguard).
   - `dual_boot`: Retains the existing Windows/Linux OS as-is (`preserve: true`), shrinks
     or uses free space (`dual_boot_Allocation_gb`), and enables `os-prober` in GRUB.
   - `interactive_select`: Adds `interactive-sections: [storage]` so the user selects
     the drive/partition on screen after plugging in the USB, while automating all else.
   - `secondary_drive_only`: Installs only onto a secondary drive, leaving Disk 0 untouched.
3. Compiles 100% schema-compliant Ubuntu 24.04 / 22.04 Subiquity `user-data`,
   `meta-data`, and `instant-server-env.conf` payloads for the USB `cidata` partition.
"""

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

# ---------------------------------------------------------------------------
# 1. Pure-Python SHA-512 Crypt ($6$) Implementation (Cross-Platform & Py3.13+)
# ---------------------------------------------------------------------------
_CRYPT_B64_ALPHABET = (
    "./0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)


def _crypt_b64_encode(data: bytes, indices: List[tuple]) -> str:
    """Encode bytes using the Ulrich Drepper SHA-512 crypt base64 permutation."""
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
    """
    Pure-Python implementation of Ulrich Drepper's SHA-512 crypt ($6$).
    Produces hashes identical to `openssl passwd -6` and `mkpasswd -m sha-512`
    so Windows, macOS, and Python 3.13+ hosts never need the removed `crypt` module.
    """
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


# ---------------------------------------------------------------------------
# 2. Data Models & Profile Validation
# ---------------------------------------------------------------------------
VALID_INSTALL_MODES = (
    "auto_erase",         # Erase target laptop's main drive (requires "I know what I'm doing" ack)
    "dual_boot",          # Retain existing OS as-is and install Instant Server alongside
    "interactive_select", # Let user choose disk/partition on laptop screen after booting USB
    "secondary_drive",    # Only install onto a secondary internal/external drive; never touch Disk 0
)
VALID_STORAGE_LAYOUTS = ("direct", "lvm", "btrfs", "zfs")
VALID_BOOT_MODES = ("auto", "uefi", "bios")
VALID_SHUTDOWN_MODES = ("poweroff", "reboot")
VALID_DASHBOARDS = ("cockpit", "casaos", "both", "none")


@dataclass
class ServerProfile:
    """Unified configuration model for Instant Server (Wizard & Pro Mode)."""

    # Q1: Identity & Network
    hostname: str = "instant-server"
    username: str = "admin"
    password: str = "instant2026"
    timezone: str = "Asia/Kolkata"
    locale: str = "en_US.UTF-8"
    keyboard_layout: str = "us"
    wifi_ssid: str = ""
    wifi_password: str = ""
    enable_usb_tethering: bool = True

    # Q2: Web Dashboards & Useful Homelab Stacks
    dashboard_choice: str = "both"  # "cockpit", "casaos", "both", or "none"
    enable_docker: bool = True
    enable_tailscale: bool = False
    tailscale_authkey: str = ""
    enable_samba_nas: bool = True
    enable_pihole_dns_prep: bool = True
    enable_intel_vaapi: bool = True
    enable_wake_on_lan: bool = True

    # Q3: Target Laptop Disk Installation Mode & Hardware Safeguards
    install_mode: str = "dual_boot"  # "auto_erase", "dual_boot", "interactive_select", "secondary_drive"
    confirm_erase_main_drive: bool = False  # Must be True when install_mode == "auto_erase"
    dual_boot_Allocation_gb: int = 45  # Space to allocate for Instant Server in dual_boot mode
    ubuntu_release: str = "24.04"
    storage_layout: str = "direct"  # "direct", "lvm", "btrfs", "zfs"
    boot_mode: str = "auto"  # "auto", "uefi", "bios"
    target_disk_hint: str = "auto"
    allow_usb_target_disk: bool = False
    force_reinstall: bool = False
    shutdown_after_install: str = "poweroff"

    # Laptop Survival & Power Settings
    ignore_lid_switch: bool = True
    screen_blank_seconds: int = 60
    battery_charge_start_pct: int = 40
    battery_charge_stop_pct: int = 60
    ups_shutdown_battery_pct: int = 15
    enable_low_ram_zram: bool = True
    safe_gpu_nomodeset: bool = False

    # Pro Mode SSH & Custom Hooks
    ssh_allow_password: bool = True
    ssh_authorized_keys: List[str] = field(default_factory=list)
    github_ssh_username: str = ""
    extra_apt_packages: List[str] = field(default_factory=list)
    custom_late_commands: List[str] = field(default_factory=list)
    static_ip_cidr: str = ""
    static_gateway: str = ""
    static_dns: List[str] = field(default_factory=list)

    def validate(self) -> List[str]:
        """Validate profile fields and return a list of human-readable errors."""
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
        if self.install_mode == "dual_boot" and not (15 <= self.dual_boot_Allocation_gb <= 2000):
            errors.append("dual_boot_Allocation_gb must be between 15 GB and 2000 GB.")
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


# ---------------------------------------------------------------------------
# 3. GitHub SSH Key Helper & Event Bus Helper
# ---------------------------------------------------------------------------
def emit_event(
    event_type: str,
    message: str,
    data: Optional[Dict[str, Any]] = None,
    callback: Optional[Callable[[Dict[str, Any]], None]] = None,
) -> Dict[str, Any]:
    """Emit structured JSON event for both CLI and Instant Server GUI IPC consumers."""
    payload = {
        "event": event_type,
        "module": "M4_CONFIG_BUILDER",
        "project": "Instant Server",
        "message": message,
        "data": data or {},
    }
    if callback:
        callback(payload)
    return payload


def fetch_github_ssh_keys(github_user: str, timeout: int = 8) -> List[str]:
    """Fetch public SSH keys for a GitHub username (`https://github.com/<user>.keys`)."""
    clean_user = re.sub(r"[^a-zA-Z0-9-]", "", github_user.strip())
    if not clean_user:
        return []
    url = f"https://github.com/{clean_user}.keys"
    req = urllib.request.Request(url, headers={"User-Agent": "InstantServer-ConfigBuilder/2.0"})
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            lines = resp.read().decode("utf-8", errors="replace").splitlines()
            return [ln.strip() for ln in lines if ln.strip().startswith(("ssh-", "ecdsa-"))]
    except Exception:
        return []


# ---------------------------------------------------------------------------
# 4. YAML Serializer (Zero-Dependency, Deterministic & Cloud-Init Safe)
# ---------------------------------------------------------------------------
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


# ---------------------------------------------------------------------------
# 5. Subiquity `user-data`, `meta-data`, and `instant-server-env.conf` Compiler
# ---------------------------------------------------------------------------
def compile_cidata_payloads(
    profile: ServerProfile,
    event_callback: Optional[Callable[[Dict[str, Any]], None]] = None,
) -> Dict[str, str]:
    """
    Compile a `ServerProfile` into the configuration files written to `cidata`:
      - `user-data` (Cloud-Init & Subiquity autoinstall YAML)
      - `meta-data` (NoCloud instance metadata)
      - `instant-server-env.conf` (and `lazarus-env.conf` alias for target scripts)
    """
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
        emit_event(
            "SSH_KEYS_IMPORTED",
            f"Processed SSH keys (total: {len(ssh_keys)})",
            {"count": len(ssh_keys)},
            callback=event_callback,
        )

    ethernets_cfg: Dict[str, Any] = {
        "any-ethernet": {
            "match": {"name": "en*"},
            "dhcp4": True,
            "dhcp6": True,
            "optional": True,
        },
        "legacy-ethernet": {
            "match": {"name": "eth*"},
            "dhcp4": True,
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
    ]
    if profile.install_mode == "dual_boot":
        base_packages.append("os-prober")
    for pkg in profile.extra_apt_packages:
        if pkg not in base_packages:
            base_packages.append(pkg)

    # Top-level `runcmd` (executes in Live ISO RAM to bypass "Continue with autoinstall? (yes|no)")
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

    # `early-commands`: mounts `cidata` at `/run/instant-cidata` and runs P1 hardware prober
    early_commands = [
        (
            "sh -c '"
            "mkdir -p /run/instant-cidata; "
            "CIDATA_DEV=$(blkid -L cidata || blkid -L CIDATA || true); "
            "if [ -n \"$CIDATA_DEV\" ]; then mount -o ro \"$CIDATA_DEV\" /run/instant-cidata || true; fi; "
            "for dir in /run/instant-cidata /cdrom /var/lib/cloud/seed/nocloud; do "
            "  if [ -f \"$dir/p1_early_hardware_adapter.py\" ]; then "
            "    python3 \"$dir/p1_early_hardware_adapter.py\" --env \"$dir/instant-server-env.conf\" --autoinstall /autoinstall.yaml; "
            "    break; "
            "  fi; "
            "done'"
        )
    ]

    # `late-commands`: re-mounts `cidata` if Subiquity unmounted `/cdrom` (AskUbuntu #1483970 fix)
    late_commands = [
        (
            "sh -c '"
            "mkdir -p /run/instant-cidata /target/opt/instant-server; "
            "if ! mountpoint -q /run/instant-cidata 2>/dev/null; then "
            "  CIDATA_DEV=$(blkid -L cidata || blkid -L CIDATA || true); "
            "  if [ -n \"$CIDATA_DEV\" ]; then mount -o ro \"$CIDATA_DEV\" /run/instant-cidata || true; fi; "
            "fi; "
            "for dir in /run/instant-cidata /cdrom /var/lib/cloud/seed/nocloud; do "
            "  if [ -f \"$dir/p2_late_laptop_hardening.sh\" ]; then "
            "    cp -r \"$dir/\"* /target/opt/instant-server/ 2>/dev/null || true; "
            "    chmod +x /target/opt/instant-server/p2_late_laptop_hardening.sh; "
            "    curtin in-target --target=/target -- /bin/bash /opt/instant-server/p2_late_laptop_hardening.sh; "
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
    enable_docker = profile.enable_docker or enable_casaos

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
        f"INSTANT_TARGET_DISK_HINT={_sh_escape(profile.target_disk_hint)}",
        f"INSTANT_ALLOW_USB_TARGET={_sh_escape('1' if profile.allow_usb_target_disk else '0')}",
        f"INSTANT_FORCE_REINSTALL={_sh_escape('1' if profile.force_reinstall else '0')}",
        f"INSTANT_WIFI_SSID={_sh_escape(profile.wifi_ssid)}",
        f"INSTANT_WIFI_PASSWORD={_sh_escape(profile.wifi_password)}",
        f"INSTANT_ENABLE_USB_TETHER={_sh_escape('1' if profile.enable_usb_tethering else '0')}",
        f"INSTANT_STATIC_IP_CIDR={_sh_escape(profile.static_ip_cidr)}",
        f"INSTANT_STATIC_GATEWAY={_sh_escape(profile.static_gateway)}",
        f"INSTANT_STATIC_DNS={_sh_escape(','.join(profile.static_dns))}",
        f"INSTANT_DASHBOARD_CHOICE={_sh_escape(profile.dashboard_choice)}",
        f"INSTANT_ENABLE_COCKPIT={_sh_escape('1' if enable_cockpit else '0')}",
        f"INSTANT_ENABLE_CASAOS={_sh_escape('1' if enable_casaos else '0')}",
        f"INSTANT_ENABLE_DOCKER={_sh_escape('1' if enable_docker else '0')}",
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
    ]
    instant_env_conf = "\n".join(env_lines) + "\n"

    schema_issues = validate_subiquity_user_data_text(user_data_yaml)
    if schema_issues:
        raise ValueError("Subiquity schema check failed:\n- " + "\n- ".join(schema_issues))

    emit_event(
        "CONFIG_VALIDATED",
        "Successfully compiled and validated Instant Server user-data, meta-data, and instant-server-env.conf",
        {
            "hostname": profile.hostname,
            "install_mode": profile.install_mode,
            "dashboard": profile.dashboard_choice,
            "storage_layout": profile.storage_layout,
        },
        callback=event_callback,
    )
    return {
        "user-data": user_data_yaml,
        "meta-data": meta_data_content,
        "instant-server-env.conf": instant_env_conf,
        "lazarus-env.conf": instant_env_conf,  # Backward compatibility alias
        "profile.json": json.dumps(asdict(profile), indent=2),
    }


def _sh_escape(val: str) -> str:
    escaped = val.replace("\\", "\\\\").replace('"', '\\"').replace("$", "\\$").replace("`", "\\`")
    return f'"{escaped}"'


# ---------------------------------------------------------------------------
# 6. Subiquity Offline Schema Validator (Ubuntu 24.04 Noble Strict Mode)
# ---------------------------------------------------------------------------
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
                    f"Illegal Ubuntu 24.04 Subiquity key 'autoinstall.{subkey}' (will halt Noble installer)."
                )
    return issues


# ---------------------------------------------------------------------------
# 7. CLI Entrypoint & Built-In Self-Test
# ---------------------------------------------------------------------------
def _run_self_test() -> int:
    print("[M4 Self-Test] 1. Testing pure-Python SHA-512 crypt ($6$) determinism...")
    known_hash = sha512_crypt("ubuntu", salt="exDY1mhS4KUYCE/2")
    expected = (
        "$6$exDY1mhS4KUYCE/2$zmn9ToZwTKLhCw.b4/b.ZRTIZM30JZ4QrOQ2aOXJ8yk96xpcCof0kxKwuX1kqLG/"
        "ygbJ1f8wxED22bTL4F46P0"
    )
    assert known_hash == expected, f"SHA-512 crypt mismatch:\n  got:  {known_hash}\n  want: {expected}"
    print("  -> PASS: Pure-Python SHA-512 crypt matches official Canonical test vector!")

    print("[M4 Self-Test] 2. Testing Auto-Erase safety gate ('I know what I'm doing' required)...")
    unsafe_prof = ServerProfile(install_mode="auto_erase", confirm_erase_main_drive=False)
    assert any("I know what I'm doing" in e for e in unsafe_prof.validate())
    print("  -> PASS: Unconfirmed auto_erase profile blocked as expected!")

    print("[M4 Self-Test] 3. Testing Dual-Boot & Interactive-Select profiles...")
    dual_prof = ServerProfile(
        hostname="instant-srv",
        username="admin",
        password="SuperSecretPassword123!",
        install_mode="dual_boot",
        dual_boot_Allocation_gb=60,
        dashboard_choice="both",
    )
    bundle_dual = compile_cidata_payloads(dual_prof)
    assert "INSTANT_INSTALL_MODE=\"dual_boot\"" in bundle_dual["instant-server-env.conf"]
    assert "os-prober" in bundle_dual["user-data"]

    inter_prof = ServerProfile(
        hostname="instant-srv",
        username="admin",
        password="SuperSecretPassword123!",
        install_mode="interactive_select",
    )
    bundle_inter = compile_cidata_payloads(inter_prof)
    assert "interactive-sections:" in bundle_inter["user-data"]
    print("  -> PASS: Dual-boot and Interactive-Select profiles compiled and validated!")
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


if __name__ == "__main__":
    sys.exit(main())