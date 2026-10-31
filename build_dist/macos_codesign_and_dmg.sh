#!/usr/bin/env bash
# Instant Server — macOS Gatekeeper Ad-Hoc Codesigning & DMG Packaging Script
set -euo pipefail

APP_PATH="dist/InstantServer.app"

if [[ ! -d "$APP_PATH" ]]; then
    echo "Error: $APP_PATH not found. Run PyInstaller first: pyinstaller InstantServer.spec"
    exit 1
fi

echo "[macOS Packager] Clearing quarantine attributes and applying ad-hoc codesign..."
xattr -cr "$APP_PATH" || true
codesign --force --deep --sign - "$APP_PATH"
codesign --verify --verbose=2 "$APP_PATH"

echo "[macOS Packager] Creating compressed InstantServer-macOS.dmg..."
hdiutil create -volname "Instant Server" \
    -srcfolder "$APP_PATH" \
    -ov -format UDZO \
    "dist/InstantServer-macOS.dmg"

echo "[macOS Packager] Done: dist/InstantServer-macOS.dmg"
