// FILE: src/instant_server/ui/view_studio.js
    function doesAppMatchCategory(app, cat) {
      if (!cat || cat === "All") return true;
      const appCats = Array.isArray(app.categories) ? app.categories : [app.category];
      if (cat === "Recommended") return !!app.recommended;
      if (cat === "Old Laptop Friendly") return app.hardware_tier === "old_friendly";
      if (cat === "Higher-End / Gaming & AI") {
        return app.hardware_tier === "high_end" || appCats.includes("Gaming Servers") || appCats.includes("AI & Machine Learning");
      }
      return app.category === cat || appCats.includes(cat);
    }

    function getFilteredAppsList() {
      const q = state.appSearchQuery.trim().toLowerCase();
      const cat = state.appCategoryFilter;
      return APP_STORE_CATALOG.filter(app => {
        if (!doesAppMatchCategory(app, cat)) return false;
        if (q) {
          const appCatsStr = Array.isArray(app.categories) ? app.categories.join(" ") : app.category;
          const hay = (app.title + " " + app.desc + " " + app.tag + " " + appCatsStr + " " + (app.image || "") + " " + (app.hardware_note || "")).toLowerCase();
          if (!hay.includes(q)) return false;
        }
        return true;
      });
    }

    function renderFilteredAppCardsHtml() {
      const filtered = getFilteredAppsList();

      if (filtered.length === 0) {
        return `<div class="win-card"><div class="card-title">No apps match your search filter.</div></div>`;
      }

      const initialLimit = 8;
      const canCollapse = !state.appSearchQuery.trim() && state.appCategoryFilter === "All" && filtered.length > initialLimit;
      const visibleApps = (canCollapse && !state.appsExpanded) ? filtered.slice(0, initialLimit) : filtered;

      const cardsHtml = visibleApps.map(app => {
        const isChecked = state.selected_apps.includes(app.id);
        const isHighEnd = app.hardware_tier === "high_end";
        return `
          <div class="app-card ${isChecked ? "checked" : ""}" data-action="toggle-app-card" data-app="${esc(app.id)}">
            <div style="display:flex;align-items:flex-start;gap:12px;min-width:0;flex:1;">
              <input type="checkbox" class="app-checkbox" ${isChecked ? "checked" : ""} />
              <div class="card-text">
                <div class="card-title">
                  <span>${esc(app.title)}</span>
                  ${app.recommended ? `<span class="badge badge-accent">Recommended</span>` : ""}
                  <span class="badge badge-success">${esc(app.tag)} • ${esc(app.ram)}</span>
                  <span class="badge" style="background:var(--card-bg-secondary);border:1px solid var(--border-card);color:var(--text-secondary);">${esc(app.category)}</span>
                  ${isHighEnd ? `<span class="badge badge-warning">${esc(app.hardware_note || "8GB+ RAM")}</span>` : ""}
                </div>
                <div class="card-desc">${esc(app.desc)}</div>
              </div>
            </div>
          </div>
        `;
      }).join("");

      const expandBtnHtml = canCollapse ? `
        <div style="grid-column: 1 / -1; display:flex; justify-content:center; padding-top:4px;">
          <button type="button" class="win-btn" data-action="toggle-expand-apps">
            ${state.appsExpanded ? "Show fewer apps ⌃" : `Show more apps (${filtered.length - initialLimit} more) ⌄`}
          </button>
        </div>
      ` : "";

      return cardsHtml + expandBtnHtml;
    }


    function getOrInitAppOverride(appId) {
      const found = APP_STORE_CATALOG.find(a => a.id === appId) || APP_STORE_CATALOG[0];
      if (!state.app_overrides[appId]) {
        const tpl = DEFAULT_APP_CONFIG_TEMPLATES[appId] || {
          ".env": `TZ=UTC\nPUID=1000\nPGID=1000\nAPP_PORT=${found ? found.port : 8080}\n`,
          "custom-config.conf": `# Persistent configuration file in /DATA/AppData/${appId}\n`
        };
        state.app_overrides[appId] = {
          host_port: found ? found.port : 8080,
          memory_limit: "",
          env_vars: {},
          config_files: Object.assign({}, tpl)
        };
      }
      return state.app_overrides[appId];
    }

    function buildUniversalAppFileStudioHtml() {
      const activeAppId = state.studio_app_id || (state.selected_apps[0] || "minecraft");
      const appObj = APP_STORE_CATALOG.find(a => a.id === activeAppId) || APP_STORE_CATALOG[0];
      const ov = getOrInitAppOverride(appObj.id);
      const fileKeys = Object.keys(ov.config_files || {});
      if (!state.studio_file_name || !ov.config_files[state.studio_file_name]) {
        state.studio_file_name = fileKeys[0] || ".env";
      }
      const currentFileContent = ov.config_files[state.studio_file_name] ?? "";

      const prioritizedApps = [
        ...APP_STORE_CATALOG.filter(a => state.selected_apps.includes(a.id)),
        ...APP_STORE_CATALOG.filter(a => !state.selected_apps.includes(a.id))
      ];

      return `
        <div class="win-card" style="flex-direction:column;align-items:stretch;gap:12px;">
          <div style="display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap;">
            <div class="card-left">
              <div class="card-text">
                <div class="card-title">
                  <span>Universal App Config &amp; <code>/DATA/AppData</code> File Studio</span>
                  <span class="badge badge-accent">All ${APP_STORE_CATALOG.length} Apps Supported</span>
                </div>
                <div class="card-desc">Pre-configure ports, RAM limits, and persistent config files (<code>server.properties</code>, <code>config.php</code>, <code>pihole-FTL.conf</code>, <code>.env</code>) for any app</div>
              </div>
            </div>
            <div style="display:flex;align-items:center;gap:8px;flex-wrap:wrap;">
              <select class="win-input" style="min-width:210px;" data-action="select-studio-app">
                ${prioritizedApps.map(a => `
                  <option value="${esc(a.id)}" ${a.id === appObj.id ? "selected" : ""}>
                    ${state.selected_apps.includes(a.id) ? " " : ""}${esc(a.title)} (Port ${a.port || 8080})
                  </option>
                `).join("")}
              </select>
            </div>
          </div>

          <div style="display:flex;align-items:center;justify-content:space-between;gap:10px;flex-wrap:wrap;padding-top:8px;border-top:1px solid var(--divider);">
            <div style="display:flex;align-items:center;gap:8px;flex-wrap:wrap;">
              <span style="font-size:12px;font-weight:600;color:var(--text-secondary);">Host Port:</span>
              <input type="number" class="win-input" style="width:95px;min-width:95px;" data-action="studio-port-input" value="${esc(ov.host_port)}" />
              <span style="font-size:12px;font-weight:600;color:var(--text-secondary);">RAM Cap:</span>
              <input type="text" class="win-input" style="width:105px;min-width:105px;" data-action="studio-mem-input" value="${esc(ov.memory_limit)}" placeholder="Unlimited (e.g. 2g)" />
            </div>
            <div style="display:flex;align-items:center;gap:6px;flex-wrap:wrap;">
              ${fileKeys.map(fn => `
                <button type="button" class="filter-pill ${state.studio_file_name === fn ? "active" : ""}" data-action="select-studio-file" data-file="${esc(fn)}">
                  CFG ${esc(fn)}
                </button>
              `).join("")}
              <input type="text" id="inputNewStudioFile" class="win-input" style="width:135px;min-width:135px;height:28px;font-size:12px;" placeholder="new-file.conf" value="${esc(state.studio_new_file_name || "")}" data-field="studio_new_file_name" />
              <button type="button" class="win-btn win-btn-sm" data-action="add-studio-file">＋ New File</button>
            </div>
          </div>

          <div>
            <div style="display:flex;align-items:center;justify-content:space-between;font-size:11.5px;color:var(--text-secondary);margin-bottom:4px;">
              <span>Editing <code>/DATA/AppData/${esc(appObj.id)}/${esc(state.studio_file_name)}</code> (pre-staged onto USB &amp; mounted into container)</span>
              <span>Image: <code>${esc(appObj.image || appObj.id)}</code></span>
            </div>
            <textarea
              class="win-input"
              style="width:100%;min-height:115px;padding:10px 12px;font-family:var(--font-mono);font-size:12px;line-height:1.5;resize:vertical;"
              data-action="studio-file-textarea"
              spellcheck="false"
            >${esc(currentFileContent)}</textarea>
          </div>
        </div>
      `;
    }