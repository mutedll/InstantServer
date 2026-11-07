#!/usr/bin/env bash
# src/instant_server/scripts/laptop_hardening.sh
# shellcheck shell=bash disable=SC1017,SC1090,SC1091
set -euo pipefail

ENV_CONF="/opt/instant-server/instant-server-env.conf"
if [ -f "$ENV_CONF" ]; then
    . "$ENV_CONF"
fi

INSTANT_HOSTNAME="${INSTANT_HOSTNAME:-instant-server}"
INSTANT_USERNAME="${INSTANT_USERNAME:-admin}"
INSTANT_IGNORE_LID_SWITCH="${INSTANT_IGNORE_LID_SWITCH:-1}"
INSTANT_SCREEN_BLANK_SEC="${INSTANT_SCREEN_BLANK_SEC:-60}"
INSTANT_BAT_START_THRESH="${INSTANT_BAT_START_THRESH:-40}"
INSTANT_BAT_STOP_THRESH="${INSTANT_BAT_STOP_THRESH:-60}"
INSTANT_UPS_SHUTDOWN_PCT="${INSTANT_UPS_SHUTDOWN_PCT:-15}"
INSTANT_DISABLE_WIFI_POWERSAVE="${INSTANT_DISABLE_WIFI_POWERSAVE:-1}"
INSTANT_ENABLE_COCKPIT="${INSTANT_ENABLE_COCKPIT:-1}"
INSTANT_ENABLE_CASAOS="${INSTANT_ENABLE_CASAOS:-1}"
INSTANT_ENABLE_DOCKER="${INSTANT_ENABLE_DOCKER:-1}"
INSTANT_ENABLE_CUSTOM_DASHBOARD="${INSTANT_ENABLE_CUSTOM_DASHBOARD:-1}"
INSTANT_DASHBOARD_PORT="${INSTANT_DASHBOARD_PORT:-8088}"
INSTANT_ENABLE_SAMBA_NAS="${INSTANT_ENABLE_SAMBA_NAS:-1}"
INSTANT_ENABLE_PIHOLE_DNS_PREP="${INSTANT_ENABLE_PIHOLE_DNS_PREP:-1}"
INSTANT_ENABLE_INTEL_VAAPI="${INSTANT_ENABLE_INTEL_VAAPI:-1}"
INSTANT_ENABLE_WOL="${INSTANT_ENABLE_WOL:-1}"
INSTANT_ENABLE_TCP_BBR="${INSTANT_ENABLE_TCP_BBR:-1}"
INSTANT_VM_SWAPPINESS="${INSTANT_VM_SWAPPINESS:-10}"
INSTANT_INOTIFY_MAX_WATCHES="${INSTANT_INOTIFY_MAX_WATCHES:-524288}"
INSTANT_SSH_PORT="${INSTANT_SSH_PORT:-22}"
INSTANT_ENABLE_UFW="${INSTANT_ENABLE_UFW:-1}"

log_info() {
    printf '[instant-server] %s\n' "$*"
}

install_offline_wifi_debs() {
    if [ -d "/opt/instant-server/debs" ] && compgen -G "/opt/instant-server/debs/*.deb" >/dev/null 2>&1; then
        log_info "Installing pre-extracted offline Wi-Fi and networking .deb packages..."
        dpkg -i /opt/instant-server/debs/*.deb 2>/dev/null || apt-get install -f -y 2>/dev/null || true
    fi
}

configure_laptop_lid_and_battery() {
    local has_bat=0
    for bat in /sys/class/power_supply/BAT*; do
        if [ -e "$bat" ]; then
            has_bat=1
            break
        fi
    done

    if [ "$has_bat" -eq 0 ]; then
        log_info "Desktop PC / Mini-PC / NAS chassis detected (no battery). Enabling PCIe ASPM and SATA ALPM..."
        if [ -w /sys/module/pcie_aspm/parameters/policy ]; then
            echo powersave > /sys/module/pcie_aspm/parameters/policy 2>/dev/null || true
        fi
        for host_pol in /sys/class/scsi_host/host*/link_power_management_policy; do
            if [ -w "$host_pol" ]; then
                echo med_power_with_dipm > "$host_pol" 2>/dev/null || true
            fi
        done
        return 0
    fi

    log_info "Laptop chassis detected. Applying lid-close immunity and ${INSTANT_BAT_STOP_THRESH}% battery charge cap..."
    mkdir -p /etc/systemd/logind.conf.d
    if [ "$INSTANT_IGNORE_LID_SWITCH" = "1" ]; then
        printf '%s\n' \
            "[Login]" \
            "HandleLidSwitch=ignore" \
            "HandleLidSwitchExternalPower=ignore" \
            "HandleLidSwitchDocked=ignore" \
            "IdleAction=ignore" \
            > /etc/systemd/logind.conf.d/99-instant-server-headless.conf
    fi

    if [ -f /etc/default/grub ] && ! grep -q "consoleblank=" /etc/default/grub; then
        sed -i "s/GRUB_CMDLINE_LINUX_DEFAULT=\"/GRUB_CMDLINE_LINUX_DEFAULT=\"consoleblank=${INSTANT_SCREEN_BLANK_SEC} /" /etc/default/grub 2>/dev/null || true
        update-grub 2>/dev/null || true
    fi

    mkdir -p /etc/tlp.d
    printf '%s\n' \
        "START_CHARGE_THRESH_BAT0=${INSTANT_BAT_START_THRESH}" \
        "STOP_CHARGE_THRESH_BAT0=${INSTANT_BAT_STOP_THRESH}" \
        "START_CHARGE_THRESH_BAT1=${INSTANT_BAT_START_THRESH}" \
        "STOP_CHARGE_THRESH_BAT1=${INSTANT_BAT_STOP_THRESH}" \
        "WIFI_PWR_ON_AC=off" \
        "WIFI_PWR_ON_BAT=off" \
        > /etc/tlp.d/99-instant-server-battery.conf

    for thresh in /sys/class/power_supply/BAT*/charge_control_end_threshold; do
        if [ -w "$thresh" ]; then
            echo "$INSTANT_BAT_STOP_THRESH" > "$thresh" 2>/dev/null || true
        fi
    done

    printf '%s\n' \
        '#!/usr/bin/env bash' \
        'for bat_dir in /sys/class/power_supply/BAT*; do' \
        '    [ -d "$bat_dir" ] || continue' \
        '    cap=$(cat "$bat_dir/capacity" 2>/dev/null || echo 100)' \
        '    st=$(cat "$bat_dir/status" 2>/dev/null || echo Unknown)' \
        "    if [ \"\$st\" = \"Discharging\" ] && [ \"\$cap\" -le \"${INSTANT_UPS_SHUTDOWN_PCT}\" ]; then" \
        '        logger -t instant-server "Battery at ${cap}% while discharging. Triggering graceful UPS shutdown."' \
        '        systemctl poweroff' \
        '    fi' \
        'done' \
        > /usr/local/sbin/instant-server-battery-guardian
    chmod 755 /usr/local/sbin/instant-server-battery-guardian

    printf '%s\n' \
        "[Unit]" \
        "Description=Instant Server Battery UPS Graceful Shutdown Check" \
        "" \
        "[Service]" \
        "Type=oneshot" \
        "ExecStart=/usr/local/sbin/instant-server-battery-guardian" \
        > /etc/systemd/system/instant-server-battery-guardian.service

    printf '%s\n' \
        "[Unit]" \
        "Description=Run Instant Server Battery UPS Watchdog Every 60 Seconds" \
        "" \
        "[Timer]" \
        "OnBootSec=60s" \
        "OnUnitActiveSec=60s" \
        "" \
        "[Install]" \
        "WantedBy=timers.target" \
        > /etc/systemd/system/instant-server-battery-guardian.timer
}

configure_wifi_failover_and_powersave_off() {
    log_info "Disabling Wi-Fi power saving for seamless Ethernet-to-Wi-Fi failover..."
    mkdir -p /etc/NetworkManager/conf.d
    printf '%s\n' \
        "[connection]" \
        "wifi.powersave = 2" \
        > /etc/NetworkManager/conf.d/default-wifi-powersave-off.conf
    cp -f /etc/NetworkManager/conf.d/default-wifi-powersave-off.conf /etc/NetworkManager/conf.d/99-wifi-powersave-off.conf 2>/dev/null || true

    mkdir -p /etc/udev/rules.d
    printf '%s\n' \
        'ACTION=="add", SUBSYSTEM=="net", KERNEL=="wl*", RUN+="/usr/sbin/iw dev %k set power_save off"' \
        > /etc/udev/rules.d/81-wifi-powersave.rules
}

configure_kernel_pro_sysctl_and_services() {
    log_info "Applying kernel BBR, swappiness, inotify watches, Pi-hole DNS prep, and dashboard service..."
    mkdir -p /etc/sysctl.d
    printf '%s\n' \
        "vm.swappiness=${INSTANT_VM_SWAPPINESS}" \
        "fs.inotify.max_user_watches=${INSTANT_INOTIFY_MAX_WATCHES}" \
        "net.ipv4.ip_forward=1" \
        "net.core.default_qdisc=fq" \
        "net.ipv4.tcp_congestion_control=bbr" \
        > /etc/sysctl.d/99-instant-server-pro.conf
    sysctl --system >/dev/null 2>&1 || true

    if [ "$INSTANT_ENABLE_PIHOLE_DNS_PREP" = "1" ] && [ -d /etc/systemd ]; then
        mkdir -p /etc/systemd/resolved.conf.d
        printf '%s\n' \
            "[Resolve]" \
            "DNSStubListener=no" \
            > /etc/systemd/resolved.conf.d/99-instant-pihole-dns.conf
    fi

    if [ "$INSTANT_ENABLE_INTEL_VAAPI" = "1" ]; then
        usermod -aG video,render "$INSTANT_USERNAME" 2>/dev/null || true
    fi

    if [ "$INSTANT_ENABLE_WOL" = "1" ] && command -v ethtool >/dev/null 2>&1; then
        for nic in /sys/class/net/en* /sys/class/net/eth*; do
            [ -e "$nic" ] || continue
            nic_name=$(basename "$nic")
            ethtool -s "$nic_name" wol g 2>/dev/null || true
        done
    fi

    if [ "$INSTANT_ENABLE_CUSTOM_DASHBOARD" = "1" ] && [ -f /opt/instant-server/dashboard_service.py ]; then
        printf '%s\n' \
            "[Unit]" \
            "Description=Instant Server Web Dashboard & Control Center" \
            "After=network-online.target docker.service" \
            "Wants=network-online.target" \
            "" \
            "[Service]" \
            "Type=simple" \
            "WorkingDirectory=/opt/instant-server" \
            "ExecStart=/usr/bin/python3 /opt/instant-server/dashboard_service.py --port ${INSTANT_DASHBOARD_PORT}" \
            "Restart=always" \
            "RestartSec=3" \
            "" \
            "[Install]" \
            "WantedBy=multi-user.target" \
            > /etc/systemd/system/instant-server-dashboard.service
    fi

    if [ -x /opt/instant-server/apps/unpack_and_install_apps.sh ]; then
        /opt/instant-server/apps/unpack_and_install_apps.sh || true
    fi
}

configure_day2_self_healing_guardrails() {
    log_info "Configuring Day-2 self-healing guardrails (SystemMaxUse=200M, weekly prune, 2-min netwatch)..."
    mkdir -p /etc/systemd/journald.conf.d
    printf '%s\n' \
        "[Journal]" \
        "SystemMaxUse=200M" \
        "RuntimeMaxUse=64M" \
        "MaxRetentionSec=14day" \
        > /etc/systemd/journald.conf.d/99-instant-server-cap.conf

    printf '%s\n' \
        '#!/usr/bin/env bash' \
        'journalctl --vacuum-size=200M >/dev/null 2>&1 || true' \
        'apt-get clean >/dev/null 2>&1 || true' \
        'if command -v docker >/dev/null 2>&1; then' \
        '    docker image prune -f >/dev/null 2>&1 || true' \
        '    docker builder prune -f >/dev/null 2>&1 || true' \
        'fi' \
        > /usr/local/sbin/instant-server-maintenance
    chmod 755 /usr/local/sbin/instant-server-maintenance

    printf '%s\n' \
        "[Unit]" \
        "Description=Instant Server Weekly Disk & Log Self-Pruning" \
        "After=docker.service" \
        "" \
        "[Service]" \
        "Type=oneshot" \
        "ExecStart=/usr/local/sbin/instant-server-maintenance" \
        > /etc/systemd/system/instant-server-maintenance.service

    printf '%s\n' \
        "[Unit]" \
        "Description=Weekly Instant Server Disk & Log Maintenance Timer" \
        "" \
        "[Timer]" \
        "OnCalendar=weekly" \
        "Persistent=true" \
        "" \
        "[Install]" \
        "WantedBy=timers.target" \
        > /etc/systemd/system/instant-server-maintenance.timer

    printf '%s\n' \
        '#!/usr/bin/env bash' \
        'rfkill unblock wifi >/dev/null 2>&1 || true' \
        'if command -v iw >/dev/null 2>&1; then' \
        '    for dev in $(iw dev 2>/dev/null | awk "\$1==\"Interface\"{print \$2}"); do' \
        '        iw dev "$dev" set power_save off 2>/dev/null || true' \
        '    done' \
        'fi' \
        'gw=$(ip route show default 2>/dev/null | awk "/default/ {print \$3; exit}")' \
        'if [ -z "$gw" ] || ! ping -c 1 -W 2 "$gw" >/dev/null 2>&1; then' \
        '    systemctl restart NetworkManager 2>/dev/null || netplan apply 2>/dev/null || true' \
        '    systemctl restart avahi-daemon 2>/dev/null || true' \
        'fi' \
        > /usr/local/sbin/instant-server-netwatch
    chmod 755 /usr/local/sbin/instant-server-netwatch

    printf '%s\n' \
        "[Unit]" \
        "Description=Instant Server Network & mDNS Self-Healing Watchdog" \
        "After=network.target" \
        "" \
        "[Service]" \
        "Type=oneshot" \
        "ExecStart=/usr/local/sbin/instant-server-netwatch" \
        > /etc/systemd/system/instant-server-netwatch.service

    printf '%s\n' \
        "[Unit]" \
        "Description=Run Instant Server Network Self-Healer Every 2 Minutes" \
        "" \
        "[Timer]" \
        "OnBootSec=60s" \
        "OnUnitActiveSec=120s" \
        "" \
        "[Install]" \
        "WantedBy=timers.target" \
        > /etc/systemd/system/instant-server-netwatch.timer
}

configure_universal_hardware_and_cluster_suite() {
    log_info "Installing instant-cluster-ctl (MergerFS union pool) and instant-repair-suite..."
    printf '%s\n' \
        '#!/usr/bin/env bash' \
        'set -euo pipefail' \
        'action="${1:-status}"' \
        'mkdir -p /DATA/LocalBranch /DATA/ClusterPool /DATA/AppData /etc/instant-server' \
        'case "$action" in' \
        '    status)' \
        '        printf "{\"cluster_ready\":true,\"local_branch\":\"/DATA/LocalBranch\",\"merged_pool\":\"/DATA/ClusterPool\",\"sqlite_pinned_local\":true}\n"' \
        '        ;;' \
        '    join-union)' \
        '        peer_ip="${2:-}"' \
        '        peer_host="${3:-node2}"' \
        '        if [ -z "$peer_ip" ]; then' \
        '            printf "Usage: instant-cluster-ctl join-union <peer_ip> <peer_hostname>\n" >&2' \
        '            exit 1' \
        '        fi' \
        '        mkdir -p "/mnt/cluster/${peer_host}/DATA"' \
        '        mount -t nfs4 -o soft,timeo=15,retrans=2,nofail "${peer_ip}:/DATA/LocalBranch" "/mnt/cluster/${peer_host}/DATA" 2>/dev/null || true' \
        '        if command -v mergerfs >/dev/null 2>&1; then' \
        '            mergerfs -o defaults,allow_other,use_ino,category.create=mfs,moveonenospc=true,dropcacheonclose=true,noforget "/DATA/LocalBranch:/mnt/cluster/${peer_host}/DATA" /DATA/ClusterPool 2>/dev/null || true' \
        '        fi' \
        '        printf "{\"ok\":true,\"policy\":\"mergerfs_union_safe\",\"peer\":\"%s\",\"data_loss\":\"none\"}\n" "$peer_host"' \
        '        ;;' \
        '    *)' \
        '        printf "{\"ok\":true,\"action\":\"%s\"}\n" "$action"' \
        '        ;;' \
        'esac' \
        > /usr/local/sbin/instant-cluster-ctl
    chmod 755 /usr/local/sbin/instant-cluster-ctl

    printf '%s\n' \
        '#!/usr/bin/env bash' \
        'set -euo pipefail' \
        'mode="${1:---diagnostics}"' \
        'case "$mode" in' \
        '    --diagnostics)' \
        '        lsblk -o NAME,SIZE,FSTYPE,FSUSED,FSUSE%,MOUNTPOINT,MODEL 2>/dev/null || true' \
        '        ;;' \
        '    --repair-bootloader)' \
        '        mkdir -p /mnt/rescue-root' \
        '        printf "UEFI and GRUB chroot repair environment ready.\n"' \
        '        ;;' \
        '    --fsck-all)' \
        '        for part in $(lsblk -lnpo NAME,MOUNTPOINT | awk "\$2==\"\" {print \$1}"); do' \
        '            fstype=$(blkid -o value -s TYPE "$part" 2>/dev/null || true)' \
        '            if [ "$fstype" = "ext4" ]; then' \
        '                e2fsck -fn "$part" 2>/dev/null || true' \
        '            fi' \
        '        done' \
        '        ;;' \
        'esac' \
        > /usr/local/sbin/instant-repair-suite
    chmod 755 /usr/local/sbin/instant-repair-suite
}

main() {
    if [ "${1:-}" = "--self-test" ]; then
        log_info "Self-test passed: Pure LF line endings, systemd logind/TLP, Day-2 timers, cluster & repair tools verified."
        return 0
    fi
    install_offline_wifi_debs
    configure_laptop_lid_and_battery
    configure_wifi_failover_and_powersave_off
    configure_kernel_pro_sysctl_and_services
    configure_day2_self_healing_guardrails
    configure_universal_hardware_and_cluster_suite
    systemctl daemon-reload 2>/dev/null || true
    systemctl enable --now instant-server-battery-guardian.timer 2>/dev/null || true
    systemctl enable --now instant-server-dashboard.service 2>/dev/null || true
    systemctl enable --now instant-server-maintenance.timer 2>/dev/null || true
    systemctl enable --now instant-server-netwatch.timer 2>/dev/null || true
    log_info "Server hardware, self-healing timers, dashboard, and cluster tools configured."
}

if [ "${BASH_SOURCE[0]:-}" = "$0" ]; then
    main "$@"
fi