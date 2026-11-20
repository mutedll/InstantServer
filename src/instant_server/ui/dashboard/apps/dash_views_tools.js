// dash_views_tools.js — Pro Control, Web Terminal, System Files, and Main Dispatcher
function renderProControlTab() {
  const s = state.settings;
  const disksHtml = (state.disks || []).map(d => `<button type="button" class="badge badge-accent" data-action="browse-disk" data-mount="${esc(d.mount)}" style="padding:4px 8px;cursor:pointer;border:none;">${esc(d.mount)} (${esc(d.device)} • ${esc(d.pct)} used)</button>`).join(" ");
  const labels = [["all","All Files"],["configs","Configs"],["system","System"],["logs","Logs"],["apps","Apps"]];
  const labelBarHtml = `<div class="file-label-pills" style="display:flex;gap:6px;flex-wrap:wrap;margin-bottom:10px;align-items:center;">${disksHtml}<span style="margin-left:auto;display:flex;gap:5px;flex-wrap:wrap;">${labels.map(([id,txt]) => `<button type="button" class="win-btn win-btn-sm ${(state.fileLabelFilter||"all")===id?"win-btn-accent":""}" data-action="set-file-label" data-label="${id}">${txt}</button>`).join("")}</span></div>`;
  return `
    ${labelBarHtml}
    <div class="section-hdr">
      <div>
        <div class="section-title">Pro Mode: Network Speed, Firewall &amp; Backups</div>
        <div class="section-desc">Advanced controls for network throughput, memory behavior, firewall rules, and scheduled backups.</div>
      </div>
      ${s.ui_mode !== "pro" ? `
        <button type="button" class="win-btn win-btn-accent win-btn-sm" data-action="set-mode" data-mode="pro">Unlock Pro Mode Controls</button>
      ` : `<span class="badge badge-pro">Pro Mode Active</span>`}
    </div>

    <div class="settings-group">
      <div class="setting-row">
        <div>
          <div class="setting-title">TCP BBR Fast Network Streaming</div>
          <div class="setting-sub">Improves streaming and file transfer speeds for Jellyfin, Nextcloud, and WireGuard VPN.</div>
        </div>
        <button type="button" class="switch-btn ${s.enable_tcp_bbr ? "on" : ""}" data-action="toggle-setting" data-key="enable_tcp_bbr" aria-label="Toggle TCP BBR">
          <span class="switch-thumb"></span>
        </button>
      </div>

      <div class="setting-row">
        <div>
          <div class="setting-title" id="lblDashSwappiness">Memory Swap Tendency (${s.vm_swappiness}%)</div>
          <div class="setting-sub">Keeps running apps and databases in physical RAM longer and reduces SSD wear.</div>
        </div>
        <div class="range-wrap">
          <input type="range" min="1" max="60" step="1" value="${s.vm_swappiness}" data-field="vm_swappiness" />
          <span class="badge badge-accent" id="valDashSwappiness">${s.vm_swappiness}%</span>
        </div>
      </div>

      <div class="setting-row">
        <div>
          <div class="setting-title">UFW Firewall &amp; SSH Login Protection</div>
          <div class="setting-sub">Automatically blocks repeated failed SSH login attempts and protects open ports.</div>
        </div>
        <button type="button" class="switch-btn ${s.enable_fail2ban ? "on" : ""}" data-action="toggle-setting" data-key="enable_fail2ban" aria-label="Toggle Fail2ban">
          <span class="switch-thumb"></span>
        </button>
      </div>

      <div class="setting-row">
        <div>
          <div class="setting-title">Automated Weekly <code>/DATA/AppData</code> Backups</div>
          <div class="setting-sub">Compresses all container settings and databases into <code>/DATA/Backups</code> (keeping ${s.backup_retention_days} days).</div>
        </div>
        <div style="display:flex;align-items:center;gap:10px;flex-wrap:wrap;">
          <button type="button" class="win-btn win-btn-sm" data-action="run-backup-now">Run Backup Now</button>
          <button type="button" class="switch-btn ${s.enable_scheduled_backups ? "on" : ""}" data-action="toggle-setting" data-key="enable_scheduled_backups" aria-label="Toggle Scheduled Backups">
            <span class="switch-thumb"></span>
          </button>
        </div>
      </div>
    </div>
  `;
}

function renderTerminalTab() {
  const host = state.settings.hostname || "instant-server";
  const quickCmds = [
    { label: "MON System Status", cmd: "uptime && free -h && df -h /DATA" },
    { label: "DKR Running Containers", cmd: "docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'" },
    { label: "NET IP & Failover", cmd: "ip -br addr" },
    { label: " Sleep Mask Check", cmd: "systemctl status sleep.target suspend.target --no-pager" },
    { label: "MOB iPhone Backup Dir", cmd: "ls -lah /DATA/iPhone_Backups" },
    { label: "CPU Temps & Battery", cmd: "cat /sys/class/power_supply/BAT*/capacity /sys/class/power_supply/BAT*/status 2>/dev/null || echo 'AC Power'" },
    { label: " Recent Logs", cmd: "journalctl -n 25 --no-pager" },
    { label: "CLN Clear", cmd: "clear" }
  ];
  return `
    <div class="section-hdr">
      <div>
        <div class="section-title">Web SSH &amp; Command Terminal (Desktop &amp; Mobile Responsive)</div>
        <div class="section-desc">Execute live shell commands directly from your phone or browser without clutter, or open a full-screen PTY session.</div>
      </div>
      <div style="display:flex;gap:8px;flex-wrap:wrap;">
        <a class="win-btn win-btn-sm" href="https://${esc(host)}.local:9090/system/terminal" target="_blank" rel="noopener">Open Full PTY Terminal (:9090) ↗</a>
        <button type="button" class="win-btn win-btn-sm" data-action="term-clear">Clear Output</button>
      </div>
    </div>

    <div class="terminal-shell">
      <div class="terminal-topbar">
        <div style="display:flex;align-items:center;gap:10px;">
          <div class="terminal-dots">
            <span class="terminal-dot" style="background:#FF5F56;"></span>
            <span class="terminal-dot" style="background:#FFBD2E;"></span>
            <span class="terminal-dot" style="background:#27C93F;"></span>
          </div>
          <span style="font-family:var(--font-mono);font-size:12px;color:#E4E4E7;">admin@${esc(host)}:${esc(state.terminalCwd)} (SSH / Web Exec)</span>
        </div>
        <span class="badge badge-success">Connected • Zero-Sleep Active</span>
      </div>

      <div class="terminal-QuickBar" aria-label="Mobile Quick Commands">
        ${quickCmds.map(q => `
          <button type="button" class="term-pill" data-action="term-quick-cmd" data-cmd="${esc(q.cmd)}">${esc(q.label)}</button>
        `).join("")}
      </div>

      <div class="terminal-viewport" id="termViewport">${state.terminalHistory.map(item => `<div style="margin-bottom:10px;"><div style="color:#30D158;font-weight:600;">admin@${esc(host)}:${esc(item.cwd)}$ <span style="color:#FFFFFF;">${esc(item.cmd)}</span></div><div style="color:${item.exit_code === 0 ? "#D4D4D8" : "#FF453A"};margin-top:2px;">${esc(item.output)}</div></div>`).join("")}</div>

      <div class="terminal-input-row">
        <span class="terminal-prompt-lbl">admin@${esc(host)}:${esc(state.terminalCwd)}$</span>
        <input
          type="text"
          id="inputWebTerminalCmd"
          class="terminal-cmd-input"
          value="${esc(state.terminalInput)}"
          placeholder="Type any Linux / Docker command (e.g. ls -la /DATA, docker ps, tailscale status)..."
          autocomplete="off"
          autocorrect="off"
          autocapitalize="off"
          spellcheck="false"
        />
        <button type="button" class="win-btn win-btn-accent win-btn-sm" data-action="term-run-cmd">Run ↵</button>
      </div>
    </div>
  `;
}

function renderFilesTab() {
  const selected = state.systemFileCatalog.find(f => f.path === state.selectedSystemFile) || state.systemFileCatalog[0];
  const canSave = !selected.is_system || state.confirmSystemFileEdit;
  return `
    <div class="section-hdr">
      <div>
        <div class="section-title">Server File &amp; System Configuration Manager</div>
        <div class="section-desc">Inspect and edit <code>/DATA</code> user directories, custom scripts, and critical <code>/etc</code> Linux system config files with automatic <code>.bak</code> rollback protection.</div>
      </div>
      <div style="display:flex;gap:8px;flex-wrap:wrap;">
        <input type="text" id="inputNewSystemFilePath" class="text-input" style="width:240px;" placeholder="/etc/custom.conf or /DATA/AppData/my.conf" value="${esc(state.newCustomFilePath)}" />
        <button type="button" class="win-btn win-btn-sm" data-action="create-system-file">＋ Add / Open File Path</button>
      </div>
    </div>

    <div class="file-mgr-grid">
      <div class="file-tree-card">
        <div style="padding:12px 14px;border-bottom:1px solid var(--border-subtle);font-size:12px;font-weight:700;color:var(--text-secondary);text-transform:uppercase;letter-spacing:0.04em;">
          System &amp; /DATA Files (${state.systemFileCatalog.length})
        </div>
        ${state.systemFileCatalog.map(file => `
          <button
            type="button"
            class="file-tree-item ${file.path === selected.path ? "active" : ""}"
            data-action="select-system-file"
            data-path="${esc(file.path)}"
          >
            <div style="min-width:0;overflow:hidden;text-overflow:ellipsis;">
              <div style="font-family:var(--font-mono);font-size:12px;word-break:break-all;">${esc(file.path)}</div>
              <div style="font-size:11px;color:var(--text-muted);margin-top:2px;">${esc(file.category)}</div>
            </div>
            <span class="badge ${file.is_system ? "badge-warning" : "badge-success"}">${file.is_system ? "SYS" : "DATA"}</span>
          </button>
        `).join("")}
      </div>

      <div class="settings-group" style="margin-bottom:0;">
        ${selected.is_system ? `
          <div class="setting-row" style="background:rgba(255, 159, 10, 0.12);border-left:4px solid var(--warning);">
            <div>
              <div class="setting-title" style="color:var(--warning);">WARN System Configuration File Warning — ${esc(selected.risk_level)}</div>
              <div class="setting-sub" style="margin-top:4px;">
                ${esc(selected.warning)}<br/>
                <strong>Safety Guarantee:</strong> Saving automatically creates <code>${esc(selected.path)}.bak</code> first so you can restore the previous working version in 1 click.
              </div>
              <label style="display:flex;align-items:center;gap:8px;margin-top:10px;font-size:12.5px;font-weight:600;color:var(--text-primary);cursor:pointer;">
                <input type="checkbox" id="chkConfirmSysFileEdit" ${state.confirmSystemFileEdit ? "checked" : ""} data-action="toggle-sysfile-confirm" />
                <span>I understand modifying <code>${esc(selected.path)}</code> affects core system behavior (Unlock Save)</span>
              </label>
            </div>
          </div>
        ` : `
          <div class="setting-row" style="background:rgba(48, 209, 88, 0.08);border-left:4px solid var(--success);">
            <div>
              <div class="setting-title" style="color:var(--success);"> Safe User Data / Custom Workflow File (${esc(selected.path)})</div>
              <div class="setting-sub">${esc(selected.warning)}</div>
            </div>
          </div>
        `}

        <div class="setting-row" style="flex-direction:column;align-items:stretch;gap:10px;">
          <div style="display:flex;align-items:center;justify-content:space-between;gap:8px;flex-wrap:wrap;">
            <span style="font-family:var(--font-mono);font-size:12.5px;font-weight:600;color:var(--accent);">${esc(selected.path)}</span>
            <div style="display:flex;gap:8px;flex-wrap:wrap;">
              <button type="button" class="win-btn win-btn-sm" data-action="restore-sysfile-backup" data-path="${esc(selected.path)}">↺ Restore .bak Backup</button>
              <button
                type="button"
                class="win-btn win-btn-accent win-btn-sm"
                data-action="save-system-file"
                data-path="${esc(selected.path)}"
                ${!canSave ? "disabled" : ""}
              >
                ${!canSave ? "SEC Check Warning Box Above to Save" : "FS Save File & Create .bak"}
              </button>
            </div>
          </div>
          <textarea id="textareaSystemFileEditor" class="compose-editor" style="min-height:290px;" spellcheck="false">${esc(selected.content)}</textarea>
        </div>
      </div>
    </div>
  `;
}


function getDashDomainWarning(rawDom) {
  const dom = "." + String(rawDom || ".local").trim().toLowerCase().replace(/^\.+/, "");
  if ([".local", ".home.arpa", ".lan", ".internal", ".home"].includes(dom)) return "";
  if ([".dev", ".app", ".page", ".new", ".google", ".bank", ".gov"].includes(dom)) {
    return `Critical Browser Block: "${dom}" is HSTS-preloaded and forces HTTPS, blocking local HTTP access. Use .local, .home.arpa, .lan, or .internal.`;
  }
  return `Warning: Public domain "${dom}" can trigger browser TLS/HSTS errors or public DNS conflicts. Recommended: .local, .home.arpa, .lan, or .internal.`;
}

function renderHomeLauncherTab() {
  const s = state.settings || {};
  const host = s.address_host || s.hostname || "instant-server";
  const dom = s.address_domain || ".local";
  const fullAddr = `${host}${dom.startsWith(".") ? dom : "." + dom}`;
  const sysApps = [
    { id: "apps", name: "App Store", sub: "Catalog, Config & Updates", color: "#0a84ff", icon: `<svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="7" height="7" rx="1.5"/><rect x="14" y="3" width="7" height="7" rx="1.5"/><rect x="3" y="14" width="7" height="7" rx="1.5"/><rect x="14" y="14" width="7" height="7" rx="1.5"/></svg>` },
    { id: "files", name: "File Manager", sub: "Disks, Logs & Configs", color: "#ff9f0a", icon: `<svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/></svg>` },
    { id: "settings", name: "Settings", sub: "Address, Wi-Fi, Logs & Updates", color: "#30d158", icon: `<svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09a1.65 1.65 0 0 0-1-1.51 1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09a1.65 1.65 0 0 0 1.51-1 1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33h.09a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82v.09a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>` },
    { id: "terminal", name: "Open SSH", sub: "Web Terminal & SSH", color: "#bf5af2", icon: `<svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/></svg>` }
  ];
  const allApps = Array.isArray(state.apps) ? state.apps : [];
  const shortcuts = (state.homeShortcuts || []).map(id => allApps.find(a => a.id === id)).filter(Boolean);
  const addable = allApps.filter(a => !(state.homeShortcuts || []).includes(a.id));
  return `
    <div class="os-home-wrap">
      <div class="os-hero-bar">
        <div>
          <div style="font-size:19px;font-weight:700;">${esc(s.server_name || host)}</div>
          <div style="font-size:12.5px;color:var(--text-secondary);">Address: <a href="http://${esc(fullAddr)}" target="_blank" style="color:var(--accent);font-weight:600;">http://${esc(fullAddr)}</a> • SSH: <code>ssh ${esc(s.username || "admin")}@${esc(fullAddr)}</code></div>
        </div>
        <button type="button" class="win-btn win-btn-sm" data-action="switch-tab" data-tab="settings">Open Settings</button>
      </div>
      <div class="os-section-label">System Apps <span class="badge badge-accent">Pinned Core</span></div>
      <div class="os-app-grid">
        ${sysApps.map(sa => `
          <button type="button" class="os-app-tile os-sys-tile" data-action="switch-tab" data-tab="${sa.id}">
            <div class="os-app-icon" style="background:${sa.color}22;color:${sa.color};border:1px solid ${sa.color}55;">${sa.icon}</div>
            <div class="os-app-name">${esc(sa.name)}</div>
            <div class="os-app-sub">${esc(sa.sub)}</div>
          </button>
        `).join("")}
      </div>
      <div style="display:flex;align-items:center;justify-content:space-between;margin-top:18px;gap:10px;flex-wrap:wrap;">
        <div class="os-section-label" style="margin:0;">App Shortcuts <span style="font-weight:400;color:var(--text-secondary);font-size:11.5px;">(Rename, remove, or add from App Store)</span></div>
        ${addable.length ? `
          <div style="display:flex;gap:6px;align-items:center;">
            <select id="selectAddHomeShortcut" class="text-input" style="height:28px;font-size:12px;">
              ${addable.map(a => `<option value="${esc(a.id)}">${esc(a.title)}</option>`).join("")}
            </select>
            <button type="button" class="win-btn win-btn-accent win-btn-sm" data-action="add-home-shortcut">+ Add Shortcut</button>
          </div>
        ` : ""}
      </div>
      <div class="os-app-grid">
        ${shortcuts.map(app => {
          const customTitle = (state.shortcutNames && state.shortcutNames[app.id]) || app.title;
          return `
            <div class="os-app-tile">
              <div class="os-tile-actions">
                <button type="button" class="os-mini-icon-btn" data-action="rename-home-shortcut" data-id="${esc(app.id)}" title="Rename shortcut">✎</button>
                <button type="button" class="os-mini-icon-btn" data-action="remove-home-shortcut" data-id="${esc(app.id)}" title="Remove shortcut from Home">✕</button>
              </div>
              <a href="http://${esc(fullAddr)}:${app.port || 80}" target="_blank" rel="noopener" style="text-decoration:none;color:inherit;display:flex;flex-direction:column;align-items:center;gap:6px;">
                <div class="os-app-icon" style="background:rgba(10,132,255,0.14);color:var(--accent);font-weight:700;font-size:16px;">${esc(customTitle.slice(0, 2).toUpperCase())}</div>
                <div class="os-app-name">${esc(customTitle)}</div>
                <div class="os-app-sub">Port :${app.port || 80}</div>
              </a>
            </div>
          `;
        }).join("") || `<div class="setting-sub">No app shortcuts pinned yet. Use "+ Add Shortcut" or open App Store.</div>`}
      </div>
    </div>
  `;
}

function renderSettingsAppTab() {
  const s = state.settings || {};
  const sub = state.settingsSubTab || "general";
  const warn = getDashDomainWarning(s.address_domain || ".local");
  const tabs = [
    { id: "general", label: "General & Address" },
    { id: "network", label: "Network & Wi-Fi" },
    { id: "hardware", label: "Hardware & Power" },
    { id: "logs", label: "Logs & Retention" },
    { id: "updates", label: "Updates" },
    { id: "security", label: "Security & SSH" }
  ];
  let panel = "";
  if (sub === "general") {
    panel = `
      <div class="setting-row"><div><div class="setting-title">Server Display Name</div><div class="setting-sub">Friendly label shown on dashboard & boot screen</div></div>
        <input id="setServerName" class="text-input" value="${esc(s.server_name || s.hostname || "Instant Server")}" /></div>
      <div class="setting-row"><div><div class="setting-title">Full Server Address (Host &amp; Domain)</div><div class="setting-sub">Independent of Server Name — use .local, .home.arpa, .lan, or .internal</div></div>
        <div style="display:flex;gap:6px;"><input id="setAddrHost" class="text-input" style="width:130px;" value="${esc(s.address_host || s.hostname || "instant-server")}" /><input id="setAddrDomain" class="text-input" style="width:105px;" value="${esc(s.address_domain || ".local")}" /></div></div>
      <div id="dashDomainWarnBox" class="setting-row" style="display:${warn ? "flex" : "none"};background:rgba(255,159,10,0.12);border-left:4px solid var(--warning);font-size:12px;">${esc(warn)}</div>
      <div class="setting-row"><div><div class="setting-title">Interface Mode (Standard / Pro)</div><div class="setting-sub">Toggle advanced kernel, SSH, and Docker Pro controls</div></div>
        <button type="button" class="win-btn win-btn-sm" data-action="set-mode" data-mode="${state.mode === "pro" ? "standard" : "pro"}">Current: ${state.mode === "pro" ? "Pro Mode" : "Standard Mode"}</button></div>`;
  } else if (sub === "network") {
    panel = `
      <div class="setting-row"><div><div class="setting-title">Wi-Fi Failover SSID</div><div class="setting-sub">Automatically connects if Ethernet cable is unplugged</div></div>
        <input id="setWifiSsid" class="text-input" value="${esc(s.wifi_ssid || "")}" placeholder="Home Wi-Fi Name" /></div>
      <div class="setting-row"><div><div class="setting-title">Wi-Fi Password</div><div class="setting-sub">Stored securely in /etc/netplan/60-instant-wifi.yaml (chmod 600)</div></div>
        <input id="setWifiPass" type="password" class="text-input" value="${esc(s.wifi_password || "")}" placeholder="Enter Wi-Fi password" /></div>`;
  } else if (sub === "hardware") {
    panel = `
      <div class="setting-row"><div><div class="setting-title">Screen Blank Timeout (Seconds)</div><div class="setting-sub">0 = Always On, 300 = Blank after 5 minutes</div></div>
        <input id="setScreenBlank" type="number" min="0" max="3600" class="text-input" style="width:110px;" value="${Number(s.screen_blank_sec ?? 300)}" /></div>
      <div class="setting-row"><div><div class="setting-title">Battery Charge Cap &amp; UPS Shutdown %</div><div class="setting-sub">Stops charging at ${s.bat_stop || 60}% and shuts down safely at ${s.ups_pct || 15}%</div></div>
        <button type="button" class="win-btn win-btn-sm" data-action="dash-enable-cool-mode">${s.disable_cpu_turbo_boost ? "Cool Mode: ON" : "Enable Cool & Quiet"}</button></div>`;
  } else if (sub === "logs") {
    panel = `
      <div class="setting-row"><div><div class="setting-title">Daily Log Auto-Erase Retention (Days)</div><div class="setting-sub">Automatically rotates &amp; erases daily logs in <code>/var/log/instant-server/</code> after this many days</div></div>
        <input id="setLogRetentionDays" type="number" min="1" max="365" class="text-input" style="width:100px;" value="${Number(s.log_retention_days || 14)}" /></div>
      <div class="setting-row"><div><div class="setting-title">Inspect All Server, First-Boot &amp; USB Install Logs</div><div class="setting-sub">Opens <code>/var/log/instant-server/</code> directly in File Manager</div></div>
        <button type="button" class="win-btn win-btn-accent win-btn-sm" data-action="open-logs-in-files">Open Logs Folder</button></div>`;
  } else if (sub === "updates") {
    panel = `
      <div class="setting-row"><div><div class="setting-title">Automatic Security &amp; OS Updates</div><div class="setting-sub">Keep Ubuntu Server packages patched automatically</div></div>
        <button type="button" class="win-btn win-btn-sm" data-action="toggle-setting" data-key="auto_updates">${s.auto_updates !== false ? "On" : "Off"}</button></div>
      <div class="setting-row"><div><div class="setting-title">Auto-Update System Apps &amp; Dashboard</div><div class="setting-sub">Automatically update Instant Server core tools</div></div>
        <button type="button" class="win-btn win-btn-sm" data-action="toggle-setting" data-key="auto_update_system_apps">${s.auto_update_system_apps !== false ? "On" : "Off"}</button></div>
      <div class="setting-row"><div><div class="setting-title">Auto-Update Installed Docker Apps</div><div class="setting-sub">Pull latest container images nightly &amp; sync with App Store Updates</div></div>
        <button type="button" class="win-btn win-btn-sm" data-action="toggle-setting" data-key="auto_update_docker_apps">${s.auto_update_docker_apps !== false ? "On" : "Off"}</button></div>
      <div class="setting-row"><div><div class="setting-title">App Store &amp; System Updates Center</div><div class="setting-sub">Manually check or apply pending updates right now</div></div>
        <button type="button" class="win-btn win-btn-accent win-btn-sm" data-action="switch-tab" data-tab="updates">Go to Updates Center</button></div>`;
  } else {
    panel = `
      <div class="setting-row"><div><div class="setting-title">SSH Port</div><div class="setting-sub">Default 22 (automatically allowed in UFW firewall)</div></div>
        <input id="setSshPort" type="number" min="1" max="65535" class="text-input" style="width:100px;" value="${Number(s.ssh_port || 22)}" /></div>
      <div class="setting-row"><div><div class="setting-title">UFW Firewall &amp; Fail2ban Protection</div><div class="setting-sub">Blocks brute-force SSH attacks while keeping LAN ports open</div></div>
        <span class="badge badge-success">Active</span></div>`;
  }
  return `
    <div class="settings-app-shell">
      <div class="settings-tab-strip">
        ${tabs.map(t => `<button type="button" class="settings-tab-pill ${sub === t.id ? "active" : ""}" data-action="switch-settings-subtab" data-subtab="${t.id}">${esc(t.label)}</button>`).join("")}
      </div>
      <div class="settings-group settings-anim-panel">${panel}</div>
      <div style="display:flex;justify-content:flex-end;gap:8px;margin-top:10px;">
        <button type="button" class="win-btn win-btn-accent" data-action="save-dash-settings-form">Save &amp; Apply Settings</button>
      </div>
    </div>
  `;
}

function renderDashboard() {
  renderTelemetry();
  renderSidebar();
  document.querySelectorAll(".mobile-dock-btn").forEach(b => b.classList.toggle("active", b.getAttribute("data-tab") === state.activeTab));
  const pane = document.getElementById("mainPane");
  if (!pane) return;
  if (state.activeTab === "home") pane.innerHTML = renderHomeLauncherTab();
  else if (state.activeTab === "settings") pane.innerHTML = (typeof renderDetailedSettingsApp === "function" ? renderDetailedSettingsApp() : renderSettingsAppTab());
  else if (state.activeTab === "apps") pane.innerHTML = renderAppsTab();
  else if (state.activeTab === "minecraft") pane.innerHTML = (typeof renderProMinecraftApp === "function" ? renderProMinecraftApp() : renderMinecraftTab());
  else if (state.activeTab === "cluster") pane.innerHTML = renderClusterTab();
  else if (state.activeTab === "kiwix") pane.innerHTML = renderKiwixTab();
  else if (state.activeTab === "hardware") pane.innerHTML = renderHardwareTab();
  else if (state.activeTab === "terminal") pane.innerHTML = renderTerminalTab();
  else if (state.activeTab === "files") pane.innerHTML = (typeof renderFullFileManagerTab === "function" ? renderFullFileManagerTab() : renderFilesTab());
  else if (state.activeTab === "updates") pane.innerHTML = (typeof renderOfficialUpdatesCenter === "function" ? renderOfficialUpdatesCenter() : renderUpdatesTab());
  else if (state.activeTab === "handbook") pane.innerHTML = renderHandbookTab();
  else if (state.activeTab === "pro_control") pane.innerHTML = renderProControlTab();
}