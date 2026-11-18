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
	"os"
	"path/filepath"
)

// loadBundledScript reads a standalone script from src/instant_server/scripts/ (or repo root),
// falling back to a minimal built-in version during isolated unit tests.
func LoadBundledScript(fileName string, fallback string) []byte {
	candidates := []string{
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
    touch /run/casper-no-prompt 2>/dev/null || true
    sync
    reboot -f
fi
`, cfg.Username, hashedPass, sshPort, dashPort)
}

const scriptStartUSBLoggerSh = `#!/usr/bin/env bash
set -uo pipefail
systemctl stop casper-md5check.service 2>/dev/null || killall -9 casper-md5check 2>/dev/null || true
mkdir -p /mnt/cidata /mnt/usb_logs
CDEV=$(blkid -L CIDATA 2>/dev/null || blkid -L cidata 2>/dev/null || true)
UDEV=$(blkid -L INSTANTUSB 2>/dev/null || true)
if [ -n "$CDEV" ]; then
    mount -o rw,sync,flush,noatime "$CDEV" /mnt/cidata 2>/dev/null || mount -o remount,rw,sync,flush /mnt/cidata 2>/dev/null || true
fi
if [ -n "$UDEV" ]; then
    mount -o rw,sync,flush,noatime "$UDEV" /mnt/usb_logs 2>/dev/null || mount -o remount,rw,sync,flush /mnt/usb_logs 2>/dev/null || true
fi
mkdir -p /mnt/cidata/logs /mnt/usb_logs/logs 2>/dev/null || true

LOG_A="/mnt/cidata/INSTANT-SERVER-INSTALL.log"
LOG_B="/mnt/usb_logs/INSTANT-SERVER-INSTALL.log"

write_both() {
    cat >> "$LOG_A" 2>/dev/null || true
    if mountpoint -q /mnt/usb_logs 2>/dev/null; then
        cp -f "$LOG_A" "$LOG_B" 2>/dev/null || true
    fi
    sync
}

{
    echo "================================================================================"
    echo " INSTANT SERVER LIVE USB INSTALLATION & DIAGNOSTICS LOG"
    echo " Started at (UTC): $(date -u '+%Y-%m-%d %H:%M:%S UTC')"
    echo "================================================================================"
    if [ -d /sys/firmware/efi ]; then
        echo "[BOOT MODE] UEFI Mode"
    else
        echo "[BOOT MODE] Legacy BIOS / CSM Mode"
    fi
    echo "[KERNEL CMDLINE] $(cat /proc/cmdline 2>/dev/null || true)"
    echo "[HARDWARE DMI] Vendor=$(cat /sys/class/dmi/id/sys_vendor 2>/dev/null || echo unknown) | Product=$(cat /sys/class/dmi/id/product_name 2>/dev/null || echo unknown) | BIOS=$(cat /sys/class/dmi/id/bios_version 2>/dev/null || echo unknown)"
    echo "--- MEMORY ---"
    free -h 2>/dev/null || true
    echo "--- BLOCK DEVICES (lsblk) ---"
    lsblk -o NAME,SIZE,TYPE,FSTYPE,LABEL,RM,ROTA,MOUNTPOINT,MODEL 2>/dev/null || true
    echo "--- BLKID ---"
    blkid 2>/dev/null || true
    echo "--- NETWORK INTERFACES (ip -br link / addr) ---"
    ip -br link 2>/dev/null || true
    ip -br addr 2>/dev/null || true
    echo "--- PCI & USB NETWORK ADAPTERS ---"
    lspci -nnk 2>/dev/null | grep -A3 -iE "net|eth|wireless|wifi" || true
    lsusb 2>/dev/null || true
    rfkill list 2>/dev/null || true
    echo "================================================================================"
} | write_both

if [ ! -f /run/instant-usb-log-daemon.pid ]; then
    nohup bash /mnt/cidata/usb_log_daemon.sh >/dev/null 2>&1 &
    echo $! > /run/instant-usb-log-daemon.pid
fi
exit 0
`

const scriptUSBLogDaemonSh = `#!/usr/bin/env bash
set -uo pipefail
LOG_A="/mnt/cidata/INSTANT-SERVER-INSTALL.log"
LOG_B="/mnt/usb_logs/INSTANT-SERVER-INSTALL.log"
LAST_J_LINES=0
LAST_SUB_LINES=0
LAST_CUR_LINES=0

while true; do
    {
        if command -v journalctl >/dev/null 2>&1; then
            TOTAL_J=$(journalctl -b --no-pager 2>/dev/null | wc -l || echo 0)
            if [ "${TOTAL_J:-0}" -gt "$LAST_J_LINES" ]; then
                NEW_CNT=$((TOTAL_J - LAST_J_LINES))
                echo "--- [JOURNALCTL +${NEW_CNT} lines @ $(date -u '+%H:%M:%S')] ---"
                journalctl -b --no-pager 2>/dev/null | tail -n "$NEW_CNT" | grep -E -i "subiquity|curtin|netplan|networkd|wpa|cloud-init|instant-server|error|fail|warn|parted|sfdisk|sgdisk|grub|mount" || true
                LAST_J_LINES="$TOTAL_J"
            fi
        fi
        if [ -f /var/log/installer/subiquity-server-debug.log ]; then
            TOTAL_S=$(wc -l < /var/log/installer/subiquity-server-debug.log 2>/dev/null || echo 0)
            if [ "${TOTAL_S:-0}" -gt "$LAST_SUB_LINES" ]; then
                NEW_S=$((TOTAL_S - LAST_SUB_LINES))
                echo "--- [SUBIQUITY DEBUG +${NEW_S} lines @ $(date -u '+%H:%M:%S')] ---"
                tail -n "$NEW_S" /var/log/installer/subiquity-server-debug.log 2>/dev/null | tail -n 80 || true
                LAST_SUB_LINES="$TOTAL_S"
            fi
            cp -f /var/log/installer/subiquity-server-debug.log /mnt/cidata/logs/subiquity-server-debug.log 2>/dev/null || true
        fi
        if [ -f /var/log/installer/curtin-install.log ]; then
            TOTAL_C=$(wc -l < /var/log/installer/curtin-install.log 2>/dev/null || echo 0)
            if [ "${TOTAL_C:-0}" -gt "$LAST_CUR_LINES" ]; then
                NEW_C=$((TOTAL_C - LAST_CUR_LINES))
                echo "--- [CURTIN INSTALL +${NEW_C} lines @ $(date -u '+%H:%M:%S')] ---"
                tail -n "$NEW_C" /var/log/installer/curtin-install.log 2>/dev/null | tail -n 80 || true
                LAST_CUR_LINES="$TOTAL_C"
            fi
            cp -f /var/log/installer/curtin-install.log /mnt/cidata/logs/curtin-install.log 2>/dev/null || true
        fi
    } >> "$LOG_A" 2>/dev/null || true

    if mountpoint -q /mnt/usb_logs 2>/dev/null; then
        cp -f "$LOG_A" "$LOG_B" 2>/dev/null || true
        cp -rf /mnt/cidata/logs/* /mnt/usb_logs/logs/ 2>/dev/null || true
    fi
    sync
    python3 - << 'PYCLEAN' 2>/dev/null || true
import subprocess
out = subprocess.check_output(["lsblk", "-lnpo", "NAME,FSTYPE"], text=True, stderr=subprocess.DEVNULL)
for line in out.splitlines():
    parts = line.split()
    if len(parts) == 2 and parts[1].lower() == "vfat":
        dev = parts[0]
        try:
            with open(dev, "r+b") as f:
                bpb = bytearray(f.read(512))
                if len(bpb) == 512:
                    if bpb[54:62] == b"FAT16   ":
                        bpb[37] = 0x00
                        f.seek(0)
                        f.write(bpb)
                        res = bpb[14] | (bpb[15] << 8)
                        f.seek(res * 512 + 2)
                        f.write(b"\xff\xff")
                    elif bpb[82:90] == b"FAT32   ":
                        bpb[65] = 0x00
                        f.seek(0)
                        f.write(bpb)
                        f.seek(6 * 512)
                        b6 = bytearray(f.read(512))
                        if len(b6) == 512:
                            b6[65] = 0x00
                            f.seek(6 * 512)
                            f.write(b6)
                        res = bpb[14] | (bpb[15] << 8)
                        f.seek(res * 512 + 4)
                        f.write(b"\xff\xff\xff\x0f")
        except Exception:
            pass
PYCLEAN
    sleep 2
done
`

const scriptDumpErrorLogsSh = `#!/usr/bin/env bash
set -uo pipefail
STATUS="${1:-ERROR}"
mkdir -p /mnt/cidata/logs /mnt/usb_logs/logs
CDEV=$(blkid -L CIDATA 2>/dev/null || blkid -L cidata 2>/dev/null || true)
UDEV=$(blkid -L INSTANTUSB 2>/dev/null || true)
[ -n "$CDEV" ] && (mount -o rw,sync,flush,noatime "$CDEV" /mnt/cidata 2>/dev/null || mount -o remount,rw,sync,flush /mnt/cidata 2>/dev/null || true)
[ -n "$UDEV" ] && (mount -o rw,sync,flush,noatime "$UDEV" /mnt/usb_logs 2>/dev/null || mount -o remount,rw,sync,flush /mnt/usb_logs 2>/dev/null || true)

LOG_A="/mnt/cidata/INSTANT-SERVER-INSTALL.log"
LOG_B="/mnt/usb_logs/INSTANT-SERVER-INSTALL.log"

{
    echo ""
    echo "================================================================================"
    echo " FINAL INSTALLER SNAPSHOT (STATUS: ${STATUS}) @ $(date -u '+%Y-%m-%d %H:%M:%S UTC')"
    echo "================================================================================"
    echo "--- /autoinstall.yaml ---"
    cat /autoinstall.yaml 2>/dev/null || true
    echo "--- LIVE /etc/netplan/*.yaml ---"
    head -n 100 /etc/netplan/*.yaml 2>/dev/null || true
    echo "--- TARGET /target/etc/netplan/*.yaml ---"
    head -n 100 /target/etc/netplan/*.yaml 2>/dev/null || true
    echo "--- NETPLAN & IP STATUS ---"
    ip -br link 2>/dev/null || true
    ip -br addr 2>/dev/null || true
    ip route 2>/dev/null || true
    echo "--- BLOCK DEVICES & MOUNTS ---"
    lsblk -o NAME,SIZE,TYPE,FSTYPE,LABEL,MOUNTPOINT 2>/dev/null || true
    echo "--- SUBIQUITY TRACEBACK / CRASH FILES (/var/crash) ---"
    for cf in /var/crash/*; do
        [ -f "$cf" ] || continue
        echo "Crash file: $cf"
        tail -n 120 "$cf" 2>/dev/null || true
    done
    echo "--- LAST 120 LINES OF /var/log/installer/subiquity-server-debug.log ---"
    tail -n 120 /var/log/installer/subiquity-server-debug.log 2>/dev/null || true
    echo "--- LAST 120 LINES OF /var/log/installer/curtin-install.log ---"
    tail -n 120 /var/log/installer/curtin-install.log 2>/dev/null || true
    echo "--- LAST 100 LINES OF DMESG ---"
    dmesg 2>/dev/null | tail -n 100 || true
    echo "================================================================================"
} >> "$LOG_A" 2>/dev/null || true

cp -f /var/log/installer/subiquity-server-debug.log /mnt/cidata/logs/ 2>/dev/null || true
cp -f /var/log/installer/curtin-install.log /mnt/cidata/logs/ 2>/dev/null || true
cp -f /autoinstall.yaml /mnt/cidata/logs/autoinstall-used.yaml 2>/dev/null || true

if mountpoint -q /mnt/usb_logs 2>/dev/null; then
    cp -f "$LOG_A" "$LOG_B" 2>/dev/null || true
    cp -rf /mnt/cidata/logs/* /mnt/usb_logs/logs/ 2>/dev/null || true
fi
sync
kill $(cat /run/instant-usb-log-daemon.pid 2>/dev/null) 2>/dev/null || true
umount /mnt/usb_logs 2>/dev/null || mount -o remount,ro /mnt/usb_logs 2>/dev/null || true
umount /mnt/cidata 2>/dev/null || mount -o remount,ro /mnt/cidata 2>/dev/null || true
    sync
    python3 - << 'PYCLEAN' 2>/dev/null || true
import subprocess
out = subprocess.check_output(["lsblk", "-lnpo", "NAME,FSTYPE"], text=True, stderr=subprocess.DEVNULL)
for line in out.splitlines():
    parts = line.split()
    if len(parts) == 2 and parts[1].lower() == "vfat":
        dev = parts[0]
        try:
            with open(dev, "r+b") as f:
                bpb = bytearray(f.read(512))
                if len(bpb) == 512:
                    if bpb[54:62] == b"FAT16   ":
                        bpb[37] = 0x00
                        f.seek(0)
                        f.write(bpb)
                        res = bpb[14] | (bpb[15] << 8)
                        f.seek(res * 512 + 2)
                        f.write(b"\xff\xff")
                    elif bpb[82:90] == b"FAT32   ":
                        bpb[65] = 0x00
                        f.seek(0)
                        f.write(bpb)
                        f.seek(6 * 512)
                        b6 = bytearray(f.read(512))
                        if len(b6) == 512:
                            b6[65] = 0x00
                            f.seek(6 * 512)
                            f.write(b6)
                        res = bpb[14] | (bpb[15] << 8)
                        f.seek(res * 512 + 4)
                        f.write(b"\xff\xff\xff\x0f")
        except Exception:
            pass
PYCLEAN
if [ "$STATUS" = "ERROR" ]; then
    printf "
[INSTANT SERVER] Installation error logs saved to INSTANT-SERVER-INSTALL.log on your USB drive.
" > /dev/console 2>/dev/null || true
    printf "
[INSTANT SERVER] Installation error logs saved to INSTANT-SERVER-INSTALL.log on your USB drive.
" > /dev/tty1 2>/dev/null || true
fi
exit 0
`
