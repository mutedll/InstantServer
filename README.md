# Instant Server

Instant Server turns spare x86_64 laptops, mini-PCs, and desktop hardware into headless, self-healing Ubuntu LTS home servers. It compiles down to a single native Go + Wails desktop executable for Windows, macOS, and Linux that flashes an unmodified Ubuntu Server ISO to a USB drive using 4096-byte page-aligned Direct Memory Access (DMA) I/O, injects a hybrid GPT/MBR CIDATA FAT16 cloud-init partition on the fly, and provisions a hardened self-hosted stack on first boot without requiring a monitor or keyboard on the target machine.

---

## Table of Contents

- [Why Instant Server](#why-instant-server)
- [Core Architecture](#core-architecture)
- [Hardware-Safe Raw USB Flashing](#hardware-safe-raw-usb-flashing)
- [Zero-Touch Cloud-Init and FAT16 CIDATA Injection](#zero-touch-cloud-init-and-fat16-cidata-injection)
- [Target Server Hardening and Self-Healing Stack](#target-server-hardening-and-self-healing-stack)
- [Self-Hosted Application Catalog and Minecraft Studio](#self-hosted-application-catalog-and-minecraft-studio)
- [Repository Layout](#repository-layout)
- [Building from Source](#building-from-source)
- [Automated CI and Release Workflows](#automated-ci-and-release-workflows)
- [Frequently Asked Questions](#frequently-asked-questions)
- [License](#license)

---

## Why Instant Server

Repurposing an old laptop into a Linux server usually stalls on the same friction points: writing an ISO requires one tool, configuring autoinstall YAML and SHA-512 password hashes requires another, Wi-Fi and USB smartphone tethering interfaces fail silently during unattended installation, and closing the laptop lid suspends the machine mid-operation.

Instant Server solves the entire hardware-to-container pipeline in one native binary:

- **Zero Runtime Dependencies:** Written in Go and Wails (webkit2gtk on Linux, WKWebView on macOS, WebView2 on Windows) with zero Python interpreter weight.
- **Unbuffered 4 MB DMA Streaming:** Writes directly to physical sectors with `FILE_FLAG_NO_BUFFERING | FILE_FLAG_WRITE_THROUGH` on Windows, `O_DIRECT | O_EXCL` on Linux, and `/dev/rdiskN` with `F_NOCACHE` on macOS.
- **In-Stream GRUB and GPT Patching:** Patches the ISO kernel command line (`autoinstall`) while streaming and appends a valid FAT16 CIDATA partition entry into the GPT Partition Entry Array with recalculated IEEE 802.3 CRC-32 checksums.
- **Pause, Resume, and Cancel at Any Boundary:** Thread-safe `sync.Cond` gate pauses and resumes both HTTP Range ISO downloads (`.part` files) and raw sector streaming without corrupting disk state.
- **Dual-Boot and Multi-Disk Awareness:** Supports wiping a dedicated server disk or shrinking free space on an existing machine (up to 64 TB) while preserving existing OS bootloaders on `/boot/efi`.

---

## Core Architecture

| Layer | Technology | Responsibility |
| :--- | :--- | :--- |
| **Desktop Shell** | Go + Wails (`main.go`, `app.go`) | Native OS windowing, IPC bindings, privilege escalation, and isolated WebView cache management |
| **USB DMA Engine** | `usb/flasher_*.go` | 4096-byte page-aligned unbuffered sector streaming, hardware cache flush, and GPT/MBR partition injection |
| **NoCloud Builder** | `usb/cidata_fat.go`, `usb/payload_builder.go` | Pure-Go FAT16 filesystem generator with VFAT Long File Names, glibc `$6$` SHA-512 crypt, and Netplan YAML |
| **App Catalog & Studio** | `usb/catalog.go`, `src/instant_server/ui/app_studio.js` | Multi-container Docker Compose generator, custom URL/command parser, and Minecraft Server Studio |
| **LAN Discovery** | `network/mdns_discovery.go` | Multicast DNS (`224.0.0.251:5353`) responder lookup and concurrent TCP port scanner |
| **Target Provisioner** | `src/instant_server/scripts/laptop_hardening.sh` | First-boot systemd hardening, ACPI lid-ignore, battery thresholds, zram swap, MergerFS, and CasaOS setup |

---

## Hardware-Safe Raw USB Flashing

Writing raw disk clusters across Windows, macOS, and Linux requires strict alignment and operating system volume coordination:

1. **Triple-Lock Drive Verification (`usb/scanner.go`):** Every candidate block device is checked across three independent hardware locks before it can be selected:
   - **Bus Lock:** Only physical `usb` bus devices are eligible (`nvme`, `sata`, `raid`, and internal buses are blocked).
   - **System Mount Lock:** Any disk backing `/`, `/boot`, `/boot/efi`, `/home`, `/var`, `/System/Volumes/Data`, `C:\`, `\\.\PhysicalDrive0`, `/dev/sda`, `/dev/nvme0n1`, `/dev/disk0`, or `/dev/disk1` is locked out at the backend level.
   - **Capacity Lock:** Drives smaller than 4 GB or marked non-removable are rejected.
2. **OS Volume Locking and Dismounting:**
   - **Windows (`usb/flasher_windows.go`):** Enumerates all mounted partition drive letters on the target `\\.\PhysicalDriveN`, issues `FSCTL_ALLOW_EXTENDED_DASD_IO` (`0x00090083`), locks each volume via `FSCTL_LOCK_VOLUME` (`0x00090018`) with exponential backoff, dismounts filesystems via `FSCTL_DISMOUNT_VOLUME` (`0x00090020`), and clears stale partition tables via `IOCTL_DISK_DELETE_DRIVE_LAYOUT` before streaming.
   - **macOS (`usb/flasher_darwin.go`):** Unmounts Disk Arbitration claims via `diskutil unmountDisk force /dev/diskN`, opens the raw character device `/dev/rdiskN` with `O_RDWR | O_EXCL`, and enables `F_NOCACHE` via `fcntl`.
   - **Linux (`usb/flasher_linux.go`):** Releases desktop auto-mounts (`udisksctl unmount` and `MNT_DETACH`), opens the block device with `O_RDWR | O_DIRECT | O_EXCL`, pads the final ISO tail block with zeros to the nearest 4096-byte page boundary, and flushes hardware buffers via `fsync`, `BLKFLSBUF`, and `BLKRRPART`.

---

## Zero-Touch Cloud-Init and FAT16 CIDATA Injection

Ubuntu's subiquity installer and cloud-init NoCloud datasource only auto-discover provisioning payloads when they reside on a formatted `vfat` or `iso9660` filesystem carrying the exact volume label `CIDATA`.

Instant Server builds a complete, standard-compliant FAT16 (`vfat`) partition image in pure Go (`usb/cidata_fat.go`) without requiring external host formatting tools (`mkfs.vfat`, `mtools`, or `xorriso`):

- **BIOS Parameter Block (BPB) & >4085 Clusters:** Formats a 16.8 MB FAT16 volume with 4100 clusters so the Linux kernel `vfat` driver and `blkid` unambiguously classify the partition as FAT16 with `LABEL="CIDATA"`.
- **VFAT Long File Name (LFN) Directory Entries:** Encodes `user-data`, `meta-data`, `instant-server-env.conf`, `instant-server-config.json`, `laptop_hardening.sh`, and `server_dashboard.html` with UTF-16LE VFAT Long File Name entries paired with 8.3 short aliases and valid VFAT checksums.
- **Pure-Go glibc `$6$` SHA-512 Password Hashing:** Implements the 5000-round Ulrich Drepper SHA-512 crypt permutation (`usb/payload_builder.go`) in pure Go so administrator passwords in autoinstall `user-data` authenticate out of the box on Linux PAM without `mkpasswd` or `openssl` on the host machine.

---

## Target Server Hardening and Self-Healing Stack

When the target laptop or mini-PC boots from the flashed USB drive, `curtin` copies the CIDATA payload into `/target/opt/instant-server/` and runs `laptop_hardening.sh` inside the target OS:

- **Clamshell Lid-Close & Console Blank Protection:** Configures `/etc/systemd/logind.conf.d/99-instant-server-clamshell.conf` (`HandleLidSwitch=ignore`) and sets kernel `consoleblank=60` so closing a laptop lid never suspends the server and OLED/LCD panels do not suffer burn-in.
- **24/7 Lithium-Ion Battery Care:** Detects `/sys/class/power_supply/BAT*` charge thresholds and installs a persistent systemd service that caps charge at 80% (start at 60%) on supported Lenovo ThinkPad, Dell, ASUS, Huawei, and Framework laptops.
- **RAM Compression (zram) & NVMe Wear Reduction:** Configures `zramswap` (zstd compression at 50% RAM allocation, priority 100), sets `vm.swappiness=10`, and caps `systemd-journald` (`SystemMaxUse=100M`) to minimize flash wear.
- **Systemd-Resolved Port 53 Stub Liberation:** Disables `DNSStubListener=no` in `/etc/systemd/resolved.conf.d/` so DNS sinkholes like Pi-hole and AdGuard Home bind cleanly to `0.0.0.0:53`.
- **Self-Healing Watchdog Timer:** Installs `instant-server-watchdog.timer` to verify Docker daemon responsiveness, prune dangling images when root filesystem usage exceeds 85%, and restart unhealthy containers automatically.

---

## Self-Hosted Application Catalog and Minecraft Studio

The UI catalog includes over 150 categorized self-hosted applications (`src/instant_server/ui/catalog_data.js` and `usb/catalog.go`) alongside two specialized builders:

- **Zero-Friction Custom Web App Importer:** Paste any `docker run` command, `docker-compose.yml` snippet, Docker Hub image tag, or GitHub repository URL (`https://github.com/owner/repo`) to synthesize a persistent CasaOS service definition automatically.
- **Multi-Container Stack Templates:** Deploys multi-service stacks with internal healthchecks and service dependencies for Immich (`immich-server`, `immich-machine-learning`, `redis`, and `tensorchord/pgvecto-rs`) and Paperless-ngx (`paperless-ngx` + `redis:7-alpine` broker), alongside Pi-hole v6 (`FTLCONF_dns_listeningMode=all`), wg-easy v15, Stirling-PDF, Nextcloud, Jellyfin, and Ollama + Open WebUI.
- **Dedicated Minecraft Server Studio & Per-App Studio:** Configures `itzg/minecraft-server` (Paper, Fabric, Forge, Purpur, Vanilla, or Geyser + Floodgate Bedrock crossplay on 25565/tcp and 19132/udp), Aikar's G1GC JVM flags, online-mode toggle, Modrinth/CurseForge auto-download slugs, RCON console access, `itzg/mc-backup` sidecar scheduling, and direct `server.properties` editing before flashing.

---

## Repository Layout

```text
.
├── .github/
│   └── workflows/
│       ├── ci.yml
│       └── release.yml
├── network/
│   └── mdns_discovery.go
├── src/
│   └── instant_server/
│       ├── scripts/
│       │   └── laptop_hardening.sh
│       └── ui/
│           ├── app_studio.js
│           └── catalog_data.js
├── usb/
│   ├── catalog.go
│   ├── cidata_fat.go
│   ├── flasher_darwin.go
│   ├── flasher_linux.go
│   ├── flasher_windows.go
│   ├── payload_builder.go
│   └── scanner.go
├── app.go
├── main.go
├── go.mod
└── go.sum
```

---

## Building from Source

### Prerequisites

- **Go:** Standard Go toolchain (`go` on `PATH`)
- **Wails CLI:** Install via:
  ```bash
  go install [github.com/wailsapp/wails/v2/cmd/wails@latest](https://github.com/wailsapp/wails/v2/cmd/wails@latest)
  ```
- **Platform Libraries:**
  - **Windows:** WebView2 Runtime (pre-installed on Windows 10/11 and Windows Server 2025)
  - **macOS:** Xcode Command Line Tools (`xcode-select --install`)
  - **Linux (Ubuntu/Debian):**
    ```bash
    sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev pkg-config build-essential
    ```

### Run Unit Tests and Static Analysis

```bash
go vet ./...
go test -v ./...
shellcheck src/instant_server/scripts/laptop_hardening.sh
```

### Compile Standalone Native Executable

```bash
wails build
```

---

## Automated CI and Release Workflows

Both GitHub Actions workflows run automatically on every push and pull request to `main` or `master`:

- **CI Verification Suite (`.github/workflows/ci.yml`):** Normalizes the Go package hierarchy, runs `go vet` and `go test -v` across `ubuntu-24.04`, `windows-2025`, and `macos-15-intel`, and validates `laptop_hardening.sh` with `bash -n` and `shellcheck`.
- **Build & Release Executables (`.github/workflows/release.yml`):** Runs automatically alongside CI to compile `InstantServer-Windows.exe`, `InstantServer-macOS.dmg`, and `InstantServer-Linux`, uploads the compiled binaries as workflow artifacts on every run, and publishes them to GitHub Releases whenever a `v*` tag is pushed.

---

## Frequently Asked Questions

### Why does Instant Server build a raw FAT16 partition in Go instead of writing files to a directory on the USB stick?

When you stream a hybrid Ubuntu Server ISO directly to a raw USB block device (`/dev/sdb` or `\\.\PhysicalDrive2`), the USB drive's partition table is overwritten with the read-only ISO9660 / GPT layout from the Ubuntu image. You cannot simply copy files into that read-only image. Instead, `usb/cidata_fat.go` formats a second partition in memory as a compliant FAT16 filesystem with volume label `CIDATA`, writes it at the first 1 MB-aligned sector after the ISO payload, and registers Partition #2 in the GPT Partition Entry Array so cloud-init mounts it automatically on first boot.

### Why did Windows previously throw Access Denied (Error 5) when writing to PhysicalDriveN even as Administrator?

Starting with Windows 10 and enforced strictly on Windows 11 and Windows Server 2025, the Windows Kernel PnP Manager blocks raw sector writes to `\\.\PhysicalDriveN` whenever a filesystem driver holds an active mount on any partition belonging to that disk. Instant Server resolves this in `usb/flasher_windows.go` by enumerating every drive letter (`D:`, `E:`, etc.) mapped to the target PhysicalDrive, issuing `FSCTL_LOCK_VOLUME` and `FSCTL_DISMOUNT_VOLUME` on each volume handle, enabling `FSCTL_ALLOW_EXTENDED_DASD_IO`, and keeping those volume handles locked until the raw DMA stream finishes and flushes.

### How does Dual-Boot mode avoid wiping my existing Windows or Linux installation?

When you select "Dual-Boot Alongside Existing OS" and choose an allocation size (from 20 GB up to 64 TB), `usb/payload_builder.go` generates a non-destructive autoinstall storage layout (`reorder_uefi: false`) and an early-commands hook that inspects free unallocated space via `lsblk` and `sfdisk` without issuing a whole-disk `wipe: superblock-recursive`. Existing EFI System Partitions are mounted at `/boot/efi` with `format: false` and `preserve: true`.

### Can I configure Minecraft server properties, GeyserMC Bedrock crossplay, or custom ports before flashing the USB?

Yes. Click the gear icon (Studio) on any app card in the catalog before flashing. Selecting Minecraft Server opens the Minecraft Server Studio, where you can switch between Paper, Fabric, Forge, Purpur, Vanilla, or Geyser + Floodgate (which opens both Java 25565/tcp and Bedrock 19132/udp), set JVM heap memory with Aikar's G1GC flags, toggle `online-mode=false`, list initial OP usernames, add Modrinth/CurseForge plugin slugs, and edit `server.properties` directly. Every file you edit is written into the CIDATA partition and applied on first boot.

### What happens if my internet drops while downloading the Ubuntu ISO or if I click Pause mid-flash?

ISO downloads use `HTTP Range: bytes=<offset>-` headers and stream into a `.part` file inside `Instant_Server_Data/iso_cache/`. Clicking Pause suspends the active Goroutine at the `PauseGate` condition variable; clicking Resume continues from the exact byte offset without re-downloading or restarting the stream.

### How do I find the server on my network after plugging the USB into the target laptop?

Once the target laptop finishes its unattended installation and reboots, it broadcasts `<hostname>.local` via Avahi mDNS (`224.0.0.251:5353`) and serves the Instant Server Web Control Center on port 8088, CasaOS on port 80, and Cockpit on port 9090. You can also click **Find Server on LAN** inside the desktop app to run an mDNS query and concurrent subnet port scan.

---

## License

This project is licensed under the GNU General Public License v3.0 (GPL-3.0). See the [GNU General Public License v3.0](LICENSE) or the `LICENSE` file in this repository for full terms and conditions.