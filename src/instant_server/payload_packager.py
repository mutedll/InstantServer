# src/instant_server/payload_packager.py
from __future__ import annotations

import argparse
import importlib
import hashlib
import json
import os
import sys
import tempfile
from typing import Any, Callable, Dict, List, Optional

_CURRENT_DIR = os.path.dirname(os.path.abspath(__file__))
if _CURRENT_DIR not in sys.path:
    sys.path.insert(0, _CURRENT_DIR)

host_probe: Any = importlib.import_module("host_probe")
iso_manager: Any = importlib.import_module("iso_manager")
usb_engine: Any = importlib.import_module("usb_engine")
config_builder: Any = importlib.import_module("config_builder")

def emit_event(
    event_type: str,
    message: str,
    data: Optional[Dict[str, Any]] = None,
    callback: Optional[Callable[[Dict[str, Any]], None]] = None,
) -> Dict[str, Any]:
    payload = {
        "event": event_type,
        "module": "PAYLOAD_PACKAGER",
        "project": "Instant Server",
        "message": message,
        "data": data or {},
    }
    if callback:
        callback(payload)
    return payload

def _build_apps_unpacker_script(selected_apps: List[str]) -> str:
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
    profile: Any,
    staging_dir: str,
    iso_path_for_debs: Optional[str] = None,
    app_cache_dir: Optional[str] = None,
    event_callback: Optional[Callable[[Dict[str, Any]], None]] = None,
) -> Dict[str, Any]:
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

    compiled = config_builder.compile_cidata_payloads(profile, event_callback=event_callback)
    written_files: List[str] = []

    for fname, text_content in compiled.items():
        fpath = os.path.join(staging_dir, fname)
        clean_lf = text_content.replace("\r\n", "\n")
        with open(fpath, "w", encoding="utf-8", newline="\n") as fh:
            fh.write(clean_lf)
        written_files.append(fpath)

    script_pairs = [
        ("hardware_adapter.py", "hardware_adapter.py"),
        ("laptop_hardening.sh", "laptop_hardening.sh"),
        ("dashboard_service.py", "dashboard_service.py"),
        ("server_dashboard.html", "server_dashboard.html"),
    ]
    for src_name, clean_name in script_pairs:
        candidates = [
            os.path.join(_CURRENT_DIR, "scripts", clean_name),
            os.path.join(_CURRENT_DIR, "ui", clean_name),
            os.path.join(_CURRENT_DIR, clean_name),
            os.path.join(_CURRENT_DIR, src_name),
        ]
        src_script = next((c for c in candidates if os.path.isfile(c)), None)
        if src_script:
            with open(src_script, "r", encoding="utf-8") as rf:
                raw_text = rf.read().replace("\r\n", "\n")
            for out_name in {src_name, clean_name}:
                dst_script = os.path.join(staging_dir, out_name)
                with open(dst_script, "w", encoding="utf-8", newline="\n") as wf:
                    wf.write(raw_text)
                if out_name.endswith((".py", ".sh")):
                    os.chmod(dst_script, 0o755)
                if dst_script not in written_files:
                    written_files.append(dst_script)

    staged_apps: List[str] = []
    valid_apps = [a for a in (profile.selected_apps or []) if a in config_builder.APP_CATALOG_BY_ID]
    for app_id in valid_apps:
        app_meta = config_builder.APP_CATALOG_BY_ID[app_id]
        app_subdir = os.path.join(apps_dir, app_id)
        os.makedirs(app_subdir, exist_ok=True)

        compose_text = config_builder.generate_casaos_app_compose(app_id, timezone=profile.timezone)
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
            resolved_app_cache = app_cache_dir or os.path.join(os.path.dirname(os.path.abspath(staging_dir)), "app_cache")
            try:
                iso_manager.pull_docker_image_tarball(
                    app_meta["image"],
                    tar_dest,
                    cache_dir=resolved_app_cache,
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

    extracted_debs: List[str] = []
    if iso_path_for_debs and os.path.exists(iso_path_for_debs):
        try:
            extracted_debs = iso_manager.extract_offline_debs_from_iso(
                iso_path_for_debs,
                debs_dir,
                event_callback=event_callback,
            )
            written_files.extend(extracted_debs)
        except Exception:
            pass

    manifest_lines: List[str] = []
    manifest_map: Dict[str, str] = {}
    for fpath in sorted(written_files):
        rel = os.path.relpath(fpath, staging_dir).replace("\\", "/")
        digest = iso_manager.compute_sha256_streaming(fpath)
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
            actual_hash = iso_manager.compute_sha256_streaming(target_file)
            if actual_hash != expected_hash:
                errors.append(f"Corrupted file {rel_path}: expected {expected_hash}, got {actual_hash}")
    return errors

def _run_self_test() -> int:
    print("[PayloadPackager Test] Testing end-to-end orchestration (M1 + M2 + M3 + M4 + M5 + Pre-Unpacked Apps)...")
    with tempfile.TemporaryDirectory() as tmpdir:
        iso_file = os.path.join(tmpdir, "ubuntu-24.04-mock.iso")
        iso_manager._build_synthetic_iso_with_deb(
            iso_file,
            "wpasupplicant_2.10-21build4_amd64.deb",
            b"!<arch>\ndebian-binary\n",
        )

        profile = config_builder.ServerProfile(
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
        prof = config_builder.ServerProfile(
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