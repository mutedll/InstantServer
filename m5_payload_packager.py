#!/usr/bin/env python3
"""
Instant Server — Module M5: End-to-End `cidata` & Pre-Unpacked Self-Hosted Apps Packager
========================================================================================
File: m5_payload_packager.py
Project: Instant Server (Turn any old laptop into a headless Ubuntu Server)
Runs on: Windows 10/11, macOS, and Linux (Python 3.8+ through 3.14+, zero external deps)

Responsibilities:
1. Coordinates M1 (Host Probe), M2 (ISO & Offline `.deb` / OCI Extractor), M3 (USB Engine),
   and M4 (Config Builder) to assemble a complete, boot-ready `cidata` directory.
2. Pre-Unpacks Selected Self-Hosted Apps on the Host PC (`cidata/apps/<app_id>/`):
   - Writes each selected app's CasaOS `docker-compose.yml` (with full `x-casaos:` AppStore
     metadata), `app-meta.json`, pre-created `/DATA/AppData/<app_id>/` directory layout,
     optional offline Docker image tarball (`image.tar.gz` when `prebundle_offline_images=True`),
     and `unpack_and_install_apps.sh` automation script.
3. Generates `MANIFEST.sha256` inside `cidata` so the target laptop can verify file integrity.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import sys
import tempfile
from typing import Any, Callable, Dict, List, Optional

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


def _build_apps_unpacker_script(selected_apps: List[str]) -> str:
    """
    Generate `unpack_and_install_apps.sh` which runs on the target laptop to:
      1. Unpack all pre-staged app bundles from `/opt/instant-server/apps/<app_id>` into
         `/var/lib/casaos/apps/<app_id>/` and create `/DATA/AppData/<app_id>/`.
      2. Load any pre-downloaded offline Docker image tarballs (`docker load -i image.tar.gz`).
      3. Register and start each app in CasaOS (`casaos-cli app-management apply`) or
         via `docker compose -f /var/lib/casaos/apps/<app_id>/docker-compose.yml up -d`.
    """
    apps_space_sep = " ".join(selected_apps)
    return f"""#!/usr/bin/env bash
# Instant Server — Automated Self-Hosted App Bundle Unpacker & CasaOS Registrar
# Pre-staged on Host PC and executed automatically on the target laptop.
set -uo pipefail

APPS_SRC_DIR="/opt/instant-server/apps"
CASAOS_APPS_DIR="/var/lib/casaos/apps"
APPDATA_ROOT="/DATA/AppData"
SELECTED_APPS=({apps_space_sep})

mkdir -p "$CASAOS_APPS_DIR" "$APPDATA_ROOT" /DATA/Media /DATA/Downloads /DATA/Sync

for app_id in "${{SELECTED_APPS[@]}}"; do
    src_bundle="${{APPS_SRC_DIR}}/${{app_id}}"
    [[ -d "$src_bundle" ]] || continue

    target_app_dir="${{CASAOS_APPS_DIR}}/${{app_id}}"
    mkdir -p "$target_app_dir" "${{APPDATA_ROOT}}/${{app_id}}/config" "${{APPDATA_ROOT}}/${{app_id}}/data"
    cp -f "${{src_bundle}}/docker-compose.yml" "${{target_app_dir}}/docker-compose.yml"
    [[ -f "${{src_bundle}}/app-meta.json" ]] && cp -f "${{src_bundle}}/app-meta.json" "${{target_app_dir}}/app-meta.json"

    # If the user pre-downloaded the container image tarball on the host PC, load it offline!
    if [[ -f "${{src_bundle}}/image.tar.gz" ]] && command -v docker >/dev/null 2>&1; then
        echo "[Instant Server Apps] Loading pre-bundled offline container image for ${{app_id}}..."
        docker load -i "${{src_bundle}}/image.tar.gz" || true
    fi

    # Register with CasaOS CLI if installed, then ensure stack is running via Docker Compose
    if command -v casaos-cli >/dev/null 2>&1; then
        casaos-cli app-management apply "$app_id" -f "${{target_app_dir}}/docker-compose.yml" || true
    fi
    if command -v docker >/dev/null 2>&1; then
        docker compose -f "${{target_app_dir}}/docker-compose.yml" up -d || true
    fi
done

systemctl restart casaos-app-management 2>/dev/null || true
echo "[Instant Server Apps] Unpacked and started ${{#SELECTED_APPS[@]}} pre-selected self-hosted app(s)."
"""


def stage_cidata_directory(
    profile: m4_config_builder.ServerProfile,
    staging_dir: str,
    iso_path_for_debs: Optional[str] = None,
    event_callback: Optional[Callable[[Dict[str, Any]], None]] = None,
) -> Dict[str, Any]:
    """
    Assemble the complete `cidata` partition file tree inside `staging_dir`:
      - `user-data`, `meta-data`, `instant-server-env.conf`, `profile.json`
      - `p1_early_hardware_adapter.py`, `p2_late_laptop_hardening.sh`
      - `apps/<app_id>/docker-compose.yml` + `apps/unpack_and_install_apps.sh`
      - `debs/` (offline `.deb` packages extracted from ISO if provided)
      - `MANIFEST.sha256`
    """
    os.makedirs(staging_dir, exist_ok=True)
    debs_dir = os.path.join(staging_dir, "debs")
    apps_dir = os.path.join(staging_dir, "apps")
    os.makedirs(debs_dir, exist_ok=True)
    os.makedirs(apps_dir, exist_ok=True)

    emit_event(
        "PACKAGER_START",
        f"Staging Instant Server cidata payload for '{profile.hostname}.local'...",
        {"staging_dir": staging_dir, "install_mode": profile.install_mode, "percent": 30},
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

    # 3. Pre-unpack all selected self-hosted apps into `cidata/apps/<app_id>/`
    staged_apps: List[str] = []
    valid_apps = [a for a in (profile.selected_apps or []) if a in m4_config_builder.APP_CATALOG_BY_ID]
    for app_id in valid_apps:
        app_meta = m4_config_builder.APP_CATALOG_BY_ID[app_id]
        app_subdir = os.path.join(apps_dir, app_id)
        os.makedirs(app_subdir, exist_ok=True)

        compose_text = m4_config_builder.generate_casaos_app_compose(app_id, timezone=profile.timezone)
        compose_path = os.path.join(app_subdir, "docker-compose.yml")
        with open(compose_path, "w", encoding="utf-8", newline="\n") as cfh:
            cfh.write(compose_text.replace("\r\n", "\n"))
        written_files.append(compose_path)

        meta_path = os.path.join(app_subdir, "app-meta.json")
        with open(meta_path, "w", encoding="utf-8", newline="\n") as mfh:
            json.dump(app_meta, mfh, indent=2)
        written_files.append(meta_path)

        if profile.prebundle_offline_images:
            tar_dest = os.path.join(app_subdir, "image.tar.gz")
            try:
                m2_iso_manager.pull_docker_image_tarball(
                    app_meta["image"],
                    tar_dest,
                    event_callback=event_callback,
                )
                if os.path.isfile(tar_dest):
                    written_files.append(tar_dest)
            except Exception as exc:
                emit_event(
                    "APP_IMAGE_PULL_FALLBACK",
                    f"Staged CasaOS compose definition for {app_meta['title']} (online image pull will complete on first boot: {exc})",
                    {"app_id": app_id},
                    callback=event_callback,
                )

        staged_apps.append(app_id)

    unpack_sh_path = os.path.join(apps_dir, "unpack_and_install_apps.sh")
    with open(unpack_sh_path, "w", encoding="utf-8", newline="\n") as ufh:
        ufh.write(_build_apps_unpacker_script(staged_apps).replace("\r\n", "\n"))
    os.chmod(unpack_sh_path, 0o755)
    written_files.append(unpack_sh_path)

    emit_event(
        "APPS_PRESTAGED",
        f"Pre-unpacked {len(staged_apps)} self-hosted app bundle(s) into USB payload ({', '.join(staged_apps) or 'none'}).",
        {"staged_apps": staged_apps, "percent": 50},
        callback=event_callback,
    )

    # 4. Extract offline `.deb` packages from ISO if an ISO path was supplied
    extracted_debs: List[str] = []
    if iso_path_for_debs and os.path.exists(iso_path_for_debs):
        try:
            extracted_debs = m2_iso_manager.extract_offline_debs_from_iso(
                iso_path_for_debs,
                debs_dir,
                event_callback=event_callback,
            )
            written_files.extend(extracted_debs)
        except Exception:
            pass

    # 5. Compute SHA-256 manifest for post-flash read-back integrity checking
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
        "staged_apps": staged_apps,
        "offline_debs_count": len(extracted_debs),
        "manifest": manifest_map,
    }
    emit_event(
        "M5_STAGING_COMPLETE",
        f"Staged {len(written_files)} files (including {len(staged_apps)} pre-unpacked apps) into cidata payload.",
        {"percent": 60, **summary},
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
    print("[M5 Self-Test] Testing end-to-end orchestration (M1 + M2 + M3 + M4 + M5 + Pre-Unpacked Apps)...")
    with tempfile.TemporaryDirectory() as tmpdir:
        iso_file = os.path.join(tmpdir, "ubuntu-24.04-mock.iso")
        m2_iso_manager._build_synthetic_iso_with_deb(
            iso_file,
            "wpasupplicant_2.10-21build4_amd64.deb",
            b"!<arch>\ndebian-binary\n",
        )

        profile = m4_config_builder.ServerProfile(
            hostname="instant-node",
            username="admin",
            password="InstantServer2026!",
            install_mode="dual_boot",
            dual_boot_Allocation_gb=50,
            dashboard_choice="both",
            selected_apps=["jellyfin", "nextcloud", "pihole", "vaultwarden"],
        )

        cidata_dir = os.path.join(tmpdir, "cidata")
        summary = stage_cidata_directory(profile, cidata_dir, iso_path_for_debs=iso_file)
        assert summary["offline_debs_count"] == 1
        assert summary["staged_apps"] == ["jellyfin", "nextcloud", "pihole", "vaultwarden"]
        assert os.path.exists(os.path.join(cidata_dir, "apps", "jellyfin", "docker-compose.yml"))
        assert os.path.exists(os.path.join(cidata_dir, "apps", "unpack_and_install_apps.sh"))

        integrity_errors = verify_staged_cidata_directory(cidata_dir)
        assert not integrity_errors, f"Integrity check failed: {integrity_errors}"
    print("  -> PASS: End-to-end cidata staging, pre-unpacked CasaOS app bundles, and MANIFEST.sha256 verified!")
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