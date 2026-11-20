// FILE: src/instant_server/ui/dashboard/dash_updates_center.js
// Official-Repository Update Center (Ubuntu OS, github.com/mutedll/InstantServer, Docker Registries)
// Features: Search for Updates, Per-Item Checkboxes, Single 1-Click Select All / Unselect All Toggle, Install Selected

const updState = {
  checking: false,
  installing: false,
  checkedAt: "Not checked yet",
  items: [
    { id: "dashboard:instant_server", category: "dashboard", name: "Instant Server Dashboard & System Scripts", current: "installed", available: "latest (main)", source: "https://github.com/mutedll/InstantServer", notes: "Official Instant Server repo" }
  ],
  selected: new Set(["dashboard:instant_server"]),
  lastLog: ""
};

async function updCheckNow() {
  updState.checking = true;
  renderDashboard();
  const res = await callDashboardApi("/api/updates/check", "GET");
  updState.checking = false;
  if (res && res.ok && Array.isArray(res.updates)) {
    updState.items = res.updates;
    updState.checkedAt = res.checked_at || "Just now";
    updState.selected = new Set(res.updates.map(u => u.id));
    showToast(`Found ${res.updates.length} official update(s)`);
  } else {
    updState.checkedAt = "Just now (Preview)";
    showToast("Checked official repositories");
  }
  renderDashboard();
}

async function updInstallSelected() {
  const ids = Array.from(updState.selected);
  if (!ids.length) {
    showToast("Select at least one update first");
    return;
  }
  updState.installing = true;
  renderDashboard();
  const res = await callDashboardApi("/api/updates/install", "POST", { ids });
  updState.installing = false;
  if (res && res.ok) {
    updState.lastLog = (res.results || []).join(" • ") || `Installed ${ids.length} update(s).`;
    updState.items = updState.items.filter(u => !updState.selected.has(u.id));
    updState.selected.clear();
    showToast(`Installed ${ids.length} official update(s)!`);
  } else {
    showToast((res && res.error) || "Update install failed");
  }
  renderDashboard();
}

function renderOfficialUpdatesCenter() {
  const allSelected = updState.items.length > 0 && updState.items.every(u => updState.selected.has(u.id));
  const selCount = updState.items.filter(u => updState.selected.has(u.id)).length;

  return `
    <div style="display:flex;flex-direction:column;gap:12px;animation:dashFadeSlide 0.2s ease;">
      <div class="os-hero-bar">
        <div>
          <div style="font-size:18px;font-weight:700;">Official Software &amp; System Updates</div>
          <div style="font-size:12px;color:var(--text-secondary);">Pulls directly from Ubuntu APT, <code>github.com/mutedll/InstantServer</code>, and official Docker registries • Last checked: ${esc(updState.checkedAt)}</div>
        </div>
        <div style="display:flex;gap:8px;flex-wrap:wrap;">
          <button type="button" class="win-btn" data-action="upd-check-now" ${updState.checking ? "disabled" : ""}>${updState.checking ? "Checking Official Repos..." : "↻ Search for Updates"}</button>
          <button type="button" class="win-btn win-btn-accent" data-action="upd-install-selected" ${updState.installing || selCount === 0 ? "disabled" : ""}>${updState.installing ? "Installing..." : `Install Selected (${selCount})`}</button>
        </div>
      </div>

      ${updState.lastLog ? `<div class="set-card" style="padding:10px 14px;font-size:12px;color:#30d158;">✓ ${esc(updState.lastLog)}</div>` : ""}

      <div class="set-card">
        <div class="set-row" style="background:rgba(255,255,255,0.025);">
          <label style="display:flex;align-items:center;gap:10px;cursor:pointer;font-weight:700;font-size:13px;">
            <input type="checkbox" id="updToggleAllChk" data-action="upd-toggle-all" ${allSelected ? "checked" : ""} />
            <span>${allSelected ? "Unselect All" : "Select All"} (${updState.items.length} available)</span>
          </label>
          <button type="button" class="win-btn win-btn-sm" data-action="switch-tab" data-tab="settings">Auto-Update Preferences</button>
        </div>
        ${updState.items.map(u => {
          const checked = updState.selected.has(u.id);
          const badge = u.category === "dashboard" ? "Instant Server Repo" : u.category === "os" ? "Ubuntu Official" : "Docker Registry";
          return `
            <div class="set-row">
              <label style="display:flex;align-items:flex-start;gap:10px;cursor:pointer;flex:1;">
                <input type="checkbox" data-action="upd-toggle-one" data-id="${esc(u.id)}" ${checked ? "checked" : ""} style="margin-top:3px;" />
                <div>
                  <div class="set-title">${esc(u.name)} <span class="badge badge-accent" style="margin-left:6px;">${badge}</span></div>
                  <div class="set-sub">${esc(u.current)} → <strong>${esc(u.available)}</strong> • Source: <code>${esc(u.source)}</code></div>
                </div>
              </label>
            </div>`;
        }).join("") || `<div class="fm-empty">Everything is up to date! Click "Search for Updates" to check official repos again.</div>`}
      </div>
    </div>`;
}

document.body.addEventListener("click", async (e) => {
  const el = e.target.closest("[data-action]");
  if (!el) return;
  const act = el.getAttribute("data-action");
  if (act === "upd-check-now") { await updCheckNow(); return; }
  if (act === "upd-install-selected") { await updInstallSelected(); return; }
  if (act === "upd-toggle-all") {
    const allSelected = updState.items.length > 0 && updState.items.every(u => updState.selected.has(u.id));
    if (allSelected) updState.selected.clear();
    else updState.selected = new Set(updState.items.map(u => u.id));
    renderDashboard();
    return;
  }
  if (act === "upd-toggle-one") {
    const id = el.getAttribute("data-id");
    if (id) {
      if (updState.selected.has(id)) updState.selected.delete(id);
      else updState.selected.add(id);
      renderDashboard();
    }
  }
});