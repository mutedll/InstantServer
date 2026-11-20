// FILE: src/instant_server/ui/dashboard/dash_minecraft_app.js
// Dedicated Pro-Level Minecraft Server Control Panel (Paid-Hosting Grade)

const MC_PROP_SCHEMA = [
  // Gameplay & World
  { k: "motd", g: "Gameplay & World", t: "text", d: "A Minecraft Server", h: "Server list description (supports § color codes)" },
  { k: "gamemode", g: "Gameplay & World", t: "select", o: ["survival", "creative", "adventure", "spectator"], d: "survival", h: "Default player game mode" },
  { k: "force-gamemode", g: "Gameplay & World", t: "bool", d: "false", h: "Reset players to default gamemode on join" },
  { k: "difficulty", g: "Gameplay & World", t: "select", o: ["peaceful", "easy", "normal", "hard"], d: "normal", h: "World difficulty" },
  { k: "hardcore", g: "Gameplay & World", t: "bool", d: "false", h: "Permanent spectator/ban on death" },
  { k: "pvp", g: "Gameplay & World", t: "bool", d: "true", h: "Allow player-vs-player combat" },
  { k: "allow-flight", g: "Gameplay & World", t: "bool", d: "false", h: "Allow survival flight (needed by many mods)" },
  { k: "allow-nether", g: "Gameplay & World", t: "bool", d: "true", h: "Enable Nether dimension" },
  { k: "enable-command-block", g: "Gameplay & World", t: "bool", d: "false", h: "Enable command blocks" },
  { k: "spawn-monsters", g: "Gameplay & World", t: "bool", d: "true", h: "Spawn hostile mobs" },
  { k: "spawn-animals", g: "Gameplay & World", t: "bool", d: "true", h: "Spawn passive animals" },
  { k: "spawn-npcs", g: "Gameplay & World", t: "bool", d: "true", h: "Spawn villagers" },
  { k: "spawn-protection", g: "Gameplay & World", t: "num", d: "16", h: "Protected spawn radius in blocks (0 = off)" },
  { k: "level-name", g: "Gameplay & World", t: "text", d: "world", h: "Active world folder name" },
  { k: "level-seed", g: "Gameplay & World", t: "text", d: "", h: "World generation seed (blank = random)" },
  { k: "level-type", g: "Gameplay & World", t: "select", o: ["minecraft:normal", "minecraft:flat", "minecraft:large_biomes", "minecraft:amplified", "minecraft:single_biome_surface"], d: "minecraft:normal", h: "World generator preset" },
  { k: "generate-structures", g: "Gameplay & World", t: "bool", d: "true", h: "Generate villages, temples & strongholds" },
  { k: "generator-settings", g: "Gameplay & World", t: "text", d: "{}", h: "Custom JSON world generator settings" },
  { k: "max-world-size", g: "Gameplay & World", t: "num", d: "29999984", h: "Maximum world border radius in blocks" },
  { k: "initial-enabled-packs", g: "Gameplay & World", t: "text", d: "vanilla", h: "Comma-separated enabled datapacks" },
  { k: "initial-disabled-packs", g: "Gameplay & World", t: "text", d: "", h: "Comma-separated disabled datapacks" },
  // Players, Access & Security
  { k: "max-players", g: "Players & Security", t: "num", d: "20", h: "Maximum concurrent players" },
  { k: "online-mode", g: "Players & Security", t: "bool", d: "true", h: "True = Official Mojang/Microsoft accounts; False = Offline/Cracked" },
  { k: "white-list", g: "Players & Security", t: "bool", d: "false", h: "Only allow players in whitelist.json" },
  { k: "enforce-whitelist", g: "Players & Security", t: "bool", d: "false", h: "Kick non-whitelisted players immediately on reload" },
  { k: "enforce-secure-profile", g: "Players & Security", t: "bool", d: "true", h: "Require Mojang-signed chat keys" },
  { k: "op-permission-level", g: "Players & Security", t: "select", o: ["1", "2", "3", "4"], d: "4", h: "Default OP level (1-4)" },
  { k: "function-permission-level", g: "Players & Security", t: "select", o: ["1", "2", "3", "4"], d: "2", h: "Permission level for datapack functions" },
  { k: "player-idle-timeout", g: "Players & Security", t: "num", d: "0", h: "Kick AFK players after N minutes (0 = off)" },
  { k: "hide-online-players", g: "Players & Security", t: "bool", d: "false", h: "Hide player list from server status pings" },
  { k: "prevent-proxy-connections", g: "Players & Security", t: "bool", d: "false", h: "Block VPN/proxy logins" },
  { k: "log-ips", g: "Players & Security", t: "bool", d: "true", h: "Log player IP addresses in server logs" },
  { k: "broadcast-console-to-ops", g: "Players & Security", t: "bool", d: "true", h: "Send console command output to online OPs" },
  { k: "broadcast-rcon-to-ops", g: "Players & Security", t: "bool", d: "true", h: "Send RCON command output to online OPs" },
  { k: "enable-code-of-conduct", g: "Players & Security", t: "bool", d: "false", h: "Show server Code of Conduct on join" },
  // Performance & Chunks
  { k: "view-distance", g: "Performance", t: "num", d: "10", h: "Chunk view distance (3-32)" },
  { k: "simulation-distance", g: "Performance", t: "num", d: "10", h: "Active mob/redstone simulation distance (3-32)" },
  { k: "entity-broadcast-range-percentage", g: "Performance", t: "num", d: "100", h: "Entity visibility range % (10-1000)" },
  { k: "max-tick-time", g: "Performance", t: "num", d: "60000", h: "Watchdog max ms per tick (-1 = disable)" },
  { k: "max-chained-neighbor-updates", g: "Performance", t: "num", d: "1000000", h: "Limit consecutive block updates" },
  { k: "pause-when-empty-seconds", g: "Performance", t: "num", d: "60", h: "Pause server ticks when 0 players online (0 = never)" },
  { k: "sync-chunk-writes", g: "Performance", t: "bool", d: "true", h: "Synchronous disk chunk writes for crash safety" },
  { k: "use-native-transport", g: "Performance", t: "bool", d: "true", h: "Linux epoll network optimization" },
  { k: "region-file-compression", g: "Performance", t: "select", o: ["deflate", "lz4", "none"], d: "deflate", h: "World chunk compression algorithm" },
  { k: "network-compression-threshold", g: "Performance", t: "num", d: "256", h: "Compress packets larger than N bytes (-1 = off)" },
  { k: "rate-limit", g: "Performance", t: "num", d: "0", h: "Max packets/sec before kick (0 = unlimited)" },
  // Networking, RCON, Query & Resource Packs
  { k: "server-port", g: "Network & Packs", t: "num", d: "25565", h: "Java Edition TCP port" },
  { k: "server-ip", g: "Network & Packs", t: "text", d: "", h: "Bind IP (leave blank for all interfaces)" },
  { k: "enable-status", g: "Network & Packs", t: "bool", d: "true", h: "Appear online in multiplayer server list" },
  { k: "accepts-transfers", g: "Network & Packs", t: "bool", d: "false", h: "Accept incoming server-transfer packets" },
  { k: "enable-rcon", g: "Network & Packs", t: "bool", d: "true", h: "Enable remote console (RCON)" },
  { k: "rcon.port", g: "Network & Packs", t: "num", d: "25575", h: "RCON TCP port" },
  { k: "rcon.password", g: "Network & Packs", t: "text", d: "instant-rcon-admin", h: "RCON authentication password" },
  { k: "enable-query", g: "Network & Packs", t: "bool", d: "false", h: "Enable GameSpy4 query protocol" },
  { k: "query.port", g: "Network & Packs", t: "num", d: "25565", h: "UDP query port" },
  { k: "enable-jmx-monitoring", g: "Network & Packs", t: "bool", d: "false", h: "Expose JMX performance metrics" },
  { k: "resource-pack", g: "Network & Packs", t: "text", d: "", h: "Direct download URL to resource pack .zip" },
  { k: "resource-pack-sha1", g: "Network & Packs", t: "text", d: "", h: "SHA-1 checksum of resource pack" },
  { k: "resource-pack-prompt", g: "Network & Packs", t: "text", d: "", h: "Custom prompt shown to players for resource pack" },
  { k: "require-resource-pack", g: "Network & Packs", t: "bool", d: "false", h: "Disconnect players who decline resource pack" },
  { k: "bug-report-link", g: "Network & Packs", t: "text", d: "", h: "Custom bug report URL shown in pause menu" },
  { k: "text-filtering-config", g: "Network & Packs", t: "text", d: "", h: "External chat filter configuration" }
];

const mcState = {
  subTab: "console",
  propSearch: "",
  propGroup: "All",
  rawPropMode: false,
  addonKind: "plugins",
  status: "running",
  cpu: "4.2%",
  memory: "1.8 GiB / 4 GiB",
  online_info: "There are 0 of a max of 20 players online",
  logs: "[Server thread/INFO]: Ready! Type a command below or click Refresh Logs.",
  properties: {},
  ops: [],
  whitelist: [],
  banned_players: [],
  banned_ips: [],
  addons: [],
  backups: [],
  schedules: [{ name: "Nightly World Backup", cron: "0 4 * * *", action: "backup", enabled: true }],
  loaded: false
};

async function mcRefreshAll() {
  mcState.loaded = true;
  const st = await callDashboardApi("/api/mc/status", "GET");
  if (st && st.ok) Object.assign(mcState, st);
  const lg = await callDashboardApi("/api/mc/console", "GET");
  if (lg && lg.logs) mcState.logs = lg.logs;
  const ad = await callDashboardApi(`/api/mc/addons?kind=${mcState.addonKind}`, "GET");
  if (ad && Array.isArray(ad.items)) mcState.addons = ad.items;
  const bk = await callDashboardApi("/api/mc/backups", "GET");
  if (bk && Array.isArray(bk.backups)) mcState.backups = bk.backups;
  renderDashboard();
}

function renderProMinecraftApp() {
  if (!mcState.loaded) setTimeout(mcRefreshAll, 20);
  const tabs = [
    ["console", "Console & Power"],
    ["properties", `All Settings (${MC_PROP_SCHEMA.length})`],
    ["players", "Players, OPs & Bans"],
    ["addons", "Plugins, Mods & Packs"],
    ["software", "Software & JVM"],
    ["backups", "Worlds & Backups"],
    ["schedules", "Schedules"]
  ];
  const isUp = mcState.status === "running";
  let body = "";

  if (mcState.subTab === "console") {
    body = `
      <div class="set-card" style="padding:12px;">
        <div style="display:flex;gap:6px;flex-wrap:wrap;margin-bottom:8px;">
          ${[["list", "Online Players"], ["save-all flush", "Save World"], ["time set day", "Set Day"], ["weather clear", "Clear Weather"], ["say Server backup starting soon!", "Broadcast"]].map(([c, l]) =>
            `<button type="button" class="win-btn win-btn-sm" data-action="mc-quick-rcon" data-cmd="${esc(c)}">${l}</button>`).join("")}
          <button type="button" class="win-btn win-btn-sm" style="margin-left:auto;" data-action="mc-refresh">↻ Refresh Console</button>
        </div>
        <pre class="fm-editor-code" style="min-height:280px;max-height:380px;overflow:auto;margin:0;border-radius:10px;">${esc(mcState.logs)}</pre>
        <div style="display:flex;gap:8px;margin-top:8px;">
          <input id="mcRconInput" class="text-input" style="flex:1;" placeholder="Enter Minecraft console / RCON command (e.g. op Steve, gamemode creative Alex, gamerule keepInventory true)..." />
          <button type="button" class="win-btn win-btn-accent" data-action="mc-send-rcon">Send Command</button>
        </div>
      </div>`;
  } else if (mcState.subTab === "properties") {
    const groups = ["All", "Gameplay & World", "Players & Security", "Performance", "Network & Packs"];
    const q = mcState.propSearch.toLowerCase();
    const filtered = MC_PROP_SCHEMA.filter(p => (mcState.propGroup === "All" || p.g === mcState.propGroup) && (!q || p.k.includes(q) || p.h.toLowerCase().includes(q)));
    const rawDump = MC_PROP_SCHEMA.map(p => `${p.k}=${mcState.properties[p.k] ?? p.d}`).join("\n");
    body = `
      <div class="set-card" style="padding:12px;display:flex;flex-direction:column;gap:10px;">
        <div style="display:flex;gap:6px;flex-wrap:wrap;align-items:center;">
          ${groups.map(g => `<button type="button" class="win-btn win-btn-sm ${mcState.propGroup === g ? "win-btn-accent" : ""}" data-action="mc-prop-group" data-group="${g}">${g}</button>`).join("")}
          <input id="mcPropSearch" class="text-input" style="width:180px;height:30px;font-size:12px;margin-left:auto;" value="${esc(mcState.propSearch)}" placeholder="Search all 62 settings..." />
          <button type="button" class="win-btn win-btn-sm" data-action="mc-toggle-raw-props">${mcState.rawPropMode ? "Structured View" : "Raw Text Editor"}</button>
        </div>
        ${mcState.rawPropMode ? `<textarea id="mcRawPropsTa" class="fm-editor-code" style="min-height:360px;border-radius:10px;">${esc(rawDump)}</textarea>` : `
          <div style="max-height:460px;overflow:auto;">
            ${filtered.map(p => {
              const val = String(mcState.properties[p.k] ?? p.d);
              const ctrl = p.t === "bool"
                ? `<select class="text-input mc-prop-inp" data-prop="${p.k}" style="width:110px;"><option value="true" ${val === "true" ? "selected" : ""}>true</option><option value="false" ${val !== "true" ? "selected" : ""}>false</option></select>`
                : p.t === "select"
                ? `<select class="text-input mc-prop-inp" data-prop="${p.k}" style="width:185px;">${p.o.map(opt => `<option value="${opt}" ${val === opt ? "selected" : ""}>${opt}</option>`).join("")}</select>`
                : `<input class="text-input mc-prop-inp" data-prop="${p.k}" type="${p.t === "num" ? "number" : "text"}" style="width:185px;" value="${esc(val)}" />`;
              return `<div class="set-row" style="padding:9px 10px;"><div><div class="set-title" style="font-family:var(--font-mono);font-size:12.5px;">${p.k}</div><div class="set-sub">${esc(p.h)}</div></div>${ctrl}</div>`;
            }).join("")}
          </div>`}
        <div style="display:flex;gap:8px;justify-content:flex-end;flex-wrap:wrap;">
          <button type="button" class="win-btn" data-action="mc-save-props" data-restart="0">Save server.properties</button>
          <button type="button" class="win-btn win-btn-accent" data-action="mc-save-props" data-restart="1">Save &amp; Restart Server</button>
        </div>
      </div>`;
  } else if (mcState.subTab === "players") {
    const isOnlineMode = (mcState.properties["online-mode"] ?? "true") === "true";
    body = `
      <div class="set-card">
        <div class="set-row">
          <div><div class="set-title">Quick Player Moderation (OP / Whitelist / Ban / Kick)</div><div class="set-sub">${esc(mcState.online_info)}</div></div>
          <div style="display:flex;gap:6px;flex-wrap:wrap;">
            <input id="mcPlayerName" class="text-input" style="width:160px;" placeholder="Player username or IP" />
            <button type="button" class="win-btn win-btn-sm win-btn-accent" data-action="mc-player-act" data-op="op">+ OP</button>
            <button type="button" class="win-btn win-btn-sm" data-action="mc-player-act" data-op="deop">De-OP</button>
            <button type="button" class="win-btn win-btn-sm" data-action="mc-player-act" data-op="whitelist_add">+ Whitelist</button>
            <button type="button" class="win-btn win-btn-sm" data-action="mc-player-act" data-op="kick">Kick</button>
            <button type="button" class="win-btn win-btn-sm" style="color:#ff6961;" data-action="mc-player-act" data-op="ban">Ban</button>
            <button type="button" class="win-btn win-btn-sm" data-action="mc-player-act" data-op="pardon">Unban</button>
          </div>
        </div>
        <div class="set-row">
          <div><div class="set-title">Online / Offline (Cracked) Authentication Mode</div><div class="set-sub">Current: <strong>${isOnlineMode ? "Online Mode (Official Mojang/Microsoft)" : "Offline Mode (Cracked/LAN Allowed)"}</strong></div></div>
          <button type="button" class="win-btn win-btn-sm" data-action="mc-toggle-online-mode">${isOnlineMode ? "Switch to Offline Mode" : "Switch to Online Mode"}</button>
        </div>
        <div class="set-row">
          <div><div class="set-title">Whitelist Enforcement</div><div class="set-sub">Allow only whitelisted players to join</div></div>
          <div style="display:flex;gap:6px;">
            <button type="button" class="win-btn win-btn-sm" data-action="mc-player-act" data-op="whitelist_on">Enable Whitelist</button>
            <button type="button" class="win-btn win-btn-sm" data-action="mc-player-act" data-op="whitelist_off">Disable Whitelist</button>
          </div>
        </div>
        <div class="set-row"><div><div class="set-title">Operators (${mcState.ops.length})</div><div class="set-sub">${mcState.ops.map(o => `${o.name} (Lvl ${o.level || 4})`).join(", ") || "None configured"}</div></div></div>
        <div class="set-row"><div><div class="set-title">Whitelisted Players (${mcState.whitelist.length})</div><div class="set-sub">${mcState.whitelist.map(w => w.name).join(", ") || "None configured"}</div></div></div>
        <div class="set-row"><div><div class="set-title">Banned Players (${mcState.banned_players.length}) &amp; IPs (${mcState.banned_ips.length})</div><div class="set-sub">${mcState.banned_players.map(b => b.name).join(", ") || "No bans"}</div></div></div>
      </div>`;
  } else if (mcState.subTab === "addons") {
    body = `
      <div class="set-card" style="padding:12px;display:flex;flex-direction:column;gap:10px;">
        <div style="display:flex;gap:6px;flex-wrap:wrap;align-items:center;">
          ${[["plugins", "Plugins (/plugins)"], ["mods", "Mods (/mods)"], ["datapacks", "Datapacks"]].map(([k, l]) =>
            `<button type="button" class="win-btn win-btn-sm ${mcState.addonKind === k ? "win-btn-accent" : ""}" data-action="mc-addon-kind" data-kind="${k}">${l}</button>`).join("")}
          <label class="win-btn win-btn-accent win-btn-sm" style="margin-left:auto;cursor:pointer;">↑ Upload Custom .jar / .zip<input type="file" id="mcAddonUpload" accept=".jar,.zip" style="display:none;" /></label>
        </div>
        <div style="display:flex;gap:6px;flex-wrap:wrap;">
          <input id="mcAddonUrl" class="text-input" style="flex:1;min-width:220px;" placeholder="Paste direct Modrinth / Hangar / GitHub .jar or .zip URL..." />
          <button type="button" class="win-btn win-btn-sm" data-action="mc-addon-url-install">Install from URL</button>
        </div>
        <table class="fm-table"><thead><tr><th>File</th><th>Status</th><th>Size</th><th>Modified</th><th>Actions</th></tr></thead><tbody>
          ${mcState.addons.map(a => `<tr><td><b>${esc(a.name)}</b></td><td><span class="badge ${a.enabled ? "badge-success" : ""}">${a.enabled ? "Enabled" : "Disabled"}</span></td><td>${(a.size / 1024).toFixed(1)} KB</td><td>${esc(a.mtime)}</td><td style="display:flex;gap:5px;"><button type="button" class="fm-act-btn" data-action="mc-addon-toggle" data-name="${esc(a.name)}">${a.enabled ? "Disable" : "Enable"}</button><button type="button" class="fm-act-btn fm-act-danger" data-action="mc-addon-del" data-name="${esc(a.name)}">Delete</button></td></tr>`).join("") || `<tr><td colspan="5" class="fm-empty">No ${mcState.addonKind} installed yet. Upload a .jar/.zip above!</td></tr>`}
        </tbody></table>
      </div>`;
  } else if (mcState.subTab === "software") {
    body = `
      <div class="set-card">
        <div class="set-row"><div><div class="set-title">Server Software Engine</div><div class="set-sub">Paper/Purpur for plugins, Fabric/Forge/NeoForge for mods</div></div>
          <select id="mcSwType" class="text-input" style="width:170px;">${["PAPER", "PURPUR", "SPIGOT", "VANILLA", "FABRIC", "FORGE", "NEOFORGE", "VELOCITY"].map(t => `<option value="${t}">${t}</option>`).join("")}</select></div>
        <div class="set-row"><div><div class="set-title">Minecraft Version</div><div class="set-sub">Enter LATEST or a specific version (e.g. 1.21.4, 1.20.6, 1.16.5)</div></div>
          <input id="mcSwVer" class="text-input" style="width:170px;" value="LATEST" /></div>
        <div class="set-row"><div><div class="set-title">Dedicated RAM Heap Allocation</div><div class="set-sub">Recommended: 2G-4G vanilla/Paper, 6G-12G heavy modpacks</div></div>
          <select id="mcSwMem" class="text-input" style="width:170px;">${["1G", "2G", "3G", "4G", "6G", "8G", "12G", "16G"].map(m => `<option value="${m}" ${m === "4G" ? "selected" : ""}>${m}</option>`).join("")}</select></div>
        <div class="set-row"><div><div class="set-title">JVM Garbage Collection Flags</div><div class="set-sub">Aikar's G1GC reduces lag spikes; ZGC excels on 8GB+ heaps</div></div>
          <select id="mcSwJvm" class="text-input" style="width:170px;"><option value="aikar">Aikar's Flags (Recommended)</option><option value="zgc">Generational ZGC</option><option value="default">Default JVM</option></select></div>
        <div class="set-row"><div><div class="set-title">GeyserMC Bedrock Crossplay (UDP 19132)</div><div class="set-sub">Allow iPhone, Android, Xbox, PlayStation &amp; Switch players to join Java server</div></div>
          <input type="checkbox" id="mcSwGeyser" /></div>
        <div class="set-row" style="justify-content:flex-end;"><button type="button" class="win-btn win-btn-accent" data-action="mc-apply-software">Apply Software &amp; Recreate Server</button></div>
      </div>`;
  } else if (mcState.subTab === "backups") {
    body = `
      <div class="set-card" style="padding:12px;display:flex;flex-direction:column;gap:10px;">
        <div style="display:flex;gap:8px;flex-wrap:wrap;align-items:center;">
          <button type="button" class="win-btn win-btn-accent win-btn-sm" data-action="mc-backup-create">+ Create RCON-Safe World Backup</button>
          <label class="win-btn win-btn-sm" style="cursor:pointer;">↑ Upload Custom World (.zip)<input type="file" id="mcWorldUpload" accept=".zip" style="display:none;" /></label>
        </div>
        <table class="fm-table"><thead><tr><th>Backup Archive</th><th>Size</th><th>Created</th><th>Actions</th></tr></thead><tbody>
          ${mcState.backups.map(b => `<tr><td><b>${esc(b.name)}</b></td><td>${(b.size / 1048576).toFixed(1)} MB</td><td>${esc(b.mtime)}</td><td style="display:flex;gap:5px;"><button type="button" class="fm-act-btn" data-action="mc-backup-restore" data-name="${esc(b.name)}">Restore</button><a class="fm-act-btn" href="/api/fm/download?path=${encodeURIComponent("/DATA/Backups/minecraft/" + b.name)}" download>Download</a><button type="button" class="fm-act-btn fm-act-danger" data-action="mc-backup-del" data-name="${esc(b.name)}">Delete</button></td></tr>`).join("") || `<tr><td colspan="4" class="fm-empty">No world backups yet. Click "+ Create RCON-Safe World Backup" above.</td></tr>`}
        </tbody></table>
      </div>`;
  } else {
    body = `
      <div class="set-card" style="padding:12px;display:flex;flex-direction:column;gap:8px;">
        <div class="set-title">Automated Server Schedules (Cron)</div>
        ${mcState.schedules.map((sc, i) => `<div class="set-row"><div><b>${esc(sc.name)}</b> <span class="badge">${esc(sc.cron)}</span> <span class="set-sub">Action: ${esc(sc.action)}</span></div><button type="button" class="fm-act-btn fm-act-danger" data-action="mc-sched-del" data-idx="${i}">Remove</button></div>`).join("")}
        <div style="display:flex;gap:6px;flex-wrap:wrap;margin-top:6px;">
          <input id="mcSchedName" class="text-input" placeholder="Task Name (e.g. Daily Restart)" />
          <input id="mcSchedCron" class="text-input" style="width:130px;" value="0 5 * * *" placeholder="Cron" />
          <select id="mcSchedAct" class="text-input"><option value="backup">World Backup</option><option value="restart">Restart Server</option><option value="save-all">Save World</option></select>
          <button type="button" class="win-btn win-btn-accent win-btn-sm" data-action="mc-sched-add">+ Add Schedule</button>
        </div>
      </div>`;
  }

  return `
    <div style="display:flex;flex-direction:column;gap:12px;animation:dashFadeSlide 0.2s ease;">
      <div class="os-hero-bar">
        <div>
          <div style="font-size:18px;font-weight:700;display:flex;align-items:center;gap:8px;">Minecraft Pro Control Panel <span class="badge ${isUp ? "badge-success" : ""}">${esc(mcState.status.toUpperCase())}</span></div>
          <div style="font-size:12px;color:var(--text-secondary);">CPU: ${esc(mcState.cpu)} • RAM: ${esc(mcState.memory)} • Java Port: ${esc(mcState.properties["server-port"] || "25565")}</div>
        </div>
        <div style="display:flex;gap:6px;flex-wrap:wrap;">
          <button type="button" class="win-btn win-btn-sm" style="background:rgba(48,209,88,0.18);color:#30d158;" data-action="mc-power" data-op="start">▶ Start</button>
          <button type="button" class="win-btn win-btn-sm" data-action="mc-power" data-op="restart">↻ Restart</button>
          <button type="button" class="win-btn win-btn-sm" data-action="mc-power" data-op="stop">■ Stop</button>
          <button type="button" class="win-btn win-btn-sm" style="color:#ff6961;" data-action="mc-power" data-op="kill">✕ Kill</button>
        </div>
      </div>
      <div class="settings-tab-strip">
        ${tabs.map(([id, lbl]) => `<button type="button" class="settings-tab-pill ${mcState.subTab === id ? "active" : ""}" data-action="mc-subtab" data-subtab="${id}">${lbl}</button>`).join("")}
      </div>
      <div class="settings-anim-panel">${body}</div>
    </div>`;
}