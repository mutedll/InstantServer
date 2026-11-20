// dash_views_apps.js — App Store, Minecraft Studio, and Cluster Tab Views
function renderAppsTab() {
  const q = state.searchQuery.trim().toLowerCase();
  const installedApps = state.apps.filter(a => a.installed && (!q || a.title.toLowerCase().includes(q) || a.description.toLowerCase().includes(q)));
  const categories = ["All", "Knowledge & Wiki", "Media & Photos", "Cloud & Files", "Network & VPN", "Smart Home", "Security & Tools", "Gaming & AI"];
  const doesDashAppMatchCat = (a, cat) => {
    if (!cat || cat === "All") return true;
    const cats = Array.isArray(a.categories) ? a.categories : [a.category];
    return a.category === cat || cats.includes(cat);
  };
  const catalogApps = state.apps.filter(a => {
    const matchCat = doesDashAppMatchCat(a, state.appCategory);
    const matchQ = !q || (a.title + " " + a.description + " " + (a.image || "") + " " + (a.category || "")).toLowerCase().includes(q);
    return matchCat && matchQ;
  });

  const host = state.settings.hostname || "instant-server";
  return `
    <div class="section-hdr">
      <div>
        <div class="section-title">Installed Web Apps (${installedApps.length})</div>
        <div class="section-desc">Click <strong>Compose &amp; Volumes</strong> on any app to view or edit its <code>docker-compose.yml</code> and storage folder paths.</div>
      </div>
      <div style="display:flex;gap:8px;flex-wrap:wrap;">
        <a class="win-btn win-btn-sm" href="https://${esc(host)}.local:9090" target="_blank" rel="noopener">Open Cockpit (:9090)</a>
        <a class="win-btn win-btn-sm" href="http://${esc(host)}.local" target="_blank" rel="noopener">Open CasaOS (:80)</a>
      </div>
    </div>

    <div class="cards-grid">
      ${installedApps.map(app => `
        <div class="app-card">
          <div>
            <div class="app-card-top">
              <div>
                <div class="app-card-title">${esc(app.title)}</div>
                <div class="app-card-meta">http://${esc(host)}.local:${app.port} • ${app.ram_mb} MB RAM</div>
              </div>
              <span class="badge badge-success">Running</span>
            </div>
            <div class="app-card-desc" style="margin-top:8px;">${esc(app.description)}</div>
          </div>
          <div class="app-card-actions">
            <a class="win-btn win-btn-accent win-btn-sm" href="http://${esc(host)}.local:${app.port}" target="_blank" rel="noopener">Open Web UI ↗</a>
            <div style="display:flex;gap:6px;flex-wrap:wrap;">
              <button type="button" class="win-btn win-btn-sm" data-action="inspect-compose" data-app="${app.id}">Compose &amp; Volumes</button>
              <button type="button" class="win-btn win-btn-sm" data-action="app-op" data-app="${app.id}" data-op="restart">Restart</button>
              <button type="button" class="win-btn win-btn-danger win-btn-sm" data-action="app-op" data-app="${app.id}" data-op="remove">Remove</button>
            </div>
          </div>
        </div>
      `).join("")}
    </div>

    <div class="section-hdr" style="margin-top:28px;">
      <div>
        <div class="section-title">Self-Hosted App Store Catalog (${state.apps.length}+ Apps)</div>
        <div class="section-desc">1-click install for ${state.apps.length}+ verified apps, or paste any Docker image / command / URL from the web below.</div>
      </div>
    </div>

    <div class="settings-group" style="margin-bottom:14px;border-color:var(--success);">
      <div class="setting-row">
        <div>
          <div class="setting-title">FIX Custom Server Workflows, NAS Shares &amp; USB Triggers <span class="badge badge-success">Full Server Control</span></div>
          <div class="setting-sub">Create and run custom automation scripts (<code>/DATA/CustomWorkflows</code>), custom Samba NAS shares (<code>smb://${esc(host)}.local/</code>), <code>udev</code> USB plug-in triggers, and scheduled <code>systemd</code> services.</div>
        </div>
        <div style="display:flex;gap:8px;flex-wrap:wrap;">
          <button type="button" class="win-btn win-btn-sm" data-action="switch-tab" data-tab="files">EXT4 Edit System &amp; Workflow Files</button>
          <button type="button" class="win-btn win-btn-accent win-btn-sm" data-action="run-custom-workflow">▶ Run Custom Workflow Script</button>
        </div>
      </div>
      <div class="setting-row" style="background:var(--bg-input);font-size:12px;font-family:var(--font-mono);color:var(--success);">
        <span> ${esc(state.customWorkflowStatus)}</span>
      </div>
    </div>

    <div class="settings-group" style="margin-bottom:14px;border-color:var(--accent);">
      <div class="setting-row">
        <div>
          <div class="setting-title">MC Minecraft Server &amp; World File Studio <span class="badge badge-success">v${esc(state.minecraftStudio.version)} • 25565 TCP / 19132 UDP</span></div>
          <div class="setting-sub">Dedicated tab for Minecraft ${esc(state.minecraftStudio.version)} ("Wilderness Bound"), Paper/Fabric/Forge/Geyser Crossplay, live RCON console, Modrinth plugins, and <code>server.properties</code> world file editing.</div>
        </div>
        <div style="display:flex;gap:8px;flex-wrap:wrap;">
          <button type="button" class="win-btn win-btn-accent win-btn-sm" data-action="switch-tab" data-tab="minecraft">MC Open Minecraft Studio Tab</button>
          <button type="button" class="win-btn win-btn-sm" data-action="inspect-compose" data-app="minecraft">Inspect Compose YAML</button>
        </div>
      </div>
    </div>

    <div class="settings-group" style="margin-bottom:14px;">
      <div class="setting-row">
        <div>
          <div class="setting-title">Add Any Custom App from the Web</div>
          <div class="setting-sub">Paste a Docker image, <code>docker run</code> command, or GitHub / Docker Hub link to auto-generate its container &amp; install immediately.</div>
        </div>
        <div style="display:flex;gap:8px;flex-wrap:wrap;width:100%;max-width:460px;">
          <input type="text" id="inputDashCustomApp" class="text-input" style="flex:1;" placeholder="e.g. louislam/uptime-kuma:1 or docker run -p 8080:80 ..." value="${esc(state.customAppInput || "")}" />
          <button type="button" class="win-btn win-btn-accent win-btn-sm" data-action="dash-add-custom-app">＋ Add &amp; Install</button>
        </div>
      </div>
    </div>

    <div class="filter-pills">
      ${categories.map(cat => {
        const cnt = state.apps.filter(a => doesDashAppMatchCat(a, cat)).length;
        return `
          <button type="button" class="filter-pill ${state.appCategory === cat ? "active" : ""}" data-action="set-app-cat" data-cat="${esc(cat)}">${esc(cat)} (${cnt})</button>
        `;
      }).join("")}
    </div>

    <div class="cards-grid">
      ${catalogApps.map(app => `
        <div class="app-card">
          <div>
            <div class="app-card-top">
              <div>
                <div class="app-card-title">${esc(app.title)}</div>
                <div class="app-card-meta">${esc(app.image)} • Port ${app.port}</div>
              </div>
              <span class="badge ${app.installed ? "badge-success" : "badge-accent"}">${app.installed ? "Installed" : `${app.ram_mb} MB`}</span>
            </div>
            <div class="app-card-desc" style="margin-top:8px;">${esc(app.description)}</div>
          </div>
          <div class="app-card-actions">
            <button type="button" class="win-btn win-btn-sm" data-action="inspect-compose" data-app="${app.id}">Inspect YAML</button>
            ${app.installed
              ? `<button type="button" class="win-btn win-btn-danger win-btn-sm" data-action="app-op" data-app="${app.id}" data-op="remove">Uninstall</button>`
              : `<button type="button" class="win-btn win-btn-accent win-btn-sm" data-action="app-op" data-app="${app.id}" data-op="install">＋ Install &amp; Start</button>`
            }
          </div>
        </div>
      `).join("")}
    </div>
  `;
}

function renderMinecraftTab() {
  const mc = state.minecraftStudio;
  const host = state.settings.hostname || "instant-server";
  const fileKeys = Object.keys(mc.files || {});
  const activeFile = mc.active_file || fileKeys[0] || "server.properties";
  const activeContent = (mc.files && mc.files[activeFile]) || "";
  return `
    <div class="section-hdr">
      <div>
        <div class="section-title">MC Minecraft Server &amp; World File Studio (v${esc(mc.version)})</div>
        <div class="section-desc">Manage your 24/7 Minecraft Java &amp; Bedrock Crossplay server (<code>${esc(host)}.local:25565</code>), edit <code>/DATA/AppData/minecraft/data</code> world files, install Modrinth plugins, and run live RCON commands.</div>
      </div>
      <div style="display:flex;gap:8px;flex-wrap:wrap;">
        <button type="button" class="win-btn win-btn-sm" data-action="mc-rcon-backup">FS 1-Click Safe World Backup</button>
        <button type="button" class="win-btn win-btn-accent win-btn-sm" data-action="mc-save-apply-config"> Save &amp; Apply Server Settings</button>
      </div>
    </div>

    <div class="settings-group" style="margin-bottom:18px;border-color:var(--accent);">
      <div class="setting-row">
        <div>
          <div class="setting-title">Server Software &amp; Minecraft Version</div>
          <div class="setting-sub">Defaults to Minecraft <strong>26.3 ("Wilderness Bound")</strong> with PaperMC + G1GC optimization and <code>WHITE_LIST=false</code> so friends on your LAN can join immediately.</div>
        </div>
        <div style="display:flex;gap:8px;flex-wrap:wrap;align-items:center;">
          <select class="select-box" data-mc-field="server_type">
            <option value="PAPER" ${mc.server_type === "PAPER" ? "selected" : ""}>PaperMC (Fastest + Plugins)</option>
            <option value="GEYSER_FLOODGATE" ${mc.server_type === "GEYSER_FLOODGATE" ? "selected" : ""}>Paper + Geyser Crossplay (Java + Bedrock 19132/UDP)</option>
            <option value="PURPUR" ${mc.server_type === "PURPUR" ? "selected" : ""}>Purpur Server</option>
            <option value="FABRIC" ${mc.server_type === "FABRIC" ? "selected" : ""}>Fabric (Lightweight Mods)</option>
            <option value="NEOFORGE" ${mc.server_type === "NEOFORGE" ? "selected" : ""}>NeoForge (Modern Modpacks)</option>
            <option value="FORGE" ${mc.server_type === "FORGE" ? "selected" : ""}>Forge (Classic Modpacks)</option>
            <option value="VANILLA" ${mc.server_type === "VANILLA" ? "selected" : ""}>Vanilla Mojang</option>
          </select>
          <select class="select-box" data-mc-field="version">
            <option value="26.3" ${mc.version === "26.3" ? "selected" : ""}>26.3 (Latest — Wilderness Bound)</option>
            <option value="26.2" ${mc.version === "26.2" ? "selected" : ""}>26.2 (Chaos Cubed)</option>
            <option value="26.1" ${mc.version === "26.1" ? "selected" : ""}>26.1 (Tiny Takeover)</option>
            <option value="1.21.11" ${mc.version === "1.21.11" ? "selected" : ""}>1.21.11 (Legacy Tricky Trials)</option>
            <option value="1.21.4" ${mc.version === "1.21.4" ? "selected" : ""}>1.21.4</option>
            <option value="LATEST" ${mc.version === "LATEST" ? "selected" : ""}>Auto-Latest Release</option>
          </select>
        </div>
      </div>

      <div class="setting-row">
        <div>
          <div class="setting-title">RAM Allocation, Gamemode, Difficulty &amp; Player Authentication</div>
          <div class="setting-sub">Choose how much memory Minecraft can use and whether players need an official Mojang login or can join on offline/local LAN.</div>
        </div>
        <div style="display:flex;gap:8px;flex-wrap:wrap;align-items:center;">
          <select class="select-box" data-mc-field="memory_heap">
            <option value="2G" ${mc.memory_heap === "2G" ? "selected" : ""}>2 GB RAM</option>
            <option value="4G" ${mc.memory_heap === "4G" ? "selected" : ""}>4 GB RAM (Recommended)</option>
            <option value="6G" ${mc.memory_heap === "6G" ? "selected" : ""}>6 GB RAM</option>
            <option value="8G" ${mc.memory_heap === "8G" ? "selected" : ""}>8 GB RAM</option>
            <option value="12G" ${mc.memory_heap === "12G" ? "selected" : ""}>12 GB RAM</option>
          </select>
          <select class="select-box" data-mc-field="gamemode">
            <option value="survival" ${mc.gamemode === "survival" ? "selected" : ""}>Survival</option>
            <option value="creative" ${mc.gamemode === "creative" ? "selected" : ""}>Creative</option>
            <option value="adventure" ${mc.gamemode === "adventure" ? "selected" : ""}>Adventure</option>
          </select>
          <select class="select-box" data-mc-field="difficulty">
            <option value="peaceful" ${mc.difficulty === "peaceful" ? "selected" : ""}>Peaceful</option>
            <option value="easy" ${mc.difficulty === "easy" ? "selected" : ""}>Easy</option>
            <option value="normal" ${mc.difficulty === "normal" ? "selected" : ""}>Normal</option>
            <option value="hard" ${mc.difficulty === "hard" ? "selected" : ""}>Hard</option>
          </select>
          <select class="select-box" data-mc-field="online_mode">
            <option value="true" ${mc.online_mode ? "selected" : ""}>Online Mode (Official Accounts)</option>
            <option value="false" ${!mc.online_mode ? "selected" : ""}>Offline / LAN Mode (Any Account)</option>
          </select>
        </div>
      </div>

      <div class="setting-row">
        <div>
          <div class="setting-title">Auto-Downloaded Modrinth Plugins / Mods &amp; Admin Operators (OPs)</div>
          <div class="setting-sub">Enter comma-separated Modrinth project slugs (downloaded automatically into <code>/DATA/AppData/minecraft/data/plugins</code>) and admin usernames.</div>
        </div>
        <div style="display:flex;gap:8px;flex-wrap:wrap;width:100%;max-width:520px;">
          <input type="text" class="text-input" style="flex:1;min-width:200px;" data-mc-field="modrinth_projects" value="${esc(mc.modrinth_projects)}" placeholder="viaversion,chunky,worldedit,geyser" />
          <input type="text" class="text-input" style="width:160px;" data-mc-field="ops_list" value="${esc(mc.ops_list)}" placeholder="OPs: admin,player2" />
        </div>
      </div>
    </div>

    <div class="settings-group" style="margin-bottom:18px;">
      <div class="setting-row">
        <div>
          <div class="setting-title">FAST Live Minecraft RCON Admin Console (<code>127.0.0.1:25575</code>)</div>
          <div class="setting-sub">Run server commands immediately without logging into the game.</div>
        </div>
        <div style="display:flex;gap:6px;flex-wrap:wrap;">
          <button type="button" class="win-btn win-btn-sm" data-action="mc-quick-rcon" data-cmd="list">Players Online</button>
          <button type="button" class="win-btn win-btn-sm" data-action="mc-quick-rcon" data-cmd="time set day">Set Day</button>
          <button type="button" class="win-btn win-btn-sm" data-action="mc-quick-rcon" data-cmd="weather clear">Clear Weather</button>
          <button type="button" class="win-btn win-btn-sm" data-action="mc-quick-rcon" data-cmd="save-all flush">Flush World to Disk</button>
        </div>
      </div>
      <div class="setting-row" style="flex-wrap:wrap;gap:8px;">
        <input type="text" id="inputMcRconCmd" class="text-input" style="flex:1;min-width:220px;font-family:var(--font-mono);" value="${esc(mc.rcon_command)}" placeholder="op PlayerName, whitelist off, gamemode creative PlayerName, tp ..." />
        <button type="button" class="win-btn win-btn-accent win-btn-sm" data-action="mc-run-rcon">▶ Send Command</button>
        <div style="width:100%;font-family:var(--font-mono);font-size:12px;color:var(--success);background:var(--bg-input);padding:10px 12px;border-radius:var(--radius-sm);border:1px solid var(--border-subtle);">
          ${esc(mc.rcon_output)}
        </div>
      </div>
    </div>

    <div class="settings-group">
      <div class="setting-row" style="flex-wrap:wrap;gap:10px;">
        <div>
          <div class="setting-title">EXT4 Minecraft Server &amp; World Config Files (<code>/DATA/AppData/minecraft/data/</code>)</div>
          <div class="setting-sub">Directly edit <code>server.properties</code>, <code>ops.json</code>, <code>whitelist.json</code>, or <code>paper-global.yml</code> with automatic <code>.bak</code> backup protection.</div>
        </div>
        <div style="display:flex;gap:6px;flex-wrap:wrap;">
          ${fileKeys.map(fn => `
            <button type="button" class="filter-pill ${activeFile === fn ? "active" : ""}" data-action="mc-select-file" data-file="${esc(fn)}">CFG ${esc(fn)}</button>
          `).join("")}
        </div>
      </div>
      <div class="setting-row" style="flex-direction:column;align-items:stretch;gap:10px;">
        <div style="display:flex;justify-content:space-between;align-items:center;gap:8px;flex-wrap:wrap;">
          <span style="font-family:var(--font-mono);font-size:12px;color:var(--accent);">/DATA/AppData/minecraft/data/${esc(activeFile)}</span>
          <button type="button" class="win-btn win-btn-accent win-btn-sm" data-action="mc-save-file">FS Save ${esc(activeFile)}</button>
        </div>
        <textarea id="textareaMcFileEditor" class="compose-editor" style="min-height:210px;" spellcheck="false">${esc(activeContent)}</textarea>
      </div>
    </div>
  `;
}

function renderClusterTab() {
  const totalGb = state.clusterActiveNodes.reduce((acc, n) => acc + n.total_gb, 0);
  const usedGb = state.clusterActiveNodes.reduce((acc, n) => acc + n.used_gb, 0);
  const expectedWipeCode = `WIPE-${state.clusterConnectHost || "second-server"}`;
  const isWipeMode = state.clusterMergePolicy === "wipe_secondary_expand";
  const wipeConfirmed = state.clusterConfirmWipe.trim() === expectedWipeCode;

  return `
    <div class="section-hdr">
      <div>
        <div class="section-title">Combine Multiple Computers into One Server (${state.clusterActiveNodes.length} Linked)</div>
        <div class="section-desc">Connect another laptop, desktop PC, or Mini-PC on your home network to combine their storage space and share apps.</div>
      </div>
      <span class="badge badge-success">Combined Pool: ${usedGb.toFixed(0)} / ${totalGb.toFixed(0)} GB</span>
    </div>

    <div class="settings-group">
      ${state.clusterActiveNodes.map(n => `
        <div class="setting-row">
          <div>
            <div class="setting-title">ON ${esc(n.hostname)}.local <span class="badge badge-accent">${esc(n.ip)}</span></div>
            <div class="setting-sub">${esc(n.device)} • ${n.used_gb} GB used of ${n.total_gb} GB • ${n.ram_gb} GB RAM</div>
          </div>
          <span class="badge badge-success">Active in Server Pool</span>
        </div>
      `).join("")}

      ${state.clusterDiscoveredNodes.map(n => `
        <div class="setting-row">
          <div>
            <div class="setting-title"> ${esc(n.hostname)}.local <span class="badge badge-warning">${esc(n.ip)}</span></div>
            <div class="setting-sub">${esc(n.device)} • Has ${n.used_gb} GB of existing files on a ${n.total_gb} GB drive</div>
          </div>
          <button type="button" class="win-btn win-btn-sm" data-action="select-cluster-candidate" data-host="${esc(n.hostname)}" data-ip="${esc(n.ip)}">Select to Connect</button>
        </div>
      `).join("")}
    </div>

    <div class="section-hdr" style="margin-top:22px;">
      <div>
        <div class="section-title">Connect Another Server on Your Network</div>
        <div class="section-desc">Choose how to handle existing files on both computers before linking them together.</div>
      </div>
    </div>

    <div class="settings-group">
      <div class="setting-row">
        <div>
          <div class="setting-title">Second server name &amp; IP address</div>
          <div class="setting-sub">Pick a discovered server above or type the IP address of another Instant Server on your Wi-Fi or LAN.</div>
        </div>
        <div style="display:flex;gap:8px;flex-wrap:wrap;">
          <input type="text" class="text-input" data-field="clusterConnectHost" value="${esc(state.clusterConnectHost)}" placeholder="Server name" style="width:160px;" />
          <input type="text" class="text-input" data-field="clusterConnectIp" value="${esc(state.clusterConnectIp)}" placeholder="IP address" style="width:135px;" />
        </div>
      </div>

      <div class="setting-row">
        <div>
          <div class="setting-title">What should happen to existing files on both computers?</div>
          <div class="setting-sub">By default, both computers keep all of their existing files and combine their free space into one shared pool.</div>
        </div>
        <select class="select-box" data-field="clusterMergePolicy">
          <option value="mergerfs_union_safe" ${state.clusterMergePolicy === "mergerfs_union_safe" ? "selected" : ""}>Keep all files on both computers (Safe combined storage)</option>
          <option value="replicated_ha_mirror" ${state.clusterMergePolicy === "replicated_ha_mirror" ? "selected" : ""}>Keep all files &amp; mirror backups across both computers</option>
          <option value="wipe_secondary_expand" ${state.clusterMergePolicy === "wipe_secondary_expand" ? "selected" : ""}>Erase the second computer for empty space (Requires confirmation)</option>
        </select>
      </div>

      ${isWipeMode ? `
        <div class="setting-row" style="background:var(--danger-soft);">
          <div>
            <div class="setting-title" style="color:var(--danger);">WARN Crucial Warning: Erasing ${esc(state.clusterConnectHost)} will delete all existing files on that computer</div>
            <div class="setting-sub">If you want to keep the files on both computers, switch back to <strong>Keep all files on both computers</strong> above. To confirm erasing the second computer, type <code>${esc(expectedWipeCode)}</code>:</div>
          </div>
          <input type="text" class="text-input" data-field="clusterConfirmWipe" value="${esc(state.clusterConfirmWipe)}" placeholder="Type ${esc(expectedWipeCode)}" style="border-color:var(--danger);width:210px;" />
        </div>
      ` : ""}

      <div class="setting-row">
        <div>
          <div class="setting-title">Ready to link computers</div>
          <div class="setting-sub">App databases stay safely on local storage so brief Wi-Fi drops never corrupt your apps.</div>
        </div>
        <button
          type="button"
          class="win-btn win-btn-accent"
          data-action="join-cluster-node"
          ${isWipeMode && !wipeConfirmed ? "disabled" : ""}
        >
          ${isWipeMode && !wipeConfirmed ? `Type ${esc(expectedWipeCode)} to Unlock` : "＋ Combine Servers Now"}
        </button>
      </div>
    </div>
  `;
}