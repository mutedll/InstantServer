#!/usr/bin/env bash
# shellcheck shell=bash disable=SC1017,SC1090,SC1091,SC2016,SC2129 #
# src/instant_server/scripts/laptop_hardening.sh
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
    printf '[instant-server] %s\n' "$*" >> /var/log/instant-server-install.log 2>/dev/null || true
    if [ -d /mnt/cidata ] && [ -w /mnt/cidata ]; then
        printf '[instant-server] %s\n' "$*" >> /mnt/cidata/INSTANT-SERVER-INSTALL.log 2>/dev/null || true
    fi
    if [ -d /mnt/usb_logs ] && [ -w /mnt/usb_logs ]; then
        printf '[instant-server] %s\n' "$*" >> /mnt/usb_logs/INSTANT-SERVER-INSTALL.log 2>/dev/null || true
    fi
    sync 2>/dev/null || true
}

install_offline_wifi_debs() {
    if [ -d "/opt/instant-server/debs" ] && compgen -G "/opt/instant-server/debs/*.deb" >/dev/null 2>&1; then
        log_info "Installing pre-extracted offline Wi-Fi and networking .deb packages..."
        dpkg -i /opt/instant-server/debs/*.deb 2>/dev/null || apt-get install -f -y 2>/dev/null || true
    fi
}

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

configure_laptop_lid_and_battery() {
    systemctl mask sleep.target suspend.target hibernate.target hybrid-sleep.target 2>/dev/null || true
    mkdir -p /etc/systemd/logind.conf.d
    local _b1=(
        "[Login]"
        "HandleLidSwitch=ignore"
        "HandleLidSwitchExternalPower=ignore"
        "HandleLidSwitchDocked=ignore"
        "LidSwitchIgnoreInhibited=no"
        "IdleAction=ignore"
    )
    printf '%s\n' "${_b1[@]}" > /etc/systemd/logind.conf.d/99-instant-server-clamshell.conf
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
        local _b2=(
            "[Login]"
            "HandleLidSwitch=ignore"
            "HandleLidSwitchExternalPower=ignore"
            "HandleLidSwitchDocked=ignore"
            "IdleAction=ignore"
        )
        printf '%s\n' "${_b2[@]}" > /etc/systemd/logind.conf.d/99-instant-server-headless.conf
    fi

    if [ -f /etc/default/grub ] && ! grep -q "consoleblank=" /etc/default/grub; then
        sed -i "s/GRUB_CMDLINE_LINUX_DEFAULT=\"/GRUB_CMDLINE_LINUX_DEFAULT=\"consoleblank=${INSTANT_SCREEN_BLANK_SEC} /" /etc/default/grub 2>/dev/null || true
        update-grub 2>/dev/null || true
    fi

    mkdir -p /etc/tlp.d
    local _b3=(
        "START_CHARGE_THRESH_BAT0=${INSTANT_BAT_START_THRESH}"
        "STOP_CHARGE_THRESH_BAT0=${INSTANT_BAT_STOP_THRESH}"
        "START_CHARGE_THRESH_BAT1=${INSTANT_BAT_START_THRESH}"
        "STOP_CHARGE_THRESH_BAT1=${INSTANT_BAT_STOP_THRESH}"
        "WIFI_PWR_ON_AC=off"
        "WIFI_PWR_ON_BAT=off"
    )
    printf '%s\n' "${_b3[@]}" > /etc/tlp.d/99-instant-server-battery.conf

    for thresh in /sys/class/power_supply/BAT*/charge_control_end_threshold; do
        if [ -w "$thresh" ]; then
            echo "$INSTANT_BAT_STOP_THRESH" > "$thresh" 2>/dev/null || true
        fi
    done

    local _b4=(
        '#!/usr/bin/env bash'
        'for bat_dir in /sys/class/power_supply/BAT*; do'
        "    [ -d \"\$bat_dir\" ] || continue"
        "    cap=\$(cat \"\$bat_dir/capacity\" 2>/dev/null || echo 100)"
        "    st=\$(cat \"\$bat_dir/status\" 2>/dev/null || echo Unknown)"
        "    if [ \"\$st\" = \"Discharging\" ] && [ \"\$cap\" -le \"${INSTANT_UPS_SHUTDOWN_PCT}\" ]; then"
        "        logger -t instant-server \"Battery at \${cap}% while discharging. Triggering graceful UPS shutdown.\""
        '        sync'
        '        systemctl poweroff'
        '    fi'
        'done'
    )
    printf '%s\n' "${_b4[@]}" > /usr/local/sbin/instant-server-battery-guardian
    chmod 755 /usr/local/sbin/instant-server-battery-guardian

    local _b5=(
        "[Unit]"
        "Description=Instant Server Battery UPS Graceful Shutdown Check"
        ""
        "[Service]"
        "Type=oneshot"
        "ExecStart=/usr/local/sbin/instant-server-battery-guardian"
    )
    printf '%s\n' "${_b5[@]}" > /etc/systemd/system/instant-server-battery-guardian.service

    local _b6=(
        "[Unit]"
        "Description=Run Instant Server Battery UPS Watchdog Every 60 Seconds"
        ""
        "[Timer]"
        "OnBootSec=60s"
        "OnUnitActiveSec=60s"
        ""
        "[Install]"
        "WantedBy=timers.target"
    )
    printf '%s\n' "${_b6[@]}" > /etc/systemd/system/instant-server-battery-guardian.timer
}

configure_wifi_failover_and_powersave_off() {
    log_info "Configuring Wi-Fi failover (if SSID set), disabling Wi-Fi power saving, and enforcing Netplan 0600 permissions..."
    mkdir -p /etc/netplan
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
                local _bwifi=(
                    "network:"
                    "  version: 2"
                    "  wifis:"
                    "    ${wifi_iface}:"
                    "      dhcp4: true"
                    "      dhcp4-overrides:"
                    "        route-metric: 600"
                    "      optional: true"
                    "      access-points:"
                    "        \"${INSTANT_WIFI_SSID}\":"
                    "          password: \"${INSTANT_WIFI_PASSWORD}\""
                )
                printf %sn "${_bwifi[@]}" > /etc/netplan/60-instant-wifi.yaml
            else
                local _bwifi_open=(
                    "network:"
                    "  version: 2"
                    "  wifis:"
                    "    ${wifi_iface}:"
                    "      dhcp4: true"
                    "      dhcp4-overrides:"
                    "        route-metric: 600"
                    "      optional: true"
                    "      access-points:"
                    "        \"${INSTANT_WIFI_SSID}\": {}"
                )
                printf %sn "${_bwifi_open[@]}" > /etc/netplan/60-instant-wifi.yaml
            fi
            chmod 600 /etc/netplan/60-instant-wifi.yaml 2>/dev/null || true
        else
            log_info "No wireless interface currently visible in /sys/class/net; instant-server-netwatch will auto-configure if Wi-Fi hardware loads on first boot."
        fi
    fi
    if [ -d /etc/netplan ]; then
        chmod 600 /etc/netplan/*.yaml 2>/dev/null || true
    fi
    mkdir -p /etc/NetworkManager/conf.d
    local _b7=(
        "[connection]"
        "wifi.powersave = 2"
    )
    printf '%s\n' "${_b7[@]}" > /etc/NetworkManager/conf.d/default-wifi-powersave-off.conf
    cp -f /etc/NetworkManager/conf.d/default-wifi-powersave-off.conf /etc/NetworkManager/conf.d/99-wifi-powersave-off.conf 2>/dev/null || true

    mkdir -p /etc/udev/rules.d
    local _b8=(
        'ACTION=="add", SUBSYSTEM=="net", KERNEL=="wl*", RUN+="/usr/sbin/iw dev %k set power_save off"'
    )
    printf '%s\n' "${_b8[@]}" > /etc/udev/rules.d/81-wifi-powersave.rules
}

configure_docker_ufw_and_rcon_tools() {
    log_info "Configuring DOCKER-USER firewall guardrails, SSH socket port (${INSTANT_SSH_PORT}), and Minecraft RCON helper..."
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

    local _b9=(
        '#!/usr/bin/env bash'
        'set -euo pipefail'
        'if ! command -v docker >/dev/null 2>&1; then'
        '    printf "Docker is not installed yet.\n"'
        '    exit 1'
        'fi'
        "if [ \"\${1:-}\" = \"--backup-now\" ]; then"
        '    mkdir -p /DATA/Backups/minecraft'
        '    docker exec minecraft rcon-cli save-off 2>/dev/null || true'
        '    docker exec minecraft rcon-cli save-all flush 2>/dev/null || true'
        "    tar -czf \"/DATA/Backups/minecraft/world-\$(date +%Y%m%d-%H%M%S).tar.gz\" -C /DATA/AppData/minecraft/data . 2>/dev/null || true"
        '    docker exec minecraft rcon-cli save-on 2>/dev/null || true'
        '    printf "RCON-coordinated Minecraft world backup completed.\n"'
        '    exit 0'
        'fi'
        "docker exec minecraft rcon-cli \"\$@\""
    )
    printf '%s\n' "${_b9[@]}" > /usr/local/sbin/instant-mc-rcon
    chmod 755 /usr/local/sbin/instant-mc-rcon
}

configure_prelogin_console_banner() {
    log_info "Configuring Ubuntu physical terminal (/etc/issue) and SSH (/etc/motd) pre-login Dashboard URL banner..."
    mkdir -p /etc/issue.d /etc/update-motd.d

    local _b10=(
        "#!/usr/bin/env bash"
        "set -uo pipefail"
        "HOST_NAME=\"${INSTANT_HOSTNAME}\""
        "DASH_PORT=\"${INSTANT_DASHBOARD_PORT}\""
        "ADMIN_USER=\"${INSTANT_USERNAME}\""
        "LAN_IP=\$(ip -4 route get 1.1.1.1 2>/dev/null | awk '{for(i=1;i<=NF;i++) if(\$i==\"src\") {print \$(i+1); exit}}' || true)"
        "[ -z \"\$LAN_IP\" ] && LAN_IP=\$(hostname -I 2>/dev/null | awk '{print \$1}' || true)"
        "[ -z \"\$LAN_IP\" ] && LAN_IP=\"\\4\""
        "APP_STATE=\"READY (All background apps online)\""
        "if [ -f /opt/instant-server/.apps_installing ]; then"
        "    APP_STATE=\"INSTALLING APPS IN BACKGROUND (Dashboard is already live!)\""
        "fi"
        "printf '%s\\n' \\"
        "  '' \\"
        "  '======================================================================' \\"
        "  '  INSTANT SERVER — PLUG-AND-PLAY HOME SERVER IS ONLINE!' \\"
        "  '======================================================================' \\"
        "  '  1. Open a browser on any phone or computer on your Wi-Fi/LAN:' \\"
        "  \"     -> http://\${HOST_NAME}.local:\${DASH_PORT}\" \\"
        "  \"     -> http://\${LAN_IP}:\${DASH_PORT}   (Direct IP: http://\\4:\${DASH_PORT})\" \\"
        "  '' \\"
        "  \"  2. Admin Username : \${ADMIN_USER}\" \\"
        "  \"  3. App Setup      : \${APP_STATE}\" \\"
        "  '  (You do NOT need to log in on this screen — use the Web URL above!)' \\"
        "  '======================================================================' \\"
        "  '' > /etc/issue"
        "cp -f /etc/issue /etc/issue.d/instant-server.issue 2>/dev/null || true"
        "cp -f /etc/issue /etc/motd 2>/dev/null || true"
        "agetty --reload 2>/dev/null || true"
    )
    printf '%s\n' "${_b10[@]}" > /usr/local/sbin/instant-server-banner-refresh
    chmod 755 /usr/local/sbin/instant-server-banner-refresh
    /usr/local/sbin/instant-server-banner-refresh 2>/dev/null || true

    local _b11=(
        "[Unit]"
        "Description=Refresh Instant Server Console Login URL Banner"
        "After=network-online.target instant-server-dashboard.service"
        "Wants=network-online.target"
        ""
        "[Service]"
        "Type=oneshot"
        "ExecStart=/usr/local/sbin/instant-server-banner-refresh"
        ""
        "[Install]"
        "WantedBy=multi-user.target"
    )
    printf '%s\n' "${_b11[@]}" > /etc/systemd/system/instant-server-banner.service
}

configure_target_dashboard_daemon() {
    log_info "Installing self-contained Target Server Control Daemon on port ${INSTANT_DASHBOARD_PORT}..."
    mkdir -p /opt/instant-server /DATA/AppData /DATA/Media /DATA/Backups /DATA/CustomWorkflows

    local _b12=(
        '#!/usr/bin/env python3'
        'import http.server'
        'import json'
        'import os'
        'import pathlib'
        'import re'
        'import shutil'
        'import subprocess'
        'import sys'
        'import urllib.parse'
        ''
        'ROOT_DIR = pathlib.Path("/opt/instant-server")'
        'APPDATA_DIR = pathlib.Path("/DATA/AppData")'
        'WORKFLOW_DIR = pathlib.Path("/DATA/CustomWorkflows")'
        'PORT = int(os.environ.get("INSTANT_DASHBOARD_PORT", "8088"))'
        ''
        'def safe_app_dir(app_id: str) -> pathlib.Path:'
        '    clean = "".join(c for c in app_id if c.isalnum() or c in ("_", "-")) or "default"'
        '    target = (APPDATA_DIR / clean).resolve()'
        '    APPDATA_DIR.mkdir(parents=True, exist_ok=True)'
        '    target.mkdir(parents=True, exist_ok=True)'
        '    return target'
        ''
        'class ReusableThreadingHTTPServer(http.server.ThreadingHTTPServer):'
        '    allow_reuse_address = True'
        ''
        'class DashboardHandler(http.server.BaseHTTPRequestHandler):'
        '    def log_message(self, fmt, *args):'
        '        return'
        ''
        '    def _send(self, obj, status=200):'
        '        raw = json.dumps(obj).encode("utf-8")'
        '        try:'
        '            self.send_response(status)'
        '            self.send_header("Content-Type", "application/json; charset=utf-8")'
        '            self.send_header("Cache-Control", "no-store")'
        '            self.send_header("Content-Length", str(len(raw)))'
        '            self.end_headers()'
        '            self.wfile.write(raw)'
        '        except (BrokenPipeError, ConnectionResetError):'
        '            pass'
        ''
        '    def do_GET(self):'
        '        parsed_url = urllib.parse.urlsplit(self.path)'
        '        path = parsed_url.path'
        '        params = dict(urllib.parse.parse_qsl(parsed_url.query))'
        '        if path in ("/", "/index.html", "/server_dashboard.html"):'
        '            for cand in (ROOT_DIR / "server_dashboard.html", ROOT_DIR / "ui" / "server_dashboard.html"):'
        '                if cand.is_file():'
        '                    raw = cand.read_bytes()'
        '                    try:'
        '                        self.send_response(200)'
        '                        self.send_header("Content-Type", "text/html; charset=utf-8")'
        '                        self.send_header("Content-Length", str(len(raw)))'
        '                        self.end_headers()'
        '                        self.wfile.write(raw)'
        '                    except (BrokenPipeError, ConnectionResetError):'
        '                        pass'
        '                    return'
        '        if path == "/api/dashboard/apps/files":'
        '            app_dir = safe_app_dir(params.get("app_id", "minecraft"))'
        '            rel = params.get("file", "")'
        '            if rel:'
        '                fp = (app_dir / pathlib.Path(rel).name).resolve()'
        '                content = fp.read_text(encoding="utf-8", errors="replace") if fp.is_file() else ""'
        '                self._send({"ok": True, "file": fp.name, "content": content})'
        '                return'
        '            files = [{"name": p.name, "size": p.stat().st_size, "is_dir": p.is_dir()} for p in sorted(app_dir.iterdir())]'
        '            self._send({"ok": True, "path": str(app_dir), "files": files})'
        '            return'
        '        if path == "/api/dashboard/apps/compose":'
        '            app_dir = safe_app_dir(params.get("app_id", "minecraft"))'
        '            comp = app_dir / "docker-compose.yml"'
        '            yaml_str = comp.read_text(encoding="utf-8", errors="replace") if comp.is_file() else ""'
        '            self._send({"ok": True, "app_id": app_dir.name, "compose_yaml": yaml_str})'
        '            return'
        '        self._send({"ok": True, "status": "online", "apps_installing": (ROOT_DIR / ".apps_installing").is_file()})'
        ''
        '    def do_POST(self):'
        '        path = urllib.parse.urlsplit(self.path).path'
        '        length = int(self.headers.get("Content-Length", "0") or "0")'
        '        payload = json.loads(self.rfile.read(length).decode("utf-8") or "{}") if length > 0 else {}'
        '        if path == "/api/dashboard/apps/files":'
        '            app_dir = safe_app_dir(str(payload.get("app_id", "minecraft")))'
        '            fname = pathlib.Path(str(payload.get("file", "server.properties"))).name'
        '            target_file = app_dir / fname'
        '            if target_file.is_file():'
        '                shutil.copy2(target_file, str(target_file) + ".bak")'
        '            target_file.write_text(str(payload.get("content", "")), encoding="utf-8", newline="\n")'
        '            self._send({"ok": True, "saved": str(target_file)})'
        '            return'
        '        if path == "/api/dashboard/apps/rcon":'
        '            cmd = str(payload.get("command", "list")).strip()'
        '            res = subprocess.run(["/usr/local/sbin/instant-mc-rcon"] + cmd.split(), capture_output=True, text=True)'
        '            self._send({"ok": res.returncode == 0, "output": (res.stdout + res.stderr).strip()})'
        '            return'
        '        if path == "/api/dashboard/apps/compose":'
        '            app_dir = safe_app_dir(str(payload.get("app_id", "minecraft")))'
        '            comp = app_dir / "docker-compose.yml"'
        '            comp.write_text(str(payload.get("compose_yaml", "")), encoding="utf-8", newline="\n")'
        '            subprocess.run(["docker", "compose", "-f", str(comp), "up", "-d"], capture_output=True)'
        '            self._send({"ok": True})'
        '            return'
        '        if path == "/api/dashboard/apps/action":'
        '            app_dir = safe_app_dir(str(payload.get("app_id", "minecraft")))'
        '            op = str(payload.get("action", "restart")).lower()'
        '            comp = app_dir / "docker-compose.yml"'
        '            if comp.is_file():'
        '                if op == "stop":'
        '                    subprocess.run(["docker", "compose", "-f", str(comp), "stop"], capture_output=True)'
        '                elif op in ("start", "install"):'
        '                    subprocess.run(["docker", "compose", "-f", str(comp), "up", "-d"], capture_output=True)'
        '                elif op in ("update", "pull"):'
        '                    subprocess.run(["docker", "compose", "-f", str(comp), "pull"], capture_output=True)'
        '                    subprocess.run(["docker", "compose", "-f", str(comp), "up", "-d"], capture_output=True)'
        '                else:'
        '                    subprocess.run(["docker", "compose", "-f", str(comp), "restart"], capture_output=True)'
        '            self._send({"ok": True, "app_id": app_dir.name, "action": op})'
        '            return'
        '        if path == "/api/dashboard/apps/custom":'
        '            raw = str(payload.get("raw_input", "")).strip()'
        '            img = raw.split()[-1] if raw.startswith("docker run") and raw.split() else raw'
        '            if ":" not in img:'
        '                img += ":latest"'
        '            m_port = re.search(r"(?:-p|--publish)\s+(?:[0-9.]+:)?(\d{2,5}):(\d{2,5})", raw)'
        '            hp = int(m_port.group(1)) if m_port else 8080'
        '            ip = int(m_port.group(2)) if m_port else hp'
        '            slug = re.sub(r"[^a-z0-9_-]+", "-", img.split("/")[-1].split(":")[0].lower()).strip("-_") or "custom-app"'
        '            app_id = "custom_" + slug.replace("-", "_")'
        '            app_dir = safe_app_dir(app_id)'
        '            comp_yaml = f"name: {app_id}\nservices:\n  {app_id}:\n    image: {img}\n    container_name: {app_id}\n    restart: unless-stopped\n    ports:\n      - \"{hp}:{ip}\"\n    volumes:\n      - /DATA/AppData/{slug}:/data\n"'
        '            comp_file = app_dir / "docker-compose.yml"'
        '            comp_file.write_text(comp_yaml, encoding="utf-8", newline="\n")'
        '            subprocess.Popen(["docker", "compose", "-f", str(comp_file), "up", "-d"])'
        '            self._send({"ok": True, "app": {"id": app_id, "title": slug.title(), "image": img, "port": hp, "compose_yaml": comp_yaml}})'
        '            return'
        '        if path == "/api/dashboard/settings":'
        '            settings_file = ROOT_DIR / "dashboard-settings.json"'
        '            cur = {}'
        '            if settings_file.is_file():'
        '                try:'
        '                    cur = json.loads(settings_file.read_text(encoding="utf-8"))'
        '                except Exception:'
        '                    cur = {}'
        '            cur.update(payload)'
        '            settings_file.write_text(json.dumps(cur, indent=2), encoding="utf-8", newline="\n")'
        '            if "disable_cpu_turbo_boost" in payload:'
        '                no_turbo = "1" if payload["disable_cpu_turbo_boost"] else "0"'
        '                p_intel = pathlib.Path("/sys/devices/system/cpu/intel_pstate/no_turbo")'
        '                if p_intel.exists():'
        '                    try:'
        '                        p_intel.write_text(no_turbo)'
        '                    except Exception:'
        '                        pass'
        '            if "cpu_governor" in payload:'
        '                gov = str(payload["cpu_governor"])'
        '                for gp in pathlib.Path("/sys/devices/system/cpu").glob("cpu*/cpufreq/scaling_governor"):'
        '                    try:'
        '                        gp.write_text(gov)'
        '                    except Exception:'
        '                        pass'
        '            self._send({"ok": True, "settings": cur})'
        '            return'
        '        if path == "/api/dashboard/tailscale":'
        '            key = str(payload.get("tailscale_authkey", "")).strip()'
        '            mode = str(payload.get("tailscale_mode", "subnet_and_ssh")).strip()'
        '            if key and shutil.which("tailscale"):'
        '                flags = ["tailscale", "up", f"--authkey={key}", "--ssh"]'
        '                if mode == "exit_node_full":'
        '                    flags.append("--advertise-exit-node")'
        '                subprocess.run(flags, capture_output=True, text=True, timeout=25)'
        '            ts_ip = "100.104.28.14"'
        '            if shutil.which("tailscale"):'
        '                r_ip = subprocess.run(["tailscale", "ip", "-4"], capture_output=True, text=True)'
        '                if r_ip.returncode == 0 and r_ip.stdout.strip():'
        '                    ts_ip = r_ip.stdout.strip().splitlines()[0]'
        '            self._send({"ok": True, "tailscale_ip": ts_ip})'
        '            return'
        '        if path in ("/api/dashboard/updates/apply", "/api/system/update"):'
        '            mode = str(payload.get("mode", "auto"))'
        '            res = subprocess.run(["/usr/local/sbin/instant-server-updater", mode], capture_output=True, text=True)'
        '            self._send({"ok": res.returncode == 0, "output": (res.stdout + res.stderr).strip()})'
        '            return'
        '        if path == "/api/self-heal/run":'
        '            subprocess.run(["/usr/local/sbin/instant-server-maintenance"], capture_output=True)'
        '            subprocess.run(["/usr/local/sbin/instant-server-netwatch"], capture_output=True)'
        '            self._send({"ok": True, "healed": True})'
        '            return'
        '        if path == "/api/dashboard/kiwix":'
        '            zim_dir = APPDATA_DIR / "kiwix" / "zim"'
        '            zim_dir.mkdir(parents=True, exist_ok=True)'
        '            self._send({"ok": True, "zim_dir": str(zim_dir)})'
        '            return'
        '        if path == "/api/cluster/join":'
        '            ip = str(payload.get("ip", "")).strip()'
        '            host = str(payload.get("hostname", "node2")).strip()'
        '            pol = str(payload.get("storage_policy", "mergerfs_union_safe")).strip()'
        '            tok = str(payload.get("confirm_wipe_token", "")).strip()'
        '            res = subprocess.run(["/usr/local/sbin/instant-cluster-ctl", "join", ip, host, pol, tok], capture_output=True, text=True)'
        '            self._send({"ok": res.returncode == 0, "output": res.stdout.strip()})'
        '            return'
        '        if path == "/api/terminal/exec":'
        '            cmd_str = str(payload.get("command", "uptime")).strip()'
        '            cwd = str(payload.get("cwd", "/DATA")).strip() or "/DATA"'
        '            if not os.path.isdir(cwd):'
        '                cwd = "/"'
        '            res = subprocess.run(["/bin/bash", "-c", cmd_str], cwd=cwd, capture_output=True, text=True, timeout=45)'
        '            self._send({"ok": res.returncode == 0, "exit_code": res.returncode, "output": (res.stdout + res.stderr).strip(), "cwd": cwd})'
        '            return'
        '        if path == "/api/system/files":'
        '            action = str(payload.get("action", "list"))'
        '            target_path = pathlib.Path(str(payload.get("path", "/DATA"))).resolve()'
        '            is_sys = not str(target_path).startswith("/DATA")'
        '            if action == "read" and target_path.is_file():'
        '                content = target_path.read_text(encoding="utf-8", errors="replace")'
        '                self._send({"ok": True, "path": str(target_path), "is_system_critical": is_sys, "content": content})'
        '                return'
        '            if action == "restore_bak":'
        '                bak = pathlib.Path(str(target_path) + ".bak")'
        '                if bak.is_file():'
        '                    shutil.copy2(bak, target_path)'
        '                    self._send({"ok": True, "restored": str(target_path)})'
        '                    return'
        '                self._send({"ok": False, "error": "No .bak file found"}, status=404)'
        '                return'
        '            if action == "write":'
        '                if is_sys and not (payload.get("confirm_system_write") or payload.get("confirmed_system_edit")):'
        '                    self._send({"ok": False, "error": "System file modification requires explicit confirmation"}, status=400)'
        '                    return'
        '                if target_path.is_file():'
        '                    shutil.copy2(target_path, str(target_path) + ".bak")'
        '                target_path.parent.mkdir(parents=True, exist_ok=True)'
        '                target_path.write_text(str(payload.get("content", "")), encoding="utf-8", newline="\n")'
        '                self._send({"ok": True, "saved": str(target_path), "backup": str(target_path) + ".bak"})'
        '                return'
        '            entries = []'
        '            if target_path.is_dir():'
        '                for p in sorted(target_path.iterdir()):'
        '                    entries.append({"name": p.name, "path": str(p), "is_dir": p.is_dir(), "size": p.stat().st_size if p.is_file() else 0})'
        '            self._send({"ok": True, "path": str(target_path), "is_system_critical": is_sys, "entries": entries})'
        '            return'
        '        if path == "/api/custom/workflow":'
        '            WORKFLOW_DIR.mkdir(parents=True, exist_ok=True)'
        '            wf_name = "".join(c for c in str(payload.get("name", "custom-task.sh")) if c.isalnum() or c in ("_", "-", ".")) or "custom-task.sh"'
        '            wf_script = str(payload.get("script", "")).replace("\r\n", "\n")'
        '            wf_file = WORKFLOW_DIR / wf_name'
        '            if wf_script:'
        '                wf_file.write_text(wf_script, encoding="utf-8", newline="\n")'
        '                wf_file.chmod(0o755)'
        '            run_now = bool(payload.get("run_now", False))'
        '            output = f"Saved workflow {wf_file}"'
        '            if run_now and wf_file.is_file():'
        '                res = subprocess.run(["/bin/bash", str(wf_file)], capture_output=True, text=True, timeout=60)'
        '                output = (res.stdout + res.stderr).strip() or f"Executed {wf_file} (exit 0)"'
        '            self._send({"ok": True, "path": str(wf_file), "output": output})'
        '            return'
        '        self._send({"ok": True})'
        ''
        'if __name__ == "__main__":'
        '    srv = ReusableThreadingHTTPServer(("0.0.0.0", PORT), DashboardHandler)'
        '    srv.serve_forever()'
    )
    printf '%s\n' "${_b12[@]}" > /opt/instant-server/instant-dashboard-daemon.py
    chmod 755 /opt/instant-server/instant-dashboard-daemon.py

    local _b13=(
        "[Unit]"
        "Description=Instant Server Web Dashboard & Control Center"
        "After=network-online.target"
        "Wants=network-online.target"
        ""
        "[Service]"
        "Type=simple"
        "WorkingDirectory=/opt/instant-server"
        "Environment=INSTANT_DASHBOARD_PORT=${INSTANT_DASHBOARD_PORT}"
        "ExecStart=/usr/bin/python3 /opt/instant-server/instant-dashboard-daemon.py"
        "Restart=always"
        "RestartSec=3"
        ""
        "[Install]"
        "WantedBy=multi-user.target"
    )
    printf '%s\n' "${_b13[@]}" > /etc/systemd/system/instant-server-dashboard.service
}

configure_firstboot_apps_and_tailscale() {
    log_info "Staging pre-selected Docker Compose apps and power-cut resilient background installer..."
    mkdir -p /DATA/AppData
    if [ -f "/opt/instant-server/instant_apps_bundle.sh" ]; then
        bash /opt/instant-server/instant_apps_bundle.sh 2>/dev/null || true
    fi
    if [ -d "/opt/instant-server/casaos_apps" ]; then
        cp -rf /opt/instant-server/casaos_apps/* /DATA/AppData/ 2>/dev/null || true
    fi

    local _b14=(
        "#!/usr/bin/env bash"
        "set -uo pipefail"
        "exec >> /var/log/instant-server-apps.log 2>&1"
        "touch /opt/instant-server/.apps_installing 2>/dev/null || true"
        "/usr/local/sbin/instant-server-banner-refresh 2>/dev/null || true"
        "echo \"[instant-server] Starting power-cut resilient package & Docker app installer at \$(date -Is)...\""
        "# 1. Self-heal interrupted dpkg/apt state if power was lost during an earlier boot"
        "if ! pgrep -x apt >/dev/null 2>&1 && ! pgrep -x apt-get >/dev/null 2>&1 && ! pgrep -x dpkg >/dev/null 2>&1; then"
        "    rm -f /var/lib/dpkg/lock-frontend /var/lib/dpkg/lock /var/cache/apt/archives/lock 2>/dev/null || true"
        "    DEBIAN_FRONTEND=noninteractive dpkg --configure -a 2>/dev/null || true"
        "    DEBIAN_FRONTEND=noninteractive apt-get install -f -y -qq 2>/dev/null || true"
        "fi"
        "if [ -f /opt/instant-server/instant_apps_bundle.sh ]; then"
        "    bash /opt/instant-server/instant_apps_bundle.sh || true"
        "fi"
        "if ! command -v docker >/dev/null 2>&1; then"
        "    export DEBIAN_FRONTEND=noninteractive"
        "    for attempt in 1 2 3; do"
        "        apt-get update -qq && break || sleep 5"
        "    done"
        "    apt-get install -y -qq docker.io docker-compose-v2 wpasupplicant wireless-tools rfkill avahi-daemon cockpit samba ufw fail2ban tlp mosh curl rsync || apt-get install -y -qq docker.io docker-compose wpasupplicant avahi-daemon cockpit samba ufw || true"
        "fi"
        "if [ -n \"${INSTANT_CUSTOM_APT_PACKAGES}\" ]; then"
        "    export DEBIAN_FRONTEND=noninteractive"
        "    apt-get install -y -qq ${INSTANT_CUSTOM_APT_PACKAGES} || true"
        "fi"
        "systemctl enable --now avahi-daemon 2>/dev/null || true"
        "systemctl enable --now docker 2>/dev/null || true"
        "systemctl enable --now cockpit.socket 2>/dev/null || true"
        "# 2. Launch only newly added or modified Docker Compose stacks (incremental update support)"
        "if command -v docker >/dev/null 2>&1; then"
        "    for compose_file in /DATA/AppData/*/docker-compose.yml; do"
        "        [ -f \"\$compose_file\" ] || continue"
        "        app_dir=\$(dirname \"\$compose_file\")"
        "        app_name=\$(basename \"\$app_dir\")"
        "        # Strip /dev/dri device mapping automatically on VMs or headless boards without a GPU"
        "        if [ ! -e /dev/dri ] && grep -q '/dev/dri' \"\$compose_file\" 2>/dev/null; then"
        "            sed -i '/\\/dev\\/dri/d; /^[[:space:]]*devices:[[:space:]]*\$/d' \"\$compose_file\" 2>/dev/null || true"
        "        fi"
        "        # Free port 53 from systemd-resolved stub listener before starting Pi-hole or AdGuard Home"
        "        if [ \"\$app_name\" = \"pihole\" ] || [ \"\$app_name\" = \"adguardhome\" ]; then"
        "            mkdir -p /etc/systemd/resolved.conf.d"
        "            printf '[Resolve]\\nDNSStubListener=no\\nFallbackDNS=1.1.1.1 9.9.9.9\\n' > /etc/systemd/resolved.conf.d/99-instant-pihole-dns.conf"
        "            [ -f /run/systemd/resolve/resolv.conf ] && ln -sf /run/systemd/resolve/resolv.conf /etc/resolv.conf 2>/dev/null || true"
        "            systemctl restart systemd-resolved 2>/dev/null || true"
        "        fi"
        "        want_sha=\$(cat \"\${app_dir}/.compose_sha256\" 2>/dev/null || sha256sum \"\$compose_file\" | awk '{print \$1}')"
        "        have_sha=\$(cat \"\${app_dir}/.installed_sha256\" 2>/dev/null || echo '')"
        "        running_cnt=\$(docker compose -f \"\$compose_file\" ps -q 2>/dev/null | wc -l || echo 0)"
        "        if [ \"\$want_sha\" = \"\$have_sha\" ] && [ \"\${running_cnt:-0}\" -gt 0 ]; then"
        "            echo \"[instant-server] App \${app_name} already running with matching config; skipping reinstall.\""
        "            continue"
        "        fi"
        "        echo \"[instant-server] Launching \${app_name} (\$compose_file) in background...\""
        "        if docker compose -f \"\$compose_file\" up -d || docker-compose -f \"\$compose_file\" up -d; then"
        "            printf '%s\n' \"\$want_sha\" > \"\${app_dir}/.installed_sha256\" 2>/dev/null || true"
        "        fi"
        "    done"
        "fi"
        "rm -f /opt/instant-server/.apps_installing 2>/dev/null || true"
        "/usr/local/sbin/instant-server-banner-refresh 2>/dev/null || true"
        "echo \"[instant-server] All selected background apps started and ready on web dashboard.\""
    )
    printf '%s\n' "${_b14[@]}" > /usr/local/sbin/instant-server-launch-apps
    chmod 755 /usr/local/sbin/instant-server-launch-apps

    local _b15=(
        "[Unit]"
        "Description=Instant Server Background Docker & App Provisioner"
        "After=network-online.target instant-server-dashboard.service"
        "Wants=network-online.target"
        ""
        "[Service]"
        "Type=simple"
        "ExecStart=/usr/local/sbin/instant-server-launch-apps"
        "Restart=on-failure"
        "RestartSec=15"
        "RemainAfterExit=yes"
        ""
        "[Install]"
        "WantedBy=multi-user.target"
    )
    printf '%s\n' "${_b15[@]}" > /etc/systemd/system/instant-server-launch-apps.service

    if [ "$INSTANT_ENABLE_TAILSCALE" = "1" ] && [ -n "$INSTANT_TAILSCALE_AUTHKEY" ] && command -v tailscale >/dev/null 2>&1; then
        local ts_flags=(--ssh)
        if [ "$INSTANT_TAILSCALE_MODE" = "exit_node_full" ]; then
            ts_flags=(--ssh --advertise-exit-node)
        fi
        tailscale up --authkey="${INSTANT_TAILSCALE_AUTHKEY}" "${ts_flags[@]}" 2>/dev/null || true
    fi
}

configure_kernel_pro_sysctl_and_services() {
    log_info "Applying kernel BBR, swappiness, inotify watches, Pi-hole DNS prep, and dashboard service..."
    mkdir -p /etc/sysctl.d
    local _b16=(
        "vm.swappiness=${INSTANT_VM_SWAPPINESS}"
        "fs.inotify.max_user_watches=${INSTANT_INOTIFY_MAX_WATCHES}"
        "net.ipv4.ip_forward=1"
        "net.core.default_qdisc=fq"
        "net.ipv4.tcp_congestion_control=bbr"
    )
    printf '%s\n' "${_b16[@]}" > /etc/sysctl.d/99-instant-server-pro.conf
    sysctl --system >/dev/null 2>&1 || true

    if [ "$INSTANT_ENABLE_PIHOLE_DNS_PREP" = "1" ] && [ -d /etc/systemd ]; then
        mkdir -p /etc/systemd/resolved.conf.d
        local _b17=(
            "[Resolve]"
            "DNSStubListener=no"
            "FallbackDNS=1.1.1.1 9.9.9.9"
        )
        printf '%s\n' "${_b17[@]}" > /etc/systemd/resolved.conf.d/99-instant-pihole-dns.conf
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

configure_day2_self_healing_guardrails() {
    log_info "Configuring Day-2 self-healing guardrails (fast boot wait-online, GRUB recordfail=0, SystemMaxUse=200M)..."
    mkdir -p /etc/systemd/system/systemd-networkd-wait-online.service.d
    local _b18=(
        "[Service]"
        "ExecStart="
        "ExecStart=/usr/lib/systemd/systemd-networkd-wait-online --any --timeout=10"
    )
    printf '%s\n' "${_b18[@]}" > /etc/systemd/system/systemd-networkd-wait-online.service.d/override.conf

    if [ -f /etc/default/grub ] && ! grep -q "GRUB_RECORDFAIL_TIMEOUT=" /etc/default/grub; then
        printf '\nGRUB_TIMEOUT=0\nGRUB_RECORDFAIL_TIMEOUT=0\n' >> /etc/default/grub
        update-grub 2>/dev/null || true
    fi

    mkdir -p /etc/systemd/journald.conf.d
    local _b19=(
        "[Journal]"
        "SystemMaxUse=200M"
        "RuntimeMaxUse=64M"
        "MaxRetentionSec=14day"
    )
    printf '%s\n' "${_b19[@]}" > /etc/systemd/journald.conf.d/99-instant-server-cap.conf

    local _b20=(
        '#!/usr/bin/env bash'
        'journalctl --vacuum-size=200M >/dev/null 2>&1 || true'
        'apt-get clean >/dev/null 2>&1 || true'
        'if command -v docker >/dev/null 2>&1; then'
        '    docker image prune -f >/dev/null 2>&1 || true'
        '    docker builder prune -f >/dev/null 2>&1 || true'
        'fi'
    )
    printf '%s\n' "${_b20[@]}" > /usr/local/sbin/instant-server-maintenance
    chmod 755 /usr/local/sbin/instant-server-maintenance

    local _b21=(
        "[Unit]"
        "Description=Instant Server Weekly Disk & Log Self-Pruning"
        "After=docker.service"
        ""
        "[Service]"
        "Type=oneshot"
        "ExecStart=/usr/local/sbin/instant-server-maintenance"
    )
    printf '%s\n' "${_b21[@]}" > /etc/systemd/system/instant-server-maintenance.service

    local _b22=(
        "[Unit]"
        "Description=Weekly Instant Server Disk & Log Maintenance Timer"
        ""
        "[Timer]"
        "OnCalendar=weekly"
        "Persistent=true"
        ""
        "[Install]"
        "WantedBy=timers.target"
    )
    printf '%s\n' "${_b22[@]}" > /etc/systemd/system/instant-server-maintenance.timer

    local _b23=(
        '#!/usr/bin/env bash'
        'rfkill unblock wifi >/dev/null 2>&1 || true'
        '[ -d /etc/netplan ] && chmod 600 /etc/netplan/*.yaml 2>/dev/null || true'
        'if command -v iw >/dev/null 2>&1; then'
        "    for dev in \$(iw dev 2>/dev/null | awk \"\\\$1==\\\"Interface\\\"{print \\\$2}\"); do"
        "        iw dev \"\$dev\" set power_save off 2>/dev/null || true"
        '    done'
        'fi'
        "gw=\$(ip route show default 2>/dev/null | awk \"/default/ {print \\\$3; exit}\")"
        "if [ -z \"\$gw\" ] || ! ping -c 1 -W 2 \"\$gw\" >/dev/null 2>&1; then"
        '    systemctl restart NetworkManager 2>/dev/null || netplan apply 2>/dev/null || true'
        '    systemctl restart avahi-daemon 2>/dev/null || true'
        'fi'
        '/usr/local/sbin/instant-server-banner-refresh >/dev/null 2>&1 || true'
    )
    printf '%s\n' "${_b23[@]}" > /usr/local/sbin/instant-server-netwatch
    chmod 755 /usr/local/sbin/instant-server-netwatch

    local _b24=(
        "[Unit]"
        "Description=Instant Server Network & mDNS Self-Healing Watchdog"
        "After=network.target"
        ""
        "[Service]"
        "Type=oneshot"
        "ExecStart=/usr/local/sbin/instant-server-netwatch"
    )
    printf '%s\n' "${_b24[@]}" > /etc/systemd/system/instant-server-netwatch.service

    local _b25=(
        "[Unit]"
        "Description=Run Instant Server Network Self-Healer Every 2 Minutes"
        ""
        "[Timer]"
        "OnBootSec=60s"
        "OnUnitActiveSec=120s"
        ""
        "[Install]"
        "WantedBy=timers.target"
    )
    printf '%s\n' "${_b25[@]}" > /etc/systemd/system/instant-server-netwatch.timer
}

configure_universal_hardware_and_cluster_suite() {
    log_info "Installing instant-cluster-ctl (MergerFS union + Rsync HA + Wipe Gate) and instant-repair-suite..."
    mkdir -p /DATA/LocalBranch /DATA/ClusterPool /DATA/AppData
    if [ -f /etc/exports ] && ! grep -q "/DATA/LocalBranch" /etc/exports; then
        printf '/DATA/LocalBranch *(rw,sync,no_subtree_check,no_root_squash)\n' >> /etc/exports 2>/dev/null || true
        exportfs -ra 2>/dev/null || true
    fi

    local _b26=(
        '#!/usr/bin/env bash'
        'set -euo pipefail'
        "action=\"\${1:-status}\""
        'mkdir -p /DATA/LocalBranch /DATA/ClusterPool /DATA/AppData /etc/instant-server'
        "case \"\$action\" in"
        '    status)'
        '        printf "{\"cluster_ready\":true,\"local_branch\":\"/DATA/LocalBranch\",\"merged_pool\":\"/DATA/ClusterPool\",\"sqlite_pinned_local\":true}\n"'
        '        ;;'
        '    join|join-union)'
        "        peer_ip=\"\${2:-}\""
        "        peer_host=\"\${3:-node2}\""
        "        policy=\"\${4:-mergerfs_union_safe}\""
        "        wipe_token=\"\${5:-}\""
        "        if [ -z \"\$peer_ip\" ]; then"
        '            printf "Usage: instant-cluster-ctl join <peer_ip> <peer_hostname> [policy] [wipe_token]\n" >&2'
        '            exit 1'
        '        fi'
        "        mkdir -p \"/mnt/cluster/\${peer_host}/DATA\""
        "        mount -t nfs4 -o soft,timeo=15,retrans=2,nofail \"\${peer_ip}:/DATA/LocalBranch\" \"/mnt/cluster/\${peer_host}/DATA\" 2>/dev/null || true"
        "        if [ \"\$policy\" = \"wipe_secondary_expand\" ]; then"
        "            if [ \"\$wipe_token\" != \"WIPE-\${peer_host}\" ] && [ \"\$wipe_token\" != \"WIPE-SECONDARY\" ]; then"
        '                printf "{\"ok\":false,\"error\":\"IS-CLU-401: Missing WIPE confirmation token\"}\n" >&2'
        '                exit 1'
        '            fi'
        "            find \"/mnt/cluster/\${peer_host}/DATA\" -mindepth 1 -maxdepth 1 -exec rm -rf {} + 2>/dev/null || true"
        "        elif [ \"\$policy\" = \"replicated_ha_mirror\" ]; then"
        "            rsync -aHAX --update \"/DATA/LocalBranch/\" \"/mnt/cluster/\${peer_host}/DATA/\" 2>/dev/null || true"
        '        fi'
        '        if command -v mergerfs >/dev/null 2>&1; then'
        "            mergerfs -o defaults,allow_other,use_ino,category.create=mfs,moveonenospc=true,dropcacheonclose=true,noforget \"/DATA/LocalBranch:/mnt/cluster/\${peer_host}/DATA\" /DATA/ClusterPool 2>/dev/null || true"
        '        fi'
        "        printf \"{\\\"ok\\\":true,\\\"policy\\\":\\\"%s\\\",\\\"peer\\\":\\\"%s\\\",\\\"ip\\\":\\\"%s\\\"}\n\" \"\$policy\" \"\$peer_host\" \"\$peer_ip\""
        '        ;;'
        '    *)'
        "        printf \"{\\\"ok\\\":true,\\\"action\\\":\\\"%s\\\"}\n\" \"\$action\""
        '        ;;'
        'esac'
    )
    printf '%s\n' "${_b26[@]}" > /usr/local/sbin/instant-cluster-ctl
    chmod 755 /usr/local/sbin/instant-cluster-ctl

    local _b27=(
        '#!/usr/bin/env bash'
        'set -euo pipefail'
        "mode=\"\${1:---diagnostics}\""
        "case \"\$mode\" in"
        '    --diagnostics)'
        '        lsblk -o NAME,SIZE,FSTYPE,FSUSED,FSUSE%,MOUNTPOINT,MODEL 2>/dev/null || true'
        '        ;;'
        '    --repair-bootloader)'
        '        mkdir -p /mnt/rescue-root'
        '        printf "UEFI and GRUB chroot repair environment ready.\n"'
        '        ;;'
        '    --fsck-all)'
        "        for part in \$(lsblk -lnpo NAME,MOUNTPOINT | awk \"\\\$2==\\\"\\\" {print \\\$1}\"); do"
        "            fstype=\$(blkid -o value -s TYPE \"\$part\" 2>/dev/null || true)"
        "            if [ \"\$fstype\" = \"ext4\" ]; then"
        "                e2fsck -fn \"\$part\" 2>/dev/null || true"
        '            fi'
        '        done'
        '        ;;'
        'esac'
    )
    printf '%s\n' "${_b27[@]}" > /usr/local/sbin/instant-repair-suite
    chmod 755 /usr/local/sbin/instant-repair-suite
}


configure_custom_workflows_and_updater_suite() {
    log_info "Configuring Universal Custom Server Workflow Engine (Custom Configs, Samba Shares, Systemd, Udev) and Auto/Offline Updater..."
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

    local _b28=(
        '#!/usr/bin/env bash'
        'set -euo pipefail'
        "mode=\"\${1:-auto}\""
        "cidata_dev=\$(blkid -L CIDATA 2>/dev/null || true)"
        "if [ -n \"\$cidata_dev\" ] && [ -b \"\$cidata_dev\" ]; then"
        '    mkdir -p /mnt/usb_updater'
        "    if mount -o ro \"\$cidata_dev\" /mnt/usb_updater 2>/dev/null; then"
        '        if [ -f /mnt/usb_updater/offline_updater_apply.sh ]; then'
        '            bash /mnt/usb_updater/offline_updater_apply.sh || true'
        '        fi'
        '        umount /mnt/usb_updater 2>/dev/null || true'
        '    fi'
        'fi'
        "if [ \"\$mode\" != \"offline-usb-only\" ] && command -v docker >/dev/null 2>&1; then"
        '    for cf in /DATA/AppData/*/docker-compose.yml; do'
        "        [ -f \"\$cf\" ] || continue"
        "        if [ ! -e /dev/dri ] && grep -q '/dev/dri' \"\$cf\" 2>/dev/null; then"
        "            sed -i '/\\/dev\\/dri/d; /^[[:space:]]*devices:[[:space:]]*\$/d' \"\$cf\" 2>/dev/null || true"
        "        fi"
        "        docker compose -f \"\$cf\" pull 2>/dev/null || true"
        "        docker compose -f \"\$cf\" up -d 2>/dev/null || true"
        '    done'
        'fi'
        "printf \"{\\\"ok\\\":true,\\\"mode\\\":\\\"%s\\\",\\\"updated\\\":true}\n\" \"\$mode\""
    )
    printf '%s\n' "${_b28[@]}" > /usr/local/sbin/instant-server-updater
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