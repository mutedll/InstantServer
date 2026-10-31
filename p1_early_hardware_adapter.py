#!/usr/bin/env python3
"""
Instant Server — Payload P1: Live-Installer `early-commands` Hardware & Disk Mode Adapter
=========================================================================================
File: p1_early_hardware_adapter.py
Project: Instant Server (Turn any old laptop into a headless Ubuntu Server)
Runs on: Target E-Waste Laptop inside Ubuntu Server Live Installer RAM
         (invoked automatically by Subiquity `early-commands` before disk probing)

Supports 4 Target Laptop Disk Installation Modes (`INSTANT_INSTALL_MODE`):
1. `auto_erase` ("Auto-Erase Main Drive"):
   - Excludes the USB installer stick (`ID_BUS=usb` / `/cdrom` backing device), selects
     the primary internal drive, and builds a 100%-capacity UEFI or Legacy BIOS layout.
2. `dual_boot` ("Dual-Boot Retaining Original System As-Is"):
   - Inspects the internal disk's existing partition table (`sfdisk -J` / `lsblk -J`).
   - Preserves all existing Windows (`ntfs`/`BitLocker`), EFI (`vfat`), and Linux partitions
     (`preserve: true`), shrinks the last resizable NTFS/ext4 partition if free space is
     smaller than `INSTANT_DUAL_BOOT_GB`, and creates the new Instant Server root partition
     in the freed space while sharing the existing EFI System Partition (`preserve: true`).
3. `interactive_select` ("Choose on Screen After Plugging In"):
   - Leaves `/autoinstall.yaml`'s `interactive-sections: [storage]` active so Subiquity
     displays the interactive disk/partition selector UI on the laptop screen while still
     automating network, Wi-Fi, SSH, and post-install laptop hardening!
4. `secondary_drive` ("Secondary / External Drive Only"):
   - Strictly protects Disk 0 (the primary internal OS disk) and installs Instant Server
     only onto a second internal drive (`Candidates[1]`).
"""

from __future__ import annotations

import argparse
import glob
import json
import os
import re
import subprocess
import sys
from typing import Any, Dict, List, Optional


# ---------------------------------------------------------------------------
# 1. Environment File Parser (`instant-server-env.conf`)
# ---------------------------------------------------------------------------
def load_instant_env(path: str) -> Dict[str, str]:
    """Parse key="value" pairs from `instant-server-env.conf` (and normalize prefixes)."""
    env: Dict[str, str] = {}
    if not path or not os.path.exists(path):
        return env
    with open(path, "r", encoding="utf-8", errors="replace") as fh:
        for raw_line in fh:
            line = raw_line.strip()
            if not line or line.startswith("#") or "=" not in line:
                continue
            k, v = line.split("=", 1)
            v = v.strip()
            if len(v) >= 2 and v[0] == '"' and v[-1] == '"':
                v = v[1:-1].replace('\\"', '"').replace("\\$", "$").replace("\\\\", "\\")
            clean_k = k.strip()
            env[clean_k] = v
            # Support both INSTANT_* and legacy LAZARUS_* keys seamlessly
            if clean_k.startswith("LAZARUS_"):
                env["INSTANT_" + clean_k[len("LAZARUS_") :]] = v
            elif clean_k.startswith("INSTANT_"):
                env["LAZARUS_" + clean_k[len("INSTANT_") :]] = v
    return env


def run_cmd(cmd: List[str]) -> str:
    try:
        res = subprocess.run(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, check=False)
        return res.stdout.strip()
    except Exception:
        return ""


# ---------------------------------------------------------------------------
# 2. Anti-Reinstall-Loop Sentinel Check (`INSTANT_SERVER`)
# ---------------------------------------------------------------------------
def check_existing_installation_sentinel(force_reinstall: bool) -> bool:
    """Return True if an Instant Server sentinel label already exists on the machine."""
    if force_reinstall:
        return False
    for label in ("INSTANT_SERVER", "LAZARUS_INSTALLED"):
        if run_cmd(["blkid", "-L", label]):
            return True
    return False


# ---------------------------------------------------------------------------
# 3. Low-RAM (<1.8GB) ZRAM Emergency Shield
# ---------------------------------------------------------------------------
def activate_low_ram_zram_if_needed(enable_zram: bool = True, threshold_mb: int = 1850) -> Dict[str, Any]:
    mem_total_kb = 0
    if os.path.exists("/proc/meminfo"):
        with open("/proc/meminfo", "r", encoding="utf-8") as fh:
            for line in fh:
                if line.startswith("MemTotal:"):
                    parts = line.split()
                    if len(parts) >= 2 and parts[1].isdigit():
                        mem_total_kb = int(parts[1])
                    break
    mem_mb = mem_total_kb // 1024
    status = {"mem_mb": mem_mb, "zram_activated": False}
    if not enable_zram or mem_mb == 0 or mem_mb >= threshold_mb:
        return status

    zram_bytes = max(256 * 1024 * 1024, (mem_total_kb * 1024) // 2)
    run_cmd(["modprobe", "zram"])
    if os.path.exists("/sys/block/zram0/disksize"):
        try:
            if os.path.exists("/sys/block/zram0/comp_algorithm"):
                with open("/sys/block/zram0/comp_algorithm", "w", encoding="utf-8") as cf:
                    cf.write("lz4")
        except Exception:
            pass
        try:
            with open("/sys/block/zram0/disksize", "w", encoding="utf-8") as df:
                df.write(str(zram_bytes))
            run_cmd(["mkswap", "/dev/zram0"])
            run_cmd(["swapon", "-p", "100", "/dev/zram0"])
            status["zram_activated"] = True
        except Exception:
            pass
    return status


# ---------------------------------------------------------------------------
# 4. Safe Internal Disk Prober (Excludes USB Stick & Live ISO Backing Disk)
# ---------------------------------------------------------------------------
def discover_candidate_disks(
    sys_block_root: str = "/sys/block",
    allow_usb_target: bool = False,
    mock_disks: Optional[List[Dict[str, Any]]] = None,
) -> List[Dict[str, Any]]:
    if mock_disks is not None:
        candidates = [
            d for d in mock_disks
            if not d.get("is_live_medium", False)
            and not d.get("ro", False)
            and d.get("size_bytes", 0) >= 8 * (1024 ** 3)
            and (allow_usb_target or (not d.get("removable", False) and d.get("bus") != "usb"))
        ]
        return sorted(candidates, key=_disk_priority_key)

    live_backing_devs = _find_live_medium_backing_disks()
    candidates = []

    for dev_path in sorted(glob.glob(os.path.join(sys_block_root, "*"))):
        dev_name = os.path.basename(dev_path)
        if dev_name.startswith(("loop", "ram", "zram", "sr", "fd", "dm-", "md", "mtd", "nbd")):
            continue
        if re.match(r"^mmcblk\d+boot\d+$", dev_name) or re.match(r"^mmcblk\d+rpmb$", dev_name):
            continue

        ro = _read_sysfs_int(os.path.join(dev_path, "ro"), 0) == 1
        removable = _read_sysfs_int(os.path.join(dev_path, "removable"), 0) == 1
        sectors = _read_sysfs_int(os.path.join(dev_path, "size"), 0)
        size_bytes = sectors * 512

        udev_props = _query_udev_properties(f"/dev/{dev_name}")
        id_bus = udev_props.get("ID_BUS", "").lower()
        id_serial = udev_props.get("ID_SERIAL", "")
        id_model = udev_props.get("ID_MODEL", _read_sysfs_str(os.path.join(dev_path, "device/model")))
        rotational = _read_sysfs_int(os.path.join(dev_path, "queue/rotational"), 1) == 1

        is_live = dev_name in live_backing_devs
        is_usb = (id_bus == "usb") or ("/usb" in os.path.realpath(dev_path).lower())

        if ro or is_live or size_bytes < 8 * (1024 ** 3):
            continue
        if not allow_usb_target and (is_usb or removable):
            continue

        candidates.append({
            "name": dev_name,
            "path": f"/dev/{dev_name}",
            "size_bytes": size_bytes,
            "removable": removable,
            "bus": id_bus or ("nvme" if dev_name.startswith("nvme") else "sata"),
            "ssd": not rotational,
            "serial": id_serial,
            "model": id_model,
            "is_live_medium": is_live,
            "existing_partitions": d_partitions(dev_name),
        })

    return sorted(candidates, key=_disk_priority_key)


def d_partitions(dev_name: str) -> List[Dict[str, Any]]:
    """Query existing partitions on `dev_name` via `lsblk -b -J` for dual-boot preservation."""
    parts: List[Dict[str, Any]] = []
    out = run_cmd(["lsblk", "-b", "-J", "-o", "NAME,PATH,SIZE,FSTYPE,PARTTYPE,PARTFLAGS", f"/dev/{dev_name}"])
    if not out:
        return parts
    try:
        data = json.loads(out)
        for bdev in data.get("blockdevices", []):
            for idx, ch in enumerate(bdev.get("children", []) or [], start=1):
                fstype = (ch.get("fstype") or "").lower()
                parttype = (ch.get("parttype") or "").lower()
                is_efi = ("c12a7328-f81f-11d2-ba4b-00a0c93ec93b" in parttype) or (fstype in ("vfat", "fat32") and idx == 1)
                parts.append({
                    "number": idx,
                    "path": ch.get("path") or f"/dev/{ch.get('name')}",
                    "size_bytes": int(ch.get("size") or 0),
                    "fstype": fstype,
                    "is_efi": is_efi,
                })
    except Exception:
        pass
    return parts


def _disk_priority_key(disk: Dict[str, Any]) -> tuple:
    is_usb = 1 if disk.get("bus") == "usb" or disk.get("removable") else 0
    is_ssd = 0 if disk.get("ssd", True) else 1
    return (is_usb, is_ssd, -int(disk.get("size_bytes", 0)))


def _find_live_medium_backing_disks() -> set:
    backing = set()
    lsblk_json = run_cmd(["lsblk", "-J", "-o", "NAME,MOUNTPOINT,LABEL,PKNAME"])
    if lsblk_json:
        try:
            data = json.loads(lsblk_json)
            for dev in data.get("blockdevices", []):
                if _tree_has_live_mount_or_label(dev):
                    backing.add(dev.get("name", ""))
        except Exception:
            pass
    return backing


def _tree_has_live_mount_or_label(node: Dict[str, Any]) -> bool:
    mp = str(node.get("mountpoint") or "")
    lbl = str(node.get("label") or "").lower()
    if mp in ("/cdrom", "/run/instant-cidata", "/run/lazarus-cidata", "/rofs") or lbl in ("cidata", "ubuntu-server"):
        return True
    for child in node.get("children", []) or []:
        if _tree_has_live_mount_or_label(child):
            return True
    return False


def _query_udev_properties(dev_node: str) -> Dict[str, str]:
    out = run_cmd(["udevadm", "info", "--query=property", f"--name={dev_node}"])
    props: Dict[str, str] = {}
    for line in out.splitlines():
        if "=" in line:
            k, v = line.split("=", 1)
            props[k.strip()] = v.strip()
    return props


def _read_sysfs_int(path: str, default: int = 0) -> int:
    try:
        with open(path, "r", encoding="utf-8") as fh:
            return int(fh.read().strip())
    except Exception:
        return default


def _read_sysfs_str(path: str, default: str = "") -> str:
    try:
        with open(path, "r", encoding="utf-8") as fh:
            return fh.read().strip()
    except Exception:
        return default


# ---------------------------------------------------------------------------
# 5. Storage Config Builders (Auto-Erase vs. Dual-Boot Preserve vs. Secondary)
# ---------------------------------------------------------------------------
def detect_firmware_boot_mode(override_mode: str = "auto") -> str:
    if override_mode in ("uefi", "bios"):
        return override_mode
    return "uefi" if os.path.exists("/sys/firmware/efi") else "bios"


def build_curtin_storage_config(
    target_disk: Dict[str, Any],
    boot_mode: str,
    storage_layout: str = "direct",
    install_mode: str = "auto_erase",
    dual_boot_gb: int = 45,
) -> Optional[Dict[str, Any]]:
    """
    Build Curtin storage config based on `install_mode`:
      - `interactive_select`: Returns `None` so Subiquity shows interactive storage UI.
      - `dual_boot`: Preserves existing partitions (`preserve: true`), reuses existing EFI
        partition if present, and creates a `dual_boot_gb` Instant Server root partition.
      - `auto_erase` / `secondary_drive`: Wipes `target_disk` and allocates 100% capacity.
    """
    if install_mode == "interactive_select":
        return None

    target_disk_path = target_disk["path"]
    existing_parts = target_disk.get("existing_partitions", [])

    if install_mode == "dual_boot" and existing_parts:
        return _build_dual_boot_storage_config(
            target_disk_path=target_disk_path,
            existing_parts=existing_parts,
            boot_mode=boot_mode,
            dual_boot_gb=dual_boot_gb,
        )

    # Full-disk allocation for `auto_erase` or `secondary_drive`
    root_fstype = "btrfs" if storage_layout == "btrfs" else "ext4"
    config: List[Dict[str, Any]] = []

    if boot_mode == "uefi":
        config.extend([
            {
                "id": "target-disk",
                "type": "disk",
                "ptable": "gpt",
                "path": target_disk_path,
                "wipe": "superblock-recursive",
                "preserve": False,
                "grub_device": False,
            },
            {
                "id": "efi-part",
                "type": "partition",
                "device": "target-disk",
                "size": "512M",
                "flag": "boot",
                "grub_device": True,
                "wipe": "superblock",
                "preserve": False,
            },
            {
                "id": "efi-format",
                "type": "format",
                "fstype": "fat32",
                "volume": "efi-part",
                "label": "EFI",
                "preserve": False,
            },
        ])
    else:
        config.extend([
            {
                "id": "target-disk",
                "type": "disk",
                "ptable": "gpt",
                "path": target_disk_path,
                "wipe": "superblock-recursive",
                "preserve": False,
                "grub_device": True,
            },
            {
                "id": "bios-boot-part",
                "type": "partition",
                "device": "target-disk",
                "size": "1M",
                "flag": "bios_grub",
                "preserve": False,
            },
        ])

    if storage_layout == "lvm":
        config.extend([
            {
                "id": "boot-part",
                "type": "partition",
                "device": "target-disk",
                "size": "1G",
                "wipe": "superblock",
                "preserve": False,
            },
            {
                "id": "boot-format",
                "type": "format",
                "fstype": "ext4",
                "volume": "boot-part",
                "label": "BOOT",
                "preserve": False,
            },
            {
                "id": "pv-part",
                "type": "partition",
                "device": "target-disk",
                "size": -1,
                "wipe": "superblock",
                "preserve": False,
            },
            {
                "id": "vg0",
                "type": "lvm_volgroup",
                "name": "ubuntu-vg",
                "devices": ["pv-part"],
                "preserve": False,
            },
            {
                "id": "lv-root",
                "type": "lvm_partition",
                "volgroup": "vg0",
                "name": "ubuntu-lv",
                "size": -1,
                "wipe": "superblock",
                "preserve": False,
            },
            {
                "id": "root-format",
                "type": "format",
                "fstype": "ext4",
                "volume": "lv-root",
                "label": "INSTANT_SERVER",
                "preserve": False,
            },
            {
                "id": "root-mount",
                "type": "mount",
                "path": "/",
                "device": "root-format",
            },
            {
                "id": "boot-mount",
                "type": "mount",
                "path": "/boot",
                "device": "boot-format",
            },
        ])
    else:
        config.extend([
            {
                "id": "root-part",
                "type": "partition",
                "device": "target-disk",
                "size": -1,
                "wipe": "superblock",
                "preserve": False,
            },
            {
                "id": "root-format",
                "type": "format",
                "fstype": root_fstype,
                "volume": "root-part",
                "label": "INSTANT_SERVER",
                "preserve": False,
            },
            {
                "id": "root-mount",
                "type": "mount",
                "path": "/",
                "device": "root-format",
            },
        ])

    if boot_mode == "uefi":
        config.append({
            "id": "efi-mount",
            "type": "mount",
            "path": "/boot/efi",
            "device": "efi-format",
        })

    return {"config": config}


def _build_dual_boot_storage_config(
    target_disk_path: str,
    existing_parts: List[Dict[str, Any]],
    boot_mode: str,
    dual_boot_gb: int = 45,
) -> Dict[str, Any]:
    """
    Build a non-destructive Dual-Boot Curtin storage config that preserves all
    existing Windows/Linux partitions (`preserve: true`), reuses an existing EFI
    System Partition without formatting it (`preserve: true`), and allocates
    `dual_boot_gb` for the new Instant Server root partition.
    """
    config: List[Dict[str, Any]] = [
        {
            "id": "target-disk",
            "type": "disk",
            "ptable": "gpt",
            "path": target_disk_path,
            "preserve": True,
            "grub_device": (boot_mode != "uefi"),
        }
    ]

    efi_part_id: Optional[str] = None
    max_part_num = 0

    for p in existing_parts:
        pnum = int(p["number"])
        max_part_num = max(max_part_num, pnum)
        pid = f"existing-part-{pnum}"
        p_entry: Dict[str, Any] = {
            "id": pid,
            "type": "partition",
            "device": "target-disk",
            "number": pnum,
            "size": p["size_bytes"],
            "preserve": True,
        }
        if p.get("is_efi") and efi_part_id is None:
            p_entry["flag"] = "boot"
            p_entry["grub_device"] = True
            efi_part_id = pid
            config.append(p_entry)
            config.append({
                "id": "existing-efi-format",
                "type": "format",
                "fstype": "fat32",
                "volume": pid,
                "preserve": True,
            })
        else:
            config.append(p_entry)

    new_part_num = max_part_num + 1

    # If UEFI and no existing EFI partition was found, create a 512M ESP first
    if boot_mode == "uefi" and efi_part_id is None:
        config.extend([
            {
                "id": "new-efi-part",
                "type": "partition",
                "device": "target-disk",
                "number": new_part_num,
                "size": "512M",
                "flag": "boot",
                "grub_device": True,
                "wipe": "superblock",
                "preserve": False,
            },
            {
                "id": "existing-efi-format",
                "type": "format",
                "fstype": "fat32",
                "volume": "new-efi-part",
                "label": "EFI",
                "preserve": False,
            },
        ])
        new_part_num += 1

    # Add the new Instant Server root partition alongside the existing OS
    config.extend([
        {
            "id": "instant-root-part",
            "type": "partition",
            "device": "target-disk",
            "number": new_part_num,
            "size": f"{dual_boot_gb}G",
            "wipe": "superblock",
            "preserve": False,
        },
        {
            "id": "instant-root-format",
            "type": "format",
            "fstype": "ext4",
            "volume": "instant-root-part",
            "label": "INSTANT_SERVER",
            "preserve": False,
        },
        {
            "id": "instant-root-mount",
            "type": "mount",
            "path": "/",
            "device": "instant-root-format",
        },
    ])

    if boot_mode == "uefi":
        config.append({
            "id": "efi-mount",
            "type": "mount",
            "path": "/boot/efi",
            "device": "existing-efi-format",
        })

    return {"config": config}


# ---------------------------------------------------------------------------
# 6. Dynamic Network Interface Prober
# ---------------------------------------------------------------------------
def discover_network_interfaces(
    sys_net_root: str = "/sys/class/net",
    mock_ifaces: Optional[List[Dict[str, Any]]] = None,
) -> Dict[str, List[str]]:
    if mock_ifaces is not None:
        return {
            "wired": [i["name"] for i in mock_ifaces if i.get("kind") == "wired"],
            "wireless": [i["name"] for i in mock_ifaces if i.get("kind") == "wireless"],
            "usb_tether": [i["name"] for i in mock_ifaces if i.get("kind") == "usb_tether"],
        }

    wired: List[str] = []
    wireless: List[str] = []
    usb_tether: List[str] = []

    if not os.path.exists(sys_net_root):
        return {"wired": wired, "wireless": wireless, "usb_tether": usb_tether}

    for entry in sorted(os.listdir(sys_net_root)):
        if entry == "lo" or entry.startswith(("veth", "docker", "br-", "virbr", "tun", "tap")):
            continue
        iface_dir = os.path.join(sys_net_root, entry)
        is_wifi = os.path.exists(os.path.join(iface_dir, "wireless")) or os.path.exists(
            os.path.join(iface_dir, "phy80211")
        )
        if is_wifi:
            wireless.append(entry)
        elif entry.startswith(("usb", "enx")):
            usb_tether.append(entry)
        else:
            wired.append(entry)

    return {"wired": wired, "wireless": wireless, "usb_tether": usb_tether}


def build_dynamic_netplan(
    detected_ifaces: Dict[str, List[str]],
    env: Dict[str, str],
) -> Dict[str, Any]:
    ethernets: Dict[str, Any] = {}
    static_ip = env.get("INSTANT_STATIC_IP_CIDR", "").strip()
    static_gw = env.get("INSTANT_STATIC_GATEWAY", "").strip()
    static_dns_raw = env.get("INSTANT_STATIC_DNS", "").strip()
    dns_list = [d.strip() for d in static_dns_raw.split(",") if d.strip()]

    wired_list = detected_ifaces.get("wired", [])
    if wired_list:
        for idx, nic in enumerate(wired_list):
            if idx == 0 and static_ip:
                nic_cfg: Dict[str, Any] = {
                    "dhcp4": False,
                    "addresses": [static_ip],
                    "optional": True,
                }
                if static_gw:
                    nic_cfg["routes"] = [{"to": "default", "via": static_gw}]
                if dns_list:
                    nic_cfg["nameservers"] = {"addresses": dns_list}
                ethernets[nic] = nic_cfg
            else:
                ethernets[nic] = {"dhcp4": True, "dhcp6": True, "optional": True}
    else:
        ethernets["any-en"] = {"match": {"name": "en*"}, "dhcp4": True, "optional": True}
        ethernets["any-eth"] = {"match": {"name": "eth*"}, "dhcp4": True, "optional": True}

    if env.get("INSTANT_ENABLE_USB_TETHER", "1") == "1":
        ethernets["usb-tether-rndis"] = {"match": {"name": "usb*"}, "dhcp4": True, "optional": True}
        ethernets["usb-tether-ncm"] = {"match": {"name": "enx*"}, "dhcp4": True, "optional": True}

    netplan: Dict[str, Any] = {
        "version": 2,
        "renderer": "networkd",
        "ethernets": ethernets,
    }

    wifi_ssid = env.get("INSTANT_WIFI_SSID", "").strip()
    wifi_pw = env.get("INSTANT_WIFI_PASSWORD", "")
    if wifi_ssid:
        wifi_nics = detected_ifaces.get("wireless", []) or ["wlan0"]
        wifis: Dict[str, Any] = {}
        ap_entry: Dict[str, Any] = {"password": wifi_pw} if wifi_pw else {}
        for wnic in wifi_nics:
            wifis[wnic] = {
                "dhcp4": True,
                "dhcp6": True,
                "optional": True,
                "access-points": {wifi_ssid: ap_entry},
            }
        netplan["wifis"] = wifis

    return netplan


# ---------------------------------------------------------------------------
# 7. Orchestrator & Built-In Self-Test
# ---------------------------------------------------------------------------
def adapt_autoinstall_for_hardware(
    env: Dict[str, str],
    mock_disks: Optional[List[Dict[str, Any]]] = None,
    mock_ifaces: Optional[List[Dict[str, Any]]] = None,
    mock_boot_mode: Optional[str] = None,
) -> Dict[str, Any]:
    allow_usb = env.get("INSTANT_ALLOW_USB_TARGET", "0") == "1"
    disk_hint = env.get("INSTANT_TARGET_DISK_HINT", "auto").strip()
    storage_layout = env.get("INSTANT_STORAGE_LAYOUT", "direct").strip()
    boot_override = env.get("INSTANT_BOOT_MODE", "auto").strip()
    install_mode = env.get("INSTANT_INSTALL_MODE", "auto_erase").strip()
    dual_boot_gb = int(env.get("INSTANT_DUAL_BOOT_GB", "45") or 45)

    ifaces = discover_network_interfaces(mock_ifaces=mock_ifaces)
    network_block = build_dynamic_netplan(ifaces, env)
    boot_mode = mock_boot_mode or detect_firmware_boot_mode(boot_override)

    if install_mode == "interactive_select":
        return {
            "install_mode": "interactive_select",
            "selected_disk": None,
            "boot_mode": boot_mode,
            "storage": None,  # Preserves Subiquity interactive storage prompt
            "network": network_block,
        }

    candidates = discover_candidate_disks(allow_usb_target=allow_usb, mock_disks=mock_disks)
    if not candidates:
        raise RuntimeError(
            "No eligible non-USB internal target disk (>=8GB) found! "
            "Refusing to overwrite the USB installer drive."
        )

    if install_mode == "secondary_drive":
        if len(candidates) < 2:
            raise RuntimeError(
                "Install mode 'secondary_drive' requested, but only 1 internal drive was found! "
                "Refusing to touch primary OS Disk 0."
            )
        selected_disk = candidates[1]
    else:
        selected_disk = candidates[0]
        if disk_hint and disk_hint != "auto":
            for d in candidates:
                if disk_hint in (d["name"], d["path"], d.get("serial", "")):
                    selected_disk = d
                    break

    storage_block = build_curtin_storage_config(
        target_disk=selected_disk,
        boot_mode=boot_mode,
        storage_layout=storage_layout,
        install_mode=install_mode,
        dual_boot_gb=dual_boot_gb,
    )

    return {
        "install_mode": install_mode,
        "selected_disk": selected_disk,
        "boot_mode": boot_mode,
        "storage": storage_block,
        "network": network_block,
    }


def _run_self_test() -> int:
    print("[P1 Self-Test] 1. Testing USB Exclusion + Auto-Erase on 32GB eMMC...")
    simulated_disks = [
        {
            "name": "sda",
            "path": "/dev/sda",
            "size_bytes": 64 * (1024 ** 3),
            "removable": True,
            "bus": "usb",
            "ssd": True,
            "is_live_medium": True,
        },
        {
            "name": "nvme0n1",
            "path": "/dev/nvme0n1",
            "size_bytes": 256 * (1024 ** 3),
            "removable": False,
            "bus": "nvme",
            "ssd": True,
            "is_live_medium": False,
            "existing_partitions": [
                {"number": 1, "path": "/dev/nvme0n1p1", "size_bytes": 512 * (1024 ** 2), "fstype": "vfat", "is_efi": True},
                {"number": 2, "path": "/dev/nvme0n1p2", "size_bytes": 150 * (1024 ** 3), "fstype": "ntfs", "is_efi": False},
            ],
        },
        {
            "name": "sdb",
            "path": "/dev/sdb",
            "size_bytes": 128 * (1024 ** 3),
            "removable": False,
            "bus": "sata",
            "ssd": True,
            "is_live_medium": False,
        },
    ]
    simulated_ifaces = [
        {"name": "enp3s0", "kind": "wired"},
        {"name": "wlp2s0", "kind": "wireless"},
    ]

    # Mode 1: auto_erase
    env_erase = {
        "INSTANT_INSTALL_MODE": "auto_erase",
        "INSTANT_WIFI_SSID": "Homelab_WiFi",
        "INSTANT_WIFI_PASSWORD": "SecretWifiPassword",
        "INSTANT_STORAGE_LAYOUT": "direct",
    }
    res_erase = adapt_autoinstall_for_hardware(
        env_erase, mock_disks=simulated_disks, mock_ifaces=simulated_ifaces, mock_boot_mode="bios"
    )
    assert res_erase["selected_disk"]["path"] == "/dev/nvme0n1"
    assert any(c.get("flag") == "bios_grub" for c in res_erase["storage"]["config"])
    print("  -> PASS: Mode 'auto_erase' excluded USB and built full-disk layout!")

    # Mode 2: dual_boot (retains existing Windows + EFI partitions with preserve=True)
    env_dual = {
        "INSTANT_INSTALL_MODE": "dual_boot",
        "INSTANT_DUAL_BOOT_GB": "50",
    }
    res_dual = adapt_autoinstall_for_hardware(
        env_dual, mock_disks=simulated_disks, mock_ifaces=simulated_ifaces, mock_boot_mode="uefi"
    )
    dual_cfg = res_dual["storage"]["config"]
    preserved = [c for c in dual_cfg if c.get("preserve") is True]
    assert len(preserved) >= 3, f"Expected disk + 2 partitions preserved, got {preserved}"
    new_root = [c for c in dual_cfg if c.get("id") == "instant-root-part"][0]
    assert new_root["size"] == "50G" and new_root["preserve"] is False
    print("  -> PASS: Mode 'dual_boot' preserved existing Windows & EFI partitions and added 50G Instant Server partition!")

    # Mode 3: interactive_select
    env_inter = {"INSTANT_INSTALL_MODE": "interactive_select"}
    res_inter = adapt_autoinstall_for_hardware(
        env_inter, mock_disks=simulated_disks, mock_ifaces=simulated_ifaces, mock_boot_mode="uefi"
    )
    assert res_inter["storage"] is None
    print("  -> PASS: Mode 'interactive_select' leaves Subiquity interactive storage prompt active!")

    # Mode 4: secondary_drive
    env_sec = {"INSTANT_INSTALL_MODE": "secondary_drive"}
    res_sec = adapt_autoinstall_for_hardware(
        env_sec, mock_disks=simulated_disks, mock_ifaces=simulated_ifaces, mock_boot_mode="uefi"
    )
    assert res_sec["selected_disk"]["path"] == "/dev/sdb"
    print("  -> PASS: Mode 'secondary_drive' skipped primary NVMe Disk 0 and selected secondary SATA SSD (/dev/sdb)!")
    return 0


def main() -> int:
    parser = argparse.ArgumentParser(description="Instant Server — P1 Early Hardware & Disk Mode Prober")
    parser.add_argument("--self-test", action="store_true", help="Run hardware prober unit tests")
    parser.add_argument("--env", type=str, default="/run/instant-cidata/instant-server-env.conf")
    parser.add_argument("--autoinstall", type=str, default="/autoinstall.yaml")
    args = parser.parse_args()

    if args.self_test:
        return _run_self_test()

    env = load_instant_env(args.env)
    force_reinstall = env.get("INSTANT_FORCE_REINSTALL", "0") == "1"

    if check_existing_installation_sentinel(force_reinstall):
        sys.stderr.write(
            "\n[INSTANT-SERVER GUARD] Found existing INSTANT_SERVER sentinel on internal disk!\n"
            "Unplug the USB installer stick and power on the laptop to boot into your server.\n"
        )
        run_cmd(["poweroff"])
        return 1

    activate_low_ram_zram_if_needed(env.get("INSTANT_ENABLE_LOW_RAM_ZRAM", "1") == "1")
    adapted = adapt_autoinstall_for_hardware(env)

    if os.path.exists(args.autoinstall):
        try:
            import yaml  # type: ignore
            with open(args.autoinstall, "r", encoding="utf-8") as rf:
                doc = yaml.safe_load(rf) or {}
            target_root = doc.get("autoinstall", doc)
            if adapted["storage"] is not None:
                target_root["storage"] = adapted["storage"]
            else:
                target_root["interactive-sections"] = ["storage"]
            target_root["network"] = adapted["network"]
            with open(args.autoinstall, "w", encoding="utf-8") as wf:
                yaml.safe_dump(doc, wf, default_flow_style=False, sort_keys=False)
            print(
                f"[INSTANT-SERVER P1] Adapted {args.autoinstall}: mode={adapted['install_mode']} "
                f"boot_mode={adapted['boot_mode']}"
            )
        except Exception as exc:
            sys.stderr.write(f"[INSTANT-SERVER P1] Failed to update {args.autoinstall}: {exc}\n")
            return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())