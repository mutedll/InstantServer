// FILE: src/instant_server/ui/dashboard/dash_settings_app.js
// Full-Featured Sleek Animated Server Settings App (Windows 11 / macOS Settings UX)

function renderDetailedSettingsApp() {
  const s = state.settings || {};
  const sub = state.settingsSubTab || "general";
  const isPro = state.mode === "pro";
  const warn = typeof getDashDomainWarning === "function" ? getDashDomainWarning(s.address_domain || ".local") : "";

  const categories = [
    { id: "general", label: "General & Address", desc: "Name, domain, theme & mode" },
    { id: "network", label: "Network & Wi-Fi", desc: "Wi-Fi, static IP, DNS & Tailscale" },
    { id: "hardware", label: "Hardware & Power", desc: "CPU, lid-close, battery & UPS" },
    { id: "storage", label: "Storage & Disks", desc: "Disk Manager, volumes & backups" },
    { id: "logs", label: "Logs & Retention", desc: "Daily auto-erase & log viewer" },
    { id: "updates", label: "Updates", desc: "OS, system apps & Docker updates" },
    { id: "security", label: "Security & SSH", desc: "SSH port, keys, UFW & Fail2ban" },
    ...(isPro ? [{ id: "pro_kernel", label: "Pro & Kernel", desc: "BBR, swappiness, ZRAM & sysctl" }] : [])
  ];

  const toggleBtn = (key, defaultVal = true) => {
    const on = s[key] !== undefined ? Boolean(s[key]) : defaultVal;
    return `<button type="button" class="set-toggle ${on ? "on" : ""}" data-action="toggle-setting" data-key="${key}" aria-pressed="${on}"><span class="set-toggle-knob"></span><span class="set-toggle-txt">${on ? "On" : "Off"}</span></button>`;
  };

  let bodyHtml = "";
  if (sub === "general") {
    const domPresets = [".local", ".home.arpa", ".lan", ".internal"];
    const curDom = String(s.address_domain || ".local").trim().toLowerCase();
    bodyHtml = `
      <div class="set-section-hdr">General &amp; Server Address</div>
      <div class="set-card">
        <div class="set-row">
          <div><div class="set-title">Server Display Name</div><div class="set-sub">Friendly name shown on Dashboard &amp; Boot Card (independent of address)</div></div>
          <input id="setServerName" class="text-input set-inp" value="${esc(s.server_name || s.hostname || "Instant Server")}" />
        </div>
        <div class="set-row">
          <div><div class="set-title">Full Custom Server Address</div><div class="set-sub">Customize both host name and domain/TLD independently</div></div>
          <div style="display:flex;gap:6px;flex-wrap:wrap;">
            <input id="setAddrHost" class="text-input" style="width:135px;" value="${esc(s.address_host || s.hostname || "instant-server")}" placeholder="myhome" />
            <input id="setAddrDomain" class="text-input" style="width:110px;" value="${esc(s.address_domain || ".local")}" placeholder=".local" />
          </div>
        </div>
        <div class="set-row" style="gap:6px;justify-content:flex-start;flex-wrap:wrap;">
          <span style="font-size:11.5px;color:var(--text-secondary);">Recommended private domains:</span>
          ${domPresets.map(d => `<button type="button" class="win-btn win-btn-sm ${curDom === d ? "win-btn-accent" : ""}" data-action="set-dash-domain-preset" data-domain="${d}">${d}</button>`).join("")}
        </div>
        <div id="dashDomainWarnBox" class="set-warn-banner" style="display:${warn ? "block" : "none"};">${esc(warn)}</div>
        <div class="set-row">
          <div><div class="set-title">Experience Mode (Standard / Pro)</div><div class="set-sub">Pro mode unlocks Kernel tuning, raw Compose inspection, and advanced SSH controls</div></div>
          <div style="display:flex;gap:6px;">
            <button type="button" class="win-btn win-btn-sm ${!isPro ? "win-btn-accent" : ""}" data-action="set-mode" data-mode="standard">Standard</button>
            <button type="button" class="win-btn win-btn-sm ${isPro ? "win-btn-accent" : ""}" data-action="set-mode" data-mode="pro">Pro</button>
          </div>
        </div>
        <div class="set-row">
          <div><div class="set-title">Appearance Theme</div><div class="set-sub">Switch between Dark and Light dashboard themes</div></div>
          <button type="button" class="win-btn win-btn-sm" data-action="toggle-theme">Switch Theme (${state.theme === "dark" ? "Dark" : "Light"})</button>
        </div>
      </div>`;
  } else if (sub === "network") {
    bodyHtml = `
      <div class="set-section-hdr">Network, Wi-Fi &amp; Remote Access</div>
      <div class="set-card">
        <div class="set-row">
          <div><div class="set-title">Wi-Fi Network Name (SSID)</div><div class="set-sub">Automatic failover when Ethernet cable is disconnected</div></div>
          <input id="setWifiSsid" class="text-input set-inp" value="${esc(s.wifi_ssid || "")}" placeholder="Home Wi-Fi SSID" />
        </div>
        <div class="set-row">
          <div><div class="set-title">Wi-Fi Password</div><div class="set-sub">Written to <code>/etc/netplan/60-instant-wifi.yaml</code> with <code>chmod 600</code></div></div>
          <input id="setWifiPass" type="password" class="text-input set-inp" value="${esc(s.wifi_password || "")}" placeholder="••••••••" />
        </div>
        <div class="set-row">
          <div><div class="set-title">Static LAN IP (CIDR) &amp; Gateway</div><div class="set-sub">Leave blank for automatic router DHCP (e.g. 192.168.1.50/24)</div></div>
          <div style="display:flex;gap:6px;flex-wrap:wrap;">
            <input id="setStaticIp" class="text-input" style="width:140px;" value="${esc(s.static_ip_cidr || "")}" placeholder="DHCP Auto" />
            <input id="setStaticGw" class="text-input" style="width:120px;" value="${esc(s.static_gateway || "")}" placeholder="Gateway" />
          </div>
        </div>
        <div class="set-row">
          <div><div class="set-title">Disable Wi-Fi Power Saving</div><div class="set-sub">Prevents wireless disconnects during 24/7 idle operation</div></div>
          ${toggleBtn("disable_wifi_power_save", true)}
        </div>
        <div class="set-row">
          <div><div class="set-title">Tailscale Remote VPN Auth Key</div><div class="set-sub">Reach your server securely from outside your home without port-forwarding</div></div>
          <div style="display:flex;gap:6px;flex-wrap:wrap;">
            <input id="inputDashTailscaleKey" class="text-input" style="width:170px;" value="${esc(s.tailscale_authkey || "")}" placeholder="tskey-auth-..." />
            <button type="button" class="win-btn win-btn-sm win-btn-accent" data-action="save-tailscale-config">Connect</button>
          </div>
        </div>
      </div>`;
  } else if (sub === "hardware") {
    bodyHtml = `
      <div class="set-section-hdr">Hardware Telemetry, Laptop Lid &amp; Battery UPS</div>
      <div class="set-card">
        <div class="set-row">
          <div><div class="set-title">Ignore Laptop Lid Close</div><div class="set-sub">Keeps the server running 24/7 with the screen closed</div></div>
          ${toggleBtn("ignore_lid_switch", true)}
        </div>
        <div class="set-row">
          <div><div class="set-title">Console Screen Blank Timeout (Seconds)</div><div class="set-sub">0 = Always On, 300 = Blank after 5 minutes (press any key to wake)</div></div>
          <input id="setScreenBlank" type="number" min="0" max="3600" class="text-input" style="width:105px;" value="${Number(s.screen_blank_sec ?? 300)}" />
        </div>
        <div class="set-row">
          <div><div class="set-title">Battery Charge Cap (Start % / Stop %)</div><div class="set-sub">Protects laptop battery from swelling while plugged in 24/7</div></div>
          <div style="display:flex;gap:6px;">
            <input id="setBatStart" type="number" min="20" max="90" class="text-input" style="width:72px;" value="${Number(s.bat_start ?? 40)}" />
            <input id="setBatStop" type="number" min="50" max="100" class="text-input" style="width:72px;" value="${Number(s.bat_stop ?? 60)}" />
          </div>
        </div>
        <div class="set-row">
          <div><div class="set-title">UPS Outage Auto-Shutdown (%)</div><div class="set-sub">Gracefully powers off during a power cut before battery empties</div></div>
          <input id="setUpsPct" type="number" min="5" max="50" class="text-input" style="width:85px;" value="${Number(s.ups_pct ?? 15)}" />
        </div>
        <div class="set-row">
          <div><div class="set-title">CPU Thermal Governor &amp; Cool Mode</div><div class="set-sub">Current governor: <strong>${esc(s.cpu_governor || "schedutil")}</strong></div></div>
          <button type="button" class="win-btn win-btn-sm" data-action="dash-enable-cool-mode">${s.disable_cpu_turbo_boost ? "Cool & Quiet: ON" : "Enable Cool & Quiet"}</button>
        </div>
      </div>`;
  } else if (sub === "storage") {
    bodyHtml = `
      <div class="set-section-hdr">Storage, Disk Management &amp; Backups</div>
      <div class="set-card">
        <div class="set-row">
          <div><div class="set-title">Interactive Disk Manager (Format, Split, Create Volumes)</div><div class="set-sub">Full partition manager with GPT/MBR init, shrink/extend, format, and wipe</div></div>
          <button type="button" class="win-btn win-btn-accent win-btn-sm" data-action="open-disk-manager-tool">Open Disk Manager</button>
        </div>
        <div class="set-row">
          <div><div class="set-title">Browse All Disks &amp; Mounts</div><div class="set-sub">Inspect <code>/</code>, <code>/DATA</code>, and external USB drives in File Manager</div></div>
          <button type="button" class="win-btn win-btn-sm" data-action="switch-tab" data-tab="files">Open File Manager</button>
        </div>
        <div class="set-row">
          <div><div class="set-title">Create AppData Backup Snapshot Now</div><div class="set-sub">Archives <code>/DATA/AppData</code> to <code>/DATA/Backups/</code></div></div>
          <button type="button" class="win-btn win-btn-sm" data-action="run-backup-now">Run Backup</button>
        </div>
      </div>`;
  } else if (sub === "logs") {
    bodyHtml = `
      <div class="set-section-hdr">Centralized Logs &amp; Daily Auto-Erase Policy</div>
      <div class="set-card">
        <div class="set-row">
          <div><div class="set-title">Erase Daily Logs After (Days)</div><div class="set-sub">Updates <code>/etc/logrotate.d/instant-server</code> for first-boot, install, command, and background logs</div></div>
          <input id="setLogRetentionDays" type="number" min="1" max="365" class="text-input" style="width:95px;" value="${Number(s.log_retention_days || 14)}" />
        </div>
        <div class="set-row">
          <div><div class="set-title">View All Server &amp; Installer Logs</div><div class="set-sub">Opens <code>/var/log/instant-server/</code> directly inside the File Manager app</div></div>
          <button type="button" class="win-btn win-btn-accent win-btn-sm" data-action="open-logs-in-files">Open Logs Folder</button>
        </div>
      </div>`;
  } else if (sub === "updates") {
    bodyHtml = `
      <div class="set-section-hdr">Automatic Updates &amp; App Store Sync</div>
      <div class="set-card">
        <div class="set-row">
          <div><div class="set-title">Automatic Ubuntu OS Security Updates</div><div class="set-sub">Installs unattended security patches overnight</div></div>
          ${toggleBtn("auto_updates", true)}
        </div>
        <div class="set-row">
          <div><div class="set-title">Auto-Update System Apps &amp; Dashboard</div><div class="set-sub">Keeps File Manager, Settings, and Disk Manager up to date</div></div>
          ${toggleBtn("auto_update_system_apps", true)}
        </div>
        <div class="set-row">
          <div><div class="set-title">Auto-Update Installed Docker Apps</div><div class="set-sub">Automatically pulls updated container images from registries</div></div>
          ${toggleBtn("auto_update_docker_apps", true)}
        </div>
        <div class="set-row">
          <div><div class="set-title">App Store Updates Center</div><div class="set-sub">View pending OS &amp; container updates or apply Offline Updater USB</div></div>
          <button type="button" class="win-btn win-btn-accent win-btn-sm" data-action="switch-tab" data-tab="updates">Open Updates Center</button>
        </div>
      </div>`;
  } else if (sub === "security") {
    bodyHtml = `
      <div class="set-section-hdr">Security, SSH &amp; Firewall</div>
      <div class="set-card">
        <div class="set-row">
          <div><div class="set-title">SSH Server Port</div><div class="set-sub">Default 22 (automatically opened in UFW firewall)</div></div>
          <input id="setSshPort" type="number" min="1" max="65535" class="text-input" style="width:95px;" value="${Number(s.ssh_port || 22)}" />
        </div>
        <div class="set-row">
          <div><div class="set-title">UFW Firewall Protection</div><div class="set-sub">Blocks unauthorized ports while keeping Dashboard, SSH, and mDNS open</div></div>
          ${toggleBtn("enable_ufw", true)}
        </div>
        <div class="set-row">
          <div><div class="set-title">Fail2ban SSH Intrusion Guard</div><div class="set-sub">Temporarily bans IPs after repeated failed SSH logins</div></div>
          ${toggleBtn("enable_fail2ban", true)}
        </div>
        <div class="set-row">
          <div><div class="set-title">Open Interactive SSH Terminal</div><div class="set-sub">Launch the built-in browser terminal or copy SSH command</div></div>
          <button type="button" class="win-btn win-btn-sm" data-action="switch-tab" data-tab="terminal">Open SSH</button>
        </div>
      </div>`;
  } else {
    bodyHtml = `
      <div class="set-section-hdr">Pro Mode Kernel &amp; Sysctl Tuning</div>
      <div class="set-card">
        <div class="set-row">
          <div><div class="set-title">TCP BBR Congestion Control</div><div class="set-sub">Maximizes LAN &amp; VPN media streaming throughput</div></div>
          ${toggleBtn("enable_tcp_bbr", true)}
        </div>
        <div class="set-row">
          <div><div class="set-title">VM Swappiness (1–100)</div><div class="set-sub">Lower values (10) keep active server apps in RAM</div></div>
          <input id="setSwappiness" type="number" min="1" max="100" class="text-input" style="width:90px;" value="${Number(s.vm_swappiness ?? 10)}" />
        </div>
        <div class="set-row">
          <div><div class="set-title">Run Self-Heal &amp; Maintenance Watchdog</div><div class="set-sub">Prunes unused Docker images, vacuums journal logs, checks network</div></div>
          <button type="button" class="win-btn win-btn-sm" data-action="run-custom-workflow">Run Watchdog</button>
        </div>
      </div>`;
  }

  return `
    <div class="settings-layout">
      <nav class="settings-nav-rail" aria-label="Settings Categories">
        ${categories.map(c => `
          <button type="button" class="set-nav-btn ${sub === c.id ? "active" : ""}" data-action="switch-settings-subtab" data-subtab="${c.id}">
            <div class="set-nav-title">${esc(c.label)}</div>
            <div class="set-nav-desc">${esc(c.desc)}</div>
          </button>`).join("")}
      </nav>
      <div class="settings-detail-pane">
        <div class="settings-anim-panel">${bodyHtml}</div>
        <div class="set-save-bar">
          <span style="font-size:12px;color:var(--text-secondary);">Changes persist to <code>/opt/instant-server/server_settings.json</code> and apply immediately.</span>
          <button type="button" class="win-btn win-btn-accent" data-action="save-dash-settings-form">Save &amp; Apply Settings</button>
        </div>
      </div>
    </div>`;
}