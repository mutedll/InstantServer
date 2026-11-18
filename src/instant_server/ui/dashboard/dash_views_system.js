// dash_views_system.js — Kiwix ZIMs, Hardware & UPS, Updates, and Handbook Views
function renderKiwixTab() {
  const host = state.settings.hostname || "instant-server";
  return `
    <div class="section-hdr">
      <div>
        <div class="section-title">Offline Wikipedia &amp; Kiwix Archive Manager</div>
        <div class="section-desc">Download official Kiwix <code>.zim</code> encyclopedias directly to <code>/DATA/AppData/kiwix/zim/</code> for 100% offline reading on any home device.</div>
      </div>
      <a class="win-btn win-btn-accent win-btn-sm" href="http://${esc(host)}.local:8086" target="_blank" rel="noopener">Open Kiwix Reader (:8086) ↗</a>
    </div>

    <div class="cards-grid">
      ${state.kiwix_zims.map(zim => `
        <div class="app-card">
          <div>
            <div class="app-card-top">
              <div>
                <div class="app-card-title">${esc(zim.title)}</div>
                <div class="app-card-meta">${zim.size_gb} GB • ${esc(zim.articles)}</div>
              </div>
              <span class="badge ${zim.installed ? "badge-success" : "badge-accent"}">${zim.installed ? "Downloaded" : `${zim.size_gb} GB`}</span>
            </div>
            <div class="app-card-desc" style="margin-top:8px;">${esc(zim.description)}</div>
          </div>
          <div class="app-card-actions">
            <span style="font-size:11.5px;font-family:var(--font-mono);color:var(--text-muted);">${esc(zim.filename)}</span>
            ${zim.installed
              ? `<button type="button" class="win-btn win-btn-danger win-btn-sm" data-action="zim-op" data-zim="${zim.id}" data-op="remove">Remove ZIM</button>`
              : `<button type="button" class="win-btn win-btn-accent win-btn-sm" data-action="zim-op" data-zim="${zim.id}" data-op="download">⬇ Download ZIM (${zim.size_gb} GB)</button>`
            }
          </div>
        </div>
      `).join("")}
    </div>

    ${state.settings.ui_mode === "pro" ? `
      <div class="settings-group">
        <div class="setting-row">
          <div>
            <div class="setting-title">Custom Kiwix <code>.zim</code> URL or Mirror Import</div>
            <div class="setting-sub">Pull any custom <code>.zim</code> archive from <code>download.kiwix.org/zim/</code> with automatic download resume.</div>
          </div>
          <div style="display:flex;gap:8px;flex-wrap:wrap;width:100%;max-width:420px;">
            <input type="url" id="inputCustomZimUrl" class="text-input" style="flex:1;" placeholder="https://download.kiwix.org/zim/..." value="${esc(state.customZimUrl)}" />
            <button type="button" class="win-btn win-btn-accent win-btn-sm" data-action="add-custom-zim">Pull ZIM</button>
          </div>
        </div>
      </div>
    ` : ""}
  `;
}

function renderHardwareTab() {
  const s = state.settings;
  const t = state.telemetry;
  const host = s.hostname || "instant-server";
  return `
    <div class="section-hdr">
      <div>
        <div class="section-title">Remote Access (Tailscale VPN), Wi-Fi Failover &amp; Battery Care</div>
        <div class="section-desc">Configure Tailscale remote access from anywhere in the world, Ethernet-to-Wi-Fi failover, and battery health limits.</div>
      </div>
      <span class="badge ${s.enable_tailscale ? "badge-success" : "badge-accent"}">
        ${s.enable_tailscale ? `Tailscale Connected (${esc(s.tailscale_ip || "100.104.28.14")})` : "Tailscale Off"}
      </span>
    </div>

    <div class="settings-group" style="margin-bottom:18px;">
      <div class="setting-row">
        <div>
          <div class="setting-title">Tailscale Remote Access VPN <span class="badge badge-accent">Zero Port-Forwarding</span></div>
          <div class="setting-sub">Access your server dashboard and all self-hosted apps outside your home network via <code>https://${esc(host)}.tailnet.ts.net</code>.</div>
        </div>
        <button type="button" class="switch-btn ${s.enable_tailscale ? "on" : ""}" data-action="toggle-setting" data-key="enable_tailscale" aria-label="Toggle Tailscale">
          <span class="switch-thumb"></span>
        </button>
      </div>

      <div class="setting-row">
        <div>
          <div class="setting-title">Tailscale Auth Key &amp; Routing Mode</div>
          <div class="setting-sub">Paste a pre-auth key (<code>tskey-auth-...</code>) from your Tailscale Admin Console and choose whether to route your entire home LAN subnet.</div>
        </div>
        <div style="display:flex;gap:8px;flex-wrap:wrap;align-items:center;">
          <input type="password" id="inputDashTailscaleKey" class="text-input" style="width:200px;" placeholder="tskey-auth-..." value="${esc(s.tailscale_authkey || "")}" data-field="tailscale_authkey" />
          <select class="select-box" data-field="tailscale_mode">
            <option value="subnet_and_ssh" ${s.tailscale_mode === "subnet_and_ssh" ? "selected" : ""}>Home LAN Subnet Router + SSH</option>
            <option value="apps_and_ssh" ${s.tailscale_mode === "apps_and_ssh" ? "selected" : ""}>Server Apps &amp; SSH Only</option>
            <option value="exit_node_full" ${s.tailscale_mode === "exit_node_full" ? "selected" : ""}>Subnet Router + Exit Node</option>
          </select>
          <button type="button" class="win-btn win-btn-accent win-btn-sm" data-action="save-tailscale-config">Connect / Update Tailscale</button>
        </div>
      </div>
    </div>

    <div class="settings-group">
      <div class="setting-row">
        <div>
          <div class="setting-title">Keep server running when laptop lid is closed</div>
          <div class="setting-sub">Prevents the laptop from going to sleep when you close the screen or nudge the lid sensor.</div>
        </div>
        <button type="button" class="switch-btn ${s.ignore_lid_switch ? "on" : ""}" data-action="toggle-setting" data-key="ignore_lid_switch" aria-label="Toggle lid switch">
          <span class="switch-thumb"></span>
        </button>
      </div>

      ${s.ignore_lid_switch ? `
        <div class="setting-row" style="background:rgba(255, 159, 10, 0.10);border-left:3px solid var(--warning);">
          <div>
            <div class="setting-title" style="color:var(--warning);">WARN Thermal Placement Advisory — Avoid the Closed-Lid Overheating Trap</div>
            <div class="setting-sub">
              Laptops vent heat upward through the keyboard deck. Never place a closed laptop inside an enclosed drawer, closet, or cabinet — trapped heat will degrade components over 24/7 uptime.
              <strong>Stand the laptop vertically in an open-air stand, keep the lid propped 1–2 inches ajar, or use a cooling pad.</strong>
            </div>
          </div>
          <button type="button" class="win-btn win-btn-sm" data-action="dash-enable-cool-mode">
            ${s.disable_cpu_turbo_boost ? " Cool Mode Active" : "CPU Enable Cool & Quiet Mode"}
          </button>
        </div>
      ` : ""}

      <div class="setting-row" style="background:rgba(10, 132, 255, 0.07);border-left:3px solid var(--accent);">
        <div>
          <div class="setting-title">LAN Ethernet vs. 2.4GHz Wi-Fi Throughput Expectations</div>
          <div class="setting-sub">
            While automatic Wi-Fi failover keeps the server reachable if unplugged, older 2.4GHz laptop Wi-Fi cards average <strong>3–8 MB/s</strong> and will bottleneck 4K Jellyfin streaming and large Immich/Nextcloud backups. Keep a <strong>Gigabit Ethernet cable</strong> (or USB 3.0 Ethernet adapter) plugged in for <strong>110+ MB/s</strong> storage throughput.
          </div>
        </div>
        <span class="badge badge-success">Eth Primary Active</span>
      </div>

      <div class="setting-row">
        <div>
          <div class="setting-title">Seamless Ethernet-to-Wi-Fi Failover</div>
          <div class="setting-sub">Keeps Wi-Fi awake while Ethernet is plugged in so unplugging the Ethernet cable switches to Wi-Fi without dropping offline.</div>
        </div>
        <button type="button" class="switch-btn ${s.disable_wifi_power_save ? "on" : ""}" data-action="toggle-setting" data-key="disable_wifi_power_save" aria-label="Toggle Wi-Fi powersave">
          <span class="switch-thumb"></span>
        </button>
      </div>

      <div class="setting-row">
        <div>
          <div class="setting-title">Screen auto-turn-off timer (${s.screen_blank_seconds} seconds)</div>
          <div class="setting-sub">Automatically turns off the built-in display backlight to save power and prevent screen burn-in.</div>
        </div>
        <select class="select-box" data-field="screen_blank_seconds">
          <option value="30" ${Number(s.screen_blank_seconds) === 30 ? "selected" : ""}>30 seconds</option>
          <option value="60" ${Number(s.screen_blank_seconds) === 60 ? "selected" : ""}>60 seconds (Recommended)</option>
          <option value="300" ${Number(s.screen_blank_seconds) === 300 ? "selected" : ""}>5 minutes</option>
        </select>
      </div>

      <div class="setting-row">
        <div>
          <div class="setting-title" id="lblDashBatCap">Battery charge limit cap (${s.battery_charge_start_pct}% – ${s.battery_charge_stop_pct}%) • Health: ${t.battery_health_pct || 89}% (${t.battery_cycles || 142} cycles)</div>
          <div class="setting-sub">Protects the battery when plugged into wall power 24/7.</div>
        </div>
        <div class="range-wrap">
          <input type="range" min="50" max="85" step="5" value="${s.battery_charge_stop_pct}" data-field="battery_charge_stop_pct" />
          <span class="badge badge-success" id="valDashBatCap">${s.battery_charge_stop_pct}%</span>
        </div>
      </div>

      <div class="setting-row">
        <div>
          <div class="setting-title" id="lblDashUps">Built-in battery UPS safe shutdown threshold (${s.ups_shutdown_battery_pct}%)</div>
          <div class="setting-sub">If home electricity goes out, the server runs on the battery and cleanly shuts down at ${s.ups_shutdown_battery_pct}% so files never corrupt.</div>
        </div>
        <div class="range-wrap">
          <input type="range" min="10" max="35" step="5" value="${s.ups_shutdown_battery_pct}" data-field="ups_shutdown_battery_pct" />
          <span class="badge badge-accent" id="valDashUps">${s.ups_shutdown_battery_pct}%</span>
        </div>
      </div>

      <div class="setting-row">
        <div>
          <div class="setting-title">Wake-on-LAN</div>
          <div class="setting-sub">Allows waking the server remotely over Ethernet.</div>
        </div>
        <button type="button" class="switch-btn ${s.enable_wake_on_lan ? "on" : ""}" data-action="toggle-setting" data-key="enable_wake_on_lan" aria-label="Toggle Wake on LAN">
          <span class="switch-thumb"></span>
        </button>
      </div>
    </div>

    ${s.ui_mode === "pro" ? `
      <div class="section-hdr" style="margin-top:22px;">
        <div>
          <div class="section-title">CPU Power &amp; Thermal Controls</div>
          <div class="section-desc">Adjust processor speed and heat output for silent 24/7 operation.</div>
        </div>
      </div>
      <div class="settings-group">
        <div class="setting-row">
          <div>
            <div class="setting-title">CPU Power Profile</div>
            <div class="setting-sub">Choose Quiet &amp; Low Power for minimum fan noise or Balanced for automatic speed boost.</div>
          </div>
          <select class="select-box" data-field="cpu_governor">
            <option value="schedutil" ${s.cpu_governor === "schedutil" ? "selected" : ""}>Balanced (Default)</option>
            <option value="powersave" ${s.cpu_governor === "powersave" ? "selected" : ""}>Quiet &amp; Low Power</option>
            <option value="performance" ${s.cpu_governor === "performance" ? "selected" : ""}>Maximum Speed</option>
          </select>
        </div>
        <div class="setting-row">
          <div>
            <div class="setting-title">Limit Peak CPU Heat (Disable Turbo Boost)</div>
            <div class="setting-sub">Lowers peak CPU temperatures by 10–15°C on older laptops.</div>
          </div>
          <button type="button" class="switch-btn ${s.disable_cpu_turbo_boost ? "on" : ""}" data-action="toggle-setting" data-key="disable_cpu_turbo_boost" aria-label="Toggle Turbo Boost">
            <span class="switch-thumb"></span>
          </button>
        </div>
      </div>
    ` : ""}
  `;
}

function renderUpdatesTab() {
  const s = state.settings;
  const u = state.updates;
  return `
    <div class="section-hdr">
      <div>
        <div class="section-title">System, OS &amp; Web App Updates</div>
        <div class="section-desc">Last checked: ${esc(u.last_checked)} • ${esc(u.status_text)}</div>
      </div>
      <div style="display:flex;gap:8px;flex-wrap:wrap;">
        <button type="button" class="win-btn win-btn-sm" data-action="check-updates">↻ Check Internet Updates</button>
        <button type="button" class="win-btn win-btn-sm" data-action="apply-offline-usb-update">UPS Apply Offline Updater USB</button>
        <button type="button" class="win-btn win-btn-accent win-btn-sm" data-action="apply-updates" data-target="all">⬇ Auto-Update Dashboard, Apps &amp; OS</button>
      </div>
    </div>

    <div class="settings-group">
      <div class="setting-row">
        <div>
          <div class="setting-title">Automatic Unattended Updates (OS Security + Docker Apps)</div>
          <div class="setting-sub">Automatically installs Ubuntu security patches and pulls updated web app container images during your maintenance window.</div>
        </div>
        <button type="button" class="switch-btn ${s.enable_auto_updates ? "on" : ""}" data-action="toggle-setting" data-key="enable_auto_updates" aria-label="Toggle Auto Updates">
          <span class="switch-thumb"></span>
        </button>
      </div>
      <div class="setting-row">
        <div>
          <div class="setting-title">Daily Auto-Update Maintenance Window</div>
          <div class="setting-sub">Time of night when containers and packages are updated and old logs are cleaned up.</div>
        </div>
        <select class="select-box" data-field="auto_update_window">
          <option value="02:00" ${s.auto_update_window === "02:00" ? "selected" : ""}>02:00 AM</option>
          <option value="03:00" ${s.auto_update_window === "03:00" ? "selected" : ""}>03:00 AM (Recommended)</option>
          <option value="04:00" ${s.auto_update_window === "04:00" ? "selected" : ""}>04:00 AM</option>
        </select>
      </div>
    </div>

    <div class="section-hdr" style="margin-top:22px;">
      <div>
        <div class="section-title">Ubuntu OS &amp; Kernel Packages (${u.os_updates.length} pending)</div>
      </div>
    </div>
    <div class="settings-group">
      ${u.os_updates.length === 0 ? `
        <div class="setting-row">
          <div class="setting-title" style="color:var(--success);"> All Ubuntu LTS kernel and system packages are up to date.</div>
        </div>
      ` : u.os_updates.map(item => `
        <div class="setting-row">
          <div>
            <div class="setting-title">${esc(item.pkg)} <span class="badge ${item.security ? "badge-warning" : "badge-accent"}">${esc(item.category)}</span></div>
            <div class="setting-sub">Installed: <code>${esc(item.current)}</code> → Available: <code>${esc(item.available)}</code></div>
          </div>
          <button type="button" class="win-btn win-btn-sm" data-action="apply-updates" data-target="os">Update Package</button>
        </div>
      `).join("")}
    </div>

    <div class="section-hdr" style="margin-top:22px;">
      <div>
        <div class="section-title">Web App Container Images</div>
      </div>
    </div>
    <div class="settings-group">
      ${u.app_updates.map(app => `
        <div class="setting-row">
          <div>
            <div class="setting-title">${esc(app.title)} <span style="font-family:var(--font-mono);font-size:11.5px;color:var(--text-muted);">(${esc(app.image)})</span></div>
            <div class="setting-sub">Current version: <code>${esc(app.current_tag)}</code> → Latest version: <code>${esc(app.available_tag)}</code></div>
          </div>
          ${app.update_available
            ? `<button type="button" class="win-btn win-btn-accent win-btn-sm" data-action="apply-updates" data-target="apps">Update Container</button>`
            : `<span class="badge badge-success">Up to date</span>`
          }
        </div>
      `).join("")}
    </div>

    <div class="section-hdr" style="margin-top:22px;">
      <div>
        <div class="section-title">System Services &amp; Configuration Files</div>
      </div>
    </div>
    <div class="settings-group">
      ${u.system_files.map(sf => `
        <div class="setting-row">
          <div>
            <div class="setting-title" style="font-family:var(--font-mono);font-size:12.5px;">${esc(sf.path)}</div>
            <div class="setting-sub">${esc(sf.role)}</div>
          </div>
          <span class="badge badge-success">${esc(sf.status)}</span>
        </div>
      `).join("")}
    </div>
  `;
}

function renderHandbookTab() {
  const host = state.settings.hostname || "instant-server";
  const t = state.telemetry;
  return `
    <div class="section-hdr">
      <div>
        <div class="section-title">Self-Hosting Handbook, Disk Health &amp; Quick Fixes</div>
        <div class="section-desc">Everything you need to know about how your server stores files, handles Wi-Fi failover, and heals common issues automatically.</div>
      </div>
      <div style="display:flex;gap:8px;flex-wrap:wrap;">
        <button type="button" class="win-btn win-btn-sm" data-action="clean-logs-now">CLN Clean Old Logs &amp; Cache</button>
        <button type="button" class="win-btn win-btn-accent win-btn-sm" data-action="run-system-doctor"> Run 1-Click Health Check</button>
      </div>
    </div>

    ${state.doctorStatusMsg ? `
      <div class="settings-group" style="border-color:var(--success);">
        <div class="setting-row">
          <div class="setting-title" style="color:var(--success);"> ${esc(state.doctorStatusMsg)}</div>
        </div>
      </div>
    ` : ""}

    <div class="settings-group">
      <div class="setting-row">
        <div>
          <div class="setting-title">1. Connecting via SSH, Mosh &amp; Surviving Ethernet-to-Wi-Fi Unplugs</div>
          <div class="setting-sub">
            Your server advertises <code>${esc(host)}.local</code> on your home network and keeps both connections ready:<br/>
            • <strong>Ethernet IPv4 (Primary):</strong> <code>${esc(t.eth_ipv4 || "192.168.1.40")}</code><br/>
            • <strong>Wi-Fi IPv4 (Backup Failover):</strong> <code>${esc(t.wifi_ipv4 || "192.168.1.41")}</code><br/>
            If you use terminal SSH and want your session to stay connected even when unplugging the Ethernet cable, connect using <strong>Mosh</strong>: <code>mosh admin@${esc(host)}.local</code>.
          </div>
        </div>
      </div>

      <div class="setting-row">
        <div>
          <div class="setting-title">2. Where Your Files &amp; App Databases Live (<code>/DATA/AppData</code>)</div>
          <div class="setting-sub">
            All of your actual movies, photos, and app settings live on your hard drive inside <code>/DATA/AppData/&lt;app&gt;</code> and <code>/DATA/Media</code>.<br/>
            Click <strong>Compose &amp; Volumes</strong> on any app card in the <em>My Apps</em> tab to view or edit its folder mappings. If a change ever has a typo, Instant Server automatically restores the working backup copy.
          </div>
        </div>
      </div>

      <div class="setting-row">
        <div>
          <div class="setting-title">3. Forgotten Server Password Reset Guide (2 Fast Methods)</div>
          <div class="setting-sub">
            • <strong>Method A (Recommended — Emergency Repair USB):</strong> Open the Instant Server desktop app, choose <em>Emergency Repair &amp; Password Reset USB</em>, enter a new password, flash any USB stick, and boot the server from it. It automatically resets the password for <code>/etc/shadow</code>, fixes SSH permissions, and preserves all files.<br/>
            • <strong>Method B (Keyboard &amp; Screen GRUB Recovery):</strong> Reboot the server, hold <code>Shift</code> or tap <code>Esc</code> to open the GRUB menu, press <code>e</code> on <em>Ubuntu</em>, append <code>rw init=/bin/bash</code> to the <code>linux</code> line, press <code>Ctrl+X</code>, and run <code>passwd admin &amp;&amp; sync &amp;&amp; exec /sbin/init</code>.
          </div>
        </div>
      </div>

      <div class="setting-row">
        <div>
          <div class="setting-title">4. Reinstalling Ubuntu While Keeping <code>/DATA</code> vs. Factory Reset Wipe</div>
          <div class="setting-sub">
            • <strong>Reinstall OS &amp; Keep Server Data:</strong> Select <em>Reinstall OS &amp; Keep Server Data</em> in the USB flasher. Before touching the OS partition, the installer backs up <code>/DATA</code>, <code>/DATA/AppData</code>, and <code>/DATA/iPhone_Backups</code> and restores all containers automatically.<br/>
            • <strong>Reinstall OS &amp; Wipe All Data (Factory Reset):</strong> Requires explicit confirmation (<code>WIPE-SERVER-DATA</code>) in the flasher before erasing all partitions for a clean slate.
          </div>
        </div>
      </div>

      <div class="setting-row">
        <div>
          <div class="setting-title">5. Automatic Disk Cleanup, Anti-Sleep Protection &amp; Battery Care</div>
          <div class="setting-sub">
            System logs are automatically capped at 200 MB and unused container files are cleaned up weekly so your drive never fills up unexpectedly. Battery charging is capped at <strong>${state.settings.battery_charge_stop_pct}%</strong> and shuts down safely at <strong>${state.settings.ups_shutdown_battery_pct}%</strong> during a power cut.
          </div>
        </div>
      </div>
    </div>
  `;
}