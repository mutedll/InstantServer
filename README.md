# ⚡ Instant Server

Turn any old laptop, desktop PC, or Mini-PC into a fast, quiet Linux home server by plugging in a USB stick.

Most people have an old laptop sitting in a drawer—maybe the battery only lasts 20 minutes, maybe the screen hinge is cracked, or maybe it's just too slow for Windows 11. **Instant Server lets you turn that old laptop (or any spare desktop PC, Intel N100 Mini-PC, or multi-drive box) into a fast, quiet Linux home server for your files, movies, photo backups, and self-hosted apps without needing to know Linux commands.**

You run the Instant Server desktop app on your regular computer, plug in a USB flash drive, pick your settings, and hit **Flash USB**. When you plug that USB into your server computer and turn it on, it sets everything up on its own.

---

## 🛠️ What It Does for Your Hardware

Normal server installers aren't built for laptops or headless home servers. Instant Server configures Ubuntu Server LTS specifically for your hardware right out of the box:

* **Close the lid and tuck it away:** Closing a laptop lid won't put the server to sleep (`HandleLidSwitch=ignore`). It turns the screen off after 60 seconds (`consoleblank=60`) so it doesn't waste power or burn in.
* **Protects laptop batteries (and acts as a built-in UPS):** Leaving a laptop plugged into the wall 24/7 at 100% can swell the battery over time. Instant Server limits battery charging between 40% and 60% via TLP and ACPI thresholds, and includes a 60-second watchdog timer that safely shuts down your databases and OS if power goes out and the battery drops to 15%.
* **Desktop PC, Mini-PC & Multi-Bay NAS tuning:** Automatically detects when no battery is present and switches from laptop battery daemons to PCIe ASPM, SATA ALPM, and Realtek 2.5GbE ASPM power-saving along with Intel QuickSync / VA-API hardware transcoding.
* **Automatic Wi-Fi failover:** Configures both Ethernet (priority metric 100) and Wi-Fi (failover metric 600) with Wi-Fi power-saving disabled so your server never drops off the network when you unplug the Ethernet cable.
* **Self-healing disk & network guardrails:** Caps system logs (`SystemMaxUse=200M`), prunes unused container layers weekly, and runs a 2-minute network/mDNS self-healing watchdog so `.local` discovery stays reachable months and years later.

---

## 📦 Offline USB Install & 26 One-Click Self-Hosted Apps

* **Resumable ISO & App Layer Caching:** Downloads the official Ubuntu Server LTS ISO (26.04.1 LTS, 24.04.5 LTS, or 22.04.5 LTS) and OCI container layers into `Instant_Server_Data/` with HTTP Range resume and SHA-256 verification. Cancelling a flash never loses already-downloaded files.
* **100% Air-Gapped Offline Install:** Enable *Pre-download app images onto USB* to pull and bundle container tarballs (`docker.io`, `ghcr.io`, `lscr.io`) and offline Wi-Fi `.deb` packages (`wpasupplicant`, `rfkill`, `libpcsclite1`) directly onto the USB drive so your server comes up ready to use even without an internet connection.
* **Offline Wikipedia & Survival Library (Kiwix):** Pre-select or download `.zim` archives directly from the server dashboard—including Simple English Wikipedia, Full Illustrated English Wikipedia, WikiMed Medical Encyclopedia, iFixit Repair Guides, and Project Gutenberg.

---

## 🔗 Combine Multiple Computers into One Server

Have two old laptops or a laptop and a Mini-PC? You can combine them either when flashing the USB or later from the live server web dashboard (`http://instant-server.local:8088`):

* **Keep all files on both computers (Default — MergerFS Union Pool):** Combines the storage of both computers into one shared folder (`/DATA/ClusterPool`) without deleting or overwriting a single file on either machine.
* **SQLite database protection:** Automatically keeps app databases (Jellyfin, Immich, Nextcloud, Vaultwarden) pinned to local SSD/NVMe storage with scheduled `.backup` snapshots so network hiccups never corrupt your apps.
* **Mirror important folders across both computers:** Keeps a live backup copy on the second machine so your files stay available if one computer is turned off.
* **Confirmation gate before erasing a second computer:** Wiping a secondary server is blocked by default unless you explicitly type `WIPE-<hostname>`.

---

## 🩺 Bootable Repair & Rescue USB Mode

Select **Bootable Repair & Rescue USB** in the installation mode dropdown to create a live diagnostic and recovery drive (inspired by SystemRescue) that works on any laptop, desktop PC, or server without touching installed files:

* **1-Click UEFI & GRUB Bootloader Repair:** Automatically mounts your Linux root and EFI System Partition in a chroot and repairs missing UEFI NVRAM entries (`efibootmgr`).
* **Safe Unmounted Filesystem Repair:** Runs `e2fsck`, `btrfs check`, or `xfs_repair` safely on unmounted partitions (`fsck`).
* **Drive Health & Emergency File Rescue:** Checks SSD/NVMe wear levels (`smartctl` / `nvme-cli`) and starts a read-only network share so you can copy files off a computer that won't boot.

---

## ⚡ Native C11 Unbuffered DMA USB Engine (`win_usb_flasher.c`)

Inspired by Rufus on Windows and BalenaEtcher on macOS and Linux:

* **Hardware-level Triple-Lock protection:** Queries `IOCTL_STORAGE_QUERY_PROPERTY` and `IOCTL_DISK_GET_DRIVE_GEOMETRY_EX` to verify USB bus type, removable media status, and non-system mountpoints before opening any drive handle.
* **Automatic Explorer volume unlocking:** Locks, flushes, dismounts, and offlines mounted partitions (`FSCTL_LOCK_VOLUME`, `FSCTL_DISMOUNT_VOLUME`, `IOCTL_VOLUME_OFFLINE`, `IOCTL_DISK_SET_DISK_ATTRIBUTES`) so having the USB drive open in Windows Explorer never crashes or blocks flashing.
* **4 MB page-aligned DMA streaming & UEFI GPT relocation:** Streams via `VirtualAlloc` (`FILE_FLAG_NO_BUFFERING | FILE_FLAG_WRITE_THROUGH`), patches GRUB autoinstall in-flight, relocates the backup UEFI GPT header to the end of the USB drive, formats the CIDATA FAT32 partition, and verifies every written block with IEEE 802.3 CRC32 and SHA-256.

---

## 📂 Repository Structure

```text
InstantServer/
├── pyproject.toml
├── README.md
├── instant_server.py
├── build_executables.py
├── .github/
│   └── workflows/
│       ├── ci.yml
│       └── release.yml
├── src/
│   └── instant_server/
│       ├── __init__.py
│       ├── cli.py
│       ├── host_probe.py
│       ├── iso_manager.py
│       ├── usb_engine.py
│       ├── config_builder.py
│       ├── payload_packager.py
│       ├── dashboard_service.py
│       ├── native/
│       │   └── win_usb_flasher.c
│       ├── scripts/
│       │   ├── hardware_adapter.py
│       │   └── laptop_hardening.sh
│       └── ui/
│           ├── app_ui.html
│           └── server_dashboard.html
└── tests/
    ├── test_suite.py
    └── vm_test_harness.py
```

---

## 🚀 Quick Start & Running Tests

### Launch the App
```bash
python3 instant_server.py
```

### Compile & Self-Test the Native C11 USB Engine
```bash
gcc -std=c11 -O3 -Wall -Wextra -Wpedantic -Werror src/instant_server/native/win_usb_flasher.c -o win_usb_flasher
./win_usb_flasher --self-test
```

### Run Automated Tests & VM Simulations
```bash
# Run the automated test suite and 8-way UEFI/BIOS VM matrix simulation
python3 tests/test_suite.py
python3 tests/vm_test_harness.py --self-test
```

---

## ⚖️ License

**GNU General Public License v3.0** — Free to use, study, modify, and share.
