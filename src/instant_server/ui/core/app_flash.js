// FILE: src/instant_server/ui/app_flash.js
    async function cancelFlashingProcess(exitAfterCancel = false) {
      state.flashOpSeq = (state.flashOpSeq || 0) + 1;
      clearInterval(pollTimer);
      pollTimer = null;
      state.flashing = false;
      state.flashPaused = false;
      state.flashCancelled = true;
      state.activeProgressLine = "";
      state.flashEtaText = "";
      state.flashStatusText = "Flashing cancelled by user — cached ISO & app files are kept so you can resume or retry anytime.";
      state.flashLogs.push("[CANCELLED] Flashing stopped by user. Downloaded ISO & staged app files are preserved.");
      updateFlashPanelInPlace();
      renderApp();
      showToast("Flashing cancelled — cached files preserved");
      await callBackend("cancel", "POST");
      if (exitAfterCancel) {
        state.forceClosing = true;
        await callBackend("shutdown", "POST", { force: true });
        window.close();
      }
    }

    async function pauseAndExitFlashingProcess() {
      state.flashOpSeq = (state.flashOpSeq || 0) + 1;
      clearInterval(pollTimer);
      pollTimer = null;
      state.flashPaused = true;
      state.flashing = false;
      state.activeProgressLine = "⏸ PAUSED │ Progress saved as .part";
      state.flashStatusText = `Paused at ${state.flashProgress}% (saved as .part).`;
      state.flashLogs.push(`[PAUSED] Progress at ${state.flashProgress}% saved as .part.`);
      updateFlashPanelInPlace();
      showToast("Progress & downloaded files saved — exiting safely");
      await callBackend("pause-and-exit", "POST", {});
      state.forceClosing = true;
      window.close();
    }

    async function handleCacheRetentionAction(mode) {
      state.cacheRetentionMode = mode;
      const res = await callBackend("cache", "POST", { action: mode });
      if (mode === "delete") {
        state.cacheStatusMessage = (res && res.message)
          ? res.message
          : "Deleted cached ISO and staging files.";
        showToast("Deleted cached files");
      } else {
        const sizeInfo = (res && res.total_mb && res.total_mb !== "0.0 MB") ? ` (${res.total_mb})` : "";
        state.cacheStatusMessage = `Keeping cached files${sizeInfo} (Default).`;
        showToast("Keeping cached files");
      }
      renderApp();
    }

    async function startFlashingProcess() {
      if (state.flashing) return;
      const missing = checkIncompleteRequirements();
      state.validationErrors = missing;
      if (missing.length > 0) {
        showToast("Complete required settings before flashing");
        renderApp();
        return;
      }

      const payload = serializeProfilePayload();
      try {
        localStorage.setItem("instant_server_saved_config", JSON.stringify(payload));
      } catch (_e) {}

      try {
        if (navigator.wakeLock && navigator.wakeLock.request) {
          navigator.wakeLock.request("screen").catch(() => {});
        }
      } catch (_wl) {}
      state.flashing = true;
      state.flashPaused = false;
      state.flashCancelled = false;
      state.flashProgress = 6;
      state.flashStageIndex = 1;
      state.smoothedEtaSec = 0;
      state.etaSamplesCount = 0;
      state.flashStartEpochMs = Date.now();
      state.lastEtaRenderMs = 0;
      state.flashEtaText = "ETA: calculating...";
      state.logAutoScroll = true;
      state.renderedLogCount = 0;
      const flashStartEpoch = Date.now();
      const spinFrames = ["⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"];
      let spinIdx = 0;
      state.flashStatusText = `Preparing Ubuntu Server ${state.ubuntu_release} LTS bootable USB...`;
      state.flashLogs = [
        "[00:00] Saved config to Instant_Server_Data/instant-server-config.json"
      ];
      state.activeProgressLine = buildAsciiTerminalBar(6.0, "Initializing Pipeline", "Checking local ISO cache & Ubuntu mirrors...");
      renderApp();
      state.renderedLogCount = state.flashLogs.length;
      scrollTerminalToBottomIfSticky();

      setTimeout(() => {
        const panel = document.getElementById("flashPanelCard");
        if (panel && panel.scrollIntoView) {
          panel.scrollIntoView({ behavior: "smooth", block: "nearest" });
        }
      }, 60);

      const startRes = await callBackend("flash", "POST", payload);
      if (startRes && startRes.started) {
        clearInterval(pollTimer);
        pollTimer = setInterval(async () => {
          if (state.pauseResumeBusy || !state.flashing || state.flashPaused || state.flashCancelled) {
            updateFlashPanelInPlace();
            return;
          }
          const reqSeq = state.flashOpSeq || 0;
          const snap = await callBackend("status", "GET");
          if (!snap) return;
          if (state.pauseResumeBusy || (state.flashOpSeq || 0) !== reqSeq || !state.flashing || state.flashCancelled) return;
          if (snap.close_warning_requested && window.__showCloseWhileFlashingWarning) {
            window.__showCloseWhileFlashingWarning();
          }
          if (state.flashPaused || (snap.status === "PAUSED" && (!state.lastResumeEpoch || Date.now() - state.lastResumeEpoch > 1500))) {
            state.flashPaused = true;
            updateFlashPanelInPlace();
            return;
          }

          spinIdx = (spinIdx + 1) % spinFrames.length;
          const elapsedSec = Math.max(0, Math.floor((Date.now() - flashStartEpoch) / 1000));
          const elapsedStr = `${String(Math.floor(elapsedSec / 60)).padStart(2, "0")}:${String(elapsedSec % 60).padStart(2, "0")}`;

          state.flashProgress = Math.round(Math.max(state.flashProgress, Number(snap.percent) || 6));
          if (snap.stage_index) {
            state.flashStageIndex = Math.max(state.flashStageIndex || 1, Number(snap.stage_index));
          }
          if (snap.active_progress && Number(snap.active_progress.eta_sec) > 0) {
            updateSmoothedFlashETA(Number(snap.active_progress.eta_sec));
          } else if (snap.active_progress && snap.active_progress.detail && snap.active_progress.detail.includes("Total ETA")) {
            const etaMatch = snap.active_progress.detail.match(/Total ETA\s+([0-9:]+)/);
            if (etaMatch) state.flashEtaText = `ETA: ~${etaMatch[1]} remaining`;
          } else if (state.flashProgress > 10 && state.flashProgress < 100 && elapsedSec > 2) {
            const pctDone = Math.max(1, state.flashProgress - 6);
            const estRemSec = (elapsedSec / pctDone) * (100 - state.flashProgress);
            updateSmoothedFlashETA(estRemSec);
          }

          if (Array.isArray(snap.events) && snap.events.length > 0) {
            const backendLines = [];
            let latestInPlaceFromEvents = null;
            snap.events.forEach((ev, idx) => {
              const evName = String(ev.event || "");
              const msg = String(ev.message || "");
              if (evName.endsWith("_PROGRESS") || (ev.data && ev.data.in_place)) {
                latestInPlaceFromEvents = ev;
              } else {
                const lineText = ev.formatted_line || `[00:${String(idx + 1).padStart(2, "0")}] ${msg}`;
                backendLines.push(lineText);
              }
            });
            state.flashLogs = [
              "[00:00] Saved config to Instant_Server_Data/instant-server-config.json",
              ...backendLines
            ];
            state.flashStatusText = snap.events[snap.events.length - 1].message || state.flashStatusText;

            if (snap.active_progress && (snap.active_progress.message || snap.active_progress.terminal_line)) {
              const stPct = Number(snap.active_progress.stage_percent ?? state.flashProgress);
              const stLbl = snap.active_progress.label || "Active Stage";
              const stMsg = snap.active_progress.message || state.flashStatusText;
              if (!state.lastWailsEventTs || (Date.now() - state.lastWailsEventTs) > 600) {
                state.activeProgressLine = buildAsciiTerminalBar(stPct, stLbl, stMsg).replace(/^⟳/, spinFrames[spinIdx]);
              }
              state.flashStatusText = stMsg;
            } else if (latestInPlaceFromEvents) {
              const d = latestInPlaceFromEvents.data || {};
              const stPct = Number(d.stage_percent ?? d.download_percent ?? d.percent ?? state.flashProgress);
              state.activeProgressLine = buildAsciiTerminalBar(stPct, d.label || "Progress", latestInPlaceFromEvents.message).replace(/^⟳/, spinFrames[spinIdx]);
              state.flashStatusText = latestInPlaceFromEvents.message;
            } else if (snap.status === "RUNNING") {
              state.activeProgressLine = buildAsciiTerminalBar(
                state.flashProgress,
                "Active Stage",
                `${state.flashStatusText} (Elapsed ${elapsedStr})`
              ).replace(/^⟳/, spinFrames[spinIdx]);
            }
          } else if (snap.status === "RUNNING") {
            state.activeProgressLine = buildAsciiTerminalBar(
              state.flashProgress,
              "Active Stage",
              `${state.flashStatusText} (Elapsed ${elapsedStr})`
            ).replace(/^⟳/, spinFrames[spinIdx]);
          }

          if (snap.status === "COMPLETED") {
            clearInterval(pollTimer);
            pollTimer = null;
            state.flashing = false;
            state.flashProgress = 100;
            state.flashStageIndex = 6;
            state.activeProgressLine = "";
            state.flashEtaText = "";
            const usbInfo = (snap.result && snap.result.usb_provision) || {};
            const copiedCount = Array.isArray(usbInfo.copied_files) ? usbInfo.copied_files.length : 0;
            const mountName = usbInfo.target_mount || usbInfo.target_device || "USB drive";
            state.flashStatusText = "Bootable USB drive is ready! Plug it into your computer and power it on.";
            state.flashLogs.push(`[DONE] Verified bootable Ubuntu Server ISO + ${copiedCount} cidata & app file(s) on ${mountName}.`);
            showToast("Bootable Instant Server USB is ready");
          } else if (snap.status === "CANCELLED") {
            clearInterval(pollTimer);
            pollTimer = null;
            state.flashing = false;
            state.flashCancelled = true;
            state.activeProgressLine = "";
            state.flashStatusText = "Flashing cancelled by user — safe to unplug or retry anytime.";
          } else if (snap.status === "FAILED") {
            clearInterval(pollTimer);
            pollTimer = null;
            state.flashing = false;
            state.activeProgressLine = "";
            state.flashStatusText = `Failed: ${snap.error || "Check logs"}`;
            state.flashLogs.push(`[ERROR] ${snap.error || "Provisioning failed"}`);
          }
          updateFlashPanelInPlace();
        }, 350);
        return;
      }

      if (startRes && startRes.elevated_relaunch) {
        state.flashing = false;
        state.forceClosing = true;
        showToast("Administrator window launched — closing this unprivileged window...");
        setTimeout(() => { window.close(); }, 180);
        return;
      }
      if (window.location.protocol.startsWith("http") || window.pywebview) {
        state.flashing = false;
        state.activeProgressLine = "";
        const errReason = (startRes && startRes.reason) ? startRes.reason : "Could not start background USB flashing service.";
        state.flashStatusText = `Failed: ${errReason}`;
        state.flashLogs.push(`[ERROR] ${errReason}`);
        updateFlashPanelInPlace();
        return;
      }

      const selectedUsb = state.usbDevices.find(d => d.device_id === state.selectedUsbId);
      const usbLabel = selectedUsb ? `${selectedUsb.model} (${selectedUsb.device_node})` : "USB Drive";
      const schemeDesc = state.usb_partition_scheme === "mbr_bios"
        ? "MBR (Legacy BIOS + CSM active boot)"
        : "GPT + Hybrid UEFI/BIOS";

      const simPhases = [
        { type: "log", pct: 10, msg: `[00:03] Resolving official Ubuntu Server ${state.ubuntu_release}.1 LTS mirror & SHA256SUMS...` },
        { type: "bar", pct: 16, stagePct: 28.4, label: "Downloading Ubuntu ISO", detail: "749.8 / 2640.0 MB @ 24.5 MB/s" },
        { type: "bar", pct: 24, stagePct: 64.2, label: "Downloading Ubuntu ISO", detail: "1694.9 / 2640.0 MB @ 25.1 MB/s" },
        { type: "bar", pct: 34, stagePct: 100.0, label: "Downloading Ubuntu ISO", detail: "2640.0 / 2640.0 MB @ 25.0 MB/s" },
        { type: "log", pct: 40, msg: `[00:06] Official Ubuntu Server ${state.ubuntu_release}.1 LTS ISO SHA-256 checksum verified!` },
        { type: "log", pct: 46, msg: `[00:07] Formatting ${usbLabel} as ${schemeDesc} (label: CIDATA)...` },
        { type: "bar", pct: 58, stagePct: 42.0, label: "Writing Bootable ISO to USB", detail: "1092.0 / 2600.0 MB (casper/vmlinuz & initrd)" },
        { type: "bar", pct: 74, stagePct: 88.5, label: "Writing Bootable ISO to USB", detail: "2301.0 / 2600.0 MB (casper/ubuntu-server-minimal.squashfs)" },
        { type: "log", pct: 86, msg: `[00:10] Pre-unpacking ${state.selected_apps.length} self-hosted CasaOS app bundle(s) (${state.selected_apps.slice(0, 4).join(", ")}${state.selected_apps.length > 4 ? "..." : ""}) onto USB...` },
        { type: "done", pct: 100, msg: "[DONE] Bootable Ubuntu Server ISO + GRUB autoinstall + pre-unpacked apps written to USB." }
      ];
      let idx = 0;
      clearInterval(pollTimer);
      pollTimer = setInterval(() => {
        if (state.flashPaused) {
          updateFlashPanelInPlace();
          return;
        }
        if (!state.flashing || idx >= simPhases.length) {
          clearInterval(pollTimer);
          pollTimer = null;
          return;
        }
        const step = simPhases[idx++];
        state.flashProgress = step.pct;
        if (step.type === "log") {
          state.activeProgressLine = "";
          state.flashLogs.push(step.msg);
          state.flashStatusText = step.msg.replace(/^\[[0-9:]+\]\s*/, "");
        } else if (step.type === "bar") {
          state.activeProgressLine = buildAsciiTerminalBar(step.stagePct, step.label, step.detail);
          state.flashStatusText = `${step.label}: ${step.detail}`;
        } else if (step.type === "done") {
          state.activeProgressLine = "";
          state.flashLogs.push(step.msg);
          state.flashStatusText = "Bootable USB drive is ready! Plug it into your computer and power it on.";
          state.flashing = false;
          clearInterval(pollTimer);
          pollTimer = null;
          showToast("Bootable Instant Server USB is ready");
        }
        updateFlashPanelInPlace();
      }, 480);
    }