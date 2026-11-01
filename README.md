# ⚡ Instant Server

> **Turn any old laptop (or spare PC) into a fast, quiet Linux home server by plugging in a USB stick.**

Most people have an old laptop sitting in a drawer—maybe the battery only lasts 20 minutes, maybe the screen hinge is cracked, or maybe it's just too slow for Windows 11. **Instant Server** turns that spare laptop into a headless Ubuntu home server for your movies (**Jellyfin / Plex**), personal cloud (**Nextcloud / Immich**), network-wide ad blocking (**Pi-hole**), **Offline Wikipedia (Kiwix)**, **Minecraft servers**, or even **Local AI models** on higher-end laptops—without needing to touch a Linux terminal.

You open the Instant Server desktop app on your regular computer, check the apps you want from the built-in app store, plug in a USB flash drive, and click **Confirm & Flash Bootable USB**. The app pulls the official Ubuntu Server ISO, formats the USB drive, and packs your settings and selected apps right onto the stick. Plug it into your old laptop, turn it on, and it sets everything up on its own.

---

## ✨ What's New

- **True Bootable USB Creator (UEFI + Legacy BIOS):** Automatically downloads the official **Ubuntu Server 26.04.1 LTS** ISO (or **24.04.5 LTS** / **22.04.5 LTS**, or uses a local `.iso` file from your PC), verifies its `SHA256SUMS` checksum, formats your USB stick, and configures both **GPT (UEFI)** and **MBR (Legacy BIOS)** bootloaders for hands-free installation.
- **Built-in 26-App Self-Hosted Store:** Pick the apps you want before flashing. Your selected apps are pre-unpacked onto the USB drive and automatically registered into **CasaOS** and Docker during setup. Once your server is running, you can install even more apps anytime from the CasaOS web store.
- **Presets for Old Laptops vs. Higher-End Machines:** Includes one-click presets for **2 GB – 4 GB RAM old laptops** (lightweight apps + `zram` memory compression) and **8 GB – 16 GB+ higher-end laptops** (adding dedicated Minecraft Java servers and local AI models).
- **100% Offline Install Support:** Extracts `wpasupplicant` Wi-Fi drivers directly from the Ubuntu ISO so home Wi-Fi connects on first boot without needing an Ethernet cable, and includes an optional **Offline Container Image Bundle** toggle that downloads the Docker images on your main PC and packs them onto the USB stick.
- **Live Terminal Progress Bar & Safe Cancellation:** Shows real-time download speed, MB progress, and ETA inside a sleek terminal progress bar with smart sticky auto-scroll and instant cancel support.

---

## 🧩 Built-In Self-Hosted App Store (26 Apps)

Check any combination of apps in the **Apps & Store** tab before flashing your USB drive, or use the quick preset buttons (**Recommended**, **Old Laptop Pack**, or **Higher-End + AI & Minecraft**):

| Category | Included Self-Hosted Apps | Laptop Hardware Tier |
| :--- | :--- | :--- |
| **📚 Knowledge & Offline Wiki** | **Offline Wikipedia (Kiwix)** *(host full Wikipedia, Wiktionary, medical guides & Project Gutenberg `.zim` files on your Wi-Fi without internet)*, **BookStack Wiki**, **Calibre-Web eBooks** | ✅ **Old Laptop Friendly** *(128 MB – 256 MB RAM)* |
| **🎬 Media & Photos** | **Jellyfin**, **Plex Media Server**, **Immich** *(self-hosted Google Photos backup)*, **Navidrome** *(music server)*, **qBittorrent** | ✅ **Old Laptop Friendly** *(64 MB – 1 GB RAM)* |
| **☁️ Personal Cloud & Files** | **Nextcloud**, **Syncthing**, **FileBrowser**, **Paperless-ngx** *(document scanner & OCR)* | ✅ **Old Laptop Friendly** *(64 MB – 512 MB RAM)* |
| **🛡️ Network, VPN & Ad-Block** | **Pi-hole**, **AdGuard Home**, **WireGuard VPN (`wg-easy`)** *(with phone QR codes)*, **Nginx Proxy Manager** | ✅ **Old Laptop Friendly** *(64 MB – 128 MB RAM)* |
| **🏠 Smart Home & Tools** | **Home Assistant**, **Vaultwarden** *(Bitwarden password server)*, **Uptime Kuma**, **Portainer CE**, **Stirling-PDF**, **Gitea** | ✅ **Old Laptop Friendly** *(64 MB – 512 MB RAM)* |
| **🎮 Gaming Servers** | **Minecraft Bedrock Server** *(mobile, console & PC cross-play — runs great on old laptops)*<br>**Minecraft Dedicated Server (Java)** *(Paper / Vanilla / Forge / Fabric on port `25565`)* | ✅ **Bedrock:** Old Laptop Friendly *(512 MB RAM)*<br>⚡ **Java:** 4 GB+ RAM Recommended |
| **🤖 Local AI Models** | **Ollama + Open WebUI** *(private local ChatGPT for Llama 3, DeepSeek, Mistral & Qwen)*<br>**LocalAI Multi-Modal Engine** *(OpenAI-compatible local API)* | ⚠️ **Higher-End Only (8 GB – 16 GB+ RAM / GPU)**<br>*Marked clearly in the app — not for old laptops* |

---

## 🛠️ What It Does for Laptop Hardware

Standard server installers aren't designed for laptops. Instant Server configures Ubuntu Server specifically for 24/7 laptop use:

- **Close the lid and tuck it away:** Closing the laptop lid won't put the server to sleep (`HandleLidSwitch=ignore`). It blanks the screen after 60 seconds so it doesn't waste power or burn in.
- **Protects the battery from swelling:** Leaving a laptop plugged into wall power 24/7 at 100% can wear out or swell the battery. Instant Server caps charging around **60%** on supported laptops.
- **Built-in power outage protection (UPS):** If your home power goes out, the laptop automatically runs on its internal battery and cleanly shuts itself down when the battery drops to **15%** so your databases and drives never get corrupted.
- **Low-RAM boost (`zram`):** Configures compressed RAM swap so older 2 GB and 4 GB RAM laptops can run multiple Docker containers smoothly.
- **Keep your existing OS or dedicate the drive:**
  - **Dual-Boot (Safe Default):** Shrinks free space and installs Instant Server alongside the laptop's existing Windows or Linux system without deleting your old files.
  - **Automatically Erase Main Drive:** Formats the laptop's main internal drive and dedicates 100% of storage to the server.
  - **Select on Screen / Secondary Drive Only:** Pick the partition manually on the laptop screen during boot, or install onto a second internal drive.

---

## 🚀 How to Download and Run

You don't need a web browser or any extra tools. There are two ways to run Instant Server:

### Option 1: Download the Ready-Made App (Easiest)

Go to the \[suspicious link removed\] section on the right side of this GitHub page and download the file for your computer:

| Operating System | File to Download | How to Run |
| :--- | :--- | :--- |
| **Windows (10 / 11)** | `InstantServer-Windows.exe` | Double-click `InstantServer-Windows.exe` to open the app *(prompts for administrator permission automatically when formatting the USB drive)* |
| **macOS** | `InstantServer-macOS.dmg` | Open `InstantServer-macOS.dmg` and launch **Instant Server** |
| **Linux** | `InstantServer-Linux` | Open Terminal, run `chmod +x InstantServer-Linux`, then `./InstantServer-Linux` |

### Option 2: Run from Source with Python

If you have Python 3.8 or newer installed, clone the repository and run the launcher directly:

```bash
git clone https://github.com/mutedll/InstantServer
cd InstantServer
python instant_server_cli.py
```

---

## 📖 Step-by-Step Guide

1. **Name your server & Wi-Fi (`Network` tab):** Enter a server name (default: `instant-server`), an admin username and password, and your home Wi-Fi name and password (or leave Wi-Fi blank if using an Ethernet cable or phone USB tethering).
2. **Pick your dashboard & apps (`Apps & Store` tab):** Choose **Cockpit + CasaOS**, then check the self-hosted apps you want pre-bundled onto the USB stick.
   > [!NOTE]
   > **Want a 100% offline app install?** Open **Advanced options** at the bottom of the *Apps & Store* tab and turn on **Pre-download container images on this PC (Offline bundle)**. Instant Server will pull the Docker image layers on your PC and pack them onto the USB drive so the target laptop doesn't even need internet to install your apps.
3. **Pick your laptop type & USB stick (`Laptop & USB` tab):**
   - If you aren't sure how old the target laptop is, leave *"How old is the target laptop?"* on **I don't know (Universal Hybrid Boot)**—it sets up both UEFI and Legacy BIOS boot records automatically.
   - Under **Advanced options**, you can switch between **Ubuntu Server 26.04.1 LTS (Default)**, **24.04.5 LTS**, and **22.04.5 LTS**, or point to a local `.iso` file you already downloaded.
   - Select your removable USB drive (internal system drives are locked and protected automatically).
4. **Flash the USB (`Finish` tab):** Click **Confirm & Flash Bootable USB**. You can watch the live progress bar and download speed in the terminal box, or cancel anytime if needed.
5. **Boot your old laptop:** Plug the finished USB stick into your old laptop, power it on, and boot from USB (usually `F12`, `F2`, `Esc`, or `Del`). The installer runs automatically and reboots into your new server.
6. **Open your dashboards:** From any computer or phone on your home Wi-Fi, open your browser and visit:
   - **CasaOS App Store & Dashboard:** `http://instant-server.local`
   - **Cockpit System Console:** `https://instant-server.local:9090`
   - **Offline Wikipedia (if selected):** `http://instant-server.local:8088`
   - **Jellyfin Media Server (if selected):** `http://instant-server.local:8096`
   - **Local AI Open WebUI (if selected):** `http://instant-server.local:3002`

> [!TIP]
> If your home router blocks `.local` addresses, open the **Configuration & LAN** tab inside Instant Server and click **Scan network** (or run `python instant_server_cli.py --discover`) to find your server's exact IP address and open ports automatically.

---

## 📂 Files in This Repository

| File | What It Does |
| :--- | :--- |
| `instant_server_cli.py` | Main desktop app launcher, local config manager (`Instant_Server_Data/instant-server-config.json`), cancellation handler, and LAN/mDNS scanner |
| `lazarus_gui.html` | Windows 11 Fluent desktop UI (First-run Setup Wizard + two-column Settings Home view, 26-app store, and smart terminal log viewer) |
| `m1_host_probe.py` | Detects host OS, CPU architecture, disk space, and privileges |
| `m2_iso_manager.py` | Resolves live Ubuntu Server LTS mirrors (`26.04.1`, `24.04.5`, `22.04.5`), streams resumable ISO downloads with `SHA256SUMS` verification, patches GRUB for unattended boot, and extracts offline `.deb` packages |
| `m3_usb_engine.py` | Scans removable USB drives with Triple-Lock system drive protection, formats GPT/MBR partitions, and writes the bootable ISO + `cidata` + pre-unpacked app bundles |
| `m4_config_builder.py` | Compiles `user-data` cloud-init configs, SHA-512 password hashes, and Docker Compose manifests for all 26 self-hosted apps |
| `m5_payload_packager.py` | Stages `cidata`, offline Wi-Fi `.deb` packages, and CasaOS app bundles with `MANIFEST.sha256` verification |
| `p1_early_hardware_adapter.py` | Runs inside the Ubuntu installer on the target laptop to configure Dual-Boot or full-disk partitioning, Wi-Fi, and UEFI/BIOS boot |
| `p2_late_laptop_hardening.sh` | Configures lid-close sleep prevention, battery charge limits, power-outage auto-shutdown, `zram`, Cockpit, CasaOS, and unpacks your selected apps |
| `vm_test_harness.py` | Runs automated end-to-end tests across all 4 disk modes and UEFI/BIOS firmware modes without needing a physical USB drive |
| `build_executables.py` | Generates multi-size icons (`.ico`, `.icns`, `.png`), PyInstaller specs, and the GitHub Actions workflow for `InstantServer-Windows.exe`, `InstantServer-macOS.dmg`, and `InstantServer-Linux` |

---

## 🧪 Running the Built-In Self-Tests

Every module includes a deterministic `--self-test` mode so you can verify changes locally in seconds:

```bash
python m1_host_probe.py --self-test
python m2_iso_manager.py --self-test
python m3_usb_engine.py --self-test
python m4_config_builder.py --self-test
python m5_payload_packager.py --self-test
python instant_server_cli.py --self-test
python vm_test_harness.py --self-test
python build_executables.py --self-test
```

---

## ⚖️ License

**GNU General Public License v3.0** — free to use, modify, and share.
