# src/instant_server/__init__.py
from __future__ import annotations

import importlib as _importlib
import pathlib as _pathlib
import sys as _sys
from typing import Any as _Any

_HERE = _pathlib.Path(__file__).resolve().parent
for _candidate in (_HERE, _HERE / "src" / "instant_server", _HERE / "scripts", _HERE.parent):
    if _candidate.is_dir() and str(_candidate) not in _sys.path:
        _sys.path.append(str(_candidate))

for _sh_norm in (
    _HERE / "scripts" / "laptop_hardening.sh",
    _HERE / "src" / "instant_server" / "scripts" / "laptop_hardening.sh",
    _HERE / "laptop_hardening.sh",
):
    if _sh_norm.is_file():
        try:
            _b = _sh_norm.read_bytes()
            if b"\r" in _b:
                _sh_norm.write_bytes(_b.replace(b"\r\n", b"\n").replace(b"\r", b"\n"))
        except Exception:
            pass

__version__ = "2.5.0"

host_probe: _Any = _importlib.import_module("host_probe")
iso_manager: _Any = _importlib.import_module("iso_manager")
usb_engine: _Any = _importlib.import_module("usb_engine")
config_builder: _Any = _importlib.import_module("config_builder")
payload_packager: _Any = _importlib.import_module("payload_packager")
dashboard_service: _Any = _importlib.import_module("dashboard_service")
cli: _Any = _importlib.import_module("cli")

__all__ = [
    "__version__",
    "host_probe",
    "iso_manager",
    "usb_engine",
    "config_builder",
    "payload_packager",
    "dashboard_service",
    "cli",
]