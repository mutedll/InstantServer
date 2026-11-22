// FILE: src/instant_server/ui/core/app_events_actions.js
// PURPOSE: Action click handlers (UI mode, elevation, flashing, USB reset, custom apps, modals)
//          delegated from app_events.js to keep each UI core module concise (< 300 lines).
async function handleAppActionClick(action, target, e) {
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
        return true;
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
        return true;
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
        return true;
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
        return true;
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
        return true;
      } else if (action === "toggle-wifi-powersave") {
        state.disable_wifi_power_save = !state.disable_wifi_power_save;
        updateSwitchInPlace(target, state.disable_wifi_power_save);
        return true;
      } else if (action === "toggle-tethering") {
        state.enable_usb_tethering = !state.enable_usb_tethering;
        updateSwitchInPlace(target, state.enable_usb_tethering);
        return true;
      } else if (action === "toggle-offline-images") {
        state.prebundle_offline_images = !state.prebundle_offline_images;
        updateSwitchInPlace(target, state.prebundle_offline_images);
        return true;
      } else if (action === "toggle-pihole") {
        state.enable_pihole_dns_prep = !state.enable_pihole_dns_prep;
        updateSwitchInPlace(target, state.enable_pihole_dns_prep);
        return true;
      } else if (action === "toggle-samba") {
        state.enable_samba_nas = !state.enable_samba_nas;
        updateSwitchInPlace(target, state.enable_samba_nas);
        return true;
      } else if (action === "toggle-vaapi") {
        state.enable_intel_vaapi = !state.enable_intel_vaapi;
        updateSwitchInPlace(target, state.enable_intel_vaapi);
        return true;
      } else if (action === "toggle-tailscale") {
        state.enable_tailscale = !state.enable_tailscale;
        document.querySelectorAll("[data-action='toggle-tailscale']").forEach(sw => updateSwitchInPlace(sw, state.enable_tailscale));
        const authIn = document.getElementById("tailscaleAuthInput");
        if (authIn) authIn.style.display = state.enable_tailscale ? "" : "none";
        const netRow = document.getElementById("tailscaleNetConfigRow");
        if (netRow) netRow.style.display = state.enable_tailscale ? "flex" : "none";
        return true;
      } else if (action === "toggle-lid") {
        state.ignore_lid_switch = !state.ignore_lid_switch;
        updateSwitchInPlace(target, state.ignore_lid_switch);
        const thBanner = document.getElementById("bannerThermalOverheatTrap");
        if (thBanner) thBanner.style.display = state.ignore_lid_switch ? "" : "none";
        return true;
      } else if (action === "enable-cool-quiet-profile") {
        const nextCool = !state.disable_cpu_turbo_boost;
        state.disable_cpu_turbo_boost = nextCool;
        state.cpu_governor = nextCool ? "powersave" : "schedutil";
        showToast(nextCool
          ? "Enabled Cool & Quiet CPU profile (Turbo Boost off + Quiet fan profile)"
          : "Disabled Cool & Quiet profile (Restored Balanced schedutil CPU speed)");
        renderApp();
        return true;
      } else if (action === "toggle-zram") {
        state.enable_low_ram_zram = !state.enable_low_ram_zram;
        updateSwitchInPlace(target, state.enable_low_ram_zram);
        return true;
      } else if (action === "toggle-nomodeset") {
        state.safe_gpu_nomodeset = !state.safe_gpu_nomodeset;
        updateSwitchInPlace(target, state.safe_gpu_nomodeset);
        return true;
      } else if (action === "toggle-auto-updates") {
        state.enable_auto_updates = !state.enable_auto_updates;
        updateSwitchInPlace(target, state.enable_auto_updates);
        return true;
      } else if (action === "toggle-scheduled-backups") {
        state.enable_scheduled_backups = !state.enable_scheduled_backups;
        updateSwitchInPlace(target, state.enable_scheduled_backups);
        return true;
      } else if (action === "toggle-prometheus") {
        state.enable_prometheus_metrics = !state.enable_prometheus_metrics;
        updateSwitchInPlace(target, state.enable_prometheus_metrics);
        return true;
      } else if (action === "toggle-tcp-bbr") {
        state.enable_tcp_bbr = !state.enable_tcp_bbr;
        updateSwitchInPlace(target, state.enable_tcp_bbr);
        return true;
      } else if (action === "toggle-fail2ban") {
        state.enable_fail2ban = !state.enable_fail2ban;
        updateSwitchInPlace(target, state.enable_fail2ban);
        return true;
      } else if (action === "toggle-disable-turbo") {
        state.disable_cpu_turbo_boost = !state.disable_cpu_turbo_boost;
        updateSwitchInPlace(target, state.disable_cpu_turbo_boost);
        return true;
      } else if (action === "toggle-confirm-erase") {
        state.confirm_erase_main_drive = !!target.checked;
        const banner = document.getElementById("bannerAutoErase");
        if (banner) banner.classList.toggle("notice-banner-warning", state.confirm_erase_main_drive);
        return true;
      } else if (action === "toggle-confirm-wipe-secondary") {
        state.confirm_wipe_secondary = !!target.checked;
        return true;
      } else if (action === "toggle-confirm-usb-erase") {
        state.confirm_usb_erase = !!target.checked;
        return true;
      } else if (action === "toggle-skip-beginner-checks") {
        state.skip_beginner_checks = !!target.checked;
        if (state.skip_beginner_checks) {
          state.confirm_erase_main_drive = true;
          state.confirm_wipe_server_data = true;
          state.confirm_wipe_secondary = true;
          state.confirm_usb_erase = true;
        }
        renderApp();
        return true;
      } else if (action === "toggle-confirm-wipe-server-data") {
        state.confirm_wipe_server_data = !!target.checked;
        return true;
      } else if (action === "toggle-iphone-backup") {
        state.enable_iphone_auto_backup = !state.enable_iphone_auto_backup;
        if (state.enable_iphone_auto_backup && !state.selected_apps.includes("immich")) {
          state.selected_apps.push("immich");
        }
        renderApp();
        return true;
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
        return true;
      } else if (action === "save-custom-sys-config") {
        const p = (state.custom_config_path || "").trim();
        if (!p.startsWith("/")) {
          showToast("Enter an absolute target path starting with / (e.g. /etc/samba/smb-custom.conf)");
          return true;
        }
        state.custom_config_files[p] = state.custom_config_body || "";
        showToast(`Staged custom config file ${p} for USB provisioning`);
        renderApp();
        return true;
      } else if (action === "toggle-mc-online-mode") {
        state.mc_online_mode = !!target.checked;
        renderApp();
        return true;
      } else if (action === "select-studio-file") {
        const fn = target.getAttribute("data-file");
        if (fn) {
          state.studio_file_name = fn;
          renderApp();
        }
        return true;
      } else if (action === "add-studio-file") {
        const rawFn = (state.studio_new_file_name || "").trim().replace(/[^a-zA-Z0-9._-]/g, "");
        if (!rawFn) {
          showToast("Enter a file name first (e.g. custom.conf or whitelist.json)");
          return true;
        }
        const ov = getOrInitAppOverride(state.studio_app_id || "minecraft");
        if (!ov.config_files[rawFn]) {
          ov.config_files[rawFn] = `# /DATA/AppData/${state.studio_app_id || "minecraft"}/${rawFn}\n`;
        }
        state.studio_file_name = rawFn;
        state.studio_new_file_name = "";
        showToast(`Created ${rawFn} in /DATA/AppData/${state.studio_app_id || "minecraft"}`);
        renderApp();
        return true;
      } else if (action === "toggle-mc-aikar") {
        state.mc_use_aikar_flags = !!target.checked;
        state.mc_enable_backups = !!target.checked;
        return true;
      }
      return false;
    }