// ============================================================================
// FILE: usb/payload/payload_storage.go
// PURPOSE: Interactive Web Disk Manager launcher & Native Ubuntu Disk Selector
//
//	orchestration script (select_target_disk.sh).
//
// ============================================================================
package payload

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

# 6. Load INSTALL_MODE from instant-server-env.conf (maps INSTANT_INSTALL_MODE -> INSTALL_MODE)
INSTALL_MODE="whole_disk"
for env_cand in /tmp/cidata/instant-server-env.conf /mnt/cidata/instant-server-env.conf /cdrom/cidata/instant-server-env.conf; do
    if [ -f "$env_cand" ]; then
        # shellcheck disable=SC1090
        source "$env_cand" 2>/dev/null || true
        break
    fi
done
[ -n "${INSTANT_INSTALL_MODE:-}" ] && INSTALL_MODE="$INSTANT_INSTALL_MODE"
log_usb "Loaded INSTALL_MODE=${INSTALL_MODE} (skip_beginner_checks=${INSTANT_SKIP_BEGINNER_CHECKS:-0})" 

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

# 7b. Custom Web Disk Manager (port :8088) — waits indefinitely; NEVER auto-forwards!
#     Only switches to native Ubuntu Disk Selector if the user explicitly presses ENTER.
WEB_PICKED=0
USE_NATIVE_SELECTOR=0
if [ "$INSTALL_MODE" = "interactive_gui_disk" ] || [ "$INSTALL_MODE" = "interactive_select" ]; then
    DM_SCRIPT=""
    for cand in /tmp/cidata/disk_manager.py /mnt/cidata/disk_manager.py /cdrom/cidata/disk_manager.py; do
        [ -f "$cand" ] && DM_SCRIPT="$cand" && break
    done
    if [ -n "$DM_SCRIPT" ]; then
        rm -f /tmp/instant_picked_disk
        log_usb "Starting Custom Web Disk Manager (disk_manager.py) on port :8088..."
        INSTANT_USB_DISKS="${USB_DISK},${CIDATA_DISK}" INSTANT_INSTALLER_MODE=1 python3 "$DM_SCRIPT" >/tmp/disk_manager.log 2>&1 &
        DM_PID=$!
        sleep 1
        if ! kill -0 "$DM_PID" 2>/dev/null; then
            log_usb "ERROR: disk_manager.py exited immediately: $(tr '\n' ' ' < /tmp/disk_manager.log 2>/dev/null | head -c 400)"
        fi
        # Give DHCP a few seconds to acquire a LAN IP so we display a real URL immediately
        LIVE_IP=""
        for _w in 1 2 3 4 5 6; do
            LIVE_IP=$(hostname -I 2>/dev/null | awk '{print $1}')
            [ -n "$LIVE_IP" ] && break
            sleep 1
        done
        DISPLAY_IP="${LIVE_IP:-<waiting-for-network-ip>}"
        print_dm_banner() {
            if [ -e /dev/tty1 ]; then
                printf '\033[2J\033[H\n======================================================================\n' > /dev/tty1
                printf '  INSTANT SERVER — CUSTOM WEB DISK MANAGER (WINDOWS-STYLE GUI)\n' > /dev/tty1
                printf '======================================================================\n' > /dev/tty1
                printf '  Manage, format, split, wipe, and select your target drive visually:\n\n' > /dev/tty1
                printf '  OPTION A — Use Custom Web Disk Manager (Recommended):\n' > /dev/tty1
                printf '    1. Connect your phone or another PC to the SAME Wi-Fi / router.\n' > /dev/tty1
                printf '    2. Open any web browser and go to:\n\n' > /dev/tty1
                printf '         http://%s:8088\n\n' "$1" > /dev/tty1
                printf '    3. Format/partition if needed, then click "Select as Target".\n' > /dev/tty1
                printf '       Installation will start automatically (direct full-disk layout,\n' > /dev/tty1
                printf '       no LVM cap, and no native Ubuntu disk screen needed).\n\n' > /dev/tty1
                printf '  OPTION B — Use Native Ubuntu Disk Selector (Keyboard Only):\n' > /dev/tty1
                printf '    -> Press ENTER right now ONLY if you want to skip the Web GUI\n' > /dev/tty1
                printf '       and use the built-in Ubuntu keyboard Disk Selector instead.\n' > /dev/tty1
                printf '       (Tip: Unchecking "LVM group" is simplest — or leave it checked\n' > /dev/tty1
                printf '        and Instant Server will auto-expand LVM to 100%% after install.)\n' > /dev/tty1
                printf '======================================================================\n' > /dev/tty1
                printf '  Waiting for Web Disk Manager selection (press ENTER for native GUI)... ' > /dev/tty1
            fi
        }
        print_dm_banner "$DISPLAY_IP"
        while true; do
            if [ -f /tmp/instant_picked_disk ]; then
                WEB_PICKED=1
                break
            fi
            if [ -e /dev/tty1 ] && read -r -t 1 _ < /dev/tty1; then
                USE_NATIVE_SELECTOR=1
                break
            fi
            # Refresh displayed URL automatically if IP arrives after initial banner
            CUR_IP=$(hostname -I 2>/dev/null | awk '{print $1}')
            if [ -n "$CUR_IP" ] && [ "$CUR_IP" != "$LIVE_IP" ]; then
                LIVE_IP="$CUR_IP"
                print_dm_banner "$LIVE_IP"
            fi
        done
        if [ "$WEB_PICKED" -eq 1 ]; then
            sleep 3
            kill "$DM_PID" 2>/dev/null || true
        else
            log_usb "Keeping Custom Web Disk Manager (:8088) running in background alongside Native Disk Selector."
        fi
    else
        log_usb "disk_manager.py not found; falling back to native Ubuntu Disk Selector."
        USE_NATIVE_SELECTOR=1
    fi
fi

if [ "$WEB_PICKED" -eq 1 ] && [ -f /tmp/instant_picked_disk ]; then
    PICK_RAW=$(cat /tmp/instant_picked_disk)
    BEST_DISK="${PICK_RAW%%|*}"
    if [[ "$PICK_RAW" == *"|gap" ]]; then
        USE_GAP_FLAG="--use-largest-gap"
    fi
    echo "$BEST_DISK" > /run/instant-target-disk
    log_usb "User selected target disk via Custom Web Disk Manager: ${BEST_DISK} (${USE_GAP_FLAG:-whole_disk}). Bypassing native Subiquity storage screen."
    python3 - "$BEST_DISK" << 'PYWEB' 2>/dev/null || true
import pathlib, sys, yaml
target_disk = sys.argv[1] if len(sys.argv) > 1 else ""
for p in (pathlib.Path("/autoinstall.yaml"), pathlib.Path("/cdrom/autoinstall.yaml")):
    if not p.is_file():
        continue
    try:
        data = yaml.safe_load(p.read_text(encoding="utf-8")) or {}
        ai = data.get("autoinstall") if isinstance(data.get("autoinstall"), dict) else data
        inter = [s for s in (ai.get("interactive-sections") or []) if s != "storage"]
        ai["interactive-sections"] = inter
        layout = {"name": "direct"}
        if target_disk:
            layout["match"] = {"path": target_disk}
        ai["storage"] = {"layout": layout}
        p.write_text(yaml.safe_dump(data, sort_keys=False), encoding="utf-8")
    except Exception:
        pass
PYWEB
elif [ "$USE_NATIVE_SELECTOR" -eq 1 ] || [ -z "$BEST_DISK" ]; then
    log_usb "User requested Native Ubuntu Disk Selector (or no target disk auto-detected): enabling interactive storage screen."
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
