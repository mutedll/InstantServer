// FILE: src/instant_server/ui/dashboard/dash_fm_events.js
// Event Handlers for Full File Manager & Extended Settings App

document.body.addEventListener("click", async (e) => {
  const el = e.target.closest("[data-action]");
  if (!el) return;
  const act = el.getAttribute("data-action");

  if (act === "fm-nav") {
    const p = el.getAttribute("data-path") || "/";
    await fmLoadDir(p, "all");
    return;
  }
  if (act === "fm-refresh") {
    await fmLoadDisks();
    await fmLoadDir(fmState.path, fmState.label);
    showToast("Refreshed folder");
    return;
  }
  if (act === "fm-label") {
    const lbl = el.getAttribute("data-label") || "all";
    const p = el.getAttribute("data-path") || fmState.path;
    await fmLoadDir(p, lbl);
    return;
  }
  if (act === "fm-toggle-view") {
    fmState.viewMode = fmState.viewMode === "list" ? "grid" : "list";
    renderDashboard();
    return;
  }
  if (act === "fm-open") {
    await fmOpenFile(el.getAttribute("data-path") || "");
    return;
  }
  if (act === "fm-close-editor") {
    fmState.editor = null;
    renderDashboard();
    return;
  }
  if (act === "fm-toggle-crit") {
    if (fmState.editor) {
      const chk = document.getElementById("fmChkCritConfirm");
      fmState.editor.confirmed = Boolean(chk && chk.checked);
      renderDashboard();
    }
    return;
  }
  if (act === "fm-save-editor") {
    if (!fmState.editor) return;
    const ta = document.getElementById("fmEditorTextarea");
    const content = ta ? ta.value : fmState.editor.content;
    const res = await callDashboardApi("/api/fm/write", "POST", {
      path: fmState.editor.path,
      content,
      confirmed: fmState.editor.confirmed
    });
    if (res && res.ok) {
      showToast("Saved " + fmState.editor.name + " (.bak backup created)");
      fmState.editor = null;
      await fmLoadDir(fmState.path, fmState.label);
    } else {
      showToast((res && res.error) || "Failed to save file");
    }
    return;
  }
  if (act === "fm-restore-bak") {
    const p = el.getAttribute("data-path") || "";
    const res = await callDashboardApi("/api/fm/restore_bak", "POST", { path: p });
    showToast(res && res.ok ? "Restored .bak backup" : ((res && res.error) || "No .bak found"));
    if (res && res.ok) await fmOpenFile(p);
    return;
  }
  if (act === "fm-mkdir") {
    const name = window.prompt("New folder name:");
    if (!name || !name.trim()) return;
    const full = (fmState.path === "/" ? "" : fmState.path) + "/" + name.trim();
    const res = await callDashboardApi("/api/fm/mkdir", "POST", { path: full });
    showToast(res && res.ok ? "Created folder " + name.trim() : ((res && res.error) || "Failed"));
    await fmLoadDir(fmState.path, fmState.label);
    return;
  }
  if (act === "fm-touch") {
    const name = window.prompt("New file name (e.g. notes.txt or config.yml):");
    if (!name || !name.trim()) return;
    const full = (fmState.path === "/" ? "" : fmState.path) + "/" + name.trim();
    const res = await callDashboardApi("/api/fm/touch", "POST", { path: full });
    showToast(res && res.ok ? "Created file " + name.trim() : ((res && res.error) || "Failed"));
    await fmLoadDir(fmState.path, fmState.label);
    return;
  }
  if (act === "fm-rename") {
    const src = el.getAttribute("data-path") || "";
    const isCrit = el.getAttribute("data-crit") === "1";
    const oldName = src.split("/").pop() || "";
    const nextName = window.prompt("Rename '" + oldName + "' to:", oldName);
    if (!nextName || !nextName.trim() || nextName.trim() === oldName) return;
    if (isCrit && !window.confirm("WARNING: '" + src + "' is a critical system file. Rename anyway?")) return;
    const res = await callDashboardApi("/api/fm/rename", "POST", { src, new_name: nextName.trim(), confirmed: true });
    showToast(res && res.ok ? "Renamed to " + nextName.trim() : ((res && res.error) || "Rename failed"));
    await fmLoadDir(fmState.path, fmState.label);
    return;
  }
  if (act === "fm-clip") {
    fmState.clipboard = { op: el.getAttribute("data-op") || "copy", src: el.getAttribute("data-path") || "" };
    showToast(`${fmState.clipboard.op === "move" ? "Cut" : "Copied"} ${fmState.clipboard.src.split("/").pop()} — navigate to destination and click Paste`);
    renderDashboard();
    return;
  }
  if (act === "fm-paste") {
    if (!fmState.clipboard) return;
    const ep = fmState.clipboard.op === "move" ? "/api/fm/move" : "/api/fm/copy";
    const res = await callDashboardApi(ep, "POST", { src: fmState.clipboard.src, dst_dir: fmState.path, confirmed: true });
    showToast(res && res.ok ? "Pasted into " + fmState.path : ((res && res.error) || "Paste failed"));
    fmState.clipboard = null;
    await fmLoadDir(fmState.path, fmState.label);
    return;
  }
  if (act === "fm-delete") {
    const p = el.getAttribute("data-path") || "";
    const isCrit = el.getAttribute("data-crit") === "1";
    const msg = isCrit
      ? `CRITICAL SYSTEM FILE WARNING!\nDeleting ${p} may break your server OS.\nAre you 100% sure you want to delete it?`
      : `Permanently delete ${p}?`;
    if (!window.confirm(msg)) return;
    const res = await callDashboardApi("/api/fm/delete", "POST", { path: p, confirmed: true });
    showToast(res && res.ok ? "Deleted " + p : ((res && res.error) || "Delete failed"));
    await fmLoadDir(fmState.path, fmState.label);
    return;
  }
  if (act === "fm-chmod") {
    const p = el.getAttribute("data-path") || "";
    const mode = window.prompt("Enter octal permissions (e.g. 644, 600, 755):", "644");
    if (!mode || !mode.trim()) return;
    const res = await callDashboardApi("/api/fm/chmod", "POST", { path: p, mode: mode.trim() });
    showToast(res && res.ok ? `Permissions set to ${mode.trim()}` : ((res && res.error) || "chmod failed"));
    return;
  }
  if (act === "set-dash-domain-preset") {
    const dom = el.getAttribute("data-domain") || ".local";
    state.settings.address_domain = dom;
    renderDashboard();
    return;
  }
  if (act === "open-disk-manager-tool") {
    window.open("/disk-manager", "_blank", "noopener");
    return;
  }
});

document.body.addEventListener("change", async (e) => {
  if (e.target && e.target.id === "fmSortSelect") {
    fmState.sortBy = e.target.value;
    await fmLoadDir(fmState.path, fmState.label);
  }
  if (e.target && e.target.id === "fmUploadInput" && e.target.files && e.target.files[0]) {
    const file = e.target.files[0];
    const reader = new FileReader();
    reader.onload = async () => {
      const b64 = String(reader.result || "").split(",")[1] || "";
      const res = await callDashboardApi("/api/fm/upload", "POST", {
        dst_dir: fmState.path,
        filename: file.name,
        data_base64: b64
      });
      showToast(res && res.ok ? "Uploaded " + file.name : ((res && res.error) || "Upload failed"));
      await fmLoadDir(fmState.path, fmState.label);
    };
    reader.readAsDataURL(file);
  }
});

document.body.addEventListener("input", (e) => {
  if (e.target && e.target.id === "fmSearchInput") {
    fmState.query = e.target.value;
    fmLoadDir(fmState.path, fmState.label);
  }
});