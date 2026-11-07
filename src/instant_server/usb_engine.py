# src/instant_server/usb_engine.py
from __future__ import annotations

import argparse
from dataclasses import asdict, dataclass, field
import importlib
from pathlib import Path
import json
import os
import plistlib
import re
import shutil
import struct
import subprocess
import sys
import tempfile
import time
import uuid
import zlib
from typing import Any, Callable, Dict, List, Optional

_CURRENT_DIR = os.path.dirname(os.path.abspath(__file__))
if _CURRENT_DIR not in sys.path:
    sys.path.insert(0, _CURRENT_DIR)

iso_manager: Any = importlib.import_module("iso_manager")

SECTOR_SIZE = 512
GPT_HEADER_SIGNATURE = b"EFI PART"
GPT_REVISION_1_0 = 0x00010000

BASIC_DATA_GUID_BYTES = uuid.UUID("EBD0A0A2-B9E5-4433-87C0-68B6B72699C7").bytes_le

def _win_no_window_kwargs() -> Dict[str, Any]:
    if sys.platform.startswith("win"):
        flags = getattr(subprocess, "CREATE_NO_WINDOW", 0x08000000)
        return {"creationflags": flags}
    return {}

@dataclass
class UsbDeviceCandidate:

    device_id: str
    device_node: str
    raw_device_node: str
    model: str
    vendor: str
    serial: str
    size_bytes: int
    size_gb: float
    bus: str
    removable: bool
    mountpoints: List[str] = field(default_factory=list)
    triple_lock_passed: bool = False
    block_reasons: List[str] = field(default_factory=list)

def emit_event(
    event_type: str,
    message: str,
    data: Optional[Dict[str, Any]] = None,
    callback: Optional[Callable[[Dict[str, Any]], None]] = None,
) -> Dict[str, Any]:
    payload = {
        "event": event_type,
        "module": "USB_ENGINE",
        "project": "Instant Server",
        "message": message,
        "data": data or {},
    }
    if callback:
        callback(payload)
    return payload

SYSTEM_CRITICAL_MOUNTS = {
    "/",
    "/boot",
    "/boot/efi",
    "/home",
    "/usr",
    "/var",
    "/system/volumes/data",
    "c:\\",
    "c:/",
    "c:",
}

def evaluate_triple_lock(
    device_id: str,
    device_node: str,
    model: str,
    vendor: str,
    serial: str,
    size_bytes: int,
    bus: str,
    removable: bool,
    mountpoints: List[str],
    max_safe_usb_gb: float = 128.0,
    allow_large_usb: bool = False,
) -> UsbDeviceCandidate:
    reasons: List[str] = []
    norm_bus = (bus or "").strip().lower()
    size_gb = round(size_bytes / (1024 ** 3), 2)

    if norm_bus != "usb":
        reasons.append(f"Lock 1 Failed: Bus type is '{bus}' (must be USB).")
    if not removable:
        reasons.append("Lock 1 Failed: Device is marked non-removable (internal drive).")

    for mp in mountpoints:
        clean_mp = mp.strip().lower()
        if clean_mp in SYSTEM_CRITICAL_MOUNTS or clean_mp.startswith(("/boot", "c:\\windows")):
            reasons.append(f"Lock 2 Failed: Hosts critical OS mountpoint '{mp}'.")

    if size_bytes < 4 * (1024 ** 3):
        reasons.append(f"Lock 3 Failed: Capacity ({size_gb} GB) is smaller than 4 GB minimum.")
    if size_gb > max_safe_usb_gb and not allow_large_usb:
        reasons.append(
            f"Lock 3 Warning: Capacity ({size_gb} GB) exceeds {max_safe_usb_gb} GB flash stick ceiling "
            "(likely an external backup SSD/HDD; pass allow_large_usb=True to override)."
        )

    raw_node = device_node
    if device_node.startswith("/dev/disk"):
        raw_node = device_node.replace("/dev/disk", "/dev/rdisk", 1)

    return UsbDeviceCandidate(
        device_id=device_id,
        device_node=device_node,
        raw_device_node=raw_node,
        model=model or "USB Flash Drive",
        vendor=vendor or "Generic",
        serial=serial or "N/A",
        size_bytes=size_bytes,
        size_gb=size_gb,
        bus=norm_bus,
        removable=removable,
        mountpoints=mountpoints,
        triple_lock_passed=(len(reasons) == 0),
        block_reasons=reasons,
    )

def scan_host_usb_devices(
    mock_raw_devices: Optional[List[Dict[str, Any]]] = None,
    allow_large_usb: bool = False,
) -> List[UsbDeviceCandidate]:
    if mock_raw_devices is not None:
        return [
            evaluate_triple_lock(
                device_id=d["id"],
                device_node=d["node"],
                model=d.get("model", ""),
                vendor=d.get("vendor", ""),
                serial=d.get("serial", ""),
                size_bytes=int(d["size_bytes"]),
                bus=d.get("bus", ""),
                removable=bool(d.get("removable", False)),
                mountpoints=list(d.get("mountpoints", [])),
                allow_large_usb=allow_large_usb,
            )
            for d in mock_raw_devices
        ]

    plat = sys.platform.lower()
    if plat.startswith("linux"):
        return _scan_linux_lsblk(allow_large_usb=allow_large_usb)
    if plat == "darwin":
        return _scan_macos_diskutil(allow_large_usb=allow_large_usb)
    if plat.startswith("win"):
        return _scan_windows_powershell(allow_large_usb=allow_large_usb)
    return []

def _scan_linux_lsblk(allow_large_usb: bool = False) -> List[UsbDeviceCandidate]:
    out: List[UsbDeviceCandidate] = []
    try:
        res = subprocess.run(
            ["lsblk", "-b", "-J", "-o", "NAME,PATH,SIZE,RM,ROTA,TRAN,VENDOR,MODEL,SERIAL,MOUNTPOINT,TYPE"],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            check=False,
        )
        data = json.loads(res.stdout or "{}")
        for dev in data.get("blockdevices", []):
            if dev.get("type") != "disk":
                continue
            mps = _collect_mountpoints(dev)
            out.append(
                evaluate_triple_lock(
                    device_id=dev.get("name", ""),
                    device_node=dev.get("path") or f"/dev/{dev.get('name')}",
                    model=(dev.get("model") or "").strip(),
                    vendor=(dev.get("vendor") or "").strip(),
                    serial=(dev.get("serial") or "").strip(),
                    size_bytes=int(dev.get("size") or 0),
                    bus=(dev.get("tran") or "").strip(),
                    removable=bool(dev.get("rm")),
                    mountpoints=mps,
                    allow_large_usb=allow_large_usb,
                )
            )
    except Exception:
        pass
    return out

def _collect_mountpoints(node: Dict[str, Any]) -> List[str]:
    mps: List[str] = []
    mp = node.get("mountpoint")
    if mp:
        mps.append(str(mp))
    for ch in node.get("children", []) or []:
        mps.extend(_collect_mountpoints(ch))
    return mps

def _scan_macos_diskutil(allow_large_usb: bool = False) -> List[UsbDeviceCandidate]:
    out: List[UsbDeviceCandidate] = []
    try:
        res = subprocess.run(
            ["diskutil", "list", "-plist"],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            check=False,
        )
        plist_data = plistlib.loads(res.stdout or b"")
        for disk_name in plist_data.get("WholeDisks", []):
            info_res = subprocess.run(
                ["diskutil", "info", "-plist", str(disk_name)],
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                check=False,
            )
            info = plistlib.loads(info_res.stdout or b"")
            if info.get("VirtualOrPhysical") == "Virtual":
                continue
            bus = str(info.get("BusProtocol") or "").lower()
            removable = bool(info.get("RemovableMedia") or info.get("Ejectable"))
            size_bytes = int(info.get("TotalSize") or info.get("Size") or 0)
            model = str(info.get("MediaName") or info.get("IORegistryEntryName") or "USB Drive")
            mps: List[str] = []
            if info.get("MountPoint"):
                mps.append(str(info["MountPoint"]))
            out.append(
                evaluate_triple_lock(
                    device_id=str(disk_name),
                    device_node=f"/dev/{disk_name}",
                    model=model,
                    vendor="",
                    serial=str(info.get("DeviceIdentifier") or ""),
                    size_bytes=size_bytes,
                    bus=bus,
                    removable=removable,
                    mountpoints=mps,
                    allow_large_usb=allow_large_usb,
                )
            )
    except Exception:
        pass
    return out

def _scan_windows_powershell(allow_large_usb: bool = False) -> List[UsbDeviceCandidate]:
    out: List[UsbDeviceCandidate] = []
    ps_cmd = (
        "Get-Disk | ForEach-Object { "
        "$d = $_; "
        "$letters = @(Get-Partition -DiskNumber $d.Number -ErrorAction SilentlyContinue | "
        "Where-Object { $_.DriveLetter } | ForEach-Object { \"$($_.DriveLetter):\\\" }); "
        "[PSCustomObject]@{ "
        "Number=$d.Number; FriendlyName=$d.FriendlyName; SerialNumber=$d.SerialNumber; "
        "Size=$d.Size; BusType=$d.BusType; IsSystem=$d.IsSystem; IsBoot=$d.IsBoot; "
        "MountPoints=$letters } } | ConvertTo-Json -Depth 3"
    )
    try:
        res = subprocess.run(
            ["powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", ps_cmd],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            check=False,
            **_win_no_window_kwargs(),
        )
        raw = json.loads(res.stdout or "[]")
        if isinstance(raw, dict):
            raw = [raw]
        for d in raw:
            num = d.get("Number")
            bus = str(d.get("BusType") or "").lower()
            is_sys = bool(d.get("IsSystem") or d.get("IsBoot"))
            raw_mps = d.get("MountPoints") or []
            if isinstance(raw_mps, str):
                raw_mps = [raw_mps]
            mps = list(raw_mps)
            if is_sys and "C:\\" not in mps:
                mps.append("C:\\")
            out.append(
                evaluate_triple_lock(
                    device_id=f"PhysicalDrive{num}",
                    device_node=f"\\\\.\\PhysicalDrive{num}",
                    model=str(d.get("FriendlyName") or ""),
                    vendor="",
                    serial=str(d.get("SerialNumber") or ""),
                    size_bytes=int(d.get("Size") or 0),
                    bus=bus,
                    removable=(bus == "usb" and not is_sys),
                    mountpoints=mps,
                    allow_large_usb=allow_large_usb,
                )
            )
    except Exception:
        pass
    return out

GPT_HDR_FMT = "<8sIIIIQQQQ16sQIII"
GPT_HDR_SIZE = 92
GPT_ENTRY_FMT = "<16s16sQQQ72s"
GPT_ENTRY_SIZE = 128

def relocate_gpt_and_append_cidata(
    disk_or_image_path: str,
    cidata_size_mb: int = 128,
    event_callback: Optional[Callable[[Dict[str, Any]], None]] = None,
) -> Dict[str, Any]:
    total_bytes = os.path.getsize(disk_or_image_path)
    total_sectors = total_bytes // SECTOR_SIZE
    if total_sectors < 4096:
        raise ValueError(f"Disk/image {disk_or_image_path} is too small ({total_bytes} bytes) for GPT.")

    with open(disk_or_image_path, "r+b") as fh:
        fh.seek(1 * SECTOR_SIZE)
        lba1 = fh.read(SECTOR_SIZE)
        raw_hdr = struct.unpack(GPT_HDR_FMT, lba1[:GPT_HDR_SIZE])
        sig = bytes(raw_hdr[0])
        rev = int(raw_hdr[1])
        first_usable = int(raw_hdr[7])
        disk_guid = bytes(raw_hdr[9])
        part_lba = int(raw_hdr[10])
        num_entries = int(raw_hdr[11])
        entry_size = int(raw_hdr[12])

        if sig != GPT_HEADER_SIGNATURE or entry_size != GPT_ENTRY_SIZE:
            raise ValueError(f"Invalid GPT signature at LBA 1 in {disk_or_image_path}: {sig!r}")

        table_bytes_len = num_entries * entry_size
        table_sectors = (table_bytes_len + SECTOR_SIZE - 1) // SECTOR_SIZE

        fh.seek(part_lba * SECTOR_SIZE)
        entries_raw = bytearray(fh.read(table_bytes_len))

        max_used_lba = first_usable - 1
        free_slot_idx: Optional[int] = None
        for idx in range(num_entries):
            offset = idx * entry_size
            entry_slice = entries_raw[offset : offset + entry_size]
            raw_entry = struct.unpack(GPT_ENTRY_FMT, entry_slice)
            type_guid = bytes(raw_entry[0])
            end_lba = int(raw_entry[3])
            if type_guid == b"\x00" * 16:
                if free_slot_idx is None:
                    free_slot_idx = idx
            else:
                if end_lba > max_used_lba:
                    max_used_lba = end_lba

        if free_slot_idx is None:
            raise RuntimeError("No free GPT partition slots remaining to append cidata partition.")

        new_backup_hdr_lba = total_sectors - 1
        new_backup_table_lba = new_backup_hdr_lba - table_sectors
        new_last_usable_lba = new_backup_table_lba - 1

        align = 2048
        cidata_start_lba = ((max_used_lba + align) // align) * align
        cidata_sectors = (cidata_size_mb * 1024 * 1024) // SECTOR_SIZE
        cidata_end_lba = cidata_start_lba + cidata_sectors - 1

        if cidata_end_lba > new_last_usable_lba:
            raise ValueError(
                f"Insufficient unallocated tail space on USB for {cidata_size_mb} MB cidata partition "
                f"(needed LBA {cidata_start_lba}..{cidata_end_lba}, last usable LBA is {new_last_usable_lba})."
            )

        part_guid_le = uuid.uuid4().bytes_le
        name_bytes = "cidata".encode("utf-16le").ljust(72, b"\x00")[:72]
        new_entry = struct.pack(
            GPT_ENTRY_FMT,
            BASIC_DATA_GUID_BYTES,
            part_guid_le,
            cidata_start_lba,
            cidata_end_lba,
            0,
            name_bytes,
        )
        slot_offset = free_slot_idx * entry_size
        entries_raw[slot_offset : slot_offset + entry_size] = new_entry

        new_entries_crc = zlib.crc32(entries_raw) & 0xFFFFFFFF

        primary_hdr_zero_crc = struct.pack(
            GPT_HDR_FMT,
            GPT_HEADER_SIGNATURE,
            rev,
            GPT_HDR_SIZE,
            0,
            0,
            1,
            new_backup_hdr_lba,
            first_usable,
            new_last_usable_lba,
            disk_guid,
            part_lba,
            num_entries,
            entry_size,
            new_entries_crc,
        )
        primary_crc = zlib.crc32(primary_hdr_zero_crc) & 0xFFFFFFFF
        primary_hdr_final = (
            primary_hdr_zero_crc[:16]
            + struct.pack("<I", primary_crc)
            + primary_hdr_zero_crc[20:]
        ).ljust(SECTOR_SIZE, b"\x00")

        backup_hdr_zero_crc = struct.pack(
            GPT_HDR_FMT,
            GPT_HEADER_SIGNATURE,
            rev,
            GPT_HDR_SIZE,
            0,
            0,
            new_backup_hdr_lba,
            1,
            first_usable,
            new_last_usable_lba,
            disk_guid,
            new_backup_table_lba,
            num_entries,
            entry_size,
            new_entries_crc,
        )
        backup_crc = zlib.crc32(backup_hdr_zero_crc) & 0xFFFFFFFF
        backup_hdr_final = (
            backup_hdr_zero_crc[:16]
            + struct.pack("<I", backup_crc)
            + backup_hdr_zero_crc[20:]
        ).ljust(SECTOR_SIZE, b"\x00")

        fh.seek(1 * SECTOR_SIZE)
        fh.write(primary_hdr_final)
        fh.seek(part_lba * SECTOR_SIZE)
        fh.write(entries_raw)

        fh.seek(new_backup_table_lba * SECTOR_SIZE)
        fh.write(entries_raw)
        fh.seek(new_backup_hdr_lba * SECTOR_SIZE)
        fh.write(backup_hdr_final)
        fh.flush()
        os.fsync(fh.fileno())

    result = {
        "partition_number": free_slot_idx + 1,
        "cidata_start_lba": cidata_start_lba,
        "cidata_end_lba": cidata_end_lba,
        "cidata_offset_bytes": cidata_start_lba * SECTOR_SIZE,
        "cidata_size_bytes": cidata_sectors * SECTOR_SIZE,
        "backup_gpt_lba": new_backup_hdr_lba,
        "primary_header_crc32": hex(primary_crc),
        "backup_header_crc32": hex(backup_crc),
        "entries_crc32": hex(new_entries_crc),
    }
    emit_event(
        "GPT_CIDATA_INJECTED",
        f"Relocated GPT backup header to LBA {new_backup_hdr_lba} and injected cidata partition #{free_slot_idx + 1}",
        result,
        callback=event_callback,
    )
    return result

def _copy_cidata_and_apps_to_mount(
    staged_cidata_dir: str,
    target_mount: str,
    event_callback: Optional[Callable[[Dict[str, Any]], None]] = None,
) -> List[str]:
    copied: List[str] = []
    emit_event(
        "USB_COPY_CIDATA",
        f"Writing cloud-init autoinstall config & pre-unpacked apps to {target_mount}...",
        {"mountpoint": target_mount, "percent": 88},
        callback=event_callback,
    )
    cidata_subdir = os.path.join(target_mount, "cidata")
    os.makedirs(cidata_subdir, exist_ok=True)

    for dest_base in (target_mount, cidata_subdir):
        for root, _dirs, files in os.walk(staged_cidata_dir):
            rel = os.path.relpath(root, staged_cidata_dir)
            dest_dir = dest_base if rel == "." else os.path.join(dest_base, rel)
            os.makedirs(dest_dir, exist_ok=True)
            for fname in files:
                src_file = os.path.join(root, fname)
                dst_file = os.path.join(dest_dir, fname)
                shutil.copy2(src_file, dst_file)
                copied.append(dst_file)
    return copied

def _close_explorer_and_dismount_windows_usb(
    disk_number: int,
    mountpoints: Optional[List[str]] = None,
) -> None:
    if not sys.platform.startswith("win") or disk_number <= 0:
        return

    letters: List[str] = []
    for mp in (mountpoints or []):
        clean_letter = mp.strip().rstrip("\\").rstrip(":")
        if len(clean_letter) == 1 and clean_letter.isalpha():
            letters.append(clean_letter.upper())

    if letters:
        drive_checks = " -or ".join(f"$_.LocationURL -like 'file:///{ltr}:*'" for ltr in letters)
        ps_close_exp = (
            "try { "
            "$shell = New-Object -ComObject Shell.Application; "
            f"$shell.Windows() | Where-Object {{ {drive_checks} }} | ForEach-Object {{ $_.Quit() }} "
            "} catch {}"
        )
        try:
            subprocess.run(
                ["powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", ps_close_exp],
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
                timeout=4,
                check=False,
                **_win_no_window_kwargs(),
            )
        except Exception:
            pass

        try:
            import ctypes
            kernel32 = ctypes.windll.kernel32
            GENERIC_READ = 0x80000000
            GENERIC_WRITE = 0x40000000
            FILE_SHARE_READ = 0x00000001
            FILE_SHARE_WRITE = 0x00000002
            OPEN_EXISTING = 3
            FSCTL_LOCK_VOLUME = 0x00090018
            FSCTL_DISMOUNT_VOLUME = 0x00090020
            INVALID_HANDLE_VALUE = ctypes.c_void_p(-1).value

            for ltr in letters:
                vol_path = f"\\\\.\\{ltr}:"
                h_vol = kernel32.CreateFileW(
                    vol_path,
                    GENERIC_READ | GENERIC_WRITE,
                    FILE_SHARE_READ | FILE_SHARE_WRITE,
                    None,
                    OPEN_EXISTING,
                    0,
                    None,
                )
                if h_vol and h_vol != INVALID_HANDLE_VALUE:
                    bytes_ret = ctypes.c_ulong(0)
                    kernel32.DeviceIoControl(
                        h_vol, FSCTL_LOCK_VOLUME, None, 0, None, 0, ctypes.byref(bytes_ret), None
                    )
                    kernel32.DeviceIoControl(
                        h_vol, FSCTL_DISMOUNT_VOLUME, None, 0, None, 0, ctypes.byref(bytes_ret), None
                    )
                    kernel32.CloseHandle(h_vol)
        except Exception:
            pass

def preflight_check_usb_target(target_device: Optional[str]) -> Optional[UsbDeviceCandidate]:
    if not target_device or target_device in ("staged_local", ""):
        return None
    if os.path.isdir(target_device):
        return None

    scanned = scan_host_usb_devices(allow_large_usb=True)
    matched: Optional[UsbDeviceCandidate] = None
    for dev in scanned:
        if target_device in (dev.device_id, dev.device_node, dev.raw_device_node) or (
            dev.mountpoints and target_device in dev.mountpoints
        ):
            if not dev.triple_lock_passed:
                raise PermissionError(
                    f"Triple-Lock safety guard blocked {dev.device_node}: {'; '.join(dev.block_reasons)}"
                )
            matched = dev
            break

    is_physical_usb = bool(
        (matched and matched.device_id.startswith("PhysicalDrive"))
        or str(target_device).startswith(("\\\\.\\PhysicalDrive", "PhysicalDrive", "/dev/sd", "/dev/disk", "/dev/rdisk"))
    )
    if is_physical_usb:
        host_probe: Any = importlib.import_module("host_probe")
        if not host_probe.check_is_admin():
            if sys.platform.startswith("win"):
                raise PermissionError(
                    "Administrator privileges are required to format and flash a USB drive. "
                    "Click 'Relaunch as Administrator' in the top banner (or right-click "
                    "InstantServer-Windows.exe and select 'Run as administrator')."
                )
            raise PermissionError(
                "Root privileges (sudo) are required to write directly to the USB block device."
            )
    return matched

def _format_usb_windows_diskpart(
    disk_number: int,
    usb_partition_scheme: str = "auto",
    disk_size_gb: float = 16.0,
    existing_mountpoints: Optional[List[str]] = None,
    event_callback: Optional[Callable[[Dict[str, Any]], None]] = None,
) -> Optional[str]:
    if disk_number <= 0:
        raise PermissionError("Refusing to format Disk 0 (primary system drive).")

    import host_probe
    if not host_probe.check_is_admin("windows"):
        raise PermissionError(
            "Administrator privileges are required to format USB drives on Windows. "
            "Please run Instant Server as Administrator."
        )

    scheme_label = "MBR (Legacy BIOS + CSM)" if usb_partition_scheme == "mbr_bios" else "GPT (UEFI / Universal)"
    emit_event(
        "USB_FORMAT_START",
        f"Unlocking & formatting USB Disk {disk_number} as bootable FAT32 ({scheme_label}, label=CIDATA)...",
        {"disk_number": disk_number, "scheme": usb_partition_scheme, "percent": 42},
        callback=event_callback,
    )

    _close_explorer_and_dismount_windows_usb(disk_number, existing_mountpoints)

    part_style = "MBR" if usb_partition_scheme == "mbr_bios" else "GPT"
    size_arg = "-Size 30GB" if disk_size_gb > 32.0 else "-UseMaximumSize"
    active_arg = "-IsActive" if usb_partition_scheme == "mbr_bios" else ""
    ps_storage_script = (
        f"$ErrorActionPreference = 'Stop'; "
        f"Set-Disk -Number {disk_number} -IsReadOnly $false -ErrorAction SilentlyContinue; "
        f"Get-Disk -Number {disk_number} | Clear-Disk -RemoveData -RemoveOEM -Confirm:$false -PassThru | "
        f"Initialize-Disk -PartitionStyle {part_style} -PassThru | "
        f"New-Partition -AssignDriveLetter {size_arg} {active_arg} | "
        f"Format-Volume -FileSystem FAT32 -NewFileSystemLabel 'CIDATA' -Force -Confirm:$false | Out-Null"
    )
    ps_res = subprocess.run(
        ["powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", ps_storage_script],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        check=False,
        **_win_no_window_kwargs(),
    )

    if ps_res.returncode != 0:
        _close_explorer_and_dismount_windows_usb(disk_number, existing_mountpoints)
        create_part_cmd = (
            "create partition primary size=30720" if disk_size_gb > 32.0 else "create partition primary"
        )
        if usb_partition_scheme == "mbr_bios":
            dp_lines = [
                "automount enable",
                f"select disk {disk_number}",
                "attributes disk clear readonly noerr",
                "online disk noerr",
                "clean",
                "convert mbr noerr",
                create_part_cmd,
                "select partition 1",
                "active",
                'format fs=fat32 label="CIDATA" quick override',
                "assign",
                "exit",
            ]
        else:
            dp_lines = [
                "automount enable",
                f"select disk {disk_number}",
                "attributes disk clear readonly noerr",
                "online disk noerr",
                "clean",
                "convert gpt noerr",
                create_part_cmd,
                "select partition 1",
                'format fs=fat32 label="CIDATA" quick override',
                "assign",
                "exit",
            ]

        with tempfile.NamedTemporaryFile("w", suffix=".txt", delete=False, encoding="utf-8") as tf:
            tf.write("\n".join(dp_lines) + "\n")
            dp_script_path = tf.name

        try:
            dp_res = subprocess.run(
                ["diskpart", "/s", dp_script_path],
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                text=True,
                check=False,
                **_win_no_window_kwargs(),
            )
            if dp_res.returncode != 0:
                dp_out = (dp_res.stdout or "") + "\n" + (dp_res.stderr or "")
                raise RuntimeError(
                    f"Could not format USB Disk {disk_number} (diskpart exit {dp_res.returncode}): "
                    f"{dp_out.strip()[-240:]}"
                )
        finally:
            try:
                os.remove(dp_script_path)
            except Exception:
                pass

    for _attempt in range(8):
        time.sleep(1.0)
        refreshed = _scan_windows_powershell(allow_large_usb=True)
        for dev in refreshed:
            if dev.device_id == f"PhysicalDrive{disk_number}" and dev.mountpoints:
                mount_candidate = dev.mountpoints[0]
                if os.path.isdir(mount_candidate):
                    return mount_candidate
    return None

def provision_usb_drive(
    staged_cidata_dir: str,
    target_device: Optional[str] = None,
    iso_path: Optional[str] = None,
    usb_partition_scheme: str = "auto",
    format_usb: bool = True,
    event_callback: Optional[Callable[[Dict[str, Any]], None]] = None,
) -> Dict[str, Any]:
    copied_files: List[str] = []
    target_mount: Optional[str] = None
    matched_dev: Optional[UsbDeviceCandidate] = None
    bootable_iso_written = False

    if target_device:
        scanned = scan_host_usb_devices(allow_large_usb=True)
        for dev in scanned:
            if target_device in (dev.device_id, dev.device_node, dev.raw_device_node) or (
                dev.mountpoints and target_device in dev.mountpoints
            ):
                if not dev.triple_lock_passed:
                    raise PermissionError(
                        f"Triple-Lock safety guard blocked {dev.device_node}: {'; '.join(dev.block_reasons)}"
                    )
                matched_dev = dev
                if dev.mountpoints:
                    target_mount = dev.mountpoints[0]
                break

        if os.path.isdir(target_device):
            target_mount = target_device

    if (
        sys.platform.startswith("win")
        and matched_dev
        and matched_dev.device_id.startswith("PhysicalDrive")
        and iso_path
        and os.path.isfile(iso_path)
    ):
        m = re.search(r"(\d+)$", matched_dev.device_id)
        if m and format_usb:
            disk_num = int(m.group(1))
            new_mount = _format_usb_windows_diskpart(
                disk_number=disk_num,
                usb_partition_scheme=usb_partition_scheme,
                disk_size_gb=matched_dev.size_gb,
                existing_mountpoints=matched_dev.mountpoints,
                event_callback=event_callback,
            )
            if new_mount:
                target_mount = new_mount

        if target_mount and os.path.isdir(target_mount):
            iso_manager.extract_full_iso_tree_to_usb(
                iso_path=iso_path,
                dest_root_dir=target_mount,
                event_callback=event_callback,
            )
            bootable_iso_written = True

    elif (
        iso_path
        and os.path.isfile(iso_path)
        and target_device
        and os.path.exists(target_device)
        and not os.path.isdir(target_device)
    ):
        iso_size = os.path.getsize(iso_path)
        emit_event(
            "USB_RAW_FLASH_START",
            f"Streaming bootable Ubuntu Server ISO ({round(iso_size / (1024 * 1024), 1)} MB) to {target_device}...",
            {"target_device": target_device, "iso_bytes": iso_size, "percent": 45},
            callback=event_callback,
        )
        written = 0
        chunk_size = 4 * 1024 * 1024
        with open(iso_path, "rb") as src_fh, open(target_device, "r+b") as dst_fh:
            while True:
                buf = src_fh.read(chunk_size)
                if not buf:
                    break
                buf = iso_manager.patch_iso_bytes_in_flight(buf)
                dst_fh.write(buf)
                written += len(buf)
                pct = min(85, int(45 + (written * 40) / max(1, iso_size)))
                emit_event(
                    "USB_RAW_FLASH_PROGRESS",
                    f"Flashing bootable ISO: {round(written / (1024 * 1024), 1)} MB / {round(iso_size / (1024 * 1024), 1)} MB",
                    {"percent": pct, "bytes_written": written},
                    callback=event_callback,
                )
            dst_fh.flush()
            os.fsync(dst_fh.fileno())
        try:
            relocate_gpt_and_append_cidata(target_device, event_callback=event_callback)
        except Exception:
            pass
        bootable_iso_written = True

    if (
        not bootable_iso_written
        and iso_path
        and os.path.isfile(iso_path)
        and target_mount
        and os.path.isdir(target_mount)
    ):
        iso_manager.extract_full_iso_tree_to_usb(
            iso_path=iso_path,
            dest_root_dir=target_mount,
            event_callback=event_callback,
        )
        bootable_iso_written = True

    if target_mount and os.path.isdir(target_mount):
        copied_files = _copy_cidata_and_apps_to_mount(
            staged_cidata_dir=staged_cidata_dir,
            target_mount=target_mount,
            event_callback=event_callback,
        )

    if target_device and target_device not in ("staged_local", ""):
        is_raw_block_target = bool(
            os.path.exists(target_device) and not os.path.isdir(target_device)
        )
        if not is_raw_block_target and (not target_mount or not os.path.isdir(target_mount)):
            raise RuntimeError(
                f"USB drive '{target_device}' was not mounted or accessible for writing. "
                "Please ensure the USB drive is plugged in, formatted, and not locked by another program."
            )
        if iso_path and os.path.isfile(iso_path) and not bootable_iso_written:
            raise RuntimeError(
                f"Ubuntu Server ISO was not written to USB drive '{target_device}'. "
                "Please re-insert the USB drive and try flashing again."
            )
        if target_mount and os.path.isdir(target_mount):
            if not copied_files:
                raise RuntimeError(
                    f"No cloud-init or app files were copied to '{target_mount}'. Flashing aborted."
                )
            required_usb_files = [
                os.path.join(target_mount, "user-data"),
                os.path.join(target_mount, "meta-data"),
                os.path.join(target_mount, "cidata", "user-data"),
                os.path.join(target_mount, "cidata", "meta-data"),
            ]
            if iso_path and os.path.isfile(iso_path):
                required_usb_files.append(os.path.join(target_mount, "boot", "grub", "grub.cfg"))
            staged_apps_dir = os.path.join(staged_cidata_dir, "apps")
            verified_apps: List[str] = []
            if os.path.isdir(staged_apps_dir):
                for entry_name in sorted(os.listdir(staged_apps_dir)):
                    entry_path = os.path.join(staged_apps_dir, entry_name)
                    if os.path.isdir(entry_path) and os.path.isfile(os.path.join(entry_path, "docker-compose.yml")):
                        required_usb_files.append(
                            os.path.join(target_mount, "cidata", "apps", entry_name, "docker-compose.yml")
                        )
                        verified_apps.append(entry_name)
                if verified_apps:
                    required_usb_files.append(
                        os.path.join(target_mount, "cidata", "apps", "unpack_and_install_apps.sh")
                    )
            missing_on_usb = [p for p in required_usb_files if not os.path.isfile(p) or os.path.getsize(p) == 0]
            if missing_on_usb:
                raise RuntimeError(
                    f"USB write verification failed — {len(missing_on_usb)} critical file(s) missing on USB "
                    f"(first missing: {os.path.basename(missing_on_usb[0])})."
                )
            sync_fn = getattr(os, "sync", None)
            if callable(sync_fn):
                try:
                    sync_fn()
                except Exception:
                    pass
            emit_event(
                "USB_VERIFY_COMPLETE",
                f"Verified {len(copied_files)} file(s) and {len(verified_apps)} pre-unpacked app bundle(s) on USB ({target_mount}).",
                {
                    "mountpoint": target_mount,
                    "copied_files_count": len(copied_files),
                    "verified_apps": verified_apps,
                    "percent": 96,
                },
                callback=event_callback,
            )

    emit_event(
        "USB_PROVISION_COMPLETE",
        "Bootable Instant Server USB (ISO + patched GRUB + cidata + pre-unpacked apps) is ready!",
        {
            "staged_cidata_dir": os.path.abspath(staged_cidata_dir),
            "target_device": target_device or "staged_local",
            "target_mount": target_mount,
            "bootable_iso_written": bootable_iso_written,
            "usb_partition_scheme": usb_partition_scheme,
            "copied_files_count": len(copied_files),
            "percent": 100,
        },
        callback=event_callback,
    )
    return {
        "staged_cidata_dir": os.path.abspath(staged_cidata_dir),
        "target_device": target_device,
        "target_mount": target_mount,
        "bootable_iso_written": bootable_iso_written,
        "usb_partition_scheme": usb_partition_scheme,
        "copied_files": copied_files,
    }

def _create_synthetic_hybrid_iso_usb_image(path: str, iso_mb: int = 8, usb_total_mb: int = 32) -> None:
    total_bytes = usb_total_mb * 1024 * 1024
    iso_sectors = (iso_mb * 1024 * 1024) // SECTOR_SIZE
    num_entries = 128
    entry_size = 128
    table_bytes = num_entries * entry_size
    table_sectors = table_bytes // SECTOR_SIZE

    entries = bytearray(table_bytes)
    p1_name = "ISO9660".encode("utf-16le").ljust(72, b"\x00")
    p1 = struct.pack(
        GPT_ENTRY_FMT,
        BASIC_DATA_GUID_BYTES,
        uuid.uuid4().bytes_le,
        2048,
        iso_sectors - 64,
        0,
        p1_name,
    )
    entries[0:128] = p1
    entries_crc = zlib.crc32(entries) & 0xFFFFFFFF

    old_alt_lba = iso_sectors - 1
    first_usable = 34
    old_last_usable = old_alt_lba - table_sectors - 1
    disk_guid = uuid.uuid4().bytes_le

    hdr_zero = struct.pack(
        GPT_HDR_FMT,
        GPT_HEADER_SIGNATURE,
        GPT_REVISION_1_0,
        GPT_HDR_SIZE,
        0,
        0,
        1,
        old_alt_lba,
        first_usable,
        old_last_usable,
        disk_guid,
        2,
        num_entries,
        entry_size,
        entries_crc,
    )
    hdr_crc = zlib.crc32(hdr_zero) & 0xFFFFFFFF
    hdr_final = (hdr_zero[:16] + struct.pack("<I", hdr_crc) + hdr_zero[20:]).ljust(SECTOR_SIZE, b"\x00")

    with open(path, "wb") as fh:
        fh.truncate(total_bytes)
        fh.seek(1 * SECTOR_SIZE)
        fh.write(hdr_final)
        fh.seek(2 * SECTOR_SIZE)
        fh.write(entries)

def _run_self_test() -> int:
    print("[UsbEngine Test] 1. Testing Triple-Lock USB Drive Guard...")
    mock_devs = [
        {
            "id": "nvme0n1",
            "node": "/dev/nvme0n1",
            "model": "Samsung 980 PRO",
            "size_bytes": 512 * (1024 ** 3),
            "bus": "nvme",
            "removable": False,
            "mountpoints": ["/", "/boot/efi"],
        },
        {
            "id": "sdb",
            "node": "/dev/sdb",
            "model": "SanDisk Ultra 32GB",
            "size_bytes": 32 * (1024 ** 3),
            "bus": "usb",
            "removable": True,
            "mountpoints": [],
        },
    ]
    scanned = scan_host_usb_devices(mock_raw_devices=mock_devs)
    assert scanned[0].triple_lock_passed is False and len(scanned[0].block_reasons) >= 2
    assert scanned[1].triple_lock_passed is True
    print("  -> PASS: System NVMe disk blocked by Triple-Lock; removable 32GB USB drive approved!")

    print("[UsbEngine Test] 2. Testing Pure-Python UEFI GPT backup header relocation & cidata injection...")
    with tempfile.TemporaryDirectory() as tmpdir:
        img_path = os.path.join(tmpdir, "simulated_usb.img")
        _create_synthetic_hybrid_iso_usb_image(img_path, iso_mb=8, usb_total_mb=32)
        res = relocate_gpt_and_append_cidata(img_path, cidata_size_mb=16)
        assert res["partition_number"] == 2
        assert res["backup_gpt_lba"] == (32 * 1024 * 1024 // SECTOR_SIZE) - 1

        with open(img_path, "rb") as fh:
            fh.seek(res["backup_gpt_lba"] * SECTOR_SIZE)
            backup_sec = fh.read(GPT_HDR_SIZE)
            unpacked = struct.unpack(GPT_HDR_FMT, backup_sec)
            assert unpacked[0] == GPT_HEADER_SIGNATURE
            assert unpacked[5] == res["backup_gpt_lba"]
            assert unpacked[6] == 1
    print(
        f"  -> PASS: Relocated GPT backup header to LBA {res['backup_gpt_lba']} and injected "
        f"partition #{res['partition_number']} ('cidata') with verified UEFI CRC32 ({res['primary_header_crc32']})!"
    )
    return 0

def main() -> int:
    parser = argparse.ArgumentParser(description="Instant Server — M3 Cross-Platform USB & GPT Engine")
    parser.add_argument("--self-test", action="store_true", help="Run built-in unit tests")
    parser.add_argument("--scan-usb", action="store_true", help="Scan host USB devices with Triple-Lock filter")
    parser.add_argument("--inject-cidata", type=str, default="", help="Path to raw USB device or disk image")
    parser.add_argument("--cidata-size-mb", type=int, default=128, help="Size of cidata partition in MB")
    args = parser.parse_args()

    if args.self_test:
        return _run_self_test()

    if args.scan_usb:
        devs = [asdict(d) for d in scan_host_usb_devices()]
        print(json.dumps(devs, indent=2))
        return 0

    if args.inject_cidata:
        info = relocate_gpt_and_append_cidata(args.inject_cidata, cidata_size_mb=args.cidata_size_mb)
        print(json.dumps(info, indent=2))
        return 0

    parser.print_help()
    return 0


def locate_native_c_flasher() -> Optional[Path]:
    """Locates the compiled win_usb_flasher binary if available on the host."""
    here = Path(__file__).resolve().parent
    candidates = [
        here / "win_usb_flasher.exe",
        here / "win_usb_flasher",
        here / "native" / "win_usb_flasher.exe",
        here / "native" / "win_usb_flasher",
        here.parent.parent / "win_usb_flasher.exe",
        here.parent.parent / "win_usb_flasher",
    ]
    for cand in candidates:
        if cand.is_file() and os.access(cand, os.X_OK):
            return cand
    return None


def get_macos_raw_disk_path(dev_path: str) -> str:
    """Converts /dev/diskN to /dev/rdiskN on macOS for unbuffered character device I/O."""
    if dev_path.startswith("/dev/disk"):
        return dev_path.replace("/dev/disk", "/dev/rdisk", 1)
    return dev_path


def run_native_c_flasher(
    iso_path: Path,
    target_device: str,
    patch_grub: bool = True,
    verify_crc: bool = True,
    progress_cb: Optional[Callable[[int, str], None]] = None,
) -> Dict[str, Any]:
    """Invokes the native C win_usb_flasher helper and streams JSON telemetry."""
    bin_path = locate_native_c_flasher()
    if not bin_path:
        return {"used_native_c": False, "error": "Native binary not found"}
    cmd = [str(bin_path), "--iso", str(iso_path), "--target", str(target_device)]
    if not patch_grub:
        cmd.append("--no-patch")
    if not verify_crc:
        cmd.append("--no-verify")
    proc = subprocess.Popen(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
    last_telemetry: Dict[str, Any] = {"used_native_c": True, "binary": str(bin_path)}
    assert proc.stdout is not None
    for line in proc.stdout:
        line = line.strip()
        if not line.startswith("{"):
            continue
        try:
            data = json.loads(line)
            last_telemetry.update(data)
            if progress_cb and "pct" in data:
                mb_s = data.get("mb_s", 0.0)
                eta = data.get("eta_sec", 0.0)
                progress_cb(int(data["pct"]), f"Flashing via native DMA engine ({mb_s:.1f} MB/s, ETA {eta:.0f}s)")
        except Exception:
            pass
    rc = proc.wait()
    if rc != 0:
        err_out = proc.stderr.read() if proc.stderr else ""
        raise RuntimeError(f"Native C flasher failed (exit {rc}): {err_out.strip()}")
    return last_telemetry


if __name__ == "__main__":
    sys.exit(main())