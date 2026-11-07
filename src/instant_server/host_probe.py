# src/instant_server/host_probe.py
from __future__ import annotations

import argparse
from dataclasses import asdict, dataclass, field
import json
import os
import platform
import shutil
import sys
import tempfile
from typing import Any, Callable, Dict, List, Optional

@dataclass
class HostProbeReport:

    project_name: str = "Instant Server"
    host_os: str = "unknown"
    platform_system: str = ""
    platform_release: str = ""
    architecture: str = ""
    python_version: str = ""
    is_admin_or_root: bool = False
    elevation_hint: str = ""
    cache_dir: str = ""
    cache_free_bytes: int = 0
    cache_free_gb: float = 0.0
    has_sufficient_disk_space: bool = False
    native_tools_found: Dict[str, Optional[str]] = field(default_factory=dict)
    pure_python_fallbacks_active: List[str] = field(default_factory=list)
    ready_for_iso_build: bool = False
    ready_for_raw_usb_flash: bool = False
    warnings: List[str] = field(default_factory=list)

def emit_event(
    event_type: str,
    message: str,
    data: Optional[Dict[str, Any]] = None,
    callback: Optional[Callable[[Dict[str, Any]], None]] = None,
) -> Dict[str, Any]:
    payload = {
        "event": event_type,
        "module": "HOST_PROBE",
        "project": "Instant Server",
        "message": message,
        "data": data or {},
    }
    if callback:
        callback(payload)
    return payload

def detect_host_os(sys_platform: Optional[str] = None) -> str:
    plat = (sys_platform or sys.platform).lower()
    if plat.startswith("win") or plat in ("cygwin", "msys"):
        return "windows"
    if plat == "darwin":
        return "macos"
    return "linux"

def check_is_admin(host_os: Optional[str] = None) -> bool:
    os_name = host_os or detect_host_os()
    if os_name == "windows":
        try:
            import ctypes
            return bool(ctypes.windll.shell32.IsUserAnAdmin())
        except Exception:
            return False
    else:
        geteuid_fn = getattr(os, "geteuid", None)
        if callable(geteuid_fn):
            try:
                return bool(geteuid_fn() == 0)
            except Exception:
                return False
        return False

def request_windows_uac_elevation(extra_args: Optional[List[str]] = None) -> bool:
    if detect_host_os() != "windows":
        return False
    if check_is_admin("windows"):
        return True
    try:
        import ctypes
        import subprocess
        exe = sys.executable
        args_list = list(extra_args if extra_args is not None else sys.argv)
        if not getattr(sys, "frozen", False):
            params = subprocess.list2cmdline(args_list)
        else:
            params = subprocess.list2cmdline(args_list[1:])
        rc = ctypes.windll.shell32.ShellExecuteW(None, "runas", exe, params, None, 1)
        return int(rc) > 32
    except Exception:
        return False

def verify_preflight_requirements(
    cache_dir: Optional[str] = None,
    requires_raw_usb_access: bool = False,
    min_free_gb: float = 3.5,
    mock_os: Optional[str] = None,
    mock_is_admin: Optional[bool] = None,
) -> HostProbeReport:
    rep = probe_host_environment(cache_dir=cache_dir, min_free_gb=min_free_gb, mock_os=mock_os)
    effective_admin = rep.is_admin_or_root if mock_is_admin is None else mock_is_admin
    if requires_raw_usb_access and not effective_admin:
        if rep.host_os == "windows":
            raise PermissionError(
                "Administrator privileges are required to format and write to the USB drive. "
                "Click 'Relaunch as Administrator' at the top of the window (or right-click "
                "InstantServer-Windows.exe and select 'Run as administrator') before flashing."
            )
        raise PermissionError(
            f"Root privileges are required to write to the USB device ({rep.elevation_hint})."
        )
    if not rep.has_sufficient_disk_space:
        raise OSError(
            f"Insufficient free disk space in '{rep.cache_dir}': {rep.cache_free_gb} GB available, "
            f"at least {min_free_gb} GB required for ISO and app caching."
        )
    return rep

def probe_host_environment(
    cache_dir: Optional[str] = None,
    min_free_gb: float = 3.5,
    mock_os: Optional[str] = None,
    event_callback: Optional[Callable[[Dict[str, Any]], None]] = None,
) -> HostProbeReport:
    emit_event("HOST_CHECK_STARTED", "Probing host operating system and disk tooling...", callback=event_callback)

    host_os = detect_host_os(mock_os)
    is_admin = check_is_admin(host_os)

    if not cache_dir:
        cache_dir = os.path.join(tempfile.gettempdir(), "instant-server-cache")
    os.makedirs(cache_dir, exist_ok=True)

    usage = shutil.disk_usage(cache_dir)
    free_gb = round(usage.free / (1024 ** 3), 2)
    sufficient_space = free_gb >= min_free_gb

    if host_os == "windows":
        expected_tools = ["powershell", "diskpart", "tar"]
        elevation_hint = "Right-click PowerShell or Instant Server and select 'Run as Administrator' before flashing USB."
    elif host_os == "macos":
        expected_tools = ["diskutil", "hdiutil", "newfs_msdos", "sgdisk"]
        elevation_hint = "Run with 'sudo python3 ...' when writing directly to /dev/rdiskN."
    else:
        expected_tools = ["lsblk", "udevadm", "sgdisk", "sfdisk", "parted", "mkfs.vfat", "blkid"]
        elevation_hint = "Run with 'sudo python3 ...' when flashing raw USB block devices."

    found_tools: Dict[str, Optional[str]] = {}
    for tool in expected_tools:
        found_tools[tool] = shutil.which(tool)

    fallbacks = [
        "pure-python-sha512-crypt (bypasses removed crypt stdlib in Python 3.13+)",
        "pure-python-iso9660-extractor (extracts boot tree & .deb packages without root mount)",
        "pure-python-gpt-relocator (relocates backup GPT header & injects cidata without sgdisk)",
    ]

    warnings: List[str] = []
    if not sufficient_space:
        warnings.append(
            f"Low disk space in '{cache_dir}': {free_gb} GB free (recommended >= {min_free_gb} GB for Ubuntu LTS ISO)."
        )
    if not is_admin:
        warnings.append(
            f"Process is not running with elevated privileges. Profile compilation & ISO download work normally, "
            f"but raw USB flashing will require elevation ({elevation_hint})."
        )

    report = HostProbeReport(
        project_name="Instant Server",
        host_os=host_os,
        platform_system=platform.system(),
        platform_release=platform.release(),
        architecture=platform.machine(),
        python_version=platform.python_version(),
        is_admin_or_root=is_admin,
        elevation_hint=elevation_hint,
        cache_dir=cache_dir,
        cache_free_bytes=usage.free,
        cache_free_gb=free_gb,
        has_sufficient_disk_space=sufficient_space,
        native_tools_found=found_tools,
        pure_python_fallbacks_active=fallbacks,
        ready_for_iso_build=sufficient_space,
        ready_for_raw_usb_flash=is_admin,
        warnings=warnings,
    )

    emit_event(
        "HOST_READY",
        f"Host probe complete: os={host_os}, arch={report.architecture}, free_gb={free_gb}",
        asdict(report),
        callback=event_callback,
    )
    return report

def _run_self_test() -> int:
    print("[HostProbe Test] 1. Testing OS normalization across Windows, macOS, and Linux...")
    assert detect_host_os("win32") == "windows"
    assert detect_host_os("darwin") == "macos"
    assert detect_host_os("linux") == "linux"
    print("  -> PASS: Host OS detection verified for Windows, macOS, and Linux!")

    print("[HostProbe Test] 2. Probing current environment and pure-Python fallbacks...")
    rep = probe_host_environment(min_free_gb=0.1)
    assert rep.project_name == "Instant Server"
    assert len(rep.pure_python_fallbacks_active) >= 3
    assert rep.ready_for_iso_build is True
    print(
        f"  -> PASS: Detected {rep.host_os} ({rep.architecture}), Python {rep.python_version}, "
        f"free space={rep.cache_free_gb} GB!"
    )
    return 0

def main() -> int:
    parser = argparse.ArgumentParser(description="Instant Server — Host Environment Inspector")
    parser.add_argument("--self-test", action="store_true", help="Run built-in unit tests")
    parser.add_argument("--cache-dir", type=str, default="", help="Custom ISO cache directory to inspect")
    args = parser.parse_args()

    if args.self_test:
        return _run_self_test()

    report = probe_host_environment(cache_dir=args.cache_dir or None)
    print(json.dumps(asdict(report), indent=2))
    return 0

if __name__ == "__main__":
    sys.exit(main())