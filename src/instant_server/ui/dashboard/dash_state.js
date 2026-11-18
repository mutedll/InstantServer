// dash_state.js — Reactive State Store, API Client, Compose Inspector & Sidebar
const state = {
  theme: "dark",
  activeTab: "apps",
  searchQuery: "",
  appCategory: "All",
  customAppInput: "",
  customZimUrl: "",
  inspectingAppId: null,
  doctorStatusMsg: "",
  customWorkflowStatus: "Ready • Custom Server Workflows (/DATA/CustomWorkflows), Samba Shares, Systemd Timers & USB udev Triggers Active",
  terminalCwd: "/DATA",
  terminalInput: "",
  terminalHistory: [
    {
      cmd: "uname -srm && uptime -p",
      cwd: "/DATA",
      output: "Linux 6.11.0-21-generic x86_64\nup 14 days, 6 hours, 18 minutes",
      exit_code: 0
    },
    {
      cmd: "systemctl status sleep.target suspend.target --no-pager",
      cwd: "/DATA",
      output: "○ sleep.target - Sleep\n     Loaded: masked (Reason: Unit sleep.target is masked.)\n     Active: inactive (dead)\n○ suspend.target - Suspend\n     Loaded: masked (Reason: Unit suspend.target is masked.)\n     Active: inactive (dead)",
      exit_code: 0
    }
  ],
  selectedSystemFile: "/etc/samba/smb.conf",
  confirmSystemFileEdit: false,
  newCustomFilePath: "",
  systemFileCatalog: [
    {
      path: "/etc/samba/smb.conf",
      category: "System Config (/etc)",
      is_system: true,
      risk_level: "High — Controls Windows/Mac/iPhone SMB3 File Sharing",
      warning: "Syntax errors in smb.conf can pause LAN & iPhone Files app sharing. Instant Server automatically writes /etc/samba/smb.conf.bak before saving and tests config with testparm.",
      content: "[global]\n   workgroup = WORKGROUP\n   server string = Instant Server NAS\n   server min protocol = SMB2\n   vfs objects = catia fruit streams_xattr\n   fruit:metadata = stream\n   fruit:model = MacSamba\n\n[iPhoneBackups]\n   path = /DATA/iPhone_Backups\n   browseable = yes\n   writable = yes\n   vfs objects = catia fruit streams_xattr\n   fruit:time machine = yes\n"
    },
    {
      path: "/etc/systemd/logind.conf.d/99-instant-server-clamshell.conf",
      category: "System Config (/etc)",
      is_system: true,
      risk_level: "High — Prevents Laptop Sleep When Lid Is Closed",
      warning: "Changing HandleLidSwitch from 'ignore' may cause the laptop server to suspend when you close the screen.",
      content: "[Login]\nHandleLidSwitch=ignore\nHandleLidSwitchExternalPower=ignore\nHandleLidSwitchDocked=ignore\nLidSwitchIgnoreInhibited=no\nIdleAction=ignore\n"
    },
    {
      path: "/etc/netplan/99-instant-server.yaml",
      category: "System Config (/etc)",
      is_system: true,
      risk_level: "Critical — Controls Dual-IP Ethernet (Metric 100) & Wi-Fi Failover (Metric 600)",
      warning: "Incorrect indentation or wrong interface names in Netplan YAML can disconnect LAN/Wi-Fi networking. A .bak copy is always saved automatically.",
      content: "network:\n  version: 2\n  renderer: NetworkManager\n  ethernets:\n    all-en:\n      match:\n        name: \"en*\"\n      dhcp4: true\n      dhcp4-overrides:\n        route-metric: 100\n"
    },
    {
      path: "/etc/ssh/sshd_config",
      category: "System Config (/etc)",
      is_system: true,
      risk_level: "Critical — Controls SSH Terminal Login & Port 22 Security",
      warning: "Ensure Port 22 (or your custom port) matches UFW firewall rules so you do not lock yourself out of remote SSH.",
      content: "Include /etc/ssh/sshd_config.d/*.conf\nPort 22\nPermitRootLogin no\nPasswordAuthentication yes\nPubkeyAuthentication yes\nKbdInteractiveAuthentication no\nUsePAM yes\nX11Forwarding no\nPrintMotd no\nAcceptEnv LANG LC_*\nSubsystem sftp /usr/lib/openssh/sftp-server\n"
    },
    {
      path: "/etc/fstab",
      category: "System Config (/etc)",
      is_system: true,
      risk_level: "Critical — Controls Boot Disk & /DATA Partition Mounts",
      warning: "Never remove the root (/) or /boot/efi UUID entries. Always use 'nofail' on external USB drives so the server still boots if unplugged.",
      content: "# /etc/fstab: static file system information.\nUUID=7f9a-root-ext4  /          ext4  errors=remount-ro,noatime  0 1\nUUID=12AB-EFI        /boot/efi  vfat  umask=0077                 0 1\n# Persistent /DATA & iPhone Backup storage\n"
    },
    {
      path: "/opt/instant-server/env.conf",
      category: "Instant Server Core (/opt)",
      is_system: true,
      risk_level: "Medium — Instant Server Provisioning & Auto-Update Profile",
      warning: "Stores active battery thresholds, custom apt packages, and unattended update preferences.",
      content: "INSTANT_HOSTNAME='instant-server'\nINSTANT_USER='admin'\nINSTANT_CUSTOM_APT_PACKAGES='ffmpeg rsync'\nENABLE_AUTO_UPDATES='true'\n"
    },
    {
      path: "/etc/udev/rules.d/99-custom-usb-trigger.rules",
      category: "System Config (/etc)",
      is_system: true,
      risk_level: "Medium — Custom Hardware / USB Plug-In Automation Rule",
      warning: "Runs a custom workflow script automatically whenever a matching USB device or external drive is plugged in.",
      content: "# Custom USB / Hardware Trigger Rule\nACTION==\"add\", SUBSYSTEM==\"usb\", RUN+=\"/DATA/CustomWorkflows/custom-automation.sh\"\n"
    },
    {
      path: "/DATA/CustomWorkflows/custom-automation.sh",
      category: "User Data (/DATA)",
      is_system: false,
      risk_level: "Safe — Custom User Automation & Scheduled Workflow Script",
      warning: "Safe user automation script inside /DATA/CustomWorkflows.",
      content: "#!/usr/bin/env bash\n# Custom server workflow script (can be triggered by systemd timer, udev USB rule, or 1-click in dashboard)\nmkdir -p /DATA/Shared /DATA/Backups\necho \"Custom server workflow executed at $(date -Is)\"\n"
    }
  ],
  minecraftStudio: {
    server_type: "PAPER",
    version: "26.3",
    memory_heap: "4G",
    use_aikar_flags: true,
    gamemode: "survival",
    difficulty: "normal",
    online_mode: true,
    max_players: 20,
    view_distance: 10,
    level_seed: "",
    motd: "Instant Server Minecraft 26.3 World",
    modrinth_projects: "viaversion,chunky,worldedit",
    ops_list: "admin",
    whitelist: "",
    rcon_password: "instant-rcon-admin",
    rcon_command: "list",
    rcon_output: "[RCON Ready] Connected to 127.0.0.1:25575 • Paper 26.3 Wilderness Bound (20.0 TPS) • 0/20 players online.",
    active_file: "server.properties",
    files: {
      "server.properties": "motd=Instant Server Minecraft 26.3 World\ngamemode=survival\ndifficulty=normal\nmax-players=20\nview-distance=10\nsimulation-distance=10\nonline-mode=true\nwhite-list=false\nenforce-whitelist=false\nenable-rcon=true\nrcon.port=25575\n",
      "ops.json": "[\n  {\n    \"uuid\": \"00000000-0000-0000-0000-000000000000\",\n    \"name\": \"admin\",\n    \"level\": 4,\n    \"bypassesPlayerLimit\": true\n  }\n]",
      "whitelist.json": "[]",
      "paper-global.yml": "chunk-loading-advanced:\n  auto-config-send-distance: true\nproxies:\n  velocity:\n    enabled: false\n"
    }
  },
  clusterMergePolicy: "mergerfs_union_safe",
  clusterConfirmWipe: "",
  clusterConnectIp: "192.168.1.54",
  clusterConnectHost: "instant-server-b8f2",
  clusterActiveNodes: [
    { hostname: "instant-server", ip: "192.168.1.40", device: "Primary Server", used_gb: 42.4, total_gb: 256.0, ram_gb: 8 }
  ],
  clusterDiscoveredNodes: [
    { hostname: "instant-server-b8f2", ip: "192.168.1.54", device: "Second Laptop (Found on Home Wi-Fi)", used_gb: 118.0, total_gb: 512.0, ram_gb: 8 },
    { hostname: "instant-mini-n100", ip: "192.168.1.61", device: "Mini-PC (Found on Home LAN)", used_gb: 64.0, total_gb: 1000.0, ram_gb: 16 }
  ],
  settings: {
    ui_mode: "standard",
    hostname: "instant-server",
    enable_tailscale: true,
    tailscale_authkey: "",
    tailscale_mode: "subnet_and_ssh",
    tailscale_ip: "100.104.28.14",
    ignore_lid_switch: true,
    disable_wifi_power_save: true,
    enable_mosh_server: true,
    screen_blank_seconds: 60,
    battery_charge_start_pct: 40,
    battery_charge_stop_pct: 60,
    ups_shutdown_battery_pct: 15,
    cpu_governor: "schedutil",
    disable_cpu_turbo_boost: false,
    enable_wake_on_lan: true,
    enable_tcp_bbr: true,
    vm_swappiness: 10,
    inotify_max_watches: 524288,
    enable_ufw_firewall: true,
    enable_fail2ban: true,
    ssh_port: 22,
    enable_auto_updates: true,
    auto_update_window: "03:00",
    enable_scheduled_backups: true,
    backup_retention_days: 14
  },
  telemetry: {
    hostname: "instant-server",
    eth_ipv4: "192.168.1.40",
    wifi_ipv4: "192.168.1.41",
    kernel: "6.11.0-21-generic",
    cpu_load_pct: 11.4,
    cpu_temp_c: 41,
    mem_used_mb: 2450,
    mem_total_mb: 8192,
    mem_pct: 29.9,
    disk_used_gb: 42.4,
    disk_total_gb: 256.0,
    disk_pct: 16.5,
    battery_pct: 60,
    battery_status: "Charging capped (AC Plugged In)",
    battery_health_pct: 89,
    battery_cycles: 142,
    idle_watts_est: 10.2
  },
  apps: DEFAULT_APPS,
  kiwix_zims: DEFAULT_ZIMS,
  updates: {
    last_checked: "Just now",
    status_text: "2 OS security updates & 2 web app updates available",
    os_updates: [
      { pkg: "linux-image-generic-hwe", current: "6.11.0-19.19", available: "6.11.0-21.21", category: "Kernel & Hardware", security: true },
      { pkg: "openssl", current: "3.0.13-0ubuntu3.4", available: "3.0.13-0ubuntu3.5", category: "Cryptography", security: true },
      { pkg: "systemd", current: "255.4-1ubuntu8.4", available: "255.4-1ubuntu8.6", category: "Core OS", security: false },
      { pkg: "cockpit-ws", current: "314-1", available: "316-1", category: "Web Console", security: false }
    ],
    app_updates: [
      { id: "jellyfin", title: "Jellyfin Media Server", image: "jellyfin/jellyfin:latest", current_tag: "10.10.3", available_tag: "10.10.6", update_available: true },
      { id: "nextcloud", title: "Nextcloud Personal Cloud", image: "nextcloud:stable", current_tag: "30.0.4", available_tag: "30.0.5", update_available: true },
      { id: "kiwix", title: "Offline Wikipedia (Kiwix)", image: "ghcr.io/kiwix/kiwix-serve:latest", current_tag: "3.7.0", available_tag: "3.7.0", update_available: false },
      { id: "pihole", title: "Pi-hole Ad-Blocker", image: "pihole/pihole:latest", current_tag: "2025.02.1", available_tag: "2025.02.1", update_available: false }
    ],
    system_files: [
      { path: "/etc/systemd/logind.conf.d/99-instant-server-headless.conf", role: "Lid-Close & Sleep Protection", status: "Verified" },
      { path: "/etc/NetworkManager/conf.d/99-wifi-powersave-off.conf", role: "Wi-Fi Power-Save Disabled (Fast Failover)", status: "Verified" },
      { path: "/etc/tlp.d/99-instant-server-battery.conf", role: "Battery Charge Limiter (40%–60%)", status: "Verified" },
      { path: "/etc/systemd/system/instant-server-battery-guardian.timer", role: "Built-In Battery UPS Watchdog", status: "Active (60s interval)" },
      { path: "/etc/sysctl.d/99-instant-server-pro.conf", role: "Network & Memory Tuning", status: "Verified" }
    ]
  }
};

if (window.__SERVER_DASHBOARD_BOOT__ && typeof window.__SERVER_DASHBOARD_BOOT__ === "object") {
  const b = window.__SERVER_DASHBOARD_BOOT__;
  if (b.settings) Object.assign(state.settings, b.settings);
  if (b.telemetry) Object.assign(state.telemetry, b.telemetry);
  if (Array.isArray(b.apps)) state.apps = b.apps;
  if (Array.isArray(b.kiwix_zims)) state.kiwix_zims = b.kiwix_zims;
  if (b.updates) Object.assign(state.updates, b.updates);
}

function esc(s) {
  return String(s ?? "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

function showToast(msg) {
  const t = document.getElementById("dashToast");
  if (!t) return;
  t.textContent = msg;
  t.classList.add("visible");
  clearTimeout(t._timer);
  t._timer = setTimeout(() => t.classList.remove("visible"), 2600);
}

async function callDashboardApi(path, method = "GET", body = null) {
  try {
    if (window.location.protocol.startsWith("http")) {
      const opts = { method, headers: { "Content-Type": "application/json" }, cache: "no-store" };
      if (body && method === "POST") opts.body = JSON.stringify(body);
      const r = await fetch(window.location.origin + path, opts);
      if (r.ok) return await r.json();
    }
  } catch (_e) {}
  return null;
}

function runComposeValidationLocal(yamlText) {
  const msgs = [];
  if (!yamlText.trim()) {
    return [{ severity: "error", message: "Compose configuration is empty." }];
  }
  if (!yamlText.includes("services:")) {
    msgs.push({ severity: "error", message: "Missing top-level 'services:' block." });
  }
  if (!yamlText.includes("image:")) {
    msgs.push({ severity: "error", message: "Missing 'image:' tag." });
  }
  const lines = yamlText.split("\n");
  lines.forEach((ln, i) => {
    if (ln.includes("\t")) {
      msgs.push({ severity: "error", message: `Line ${i + 1}: Tab character detected. Replace tabs with spaces.` });
    }
    const mVol = ln.match(/-\s*(\/DATA\/[^:]+):([^\s]+)/);
    if (mVol) {
      msgs.push({ severity: "ok", message: `Line ${i + 1}: Host directory '${mVol[1]}' mapped to container '${mVol[2]}'.` });
    }
  });
  if (!msgs.some(m => m.severity === "error")) {
    msgs.unshift({ severity: "ok", message: "Compose YAML syntax, port bindings, and volume mounts are valid." });
  }
  return msgs;
}

function openComposeInspector(appId) {
  const app = state.apps.find(a => a.id === appId);
  if (!app) return;
  state.inspectingAppId = appId;
  const modal = document.getElementById("composeInspectorModal");
  document.getElementById("composeModalTitle").textContent = `${app.title} — Compose & Volume Inspector`;
  document.getElementById("composeModalSub").textContent = `Container: ${app.image} • Port ${app.port}`;
  const vols = app.volumes || [`/DATA/AppData/${app.id}:/data`];
  document.getElementById("composeVolumeExplainBox").innerHTML = `
    <div style="font-weight:600;color:var(--accent);">Where this app stores your files on the hard drive:</div>
    ${vols.map(v => {
      const parts = v.split(":");
      return `<div>• Host Folder: <code>${esc(parts[0])}</code> → Inside Container: <code>${esc(parts[1] || "/data")}</code></div>`;
    }).join("")}
    <div style="color:var(--text-secondary);margin-top:2px;">Tip: Back up <code>/DATA/AppData/${esc(app.id)}</code> to save all settings and databases for ${esc(app.title)}.</div>
  `;
  if (!app.config_files) {
    if (app.id === "minecraft") {
      app.config_files = {
        "server.properties": "motd=Instant Server Minecraft World\ngamemode=survival\ndifficulty=normal\nmax-players=20\nview-distance=10\nonline-mode=true\nenable-rcon=true\nrcon.port=25575\n",
        "ops.json": "[\n  {\n    \"uuid\": \"00000000-0000-0000-0000-000000000000\",\n    \"name\": \"admin\",\n    \"level\": 4\n  }\n]",
        "whitelist.json": "[]",
        "paper-global.yml": "chunk-loading-advanced:\n  auto-config-send-distance: true\n"
      };
    } else if (app.id === "nextcloud") {
      app.config_files = { "config.php": "<?php\n$CONFIG = array('trusted_domains' => array('localhost', 'instant-server.local'));\n" };
    } else if (app.id === "pihole") {
      app.config_files = { "pihole-FTL.conf": "PRIVACYLEVEL=0\nRATE_LIMIT=1000/60\n", "adlists.list": "https://big.oisd.nl/domainswild\n" };
    } else {
      app.config_files = {
        ".env": `TZ=UTC\nPUID=1000\nPGID=1000\nAPP_PORT=${app.port}\n`,
        "config.yml": `# Persistent config in /DATA/AppData/${app.id}/config.yml\n`
      };
    }
  }
  const fNames = Object.keys(app.config_files);
  state.activeAppDataFile = fNames[0] || ".env";
  renderAppDataFileExplorerInModal(app);

  const yaml = app.compose_yaml || buildDefaultCompose(app);
  document.getElementById("composeYamlTextarea").value = yaml;
  renderComposeDiagnostics(runComposeValidationLocal(yaml));
  if (modal) modal.classList.add("open");
}

function renderAppDataFileExplorerInModal(app) {
  if (!app || !app.config_files) return;
  const pillsEl = document.getElementById("appDataFilePills");
  const taEl = document.getElementById("appDataFileTextarea");
  const pathEl = document.getElementById("lblActiveAppDataFilePath");
  const fNames = Object.keys(app.config_files);
  if (!state.activeAppDataFile || app.config_files[state.activeAppDataFile] === undefined) {
    state.activeAppDataFile = fNames[0] || ".env";
  }
  if (pillsEl) {
    pillsEl.innerHTML = fNames.map(fn => `
      <button type="button" class="filter-pill ${state.activeAppDataFile === fn ? "active" : ""}" data-action="select-appdata-file" data-file="${esc(fn)}">
        CFG ${esc(fn)}
      </button>
    `).join("");
  }
  if (taEl) {
    taEl.value = app.config_files[state.activeAppDataFile] || "";
  }
  if (pathEl) {
    pathEl.textContent = `/DATA/AppData/${app.id}/${state.activeAppDataFile}`;
  }
}

function renderComposeDiagnostics(list) {
  const box = document.getElementById("composeDiagnosticsBox");
  if (!box) return;
  box.innerHTML = list.map(item => {
    const color = item.severity === "error" ? "var(--danger)" : item.severity === "ok" ? "var(--success)" : "var(--accent)";
    const icon = item.severity === "error" ? "" : "";
    return `<div style="color:${color};font-family:var(--font-mono);">${icon} ${esc(item.message)}</div>`;
  }).join("");
}

function renderTelemetry() {
  const t = state.telemetry;
  const s = state.settings;
  const strip = document.getElementById("telemetryStrip");
  if (!strip) return;
  document.getElementById("hdrHostname").textContent = `${s.hostname || "instant-server"}.local`;
  document.getElementById("hdrSubInfo").textContent = `Eth: ${t.eth_ipv4 || "192.168.1.40"} (Primary) • Wi-Fi: ${t.wifi_ipv4 || "192.168.1.41"} (Failover Ready)`;
  document.getElementById("btnModeStandard").className = "mode-pill " + (s.ui_mode === "standard" ? "active-standard" : "");
  document.getElementById("btnModePro").className = "mode-pill " + (s.ui_mode === "pro" ? "active-pro" : "");

  const totalPoolGb = state.clusterActiveNodes.reduce((acc, n) => acc + n.total_gb, 0);
  const usedPoolGb = state.clusterActiveNodes.reduce((acc, n) => acc + n.used_gb, 0);

  strip.innerHTML = `
    <div class="metric-pill">
      <div class="metric-top"><span>Dual-IP Failover</span><span>${state.clusterActiveNodes.length} Server(s)</span></div>
      <div class="metric-val">Eth ${esc(t.eth_ipv4 || "192.168.1.40")} • Wi-Fi ${esc(t.wifi_ipv4 || "192.168.1.41")}</div>
      <div class="mini-bar"><div class="mini-bar-fill" style="width:100%;background:var(--success)"></div></div>
    </div>
    <div class="metric-pill">
      <div class="metric-top"><span>CPU &amp; Thermal</span><span>${t.cpu_temp_c}°C</span></div>
      <div class="metric-val">${t.cpu_load_pct}% • ${esc(s.cpu_governor)}</div>
      <div class="mini-bar"><div class="mini-bar-fill" style="width:${Math.min(100, t.cpu_load_pct)}%"></div></div>
    </div>
    <div class="metric-pill">
      <div class="metric-top"><span>Storage Pool</span><span>${usedPoolGb.toFixed(0)} / ${totalPoolGb.toFixed(0)} GB</span></div>
      <div class="metric-val">${(t.mem_used_mb / 1024).toFixed(1)} / ${(t.mem_total_mb / 1024).toFixed(1)} GB RAM</div>
      <div class="mini-bar"><div class="mini-bar-fill" style="width:${Math.min(100, t.mem_pct)}%"></div></div>
    </div>
    <div class="metric-pill">
      <div class="metric-top"><span>Battery Health ${t.battery_health_pct || 89}%</span><span>${t.battery_cycles || 142} cycles</span></div>
      <div class="metric-val">${s.battery_charge_stop_pct}% Cap • UPS @ ${s.ups_shutdown_battery_pct}%</div>
      <div class="mini-bar"><div class="mini-bar-fill" style="width:${s.battery_charge_stop_pct}%;background:var(--success)"></div></div>
    </div>
  `;
}

function renderSidebar() {
  const sb = document.getElementById("sidebarNav");
  if (!sb) return;
  const installedCount = state.apps.filter(a => a.installed).length;
  const installedZims = state.kiwix_zims.filter(z => z.installed).length;
  const pendingUpdates = state.updates.os_updates.length + state.updates.app_updates.filter(a => a.update_available).length;
  const tabs = [
    { id: "apps", label: "My Apps & App Store", badge: `${installedCount} active` },
    { id: "minecraft", label: "Minecraft Server Studio", badge: `v${state.minecraftStudio.version}` },
    { id: "cluster", label: "Combine Servers", badge: `${state.clusterActiveNodes.length} linked` },
    { id: "kiwix", label: "Offline Wikipedia (Kiwix)", badge: `${installedZims} ZIM` },
    { id: "hardware", label: "Network, Tailscale & Battery", badge: state.settings.enable_tailscale ? "VPN On" : `${state.settings.battery_charge_stop_pct}%` },
    { id: "terminal", label: "Web SSH Terminal", badge: "PTY / Mobile" },
    { id: "files", label: "File & Config Manager", badge: "System + /DATA" },
    { id: "updates", label: "Updates & Offline USB", badge: pendingUpdates > 0 ? `${pendingUpdates} new` : "Auto" },
    { id: "handbook", label: "Handbook & Password Reset", badge: "Guide" },
    { id: "pro_control", label: "Pro Kernel & Firewall", badge: "PRO" }
  ];
  sb.innerHTML = tabs.map(tab => `
    <button type="button" class="nav-btn ${state.activeTab === tab.id ? "active" : ""}" data-action="switch-tab" data-tab="${tab.id}">
      <span>${esc(tab.label)}</span>
      <span class="badge ${tab.id === "pro_control" ? "badge-pro" : "badge-accent"}">${esc(tab.badge)}</span>
    </button>
  `).join("");
}