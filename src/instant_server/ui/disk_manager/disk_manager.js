(() => {
  const fmtGB = (b) => (Number(b || 0) / 1073741824).toFixed(1) + " GB";
  const esc = (s) =>
    String(s ?? "")
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");

  const state = {
    disks: [],
    selectedPath: null,
    expanded: new Set(),
    busy: false,
    installStarted: false,
    pendingDisk: null,
  };

  const elSkeleton = document.getElementById("skeletonView");
  const elDiskList = document.getElementById("diskList");
  const elMainView = document.getElementById("mainView");
  const elHandoff = document.getElementById("handoffView");
  const elModal = document.getElementById("confirmModal");
  const elModalBody = document.getElementById("confirmBody");
  const elModalConfirm = document.getElementById("btnModalConfirm");
  const elToast = document.getElementById("toast");
  let toastTimer = null;

  function showToast(msg) {
    elToast.textContent = msg;
    elToast.hidden = false;
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => { elToast.hidden = true; }, 4500);
  }

  async function api(url, body) {
    const opts = body
      ? { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) }
      : { method: "GET" };
    const r = await fetch(url, opts);
    return r.json();
  }

  function segClass(fstype) {
    const f = String(fstype || "").toLowerCase();
    if (f.includes("ext")) return "seg-ext";
    if (f.includes("fat")) return "seg-fat";
    if (f.includes("ntfs")) return "seg-ntfs";
    if (f.includes("lvm")) return "seg-lvm";
    return "seg-other";
  }

  function renderPartBar(disk) {
    const total = Math.max(1, Number(disk.size_bytes) || 1);
    const segs = [];
    for (const p of disk.partitions || []) {
      const pct = Math.max(2, (Number(p.size_bytes || 0) / total) * 100);
      segs.push(`<div class="part-seg ${segClass(p.fstype)}" style="width:${pct.toFixed(1)}%" title="${esc(p.path)} (${esc(p.fstype || "raw")}, ${fmtGB(p.size_bytes)})"></div>`);
    }
    for (const g of disk.free_gaps || []) {
      const pct = Math.max(2, (Number(g.size_bytes || 0) / total) * 100);
      segs.push(`<div class="part-seg seg-free" style="width:${pct.toFixed(1)}%" title="Free space (${fmtGB(g.size_bytes)})"></div>`);
    }
    if (!segs.length) {
      segs.push(`<div class="part-seg seg-free" style="width:100%" title="Unallocated"></div>`);
    }
    return `<div class="part-bar">${segs.join("")}</div>`;
  }

  function renderDisks() {
    if (!state.disks.length) {
      elDiskList.innerHTML = `<div class="disk-card"><p class="subtitle" style="margin:0;">No storage drives detected. Click Refresh to scan again.</p></div>`;
      return;
    }
    elDiskList.innerHTML = state.disks.map((d) => {
      const isSelected = state.selectedPath === d.path;
      const isOpen = state.expanded.has(d.path);
      const badge = d.protected
        ? `<span class="badge badge-warn">Installer USB &bull; Protected</span>`
        : d.eligible_target
          ? (isSelected ? `<span class="badge badge-accent">Selected</span>` : `<span class="badge badge-ok">Ready for Install</span>`)
          : `<span class="badge badge-muted">${esc(d.reason || "Not eligible")}</span>`;

      const partCount = (d.partitions || []).length;
      const freeCount = (d.free_gaps || []).length;

      let advHtml = "";
      if (!d.protected && isOpen) {
        const rows = [];
        for (const p of d.partitions || []) {
          rows.push(`
            <div class="part-row">
              <div class="part-info">
                <strong>${esc(p.path)}</strong>
                <span>${esc(p.fstype || "unformatted")} &bull; ${fmtGB(p.size_bytes)}${p.label ? " &bull; " + esc(p.label) : ""}</span>
              </div>
              <div class="part-btns">
                <button type="button" class="btn btn-sm" data-act="format" data-dev="${esc(p.path)}">Format</button>
                <button type="button" class="btn btn-sm" data-act="resize" data-dev="${esc(p.path)}" data-size="${p.size_bytes}">Resize</button>
                <button type="button" class="btn btn-sm btn-danger" data-act="delete" data-dev="${esc(p.path)}">Delete</button>
              </div>
            </div>
          `);
        }
        for (const g of d.free_gaps || []) {
          rows.push(`
            <div class="part-row">
              <div class="part-info">
                <strong>Unallocated Space</strong>
                <span>${fmtGB(g.size_bytes)} free</span>
              </div>
              <div class="part-btns">
                <button type="button" class="btn btn-sm" data-act="create" data-disk="${esc(d.path)}" data-start="${g.start_bytes}" data-max="${g.size_bytes}">Create Partition</button>
              </div>
            </div>
          `);
        }
        advHtml = `
          <div class="adv-drawer">
            <div class="part-list">${rows.join("") || `<div class="part-row"><span class="part-info">No partitions on this drive.</span></div>`}</div>
            <button type="button" class="btn btn-sm btn-danger" data-act="wipe" data-disk="${esc(d.path)}">Wipe Partition Table (GPT)</button>
          </div>
        `;
      }

      return `
        <article class="disk-card ${isSelected ? "selected" : ""} ${d.protected ? "protected" : ""}">
          <div class="disk-head">
            <div class="disk-title-wrap">
              <div class="disk-name">${esc(d.model || d.path)}</div>
              <div class="disk-meta">
                <span>${esc(d.path)}</span>
                <span>${fmtGB(d.size_bytes)}</span>
                <span>${esc(d.media || "Disk")}${d.tran ? " (" + esc(d.tran.toUpperCase()) + ")" : ""}</span>
                <span>${partCount} partition${partCount === 1 ? "" : "s"}${freeCount ? " &bull; " + freeCount + " free region" : ""}</span>
              </div>
            </div>
            ${badge}
          </div>
          ${renderPartBar(d)}
          ${!d.protected ? `
            <div class="disk-actions">
              ${d.eligible_target ? `<button type="button" class="btn btn-primary" data-act="select" data-disk="${esc(d.path)}" ${state.busy ? "disabled" : ""}>Select for Install</button>` : `<span></span>`}
              <button type="button" class="btn btn-ghost btn-sm" data-act="toggle-adv" data-disk="${esc(d.path)}" ${state.busy ? "disabled" : ""}>${isOpen ? "Hide Partition Tools" : "Partition Tools"}</button>
            </div>
          ` : ""}
          ${advHtml}
        </article>
      `;
    }).join("");
  }

  function openConfirmModal(disk) {
    state.selectedPath = disk.path;
    state.pendingDisk = disk;
    renderDisks();
    elModalBody.innerHTML = `
      <p style="margin:0;">You selected the following drive for Ubuntu Server installation:</p>
      <div class="confirm-box">
        <div><strong>${esc(disk.model || disk.path)}</strong></div>
        <div style="margin-top:4px;font-size:12.5px;color:var(--muted);">${esc(disk.path)} &bull; ${fmtGB(disk.size_bytes)} &bull; ${esc(disk.media || "Disk")}</div>
      </div>
      <div class="warn-note">All existing partitions and data on <strong>${esc(disk.path)}</strong> will be erased. Your installer USB remains untouched.</div>
    `;
    elModalConfirm.disabled = false;
    elModalConfirm.textContent = "Confirm & Start Installation";
    elModal.hidden = false;
  }

  function closeConfirmModal() {
    if (state.busy) return;
    elModal.hidden = true;
    state.pendingDisk = null;
  }

  function showHandoffScreen(disk) {
    state.installStarted = true;
    elModal.hidden = true;
    elMainView.hidden = true;
    document.getElementById("btnRefresh").hidden = true;
    document.getElementById("handoffTarget").textContent = `${disk.model || disk.path} (${disk.path} • ${fmtGB(disk.size_bytes)})`;
    document.getElementById("handoffDashUrl").textContent = `${window.location.protocol}//${window.location.host}`;
    elHandoff.hidden = false;
  }

  async function runAction(body, confirmMsg) {
    if (state.busy || state.installStarted) return;
    if (confirmMsg && !window.confirm(confirmMsg)) return;
    state.busy = true;
    renderDisks();
    try {
      const res = await api("/api/disk/action", body);
      showToast(res.ok ? (res.output || "Completed") : ("Error: " + (res.error || "Action failed")));
      await loadDisks();
    } catch {
      showToast("Connection interrupted. Click Refresh to check drive status.");
    } finally {
      state.busy = false;
      renderDisks();
    }
  }

  async function loadDisks() {
    if (state.installStarted) return;
    elSkeleton.hidden = false;
    elDiskList.hidden = true;
    try {
      const data = await api("/api/disk/list");
      state.disks = data.disks || [];
      renderDisks();
    } catch {
      elDiskList.innerHTML = `<div class="disk-card"><p class="subtitle" style="margin:0;">Unable to reach Disk Management service. If installation already started, you can safely close this tab.</p></div>`;
    } finally {
      elSkeleton.hidden = true;
      elDiskList.hidden = false;
    }
  }

  elDiskList.addEventListener("click", (e) => {
    const btn = e.target.closest("button[data-act]");
    if (!btn || state.busy || state.installStarted) return;
    const act = btn.dataset.act;
    if (act === "toggle-adv") {
      const dk = btn.dataset.disk;
      if (state.expanded.has(dk)) state.expanded.delete(dk); else state.expanded.add(dk);
      renderDisks();
    } else if (act === "select") {
      const disk = state.disks.find((d) => d.path === btn.dataset.disk);
      if (disk) openConfirmModal(disk);
    } else if (act === "wipe") {
      const dk = btn.dataset.disk;
      runAction({ action: "wipe_disk", disk: dk }, `Erase ALL partitions on ${dk} and create a blank GPT table?`);
    } else if (act === "delete") {
      const dev = btn.dataset.dev;
      runAction({ action: "delete", partition: dev }, `Permanently delete partition ${dev}?`);
    } else if (act === "format") {
      const dev = btn.dataset.dev;
      const fs = window.prompt(`Filesystem for ${dev} (ext4, xfs, btrfs, fat32, ntfs, swap):`, "ext4");
      if (!fs) return;
      const label = window.prompt("Optional volume label (leave blank for none):", "") || "";
      runAction({ action: "format", partition: dev, fstype: fs, label }, `Format ${dev} as ${fs}? All data on ${dev} will be erased.`);
    } else if (act === "create") {
      const maxGB = Math.floor(Number(btn.dataset.max) / 1073741824);
      const sz = window.prompt(`New partition size in GB (max ${maxGB} GB):`, String(maxGB));
      if (!sz || Number(sz) <= 0) return;
      const fs = window.prompt("Filesystem (ext4, xfs, btrfs, fat32, ntfs, swap, none):", "ext4") || "ext4";
      runAction({
        action: "create",
        disk: btn.dataset.disk,
        start_bytes: Number(btn.dataset.start),
        end_bytes: Number(btn.dataset.start) + Math.floor(Number(sz) * 1073741824),
        fstype: fs,
      });
    } else if (act === "resize") {
      const curGB = Math.floor(Number(btn.dataset.size) / 1073741824);
      const ns = window.prompt(`New size in GB for ${btn.dataset.dev} (current ${curGB} GB):`, String(curGB));
      if (!ns || Number(ns) <= 0) return;
      runAction({ action: "resize", partition: btn.dataset.dev, new_size_mb: Math.floor(Number(ns) * 1024) }, `Resize ${btn.dataset.dev} to ${ns} GB?`);
    }
  });

  elModalConfirm.addEventListener("click", async () => {
    const disk = state.pendingDisk;
    if (!disk || state.busy) return;
    state.busy = true;
    elModalConfirm.disabled = true;
    elModalConfirm.textContent = "Starting Installation...";
    try {
      const res = await api("/api/disk/action", { action: "select_target", disk: disk.path, mode: "wipe_entire_disk" });
      if (res.ok) {
        showHandoffScreen(disk);
      } else {
        state.busy = false;
        elModalConfirm.disabled = false;
        elModalConfirm.textContent = "Confirm & Start Installation";
        showToast("Error: " + (res.error || "Could not select drive"));
      }
    } catch {
      // Server may have already closed after accepting the handoff
      showHandoffScreen(disk);
    }
  });

  document.getElementById("btnModalCancel").addEventListener("click", closeConfirmModal);
  document.getElementById("btnModalClose").addEventListener("click", closeConfirmModal);
  document.getElementById("btnRefresh").addEventListener("click", loadDisks);
  document.getElementById("btnCloseTab").addEventListener("click", () => {
    window.close();
    setTimeout(() => { document.getElementById("closeTabHint").hidden = false; }, 200);
  });

  loadDisks();
})();