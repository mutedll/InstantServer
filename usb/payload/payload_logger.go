// ============================================================================
// FILE: usb/payload/payload_logger.go
// PURPOSE: Live USB diagnostics logger, RAM-buffered clean-unmount log daemon,
//
//	and final error/success snapshot dumper with FAT1+FAT2 clean-bit sync.
//
// ============================================================================
package payload

const scriptStartUSBLoggerSh = `#!/usr/bin/env bash
set -uo pipefail
systemctl stop casper-md5check.service 2>/dev/null || true
mkdir -p /mnt/cidata /mnt/usb_logs /tmp/instant_server_logs
CDEV=$(blkid -L CIDATA 2>/dev/null || blkid -L cidata 2>/dev/null || true)
if [ -n "$CDEV" ] && ! mountpoint -q /mnt/cidata 2>/dev/null; then
    mount -o ro,noatime "$CDEV" /mnt/cidata 2>/dev/null || true
fi

LOG_RAM="/tmp/instant_server_logs/instant_server_install.log"
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
} >> "$LOG_RAM" 2>/dev/null || true

UDEV=$(blkid -L INSTANTUSB 2>/dev/null || true)
if [ -n "$UDEV" ]; then
    if mount -o rw,sync,flush,noatime "$UDEV" /mnt/usb_logs 2>/dev/null; then
        mkdir -p /mnt/usb_logs/instant_server_logs 2>/dev/null || true
        cp -f "$LOG_RAM" /mnt/usb_logs/instant_server_logs/instant_server_install.log 2>/dev/null || true
        sync
        umount /mnt/usb_logs 2>/dev/null || true
    fi
fi

if [ ! -f /run/instant-usb-log-daemon.pid ]; then
    nohup bash /tmp/cidata/usb_log_daemon.sh >/dev/null 2>&1 &
    echo $! > /run/instant-usb-log-daemon.pid
fi
exit 0
`

const scriptUSBLogDaemonSh = `#!/usr/bin/env bash
set -uo pipefail
mkdir -p /tmp/instant_server_logs /mnt/usb_logs
LOG_RAM="/tmp/instant_server_logs/instant_server_install.log"
LAST_J_LINES=0
LAST_SUB_LINES=0
LAST_CUR_LINES=0

# Buffer logs in RAM and briefly mount -> copy -> sync -> umount INSTANTUSB on each flush.
# Keeping INSTANTUSB cleanly unmounted prevents Linux vfat from leaving the FAT32 dirty bit
# set on disk if the machine is powered off or unplugged, eliminating Windows "Scan and Fix".
while true; do
    CHANGED=0
    {
        if command -v journalctl >/dev/null 2>&1; then
            TOTAL_J=$(journalctl -b --no-pager 2>/dev/null | wc -l || echo 0)
            if [ "${TOTAL_J:-0}" -gt "$LAST_J_LINES" ]; then
                NEW_CNT=$((TOTAL_J - LAST_J_LINES))
                echo "--- [JOURNALCTL +${NEW_CNT} lines @ $(date -u '+%H:%M:%S')] ---"
                journalctl -b --no-pager 2>/dev/null | tail -n "$NEW_CNT" | grep -E -i "subiquity|curtin|netplan|networkd|wpa|cloud-init|instant-server|error|fail|warn|parted|sfdisk|sgdisk|grub|mount" || true
                LAST_J_LINES="$TOTAL_J"
                CHANGED=1
            fi
        fi
        if [ -f /var/log/installer/subiquity-server-debug.log ]; then
            TOTAL_S=$(wc -l < /var/log/installer/subiquity-server-debug.log 2>/dev/null || echo 0)
            if [ "${TOTAL_S:-0}" -gt "$LAST_SUB_LINES" ]; then
                NEW_S=$((TOTAL_S - LAST_SUB_LINES))
                echo "--- [SUBIQUITY DEBUG +${NEW_S} lines @ $(date -u '+%H:%M:%S')] ---"
                tail -n "$NEW_S" /var/log/installer/subiquity-server-debug.log 2>/dev/null | tail -n 80 || true
                LAST_SUB_LINES="$TOTAL_S"
                CHANGED=1
            fi
        fi
        if [ -f /var/log/installer/curtin-install.log ]; then
            TOTAL_C=$(wc -l < /var/log/installer/curtin-install.log 2>/dev/null || echo 0)
            if [ "${TOTAL_C:-0}" -gt "$LAST_CUR_LINES" ]; then
                NEW_C=$((TOTAL_C - LAST_CUR_LINES))
                echo "--- [CURTIN INSTALL +${NEW_C} lines @ $(date -u '+%H:%M:%S')] ---"
                tail -n "$NEW_C" /var/log/installer/curtin-install.log 2>/dev/null | tail -n 80 || true
                LAST_CUR_LINES="$TOTAL_C"
                CHANGED=1
            fi
        fi
    } >> "$LOG_RAM" 2>/dev/null || true

    if [ -d /target/var/log/instant-server/install ]; then
        cp -f "$LOG_RAM" /target/var/log/instant-server/install/instant_server_install.log 2>/dev/null || true
    fi

    UDEV=$(blkid -L INSTANTUSB 2>/dev/null || true)
    if [ -n "$UDEV" ] && [ ! -f /run/instant-usb-flush.lock ]; then
        if mount -o rw,sync,flush,noatime "$UDEV" /mnt/usb_logs 2>/dev/null; then
            mkdir -p /mnt/usb_logs/instant_server_logs 2>/dev/null || true
            cp -f "$LOG_RAM" /mnt/usb_logs/instant_server_logs/instant_server_install.log 2>/dev/null || true
            [ -f /var/log/installer/subiquity-server-debug.log ] && cp -f /var/log/installer/subiquity-server-debug.log /mnt/usb_logs/instant_server_logs/subiquity_debug.log 2>/dev/null || true
            [ -f /var/log/installer/curtin-install.log ] && cp -f /var/log/installer/curtin-install.log /mnt/usb_logs/instant_server_logs/curtin-install.log 2>/dev/null || true
            sync
            umount /mnt/usb_logs 2>/dev/null || true
        fi
    fi
    sleep 10
done
`

const scriptDumpErrorLogsSh = `#!/usr/bin/env bash
set -uo pipefail
STATUS="${1:-ERROR}"
touch /run/instant-usb-flush.lock 2>/dev/null || true
kill $(cat /run/instant-usb-log-daemon.pid 2>/dev/null) 2>/dev/null || true
mkdir -p /tmp/instant_server_logs /mnt/usb_logs

LOG_RAM="/tmp/instant_server_logs/instant_server_install.log"
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
} >> "$LOG_RAM" 2>/dev/null || true

if [ -d /target/var/log/instant-server/install ]; then
    cp -f "$LOG_RAM" /target/var/log/instant-server/install/instant_server_install.log 2>/dev/null || true
    cp -f /var/log/installer/subiquity-server-debug.log /target/var/log/instant-server/install/subiquity_debug.log 2>/dev/null || true
    cp -f /var/log/installer/curtin-install.log /target/var/log/instant-server/install/ 2>/dev/null || true
    cp -f /autoinstall.yaml /target/var/log/instant-server/install/autoinstall_used.yaml 2>/dev/null || true
fi

UDEV=$(blkid -L INSTANTUSB 2>/dev/null || true)
if [ -n "$UDEV" ]; then
    mount -o rw,sync,flush,noatime "$UDEV" /mnt/usb_logs 2>/dev/null || mount -o remount,rw,sync,flush /mnt/usb_logs 2>/dev/null || true
    if mountpoint -q /mnt/usb_logs 2>/dev/null; then
        mkdir -p /mnt/usb_logs/instant_server_logs 2>/dev/null || true
        cp -f "$LOG_RAM" /mnt/usb_logs/instant_server_logs/instant_server_install.log 2>/dev/null || true
        cp -f /var/log/installer/subiquity-server-debug.log /mnt/usb_logs/instant_server_logs/subiquity_debug.log 2>/dev/null || true
        cp -f /var/log/installer/curtin-install.log /mnt/usb_logs/instant_server_logs/ 2>/dev/null || true
        cp -f /autoinstall.yaml /mnt/usb_logs/instant_server_logs/autoinstall_used.yaml 2>/dev/null || true
        sync
        umount /mnt/usb_logs 2>/dev/null || mount -o remount,ro /mnt/usb_logs 2>/dev/null || true
    fi
fi
umount /mnt/cidata 2>/dev/null || true
sync

# Verify & clear FAT dirty bits ONLY on unmounted USB partitions (CIDATA/INSTANTUSB),
# updating BOTH FAT1 and FAT2 so Windows fastfat.sys never flags "Scan and Fix".
for pdev in "$UDEV" "$(blkid -L CIDATA 2>/dev/null || blkid -L cidata 2>/dev/null || true)"; do
    [ -n "$pdev" ] && [ -b "$pdev" ] || continue
    findmnt -n "$pdev" >/dev/null 2>&1 && continue
    if command -v fsck.vfat >/dev/null 2>&1; then
        fsck.vfat -a -w "$pdev" >/dev/null 2>&1 || true
    fi
done
python3 - << 'PYCLEAN' 2>/dev/null || true
import struct, subprocess
for label in ("CIDATA", "INSTANTUSB"):
    dev = subprocess.run(["blkid", "-L", label], capture_output=True, text=True).stdout.strip()
    if not dev or subprocess.run(["findmnt", "-n", dev], stdout=subprocess.DEVNULL).returncode == 0:
        continue
    try:
        with open(dev, "r+b") as f:
            bpb = bytearray(f.read(512))
            if len(bpb) != 512:
                continue
            bps = struct.unpack_from("<H", bpb, 11)[0] or 512
            res = struct.unpack_from("<H", bpb, 14)[0]
            nfats = bpb[16] or 2
            if bpb[54:62] == b"FAT16   ":
                spf = struct.unpack_from("<H", bpb, 22)[0]
                bpb[37] = 0x00
                f.seek(0); f.write(bpb)
                for idx in range(nfats):
                    f.seek((res + idx * spf) * bps + 2)
                    cur = struct.unpack("<H", f.read(2))[0]
                    f.seek((res + idx * spf) * bps + 2)
                    f.write(struct.pack("<H", cur | 0xC000))
            elif bpb[82:90] == b"FAT32   ":
                spf = struct.unpack_from("<I", bpb, 36)[0]
                bpb[65] = 0x00
                f.seek(0); f.write(bpb)
                f.seek(6 * bps)
                b6 = bytearray(f.read(512))
                if len(b6) == 512 and b6[82:90] == b"FAT32   ":
                    b6[65] = 0x00
                    f.seek(6 * bps); f.write(b6)
                for idx in range(nfats):
                    f.seek((res + idx * spf) * bps + 4)
                    cur = struct.unpack("<I", f.read(4))[0]
                    f.seek((res + idx * spf) * bps + 4)
                    f.write(struct.pack("<I", cur | 0x0C000000))
    except Exception:
        pass
PYCLEAN
sync
if [ "$STATUS" = "ERROR" ]; then
    printf "\n[INSTANT SERVER] Installation error logs saved to instant_server_install.log on your USB drive.\n" > /dev/console 2>/dev/null || true
    printf "\n[INSTANT SERVER] Installation error logs saved to instant_server_install.log on your USB drive.\n" > /dev/tty1 2>/dev/null || true
fi
exit 0
`
