#!/usr/bin/env python3
"""
FILE: src/instant_server/scripts/autoinstall_config.py
PURPOSE:
  Updates /autoinstall.yaml during Subiquity early-commands using PyYAML and
  argparse so all flags (--config, --disk, --wired-online, --strip-storage-config)
  work together reliably.

OFFICIAL SUBIQUITY REFERENCES:
  - Autoinstall Reference: https://canonical-subiquity.readthedocs-hosted.com/en/latest/reference/autoinstall-reference.html
  - Curtin Storage Config: https://curtin.readthedocs.io/en/latest/topics/storage.html
"""

import argparse
import os
import sys
from typing import Any
import yaml  # type: ignore[import-untyped,import-not-found]


def load_yaml(path: str) -> dict[str, Any]:
    if not os.path.exists(path):
        return {}
    with open(path, "r", encoding="utf-8") as f:
        loaded = yaml.safe_load(f)
        return loaded if isinstance(loaded, dict) else {}


def save_yaml(path: str, data: dict[str, Any]) -> None:
    with open(path, "w", encoding="utf-8") as f:
        yaml.safe_dump(data, f, default_flow_style=False, sort_keys=False)


def enforce_nonblocking_controllers(ai: dict[str, Any]) -> None:
    """
    Disables interactive or online-blocking Subiquity controllers (Drivers, OEM,
    and GeoIP/Refresh) while keeping a valid autoinstall version: 1 schema.
    """
    ai["version"] = 1
    if "interactive-sections" not in ai:
        ai["interactive-sections"] = []

    raw_src = ai.get("source")
    src: dict[str, Any] = dict(raw_src) if isinstance(raw_src, dict) else {}
    if not src.get("id"):
        src["id"] = "ubuntu-server-minimal"
    src["search_drivers"] = False
    ai["source"] = src

    ai["drivers"] = {"install": False}
    ai["oem"] = {"install": False}
    ai["refresh-installer"] = {"update": False}


def apply_wired_only_network(ai: dict[str, Any]) -> None:
    """
    Replaces `network:` with a non-blocking wired-only Netplan v2 configuration
    (`optional: true`, no `wifis:`) when an active Ethernet lease is already up.
    """
    ai["network"] = {
        "version": 2,
        "ethernets": {
            "wired-en": {
                "match": {"name": "en*"},
                "dhcp4": True,
                "dhcp6": False,
                "optional": True,
            },
            "wired-eth": {
                "match": {"name": "eth*"},
                "dhcp4": True,
                "dhcp6": False,
                "optional": True,
            },
        },
    }


def pin_target_disk_in_storage(ai: dict[str, Any], disk_path: str) -> None:
    """
    Pins `storage.layout.match.path` (or `storage.config` disk entry) to the
    exact internal disk path (e.g. /dev/sda or /dev/nvme0n1) so Subiquity never
    touches the USB installer drive.
    """
    if not disk_path:
        return
    storage = ai.get("storage")
    if not isinstance(storage, dict):
        storage = {"layout": {"name": "direct", "match": {"path": disk_path}}}
        ai["storage"] = storage
        return

    layout = storage.get("layout")
    if isinstance(layout, dict):
        layout["match"] = {"path": disk_path}
        storage["layout"] = layout
    elif isinstance(storage.get("config"), list):
        for item in storage["config"]:
            if isinstance(item, dict) and item.get("type") == "disk":
                item["path"] = disk_path
                break


def main() -> None:
    parser = argparse.ArgumentParser(description="Sanitize Subiquity /autoinstall.yaml")
    parser.add_argument("--config", default="/autoinstall.yaml", help="Path to autoinstall.yaml")
    parser.add_argument("--disk", default="", help="Target internal disk path (e.g. /dev/sda)")
    parser.add_argument("--wired-online", action="store_true", help="Keep wired Ethernet and defer Wi-Fi")
    parser.add_argument("--use-largest-gap", action="store_true", help="Mark for dual-boot free-gap usage")
    parser.add_argument("--strip-storage-config", action="store_true", help="Replace storage.config with layout")
    parser.add_argument("--layout", default="direct", help="Layout name when stripping storage.config")
    args, unknown = parser.parse_known_args()

    # Support legacy positional invocation: autoinstall_config.py --wired-online
    if "--wired-online" in unknown:
        args.wired_online = True

    doc = load_yaml(args.config)
    if not doc:
        return

    # Support both cloud-config wrapped (`autoinstall:`) and bare autoinstall.yaml
    has_wrapper = isinstance(doc.get("autoinstall"), dict)
    ai: dict[str, Any] = doc["autoinstall"] if has_wrapper else doc

    enforce_nonblocking_controllers(ai)

    if args.strip_storage_config:
        match_spec: dict[str, Any] = {"path": args.disk} if args.disk else {"size": "largest"}
        ai["storage"] = {"layout": {"name": args.layout, "match": match_spec}}
    elif args.disk:
        pin_target_disk_in_storage(ai, args.disk)

    if args.wired_online:
        apply_wired_only_network(ai)

    if has_wrapper:
        doc["autoinstall"] = ai
        save_yaml(args.config, doc)
    else:
        save_yaml(args.config, ai)

    # If /cdrom/autoinstall.yaml or /run/subiquity/cloud.autoinstall.yaml exists and is writable, sync it too
    for mirror_path in ("/autoinstall.yaml", "/run/subiquity/cloud.autoinstall.yaml"):
        if mirror_path != args.config and os.path.exists(mirror_path):
            try:
                save_yaml(mirror_path, {"autoinstall": ai} if has_wrapper else ai)
            except Exception:
                pass


if __name__ == "__main__":
    main()