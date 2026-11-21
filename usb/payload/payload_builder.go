// ============================================================================
// FILE: usb/payload/payload_builder.go
// PURPOSE: Generates Ubuntu Autoinstall Cloud-Config ("user-data", "meta-data"),
//
//	target environment variables ("instant-server-env.conf"), and stages
//	all provisioning scripts and Docker Compose stacks into CIDATA.
//
// ============================================================================
package payload

import (
	"fmt"
	"strconv"
	"strings"
)

func GenerateAutoinstallUserData(cfg ServerProfileConfig) string {
	host := strings.TrimSpace(cfg.ServerAddressHost)
	if host == "" {
		host = strings.TrimSpace(cfg.Hostname)
	}
	if host == "" {
		host = "instant-server"
	}
	user := strings.TrimSpace(cfg.Username)
	if user == "" {
		user = "admin"
	}
	pass := cfg.Password
	if pass == "" {
		pass = "admin"
	}
	hashedPass := Sha512Crypt(pass, "instantserverlts")

	layout := strings.TrimSpace(cfg.StorageLayout)
	if layout == "" {
		layout = "direct"
	}
	sizingLine := ""
	if layout == "lvm" {
		sizingLine = "\n      sizing-policy: all"
	}

	var sshKeysBlock string
	if len(cfg.SSHAuthorizedKeys) > 0 {
		var kl []string
		for _, k := range cfg.SSHAuthorizedKeys {
			if strings.TrimSpace(k) != "" {
				kl = append(kl, fmt.Sprintf("      - %q", strings.TrimSpace(k)))
			}
		}
		if len(kl) > 0 {
			sshKeysBlock = "\n    authorized-keys:\n" + strings.Join(kl, "\n")
		}
	}

	ethSection := `      eth_en:
        match:
          name: "en*"
        dhcp4: true
        dhcp4-overrides:
          route-metric: 100
        optional: true
      eth_legacy:
        match:
          name: "eth*"
        dhcp4: true
        dhcp4-overrides:
          route-metric: 100
        optional: true`
	if strings.TrimSpace(cfg.StaticIPCIDR) != "" && strings.TrimSpace(cfg.StaticGateway) != "" {
		dnsList := `"1.1.1.1", "9.9.9.9"`
		if len(cfg.StaticDNS) > 0 {
			dnsList = `"` + strings.Join(cfg.StaticDNS, `", "`) + `"`
		}
		ethSection = fmt.Sprintf(`      eth_en:
        match:
          name: "en*"
        dhcp4: false
        addresses:
          - %s
        routes:
          - to: default
            via: %s
            metric: 100
        nameservers:
          addresses: [%s]
        optional: true
      eth_legacy:
        match:
          name: "eth*"
        dhcp4: true
        optional: true`, strings.TrimSpace(cfg.StaticIPCIDR), strings.TrimSpace(cfg.StaticGateway), dnsList)
	}

	usbTetherSection := ""
	if cfg.EnableUSBTethering {
		usbTetherSection = `
      usb_tether:
        match:
          name: "usb*"
        dhcp4: true
        dhcp4-overrides:
          route-metric: 250
        optional: true`
	}

	wifiSection := ""
	if strings.TrimSpace(cfg.WifiSSID) != "" {
		safeSSID := strings.ReplaceAll(strings.TrimSpace(cfg.WifiSSID), `"`, `\"`)
		safePSK := strings.ReplaceAll(cfg.WifiPassword, `"`, `\"`)
		authLine := " {}"
		if safePSK != "" {
			authLine = fmt.Sprintf("\n            password: \"%s\"", safePSK)
		}
		wifiSection = fmt.Sprintf(`
    # BEGIN_INSTANT_WIFI
    wifis:
      wlan0:
        dhcp4: true
        dhcp4-overrides:
          route-metric: 600
        optional: true
        access-points:
          "%s":%s
    # END_INSTANT_WIFI`, safeSSID, authLine)
	}

	liveNoSleepCmds := `    - systemctl stop casper-md5check.service multipathd.service multipathd.socket 2>/dev/null || true
    - 'printf "{\"result\":\"pass\"}\n" > /run/casper-md5check.json 2>/dev/null || true'
    - mkdir -p /mnt/cidata /mnt/usb_logs /tmp/cidata
    - cdev=$(blkid -L CIDATA 2>/dev/null || blkid -L cidata 2>/dev/null || true); [ -n "$cdev" ] && (mount -o rw,sync,flush,noatime "$cdev" /mnt/cidata 2>/dev/null || mount -o remount,rw,sync,flush /mnt/cidata 2>/dev/null || true)
    - cp -rf /cdrom/cidata/* /tmp/cidata/ 2>/dev/null || true
    - cp -rf /mnt/cidata/* /tmp/cidata/ 2>/dev/null || true
    - bash /tmp/cidata/check_existing_os_safeguard.sh 2>/dev/null || bash /mnt/cidata/check_existing_os_safeguard.sh 2>/dev/null || true
    - udev=$(blkid -L INSTANTUSB 2>/dev/null || true); [ -n "$udev" ] && (mount -o rw,sync,flush,noatime "$udev" /mnt/usb_logs 2>/dev/null || true)
    - bash /tmp/cidata/start_usb_logger.sh 2>/dev/null || bash /mnt/cidata/start_usb_logger.sh 2>/dev/null || true
    - rfkill unblock all 2>/dev/null || true
    - systemctl mask sleep.target suspend.target hibernate.target hybrid-sleep.target 2>/dev/null || true
    - 'mkdir -p /etc/systemd/logind.conf.d && printf "[Login]\nHandlePowerKey=poweroff\nHandlePowerKeyLongPress=poweroff\nHandleLidSwitch=ignore\nHandleLidSwitchExternalPower=ignore\nHandleLidSwitchDocked=ignore\nIdleAction=ignore\n" > /etc/systemd/logind.conf.d/00-live-nosleep.conf && systemctl restart systemd-logind 2>/dev/null || true'
    - chmod 600 /etc/netplan/*.yaml 2>/dev/null || true
    - bash /tmp/cidata/select_target_disk.sh 2>/dev/null || bash /mnt/cidata/select_target_disk.sh 2>/dev/null || true
    - udevadm settle --timeout=15 2>/dev/null || true`

	var earlyCommandsBlock string
	var storageBlock string
	var extraLateCommands string

	switch cfg.InstallMode {
	case "dual_boot":
		allocGB := cfg.DualBootAllocationGB
		if allocGB < 15 {
			allocGB = 45
		}
		earlyCommandsBlock = fmt.Sprintf(`  early-commands:
%s
    - bash /tmp/cidata/dynamic_dualboot_storage.sh %d 2>/dev/null || bash /mnt/cidata/dynamic_dualboot_storage.sh %d 2>/dev/null || bash /cdrom/cidata/dynamic_dualboot_storage.sh %d 2>/dev/null || true
    - udevadm settle --timeout=15 2>/dev/null || true
`, liveNoSleepCmds, allocGB, allocGB, allocGB)
		storageBlock = fmt.Sprintf(`  storage:
    layout:
      name: %s%s
      match:
        size: largest`, layout, sizingLine)

	case "reinstall_keep_data":
		earlyCommandsBlock = fmt.Sprintf(`  early-commands:
%s
    - bash /mnt/cidata/reinstall_preserve_data.sh backup 2>/dev/null || bash /cdrom/cidata/reinstall_preserve_data.sh backup 2>/dev/null || bash /cdrom/reinstall_preserve_data.sh backup 2>/dev/null || true
`, liveNoSleepCmds)
		storageBlock = fmt.Sprintf(`  storage:
    layout:
      name: %s%s
      match:
        size: largest`, layout, sizingLine)
		extraLateCommands = "\n    - bash /mnt/cidata/reinstall_preserve_data.sh restore 2>/dev/null || bash /cdrom/cidata/reinstall_preserve_data.sh restore 2>/dev/null || bash /cdrom/reinstall_preserve_data.sh restore 2>/dev/null || true"

	case "reinstall_wipe_data":
		earlyCommandsBlock = fmt.Sprintf(`  early-commands:
%s
`, liveNoSleepCmds)
		storageBlock = fmt.Sprintf(`  storage:
    layout:
      name: %s%s
      match:
        size: largest`, layout, sizingLine)

	case "offline_updater_usb":
		earlyCommandsBlock = fmt.Sprintf(`  early-commands:
%s
    - bash /mnt/cidata/offline_updater_apply.sh 2>/dev/null || bash /cdrom/cidata/offline_updater_apply.sh 2>/dev/null || bash /cdrom/offline_updater_apply.sh 2>/dev/null || true
`, liveNoSleepCmds)
		storageBlock = `  # Offline Updater USB: updates existing server dashboard, scripts, and app stacks without formatting`

	case "repair_rescue_usb":
		earlyCommandsBlock = fmt.Sprintf(`  early-commands:
%s
    - bash /mnt/cidata/emergency_repair_reset.sh 2>/dev/null || bash /cdrom/cidata/emergency_repair_reset.sh 2>/dev/null || bash /cdrom/emergency_repair_reset.sh 2>/dev/null || true
`, liveNoSleepCmds)
		storageBlock = `  # Live Repair & Password Reset Mode: repairs bootloader/filesystem and resets credentials without formatting`

	case "interactive_select", "interactive_gui_disk":
		earlyCommandsBlock = fmt.Sprintf(`  early-commands:
%s
`, liveNoSleepCmds)
		storageBlock = `  # Interactive guided storage selection enabled via interactive-sections`

	case "secondary_drive":
		earlyCommandsBlock = fmt.Sprintf(`  early-commands:
%s
`, liveNoSleepCmds)
		storageBlock = fmt.Sprintf(`  storage:
    layout:
      name: %s%s
      match:
        path: /dev/sdb`, layout, sizingLine)

	default:
		earlyCommandsBlock = fmt.Sprintf(`  early-commands:
%s
`, liveNoSleepCmds)
		storageBlock = fmt.Sprintf(`  storage:
    layout:
      name: %s%s
      match:
        size: largest`, layout, sizingLine)
	}

	interactiveBlock := "  interactive-sections: []"
	if cfg.InstallMode == "interactive_select" || cfg.InstallMode == "interactive_gui_disk" {
		interactiveBlock = "  interactive-sections:\n    - storage"
	}

	return fmt.Sprintf(`#cloud-config
autoinstall:
  version: 1
%s
  source:
    id: ubuntu-server-minimal
    search_drivers: false
  drivers:
    install: false
  oem:
    install: false
  shutdown: reboot
  apt:
    fallback: offline-install
    geoip: false
  identity:
    hostname: %s
    username: %s
    password: '%s'
  ssh:
    install-server: true
    allow-pw: true%s
  network:
    version: 2
    ethernets:
%s%s%s
%s%s
  late-commands:
    - mkdir -p /target/opt/instant-server /target/opt/instant-server/debs /mnt/cidata /mnt/usb_logs
    - cdev=$(blkid -L CIDATA 2>/dev/null || blkid -L cidata 2>/dev/null || true); [ -n "$cdev" ] && (mount -o rw,sync,flush,noatime "$cdev" /mnt/cidata 2>/dev/null || mount -o remount,rw,sync,flush /mnt/cidata 2>/dev/null || true)
    - udev=$(blkid -L INSTANTUSB 2>/dev/null || true); [ -n "$udev" ] && (mount -o rw,sync,flush,noatime "$udev" /mnt/usb_logs 2>/dev/null || true)
    - if [ -f /mnt/cidata/laptop_hardening.sh ]; then cp -rf /mnt/cidata/* /target/opt/instant-server/ 2>/dev/null || true; elif [ -d /cdrom/cidata ]; then cp -rf /cdrom/cidata/* /target/opt/instant-server/ 2>/dev/null || true; fi
    - find /cdrom/pool -type f \( -name "wpasupplicant*.deb" -o -name "wireless-tools*.deb" -o -name "libnl-*.deb" -o -name "libpcsclite*.deb" -o -name "rfkill*.deb" -o -name "ethtool*.deb" \) -exec cp -f {} /target/opt/instant-server/debs/ \; 2>/dev/null || true%s
    - date +%%s > /target/opt/instant-server/.install_complete_epoch 2>/dev/null || true
    - chmod 600 /etc/netplan/*.yaml 2>/dev/null || true
    - mkdir -p /target/var/log/instant-server/install /mnt/usb_logs/instant_server_logs 2>/dev/null || true
    - chmod 600 /target/etc/netplan/*.yaml /etc/netplan/*.yaml 2>/dev/null || true
    - 'if lvdisplay /dev/ubuntu-vg/ubuntu-lv >/dev/null 2>&1; then lvextend -l +100%%FREE /dev/ubuntu-vg/ubuntu-lv 2>/dev/null || true; resize2fs /dev/mapper/ubuntu--vg-ubuntu--lv 2>/dev/null || true; fi'
    - curtin in-target -- chmod +x /opt/instant-server/laptop_hardening.sh 2>/dev/null || true
    - curtin in-target -- bash /opt/instant-server/laptop_hardening.sh >> /mnt/usb_logs/instant_server_logs/instant_server_install.log 2>&1 || true
    - cp -f /mnt/usb_logs/instant_server_logs/instant_server_install.log /target/var/log/instant-server/install/instant_server_install.log 2>/dev/null || true
    - bash /tmp/cidata/dump_error_logs_to_usb.sh SUCCESS 2>/dev/null || bash /mnt/cidata/dump_error_logs_to_usb.sh SUCCESS 2>/dev/null || true
    - sync
    - 'USB_PART=$(findmnt -n -o SOURCE /cdrom 2>/dev/null || blkid -L CIDATA 2>/dev/null || true); USB_PK=$([ -n "$USB_PART" ] && lsblk -no PKNAME "$USB_PART" 2>/dev/null | head -n1 || true); USB_DISK=$([ -n "$USB_PK" ] && echo "/dev/$USB_PK" || echo ""); printf "\033[2J\033[H\n======================================================================\n  INSTANT SERVER INSTALLATION COMPLETE!\n======================================================================\n  1. UNPLUG YOUR USB DRIVE NOW.\n     As soon as you unplug it, this computer will restart automatically\n     in 5 seconds and boot into your new Instant Server.\n\n  2. Or press ENTER (or tap the Power button once) to restart now.\n======================================================================\n" > /dev/tty1 2>/dev/null || true; while true; do if [ -n "$USB_DISK" ] && [ ! -b "$USB_DISK" ]; then printf "\n  USB drive unplugged! Restarting into Instant Server in 5 seconds...\n" > /dev/tty1 2>/dev/null || true; sleep 5; break; fi; if [ -e /dev/tty1 ] && read -r -t 1 _ < /dev/tty1; then printf "\n  Restarting now...\n" > /dev/tty1 2>/dev/null || true; break; fi; sleep 1; done; sync; reboot -f'
  error-commands:
    - bash /mnt/cidata/dump_error_logs_to_usb.sh ERROR 2>/dev/null || bash /cdrom/cidata/dump_error_logs_to_usb.sh ERROR 2>/dev/null || true
    - sync
`, interactiveBlock, host, user, hashedPass, sshKeysBlock, ethSection, usbTetherSection, wifiSection, earlyCommandsBlock, storageBlock, extraLateCommands)
}

func GenerateTargetEnvConf(cfg ServerProfileConfig) string {
	bool01 := func(b bool) string {
		if b {
			return "1"
		}
		return "0"
	}
	host := strings.TrimSpace(cfg.ServerAddressHost)
	if host == "" {
		host = strings.TrimSpace(cfg.Hostname)
	}
	if host == "" {
		host = "instant-server"
	}
	user := strings.TrimSpace(cfg.Username)
	if user == "" {
		user = "admin"
	}
	dashPort := cfg.DashboardPort
	if dashPort <= 0 {
		dashPort = 8088
	}
	sshPort := cfg.SSHPort
	if sshPort <= 0 || sshPort > 65535 {
		sshPort = 22
	}
	swappiness := cfg.VMSwappiness
	if swappiness <= 0 || swappiness > 100 {
		swappiness = 10
	}
	inotifyWatches := cfg.InotifyMaxWatches
	if inotifyWatches < 65536 {
		inotifyWatches = 524288
	}
	batStart := cfg.BatteryChargeStartPct
	if batStart <= 0 {
		batStart = 40
	}
	batStop := cfg.BatteryChargeStopPct
	if batStop <= 0 {
		batStop = 60
	}
	upsPct := cfg.UPSShutdownBatteryPct
	if upsPct <= 0 {
		upsPct = 15
	}
	blankSec := cfg.ScreenBlankSeconds
	if blankSec < 0 {
		blankSec = 300
	}
	serverName := strings.TrimSpace(cfg.ServerName)
	if serverName == "" {
		serverName = host
	}
	addrHost := strings.TrimSpace(cfg.ServerAddressHost)
	if addrHost == "" {
		addrHost = host
	}
	addrDomain := strings.TrimSpace(cfg.ServerAddressDomain)
	if addrDomain == "" {
		addrDomain = ".local"
	}
	if !strings.HasPrefix(addrDomain, ".") {
		addrDomain = "." + addrDomain
	}
	logDays := cfg.LogRetentionDays
	if logDays <= 0 {
		logDays = 14
	}
	gov := strings.TrimSpace(cfg.CPUgovernor)
	if gov == "" {
		gov = "schedutil"
	}
	clusterMode := strings.TrimSpace(cfg.ClusterMode)
	if clusterMode == "" {
		clusterMode = "standalone"
	}
	clusterPol := strings.TrimSpace(cfg.ClusterStoragePolicy)
	if clusterPol == "" {
		clusterPol = "mergerfs_union_safe"
	}
	tsMode := strings.TrimSpace(cfg.TailscaleMode)
	if tsMode == "" {
		tsMode = "subnet_and_ssh"
	}
	enableCockpit := cfg.DashboardChoice == "both" || cfg.DashboardChoice == "cockpit" || cfg.DashboardChoice == ""
	enableCasaOS := cfg.DashboardChoice == "both" || cfg.DashboardChoice == "casaos" || cfg.DashboardChoice == ""

	return fmt.Sprintf(`INSTANT_HOSTNAME=%q
INSTANT_SERVER_NAME=%q
INSTANT_ADDRESS_HOST=%q
INSTANT_ADDRESS_DOMAIN=%q
INSTANT_LOG_RETENTION_DAYS=%q
INSTANT_USERNAME=%q
INSTANT_WIFI_SSID=%q
INSTANT_WIFI_PASSWORD=%q
INSTANT_STATIC_IP_CIDR=%q
INSTANT_STATIC_GATEWAY=%q
INSTANT_STATIC_DNS=%q
INSTANT_USB_TETHERING=%q
INSTANT_IGNORE_LID_SWITCH=%q
INSTANT_SCREEN_BLANK_SEC=%q
INSTANT_BAT_START_THRESH=%q
INSTANT_BAT_STOP_THRESH=%q
INSTANT_UPS_SHUTDOWN_PCT=%q
INSTANT_DISABLE_WIFI_POWERSAVE=%q
INSTANT_ENABLE_COCKPIT=%q
INSTANT_ENABLE_CASAOS=%q
INSTANT_ENABLE_DOCKER="1"
INSTANT_ENABLE_CUSTOM_DASHBOARD="1"
INSTANT_DASHBOARD_PORT=%q
INSTANT_SSH_PORT=%q
INSTANT_ENABLE_UFW=%q
INSTANT_ENABLE_FAIL2BAN=%q
INSTANT_ENABLE_TCP_BBR=%q
INSTANT_VM_SWAPPINESS=%q
INSTANT_INOTIFY_MAX_WATCHES=%q
INSTANT_ENABLE_AUTO_UPDATES=%q
INSTANT_CUSTOM_APT_PACKAGES=%q
INSTANT_ENABLE_TAILSCALE=%q
INSTANT_TAILSCALE_AUTHKEY=%q
INSTANT_TAILSCALE_MODE=%q
INSTANT_CLUSTER_MODE=%q
INSTANT_CLUSTER_POLICY=%q
INSTANT_CPU_GOVERNOR=%q
INSTANT_DISABLE_TURBO=%q
INSTANT_INSTALL_MODE=%q
INSTANT_SELECTED_APPS=%q
`,
		host,
		serverName,
		addrHost,
		addrDomain,
		strconv.Itoa(logDays),
		user,
		strings.TrimSpace(cfg.WifiSSID),
		cfg.WifiPassword,
		strings.TrimSpace(cfg.StaticIPCIDR),
		strings.TrimSpace(cfg.StaticGateway),
		strings.Join(cfg.StaticDNS, ","),
		bool01(cfg.EnableUSBTethering),
		bool01(cfg.IgnoreLidSwitch),
		strconv.Itoa(blankSec),
		strconv.Itoa(batStart),
		strconv.Itoa(batStop),
		strconv.Itoa(upsPct),
		bool01(cfg.DisableWifiPowerSave),
		bool01(enableCockpit),
		bool01(enableCasaOS),
		strconv.Itoa(dashPort),
		strconv.Itoa(sshPort),
		bool01(cfg.EnableUFWFirewall),
		bool01(cfg.EnableFail2ban),
		bool01(cfg.EnableTCPBBR),
		strconv.Itoa(swappiness),
		strconv.Itoa(inotifyWatches),
		bool01(cfg.EnableAutoUpdates),
		strings.Join(cfg.CustomAptPackages, " "),
		bool01(cfg.EnableTailscale),
		strings.TrimSpace(cfg.TailscaleAuthKey),
		tsMode,
		clusterMode,
		clusterPol,
		gov,
		bool01(cfg.DisableCPUTurboBoost),
		cfg.InstallMode,
		strings.Join(cfg.SelectedApps, ","),
	)
}
