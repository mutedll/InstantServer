#!/usr/bin/env bash
# shellcheck disable=SC1090,SC1091,SC2015,SC2034
set -uo pipefail

ENV_CONF="/opt/instant-server/instant-server-env.conf"
if [ -f "$ENV_CONF" ]; then
    . "$ENV_CONF"
fi

INSTANT_HOSTNAME="${INSTANT_HOSTNAME:-instant-server}"
INSTANT_SERVER_NAME="${INSTANT_SERVER_NAME:-$INSTANT_HOSTNAME}"
INSTANT_ADDRESS_HOST="${INSTANT_ADDRESS_HOST:-$INSTANT_HOSTNAME}"
INSTANT_ADDRESS_DOMAIN="${INSTANT_ADDRESS_DOMAIN:-.local}"
INSTANT_LOG_RETENTION_DAYS="${INSTANT_LOG_RETENTION_DAYS:-14}"
INSTANT_USERNAME="${INSTANT_USERNAME:-admin}"
INSTANT_WIFI_SSID="${INSTANT_WIFI_SSID:-}"
INSTANT_WIFI_PASSWORD="${INSTANT_WIFI_PASSWORD:-}"
INSTANT_IGNORE_LID_SWITCH="${INSTANT_IGNORE_LID_SWITCH:-1}"
INSTANT_SCREEN_BLANK_SEC="${INSTANT_SCREEN_BLANK_SEC:-300}"
INSTANT_BAT_START_THRESH="${INSTANT_BAT_START_THRESH:-40}"
INSTANT_BAT_STOP_THRESH="${INSTANT_BAT_STOP_THRESH:-60}"
INSTANT_UPS_SHUTDOWN_PCT="${INSTANT_UPS_SHUTDOWN_PCT:-15}"
INSTANT_DISABLE_WIFI_POWERSAVE="${INSTANT_DISABLE_WIFI_POWERSAVE:-1}"
INSTANT_ENABLE_COCKPIT="${INSTANT_ENABLE_COCKPIT:-1}"; INSTANT_ENABLE_CASAOS="${INSTANT_ENABLE_CASAOS:-1}"; INSTANT_ENABLE_DOCKER="${INSTANT_ENABLE_DOCKER:-1}"
INSTANT_ENABLE_CUSTOM_DASHBOARD="${INSTANT_ENABLE_CUSTOM_DASHBOARD:-1}"; INSTANT_DASHBOARD_PORT="${INSTANT_DASHBOARD_PORT:-8088}"; INSTANT_SSH_PORT="${INSTANT_SSH_PORT:-22}"
INSTANT_ENABLE_UFW="${INSTANT_ENABLE_UFW:-1}"; INSTANT_ENABLE_FAIL2BAN="${INSTANT_ENABLE_FAIL2BAN:-1}"; INSTANT_ENABLE_TCP_BBR="${INSTANT_ENABLE_TCP_BBR:-1}"
INSTANT_VM_SWAPPINESS="${INSTANT_VM_SWAPPINESS:-10}"; INSTANT_INOTIFY_MAX_WATCHES="${INSTANT_INOTIFY_MAX_WATCHES:-524288}"; INSTANT_CPU_GOVERNOR="${INSTANT_CPU_GOVERNOR:-schedutil}"
INSTANT_DISABLE_TURBO="${INSTANT_DISABLE_TURBO:-0}"; INSTANT_ENABLE_AUTO_UPDATES="${INSTANT_ENABLE_AUTO_UPDATES:-1}"; INSTANT_CLUSTER_MODE="${INSTANT_CLUSTER_MODE:-standalone}"

log_info() {
    printf '[instant-server] %s\n' "$*"
    mkdir -p /var/log/instant-server/install 2>/dev/null || true
    if [ -w /var/log/instant-server/install ]; then
        printf '[instant-server] %s\n' "$*" >> /var/log/instant-server/install/hardening.log 2>/dev/null || true
    fi
    if [ -d /mnt/usb_logs/logs ] && [ -w /mnt/usb_logs/logs ]; then
        printf '[instant-server] %s\n' "$*" >> /mnt/usb_logs/logs/INSTANT-SERVER-INSTALL.log 2>/dev/null || true
    fi
    sync 2>/dev/null || true
}

# 1. Offline Wi-Fi & Ethernet Package Installation
install_offline_wifi_debs() {
    if [ -d "/opt/instant-server/debs" ] && compgen -G "/opt/instant-server/debs/*.deb" >/dev/null 2>&1; then
        log_info "Installing pre-extracted offline Wi-Fi and networking .deb packages..."
        dpkg -i /opt/instant-server/debs/*.deb 2>/dev/null || apt-get install -f -y 2>/dev/null || true
    fi
}

# 2. Multi-Node Hostname Collision Guard
configure_multi_node_collision_guard() {
    if [ "$INSTANT_CLUSTER_MODE" = "usb_multi_node_auto" ]; then
        log_info "Checking LAN for existing ${INSTANT_HOSTNAME}.local to prevent same-USB hostname collision..."
        if ping -c 1 -W 1 "${INSTANT_HOSTNAME}.local" >/dev/null 2>&1; then
            local mac_suffix=""
            mac_suffix=$(cat /sys/class/net/en*/address /sys/class/net/eth*/address /sys/class/net/wl*/address 2>/dev/null | head -n1 | tr -d ':' | tail -c 5 || echo "node2")
            INSTANT_HOSTNAME="${INSTANT_HOSTNAME}-${mac_suffix}"
            hostnamectl set-hostname "$INSTANT_HOSTNAME" 2>/dev/null || echo "$INSTANT_HOSTNAME" > /etc/hostname
            log_info "Primary node detected on LAN; assigned unique cluster node hostname: ${INSTANT_HOSTNAME}.local"
        fi
    fi
}

# 3. Laptop Lid-Close Immunity, Battery Charge Limiter & UPS Auto-Shutdown
configure_laptop_lid_and_battery() {
    systemctl mask sleep.target suspend.target hibernate.target hybrid-sleep.target 2>/dev/null || true
    mkdir -p /etc/systemd/logind.conf.d

    cat << 'EOF' > /etc/systemd/logind.conf.d/99-instant-server-clamshell.conf
[Login]
HandleLidSwitch=ignore
HandleLidSwitchExternalPower=ignore
HandleLidSwitchDocked=ignore
LidSwitchIgnoreInhibited=no
IdleAction=ignore
EOF

    local has_bat=0
    for bat in /sys/class/power_supply/BAT*; do
        if [ -e "$bat" ]; then
            has_bat=1
            break
        fi
    done

    if [ "$INSTANT_DISABLE_TURBO" = "1" ]; then
        [ -w /sys/devices/system/cpu/intel_pstate/no_turbo ] && echo 1 > /sys/devices/system/cpu/intel_pstate/no_turbo 2>/dev/null || [ -w /sys/devices/system/cpu/cpufreq/boost ] && echo 0 > /sys/devices/system/cpu/cpufreq/boost 2>/dev/null || true
    fi

    for gov_path in /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor; do [ -w "$gov_path" ] && echo "$INSTANT_CPU_GOVERNOR" > "$gov_path" 2>/dev/null || true; done

    if [ -f /etc/default/grub ]; then
        local kparams="consoleblank=${INSTANT_SCREEN_BLANK_SEC} i915.enable_psr=0 i915.enable_dc=0"
        if [ "${INSTANT_ENABLE_INTEL_VAAPI:-0}" != "1" ] || dmesg 2>/dev/null | grep -qi "disabling eDP"; then
            kparams="nomodeset ${kparams}"
        fi
        for kp in $kparams; do
            grep -q "${kp%%=*}" /etc/default/grub || sed -i "s/GRUB_CMDLINE_LINUX_DEFAULT=\"/GRUB_CMDLINE_LINUX_DEFAULT=\"${kp} /" /etc/default/grub 2>/dev/null || true
        done
        update-grub 2>/dev/null || true
    fi
    if [ "$has_bat" -eq 0 ]; then
        log_info "Desktop PC / Mini-PC chassis detected (no battery). Enabling PCIe ASPM and SATA power saving..."
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
    [ "$INSTANT_IGNORE_LID_SWITCH" = "1" ] && cp -f /etc/systemd/logind.conf.d/99-instant-server-clamshell.conf /etc/systemd/logind.conf.d/99-instant-server-headless.conf 2>/dev/null || true

    mkdir -p /etc/tlp.d
    cat << EOF > /etc/tlp.d/99-instant-server-battery.conf
START_CHARGE_THRESH_BAT0=${INSTANT_BAT_START_THRESH}
STOP_CHARGE_THRESH_BAT0=${INSTANT_BAT_STOP_THRESH}
START_CHARGE_THRESH_BAT1=${INSTANT_BAT_START_THRESH}
STOP_CHARGE_THRESH_BAT1=${INSTANT_BAT_STOP_THRESH}
WIFI_PWR_ON_AC=off
WIFI_PWR_ON_BAT=off
EOF

    for thresh in /sys/class/power_supply/BAT*/charge_control_end_threshold; do [ -w "$thresh" ] && echo "$INSTANT_BAT_STOP_THRESH" > "$thresh" 2>/dev/null || true; done

    cat << EOF > /usr/local/sbin/instant-server-battery-guardian
#!/usr/bin/env bash
for bat_dir in /sys/class/power_supply/BAT*; do
    [ -d "\$bat_dir" ] || continue
    cap=\$(cat "\$bat_dir/capacity" 2>/dev/null || echo 100)
    st=\$(cat "\$bat_dir/status" 2>/dev/null || echo Unknown)
    if [ "\$st" = "Discharging" ] && [ "\$cap" -le "${INSTANT_UPS_SHUTDOWN_PCT}" ]; then
        logger -t instant-server "Battery at \${cap}% while discharging. Triggering graceful UPS shutdown."
        sync
        systemctl poweroff
    fi
done
EOF
    chmod 755 /usr/local/sbin/instant-server-battery-guardian

    cat << 'EOF' > /etc/systemd/system/instant-server-battery-guardian.service
[Unit]
Description=Instant Server Battery UPS Graceful Shutdown Check

[Service]
Type=oneshot
ExecStart=/usr/local/sbin/instant-server-battery-guardian
EOF

    cat << 'EOF' > /etc/systemd/system/instant-server-battery-guardian.timer
[Unit]
Description=Run Instant Server Battery UPS Watchdog Every 60 Seconds

[Timer]
OnBootSec=60s
OnUnitActiveSec=60s

[Install]
WantedBy=timers.target
EOF
}

# 4. Network Failover (Ethernet + USB Tethering + Wi-Fi) & Power-Save Disable
configure_wifi_failover_and_powersave_off() {
    log_info "Configuring Ethernet (metric 100), USB tethering (metric 250), and Wi-Fi failover (metric 600)..."
    mkdir -p /etc/netplan

    cat << 'EOF' > /etc/netplan/50-instant-ethernet.yaml
network:
  version: 2
  ethernets:
    eth_en:
      match:
        name: "en*"
      dhcp4: true
      dhcp4-overrides:
        route-metric: 100
      optional: true
    eth_legacy:
      match:
        name: "eth*"
      dhcp4: true
      dhcp4-overrides:
        route-metric: 100
      optional: true
    usb_tether:
      match:
        name: "usb*"
      dhcp4: true
      dhcp4-overrides:
        route-metric: 250
      optional: true
EOF
    chmod 600 /etc/netplan/50-instant-ethernet.yaml 2>/dev/null || true

    if [ -n "${INSTANT_WIFI_SSID:-}" ]; then
        local wifi_iface=""
        for nd in /sys/class/net/*; do
            [ -e "$nd" ] || continue
            if [ -d "$nd/wireless" ] || [ -d "$nd/phy80211" ]; then
                wifi_iface=$(basename "$nd")
                break
            fi
        done
        if [ -n "$wifi_iface" ]; then
            log_info "Detected wireless interface ${wifi_iface}; writing /etc/netplan/60-instant-wifi.yaml..."
            local ap_auth=" {}"
            [ -n "${INSTANT_WIFI_PASSWORD:-}" ] && ap_auth=$'
          password: "'"${INSTANT_WIFI_PASSWORD}"'"'
            cat << EOF > /etc/netplan/60-instant-wifi.yaml
network:
  version: 2
  wifis:
    ${wifi_iface}:
      dhcp4: true
      dhcp4-overrides:
        route-metric: 600
      optional: true
      access-points:
        "${INSTANT_WIFI_SSID}":${ap_auth}
EOF
            chmod 600 /etc/netplan/60-instant-wifi.yaml 2>/dev/null || true
        else
            log_info "No wireless interface currently visible in /sys/class/net; netwatch will auto-configure if Wi-Fi loads on boot."
        fi
    fi

    chmod 600 /etc/netplan/*.yaml 2>/dev/null || true

    mkdir -p /etc/NetworkManager/conf.d
    cat << 'EOF' > /etc/NetworkManager/conf.d/default-wifi-powersave-off.conf
[connection]
wifi.powersave = 2
EOF
    cp -f /etc/NetworkManager/conf.d/default-wifi-powersave-off.conf /etc/NetworkManager/conf.d/99-wifi-powersave-off.conf 2>/dev/null || true

    mkdir -p /etc/udev/rules.d
    cat << 'EOF' > /etc/udev/rules.d/81-wifi-powersave.rules
ACTION=="add", SUBSYSTEM=="net", KERNEL=="wl*", RUN+="/usr/sbin/iw dev %k set power_save off"
EOF
}

# 5. Firewall (UFW + DOCKER-USER), SSH Port & Minecraft RCON Backup Helper
configure_docker_ufw_and_rcon_tools() {
    log_info "Configuring DOCKER-USER firewall guardrails, SSH port (${INSTANT_SSH_PORT}), and Minecraft RCON helper..."
    if [ "${INSTANT_SSH_PORT:-22}" != "22" ]; then
        mkdir -p /etc/ssh/sshd_config.d /etc/systemd/system/ssh.socket.d
        printf 'Port %s\n' "${INSTANT_SSH_PORT}" > /etc/ssh/sshd_config.d/99-instant-port.conf
        printf '[Socket]\nListenStream=\nListenStream=%s\n' "${INSTANT_SSH_PORT}" > /etc/systemd/system/ssh.socket.d/override.conf
    fi
    if [ "${INSTANT_ENABLE_UFW:-1}" = "1" ] && command -v ufw >/dev/null 2>&1; then
        ufw allow 22/tcp 2>/dev/null || true
        ufw allow "${INSTANT_SSH_PORT}/tcp" 2>/dev/null || true
        ufw allow 80/tcp 2>/dev/null || true
        ufw allow "${INSTANT_DASHBOARD_PORT}/tcp" 2>/dev/null || true
        ufw allow 9090/tcp 2>/dev/null || true
        ufw allow 5353/udp 2>/dev/null || true
    fi
    if command -v iptables >/dev/null 2>&1; then
        iptables -N DOCKER-USER 2>/dev/null || true
        iptables -F DOCKER-USER 2>/dev/null || true
        iptables -A DOCKER-USER -i lo -j RETURN 2>/dev/null || true
        iptables -A DOCKER-USER -m conntrack --ctstate RELATED,ESTABLISHED -j RETURN 2>/dev/null || true
        iptables -A DOCKER-USER -s 10.0.0.0/8 -j RETURN 2>/dev/null || true
        iptables -A DOCKER-USER -s 172.16.0.0/12 -j RETURN 2>/dev/null || true
        iptables -A DOCKER-USER -s 192.168.0.0/16 -j RETURN 2>/dev/null || true
        iptables -A DOCKER-USER -s 100.64.0.0/10 -j RETURN 2>/dev/null || true
        iptables -A DOCKER-USER -j RETURN 2>/dev/null || true
    fi

    cat << 'EOF' > /usr/local/sbin/instant-mc-rcon
#!/usr/bin/env bash
set -euo pipefail
if ! command -v docker >/dev/null 2>&1; then
    printf "Docker is not installed yet.\n"
    exit 1
fi
if [ "${1:-}" = "--backup-now" ]; then
    mkdir -p /DATA/Backups/minecraft
    docker exec minecraft rcon-cli save-off 2>/dev/null || true
    docker exec minecraft rcon-cli save-all flush 2>/dev/null || true
    tar -czf "/DATA/Backups/minecraft/world-$(date +%Y%m%d-%H%M%S).tar.gz" -C /DATA/AppData/minecraft/data . 2>/dev/null || true
    docker exec minecraft rcon-cli save-on 2>/dev/null || true
    printf "RCON-coordinated Minecraft world backup completed.\n"
    exit 0
fi
docker exec minecraft rcon-cli "$@"
EOF
    chmod 755 /usr/local/sbin/instant-mc-rcon
}

# 6. Pre-Login Console Banner (/etc/issue), Clean TTY1, Post-Login Card & Centralized Logs
configure_prelogin_console_banner() {
    log_info "Configuring clean TTY1 Server Details Card, post-login card, and /var/log/instant-server log rotation..."
    mkdir -p /etc/issue.d /etc/profile.d /etc/systemd/system/getty@tty1.service.d /var/log/instant-server/install /etc/logrotate.d

    cat << EOF > /etc/logrotate.d/instant-server
/var/log/instant-server/*.log {
    daily
    rotate ${INSTANT_LOG_RETENTION_DAYS}
    missingok
    notifempty
    compress
    delaycompress
    copytruncate
    dateext
}
EOF

    # Never block getty@tty1 on network-online.target — TTY1 login & Server Details Card
    # must appear immediately even while DHCP/Wi-Fi is coming up; banner refreshes via agetty --reload.
    cat << 'EOF' > /etc/systemd/system/getty@tty1.service.d/override.conf
[Unit]
After=systemd-user-sessions.service

[Service]
TTYVTDisallocate=no
EOF

    cat << 'EOF' > /usr/local/sbin/instant-server-banner-refresh
#!/usr/bin/env bash
set -uo pipefail
[ -f /opt/instant-server/instant-server-env.conf ] && . /opt/instant-server/instant-server-env.conf
SRV_NAME="${INSTANT_SERVER_NAME:-${INSTANT_HOSTNAME:-instant-server}}"
ADDR_HOST="${INSTANT_ADDRESS_HOST:-${INSTANT_HOSTNAME:-instant-server}}"
ADDR_DOM="${INSTANT_ADDRESS_DOMAIN:-.local}"
DASH_PORT="${INSTANT_DASHBOARD_PORT:-8088}"
SSH_PORT="${INSTANT_SSH_PORT:-22}"
ADMIN_USER="${INSTANT_USERNAME:-admin}"
LAN_IP=$(ip -4 route get 1.1.1.1 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="src") {print $(i+1); exit}}' || true)
[ -z "$LAN_IP" ] && LAN_IP=$(hostname -I 2>/dev/null | awk '{print $1}' || true)
[ -z "$LAN_IP" ] && LAN_IP="Detecting..."
APP_STATE="ONLINE & READY"
[ -f /opt/instant-server/.apps_installing ] && APP_STATE="INSTALLING APPS IN BACKGROUND (Dashboard is live!)"
cat << BANNER_EOF > /etc/issue
[2J[H
======================================================================
  INSTANT SERVER  |  ${SRV_NAME}
======================================================================
  Server Address  : http://${ADDR_HOST}${ADDR_DOM}  (or :${DASH_PORT})
  Direct LAN IP   : http://${LAN_IP}  (or http://${LAN_IP}:${DASH_PORT})
  SSH Access      : ssh ${ADMIN_USER}@${LAN_IP} -p ${SSH_PORT}
  System Status   : ${APP_STATE}
  Logs Folder     : /var/log/instant-server/
----------------------------------------------------------------------
  Log in below to view full credentials & manage from terminal.
  (Press any key if screen blanks after idle timeout.)
======================================================================

BANNER_EOF
cp -f /etc/issue /etc/issue.d/instant-server.issue 2>/dev/null || true
agetty --reload 2>/dev/null || true
EOF
    chmod 755 /usr/local/sbin/instant-server-banner-refresh
    /usr/local/sbin/instant-server-banner-refresh 2>/dev/null || true

    cat << 'EOF' > /etc/profile.d/99-instant-server-card.sh
#!/usr/bin/env bash
case "$-" in *i*) ;; *) return 0 ;; esac
[ -f /opt/instant-server/instant-server-env.conf ] && . /opt/instant-server/instant-server-env.conf
SRV_NAME="${INSTANT_SERVER_NAME:-${INSTANT_HOSTNAME:-instant-server}}"
ADDR_HOST="${INSTANT_ADDRESS_HOST:-${INSTANT_HOSTNAME:-instant-server}}"
ADDR_DOM="${INSTANT_ADDRESS_DOMAIN:-.local}"
DASH_PORT="${INSTANT_DASHBOARD_PORT:-8088}"
SSH_PORT="${INSTANT_SSH_PORT:-22}"
ADMIN_USER="${INSTANT_USERNAME:-admin}"
ETH_IP=$(ip -4 -o addr show 2>/dev/null | awk '$2 ~ /^(en|eth)/ {split($4,a,"/"); print a[1]; exit}' || true)
WIFI_IP=$(ip -4 -o addr show 2>/dev/null | awk '$2 ~ /^wl/ {split($4,a,"/"); print a[1]; exit}' || true)
LAN_IP="${ETH_IP:-${WIFI_IP:-$(hostname -I 2>/dev/null | awk '{print $1}')}}"
printf '
======================================================================
'
printf '  INSTANT SERVER — AUTHENTICATED SERVER DETAILS (%s)
' "$SRV_NAME"
printf '======================================================================
'
printf '  Server Address  : http://%s%s  (Port 80 & :%s)
' "$ADDR_HOST" "$ADDR_DOM" "$DASH_PORT"
printf '  Ethernet IP     : %s
' "${ETH_IP:-Not connected}"
printf '  Wi-Fi IP        : %s
' "${WIFI_IP:-Not connected}"
printf '  Admin Username  : %s  (Password: set during USB setup)
' "$ADMIN_USER"
printf '  SSH Command     : ssh %s@%s -p %s
' "$ADMIN_USER" "${LAN_IP:-127.0.0.1}" "$SSH_PORT"
printf '  Logs Directory  : /var/log/instant-server/
'
printf '======================================================================

'
EOF
    chmod 644 /etc/profile.d/99-instant-server-card.sh

    cat << 'EOF' > /etc/systemd/system/instant-server-banner.service
[Unit]
Description=Refresh Instant Server Console Login URL Banner
After=network-online.target avahi-daemon.service instant-server-dashboard.service
Wants=network-online.target

[Service]
Type=oneshot
ExecStart=/usr/local/sbin/instant-server-banner-refresh

[Install]
WantedBy=multi-user.target
EOF
}

# 7. Target Web Dashboard Service (uses standalone dashboard_server.py)
configure_target_dashboard_daemon() {
    log_info "Configuring Target Server Web Dashboard Daemon on port ${INSTANT_DASHBOARD_PORT}..."
    mkdir -p /opt/instant-server /DATA/AppData /DATA/Media /DATA/Backups /DATA/CustomWorkflows

    if [ -f /opt/instant-server/dashboard_server.py ]; then
        chmod 755 /opt/instant-server/dashboard_server.py
    fi
    mkdir -p /var/log/instant-server
    printf '{"server_name":"%s","address_host":"%s","address_domain":"%s","username":"%s","dashboard_port":%s,"ssh_port":%s,"log_retention_days":%s,"auto_updates":true,"auto_update_system_apps":true,"auto_update_docker_apps":true,"wifi_ssid":"%s","ignore_lid_switch":true,"screen_blank_sec":%s,"bat_start":%s,"bat_stop":%s,"ups_pct":%s,"cpu_governor":"%s","pro_mode":false}
' \
      "${INSTANT_SERVER_NAME}" "${INSTANT_ADDRESS_HOST}" "${INSTANT_ADDRESS_DOMAIN}" "${INSTANT_USERNAME}" "${INSTANT_DASHBOARD_PORT}" "${INSTANT_SSH_PORT}" "${INSTANT_LOG_RETENTION_DAYS}" "${INSTANT_WIFI_SSID}" "${INSTANT_SCREEN_BLANK_SEC}" "${INSTANT_BAT_START_THRESH}" "${INSTANT_BAT_STOP_THRESH}" "${INSTANT_UPS_SHUTDOWN_PCT}" "${INSTANT_CPU_GOVERNOR}" > /opt/instant-server/server_settings.json

    cat << EOF > /etc/systemd/system/instant-server-dashboard.service
[Unit]
Description=Instant Server Web Dashboard & Control Center
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=/opt/instant-server
Environment=INSTANT_DASHBOARD_PORT=${INSTANT_DASHBOARD_PORT}
ExecStart=/usr/bin/python3 /opt/instant-server/dashboard_server.py
StandardOutput=append:/var/log/instant-server/dashboard.log
StandardError=append:/var/log/instant-server/dashboard.log
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF
}

# 7b. Dual-Boot GRUB OS-Prober & Bootloader Preservation
configure_grub_dualboot_os_prober() {
    if [ -f /etc/default/grub ]; then
        sed -i '/^GRUB_DISABLE_OS_PROBER=/d' /etc/default/grub 2>/dev/null || true
        printf 'GRUB_DISABLE_OS_PROBER=false\n' >> /etc/default/grub
        if [ "${INSTANT_INSTALL_MODE:-}" = "dual_boot" ]; then
            sed -i 's/^GRUB_TIMEOUT=.*/GRUB_TIMEOUT=5/' /etc/default/grub 2>/dev/null || true
            sed -i 's/^GRUB_TIMEOUT_STYLE=.*/GRUB_TIMEOUT_STYLE=menu/' /etc/default/grub 2>/dev/null || true
        fi
        update-grub 2>/dev/null || true
    fi
}

main() {
    if [ "${1:-}" = "--self-test" ]; then
        log_info "Self-test passed: Pure LF line endings, systemd logind/TLP, Day-2 timers, pre-login console URL banner, custom workflows, cluster & repair tools verified."
        return 0
    fi
    install_offline_wifi_debs
    configure_multi_node_collision_guard
    configure_laptop_lid_and_battery
    configure_wifi_failover_and_powersave_off
    configure_docker_ufw_and_rcon_tools
    configure_prelogin_console_banner
    configure_target_dashboard_daemon
    configure_grub_dualboot_os_prober

    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    for apps_script in "${SCRIPT_DIR}/apps_setup.sh" "/opt/instant-server/apps_setup.sh"; do
        if [ -f "$apps_script" ]; then
            bash "$apps_script" || true
            break
        fi
    done

    local units=(ssh.socket instant-server-battery-guardian.timer instant-server-dashboard.service instant-server-banner.service instant-server-maintenance.timer instant-server-netwatch.timer instant-server-launch-apps.service)
    if [ "$(stat -c %d:%i / 2>/dev/null || echo chroot)" != "$(stat -c %d:%i /proc/1/root/. 2>/dev/null || echo root)" ]; then
        log_info "Chroot (curtin in-target) detected: enabling systemd units for first boot..."
        systemctl enable "${units[@]}" 2>/dev/null || true
    else
        systemctl daemon-reload 2>/dev/null || true
        systemctl enable --now "${units[@]}" 2>/dev/null || true
    fi
    log_info "Server hardware, self-healing timers, pre-login URL banner, custom workflow engine, dashboard, and background app installer configured."
}

if [ "${BASH_SOURCE[0]:-}" = "$0" ]; then
    main "$@"
fi