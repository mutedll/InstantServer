// FILE: src/instant_server/ui/app_events.js
    document.getElementById("btnThemeToggle").addEventListener("click", () => {
      state.theme = state.theme === "dark" ? "light" : "dark";
      document.documentElement.setAttribute("data-theme", state.theme);
      const themeIcon = document.getElementById("themeToggleIcon");
      if (themeIcon) {
        themeIcon.innerHTML = state.theme === "dark"
          ? `<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="5"/><line x1="12" y1="1" x2="12" y2="3"/><line x1="12" y1="21" x2="12" y2="23"/><line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/><line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/><line x1="1" y1="12" x2="3" y2="12"/><line x1="21" y1="12" x2="23" y2="12"/><line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/><line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/></svg>`
          : `<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/></svg>`;
      }
    });
    document.getElementById("btnToggleModeView").addEventListener("click", () => {
      state.viewMode = state.viewMode === "setup" ? "settings" : "setup";
      state.openDropdownField = null;
      shouldAnimateTab = true;
      renderApp();
    });
    document.getElementById("btnTitleBack").addEventListener("click", () => {
      state.openDropdownField = null;
      if (state.viewMode === "setup") {
        const idx = SETUP_TABS.findIndex(t => t.id === state.setupTab);
        if (idx > 0) {
          state.setupTab = SETUP_TABS[idx - 1].id;
          shouldAnimateTab = true;
          renderApp();
        }
      } else {
        if (state.searchQuery) {
          state.searchQuery = "";
          document.getElementById("globalSearchInput").value = "";
          shouldAnimateTab = true;
          renderApp();
        } else if (state.settingsTab !== "system") {
          state.settingsTab = "system";
          shouldAnimateTab = true;
          renderApp();
        }
      }
    });
    document.getElementById("globalSearchInput").addEventListener("input", (e) => {
      state.searchQuery = e.target.value;
      renderSettingsView();
    });
    document.body.addEventListener("click", async (e) => {
      const target = e.target.closest("[data-action]");
      if (!e.target.closest(".win-combobox") && state.openDropdownField !== null) {
        state.openDropdownField = null;
        document.querySelectorAll(".win-combobox.open").forEach(cb => cb.classList.remove("open"));
        document.querySelectorAll(".expander-wrap.dropdown-open-inside").forEach(ex => ex.classList.remove("dropdown-open-inside"));
      }
      if (!target) return;
      const action = target.getAttribute("data-action");
      const allowedWhileFlashing = new Set([
        "pause-resume-flash",
        "cancel-flash",
        "resume-log-autoscroll",
        "dismiss-close-warning",
        "pause-and-exit-flash",
        "confirm-cancel-and-exit",
        "open-dashboard-preview",
        "copy-yaml",
        "copy-text",
        "toggle-sidebar-pass",
        "set-ui-mode"
      ]);
      if (state.flashing && action && !allowedWhileFlashing.has(action)) {
        showToast("Configuration is locked while USB flashing/downloading is active");
        return;
      }
      if (action === "toggle-dropdown") {
        const field = target.getAttribute("data-field");
        const box = target.closest(".win-combobox");
        const willOpen = state.openDropdownField !== field;
        document.querySelectorAll(".win-combobox.open").forEach(cb => cb.classList.remove("open", "drop-up", "align-right"));
        document.querySelectorAll(".win-card.has-open-dropdown").forEach(c => c.classList.remove("has-open-dropdown"));
        document.querySelectorAll(".expander-wrap.dropdown-open-inside").forEach(ex => ex.classList.remove("dropdown-open-inside"));
        state.openDropdownField = willOpen ? field : null;
        if (box && willOpen) {
          box.classList.add("open");
          const parentCard = box.closest(".win-card");
          if (parentCard) parentCard.classList.add("has-open-dropdown");
          const parentExp = box.closest(".expander-wrap.open");
          if (parentExp) parentExp.classList.add("dropdown-open-inside");
          const rect = box.getBoundingClientRect();
          if (window.innerHeight - rect.bottom < 250 && rect.top > 240) {
            box.classList.add("drop-up");
          }
          if (rect.left > window.innerWidth * 0.48) {
            box.classList.add("align-right");
          }
        }
        return;
      } else if (action === "select-dropdown-option") {
        const field = target.getAttribute("data-field");
        const val = target.getAttribute("data-value");
        const label = target.getAttribute("data-label") || val;
        if (field) {
          state[field] = val;
        }
        if (field === "laptop_age_hint") {
          if (val === "modern_uefi") state.usb_partition_scheme = "gpt_uefi";
          else if (val === "legacy_bios") state.usb_partition_scheme = "mbr_bios";
          else state.usb_partition_scheme = "auto";
          state.openDropdownField = null;
          renderApp();
          return;
        }
        if (field === "cluster_mode" || field === "cluster_storage_policy" || field === "install_mode" || field === "dashboard_choice") {
          state.openDropdownField = null;
          renderApp();
          return;
        }
        if (field === "iso_source_mode") {
          const isoIn = document.getElementById("customIsoPathInput");
          if (isoIn) isoIn.style.display = val === "local_file" ? "" : "none";
        }
        state.openDropdownField = null;
        const box = target.closest(".win-combobox");
        if (box) {
          box.classList.remove("open");
          const trigLbl = box.querySelector(".win-combobox-label");
          if (trigLbl) trigLbl.textContent = label;
          box.querySelectorAll(".win-combobox-option").forEach(opt => {
            const sel = opt.getAttribute("data-value") === val;
            opt.classList.toggle("selected", sel);
            opt.setAttribute("aria-selected", String(sel));
          });
        }
        if (field === "screen_blank_seconds") {
          const blankTitle = document.getElementById("lblScreenBlankTitle");
          if (blankTitle) blankTitle.textContent = `Turn off screen after ${val} seconds`;
        }
        return;
      }
      if (action === "set-domain-preset") {
        state.server_address_domain = target.getAttribute("data-domain") || ".local";
        renderApp();
        return;
      }
      if (action === "toggle-sidebar-pass") {
        state.sidebarRevealPass = !state.sidebarRevealPass;
        if (typeof renderSidebarSummaryCard === "function") renderSidebarSummaryCard();
        return;
      }
      if (action === "copy-text") {
        const txt = target.getAttribute("data-copy") || "";
        if (txt && navigator.clipboard) {
          navigator.clipboard.writeText(txt).catch(() => {});
          showToast("Copied: " + txt);
        }
        return;
      }
      if (action === "open-quick-edit-modal") {
        const modal = document.getElementById("quickEditServerModal");
        const body = document.getElementById("quickEditModalBody");
        if (modal && body) {
          const warn = getDomainSecurityWarning(state.server_address_domain);
          body.innerHTML = `
            <label style="font-size:12px;color:var(--text-secondary);">Server Display Name
              <input id="qeServerName" class="win-input" style="width:100%;min-width:0;max-width:100%;box-sizing:border-box;margin-top:4px;" value="${esc(state.server_name || state.hostname)}" />
            </label>
            <div style="display:flex;gap:8px;">
              <label style="flex:1;min-width:0;font-size:12px;color:var(--text-secondary);">Address Host
                <input id="qeAddrHost" class="win-input" style="width:100%;min-width:0;max-width:100%;box-sizing:border-box;margin-top:4px;" value="${esc(state.server_address_host || state.hostname)}" />
              </label>
              <label style="flex:0 0 132px;min-width:0;font-size:12px;color:var(--text-secondary);">Domain
                <input id="qeAddrDomain" class="win-input" style="width:100%;min-width:0;max-width:100%;box-sizing:border-box;margin-top:4px;" value="${esc(state.server_address_domain || ".local")}" />
              </label>
            </div>
            ${warn ? `<div class="notice-banner" style="margin:0;font-size:11.5px;">${esc(warn)}</div>` : ""}
            <div style="display:flex;gap:8px;">
              <label style="flex:1;min-width:0;font-size:12px;color:var(--text-secondary);">Admin Username
                <input id="qeUsername" class="win-input" style="width:100%;min-width:0;max-width:100%;box-sizing:border-box;margin-top:4px;" value="${esc(state.username || "admin")}" />
              </label>
              <label style="flex:1;min-width:0;font-size:12px;color:var(--text-secondary);">Admin Password
                <input id="qePassword" type="password" class="win-input" style="width:100%;min-width:0;max-width:100%;box-sizing:border-box;margin-top:4px;" value="${esc(state.password || "")}" />
              </label>
            </div>
            <div style="display:flex;gap:8px;">
              <label style="flex:1;min-width:0;font-size:12px;color:var(--text-secondary);">Wi-Fi SSID (Optional)
                <input id="qeWifiSsid" class="win-input" style="width:100%;min-width:0;max-width:100%;box-sizing:border-box;margin-top:4px;" value="${esc(state.wifi_ssid || "")}" />
              </label>
              <label style="flex:0 0 132px;min-width:0;font-size:12px;color:var(--text-secondary);">Log Retention (Days)
                <input id="qeLogDays" type="number" min="1" max="365" class="win-input" style="width:100%;min-width:0;max-width:100%;box-sizing:border-box;margin-top:4px;" value="${Number(state.log_retention_days) || 14}" />
              </label>
            </div>
          `;
          modal.classList.add("open");
        }
        return;
      }
      if (action === "close-quick-edit-modal") {
        const modal = document.getElementById("quickEditServerModal");
        if (modal) modal.classList.remove("open");
        return;
      }
      if (action === "save-quick-edit-modal") {
        const g = id => document.getElementById(id);
        if (g("qeServerName")) state.server_name = g("qeServerName").value.trim() || "Instant Server";
        if (g("qeAddrHost")) { state.server_address_host = g("qeAddrHost").value.trim() || "instant-server"; state.hostname = state.server_address_host; }
        if (g("qeAddrDomain")) state.server_address_domain = normalizeServerDomain(g("qeAddrDomain").value);
        if (g("qeUsername")) state.username = g("qeUsername").value.trim() || "admin";
        if (g("qePassword")) state.password = g("qePassword").value;
        if (g("qeWifiSsid")) state.wifi_ssid = g("qeWifiSsid").value.trim();
        if (g("qeLogDays")) state.log_retention_days = Math.max(1, Math.min(365, Number(g("qeLogDays").value) || 14));
        const m = g("quickEditServerModal"); if (m) m.classList.remove("open");
        showToast("Server details updated"); renderApp(); return;
      }
      if (action === "toggle-app-card") {
        const appId = target.getAttribute("data-app");
        if (!appId) return;
        const idx = state.selected_apps.indexOf(appId);
        if (idx >= 0) state.selected_apps.splice(idx, 1);
        else state.selected_apps.push(appId);
        const nowChecked = state.selected_apps.includes(appId);
        target.classList.toggle("checked", nowChecked);
        const cb = target.querySelector(".app-checkbox");
        if (cb) cb.checked = nowChecked;
        syncSelectAllCheckboxInPlace();
        return;
      } else if (action === "toggle-select-all-apps") {
        e.preventDefault();
        const filtered = getFilteredAppsList();
        const targetList = filtered.length > 0 ? filtered : APP_STORE_CATALOG;
        const allInFilterSelected = targetList.every(a => state.selected_apps.includes(a.id));
        if (allInFilterSelected) {
          const removeSet = new Set(targetList.map(a => a.id));
          state.selected_apps = state.selected_apps.filter(id => !removeSet.has(id));
        } else {
          const currentSet = new Set(state.selected_apps);
          targetList.forEach(a => currentSet.add(a.id));
          state.selected_apps = Array.from(currentSet);
        }
        refreshAppGridInPlace();
        return;
      } else if (action === "apps-select-all") {
        state.selected_apps = APP_STORE_CATALOG.map(a => a.id);
        refreshAppGridInPlace();
        return;
      } else if (action === "apps-select-recommended") {
        state.selected_apps = APP_STORE_CATALOG.filter(a => a.recommended).map(a => a.id);
        refreshAppGridInPlace();
        return;
      } else if (action === "apps-select-old-laptop") {
        state.selected_apps = ["jellyfin", "pihole", "kiwix", "vaultwarden", "filebrowser", "syncthing", "navidrome"];
        refreshAppGridInPlace();
        showToast("Selected lightweight pack for 2 GB – 4 GB old laptops");
        return;
      } else if (action === "apps-select-high-end") {
        state.selected_apps = ["jellyfin", "nextcloud", "pihole", "kiwix", "immich", "homeassistant", "vaultwarden", "minecraft", "ollama_webui"];
        refreshAppGridInPlace();
        showToast("Selected Higher-End Homelab + Minecraft + Local AI pack");
        return;
      } else if (action === "apps-unselect-all") {
        state.selected_apps = [];
        refreshAppGridInPlace();
        return;
      } else if (action === "filter-app-category") {
        state.appCategoryFilter = target.getAttribute("data-category") || "All";
        refreshAppGridInPlace();
        return;
      }
      if (action === "toggle-expander") {
        const key = target.getAttribute("data-expander");
        if (key) {
          state.advancedExpanded[key] = !state.advancedExpanded[key];
          const wrap = target.closest(".expander-wrap");
          if (wrap) {
            wrap.classList.remove("dropdown-open-inside");
            wrap.classList.toggle("open", state.advancedExpanded[key]);
          }
        }
        return;
      }
      if (await handleAppActionClick(action, target, e)) return;
    });