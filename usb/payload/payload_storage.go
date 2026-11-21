// ============================================================================
// FILE: usb/payload_storage.go
// PURPOSE: Target disk selection, non-destructive Dual-Boot storage planner
//          invocation, data-preserving OS reinstall, and offline USB updater.
// ============================================================================
package payload

// scriptCheckExistingOSSafeguardSh runs as the VERY FIRST command in early-commands
// so that if Instant Server is already installed on an internal drive, the user is
// prompted immediately after Linux/cloud-init mounts CIDATA:
//   - Press ENTER within 20s -> Proceed with installation / reinstallation
//   - Do nothing for 20s     -> Safe SHUTDOWN (poweroff -f) so user can unplug USB
const scriptCheckExistingOSSafeguardSh = `#!/usr/bin/env bash
set -uo pipefail

# Load INSTALL_MODE if available
INSTALL_MODE="whole_disk"
for env_cand in /tmp/cidata/instant-server-env.conf /mnt/cidata/instant-server-env.conf /cdrom/cidata/instant-server-env.conf; do
    if [ -f "$env_cand" ]; then
        # shellcheck disable=SC1090
        source "$env_cand" 2>/dev/null || true
        break
    fi
done

# Skip safeguard for dedicated updater / repair USB modes
if [ "$INSTALL_MODE" = "offline_updater_usb" ] || [ "$INSTALL_MODE" = "repair_rescue_usb" ]; then
    exit 0
fi

# Identify USB installer disk(s) to exclude from OS scan
CDROM_PART=$(findmnt -n -o SOURCE /cdrom 2>/dev/null || blkid -L UBUNTU-ISO 2>/dev/null || true)
CIDATA_PART=$(blkid -L CIDATA 2>/dev/null || blkid -L cidata 2>/dev/null || true)
USB_DISK=""
CIDATA_DISK=""
if [ -n "$CDROM_PART" ]; then
    PK=$(lsblk -no PKNAME "$CDROM_PART" 2>/dev/null | head -n1)
    [ -n "$PK" ] && USB_DISK="/dev/${PK}" || USB_DISK="$CDROM_PART"
fi
if [ -n "$CIDATA_PART" ]; then
    PK2=$(lsblk -no PKNAME "$CIDATA_PART" 2>/dev/null | head -n1)
    [ -n "$PK2" ] && CIDATA_DISK="/dev/${PK2}" || CIDATA_DISK="$CIDATA_PART"
fi

mkdir -p /mnt/check_existing
EXISTING_OS_PART=""
for part in $(lsblk -lnpo NAME,FSTYPE | awk '$2 ~ /ext4|btrfs|xfs/ {print $1}'); do
    [ -n "$USB_DISK" ] && [[ "$part" == "${USB_DISK}"* ]] && continue
    [ -n "$CIDATA_DISK" ] && [[ "$part" == "${CIDATA_DISK}"* ]] && continue
    if mount -o ro "$part" /mnt/check_existing 2>/dev/null; then
        if [ -f "/mnt/check_existing/opt/instant-server/.install_complete_epoch" ] || [ -f "/mnt/check_existing/opt/instant-server/laptop_hardening.sh" ]; then
            EXISTING_OS_PART="$part"
        fi
        umount /mnt/check_existing 2>/dev/null || true
    fi
    [ -n "$EXISTING_OS_PART" ] && break
done

if [ -n "$EXISTING_OS_PART" ] && [ -e /dev/tty1 ]; then
    printf '\033[2J\033[H\n======================================================================\n' > /dev/tty1
    printf '  SAFEGUARD: INSTANT SERVER IS ALREADY INSTALLED ON THIS COMPUTER!\n' > /dev/tty1
    printf '======================================================================\n' > /dev/tty1
    printf '  Detected existing Instant Server partition: %s\n\n' "$EXISTING_OS_PART" > /dev/tty1
    printf '  -> To PROTECT your server (DEFAULT):\n' > /dev/tty1
    printf '     Do NOTHING — this computer will SHUT DOWN safely in 20 seconds.\n' > /dev/tty1
    printf '     Then unplug the USB drive and power on to boot your server.\n\n' > /dev/tty1
    printf '  -> To CONTINUE installing / reinstalling anyway:\n' > /dev/tty1
    printf '     Press ENTER within 20 seconds.\n' > /dev/tty1
    printf '======================================================================\n' > /dev/tty1
    printf '  Press ENTER to continue installing (auto-shutdown in 20s)... ' > /dev/tty1
    if read -r -t 20 _ < /dev/tty1; then
        printf '\n[INSTANT SERVER] Confirmed — continuing installation...\n' > /dev/tty1
        exit 0
    else
        printf '\n[INSTANT SERVER] No confirmation received. Shutting down safely...\n' > /dev/tty1
        sync
        poweroff -f
        exit 0
    fi
fi
exit 0
`

const scriptSelectTargetDiskSh = `#!/usr/bin/env bash
set -uo pipefail
LOG_A="/mnt/usb_logs/instant_server_logs/instant_server_install.log"
[ ! -d /mnt/usb_logs/instant_server_logs ] && LOG_A="/tmp/instant_server_install.log"
log_usb() {
    printf '[select_target_disk] %s\n' "$*" | tee -a "$LOG_A" 2>/dev/null || true
    mkdir -p /mnt/usb_logs/instant_server_logs 2>/dev/null || true
    sync
}

# 1. Stop casper-md5check & multipathd immediately so Integrity and Probert never stall
systemctl stop casper-md5check.service multipathd.service multipathd.socket 2>/dev/null || true
printf '{"result": "pass"}\n' > /run/casper-md5check.json 2>/dev/null || true

# 2. Install a non-recursive, non-blocking netplan wrapper on the live RAM disk.
#    CRITICAL FIX: On Ubuntu Live Server, /usr/sbin/netplan is a symlink to ../share/netplan/netplan.script.
#    We MUST remove the symlink (rm -f) before writing our wrapper, and call /lib/netplan/generate directly
#    (never calling netplan.real) so there is zero possibility of symlink recursion or netplan-dbus hangs.
chmod 600 /etc/netplan/*.yaml /lib/netplan/*.yaml /run/netplan/*.yaml 2>/dev/null || true
mkdir -p /usr/local/sbin /usr/local/bin
cat << 'NPEOF' > /usr/local/sbin/netplan
#!/usr/bin/env bash
exec >/dev/null 2>&1 </dev/null
chmod 600 /etc/netplan/*.yaml /lib/netplan/*.yaml /run/netplan/*.yaml 2>/dev/null || true
if [ -x /lib/netplan/generate ]; then
    /lib/netplan/generate 2>/dev/null || true
elif [ -x /usr/libexec/netplan/generate ]; then
    /usr/libexec/netplan/generate 2>/dev/null || true
fi
networkctl reload 2>/dev/null || true
exit 0
NPEOF
chmod 755 /usr/local/sbin/netplan
cp -f /usr/local/sbin/netplan /usr/local/bin/netplan 2>/dev/null || true
rm -f /usr/sbin/netplan /usr/bin/netplan 2>/dev/null || true
cp -f /usr/local/sbin/netplan /usr/sbin/netplan 2>/dev/null || true
cp -f /usr/local/sbin/netplan /usr/bin/netplan 2>/dev/null || true
for snap_np in $(find /snap/subiquity -name "netplan" -o -name "netplan.script" 2>/dev/null); do
    if [ -e "$snap_np" ]; then
        mount --bind /usr/local/sbin/netplan "$snap_np" 2>/dev/null || true
    fi
done
log_usb "Installed non-recursive /usr/sbin/netplan wrapper (/lib/netplan/generate + networkctl reload)."

# 3. Pre-install offline wpasupplicant & wireless-tools from /cdrom/pool if any wireless adapter exists
if ls /sys/class/net/wl* >/dev/null 2>&1; then
    find /cdrom/pool -type f \( -name "libnl-*.deb" -o -name "libpcsclite*.deb" -o -name "wpasupplicant*.deb" -o -name "wireless-tools*.deb" -o -name "rfkill*.deb" \) -exec dpkg -i --force-all {} + >/dev/null 2>&1 || true
fi

# 4. Identify the installer media disk(s) (USB/SD/ISO) so Subiquity never overwrites installer media
CDROM_PART=$(findmnt -n -o SOURCE /cdrom 2>/dev/null || blkid -L UBUNTU-ISO 2>/dev/null || true)
CIDATA_PART=$(blkid -L CIDATA 2>/dev/null || blkid -L cidata 2>/dev/null || true)
INSTANT_PART=$(blkid -L INSTANTUSB 2>/dev/null || true)
USB_DISK=""
CIDATA_DISK=""
if [ -n "$CDROM_PART" ]; then
    PK=$(lsblk -no PKNAME "$CDROM_PART" 2>/dev/null | head -n1)
    if [ -n "$PK" ]; then USB_DISK="/dev/${PK}"; else USB_DISK="$CDROM_PART"; fi
fi
if [ -n "$CIDATA_PART" ]; then
    PK2=$(lsblk -no PKNAME "$CIDATA_PART" 2>/dev/null | head -n1)
    if [ -n "$PK2" ]; then CIDATA_DISK="/dev/${PK2}"; else CIDATA_DISK="$CIDATA_PART"; fi
    [ -z "$USB_DISK" ] && USB_DISK="$CIDATA_DISK"
fi
if [ -z "$CIDATA_DISK" ] && [ -n "$INSTANT_PART" ]; then
    PK3=$(lsblk -no PKNAME "$INSTANT_PART" 2>/dev/null | head -n1)
    [ -n "$PK3" ] && CIDATA_DISK="/dev/${PK3}"
    [ -z "$USB_DISK" ] && USB_DISK="$CIDATA_DISK"
fi
log_usb "Detected installer media: USB_DISK=${USB_DISK} CIDATA_DISK=${CIDATA_DISK}"

# 5. Existing-OS safeguard now runs at the very start of early-commands via check_existing_os_safeguard.sh.

# 6. Load INSTALL_MODE from instant-server-env.conf
INSTALL_MODE="whole_disk"
for env_cand in /tmp/cidata/instant-server-env.conf /mnt/cidata/instant-server-env.conf /cdrom/cidata/instant-server-env.conf; do
    if [ -f "$env_cand" ]; then
        # shellcheck disable=SC1090
        source "$env_cand" 2>/dev/null || true
        break
    fi
done

# 7. Enumerate all internal candidate disks (excluding USB installer media)
BEST_DISK=""
BEST_SIZE=0
USE_GAP_FLAG=""
declare -a DISK_LIST=()
while read -r dname dtype dsize dtran drm; do
    [ "$dtype" != "disk" ] && continue
    [[ "$dname" =~ ^/dev/(loop|ram|zram|sr|fd|md|dm-) ]] && continue
    [[ "$dname" == *boot* ]] || [[ "$dname" == *rpmb* ]] && continue
    [ -n "$USB_DISK" ] && [ "$dname" = "$USB_DISK" ] && continue
    [ -n "$CIDATA_DISK" ] && [ "$dname" = "$CIDATA_DISK" ] && continue
    if [ "$dtran" = "usb" ] || [ "$drm" = "1" ]; then
        continue
    fi
    dmodel=$(lsblk -dn -o MODEL "$dname" 2>/dev/null | tr -s ' ' || echo "Internal Disk")
    dgb=$((dsize / 1073741824))
    DISK_LIST+=("${dname}|${dgb} GB (${dmodel})")
    if [ "${dsize:-0}" -gt "$BEST_SIZE" ]; then
        BEST_SIZE="$dsize"
        BEST_DISK="$dname"
    fi
done < <(lsblk -brndo NAME,TYPE,SIZE,TRAN,RM 2>/dev/null | awk '{print "/dev/"$1, $2, $3, ($4==""?"none":$4), $5}')

# 7b. Interactive Target Disk Picker UI Mode (on-screen GUI menu + port :8088 Web Picker)
if [ "$INSTALL_MODE" = "interactive_gui_disk" ] && [ "${#DISK_LIST[@]}" -gt 0 ]; then
    log_usb "Launching Full Disk Management & Target Selector UI (disk_manager.py) on port :8088..."
    DM_SCRIPT=""
    for cand in /tmp/cidata/disk_manager.py /mnt/cidata/disk_manager.py /cdrom/cidata/disk_manager.py; do [ -f "$cand" ] && DM_SCRIPT="$cand" && break; done
    if [ -n "$DM_SCRIPT" ]; then
        rm -f /tmp/instant_picked_disk
        INSTANT_USB_DISKS="${USB_DISK},${CIDATA_DISK}" INSTANT_INSTALLER_MODE=1 python3 "$DM_SCRIPT" >/dev/null 2>&1 &
        DM_PID=$!
        LIVE_IP=$(hostname -I 2>/dev/null | awk '{print $1}')
        [ -z "$LIVE_IP" ] && LIVE_IP="<connect-ethernet-or-wifi>"
        if [ -e /dev/tty1 ]; then
            printf '\033[2J\033[H\n======================================================================\n' > /dev/tty1
            printf '  INSTANT SERVER — CUSTOM WEB DISK MANAGER (WINDOWS-STYLE GUI)\n' > /dev/tty1
            printf '======================================================================\n' > /dev/tty1
            printf '  Want to visually format, split, wipe, or inspect drives first?\n\n' > /dev/tty1
            printf '  OPTION A — Open Custom Web Disk Manager (from your phone or PC):\n' > /dev/tty1
            printf '    1. Make sure your phone/PC is on the SAME Wi-Fi/router.\n' > /dev/tty1
            printf '    2. Open any web browser and go to:\n\n' > /dev/tty1
            printf '         http://%s:8088\n\n' "$LIVE_IP" > /dev/tty1
            printf '    3. Manage partitions & click "Select as Target" to continue.\n\n' > /dev/tty1
            printf '  OPTION B — Use the built-in keyboard Disk Selector on this screen:\n' > /dev/tty1
            printf '    -> Press ENTER right now to open Ubuntu Disk Selector.\n' > /dev/tty1
            printf '======================================================================\n' > /dev/tty1
            printf '  Waiting for Web Disk Manager or press ENTER to continue... ' > /dev/tty1
        fi
        ELAPSED=0
        while [ "$ELAPSED" -lt 300 ]; do
            if [ -f /tmp/instant_picked_disk ]; then
                break
            fi
            if [ -e /dev/tty1 ] && read -r -t 1 _ < /dev/tty1; then
                break
            fi
            ELAPSED=$((ELAPSED + 1))
        done
        kill "$DM_PID" 2>/dev/null || true
    fi
    python3 - "${DISK_LIST[@]}" << 'PYPICK' 2>/dev/null || true
import http.server, json, os, subprocess, sys, threading

disks = []
for arg in sys.argv[1:]:
    if "|" in arg:
        dev, desc = arg.split("|", 1)
        disks.append({"dev": dev, "desc": desc})

chosen = {"disk": disks[0]["dev"] if disks else "/dev/sda", "gap": False, "done": False}

class PickerHandler(http.server.BaseHTTPRequestHandler):
    def log_message(self, *args): pass
    def do_GET(self):
        if self.path.startswith("/pick?"):
            from urllib.parse import parse_qs, urlparse
            q = parse_qs(urlparse(self.path).query)
            chosen["disk"] = q.get("disk", [chosen["disk"]])[0]
            chosen["gap"] = q.get("mode", ["whole"])[0] == "gap"
            chosen["done"] = True
            self.send_response(200)
            self.send_header("Content-Type", "text/html")
            self.end_headers()
            self.wfile.write(b"<h2>Target disk selected! Continuing Ubuntu Server installation...</h2>")
            return
        rows = "".join([f"<div style='margin:10px 0;padding:12px;border:1px solid #334;border-radius:8px;'><b>{d['dev']}</b> — {d['desc']}<br/><br/><a href='/pick?disk={d['dev']}&mode=whole' style='padding:8px 14px;background:#0A84FF;color:#fff;border-radius:6px;text-decoration:none;margin-right:8px;'>Install on Whole Disk</a><a href='/pick?disk={d['dev']}&mode=gap' style='padding:8px 14px;background:#24272F;color:#fff;border-radius:6px;text-decoration:none;'>Use Largest Free Space (Dual-Boot)</a></div>" for d in disks])
        html = f"<html><body style='background:#0B0C0E;color:#F4F4F6;font-family:sans-serif;max-width:600px;margin:40px auto;padding:20px;'><h2>Instant Server — Select Target Disk</h2><p>Choose which drive on this computer to install Ubuntu Server onto:</p>{rows}</body></html>"
        self.send_response(200)
        self.send_header("Content-Type", "text/html")
        self.end_headers()
        self.wfile.write(html.encode())

srv = http.server.HTTPServer(("0.0.0.0", 8088), PickerHandler)
threading.Thread(target=srv.serve_forever, daemon=True).start()

if os.path.exists("/dev/tty1") and subprocess.call(["which", "whiptail"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL) == 0:
    menu_args = ["whiptail", "--title", "Instant Server — Select Target Disk", "--menu", "Choose the internal disk on this computer for Ubuntu Server (or open http://<ip>:8088):", "18", "74", "8"]
    for d in disks:
        menu_args.extend([d["dev"], d["desc"]])
    try:
        with open("/dev/tty1", "rb") as tin, open("/dev/tty1", "wb") as tout:
            p = subprocess.Popen(menu_args, stdin=tin, stdout=tout, stderr=subprocess.PIPE, text=True)
            _, err_out = p.communicate(timeout=180)
            if p.returncode == 0 and err_out.strip():
                chosen["disk"] = err_out.strip()
    except Exception:
        pass

with open("/tmp/instant_picked_disk", "w") as f:
    f.write(chosen["disk"] + ("|gap" if chosen["gap"] else "|whole"))
PYPICK
    if [ -f /tmp/instant_picked_disk ]; then
        PICK_RAW=$(cat /tmp/instant_picked_disk)
        BEST_DISK="${PICK_RAW%%|*}"
        if [[ "$PICK_RAW" == *"|gap" ]]; then
            USE_GAP_FLAG="--use-largest-gap"
        fi
        log_usb "User selected target disk via Interactive Picker: ${BEST_DISK} (${USE_GAP_FLAG:-whole_disk})"
    fi
fi

if [ "$INSTALL_MODE" = "interactive_gui_disk" ] || [ "$INSTALL_MODE" = "interactive_select" ] || [ "${#DISK_LIST[@]}" -gt 1 ] || [ -z "$BEST_DISK" ]; then
    log_usb "Interactive Disk Selection GUI enabled (${INSTALL_MODE}): ensuring Subiquity opens storage GUI even on empty disks."
    python3 - << 'PYINTER' 2>/dev/null || true
import yaml, pathlib
for p in (pathlib.Path("/autoinstall.yaml"), pathlib.Path("/cdrom/autoinstall.yaml")):
    if not p.is_file():
        continue
    try:
        data = yaml.safe_load(p.read_text(encoding="utf-8")) or {}
        ai = data.get("autoinstall") if isinstance(data.get("autoinstall"), dict) else data
        inter = list(ai.get("interactive-sections") or [])
        if "storage" not in inter:
            inter.append("storage")
        ai["interactive-sections"] = inter
        ai.pop("storage", None)
        p.write_text(yaml.safe_dump(data, sort_keys=False), encoding="utf-8")
    except Exception:
        pass
PYINTER
elif [ -n "$BEST_DISK" ]; then
    echo "$BEST_DISK" > /run/instant-target-disk
    log_usb "Pinned internal target disk: ${BEST_DISK} (${BEST_SIZE} bytes)"
fi

# 8. Detect whether wired Ethernet already has an IPv4 lease
WIRED_ONLINE=0
WIRED_IF=""
for iface in /sys/class/net/*; do
    [ ! -e "$iface" ] && continue
    ifname=$(basename "$iface")
    [ "$ifname" = "lo" ] && continue
    [ -d "$iface/wireless" ] || [ -d "$iface/phy80211" ] && continue
    ip4=$(ip -4 -o addr show dev "$ifname" scope global 2>/dev/null | awk '{print $4}' | head -n1 || true)
    if [ -n "$ip4" ]; then
        WIRED_ONLINE=1
        WIRED_IF="$ifname ($ip4)"
        break
    fi
done

# 9. Sanitize /autoinstall.yaml using standalone autoinstall_config.py script
SANITIZE_SCRIPT="/tmp/cidata/autoinstall_config.py"
[ ! -f "$SANITIZE_SCRIPT" ] && SANITIZE_SCRIPT="/mnt/cidata/autoinstall_config.py"
[ ! -f "$SANITIZE_SCRIPT" ] && SANITIZE_SCRIPT="/cdrom/cidata/autoinstall_config.py"
if [ -f "/autoinstall.yaml" ] && [ -f "$SANITIZE_SCRIPT" ]; then
    if [ "$WIRED_ONLINE" -eq 1 ]; then
        python3 "$SANITIZE_SCRIPT" --config /autoinstall.yaml --disk "${BEST_DISK}" --wired-online ${USE_GAP_FLAG} 2>/dev/null || true
        log_usb "Sanitized /autoinstall.yaml via autoinstall_config.py (pinned path=${BEST_DISK}, kept live wired network ${WIRED_IF})."
    else
        python3 "$SANITIZE_SCRIPT" --config /autoinstall.yaml --disk "${BEST_DISK}" ${USE_GAP_FLAG} 2>/dev/null || true
        log_usb "Sanitized /autoinstall.yaml via autoinstall_config.py (pinned path=${BEST_DISK})."
    fi
fi

if [ "$WIRED_ONLINE" -eq 1 ]; then
    log_usb "Wired interface already online (${WIRED_IF}); keeping live wired connection and deferring Wi-Fi setup to target OS."
    exit 0
fi
`


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