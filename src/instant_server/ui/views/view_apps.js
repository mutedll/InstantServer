// FILE: src/instant_server/ui/view_apps.js
    function buildAppsSection() {
      const dashboards = [
        { id: "both", icon: "", title: "Cockpit + CasaOS", desc: "System web console plus the CasaOS 1-click app store (install even more apps anytime later)", rec: true },
        { id: "cockpit", icon: "PRO", title: "Cockpit", desc: "Lightweight system manager for storage, network, and terminal", rec: false },
        { id: "casaos", icon: "DKR", title: "CasaOS", desc: "Self-hosted home cloud dashboard and 1-click Docker app store", rec: false },
        { id: "none", icon: "CLI", title: "SSH Only", desc: "Minimal terminal server with no web dashboard", rec: false }
      ];

      const estRamMb = state.selected_apps.reduce((acc, id) => {
        const found = APP_STORE_CATALOG.find(a => a.id === id);
        if (!found) return acc;
        const m = String(found.ram || "128 MB").match(/([0-9.]+)\s*(GB|MB)/i);
        if (!m) return acc + 128;
        return acc + (m[2].toUpperCase() === "GB" ? Number(m[1]) * 1024 : Number(m[1]));
      }, 420);
      const estRamGb = (estRamMb / 1024).toFixed(1);
      const estWatts = state.device_form_factor === "mini_pc_n100" ? "5.2W" : (state.cpu_governor === "powersave" ? "9.4W" : "11.8W");

      const basicHtml = `
        <div class="section-label">1. One-click server role presets (Or customize individual apps below)</div>
        <div class="presets-grid">
          ${SERVER_ROLE_PRESETS.map(p => `
            <div class="preset-card ${state.preset_role === p.id ? "selected" : ""}" data-action="apply-role-preset" data-preset="${esc(p.id)}">
              <div style="display:flex;align-items:center;justify-content:space-between;gap:6px;">
                <span style="font-weight:600;font-size:13px;">${esc(p.title)}</span>
                <span class="badge badge-accent">${esc(p.badge)}</span>
              </div>
              <div style="font-size:11.5px;color:var(--text-secondary);line-height:1.4;">${esc(p.desc)}</div>
            </div>
          `).join("")}
        </div>

        <div class="win-card" style="border-color:var(--accent);">
          <div class="card-left">
            <div class="card-text">
              <div class="card-title">Estimated Memory &amp; Server Homepage</div>
              <div class="card-desc">Selected apps (${state.selected_apps.length}) use ~<strong>${estRamGb} GB RAM</strong> • Idle power ~<strong>${estWatts}</strong> • Includes Server Homepage at <code>http://${esc(state.hostname || "instant-server")}.local:${state.dashboard_port}</code></div>
            </div>
          </div>
          <div class="card-right">
            <span class="badge badge-success">${Number(estRamGb) <= 3.5 ? "Fits 4GB+ Computers" : "8GB+ RAM Recommended"}</span>
          </div>
        </div>

        <div class="section-label">2. Primary web dashboard</div>
        <div class="card-stack">
          ${dashboards.map(d => {
            const active = state.dashboard_choice === d.id;
            return `
              <div class="win-card clickable ${active ? "selected-accent" : ""}" data-action="select-dashboard" data-value="${d.id}">
                <div class="card-left">
                  <div class="card-text">
                    <div class="card-title">
                      ${esc(d.title)}
                      ${d.rec ? `<span class="badge badge-accent">Recommended</span>` : ""}
                    </div>
                    <div class="card-desc">${esc(d.desc)}</div>
                  </div>
                </div>
                <div class="card-right">
                  <input type="radio" name="dash_choice" ${active ? "checked" : ""} style="accent-color:var(--accent);width:17px;height:17px;pointer-events:none;" />
                </div>
              </div>
            `;
          }).join("")}
        </div>

        <div style="display:flex;align-items:center;justify-content:space-between;flex-wrap:wrap;gap:8px;margin-top:4px;">
          <div>
            <div class="section-label" style="margin:0;">3. Pre-select self-hosted apps to bundle onto USB (<span id="lblSelectedAppsCount">${state.selected_apps.length}</span> selected)</div>
            <div style="font-size:12px;color:var(--text-secondary);">Selected apps are staged onto your USB drive and automatically set up in CasaOS &amp; Docker.</div>
          </div>
          <div style="display:flex;align-items:center;gap:8px;flex-wrap:wrap;">
            <label class="select-all-checkbox-pill" id="chkSelectAllAppsWrap" data-action="toggle-select-all-apps" title="Toggle selecting or unselecting all apps">
              <input type="checkbox" id="chkSelectAllApps" ${state.selected_apps.length >= APP_STORE_CATALOG.length ? "checked" : ""} />
              <span id="lblSelectAllAppsText">${state.selected_apps.length >= APP_STORE_CATALOG.length ? "Unselect all apps" : `Select all (${state.selected_apps.length}/${APP_STORE_CATALOG.length})`}</span>
            </label>
            <button type="button" class="win-btn win-btn-sm" data-action="apps-select-recommended">Recommended</button>
            <button type="button" class="win-btn win-btn-sm" data-action="apps-select-old-laptop">Old Laptop Pack</button>
            <button type="button" class="win-btn win-btn-sm" data-action="apps-select-high-end">Higher-End + AI &amp; Gaming</button>
          </div>
        </div>

        <div class="win-card" style="padding:10px 14px;">
          <div class="card-left">
            <div class="card-text">
              <div class="card-title">Add custom app or any app from the web</div>
              <div class="card-desc">Paste a Docker image, <code>docker run</code> command, or GitHub / Docker Hub URL</div>
            </div>
          </div>
          <div class="card-right" style="flex:1;max-width:460px;display:flex;gap:8px;">
            <input
              type="text"
              class="win-input"
              style="flex:1;min-width:200px;"
              id="customWebAppInput"
              data-field="customWebAppInput"
              value="${esc(state.customWebAppInput || "")}"
              placeholder="e.g. linuxserver/plex:latest, docker run -p 8080:80 ..., or GitHub URL"
            />
            <button type="button" class="win-btn win-btn-accent" data-action="add-custom-web-app">＋ Add App</button>
          </div>
        </div>

        <div class="appstore-toolbar">
          <div class="appstore-top-row">
            <input
              type="text"
              class="win-input"
              style="flex:1;min-width:220px;"
              id="appStoreSearchInput"
              data-field="appSearchQuery"
              value="${esc(state.appSearchQuery)}"
              placeholder="Search ${APP_STORE_CATALOG.length}+ self-hosted apps (Jellyfin, Nextcloud, Immich, Pi-hole, Ollama, Minecraft, Grafana, Frigate)..."
            />
          </div>
          <div class="appstore-filter-pills" id="appCategoryPills">
            ${APP_CATEGORIES.map(cat => {
              const cnt = APP_STORE_CATALOG.filter(a => doesAppMatchCategory(a, cat)).length;
              return `
                <button type="button" class="filter-pill ${state.appCategoryFilter === cat ? "active" : ""}" data-action="filter-app-category" data-category="${esc(cat)}">
                  ${esc(cat)} <span style="opacity:0.75;font-size:11px;margin-left:3px;">(${cnt})</span>
                </button>
              `;
            }).join("")}
          </div>
        </div>

        
        <div class="win-card" id="minecraftCustomizationStudioCard" style="flex-direction:column;align-items:stretch;gap:12px;border-color:var(--accent);display:${(state.selected_apps.includes("minecraft") || state.selected_apps.includes("minecraft_bedrock")) ? "flex" : "none"};">
          <div style="display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap;">
            <div class="card-left">
              <div class="card-text">
                <div class="card-title">
                  <span>Minecraft Server Settings</span>
                  <span class="badge badge-accent">Java &amp; Bedrock</span>
                </div>
                <div class="card-desc">Quick setup for Minecraft <strong>${esc(state.mc_version || "26.3")}</strong>. Manage worlds, files, and console later in the Web Dashboard.</div>
              </div>
            </div>
          </div>

          <div style="display:grid;grid-template-columns:repeat(auto-fit, minmax(210px, 1fr));gap:12px;padding-top:8px;border-top:1px solid var(--divider);">
            <div>
              <div style="font-size:11.5px;font-weight:600;color:var(--text-secondary);margin-bottom:4px;">Server Type</div>
              ${renderComboBox("mc_server_type", [
                { value: "PAPER", label: "Paper (Fastest + Plugins)" },
                { value: "GEYSER_FLOODGATE", label: "Paper + Bedrock Crossplay" },
                { value: "PURPUR", label: "Purpur" },
                { value: "FABRIC", label: "Fabric (Mods)" },
                { value: "FORGE", label: "Forge (Modpacks)" },
                { value: "NEOFORGE", label: "NeoForge" },
                { value: "VANILLA", label: "Vanilla" }
              ])}
            </div>

            <div>
              <div style="font-size:11.5px;font-weight:600;color:var(--text-secondary);margin-bottom:4px;">Version</div>
              ${renderComboBox("mc_version", [
                { value: "26.3", label: "26.3 (Latest)" },
                { value: "26.2", label: "26.2" },
                { value: "26.1", label: "26.1" },
                { value: "1.21.11", label: "1.21.11" },
                { value: "1.21.4", label: "1.21.4" },
                { value: "LATEST", label: "Auto-Latest" }
              ])}
            </div>

            <div>
              <div style="font-size:11.5px;font-weight:600;color:var(--text-secondary);margin-bottom:4px;">Memory (RAM)</div>
              ${renderComboBox("mc_memory_heap", [
                { value: "2G", label: "2 GB RAM" },
                { value: "4G", label: "4 GB RAM (Recommended)" },
                { value: "6G", label: "6 GB RAM" },
                { value: "8G", label: "8 GB RAM" },
                { value: "12G", label: "12 GB RAM" }
              ])}
            </div>

            <div>
              <div style="font-size:11.5px;font-weight:600;color:var(--text-secondary);margin-bottom:4px;">Game Mode</div>
              ${renderComboBox("mc_gamemode", [
                { value: "survival", label: "Survival" },
                { value: "creative", label: "Creative" },
                { value: "adventure", label: "Adventure" }
              ])}
            </div>

            <div>
              <div style="font-size:11.5px;font-weight:600;color:var(--text-secondary);margin-bottom:4px;">Difficulty</div>
              ${renderComboBox("mc_difficulty", [
                { value: "normal", label: "Normal" },
                { value: "hard", label: "Hard" },
                { value: "easy", label: "Easy" },
                { value: "peaceful", label: "Peaceful" }
              ])}
            </div>

            <div>
              <div style="font-size:11.5px;font-weight:600;color:var(--text-secondary);margin-bottom:4px;">Max Players &amp; Admins</div>
              <div style="display:flex;gap:6px;min-width:0;">
                <input class="win-input" type="number" style="width:74px;min-width:0;flex-shrink:0;" data-field="mc_max_players" value="${esc(state.mc_max_players)}" placeholder="20" title="Max Players" />
                <input class="win-input" style="flex:1;min-width:0;" data-field="mc_ops_list" value="${esc(state.mc_ops_list)}" placeholder="Admin usernames" />
              </div>
            </div>

            <div>
              <div style="font-size:11.5px;font-weight:600;color:var(--text-secondary);margin-bottom:4px;">Plugins / Mods (Optional)</div>
              <input class="win-input" style="width:100%;min-width:0;" data-field="mc_modrinth_projects" value="${esc(state.mc_modrinth_projects)}" placeholder="e.g. viaversion, worldedit" />
            </div>

            <div>
              <div style="font-size:11.5px;font-weight:600;color:var(--text-secondary);margin-bottom:4px;">World Seed &amp; Server Name</div>
              <div style="display:flex;gap:6px;min-width:0;">
                <input class="win-input" style="width:96px;min-width:0;flex-shrink:0;" data-field="mc_level_seed" value="${esc(state.mc_level_seed)}" placeholder="Seed" />
                <input class="win-input" style="flex:1;min-width:0;" data-field="mc_motd" value="${esc(state.mc_motd)}" placeholder="Server name" />
              </div>
            </div>
          </div>

          <div style="display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap;padding-top:8px;border-top:1px solid var(--divider);font-size:12px;">
            <label style="display:flex;align-items:center;gap:8px;cursor:pointer;">
              <input type="checkbox" data-action="toggle-mc-online-mode" ${state.mc_online_mode ? "checked" : ""} style="accent-color:var(--accent);width:16px;height:16px;" />
              <span>Require Mojang sign-in (uncheck for offline/LAN players)</span>
            </label>
            <label style="display:flex;align-items:center;gap:8px;cursor:pointer;">
              <input type="checkbox" data-action="toggle-mc-aikar" ${state.mc_use_aikar_flags ? "checked" : ""} style="accent-color:var(--accent);width:16px;height:16px;" />
              <span>Performance flags &amp; automatic backups</span>
            </label>
          </div>
        </div>

      `;

      const advHtml = `
        <div class="win-card">
          <div class="card-left">
            <div class="card-text">
              <div class="card-title">Pre-download container images on this PC (Offline bundle)</div>
              <div class="card-desc">Downloads the full Docker image layers onto the USB stick from this PC so apps install even without internet</div>
            </div>
          </div>
          <div class="card-right">${renderSwitch("toggle-offline-images", state.prebundle_offline_images)}</div>
        </div>

        <div class="win-card">
          <div class="card-left">
            <div class="card-text">
              <div class="card-title">Pi-hole DNS</div>
              <div class="card-desc">Prepare DNS settings so Pi-hole or AdGuard Home can run without port conflicts</div>
            </div>
          </div>
          <div class="card-right">${renderSwitch("toggle-pihole", state.enable_pihole_dns_prep)}</div>
        </div>

        <div class="win-card">
          <div class="card-left">
            <div class="card-text">
              <div class="card-title">Samba File Share</div>
              <div class="card-desc">Share folders across Windows, macOS, and Linux computers on your home network</div>
            </div>
          </div>
          <div class="card-right">${renderSwitch("toggle-samba", state.enable_samba_nas)}</div>
        </div>

        <div class="win-card">
          <div class="card-left">
            <div class="card-text">
              <div class="card-title">Hardware Video Transcoding</div>
              <div class="card-desc">Enable Intel, AMD, or NVIDIA graphics acceleration for Jellyfin and Plex</div>
            </div>
          </div>
          <div class="card-right">${renderSwitch("toggle-vaapi", state.enable_intel_vaapi)}</div>
        </div>

        <div class="win-card">
          <div class="card-left">
            <div class="card-text">
              <div class="card-title">Tailscale VPN</div>
              <div class="card-desc">Access your server remotely without opening router ports</div>
            </div>
          </div>
          <div class="card-right" style="flex-wrap:wrap;">
            <input
              id="tailscaleAuthInput"
              class="win-input"
              style="min-width:160px; ${state.enable_tailscale ? "" : "display:none;"}"
              data-field="tailscale_authkey"
              value="${esc(state.tailscale_authkey)}"
              placeholder="Auth key (optional)"
            />
            ${renderSwitch("toggle-tailscale", state.enable_tailscale)}
          </div>
        </div>

        <div class="win-card">
          <div class="card-left">
            <div class="card-text">
              <div class="card-title">Offline Wikipedia &amp; survival pack for Kiwix</div>
              <div class="card-desc">Choose which offline encyclopedia (.zim) to pre-configure in your Server Homepage &amp; Kiwix library</div>
            </div>
          </div>
          <div class="card-right">
            ${renderComboBox("preselect_kiwix_zim", KIWIX_ZIM_PACKS.map(z => ({ value: z.id, label: z.label })))}
          </div>
        </div>

        <div class="win-card" style="flex-direction:column;align-items:stretch;gap:10px;border:1px solid var(--accent);">
          <div style="display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap;">
            <div class="card-left">
              <div class="card-text">
                <div class="card-title">Custom Server Workflow &amp; Automation Builder <span class="badge badge-accent">Full Control</span></div>
                <div class="card-desc">Build any custom server workflow: install extra Ubuntu packages (<code>apt</code>), create custom Samba NAS shares, add USB hardware trigger rules (<code>/etc/udev/rules.d/</code>), add background <code>systemd</code> services/timers, or stage any <code>/etc</code> / <code>/DATA</code> config file &amp; post-install script (${Object.keys(state.custom_config_files || {}).length} file(s) staged).</div>
              </div>
            </div>
            <div class="card-right" style="gap:6px;flex-wrap:wrap;">
              <button type="button" class="win-btn win-btn-sm" data-action="load-workflow-template" data-tpl="samba_share">DIR + Custom NAS Share</button>
              <button type="button" class="win-btn win-btn-sm" data-action="load-workflow-template" data-tpl="usb_udev">UPS + USB Device Trigger</button>
              <button type="button" class="win-btn win-btn-sm" data-action="load-workflow-template" data-tpl="systemd_timer">TMR + Scheduled Service</button>
            </div>
          </div>
          <div style="display:flex;align-items:center;gap:8px;flex-wrap:wrap;padding-top:6px;border-top:1px solid var(--divider);">
            <span style="font-size:12px;font-weight:600;color:var(--text-secondary);">Extra Ubuntu Packages (apt):</span>
            <input class="win-input" style="flex:1;min-width:180px;" data-field="custom_apt_packages_raw" value="${esc(state.custom_apt_packages_raw || "")}" placeholder="e.g. ffmpeg rsync python3-pip rclone usbmuxd" />
            <span style="font-size:12px;font-weight:600;color:var(--text-secondary);">Target File Path:</span>
            <input class="win-input" style="min-width:210px;" data-field="custom_config_path" value="${esc(state.custom_config_path)}" placeholder="/etc/samba/smb-custom.conf or /etc/udev/rules.d/99-usb.rules" />
            <button type="button" class="win-btn win-btn-accent win-btn-sm" data-action="save-custom-sys-config">＋ Stage Custom File</button>
          </div>
          <textarea class="win-input" rows="4" style="width:100%;font-family:var(--font-mono);font-size:12px;" data-field="custom_config_body" placeholder="Paste custom config file, systemd unit, udev rule, or automation script here...">${esc(state.custom_config_body)}</textarea>
        </div>

        <div class="win-card">
          <div class="card-left">
            <div class="card-text">
              <div class="card-title">Automatic OS security &amp; web app updates</div>
              <div class="card-desc">Automatically installs Ubuntu security patches and updates Docker apps at ${esc(state.auto_update_window)}</div>
            </div>
          </div>
          <div class="card-right">${renderSwitch("toggle-auto-updates", state.enable_auto_updates)}</div>
        </div>

        <div class="win-card">
          <div class="card-left">
            <div class="card-text">
              <div class="card-title">Automated weekly AppData backups</div>
              <div class="card-desc">Automatically creates compressed snapshots of /DATA/AppData in /DATA/Backups</div>
            </div>
          </div>
          <div class="card-right">${renderSwitch("toggle-scheduled-backups", state.enable_scheduled_backups)}</div>
        </div>

        ${state.ui_mode === "pro" ? `
          <div class="win-card">
            <div class="card-left">
              <div class="card-text">
                <div class="card-title">Docker network range &amp; Prometheus metrics</div>
                <div class="card-desc">Custom Docker bridge network range and optional system metrics endpoint</div>
              </div>
            </div>
            <div class="card-right" style="flex-wrap:wrap;">
              <input class="win-input" style="min-width:150px;" data-field="docker_subnet_cidr" value="${esc(state.docker_subnet_cidr)}" placeholder="172.17.0.1/16" />
              ${renderSwitch("toggle-prometheus", state.enable_prometheus_metrics)}
            </div>
          </div>
        ` : ""}
      `;

      return `
        <div class="tab-flow-stack">
          ${basicHtml}
          <div class="appstore-grid" id="appStoreGridContainer">
            ${renderFilteredAppCardsHtml()}
          </div>
          ${renderExpander("apps", "Advanced options", advHtml + buildUniversalAppFileStudioHtml())}
        </div>
      `;
    }