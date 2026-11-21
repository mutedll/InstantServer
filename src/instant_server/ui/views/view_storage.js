// FILE: src/instant_server/ui/view_storage.js
    function buildStorageSection() {
      const modes = [
        { id: "dual_boot", icon: "BAL", title: "Dual-Boot alongside existing OS", desc: "Keeps your computer's original Windows or Linux system intact (15 GB – 64 TB)", badge: "Safe Default" },
        { id: "interactive_gui_disk", icon: "", title: "Choose target disk on target laptop (Interactive Disk Picker GUI)", desc: "Displays an interactive disk selector on the target laptop screen (and on http://<laptop-ip>:8088) during boot so you can pick which internal drive to install onto", badge: "Interactive Disk Picker" },
        { id: "auto_erase", icon: "CLN", title: "Fresh Server Install (Erase main drive)", desc: "Formats the main internal drive and dedicates 100% of space to the server", badge: "Requires confirmation" },
        { id: "reinstall_keep_data", icon: "REC", title: "Reinstall OS while retaining server data (/DATA & apps preserved)", desc: "Backs up existing /DATA, photos, Nextcloud, and app configs before reinstalling Ubuntu LTS and restores them automatically", badge: "Preserves /DATA" },
        { id: "reinstall_wipe_data", icon: "", title: "Reinstall OS without retaining server data (Factory reset wipe)", desc: "Erases all existing server data, apps, and partitions for a clean factory reset", badge: "Requires WIPE confirmation" },
        { id: "offline_updater_usb", icon: "PKG", title: "Offline Updater USB (For local-only / air-gapped servers)", desc: "Creates an offline update USB stick that updates the Web Control Center, scripts, and app stacks without internet", badge: "Zero-Internet Update" },
        { id: "repair_rescue_usb", icon: "SSH", title: "Emergency Repair & Password Reset USB", desc: "Resets forgotten admin/SSH/dashboard password to the password set in Tab 1, repairs GRUB/EFI boot, and runs e2fsck without erasing files", badge: "Does not erase drive" },
        { id: "secondary_drive", icon: "DSK", title: "Secondary drive only", desc: "Installs onto a second internal drive and leaves the primary drive untouched", badge: "" }
      ];

      const displaySizeLabel = state.dual_boot_Allocation_gb >= 1024 && (state.dual_boot_Allocation_gb % 1024 === 0)
        ? `${state.dual_boot_Allocation_gb / 1024} TB (${state.dual_boot_Allocation_gb} GB)`
        : `${state.dual_boot_Allocation_gb} GB`;
      const sizePresets = [
        { gb: 64, label: "64 GB" },
        { gb: 250, label: "250 GB" },
        { gb: 500, label: "500 GB" },
        { gb: 1024, label: "1 TB" },
        { gb: 2048, label: "2 TB" },
        { gb: 4096, label: "4 TB" },
        { gb: 8192, label: "8 TB" },
        { gb: 16384, label: "16 TB" }
      ];
      const modeConfigCard = state.install_mode === "dual_boot" ? `
        <div class="win-card" style="flex-direction:column;align-items:stretch;gap:10px;">
          <div style="display:flex;align-items:center;justify-content:space-between;gap:14px;flex-wrap:wrap;">
            <div class="card-left">
              <div class="card-text">
                <div class="card-title" id="lblDualBootSize">Server partition size (${esc(displaySizeLabel)})</div>
                <div class="card-desc" id="lblStorageValidation" style="${state.storageInputError ? "color:var(--danger);font-weight:600;" : ""}">
                  ${esc(state.storageInputError || "Type any size from 15 GB up to 64 TB (65,536 GB) or pick a preset below")}
                </div>
              </div>
            </div>
            <div class="storage-input-group">
              <input
                type="number"
                id="inputStorageAllocation"
                class="storage-num-input ${state.storageInputError ? "invalid" : ""}"
                min="1"
                max="65536"
                step="any"
                data-field="storage_custom_input"
                value="${esc(state.storageInputRaw)}"
                aria-label="Server partition size"
              />
              <div class="mode-segmented-switch" role="group" aria-label="Storage Unit">
                <button type="button" class="mode-seg-btn ${state.storageUnit === "GB" ? "active-standard" : ""}" data-action="set-storage-unit" data-unit="GB">GB</button>
                <button type="button" class="mode-seg-btn ${state.storageUnit === "TB" ? "active-standard" : ""}" data-action="set-storage-unit" data-unit="TB">TB</button>
              </div>
            </div>
          </div>
          <div class="storage-presets-row">
            ${sizePresets.map(p => `
              <button type="button" class="storage-preset-pill ${Number(state.dual_boot_Allocation_gb) === p.gb ? "active" : ""}" data-action="set-storage-preset" data-gb="${p.gb}">
                ${esc(p.label)}
              </button>
            `).join("")}
          </div>
        </div>
      ` : state.install_mode === "auto_erase" ? `
        <div class="notice-banner ${state.confirm_erase_main_drive ? "notice-banner-warning" : ""}" id="bannerAutoErase">
          <div style="font-weight:600;">Warning: This will erase all files on the target computer's main internal drive</div>
          <label style="display:flex;align-items:center;gap:8px;cursor:pointer;margin-top:4px;">
            <input type="checkbox" data-action="toggle-confirm-erase" ${state.confirm_erase_main_drive ? "checked" : ""} style="accent-color:var(--danger);width:16px;height:16px;" />
            <span>I know what I'm doing — erase the target computer's main internal drive</span>
          </label>
        </div>
      ` : state.install_mode === "reinstall_wipe_data" ? `
        <div class="notice-banner notice-banner-warning" id="bannerReinstallWipe">
          <div style="font-weight:600;color:var(--danger);">Factory Reset Warning: This will permanently erase all existing server data (/DATA), containers, and backups</div>
          <label style="display:flex;align-items:center;gap:8px;cursor:pointer;margin-top:4px;">
            <input type="checkbox" data-action="toggle-confirm-wipe-server-data" ${state.confirm_wipe_server_data ? "checked" : ""} style="accent-color:var(--danger);width:16px;height:16px;" />
            <span>I confirm erasing all existing server data (WIPE-SERVER-DATA) and performing a clean factory reinstall</span>
          </label>
        </div>
      ` : state.install_mode === "reinstall_keep_data" ? `
        <div class="notice-banner" style="border-color:var(--success);">
          <div style="font-weight:600;">Safe OS Reinstall Mode Active</div>
          <div style="font-size:12px;">Existing <code>/DATA</code> (Photos, Nextcloud, Minecraft worlds, AppData) and <code>/opt/instant-server</code> are backed up prior to OS formatting and restored automatically after Ubuntu LTS finishes installing.</div>
        </div>
      ` : state.install_mode === "interactive_gui_disk" ? `
        <div class="notice-banner" style="border-color:var(--accent);">
          <div style="font-weight:600;">Interactive Target Disk Picker GUI Mode</div>
          <div style="font-size:12px;">When you boot the target laptop from this USB drive, an interactive disk selection menu appears on the laptop screen (and on <code>http://&lt;laptop-ip&gt;:8088</code>) so you can choose which internal disk to install onto while your USB stick stays hidden and protected.</div>
        </div>
      ` : state.install_mode === "offline_updater_usb" ? `
        <div class="notice-banner" style="border-color:var(--accent);">
          <div style="font-weight:600;">Offline Air-Gapped Updater USB Mode Active</div>
          <div style="font-size:12px;">Plug this USB into your running local server (or boot from it) to update the Web Dashboard, hardening scripts, and Docker Compose app stacks without needing internet access.</div>
        </div>
      ` : state.install_mode === "repair_rescue_usb" ? `
        <div class="notice-banner" style="border-color:var(--success);">
          <div style="font-weight:600;">Emergency Repair &amp; Password Reset Mode Active</div>
          <div style="font-size:12px;">Booting from this USB automatically resets the password for <strong>${esc(state.username || "admin")}</strong> to the password entered in Tab 1, runs <code>e2fsck -fy</code> on ext4 partitions, and clears UFW/Fail2ban lockouts without erasing your data.</div>
        </div>
      ` : "";

      const usbListHtml = state.usbDevices.map(dev => {
        const selected = state.selectedUsbId === dev.device_id && dev.triple_lock_passed;
        return `
          <div class="win-card ${dev.triple_lock_passed ? "clickable" : ""} ${selected ? "selected-accent" : ""}"
               ${dev.triple_lock_passed ? `data-action="select-usb" data-usb="${esc(dev.device_id)}"` : ""}
               style="${!dev.triple_lock_passed ? "opacity:0.55;" : ""}">
            <div class="card-left">
              <div class="card-text">
                <div class="card-title">
                  ${esc(dev.model)} (${dev.size_gb} GB)
                  <span class="badge ${dev.triple_lock_passed ? "badge-success" : "badge-danger"}">
                    ${dev.triple_lock_passed ? (dev.bus === "virtual" ? "Ready (Virtual Disk)" : "Ready") : "Protected System Drive"}
                  </span>
                </div>
                <div class="card-desc">${esc(dev.device_node)} ${dev.mountpoints && dev.mountpoints.length ? "(" + esc(dev.mountpoints.join(", ")) + ")" : ""} ${dev.block_reasons && dev.block_reasons.length ? "• " + esc(dev.block_reasons[0]) : ""}</div>
              </div>
            </div>
            <div class="card-right">
              ${dev.triple_lock_passed
                ? `<input type="radio" name="usb_sel" ${selected ? "checked" : ""} style="accent-color:var(--accent);width:17px;height:17px;pointer-events:none;" />`
                : `<span style="font-size:12px;color:var(--text-tertiary);">Blocked</span>`}
            </div>
          </div>
        `;
      }).join("");

      const advHtml = `
        <div class="win-card">
          <div class="card-left">
            <div class="card-text">
              <div class="card-title">USB partition scheme &amp; target firmware</div>
              <div class="card-desc">Automatically set from your computer age answer above, or override manually</div>
            </div>
          </div>
          <div class="card-right">
            ${renderComboBox("usb_partition_scheme", [
              { value: "auto", label: "Universal Hybrid (UEFI + Legacy BIOS)" },
              { value: "gpt_uefi", label: "GPT Partition Scheme for UEFI" },
              { value: "mbr_bios", label: "MBR Partition Scheme for BIOS / CSM" }
            ])}
          </div>
        </div>

        <div class="win-card">
          <div class="card-left">
            <div class="card-text">
              <div class="card-title">Ubuntu Server ISO source</div>
              <div class="card-desc">Pull official ISO automatically from Ubuntu, or use a downloaded .iso file on your PC</div>
            </div>
          </div>
          <div class="card-right" style="flex-wrap:wrap;">
            ${renderComboBox("iso_source_mode", [
              { value: "official_dl", label: "Auto-Download Official Ubuntu ISO" },
              { value: "local_file", label: "Use Local .iso File on This PC" }
            ])}
            <input
              id="customIsoPathInput"
              class="win-input"
              style="min-width:240px; ${state.iso_source_mode === "local_file" ? "" : "display:none;"}"
              data-field="custom_iso_path"
              value="${esc(state.custom_iso_path)}"
              placeholder="C:\\Downloads\\ubuntu-26.04.1-live-server-amd64.iso"
            />
          </div>
        </div>

        <div class="win-card">
          <div class="card-left">
            <div class="card-text">
              <div class="card-title">Ubuntu Server LTS release</div>
              <div class="card-desc">Official Ubuntu Server LTS version</div>
            </div>
          </div>
          <div class="card-right">
            ${renderComboBox("ubuntu_release", [
              { value: "26.04", label: "Ubuntu Server 26.04.1 LTS (Default — Latest LTS)" },
              { value: "24.04", label: "Ubuntu Server 24.04.5 LTS" },
              { value: "22.04", label: "Ubuntu Server 22.04.5 LTS" }
            ])}
          </div>
        </div>

        <div class="win-card">
          <div class="card-left">
            <div class="card-text">
              <div class="card-title">Target filesystem layout</div>
              <div class="card-desc">Partition structure on the hard drive</div>
            </div>
          </div>
          <div class="card-right">
            ${renderComboBox("storage_layout", [
              { value: "direct", label: "Standard ext4 (Recommended)" },
              { value: "lvm", label: "LVM Volume Group" },
              { value: "btrfs", label: "Btrfs Snapshots" }
            ])}
          </div>
        </div>
      `;

      return `
        <div class="tab-flow-stack">
          <div class="section-label">1. About the target computer (If you don't know, leave on "Auto-detect")</div>
          <div class="card-stack">
            <div class="win-card">
              <div class="card-left">
                <div class="card-text">
                  <div class="card-title">What type of computer is this?</div>
                  <div class="card-desc">Automatically enables battery &amp; lid care on laptops, or low-power desktop settings on Mini-PCs and Desktops</div>
                </div>
              </div>
              <div class="card-right">
                ${renderComboBox("device_form_factor", [
                  { value: "auto_detect", label: "Auto-detect (Laptop, Desktop, or Mini-PC)" },
                  { value: "laptop_ewaste", label: "Old laptop (Battery & lid protection + low-RAM boost)" },
                  { value: "laptop_modern", label: "Modern or gaming laptop" },
                  { value: "mini_pc_n100", label: "Mini-PC / Intel N100 (Ultra-low-power desktop mode)" },
                  { value: "desktop_pc", label: "Desktop PC or Tower (No battery settings needed)" },
                  { value: "nas_multibay", label: "Multi-drive NAS or Workstation" }
                ])}
              </div>
            </div>

            <div class="win-card">
              <div class="card-left">
                <div class="card-text">
                  <div class="card-title">How old is the computer?</div>
                  <div class="card-desc">Selects the right boot loader (UEFI GPT vs Legacy BIOS MBR). "I don't know" creates a universal hybrid boot USB.</div>
                </div>
              </div>
              <div class="card-right">
                ${renderComboBox("laptop_age_hint", [
                  { value: "unknown", label: "I don't know (Universal Hybrid Boot)" },
                  { value: "modern_uefi", label: "2013 or newer (UEFI + GPT)" },
                  { value: "legacy_bios", label: "2012 or older (Legacy BIOS + MBR)" }
                ])}
              </div>
            </div>

            <div class="win-card">
              <div class="card-left">
                <div class="card-text">
                  <div class="card-title">How much RAM does it have?</div>
                  <div class="card-desc">Tunes memory compression (zram) so even 2 GB – 4 GB computers run containers smoothly</div>
                </div>
              </div>
              <div class="card-right">
                ${renderComboBox("laptop_ram_hint", [
                  { value: "unknown", label: "I don't know (Auto-tune memory)" },
                  { value: "low_2_4gb", label: "2 GB – 4 GB RAM (Low-RAM zram boost)" },
                  { value: "standard_8gb_plus", label: "8 GB RAM (Standard Homelab & Minecraft)" },
                  { value: "high_end_16gb_gpu", label: "16 GB+ RAM / GPU (Higher-End — Local AI & Gaming)" }
                ])}
              </div>
            </div>
          </div>

          <div class="section-label">2. Target hard drive installation mode</div>
          <div class="card-stack">
            ${modes.map(m => {
              const active = state.install_mode === m.id;
              return `
                <div class="win-card clickable ${active ? "selected-accent" : ""}" data-action="select-install-mode" data-value="${m.id}">
                  <div class="card-left">
                    <div class="card-text">
                      <div class="card-title">
                        ${esc(m.title)}
                        ${m.badge ? `<span class="badge ${m.id === "auto_erase" ? "badge-warning" : m.id === "repair_rescue_usb" ? "badge-success" : "badge-accent"}">${esc(m.badge)}</span>` : ""}
                      </div>
                      <div class="card-desc">${esc(m.desc)}</div>
                    </div>
                  </div>
                  <div class="card-right">
                    <input type="radio" name="inst_mode" ${active ? "checked" : ""} style="accent-color:var(--accent);width:17px;height:17px;pointer-events:none;" />
                  </div>
                </div>
              `;
            }).join("")}
            ${modeConfigCard}
          </div>

          <div style="display:flex;align-items:center;justify-content:space-between;gap:8px;flex-wrap:wrap;">
            <div class="section-label" style="margin:0;">3. Select USB drive to format &amp; make bootable</div>
            <div style="display:flex;align-items:center;gap:8px;flex-wrap:wrap;">
              <button type="button" class="win-btn" data-action="boot-usb-next-restart" ${state.flashing ? "disabled" : ""} title="Configure this PC to boot directly into the USB drive on next restart without entering BIOS or Boot Menu">
                Boot USB on Next Restart
              </button>
              <button type="button" class="win-btn" data-action="reset-usb-drive" ${state.resettingUsb || state.flashing ? "disabled" : ""} title="Clear USB write-protection and format as a normal full-capacity FAT32 drive">
                ${state.resettingUsb ? "Resetting USB..." : "Reset USB to Normal Drive"}
              </button>
              <button type="button" class="win-btn" data-action="scan-usb" ${state.scanningUsb || state.flashing ? "disabled" : ""}>
                ${state.scanningUsb ? "Scanning..." : "Refresh Drives"}
              </button>
            </div>
          </div>
          <div class="card-stack">
            ${usbListHtml}
          </div>

          <div class="notice-banner notice-banner-warning" id="bannerUsbFormatWarn">
            <div style="font-weight:600;">USB Format Warning: All existing files on the selected USB stick will be erased</div>
            <label style="display:flex;align-items:center;gap:8px;cursor:pointer;margin-top:2px;">
              <input type="checkbox" data-action="toggle-confirm-usb-erase" ${state.confirm_usb_erase ? "checked" : ""} style="accent-color:var(--accent);width:16px;height:16px;" />
              <span>Format the selected USB drive and write the bootable Ubuntu Server ISO + pre-selected apps</span>
            </label>
          </div>

          <div class="win-card" style="border-color:${state.skip_beginner_checks ? "var(--warning)" : "var(--border-subtle)"};">
            <div class="card-left">
              <div class="card-text">
                <div class="card-title">I know what I'm doing (Skip beginner checks &amp; confirmations)</div>
                <div class="card-desc">Bypasses the "Instant Server already installed" safeguard prompt on USB boot and extra erase/wipe confirmation checkboxes</div>
              </div>
            </div>
            <div class="card-right">
              <label style="display:flex;align-items:center;gap:8px;cursor:pointer;font-size:12.5px;font-weight:600;">
                <input type="checkbox" data-action="toggle-skip-beginner-checks" ${state.skip_beginner_checks ? "checked" : ""} style="accent-color:var(--warning);width:17px;height:17px;" />
                <span>Skip checks</span>
              </label>
            </div>
          </div>

          
          <div class="win-card" style="flex-direction:column;align-items:stretch;gap:10px;">
            <div style="display:flex;align-items:center;justify-content:space-between;gap:10px;flex-wrap:wrap;">
              <div class="card-left">
                <div class="card-text">
                  <div class="card-title">How to Boot From USB &amp; Reset Forgotten Passwords (Quick Guide)</div>
                  <div class="card-desc">Press the Boot Menu key repeatedly right after powering on the target laptop/PC • Sleep is automatically inhibited during install</div>
                </div>
              </div>
              <span class="badge badge-accent">No Monitor Needed After Boot Selection</span>
            </div>
            <div style="display:grid;grid-template-columns:repeat(auto-fit,minmax(210px,1fr));gap:8px;font-size:12px;">
              <div style="padding:8px 10px;border-radius:8px;background:var(--card-bg-secondary);border:1px solid var(--border-subtle);">
                <strong>Lenovo / ThinkPad / Dell / Acer:</strong> Tap <code>F12</code> repeatedly at power-on (BIOS Setup: <code>F2</code> or <code>F1</code>)
              </div>
              <div style="padding:8px 10px;border-radius:8px;background:var(--card-bg-secondary);border:1px solid var(--border-subtle);">
                <strong>HP (Pavilion / EliteBook / Omen):</strong> Tap <code>F9</code> or <code>Esc → F9</code> at power-on (BIOS Setup: <code>F10</code>)
              </div>
              <div style="padding:8px 10px;border-radius:8px;background:var(--card-bg-secondary);border:1px solid var(--border-subtle);">
                <strong>ASUS / MSI / Desktop Boards:</strong> Tap <code>F8</code> or <code>Esc</code> (ASUS), <code>F11</code> (MSI), <code>F12</code> (Gigabyte)
              </div>
              <div style="padding:8px 10px;border-radius:8px;background:var(--card-bg-secondary);border:1px solid var(--border-subtle);">
                <strong>Forgot Server Password?</strong> Pick <em>Emergency Repair &amp; Password Reset USB</em> above, set a new password in Tab 1, flash, and boot once
              </div>
            </div>
          </div>

          ${renderExpander("storage", "Advanced options", advHtml)}
        </div>
      `;
    }