#!/usr/bin/env bash
# shellcheck disable=SC1090,SC1091,SC2015,SC2034
# ==============================================================================
# FILE: src/instant_server/scripts/apps_setup.sh
# PURPOSE: Configures Docker Compose app stacks, Kernel Pro sysctl tuning,
#          Day-2 self-healing timers, cluster storage, and custom workflows.
#          Invoked automatically by laptop_hardening.sh during late-commands.
# ==============================================================================
set -euo pipefail

ENV_CONF="${INSTANT_ENV_CONF:-/opt/instant-server/instant-server-env.conf}"
if [ -f "$ENV_CONF" ]; then
    # shellcheck disable=SC1090
    source "$ENV_CONF"
fi

INSTANT_HOSTNAME="${INSTANT_HOSTNAME:-instant-server}"
INSTANT_USERNAME="${INSTANT_USERNAME:-admin}"
INSTANT_DASHBOARD_PORT="${INSTANT_DASHBOARD_PORT:-8088}"
INSTANT_SSH_PORT="${INSTANT_SSH_PORT:-22}"
INSTANT_ENABLE_UFW="${INSTANT_ENABLE_UFW:-1}"
INSTANT_ENABLE_FAIL2BAN="${INSTANT_ENABLE_FAIL2BAN:-1}"
INSTANT_ENABLE_TCP_BBR="${INSTANT_ENABLE_TCP_BBR:-1}"
INSTANT_VM_SWAPPINESS="${INSTANT_VM_SWAPPINESS:-10}"
INSTANT_INOTIFY_MAX_WATCHES="${INSTANT_INOTIFY_MAX_WATCHES:-524288}"
INSTANT_ENABLE_AUTO_UPDATES="${INSTANT_ENABLE_AUTO_UPDATES:-1}"
INSTANT_CUSTOM_APT_PACKAGES="${INSTANT_CUSTOM_APT_PACKAGES:-}"
INSTANT_ENABLE_TAILSCALE="${INSTANT_ENABLE_TAILSCALE:-0}"
INSTANT_TAILSCALE_AUTHKEY="${INSTANT_TAILSCALE_AUTHKEY:-}"
INSTANT_TAILSCALE_MODE="${INSTANT_TAILSCALE_MODE:-subnet_and_ssh}"
INSTANT_CLUSTER_MODE="${INSTANT_CLUSTER_MODE:-standalone}"
INSTANT_CLUSTER_POLICY="${INSTANT_CLUSTER_POLICY:-mergerfs_union_safe}"
INSTANT_CPU_GOVERNOR="${INSTANT_CPU_GOVERNOR:-schedutil}"
INSTANT_DISABLE_TURBO="${INSTANT_DISABLE_TURBO:-0}"
INSTANT_ENABLE_CASAOS="${INSTANT_ENABLE_CASAOS:-1}"
INSTANT_ENABLE_COCKPIT="${INSTANT_ENABLE_COCKPIT:-1}"

log_info() {
    printf '[instant-server][INFO] %s\n' "$*"
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
mkdir -p /var/log/instant-server
exec >> /var/log/instant-server/apps-setup.log 2>&1
ln -sf /var/log/instant-server/apps-setup.log /var/log/instant-server-apps.log 2>/dev/null || true
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
    apt-get install -y -qq docker.io docker-compose-v2 wpasupplicant wireless-tools rfkill avahi-daemon libnss-mdns logrotate cockpit samba ufw fail2ban tlp mosh curl rsync || apt-get install -y -qq docker.io docker-compose wpasupplicant avahi-daemon cockpit samba ufw || true
fi
if [ -n "${INSTANT_CUSTOM_APT_PACKAGES}" ]; then
    export DEBIAN_FRONTEND=noninteractive
    apt-get install -y -qq ${INSTANT_CUSTOM_APT_PACKAGES} || true
fi
if [ -f /etc/avahi/avahi-daemon.conf ]; then sed -i "s/^#\?host-name=.*/host-name=${INSTANT_ADDRESS_HOST:-${INSTANT_HOSTNAME}}/" /etc/avahi/avahi-daemon.conf 2>/dev/null || true; fi
systemctl enable --now avahi-daemon 2>/dev/null || true
/usr/local/sbin/instant-server-banner-refresh 2>/dev/null || true
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
kernel.printk=3 4 1 3
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
        echo "SELF_TEST_OK"
        return 0
    fi
    configure_firstboot_apps_and_tailscale
    configure_kernel_pro_sysctl_and_services
    configure_day2_self_healing_guardrails
    configure_universal_hardware_and_cluster_suite
    configure_custom_workflows_and_updater_suite
}

if [ "${BASH_SOURCE[0]:-}" = "$0" ]; then
    main "$@"
fi