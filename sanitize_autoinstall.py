#!/usr/bin/env python3
"""
=============================================================================
HOW TO STUDY & MODIFY THIS FILE (Step-by-Step Beginner Guide)
=============================================================================
File: sanitize_autoinstall.py
Purpose:
  Safely modifies /autoinstall.yaml during early-commands on the target Ubuntu
  Server machine using PyYAML instead of fragile regex/sed edits.

Why this exists:
  1. Removes `storage.config` before injected `swap:` or `updates:` keys so
     Subiquity can smoothly switch to `storage.layout` when a hardware disk
     sizing fallback is needed.
  2. When a wired Ethernet cable (`en*`/`eth*`) is already online (`--wired-online`),
     replaces `network:` with a minimal non-blocking wired-only Netplan v2 config
     (`optional: true`, no `wifis:`) so Subiquity's NetworkController does not
     synthesize a blocking config for unplugged ports or fail on missing
     `wpasupplicant`.
  3. Explicitly sets `source.search_drivers = False`, `drivers.install = False`,
     and `oem.install = False` so Ubuntu 24.04 / 26.04 Subiquity does not stall
     at `subiquity/OEM/load_metapackages_list/wait_confirmation` or
     `subiquity/Drivers/_list_drivers/wait_apt`.
=============================================================================
"""

import sys
import yaml

AUTOINSTALL_PATH = "/autoinstall.yaml"


def load_autoinstall(path: str) -> dict:
    with open(path, "r", encoding="utf-8") as f:
        return yaml.safe_load(f) or {}


def save_autoinstall(path: str, data: dict) -> None:
    with open(path, "w", encoding="utf-8") as f:
        yaml.safe_dump(data, f, default_flow_style=False, sort_keys=False)


def enforce_nonblocking_controllers(ai: dict) -> None:
    """
    Ensures Subiquity's Drivers and OEM controllers never pause waiting for
    interactive confirmation or online apt driver queries during autoinstall.
    Reference: https://canonical-subiquity.readthedocs-hosted.com/en/latest/reference/autoinstall-reference.html
    """
    src = ai.get("source")
    if not isinstance(src, dict):
        src = {"id": "ubuntu-server-minimal"}
    src["search_drivers"] = False
    ai["source"] = src

    ai["drivers"] = {"install": False}
    ai["oem"] = {"install": False}


def main() -> None:
    mode = sys.argv[1] if len(sys.argv) > 1 else "--sanitize"
    data = load_autoinstall(AUTOINSTALL_PATH)
    ai = data.get("autoinstall")
    if not isinstance(ai, dict):
        return

    enforce_nonblocking_controllers(ai)

    if mode == "--wired-online":
        # Instead of deleting `network:` (which makes Subiquity generate a default
        # config for all eth*/en* ports without optional:true), keep a minimal
        # wired-only config with optional: true and no wifis block.
        ai["network"] = {
            "version": 2,
            "ethernets": {
                "wired-lan": {
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
    elif mode == "--strip-storage-config":
        layout_name = sys.argv[2] if len(sys.argv) > 2 else "lvm"
        disk_path = sys.argv[3] if len(sys.argv) > 3 else ""
        layout = {"name": layout_name}
        if disk_path:
            layout["match"] = {"path": disk_path}
        ai["storage"] = {"layout": layout}

    save_autoinstall(AUTOINSTALL_PATH, data)


if __name__ == "__main__":
    main()