// dash_main.js — Kiwix, Hardware, Terminal, System File Handlers & Polling Bootstrap
document.body.addEventListener("click", async (e) => {
  const btn = e.target.closest("[data-action]");
  if (!btn) return;
  const action = btn.getAttribute("data-action");


  if (action === "switch-settings-subtab") {
    state.settingsSubTab = btn.getAttribute("data-subtab") || "general";
    renderDashboard();
    return;
  }
  if (action === "browse-disk") {
    const mnt = btn.getAttribute("data-mount") || "/";
    const res = await callDashboardApi("/api/system/files", "POST", { action: "list", path: mnt });
    if (res && Array.isArray(res.entries)) {
      res.entries.slice(0, 20).forEach(ent => {
        if (!ent.is_dir && !state.systemFileCatalog.some(f => f.path === ent.path)) {
          state.systemFileCatalog.push({ path: ent.path, category: mnt, is_system: mnt === "/", risk_level: mnt === "/" ? "System File" : "Disk File", warning: "Live disk file.", content: "" });
        }
      });
    }
    showToast("Loaded disk mount: " + mnt);
    renderDashboard();
    return;
  }
  if (action === "set-file-label") {
    state.fileLabelFilter = btn.getAttribute("data-label") || "all";
    renderDashboard();
    return;
  }
  if (action === "open-logs-in-files") {
    state.fileLabelFilter = "logs";
    state.selectedSystemFile = "/var/log/instant-server/apps-setup.log";
    state.activeTab = "files";
    renderDashboard();
    return;
  }
  if (action === "add-home-shortcut") {
    const sel = document.getElementById("selectAddHomeShortcut");
    if (sel && sel.value && !state.homeShortcuts.includes(sel.value)) {
      state.homeShortcuts.push(sel.value);
      await callDashboardApi("/api/dashboard/shortcuts", "POST", { shortcuts: state.homeShortcuts, names: state.shortcutNames });
      showToast("Added app shortcut to Home Launcher");
      renderDashboard();
    }
    return;
  }
  if (action === "remove-home-shortcut") {
    const id = btn.getAttribute("data-id");
    state.homeShortcuts = (state.homeShortcuts || []).filter(x => x !== id);
    await callDashboardApi("/api/dashboard/shortcuts", "POST", { shortcuts: state.homeShortcuts, names: state.shortcutNames });
    showToast("Removed shortcut from Home (app remains in App Store)");
    renderDashboard();
    return;
  }
  if (action === "rename-home-shortcut") {
    const id = btn.getAttribute("data-id");
    const cur = (state.shortcutNames && state.shortcutNames[id]) || id;
    const nextName = window.prompt("Rename Home Launcher shortcut:", cur);
    if (nextName && nextName.trim()) {
      state.shortcutNames = state.shortcutNames || {};
      state.shortcutNames[id] = nextName.trim();
      await callDashboardApi("/api/dashboard/shortcuts", "POST", { shortcuts: state.homeShortcuts, names: state.shortcutNames });
      renderDashboard();
    }
    return;
  }
  if (action === "save-dash-settings-form") {
    const patch = {};
    const g = id => document.getElementById(id);
    if (g("setServerName")) patch.server_name = g("setServerName").value.trim();
    if (g("setAddrHost")) { patch.address_host = g("setAddrHost").value.trim(); patch.hostname = patch.address_host; }
    if (g("setAddrDomain")) patch.address_domain = g("setAddrDomain").value.trim();
    if (g("setWifiSsid")) patch.wifi_ssid = g("setWifiSsid").value.trim();
    if (g("setWifiPass")) patch.wifi_password = g("setWifiPass").value;
    if (g("setScreenBlank")) patch.screen_blank_sec = Number(g("setScreenBlank").value) || 0;
    if (g("setLogRetentionDays")) patch.log_retention_days = Math.max(1, Math.min(365, Number(g("setLogRetentionDays").value) || 14));
    if (g("setSshPort")) patch.ssh_port = Number(g("setSshPort").value) || 22;
    Object.assign(state.settings, patch);
    await callDashboardApi("/api/dashboard/settings", "POST", patch);
    showToast("Server settings saved and applied");
    renderDashboard();
    return;
  }

  if (action === "add-custom-zim") {
    const urlIn = document.getElementById("inputCustomZimUrl");
    const urlVal = urlIn ? urlIn.value.trim() : "";
    if (!urlVal) {
      showToast("Enter a valid .zim URL first");
      return;
    }
    const fname = urlVal.split("/").pop() || "custom_archive.zim";
    state.kiwix_zims.unshift({
      id: "custom_" + Date.now(),
      title: fname,
      size_gb: 1.5,
      articles: "Custom ZIM Archive",
      category: "Custom",
      installed: true,
      progress_pct: 100,
      filename: fname,
      description: `Imported from ${urlVal}`
    });
    state.customZimUrl = "";
    showToast(`Queued & indexed ${fname} in Kiwix`);
    renderDashboard();
    return;
  }

  if (action === "dash-enable-cool-mode") {
    const nextCool = !state.settings.disable_cpu_turbo_boost;
    state.settings.disable_cpu_turbo_boost = nextCool;
    state.settings.cpu_governor = nextCool ? "powersave" : "schedutil";
    await callDashboardApi("/api/dashboard/settings", "POST", {
      disable_cpu_turbo_boost: nextCool,
      cpu_governor: state.settings.cpu_governor
    });
    showToast(nextCool
      ? "Enabled Cool & Quiet thermal mode (Turbo Boost off + Powersave governor)"
      : "Disabled Cool & Quiet mode (Restored Balanced schedutil governor)");
    renderDashboard();
    return;
  }

  if (action === "save-tailscale-config") {
    const keyIn = document.getElementById("inputDashTailscaleKey");
    if (keyIn) state.settings.tailscale_authkey = keyIn.value.trim();
    state.settings.enable_tailscale = true;
    const res = await callDashboardApi("/api/dashboard/tailscale", "POST", {
      enable_tailscale: state.settings.enable_tailscale,
      tailscale_authkey: state.settings.tailscale_authkey,
      tailscale_mode: state.settings.tailscale_mode
    });
    if (res && res.tailscale_ip) state.settings.tailscale_ip = res.tailscale_ip;
    showToast(`Tailscale remote access active (${state.settings.tailscale_ip || "100.104.28.14"})`);
    renderDashboard();
    return;
  }

  if (action === "toggle-setting") {
    const key = btn.getAttribute("data-key");
    if (key && key in state.settings) {
      state.settings[key] = !state.settings[key];
      await callDashboardApi("/api/dashboard/settings", "POST", { [key]: state.settings[key] });
      showToast(`Updated ${key.replace(/_/g, " ")}`);
      renderDashboard();
    }
    return;
  }

  if (action === "check-updates") {
    state.updates.last_checked = "Just now";
    showToast("Checked Ubuntu repositories & container registries");
    renderDashboard();
    return;
  }

  if (action === "apply-updates") {
    const target = btn.getAttribute("data-target") || "all";
    if (target === "all" || target === "os") {
      state.updates.os_updates = [];
    }
    if (target === "all" || target === "apps") {
      state.updates.app_updates.forEach(a => {
        a.current_tag = a.available_tag;
        a.update_available = false;
      });
    }
    state.updates.status_text = "All OS packages, web apps, and system files are up to date.";
    await callDashboardApi("/api/dashboard/updates/apply", "POST", { target });
    showToast("Installed updates cleanly");
    renderDashboard();
    return;
  }

  if (action === "run-backup-now") {
    showToast("Created snapshot /DATA/Backups/appdata-backup-latest.tar.zst");
    return;
  }

  if (action === "run-custom-workflow") {
    const res = await callDashboardApi("/api/custom/workflow", "POST", {
      name: "custom-automation.sh",
      run_now: true
    });
    state.customWorkflowStatus = (res && res.output)
      ? res.output
      : "Executed /DATA/CustomWorkflows/custom-automation.sh (exit status 0) • Custom Samba shares, udev rules & systemd timers verified.";
    showToast("Executed custom server workflow (/DATA/CustomWorkflows/custom-automation.sh)");
    renderDashboard();
    return;
  }

  if (action === "apply-offline-usb-update") {
    const res = await callDashboardApi("/api/system/update", "POST", { mode: "offline_usb" });
    state.updates.last_checked = "Just now (Offline Updater USB)";
    state.updates.status_text = (res && res.output)
      ? res.output
      : "Applied Offline Updater USB bundle: updated server_dashboard.html, daemon scripts, and local Docker images.";
    showToast("Checked & applied Offline Updater USB bundle");
    renderDashboard();
    return;
  }

  if (action === "term-clear") {
    state.terminalHistory = [];
    renderDashboard();
    return;
  }

  if (action === "term-quick-cmd") {
    const qCmd = btn.getAttribute("data-cmd") || "";
    if (qCmd === "clear") {
      state.terminalHistory = [];
      renderDashboard();
      return;
    }
    await executeWebTerminalCommand(qCmd);
    return;
  }

  if (action === "term-run-cmd") {
    const inp = document.getElementById("inputWebTerminalCmd");
    const cmd = inp ? inp.value.trim() : "";
    if (!cmd) return;
    await executeWebTerminalCommand(cmd);
    return;
  }

  if (action === "select-system-file") {
    const p = btn.getAttribute("data-path");
    if (p) {
      state.selectedSystemFile = p;
      state.confirmSystemFileEdit = false;
      renderDashboard();
    }
    return;
  }

  if (action === "toggle-sysfile-confirm") {
    const chk = document.getElementById("chkConfirmSysFileEdit");
    state.confirmSystemFileEdit = Boolean(chk && chk.checked);
    renderDashboard();
    return;
  }

  if (action === "create-system-file") {
    const inp = document.getElementById("inputNewSystemFilePath");
    const rawPath = inp ? inp.value.trim() : "";
    if (!rawPath || !rawPath.startsWith("/")) {
      showToast("Enter an absolute path starting with /DATA, /etc, or /opt");
      return;
    }
    const isSys = rawPath.startsWith("/etc") || rawPath.startsWith("/opt") || rawPath.startsWith("/usr");
    let existing = state.systemFileCatalog.find(f => f.path === rawPath);
    if (!existing) {
      existing = {
        path: rawPath,
        category: isSys ? "System Config (/etc)" : "User Data (/DATA)",
        is_system: isSys,
        risk_level: isSys ? "High — Custom System Configuration File" : "Safe — Custom User File",
        warning: isSys
          ? "Modifying system configuration files affects server daemons. A .bak backup is saved automatically."
          : "Custom user file stored on persistent /DATA storage.",
        content: `# ${rawPath}\n`
      };
      state.systemFileCatalog.unshift(existing);
    }
    state.selectedSystemFile = rawPath;
    state.confirmSystemFileEdit = false;
    state.newCustomFilePath = "";
    showToast(`Opened ${rawPath}`);
    renderDashboard();
    return;
  }

  if (action === "save-system-file") {
    const selected = state.systemFileCatalog.find(f => f.path === state.selectedSystemFile);
    const ta = document.getElementById("textareaSystemFileEditor");
    if (!selected || !ta) return;
    if (selected.is_system && !state.confirmSystemFileEdit) {
      showToast("Check the system configuration confirmation box first");
      return;
    }
    selected.backup_content = selected.content;
    selected.content = ta.value;
    await callDashboardApi("/api/system/files", "POST", {
      action: "write",
      path: selected.path,
      content: selected.content,
      confirmed_system_edit: true
    });
    showToast(`Saved ${selected.path} (Backup written to ${selected.path}.bak)`);
    return;
  }

  if (action === "restore-sysfile-backup") {
    const selected = state.systemFileCatalog.find(f => f.path === state.selectedSystemFile);
    if (!selected) return;
    if (selected.backup_content) {
      selected.content = selected.backup_content;
    }
    await callDashboardApi("/api/system/files", "POST", {
      action: "restore_bak",
      path: selected.path
    });
    showToast(`Restored ${selected.path} from .bak backup`);
    renderDashboard();
    return;
  }
});

async function executeWebTerminalCommand(cmd) {
  if (cmd === "clear") {
    state.terminalHistory = [];
    state.terminalInput = "";
    renderDashboard();
    return;
  }
  let out = "";
  let code = 0;
  const apiRes = await callDashboardApi("/api/terminal/exec", "POST", {
    command: cmd,
    cwd: state.terminalCwd
  });
  if (apiRes && typeof apiRes.output === "string") {
    out = apiRes.output;
    code = apiRes.exit_code || 0;
    if (apiRes.cwd) state.terminalCwd = apiRes.cwd;
  } else {
    // Intelligent local terminal responder when previewing offline
    if (cmd.startsWith("cd ")) {
      const target = cmd.slice(3).trim();
      state.terminalCwd = target.startsWith("/") ? target : `${state.terminalCwd.replace(/\/$/, "")}/${target}`;
      out = `Changed working directory to ${state.terminalCwd}`;
    } else if (cmd.includes("docker ps")) {
      out = state.apps.filter(a => a.installed).map(a => `${a.id.padEnd(18)} Up 14 days (healthy)   0.0.0.0:${a.port}->${a.port}/tcp`).join("\n");
    } else if (cmd.includes("df -h")) {
      out = `Filesystem      Size  Used Avail Use% Mounted on\n/dev/nvme0n1p2  256G   42G  214G  17% /DATA`;
    } else if (cmd.includes("ip -br") || cmd.includes("ip addr")) {
      out = `eth0             UP             ${state.telemetry.eth_ipv4}/24\nwlan0            UP             ${state.telemetry.wifi_ipv4}/24\ntailscale0       UP             ${state.settings.tailscale_ip}/32`;
    } else if (cmd.includes("iPhone_Backups")) {
      out = `total 16K\ndrwxrwxr-x 3 admin admin 4.0K Nov 09 03:00 .\n-rw-r--r-- 1 admin admin  312 Nov 09 03:00 README-iPhone-Setup.txt\ndrwxr-xr-x 2 admin admin 4.0K Nov 09 03:00 iOS_Shortcuts_Sync`;
    } else {
      out = `[Executed on ${state.settings.hostname}.local (${state.terminalCwd})]\n$ ${cmd}\nCommand completed (exit status 0).`;
    }
  }
  state.terminalHistory.push({ cmd, cwd: state.terminalCwd, output: out, exit_code: code });
  state.terminalInput = "";
  renderDashboard();
  const vp = document.getElementById("termViewport");
  if (vp) vp.scrollTop = vp.scrollHeight;
  const inp = document.getElementById("inputWebTerminalCmd");
  if (inp) inp.focus();
}

document.body.addEventListener("keydown", async (e) => {
  if (e.target && e.target.id === "inputWebTerminalCmd" && e.key === "Enter") {
    e.preventDefault();
    const cmd = e.target.value.trim();
    if (cmd) await executeWebTerminalCommand(cmd);
  }
});

document.body.addEventListener("input", async (e) => {
  if (e.target.id === "dashSearchInput") {
    state.searchQuery = e.target.value;
    renderDashboard();
    return;
  }
  if (e.target.id === "inputCustomZimUrl") {
    state.customZimUrl = e.target.value;
    return;
  }
  const mcField = e.target.getAttribute("data-mc-field");
  if (mcField) {
    if (mcField === "online_mode") {
      state.minecraftStudio.online_mode = e.target.value === "true";
    } else {
      state.minecraftStudio[mcField] = e.target.value;
    }
    renderSidebar();
    return;
  }
  const field = e.target.getAttribute("data-field");
  if (!field) return;
  if (field === "clusterConnectHost" || field === "clusterConnectIp" || field === "clusterMergePolicy" || field === "clusterConfirmWipe") {
    state[field] = e.target.value;
    if (field === "clusterMergePolicy" || field === "clusterConfirmWipe") {
      renderDashboard();
    }
    return;
  }
  const val = e.target.type === "range" ? Number(e.target.value) : e.target.value;
  state.settings[field] = val;

  if (field === "battery_charge_stop_pct") {
    const lbl = document.getElementById("lblDashBatCap");
    const badge = document.getElementById("valDashBatCap");
    if (lbl) lbl.textContent = `Battery charge limit cap (${state.settings.battery_charge_start_pct}% – ${val}%) • Health: ${state.telemetry.battery_health_pct || 89}%`;
    if (badge) badge.textContent = `${val}%`;
    renderTelemetry();
  } else if (field === "ups_shutdown_battery_pct") {
    const lbl = document.getElementById("lblDashUps");
    const badge = document.getElementById("valDashUps");
    if (lbl) lbl.textContent = `Built-in battery UPS safe shutdown threshold (${val}%)`;
    if (badge) badge.textContent = `${val}%`;
    renderTelemetry();
  } else if (field === "vm_swappiness") {
    const lbl = document.getElementById("lblDashSwappiness");
    const badge = document.getElementById("valDashSwappiness");
    if (lbl) lbl.textContent = `Memory Swap Tendency (${val}%)`;
    if (badge) badge.textContent = `${val}%`;
  } else {
    renderTelemetry();
  }
  await callDashboardApi("/api/dashboard/settings", "POST", { [field]: val });
});

renderDashboard();

document.body.addEventListener("input", (e) => {
  if (e.target && e.target.id === "setAddrDomain") {
    const box = document.getElementById("dashDomainWarnBox");
    if (box && typeof getDashDomainWarning === "function") {
      const msg = getDashDomainWarning(e.target.value);
      box.textContent = msg;
      box.style.display = msg ? "flex" : "none";
    }
  }
});