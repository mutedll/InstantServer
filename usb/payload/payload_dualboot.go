// ============================================================================
// FILE: usb/payload/payload_dualboot.go
// PURPOSE: Dynamic dual-boot storage, reinstall-preserve-data, and offline
//
//	updater helper scripts.
//
// ============================================================================
package payload

const ScriptDynamicDualBootStorageSh = `#!/usr/bin/env bash
# ==============================================================================
# NON-DESTRUCTIVE DUAL-BOOT STORAGE WRAPPER
# Guarantees existing Windows (NTFS/ESP) and Linux (ext4/btrfs/xfs/LVM) partitions
# are NEVER wiped. Delegates partition shrinking, free-gap creation, and Curtin
# preserve:true storage.config generation to dualboot_storage.py.
# ==============================================================================
set -uo pipefail
REQ_GB="${1:-45}"
LOG_A="/mnt/usb_logs/instant_server_logs/instant_server_install.log"
[ ! -d /mnt/usb_logs/instant_server_logs ] && LOG_A="/tmp/instant_server_install.log"
log_usb() {
    printf '[dynamic_dualboot] %s\n' "$*" | tee -a "$LOG_A" 2>/dev/null || true
    if mountpoint -q /mnt/usb_logs 2>/dev/null; then
        cp -f "$LOG_A" /mnt/usb_logs/instant_server_install.log 2>/dev/null || true
        cp -f "$LOG_A" /mnt/usb_logs/instant_server_logs/instant_server_install.log 2>/dev/null || true
    fi
    sync
}

TARGET_DISK=$(cat /run/instant-target-disk 2>/dev/null || true)
if [ -z "$TARGET_DISK" ]; then
    log_usb "No target disk recorded in /run/instant-target-disk; skipping dual-boot builder."
    exit 0
fi

CDROM_PART=$(findmnt -n -o SOURCE /cdrom 2>/dev/null || true)
USB_DISK=""
if [ -n "$CDROM_PART" ]; then
    PK=$(lsblk -no PKNAME "$CDROM_PART" 2>/dev/null | head -n1)
    [ -n "$PK" ] && USB_DISK="/dev/${PK}"
fi
if [ "$TARGET_DISK" = "$USB_DISK" ] || grep -q "^${TARGET_DISK}" /proc/mounts 2>/dev/null; then
    log_usb "Skipping ${TARGET_DISK} because it has active installer mounts."
    exit 0
fi

BUILDER_PY=""
for cand in /tmp/cidata/dualboot_storage.py /mnt/cidata/dualboot_storage.py /cdrom/cidata/dualboot_storage.py; do
    if [ -f "$cand" ]; then
        BUILDER_PY="$cand"
        break
    fi
done

if [ -n "$BUILDER_PY" ] && [ -f "/autoinstall.yaml" ]; then
    log_usb "Running non-destructive dualboot_storage.py on ${TARGET_DISK} (req=${REQ_GB}GB)..."
    python3 "$BUILDER_PY" --disk "$TARGET_DISK" --req-gb "$REQ_GB" --config /autoinstall.yaml --log "$LOG_A" 2>&1 | tee -a "$LOG_A" || true
else
    log_usb "WARNING: dualboot_storage.py not found; falling back to interactive storage selection to protect existing data."
fi
exit 0
`

const DefaultDualBootStoragePy = `#!/usr/bin/env python3
import sys, yaml
path = "/autoinstall.yaml"
for i, a in enumerate(sys.argv):
    if a == "--config" and i + 1 < len(sys.argv):
        path = sys.argv[i + 1]
try:
    with open(path, "r", encoding="utf-8") as f:
        doc = yaml.safe_load(f) or {}
    ai = doc.get("autoinstall", doc)
    inter = list(ai.get("interactive-sections") or [])
    if "storage" not in inter:
        inter.append("storage")
    ai["interactive-sections"] = inter
    with open(path, "w", encoding="utf-8") as f:
        yaml.safe_dump(doc, f, default_flow_style=False, sort_keys=False)
except Exception:
    pass
`

const scriptReinstallPreserveDataSh = `#!/usr/bin/env bash
set -uo pipefail
ACTION="${1:-backup}"
STAGE_BACKUP="/dev/shm/instant-preserved-data"
mkdir -p "$STAGE_BACKUP" /mnt/existing_os
if [ "$ACTION" = "backup" ]; then
    vgchange -ay >/dev/null 2>&1 || true
    AVAIL_KB=$(df -Pk /dev/shm 2>/dev/null | awk 'NR==2 {print $4}' || echo 0)
    for part in $(lsblk -lnpo NAME,FSTYPE | awk '$2 ~ /ext4|btrfs|xfs/ {print $1}'); do
        if mount -o ro "$part" /mnt/existing_os 2>/dev/null; then
            if [ -d "/mnt/existing_os/opt/instant-server" ]; then
                cp -a /mnt/existing_os/opt/instant-server "$STAGE_BACKUP/" 2>/dev/null || true
            fi
            if [ -d "/mnt/existing_os/DATA" ]; then
                DATA_KB=$(du -sk /mnt/existing_os/DATA 2>/dev/null | awk '{print $1}' || echo 0)
                if [ "${AVAIL_KB:-0}" -gt $((DATA_KB + 131072)) ]; then
                    echo "[instant-server] Backing up full /DATA (${DATA_KB} KB) from $part..."
                    cp -a /mnt/existing_os/DATA "$STAGE_BACKUP/" 2>/dev/null || true
                else
                    echo "[instant-server] Backing up critical /DATA/AppData, /DATA/Backups, and /DATA/CustomWorkflows from $part..."
                    mkdir -p "$STAGE_BACKUP/DATA"
                    for sub in AppData Backups CustomWorkflows Shared; do
                        [ -d "/mnt/existing_os/DATA/$sub" ] && cp -a "/mnt/existing_os/DATA/$sub" "$STAGE_BACKUP/DATA/" 2>/dev/null || true
                    done
                fi
            fi
            umount /mnt/existing_os 2>/dev/null || true
        fi
    done
    TARGET_DISK=$(cat /run/instant-target-disk 2>/dev/null || true)
    if [ -n "$TARGET_DISK" ] && ! grep -q "^${TARGET_DISK}" /proc/mounts 2>/dev/null; then
        for vg in $(pvs --noheadings -o pv_name,vg_name 2>/dev/null | awk -v d="$TARGET_DISK" '$1 ~ "^"d && $2 != "" {print $2}' | sort -u); do
            lvremove -ff -y "$vg" >/dev/null 2>&1 || true
            vgremove -ff -y "$vg" >/dev/null 2>&1 || true
        done
        for pv in $(pvs --noheadings -o pv_name 2>/dev/null | awk -v d="$TARGET_DISK" '$1 ~ "^"d {print $1}'); do
            pvremove -ff -y "$pv" >/dev/null 2>&1 || true
        done
        dmsetup remove_all >/dev/null 2>&1 || true
        wipefs -a "$TARGET_DISK" >/dev/null 2>&1 || true
        sgdisk --zap-all "$TARGET_DISK" >/dev/null 2>&1 || true
        partprobe "$TARGET_DISK" 2>/dev/null || true
    fi
elif [ "$ACTION" = "restore" ]; then
    if [ -d "$STAGE_BACKUP/DATA" ]; then
        mkdir -p /target/DATA
        cp -a "$STAGE_BACKUP/DATA/." /target/DATA/ 2>/dev/null || true
        rm -rf "$STAGE_BACKUP" 2>/dev/null || true
        echo "[instant-server] Restored preserved /DATA to reinstalled server."
    fi
fi
exit 0
`

const scriptOfflineUpdaterApplySh = `#!/usr/bin/env bash
set -euo pipefail
SRC_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TARGET_ROOT="/"
if [ -d "/target/opt/instant-server" ]; then
    TARGET_ROOT="/target"
elif [ ! -d "/opt/instant-server" ]; then
    mkdir -p /mnt/target_os
    for part in $(lsblk -lnpo NAME,FSTYPE | awk '$2 ~ /ext4|btrfs|xfs/ {print $1}'); do
        if mount "$part" /mnt/target_os 2>/dev/null; then
            if [ -d "/mnt/target_os/opt/instant-server" ]; then
                TARGET_ROOT="/mnt/target_os"
                break
            fi
            umount /mnt/target_os 2>/dev/null || true
        fi
    done
fi
mkdir -p "${TARGET_ROOT}/opt/instant-server" "${TARGET_ROOT}/DATA/AppData"
cp -rf "${SRC_DIR}/server_dashboard.html" "${TARGET_ROOT}/opt/instant-server/" 2>/dev/null || true
cp -rf "${SRC_DIR}/laptop_hardening.sh" "${TARGET_ROOT}/opt/instant-server/" 2>/dev/null || true
cp -rf "${SRC_DIR}/instant_apps_bundle.sh" "${TARGET_ROOT}/opt/instant-server/" 2>/dev/null || true
if [ -d "${SRC_DIR}/casaos_apps" ]; then
    cp -rf "${SRC_DIR}/casaos_apps/"* "${TARGET_ROOT}/DATA/AppData/" 2>/dev/null || true
fi
if [ "$TARGET_ROOT" = "/" ]; then
    [ -f /opt/instant-server/instant_apps_bundle.sh ] && bash /opt/instant-server/instant_apps_bundle.sh 2>/dev/null || true
    systemctl restart instant-server-dashboard.service 2>/dev/null || true
    systemctl start instant-server-launch-apps.service 2>/dev/null || true
elif [ "$TARGET_ROOT" = "/mnt/target_os" ]; then
    chroot /mnt/target_os /bin/bash /opt/instant-server/instant_apps_bundle.sh 2>/dev/null || true
fi
echo "[instant-server] Offline USB update and incremental apps applied to ${TARGET_ROOT}."
if [ "$TARGET_ROOT" = "/mnt/target_os" ]; then
    umount /mnt/target_os 2>/dev/null || true
fi
if [ -d "/cdrom" ] || grep -q "casper" /proc/cmdline 2>/dev/null; then
    sync
    reboot -f
fi
`
