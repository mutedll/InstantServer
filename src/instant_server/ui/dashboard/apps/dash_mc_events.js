// FILE: src/instant_server/ui/dashboard/dash_mc_events.js
// Event Handlers for Dedicated Pro-Level Minecraft Control Panel

document.body.addEventListener("click", async (e) => {
  const el = e.target.closest("[data-action]");
  if (!el) return;
  const act = el.getAttribute("data-action");
  if (!act || !act.startsWith("mc-")) return;

  if (act === "mc-subtab") {
    mcState.subTab = el.getAttribute("data-subtab") || "console";
    renderDashboard();
    return;
  }
  if (act === "mc-refresh") {
    await mcRefreshAll();
    showToast("Minecraft console & status refreshed");
    return;
  }
  if (act === "mc-power") {
    const op = el.getAttribute("data-op") || "restart";
    showToast(`Sending Minecraft ${op}...`);
    const res = await callDashboardApi("/api/mc/power", "POST", { action: op });
    showToast((res && (res.output || res.error)) || `Minecraft ${op} complete`);
    await mcRefreshAll();
    return;
  }
  if (act === "mc-quick-rcon" || act === "mc-send-rcon") {
    const cmd = act === "mc-quick-rcon"
      ? (el.getAttribute("data-cmd") || "")
      : ((document.getElementById("mcRconInput") || {}).value || "").trim();
    if (!cmd) return;
    const res = await callDashboardApi("/api/mc/rcon", "POST", { command: cmd });
    mcState.logs += `\n> ${cmd}\n${(res && res.output) || "Sent"}`;
    const inp = document.getElementById("mcRconInput");
    if (inp) inp.value = "";
    renderDashboard();
    return;
  }
  if (act === "mc-prop-group") {
    mcState.propGroup = el.getAttribute("data-group") || "All";
    renderDashboard();
    return;
  }
  if (act === "mc-toggle-raw-props") {
    mcState.rawPropMode = !mcState.rawPropMode;
    renderDashboard();
    return;
  }
  if (act === "mc-save-props") {
    const restart = el.getAttribute("data-restart") === "1";
    let body = { restart };
    if (mcState.rawPropMode) {
      const ta = document.getElementById("mcRawPropsTa");
      body.raw_text = ta ? ta.value : "";
    } else {
      const props = {};
      document.querySelectorAll(".mc-prop-inp").forEach(inp => {
        const k = inp.getAttribute("data-prop");
        if (k) props[k] = inp.value;
      });
      Object.assign(mcState.properties, props);
      body.properties = props;
    }
    const res = await callDashboardApi("/api/mc/properties", "POST", body);
    showToast(res && res.ok ? (restart ? "Saved server.properties & restarted!" : "Saved server.properties") : ((res && res.error) || "Save failed"));
    return;
  }
  if (act === "mc-player-act") {
    const op = el.getAttribute("data-op") || "";
    const p = ((document.getElementById("mcPlayerName") || {}).value || "").trim();
    const res = await callDashboardApi("/api/mc/players", "POST", { action: op, player: p });
    showToast((res && (res.output || res.error)) || "Executed player action");
    await mcRefreshAll();
    return;
  }
  if (act === "mc-toggle-online-mode") {
    const cur = (mcState.properties["online-mode"] ?? "true") === "true";
    const next = cur ? "false" : "true";
    mcState.properties["online-mode"] = next;
    await callDashboardApi("/api/mc/properties", "POST", { properties: { "online-mode": next }, restart: true });
    showToast(`Switched to ${next === "true" ? "Online (Mojang)" : "Offline (Cracked/LAN)"} mode & restarted`);
    renderDashboard();
    return;
  }
  if (act === "mc-addon-kind") {
    mcState.addonKind = el.getAttribute("data-kind") || "plugins";
    await mcRefreshAll();
    return;
  }
  if (act === "mc-addon-url-install") {
    const url = ((document.getElementById("mcAddonUrl") || {}).value || "").trim();
    if (!url) return;
    const res = await callDashboardApi("/api/mc/addons", "POST", { action: "url_install", kind: mcState.addonKind, url });
    showToast((res && (res.output || res.error)) || "Installed addon");
    await mcRefreshAll();
    return;
  }
  if (act === "mc-addon-toggle" || act === "mc-addon-del") {
    const name = el.getAttribute("data-name") || "";
    if (act === "mc-addon-del" && !window.confirm(`Delete ${name}?`)) return;
    const res = await callDashboardApi("/api/mc/addons", "POST", { action: act === "mc-addon-del" ? "delete" : "toggle", kind: mcState.addonKind, name });
    showToast((res && (res.output || res.error)) || "Updated addon");
    await mcRefreshAll();
    return;
  }
  if (act === "mc-apply-software") {
    const g = id => document.getElementById(id);
    const res = await callDashboardApi("/api/mc/software", "POST", {
      type: g("mcSwType")?.value || "PAPER",
      version: g("mcSwVer")?.value || "LATEST",
      memory: g("mcSwMem")?.value || "4G",
      jvm_flags: g("mcSwJvm")?.value || "aikar",
      geyser_bedrock: Boolean(g("mcSwGeyser")?.checked),
      eula: true
    });
    showToast((res && (res.output || res.error)) || "Updated Minecraft software");
    await mcRefreshAll();
    return;
  }
  if (act === "mc-backup-create") {
    showToast("Creating RCON-safe world backup...");
    const res = await callDashboardApi("/api/mc/backups", "POST", { action: "create" });
    showToast((res && (res.output || res.error)) || "Backup created");
    await mcRefreshAll();
    return;
  }
  if (act === "mc-backup-restore" || act === "mc-backup-del") {
    const name = el.getAttribute("data-name") || "";
    if (!window.confirm(act === "mc-backup-restore" ? `Stop server and restore ${name}?` : `Delete backup ${name}?`)) return;
    const res = await callDashboardApi("/api/mc/backups", "POST", { action: act === "mc-backup-restore" ? "restore" : "delete", name });
    showToast((res && (res.output || res.error)) || "Done");
    await mcRefreshAll();
    return;
  }
  if (act === "mc-sched-add") {
    const g = id => document.getElementById(id);
    const name = (g("mcSchedName")?.value || "").trim() || "Scheduled Task";
    mcState.schedules.push({ name, cron: g("mcSchedCron")?.value || "0 5 * * *", action: g("mcSchedAct")?.value || "backup", enabled: true });
    await callDashboardApi("/api/mc/schedules", "POST", { schedules: mcState.schedules });
    showToast("Added schedule");
    renderDashboard();
    return;
  }
  if (act === "mc-sched-del") {
    const idx = Number(el.getAttribute("data-idx"));
    mcState.schedules.splice(idx, 1);
    await callDashboardApi("/api/mc/schedules", "POST", { schedules: mcState.schedules });
    renderDashboard();
    return;
  }
});

document.body.addEventListener("change", (e) => {
  if (e.target && (e.target.id === "mcAddonUpload" || e.target.id === "mcWorldUpload") && e.target.files?.[0]) {
    const file = e.target.files[0];
    const isWorld = e.target.id === "mcWorldUpload";
    const reader = new FileReader();
    reader.onload = async () => {
      const b64 = String(reader.result || "").split(",")[1] || "";
      const url = isWorld ? "/api/mc/backups" : "/api/mc/addons";
      const body = isWorld
        ? { action: "upload_world", world_name: file.name.replace(/\.zip$/i, ""), data_base64: b64 }
        : { action: "upload", kind: mcState.addonKind, filename: file.name, data_base64: b64 };
      const res = await callDashboardApi(url, "POST", body);
      showToast((res && (res.output || res.error)) || "Upload complete");
      await mcRefreshAll();
    };
    reader.readAsDataURL(file);
  }
});

document.body.addEventListener("input", (e) => {
  if (e.target && e.target.id === "mcPropSearch") {
    mcState.propSearch = e.target.value;
    renderDashboard();
  }
});