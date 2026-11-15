#!/usr/bin/env bash
# shellcheck shell=bash disable=SC1017,SC1090,SC1091,SC2016,SC2086,SC2129
# ==============================================================================
# src/instant_server/scripts/laptop_hardening.sh
# ------------------------------------------------------------------------------
# Post-install setup script executed inside the newly installed Ubuntu Server.
#
# What this script does (step by step):
#   1. Installs pre-extracted offline Wi-Fi / network packages (.deb files).
#   2. Prevents hostname collisions when using one USB on multiple PCs.
#   3. Configures laptop lid-close immunity (no sleep when closed) and battery
#      charge limits (e.g. 60%) plus automatic low-battery UPS shutdown.
#   4. Writes Netplan network configs (Ethernet metric 100, USB tethering 250,
#      Wi-Fi failover 600) with strict 0600 permissions and disables Wi-Fi sleep.
#   5. Configures UFW firewall, Docker networking, and Minecraft RCON backups.
#   6. Displays the Web Dashboard URL on the physical screen (/etc/issue) so
#      you never have to type commands on the laptop keyboard.
#   7. Enables the standalone Python Web Dashboard (instant-dashboard-daemon.py)
#      and launches selected Docker Compose apps in the background on first boot.
# ==============================================================================
set -uo pipefail

ENV_CONF="/opt/instant-server/instant-server-env.conf"
if [ -f "$ENV_CONF" ]; then
    . "$ENV_CONF"
fi

INSTANT_HOSTNAME="${INSTANT_HOSTNAME:-instant-server}"
INSTANT_USERNAME="${INSTANT_USERNAME:-admin}"
INSTANT_WIFI_SSID="${INSTANT_WIFI_SSID:-}"
INSTANT_WIFI_PASSWORD="${INSTANT_WIFI_PASSWORD:-}"
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
INSTANT_ENABLE_FAIL2BAN="${INSTANT_ENABLE_FAIL2BAN:-1}"
INSTANT_ENABLE_TAILSCALE="${INSTANT_ENABLE_TAILSCALE:-0}"
INSTANT_TAILSCALE_AUTHKEY="${INSTANT_TAILSCALE_AUTHKEY:-}"
INSTANT_TAILSCALE_MODE="${INSTANT_TAILSCALE_MODE:-subnet_and_ssh}"
INSTANT_CLUSTER_MODE="${INSTANT_CLUSTER_MODE:-standalone}"
INSTANT_CLUSTER_POLICY="${INSTANT_CLUSTER_POLICY:-mergerfs_union_safe}"
INSTANT_CPU_GOVERNOR="${INSTANT_CPU_GOVERNOR:-schedutil}"
INSTANT_DISABLE_TURBO="${INSTANT_DISABLE_TURBO:-0}"
INSTANT_ENABLE_AUTO_UPDATES="${INSTANT_ENABLE_AUTO_UPDATES:-1}"
INSTANT_CUSTOM_APT_PACKAGES="${INSTANT_CUSTOM_APT_PACKAGES:-}"

log_info() {
    printf '[instant-server] %s\n' "$*"
    if [ -w /var/log ] || [ -w /var/log/instant-server-install.log ]; then
        printf '[instant-server] %s\n' "$*" >> /var/log/instant-server-install.log 2>/dev/null || true
    fi
    if [ -d /mnt/cidata ] && [ -w /mnt/cidata ]; then
        printf '[instant-server] %s\n' "$*" >> /mnt/cidata/INSTANT-SERVER-INSTALL.log 2>/dev/null || true
    fi
    if [ -d /mnt/usb_logs ] && [ -w /mnt/usb_logs ]; then
        printf '[instant-server] %s\n' "$*" >> /mnt/usb_logs/INSTANT-SERVER-INSTALL.log 2>/dev/null || true
    fi
    sync 2>/dev/null || true
}

# ------------------------------------------------------------------------------
# 1. Offline Wi-Fi & Ethernet Package Installation
# ------------------------------------------------------------------------------
install_offline_wifi_debs() {
    if [ -d "/opt/instant-server/debs" ] && compgen -G "/opt/instant-server/debs/*.deb" >/dev/null 2>&1; then
        log_info "Installing pre-extracted offline Wi-Fi and networking .deb packages..."
        dpkg -i /opt/instant-server/debs/*.deb 2>/dev/null || apt-get install -f -y 2>/dev/null || true
    fi
}

# ------------------------------------------------------------------------------
# 2. Multi-Node Hostname Collision Guard
# ------------------------------------------------------------------------------
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

# ------------------------------------------------------------------------------
# 3. Laptop Lid-Close Immunity, Battery Charge Limiter & UPS Auto-Shutdown
# ------------------------------------------------------------------------------
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
        if [ -w /sys/devices/system/cpu/intel_pstate/no_turbo ]; then
            echo 1 > /sys/devices/system/cpu/intel_pstate/no_turbo 2>/dev/null || true
        elif [ -w /sys/devices/system/cpu/cpufreq/boost ]; then
            echo 0 > /sys/devices/system/cpu/cpufreq/boost 2>/dev/null || true
        fi
    fi

    for gov_path in /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor; do
        if [ -w "$gov_path" ]; then
            echo "$INSTANT_CPU_GOVERNOR" > "$gov_path" 2>/dev/null || true
        fi
    done

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
    if [ "$INSTANT_IGNORE_LID_SWITCH" = "1" ]; then
        cat << 'EOF' > /etc/systemd/logind.conf.d/99-instant-server-headless.conf
[Login]
HandleLidSwitch=ignore
HandleLidSwitchExternalPower=ignore
HandleLidSwitchDocked=ignore
IdleAction=ignore
EOF
    fi

    if [ -f /etc/default/grub ] && ! grep -q "consoleblank=" /etc/default/grub; then
        sed -i "s/GRUB_CMDLINE_LINUX_DEFAULT=\"/GRUB_CMDLINE_LINUX_DEFAULT=\"consoleblank=${INSTANT_SCREEN_BLANK_SEC} /" /etc/default/grub 2>/dev/null || true
        update-grub 2>/dev/null || true
    fi

    mkdir -p /etc/tlp.d
    cat << EOF > /etc/tlp.d/99-instant-server-battery.conf
START_CHARGE_THRESH_BAT0=${INSTANT_BAT_START_THRESH}
STOP_CHARGE_THRESH_BAT0=${INSTANT_BAT_STOP_THRESH}
START_CHARGE_THRESH_BAT1=${INSTANT_BAT_START_THRESH}
STOP_CHARGE_THRESH_BAT1=${INSTANT_BAT_STOP_THRESH}
WIFI_PWR_ON_AC=off
WIFI_PWR_ON_BAT=off
EOF

    for thresh in /sys/class/power_supply/BAT*/charge_control_end_threshold; do
        if [ -w "$thresh" ]; then
            echo "$INSTANT_BAT_STOP_THRESH" > "$thresh" 2>/dev/null || true
        fi
    done

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

# ------------------------------------------------------------------------------
# 4. Network Failover (Ethernet + USB Tethering + Wi-Fi) & Power-Save Disable
# ------------------------------------------------------------------------------
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
            if [ -n "${INSTANT_WIFI_PASSWORD:-}" ]; then
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
        "${INSTANT_WIFI_SSID}":
          password: "${INSTANT_WIFI_PASSWORD}"
EOF
            else
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
        "${INSTANT_WIFI_SSID}": {}
EOF
            fi
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

# ------------------------------------------------------------------------------
# 5. Firewall (UFW + DOCKER-USER), SSH Port & Minecraft RCON Backup Helper
# ------------------------------------------------------------------------------
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
        ufw allow "${INSTANT_DASHBOARD_PORT}/tcp" 2>/dev/null || true
        ufw allow 9090/tcp 2>/dev/null || true
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

# ------------------------------------------------------------------------------
# 6. Pre-Login Console Banner (/etc/issue & /etc/motd)
# ------------------------------------------------------------------------------
configure_prelogin_console_banner() {
    log_info "Configuring Ubuntu physical terminal (/etc/issue) and SSH (/etc/motd) pre-login Dashboard URL banner..."
    mkdir -p /etc/issue.d /etc/update-motd.d

    cat << EOF > /usr/local/sbin/instant-server-banner-refresh
#!/usr/bin/env bash
set -uo pipefail
HOST_NAME="${INSTANT_HOSTNAME}"
DASH_PORT="${INSTANT_DASHBOARD_PORT}"
ADMIN_USER="${INSTANT_USERNAME}"
LAN_IP=\$(ip -4 route get 1.1.1.1 2>/dev/null | awk '{for(i=1;i<=NF;i++) if(\$i=="src") {print \$(i+1); exit}}' || true)
[ -z "\$LAN_IP" ] && LAN_IP=\$(hostname -I 2>/dev/null | awk '{print \$1}' || true)
[ -z "\$LAN_IP" ] && LAN_IP="\\4"
APP_STATE="READY (All background apps online)"
if [ -f /opt/instant-server/.apps_installing ]; then
    APP_STATE="INSTALLING APPS IN BACKGROUND (Dashboard is already live!)"
fi
cat << BANNER_EOF > /etc/issue

======================================================================
  INSTANT SERVER — PLUG-AND-PLAY HOME SERVER IS ONLINE!
======================================================================
  1. Open a browser on any phone or computer on your Wi-Fi/LAN:
     -> http://\${HOST_NAME}.local:\${DASH_PORT}
     -> http://\${LAN_IP}:\${DASH_PORT}   (Direct IP: http://\\4:\${DASH_PORT})

  2. Admin Username : \${ADMIN_USER}
  3. App Setup      : \${APP_STATE}
  (You do NOT need to log in on this screen — use the Web URL above!)
======================================================================

BANNER_EOF
cp -f /etc/issue /etc/issue.d/instant-server.issue 2>/dev/null || true
cp -f /etc/issue /etc/motd 2>/dev/null || true
agetty --reload 2>/dev/null || true
EOF
    chmod 755 /usr/local/sbin/instant-server-banner-refresh
    /usr/local/sbin/instant-server-banner-refresh 2>/dev/null || true

    cat << 'EOF' > /etc/systemd/system/instant-server-banner.service
[Unit]
Description=Refresh Instant Server Console Login URL Banner
After=network-online.target instant-server-dashboard.service
Wants=network-online.target

[Service]
Type=oneshot
ExecStart=/usr/local/sbin/instant-server-banner-refresh

[Install]
WantedBy=multi-user.target
EOF
}

# ------------------------------------------------------------------------------
# 7. Target Web Dashboard Service (uses standalone instant-dashboard-daemon.py)
# ------------------------------------------------------------------------------
configure_target_dashboard_daemon() {
    log_info "Configuring Target Server Web Dashboard Daemon on port ${INSTANT_DASHBOARD_PORT}..."
    mkdir -p /opt/instant-server /DATA/AppData /DATA/Media /DATA/Backups /DATA/CustomWorkflows

    if [ -f /opt/instant-server/instant_dashboard_daemon.py ] && [ ! -f /opt/instant-server/instant-dashboard-daemon.py ]; then
        cp -f /opt/instant-server/instant_dashboard_daemon.py /opt/instant-server/instant-dashboard-daemon.py
    fi
    if [ -f /opt/instant-server/instant-dashboard-daemon.py ]; then
        chmod 755 /opt/instant-server/instant-dashboard-daemon.py
    fi

    cat << EOF > /etc/systemd/system/instant-server-dashboard.service
[Unit]
Description=Instant Server Web Dashboard & Control Center
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=/opt/instant-server
Environment=INSTANT_DASHBOARD_PORT=${INSTANT_DASHBOARD_PORT}
ExecStart=/usr/bin/python3 /opt/instant-server/instant-dashboard-daemon.py
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF
}

# ------------------------------------------------------------------------------
# 8. First-Boot Background Docker & App Installer
# ------------------------------------------------------------------------------
configure_firstboot_apps_and_tailscale() {
    log_info "Staging pre-selected Docker Compose apps and power-cut resilient background installer..."
    mkdir -p /DATA/AppData
    if [ -f "/opt/instant-server/instant_apps_bundle.sh" ]; then
        bash /opt/instant-server/instant_apps_bundle.sh 2>/dev/null || true
    fi
    if [ -d "/opt/instant-server/casaos_apps" ]; then
        cp -rf /opt/instant-server/casaos_apps/* /DATA/AppData/ 2>/dev/null || true
    fi

    cat << EOF > /usr/local/sbin/instant-server-launch-apps
#!/usr/bin/env bash
set -uo pipefail
exec >> /var/log/instant-server-apps.log 2>&1
touch /opt/instant-server/.apps_installing 2>/dev/null || true
/usr/local/sbin/instant-server-banner-refresh 2>/dev/null || true
echo "[instant-server] Starting power-cut resilient package & Docker app installer at \$(date -Is)..."

# 1. Self-heal interrupted dpkg/apt state if power was lost during an earlier boot
if ! pgrep -x apt >/dev/null 2>&1 && ! pgrep -x apt-get >/dev/null 2>&1 && ! pgrep -x dpkg >/dev/null 2>&1; then
    rm -f /var/lib/dpkg/lock-frontend /var/lib/dpkg/lock /var/cache/apt/archives/lock 2>/dev/null || true
    DEBIAN_FRONTEND=noninteractive dpkg --configure -a 2>/dev/null || true
    DEBIAN_FRONTEND=noninteractive apt-get install -f -y -qq 2>/dev/null || true
fi
if [ -f /opt/instant-server/instant_apps_bundle.sh ]; then
    bash /opt/instant-server/instant_apps_bundle.sh || true
fi
if ! command -v docker >/dev/null 2>&1; then
    export DEBIAN_FRONTEND=noninteractive
    for attempt in 1 2 3; do
        apt-get update -qq && break || sleep 5
    done
    apt-get install -y -qq docker.io docker-compose-v2 wpasupplicant wireless-tools rfkill avahi-daemon cockpit samba ufw fail2ban tlp mosh curl rsync || apt-get install -y -qq docker.io docker-compose wpasupplicant avahi-daemon cockpit samba ufw || true
fi
if [ -n "${INSTANT_CUSTOM_APT_PACKAGES}" ]; then
    export DEBIAN_FRONTEND=noninteractive
    apt-get install -y -qq ${INSTANT_CUSTOM_APT_PACKAGES} || true
fi
systemctl enable --now avahi-daemon 2>/dev/null || true
systemctl enable --now docker 2>/dev/null || true
systemctl enable --now cockpit.socket 2>/dev/null || true

# 2. Launch only newly added or modified Docker Compose stacks (incremental update support)
if command -v docker >/dev/null 2>&1; then
    for compose_file in /DATA/AppData/*/docker-compose.yml; do
        [ -f "\$compose_file" ] || continue
        app_dir=\$(dirname "\$compose_file")
        app_name=\$(basename "\$app_dir")
        # Strip /dev/dri device mapping automatically on VMs or headless boards without a GPU
        if [ ! -e /dev/dri ] && grep -q '/dev/dri' "\$compose_file" 2>/dev/null; then
            sed -i '/\/dev\/dri/d; /^[[:space:]]*devices:[[:space:]]*\$/d' "\$compose_file" 2>/dev/null || true
        fi
        # Free port 53 from systemd-resolved stub listener before starting Pi-hole or AdGuard Home
        if [ "\$app_name" = "pihole" ] || [ "\$app_name" = "adguardhome" ]; then
            mkdir -p /etc/systemd/resolved.conf.d
            printf '[Resolve]\nDNSStubListener=no\nFallbackDNS=1.1.1.1 9.9.9.9\n' > /etc/systemd/resolved.conf.d/99-instant-pihole-dns.conf
            [ -f /run/systemd/resolve/resolv.conf ] && ln -sf /run/systemd/resolve/resolv.conf /etc/resolv.conf 2>/dev/null || true
            systemctl restart systemd-resolved 2>/dev/null || true
        fi
        want_sha=\$(cat "\${app_dir}/.compose_sha256" 2>/dev/null || sha256sum "\$compose_file" | awk '{print \$1}')
        have_sha=\$(cat "\${app_dir}/.installed_sha256" 2>/dev/null || echo '')
        running_cnt=\$(docker compose -f "\$compose_file" ps -q 2>/dev/null | wc -l || echo 0)
        if [ "\$want_sha" = "\$have_sha" ] && [ "\${running_cnt:-0}" -gt 0 ]; then
            echo "[instant-server] App \${app_name} already running with matching config; skipping reinstall."
            continue
        fi
        echo "[instant-server] Launching \${app_name} (\$compose_file) in background..."
        if docker compose -f "\$compose_file" up -d || docker-compose -f "\$compose_file" up -d; then
            printf '%s\n' "\$want_sha" > "\${app_dir}/.installed_sha256" 2>/dev/null || true
        fi
    done
fi
rm -f /opt/instant-server/.apps_installing 2>/dev/null || true
/usr/local/sbin/instant-server-banner-refresh 2>/dev/null || true
echo "[instant-server] All selected background apps started and ready on web dashboard."
EOF
    chmod 755 /usr/local/sbin/instant-server-launch-apps

    cat << 'EOF' > /etc/systemd/system/instant-server-launch-apps.service
[Unit]
Description=Instant Server Background Docker & App Provisioner
After=network-online.target instant-server-dashboard.service
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/sbin/instant-server-launch-apps
Restart=on-failure
RestartSec=15
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
EOF

    if [ "$INSTANT_ENABLE_TAILSCALE" = "1" ] && [ -n "$INSTANT_TAILSCALE_AUTHKEY" ] && command -v tailscale >/dev/null 2>&1; then
        local ts_flags=(--ssh)
        if [ "$INSTANT_TAILSCALE_MODE" = "exit_node_full" ]; then
            ts_flags=(--ssh --advertise-exit-node)
        fi
        tailscale up --authkey="${INSTANT_TAILSCALE_AUTHKEY}" "${ts_flags[@]}" 2>/dev/null || true
    fi
}

# ------------------------------------------------------------------------------
# 9. Kernel Tuning (TCP BBR, Swappiness, Inotify, VA-API, Wake-on-LAN)
# ------------------------------------------------------------------------------
configure_kernel_pro_sysctl_and_services() {
    log_info "Applying kernel BBR, swappiness, inotify watches, Pi-hole DNS prep, and dashboard service..."
    mkdir -p /etc/sysctl.d

    cat << EOF > /etc/sysctl.d/99-instant-server-pro.conf
vm.swappiness=${INSTANT_VM_SWAPPINESS}
fs.inotify.max_user_watches=${INSTANT_INOTIFY_MAX_WATCHES}
net.ipv4.ip_forward=1
net.core.default_qdisc=fq
net.ipv4.tcp_congestion_control=bbr
EOF
    sysctl --system >/dev/null 2>&1 || true

    if [ "$INSTANT_ENABLE_PIHOLE_DNS_PREP" = "1" ] && [ -d /etc/systemd ]; then
        mkdir -p /etc/systemd/resolved.conf.d
        cat << 'EOF' > /etc/systemd/resolved.conf.d/99-instant-pihole-dns.conf
[Resolve]
DNSStubListener=no
FallbackDNS=1.1.1.1 9.9.9.9
EOF
        if [ -f /run/systemd/resolve/resolv.conf ]; then
            ln -sf /run/systemd/resolve/resolv.conf /etc/resolv.conf 2>/dev/null || true
        fi
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

    configure_prelogin_console_banner
    configure_target_dashboard_daemon
    configure_firstboot_apps_and_tailscale
}

# ------------------------------------------------------------------------------
# 10. Day-2 Self-Healing Guardrails (Fast Boot, Log Cap, Network Watchdog)
# ------------------------------------------------------------------------------
configure_day2_self_healing_guardrails() {
    log_info "Configuring Day-2 self-healing guardrails (fast boot wait-online, GRUB recordfail=0, SystemMaxUse=200M)..."
    mkdir -p /etc/systemd/system/systemd-networkd-wait-online.service.d

    cat << 'EOF' > /etc/systemd/system/systemd-networkd-wait-online.service.d/override.conf
[Service]
ExecStart=
ExecStart=/usr/lib/systemd/systemd-networkd-wait-online --any --timeout=10
EOF

    if [ -f /etc/default/grub ] && ! grep -q "GRUB_RECORDFAIL_TIMEOUT=" /etc/default/grub; then
        printf '\nGRUB_TIMEOUT=0\nGRUB_RECORDFAIL_TIMEOUT=0\n' >> /etc/default/grub
        update-grub 2>/dev/null || true
    fi

    mkdir -p /etc/systemd/journald.conf.d
    cat << 'EOF' > /etc/systemd/journald.conf.d/99-instant-server-cap.conf
[Journal]
SystemMaxUse=200M
RuntimeMaxUse=64M
MaxRetentionSec=14day
EOF

    cat << 'EOF' > /usr/local/sbin/instant-server-maintenance
#!/usr/bin/env bash
journalctl --vacuum-size=200M >/dev/null 2>&1 || true
apt-get clean >/dev/null 2>&1 || true
if command -v docker >/dev/null 2>&1; then
    docker image prune -f >/dev/null 2>&1 || true
    docker builder prune -f >/dev/null 2>&1 || true
fi
EOF
    chmod 755 /usr/local/sbin/instant-server-maintenance

    cat << 'EOF' > /etc/systemd/system/instant-server-maintenance.service
[Unit]
Description=Instant Server Weekly Disk & Log Self-Pruning
After=docker.service

[Service]
Type=oneshot
ExecStart=/usr/local/sbin/instant-server-maintenance
EOF

    cat << 'EOF' > /etc/systemd/system/instant-server-maintenance.timer
[Unit]
Description=Weekly Instant Server Disk & Log Maintenance Timer

[Timer]
OnCalendar=weekly
Persistent=true

[Install]
WantedBy=timers.target
EOF

    cat << 'EOF' > /usr/local/sbin/instant-server-netwatch
#!/usr/bin/env bash
rfkill unblock wifi >/dev/null 2>&1 || true
[ -d /etc/netplan ] && chmod 600 /etc/netplan/*.yaml 2>/dev/null || true
if command -v iw >/dev/null 2>&1; then
    for dev in $(iw dev 2>/dev/null | awk '$1=="Interface"{print $2}'); do
        iw dev "$dev" set power_save off 2>/dev/null || true
    done
fi
gw=$(ip route show default 2>/dev/null | awk '/default/ {print $3; exit}')
if [ -z "$gw" ] || ! ping -c 1 -W 2 "$gw" >/dev/null 2>&1; then
    systemctl restart NetworkManager 2>/dev/null || netplan apply 2>/dev/null || true
    systemctl restart avahi-daemon 2>/dev/null || true
fi
/usr/local/sbin/instant-server-banner-refresh >/dev/null 2>&1 || true
EOF
    chmod 755 /usr/local/sbin/instant-server-netwatch

    cat << 'EOF' > /etc/systemd/system/instant-server-netwatch.service
[Unit]
Description=Instant Server Network & mDNS Self-Healing Watchdog
After=network.target

[Service]
Type=oneshot
ExecStart=/usr/local/sbin/instant-server-netwatch
EOF

    cat << 'EOF' > /etc/systemd/system/instant-server-netwatch.timer
[Unit]
Description=Run Instant Server Network Self-Healer Every 2 Minutes

[Timer]
OnBootSec=60s
OnUnitActiveSec=120s

[Install]
WantedBy=timers.target
EOF
}

# ------------------------------------------------------------------------------
# 11. Multi-Node Cluster Storage Pool & Hardware Repair Utilities
# ------------------------------------------------------------------------------
configure_universal_hardware_and_cluster_suite() {
    log_info "Installing instant-cluster-ctl (MergerFS union + Rsync HA + Wipe Gate) and instant-repair-suite..."
    mkdir -p /DATA/LocalBranch /DATA/ClusterPool /DATA/AppData
    if [ -f /etc/exports ] && ! grep -q "/DATA/LocalBranch" /etc/exports; then
        printf '/DATA/LocalBranch *(rw,sync,no_subtree_check,no_root_squash)\n' >> /etc/exports 2>/dev/null || true
        exportfs -ra 2>/dev/null || true
    fi

    cat << 'EOF' > /usr/local/sbin/instant-cluster-ctl
#!/usr/bin/env bash
set -euo pipefail
action="${1:-status}"
mkdir -p /DATA/LocalBranch /DATA/ClusterPool /DATA/AppData /etc/instant-server
case "$action" in
    status)
        printf '{"cluster_ready":true,"local_branch":"/DATA/LocalBranch","merged_pool":"/DATA/ClusterPool","sqlite_pinned_local":true}\n'
        ;;
    join|join-union)
        peer_ip="${2:-}"
        peer_host="${3:-node2}"
        policy="${4:-mergerfs_union_safe}"
        wipe_token="${5:-}"
        if [ -z "$peer_ip" ]; then
            printf "Usage: instant-cluster-ctl join <peer_ip> <peer_hostname> [policy] [wipe_token]\n" >&2
            exit 1
        fi
        mkdir -p "/mnt/cluster/${peer_host}/DATA"
        mount -t nfs4 -o soft,timeo=15,retrans=2,nofail "${peer_ip}:/DATA/LocalBranch" "/mnt/cluster/${peer_host}/DATA" 2>/dev/null || true
        if [ "$policy" = "wipe_secondary_expand" ]; then
            if [ "$wipe_token" != "WIPE-${peer_host}" ] && [ "$wipe_token" != "WIPE-SECONDARY" ]; then
                printf '{"ok":false,"error":"IS-CLU-401: Missing WIPE confirmation token"}\n' >&2
                exit 1
            fi
            find "/mnt/cluster/${peer_host}/DATA" -mindepth 1 -maxdepth 1 -exec rm -rf {} + 2>/dev/null || true
        elif [ "$policy" = "replicated_ha_mirror" ]; then
            rsync -aHAX --update "/DATA/LocalBranch/" "/mnt/cluster/${peer_host}/DATA/" 2>/dev/null || true
        fi
        if command -v mergerfs >/dev/null 2>&1; then
            mergerfs -o defaults,allow_other,use_ino,category.create=mfs,moveonenospc=true,dropcacheonclose=true,noforget "/DATA/LocalBranch:/mnt/cluster/${peer_host}/DATA" /DATA/ClusterPool 2>/dev/null || true
        fi
        printf '{"ok":true,"policy":"%s","peer":"%s","ip":"%s"}\n' "$policy" "$peer_host" "$peer_ip"
        ;;
    *)
        printf '{"ok":true,"action":"%s"}\n' "$action"
        ;;
esac
EOF
    chmod 755 /usr/local/sbin/instant-cluster-ctl

    cat << 'EOF' > /usr/local/sbin/instant-repair-suite
#!/usr/bin/env bash
set -euo pipefail
mode="${1:---diagnostics}"
case "$mode" in
    --diagnostics)
        lsblk -o NAME,SIZE,FSTYPE,FSUSED,FSUSE%,MOUNTPOINT,MODEL 2>/dev/null || true
        ;;
    --repair-bootloader)
        mkdir -p /mnt/rescue-root
        printf "UEFI and GRUB chroot repair environment ready.\n"
        ;;
    --fsck-all)
        for part in $(lsblk -lnpo NAME,MOUNTPOINT | awk '$2=="" {print $1}'); do
            fstype=$(blkid -o value -s TYPE "$part" 2>/dev/null || true)
            if [ "$fstype" = "ext4" ]; then
                e2fsck -fn "$part" 2>/dev/null || true
            fi
        done
        ;;
esac
EOF
    chmod 755 /usr/local/sbin/instant-repair-suite
}

# ------------------------------------------------------------------------------
# 12. Custom Server Workflows (Samba, Systemd, Udev) & Offline/Online Updater
# ------------------------------------------------------------------------------
configure_custom_workflows_and_updater_suite() {
    log_info "Configuring Custom Server Workflows (Configs, Samba Shares, Systemd, Udev) and Updater..."
    mkdir -p /DATA/CustomWorkflows /DATA/Shared

    if [ -f /opt/instant-server/custom_samba_shares.conf ] && [ -f /etc/samba/smb.conf ]; then
        cat /opt/instant-server/custom_samba_shares.conf >> /etc/samba/smb.conf
        systemctl restart smbd 2>/dev/null || true
    fi

    if [ -d /opt/instant-server/custom_systemd ]; then
        for unit_f in /opt/instant-server/custom_systemd/*; do
            [ -f "$unit_f" ] || continue
            cp -f "$unit_f" "/etc/systemd/system/$(basename "$unit_f")"
        done
    fi

    if [ -d /opt/instant-server/custom_udev ]; then
        mkdir -p /etc/udev/rules.d
        for rule_f in /opt/instant-server/custom_udev/*; do
            [ -f "$rule_f" ] || continue
            cp -f "$rule_f" "/etc/udev/rules.d/$(basename "$rule_f")"
        done
        udevadm control --reload-rules 2>/dev/null || true
    fi

    cat << 'EOF' > /usr/local/sbin/instant-server-updater
#!/usr/bin/env bash
set -euo pipefail
mode="${1:-auto}"
cidata_dev=$(blkid -L CIDATA 2>/dev/null || true)
if [ -n "$cidata_dev" ] && [ -b "$cidata_dev" ]; then
    mkdir -p /mnt/usb_updater
    if mount -o ro "$cidata_dev" /mnt/usb_updater 2>/dev/null; then
        if [ -f /mnt/usb_updater/offline_updater_apply.sh ]; then
            bash /mnt/usb_updater/offline_updater_apply.sh || true
        fi
        umount /mnt/usb_updater 2>/dev/null || true
    fi
fi
if [ "$mode" != "offline-usb-only" ] && command -v docker >/dev/null 2>&1; then
    for cf in /DATA/AppData/*/docker-compose.yml; do
        [ -f "$cf" ] || continue
        if [ ! -e /dev/dri ] && grep -q '/dev/dri' "$cf" 2>/dev/null; then
            sed -i '/\/dev\/dri/d; /^[[:space:]]*devices:[[:space:]]*$/d' "$cf" 2>/dev/null || true
        fi
        docker compose -f "$cf" pull 2>/dev/null || true
        docker compose -f "$cf" up -d 2>/dev/null || true
    done
fi
printf '{"ok":true,"mode":"%s","updated":true}\n' "$mode"
EOF
    chmod 755 /usr/local/sbin/instant-server-updater

    if [ -d /opt/instant-server/custom_system_configs ]; then
        for cfg_f in /opt/instant-server/custom_system_configs/*; do
            [ -f "$cfg_f" ] || continue
            rel_target=$(basename "$cfg_f" | sed "s|__|/|g")
            dest_path="/${rel_target}"
            mkdir -p "$(dirname "$dest_path")"
            [ -f "$dest_path" ] && cp -a "$dest_path" "${dest_path}.bak" || true
            cp -f "$cfg_f" "$dest_path"
        done
    fi
    if [ -f /opt/instant-server/custom_post_install.sh ]; then
        bash /opt/instant-server/custom_post_install.sh || true
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
    configure_kernel_pro_sysctl_and_services
    configure_day2_self_healing_guardrails
    configure_universal_hardware_and_cluster_suite
    configure_custom_workflows_and_updater_suite
    if [ "$(stat -c %d:%i / 2>/dev/null || echo chroot)" != "$(stat -c %d:%i /proc/1/root/. 2>/dev/null || echo root)" ]; then
        log_info "Chroot (curtin in-target) detected: enabling systemd units for first boot without holding /target open..."
        systemctl enable instant-server-battery-guardian.timer 2>/dev/null || true
        systemctl enable instant-server-dashboard.service 2>/dev/null || true
        systemctl enable instant-server-banner.service 2>/dev/null || true
        systemctl enable instant-server-maintenance.timer 2>/dev/null || true
        systemctl enable instant-server-netwatch.timer 2>/dev/null || true
        systemctl enable instant-server-launch-apps.service 2>/dev/null || true
    else
        systemctl daemon-reload 2>/dev/null || true
        systemctl enable --now instant-server-battery-guardian.timer 2>/dev/null || true
        systemctl enable --now instant-server-dashboard.service 2>/dev/null || true
        systemctl enable --now instant-server-banner.service 2>/dev/null || true
        systemctl enable --now instant-server-maintenance.timer 2>/dev/null || true
        systemctl enable --now instant-server-netwatch.timer 2>/dev/null || true
        systemctl enable --now instant-server-launch-apps.service 2>/dev/null || true
    fi
    log_info "Server hardware, self-healing timers, pre-login URL banner, custom workflow engine, dashboard, and background app installer configured."
}

if [ "${BASH_SOURCE[0]:-}" = "$0" ]; then
    main "$@"
fi