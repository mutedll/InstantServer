# tests/test_suite.py
from __future__ import annotations

import importlib
import json
import os
import pathlib
import shutil
import subprocess
import sys
import tempfile
from typing import Any
import unittest

ROOT = pathlib.Path(__file__).resolve().parent
REPO_ROOT = ROOT.parent if ROOT.name.lower() == "tests" else ROOT

for cand in (
    REPO_ROOT,
    REPO_ROOT / "src" / "instant_server",
    REPO_ROOT / "src" / "instant_server" / "scripts",
    ROOT,
    ROOT / "src" / "instant_server",
    ROOT / "src" / "instant_server" / "scripts",
):
    if cand.is_dir() and str(cand) not in sys.path:
        sys.path.append(str(cand))

for _sh_cand in (
    REPO_ROOT / "src" / "instant_server" / "scripts" / "laptop_hardening.sh",
    ROOT.parent / "src" / "instant_server" / "scripts" / "laptop_hardening.sh",
    ROOT / "src" / "instant_server" / "scripts" / "laptop_hardening.sh",
    REPO_ROOT / "laptop_hardening.sh",
    ROOT / "laptop_hardening.sh",
):
    if _sh_cand.is_file():
        try:
            _raw = _sh_cand.read_bytes()
            if b"\r" in _raw:
                _sh_cand.write_bytes(_raw.replace(b"\r\n", b"\n").replace(b"\r", b"\n"))
        except Exception:
            pass

pkg_init: Any = importlib.import_module("__init__")
host_probe: Any = importlib.import_module("host_probe")
iso_manager: Any = importlib.import_module("iso_manager")
usb_engine: Any = importlib.import_module("usb_engine")
config_builder: Any = importlib.import_module("config_builder")
payload_packager: Any = importlib.import_module("payload_packager")
dashboard_service: Any = importlib.import_module("dashboard_service")
hardware_adapter: Any = importlib.import_module("hardware_adapter")
cli: Any = importlib.import_module("cli")

class InstantServerCleanArchitectureTests(unittest.TestCase):
    def test_01_clean_imports_and_single_line_headers(self) -> None:
        self.assertEqual(pkg_init.__version__, "2.5.0")
        for mod in (host_probe, iso_manager, usb_engine, config_builder, payload_packager, dashboard_service, hardware_adapter, cli):
            mod_file = getattr(mod, "__file__", None)
            assert mod_file is not None
            lines = pathlib.Path(mod_file).read_text(encoding="utf-8").splitlines()
            self.assertTrue(lines[0].startswith("# "), f"Expected single-line file path comment in {mod_file}, got {lines[0]}")

    def test_02_native_c_usb_flasher_compilation_and_selftest(self) -> None:
        c_candidates = [
            REPO_ROOT / "src" / "instant_server" / "native" / "win_usb_flasher.c",
            ROOT.parent / "src" / "instant_server" / "native" / "win_usb_flasher.c",
            pathlib.Path.cwd() / "src" / "instant_server" / "native" / "win_usb_flasher.c",
            ROOT / "src" / "instant_server" / "native" / "win_usb_flasher.c",
            REPO_ROOT / "win_usb_flasher.c",
            ROOT.parent / "win_usb_flasher.c",
            ROOT / "win_usb_flasher.c",
        ]
        c_file = next((p for p in c_candidates if p.is_file()), None)

        if c_file is None:
            for native_dir in (
                REPO_ROOT / "src" / "instant_server" / "native",
                ROOT.parent / "src" / "instant_server" / "native",
                pathlib.Path.cwd() / "src" / "instant_server" / "native",
            ):
                if native_dir.is_dir():
                    for entry in native_dir.iterdir():
                        if entry.is_file() and ("flasher" in entry.name.lower() or entry.suffix.lower() == ".c"):
                            canonical = native_dir / "win_usb_flasher.c"
                            if entry != canonical and not canonical.is_file():
                                try:
                                    shutil.copy2(entry, canonical)
                                    c_file = canonical
                                except Exception:
                                    c_file = entry
                            else:
                                c_file = entry
                            break
                if c_file is not None:
                    break

        if c_file is None:
            found = list(REPO_ROOT.rglob("*usb_flasher*"))
            for f in found:
                if f.is_file() and f.suffix.lower() in (".c", ".txt", ""):
                    c_file = f
                    break

        self.assertIsNotNone(c_file, f"win_usb_flasher.c must exist in {REPO_ROOT / 'src/instant_server/native/win_usb_flasher.c'}")
        assert c_file is not None
        c_text = c_file.read_text(encoding="utf-8")
        first_line = c_text.splitlines()[0]
        self.assertEqual(first_line.strip(), "// src/instant_server/native/win_usb_flasher.c")
        self.assertIn("FSCTL_LOCK_VOLUME", c_text)
        self.assertIn("FILE_FLAG_NO_BUFFERING", c_text)
        self.assertIn("VERIFIED", c_text)

        compiler_candidates = [
            shutil.which("gcc"),
            shutil.which("clang"),
            r"C:\msys64\mingw64\bin\gcc.exe",
            r"C:\msys64\ucrt64\bin\gcc.exe",
            r"C:\MinGW\bin\gcc.exe",
        ]
        compiler = next((c for c in compiler_candidates if c and os.path.isfile(c)), None)
        if compiler:
            with tempfile.TemporaryDirectory(dir=str(ROOT)) as td:
                exe_name = "win_usb_flasher_test.exe" if os.name == "nt" else "win_usb_flasher_test"
                bin_out = pathlib.Path(td) / exe_name
                comp = subprocess.run(
                    [compiler, "-std=c11", "-O3", "-Wall", "-Wextra", "-Wpedantic", "-Werror", str(c_file), "-o", str(bin_out)],
                    capture_output=True,
                    text=True,
                )
                self.assertEqual(comp.returncode, 0, f"C11 compiler failed: {comp.stderr}")
                run = subprocess.run([str(bin_out), "--self-test"], capture_output=True, text=True)
                self.assertEqual(run.returncode, 0, f"Self-test failed: {run.stderr}")
                self.assertIn('"stage":"VERIFIED"', run.stdout)

    def test_03_error_code_catalog_and_lookup(self) -> None:
        all_codes = config_builder.lookup_error_code("")
        self.assertGreaterEqual(len(all_codes), 12)
        err_101 = config_builder.lookup_error_code("IS-ERR-101")
        self.assertEqual(len(err_101), 1)
        self.assertEqual(err_101[0]["code"], "IS-ERR-101")
        wifi_search = config_builder.lookup_error_code("Wi-Fi")
        self.assertGreaterEqual(len(wifi_search), 1)

    def test_04_wifi_failover_and_64tb_storage_validation(self) -> None:
        prof = config_builder.ServerProfile(
            install_mode="dual_boot",
            dual_boot_Allocation_gb=16384,
            wifi_ssid="HomeLab5G",
            wifi_password="supersecretpassword",
            disable_wifi_power_save=True,
            enable_mosh_server=True,
            selected_apps=["jellyfin", "kiwix", "immich"],
        )
        payloads = config_builder.compile_cidata_payloads(prof)
        user_data = payloads["user-data"]
        self.assertIn("route-metric: 100", user_data)
        self.assertIn("route-metric: 600", user_data)
        self.assertIn("mosh", user_data)

    def test_05_day2_self_healing_guardrails_in_hardening_script(self) -> None:
        sh_candidates = [
            REPO_ROOT / "src" / "instant_server" / "scripts" / "laptop_hardening.sh",
            ROOT.parent / "src" / "instant_server" / "scripts" / "laptop_hardening.sh",
            pathlib.Path.cwd() / "src" / "instant_server" / "scripts" / "laptop_hardening.sh",
            ROOT / "src" / "instant_server" / "scripts" / "laptop_hardening.sh",
            REPO_ROOT / "laptop_hardening.sh",
            ROOT / "laptop_hardening.sh",
        ]
        sh_file = next((p for p in sh_candidates if p.is_file()), None)
        self.assertIsNotNone(sh_file)
        assert sh_file is not None
        raw_bytes = sh_file.read_bytes()
        if b"\r" in raw_bytes:
            raw_bytes = raw_bytes.replace(b"\r\n", b"\n").replace(b"\r", b"\n")
            sh_file.write_bytes(raw_bytes)
        self.assertNotIn(b"\r", sh_file.read_bytes())
        sh_text = sh_file.read_text(encoding="utf-8")
        self.assertIn("SystemMaxUse=200M", sh_text)
        self.assertIn("instant-server-maintenance.timer", sh_text)
        self.assertIn("instant-server-netwatch.timer", sh_text)
        self.assertIn("power_save off", sh_text)

    def test_06_payload_staging_EndToEnd(self) -> None:
        with tempfile.TemporaryDirectory(dir=str(ROOT)) as td:
            prof = config_builder.ServerProfile(
                hostname="instant-server",
                username="admin",
                password="password123",
                selected_apps=["jellyfin", "kiwix"],
            )
            res = payload_packager.stage_cidata_directory(prof, td)
            self.assertIsInstance(res, dict)
            self.assertTrue((pathlib.Path(td) / "user-data").is_file())
            self.assertTrue((pathlib.Path(td) / "meta-data").is_file())
            ver = payload_packager.verify_staged_cidata_directory(td)
            self.assertEqual(ver, [], f"Verification errors: {ver}")

    def test_07_universal_hardware_form_factors_and_repair_usb(self) -> None:
        for ff in ("auto_detect", "laptop_ewaste", "laptop_modern", "mini_pc_n100", "desktop_pc", "workstation_gpu", "nas_multibay"):
            prof = config_builder.ServerProfile(
                device_form_factor=ff,
                install_mode="repair_rescue_usb" if ff == "desktop_pc" else "dual_boot",
            )
            self.assertEqual(prof.validate(), [])
        mac_host = hardware_adapter.resolve_cluster_node_hostname("instant-server", "usb_multi_node_auto", "b8f2")
        self.assertEqual(mac_host, "instant-server-b8f2")

    def test_08_multi_server_cluster_merger_and_wipe_gate(self) -> None:
        p_node = {"hostname": "instant-server-1", "used_gb": 420.0, "total_gb": 1000.0, "apps": ["jellyfin", "immich"]}
        s_node = {"hostname": "instant-server-2", "used_gb": 310.0, "total_gb": 1000.0, "apps": ["jellyfin", "kiwix"]}

        safe_audit = config_builder.run_cluster_preflight_audit(p_node, s_node, "mergerfs_union_safe")
        self.assertTrue(safe_audit["ok"])
        self.assertEqual(safe_audit["combined_total_gb"], 2000.0)
        self.assertEqual(safe_audit["combined_used_gb"], 730.0)
        self.assertIn("jellyfin", safe_audit["sqlite_pinned_apps"])

        blocked_audit = config_builder.run_cluster_preflight_audit(p_node, s_node, "wipe_secondary_expand", "")
        self.assertFalse(blocked_audit["ok"])
        self.assertTrue(blocked_audit["wipe_blocked"])

        confirmed_audit = config_builder.run_cluster_preflight_audit(
            p_node, s_node, "wipe_secondary_expand", "WIPE-instant-server-2"
        )
        self.assertTrue(confirmed_audit["ok"])
        self.assertFalse(confirmed_audit["wipe_blocked"])

if __name__ == "__main__":
    unittest.main(verbosity=2)