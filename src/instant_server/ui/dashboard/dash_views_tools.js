// dash_views_tools.js — Pro Control, Web Terminal, System Files, and Main Dispatcher
function renderProControlTab() {
  const s = state.settings;
  return `
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

function renderDashboard() {
  renderTelemetry();
  renderSidebar();
  const pane = document.getElementById("mainPane");
  if (!pane) return;
  if (state.activeTab === "apps") pane.innerHTML = renderAppsTab();
  else if (state.activeTab === "minecraft") pane.innerHTML = renderMinecraftTab();
  else if (state.activeTab === "cluster") pane.innerHTML = renderClusterTab();
  else if (state.activeTab === "kiwix") pane.innerHTML = renderKiwixTab();
  else if (state.activeTab === "hardware") pane.innerHTML = renderHardwareTab();
  else if (state.activeTab === "terminal") pane.innerHTML = renderTerminalTab();
  else if (state.activeTab === "files") pane.innerHTML = renderFilesTab();
  else if (state.activeTab === "updates") pane.innerHTML = renderUpdatesTab();
  else if (state.activeTab === "handbook") pane.innerHTML = renderHandbookTab();
  else if (state.activeTab === "pro_control") pane.innerHTML = renderProControlTab();
}