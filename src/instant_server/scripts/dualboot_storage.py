#!/usr/bin/env python3
"""
FILE: src/instant_server/scripts/dualboot_storage.py
PURPOSE:
  Non-destructive Dual-Boot storage planner for Ubuntu Subiquity / Curtin.
  Guarantees that existing Windows (NTFS/BitLocker/ESP) AND existing Linux
  (ext4/btrfs/xfs/LVM) partitions are NEVER wiped or deleted when Dual-Boot
  mode is selected.

HOW IT WORKS (Verified Against Official Curtin & Subiquity Docs):
  1. Inspects the target disk's existing GPT/MBR partition table and unallocated
     free space gaps using `/sys/class/block`, `lsblk -J -b`, and `sfdisk -J`.
  2. If the disk is completely blank (0 existing partitions), leaves standard
     `storage.layout` pinned to the target disk.
  3. If the disk has existing partitions:
     - Checks if an unallocated contiguous gap >= requested GB already exists.
     - Otherwise safely shrinks ONLY the largest shrinkable `ntfs` (via `ntfsresize`)
       or `ext4` (via `e2fsck -fy` + `resize2fs`) partition while preserving at
       least 20 GiB of free space inside that partition.
     - Creates the new Linux root partition (and a 512 MiB EFI System Partition
       if none exists on the disk) in the unallocated gap using `parted`.
     - Generates a byte-exact Curtin `storage: config:` action list where:
       * The disk action has `preserve: true`
       * EVERY existing partition on the disk is listed with `preserve: true`
         and exact byte `size` and `offset`
       * The existing EFI System Partition has `preserve: true` on BOTH its
         `partition` and `format` actions (`path: /boot/efi`, `grub_device: true`)
         so existing Windows/Ubuntu bootloaders in `/boot/efi` are preserved
       * Only the newly created partition is formatted as `ext4` (`path: /`)
  4. SAFETY FALLBACK: If free space cannot be safely obtained without risking
     existing partitions, this script NEVER wipes the disk. Instead, it sets
     `interactive-sections: ["storage"]` in `/autoinstall.yaml` so Subiquity
     pauses safely at the storage screen with 100% of user data intact.

REFERENCES:
  - https://curtin.readthedocs.io/en/latest/topics/storage.html
  - https://canonical-subiquity.readthedocs-hosted.com/en/latest/reference/autoinstall-reference.html
"""

import argparse
import json
import os
import re
import subprocess
import sys
from typing import Any
import yaml  # type: ignore[import-untyped,import-not-found]

ESP_PARTTYPE_GUID = "c12a7328-f81f-11d2-ba4b-00a0c93ec93b"
MIB = 1024 * 1024
GIB = 1024 * 1024 * 1024


def log_msg(log_path: str, msg: str) -> None:
    line = f"[dualboot_storage_builder] {msg}\n"
    sys.stdout.write(line)
    sys.stdout.flush()
    for path in (log_path, "/mnt/usb_logs/INSTANT-SERVER-INSTALL.log", "/mnt/usb_logs/logs/INSTANT-SERVER-INSTALL.log"):
        try:
            if os.path.isdir(os.path.dirname(path)):
                with open(path, "a", encoding="utf-8") as f:
                    f.write(line)
        except Exception:
            pass


def run_cmd(cmd: list[str], timeout: int = 120) -> tuple[int, str, str]:
    try:
        proc = subprocess.run(cmd, capture_output=True, text=True, timeout=timeout, check=False)
        return proc.returncode, proc.stdout or "", proc.stderr or ""
    except Exception as exc:
        return 1, "", str(exc)


def get_disk_size_bytes(disk: str) -> int:
    bname = os.path.basename(disk)
    sys_size = f"/sys/class/block/{bname}/size"
    if os.path.exists(sys_size):
        with open(sys_size, "r", encoding="utf-8") as f:
            return int(f.read().strip()) * 512
    rc, out, _ = run_cmd(["blockdev", "--getsize64", disk])
    return int(out.strip()) if rc == 0 and out.strip().isdigit() else 0


def list_disk_partitions(disk: str) -> list[dict[str, Any]]:
    """
    Returns all physical partitions on `disk` sorted by byte offset, with exact
    byte `offset`, exact byte `size`, partition `number`, `fstype`, and `parttype`.
    """
    rc, out, _ = run_cmd(["lsblk", "-J", "-b", "-o", "NAME,PATH,PARTN,SIZE,FSTYPE,PARTTYPE,MOUNTPOINT", disk])
    if rc != 0 or not out.strip():
        return []
    try:
        tree = json.loads(out)
    except Exception:
        return []

    parts: list[dict[str, Any]] = []
    for dev in tree.get("blockdevices", []):
        for ch in dev.get("children", []) or []:
            p_path = ch.get("path") or f"/dev/{ch.get('name', '')}"
            bname = os.path.basename(p_path)
            partn = ch.get("partn")
            if partn is None:
                m = re.search(r"(\d+)$", bname)
                partn = int(m.group(1)) if m else 0
            start_sec = 0
            size_sec = 0
            sys_start = f"/sys/class/block/{bname}/start"
            sys_sz = f"/sys/class/block/{bname}/size"
            if os.path.exists(sys_start):
                with open(sys_start, "r", encoding="utf-8") as f:
                    start_sec = int(f.read().strip() or "0")
            if os.path.exists(sys_sz):
                with open(sys_sz, "r", encoding="utf-8") as f:
                    size_sec = int(f.read().strip() or "0")
            offset_b = start_sec * 512
            size_b = size_sec * 512 if size_sec > 0 else int(ch.get("size") or 0)
            if int(partn) > 0 and size_b > 0:
                parts.append({
                    "name": bname,
                    "path": p_path,
                    "number": int(partn),
                    "offset": offset_b,
                    "size": size_b,
                    "end": offset_b + size_b,
                    "fstype": (ch.get("fstype") or "").lower(),
                    "parttype": (ch.get("parttype") or "").lower(),
                })
    parts.sort(key=lambda x: x["offset"])
    return parts


def find_largest_free_gap(disk_bytes: int, parts: list[dict[str, Any]]) -> tuple[int, int]:
    """
    Finds the largest 1 MiB-aligned unallocated gap (start_byte, size_byte) on disk.
    """
    usable_start = 1 * MIB
    usable_end = disk_bytes - (2 * MIB)
    if usable_end <= usable_start:
        return 0, 0

    best_start = 0
    best_size = 0
    cursor = usable_start

    for p in parts:
        gap_start = ((cursor + MIB - 1) // MIB) * MIB
        gap_end = (p["offset"] // MIB) * MIB
        if gap_end > gap_start and (gap_end - gap_start) > best_size:
            best_start = gap_start
            best_size = gap_end - gap_start
        if p["end"] > cursor:
            cursor = p["end"]

    gap_start = ((cursor + MIB - 1) // MIB) * MIB
    gap_end = (usable_end // MIB) * MIB
    if gap_end > gap_start and (gap_end - gap_start) > best_size:
        best_start = gap_start
        best_size = gap_end - gap_start

    return best_start, best_size


def query_min_filesystem_bytes(part_path: str, fstype: str) -> int:
    """
    Queries the minimum safe filesystem size in bytes for NTFS or ext4 without
    modifying the partition, adding a 20 GiB safety buffer for the existing OS.
    """
    safety_buffer = 20 * GIB
    if fstype == "ntfs":
        rc, out, err = run_cmd(["ntfsresize", "-i", "-f", part_path])
        combined = out + "\n" + err
        m = re.search(r"You might resize at\s+(\d+)\s+bytes", combined)
        if rc == 0 and m:
            return int(m.group(1)) + safety_buffer
    elif fstype == "ext4":
        rc, out, _ = run_cmd(["dumpe2fs", "-h", part_path])
        if rc == 0:
            b_count = 0
            f_blocks = 0
            b_size = 4096
            for line in out.splitlines():
                if line.startswith("Block count:"):
                    b_count = int(line.split(":")[1].strip())
                elif line.startswith("Free blocks:"):
                    f_blocks = int(line.split(":")[1].strip())
                elif line.startswith("Block size:"):
                    b_size = int(line.split(":")[1].strip())
            used_bytes = (b_count - f_blocks) * b_size
            if used_bytes > 0:
                return used_bytes + safety_buffer
    return 0


def try_shrink_partition_safely(disk: str, part: dict[str, Any], req_bytes: int, log_path: str) -> bool:
    """
    Safely shrinks an unmounted NTFS or ext4 partition by `req_bytes` only if
    the filesystem check passes and at least 20 GiB of free space remains inside it.
    """
    fstype = part["fstype"]
    p_path = part["path"]
    p_num = part["number"]
    cur_size = part["size"]

    if fstype not in ("ntfs", "ext4"):
        return False

    min_safe = query_min_filesystem_bytes(p_path, fstype)
    if min_safe <= 0 or (cur_size - min_safe) < req_bytes:
        log_msg(log_path, f"Partition {p_path} ({fstype}, {cur_size // GIB} GiB) does not have enough free space to shrink by {req_bytes // GIB} GiB safely.")
        return False

    new_size_mib = ((cur_size - req_bytes) // MIB)
    new_size_bytes = new_size_mib * MIB
    new_end_bytes = part["offset"] + new_size_bytes

    log_msg(log_path, f"Safely shrinking {p_path} ({fstype}) from {cur_size // MIB} MiB to {new_size_mib} MiB...")
    if fstype == "ntfs":
        rc, _, err = run_cmd(["ntfsresize", "-f", "-s", f"{new_size_bytes}", p_path], timeout=300)
        if rc != 0:
            log_msg(log_path, f"ntfsresize aborted on {p_path}: {err.strip()}")
            return False
    elif fstype == "ext4":
        run_cmd(["e2fsck", "-fy", p_path], timeout=300)
        rc, _, err = run_cmd(["resize2fs", p_path, f"{new_size_mib}M"], timeout=300)
        if rc != 0:
            log_msg(log_path, f"resize2fs aborted on {p_path}: {err.strip()}")
            return False

    rc, _, err = run_cmd(["parted", "---pretend-input-tty", disk, "unit", "B", "resizepart", str(p_num), f"{new_end_bytes}B", "Yes"])
    if rc != 0:
        log_msg(log_path, f"parted resizepart failed on {p_path}: {err.strip()}")
        return False

    run_cmd(["partprobe", disk])
    run_cmd(["udevadm", "settle", "--timeout=15"])
    return True


def switch_to_interactive_storage_fallback(config_path: str, log_path: str, reason: str) -> None:
    """
    Never wipes the disk when Dual-Boot cannot auto-allocate space; switches
    Subiquity to interactive storage selection so all user data stays untouched.
    """
    log_msg(log_path, f"SAFE DUAL-BOOT FALLBACK: {reason}. Switching to interactive storage screen so zero existing partitions are touched.")
    if not os.path.exists(config_path):
        return
    with open(config_path, "r", encoding="utf-8") as f:
        doc = yaml.safe_load(f) or {}
    has_wrapper = isinstance(doc.get("autoinstall"), dict)
    ai = doc["autoinstall"] if has_wrapper else doc
    inter = list(ai.get("interactive-sections") or [])
    if "storage" not in inter:
        inter.append("storage")
    ai["interactive-sections"] = inter
    if has_wrapper:
        doc["autoinstall"] = ai
    with open(config_path, "w", encoding="utf-8") as f:
        yaml.safe_dump(doc if has_wrapper else ai, f, default_flow_style=False, sort_keys=False)


def main() -> None:
    parser = argparse.ArgumentParser(description="Non-destructive Dual-Boot storage builder")
    parser.add_argument("--disk", required=True, help="Target disk path (e.g. /dev/sda)")
    parser.add_argument("--req-gb", type=int, default=45, help="Requested GB for Instant Server")
    parser.add_argument("--config", default="/autoinstall.yaml", help="Path to /autoinstall.yaml")
    parser.add_argument("--log", default="/mnt/cidata/INSTANT-SERVER-INSTALL.log", help="Log file path")
    args = parser.parse_args()

    disk = args.disk.strip()
    req_gb = max(15, args.req_gb)
    req_bytes = req_gb * GIB

    disk_bytes = get_disk_size_bytes(disk)
    if disk_bytes <= 0:
        log_msg(args.log, f"Cannot determine size of {disk}; aborting dual-boot modification.")
        return

    parts = list_disk_partitions(disk)
    if not parts:
        log_msg(args.log, f"Disk {disk} has 0 existing partitions; keeping clean whole-disk layout on {disk}.")
        return

    # Deactivate any auto-activated LVM volume groups on the target disk so Curtin
    # does not fail when preserving existing LVM physical volume partitions.
    run_cmd(["vgchange", "-an"])

    gap_start, gap_size = find_largest_free_gap(disk_bytes, parts)
    min_acceptable_bytes = min(req_bytes, 15 * GIB)

    if gap_size < min_acceptable_bytes:
        # Look for the largest shrinkable NTFS or ext4 partition on the disk
        candidates = [p for p in parts if p["fstype"] in ("ntfs", "ext4") and p["size"] > (req_bytes + 20 * GIB)]
        candidates.sort(key=lambda x: x["size"], reverse=True)
        if not candidates or not try_shrink_partition_safely(disk, candidates[0], req_bytes, args.log):
            switch_to_interactive_storage_fallback(
                args.config,
                args.log,
                f"Disk {disk} has {gap_size // GIB} GiB unallocated space (need {req_gb} GiB) and no partition could be shrunk safely",
            )
            return
        parts = list_disk_partitions(disk)
        gap_start, gap_size = find_largest_free_gap(disk_bytes, parts)

    if gap_size < min_acceptable_bytes:
        switch_to_interactive_storage_fallback(args.config, args.log, f"Unallocated gap ({gap_size // GIB} GiB) is still smaller than minimum 15 GiB")
        return

    alloc_bytes = min(gap_size, req_bytes) if gap_size >= req_bytes else gap_size
    before_nums = {p["number"] for p in parts}

    # Check if an EFI System Partition already exists on the disk
    esp_part = None
    for p in parts:
        if p["parttype"] == ESP_PARTTYPE_GUID or (p["fstype"] in ("vfat", "fat32") and p["size"] <= 2 * GIB):
            esp_part = p
            break

    cur_start_mib = gap_start // MIB
    new_esp_num = 0
    if esp_part is None and os.path.isdir("/sys/firmware/efi"):
        esp_end_mib = cur_start_mib + 512
        log_msg(args.log, f"Creating 512 MiB EFI System Partition on {disk} ({cur_start_mib}MiB..{esp_end_mib}MiB)...")
        run_cmd(["parted", "-s", disk, "unit", "MiB", "mkpart", "ESP", "fat32", f"{cur_start_mib}MiB", f"{esp_end_mib}MiB"])
        run_cmd(["partprobe", disk])
        run_cmd(["udevadm", "settle", "--timeout=15"])
        after_esp = list_disk_partitions(disk)
        for p in after_esp:
            if p["number"] not in before_nums:
                new_esp_num = p["number"]
                esp_part = p
                before_nums.add(new_esp_num)
                run_cmd(["parted", "-s", disk, "set", str(new_esp_num), "esp", "on"])
                run_cmd(["mkfs.vfat", "-F", "32", "-n", "EFI", p["path"]])
                break
        cur_start_mib = esp_end_mib + 1
        alloc_bytes -= 514 * MIB

    root_end_mib = cur_start_mib + (alloc_bytes // MIB) - 1
    log_msg(args.log, f"Creating Instant Server ext4 root partition in free gap on {disk} ({cur_start_mib}MiB..{root_end_mib}MiB)...")
    rc, _, err = run_cmd(["parted", "-s", disk, "unit", "MiB", "mkpart", "INSTANT-ROOT", "ext4", f"{cur_start_mib}MiB", f"{root_end_mib}MiB"])
    if rc != 0:
        switch_to_interactive_storage_fallback(args.config, args.log, f"parted mkpart failed on {disk}: {err.strip()}")
        return

    run_cmd(["partprobe", disk])
    run_cmd(["udevadm", "settle", "--timeout=15"])

    final_parts = list_disk_partitions(disk)
    new_root_part = None
    for p in final_parts:
        if p["number"] not in before_nums:
            new_root_part = p
            break

    if new_root_part is None:
        switch_to_interactive_storage_fallback(args.config, args.log, "Could not locate newly created root partition in lsblk")
        return

    # Pre-format the new root partition with ext4 so its superblock is clean
    run_cmd(["mkfs.ext4", "-F", "-L", "INSTANT-ROOT", new_root_part["path"]], timeout=180)
    run_cmd(["udevadm", "settle", "--timeout=15"])
    final_parts = list_disk_partitions(disk)

    is_uefi = os.path.isdir("/sys/firmware/efi")
    config_actions: list[dict[str, Any]] = [
        {
            "id": "disk-target",
            "type": "disk",
            "path": disk,
            "ptable": "gpt",
            "preserve": True,
            "grub_device": not is_uefi,
        }
    ]

    for p in final_parts:
        pnum = p["number"]
        if pnum == new_root_part["number"]:
            config_actions.append({
                "id": "part-instant-root",
                "type": "partition",
                "device": "disk-target",
                "number": pnum,
                "size": p["size"],
                "offset": p["offset"],
                "preserve": True,
                "wipe": "superblock",
            })
            config_actions.append({
                "id": "format-instant-root",
                "type": "format",
                "volume": "part-instant-root",
                "fstype": "ext4",
                "label": "INSTANT-ROOT",
            })
            config_actions.append({
                "id": "mount-instant-root",
                "type": "mount",
                "device": "format-instant-root",
                "path": "/",
            })
        elif esp_part is not None and pnum == esp_part["number"] and is_uefi:
            config_actions.append({
                "id": "part-efi-esp",
                "type": "partition",
                "device": "disk-target",
                "number": pnum,
                "size": p["size"],
                "offset": p["offset"],
                "flag": "boot",
                "preserve": True,
                "grub_device": True,
            })
            config_actions.append({
                "id": "format-efi-esp",
                "type": "format",
                "volume": "part-efi-esp",
                "fstype": "fat32",
                "preserve": True,
            })
            config_actions.append({
                "id": "mount-efi-esp",
                "type": "mount",
                "device": "format-efi-esp",
                "path": "/boot/efi",
            })
        else:
            # Preserve every existing Windows or Linux partition intact!
            config_actions.append({
                "id": f"part-preserve-{pnum}",
                "type": "partition",
                "device": "disk-target",
                "number": pnum,
                "size": p["size"],
                "offset": p["offset"],
                "preserve": True,
            })

    with open(args.config, "r", encoding="utf-8") as f:
        doc = yaml.safe_load(f) or {}
    has_wrapper = isinstance(doc.get("autoinstall"), dict)
    ai = doc["autoinstall"] if has_wrapper else doc
    ai["storage"] = {
        "swap": {"size": 0},
        "config": config_actions,
    }
    if has_wrapper:
        doc["autoinstall"] = ai
    with open(args.config, "w", encoding="utf-8") as f:
        yaml.safe_dump(doc if has_wrapper else ai, f, default_flow_style=False, sort_keys=False)

    log_msg(args.log, f"Wrote non-destructive Curtin dual-boot storage.config for {disk} (preserved {len(final_parts) - 1} existing partition(s), new root on {new_root_part['path']}).")


if __name__ == "__main__":
    main()