#!/usr/bin/env python3
"""
sanitize_autoinstall.py
-----------------------
Standalone helper script run during Ubuntu Server Autoinstall early-commands.

Why this script exists (separated from Bash for readability):
1. Subiquity generates /autoinstall.yaml from our cloud-init user-data and strips YAML comments.
2. Using PyYAML (yaml.safe_load / yaml.safe_dump) is much safer than regex/sed on YAML files.
3. This script performs three clean updates on /autoinstall.yaml:
   - Pins storage.layout.match.path to the detected internal disk (e.g. /dev/sda or /dev/nvme0n1)
     so the installer never touches the USB stick.
   - Removes 'wifis' from the live installer network config so Subiquity never hangs waiting for
     wlan0 or wpa_supplicant during OS installation (Wi-Fi is configured on the target OS in late-commands).
   - If wired Ethernet is already online (--wired-online), removes the 'network' block from
     /autoinstall.yaml so Subiquity keeps the working live DHCP connection without re-running
     'netplan apply' (which can otherwise hang Subiquity's NetworkController).
   - If Windows dual-boot free space was prepared (--use-largest-gap), sets
     storage.layout['use-gap'] = 'largest'.
"""

import argparse
import os
import sys
import yaml


def load_yaml(path: str) -> dict:
    if not os.path.isfile(path):
        return {}
    with open(path, "r", encoding="utf-8") as handle:
        loaded = yaml.safe_load(handle)
    return loaded if isinstance(loaded, dict) else {}


def save_yaml(path: str, data: dict) -> None:
    with open(path, "w", encoding="utf-8") as handle:
        yaml.safe_dump(data, handle, default_flow_style=False, sort_keys=False)


def get_autoinstall_root(data: dict) -> dict:
    """Return the dictionary containing autoinstall keys (handles both wrapped and unwrapped YAML)."""
    if isinstance(data.get("autoinstall"), dict):
        return data["autoinstall"]
    return data


def main() -> int:
    parser = argparse.ArgumentParser(description="Sanitize /autoinstall.yaml for Instant Server.")
    parser.add_argument("--config", default="/autoinstall.yaml", help="Path to /autoinstall.yaml")
    parser.add_argument("--target-disk", default="", help="Internal disk path to pin (e.g. /dev/sda)")
    parser.add_argument("--wired-online", action="store_true", help="Keep live wired DHCP connection and skip Subiquity netplan apply")
    parser.add_argument("--use-largest-gap", action="store_true", help="Install into largest unallocated gap (Windows dual-boot)")
    args = parser.parse_args()

    if not os.path.isfile(args.config):
        return 0

    try:
        data = load_yaml(args.config)
        root = get_autoinstall_root(data)

        # 1. Disable slow online third-party driver and OEM metapackage queries during install
        root["drivers"] = {"install": False}
        root["oem"] = {"install": False}
        root["refresh-installer"] = {"update": False}

        # 2. Pin target internal disk path if provided
        target_disk = args.target_disk.strip()
        if target_disk and isinstance(root.get("storage"), dict):
            layout = root["storage"].get("layout")
            if isinstance(layout, dict):
                match_cfg = layout.get("match")
                if isinstance(match_cfg, dict) and "size" in match_cfg:
                    layout["match"] = {"path": target_disk}

        # 3. Configure use-gap: largest when dual-booting alongside Windows NTFS
        if args.use_largest_gap and isinstance(root.get("storage"), dict):
            layout = root["storage"].get("layout")
            if isinstance(layout, dict):
                layout["use-gap"] = "largest"

        # 4. Prevent Subiquity NetworkController from hanging on live 'netplan apply' or 'wlan0'
        if isinstance(root.get("network"), dict):
            root["network"].pop("wifis", None)
            if args.wired_online:
                root.pop("network", None)

        save_yaml(args.config, data)
        return 0
    except Exception as exc:
        print(f"[sanitize_autoinstall] warning: {exc}", file=sys.stderr)
        return 0


if __name__ == "__main__":
    sys.exit(main())