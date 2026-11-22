#!/usr/bin/env python3
"""
disk_ops.py
-----------
Safe block-device discovery, free-gap scanning, and partition operations
(format, create, delete, resize, wipe_disk, select_target) for disk_manager.py.
"""

import json
import os
import pathlib
import re
import shutil
import subprocess
from typing import Callable, Optional

PORT = int(os.environ.get("INSTANT_DISK_MANAGER_PORT", "8088"))
MIN_TARGET_BYTES = 15 * 1024 * 1024 * 1024  # 15 GB minimum for Ubuntu Server
USB_EXCLUDE = set(
    p.strip() for p in os.environ.get("INSTANT_USB_DISKS", "").split(",") if p.strip()
)
PICK_OUT = pathlib.Path(os.environ.get("INSTANT_PICK_OUT", "/tmp/instant_picked_disk"))
INSTALLER_MODE = os.environ.get("INSTANT_INSTALLER_MODE", "1") == "1"


def is_root() -> bool:
    geteuid = getattr(os, "geteuid", None)
    return callable(geteuid) and geteuid() == 0


def run_cmd(cmd: list[str], timeout: int = 120) -> tuple[bool, str]:
    try:
        res = subprocess.run(cmd, capture_output=True, text=True, timeout=timeout)
        out = (res.stdout + "\n" + res.stderr).strip()
        return res.returncode == 0, out
    except Exception as exc:
        return False, str(exc)


def valid_dev_path(path: str) -> bool:
    return bool(re.match(r"^/dev/(sd[a-z]+[0-9]*|nvme[0-9]+n[0-9]+(p[0-9]+)?|vd[a-z]+[0-9]*|mmcblk[0-9]+(p[0-9]+)?|hd[a-z]+[0-9]*)$", path))


def is_protected_usb(dev_path: str, tran: str = "", rm: bool = False) -> bool:
    for u in USB_EXCLUDE:
        if dev_path == u or dev_path.startswith(u):
            return True
    if INSTALLER_MODE and (tran.lower() == "usb" or rm):
        return True
    return False


def get_free_gaps(disk_path: str) -> list[dict]:
    """Use parted -m unit B print free to find unallocated gaps >= 100 MB."""
    ok, out = run_cmd(["parted", "-s", "-m", disk_path, "unit", "B", "print", "free"], timeout=20)
    gaps = []
    if not ok:
        return gaps
    for line in out.splitlines():
        parts = line.rstrip(";").split(":")
        if len(parts) >= 5 and parts[4] == "free":
            try:
                start_b = int(parts[1].rstrip("B"))
                end_b = int(parts[2].rstrip("B"))
                size_b = int(parts[3].rstrip("B"))
                if size_b >= 100 * 1024 * 1024:
                    gaps.append({"start_b": start_b, "end_b": end_b, "size_b": size_b})
            except ValueError:
                pass
    return gaps


def evaluate_target(size_b: int, protected: bool, ro: bool, dtype: str) -> tuple[bool, str]:
    if protected:
        return False, "Protected: Installer USB drive (cannot install onto boot USB)"
    if ro:
        return False, "Read-only media (cannot write OS)"
    if dtype not in ("disk", "part", "free"):
        return False, f"Unsupported device type ({dtype})"
    if size_b < MIN_TARGET_BYTES:
        gb = size_b / (1024 ** 3)
        return False, f"Too small ({gb:.1f} GB — minimum 15.0 GB required)"
    return True, "Valid install target"


def list_disks() -> list[dict]:
    ok, out = run_cmd([
        "lsblk", "-J", "-b", "-o",
        "NAME,PATH,SIZE,TYPE,FSTYPE,LABEL,MOUNTPOINT,MODEL,TRAN,RM,RO,PTTYPE"
    ], timeout=20)
    if not ok:
        return []
    try:
        raw = json.loads(out)
    except Exception:
        return []
    disks = []
    for d in raw.get("blockdevices", []):
        if d.get("type") != "disk":
            continue
        name = str(d.get("name", ""))
        if re.match(r"^(loop|ram|zram|sr|fd|md|dm-)", name):
            continue
        path = str(d.get("path") or f"/dev/{name}")
        size_b = int(d.get("size") or 0)
        tran = str(d.get("tran") or "")
        rm = bool(d.get("rm"))
        ro = bool(d.get("ro"))
        prot = is_protected_usb(path, tran, rm)
        can_sel, reason = evaluate_target(size_b, prot, ro, "disk")
        parts = []
        for ch in d.get("children", []) or []:
            psize = int(ch.get("size") or 0)
            ppath = str(ch.get("path") or f"/dev/{ch.get('name', '')}")
            p_can, p_reason = evaluate_target(psize, prot, bool(ch.get("ro")), "part")
            parts.append({
                "name": ch.get("name", ""),
                "path": ppath,
                "size_b": psize,
                "fstype": ch.get("fstype") or "RAW / Unformatted",
                "label": ch.get("label") or "",
                "mountpoint": ch.get("mountpoint") or "",
                "can_select": p_can,
                "select_reason": p_reason,
            })
        gaps = [] if prot else get_free_gaps(path)
        for g in gaps:
            g_can, g_reason = evaluate_target(g["size_b"], prot, ro, "free")
            g["can_select"] = g_can
            g["select_reason"] = g_reason
        disks.append({
            "name": name,
            "path": path,
            "model": (d.get("model") or "Internal Storage").strip(),
            "size_b": size_b,
            "tran": tran or "sata/nvme",
            "pttype": d.get("pttype") or "Not Initialized",
            "protected": prot,
            "ro": ro,
            "can_select": can_sel,
            "select_reason": reason,
            "partitions": parts,
            "free_gaps": gaps,
        })
    return disks


def unmount_device_tree(dev: str) -> None:
    ok, out = run_cmd(["lsblk", "-lnpo", "NAME,MOUNTPOINT", dev], timeout=15)
    if ok:
        for ln in out.splitlines():
            cols = ln.split(None, 1)
            if len(cols) == 2 and cols[1].strip():
                run_cmd(["umount", "-l", cols[0]], timeout=15)
    run_cmd(["swapoff", dev], timeout=10)


def refresh_kernel_pt(disk: str) -> None:
    run_cmd(["partprobe", disk], timeout=15)
    run_cmd(["udevadm", "settle", "--timeout=10"], timeout=15)


def format_partition_fs(part: str, fstype: str, label: str = "") -> tuple[bool, str]:
    clean_lbl = re.sub(r"[^A-Za-z0-9_-]", "", label)[:16]
    unmount_device_tree(part)
    fs = fstype.lower()
    if fs == "ext4":
        cmd = ["mkfs.ext4", "-F"] + (["-L", clean_lbl] if clean_lbl else []) + [part]
    elif fs == "fat32":
        cmd = ["mkfs.vfat", "-F", "32"] + (["-n", clean_lbl[:11].upper()] if clean_lbl else []) + [part]
    elif fs == "ntfs":
        cmd = ["mkfs.ntfs", "-Q", "-F"] + (["-L", clean_lbl] if clean_lbl else []) + [part]
    elif fs == "exfat":
        cmd = ["mkfs.exfat"] + (["-n", clean_lbl] if clean_lbl else []) + [part]
    elif fs == "btrfs":
        cmd = ["mkfs.btrfs", "-f"] + (["-L", clean_lbl] if clean_lbl else []) + [part]
    elif fs == "xfs":
        cmd = ["mkfs.xfs", "-f"] + (["-L", clean_lbl[:12]] if clean_lbl else []) + [part]
    else:
        return False, f"Unsupported filesystem: {fstype}"
    return run_cmd(cmd, timeout=120)


def handle_disk_action(
    payload: dict,
    subiquity_driver: Optional[Callable[[str], tuple[bool, str]]] = None,
) -> dict:
    if not is_root():
        return {"ok": False, "error": "Root/sudo privileges are required to modify disks and partitions."}
    op = str(payload.get("action", "")).strip()
    target = str(payload.get("device", "")).strip()
    if not valid_dev_path(target):
        return {"ok": False, "error": f"Invalid block device path: {target}"}
    if is_protected_usb(target):
        return {"ok": False, "error": f"Refused: {target} is the protected Installer USB drive."}

    if op == "initialize":
        style = "gpt" if str(payload.get("style", "gpt")).lower() == "gpt" else "msdos"
        unmount_device_tree(target)
        ok, out = run_cmd(["parted", "-s", target, "mklabel", style])
        refresh_kernel_pt(target)
        return {"ok": ok, "output": out or f"Initialized {target} as {style.upper()}."}

    if op == "wipe":
        unmount_device_tree(target)
        run_cmd(["wipefs", "-a", target])
        ok, out = run_cmd(["sgdisk", "--zap-all", target])
        if not ok:
            ok, out = run_cmd(["parted", "-s", target, "mklabel", "gpt"])
        refresh_kernel_pt(target)
        return {"ok": ok, "output": out or f"Wiped all partitions & signatures on {target}."}

    if op == "create_volume":
        start_b = int(payload.get("start_b", 1048576))
        size_gb = float(payload.get("size_gb", 20))
        fstype = str(payload.get("fstype", "ext4"))
        label = str(payload.get("label", ""))
        end_b = start_b + int(size_gb * 1024 * 1024 * 1024)
        ok, out = run_cmd(["parted", "-s", "-a", "optimal", target, "mkpart", "primary", f"{start_b}B", f"{end_b}B"])
        refresh_kernel_pt(target)
        if not ok:
            return {"ok": False, "error": out}
        # Find newest partition on target
        _, p_out = run_cmd(["lsblk", "-lnpo", "NAME,TYPE", target])
        parts = [ln.split()[0] for ln in p_out.splitlines() if len(ln.split()) == 2 and ln.split()[1] == "part"]
        if parts and fstype.lower() != "unformatted":
            f_ok, f_out = format_partition_fs(parts[-1], fstype, label)
            return {"ok": f_ok, "output": f_out or f"Created & formatted {parts[-1]} as {fstype}."}
        return {"ok": True, "output": f"Created new volume on {target}."}

    if op == "format":
        fstype = str(payload.get("fstype", "ext4"))
        label = str(payload.get("label", ""))
        ok, out = format_partition_fs(target, fstype, label)
        return {"ok": ok, "output": out or f"Formatted {target} as {fstype}."}

    if op == "delete_partition":
        m = re.match(r"^(/dev/(?:nvme\d+n\d+|mmcblk\d+))p(\d+)$|^(/dev/[a-z]+)(\d+)$", target)
        if not m:
            return {"ok": False, "error": f"Cannot parse partition number from {target}"}
        parent = m.group(1) or m.group(3)
        pnum = m.group(2) or m.group(4)
        unmount_device_tree(target)
        ok, out = run_cmd(["parted", "-s", parent, "rm", pnum])
        refresh_kernel_pt(parent)
        return {"ok": ok, "output": out or f"Deleted partition {target}."}

    if op in ("shrink", "extend"):
        new_gb = float(payload.get("new_size_gb", 0))
        if new_gb < 1:
            return {"ok": False, "error": "New partition size must be at least 1 GB."}
        m = re.match(r"^(/dev/(?:nvme\d+n\d+|mmcblk\d+))p(\d+)$|^(/dev/[a-z]+)(\d+)$", target)
        if not m:
            return {"ok": False, "error": f"Cannot parse partition number from {target}"}
        parent = m.group(1) or m.group(3)
        pnum = m.group(2) or m.group(4)
        unmount_device_tree(target)
        _, fs_out = run_cmd(["blkid", "-o", "value", "-s", "TYPE", target])
        fs = fs_out.strip().lower()
        new_mb = int(new_gb * 1024)
        if op == "shrink":
            if fs.startswith("ext"):
                run_cmd(["e2fsck", "-fy", target], timeout=180)
                ok, out = run_cmd(["resize2fs", target, f"{new_mb}M"], timeout=180)
                if not ok:
                    return {"ok": False, "error": out}
            elif fs == "ntfs" and shutil.which("ntfsresize"):
                ok, out = run_cmd(["ntfsresize", "-f", "-s", f"{new_mb}M", target], timeout=180)
                if not ok:
                    return {"ok": False, "error": out}
            ok, out = run_cmd(["parted", "-s", "---pretend-input-tty", parent, "resizepart", pnum, f"{new_mb}MiB"])
            refresh_kernel_pt(parent)
            return {"ok": ok, "output": out or f"Shrunk {target} to {new_gb:.1f} GB (freed unallocated space)."}
        else:
            ok, out = run_cmd(["parted", "-s", parent, "resizepart", pnum, f"{new_mb}MiB"])
            refresh_kernel_pt(parent)
            if ok and fs.startswith("ext"):
                run_cmd(["e2fsck", "-fy", target], timeout=180)
                run_cmd(["resize2fs", target], timeout=180)
            elif ok and fs == "ntfs" and shutil.which("ntfsresize"):
                run_cmd(["ntfsresize", "-f", target], timeout=180)
            return {"ok": ok, "output": out or f"Extended {target} to {new_gb:.1f} GB."}

    if op == "relabel":
        label = re.sub(r"[^A-Za-z0-9_-]", "", str(payload.get("label", "")))[:16]
        _, fs_out = run_cmd(["blkid", "-o", "value", "-s", "TYPE", target])
        fs = fs_out.strip().lower()
        if fs.startswith("ext"):
            ok, out = run_cmd(["e2label", target, label])
        elif fs == "vfat":
            ok, out = run_cmd(["fatlabel", target, label[:11].upper()])
        elif fs == "ntfs":
            ok, out = run_cmd(["ntfslabel", target, label])
        else:
            return {"ok": False, "error": f"Relabel not supported for filesystem: {fs or 'unknown'}"}
        return {"ok": ok, "output": out or f"Updated label on {target} to '{label}'."}

    if op == "select_target":
        mode = str(payload.get("mode", "whole"))
        disks = list_disks()
        chosen_ok = False
        reason = "Target not found"
        for d in disks:
            if d["path"] == target:
                chosen_ok, reason = d["can_select"], d["select_reason"]
                break
            for p in d["partitions"]:
                if p["path"] == target:
                    chosen_ok, reason = p["can_select"], p["select_reason"]
                    break
        if not chosen_ok:
            return {"ok": False, "error": f"Cannot select {target}: {reason}"}
        PICK_OUT.write_text(f"{target}|{mode}\n", encoding="utf-8")
        live_ok, live_msg = (subiquity_driver(target) if callable(subiquity_driver) else (False, "no driver"))
        out_msg = live_msg if live_ok else f"Selected {target} ({mode}). Continuing Ubuntu Server installation automatically..."
        return {"ok": True, "selected": target, "mode": mode, "live_applied": live_ok, "output": out_msg}

    return {"ok": False, "error": f"Unknown disk action: {op}"}