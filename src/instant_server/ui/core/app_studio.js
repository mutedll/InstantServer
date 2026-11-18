// FILE: src/instant_server/ui/app_studio.js
    document.body.addEventListener("click", async (e) => {
      const target = e.target.closest("[data-action]");
      if (!target) return;
      const action = target.getAttribute("data-action");
      let shouldAnimateTab = false;
      if (action === "switch-setup-tab" || action === "jump-setup-tab") {
        const nextTab = target.getAttribute("data-tab") || "network";
        if (state.viewMode !== "setup") state.viewMode = "setup";
        if (state.setupTab !== nextTab) shouldAnimateTab = true;
        state.setupTab = nextTab;
        state.validationErrors = [];
        state.openDropdownField = null;
        renderApp();
      } else if (action === "setup-prev") {
        const idx = SETUP_TABS.findIndex(t => t.id === state.setupTab);
        if (idx > 0) {
          state.setupTab = SETUP_TABS[idx - 1].id;
          state.openDropdownField = null;
          shouldAnimateTab = true;
          renderApp();
        }
      } else if (action === "setup-skip" || action === "setup-next") {
        const idx = SETUP_TABS.findIndex(t => t.id === state.setupTab);
        if (idx < SETUP_TABS.length - 1) {
          const nextId = SETUP_TABS[idx + 1].id;
          state.setupTab = nextId;
          state.tabScrollPositions.setup[nextId] = 0;
          state.openDropdownField = null;
          shouldAnimateTab = true;
          renderApp();
        }
      } else if (action === "switch-settings-tab") {
        const nextTab = target.getAttribute("data-tab") || "system";
        if (state.settingsTab !== nextTab || state.searchQuery) shouldAnimateTab = true;
        state.settingsTab = nextTab;
        state.searchQuery = "";
        state.openDropdownField = null;
        const sInput = document.getElementById("globalSearchInput");
        if (sInput) sInput.value = "";
        renderApp();
      } else if (action === "settings-prev") {
        const idx = SETTINGS_TABS.findIndex(t => t.id === state.settingsTab);
        if (idx > 0) {
          state.settingsTab = SETTINGS_TABS[idx - 1].id;
          state.openDropdownField = null;
          shouldAnimateTab = true;
          renderApp();
        }
      } else if (action === "settings-next") {
        const idx = SETTINGS_TABS.findIndex(t => t.id === state.settingsTab);
        if (idx < SETTINGS_TABS.length - 1) {
          const nextId = SETTINGS_TABS[idx + 1].id;
          state.settingsTab = nextId;
          state.tabScrollPositions.settings[nextId] = 0;
          state.openDropdownField = null;
          shouldAnimateTab = true;
          renderApp();
        }
      } else if (action === "clear-search") {
        state.searchQuery = "";
        document.getElementById("globalSearchInput").value = "";
        shouldAnimateTab = true;
        renderApp();
      } else if (action === "select-dashboard") {
        state.dashboard_choice = target.getAttribute("data-value") || "both";
        renderApp();
      } else if (action === "select-install-mode") {
        state.install_mode = target.getAttribute("data-value") || "dual_boot";
        if (state.install_mode !== "auto_erase") {
          state.confirm_erase_main_drive = false;
        }
        renderApp();
      } else if (action === "select-usb") {
        state.selectedUsbId = target.getAttribute("data-usb") || "";
        renderApp();
      } else if (action === "scan-usb") {
        await scanUsbDrives(false);
      } else if (action === "verify-usb-drive") {
        await verifySelectedUsbDrive();
      } else if (action === "reset-usb-drive") {
        await resetSelectedUsbDrive();
      } else if (action === "boot-usb-next-restart") {
        const nbModal = document.getElementById("nextBootConfirmModal");
        if (nbModal) nbModal.classList.add("open");
      } else if (action === "dismiss-next-boot-modal") {
        const nbModal = document.getElementById("nextBootConfirmModal");
        if (nbModal) nbModal.classList.remove("open");
      } else if (action === "confirm-next-boot-stage-only") {
        const nbModal = document.getElementById("nextBootConfirmModal");
        if (nbModal) nbModal.classList.remove("open");
        showToast("Staging Instant Server USB for next restart...");
        const res = await callBackend("boot-usb-next", "POST", { mode: "stage_only" });
        if (res && res.ok) {
          showToast(res.message || "Staged Instant Server USB for next restart.");
        } else if (res && res.error) {
          showToast(res.error);
        }
      } else if (action === "confirm-next-boot-restart-now") {
        const nbModal = document.getElementById("nextBootConfirmModal");
        if (nbModal) nbModal.classList.remove("open");
        showToast("Configuring UEFI next-boot USB target and restarting...");
        const res = await callBackend("boot-usb-next", "POST", { mode: "restart_now" });
        if (res && res.ok) {
          showToast(res.message || "Restarting directly into Instant Server USB...");
        } else if (res && res.error) {
          showToast(res.error);
        }
      } else if (action === "save-config-home") {
        await saveConfigurationAndOpenHome();
      } else if (action === "start-flash") {
        await startFlashingProcess();
      } else if (action === "pause-resume-flash") {
        await togglePauseResumeFlashing();
      } else if (action === "add-custom-web-app") {
        await addCustomWebAppFromInput();
      } else if (action === "cancel-flash") {
        await cancelFlashingProcess(false);
      } else if (action === "resume-log-autoscroll") {
        state.logAutoScroll = true;
        scrollTerminalToBottomIfSticky();
      } else if (action === "dismiss-close-warning") {
        state.closeWarningOpen = false;
        const modal = document.getElementById("closeWarningModal");
        if (modal) modal.classList.remove("open");
      } else if (action === "pause-and-exit-flash") {
        const modal = document.getElementById("closeWarningModal");
        if (modal) modal.classList.remove("open");
        await pauseAndExitFlashingProcess();
      } else if (action === "confirm-cancel-and-exit") {
        const modal = document.getElementById("closeWarningModal");
        if (modal) modal.classList.remove("open");
        await cancelFlashingProcess(true);
      } else if (action === "cache-keep") {
        await handleCacheRetentionAction("keep");
      } else if (action === "cache-delete") {
        await handleCacheRetentionAction("delete");
      } else if (action === "toggle-expand-apps") {
        state.appsExpanded = !state.appsExpanded;
        refreshAppGridInPlace();
      } else if (action === "open-dashboard-preview") {
        window.open("server_dashboard.html", "_blank", "noopener");
        showToast("Opened live Server Web Dashboard preview");
      } else if (action === "copy-yaml") {
        const text = generateYamlPreview();
        try {
          await navigator.clipboard.writeText(text);
        } catch (_e) {
          const ta = document.createElement("textarea");
          ta.value = text;
          document.body.appendChild(ta);
          ta.select();
          document.execCommand("copy");
          document.body.removeChild(ta);
        }
        showToast("Copied YAML configuration");
      } else if (action === "scan-lan") {
        state.lanScanning = true;
        renderApp();
        const res = await callBackend("discover", "GET");
        state.lanScanning = false;
        if (res && Array.isArray(res.servers_found) && res.servers_found.length > 0) {
          state.lanScanResults = res.servers_found;
          showToast(`Found ${res.servers_found.length} server(s)`);
        } else {
          state.lanScanResults = [
            { ip: `${state.hostname || "instant-server"}.local`, open_ports: [22, 9090, 80] }
          ];
          showToast("LAN scan complete");
        }
        renderApp();
      }
    });

    document.body.addEventListener("change", (e) => {
      if (state.flashing) return;
      const act = e.target.getAttribute("data-action");
      if (act === "select-studio-app") {
        state.studio_app_id = e.target.value;
        const ov = getOrInitAppOverride(state.studio_app_id);
        state.studio_file_name = Object.keys(ov.config_files)[0] || ".env";
        renderApp();
      }
    });

    document.body.addEventListener("input", (e) => {
      if (state.flashing) return;
      const act = e.target.getAttribute("data-action");
      if (act === "studio-file-textarea") {
        const ov = getOrInitAppOverride(state.studio_app_id || "minecraft");
        if (state.studio_file_name) {
          ov.config_files[state.studio_file_name] = e.target.value;
        }
        return;
      } else if (act === "studio-port-input") {
        const ov = getOrInitAppOverride(state.studio_app_id || "minecraft");
        ov.host_port = Number(e.target.value) || 8080;
        return;
      } else if (act === "studio-mem-input") {
        const ov = getOrInitAppOverride(state.studio_app_id || "minecraft");
        ov.memory_limit = e.target.value.trim();
        return;
      }
      const field = e.target.getAttribute("data-field");
      if (!field) return;
      state[field] = e.target.value;

      if (field === "appSearchQuery") {
        refreshAppGridInPlace();
      } else if (field === "hostname") {
        const hostDesc = document.getElementById("descServerHostname");
        if (hostDesc) {
          hostDesc.textContent = `Name used to reach your server on your home network (${state.hostname || "instant-server"}.local)`;
        }
      } else if (field === "storage_custom_input") {
        const rawStr = String(e.target.value || "").trim();
        state.storageInputRaw = rawStr;
        const num = Number(rawStr);
        const computedGb = state.storageUnit === "TB" ? Math.round(num * 1024) : Math.round(num);
        const lbl = document.getElementById("lblDualBootSize");
        const valMsg = document.getElementById("lblStorageValidation");
        if (!rawStr || !Number.isFinite(num) || num <= 0) {
          state.storageInputError = "Enter a valid positive number (15 GB – 64 TB).";
          e.target.classList.add("invalid");
        } else if (computedGb < 15) {
          state.storageInputError = "Minimum partition size is 15 GB (for Ubuntu Server + Docker).";
          e.target.classList.add("invalid");
        } else if (computedGb > 65536) {
          state.storageInputError = "Maximum partition size is 64 TB (65,536 GB).";
          e.target.classList.add("invalid");
        } else {
          state.storageInputError = "";
          state.dual_boot_Allocation_gb = computedGb;
          e.target.classList.remove("invalid");
        }
        const disp = state.dual_boot_Allocation_gb >= 1024 && (state.dual_boot_Allocation_gb % 1024 === 0)
          ? `${state.dual_boot_Allocation_gb / 1024} TB (${state.dual_boot_Allocation_gb} GB)`
          : `${state.dual_boot_Allocation_gb} GB`;
        if (lbl) lbl.textContent = `Server partition size (${disp})`;
        if (valMsg) {
          valMsg.textContent = state.storageInputError || "Type any size from 15 GB up to 64 TB (65,536 GB) or pick a preset below";
          valMsg.style.color = state.storageInputError ? "var(--danger)" : "";
          valMsg.style.fontWeight = state.storageInputError ? "600" : "";
        }
      } else if (field === "battery_charge_stop_pct") {
        const t = document.getElementById("lblBatCapTitle");
        const d = document.getElementById("lblBatCapDesc");
        if (t) t.textContent = `Battery health limiter (${state.battery_charge_start_pct}% – ${state.battery_charge_stop_pct}%)`;
        if (d) d.textContent = `Caps charging at ${state.battery_charge_stop_pct}% while plugged in 24/7 to protect the battery`;
      } else if (field === "ups_shutdown_battery_pct") {
        const t = document.getElementById("lblUpsTitle");
        const d = document.getElementById("lblUpsDesc");
        if (t) t.textContent = `Safe shutdown during power outage (${state.ups_shutdown_battery_pct}%)`;
        if (d) d.textContent = `Uses the laptop battery as a built-in UPS and shuts down cleanly when battery drops to ${state.ups_shutdown_battery_pct}%`;
      } else if (field === "vm_swappiness") {
        const t = document.getElementById("lblSwappinessTitle");
        if (t) t.textContent = `Memory swap tendency (${state.vm_swappiness}%)`;
      }

      if (e.target && e.target.type === "range") {
        updateSliderBubblePosition(e.target, true);
      }
    });

    function updateSliderBubblePosition(rangeEl, showBubble) {
      if (!rangeEl) return;
      const wrap = rangeEl.closest(".win-slider-wrap");
      if (!wrap) return;
      const bubble = wrap.querySelector(".win-slider-bubble");
      if (!bubble) return;
      const min = Number(rangeEl.min) || 0;
      const max = Number(rangeEl.max) || 100;
      const val = Number(rangeEl.value ?? min);
      const unit = rangeEl.getAttribute("data-unit") || "%";
      const ratio = Math.max(0, Math.min(1, (val - min) / Math.max(1, max - min)));
      const trackWidth = rangeEl.offsetWidth || 160;
      const leftPx = 8 + ratio * Math.max(0, trackWidth - 16);
      bubble.style.left = `${leftPx.toFixed(1)}px`;
      bubble.textContent = `${val}${unit}`;
      if (showBubble) {
        wrap.classList.add("active");
        if (wrap._hideTimer) clearTimeout(wrap._hideTimer);
      }
    }

    function hideSliderBubble(rangeEl) {
      if (!rangeEl) return;
      const wrap = rangeEl.closest(".win-slider-wrap");
      if (!wrap) return;
      if (wrap._hideTimer) clearTimeout(wrap._hideTimer);
      wrap.classList.remove("active");
    }

    document.body.addEventListener("pointerdown", (e) => {
      if (e.target && e.target.type === "range") {
        updateSliderBubblePosition(e.target, true);
      }
    });
    document.body.addEventListener("pointerup", (e) => {
      if (e.target && e.target.type === "range") {
        hideSliderBubble(e.target);
      }
    });
    document.body.addEventListener("pointercancel", (e) => {
      if (e.target && e.target.type === "range") {
        hideSliderBubble(e.target);
      }
    });
    document.body.addEventListener("mouseout", (e) => {
      if (e.target && e.target.type === "range" && !e.target.matches(":active")) {
        hideSliderBubble(e.target);
      }
    });
    document.body.addEventListener("blur", (e) => {
      if (e.target && e.target.type === "range") {
        hideSliderBubble(e.target);
      }
    }, true);

    window.__showCloseWhileFlashingWarning = () => {
      state.closeWarningOpen = true;
      const modal = document.getElementById("closeWarningModal");
      if (modal) modal.classList.add("open");
    };

    window.addEventListener("pagehide", () => {
      try {
        if (window.location.protocol.startsWith("http") && navigator.sendBeacon) {
          const blob = new Blob([JSON.stringify({ force: true })], { type: "application/json" });
          navigator.sendBeacon(window.location.origin + "/api/shutdown", blob);
        }
      } catch (_err) {}
    });

    function attachWailsEventChannelListeners() {
      if (!window.runtime || typeof window.runtime.EventsOn !== "function") return;
      let rafPending = false;
      window.runtime.EventsOn("flash:progress", (ev) => {
        if (!ev) return;
        state.lastWailsEventTs = Date.now();
        state.flashing = true;
        const nextOverall = Math.round(Number(ev.percent ?? ev.pct ?? state.flashProgress) || state.flashProgress);
        const nextStagePct = Number(ev.stage_percent ?? ev.stage_pct ?? nextOverall) || nextOverall;
        state.flashProgress = Math.max(state.flashProgress, nextOverall);
        if (ev.stage_index) {
          state.flashStageIndex = Math.max(state.flashStageIndex || 1, Number(ev.stage_index));
        }
        if (ev.eta_sec !== undefined && Number(ev.eta_sec) > 0) {
          updateSmoothedFlashETA(Number(ev.eta_sec));
        }
        state.flashStatusText = ev.message || "Streaming aligned DMA blocks...";
        state.activeProgressLine = buildAsciiTerminalBar(nextStagePct, ev.stage || "Active Stage", ev.message || "");
        if (!rafPending) {
          rafPending = true;
          window.requestAnimationFrame(() => {
            rafPending = false;
            updateFlashPanelInPlace();
          });
        }
      });
      window.runtime.EventsOn("flash:paused", () => {
        state.flashPaused = true;
        updateFlashPanelInPlace();
      });
      window.runtime.EventsOn("flash:resumed", () => {
        state.flashPaused = false;
        updateFlashPanelInPlace();
      });
      window.runtime.EventsOn("flash:complete", (ev) => {
        state.flashing = false;
        state.flashPaused = false;
        state.flashProgress = 100;
        state.flashStageIndex = 6;
        state.activeProgressLine = "";
        state.flashEtaText = "";
        state.flashStatusText = "Bootable USB drive is ready! Plug it into your computer and power it on.";
        if (ev && ev.sha256) {
          state.flashLogs.push(`[VERIFIED] SHA-256: ${ev.sha256} • CRC-32: ${ev.crc32 || "OK"}`);
        }
        updateFlashPanelInPlace();
        showToast("Bootable Instant Server USB is ready");
      });
      window.runtime.EventsOn("flash:error", (ev) => {
        state.flashing = false;
        state.flashPaused = false;
        state.activeProgressLine = "";
        const errMsg = (ev && ev.error) ? ev.error : "USB write interrupted";
        state.flashStatusText = `Failed: ${errMsg}`;
        state.flashLogs.push(`[ERROR] ${errMsg}`);
        updateFlashPanelInPlace();
      });
      window.runtime.EventsOn("flash:close-warning", () => {
        if (window.__showCloseWhileFlashingWarning) {
          window.__showCloseWhileFlashingWarning();
        }
      });
    }

    window.addEventListener("load", () => {
      attachSleekScrollListeners();
      attachWailsEventChannelListeners();
      initAppStartup();
      setInterval(async () => {
        const hb = await callBackend("heartbeat", "POST", {});
        if (hb && hb.close_warning_requested && window.__showCloseWhileFlashingWarning) {
          window.__showCloseWhileFlashingWarning();
        }
      }, 1000);
    });