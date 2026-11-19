// FILE: src/instant_server/ui/view_shell.js
    function renderSetupView() {
      const topTabsEl = document.getElementById("setupTopTabs");
      const bodyEl = document.getElementById("setupBodyContent");
      const footerEl = document.getElementById("setupFooterBar");

      topTabsEl.innerHTML = SETUP_TABS.map((t, idx) => {
        const active = state.setupTab === t.id;
        return `
          <button type="button" class="setup-tab-btn ${active ? "active" : ""}" data-action="switch-setup-tab" data-tab="${t.id}">
            <span class="setup-tab-step-dot">${idx + 1}</span>
            <span>${esc(t.label)}</span>
          </button>
        `;
      }).join("");

      const tabIdx = SETUP_TABS.findIndex(t => t.id === state.setupTab);
      const headers = {
        network: { title: "Connect your server", desc: "Name your server and optionally enter your home Wi-Fi details." },
        apps: { title: "Choose dashboards & self-hosted apps", desc: "Pick your web dashboard and check the self-hosted apps you want pre-bundled onto the USB drive." },
        storage: { title: "Target computer details & bootable USB", desc: "Choose your computer type and installation mode (or Repair USB mode) and select your USB stick." },
        power: { title: "Power & battery settings", desc: "Configure lid-close behavior and battery protection for 24/7 operation." },
        finish: { title: "Ready to format & flash bootable USB", desc: "Pulls the official Ubuntu Server ISO, formats the USB drive, and bundles your config and selected apps." }
      };
      const hdr = headers[state.setupTab] || headers.network;

      let contentHtml = "";
      if (state.setupTab === "network") contentHtml = buildNetworkSection();
      else if (state.setupTab === "apps") contentHtml = buildAppsSection();
      else if (state.setupTab === "storage") contentHtml = buildStorageSection();
      else if (state.setupTab === "power") contentHtml = buildPowerSection();
      else contentHtml = buildFinishSection();

      const animClass = shouldAnimateTab ? "animate-tab-switch" : "";
      shouldAnimateTab = false;

      bodyEl.innerHTML = `
        <div class="${animClass} tab-flow-stack">
          <div>
            <h1 class="setup-header-title">${esc(hdr.title)}</h1>
            <p class="setup-header-desc">${esc(hdr.desc)}</p>
          </div>
          ${contentHtml}
        </div>
      `;
      bodyEl._restoringScroll = true;
      bodyEl.scrollTop = (state.tabScrollPositions && state.tabScrollPositions.setup && state.tabScrollPositions.setup[state.setupTab]) || 0;
      bodyEl._restoringScroll = false;

      const isFirst = tabIdx <= 0;
      const isLast = tabIdx === SETUP_TABS.length - 1;

      footerEl.innerHTML = `
        <div>
          <button type="button" class="win-btn" data-action="setup-prev" ${isFirst ? "disabled" : ""}>Previous</button>
        </div>
        <div class="setup-footer-right">
          ${!isLast ? `
            <button type="button" class="win-btn" data-action="setup-skip">Skip</button>
            <button type="button" class="win-btn win-btn-accent" data-action="setup-next">Proceed</button>
          ` : `
            <button type="button" class="win-btn" data-action="save-config-home">Save configuration</button>
            <span class="footer-cancel-flash-wrap" style="display:${state.flashing ? "inline-flex" : "none"};gap:8px;">
              <button type="button" class="win-btn" data-action="pause-resume-flash">${state.flashPaused ? `Resume (${state.flashProgress}%)` : "Pause"}</button>
              <button type="button" class="win-btn win-btn-danger" data-action="cancel-flash">Cancel</button>
            </span>
            <button type="button" class="win-btn" data-action="verify-usb-drive" ${state.verifyingUsb || state.flashing ? "disabled" : ""}>
              ${state.verifyingUsb ? "Verifying..." : "Verify Bootable USB"}
            </button>
            <button type="button" class="win-btn" data-action="boot-usb-next-restart" ${state.flashing ? "disabled" : ""}>
              Boot USB on Next Restart
            </button>
            <button type="button" class="win-btn win-btn-accent win-btn-large" data-action="start-flash" ${state.flashing ? "disabled" : ""}>
              ${state.flashing ? "Flashing Bootable USB..." : "Confirm & Flash Bootable USB"}
            </button>
          `}
        </div>
      `;
    }


    function renderSidebarSummaryCard() {
      const slot = document.getElementById("sidebarSummaryCardSlot");
      if (!slot) return;
      const addr = getFullServerAddress();
      const dashPort = Number(state.dashboard_port) || 8088;
      const sshPort = Number(state.ssh_port) || 22;
      const sum = buildSummaryLabels();
      const passDisplay = !state.password ? "(not set)" : (state.sidebarRevealPass ? state.password : "•".repeat(Math.min(10, state.password.length)));
      const firstLan = (state.lanScanResults && state.lanScanResults[0]) || null;
      const ethIp = firstLan ? (firstLan.ip || "Auto DHCP") : (state.static_ip_cidr ? state.static_ip_cidr.split("/")[0] : "Auto DHCP (LAN)");
      const wifiIp = state.wifi_ssid ? `Failover (${state.wifi_ssid})` : "Not configured";
      const isPro = state.ui_mode === "pro";

      slot.innerHTML = `
        <div class="sidebar-summary-card ${isPro ? "pro-expanded" : ""}">
          <div class="ssc-header">
            <div class="ssc-title-wrap">
              <span class="ssc-dot"></span>
              <span class="ssc-name" title="${esc(state.server_name || state.hostname)}">${esc(state.server_name || state.hostname || "Instant Server")}</span>
            </div>
            <button type="button" class="ssc-pen-btn" data-action="open-quick-edit-modal" title="Quick edit server details" aria-label="Quick edit server details">
              <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 20h9"/><path d="M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4L16.5 3.5z"/></svg>
            </button>
          </div>
          <div class="ssc-addr-row">
            <a class="ssc-addr-link" href="http://${esc(addr)}" target="_blank" rel="noopener" title="Open http://${esc(addr)}">http://${esc(addr)}</a>
            <button type="button" class="ssc-copy-btn" data-action="copy-text" data-copy="http://${esc(addr)}" title="Copy address">Copy</button>
          </div>
          <div class="ssc-kv"><span>User:</span><strong>${esc(state.username || "admin")}</strong></div>
          <div class="ssc-kv">
            <span>Pass:</span>
            <span style="display:inline-flex;align-items:center;gap:4px;">
              <strong>${esc(passDisplay)}</strong>
              ${state.password ? `<button type="button" class="ssc-mini-btn" data-action="toggle-sidebar-pass">${state.sidebarRevealPass ? "Hide" : "Show"}</button>` : ""}
            </span>
          </div>
          <div class="ssc-brief">${esc(sum.modeLabel)} • ${state.selected_apps.length} apps</div>
          ${isPro ? `
            <div class="ssc-pro-block">
              <div class="ssc-kv"><span>Eth IP:</span><code>${esc(ethIp)}</code></div>
              <div class="ssc-kv"><span>Wi-Fi:</span><code>${esc(wifiIp)}</code></div>
              <div class="ssc-kv"><span>Dash:</span><a class="ssc-mini-link" href="http://${esc(addr)}:${dashPort}" target="_blank" rel="noopener">:${dashPort}</a></div>
              <div class="ssc-kv"><span>SSH:</span><button type="button" class="ssc-mini-btn" data-action="copy-text" data-copy="ssh ${esc(state.username || "admin")}@${esc(addr)} -p ${sshPort}">ssh ${esc(state.username || "admin")}@…</button></div>
            </div>
          ` : ""}
        </div>
      `;
    }

    function renderSettingsView() {
      const navEl = document.getElementById("settingsNavList");
      const mainEl = document.getElementById("settingsMainContent");

      renderSidebarSummaryCard();
      navEl.innerHTML = SETTINGS_TABS.map(item => {
        const active = state.settingsTab === item.id && !state.searchQuery.trim();
        return `
          <button type="button" class="nav-item ${active ? "active" : ""}" data-action="switch-settings-tab" data-tab="${item.id}">
            <span>${esc(item.label)}</span>
          </button>
        `;
      }).join("");

      if (state.searchQuery.trim()) {
        mainEl.innerHTML = renderSearchResults(state.searchQuery.trim());
        return;
      }

      const tabIdx = SETTINGS_TABS.findIndex(t => t.id === state.settingsTab);
      const currentMeta = SETTINGS_TABS[tabIdx] || SETTINGS_TABS[0];
      const isFirst = tabIdx <= 0;
      const isLast = tabIdx === SETTINGS_TABS.length - 1;

      const animClass = shouldAnimateTab ? "animate-tab-switch" : "";
      shouldAnimateTab = false;

      const bottomPagerHtml = `
        <div class="bottom-nav-row">
          <button type="button" class="win-btn" data-action="settings-prev" ${isFirst ? "disabled" : ""}>Previous</button>
          <div style="display:flex;align-items:center;gap:10px;">
            <button type="button" class="win-btn" data-action="save-config-home">Save configuration</button>
            ${isLast ? `
              <button type="button" class="win-btn win-btn-accent win-btn-large" data-action="start-flash" ${state.flashing ? "disabled" : ""}>
                ${state.flashing ? "Flashing Bootable USB..." : "Flash Bootable USB"}
              </button>
            ` : `
              <button type="button" class="win-btn win-btn-accent" data-action="settings-next">Next</button>
            `}
          </div>
        </div>
      `;

      if (state.settingsTab === "system") {
        const sum = buildSummaryLabels();
        const selectedTitles = APP_STORE_CATALOG.filter(a => state.selected_apps.includes(a.id)).map(a => a.title);

        mainEl.innerHTML = `
          <div class="${animClass} tab-flow-stack">
            <div class="page-header">
              <div class="breadcrumb-bar"><span>System</span></div>
            </div>

            <div class="system-summary-banner">
              <div class="summary-left">
                <div class="summary-title">${esc(state.hostname || "instant-server")}</div>
                <div class="summary-subtitle">Ubuntu Server ${esc(state.ubuntu_release)} LTS • Admin: ${esc(state.username || "admin")}</div>
              </div>
              <div class="summary-pills">
                <div class="summary-pill">Network: <strong>${esc(sum.wifiLabel)}</strong></div>
                <div class="summary-pill">Apps: <strong>${esc(sum.dashLabel)}</strong></div>
                <div class="summary-pill">Mode: <strong>${esc(sum.modeLabel)}</strong></div>
                <div class="summary-pill">USB: <strong>${esc(sum.usbLabel)}</strong></div>
              </div>
            </div>

            <div class="card-stack">
              <div class="win-card clickable" data-action="switch-settings-tab" data-tab="network">
                <div class="card-left">
                  <div class="card-text">
                    <div class="card-title">Network &amp; multi-server</div>
                    <div class="card-desc">Server name (${esc(state.hostname || "instant-server")}), admin login, Wi-Fi (${esc(sum.wifiLabel)}), combine multiple computers</div>
                  </div>
                </div>
                <div class="card-right"><span>›</span></div>
              </div>

              <div class="win-card clickable" data-action="switch-settings-tab" data-tab="apps">
                <div class="card-left">
                  <div class="card-text">
                    <div class="card-title">Apps &amp; store (${selectedTitles.length} selected)</div>
                    <div class="card-desc">${esc(selectedTitles.join(", ") || "No extra apps selected")}</div>
                  </div>
                </div>
                <div class="card-right"><span>›</span></div>
              </div>

              <div class="win-card clickable" data-action="switch-settings-tab" data-tab="storage">
                <div class="card-left">
                  <div class="card-text">
                    <div class="card-title">Laptop, PC &amp; USB flash</div>
                    <div class="card-desc">${esc(sum.modeLabel)} • Target USB: ${esc(sum.usbLabel)} • Boot scheme: ${esc(state.usb_partition_scheme.toUpperCase())}</div>
                  </div>
                </div>
                <div class="card-right"><span>›</span></div>
              </div>

              <div class="win-card clickable" data-action="switch-settings-tab" data-tab="power">
                <div class="card-left">
                  <div class="card-text">
                    <div class="card-title">Power &amp; battery</div>
                    <div class="card-desc">Lid-close sleep ignore (${state.ignore_lid_switch ? "On" : "Off"}), battery charge cap (${state.battery_charge_stop_pct}%), outage auto-shutdown (${state.ups_shutdown_battery_pct}%)</div>
                  </div>
                </div>
                <div class="card-right"><span>›</span></div>
              </div>
            </div>

            <div class="center-flash-row" style="flex-direction:row;flex-wrap:wrap;gap:10px;">
              <button type="button" class="win-btn win-btn-accent win-btn-large" data-action="start-flash" ${state.flashing ? "disabled" : ""}>
                ${state.flashing ? "Flashing Bootable USB..." : "Flash Bootable USB"}
              </button>
              <button type="button" class="win-btn win-btn-large" data-action="reset-usb-drive" ${state.resettingUsb || state.flashing ? "disabled" : ""}>
                ${state.resettingUsb ? "Resetting USB..." : "Reset USB to Normal Drive"}
              </button>
              <span class="footer-cancel-flash-wrap" style="display:${state.flashing ? "inline-flex" : "none"};gap:8px;">
                <button type="button" class="win-btn win-btn-large" data-action="pause-resume-flash">${state.flashPaused ? `Resume (${state.flashProgress}%)` : "Pause"}</button>
                <button type="button" class="win-btn win-btn-danger win-btn-large" data-action="cancel-flash">Cancel</button>
              </span>
            </div>

            ${renderCacheRetentionCardHtml()}
            ${renderFlashProgressAndLogsCard()}
            ${bottomPagerHtml}
          </div>
        `;
        return;
      }

      let bodyHtml = "";
      if (state.settingsTab === "network") bodyHtml = buildNetworkSection();
      else if (state.settingsTab === "apps") bodyHtml = buildAppsSection();
      else if (state.settingsTab === "storage") bodyHtml = buildStorageSection();
      else if (state.settingsTab === "power") bodyHtml = buildPowerSection();
      else if (state.settingsTab === "config") {
        bodyHtml = `
          <div class="tab-flow-stack">
            <div class="card-stack">
              <div class="win-card">
                <div class="card-left">
                  <div class="card-text">
                    <div class="card-title">Local configuration &amp; workspace folder</div>
                    <div class="card-desc">Automatically saved in ${esc(state.savedConfigPath)}</div>
                  </div>
                </div>
                <div class="card-right">
                  <button type="button" class="win-btn" data-action="save-config-home">Save configuration</button>
                  <button type="button" class="win-btn" data-action="copy-yaml">Copy YAML</button>
                </div>
              </div>

              ${renderCacheRetentionCardHtml()}

              <div class="win-card">
                <div class="card-left">
                  <div class="card-text">
                    <div class="card-title">Find Instant Server on your network</div>
                    <div class="card-desc">Scan your home network for running Instant Server computers</div>
                  </div>
                </div>
                <div class="card-right">
                  <button type="button" class="win-btn" data-action="scan-lan">${state.lanScanning ? "Scanning..." : "Scan network"}</button>
                </div>
              </div>
            </div>

            ${state.lanScanResults.length > 0 ? `
              <div class="card-stack">
                ${state.lanScanResults.map(s => `
                  <div class="win-card">
                    <div class="card-left">
                      <div class="card-text">
                        <div class="card-title">${esc(s.ip)}</div>
                        <div class="card-desc">Open ports: ${esc((s.open_ports || []).join(", "))}</div>
                      </div>
                    </div>
                  </div>
                `).join("")}
              </div>
            ` : ""}

            <div class="section-label">Common questions &amp; quick fixes</div>
            <div class="card-stack">
              ${QUICK_HELP_ITEMS.map(item => `
                <div class="win-card">
                  <div class="card-left">
                    <div class="card-text">
                      <div class="card-title">${esc(item.title)}</div>
                      <div class="card-desc">${esc(item.desc)}</div>
                    </div>
                  </div>
                  <div class="card-right">
                    <button type="button" class="win-btn win-btn-sm" data-action="${esc(item.action)}" ${item.tab ? `data-tab="${esc(item.tab)}"` : ""}>${esc(item.btn)}</button>
                  </div>
                </div>
              `).join("")}
            </div>

          </div>
        `;
      }

      mainEl.innerHTML = `
        <div class="${animClass} tab-flow-stack">
          <div class="page-header">
            <div class="breadcrumb-bar">
              <span class="breadcrumb-link" data-action="switch-settings-tab" data-tab="system">System</span>
              <span class="breadcrumb-sep">›</span>
              <span>${esc(currentMeta.label)}</span>
            </div>
          </div>
          ${bodyHtml}
          ${renderFlashProgressAndLogsCard()}
          ${bottomPagerHtml}
        </div>
      `;
      const scArea = document.getElementById("settingsScrollArea");
      if (scArea) {
        scArea._restoringScroll = true;
        scArea.scrollTop = (state.tabScrollPositions && state.tabScrollPositions.settings && state.tabScrollPositions.settings[state.settingsTab]) || 0;
        scArea._restoringScroll = false;
      }
    }

    function renderSearchResults(query) {
      const q = query.toLowerCase();
      const items = [
        { tab: "network", title: "Server name & admin login", desc: "Change hostname, admin username, and password", keys: "hostname password username login" },
        { tab: "network", title: "Combine multiple computers into one server", desc: "Use the same USB on multiple laptops/PCs or merge storage without losing files", keys: "combine multiple servers cluster mergerfs pool" },
        { tab: "network", title: "Wi-Fi & USB tethering", desc: "Configure wireless SSID, password, and phone USB tethering", keys: "wifi wireless ssid tethering static ip ssh" },
        { tab: "apps", title: "Self-Hosted App Store & CasaOS", desc: "Pre-select Jellyfin, Nextcloud, Pi-hole, Immich, Home Assistant, Vaultwarden, and more", keys: "cockpit casaos docker jellyfin nextcloud pihole immich homeassistant vaultwarden plex syncthing qbittorrent portainer" },
        { tab: "storage", title: "Computer type, Repair USB & Bootable Flash", desc: "Configure Laptop/Desktop/Mini-PC mode, Repair USB mode, and Dual-Boot size", keys: "dual boot erase disk drive storage partition usb flash iso repair rescue desktop mini pc" },
        { tab: "power", title: "Lid close & battery limiter", desc: "Keep running with lid closed and cap battery charge at 60%", keys: "power lid battery ups zram screen blank" },
        { tab: "config", title: "Saved configuration, LAN scanner & quick fixes", desc: "Save instant-server-config.json, scan LAN, or fix common issues", keys: "yaml config save lan discover help fix" }
      ].filter(it => (it.title + " " + it.desc + " " + it.keys).toLowerCase().includes(q));

      return `
        <div class="tab-flow-stack">
          <div class="page-header">
            <div class="breadcrumb-bar"><span>Search results for "${esc(query)}"</span></div>
            <button type="button" class="win-btn" data-action="clear-search">Clear</button>
          </div>
          <div class="card-stack">
            ${items.length ? items.map(it => `
              <div class="win-card clickable" data-action="switch-settings-tab" data-tab="${it.tab}">
                <div class="card-left">
                  <div class="card-text">
                    <div class="card-title">${esc(it.title)}</div>
                    <div class="card-desc">${esc(it.desc)}</div>
                  </div>
                </div>
                <div class="card-right"><span>Open ›</span></div>
              </div>
            `).join("") : `<div class="win-card"><div class="card-title">No matching settings found.</div></div>`}
          </div>
        </div>
      `;
    }

    function renderApp() {
      syncFlashingLockState();
      document.documentElement.setAttribute("data-theme", state.theme);
      const stdBtn = document.getElementById("btnUiModeStandard");
      const proBtn = document.getElementById("btnUiModePro");
      if (stdBtn && proBtn) {
        stdBtn.className = "mode-seg-btn " + (state.ui_mode === "standard" ? "active-standard" : "");
        proBtn.className = "mode-seg-btn " + (state.ui_mode === "pro" ? "active-pro" : "");
      }
      const themeIcon = document.getElementById("themeToggleIcon");
      if (themeIcon) {
        themeIcon.innerHTML = state.theme === "dark"
          ? `<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="5"/><line x1="12" y1="1" x2="12" y2="3"/><line x1="12" y1="21" x2="12" y2="23"/><line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/><line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/><line x1="1" y1="12" x2="3" y2="12"/><line x1="21" y1="12" x2="23" y2="12"/><line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/><line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/></svg>`
          : `<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/></svg>`;
      }

      const modeBtn = document.getElementById("btnToggleModeView");
      if (modeBtn) {
        modeBtn.textContent = state.viewMode === "setup" ? "Home Settings" : "Setup Wizard";
      }

      const searchWrap = document.getElementById("titleSearchWrap");
      const setupView = document.getElementById("setupShellView");
      const settingsView = document.getElementById("settingsShellView");

      if (state.viewMode === "setup") {
        searchWrap.classList.add("hidden-in-setup");
        setupView.style.display = "flex";
        settingsView.style.display = "none";
        renderSetupView();
      } else {
        searchWrap.classList.remove("hidden-in-setup");
        setupView.style.display = "none";
        settingsView.style.display = "flex";
        renderSettingsView();
      }
    }

    function updateSwitchInPlace(switchEl, isOn) {
      if (!switchEl) return;
      switchEl.classList.toggle("on", isOn);
      switchEl.setAttribute("aria-checked", String(isOn));
      const lbl = switchEl.querySelector(".win-switch-label");
      if (lbl) lbl.textContent = isOn ? "On" : "Off";
    }

    function syncSelectAllCheckboxInPlace() {
      const countLbl = document.getElementById("lblSelectedAppsCount");
      if (countLbl) countLbl.textContent = String(state.selected_apps.length);
      const mcCard = document.getElementById("minecraftCustomizationStudioCard");
      if (mcCard) {
        mcCard.style.display = (state.selected_apps.includes("minecraft") || state.selected_apps.includes("minecraft_bedrock")) ? "flex" : "none";
      }
      const chk = document.getElementById("chkSelectAllApps");
      const txt = document.getElementById("lblSelectAllAppsText");
      const filtered = getFilteredAppsList();
      const targetList = filtered.length > 0 ? filtered : APP_STORE_CATALOG;
      const selectedInTarget = targetList.filter(a => state.selected_apps.includes(a.id)).length;
      const allSelected = targetList.length > 0 && selectedInTarget === targetList.length;
      if (chk) {
        chk.checked = allSelected;
        chk.indeterminate = selectedInTarget > 0 && !allSelected;
      }
      if (txt) {
        txt.textContent = allSelected
          ? (state.appCategoryFilter === "All" ? `Unselect all (${state.selected_apps.length})` : `Unselect ${state.appCategoryFilter} (${selectedInTarget})`)
          : (state.appCategoryFilter === "All" ? `Select all (${state.selected_apps.length}/${APP_STORE_CATALOG.length})` : `Select all in filter (${selectedInTarget}/${targetList.length})`);
      }
    }

    function refreshAppGridInPlace() {
      const grid = document.getElementById("appStoreGridContainer");
      if (grid) grid.innerHTML = renderFilteredAppCardsHtml();
      const pillsWrap = document.getElementById("appCategoryPills");
      if (pillsWrap) {
        pillsWrap.querySelectorAll(".filter-pill").forEach(btn => {
          btn.classList.toggle("active", btn.getAttribute("data-category") === state.appCategoryFilter);
        });
      }
      syncSelectAllCheckboxInPlace();
    }

    function attachSleekScrollListeners() {
      document.addEventListener("scroll", (e) => {
        const el = e.target;
        if (!el || el._restoringScroll) return;
        if (el.id === "setupBodyContent") {
          state.tabScrollPositions.setup[state.setupTab] = el.scrollTop;
        } else if (el.id === "settingsScrollArea") {
          state.tabScrollPositions.settings[state.settingsTab] = el.scrollTop;
        }
        if (el && el.id === "flashLogBox") {
          if (el._programmaticScroll) return;
          const distFromBottom = el.scrollHeight - el.scrollTop - el.clientHeight;
          const atBottom = distFromBottom <= 24;
          if (state.logAutoScroll !== atBottom) {
            state.logAutoScroll = atBottom;
            const resumeBtn = document.getElementById("btnResumeAutoScroll");
            if (resumeBtn) {
              resumeBtn.style.display = state.logAutoScroll ? "none" : "inline-flex";
            }
          }
        }
      }, true);


    }