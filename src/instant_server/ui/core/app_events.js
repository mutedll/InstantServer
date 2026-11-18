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
        "copy-yaml"
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
        if (field === "cluster_mode" || field === "cluster_storage_policy") {
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

      if (action === "set-ui-mode") {
        const nextMode = target.getAttribute("data-mode") || "standard";
        state.ui_mode = nextMode;
        if (nextMode === "pro") {
          state.advancedExpanded = { network: true, apps: true, storage: true, power: true };
          showToast("Pro Mode enabled — Additional system, network & power options unlocked");
        } else {
          showToast("Standard Mode enabled — Guided setup with smart defaults");
        }
        renderApp();
        return;
      } else if (action === "elevate-admin") {
        await callBackend("save-config", "POST", serializeProfilePayload());
        const res = await callBackend("elevate", "POST", {});
        if (res && res.already_elevated) {
          state.hostIsAdmin = true;
          const adminBanner = document.getElementById("adminPrivilegeBanner");
          if (adminBanner) adminBanner.style.display = "none";
          showToast("Already running with Administrator privileges");
        } else if (res && res.elevated_spawned) {
          state.forceClosing = true;
          showToast("Administrator window opened — closing this window...");
          setTimeout(() => { window.close(); }, 180);
        } else {
          showToast((res && res.message) ? res.message : "Right-click InstantServer-Windows.exe and choose 'Run as administrator'");
        }
        return;
      } else if (action === "apply-role-preset") {
        const presetId = target.getAttribute("data-preset") || "";
        const preset = SERVER_ROLE_PRESETS.find(p => p.id === presetId);
        if (preset) {
          state.preset_role = preset.id;
          state.selected_apps = [...preset.apps];
          if (preset.vaapi !== undefined) state.enable_intel_vaapi = preset.vaapi;
          if (preset.samba !== undefined) state.enable_samba_nas = preset.samba;
          if (preset.pihole_dns !== undefined) state.enable_pihole_dns_prep = preset.pihole_dns;
          if (preset.zram !== undefined) state.enable_low_ram_zram = preset.zram;
          if (preset.governor) state.cpu_governor = preset.governor;
          if (preset.zims && preset.zims.length) state.preselect_kiwix_zim = preset.zims[0];
          showToast(`Applied '${preset.title}' preset (${preset.apps.length} apps selected)`);
          renderApp();
        }
        return;
      }

      if (action === "set-storage-unit") {
        const nextUnit = target.getAttribute("data-unit") || "GB";
        if (nextUnit !== state.storageUnit) {
          state.storageUnit = nextUnit;
          if (nextUnit === "TB") {
            const tbVal = +(state.dual_boot_Allocation_gb / 1024).toFixed(2);
            state.storageInputRaw = String(tbVal || 1);
          } else {
            state.storageInputRaw = String(state.dual_boot_Allocation_gb);
          }
          state.storageInputError = "";
          renderApp();
        }
        return;
      } else if (action === "set-storage-preset") {
        const gb = Number(target.getAttribute("data-gb")) || 250;
        state.dual_boot_Allocation_gb = gb;
        if (gb >= 1024 && gb % 1024 === 0) {
          state.storageUnit = "TB";
          state.storageInputRaw = String(gb / 1024);
        } else {
          state.storageUnit = "GB";
          state.storageInputRaw = String(gb);
        }
        state.storageInputError = "";
        renderApp();
        return;
      } else if (action === "toggle-wifi-powersave") {
        state.disable_wifi_power_save = !state.disable_wifi_power_save;
        updateSwitchInPlace(target, state.disable_wifi_power_save);
        return;
      } else if (action === "toggle-tethering") {
        state.enable_usb_tethering = !state.enable_usb_tethering;
        updateSwitchInPlace(target, state.enable_usb_tethering);
        return;
      } else if (action === "toggle-offline-images") {
        state.prebundle_offline_images = !state.prebundle_offline_images;
        updateSwitchInPlace(target, state.prebundle_offline_images);
        return;
      } else if (action === "toggle-pihole") {
        state.enable_pihole_dns_prep = !state.enable_pihole_dns_prep;
        updateSwitchInPlace(target, state.enable_pihole_dns_prep);
        return;
      } else if (action === "toggle-samba") {
        state.enable_samba_nas = !state.enable_samba_nas;
        updateSwitchInPlace(target, state.enable_samba_nas);
        return;
      } else if (action === "toggle-vaapi") {
        state.enable_intel_vaapi = !state.enable_intel_vaapi;
        updateSwitchInPlace(target, state.enable_intel_vaapi);
        return;
      } else if (action === "toggle-tailscale") {
        state.enable_tailscale = !state.enable_tailscale;
        document.querySelectorAll("[data-action='toggle-tailscale']").forEach(sw => updateSwitchInPlace(sw, state.enable_tailscale));
        const authIn = document.getElementById("tailscaleAuthInput");
        if (authIn) authIn.style.display = state.enable_tailscale ? "" : "none";
        const netRow = document.getElementById("tailscaleNetConfigRow");
        if (netRow) netRow.style.display = state.enable_tailscale ? "flex" : "none";
        return;
      } else if (action === "toggle-lid") {
        state.ignore_lid_switch = !state.ignore_lid_switch;
        updateSwitchInPlace(target, state.ignore_lid_switch);
        const thBanner = document.getElementById("bannerThermalOverheatTrap");
        if (thBanner) thBanner.style.display = state.ignore_lid_switch ? "" : "none";
        return;
      } else if (action === "enable-cool-quiet-profile") {
        const nextCool = !state.disable_cpu_turbo_boost;
        state.disable_cpu_turbo_boost = nextCool;
        state.cpu_governor = nextCool ? "powersave" : "schedutil";
        showToast(nextCool
          ? "Enabled Cool & Quiet CPU profile (Turbo Boost off + Quiet fan profile)"
          : "Disabled Cool & Quiet profile (Restored Balanced schedutil CPU speed)");
        renderApp();
        return;
      } else if (action === "toggle-zram") {
        state.enable_low_ram_zram = !state.enable_low_ram_zram;
        updateSwitchInPlace(target, state.enable_low_ram_zram);
        return;
      } else if (action === "toggle-nomodeset") {
        state.safe_gpu_nomodeset = !state.safe_gpu_nomodeset;
        updateSwitchInPlace(target, state.safe_gpu_nomodeset);
        return;
      } else if (action === "toggle-auto-updates") {
        state.enable_auto_updates = !state.enable_auto_updates;
        updateSwitchInPlace(target, state.enable_auto_updates);
        return;
      } else if (action === "toggle-scheduled-backups") {
        state.enable_scheduled_backups = !state.enable_scheduled_backups;
        updateSwitchInPlace(target, state.enable_scheduled_backups);
        return;
      } else if (action === "toggle-prometheus") {
        state.enable_prometheus_metrics = !state.enable_prometheus_metrics;
        updateSwitchInPlace(target, state.enable_prometheus_metrics);
        return;
      } else if (action === "toggle-tcp-bbr") {
        state.enable_tcp_bbr = !state.enable_tcp_bbr;
        updateSwitchInPlace(target, state.enable_tcp_bbr);
        return;
      } else if (action === "toggle-fail2ban") {
        state.enable_fail2ban = !state.enable_fail2ban;
        updateSwitchInPlace(target, state.enable_fail2ban);
        return;
      } else if (action === "toggle-disable-turbo") {
        state.disable_cpu_turbo_boost = !state.disable_cpu_turbo_boost;
        updateSwitchInPlace(target, state.disable_cpu_turbo_boost);
        return;
      } else if (action === "toggle-confirm-erase") {
        state.confirm_erase_main_drive = !!target.checked;
        const banner = document.getElementById("bannerAutoErase");
        if (banner) banner.classList.toggle("notice-banner-warning", state.confirm_erase_main_drive);
        return;
      } else if (action === "toggle-confirm-wipe-secondary") {
        state.confirm_wipe_secondary = !!target.checked;
        return;
      } else if (action === "toggle-confirm-usb-erase") {
        state.confirm_usb_erase = !!target.checked;
        return;
      } else if (action === "toggle-confirm-wipe-server-data") {
        state.confirm_wipe_server_data = !!target.checked;
        return;
      } else if (action === "toggle-iphone-backup") {
        state.enable_iphone_auto_backup = !state.enable_iphone_auto_backup;
        if (state.enable_iphone_auto_backup && !state.selected_apps.includes("immich")) {
          state.selected_apps.push("immich");
        }
        renderApp();
        return;
      } else if (action === "load-workflow-template") {
        const tpl = target.getAttribute("data-tpl") || "samba_share";
        if (tpl === "samba_share") {
          state.custom_config_path = "/etc/samba/smb-custom.conf";
          state.custom_config_body = "[CustomShare]\n   comment = Custom Network Share\n   path = /DATA/CustomShare\n   browseable = yes\n   read only = no\n   guest ok = no\n   vfs objects = catia fruit streams_xattr\n";
        } else if (tpl === "usb_udev") {
          state.custom_config_path = "/etc/udev/rules.d/99-custom-usb-trigger.rules";
          state.custom_config_body = "# Automatically run a custom script whenever a specific USB device is plugged in\nACTION==\"add\", SUBSYSTEM==\"usb\", RUN+=\"/DATA/CustomWorkflows/on-usb-plug.sh\"\n";
        } else if (tpl === "systemd_timer") {
          state.custom_config_path = "/etc/systemd/system/custom-automation.service";
          state.custom_config_body = "[Unit]\nDescription=Custom Server Background Automation\nAfter=network-online.target\n\n[Service]\nType=oneshot\nExecStart=/bin/bash /DATA/CustomWorkflows/custom-job.sh\n";
        }
        showToast("Loaded custom workflow template — edit as needed and click + Stage Custom File");
        renderApp();
        return;
      } else if (action === "save-custom-sys-config") {
        const p = (state.custom_config_path || "").trim();
        if (!p.startsWith("/")) {
          showToast("Enter an absolute target path starting with / (e.g. /etc/samba/smb-custom.conf)");
          return;
        }
        state.custom_config_files[p] = state.custom_config_body || "";
        showToast(`Staged custom config file ${p} for USB provisioning`);
        renderApp();
        return;
      } else if (action === "toggle-mc-online-mode") {
        state.mc_online_mode = !!target.checked;
        renderApp();
        return;
      } else if (action === "select-studio-file") {
        const fn = target.getAttribute("data-file");
        if (fn) {
          state.studio_file_name = fn;
          renderApp();
        }
        return;
      } else if (action === "add-studio-file") {
        const rawFn = (state.studio_new_file_name || "").trim().replace(/[^a-zA-Z0-9._-]/g, "");
        if (!rawFn) {
          showToast("Enter a file name first (e.g. custom.conf or whitelist.json)");
          return;
        }
        const ov = getOrInitAppOverride(state.studio_app_id || "minecraft");
        if (!ov.config_files[rawFn]) {
          ov.config_files[rawFn] = `# /DATA/AppData/${state.studio_app_id || "minecraft"}/${rawFn}\n`;
        }
        state.studio_file_name = rawFn;
        state.studio_new_file_name = "";
        showToast(`Created ${rawFn} in /DATA/AppData/${state.studio_app_id || "minecraft"}`);
        renderApp();
        return;
      } else if (action === "toggle-mc-aikar") {
        state.mc_use_aikar_flags = !!target.checked;
        state.mc_enable_backups = !!target.checked;
        return;
      }

    });