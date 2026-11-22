// ============================================================================
// FILE: usb/payload_boot.go
// PURPOSE: Loads bundled Python/Bash scripts and provides early-boot USB logging,
//
//	error dumping, and emergency password/bootloader repair scripts.
//
// ============================================================================
package payload

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

var embeddedAssetsFS fs.FS

// SetEmbeddedAssets registers the Wails embed.FS so standalone binaries can stage
// bundled Python/Bash scripts and dashboard assets without external src/ files on disk.
func SetEmbeddedAssets(fsys fs.FS) {
	embeddedAssetsFS = fsys
}

// LoadBundledScript reads a standalone script from embed.FS first, then src/instant_server/scripts/
// on disk, falling back to a minimal built-in version during isolated unit tests.
func LoadBundledScript(fileName string, fallback string) []byte {
	if embeddedAssetsFS != nil {
		for _, embPath := range []string{
			"src/instant_server/scripts/os/" + fileName,
			"src/instant_server/scripts/dashboard/" + fileName,
			"src/instant_server/scripts/" + fileName,
		} {
			if data, err := fs.ReadFile(embeddedAssetsFS, embPath); err == nil && len(bytes.TrimSpace(data)) > 0 {
				return data
			}
		}
	}
	candidates := []string{
		filepath.Join("src", "instant_server", "scripts", "os", fileName),
		filepath.Join("src", "instant_server", "scripts", "dashboard", fileName),
		filepath.Join("..", "src", "instant_server", "scripts", "os", fileName),
		filepath.Join("..", "src", "instant_server", "scripts", "dashboard", fileName),
		filepath.Join("..", "..", "src", "instant_server", "scripts", "os", fileName),
		filepath.Join("..", "..", "src", "instant_server", "scripts", "dashboard", fileName),
		filepath.Join("src", "instant_server", "scripts", fileName),
		filepath.Join("..", "src", "instant_server", "scripts", fileName),
		filepath.Join("..", "..", "src", "instant_server", "scripts", fileName),
		fileName,
		filepath.Join("..", fileName),
	}
	for _, path := range candidates {
		if data, err := os.ReadFile(path); err == nil && len(bytes.TrimSpace(data)) > 0 {
			return data
		}
	}
	return []byte(fallback)
}

// LoadBundledDashboardAsset reads a dashboard CSS/JS asset from embed.FS first, then disk.
func LoadBundledDashboardAsset(assetName string) []byte {
	if embeddedAssetsFS != nil {
		for _, embPath := range []string{
			"src/instant_server/ui/dashboard/styles/" + assetName,
			"src/instant_server/ui/dashboard/core/" + assetName,
			"src/instant_server/ui/dashboard/apps/" + assetName,
			"src/instant_server/ui/disk_manager/" + assetName,
			"src/instant_server/ui/dashboard/" + assetName,
			"src/instant_server/ui/" + assetName,
		} {
			if data, err := fs.ReadFile(embeddedAssetsFS, embPath); err == nil && len(data) > 0 {
				return data
			}
		}
	}
	for _, cand := range []string{
		filepath.Join("src", "instant_server", "ui", "dashboard", "styles", assetName),
		filepath.Join("src", "instant_server", "ui", "dashboard", "core", assetName),
		filepath.Join("src", "instant_server", "ui", "dashboard", "apps", assetName),
		filepath.Join("src", "instant_server", "ui", "disk_manager", assetName),
		filepath.Join("..", "src", "instant_server", "ui", "disk_manager", assetName),
		filepath.Join("..", "..", "src", "instant_server", "ui", "disk_manager", assetName),
		filepath.Join("..", "src", "instant_server", "ui", "dashboard", "styles", assetName),
		filepath.Join("..", "src", "instant_server", "ui", "dashboard", "core", assetName),
		filepath.Join("..", "src", "instant_server", "ui", "dashboard", "apps", assetName),
		filepath.Join("..", "..", "src", "instant_server", "ui", "dashboard", "styles", assetName),
		filepath.Join("..", "..", "src", "instant_server", "ui", "dashboard", "core", assetName),
		filepath.Join("..", "..", "src", "instant_server", "ui", "dashboard", "apps", assetName),
		filepath.Join("src", "instant_server", "ui", "dashboard", assetName),
		filepath.Join("..", "src", "instant_server", "ui", "dashboard", assetName),
		filepath.Join("src", "instant_server", "ui", assetName),
		assetName,
		filepath.Join("..", assetName),
	} {
		if rawAsset, err := os.ReadFile(cand); err == nil && len(rawAsset) > 0 {
			return rawAsset
		}
	}
	return nil
}

const DefaultAutoinstallConfigPy = `#!/usr/bin/env python3
"""
=============================================================================
HOW TO STUDY & MODIFY THIS FILE (Step-by-Step Beginner Guide)
=============================================================================
File: autoinstall_config.py
Purpose:
  Safely modifies /autoinstall.yaml during early-commands on the target Ubuntu
  Server machine using PyYAML instead of fragile regex/sed edits.

Why this exists:
  1. Removes 'storage.config' before injected 'swap:' or 'updates:' keys so
     Subiquity can smoothly switch to 'storage.layout' when a hardware disk
     sizing fallback is needed.
  2. When a wired Ethernet cable ('en*'/'eth*') is already online ('--wired-online'),
     replaces 'network:' with a minimal non-blocking wired-only Netplan v2 config
     ('optional: true', no 'wifis:') so Subiquity's NetworkController does not
     synthesize a blocking config for unplugged ports or fail on missing
     'wpasupplicant'.
  3. Explicitly sets 'source.search_drivers = False', 'drivers.install = False',
     and 'oem.install = False' so Ubuntu 24.04 / 26.04 Subiquity does not stall
     at 'subiquity/OEM/load_metapackages_list/wait_confirmation' or
     'subiquity/Drivers/_list_drivers/wait_apt'.
=============================================================================
"""

import sys
from typing import Any
import yaml  # type: ignore[import-untyped,import-not-found]

AUTOINSTALL_PATH = "/autoinstall.yaml"


def load_autoinstall(path: str) -> dict[str, Any]:
    with open(path, "r", encoding="utf-8") as f:
        loaded = yaml.safe_load(f)
        return loaded if isinstance(loaded, dict) else {}


def save_autoinstall(path: str, data: dict[str, Any]) -> None:
    with open(path, "w", encoding="utf-8") as f:
        yaml.safe_dump(data, f, default_flow_style=False, sort_keys=False)


def enforce_nonblocking_controllers(ai: dict[str, Any]) -> None:
    """
    Ensures Subiquity's Drivers and OEM controllers never pause waiting for
    interactive confirmation or online apt driver queries during autoinstall.
    Reference: https://canonical-subiquity.readthedocs-hosted.com/en/latest/reference/autoinstall-reference.html
    """
    raw_src = ai.get("source")
    src: dict[str, Any] = dict(raw_src) if isinstance(raw_src, dict) else {"id": "ubuntu-server-minimal"}
    src["search_drivers"] = False
    ai["source"] = src

    ai["drivers"] = {"install": False}
    ai["oem"] = {"install": False}


def main() -> None:
    mode = sys.argv[1] if len(sys.argv) > 1 else "--sanitize"
    data = load_autoinstall(AUTOINSTALL_PATH)
    ai = data.get("autoinstall")
    if not isinstance(ai, dict):
        return

    enforce_nonblocking_controllers(ai)

    if mode == "--wired-online":
        # Instead of deleting 'network:' (which makes Subiquity generate a default
        # config for all eth*/en* ports without optional:true), keep a minimal
        # wired-only config with optional: true and no wifis block.
        ai["network"] = {
            "version": 2,
            "ethernets": {
                "wired-lan": {
                    "match": {"name": "en*"},
                    "dhcp4": True,
                    "dhcp6": False,
                    "optional": True,
                },
                "wired-eth": {
                    "match": {"name": "eth*"},
                    "dhcp4": True,
                    "dhcp6": False,
                    "optional": True,
                },
            },
        }
    elif mode == "--strip-storage-config":
        layout_name = sys.argv[2] if len(sys.argv) > 2 else "lvm"
        disk_path = sys.argv[3] if len(sys.argv) > 3 else ""
        layout: dict[str, Any] = {"name": layout_name}
        if disk_path:
            layout["match"] = {"path": disk_path}
        ai["storage"] = {"layout": layout}

    save_autoinstall(AUTOINSTALL_PATH, data)


if __name__ == "__main__":
    main()
`

const DefaultDashboardServerPy = `#!/usr/bin/env python3
# Standalone HTTP daemon for Instant Server Web Dashboard (:8088)
import http.server, json, os, socketserver

PORT = int(os.environ.get("DASHBOARD_PORT", "8088"))

class Handler(http.server.SimpleHTTPRequestHandler):
    def do_GET(self):
        if self.path.startswith("/api/status"):
            body = json.dumps({"ok": True, "hostname": os.uname().nodename}).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(body)
            return
        return super().do_GET()

if __name__ == "__main__":
    socketserver.TCPServer.allow_reuse_address = True
    with socketserver.TCPServer(("0.0.0.0", PORT), Handler) as srv:
        srv.serve_forever()
`

// ============================================================================
// SECTION 6: EARLY-BOOT & AUTOINSTALL HELPER SCRIPTS
// ============================================================================
// Moving these shell scripts out of StageCIDATAPayload keeps the Go staging
// logic short and makes each target helper script easy to study on its own.

func buildEmergencyRepairScript(cfg ServerProfileConfig) string {
	hashedPass := Sha512Crypt(cfg.Password, "instantserverlts")
	sshPort := cfg.SSHPort
	if sshPort <= 0 || sshPort > 65535 {
		sshPort = 22
	}
	dashPort := cfg.DashboardPort
	if dashPort <= 0 || dashPort > 65535 {
		dashPort = 8088
	}
	return fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
TARGET_USER=%q
NEW_HASH=%q
SSH_PORT="%d"
DASH_PORT="%d"
mkdir -p /mnt/repair_os
for part in $(lsblk -lnpo NAME,FSTYPE | awk '$2 == "ext4" {print $1}'); do
    e2fsck -fy "$part" 2>/dev/null || true
    if mount "$part" /mnt/repair_os 2>/dev/null; then
        if [ -f "/mnt/repair_os/etc/shadow" ]; then
            chroot /mnt/repair_os /bin/bash -c "rm -f /var/lib/dpkg/lock-frontend /var/lib/dpkg/lock && dpkg --configure -a" 2>/dev/null || true
            chroot /mnt/repair_os /bin/bash -c "printf '%%s:%%s\n' '$TARGET_USER' '$NEW_HASH' | chpasswd -e" 2>/dev/null || true
            chroot /mnt/repair_os /bin/bash -c "ufw allow 22/tcp && ufw allow ${SSH_PORT}/tcp && ufw allow ${DASH_PORT}/tcp && fail2ban-client unban --all" 2>/dev/null || true
            echo "[instant-server] Reset password for $TARGET_USER and repaired filesystem on $part."
        fi
        umount /mnt/repair_os 2>/dev/null || true
    fi
done
if [ -d "/cdrom" ] || grep -q "casper" /proc/cmdline 2>/dev/null; then
    sync
    reboot -f
fi
`, cfg.Username, hashedPass, sshPort, dashPort)
}
