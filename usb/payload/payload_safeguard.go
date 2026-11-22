// ============================================================================
// FILE: usb/payload/payload_safeguard.go
// PURPOSE: Early-boot safeguard script that detects an existing Instant Server
//
//	installation and asks for 60s Enter confirmation or instant PowerOff.
//
// ============================================================================
package payload

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

[ -n "${INSTANT_INSTALL_MODE:-}" ] && INSTALL_MODE="$INSTANT_INSTALL_MODE"

# Skip safeguard when user enabled "I know what I'm doing" or for updater/repair USBs
if [ "${INSTANT_SKIP_BEGINNER_CHECKS:-0}" = "1" ] || [ "$INSTALL_MODE" = "offline_updater_usb" ] || [ "$INSTALL_MODE" = "repair_rescue_usb" ]; then
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
    # Ensure a short press of the Power button shuts down immediately without delay
    mkdir -p /etc/systemd/logind.conf.d 2>/dev/null || true
    printf '[Login]
HandlePowerKey=poweroff
HandlePowerKeyLongPress=poweroff
' > /etc/systemd/logind.conf.d/01-powerkey-immediate.conf 2>/dev/null || true
    systemctl reload-or-restart systemd-logind 2>/dev/null || true

    printf '\033[2J\033[H\n======================================================================\n' > /dev/tty1
    printf '  SAFEGUARD: INSTANT SERVER IS ALREADY INSTALLED ON THIS COMPUTER!\n' > /dev/tty1
    printf '======================================================================\n' > /dev/tty1
    printf '  Detected existing Instant Server partition: %s\n\n' "$EXISTING_OS_PART" > /dev/tty1
    printf '  -> To TURN OFF right now: Press the POWER BUTTON (no delay).\n' > /dev/tty1
    printf '  -> To PROTECT your server (DEFAULT): Do NOTHING — shuts down in 60s.\n' > /dev/tty1
    printf '     (Then unplug the USB drive and power on to boot your server.)\n\n' > /dev/tty1
    printf '  -> To CONTINUE installing / reinstalling anyway:\n' > /dev/tty1
    printf '     Press ENTER within 60 seconds.\n' > /dev/tty1
    printf '======================================================================\n' > /dev/tty1
    CONFIRMED=0
    for rem in $(seq 60 -1 1); do
        printf '\r  Press ENTER to proceed | Power button to turn off | Auto-off in %2ds... ' "$rem" > /dev/tty1
        if read -r -t 1 _ < /dev/tty1; then
            CONFIRMED=1
            break
        fi
    done
    if [ "$CONFIRMED" -eq 1 ]; then
        printf '\n\n[INSTANT SERVER] Confirmed — continuing installation...\n' > /dev/tty1
        exit 0
    else
        printf '\n\n[INSTANT SERVER] 60 seconds elapsed with no confirmation. Shutting down safely...\n' > /dev/tty1
        sync
        poweroff -f
        exit 0
    fi
fi
exit 0
`
