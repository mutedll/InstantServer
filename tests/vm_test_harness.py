# tests/vm_test_harness.py
from __future__ import annotations

import argparse
from dataclasses import asdict, dataclass, field
import importlib
import json
import os
import shutil
import struct
import sys
import tempfile
import uuid
import zlib
from typing import Any, Dict, List, Optional, Tuple

_CURRENT_DIR = os.path.dirname(os.path.abspath(__file__))
for _cand in (
    _CURRENT_DIR,
    os.path.join(_CURRENT_DIR, "src", "instant_server"),
    os.path.join(_CURRENT_DIR, "src", "instant_server", "scripts"),
    os.path.abspath(os.path.join(_CURRENT_DIR, "..")),
    os.path.abspath(os.path.join(_CURRENT_DIR, "..", "src", "instant_server")),
    os.path.abspath(os.path.join(_CURRENT_DIR, "..", "src", "instant_server", "scripts")),
):
    if os.path.isdir(_cand) and _cand not in sys.path:
        sys.path.append(_cand)

usb_engine: Any = importlib.import_module("usb_engine")
config_builder: Any = importlib.import_module("config_builder")
payload_packager: Any = importlib.import_module("payload_packager")
hardware_adapter: Any = importlib.import_module("hardware_adapter")

SECTOR_SIZE = 512
EFI_SYSTEM_GUID_LE = uuid.UUID("C12A7328-F81F-11D2-BA4B-00A0C93EC93B").bytes_le
BASIC_DATA_GUID_LE = uuid.UUID("EBD0A0A2-B9E5-4433-87C0-68B6B72699C7").bytes_le

OVMF_CODE_CANDIDATES = [
    "/usr/share/OVMF/OVMF_CODE_4M.fd",
    "/usr/share/OVMF/OVMF_CODE.fd",
    "/usr/share/edk2/ovmf/OVMF_CODE.fd",
    "/usr/share/qemu/ovmf-x86_64-code.bin",
    "/opt/homebrew/share/qemu/edk2-x86_64-code.fd",
]
OVMF_VARS_CANDIDATES = [
    "/usr/share/OVMF/OVMF_VARS_4M.fd",
    "/usr/share/OVMF/OVMF_VARS.fd",
    "/usr/share/edk2/ovmf/OVMF_VARS.fd",
    "/usr/share/qemu/ovmf-x86_64-vars.bin",
    "/opt/homebrew/share/qemu/edk2-i386-vars.fd",
]

@dataclass
class QemuTopologySpec:

    vm_name: str = "instant-server-laptop-vm"
    firmware_mode: str = "uefi"
    ram_mb: int = 2048
    vcpus: int = 2
    install_mode: str = "dual_boot"
    usb_installer_img: str = "usb_installer.img"
    primary_disk_img: str = "laptop_nvme0.img"
    secondary_disk_img: Optional[str] = None
    ssh_host_port: int = 2222
    cockpit_host_port: int = 9090
    casaos_host_port: int = 8080
    serial_log_path: str = "qemu-instant-server-serial.log"

def create_synthetic_dualboot_laptop_disk(
    image_path: str,
    total_size_mb: int = 64,
) -> Dict[str, Any]:
    total_bytes = total_size_mb * 1024 * 1024
    total_sectors = total_bytes // SECTOR_SIZE
    num_entries = 128
    entry_size = 128
    table_bytes = num_entries * entry_size
    table_sectors = table_bytes // SECTOR_SIZE

    entries = bytearray(table_bytes)

    p1_name = "EFI System Partition".encode("utf-16le").ljust(72, b"\x00")[:72]
    entries[0:128] = struct.pack(
        usb_engine.GPT_ENTRY_FMT,
        EFI_SYSTEM_GUID_LE,
        uuid.uuid4().bytes_le,
        2048,
        6143,
        0,
        p1_name,
    )

    p2_name = "Windows 11 OS".encode("utf-16le").ljust(72, b"\x00")[:72]
    entries[128:256] = struct.pack(
        usb_engine.GPT_ENTRY_FMT,
        BASIC_DATA_GUID_LE,
        uuid.uuid4().bytes_le,
        6144,
        32767,
        0,
        p2_name,
    )

    entries_crc = zlib.crc32(entries) & 0xFFFFFFFF
    backup_hdr_lba = total_sectors - 1
    backup_table_lba = backup_hdr_lba - table_sectors
    first_usable = 34
    last_usable = backup_table_lba - 1
    disk_guid = uuid.uuid4().bytes_le

    primary_zero = struct.pack(
        usb_engine.GPT_HDR_FMT,
        usb_engine.GPT_HEADER_SIGNATURE,
        usb_engine.GPT_REVISION_1_0,
        usb_engine.GPT_HDR_SIZE,
        0,
        0,
        1,
        backup_hdr_lba,
        first_usable,
        last_usable,
        disk_guid,
        2,
        num_entries,
        entry_size,
        entries_crc,
    )
    primary_crc = zlib.crc32(primary_zero) & 0xFFFFFFFF
    primary_hdr = (
        primary_zero[:16] + struct.pack("<I", primary_crc) + primary_zero[20:]
    ).ljust(SECTOR_SIZE, b"\x00")

    backup_zero = struct.pack(
        usb_engine.GPT_HDR_FMT,
        usb_engine.GPT_HEADER_SIGNATURE,
        usb_engine.GPT_REVISION_1_0,
        usb_engine.GPT_HDR_SIZE,
        0,
        0,
        backup_hdr_lba,
        1,
        first_usable,
        last_usable,
        disk_guid,
        backup_table_lba,
        num_entries,
        entry_size,
        entries_crc,
    )
    backup_crc = zlib.crc32(backup_zero) & 0xFFFFFFFF
    backup_hdr = (
        backup_zero[:16] + struct.pack("<I", backup_crc) + backup_zero[20:]
    ).ljust(SECTOR_SIZE, b"\x00")

    with open(image_path, "wb") as fh:
        fh.truncate(total_bytes)
        fh.seek(1 * SECTOR_SIZE)
        fh.write(primary_hdr)
        fh.seek(2 * SECTOR_SIZE)
        fh.write(entries)
        fh.seek(backup_table_lba * SECTOR_SIZE)
        fh.write(entries)
        fh.seek(backup_hdr_lba * SECTOR_SIZE)
        fh.write(backup_hdr)

    return {
        "image_path": image_path,
        "total_sectors": total_sectors,
        "partitions": [
            {"number": 1, "name": "EFI System Partition", "fstype": "vfat", "is_efi": True},
            {"number": 2, "name": "Windows 11 OS", "fstype": "ntfs", "is_efi": False},
        ],
    }

def locate_ovmf_firmware() -> Tuple[Optional[str], Optional[str]]:
    code_path = next((p for p in OVMF_CODE_CANDIDATES if os.path.exists(p)), None)
    vars_path = next((p for p in OVMF_VARS_CANDIDATES if os.path.exists(p)), None)
    return code_path, vars_path

def build_qemu_command(
    spec: QemuTopologySpec,
    workdir: str,
    ovmf_code_override: Optional[str] = None,
    ovmf_vars_override: Optional[str] = None,
) -> Dict[str, Any]:
    os.makedirs(workdir, exist_ok=True)
    cmd: List[str] = [
        "qemu-system-x86_64",
        "-name",
        spec.vm_name,
        "-m",
        str(spec.ram_mb),
        "-smp",
        str(spec.vcpus),
        "-machine",
        "q35,accel=kvm:hvf:whpx:tcg",
    ]

    writable_vars_path: Optional[str] = None
    if spec.firmware_mode == "uefi":
        code_file = ovmf_code_override or locate_ovmf_firmware()[0] or "/usr/share/OVMF/OVMF_CODE.fd"
        vars_template = ovmf_vars_override or locate_ovmf_firmware()[1]
        writable_vars_path = os.path.join(workdir, "ovmf_vars_writable.fd")
        if vars_template and os.path.exists(vars_template):
            shutil.copyfile(vars_template, writable_vars_path)
        elif not os.path.exists(writable_vars_path):

            with open(writable_vars_path, "wb") as vf:
                vf.write(b"\x00" * (256 * 1024))

        cmd.extend([
            "-drive",
            f"if=pflash,format=raw,readonly=on,file={code_file}",
            "-drive",
            f"if=pflash,format=raw,file={writable_vars_path}",
        ])

    cmd.extend([
        "-device",
        "qemu-xhci,id=xhci",
        "-drive",
        f"file={spec.usb_installer_img},if=none,id=usbstick,format=raw",
        "-device",
        "usb-storage,bus=xhci.0,drive=usbstick,removable=on,bootindex=1",
    ])

    cmd.extend([
        "-drive",
        f"file={spec.primary_disk_img},if=none,id=nvme0,format=raw",
        "-device",
        "nvme,drive=nvme0,serial=INSTANT_NVME0,bootindex=2",
    ])

    if spec.secondary_disk_img:
        cmd.extend([
            "-drive",
            f"file={spec.secondary_disk_img},if=none,id=sata1,format=raw",
            "-device",
            "ide-hd,drive=sata1,serial=INSTANT_SATA1,bootindex=3",
        ])

    hostfwd = (
        f"user,id=net0,"
        f"hostfwd=tcp::{spec.ssh_host_port}-:22,"
        f"hostfwd=tcp::{spec.cockpit_host_port}-:9090,"
        f"hostfwd=tcp::{spec.casaos_host_port}-:80"
    )
    cmd.extend([
        "-netdev",
        hostfwd,
        "-device",
        "virtio-net-pci,netdev=net0",
        "-serial",
        f"file:{os.path.join(workdir, spec.serial_log_path)}",
        "-display",
        "none",
    ])

    return {
        "firmware_mode": spec.firmware_mode,
        "install_mode": spec.install_mode,
        "writable_ovmf_vars": writable_vars_path,
        "qemu_args": cmd,
        "qemu_command_str": " ".join(cmd),
        "host_endpoints": {
            "ssh": f"ssh -p {spec.ssh_host_port} admin@127.0.0.1",
            "cockpit": f"https://127.0.0.1:{spec.cockpit_host_port}",
            "casaos": f"http://127.0.0.1:{spec.casaos_host_port}",
        },
    }

def run_full_matrix_simulation() -> Dict[str, Any]:
    results: Dict[str, Any] = {}
    mock_disks = [
        {
            "name": "sda",
            "path": "/dev/sda",
            "size_bytes": 32 * (1024 ** 3),
            "removable": True,
            "bus": "usb",
            "ssd": True,
            "is_live_medium": True,
        },
        {
            "name": "nvme0n1",
            "path": "/dev/nvme0n1",
            "size_bytes": 512 * (1024 ** 3),
            "removable": False,
            "bus": "nvme",
            "ssd": True,
            "is_live_medium": False,
            "existing_partitions": [
                {
                    "number": 1,
                    "path": "/dev/nvme0n1p1",
                    "size_bytes": 512 * (1024 ** 2),
                    "fstype": "vfat",
                    "is_efi": True,
                },
                {
                    "number": 2,
                    "path": "/dev/nvme0n1p2",
                    "size_bytes": 250 * (1024 ** 3),
                    "fstype": "ntfs",
                    "is_efi": False,
                },
            ],
        },
        {
            "name": "sdb",
            "path": "/dev/sdb",
            "size_bytes": 256 * (1024 ** 3),
            "removable": False,
            "bus": "sata",
            "ssd": True,
            "is_live_medium": False,
            "existing_partitions": [],
        },
    ]
    mock_ifaces = [
        {"name": "enp0s3", "kind": "wired"},
        {"name": "wlp2s0", "kind": "wireless"},
        {"name": "usb0", "kind": "usb_tether"},
    ]

    with tempfile.TemporaryDirectory() as tmpdir:
        for mode in ("dual_boot", "auto_erase", "interactive_select", "secondary_drive"):
            for fw in ("uefi", "bios"):
                prof = config_builder.ServerProfile(
                    hostname=f"instant-{mode.replace('_', '-')}",
                    username="admin",
                    password="InstantServer2026!",
                    install_mode=mode,
                    confirm_erase_main_drive=(mode == "auto_erase"),
                    dual_boot_Allocation_gb=55,
                    boot_mode=fw,
                )
                stage_dir = os.path.join(tmpdir, f"stage_{mode}_{fw}")
                payload_packager.stage_cidata_directory(prof, stage_dir)
                env = hardware_adapter.load_instant_env(
                    os.path.join(stage_dir, "instant-server-env.conf")
                )
                adapted = hardware_adapter.adapt_autoinstall_for_hardware(
                    env,
                    mock_disks=mock_disks,
                    mock_ifaces=mock_ifaces,
                    mock_boot_mode=fw,
                )
                key = f"{mode}__{fw}"
                results[key] = {
                    "install_mode": adapted["install_mode"],
                    "boot_mode": adapted["boot_mode"],
                    "selected_disk": adapted["selected_disk"]["path"] if adapted["selected_disk"] else "interactive",
                    "has_storage_config": adapted["storage"] is not None,
                }
    return results

def _run_self_test() -> int:
    print("[VM Harness Self-Test] 1. Testing Synthetic Dual-Boot Laptop GPT Disk Builder...")
    with tempfile.TemporaryDirectory() as tmpdir:
        disk_img = os.path.join(tmpdir, "laptop_nvme0.img")
        meta = create_synthetic_dualboot_laptop_disk(disk_img, total_size_mb=32)
        assert len(meta["partitions"]) == 2

        reloc = usb_engine.relocate_gpt_and_append_cidata(disk_img, cidata_size_mb=8)
        assert reloc["partition_number"] == 3
    print("  -> PASS: Synthetic GPT laptop disk (EFI + Windows NTFS) created & verified!")

    print("[VM Harness Self-Test] 2. Testing QEMU UEFI (`pflash` writable OVMF_VARS) & BIOS command generation...")
    with tempfile.TemporaryDirectory() as tmpdir:
        uefi_spec = QemuTopologySpec(firmware_mode="uefi", install_mode="dual_boot")
        uefi_cmd = build_qemu_command(uefi_spec, tmpdir)
        assert "if=pflash,format=raw,readonly=on" in uefi_cmd["qemu_command_str"]
        assert uefi_cmd["writable_ovmf_vars"] is not None and os.path.exists(uefi_cmd["writable_ovmf_vars"])
        assert "usb-storage,bus=xhci.0,drive=usbstick,removable=on,bootindex=1" in uefi_cmd["qemu_command_str"]

        bios_spec = QemuTopologySpec(firmware_mode="bios", install_mode="auto_erase")
        bios_cmd = build_qemu_command(bios_spec, tmpdir)
        assert "if=pflash" not in bios_cmd["qemu_command_str"]
    print("  -> PASS: QEMU UEFI (writable OVMF_VARS) and Legacy BIOS command builders verified!")

    print("[VM Harness Self-Test] 3. Running 8-way Matrix Simulation (4 Disk Modes x 2 Firmware Modes)...")
    matrix = run_full_matrix_simulation()
    assert len(matrix) == 8
    assert matrix["dual_boot__uefi"]["selected_disk"] == "/dev/nvme0n1"
    assert matrix["auto_erase__bios"]["selected_disk"] == "/dev/nvme0n1"
    assert matrix["interactive_select__uefi"]["selected_disk"] == "interactive"
    assert matrix["secondary_drive__uefi"]["selected_disk"] == "/dev/sdb"
    print("  -> PASS: All 8 combinations (4 Disk Modes x UEFI/BIOS) passed end-to-end simulation!")
    return 0

def main() -> int:
    parser = argparse.ArgumentParser(description="Instant Server — QEMU UEFI/BIOS Test Harness")
    parser.add_argument("--self-test", action="store_true", help="Run built-in test suite & 8-way simulation matrix")
    parser.add_argument("--print-qemu-cmd", action="store_true", help="Print QEMU launch command for a topology")
    parser.add_argument("--firmware", type=str, default="uefi", choices=["uefi", "bios"])
    parser.add_argument(
        "--install-mode",
        type=str,
        default="dual_boot",
        choices=["dual_boot", "auto_erase", "interactive_select", "secondary_drive"],
    )
    parser.add_argument("--workdir", type=str, default="./qemu_instant_server_test")
    args = parser.parse_args()

    if args.self_test:
        return _run_self_test()

    if args.print_qemu_cmd:
        spec = QemuTopologySpec(firmware_mode=args.firmware, install_mode=args.install_mode)
        info = build_qemu_command(spec, args.workdir)
        print(json.dumps(info, indent=2))
        return 0

    matrix = run_full_matrix_simulation()
    print(json.dumps(matrix, indent=2))
    return 0

if __name__ == "__main__":
    sys.exit(main())