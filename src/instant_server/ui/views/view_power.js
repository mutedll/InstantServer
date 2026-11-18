// FILE: src/instant_server/ui/view_power.js
    function buildPowerSection() {
      const basicHtml = `
        <div class="card-stack">
          <div class="win-card">
            <div class="card-left">
              <div class="card-text">
                <div class="card-title">Keep running with the laptop lid closed</div>
                <div class="card-desc">Prevents the laptop from going to sleep when you close the screen (automatically skipped on desktop PCs)</div>
              </div>
            </div>
            <div class="card-right">${renderSwitch("toggle-lid", state.ignore_lid_switch)}</div>
          </div>

          <div class="notice-banner notice-banner-warning" id="bannerThermalOverheatTrap" style="${state.ignore_lid_switch ? "" : "display:none;"}">
            <div style="display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap;">
              <div style="font-weight:600;">Keep laptop in an open, ventilated area</div>
              <button type="button" class="win-btn win-btn-sm ${state.disable_cpu_turbo_boost ? "win-btn-accent" : ""}" data-action="enable-cool-quiet-profile">
                ${state.disable_cpu_turbo_boost ? " Cool & Quiet Mode ON" : "Enable Cool & Quiet Mode"}
              </button>
            </div>
            <div style="font-size:12px;line-height:1.45;color:var(--text-primary);">
              Avoid placing a closed laptop inside enclosed drawers or cabinets. Keep airflow unobstructed or prop the lid slightly ajar.
            </div>
          </div>

          <div class="win-card">
            <div class="card-left">
              <div class="card-text">
                <div class="card-title" id="lblBatCapTitle">Battery health limiter (${state.battery_charge_start_pct}% – ${state.battery_charge_stop_pct}%)</div>
                <div class="card-desc" id="lblBatCapDesc">Caps charging at ${state.battery_charge_stop_pct}% while plugged in 24/7 to protect the battery</div>
              </div>
            </div>
            <div class="card-right">
              ${renderSlider("battery_charge_stop_pct", 50, 85, 5, state.battery_charge_stop_pct, "%", 160)}
            </div>
          </div>

          <div class="win-card">
            <div class="card-left">
              <div class="card-text">
                <div class="card-title" id="lblUpsTitle">Safe shutdown during power outage (${state.ups_shutdown_battery_pct}%)</div>
                <div class="card-desc" id="lblUpsDesc">Uses the laptop battery as a built-in UPS and shuts down cleanly when battery drops to ${state.ups_shutdown_battery_pct}%</div>
              </div>
            </div>
            <div class="card-right">
              ${renderSlider("ups_shutdown_battery_pct", 10, 35, 5, state.ups_shutdown_battery_pct, "%", 160)}
            </div>
          </div>
        </div>
      `;

      const advHtml = `
        <div class="win-card">
          <div class="card-left">
            <div class="card-text">
              <div class="card-title" id="lblScreenBlankTitle">Turn off screen after ${esc(state.screen_blank_seconds)} seconds</div>
              <div class="card-desc">Saves power and prevents screen burn-in</div>
            </div>
          </div>
          <div class="card-right">
            ${renderComboBox("screen_blank_seconds", [
              { value: "30", label: "30 seconds" },
              { value: "60", label: "60 seconds (Recommended)" },
              { value: "180", label: "3 minutes" }
            ])}
          </div>
        </div>

        <div class="win-card">
          <div class="card-left">
            <div class="card-text">
              <div class="card-title">Low-RAM memory compression</div>
              <div class="card-desc">Enables zram swap so older 2 GB and 4 GB RAM computers run smoothly</div>
            </div>
          </div>
          <div class="card-right">${renderSwitch("toggle-zram", state.enable_low_ram_zram)}</div>
        </div>

        <div class="win-card">
          <div class="card-left">
            <div class="card-text">
              <div class="card-title">Safe graphics mode</div>
              <div class="card-desc">Use if the computer has an older NVIDIA or AMD graphics chip</div>
            </div>
          </div>
          <div class="card-right">${renderSwitch("toggle-nomodeset", state.safe_gpu_nomodeset)}</div>
        </div>

        ${state.ui_mode === "pro" ? `
          <div class="win-card">
            <div class="card-left">
              <div class="card-text">
                <div class="card-title">CPU power &amp; fan profile</div>
                <div class="card-desc">Select Quiet &amp; Low Power for silent fans, or Balanced for automatic speed boost</div>
              </div>
            </div>
            <div class="card-right">
              ${renderComboBox("cpu_governor", [
                { value: "schedutil", label: "Balanced (Default)" },
                { value: "powersave", label: "Quiet & Low Power" },
                { value: "performance", label: "Maximum Speed" }
              ])}
            </div>
          </div>

          <div class="win-card">
            <div class="card-left">
              <div class="card-text">
                <div class="card-title">Limit peak CPU heat (Disable Turbo Boost)</div>
                <div class="card-desc">Lowers peak CPU temperatures and fan noise on older computers</div>
              </div>
            </div>
            <div class="card-right">${renderSwitch("toggle-disable-turbo", state.disable_cpu_turbo_boost)}</div>
          </div>

          <div class="win-card">
            <div class="card-left">
              <div class="card-text">
                <div class="card-title" id="lblSwappinessTitle">Memory swap tendency (${state.vm_swappiness}%)</div>
                <div class="card-desc">Keeps apps in physical RAM longer and reduces drive wear</div>
              </div>
            </div>
            <div class="card-right">
              ${renderSlider("vm_swappiness", 1, 60, 1, state.vm_swappiness, "%", 140)}
            </div>
          </div>
        ` : ""}
      `;

      return `
        <div class="tab-flow-stack">
          ${basicHtml}
          ${renderExpander("power", "Advanced options", advHtml)}
        </div>
      `;
    }

    function renderCacheRetentionCardHtml() {
      return `
        <div class="win-card" style="flex-direction:column;align-items:stretch;gap:10px;border-color:var(--success);">
          <div style="display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap;">
            <div class="card-left">
              <div class="card-text">
                <div class="card-title">
                  <span>Cached ISO &amp; App Files</span>
                  <span class="badge badge-success">Kept by Default</span>
                </div>
                <div class="card-desc">Keep downloaded ISO and app files in <code>Instant_Server_Data</code> for fast reflashing, or delete them to free disk space.</div>
              </div>
            </div>
            <div class="card-right" style="gap:8px;flex-wrap:wrap;">
              <button type="button" class="win-btn win-btn-sm ${state.cacheRetentionMode !== "delete" ? "win-btn-accent" : ""}" data-action="cache-keep">
                Keep Cached Files
              </button>
              <button type="button" class="win-btn win-btn-sm ${state.cacheRetentionMode === "delete" ? "win-btn-danger" : ""}" data-action="cache-delete">
                Delete Cached Files
              </button>
            </div>
          </div>
          <div style="font-size:12px;color:var(--text-secondary);">
            Status: <strong>${esc(state.cacheStatusMessage)}</strong>
          </div>
        </div>
      `;
    }

    function buildFinishSection() {
      const sum = buildSummaryLabels();
      const selectedTitles = APP_STORE_CATALOG.filter(a => state.selected_apps.includes(a.id)).map(a => a.title);
      return `
        <div class="tab-flow-stack">
          <div class="card-stack">
            <div class="win-card">
              <div class="card-left">
                <div class="card-text">
                  <div class="card-title">${esc(state.hostname || "instant-server")}.local</div>
                  <div class="card-desc">Ubuntu Server ${esc(state.ubuntu_release)} LTS • Admin user: ${esc(state.username || "admin")}</div>
                </div>
              </div>
              <div class="card-right">
                <span class="badge ${state.password ? "badge-success" : "badge-warning"}">
                  ${state.password ? "Password set" : "Password empty"}
                </span>
              </div>
            </div>

            <div class="win-card">
              <div class="card-left">
                <div class="card-text">
                  <div class="card-title">Network</div>
                  <div class="card-desc">${esc(sum.wifiLabel)}${state.cluster_mode !== "standalone" ? " • Multi-computer cluster enabled" : ""}</div>
                </div>
              </div>
              <div class="card-right">
                <button type="button" class="win-btn" data-action="jump-setup-tab" data-tab="network">Edit</button>
              </div>
            </div>

            <div class="win-card">
              <div class="card-left">
                <div class="card-text">
                  <div class="card-title">Dashboards &amp; ${selectedTitles.length} pre-selected apps</div>
                  <div class="card-desc">${esc(selectedTitles.join(", ") || "None selected")}</div>
                </div>
              </div>
              <div class="card-right">
                <button type="button" class="win-btn" data-action="jump-setup-tab" data-tab="apps">Edit</button>
              </div>
            </div>

            <div class="win-card">
              <div class="card-left">
                <div class="card-text">
                  <div class="card-title">Bootable USB &amp; target installation mode</div>
                  <div class="card-desc">${esc(sum.modeLabel)} • ${esc(sum.usbLabel)} • Scheme: ${esc(state.usb_partition_scheme.toUpperCase())}</div>
                </div>
              </div>
              <div class="card-right" style="gap:8px;">
                <button type="button" class="win-btn" data-action="reset-usb-drive" ${state.resettingUsb || state.flashing ? "disabled" : ""}>
                  ${state.resettingUsb ? "Resetting USB..." : "Reset USB"}
                </button>
                <button type="button" class="win-btn" data-action="jump-setup-tab" data-tab="storage">Edit</button>
              </div>
            </div>
          </div>

          ${renderCacheRetentionCardHtml()}

          <div class="win-card" style="flex-direction:column;align-items:stretch;gap:8px;">
            <div class="card-left">
              <div class="card-text">
                <div class="card-title">Next steps after flashing</div>
                <div class="card-desc">
                  <strong>1. Boot from USB:</strong> Plug the USB into your target computer, power it on, and select the USB drive from the boot menu (F12 / F9 / F8 / F11).<br/>
                  <strong>2. Automatic setup:</strong> Ubuntu Server and your selected apps install automatically.<br/>
                  <strong>3. Open Web Dashboard:</strong> Once complete, the screen displays your Web Dashboard URL (<code>http://${esc(state.hostname || "instant-server")}.local:${state.dashboard_port}</code>).
                </div>
              </div>
            </div>
          </div>

          ${renderFlashProgressAndLogsCard()}
        </div>
      `;
    }