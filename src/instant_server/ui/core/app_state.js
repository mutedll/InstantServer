// FILE: src/instant_server/ui/app_state.js
    const state = {
      theme: "dark",
      ui_mode: "standard",
      preset_role: "custom",
      hostIsAdmin: true,
      hostOsName: "windows",
      viewMode: "setup",
      setupTab: "network",
      settingsTab: "system",
      tabScrollPositions: { setup: {}, settings: {} },
      verifyingUsb: false,
      searchQuery: "",
      openDropdownField: null,
      advancedExpanded: {
        network: false,
        apps: false,
        storage: false,
        power: false
      },
      configSavedLocally: false,
      savedConfigPath: "Instant_Server_Data/instant-server-config.json",

      // Q1: Identity, Network & Multi-Computer Cluster
      server_name: "Instant Server",
      server_address_host: "instant-server",
      server_address_domain: ".local",
      log_retention_days: 14,
      sidebarRevealPass: false,
      hostname: "instant-server",
      username: "admin",
      password: "",
      wifi_ssid: "",
      wifi_password: "",
      enable_usb_tethering: true,
      disable_wifi_power_save: true,
      enable_mosh_server: true,
      static_ip_cidr: "",
      static_gateway: "",
      static_dns: "",
      ssh_authorized_key: "",
      cluster_mode: "standalone",
      cluster_storage_policy: "mergerfs_union_safe",
      confirm_wipe_secondary: false,

      // Q2: Dashboards & Pre-Selected Self-Hosted Apps
      dashboard_choice: "both",
      selected_apps: ["jellyfin", "nextcloud", "pihole", "kiwix", "immich", "homeassistant", "vaultwarden"],
      appSearchQuery: "",
      appCategoryFilter: "All",
      custom_apps: [],
      customWebAppInput: "",
      prebundle_offline_images: false,
      enable_pihole_dns_prep: true,
      enable_samba_nas: true,
      enable_intel_vaapi: true,
      enable_tailscale: false,
      tailscale_authkey: "",
      tailscale_mode: "subnet_and_ssh",
      mc_server_type: "PAPER",
      mc_version: "26.3",
      mc_memory_heap: "4G",
      mc_use_aikar_flags: true,
      mc_gamemode: "survival",
      mc_difficulty: "normal",
      mc_online_mode: true,
      mc_max_players: 20,
      mc_view_distance: 10,
      mc_level_seed: "",
      mc_motd: "Instant Server Minecraft World",
      mc_ops_list: "",
      mc_whitelist: "",
      mc_modrinth_projects: "viaversion,chunky",
      mc_enable_rcon: true,
      mc_rcon_password: "instant-rcon-admin",
      mc_enable_backups: true,
      studio_app_id: "minecraft",
      studio_file_name: "server.properties",
      studio_new_file_name: "",
      app_overrides: {},

      // Q3: Target Computer Type, Age, RAM, ISO Source & USB Partitioning
      device_form_factor: "auto_detect",
      laptop_age_hint: "unknown",
      laptop_ram_hint: "unknown",
      usb_partition_scheme: "auto",
      iso_source_mode: "official_dl",
      custom_iso_path: "",
      confirm_usb_erase: true,

      install_mode: "dual_boot",
      dual_boot_Allocation_gb: 45,
      storageUnit: "GB",
      storageInputRaw: "45",
      storageInputError: "",
      confirm_erase_main_drive: false,
      confirm_wipe_server_data: false,
      enable_iphone_auto_backup: false,
      iphone_backup_mode: "",
      custom_apt_packages_raw: "",
      custom_config_path: "/etc/samba/smb-custom.conf",
      custom_config_body: "# Custom server configuration file\n",
      custom_config_files: {},
      ubuntu_release: "26.04",
      storage_layout: "direct",
      boot_mode: "auto",

      // Q4: Laptop Power & Survival
      ignore_lid_switch: true,
      screen_blank_seconds: "300",
      battery_charge_start_pct: 40,
      battery_charge_stop_pct: 60,
      ups_shutdown_battery_pct: 15,
      enable_low_ram_zram: true,
      safe_gpu_nomodeset: false,

      // Custom Server Homepage Dashboard, Offline Wikipedia & Pro Mode Controls
      enable_custom_dashboard: true,
      dashboard_port: 8088,
      enable_auto_updates: true,
      auto_update_window: "03:00",
      enable_scheduled_backups: true,
      backup_retention_days: 14,
      preselect_kiwix_zim: "wikipedia_en_simple",
      ssh_port: 22,
      enable_ufw_firewall: true,
      enable_fail2ban: true,
      enable_ipv4_forwarding: true,
      enable_tcp_bbr: true,
      vm_swappiness: 10,
      inotify_max_watches: 524288,
      root_filesystem: "ext4",
      enable_ssd_noatime_trim: true,
      enable_smartd_monitor: true,
      zram_algorithm: "zstd",
      zram_percent: 50,
      cpu_governor: "schedutil",
      disable_cpu_turbo_boost: false,
      docker_subnet_cidr: "172.17.0.1/16",
      enable_prometheus_metrics: false,
      custom_kernel_args: "",
      custom_post_install_script: "",

      // Connected USB Devices & Flashing Status
      usbDevices: [],
      selectedUsbId: "",
      scanningUsb: false,
      resettingUsb: false,

      flashing: false,
      flashPaused: false,
      flashCancelled: false,
      flashOpSeq: 0,
      cacheRetentionMode: "keep",
      cacheStatusMessage: "Keep cached ISO & app files (Default) — If you close the app or reflash later with new apps/edits, only the new/changed apps are installed.",
      appsExpanded: false,
      flashProgress: 0,
      flashStageIndex: 0,
      flashStatusText: "",
      flashEtaText: "",
      flashLogs: [],
      renderedLogCount: 0,
      activeProgressLine: "",
      logAutoScroll: true,
      closeWarningOpen: false,
      forceClosing: false,
      validationErrors: [],

      lanScanResults: [],
      lanScanning: false
    };

    let pollTimer = null;

    function esc(val) {
      return String(val ?? "")
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;");
    }

    function showToast(msg) {
      const el = document.getElementById("toastMsg");
      if (!el) return;
      el.textContent = msg;
      el.classList.add("visible");
      clearTimeout(el._timer);
      el._timer = setTimeout(() => el.classList.remove("visible"), 2800);
    }

    async function callBackend(endpoint, method = "GET", payload = null) {
      try {
        if (window.go && window.go.main && window.go.main.App) {
          const wailsApp = window.go.main.App;
          if (endpoint === "host" && wailsApp.ProbeHost) return await wailsApp.ProbeHost();
          if (endpoint === "fit-window" && wailsApp.FitWindowToScreen && payload) return await wailsApp.FitWindowToScreen(payload.avail_width || 1280, payload.avail_height || 720);
          if (endpoint === "toggle-maximize" && wailsApp.ToggleMaximizeWindow) return await wailsApp.ToggleMaximizeWindow();
          if (endpoint === "elevate" && wailsApp.ElevateAdmin) return await wailsApp.ElevateAdmin();
          if (endpoint === "usb" && wailsApp.ScanUSBDevices) return await wailsApp.ScanUSBDevices();
          if (endpoint === "reset-usb" && wailsApp.ResetUSBDrive) return await wailsApp.ResetUSBDrive((payload && payload.device) || "");
          if (endpoint === "verify-usb" && wailsApp.VerifyUSBDrive) return await wailsApp.VerifyUSBDrive(body && body.device ? String(body.device) : "");
          if (endpoint === "boot-usb-next" && wailsApp.BootUSBNextRestart) return await wailsApp.BootUSBNextRestart(body && body.mode ? String(body.mode) : "restart_now");
          if (endpoint === "load-config" && wailsApp.LoadConfig) return await wailsApp.LoadConfig();
          if (endpoint === "save-config" && wailsApp.SaveConfig) return await wailsApp.SaveConfig(payload);
          if (endpoint === "custom-app" && wailsApp.AddCustomApp) return await wailsApp.AddCustomApp(payload);
          if (endpoint === "flash" && wailsApp.FlashUSBDevice) return await wailsApp.FlashUSBDevice(payload, (payload && payload.custom_iso_path) || "");
          if (endpoint === "pause" && wailsApp.PauseFlashing) return await wailsApp.PauseFlashing();
          if (endpoint === "resume" && wailsApp.ResumeFlashing) return await wailsApp.ResumeFlashing();
          if (endpoint === "cancel" && wailsApp.CancelFlashing) return await wailsApp.CancelFlashing();
          if (endpoint === "pause-and-exit" && wailsApp.PauseAndPrepareExit) return await wailsApp.PauseAndPrepareExit();
          if (endpoint === "cache" && wailsApp.ManageFlashCache) return await wailsApp.ManageFlashCache((payload && payload.action) || "keep");
          if (endpoint === "status" && wailsApp.PollBuildStatus) return await wailsApp.PollBuildStatus();
          if (endpoint === "discover" && wailsApp.DiscoverServers) return await wailsApp.DiscoverServers(state.hostname);
        }
        if (window.location.protocol.startsWith("http")) {
          const targetUrl = window.location.origin + "/" + "api/" + endpoint;
          const opts = {
            method,
            headers: { "Content-Type": "application/json" },
            cache: "no-store"
          };
          if (payload && method === "POST") {
            opts.body = JSON.stringify(payload);
          }
          const resp = await fetch(targetUrl, opts);
          if (resp.ok) {
            return await resp.json();
          }
        }
        } catch (_err) {}
      return null;
    }

    function serializeProfilePayload() {
      const selectedDev = state.usbDevices.find(d => d.device_id === state.selectedUsbId);
      return {
        hostname: (state.hostname || "instant-server").trim(),
        username: (state.username || "admin").trim(),
        password: state.password,
        wifi_ssid: state.wifi_ssid.trim(),
        wifi_password: state.wifi_password,
        enable_usb_tethering: state.enable_usb_tethering,
        disable_wifi_power_save: state.disable_wifi_power_save,
        enable_mosh_server: state.enable_mosh_server,
        static_ip_cidr: state.static_ip_cidr.trim(),
        static_gateway: state.static_gateway.trim(),
        static_dns: state.static_dns ? state.static_dns.split(",").map(s => s.trim()).filter(Boolean) : [],
        ssh_authorized_keys: state.ssh_authorized_key.trim() ? [state.ssh_authorized_key.trim()] : [],
        cluster_mode: state.cluster_mode,
        cluster_storage_policy: state.cluster_storage_policy,
        cluster_confirm_wipe_secondary: state.confirm_wipe_secondary ? "WIPE-SECONDARY" : "",
        device_form_factor: state.device_form_factor,
        dashboard_choice: state.dashboard_choice,
        selected_apps: Array.isArray(state.selected_apps) ? state.selected_apps : [],
        custom_apps: Array.isArray(state.custom_apps) ? state.custom_apps : [],
        app_overrides: state.app_overrides || {},
        minecraft_config: {
          server_type: state.mc_server_type || "PAPER",
          version: state.mc_version || "26.3",
          memory_heap: state.mc_memory_heap || "4G",
          use_aikar_flags: !!state.mc_use_aikar_flags,
          gamemode: state.mc_gamemode || "survival",
          difficulty: state.mc_difficulty || "normal",
          online_mode: !!state.mc_online_mode,
          max_players: Number(state.mc_max_players) || 20,
          view_distance: Number(state.mc_view_distance) || 10,
          simulation_distance: 10,
          level_seed: (state.mc_level_seed || "").trim(),
          motd: (state.mc_motd || "Instant Server Minecraft World").trim(),
          ops_list: (state.mc_ops_list || "").trim(),
          whitelist: (state.mc_whitelist || "").trim(),
          modrinth_projects: (state.mc_modrinth_projects || "").trim(),
          enable_rcon: !!state.mc_enable_rcon,
          rcon_password: (state.mc_rcon_password || "instant-rcon-admin").trim(),
          enable_backups: !!state.mc_enable_backups
        },
        prebundle_offline_images: state.prebundle_offline_images,
        enable_pihole_dns_prep: state.enable_pihole_dns_prep,
        enable_samba_nas: state.enable_samba_nas,
        enable_intel_vaapi: state.enable_intel_vaapi,
        enable_tailscale: state.enable_tailscale,
        tailscale_authkey: state.tailscale_authkey.trim(),
        tailscale_mode: state.tailscale_mode || "subnet_and_ssh",
        laptop_age_hint: state.laptop_age_hint,
        laptop_ram_hint: state.laptop_ram_hint,
        usb_partition_scheme: state.usb_partition_scheme,
        custom_iso_path: state.iso_source_mode === "local_file" ? state.custom_iso_path.trim() : "",
        confirm_usb_erase: state.confirm_usb_erase,
        format_usb: true,
        install_mode: state.install_mode,
        dual_boot_Allocation_gb: Number(state.dual_boot_Allocation_gb) || 45,
        confirm_erase_main_drive: state.confirm_erase_main_drive,
        confirm_wipe_server_data: state.confirm_wipe_server_data ? "WIPE-SERVER-DATA" : "",
        enable_iphone_auto_backup: !!state.enable_iphone_auto_backup,
        iphone_backup_mode: state.iphone_backup_mode || "all_three",
        custom_config_files: state.custom_config_files || {},
        custom_apt_packages: (state.custom_apt_packages_raw || "").split(/[\s,]+/).map(s => s.trim()).filter(Boolean),
        ubuntu_release: String(state.ubuntu_release),
        storage_layout: state.storage_layout,
        boot_mode: state.boot_mode,
        ignore_lid_switch: state.ignore_lid_switch,
        screen_blank_seconds: Number(state.screen_blank_seconds) || 60,
        battery_charge_start_pct: Number(state.battery_charge_start_pct) || 40,
        battery_charge_stop_pct: Number(state.battery_charge_stop_pct) || 60,
        ups_shutdown_battery_pct: Number(state.ups_shutdown_battery_pct) || 15,
        enable_low_ram_zram: state.enable_low_ram_zram,
        safe_gpu_nomodeset: state.safe_gpu_nomodeset,
        ui_mode: state.ui_mode,
        preset_role: state.preset_role,
        enable_custom_dashboard: state.enable_custom_dashboard,
        dashboard_port: Number(state.dashboard_port) || 8088,
        enable_auto_updates: state.enable_auto_updates,
        auto_update_window: state.auto_update_window,
        enable_scheduled_backups: state.enable_scheduled_backups,
        backup_retention_days: Number(state.backup_retention_days) || 14,
        preselect_kiwix_zims: [state.preselect_kiwix_zim || "wikipedia_en_simple"],
        ssh_port: Number(state.ssh_port) || 22,
        enable_ufw_firewall: state.enable_ufw_firewall,
        enable_fail2ban: state.enable_fail2ban,
        enable_ipv4_forwarding: state.enable_ipv4_forwarding,
        enable_tcp_bbr: state.enable_tcp_bbr,
        vm_swappiness: Number(state.vm_swappiness) || 10,
        inotify_max_watches: Number(state.inotify_max_watches) || 524288,
        root_filesystem: state.root_filesystem,
        enable_ssd_noatime_trim: state.enable_ssd_noatime_trim,
        enable_smartd_monitor: state.enable_smartd_monitor,
        zram_algorithm: state.zram_algorithm,
        zram_percent: Number(state.zram_percent) || 50,
        cpu_governor: state.cpu_governor,
        disable_cpu_turbo_boost: state.disable_cpu_turbo_boost,
        docker_subnet_cidr: state.docker_subnet_cidr.trim() || "172.17.0.1/16",
        enable_prometheus_metrics: state.enable_prometheus_metrics,
        custom_kernel_args: state.custom_kernel_args.trim(),
        custom_post_install_script: state.custom_post_install_script,
        selected_usb_device: selectedDev ? selectedDev.device_node : state.selectedUsbId
      };
    }

    function applyConfigToState(cfg) {
      if (!cfg || typeof cfg !== "object") return;
      const keys = [
        "hostname", "username", "password", "wifi_ssid", "wifi_password",
        "enable_usb_tethering", "static_ip_cidr", "static_gateway",
        "cluster_mode", "cluster_storage_policy", "device_form_factor",
        "dashboard_choice", "prebundle_offline_images", "enable_pihole_dns_prep",
        "enable_samba_nas", "enable_intel_vaapi", "enable_tailscale", "tailscale_authkey", "tailscale_mode",
        "laptop_age_hint", "laptop_ram_hint", "usb_partition_scheme", "custom_iso_path",
        "confirm_usb_erase", "install_mode", "dual_boot_Allocation_gb", "confirm_erase_main_drive",
        "ubuntu_release", "storage_layout", "boot_mode", "ignore_lid_switch",
        "screen_blank_seconds", "battery_charge_start_pct", "battery_charge_stop_pct",
        "ups_shutdown_battery_pct", "enable_low_ram_zram", "safe_gpu_nomodeset",
        "ui_mode", "preset_role", "enable_custom_dashboard", "dashboard_port",
        "enable_auto_updates", "auto_update_window", "enable_scheduled_backups",
        "backup_retention_days", "ssh_port", "enable_ufw_firewall", "enable_fail2ban",
        "enable_ipv4_forwarding", "enable_tcp_bbr", "vm_swappiness", "inotify_max_watches",
        "root_filesystem", "enable_ssd_noatime_trim", "enable_smartd_monitor",
        "zram_algorithm", "zram_percent", "cpu_governor", "disable_cpu_turbo_boost",
        "docker_subnet_cidr", "enable_prometheus_metrics", "custom_kernel_args",
        "custom_post_install_script"
      ];
      keys.forEach(k => {
        if (cfg[k] !== undefined) state[k] = cfg[k];
      });
      if (Array.isArray(cfg.selected_apps)) {
        state.selected_apps = cfg.selected_apps;
      }
      if (Array.isArray(cfg.custom_apps)) {
        state.custom_apps = cfg.custom_apps;
        cfg.custom_apps.forEach(ca => {
          if (ca && ca.id && !APP_STORE_CATALOG.some(a => a.id === ca.id)) {
            APP_STORE_CATALOG.unshift({
              id: ca.id,
              title: ca.title || ca.id,
              category: ca.category || "Security & Tools",
              recommended: true,
              tag: "Custom • Port " + (ca.port || 8080),
              ram: "128 MB",
              hardware_tier: "old_friendly",
              image: ca.image || "",
              port: ca.port || 8080,
              desc: ca.description || ("Custom container (" + (ca.image || ca.id) + ")")
            });
          }
        });
      }
      if (cfg.custom_iso_path && String(cfg.custom_iso_path).trim()) {
        state.iso_source_mode = "local_file";
      }
      state.screen_blank_seconds = String(state.screen_blank_seconds || "60");
      state.ubuntu_release = String(state.ubuntu_release || "26.04");
      if (Array.isArray(cfg.static_dns)) {
        state.static_dns = cfg.static_dns.join(", ");
      }
      if (Array.isArray(cfg.ssh_authorized_keys) && cfg.ssh_authorized_keys.length > 0) {
        state.ssh_authorized_key = cfg.ssh_authorized_keys[0];
      }
    }

    async function initAppStartup() {
      try {
        await callBackend("fit-window", "POST", {
          avail_width: Number(window.screen && window.screen.availWidth) || 1280,
          avail_height: Number(window.screen && window.screen.availHeight) || 720
        });
      } catch (_e) {}
      let loadedFromDisk = false;

      if (window.__INSTANT_SERVER_BOOT_CONFIG__ && window.__INSTANT_SERVER_BOOT_CONFIG__.exists && window.__INSTANT_SERVER_BOOT_CONFIG__.config) {
        applyConfigToState(window.__INSTANT_SERVER_BOOT_CONFIG__.config);
        state.configSavedLocally = true;
        state.savedConfigPath = window.__INSTANT_SERVER_BOOT_CONFIG__.path || "Instant_Server_Data/instant-server-config.json";
        state.viewMode = "settings";
        loadedFromDisk = true;
      }

      if (!loadedFromDisk) {
        const res = await callBackend("load-config", "GET");
        if (res && res.exists && res.config) {
          applyConfigToState(res.config);
          state.configSavedLocally = true;
          state.savedConfigPath = res.path || "Instant_Server_Data/instant-server-config.json";
          state.viewMode = "settings";
          loadedFromDisk = true;
        }
      }

      if (!loadedFromDisk) {
        try {
          const cached = localStorage.getItem("instant_server_saved_config");
          if (cached) {
            applyConfigToState(JSON.parse(cached));
            state.configSavedLocally = true;
            state.viewMode = "settings";
          }
        } catch (_e) {}
      }

      const hostInfo = await callBackend("host", "GET");
      if (hostInfo && typeof hostInfo === "object") {
        state.hostIsAdmin = !!hostInfo.is_admin_or_root;
        state.hostOsName = String(hostInfo.host_os || "windows");
        const adminBanner = document.getElementById("adminPrivilegeBanner");
        if (adminBanner) {
          adminBanner.style.display = (!state.hostIsAdmin && state.hostOsName === "windows") ? "flex" : "none";
        }
      }
      await scanUsbDrives(true);
      shouldAnimateTab = true;
      renderApp();
    }

    


    function normalizeServerDomain(raw) {
      const cleaned = String(raw || ".local").trim().toLowerCase().replace(/^\.+/, "");
      return cleaned ? ("." + cleaned) : ".local";
    }

    function getFullServerAddress() {
      const host = (state.server_address_host || state.hostname || "instant-server").trim().toLowerCase() || "instant-server";
      return host + normalizeServerDomain(state.server_address_domain);
    }

    function getDomainSecurityWarning(domainRaw) {
      const dom = normalizeServerDomain(domainRaw);
      const safePrivate = new Set([".local", ".home.arpa", ".lan", ".internal", ".home", ".arpa"]);
      if (safePrivate.has(dom)) return "";
      const hstsCritical = new Set([".dev", ".app", ".page", ".new", ".foo", ".google", ".bank", ".gov"]);
      if (hstsCritical.has(dom)) {
        return `Critical Browser Block: "${dom}" is HSTS-preloaded in Chrome/Edge/Safari, forcing HTTPS and blocking local HTTP access. Use .local, .home.arpa, .lan, or .internal instead.`;
      }
      return `Warning: Public domain "${dom}" can cause browser TLS/HSTS security errors or public DNS conflicts on local networks. Recommended for home servers: .local, .home.arpa, .lan, or .internal.`;
    }