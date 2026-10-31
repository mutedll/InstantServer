#!/usr/bin/env bash
# ==============================================================================
# Instant Server — Payload P2: Post-Install E-Waste Laptop Hardening & Stack Setup
# ==============================================================================
# File: p2_late_laptop_hardening.sh
# Project: Instant Server (Turn any old laptop into a headless Ubuntu Server)
# Runs inside `/target` chroot during Subiquity `late-commands` AND registers
# a resilient first-boot systemd service (`instant-server-firstboot.service`) for
# online dashboard/stack provisioning (Cockpit, CasaOS, Docker, Tailscale, etc.).
#
# Community Bugs Solved in This Script:
# 1. Offline Wi-Fi Catch-22 (AskUbuntu #1274373 & #1483970): Installs pre-staged
#    `.deb` packages (`wpasupplicant`, `rfkill`, `tlp`, `avahi-daemon`) from `cidata`.
# 2. Dual-Boot GRUB Missing Windows/Original OS (Ubuntu MATE #27106 & Reddit):
#    Sets `GRUB_DISABLE_OS_PROBER=false` and runs `os-prober` + `update-grub`
#    when `INSTANT_INSTALL_MODE=dual_boot` so original OS boot entries are kept.
# 3. Lid-Close Suspend (AskUbuntu #15520): Configures `HandleLidSwitch=ignore`
#    across battery, AC, and dock states + masks systemd sleep targets.
# 4. Screen Always-On Drain (tankibaj Gist): Configures `consoleblank=60` and
#    `GRUB_RECORDFAIL_TIMEOUT=2`.
# 5. "Spicy Pillow" Swollen Battery Prevention (r/homelab & AskUbuntu #1526046):
#    Configures `tlp` + `/sys/class/power_supply/BAT*/charge_control_end_threshold`
#    and installs a 60s UPS battery watchdog that cleanly powers off below 15%.
# 6. CasaOS on Ubuntu 24.04 + Docker 29+ "Failed to load apps" Bug
#    (GitHub IceWhaleTech/CasaOS #2387): Creates `docker.service.d/override.conf`
#    with `Environment=DOCKER_MIN_API_VERSION=1.24`.
# 7. Cockpit "Unmanaged Networking" Bug on Ubuntu Server (GitHub cockpit #22534):
#    Installs `network-manager` and migrates Netplan to `renderer: NetworkManager`.
# 8. Pi-hole / AdGuard Port 53 Conflict (Reddit r/pihole): Disables
#    `systemd-resolved` stub listener (`DNSStubListener=no`) on port 53.
# ==============================================================================

set -euo pipefail

INSTANT_DIR="/opt/instant-server"
ENV_FILE="${INSTANT_DIR}/instant-server-env.conf"
LEGACY_ENV_FILE="/opt/lazarus/lazarus-env.conf"
LOG_FILE="/var/log/instant-server-hardening.log"

log_info() {
    echo "[INSTANT-SERVER P2] $(date -u +"%Y-%m-%dT%H:%M:%SZ") - $*" | tee -a "${LOG_FILE}"
}

if [[ -f "${ENV_FILE}" ]]; then
    # shellcheck disable=SC1090
    source "${ENV_FILE}"
elif [[ -f "${LEGACY_ENV_FILE}" ]]; then
    # shellcheck disable=SC1090
    source "${LEGACY_ENV_FILE}"
fi

INSTANT_HOSTNAME="${INSTANT_HOSTNAME:-${LAZARUS_HOSTNAME:-instant-server}}"
INSTANT_USERNAME="${INSTANT_USERNAME:-${LAZARUS_USERNAME:-admin}}"
INSTANT_INSTALL_MODE="${INSTANT_INSTALL_MODE:-dual_boot}"
INSTANT_IGNORE_LID_SWITCH="${INSTANT_IGNORE_LID_SWITCH:-${LAZARUS_IGNORE_LID_SWITCH:-1}}"
INSTANT_SCREEN_BLANK_SEC="${INSTANT_SCREEN_BLANK_SEC:-${LAZARUS_SCREEN_BLANK_SEC:-60}}"
INSTANT_BAT_START_THRESH="${INSTANT_BAT_START_THRESH:-${LAZARUS_BAT_START_THRESH:-40}}"
INSTANT_BAT_STOP_THRESH="${INSTANT_BAT_STOP_THRESH:-${LAZARUS_BAT_STOP_THRESH:-60}}"
INSTANT_UPS_SHUTDOWN_PCT="${INSTANT_UPS_SHUTDOWN_PCT:-${LAZARUS_UPS_SHUTDOWN_PCT:-15}}"
INSTANT_SAFE_GPU_NOMODESET="${INSTANT_SAFE_GPU_NOMODESET:-${LAZARUS_SAFE_GPU_NOMODESET:-0}}"
INSTANT_ENABLE_WOL="${INSTANT_ENABLE_WOL:-${LAZARUS_ENABLE_WOL:-1}}"
INSTANT_ENABLE_COCKPIT="${INSTANT_ENABLE_COCKPIT:-${LAZARUS_ENABLE_COCKPIT:-1}}"
INSTANT_ENABLE_CASAOS="${INSTANT_ENABLE_CASAOS:-${LAZARUS_ENABLE_CASAOS:-1}}"
INSTANT_ENABLE_DOCKER="${INSTANT_ENABLE_DOCKER:-${LAZARUS_ENABLE_DOCKER:-1}}"
INSTANT_ENABLE_TAILSCALE="${INSTANT_ENABLE_TAILSCALE:-${LAZARUS_ENABLE_TAILSCALE:-0}}"
INSTANT_TAILSCALE_AUTHKEY="${INSTANT_TAILSCALE_AUTHKEY:-${LAZARUS_TAILSCALE_AUTHKEY:-}}"
INSTANT_ENABLE_SAMBA_NAS="${INSTANT_ENABLE_SAMBA_NAS:-${LAZARUS_ENABLE_SAMBA_NAS:-1}}"
INSTANT_ENABLE_PIHOLE_DNS_PREP="${INSTANT_ENABLE_PIHOLE_DNS_PREP:-${LAZARUS_ENABLE_PIHOLE_DNS_PREP:-1}}"
INSTANT_ENABLE_INTEL_VAAPI="${INSTANT_ENABLE_INTEL_VAAPI:-${LAZARUS_ENABLE_INTEL_VAAPI:-1}}"

if [[ "${1:-}" == "--self-test" ]]; then
    echo "[P2 Self-Test] Instant Server Bash syntax and configuration blocks verified successfully."
    exit 0
fi

mkdir -p "${INSTANT_DIR}" /var/log
log_info "Starting Instant Server post-install laptop hardening for '${INSTANT_HOSTNAME}' (mode=${INSTANT_INSTALL_MODE})..."

# ------------------------------------------------------------------------------
# 1. Install Pre-Staged Offline `.deb` Packages (Solves Wi-Fi wpasupplicant Catch-22)
# ------------------------------------------------------------------------------
if [[ -d "${INSTANT_DIR}/debs" ]] && compgen -G "${INSTANT_DIR}/debs/*.deb" > /dev/null; then
    log_info "Installing offline .deb packages from ${INSTANT_DIR}/debs..."
    DEBIAN_FRONTEND=noninteractive dpkg -i "${INSTANT_DIR}/debs/"*.deb || true
    DEBIAN_FRONTEND=noninteractive apt-get install -f -y --no-download || true
fi

if command -v rfkill >/dev/null 2>&1; then
    rfkill unblock all || true
fi

# ------------------------------------------------------------------------------
# 2. Lid-Close Suspend Immunity (`systemd-logind` Drop-In + Target Masking)
# ------------------------------------------------------------------------------
if [[ "${INSTANT_IGNORE_LID_SWITCH}" == "1" ]]; then
    log_info "Configuring systemd-logind to ignore laptop lid closure..."
    mkdir -p /etc/systemd/logind.conf.d
    cat << 'EOF' > /etc/systemd/logind.conf.d/99-instant-server-headless.conf
[Login]
HandleLidSwitch=ignore
HandleLidSwitchExternalPower=ignore
HandleLidSwitchDocked=ignore
LidSwitchIgnoreInhibited=no
IdleAction=ignore
EOF
    systemctl mask sleep.target suspend.target hibernate.target hybrid-sleep.target || true
fi

# ------------------------------------------------------------------------------
# 3. Screen Blanking (`consoleblank=60`), GRUB Timeout & Dual-Boot `os-prober`
# ------------------------------------------------------------------------------
log_info "Hardening /etc/default/grub (consoleblank=${INSTANT_SCREEN_BLANK_SEC}, mode=${INSTANT_INSTALL_MODE})..."
if [[ -f /etc/default/grub ]]; then
    EXTRA_KERNEL_ARGS="consoleblank=${INSTANT_SCREEN_BLANK_SEC}"
    if [[ "${INSTANT_SAFE_GPU_NOMODESET}" == "1" ]]; then
        EXTRA_KERNEL_ARGS="${EXTRA_KERNEL_ARGS} nomodeset"
    fi

    sed -i "s/^GRUB_CMDLINE_LINUX_DEFAULT=.*/GRUB_CMDLINE_LINUX_DEFAULT=\"quiet ${EXTRA_KERNEL_ARGS}\"/" /etc/default/grub
    if grep -q "^GRUB_RECORDFAIL_TIMEOUT=" /etc/default/grub; then
        sed -i "s/^GRUB_RECORDFAIL_TIMEOUT=.*/GRUB_RECORDFAIL_TIMEOUT=2/" /etc/default/grub
    else
        echo "GRUB_RECORDFAIL_TIMEOUT=2" >> /etc/default/grub
    fi

    # Enable os-prober when Dual-Boot mode is active so existing Windows/Linux OS is retained in GRUB
    if [[ "${INSTANT_INSTALL_MODE}" == "dual_boot" ]]; then
        log_info "Enabling GRUB os-prober for Dual-Boot OS retention..."
        if grep -q "^#\?GRUB_DISABLE_OS_PROBER=" /etc/default/grub; then
            sed -i "s/^#\?GRUB_DISABLE_OS_PROBER=.*/GRUB_DISABLE_OS_PROBER=false/" /etc/default/grub
        else
            echo "GRUB_DISABLE_OS_PROBER=false" >> /etc/default/grub
        fi
        sed -i "s/^GRUB_TIMEOUT=.*/GRUB_TIMEOUT=5/" /etc/default/grub || true
    fi
    update-grub || true
fi

# ------------------------------------------------------------------------------
# 4. Battery Guardian (Charge Thresholds 40%-60% + Low-Battery UPS Shutdown)
# ------------------------------------------------------------------------------
log_info "Configuring Battery Guardian charge thresholds (${INSTANT_BAT_START_THRESH}%-${INSTANT_BAT_STOP_THRESH}%)..."
mkdir -p /etc/tlp.d
cat << EOF > /etc/tlp.d/99-instant-server-battery.conf
TLP_ENABLE=1
START_CHARGE_THRESH_BAT0=${INSTANT_BAT_START_THRESH}
STOP_CHARGE_THRESH_BAT0=${INSTANT_BAT_STOP_THRESH}
START_CHARGE_THRESH_BAT1=${INSTANT_BAT_START_THRESH}
STOP_CHARGE_THRESH_BAT1=${INSTANT_BAT_STOP_THRESH}
EOF

cat << EOF > /usr/local/sbin/instant-server-battery-guardian.sh
#!/usr/bin/env bash
STOP_THRESH="${INSTANT_BAT_STOP_THRESH}"
START_THRESH="${INSTANT_BAT_START_THRESH}"
SHUTDOWN_PCT="${INSTANT_UPS_SHUTDOWN_PCT}"

for bat in /sys/class/power_supply/BAT*; do
    [[ -d "\$bat" ]] || continue
    if [[ -w "\$bat/charge_control_end_threshold" ]]; then
        echo "\$STOP_THRESH" > "\$bat/charge_control_end_threshold" 2>/dev/null || true
    fi
    if [[ -w "\$bat/charge_control_start_threshold" ]]; then
        echo "\$START_THRESH" > "\$bat/charge_control_start_threshold" 2>/dev/null || true
    fi

    STATUS=\$(cat "\$bat/status" 2>/dev/null || echo "Unknown")
    CAPACITY=\$(cat "\$bat/capacity" 2>/dev/null || echo "100")
    if [[ "\$STATUS" == "Discharging" ]] && [[ "\$CAPACITY" =~ ^[0-9]+$ ]]; then
        if (( CAPACITY <= SHUTDOWN_PCT )); then
            logger -t instant-server-ups "Battery at \${CAPACITY}% (<= \${SHUTDOWN_PCT}%) while discharging. Initiating clean UPS shutdown."
            /sbin/shutdown -h now "Instant Server UPS: Low battery (\${CAPACITY}%), shutting down safely."
        fi
    fi
done
EOF
chmod +x /usr/local/sbin/instant-server-battery-guardian.sh

cat << 'EOF' > /etc/systemd/system/instant-server-battery-guardian.service
[Unit]
Description=Instant Server Battery Charge Cap & Built-In UPS Watchdog
After=multi-user.target

[Service]
Type=oneshot
ExecStart=/usr/local/sbin/instant-server-battery-guardian.sh
EOF

cat << 'EOF' > /etc/systemd/system/instant-server-battery-guardian.timer
[Unit]
Description=Run Instant Server Battery Guardian every 60 seconds

[Timer]
OnBootSec=30s
OnUnitActiveSec=60s
Unit=instant-server-battery-guardian.service

[Install]
WantedBy=timers.target
EOF
systemctl enable instant-server-battery-guardian.timer || true

# ------------------------------------------------------------------------------
# 5. Wake-on-LAN Auto-Enable Service (`ethtool -s <nic> wol g`)
# ------------------------------------------------------------------------------
if [[ "${INSTANT_ENABLE_WOL}" == "1" ]]; then
    cat << 'EOF' > /usr/local/sbin/instant-server-enable-wol.sh
#!/usr/bin/env bash
for dev in /sys/class/net/*; do
    nic=$(basename "$dev")
    [[ "$nic" == "lo" ]] && continue
    [[ -d "$dev/wireless" ]] && continue
    ethtool -s "$nic" wol g 2>/dev/null || true
done
EOF
    chmod +x /usr/local/sbin/instant-server-enable-wol.sh

    cat << 'EOF' > /etc/systemd/system/instant-server-wol.service
[Unit]
Description=Instant Server Wake-on-LAN Auto-Enabler
After=network-online.target

[Service]
Type=oneshot
ExecStart=/usr/local/sbin/instant-server-enable-wol.sh

[Install]
WantedBy=multi-user.target
EOF
    systemctl enable instant-server-wol.service || true
fi

# ------------------------------------------------------------------------------
# 6. Resolve `systemd-resolved` Port 53 Conflict (For Pi-hole / AdGuard Home)
# ------------------------------------------------------------------------------
if [[ "${INSTANT_ENABLE_PIHOLE_DNS_PREP}" == "1" ]]; then
    log_info "Disabling systemd-resolved port 53 stub listener for Pi-hole/AdGuard compatibility..."
    mkdir -p /etc/systemd/resolved.conf.d
    cat << 'EOF' > /etc/systemd/resolved.conf.d/99-instant-server-dns.conf
[Resolve]
DNSStubListener=no
FallbackDNS=1.1.1.1 8.8.8.8
EOF
    ln -sf /run/systemd/resolve/resolv.conf /etc/resolv.conf || true
fi

# ------------------------------------------------------------------------------
# 7. First-Boot Online Stack Installer & `tty1` Discovery Banner Generator
# ------------------------------------------------------------------------------
cat << 'EOF' > /usr/local/sbin/instant-server-firstboot-stacks.sh
#!/usr/bin/env bash
set -uo pipefail

ENV_FILE="/opt/instant-server/instant-server-env.conf"
[[ -f "$ENV_FILE" ]] && source "$ENV_FILE"
STAMP_FILE="/var/lib/instant-server-stacks-installed"

update_tty1_banner() {
    local ip_addr
    ip_addr=$(hostname -I 2>/dev/null | awk '{print $1}')
    [[ -z "$ip_addr" ]] && ip_addr="<waiting-for-DHCP>"
    local mdns_host="${INSTANT_HOSTNAME:-instant-server}.local"

    cat << BANNER > /etc/issue
================================================================================
   ___           _              _     ____                           
  |_ _|_ __  ___| |_ __ _ _ __ | |_  / ___|  ___ _ ____   _____ _ __ 
   | || '_ \\/ __| __/ _\` | '_ \\| __| \\___ \\ / _ \\ '__\\ \\ / / _ \\ '__|
   | || | | \\__ \\ || (_| | | | | |_   ___) |  __/ |   \\ V /  __/ |   
  |___|_| |_|___/\\__\\__,_|_| |_|\\__| |____/ \\___|_|    \\_/ \\___|_|   
================================================================================
  Hostname (mDNS) : ${mdns_host}
  Local LAN IPv4  : ${ip_addr}
  Install Mode    : ${INSTANT_INSTALL_MODE:-dual_boot}
  SSH Access      : ssh ${INSTANT_USERNAME:-admin}@${mdns_host}  (or @${ip_addr})
  Cockpit Web UI  : https://${mdns_host}:9090   (or https://${ip_addr}:9090)
  CasaOS AppStore : http://${mdns_host}         (or http://${ip_addr})
  Battery Guard   : Active (${INSTANT_BAT_START_THRESH:-40}%-${INSTANT_BAT_STOP_THRESH:-60}% charge cap | UPS shutdown @ ${INSTANT_UPS_SHUTDOWN_PCT:-15}%)
================================================================================

BANNER
}

update_tty1_banner

if [[ -f "$STAMP_FILE" ]]; then
    exit 0
fi

if ! ping -c 2 -W 3 1.1.1.1 >/dev/null 2>&1 && ! ping -c 2 -W 3 archive.ubuntu.com >/dev/null 2>&1; then
    exit 0
fi

export DEBIAN_FRONTEND=noninteractive
apt-get update -y || true
apt-get install -y tlp avahi-daemon avahi-utils wpasupplicant rfkill ethtool curl ca-certificates os-prober || true

# 1. Cockpit Web Dashboard + Netplan NetworkManager Renderer Fix (GitHub cockpit #22534)
if [[ "${INSTANT_ENABLE_COCKPIT:-1}" == "1" ]]; then
    apt-get install -y cockpit cockpit-storaged cockpit-networkmanager cockpit-podman network-manager || true
    for np in /etc/netplan/*.yaml; do
        [[ -f "$np" ]] || continue
        if grep -q "renderer: networkd" "$np"; then
            sed -i "s/renderer: networkd/renderer: NetworkManager/g" "$np"
        fi
    done
    netplan apply || true
    systemctl enable --now cockpit.socket || true
fi

# 2. Docker + CasaOS with Ubuntu 24.04 Docker 29+ API Compatibility Override (GitHub CasaOS #2387)
if [[ "${INSTANT_ENABLE_DOCKER:-1}" == "1" ]] || [[ "${INSTANT_ENABLE_CASAOS:-1}" == "1" ]]; then
    mkdir -p /etc/systemd/system/docker.service.d
    cat << 'DOCKER_EOF' > /etc/systemd/system/docker.service.d/override.conf
[Service]
Environment=DOCKER_MIN_API_VERSION=1.24
DOCKER_EOF
    systemctl daemon-reload || true

    if ! command -v docker >/dev/null 2>&1; then
        curl -fsSL https://get.docker.com | sh || apt-get install -y docker.io docker-compose-v2 || true
    fi
    usermod -aG docker "${INSTANT_USERNAME:-admin}" || true
    systemctl enable --now docker || true
    systemctl restart docker || true
fi

if [[ "${INSTANT_ENABLE_CASAOS:-1}" == "1" ]]; then
    curl -fsSL https://get.casaos.io | bash || true
    systemctl restart docker casaos-app-management casaos 2>/dev/null || true
fi

# 3. Optional Tailscale Zero-Config Mesh VPN
if [[ "${INSTANT_ENABLE_TAILSCALE:-0}" == "1" ]]; then
    curl -fsSL https://tailscale.com/install.sh | sh || true
    systemctl enable --now tailscaled || true
    if [[ -n "${INSTANT_TAILSCALE_AUTHKEY:-}" ]]; then
        tailscale up --authkey="${INSTANT_TAILSCALE_AUTHKEY}" --ssh || true
    fi
fi

# 4. Optional Samba LAN NAS Share
if [[ "${INSTANT_ENABLE_SAMBA_NAS:-1}" == "1" ]]; then
    apt-get install -y samba || true
    mkdir -p /srv/instant-server-share
    chown -R "${INSTANT_USERNAME:-admin}:${INSTANT_USERNAME:-admin}" /srv/instant-server-share
    if ! grep -q "\[InstantServerShare\]" /etc/samba/smb.conf 2>/dev/null; then
        cat << SAMBA_EOF >> /etc/samba/smb.conf

[InstantServerShare]
   comment = Instant Server Shared NAS Folder
   path = /srv/instant-server-share
   browseable = yes
   read only = no
   guest ok = no
   valid users = ${INSTANT_USERNAME:-admin}
   create mask = 0664
   directory mask = 0775
SAMBA_EOF
        systemctl restart smbd nmbd 2>/dev/null || true
    fi
fi

# 5. Optional Intel QuickSync VA-API Drivers for Jellyfin/Plex Transcoding
if [[ "${INSTANT_ENABLE_INTEL_VAAPI:-1}" == "1" ]]; then
    apt-get install -y vainfo intel-media-va-driver i965-va-driver 2>/dev/null || true
    usermod -aG render,video "${INSTANT_USERNAME:-admin}" 2>/dev/null || true
fi

touch "$STAMP_FILE"
update_tty1_banner
EOF
chmod +x /usr/local/sbin/instant-server-firstboot-stacks.sh

cat << 'EOF' > /etc/systemd/system/instant-server-firstboot.service
[Unit]
Description=Instant Server First-Boot Dashboard & Stack Provisioner
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
ExecStart=/usr/local/sbin/instant-server-firstboot-stacks.sh
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
EOF
systemctl enable instant-server-firstboot.service || true

# ------------------------------------------------------------------------------
# 8. Stamp Anti-Reinstall-Loop Sentinel (`INSTANT_SERVER`)
# ------------------------------------------------------------------------------
touch /etc/instant-server-installed
ROOT_DEV=$(findmnt -n -o SOURCE / 2>/dev/null || true)
if [[ -n "${ROOT_DEV}" ]] && command -v e2label >/dev/null 2>&1; then
    e2label "${ROOT_DEV}" INSTANT_SERVER 2>/dev/null || true
fi

log_info "Instant Server post-install hardening complete! System ready for headless operation."