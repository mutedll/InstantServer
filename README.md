# ⚡ Instant Server

**Instant Server is a native, cross-platform provisioning engine and zero-touch Ubuntu LTS homelab builder powered by Go and Wails.**

It effortlessly transforms spare laptops, Mini-PCs, desktop towers, and multi-computer setups into a self-healing personal cloud, media server, network ad-blocker, offline Wikipedia box, or local AI/gaming host—**completely eliminating the need for terminal commands or manual router port-forwarding.**

---

## ✨ Key Capabilities

### 1. Native Go + Wails Architecture

* **Zero Runtime Extraction:** Compiles `app_ui.html`, `server_dashboard.html`, and `laptop_hardening.sh` directly into the binary's read-only data segment via `//go:embed`. Serves the entire interface straight from RAM without extracting temporary files to disk.
* **4096-Byte Page-Aligned Direct DMA Engine (`usb/`):**
  * **Linux (`usb/flasher_linux.go`):** Allocates 4096-byte page-aligned 4 MB buffers (`AllocPageAlignedBuffer`) and streams directly to block devices using `syscall.O_DIRECT | syscall.O_EXCL` (avoiding the `O_SYNC` per-block stall mirage), followed by a final hardware fsync, `BLKFLSBUF`, and `BLKRRPART`.
  * **macOS (`usb/flasher_darwin.go`):** Automatically routes block paths (`/dev/diskN`) to raw character devices (`/dev/rdiskN`), unlocks DiskArbitration mounts (`diskutil unmountDisk force`), and bypasses the XNU buffer cache via `F_NOCACHE`.
  * **Windows (`usb/flasher_windows.go` & `build/windows/wails.exe.manifest`):** Embeds an explicit `requireAdministrator` UAC manifest, isolates `WebviewUserDataPath` to prevent elevated WebView2 permission conflicts, locks and dismounts mounted partitions (`FSCTL_LOCK_VOLUME` + `FSCTL_DISMOUNT_VOLUME`), and writes unbuffered 4 MB blocks (`FILE_FLAG_NO_BUFFERING` | FILE_FLAG_WRITE_THROUGH).
* **Stateful Pause, Resume & Cancel:** Freeze USB provisioning at any byte or container layer and resume from the exact position without losing downloaded ISO data or staged app manifests.
* **Native mDNS `.local` Discovery (`network/mdns_discovery.go`):** Built-in pure-Go multicast DNS resolver and concurrent LAN scanner that locates `instant-server.local` nodes without external scripts.

### 2. Hardware Survival, Thermal Safety & Network Failover

* **Laptop Lid-Close & Thermal Protection:** Keeps the server awake when the laptop lid is closed while actively warning against enclosed-drawer overheating traps. Includes a 1-click **Cool & Quiet CPU Mode** (disables Turbo Boost and switches to the `powersave` governor to lower peak temperatures by 10–15°C).
* **Battery Health Limiter & Built-In UPS Watchdog:** Caps continuous AC charging (default 40%–60%) to prevent lithium-ion swelling. Triggers a clean filesystem sync and shutdown when battery charge drops below your configured threshold during a power outage.
* **Seamless Ethernet-to-Wi-Fi Failover & Tailscale Remote Access:** Configures dual-interface netplan metrics with Wi-Fi power-save disabled for instant cable-unplug failover. Integrates built-in Tailscale VPN provisioning (Subnet Router + Tailscale SSH) for secure remote access outside your home network.

### 3. App Ecosystem & Multi-Server Pooling

* **139+ Verified Self-Hosted Apps:** Pre-bundle Jellyfin, Nextcloud, Immich, Pi-hole, AdGuard Home, WireGuard (`wg-easy`), Offline Wikipedia (Kiwix), Home Assistant, Vaultwarden, Paperless-ngx, Stirling-PDF, Portainer, Uptime Kuma, Ollama + Open WebUI, and Minecraft Java/Bedrock servers directly onto your USB stick.
* **Zero-Friction Custom Web App Importer:** Paste any Docker image (`owner/app:tag`), `docker run ...` command, `docker-compose.yml` snippet, or GitHub / Docker Hub URL to automatically generate its container configuration and `/DATA/AppData/<app>` volume bindings.
* **Multi-Computer Storage Pooling:** Combine multiple laptops or PCs on your home network using safe non-destructive MergerFS union pooling (preserving existing files on all machines) or confirmed secondary drive expansion.
* **5 Installation Modes (15 GB – 64 TB):** Supports *Dual-Boot Alongside Existing OS*, *Erase Main Drive*, *Select on Screen*, *Secondary Drive Only*, and *Live Repair & Rescue USB*.

---

## 📂 Project Structure

```text
Instant Server/
├── .github/workflows/
│   ├── ci.yml                 # Cross-platform Go vet, unit tests & ShellCheck CI
│   └── release.yml            # Automated Windows (.exe), macOS (.dmg) & Linux builds
├── build/
│   └── windows/
│       └── wails.exe.manifest # Windows UAC requireAdministrator & PerMonitorV2 DPI manifest
├── frontend/
│   └── wailsjs/go/main/
│       └── App.d.ts           # Statically typed Wails TypeScript/JavaScript bindings
├── network/
│   ├── mdns_discovery.go      # Pure-Go multicast DNS (.local) resolver & LAN scanner
│   └── mdns_test.go           # Unit tests for DNS wire-format encoding
├── privilege/
│   ├── escalate.go            # Native Polkit (pkexec), macOS AuthorizationServices & Windows UAC
│   └── escalate_test.go       # Unit tests for privilege detection
├── src/
│   └── instant_server/
│       ├── scripts/
│       │   └── laptop_hardening.sh # Target Ubuntu LTS hardening, cluster & self-healing daemon script
│       └── ui/
│           ├── app_ui.html           # Desktop USB provisioner UI (embedded into RAM via //go:embed)
│           └── server_dashboard.html # Target server web dashboard (:8088)
├── usb/
│   ├── cidata_fat.go          # Pure-Go FAT16 (LABEL="CIDATA") VFAT partition image builder
│   ├── catalog.go             # Verified container port/cap specs & Minecraft Studio builder
│   ├── iso_downloader.go      # Resumable HTTP Range ISO downloader & SHA-256 verifier
│   ├── flasher_common.go      # 4096-byte aligned DMA allocator, GRUB patcher & GPT relocator
│   ├── flasher_darwin.go      # macOS /dev/rdiskN routing, diskutil unmount & F_NOCACHE
│   ├── flasher_linux.go       # Linux O_DIRECT block streaming & BLKFLSBUF ioctl
│   ├── flasher_windows.go     # Windows FSCTL_LOCK_VOLUME & FILE_FLAG_NO_BUFFERING engine
│   ├── payload_builder.go     # Cloud-init autoinstall generator & custom web app parser
│   ├── scanner.go             # Cross-platform removable USB detector & Triple-Lock guard
│   └── flasher_test.go        # Unit tests for DMA alignment, streaming, GPT & validation
├── app.go                     # Wails application controller & asynchronous event channel
├── go.mod                     # Go module definition
├── LICENSE                    # GNU General Public License v3.0
├── main.go                    # Application entry point & WebviewUserDataPath isolation
├── README.md                  # Project documentation
└── wails.json                 # Wails project configuration
```

---

## 🛠️ Building & Running from Source

### Prerequisites

* **Go 1.22** or newer
* **Wails CLI:** Installed via `go install ://github.com`

#### Platform WebView Dependencies

* **Windows:** Microsoft Edge WebView2 Runtime (Pre-installed on Windows 10/11)
* **macOS:** Xcode Command Line Tools (`xcode-select --install`)
* **Linux (Ubuntu / Debian):**

```bash
sudo apt-get update
sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev pkg-config build-essential
```

### Running Unit Tests

```bash
go vet ./usb/... ./network/... ./privilege/...
go test -v ./usb/... ./network/... ./privilege/...
```

### Compiling Standalone Native Binaries

* **Windows:** `wails build -clean -o InstantServer-Windows.exe`
* **macOS:** `wails build -clean`
* **Linux (Ubuntu 24.04+ / WebKit2GTK 4.1):** `wails build -clean -tags webkit2_41 -o InstantServer-Linux`

---

## 💡 Real-World Hardware Best Practices

* **Avoid the Closed-Lid Overheating Trap:** Laptops are engineered to dissipate heat upward through the keyboard deck. **Never place a closed laptop inside an enclosed drawer, closet, or cabinet.** Trapped heat degrades the battery, NVMe/SATA drives, and the motherboard under 24/7 workloads. Stand the laptop vertically in an open-air stand, prop the lid slightly ajar (1–2 inches), or place it on a cooling pad, and enable **Cool & Quiet CPU Mode** in the app's Power tab.
* **Hardwired Gigabit Ethernet vs. 2.4GHz Wi-Fi:** While Instant Server configures automatic Wi-Fi failover so unplugging a cable never drops your connection, older laptops with 2.4GHz Wi-Fi cards typically average only 3–8 MB/s with high latency jitter. For smooth 4K Jellyfin streaming, fast Immich camera-roll backups, and Nextcloud transfers (110+ MB/s), connect your server via a **Gigabit Ethernet cable** (or a USB 3.0-to-Gigabit Ethernet adapter) and keep Wi-Fi active purely as your automatic backup failover link.

---

## 📄 License

This project is licensed under the **GNU General Public License v3.0 (GPL-3.0)** — see the [LICENSE](LICENSE) file for full details.
