# instant_server.py
from __future__ import annotations

import importlib as _importlib
import pathlib as _pathlib
import sys as _sys
from typing import Any as _Any

_ROOT = _pathlib.Path(__file__).resolve().parent
for _candidate in (_ROOT, _ROOT / "src" / "instant_server", _ROOT / "src" / "instant_server" / "scripts", _ROOT / "src"):
    if _candidate.is_dir() and str(_candidate) not in _sys.path:
        _sys.path.append(str(_candidate))

_cli: _Any = _importlib.import_module("cli")
main = _cli.main

if __name__ == "__main__":
    raise SystemExit(main())