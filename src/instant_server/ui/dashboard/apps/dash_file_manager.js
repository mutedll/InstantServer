// FILE: src/instant_server/ui/dashboard/dash_file_manager.js
// Sleek Windows Explorer / macOS Finder–style Web File Manager
const fmState = {
  path: "/DATA",
  parent: "/",
  label: "all",
  query: "",
  sortBy: "name",
  viewMode: "list", // "list" | "grid"
  disks: [
    { device: "/dev/sda2", mount: "/", size: 500e9, used: 18e9, avail: 482e9, pct: "4%" },
    { device: "/dev/sda2", mount: "/DATA", size: 500e9, used: 18e9, avail: 482e9, pct: "4%" }
  ],
  entries: [],
  clipboard: null, // { op: "copy"|"move", src: "/path" }
  editor: null,    // { path, name, content, is_critical, has_bak, confirmed }
  loading: false,
  loadedOnce: false
};

function fmFormatSize(bytes) {
  if (!bytes || bytes <= 0) return "—";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let i = 0; let v = bytes;
  while (v >= 1024 && i < units.length - 1) { v /= 1024; i++; }
  return `${v.toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
}

function fmIconSvg(entry) {
  if (entry.is_dir) {
    return `<svg width="20" height="20" viewBox="0 0 24 24" fill="rgba(255,159,10,0.22)" stroke="#ff9f0a" stroke-width="1.9"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/></svg>`;
  }
  const color = entry.label === "logs" ? "#30d158" : entry.label === "configs" ? "#0a84ff" : entry.label === "system" ? "#ff453a" : "#94a3b8";
  return `<svg width="19" height="19" viewBox="0 0 24 24" fill="none" stroke="${color}" stroke-width="1.9"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/></svg>`;
}

async function fmLoadDisks() {
  const res = await callDashboardApi("/api/fm/disks", "GET");
  if (res && Array.isArray(res.disks) && res.disks.length) fmState.disks = res.disks;
}

async function fmLoadDir(path, label) {
  if (typeof path === "string") fmState.path = path;
  if (typeof label === "string") fmState.label = label;
  fmState.loading = true;
  fmState.loadedOnce = true;
  const qs = new URLSearchParams({
    path: fmState.path,
    label: fmState.label,
    q: fmState.query,
    sort: fmState.sortBy
  });
  const res = await callDashboardApi(`/api/fm/list?${qs.toString()}`, "GET");
  fmState.loading = false;
  if (res && res.ok && Array.isArray(res.entries)) {
    fmState.path = res.path;
    fmState.parent = res.parent || "/";
    fmState.entries = res.entries;
  } else {
    // Graceful preview fallback when viewing HTML offline
    fmState.entries = (state.systemFileCatalog || []).map(f => ({
      name: f.path.split("/").pop(),
      path: f.path,
      is_dir: false,
      size: (f.content || "").length,
      mtime_str: "Live",
      perms: f.is_system ? "600" : "644",
      label: f.path.includes("/var/log") ? "logs" : (f.is_system ? "configs" : "apps"),
      is_critical: !!f.is_system
    })).filter(e => fmState.label === "all" || e.label === fmState.label);
  }
  renderDashboard();
}

async function fmOpenFile(path) {
  const res = await callDashboardApi(`/api/fm/read?path=${encodeURIComponent(path)}`, "GET");
  if (res && res.ok) {
    fmState.editor = { ...res, confirmed: !res.is_critical };
  } else {
    const local = (state.systemFileCatalog || []).find(f => f.path === path);
    fmState.editor = {
      path,
      name: path.split("/").pop(),
      content: local ? local.content : "",
      is_critical: local ? !!local.is_system : path.startsWith("/etc"),
      has_bak: false,
      confirmed: !(local && local.is_system)
    };
  }
  renderDashboard();
}

function renderFullFileManagerTab() {
  if (!fmState.loadedOnce) {
    setTimeout(() => { fmLoadDisks().then(() => fmLoadDir(fmState.path, fmState.label)); }, 10);
  }
  const crumbs = fmState.path.split("/").filter(Boolean);
  let acc = "";
  const crumbHtml = `<button type="button" class="fm-crumb" data-action="fm-nav" data-path="/">/</button>` +
    crumbs.map(c => {
      acc += "/" + c;
      return `<span class="fm-crumb-sep">›</span><button type="button" class="fm-crumb" data-action="fm-nav" data-path="${esc(acc)}">${esc(c)}</button>`;
    }).join("");

  const quickLabels = [
    { id: "all", name: "All Files", path: fmState.path },
    { id: "logs", name: "Server Logs", path: "/var/log/instant-server" },
    { id: "configs", name: "Configs (/etc)", path: "/etc" },
    { id: "apps", name: "App Data", path: "/DATA/AppData" },
    { id: "backups", name: "Backups", path: "/DATA/Backups" },
    { id: "system", name: "System Core", path: "/opt/instant-server" }
  ];

  const rowsHtml = fmState.entries.map(e => {
    const badgeCls = `fm-lbl-${e.label || "user"}`;
    const actionsHtml = `
      <div class="fm-row-actions">
        ${e.is_dir
          ? `<button type="button" class="fm-act-btn" data-action="fm-nav" data-path="${esc(e.path)}">Open</button>`
          : `<button type="button" class="fm-act-btn" data-action="fm-open" data-path="${esc(e.path)}">Edit</button>
             <a class="fm-act-btn" href="/api/fm/download?path=${encodeURIComponent(e.path)}" download>Save</a>`}
        <button type="button" class="fm-act-btn" data-action="fm-rename" data-path="${esc(e.path)}" data-crit="${e.is_critical ? "1" : "0"}">Rename</button>
        <button type="button" class="fm-act-btn" data-action="fm-clip" data-op="copy" data-path="${esc(e.path)}">Copy</button>
        <button type="button" class="fm-act-btn" data-action="fm-clip" data-op="move" data-path="${esc(e.path)}">Move</button>
        <button type="button" class="fm-act-btn fm-act-danger" data-action="fm-delete" data-path="${esc(e.path)}" data-crit="${e.is_critical ? "1" : "0"}">Delete</button>
      </div>`;
    if (fmState.viewMode === "grid") {
      return `
        <div class="fm-grid-card ${e.is_critical ? "fm-crit-card" : ""}">
          <div class="fm-grid-top" data-action="${e.is_dir ? "fm-nav" : "fm-open"}" data-path="${esc(e.path)}">
            ${fmIconSvg(e)}
            <span class="fm-lbl-badge ${badgeCls}">${esc(e.label)}</span>
          </div>
          <div class="fm-grid-name" title="${esc(e.name)}" data-action="${e.is_dir ? "fm-nav" : "fm-open"}" data-path="${esc(e.path)}">${esc(e.name)}</div>
          <div class="fm-grid-meta">${e.is_dir ? "Folder" : fmFormatSize(e.size)} • ${esc(e.perms || "")}</div>
          ${actionsHtml}
        </div>`;
    }
    return `
      <tr class="fm-list-row">
        <td class="fm-name-cell" data-action="${e.is_dir ? "fm-nav" : "fm-open"}" data-path="${esc(e.path)}">
          ${fmIconSvg(e)}
          <span class="fm-fname">${esc(e.name)}</span>
          ${e.is_critical ? `<span class="fm-crit-dot" title="Critical system file">●</span>` : ""}
        </td>
        <td><span class="fm-lbl-badge ${badgeCls}">${esc(e.label)}</span></td>
        <td>${e.is_dir ? "Folder" : fmFormatSize(e.size)}</td>
        <td class="fm-hide-sm">${esc(e.mtime_str || "—")}</td>
        <td class="fm-hide-sm"><code>${esc(e.perms || "—")}</code></td>
        <td>${actionsHtml}</td>
      </tr>`;
  }).join("");

  const editorModal = fmState.editor ? `
    <div class="fm-editor-overlay">
      <div class="fm-editor-dialog">
        <div class="fm-editor-hdr">
          <div>
            <div style="font-weight:700;font-size:14.5px;">${esc(fmState.editor.name)}</div>
            <div style="font-size:11.5px;color:var(--text-secondary);font-family:var(--font-mono);">${esc(fmState.editor.path)}</div>
          </div>
          <button type="button" class="win-btn win-btn-sm" data-action="fm-close-editor">✕ Close</button>
        </div>
        ${fmState.editor.is_critical ? `
          <div class="fm-crit-banner">
            <strong>Critical System File Warning:</strong> Editing this file affects Ubuntu OS or server services. A <code>.bak</code> backup is automatically created before saving.
            <label style="display:flex;align-items:center;gap:7px;margin-top:6px;cursor:pointer;">
              <input type="checkbox" id="fmChkCritConfirm" data-action="fm-toggle-crit" ${fmState.editor.confirmed ? "checked" : ""} />
              <span>Unlock saving for this critical system file</span>
            </label>
          </div>` : ""}
        <textarea id="fmEditorTextarea" class="fm-editor-code" spellcheck="false">${esc(fmState.editor.content || "")}</textarea>
        <div class="fm-editor-ftr">
          <button type="button" class="win-btn win-btn-sm" data-action="fm-chmod" data-path="${esc(fmState.editor.path)}">Permissions (chmod)</button>
          <div style="display:flex;gap:8px;flex-wrap:wrap;">
            ${fmState.editor.has_bak ? `<button type="button" class="win-btn win-btn-sm" data-action="fm-restore-bak" data-path="${esc(fmState.editor.path)}">↺ Restore .bak</button>` : ""}
            <button type="button" class="win-btn win-btn-accent win-btn-sm" data-action="fm-save-editor" ${fmState.editor.is_critical && !fmState.editor.confirmed ? "disabled" : ""}>Save File</button>
          </div>
        </div>
      </div>
    </div>` : "";

  return `
    <div class="fm-shell">
      <aside class="fm-sidebar">
        <div class="fm-side-title">Disks &amp; Volumes</div>
        ${fmState.disks.map(d => `
          <button type="button" class="fm-disk-item ${fmState.path === d.mount ? "active" : ""}" data-action="fm-nav" data-path="${esc(d.mount)}">
            <div style="display:flex;justify-content:space-between;width:100%;font-size:12px;">
              <strong>${esc(d.mount)}</strong><span>${esc(d.pct)}</span>
            </div>
            <div class="fm-disk-bar"><div class="fm-disk-fill" style="width:${esc(d.pct)};"></div></div>
            <div style="font-size:10.5px;color:var(--text-secondary);">${esc(d.device)} • ${fmFormatSize(d.avail)} free</div>
          </button>`).join("")}
        <div class="fm-side-title" style="margin-top:12px;">Labels &amp; Quick Access</div>
        ${quickLabels.map(l => `
          <button type="button" class="fm-label-item ${fmState.label === l.id ? "active" : ""}" data-action="fm-label" data-label="${l.id}" data-path="${esc(l.path)}">
            <span class="fm-lbl-dot fm-lbl-${l.id}"></span><span>${esc(l.name)}</span>
          </button>`).join("")}
      </aside>

      <section class="fm-main">
        <div class="fm-toolbar">
          <div style="display:flex;gap:5px;align-items:center;">
            <button type="button" class="win-btn win-btn-sm" data-action="fm-nav" data-path="${esc(fmState.parent)}" title="Up one folder">↑</button>
            <button type="button" class="win-btn win-btn-sm" data-action="fm-refresh" title="Refresh">↻</button>
          </div>
          <div class="fm-breadcrumbs">${crumbHtml}</div>
          <div class="fm-toolbar-right">
            <input type="search" id="fmSearchInput" class="text-input fm-search" value="${esc(fmState.query)}" placeholder="Filter files..." />
            <select id="fmSortSelect" class="text-input" style="height:30px;font-size:12px;" data-action="fm-sort">
              <option value="name" ${fmState.sortBy === "name" ? "selected" : ""}>Name</option>
              <option value="size" ${fmState.sortBy === "size" ? "selected" : ""}>Size</option>
              <option value="mtime" ${fmState.sortBy === "mtime" ? "selected" : ""}>Modified</option>
              <option value="type" ${fmState.sortBy === "type" ? "selected" : ""}>Type</option>
            </select>
            <button type="button" class="win-btn win-btn-sm" data-action="fm-toggle-view">${fmState.viewMode === "list" ? "⊞ Grid" : "☰ List"}</button>
          </div>
        </div>

        <div class="fm-actionbar">
          <button type="button" class="win-btn win-btn-sm" data-action="fm-mkdir">+ New Folder</button>
          <button type="button" class="win-btn win-btn-sm" data-action="fm-touch">+ New File</button>
          <label class="win-btn win-btn-sm" style="cursor:pointer;">↑ Upload File<input type="file" id="fmUploadInput" style="display:none;" /></label>
          ${fmState.clipboard ? `<button type="button" class="win-btn win-btn-accent win-btn-sm" data-action="fm-paste">Paste (${esc(fmState.clipboard.op)}: ${esc(fmState.clipboard.src.split("/").pop())})</button>` : ""}
          <span style="margin-left:auto;font-size:11.5px;color:var(--text-secondary);">${fmState.entries.length} item(s)</span>
        </div>

        <div class="fm-content-scroll">
          ${fmState.viewMode === "grid"
            ? `<div class="fm-grid">${rowsHtml || `<div class="fm-empty">This folder is empty.</div>`}</div>`
            : `<table class="fm-table"><thead><tr><th>Name</th><th>Label</th><th>Size</th><th class="fm-hide-sm">Modified</th><th class="fm-hide-sm">Perms</th><th>Actions</th></tr></thead><tbody>${rowsHtml || `<tr><td colspan="6" class="fm-empty">This folder is empty.</td></tr>`}</tbody></table>`}
        </div>
      </section>
      ${editorModal}
    </div>`;
}