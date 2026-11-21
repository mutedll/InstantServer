// FILE: src/instant_server/ui/view_network.js
    function renderSwitch(action, isOn) {
      return `
        <div class="win-switch ${isOn ? "on" : ""}" data-action="${esc(action)}" role="switch" aria-checked="${isOn}">
          <span class="win-switch-label">${isOn ? "On" : "Off"}</span>
          <div class="win-switch-track"><div class="win-switch-thumb"></div></div>
        </div>
      `;
    }

    function renderSlider(field, min, max, step, val, unit = "%", widthPx = 160) {
      const numMin = Number(min) || 0;
      const numMax = Number(max) || 100;
      const numVal = Number(val ?? numMin);
      const ratio = Math.max(0, Math.min(1, (numVal - numMin) / Math.max(1, numMax - numMin)));
      const thumbOffsetPx = 8 + ratio * (widthPx - 16);
      return `
        <div class="win-slider-wrap" style="width:${widthPx}px;">
          <div class="win-slider-bubble" style="left:${thumbOffsetPx.toFixed(1)}px;">${esc(String(numVal))}${esc(unit)}</div>
          <input
            type="range"
            min="${min}"
            max="${max}"
            step="${step}"
            value="${numVal}"
            data-field="${esc(field)}"
            data-unit="${esc(unit)}"
            style="accent-color:var(--accent);width:${widthPx}px;"
          />
        </div>
      `;
    }

    function renderComboBox(field, options) {
      const currentVal = String(state[field] ?? "");
      const currentOpt = options.find(o => String(o.value) === currentVal) || options[0];
      const isOpen = state.openDropdownField === field;
      return `
        <div class="win-combobox ${isOpen ? "open" : ""}" data-combobox="${esc(field)}">
          <button type="button" class="win-combobox-trigger" data-action="toggle-dropdown" data-field="${esc(field)}">
            <span class="win-combobox-label">${esc(currentOpt ? currentOpt.label : "")}</span>
            <span class="win-combobox-chevron">⌄</span>
          </button>
          <div class="win-combobox-menu" role="listbox">
            ${options.map(opt => {
              const isSel = String(opt.value) === currentVal;
              return `
                <button
                  type="button"
                  class="win-combobox-option ${isSel ? "selected" : ""}"
                  data-action="select-dropdown-option"
                  data-field="${esc(field)}"
                  data-value="${esc(opt.value)}"
                  data-label="${esc(opt.label)}"
                  role="option"
                  aria-selected="${isSel}"
                >
                  <span>${esc(opt.label)}</span>
                </button>
              `;
            }).join("")}
          </div>
        </div>
      `;
    }

    function renderExpander(tabKey, title, innerHtml) {
      const isOpen = state.ui_mode === "pro" || !!state.advancedExpanded[tabKey];
      return `
        <div class="expander-wrap ${isOpen ? "open" : ""}">
          <button type="button" class="expander-header" data-action="toggle-expander" data-expander="${esc(tabKey)}">
            <span>${esc(title)}</span>
            <span class="expander-chevron">⌄</span>
          </button>
          <div class="expander-content-grid">
            <div class="expander-content-inner">
              <div class="expander-body">
                ${innerHtml}
              </div>
            </div>
          </div>
        </div>
      `;
    }

    function renderFlashProgressAndLogsCard() {
      const hasActivity = state.flashing || state.flashProgress > 0 || state.flashLogs.length > 0;
      const missingHtml = state.validationErrors.length > 0 ? `
        <div class="notice-banner">
          <strong>Complete the following before flashing:</strong>
          <ul style="padding-left: 18px; margin-top: 2px;">
            ${state.validationErrors.map(e => `<li>${esc(e)}</li>`).join("")}
          </ul>
        </div>
      ` : "";

      if (!hasActivity) {
        return missingHtml;
      }

      const badgeClass = state.flashProgress === 100
        ? "badge-success"
        : state.flashCancelled
        ? "badge-warning"
        : "badge-accent";

      return `
        ${missingHtml}
        ${(!state.flashing && state.flashProgress === 100 && !state.flashCancelled) ? `
        <div class="flash-complete-highlight" id="flashCompleteBanner">
          <div class="flash-complete-icon">✓</div>
          <div style="flex:1;min-width:0;">
            <div style="font-weight:700;font-size:14px;color:var(--success);">Bootable USB Flash Complete!</div>
            <div style="font-size:12.5px;color:var(--text-secondary);margin-top:2px;">
              Your Instant Server USB is verified and ready. Unplug it, boot your target computer from USB, and after installation completes unplug the USB and allow 2–5 minutes on first boot for Docker &amp; apps to finish starting.
            </div>
          </div>
          <button type="button" class="win-btn win-btn-sm win-btn-accent" data-action="boot-usb-next-restart">Boot USB on Next Restart</button>
        </div>` : ""}
        <div class="flash-panel ${(!state.flashing && state.flashProgress === 100 && !state.flashCancelled) ? "flash-panel-complete" : ""}" id="flashPanelCard">
          <div style="display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap;">
            <div style="display:flex;align-items:center;gap:10px;min-width:0;flex:1;">
              <span id="flashStatusTitle" style="font-weight:600;font-size:13.5px;">${esc(state.flashStatusText || "Preparing bootable USB...")}</span>
              <span id="flashBadgePct" class="badge ${badgeClass}">${state.flashProgress}%</span>
              <span id="flashEtaBadge" class="badge badge-accent" style="${state.flashEtaText ? "" : "display:none;"}">${esc(state.flashEtaText)}</span>
              <span id="flashConfigLockedBadge" class="badge badge-warning" style="${state.flashing ? "" : "display:none;"}">Config locked</span>
            </div>
            <div id="flashPanelHeaderActions" style="display:flex;align-items:center;gap:8px;">
              ${state.flashing ? `
                <button type="button" class="win-btn win-btn-sm ${state.flashPaused ? "win-btn-accent" : ""}" data-action="pause-resume-flash">
                  ${state.flashPaused ? `Resume (${state.flashProgress}%)` : "Pause"}
                </button>
                <button type="button" class="win-btn win-btn-sm win-btn-danger" data-action="cancel-flash">
                  Cancel
                </button>
              ` : ""}
            </div>
          </div>
          <div id="flashStageStepsRow" style="display:flex;align-items:center;gap:6px;flex-wrap:wrap;margin:2px 0;">
            ${renderFlashPipelineStagesHtml()}
          </div>
          <div class="progress-track">
            <div class="progress-fill" id="flashProgressFill" style="width: ${state.flashProgress}%;"></div>
          </div>
          <div class="flash-terminal-wrap">
            <div class="flash-log-box" id="flashLogBox" tabindex="0">
              <div id="flashLogLines">${state.flashLogs.map(l => `<div class="term-log-line">${esc(l)}</div>`).join("")}</div>
              <div id="flashTerminalLiveRow" class="term-live-progress" style="${state.activeProgressLine ? "" : "display:none;"}">${esc(state.activeProgressLine || "")}</div>
            </div>
            <button
              type="button"
              id="btnResumeAutoScroll"
              class="resume-scroll-pill"
              data-action="resume-log-autoscroll"
              style="${state.logAutoScroll ? "display:none;" : "display:inline-flex;"}"
            >
              ↓ Auto-scroll paused — Jump to bottom
            </button>
          </div>
          <details style="margin-top:6px;border-top:1px solid var(--border-subtle);padding-top:8px;">
            <summary style="cursor:pointer;font-size:12px;font-weight:600;color:var(--text-secondary);">
              Config and files overview
            </summary>
            <div class="code-viewer" style="margin-top:8px;max-height:170px;overflow:auto;">${esc(generateYamlPreview())}</div>
          </details>
        </div>
      `;
    }

    function buildSummaryLabels() {
      const modeNames = {
        dual_boot: `Dual-Boot (${state.dual_boot_Allocation_gb} GB)`,
        auto_erase: "Erase Main Drive",
        whole_disk: "Erase Main Drive",
        interactive_select: "Select Disk on Screen",
        interactive_gui_disk: "Disk Manager & Selector",
        secondary_drive: "Secondary Drive Only",
        reinstall_keep_data: "Reinstall (Keep Data)",
        reinstall_wipe_data: "Factory Reinstall (Wipe)",
        offline_updater_usb: "Offline Updater USB",
        repair_rescue_usb: "Repair & Rescue USB"
      };
      const dashNames = {
        both: "Cockpit + CasaOS",
        cockpit: "Cockpit",
        casaos: "CasaOS",
        none: "SSH Only"
      };
      const usb = state.usbDevices.find(d => d.device_id === state.selectedUsbId && d.triple_lock_passed);
      return {
        modeLabel: modeNames[state.install_mode] || "Dual-Boot",
        dashLabel: `${dashNames[state.dashboard_choice] || "Cockpit + CasaOS"} (${state.selected_apps.length} apps)`,
        usbLabel: usb ? `${usb.model} (${usb.size_gb} GB)` : "No USB selected",
        wifiLabel: state.wifi_ssid.trim() ? state.wifi_ssid.trim() : "Ethernet / USB Tethering"
      };
    }

    function generateYamlPreview() {
      const sizingLine = state.storage_layout === "lvm" ? "\n      sizing-policy: all" : "";
      const storageSnippet = state.install_mode === "dual_boot"
        ? `  # Dual-Boot Mode (${state.dual_boot_Allocation_gb} GB): dynamic_dualboot_storage.sh shrinks existing OS safely\n  storage:\n    layout:\n      name: ${state.storage_layout}${sizingLine}\n      match:\n        size: largest`
        : state.install_mode === "interactive_select"
        ? `  interactive-sections:\n    - storage`
        : state.install_mode === "repair_rescue_usb"
        ? `  # Live Repair & Password Reset Mode: repairs bootloader/filesystem without formatting target drive`
        : state.install_mode === "offline_updater_usb"
        ? `  # Offline Updater USB Mode: updates dashboard, scripts, and incremental apps without formatting`
        : `  storage:\n    layout:\n      name: ${state.storage_layout}${sizingLine}\n      match:\n        size: largest`;

      return `#cloud-config
autoinstall:
  version: 1
  identity:
    hostname: ${state.hostname || "instant-server"}
    username: ${state.username || "admin"}
    password: "${state.password ? "$6$rounds=5000$hashed..." : "<set-password>"}"
  ssh:
    install-server: true
    allow-pw: true
${storageSnippet}
  # Pre-unpacked CasaOS apps on USB: ${state.selected_apps.join(", ") || "none"}
  late-commands:
    - curtin in-target -- bash /cdrom/cidata/laptop_hardening.sh`;
    }

    function buildNetworkSection() {
      const basicHtml = `
        <div class="card-stack">
          <div class="win-card">
            <div class="card-left">
              <div class="card-text">
                <div class="card-title">Server display name</div>
                <div class="card-desc">Friendly name shown in the dashboard and boot screen (can differ from the server address)</div>
              </div>
            </div>
            <div class="card-right">
              <input class="win-input" data-field="server_name" value="${esc(state.server_name || state.hostname)}" placeholder="My Home Server" />
            </div>
          </div>

          <div class="win-card" style="flex-direction:column;align-items:stretch;gap:10px;">
            <div style="display:flex;align-items:center;justify-content:space-between;gap:14px;flex-wrap:wrap;">
              <div class="card-left">
                <div class="card-text">
                  <div class="card-title">Full server address (Host &amp; Domain)</div>
                  <div class="card-desc" id="descServerHostname">Reach your dashboard at <strong>http://${esc(getFullServerAddress())}</strong></div>
                </div>
              </div>
              <div class="card-right" style="flex-wrap:wrap;gap:6px;">
                <input class="win-input" style="min-width:130px;width:140px;" data-field="server_address_host" value="${esc(state.server_address_host || state.hostname)}" placeholder="myhome" />
                <input class="win-input" style="min-width:105px;width:115px;" data-field="server_address_domain" value="${esc(state.server_address_domain || ".local")}" placeholder=".local" />
              </div>
            </div>
            <div style="display:flex;align-items:center;gap:6px;flex-wrap:wrap;">
              <span style="font-size:11.5px;color:var(--text-secondary);">Safe local domains:</span>
              ${[".local", ".home.arpa", ".lan", ".internal"].map(d => `<button type="button" class="win-btn win-btn-sm ${normalizeServerDomain(state.server_address_domain) === d ? "win-btn-accent" : ""}" data-action="set-domain-preset" data-domain="${d}">${d}</button>`).join("")}
            </div>
            <div id="domainSecurityWarnBox" class="notice-banner" style="display:${getDomainSecurityWarning(state.server_address_domain) ? "block" : "none"};margin:0;font-size:12px;">
              ${esc(getDomainSecurityWarning(state.server_address_domain))}
            </div>
          </div>

          <div class="win-card">
            <div class="card-left">
              <div class="card-text">
                <div class="card-title">Admin username &amp; password</div>
                <div class="card-desc">Login credentials for the web dashboard and SSH</div>
              </div>
            </div>
            <div class="card-right" style="flex-wrap:wrap;">
              <input class="win-input" style="min-width:140px;" data-field="username" value="${esc(state.username)}" placeholder="admin" />
              <input class="win-input" style="min-width:170px;" type="password" data-field="password" value="${esc(state.password)}" placeholder="Enter password" />
            </div>
          </div>

          <div class="win-card" style="flex-direction:column;align-items:stretch;gap:10px;">
            <div style="display:flex;align-items:center;justify-content:space-between;gap:14px;flex-wrap:wrap;">
              <div class="card-left">
                <div class="card-text">
                  <div class="card-title">
                    <span>Wi-Fi network &amp; backup failover</span>
                    <span class="badge badge-warning">Ethernet Recommended for Media &amp; Backups</span>
                  </div>
                  <div class="card-desc">Leave blank if using Ethernet only, or enter Wi-Fi as an automatic failover backup</div>
                </div>
              </div>
              <div class="card-right" style="flex-wrap:wrap;">
                <input class="win-input" style="min-width:155px;" data-field="wifi_ssid" value="${esc(state.wifi_ssid)}" placeholder="Wi-Fi name (SSID)" />
                <input class="win-input" style="min-width:155px;" type="password" data-field="wifi_password" value="${esc(state.wifi_password)}" placeholder="Wi-Fi password" />
              </div>
            </div>
            <div style="font-size:12px;color:var(--text-secondary);padding-top:8px;border-top:1px solid var(--divider);line-height:1.45;">
              <strong>Tip:</strong> Plug in an Ethernet cable when possible for faster file transfers and streaming; Wi-Fi remains active as automatic backup failover.
            </div>
          </div>

          <div class="win-card" style="flex-direction:column;align-items:stretch;gap:10px;">
            <div style="display:flex;align-items:center;justify-content:space-between;gap:14px;flex-wrap:wrap;">
              <div class="card-left">
                <div class="card-text">
                  <div class="card-title">
                    <span>Remote access outside home (Tailscale VPN)</span>
                    <span class="badge badge-accent">Zero Port-Forwarding</span>
                  </div>
                  <div class="card-desc">Reach your server, dashboard, and self-hosted apps securely from anywhere in the world</div>
                </div>
              </div>
              <div class="card-right">
                ${renderSwitch("toggle-tailscale", state.enable_tailscale)}
              </div>
            </div>
            <div id="tailscaleNetConfigRow" style="display:${state.enable_tailscale ? "flex" : "none"};align-items:center;justify-content:space-between;gap:10px;flex-wrap:wrap;padding-top:8px;border-top:1px solid var(--divider);">
              <input
                id="tailscaleAuthInputNet"
                class="win-input"
                style="flex:1;min-width:220px;"
                data-field="tailscale_authkey"
                value="${esc(state.tailscale_authkey)}"
                placeholder="Pre-auth key: tskey-auth-... (optional — or sign in later in dashboard)"
              />
              ${renderComboBox("tailscale_mode", [
                { value: "subnet_and_ssh", label: "Home LAN Subnet Router + Tailscale SSH (Recommended)" },
                { value: "apps_and_ssh", label: "Server Apps & Tailscale SSH Only" },
                { value: "exit_node_full", label: "Subnet Router + Advertise as Exit Node" }
              ])}
            </div>
          </div>
        </div>
      `;

      const clusterStorageHtml = state.cluster_mode !== "standalone" ? `
        <div class="win-card">
          <div class="card-left">
            <div class="card-text">
              <div class="card-title">Existing files on the second computer</div>
              <div class="card-desc">Choose whether to keep all existing files on both computers or start with empty space on the second one</div>
            </div>
          </div>
          <div class="card-right">
            ${renderComboBox("cluster_storage_policy", [
              { value: "mergerfs_union_safe", label: "Keep all files on both computers (Combined storage)" },
              { value: "replicated_ha_mirror", label: "Keep all files & mirror backups across both" },
              { value: "wipe_secondary_expand", label: "Erase second computer for empty space" }
            ])}
          </div>
        </div>
        ${state.cluster_storage_policy === "wipe_secondary_expand" ? `
          <div class="notice-banner" id="bannerWipeSecondary">
            <div style="font-weight:600;">Warning: All existing files on the second computer will be erased when it joins</div>
            <label style="display:flex;align-items:center;gap:8px;cursor:pointer;margin-top:4px;">
              <input type="checkbox" data-action="toggle-confirm-wipe-secondary" ${state.confirm_wipe_secondary ? "checked" : ""} style="accent-color:var(--danger);width:16px;height:16px;" />
              <span>I understand — erase the second computer's files when combining servers</span>
            </label>
          </div>
        ` : ""}
      ` : "";

      const advHtml = `
        <div class="win-card">
          <div class="card-left">
            <div class="card-text">
              <div class="card-title">Combine multiple computers into one server</div>
              <div class="card-desc">Use this same USB stick on multiple laptops/PCs to link them automatically, or connect another server later in the web dashboard</div>
            </div>
          </div>
          <div class="card-right">
            ${renderComboBox("cluster_mode", [
              { value: "standalone", label: "Single computer (Can still link another server later)" },
              { value: "usb_multi_node_auto", label: "Use this same USB on multiple computers (Auto-connect)" },
              { value: "cluster_worker", label: "Add this computer to an existing Instant Server" }
            ])}
          </div>
        </div>
        ${clusterStorageHtml}

        <div class="win-card">
          <div class="card-left">
            <div class="card-text">
              <div class="card-title">Phone USB tethering</div>
              <div class="card-desc">Automatically use internet from a plugged-in Android or iPhone if Wi-Fi is unavailable</div>
            </div>
          </div>
          <div class="card-right">
            ${renderSwitch("toggle-tethering", state.enable_usb_tethering)}
          </div>
        </div>

        <div class="win-card">
          <div class="card-left">
            <div class="card-text">
              <div class="card-title">Seamless Ethernet-to-Wi-Fi failover</div>
              <div class="card-desc">Keeps Wi-Fi awake while Ethernet is plugged in so unplugging the cable never drops your connection</div>
            </div>
          </div>
          <div class="card-right">
            ${renderSwitch("toggle-wifi-powersave", state.disable_wifi_power_save)}
          </div>
        </div>

        <div class="win-card">
          <div class="card-left">
            <div class="card-text">
              <div class="card-title">Static IP address</div>
              <div class="card-desc">Optional fixed IP (leave empty for automatic DHCP)</div>
            </div>
          </div>
          <div class="card-right" style="flex-wrap:wrap;">
            <input class="win-input" style="min-width:145px;" data-field="static_ip_cidr" value="${esc(state.static_ip_cidr)}" placeholder="IP / CIDR" />
            <input class="win-input" style="min-width:135px;" data-field="static_gateway" value="${esc(state.static_gateway)}" placeholder="Gateway" />
          </div>
        </div>

        <div class="win-card">
          <div class="card-left">
            <div class="card-text">
              <div class="card-title">SSH public key</div>
              <div class="card-desc">Optional key for passwordless SSH login</div>
            </div>
          </div>
          <div class="card-right">
            <input class="win-input" data-field="ssh_authorized_key" value="${esc(state.ssh_authorized_key)}" placeholder="ssh-ed25519 AAAA..." />
          </div>
        </div>

        ${state.ui_mode === "pro" ? `
          <div class="win-card">
            <div class="card-left">
              <div class="card-text">
                <div class="card-title">TCP BBR fast network streaming</div>
                <div class="card-desc">Improves local network and VPN speed for Jellyfin video streaming and Nextcloud file transfers</div>
              </div>
            </div>
            <div class="card-right">${renderSwitch("toggle-tcp-bbr", state.enable_tcp_bbr)}</div>
          </div>

          <div class="win-card">
            <div class="card-left">
              <div class="card-text">
                <div class="card-title">Firewall &amp; SSH login protection</div>
                <div class="card-desc">Blocks repeated failed SSH login attempts and protects open ports</div>
              </div>
            </div>
            <div class="card-right" style="gap:10px;">
              <input class="win-input" style="width:90px;min-width:90px;" data-field="ssh_port" value="${esc(state.ssh_port)}" placeholder="Port 22" title="Custom SSH Port" />
              ${renderSwitch("toggle-fail2ban", state.enable_fail2ban)}
            </div>
          </div>
        ` : ""}
      `;

      return `
        <div class="tab-flow-stack">
          ${basicHtml}
          ${renderExpander("network", "Advanced options", advHtml)}
        </div>
      `;
    }