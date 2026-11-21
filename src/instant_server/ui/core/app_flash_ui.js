// FILE: src/instant_server/ui/app_flash_ui.js
    async function verifySelectedUsbDrive() {
      if (state.flashing || state.verifyingUsb) return;
      const selectedDev = state.usbDevices.find(d => d.device_id === state.selectedUsbId && d.triple_lock_passed)
        || state.usbDevices.find(d => d.triple_lock_passed);
      const targetNode = selectedDev ? selectedDev.device_node : state.selectedUsbId;
      if (!targetNode) {
        showToast("Select a USB drive first to verify its bootable partitions.");
        return;
      }
      state.verifyingUsb = true;
      renderApp();
      showToast(`Verifying bootable partitions on ${targetNode}...`);
      const res = await callBackend("verify-usb", "POST", { device: targetNode });
      state.verifyingUsb = false;
      if (res && Array.isArray(res.checks)) {
        res.checks.forEach(c => {
          state.flashLogs.push(`[VERIFY] ${c}`);
        });
      }
      if (res && res.ok) {
        showToast(res.message || `Verified ${targetNode}: all bootable partitions and FAT clean bits passed!`);
      } else {
        showToast((res && (res.error || res.message)) ? (res.error || res.message) : `Verification failed on ${targetNode}`);
      }
      renderApp();
    }

    async function resetSelectedUsbDrive() {
      if (state.flashing || state.resettingUsb) return;
      const selectedDev = state.usbDevices.find(d => d.device_id === state.selectedUsbId && d.triple_lock_passed)
        || state.usbDevices.find(d => d.triple_lock_passed);
      const targetNode = selectedDev ? selectedDev.device_node : state.selectedUsbId;
      if (!targetNode || targetNode === "usb_sim_none") {
        showToast("Plug in and select a removable USB drive first");
        return;
      }
      state.resettingUsb = true;
      renderApp();
      showToast("Clearing write-protection and formatting " + targetNode + " as FAT32...");
      const res = await callBackend("reset-usb", "POST", { device: targetNode });
      state.resettingUsb = false;
      if (res && res.ok) {
        showToast(res.message || ("Reset " + targetNode + " to a normal FAT32 drive"));
        await scanUsbDrives(true);
      } else if (res && res.elevated_relaunch) {
        showToast(res.error || "Relaunching as Administrator...");
      } else {
        showToast((res && res.error) ? res.error : ("Failed to reset " + targetNode));
      }
      renderApp();
    }

    async function scanUsbDrives(silent = false) {
      state.scanningUsb = true;
      if (!silent) renderApp();

      const liveList = await callBackend("usb", "GET");
      if (Array.isArray(liveList) && liveList.length > 0) {
        state.usbDevices = liveList;
      } else if (state.usbDevices.length === 0) {
        state.usbDevices = [
          {
            device_id: "usb0",
            device_node: "/dev/sdb",
            model: "SanDisk Ultra USB 3.0",
            size_gb: 32.0,
            bus: "usb",
            removable: true,
            mountpoints: ["D:\\"],
            triple_lock_passed: true,
            block_reasons: []
          },
          {
            device_id: "sys0",
            device_node: "System Disk 0",
            model: "Internal OS Drive",
            size_gb: 512.0,
            bus: "nvme",
            removable: false,
            mountpoints: ["C:\\"],
            triple_lock_passed: false,
            block_reasons: ["System OS drive is protected"]
          }
        ];
      }

      const firstValid = state.usbDevices.find(d => d.triple_lock_passed);
      if (firstValid && !state.selectedUsbId) {
        state.selectedUsbId = firstValid.device_id;
      }
      state.scanningUsb = false;
      if (!silent) {
        showToast("USB drives refreshed");
        renderApp();
      }
    }

    async function saveConfigurationAndOpenHome() {
      const payload = serializeProfilePayload();
      const res = await callBackend("save-config", "POST", payload);
      try {
        localStorage.setItem("instant_server_saved_config", JSON.stringify(payload));
      } catch (_e) {}
      state.configSavedLocally = true;
      if (res && res.path) {
        state.savedConfigPath = res.path;
      }
      if (state.viewMode !== "settings") {
        shouldAnimateTab = true;
      }
      state.viewMode = "settings";
      state.settingsTab = "system";
      showToast("Saved to Instant_Server_Data/instant-server-config.json");
      renderApp();
    }

    function checkIncompleteRequirements() {
      const missing = [];
      if (!state.hostname || !/^[a-z0-9][a-z0-9-]{0,61}$/i.test(state.hostname.trim())) {
        missing.push("Enter a valid Server Name (letters, numbers, hyphens).");
      }
      if (!state.username || !/^[a-z_][a-z0-9_-]{0,31}$/i.test(state.username.trim())) {
        missing.push("Enter a valid Admin Username.");
      }
      if (!state.password && !state.ssh_authorized_key.trim()) {
        missing.push("Set an Admin Password (or add an SSH key in Network advanced options).");
      }
      if (state.wifi_ssid.trim() && state.wifi_password.length > 0 && state.wifi_password.length < 8) {
        missing.push("Wi-Fi password must be at least 8 characters.");
      }
      if (state.install_mode === "dual_boot") {
        const gbVal = Number(state.dual_boot_Allocation_gb);
        if (!Number.isFinite(gbVal) || gbVal < 15 || gbVal > 65536 || state.storageInputError) {
          missing.push(state.storageInputError || "Enter a valid Server Partition Size between 15 GB and 64 TB (65,536 GB).");
        }
      }
      if (state.install_mode === "auto_erase" && !state.confirm_erase_main_drive && !state.skip_beginner_checks) {
        missing.push("Confirm 'I know what I'm doing' for Erase Main Drive mode, or switch to Dual-Boot.");
      }
      if (state.install_mode === "reinstall_wipe_data" && !state.confirm_wipe_server_data && !state.skip_beginner_checks) {
        missing.push("Confirm wiping all existing server data for Factory Reinstall mode, or switch to Reinstall While Retaining Data.");
      }
      if (state.cluster_mode !== "standalone" && state.cluster_storage_policy === "wipe_secondary_expand" && !state.confirm_wipe_secondary && !state.skip_beginner_checks) {
        missing.push("Confirm erasing the second computer's files, or switch to 'Keep all files on both computers'.");
      }
      if (state.iso_source_mode === "local_file" && !state.custom_iso_path.trim()) {
        missing.push("Enter the file path to your local Ubuntu Server .iso file, or switch to Automatic Official ISO Download.");
      }
      const usb = state.usbDevices.find(d => d.device_id === state.selectedUsbId && d.triple_lock_passed);
      if (!usb) {
        missing.push("Select an approved removable USB flash drive in Laptop & USB.");
      }
      if (!state.confirm_usb_erase && !state.skip_beginner_checks) {
        missing.push("Confirm that all existing data on the selected USB drive will be erased and formatted.");
      }
      return missing;
    }

    function buildAsciiTerminalBar(pct, label, detail) {
      const clamped = Math.max(0, Math.min(100, Number(pct) || 0));
      const width = 22;
      const filled = Math.round((clamped / 100) * width);
      const bar = "█".repeat(filled) + "░".repeat(Math.max(0, width - filled));
      const pctStr = clamped.toFixed(1).padStart(5, " ") + "%";
      return detail
        ? `⟳ ${label}: [${bar}] ${pctStr} │ ${detail}`
        : `⟳ ${label}: [${bar}] ${pctStr}`;
    }

    function updateSmoothedFlashETA(rawSec) {
      const sec = Number(rawSec);
      if (!Number.isFinite(sec) || sec <= 0 || sec > 21600) return;
      const now = Date.now();
      state.etaSamplesCount = (state.etaSamplesCount || 0) + 1;
      const elapsedSinceStart = state.flashStartEpochMs ? (now - state.flashStartEpochMs) / 1000 : 0;

      // Suppress wild initial burst/setup estimates during the first 5 seconds or first 3 samples
      if (elapsedSinceStart < 5 || state.etaSamplesCount < 3 || (state.flashProgress || 0) < 8) {
        state.smoothedEtaSec = state.smoothedEtaSec ? (state.smoothedEtaSec * 0.5 + sec * 0.5) : sec;
        state.flashEtaText = "ETA: calculating...";
        return;
      }

      if (!state.smoothedEtaSec || state.smoothedEtaSec <= 0) {
        state.smoothedEtaSec = sec;
      } else {
        // Adaptive EMA: faster convergence early on, steady smoothing once settled
        const alpha = state.etaSamplesCount <= 8 ? 0.35 : 0.18;
        const bounded = Math.min(sec, state.smoothedEtaSec * 1.25 + 5);
        state.smoothedEtaSec = state.smoothedEtaSec * (1 - alpha) + bounded * alpha;
      }
      if (!state.lastEtaRenderMs || now - state.lastEtaRenderMs >= 1000) {
        state.lastEtaRenderMs = now;
        const rounded = Math.max(1, Math.round(state.smoothedEtaSec));
        const remMin = String(Math.floor(rounded / 60)).padStart(2, "0");
        const remSec = String(rounded % 60).padStart(2, "0");
        state.flashEtaText = `ETA: ~${remMin}:${remSec} remaining`;
      }
    }

    function scrollTerminalToBottomIfSticky() {
      const logBox = document.getElementById("flashLogBox");
      if (!logBox) return;
      if (state.logAutoScroll) {
        logBox._programmaticScroll = true;
        logBox.scrollTop = logBox.scrollHeight;
        setTimeout(() => { logBox._programmaticScroll = false; }, 40);
      }
      const resumeBtn = document.getElementById("btnResumeAutoScroll");
      if (resumeBtn) {
        resumeBtn.style.display = state.logAutoScroll ? "none" : "inline-flex";
      }
    }

    function syncFlashingLockState() {
      document.body.classList.toggle("is-flashing-locked", !!state.flashing);
      const lockBadge = document.getElementById("flashConfigLockedBadge");
      if (lockBadge) {
        lockBadge.style.display = state.flashing ? "inline-flex" : "none";
      }
    }

    function updateFlashPanelInPlace() {
      syncFlashingLockState();
      const panel = document.getElementById("flashPanelCard");
      if (!panel) {
        renderApp();
        return;
      }

      const titleEl = document.getElementById("flashStatusTitle");
      const nextTitle = state.flashStatusText || "Preparing bootable USB...";
      if (titleEl && titleEl.textContent !== nextTitle) {
        titleEl.textContent = nextTitle;
      }

      const badgeEl = document.getElementById("flashBadgePct");
      if (badgeEl) {
        const nextPctTxt = `${state.flashProgress}%`;
        if (badgeEl.textContent !== nextPctTxt) badgeEl.textContent = nextPctTxt;
        const nextBadgeCls = "badge " + (
          state.flashProgress === 100
            ? "badge-success"
            : state.flashCancelled
            ? "badge-warning"
            : "badge-accent"
        );
        if (badgeEl.className !== nextBadgeCls) badgeEl.className = nextBadgeCls;
      }
      const etaEl = document.getElementById("flashEtaBadge");
      if (etaEl) {
        if (state.flashEtaText && state.flashing) {
          if (etaEl.style.display !== "inline-flex") etaEl.style.display = "inline-flex";
          if (etaEl.textContent !== state.flashEtaText) etaEl.textContent = state.flashEtaText;
        } else if (etaEl.style.display !== "none") {
          etaEl.style.display = "none";
          etaEl.textContent = "";
        }
      }

      const fillEl = document.getElementById("flashProgressFill");
      if (fillEl) {
        const nextWidth = `${state.flashProgress}%`;
        if (fillEl.style.width !== nextWidth) fillEl.style.width = nextWidth;
      }

      const hdrActions = document.getElementById("flashPanelHeaderActions");
      if (hdrActions) {
        if (state.flashing) {
          const pauseLbl = state.flashPaused ? `Resume (${state.flashProgress}%)` : "Pause";
          const pauseCls = state.flashPaused ? "win-btn win-btn-sm win-btn-accent" : "win-btn win-btn-sm";
          let pauseBtn = hdrActions.querySelector("[data-action='pause-resume-flash']");
          if (!pauseBtn) {
            hdrActions.innerHTML = `
              <button type="button" class="${pauseCls}" data-action="pause-resume-flash">${pauseLbl}</button>
              <button type="button" class="win-btn win-btn-sm win-btn-danger" data-action="cancel-flash">Cancel</button>
            `;
          } else {
            if (pauseBtn.textContent !== pauseLbl) pauseBtn.textContent = pauseLbl;
            if (pauseBtn.className !== pauseCls) pauseBtn.className = pauseCls;
          }
        } else if (hdrActions.innerHTML !== "") {
          hdrActions.innerHTML = "";
        }
      }

      const stageWrap = document.getElementById("flashStageStepsRow");
      if (stageWrap) {
        const nextStageHtml = renderFlashPipelineStagesHtml();
        if (stageWrap._lastHtml !== nextStageHtml) {
          stageWrap._lastHtml = nextStageHtml;
          stageWrap.innerHTML = nextStageHtml;
        }
      }

      document.querySelectorAll("[data-action='start-flash']").forEach(btn => {
        btn.disabled = state.flashing;
        btn.textContent = state.flashing
          ? (state.flashPaused ? `Paused at ${state.flashProgress}%` : "Flashing Bootable USB...")
          : "Confirm & Flash Bootable USB";
      });
      document.querySelectorAll("[data-action='pause-resume-flash']").forEach(btn => {
        btn.textContent = state.flashPaused ? `Resume (${state.flashProgress}%)` : "Pause";
      });
      document.querySelectorAll(".footer-cancel-flash-wrap").forEach(wrap => {
        wrap.style.display = state.flashing ? "inline-flex" : "none";
      });

      const linesContainer = document.getElementById("flashLogLines");
      if (linesContainer) {
        const existingNodes = linesContainer.children;
        while (existingNodes.length > state.flashLogs.length) {
          linesContainer.removeChild(linesContainer.lastChild);
        }
        for (let i = 0; i < state.flashLogs.length; i++) {
          if (i < existingNodes.length) {
            if (existingNodes[i].textContent !== state.flashLogs[i]) {
              existingNodes[i].textContent = state.flashLogs[i];
            }
          } else {
            const row = document.createElement("div");
            row.className = "term-log-line";
            row.textContent = state.flashLogs[i];
            linesContainer.appendChild(row);
          }
        }
        state.renderedLogCount = state.flashLogs.length;
      }

      const liveRow = document.getElementById("flashTerminalLiveRow");
      if (liveRow) {
        if (state.activeProgressLine) {
          if (liveRow.style.display !== "block") liveRow.style.display = "block";
          if (liveRow.textContent !== state.activeProgressLine) liveRow.textContent = state.activeProgressLine;
        } else if (liveRow.style.display !== "none") {
          liveRow.style.display = "none";
          liveRow.textContent = "";
        }
      }

      scrollTerminalToBottomIfSticky();
    }

    function renderFlashPipelineStagesHtml() {
      const curIdx = state.flashProgress >= 100
        ? 6
        : (Number(state.flashStageIndex) || (state.flashing ? 1 : 0));
      const stages = [
        { idx: 1, label: "1. Config & Checks" },
        { idx: 2, label: "2. Stage Apps" },
        { idx: 3, label: "3. Ubuntu ISO" },
        { idx: 4, label: "4. USB Write" },
        { idx: 5, label: "5. Verify & Ready" }
      ];
      return stages.map(st => {
        const isDone = curIdx > st.idx;
        const isActive = curIdx === st.idx && (state.flashing || state.flashPaused);
        const badgeCls = isDone ? "badge-success" : (isActive ? (state.flashPaused ? "badge-warning" : "badge-accent") : "");
        const prefix = isDone ? "✓ " : (isActive ? (state.flashPaused ? "⏸ " : "⟳ ") : "");
        return `<span class="badge ${badgeCls}" style="font-size:11px;padding:3px 8px;">${prefix}${esc(st.label)}</span>`;
      }).join("");
    }

    async function togglePauseResumeFlashing() {
      if (!state.flashing && !state.flashPaused) return;
      if (state.pauseResumeBusy) return;
      state.pauseResumeBusy = true;
      state.flashOpSeq = (state.flashOpSeq || 0) + 1;
      try {
        if (!state.flashPaused) {
          state.flashPaused = true;
          state.activeProgressLine = "⏸ PAUSED │ Progress saved as .part";
          state.flashStatusText = `Paused at ${state.flashProgress}% (saved as .part). Click Resume to continue.`;
          state.flashLogs.push(`[PAUSED] Paused at ${state.flashProgress}% (saved as .part).`);
          updateFlashPanelInPlace();
          showToast("Paused (saved as .part)");
          await callBackend("pause", "POST");
        } else {
          state.flashPaused = false;
          state.flashing = true;
          state.lastResumeEpoch = Date.now();
          state.activeProgressLine = `⟳ Resuming from ${state.flashProgress}% (.part)...`;
          state.flashStatusText = `Resuming from ${state.flashProgress}% (saved as .part)...`;
          state.flashLogs.push(`[RESUMED] Continuing from ${state.flashProgress}% (.part).`);
          updateFlashPanelInPlace();
          showToast("Resumed from .part");
          await callBackend("resume", "POST");
          state.flashOpSeq = (state.flashOpSeq || 0) + 1;
        }
      } finally {
        state.pauseResumeBusy = false;
        updateFlashPanelInPlace();
      }
    }

    async function addCustomWebAppFromInput() {
      const raw = (state.customWebAppInput || "").trim();
      if (!raw) {
        showToast("Paste a Docker image, docker run command, or GitHub/Docker Hub URL");
        return;
      }
      let parsed = null;
      const res = await callBackend("custom-app", "POST", { raw_input: raw });
      if (res && res.ok && res.app) {
        parsed = res.app;
      } else {
        let image = raw;
        let port = 8080;
        const mPort = raw.match(/-p\s+(\d{2,5}):(\d{2,5})/) || raw.match(/:(\d{2,5})\b/);
        if (mPort) port = Number(mPort[1]);
        if (raw.includes("hub.docker.com/r/")) {
          image = raw.split("hub.docker.com/r/")[1].split(/[/?#]/).slice(0, 2).join("/");
        } else if (raw.includes("github.com/")) {
          const parts = raw.split("github.com/")[1].split(/[/?#]/).slice(0, 2);
          image = "ghcr.io/" + parts.join("/").toLowerCase() + ":latest";
        } else if (raw.startsWith("docker run")) {
          const tokens = raw.split(/\s+/);
          image = tokens[tokens.length - 1];
        }
        if (!image.includes(":")) image += ":latest";
        const baseName = image.split("/").pop().split(":")[0].replace(/[^a-z0-9_-]/gi, "-").toLowerCase() || "custom-app";
        parsed = {
          id: "custom_" + baseName.replace(/-/g, "_"),
          title: baseName.replace(/[-_]/g, " ").replace(/\b\w/g, c => c.toUpperCase()),
          category: "Security & Tools",
          image,
          port,
          volumes: [`/DATA/AppData/${baseName}:/data`],
          description: `Custom container (${image}) on port ${port}.`
        };
      }
      if (!state.custom_apps.some(a => a.id === parsed.id)) {
        state.custom_apps.push(parsed);
      }
      if (!APP_STORE_CATALOG.some(a => a.id === parsed.id)) {
        APP_STORE_CATALOG.unshift({
          id: parsed.id,
          title: parsed.title,
          category: "Security & Tools",
          recommended: true,
          tag: `Custom • Port ${parsed.port}`,
          ram: "128 MB",
          hardware_tier: "old_friendly",
          image: parsed.image,
          port: parsed.port,
          desc: parsed.description
        });
      }
      if (!state.selected_apps.includes(parsed.id)) {
        state.selected_apps.push(parsed.id);
      }
      state.customWebAppInput = "";
      showToast(`Added & selected ${parsed.title} (${parsed.image})`);
      renderApp();
    }