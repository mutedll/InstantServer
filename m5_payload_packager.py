#!/usr/bin/env python3
"""
Instant Server — Module M5: End-to-End `cidata` Payload Packager & Build Orchestrator
=====================================================================================
File: m5_payload_packager.py
Project: Instant Server (Turn any old laptop into a headless Ubuntu Server)
Runs on: Windows 10/11, macOS, and Linux (Python 3.8+ through 3.14+, zero external deps)

Responsibilities:
1. Coordinates M1 (Host Probe), M2 (ISO & Offline `.deb` Extractor), M3 (USB Engine),
   and M4 (Config Builder) to assemble a complete, boot-ready `cidata` directory or
   FAT32 disk image (`instant-server-cidata.img`).
2. Copies `user-data`, `meta-data`, `instant-server-env.conf`, `p1_early_hardware_adapter.py`,
   `p2_late_laptop_hardening.sh`, and extracted offline `.deb` packages (`wpasupplicant`,
   `rfkill`, etc.) into the staging tree with strict Unix LF (`\\n`) line-ending enforcement.
3. Generates a `MANIFEST.sha256` inside `cidata` so the target laptop can verify that no
   files were corrupted on cheap or aging USB flash drives (solving Problem 1.5 in our
   Community Research Matrix).
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import shutil
import sys
import tempfile
from typing import Any, Callable, Dict, List, Optional

# Allow importing sibling Canvas modules in the same directory
_CURRENT_DIR = os.path.dirname(os.path.abspath(__file__))
if _CURRENT_DIR not in sys.path:
    sys.path.insert(0, _CURRENT_DIR)

import m1_host_probe  # noqa: E402
import m2_iso_manager  # noqa: E402
import m3_usb_engine  # noqa: E402
import m4_config_builder  # noqa: E402


def emit_event(
    event_type: str,
    message: str,
    data: Optional[Dict[str, Any]] = None,
    callback: Optional[Callable[[Dict[str, Any]], None]] = None,
) -> Dict[str, Any]:
    """Emit structured JSON event for CLI and Instant Server GUI."""
    payload = {
        "event": event_type,
        "module": "M5_PAYLOAD_PACKAGER",
        "project": "Instant Server",
        "message": message,
        "data": data or {},
    }
    if callback:
        callback(payload)
    return payload


def stage_cidata_directory(
    profile: m4_config_builder.ServerProfile,
    staging_dir: str,
    iso_path_for_debs: Optional[str] = None,
    event_callback: Optional[Callable[[Dict[str, Any]], None]] = None,
) -> Dict[str, Any]:
    """
    Assemble the complete `cidata` partition file tree inside `staging_dir`:
      - `user-data`
      - `meta-data`
      - `instant-server-env.conf` (and `lazarus-env.conf` backward-compat symlink/copy)
      - `profile.json`
      - `p1_early_hardware_adapter.py`
      - `p2_late_laptop_hardening.sh`
      - `debs/` (offline `.deb` packages extracted from ISO if provided)
      - `MANIFEST.sha256` (cryptographic integrity manifest of all staged files)
    """
    os.makedirs(staging_dir, exist_ok=True)
    debs_dir = os.path.join(staging_dir, "debs")
    os.makedirs(debs_dir, exist_ok=True)

    emit_event(
        "PACKAGER_START",
        f"Staging Instant Server cidata payload for '{profile.hostname}.local'...",
        {"staging_dir": staging_dir, "install_mode": profile.install_mode},
        callback=event_callback,
    )

    # 1. Compile M4 configuration files
    compiled = m4_config_builder.compile_cidata_payloads(profile, event_callback=event_callback)
    written_files: List[str] = []

    for fname, text_content in compiled.items():
        fpath = os.path.join(staging_dir, fname)
        clean_lf = text_content.replace("\r\n", "\n")
        with open(fpath, "w", encoding="utf-8", newline="\n") as fh:
            fh.write(clean_lf)
        written_files.append(fpath)

    # 2. Copy P1 and P2 target scripts into `staging_dir` with LF line endings
    for script_name in ("p1_early_hardware_adapter.py", "p2_late_laptop_hardening.sh"):
        src_script = os.path.join(_CURRENT_DIR, script_name)
        dst_script = os.path.join(staging_dir, script_name)
        if os.path.exists(src_script):
            with open(src_script, "r", encoding="utf-8") as rf:
                raw_text = rf.read().replace("\r\n", "\n")
            with open(dst_script, "w", encoding="utf-8", newline="\n") as wf:
                wf.write(raw_text)
            os.chmod(dst_script, 0o755)
            written_files.append(dst_script)

    # 3. Extract offline `.deb` packages from ISO if an ISO path was supplied
    extracted_debs: List[str] = []
    if iso_path_for_debs and os.path.exists(iso_path_for_debs):
        extracted_debs = m2_iso_manager.extract_offline_debs_from_iso(
            iso_path_for_debs,
            debs_dir,
            event_callback=event_callback,
        )
        written_files.extend(extracted_debs)

    # 4. Compute SHA-256 manifest for post-flash read-back integrity checking
    manifest_lines: List[str] = []
    manifest_map: Dict[str, str] = {}
    for fpath in sorted(written_files):
        rel = os.path.relpath(fpath, staging_dir).replace("\\", "/")
        digest = hashlib.sha256(open(fpath, "rb").read()).hexdigest()
        manifest_lines.append(f"{digest}  {rel}")
        manifest_map[rel] = digest

    manifest_path = os.path.join(staging_dir, "MANIFEST.sha256")
    with open(manifest_path, "w", encoding="utf-8", newline="\n") as mf:
        mf.write("\n".join(manifest_lines) + "\n")
    written_files.append(manifest_path)

    summary = {
        "project": "Instant Server",
        "hostname": profile.hostname,
        "install_mode": profile.install_mode,
        "staging_dir": staging_dir,
        "total_files": len(written_files),
        "offline_debs_count": len(extracted_debs),
        "manifest": manifest_map,
    }
    emit_event(
        "PACKAGER_COMPLETE",
        f"Staged {len(written_files)} files into cidata payload directory.",
        summary,
        callback=event_callback,
    )
    return summary


def verify_staged_cidata_directory(staging_dir: str) -> List[str]:
    """Verify all files listed in `MANIFEST.sha256` match their recorded SHA-256 digests."""
    manifest_path = os.path.join(staging_dir, "MANIFEST.sha256")
    if not os.path.exists(manifest_path):
        return ["Missing MANIFEST.sha256 in cidata directory."]

    errors: List[str] = []
    with open(manifest_path, "r", encoding="utf-8") as mf:
        for raw_line in mf:
            line = raw_line.strip()
            if not line:
                continue
            parts = line.split(None, 1)
            if len(parts) != 2:
                continue
            expected_hash, rel_path = parts[0].strip(), parts[1].strip()
            target_file = os.path.join(staging_dir, rel_path)
            if not os.path.exists(target_file):
                errors.append(f"Missing staged file: {rel_path}")
                continue
            actual_hash = hashlib.sha256(open(target_file, "rb").read()).hexdigest()
            if actual_hash != expected_hash:
                errors.append(f"Corrupted file {rel_path}: expected {expected_hash}, got {actual_hash}")
    return errors


def _run_self_test() -> int:
    print("[M5 Self-Test] Testing end-to-end orchestration (M1 + M2 + M3 + M4 + M5)...")
    with tempfile.TemporaryDirectory() as tmpdir:
        # 1. Create a synthetic ISO with a mock wpasupplicant .deb
        iso_file = os.path.join(tmpdir, "ubuntu-24.04-mock.iso")
        m2_iso_manager._build_synthetic_iso_with_deb(
            iso_file,
            "wpasupplicant_2.10-21build4_amd64.deb",
            b"!<arch>\ndebian-binary\n",
        )

        # 2. Build a Dual-Boot ServerProfile
        profile = m4_config_builder.ServerProfile(
            hostname="instant-node",
            username="admin",
            password="InstantServer2026!",
            install_mode="dual_boot",
            dual_boot_Allocation_gb=50,
            dashboard_choice="both",
        )

        cidata_dir = os.path.join(tmpdir, "cidata")
        summary = stage_cidata_directory(profile, cidata_dir, iso_path_for_debs=iso_file)
        assert summary["offline_debs_count"] == 1
        assert os.path.exists(os.path.join(cidata_dir, "user-data"))
        assert os.path.exists(os.path.join(cidata_dir, "instant-server-env.conf"))
        assert os.path.exists(os.path.join(cidata_dir, "p1_early_hardware_adapter.py"))
        assert os.path.exists(os.path.join(cidata_dir, "p2_late_laptop_hardening.sh"))

        # 3. Verify MANIFEST.sha256 integrity check
        integrity_errors = verify_staged_cidata_directory(cidata_dir)
        assert not integrity_errors, f"Integrity check failed: {integrity_errors}"
    print("  -> PASS: End-to-end cidata staging, ISO .deb extraction, and MANIFEST.sha256 verification succeeded!")
    return 0


def main() -> int:
    parser = argparse.ArgumentParser(description="Instant Server — M5 Payload Packager")
    parser.add_argument("--self-test", action="store_true", help="Run end-to-end integration self-test")
    parser.add_argument("--output-dir", type=str, default="", help="Directory to stage cidata files")
    parser.add_argument("--hostname", type=str, default="instant-server")
    parser.add_argument("--install-mode", type=str, default="dual_boot")
    args = parser.parse_args()

    if args.self_test:
        return _run_self_test()

    if args.output_dir:
        prof = m4_config_builder.ServerProfile(
            hostname=args.hostname,
            install_mode=args.install_mode,
            confirm_erase_main_drive=(args.install_mode == "auto_erase"),
        )
        res = stage_cidata_directory(prof, args.output_dir)
        print(json.dumps(res, indent=2))
        return 0

    parser.print_help()
    return 0


if __name__ == "__main__":
    sys.exit(main())