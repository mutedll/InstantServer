# ⚡ Instant Server

> **Turn any old laptop into a fast, quiet Linux home server by plugging in a USB stick.**

Most people have an old laptop sitting in a drawer—maybe the battery only lasts 20 minutes, maybe the screen hinge is cracked, or maybe it's just too slow for Windows 11. **Instant Server** lets you turn that old laptop into a fast, quiet Linux home server for your files, movies (Jellyfin/Plex), ad-blocking (Pi-hole), and self-hosted apps without needing to know Linux commands.

You run the Instant Server desktop app on your regular computer, plug in a USB flash drive, pick your settings, and hit **Flash USB**. When you plug that USB into your old laptop and turn it on, it sets everything up on its own.

---

## 🛠️ What it does for your old laptop

Normal server installers aren't built for laptops. Instant Server configures Ubuntu Server specifically for laptop hardware right out of the box:

* **Close the lid and tuck it away:** Closing the laptop lid won't put the server to sleep. It turns the screen off after 60 seconds so it doesn't waste power or burn in.
* **Protects the battery:** If you leave a laptop plugged into the wall 24/7 at 100%, the battery can swell up over time. Instant Server limits battery charging to around 60% on supported laptops.
* **Built-in power outage protection:** If your home power goes out, the laptop runs on its battery and safely shuts itself down when the battery drops to 15% so your files don't get corrupted.
* **Keep your existing OS or wipe it:** You can Dual-Boot (keep the laptop's existing Windows or Linux system untouched), Erase the main drive for a dedicated server, or Select the partition on screen after plugging in the USB.
* **Web dashboards included:** Choose Cockpit (for managing storage, network, and terminal in your browser), CasaOS (a 1-click app store for Jellyfin, Nextcloud, Pi-hole, and Docker apps), or both.

---

## 🚀 How to download and run

You don't need any web server or complicated setup. There are two ways to run the app:

### Option 1: Download the ready-made app (Easiest)

Go to the **Releases** section on the right side of this GitHub page and download the file for your computer:

| Operating System | File to Download | How to Run |
| :--- | :--- | :--- |
| **Windows** | `InstantServer-Windows.exe` | Right-click and choose **Run as administrator** _(needed to write to USB drives)_ |
| **macOS** | `InstantServer-macOS` | Open Terminal, run `chmod +x InstantServer-macOS`, then `sudo ./InstantServer-macOS` |
| **Linux** | `InstantServer-Linux` | Open Terminal, run `chmod +x InstantServer-Linux`, then `sudo ./InstantServer-Linux` |

### Option 2: Download the code and run with Python

If you have Python 3.8 or newer installed, you can download or clone this repository and run it directly—no extra `pip install` packages are required:

```bash
git clone https://github.com/mutedll/InstantServer
cd instant-server
python instant_server_cli.py
```

---

## 📖 How to use it

1. **First-time setup:** When you open Instant Server for the first time, a simple setup window walks you through naming your server, entering your Wi-Fi (or skipping if you use Ethernet/phone USB tethering), choosing your apps (Cockpit, CasaOS), and picking your USB drive.
2. **Save or Flash:** At the last tab, you can either click **Confirm & Flash USB** right away, or click **Save configuration**. Saving stores `instant-server-config.json` in the app folder and opens the main Settings home screen next time you launch the app.
3. **Boot your old laptop:** Plug the finished USB drive into your old laptop, turn it on, and boot from the USB stick (usually by pressing `F12`, `F2`, `Esc`, or `Del` at startup).
4. **Open your server dashboard:** Once the laptop finishes installing, open your browser on any computer on your home Wi-Fi and visit:
   * **Cockpit:** `https://instant-server.local:9090`
   * **CasaOS:** `http://instant-server.local`

> [!TIP]
> If your router blocks `.local` addresses, click **Scan network** inside the app's *Configuration & LAN* tab (or run `python instant_server_cli.py --discover`) to find your laptop's IP address automatically.

---

## 📂 Files in this repository

| File | What it does |
| :--- | :--- |
| `instant_server_cli.py` | Main app launcher, local config saver (`instant-server-config.json`), and LAN scanner |
| `lazarus_gui.html` | Desktop user interface (Setup wizard + Windows 11 style Settings home view) |
| `m1_host_probe.py` | Checks your OS, free disk space, and privileges |
| `m2_iso_manager.py` | Downloads and checks the official Ubuntu Server LTS ISO |
| `m3_usb_engine.py` | Scans USB drives safely (blocks system drives) and creates the cidata boot partition |
| `m4_config_builder.py` | Builds the automated Ubuntu installer config and encrypts your password |
| `m5_payload_packager.py` | Packages the offline scripts and configs onto the USB drive |
| `p1_early_hardware_adapter.py` | Runs during install on the old laptop to match its hard drive, Wi-Fi, and UEFI/BIOS mode |
| `p2_late_laptop_hardening.sh` | Sets up lid-close rules, battery charge limits, power-outage shutdown, Cockpit, and CasaOS |
| `vm_test_harness.py` | Tests all modules without needing a physical USB stick |
| `build_executables.py` | Packages the app into standalone Windows, macOS, and Linux executables |

---

## 🧪 Testing changes locally

If you edit the code and want to make sure everything still works before pushing to GitHub, run the built-in test suite:

```bash
python vm_test_harness.py --self-test
```

---

## ⚖️ License

**GNU General Public License v3.0** — free to use, modify, and share.
