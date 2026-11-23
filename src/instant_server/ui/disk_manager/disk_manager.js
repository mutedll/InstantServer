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
    pendingTarget: null,
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

  function sizeOf(item) {
    return Number(item?.size_b ?? item?.size_bytes ?? 0);
  }

  function canSelect(item) {
    return Boolean(item?.can_select ?? item?.eligible_target);
  }

  function selectReason(item) {
    return item?.select_reason || item?.reason || "Not eligible";
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
    const total = Math.max(1, sizeOf(disk));
    const segs = [];
    for (const p of disk.partitions || []) {
      const sz = sizeOf(p);
      const pct = Math.max(2, (sz / total) * 100);
      segs.push(`<div class="part-seg ${segClass(p.fstype)}" style="width:${pct.toFixed(1)}%" title="${esc(p.path)} (${esc(p.fstype || "RAW")}, ${fmtGB(sz)})"></div>`);
    }
    for (const g of disk.free_gaps || []) {
      const sz = sizeOf(g);
      const pct = Math.max(2, (sz / total) * 100);
      segs.push(`<div class="part-seg seg-free" style="width:${pct.toFixed(1)}%" title="Unallocated (${fmtGB(sz)})"></div>`);
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
      const diskSize = sizeOf(d);
      const eligible = canSelect(d);
      const isSelected = state.selectedPath === d.path;
      const isOpen = state.expanded.has(d.path);
      const badge = d.protected
        ? `<span class="badge badge-warn">Installer USB &bull; Protected</span>`
        : eligible
          ? (isSelected ? `<span class="badge badge-accent">Selected</span>` : `<span class="badge badge-ok">Ready for Install</span>`)
          : `<span class="badge badge-muted">${esc(selectReason(d))}</span>`;

      const partCount = (d.partitions || []).length;
      const freeCount = (d.free_gaps || []).length;
      const ptLabel = d.pttype ? String(d.pttype).toUpperCase() : "RAW";

      let advHtml = "";
      if (!d.protected && isOpen) {
        const rows = [];
        for (const p of d.partitions || []) {
          const pSize = sizeOf(p);
          rows.push(`
            <div class="part-row">
              <div class="part-info">
                <strong>${esc(p.path)}</strong>
                <span>${esc(p.fstype || "RAW")} &bull; ${fmtGB(pSize)}${p.label ? " &bull; " + esc(p.label) : ""}</span>
              </div>
              <div class="part-btns">
                <button type="button" class="btn btn-sm" data-act="format" data-dev="${esc(p.path)}">Format</button>
                <button type="button" class="btn btn-sm" data-act="shrink" data-dev="${esc(p.path)}" data-size="${pSize}">Shrink</button>
                <button type="button" class="btn btn-sm" data-act="extend" data-dev="${esc(p.path)}" data-size="${pSize}">Extend</button>
                <button type="button" class="btn btn-sm" data-act="relabel" data-dev="${esc(p.path)}" data-label="${esc(p.label || "")}">Relabel</button>
                <button type="button" class="btn btn-sm btn-danger" data-act="delete_partition" data-dev="${esc(p.path)}">Delete</button>
              </div>
            </div>
          `);
        }
        for (const g of d.free_gaps || []) {
          const gSize = sizeOf(g);
          const gStart = Number(g.start_b ?? g.start_bytes ?? 1048576);
          rows.push(`
            <div class="part-row">
              <div class="part-info">
                <strong>Unallocated Space</strong>
                <span>${fmtGB(gSize)} free</span>
              </div>
              <div class="part-btns">
                ${canSelect(g) ? `<button type="button" class="btn btn-sm btn-primary" data-act="select-gap" data-disk="${esc(d.path)}" data-size="${gSize}">Install Here</button>` : ""}
                <button type="button" class="btn btn-sm" data-act="create_volume" data-disk="${esc(d.path)}" data-start="${gStart}" data-max="${gSize}">Create Volume</button>
              </div>
            </div>
          `);
        }
        advHtml = `
          <div class="adv-drawer">
            <div class="part-list">${rows.join("") || `<div class="part-row"><span class="part-info">No partitions on this drive.</span></div>`}</div>
            <div class="disk-tools-row">
              <button type="button" class="btn btn-sm" data-act="init-gpt" data-disk="${esc(d.path)}">Initialize GPT</button>
              <button type="button" class="btn btn-sm" data-act="init-mbr" data-disk="${esc(d.path)}">Initialize MBR</button>
              <button type="button" class="btn btn-sm btn-danger" data-act="wipe" data-disk="${esc(d.path)}">Wipe Disk</button>
            </div>
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
                <span>${fmtGB(diskSize)}</span>
                <span>${esc(String(d.tran || "Disk").toUpperCase())} &bull; ${esc(ptLabel)}</span>
                <span>${partCount} partition${partCount === 1 ? "" : "s"}${freeCount ? " &bull; " + freeCount + " free" : ""}</span>
              </div>
            </div>
            ${badge}
          </div>
          ${renderPartBar(d)}
          ${!d.protected ? `
            <div class="disk-actions">
              ${eligible ? `<button type="button" class="btn btn-primary" data-act="select" data-disk="${esc(d.path)}" ${state.busy ? "disabled" : ""}>Select for Install</button>` : `<span></span>`}
              <button type="button" class="btn btn-ghost btn-sm" data-act="toggle-adv" data-disk="${esc(d.path)}" ${state.busy ? "disabled" : ""}>${isOpen ? "Hide Partition Tools" : "Partition Tools"}</button>
            </div>
          ` : ""}
          ${advHtml}
        </article>
      `;
    }).join("");
  }

  function openConfirmModal(disk, mode = "whole", gapBytes = 0) {
    state.selectedPath = disk.path;
    state.pendingTarget = { disk, mode, gapBytes };
    renderDisks();
    const isGap = mode === "gap";
    const warnText = isGap
      ? `Ubuntu Server will install into the largest free space (${fmtGB(gapBytes)}) on <strong>${esc(disk.path)}</strong>. Existing partitions will be kept.`
      : `All existing partitions and data on <strong>${esc(disk.path)}</strong> will be erased. Your installer USB remains untouched.`;
    elModalBody.innerHTML = `
      <p style="margin:0;">You selected the following target for Ubuntu Server installation:</p>
      <div class="confirm-box">
        <div><strong>${esc(disk.model || disk.path)}</strong></div>
        <div style="margin-top:4px;font-size:12.5px;color:var(--muted);">${esc(disk.path)} &bull; ${fmtGB(sizeOf(disk))} &bull; ${isGap ? "Free Space Install" : "Full Disk (Direct, No LVM)"}</div>
      </div>
      <div class="warn-note">${warnText}</div>
    `;
    elModalConfirm.disabled = false;
    elModalConfirm.textContent = "Confirm & Start Installation";
    elModal.hidden = false;
  }

  function closeConfirmModal() {
    if (state.busy) return;
    elModal.hidden = true;
    state.pendingTarget = null;
  }

  function showHandoffScreen(disk) {
    state.installStarted = true;
    elModal.hidden = true;
    elMainView.hidden = true;
    document.getElementById("btnRefresh").hidden = true;
    document.getElementById("handoffTarget").textContent = `${disk.model || disk.path} (${disk.path} • ${fmtGB(sizeOf(disk))})`;
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
      if (disk) openConfirmModal(disk, "whole");
    } else if (act === "select-gap") {
      const disk = state.disks.find((d) => d.path === btn.dataset.disk);
      if (disk) openConfirmModal(disk, "gap", Number(btn.dataset.size || 0));
    } else if (act === "init-gpt" || act === "init-mbr") {
      const style = act === "init-gpt" ? "gpt" : "msdos";
      const dk = btn.dataset.disk;
      runAction({ action: "initialize", device: dk, style }, `Initialize ${dk} as ${style.toUpperCase()}? All existing partitions will be erased.`);
    } else if (act === "wipe") {
      const dk = btn.dataset.disk;
      runAction({ action: "wipe", device: dk }, `Erase ALL partitions and signatures on ${dk}?`);
    } else if (act === "delete_partition") {
      const dev = btn.dataset.dev;
      runAction({ action: "delete_partition", device: dev }, `Permanently delete partition ${dev}?`);
    } else if (act === "format") {
      const dev = btn.dataset.dev;
      const fs = window.prompt(`Filesystem for ${dev} (ext4, fat32, ntfs, exfat, btrfs, xfs):`, "ext4");
      if (!fs) return;
      const label = window.prompt("Optional volume label (leave blank for none):", "") || "";
      runAction({ action: "format", device: dev, fstype: fs.trim(), label }, `Format ${dev} as ${fs}? All data on ${dev} will be erased.`);
    } else if (act === "relabel") {
      const dev = btn.dataset.dev;
      const lbl = window.prompt(`New volume label for ${dev}:`, btn.dataset.label || "");
      if (lbl === null) return;
      runAction({ action: "relabel", device: dev, label: lbl.trim() });
    } else if (act === "create_volume") {
      const maxGB = Math.max(1, Math.floor(Number(btn.dataset.max) / 1073741824));
      const sz = window.prompt(`New volume size in GB (max ${maxGB} GB):`, String(maxGB));
      if (!sz || Number(sz) <= 0) return;
      const fs = window.prompt("Filesystem (ext4, fat32, ntfs, exfat, btrfs, xfs, unformatted):", "ext4") || "ext4";
      const label = window.prompt("Optional volume label:", "") || "";
      runAction({
        action: "create_volume",
        device: btn.dataset.disk,
        start_b: Number(btn.dataset.start),
        size_gb: Number(sz),
        fstype: fs.trim(),
        label,
      });
    } else if (act === "shrink" || act === "extend") {
      const curGB = (Number(btn.dataset.size) / 1073741824).toFixed(1);
      const ns = window.prompt(`${act === "shrink" ? "Shrink" : "Extend"} ${btn.dataset.dev} (current ${curGB} GB) — new size in GB:`, curGB);
      if (!ns || Number(ns) < 1) return;
      runAction({ action: act, device: btn.dataset.dev, new_size_gb: Number(ns) }, `${act === "shrink" ? "Shrink" : "Extend"} ${btn.dataset.dev} to ${ns} GB?`);
    }
  });

  elModalConfirm.addEventListener("click", async () => {
    const pending = state.pendingTarget;
    if (!pending || state.busy) return;
    state.busy = true;
    elModalConfirm.disabled = true;
    elModalConfirm.textContent = "Starting Installation...";
    try {
      const res = await api("/api/disk/action", {
        action: "select_target",
        device: pending.disk.path,
        mode: pending.mode || "whole",
      });
      if (res.ok) {
        showHandoffScreen(pending.disk);
      } else {
        state.busy = false;
        elModalConfirm.disabled = false;
        elModalConfirm.textContent = "Confirm & Start Installation";
        showToast("Error: " + (res.error || "Could not select drive"));
      }
    } catch {
      showHandoffScreen(pending.disk);
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
