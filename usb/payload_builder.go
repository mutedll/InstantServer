// usb/payload_builder.go
package usb

import (
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type CustomSambaShare struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Comment    string `json:"comment,omitempty"`
	ReadOnly   bool   `json:"read_only,omitempty"`
	GuestOK    bool   `json:"guest_ok,omitempty"`
	AppleFruit bool   `json:"apple_fruit,omitempty"`
}

type ServerProfileConfig struct {
	Hostname                    string                 `json:"hostname"`
	Username                    string                 `json:"username"`
	Password                    string                 `json:"password"`
	WifiSSID                    string                 `json:"wifi_ssid"`
	WifiPassword                string                 `json:"wifi_password"`
	EnableUSBTethering          bool                   `json:"enable_usb_tethering"`
	DisableWifiPowerSave        bool                   `json:"disable_wifi_power_save"`
	EnableMoshServer            bool                   `json:"enable_mosh_server"`
	StaticIPCIDR                string                 `json:"static_ip_cidr"`
	StaticGateway               string                 `json:"static_gateway"`
	StaticDNS                   []string               `json:"static_dns"`
	SSHAuthorizedKeys           []string               `json:"ssh_authorized_keys"`
	EnableTailscale             bool                   `json:"enable_tailscale"`
	TailscaleAuthKey            string                 `json:"tailscale_authkey"`
	TailscaleMode               string                 `json:"tailscale_mode"`
	ClusterMode                 string                 `json:"cluster_mode"`
	ClusterStoragePolicy        string                 `json:"cluster_storage_policy"`
	ClusterConfirmWipeSecondary string                 `json:"cluster_confirm_wipe_secondary"`
	DeviceFormFactor            string                 `json:"device_form_factor"`
	DashboardChoice             string                 `json:"dashboard_choice"`
	DashboardPort               int                    `json:"dashboard_port"`
	EnableAutoUpdates           bool                   `json:"enable_auto_updates"`
	EnableIPhoneAutoBackup      bool                   `json:"enable_iphone_auto_backup,omitempty"`
	IPhoneBackupMode            string                 `json:"iphone_backup_mode,omitempty"`
	SelectedApps                []string               `json:"selected_apps"`
	CustomApps                  []CustomAppEntry       `json:"custom_apps"`
	MinecraftConfig             *MinecraftServerConfig `json:"minecraft_config,omitempty"`
	AppOverrides                map[string]AppOverride `json:"app_overrides,omitempty"`
	CustomConfigFiles           map[string]string      `json:"custom_config_files,omitempty"`
	CustomAptPackages           []string               `json:"custom_apt_packages,omitempty"`
	CustomSambaShares           []CustomSambaShare     `json:"custom_samba_shares,omitempty"`
	CustomSystemdUnits          map[string]string      `json:"custom_systemd_units,omitempty"`
	CustomUdevRules             map[string]string      `json:"custom_udev_rules,omitempty"`
	CustomPostInstallScript     string                 `json:"custom_post_install_script,omitempty"`
	InstallMode                 string                 `json:"install_mode"`
	DualBootAllocationGB        int                    `json:"dual_boot_Allocation_gb"`
	ConfirmEraseMainDrive       bool                   `json:"confirm_erase_main_drive"`
	ConfirmWipeServerData       string                 `json:"confirm_wipe_server_data,omitempty"`
	UbuntuRelease               string                 `json:"ubuntu_release"`
	StorageLayout               string                 `json:"storage_layout"`
	BootMode                    string                 `json:"boot_mode"`
	USBPartitionScheme          string                 `json:"usb_partition_scheme"`
	IgnoreLidSwitch             bool                   `json:"ignore_lid_switch"`
	ScreenBlankSeconds          int                    `json:"screen_blank_seconds"`
	BatteryChargeStartPct       int                    `json:"battery_charge_start_pct"`
	BatteryChargeStopPct        int                    `json:"battery_charge_stop_pct"`
	UPSShutdownBatteryPct       int                    `json:"ups_shutdown_battery_pct"`
	EnableLowRAMZram            bool                   `json:"enable_low_ram_zram"`
	CPUgovernor                 string                 `json:"cpu_governor"`
	DisableCPUTurboBoost        bool                   `json:"disable_cpu_turbo_boost"`
	SSHPort                     int                    `json:"ssh_port"`
	EnableUFWFirewall           bool                   `json:"enable_ufw_firewall"`
	EnableFail2ban              bool                   `json:"enable_fail2ban"`
	EnableTCPBBR                bool                   `json:"enable_tcp_bbr"`
	VMSwappiness                int                    `json:"vm_swappiness"`
	InotifyMaxWatches           int                    `json:"inotify_max_watches"`
	CustomISOPath               string                 `json:"custom_iso_path,omitempty"`
	SelectedUSBDevice           string                 `json:"selected_usb_device"`
}

type AppOverride struct {
	HostPort    int               `json:"host_port,omitempty"`
	MemoryLimit string            `json:"memory_limit,omitempty"`
	EnvVars     map[string]string `json:"env_vars,omitempty"`
	ConfigFiles map[string]string `json:"config_files,omitempty"`
}

type CustomAppEntry struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Category    string   `json:"category"`
	Image       string   `json:"image"`
	Port        int      `json:"port"`
	Volumes     []string `json:"volumes"`
	Description string   `json:"description"`
	ComposeYAML string   `json:"compose_yaml"`
}

const sha512CryptAlphabet = "./0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

func b64From24Bit(b2, b1, b0 byte, n int) string {
	w := uint32(b2)<<16 | uint32(b1)<<8 | uint32(b0)
	var out [4]byte
	for i := 0; i < n; i++ {
		out[i] = sha512CryptAlphabet[w&0x3f]
		w >>= 6
	}
	return string(out[:n])
}

// Sha512Crypt implements the Ulrich Drepper glibc $6$ SHA-512 crypt algorithm
// used by Ubuntu /etc/shadow, PAM, and cloud-init autoinstall.
func Sha512Crypt(password string, salt string) string {
	if strings.HasPrefix(password, "$6$") || strings.HasPrefix(password, "$y$") || strings.HasPrefix(password, "$5$") {
		return password
	}
	cleanSalt := strings.Map(func(r rune) rune {
		if strings.ContainsRune(sha512CryptAlphabet, r) {
			return r
		}
		return -1
	}, salt)
	if len(cleanSalt) == 0 {
		cleanSalt = "instantserverlts"
	}
	if len(cleanSalt) > 16 {
		cleanSalt = cleanSalt[:16]
	}

	key := []byte(password)
	saltBytes := []byte(cleanSalt)
	keyLen := len(key)

	altHash := sha512.New()
	altHash.Write(key)
	altHash.Write(saltBytes)
	altHash.Write(key)
	altSum := altHash.Sum(nil)

	mainHash := sha512.New()
	mainHash.Write(key)
	mainHash.Write(saltBytes)
	for cnt := keyLen; cnt > 64; cnt -= 64 {
		mainHash.Write(altSum)
	}
	if keyLen > 0 {
		rem := keyLen % 64
		if rem == 0 {
			rem = 64
		}
		mainHash.Write(altSum[:rem])
	}
	for cnt := keyLen; cnt > 0; cnt >>= 1 {
		if (cnt & 1) != 0 {
			mainHash.Write(altSum)
		} else {
			mainHash.Write(key)
		}
	}
	digestA := mainHash.Sum(nil)

	dpHash := sha512.New()
	for i := 0; i < keyLen; i++ {
		dpHash.Write(key)
	}
	dpSum := dpHash.Sum(nil)
	pBytes := make([]byte, 0, keyLen)
	for cnt := keyLen; cnt >= 64; cnt -= 64 {
		pBytes = append(pBytes, dpSum...)
	}
	if keyLen%64 > 0 {
		pBytes = append(pBytes, dpSum[:keyLen%64]...)
	}

	dsHash := sha512.New()
	for i := 0; i < 16+int(digestA[0]); i++ {
		dsHash.Write(saltBytes)
	}
	dsSum := dsHash.Sum(nil)
	sBytes := make([]byte, len(saltBytes))
	copy(sBytes, dsSum[:len(saltBytes)])

	curDigest := digestA
	for round := 0; round < 5000; round++ {
		cHash := sha512.New()
		if (round & 1) != 0 {
			cHash.Write(pBytes)
		} else {
			cHash.Write(curDigest)
		}
		if round%3 != 0 {
			cHash.Write(sBytes)
		}
		if round%7 != 0 {
			cHash.Write(pBytes)
		}
		if (round & 1) != 0 {
			cHash.Write(curDigest)
		} else {
			cHash.Write(pBytes)
		}
		curDigest = cHash.Sum(nil)
	}

	var b strings.Builder
	b.WriteString("$6$")
	b.WriteString(cleanSalt)
	b.WriteByte('$')
	b.WriteString(b64From24Bit(curDigest[0], curDigest[21], curDigest[42], 4))
	b.WriteString(b64From24Bit(curDigest[22], curDigest[43], curDigest[1], 4))
	b.WriteString(b64From24Bit(curDigest[44], curDigest[2], curDigest[23], 4))
	b.WriteString(b64From24Bit(curDigest[3], curDigest[24], curDigest[45], 4))
	b.WriteString(b64From24Bit(curDigest[25], curDigest[46], curDigest[4], 4))
	b.WriteString(b64From24Bit(curDigest[47], curDigest[5], curDigest[26], 4))
	b.WriteString(b64From24Bit(curDigest[6], curDigest[27], curDigest[48], 4))
	b.WriteString(b64From24Bit(curDigest[28], curDigest[49], curDigest[7], 4))
	b.WriteString(b64From24Bit(curDigest[50], curDigest[8], curDigest[29], 4))
	b.WriteString(b64From24Bit(curDigest[9], curDigest[30], curDigest[51], 4))
	b.WriteString(b64From24Bit(curDigest[31], curDigest[52], curDigest[10], 4))
	b.WriteString(b64From24Bit(curDigest[53], curDigest[11], curDigest[32], 4))
	b.WriteString(b64From24Bit(curDigest[12], curDigest[33], curDigest[54], 4))
	b.WriteString(b64From24Bit(curDigest[34], curDigest[55], curDigest[13], 4))
	b.WriteString(b64From24Bit(curDigest[56], curDigest[14], curDigest[35], 4))
	b.WriteString(b64From24Bit(curDigest[15], curDigest[36], curDigest[57], 4))
	b.WriteString(b64From24Bit(curDigest[37], curDigest[58], curDigest[16], 4))
	b.WriteString(b64From24Bit(curDigest[59], curDigest[17], curDigest[38], 4))
	b.WriteString(b64From24Bit(curDigest[18], curDigest[39], curDigest[60], 4))
	b.WriteString(b64From24Bit(curDigest[40], curDigest[61], curDigest[19], 4))
	b.WriteString(b64From24Bit(curDigest[62], curDigest[20], curDigest[41], 4))
	b.WriteString(b64From24Bit(0, 0, curDigest[63], 2))
	return b.String()
}

var (
	rePortMapping  = regexp.MustCompile(`(?:-p|--publish)\s+(?:[0-9.]+:)?(\d{2,5}):(\d{2,5})`)
	reColonPort    = regexp.MustCompile(`:(\d{2,5})\b`)
	reSlugClean    = regexp.MustCompile(`[^a-z0-9_-]+`)
	reHostnameSafe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]{0,61}$`)
	reUsernameSafe = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)
)

func ValidateProfileConfig(cfg ServerProfileConfig) error {
	host := strings.TrimSpace(cfg.Hostname)
	if host == "" || !reHostnameSafe.MatchString(host) {
		return errors.New("IS-CFG-001: Server hostname must be 1-62 alphanumeric characters or hyphens")
	}
	user := strings.TrimSpace(cfg.Username)
	if user == "" || !reUsernameSafe.MatchString(user) {
		return errors.New("IS-CFG-002: Admin username must start with a lowercase letter or underscore")
	}
	if strings.TrimSpace(cfg.WifiSSID) != "" && len(cfg.WifiPassword) > 0 && len(cfg.WifiPassword) < 8 {
		return errors.New("IS-CFG-003: WPA2/WPA3 Wi-Fi password must be at least 8 characters")
	}
	if cfg.InstallMode == "dual_boot" {
		if cfg.DualBootAllocationGB < 15 || cfg.DualBootAllocationGB > 65536 {
			return fmt.Errorf("IS-CFG-004: Dual-boot partition allocation (%d GB) must be between 15 GB and 64 TB (65,536 GB)", cfg.DualBootAllocationGB)
		}
	}
	if cfg.InstallMode == "auto_erase" && !cfg.ConfirmEraseMainDrive {
		return errors.New("IS-CFG-005: Auto-erase mode requires explicit confirmation")
	}
	if cfg.InstallMode == "reinstall_wipe_data" && !cfg.ConfirmEraseMainDrive && cfg.ConfirmWipeServerData != "WIPE-SERVER-DATA" {
		return errors.New("IS-CFG-006: Reinstalling without retaining server data requires explicit WIPE-SERVER-DATA confirmation")
	}
	if cfg.ClusterMode != "" && cfg.ClusterMode != "standalone" && cfg.ClusterStoragePolicy == "wipe_secondary_expand" {
		if !strings.HasPrefix(cfg.ClusterConfirmWipeSecondary, "WIPE-") {
			return errors.New("IS-CLU-401: Wiping secondary server storage requires explicit WIPE confirmation token")
		}
	}
	return nil
}

func ParseCustomAppInput(rawInput string, customTitle string, customPort int) CustomAppEntry {
	raw := strings.TrimSpace(rawInput)
	image := raw
	hostPort := customPort
	internalPort := 0

	if m := rePortMapping.FindStringSubmatch(raw); len(m) >= 3 {
		if hp, err := strconv.Atoi(m[1]); err == nil && hp > 0 {
			hostPort = hp
		}
		if ip, err := strconv.Atoi(m[2]); err == nil && ip > 0 {
			internalPort = ip
		}
	} else if m := reColonPort.FindStringSubmatch(raw); len(m) >= 2 && !strings.HasPrefix(raw, "http") {
		if p, err := strconv.Atoi(m[1]); err == nil && p >= 80 && p <= 65535 {
			hostPort = p
		}
	}
	if hostPort <= 0 {
		hostPort = 8080
	}
	if internalPort <= 0 {
		internalPort = hostPort
	}

	if strings.Contains(raw, "hub.docker.com/r/") {
		tail := strings.SplitN(raw, "hub.docker.com/r/", 2)[1]
		parts := strings.Split(strings.Trim(tail, "/"), "/")
		if len(parts) >= 2 {
			image = parts[0] + "/" + parts[1]
		}
	} else if strings.Contains(raw, "github.com/") {
		tail := strings.SplitN(raw, "github.com/", 2)[1]
		parts := strings.Split(strings.Trim(tail, "/"), "/")
		if len(parts) >= 2 {
			repo := strings.TrimSuffix(parts[1], ".git")
			image = "ghcr.io/" + strings.ToLower(parts[0]+"/"+repo) + ":latest"
		}
	} else if strings.HasPrefix(raw, "docker run") {
		tokens := strings.Fields(raw)
		if len(tokens) > 0 {
			image = tokens[len(tokens)-1]
		}
	}

	if !strings.Contains(image, ":") {
		image += ":latest"
	}

	imgSegments := strings.Split(image, "/")
	baseName := strings.Split(imgSegments[len(imgSegments)-1], ":")[0]
	slug := reSlugClean.ReplaceAllString(strings.ToLower(baseName), "-")
	slug = strings.Trim(slug, "-_")
	if slug == "" {
		slug = "custom-app"
	}
	appID := "custom_" + strings.ReplaceAll(slug, "-", "_")
	title := strings.TrimSpace(customTitle)
	if title == "" {
		words := strings.Fields(strings.NewReplacer("-", " ", "_", " ").Replace(slug))
		for i, w := range words {
			if len(w) > 0 {
				words[i] = strings.ToUpper(w[:1]) + w[1:]
			}
		}
		title = strings.Join(words, " ")
	}

	vols := []string{fmt.Sprintf("/DATA/AppData/%s:/data", slug)}
	compose := fmt.Sprintf(`name: %s
services:
  %s:
    image: %s
    container_name: %s
    restart: unless-stopped
    security_opt:
      - no-new-privileges:true
    environment:
      - TZ=UTC
      - PUID=1000
      - PGID=1000
    ports:
      - "%d:%d"
    volumes:
      - /DATA/AppData/%s:/data
`, appID, appID, image, appID, hostPort, internalPort, slug)

	return CustomAppEntry{
		ID:          appID,
		Title:       title,
		Category:    "Security & Tools",
		Image:       image,
		Port:        hostPort,
		Volumes:     vols,
		Description: fmt.Sprintf("Custom container (%s) on port %d.", image, hostPort),
		ComposeYAML: compose,
	}
}

func GenerateAutoinstallUserData(cfg ServerProfileConfig) string {
	host := strings.TrimSpace(cfg.Hostname)
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
		// Per Canonical Subiquity Reference, LVM defaults to `scaled` (which only uses 50% or max 100 GiB of large disks);
		// `sizing-policy: all` ensures 100% of the server partition is allocated to the root LV.
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

	// Note: Subiquity strips YAML comments when writing /autoinstall.yaml, so select_target_disk.sh
	// uses Python3 (yaml.safe_load/safe_dump) during early-commands to sanitize `network.wifis` out of
	// /autoinstall.yaml and wraps /usr/sbin/netplan with a 10s D-Bus timeout guard before Subiquity runs
	// Network/apply_config. Target Wi-Fi is configured in /target/etc/netplan/60-instant-wifi.yaml
	// using the real kernel interface name (e.g. wlx7cc2c626d7ec) during late-commands.
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

	// Stop casper-md5check immediately so it does not thrash the USB stick during installation,
	// mount CIDATA & INSTANTUSB rw+sync, start crash-proof live USB logging (`start_usb_logger.sh`),
	// inhibit sleep/suspend, ignore laptop lid close, and run select_target_disk.sh.
	liveNoSleepCmds := `    - systemctl stop casper-md5check.service 2>/dev/null || killall -9 casper-md5check 2>/dev/null || true
    - mkdir -p /mnt/cidata /mnt/usb_logs
    - cdev=$(blkid -L CIDATA 2>/dev/null || blkid -L cidata 2>/dev/null || true); [ -n "$cdev" ] && (mount -o rw,sync,flush,noatime "$cdev" /mnt/cidata 2>/dev/null || mount -o remount,rw,sync,flush /mnt/cidata 2>/dev/null || true)
    - udev=$(blkid -L INSTANTUSB 2>/dev/null || true); [ -n "$udev" ] && (mount -o rw,sync,flush,noatime "$udev" /mnt/usb_logs 2>/dev/null || true)
    - bash /mnt/cidata/start_usb_logger.sh 2>/dev/null || bash /cdrom/cidata/start_usb_logger.sh 2>/dev/null || true
    - rfkill unblock all 2>/dev/null || true
    - systemctl mask sleep.target suspend.target hibernate.target hybrid-sleep.target 2>/dev/null || true
    - mkdir -p /etc/systemd/logind.conf.d && printf "[Login]\nHandleLidSwitch=ignore\nHandleLidSwitchExternalPower=ignore\nHandleLidSwitchDocked=ignore\nIdleAction=ignore\n" > /etc/systemd/logind.conf.d/00-live-nosleep.conf && systemctl restart systemd-logind 2>/dev/null || true
    - touch /run/casper-no-prompt 2>/dev/null || true
    - bash /mnt/cidata/select_target_disk.sh 2>/dev/null || bash /cdrom/cidata/select_target_disk.sh 2>/dev/null || true
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
    - bash /mnt/cidata/dynamic_dualboot_storage.sh %d 2>/dev/null || bash /cdrom/cidata/dynamic_dualboot_storage.sh %d 2>/dev/null || bash /cdrom/dynamic_dualboot_storage.sh %d 2>/dev/null || true
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

	case "interactive_select":
		earlyCommandsBlock = fmt.Sprintf(`  early-commands:
%s
`, liveNoSleepCmds)
		storageBlock = `  interactive-sections:
    - storage`

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

	return fmt.Sprintf(`#cloud-config
autoinstall:
  version: 1
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
    - touch /run/casper-no-prompt 2>/dev/null || true
    - curtin in-target -- chmod +x /opt/instant-server/laptop_hardening.sh 2>/dev/null || true
    - curtin in-target -- bash /opt/instant-server/laptop_hardening.sh >> /mnt/cidata/INSTANT-SERVER-INSTALL.log 2>&1 || true
    - cp -f /mnt/cidata/INSTANT-SERVER-INSTALL.log /mnt/usb_logs/INSTANT-SERVER-INSTALL.log 2>/dev/null || true
    - cp -f /mnt/cidata/INSTANT-SERVER-INSTALL.log /target/var/log/instant-server-install.log 2>/dev/null || true
    - bash /mnt/cidata/dump_error_logs_to_usb.sh SUCCESS 2>/dev/null || true
    - sync
  error-commands:
    - bash /mnt/cidata/dump_error_logs_to_usb.sh ERROR 2>/dev/null || bash /cdrom/cidata/dump_error_logs_to_usb.sh ERROR 2>/dev/null || true
    - sync
`, host, user, hashedPass, sshKeysBlock, ethSection, usbTetherSection, wifiSection, earlyCommandsBlock, storageBlock, extraLateCommands)
}

func GenerateTargetEnvConf(cfg ServerProfileConfig) string {
	bool01 := func(b bool) string {
		if b {
			return "1"
		}
		return "0"
	}
	host := strings.TrimSpace(cfg.Hostname)
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
	if blankSec <= 0 {
		blankSec = 60
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

func NormalizeShellScriptLF(raw []byte) []byte {
	s := strings.ReplaceAll(string(raw), "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	return []byte(s)
}

func StageCIDATAPayload(targetDir string, cfg ServerProfileConfig, hardeningScript []byte, dashboardHTML []byte) ([]string, error) {
	if err := ValidateProfileConfig(cfg); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return nil, err
	}

	var writtenFiles []string
	writeItem := func(relPath string, data []byte, perm os.FileMode) error {
		full := filepath.Join(targetDir, relPath)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(full, data, perm); err != nil {
			return err
		}
		writtenFiles = append(writtenFiles, relPath)
		return nil
	}

	userData := []byte(GenerateAutoinstallUserData(cfg))
	if err := writeItem("user-data", userData, 0644); err != nil {
		return nil, err
	}
	metaData := []byte("instance-id: " + cfg.Hostname + "-01\nlocal-hostname: " + cfg.Hostname + "\n")
	if err := writeItem("meta-data", metaData, 0644); err != nil {
		return nil, err
	}
	// Crash-Proof Real-Time USB Logger:
	// Stops casper-md5check immediately, mounts both CIDATA and INSTANTUSB with `rw,sync,flush,noatime`,
	// and streams every boot, hardware, early-commands, Subiquity, Curtin, Netplan, and kernel message
	// directly onto the USB drive (`INSTANT-SERVER-INSTALL.log`) every 2 seconds with hardware `sync`.
	startUSBLoggerScript := []byte(`#!/usr/bin/env bash
set -uo pipefail
systemctl stop casper-md5check.service 2>/dev/null || killall -9 casper-md5check 2>/dev/null || true
mkdir -p /mnt/cidata /mnt/usb_logs
CDEV=$(blkid -L CIDATA 2>/dev/null || blkid -L cidata 2>/dev/null || true)
UDEV=$(blkid -L INSTANTUSB 2>/dev/null || true)
if [ -n "$CDEV" ]; then
    mount -o rw,sync,flush,noatime "$CDEV" /mnt/cidata 2>/dev/null || mount -o remount,rw,sync,flush /mnt/cidata 2>/dev/null || true
fi
if [ -n "$UDEV" ]; then
    mount -o rw,sync,flush,noatime "$UDEV" /mnt/usb_logs 2>/dev/null || mount -o remount,rw,sync,flush /mnt/usb_logs 2>/dev/null || true
fi
mkdir -p /mnt/cidata/logs /mnt/usb_logs/logs 2>/dev/null || true

LOG_A="/mnt/cidata/INSTANT-SERVER-INSTALL.log"
LOG_B="/mnt/usb_logs/INSTANT-SERVER-INSTALL.log"

write_both() {
    cat >> "$LOG_A" 2>/dev/null || true
    if mountpoint -q /mnt/usb_logs 2>/dev/null; then
        cp -f "$LOG_A" "$LOG_B" 2>/dev/null || true
    fi
    sync
}

{
    echo "================================================================================"
    echo " INSTANT SERVER LIVE USB INSTALLATION & DIAGNOSTICS LOG"
    echo " Started at (UTC): $(date -u '+%Y-%m-%d %H:%M:%S UTC')"
    echo "================================================================================"
    if [ -d /sys/firmware/efi ]; then
        echo "[BOOT MODE] UEFI Mode"
    else
        echo "[BOOT MODE] Legacy BIOS / CSM Mode"
    fi
    echo "[KERNEL CMDLINE] $(cat /proc/cmdline 2>/dev/null || true)"
    echo "[HARDWARE DMI] Vendor=$(cat /sys/class/dmi/id/sys_vendor 2>/dev/null || echo unknown) | Product=$(cat /sys/class/dmi/id/product_name 2>/dev/null || echo unknown) | BIOS=$(cat /sys/class/dmi/id/bios_version 2>/dev/null || echo unknown)"
    echo "--- MEMORY ---"
    free -h 2>/dev/null || true
    echo "--- BLOCK DEVICES (lsblk) ---"
    lsblk -o NAME,SIZE,TYPE,FSTYPE,LABEL,RM,ROTA,MOUNTPOINT,MODEL 2>/dev/null || true
    echo "--- BLKID ---"
    blkid 2>/dev/null || true
    echo "--- NETWORK INTERFACES (ip -br link / addr) ---"
    ip -br link 2>/dev/null || true
    ip -br addr 2>/dev/null || true
    echo "--- PCI & USB NETWORK ADAPTERS ---"
    lspci -nnk 2>/dev/null | grep -A3 -iE "net|eth|wireless|wifi" || true
    lsusb 2>/dev/null || true
    rfkill list 2>/dev/null || true
    echo "================================================================================"
} | write_both

if [ ! -f /run/instant-usb-log-daemon.pid ]; then
    nohup bash /mnt/cidata/usb_log_daemon.sh >/dev/null 2>&1 &
    echo $! > /run/instant-usb-log-daemon.pid
fi
exit 0
`)
	if err := writeItem("start_usb_logger.sh", NormalizeShellScriptLF(startUSBLoggerScript), 0755); err != nil {
		return nil, err
	}

	usbLogDaemonScript := []byte(`#!/usr/bin/env bash
set -uo pipefail
LOG_A="/mnt/cidata/INSTANT-SERVER-INSTALL.log"
LOG_B="/mnt/usb_logs/INSTANT-SERVER-INSTALL.log"
LAST_J_LINES=0
LAST_SUB_LINES=0
LAST_CUR_LINES=0

while true; do
    {
        if command -v journalctl >/dev/null 2>&1; then
            TOTAL_J=$(journalctl -b --no-pager 2>/dev/null | wc -l || echo 0)
            if [ "${TOTAL_J:-0}" -gt "$LAST_J_LINES" ]; then
                NEW_CNT=$((TOTAL_J - LAST_J_LINES))
                echo "--- [JOURNALCTL +${NEW_CNT} lines @ $(date -u '+%H:%M:%S')] ---"
                journalctl -b --no-pager 2>/dev/null | tail -n "$NEW_CNT" | grep -E -i "subiquity|curtin|netplan|networkd|wpa|cloud-init|instant-server|error|fail|warn|parted|sfdisk|sgdisk|grub|mount" || true
                LAST_J_LINES="$TOTAL_J"
            fi
        fi
        if [ -f /var/log/installer/subiquity-server-debug.log ]; then
            TOTAL_S=$(wc -l < /var/log/installer/subiquity-server-debug.log 2>/dev/null || echo 0)
            if [ "${TOTAL_S:-0}" -gt "$LAST_SUB_LINES" ]; then
                NEW_S=$((TOTAL_S - LAST_SUB_LINES))
                echo "--- [SUBIQUITY DEBUG +${NEW_S} lines @ $(date -u '+%H:%M:%S')] ---"
                tail -n "$NEW_S" /var/log/installer/subiquity-server-debug.log 2>/dev/null | tail -n 80 || true
                LAST_SUB_LINES="$TOTAL_S"
            fi
            cp -f /var/log/installer/subiquity-server-debug.log /mnt/cidata/logs/subiquity-server-debug.log 2>/dev/null || true
        fi
        if [ -f /var/log/installer/curtin-install.log ]; then
            TOTAL_C=$(wc -l < /var/log/installer/curtin-install.log 2>/dev/null || echo 0)
            if [ "${TOTAL_C:-0}" -gt "$LAST_CUR_LINES" ]; then
                NEW_C=$((TOTAL_C - LAST_CUR_LINES))
                echo "--- [CURTIN INSTALL +${NEW_C} lines @ $(date -u '+%H:%M:%S')] ---"
                tail -n "$NEW_C" /var/log/installer/curtin-install.log 2>/dev/null | tail -n 80 || true
                LAST_CUR_LINES="$TOTAL_C"
            fi
            cp -f /var/log/installer/curtin-install.log /mnt/cidata/logs/curtin-install.log 2>/dev/null || true
        fi
    } >> "$LOG_A" 2>/dev/null || true

    if mountpoint -q /mnt/usb_logs 2>/dev/null; then
        cp -f "$LOG_A" "$LOG_B" 2>/dev/null || true
        cp -rf /mnt/cidata/logs/* /mnt/usb_logs/logs/ 2>/dev/null || true
    fi
    sync
    sleep 2
done
`)
	if err := writeItem("usb_log_daemon.sh", NormalizeShellScriptLF(usbLogDaemonScript), 0755); err != nil {
		return nil, err
	}

	dumpErrorLogsScript := []byte(`#!/usr/bin/env bash
set -uo pipefail
STATUS="${1:-ERROR}"
mkdir -p /mnt/cidata/logs /mnt/usb_logs/logs
CDEV=$(blkid -L CIDATA 2>/dev/null || blkid -L cidata 2>/dev/null || true)
UDEV=$(blkid -L INSTANTUSB 2>/dev/null || true)
[ -n "$CDEV" ] && (mount -o rw,sync,flush,noatime "$CDEV" /mnt/cidata 2>/dev/null || mount -o remount,rw,sync,flush /mnt/cidata 2>/dev/null || true)
[ -n "$UDEV" ] && (mount -o rw,sync,flush,noatime "$UDEV" /mnt/usb_logs 2>/dev/null || mount -o remount,rw,sync,flush /mnt/usb_logs 2>/dev/null || true)

LOG_A="/mnt/cidata/INSTANT-SERVER-INSTALL.log"
LOG_B="/mnt/usb_logs/INSTANT-SERVER-INSTALL.log"

{
    echo ""
    echo "================================================================================"
    echo " FINAL INSTALLER SNAPSHOT (STATUS: ${STATUS}) @ $(date -u '+%Y-%m-%d %H:%M:%S UTC')"
    echo "================================================================================"
    echo "--- /autoinstall.yaml ---"
    cat /autoinstall.yaml 2>/dev/null || true
    echo "--- LIVE /etc/netplan/*.yaml ---"
    head -n 100 /etc/netplan/*.yaml 2>/dev/null || true
    echo "--- TARGET /target/etc/netplan/*.yaml ---"
    head -n 100 /target/etc/netplan/*.yaml 2>/dev/null || true
    echo "--- NETPLAN & IP STATUS ---"
    ip -br link 2>/dev/null || true
    ip -br addr 2>/dev/null || true
    ip route 2>/dev/null || true
    echo "--- BLOCK DEVICES & MOUNTS ---"
    lsblk -o NAME,SIZE,TYPE,FSTYPE,LABEL,MOUNTPOINT 2>/dev/null || true
    echo "--- SUBIQUITY TRACEBACK / CRASH FILES (/var/crash) ---"
    for cf in /var/crash/*; do
        [ -f "$cf" ] || continue
        echo "Crash file: $cf"
        tail -n 120 "$cf" 2>/dev/null || true
    done
    echo "--- LAST 120 LINES OF /var/log/installer/subiquity-server-debug.log ---"
    tail -n 120 /var/log/installer/subiquity-server-debug.log 2>/dev/null || true
    echo "--- LAST 120 LINES OF /var/log/installer/curtin-install.log ---"
    tail -n 120 /var/log/installer/curtin-install.log 2>/dev/null || true
    echo "--- LAST 100 LINES OF DMESG ---"
    dmesg 2>/dev/null | tail -n 100 || true
    echo "================================================================================"
} >> "$LOG_A" 2>/dev/null || true

cp -f /var/log/installer/subiquity-server-debug.log /mnt/cidata/logs/ 2>/dev/null || true
cp -f /var/log/installer/curtin-install.log /mnt/cidata/logs/ 2>/dev/null || true
cp -f /autoinstall.yaml /mnt/cidata/logs/autoinstall-used.yaml 2>/dev/null || true

if mountpoint -q /mnt/usb_logs 2>/dev/null; then
    cp -f "$LOG_A" "$LOG_B" 2>/dev/null || true
    cp -rf /mnt/cidata/logs/* /mnt/usb_logs/logs/ 2>/dev/null || true
fi
sync
sync
if [ "$STATUS" = "ERROR" ]; then
    printf "
[INSTANT SERVER] Installation error logs saved to INSTANT-SERVER-INSTALL.log on your USB drive.
" > /dev/console 2>/dev/null || true
    printf "
[INSTANT SERVER] Installation error logs saved to INSTANT-SERVER-INSTALL.log on your USB drive.
" > /dev/tty1 2>/dev/null || true
fi
exit 0
`)
	if err := writeItem("dump_error_logs_to_usb.sh", NormalizeShellScriptLF(dumpErrorLogsScript), 0755); err != nil {
		return nil, err
	}

	selectDiskHelper := []byte(`#!/usr/bin/env bash
set -uo pipefail
LOG_A="/mnt/cidata/INSTANT-SERVER-INSTALL.log"
log_usb() {
    printf '[select_target_disk] %s
' "$*" | tee -a "$LOG_A" 2>/dev/null || true
    if mountpoint -q /mnt/usb_logs 2>/dev/null; then
        cp -f "$LOG_A" /mnt/usb_logs/INSTANT-SERVER-INSTALL.log 2>/dev/null || true
    fi
    sync
}

# 0. Stop casper-md5check immediately so it stops reading all 3 GB of the USB drive during installation
systemctl stop casper-md5check.service 2>/dev/null || killall -9 casper-md5check 2>/dev/null || true

# 0b. Install a fault-tolerant /usr/sbin/netplan wrapper on the live host so Subiquity's D-Bus call
# (busctl -> io.netplan.Netplan -> /usr/sbin/netplan apply, which has a 25s D-Bus timeout) can NEVER
# time out or exit with code 1 (`Call failed: Connection timed out`).
chmod 600 /etc/netplan/*.yaml /lib/netplan/*.yaml /run/netplan/*.yaml 2>/dev/null || true
if [ -x /usr/sbin/netplan ] && [ ! -f /usr/sbin/netplan.real ]; then
    cp -a /usr/sbin/netplan /usr/sbin/netplan.real
    cat << 'NPEOF' > /usr/sbin/netplan
#!/usr/bin/env bash
chmod 600 /etc/netplan/*.yaml /lib/netplan/*.yaml /run/netplan/*.yaml 2>/dev/null || true
if [ "${1:-}" = "apply" ]; then
    timeout 10 /usr/sbin/netplan.real "$@" 2>/dev/null || {
        /usr/sbin/netplan.real generate 2>/dev/null || true
        networkctl reload 2>/dev/null || true
    }
    exit 0
fi
exec /usr/sbin/netplan.real "$@"
NPEOF
    chmod 755 /usr/sbin/netplan
    log_usb "Installed fault-tolerant /usr/sbin/netplan wrapper (10s D-Bus timeout guard)."
fi

# 1. Identify both the ISO/cdrom disk and the CIDATA/INSTANTUSB disk so Subiquity NEVER overwrites installer media
CDROM_PART=$(findmnt -n -o SOURCE /cdrom 2>/dev/null || true)
CIDATA_PART=$(blkid -L CIDATA 2>/dev/null || blkid -L cidata 2>/dev/null || true)
USB_DISK=""
CIDATA_DISK=""
if [ -n "$CDROM_PART" ]; then
    PK=$(lsblk -no PKNAME "$CDROM_PART" 2>/dev/null | head -n1)
    if [ -n "$PK" ]; then USB_DISK="/dev/${PK}"; else USB_DISK="$CDROM_PART"; fi
fi
if [ -n "$CIDATA_PART" ]; then
    PK2=$(lsblk -no PKNAME "$CIDATA_PART" 2>/dev/null | head -n1)
    if [ -n "$PK2" ]; then CIDATA_DISK="/dev/${PK2}"; else CIDATA_DISK="$CIDATA_PART"; fi
    [ -z "$USB_DISK" ] && USB_DISK="$CIDATA_DISK"
fi
log_usb "Detected installer media: USB_DISK=${USB_DISK} CIDATA_DISK=${CIDATA_DISK}"

# 2. Guard against infinite reinstall loop if user left the USB plugged in during automatic reboot
mkdir -p /mnt/check_existing
for part in $(lsblk -lnpo NAME,FSTYPE | awk '$2 == "ext4" {print $1}'); do
    [ -n "$USB_DISK" ] && [[ "$part" == "${USB_DISK}"* ]] && continue
    [ -n "$CIDATA_DISK" ] && [[ "$part" == "${CIDATA_DISK}"* ]] && continue
    if mount -o ro "$part" /mnt/check_existing 2>/dev/null; then
        if [ -f "/mnt/check_existing/opt/instant-server/.install_complete_epoch" ]; then
            STAMP=$(cat /mnt/check_existing/opt/instant-server/.install_complete_epoch 2>/dev/null || echo 0)
            NOW=$(date +%s)
            DIFF=$((NOW - STAMP))
            if [ "$DIFF" -ge 0 ] && [ "$DIFF" -lt 1800 ]; then
                umount /mnt/check_existing 2>/dev/null || true
                log_usb "Fresh installation detected on $part (${DIFF}s ago). Rebooting into installed OS..."
                touch /run/casper-no-prompt 2>/dev/null || true
                sync
                reboot -f
                exit 0
            fi
        fi
        umount /mnt/check_existing 2>/dev/null || true
    fi
done

# 3. Pin largest internal non-USB disk in /autoinstall.yaml so a large USB stick never wipes itself
BEST_DISK=""
BEST_SIZE=0
while read -r dname dsize drm dtype; do
    [ "$dtype" = "disk" ] || continue
    [ -n "$USB_DISK" ] && [ "$dname" = "$USB_DISK" ] && continue
    [ -n "$CIDATA_DISK" ] && [ "$dname" = "$CIDATA_DISK" ] && continue
    [[ "$dname" == /dev/loop* || "$dname" == /dev/ram* || "$dname" == /dev/zram* || "$dname" == /dev/sr* ]] && continue
    if [ "$drm" = "0" ] && [ "${dsize:-0}" -gt "$BEST_SIZE" ]; then
        BEST_SIZE="$dsize"
        BEST_DISK="$dname"
    fi
done < <(lsblk -dbnpo NAME,SIZE,RM,TYPE 2>/dev/null)

# Fallback for older laptops whose SATA/eMMC controller marks internal drives with RM=1
if [ -z "$BEST_DISK" ]; then
    while read -r dname dsize drm dtype; do
        [ "$dtype" = "disk" ] || continue
        [ -n "$USB_DISK" ] && [ "$dname" = "$USB_DISK" ] && continue
        [ -n "$CIDATA_DISK" ] && [ "$dname" = "$CIDATA_DISK" ] && continue
        [[ "$dname" == /dev/loop* || "$dname" == /dev/ram* || "$dname" == /dev/zram* || "$dname" == /dev/sr* ]] && continue
        if [ "${dsize:-0}" -gt "$BEST_SIZE" ]; then
            BEST_SIZE="$dsize"
            BEST_DISK="$dname"
        fi
    done < <(lsblk -dbnpo NAME,SIZE,RM,TYPE 2>/dev/null)
fi

if [ -f /mnt/cidata/instant-server-env.conf ]; then
    . /mnt/cidata/instant-server-env.conf
fi

if [ -n "$BEST_DISK" ]; then
    echo "$BEST_DISK" > /run/instant-target-disk
    swapoff -a 2>/dev/null || true
    for p in $(lsblk -lnpo NAME "$BEST_DISK" 2>/dev/null | tail -n +2); do
        umount -lf "$p" 2>/dev/null || true
    done
    # If mode is auto_erase or reinstall_wipe_data, destroy any existing LVM volume groups (e.g. ubuntu-vg)
    # on BEST_DISK now so Subiquity's probert.lvm (`vgchange -ay`) does NOT re-activate dm-0 during Filesystem/_probe.
    if [ "${INSTANT_INSTALL_MODE:-}" = "auto_erase" ] || [ "${INSTANT_INSTALL_MODE:-}" = "reinstall_wipe_data" ]; then
        if [ "$BEST_DISK" != "$USB_DISK" ] && [ "$BEST_DISK" != "$CIDATA_DISK" ] && ! grep -q "^${BEST_DISK}" /proc/mounts 2>/dev/null; then
            log_usb "Cleaning existing LVM volume groups and partition signatures on ${BEST_DISK} for ${INSTANT_INSTALL_MODE}..."
            for vg in $(vgs --noheadings -o vg_name 2>/dev/null || true); do
                lvremove -ff -y "$vg" >/dev/null 2>&1 || true
                vgremove -ff -y "$vg" >/dev/null 2>&1 || true
            done
            for pv in $(pvs --noheadings -o pv_name 2>/dev/null | grep "^${BEST_DISK}" || true); do
                pvremove -ff -y "$pv" >/dev/null 2>&1 || true
            done
            dmsetup remove_all >/dev/null 2>&1 || true
            for p in $(lsblk -lnpo NAME "$BEST_DISK" 2>/dev/null | tail -n +2 | tac); do
                wipefs -a "$p" >/dev/null 2>&1 || true
            done
            wipefs -a "$BEST_DISK" >/dev/null 2>&1 || true
            sgdisk --zap-all "$BEST_DISK" >/dev/null 2>&1 || true
            partprobe "$BEST_DISK" 2>/dev/null || true
        fi
    else
        vgchange -an >/dev/null 2>&1 || true
        mdadm --stop --scan >/dev/null 2>&1 || true
    fi
    if command -v sgdisk >/dev/null 2>&1; then
        sgdisk -e "$BEST_DISK" >/dev/null 2>&1 || true
    fi
    log_usb "Pinned internal target disk: ${BEST_DISK} (${BEST_SIZE} bytes)"
else
    log_usb "WARNING: No separate internal target disk detected beyond USB_DISK=${USB_DISK}"
fi

# 4. Sanitize /autoinstall.yaml using Python3 (since Subiquity's yaml.dump strips YAML comments):
#    - Pin storage.layout.match.path to BEST_DISK
#    - Remove `wifis` (wlan0) from `network` so Subiquity netplan apply and Curtin net-meta never fail
#      on non-existent `wlan0` or missing target wpasupplicant (laptop_hardening.sh writes 60-instant-wifi.yaml
#      with the real interface name in late-commands).
if [ -f /autoinstall.yaml ] && command -v python3 >/dev/null 2>&1; then
    python3 - "$BEST_DISK" << 'PYAUTOCFG'
import sys, yaml
best_disk = sys.argv[1] if len(sys.argv) > 1 else ""
path = "/autoinstall.yaml"
try:
    with open(path, "r", encoding="utf-8") as f:
        data = yaml.safe_load(f) or {}
    if best_disk and isinstance(data.get("storage"), dict):
        layout = data["storage"].get("layout")
        if isinstance(layout, dict) and isinstance(layout.get("match"), dict):
            if "size" in layout["match"]:
                layout["match"] = {"path": best_disk}
    if isinstance(data.get("network"), dict):
        data["network"].pop("wifis", None)
    with open(path, "w", encoding="utf-8") as f:
        yaml.safe_dump(data, f, default_flow_style=False, sort_keys=False)
except Exception as e:
    print(f"[select_target_disk] python3 yaml update warning: {e}", file=sys.stderr)
PYAUTOCFG
    log_usb "Sanitized /autoinstall.yaml via python3 (removed wifis.wlan0 and pinned storage path=${BEST_DISK})."
fi

# 5. Live Installer Network Check:
#    If a wired Ethernet or USB-tether interface (en*, eth*, usb*) ALREADY has a global IPv4 address
#    (e.g. enp7s0 DHCP), DO NOT install or spawn a conflicting wpa_supplicant/dhclient in the live installer!
WIRED_IPV4=$(ip -4 -o addr show scope global 2>/dev/null | awk '$2 ~ /^(en|eth|usb)/ {print $2 ":" $4}' | head -n1 || true)
if [ -n "$WIRED_IPV4" ]; then
    log_usb "Wired interface already online (${WIRED_IPV4}); skipping live wpa_supplicant/dhclient to avoid D-Bus/route contention (target Wi-Fi will be configured in late-commands)."
elif [ -n "${INSTANT_WIFI_SSID:-}" ]; then
    find /cdrom/pool -type f \( -name "libnl-*.deb" -o -name "libpcsclite*.deb" -o -name "wpasupplicant*.deb" -o -name "wireless-tools*.deb" -o -name "rfkill*.deb" \) -exec dpkg -i {} + >/dev/null 2>&1 || true
    WIFI_DEV=""
    for nd in /sys/class/net/*; do
        [ -e "$nd" ] || continue
        if [ -d "$nd/wireless" ] || [ -d "$nd/phy80211" ]; then
            WIFI_DEV=$(basename "$nd")
            break
        fi
    done
    if [ -n "$WIFI_DEV" ] && command -v wpa_supplicant >/dev/null 2>&1; then
        log_usb "No wired IPv4 detected; connecting live Wi-Fi interface ${WIFI_DEV} to SSID ${INSTANT_WIFI_SSID}..."
        ip link set "$WIFI_DEV" up 2>/dev/null || true
        if [ -n "${INSTANT_WIFI_PASSWORD:-}" ] && command -v wpa_passphrase >/dev/null 2>&1; then
            wpa_passphrase "$INSTANT_WIFI_SSID" "$INSTANT_WIFI_PASSWORD" > /run/live-wpa.conf 2>/dev/null || true
        else
            printf 'network={
\tssid="%s"
\tkey_mgmt=NONE
}
' "$INSTANT_WIFI_SSID" > /run/live-wpa.conf
        fi
        wpa_supplicant -B -i "$WIFI_DEV" -c /run/live-wpa.conf >/dev/null 2>&1 || true
        (dhclient -1 "$WIFI_DEV" >/dev/null 2>&1 || networkctl reconfigure "$WIFI_DEV" >/dev/null 2>&1 || true) &
    else
        log_usb "No live wireless interface found during early-commands (WIFI_DEV=${WIFI_DEV:-none}); continuing with offline install."
    fi
fi

udevadm settle --timeout=15 2>/dev/null || true
exit 0
`)
	if err := writeItem("select_target_disk.sh", NormalizeShellScriptLF(selectDiskHelper), 0755); err != nil {
		return nil, err
	}

	// Non-destructive Dual-Boot & Stale Linux LVM Cleaner:
	// - Only inspects actual disk partitions (`TYPE == "part"`), NEVER LVM logical volumes (`TYPE == "lvm"`).
	// - If TARGET_DISK has a Windows NTFS partition (`ntfs` / `BitLocker`), shrinks the NTFS partition using
	//   `ntfsresize` + `parted` (using true end offset `START_MB + NEW_SIZE_MB`) and sets `use-gap: largest`
	//   in `/autoinstall.yaml` so Windows is preserved untouched.
	// - If TARGET_DISK has NO Windows NTFS partition (e.g., it only contains a previous Ubuntu Server LVM
	//   installation like `ubuntu-vg/ubuntu-lv` on `/dev/sda3`), removes the old LVM volume groups and
	//   partition signatures so `probert.lvm` (`vgchange -ay`) does NOT lock `/dev/sda3` under `/dev/dm-0`
	//   during `Filesystem/_probe`!
	dualBootHelper := []byte(`#!/usr/bin/env bash
set -uo pipefail
REQ_GB="${1:-45}"
LOG_A="/mnt/cidata/INSTANT-SERVER-INSTALL.log"
log_usb() {
    printf '[dynamic_dualboot] %s
' "$*" | tee -a "$LOG_A" 2>/dev/null || true
    if mountpoint -q /mnt/usb_logs 2>/dev/null; then
        cp -f "$LOG_A" /mnt/usb_logs/INSTANT-SERVER-INSTALL.log 2>/dev/null || true
    fi
    sync
}

TARGET_DISK=$(cat /run/instant-target-disk 2>/dev/null || true)
[ -z "$TARGET_DISK" ] && exit 0

# Safety check: never modify the USB installer disk
CDROM_PART=$(findmnt -n -o SOURCE /cdrom 2>/dev/null || true)
USB_DISK=""
if [ -n "$CDROM_PART" ]; then
    PK=$(lsblk -no PKNAME "$CDROM_PART" 2>/dev/null | head -n1)
    [ -n "$PK" ] && USB_DISK="/dev/${PK}"
fi
if [ "$TARGET_DISK" = "$USB_DISK" ] || grep -q "^${TARGET_DISK}" /proc/mounts 2>/dev/null; then
    log_usb "Skipping ${TARGET_DISK} because it has active installer mounts."
    exit 0
fi

# Count actual disk partitions (TYPE=="part") on TARGET_DISK
PART_COUNT=$(lsblk -lnpo NAME,TYPE "$TARGET_DISK" 2>/dev/null | awk '$2=="part"{count++} END{print count+0}')
if [ "${PART_COUNT:-0}" -eq 0 ]; then
    log_usb "Target disk ${TARGET_DISK} has no existing partitions; using full disk layout."
    exit 0
fi

# Inspect ONLY physical partitions (TYPE=="part", never TYPE=="lvm")
HAS_WINDOWS_NTFS=0
SHRINK_PART=""
SHRINK_FSTYPE=""
SHRINK_SIZE_BYTES=0

while read -r pname psize ptype pfstype; do
    [ "$ptype" = "part" ] || continue
    if [ "$pfstype" = "ntfs" ] || [ "$pfstype" = "BitLocker" ]; then
        HAS_WINDOWS_NTFS=1
        if [ "${psize:-0}" -gt "$SHRINK_SIZE_BYTES" ]; then
            SHRINK_SIZE_BYTES="$psize"
            SHRINK_PART="$pname"
            SHRINK_FSTYPE="$pfstype"
        fi
    fi
done < <(lsblk -lnbpo NAME,SIZE,TYPE,FSTYPE "$TARGET_DISK" 2>/dev/null)

# If TARGET_DISK has NO Windows NTFS partition (only old Linux/Ubuntu LVM or ext4 partitions),
# remove the old Linux LVM volume groups and partition signatures so probert.lvm (`vgchange -ay`)
# cannot re-activate `ubuntu-vg` (`/dev/dm-0`) and block Subiquity from partitioning TARGET_DISK!
if [ "$HAS_WINDOWS_NTFS" -eq 0 ]; then
    log_usb "No Windows NTFS partition found on ${TARGET_DISK} (only previous Linux/LVM partitions exist). Removing old Linux LVM/partitions for clean install..."
    swapoff -a 2>/dev/null || true
    for mp in $(lsblk -lnpo NAME "$TARGET_DISK" 2>/dev/null | tail -n +2); do
        umount -lf "$mp" 2>/dev/null || true
    done
    for vg in $(vgs --noheadings -o vg_name 2>/dev/null || true); do
        lvremove -ff -y "$vg" >/dev/null 2>&1 || true
        vgremove -ff -y "$vg" >/dev/null 2>&1 || true
    done
    for pv in $(pvs --noheadings -o pv_name 2>/dev/null | grep "^${TARGET_DISK}" || true); do
        pvremove -ff -y "$pv" >/dev/null 2>&1 || true
    done
    dmsetup remove_all >/dev/null 2>&1 || true
    for p in $(lsblk -lnpo NAME "$TARGET_DISK" 2>/dev/null | tail -n +2 | tac); do
        wipefs -a "$p" >/dev/null 2>&1 || true
    done
    wipefs -a "$TARGET_DISK" >/dev/null 2>&1 || true
    sgdisk --zap-all "$TARGET_DISK" >/dev/null 2>&1 || dd if=/dev/zero of="$TARGET_DISK" bs=1M count=16 conv=fsync >/dev/null 2>&1 || true
    partprobe "$TARGET_DISK" 2>/dev/null || true
    udevadm settle --timeout=15 2>/dev/null || true
    log_usb "Cleared old Linux LVM and partition table on ${TARGET_DISK}."
    exit 0
fi

# True Windows Dual-Boot: shrink Windows NTFS partition if needed and configure Subiquity to use largest free gap
REQ_BYTES=$((REQ_GB * 1024 * 1024 * 1024))
MIN_REMAIN_BYTES=$((20 * 1024 * 1024 * 1024))

if [ -n "$SHRINK_PART" ] && [ "$SHRINK_FSTYPE" = "ntfs" ] && [ "$SHRINK_SIZE_BYTES" -gt $((REQ_BYTES + MIN_REMAIN_BYTES)) ]; then
    NEW_SIZE_BYTES=$((SHRINK_SIZE_BYTES - REQ_BYTES))
    NEW_SIZE_MB=$((NEW_SIZE_BYTES / 1048576))
    PNUM=$(cat "/sys/class/block/$(basename "$SHRINK_PART")/partition" 2>/dev/null || echo "$SHRINK_PART" | grep -o '[0-9]*$' || true)
    START_SECT=$(cat "/sys/class/block/$(basename "$SHRINK_PART")/start" 2>/dev/null || echo 0)
    START_MB=$(( (START_SECT * 512) / 1048576 ))
    NEW_END_MB=$((START_MB + NEW_SIZE_MB))
    log_usb "Windows NTFS partition detected on ${SHRINK_PART} (part #${PNUM}, start=${START_MB}MB). Shrinking to ${NEW_SIZE_MB}MB (end=${NEW_END_MB}MB) to free ${REQ_GB}GB for dual-boot..."
    if command -v ntfsresize >/dev/null 2>&1; then
        ntfsresize -f -s "${NEW_SIZE_MB}M" "$SHRINK_PART" >/dev/null 2>&1 || true
        [ -n "$PNUM" ] && parted ---pretend-input-tty "$TARGET_DISK" resizepart "$PNUM" "${NEW_END_MB}MB" Yes >/dev/null 2>&1 || true
        partprobe "$TARGET_DISK" 2>/dev/null || true
        udevadm settle --timeout=15 2>/dev/null || true
    fi
fi

if [ -f /autoinstall.yaml ] && command -v python3 >/dev/null 2>&1; then
    python3 - << 'PYGAP'
import yaml
path = "/autoinstall.yaml"
try:
    with open(path, "r", encoding="utf-8") as f:
        data = yaml.safe_load(f) or {}
    if isinstance(data.get("storage"), dict) and isinstance(data["storage"].get("layout"), dict):
        data["storage"]["layout"]["use-gap"] = "largest"
    with open(path, "w", encoding="utf-8") as f:
        yaml.safe_dump(data, f, default_flow_style=False, sort_keys=False)
except Exception:
    pass
PYGAP
    log_usb "Configured /autoinstall.yaml storage.layout with use-gap=largest to preserve Windows."
fi
exit 0
`)
	if err := writeItem("dynamic_dualboot_storage.sh", NormalizeShellScriptLF(dualBootHelper), 0755); err != nil {
		return nil, err
	}

	// Reinstall While Retaining Server Data helper script:
	// Uses /dev/shm/instant-preserved-data (50% RAM tmpfs, never /run which is limited to 10% RAM),
	// activates any existing LVM volumes first so `/DATA` on an LVM root can be backed up,
	// and then removes old LVM locks after backup so Subiquity can cleanly partition the target disk.
	reinstallPreserveScript := []byte(`#!/usr/bin/env bash
set -uo pipefail
ACTION="${1:-backup}"
STAGE_BACKUP="/dev/shm/instant-preserved-data"
mkdir -p "$STAGE_BACKUP" /mnt/existing_os
if [ "$ACTION" = "backup" ]; then
    vgchange -ay >/dev/null 2>&1 || true
    AVAIL_KB=$(df -Pk /dev/shm 2>/dev/null | awk 'NR==2 {print $4}' || echo 0)
    for part in $(lsblk -lnpo NAME,FSTYPE | awk '$2 ~ /ext4|btrfs|xfs/ {print $1}'); do
        if mount -o ro "$part" /mnt/existing_os 2>/dev/null; then
            if [ -d "/mnt/existing_os/opt/instant-server" ]; then
                cp -a /mnt/existing_os/opt/instant-server "$STAGE_BACKUP/" 2>/dev/null || true
            fi
            if [ -d "/mnt/existing_os/DATA" ]; then
                DATA_KB=$(du -sk /mnt/existing_os/DATA 2>/dev/null | awk '{print $1}' || echo 0)
                if [ "${AVAIL_KB:-0}" -gt $((DATA_KB + 131072)) ]; then
                    echo "[instant-server] Backing up full /DATA (${DATA_KB} KB) from $part..."
                    cp -a /mnt/existing_os/DATA "$STAGE_BACKUP/" 2>/dev/null || true
                else
                    echo "[instant-server] Backing up critical /DATA/AppData, /DATA/Backups, and /DATA/CustomWorkflows from $part..."
                    mkdir -p "$STAGE_BACKUP/DATA"
                    for sub in AppData Backups CustomWorkflows Shared; do
                        [ -d "/mnt/existing_os/DATA/$sub" ] && cp -a "/mnt/existing_os/DATA/$sub" "$STAGE_BACKUP/DATA/" 2>/dev/null || true
                    done
                fi
            fi
            umount /mnt/existing_os 2>/dev/null || true
        fi
    done
    TARGET_DISK=$(cat /run/instant-target-disk 2>/dev/null || true)
    if [ -n "$TARGET_DISK" ] && ! grep -q "^${TARGET_DISK}" /proc/mounts 2>/dev/null; then
        for vg in $(vgs --noheadings -o vg_name 2>/dev/null || true); do
            lvremove -ff -y "$vg" >/dev/null 2>&1 || true
            vgremove -ff -y "$vg" >/dev/null 2>&1 || true
        done
        for pv in $(pvs --noheadings -o pv_name 2>/dev/null | grep "^${TARGET_DISK}" || true); do
            pvremove -ff -y "$pv" >/dev/null 2>&1 || true
        done
        dmsetup remove_all >/dev/null 2>&1 || true
        wipefs -a "$TARGET_DISK" >/dev/null 2>&1 || true
        sgdisk --zap-all "$TARGET_DISK" >/dev/null 2>&1 || true
        partprobe "$TARGET_DISK" 2>/dev/null || true
    fi
elif [ "$ACTION" = "restore" ]; then
    if [ -d "$STAGE_BACKUP/DATA" ]; then
        mkdir -p /target/DATA
        cp -a "$STAGE_BACKUP/DATA/." /target/DATA/ 2>/dev/null || true
        rm -rf "$STAGE_BACKUP" 2>/dev/null || true
        echo "[instant-server] Restored preserved /DATA to reinstalled server."
    fi
fi
exit 0
`)
	if err := writeItem("reinstall_preserve_data.sh", NormalizeShellScriptLF(reinstallPreserveScript), 0755); err != nil {
		return nil, err
	}

	// Offline Updater USB helper script (for local-only / air-gapped servers or incremental app additions)
	offlineUpdaterScript := []byte(`#!/usr/bin/env bash
set -euo pipefail
SRC_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TARGET_ROOT="/"
if [ -d "/target/opt/instant-server" ]; then
    TARGET_ROOT="/target"
elif [ ! -d "/opt/instant-server" ]; then
    mkdir -p /mnt/target_os
    for part in $(lsblk -lnpo NAME,FSTYPE | awk '$2 ~ /ext4|btrfs|xfs/ {print $1}'); do
        if mount "$part" /mnt/target_os 2>/dev/null; then
            if [ -d "/mnt/target_os/opt/instant-server" ]; then
                TARGET_ROOT="/mnt/target_os"
                break
            fi
            umount /mnt/target_os 2>/dev/null || true
        fi
    done
fi
mkdir -p "${TARGET_ROOT}/opt/instant-server" "${TARGET_ROOT}/DATA/AppData"
cp -rf "${SRC_DIR}/server_dashboard.html" "${TARGET_ROOT}/opt/instant-server/" 2>/dev/null || true
cp -rf "${SRC_DIR}/laptop_hardening.sh" "${TARGET_ROOT}/opt/instant-server/" 2>/dev/null || true
cp -rf "${SRC_DIR}/instant_apps_bundle.sh" "${TARGET_ROOT}/opt/instant-server/" 2>/dev/null || true
if [ -d "${SRC_DIR}/casaos_apps" ]; then
    cp -rf "${SRC_DIR}/casaos_apps/"* "${TARGET_ROOT}/DATA/AppData/" 2>/dev/null || true
fi
if [ "$TARGET_ROOT" = "/" ]; then
    [ -f /opt/instant-server/instant_apps_bundle.sh ] && bash /opt/instant-server/instant_apps_bundle.sh 2>/dev/null || true
    systemctl restart instant-server-dashboard.service 2>/dev/null || true
    systemctl start instant-server-launch-apps.service 2>/dev/null || true
elif [ "$TARGET_ROOT" = "/mnt/target_os" ]; then
    chroot /mnt/target_os /bin/bash /opt/instant-server/instant_apps_bundle.sh 2>/dev/null || true
fi
echo "[instant-server] Offline USB update and incremental apps applied to ${TARGET_ROOT}."
if [ "$TARGET_ROOT" = "/mnt/target_os" ]; then
    umount /mnt/target_os 2>/dev/null || true
fi
if [ -d "/cdrom" ] || grep -q "casper" /proc/cmdline 2>/dev/null; then
    touch /run/casper-no-prompt 2>/dev/null || true
    sync
    reboot -f
fi
`)
	if err := writeItem("offline_updater_apply.sh", NormalizeShellScriptLF(offlineUpdaterScript), 0755); err != nil {
		return nil, err
	}

	// Emergency Repair & Password Reset USB helper script
	hashedPass := Sha512Crypt(cfg.Password, "instantserverlts")
	sshPort := cfg.SSHPort
	if sshPort <= 0 || sshPort > 65535 {
		sshPort = 22
	}
	dashPort := cfg.DashboardPort
	if dashPort <= 0 || dashPort > 65535 {
		dashPort = 8088
	}
	repairResetScript := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
TARGET_USER=%q
NEW_HASH=%q
SSH_PORT="%d"
DASH_PORT="%d"
mkdir -p /mnt/repair_os
for part in $(lsblk -lnpo NAME,FSTYPE | awk '$2 == "ext4" {print $1}'); do
    e2fsck -fy "$part" 2>/dev/null || true
    if mount "$part" /mnt/repair_os 2>/dev/null; then
        if [ -f "/mnt/repair_os/etc/shadow" ]; then
            chroot /mnt/repair_os /bin/bash -c "rm -f /var/lib/dpkg/lock-frontend /var/lib/dpkg/lock && dpkg --configure -a" 2>/dev/null || true
            chroot /mnt/repair_os /bin/bash -c "printf '%%s:%%s\n' '$TARGET_USER' '$NEW_HASH' | chpasswd -e" 2>/dev/null || true
            chroot /mnt/repair_os /bin/bash -c "ufw allow 22/tcp && ufw allow ${SSH_PORT}/tcp && ufw allow ${DASH_PORT}/tcp && fail2ban-client unban --all" 2>/dev/null || true
            echo "[instant-server] Reset password for $TARGET_USER and repaired filesystem on $part."
        fi
        umount /mnt/repair_os 2>/dev/null || true
    fi
done
if [ -d "/cdrom" ] || grep -q "casper" /proc/cmdline 2>/dev/null; then
    touch /run/casper-no-prompt 2>/dev/null || true
    sync
    reboot -f
fi
`, cfg.Username, hashedPass, sshPort, dashPort)
	if err := writeItem("emergency_repair_reset.sh", NormalizeShellScriptLF([]byte(repairResetScript)), 0755); err != nil {
		return nil, err
	}

	envConf := []byte(GenerateTargetEnvConf(cfg))
	if err := writeItem("instant-server-env.conf", envConf, 0600); err != nil {
		return nil, err
	}

	cfgBytes, _ := json.MarshalIndent(cfg, "", "  ")
	if err := writeItem("instant-server-config.json", cfgBytes, 0644); err != nil {
		return nil, err
	}

	if len(hardeningScript) > 0 {
		if err := writeItem("laptop_hardening.sh", NormalizeShellScriptLF(hardeningScript), 0755); err != nil {
			return nil, err
		}
	}
	if len(dashboardHTML) > 0 {
		if err := writeItem("server_dashboard.html", dashboardHTML, 0644); err != nil {
			return nil, err
		}
	}
	if strings.TrimSpace(cfg.CustomPostInstallScript) != "" {
		if err := writeItem("custom_post_install.sh", NormalizeShellScriptLF([]byte(cfg.CustomPostInstallScript)), 0755); err != nil {
			return nil, err
		}
	}
	for customPath, customContent := range cfg.CustomConfigFiles {
		safeName := strings.ReplaceAll(strings.Trim(customPath, "/\\"), "/", "__")
		if safeName != "" {
			_ = writeItem(filepath.Join("custom_system_configs", safeName), []byte(customContent), 0644)
		}
	}
	for unitName, unitContent := range cfg.CustomSystemdUnits {
		safeUnit := filepath.Base(strings.TrimSpace(unitName))
		if safeUnit != "" && safeUnit != "." && safeUnit != ".." {
			_ = writeItem(filepath.Join("custom_systemd", safeUnit), NormalizeShellScriptLF([]byte(unitContent)), 0644)
		}
	}
	for ruleName, ruleContent := range cfg.CustomUdevRules {
		safeRule := filepath.Base(strings.TrimSpace(ruleName))
		if safeRule != "" && safeRule != "." && safeRule != ".." {
			_ = writeItem(filepath.Join("custom_udev", safeRule), NormalizeShellScriptLF([]byte(ruleContent)), 0644)
		}
	}

	for _, appID := range cfg.SelectedApps {
		cleanID := strings.TrimSpace(appID)
		if cleanID == "" || strings.HasPrefix(cleanID, "custom_") {
			continue
		}
		var override *AppOverride
		if cfg.AppOverrides != nil {
			if ov, ok := cfg.AppOverrides[cleanID]; ok {
				override = &ov
			}
		}
		composeYAML := BuildVerifiedAppComposeWithOverride(cleanID, "", 0, cfg.MinecraftConfig, override)
		rel := filepath.Join("casaos_apps", cleanID, "docker-compose.yml")
		if err := writeItem(rel, []byte(composeYAML), 0644); err != nil {
			return nil, err
		}
		if override != nil {
			for cfgFileName, cfgFileContent := range override.ConfigFiles {
				safeName := filepath.Base(cfgFileName)
				if safeName != "" && safeName != "." && safeName != ".." {
					_ = writeItem(filepath.Join("casaos_apps", cleanID, "config_files", safeName), []byte(cfgFileContent), 0644)
				}
			}
		}
	}
	for _, ca := range cfg.CustomApps {
		if ca.ID != "" && ca.ComposeYAML != "" {
			rel := filepath.Join("casaos_apps", ca.ID, "docker-compose.yml")
			if err := writeItem(rel, []byte(ca.ComposeYAML), 0644); err != nil {
				return nil, err
			}
		}
	}

	// Build a self-extracting root-level script (instant_apps_bundle.sh) so the flat FAT16 CIDATA
	// root directory carries every selected app's docker-compose.yml, SHA-256 fingerprint, and custom workflows.
	var bundleBuilder strings.Builder
	bundleBuilder.WriteString("#!/usr/bin/env bash\nset -euo pipefail\nmkdir -p /DATA/AppData /opt/instant-server/casaos_apps /opt/instant-server/custom_system_configs /opt/instant-server/custom_systemd /opt/instant-server/custom_udev\n")
	for _, appID := range cfg.SelectedApps {
		cleanID := strings.TrimSpace(appID)
		if cleanID == "" || strings.HasPrefix(cleanID, "custom_") {
			continue
		}
		var override *AppOverride
		if cfg.AppOverrides != nil {
			if ov, ok := cfg.AppOverrides[cleanID]; ok {
				override = &ov
			}
		}
		composeYAML := BuildVerifiedAppComposeWithOverride(cleanID, "", 0, cfg.MinecraftConfig, override)
		sum := sha256.Sum256([]byte(strings.TrimSpace(composeYAML)))
		hashHex := hex.EncodeToString(sum[:])
		bundleBuilder.WriteString(fmt.Sprintf("mkdir -p /DATA/AppData/%s /opt/instant-server/casaos_apps/%s\n", cleanID, cleanID))
		bundleBuilder.WriteString(fmt.Sprintf("cat << 'APPEOF_%s' > /DATA/AppData/%s/docker-compose.yml\n%s\nAPPEOF_%s\n", cleanID, cleanID, strings.TrimRight(composeYAML, "\r\n"), cleanID))
		bundleBuilder.WriteString(fmt.Sprintf("printf '%%s\\n' %q > /DATA/AppData/%s/.compose_sha256\n", hashHex, cleanID))
		bundleBuilder.WriteString(fmt.Sprintf("cp -f /DATA/AppData/%s/docker-compose.yml /opt/instant-server/casaos_apps/%s/docker-compose.yml 2>/dev/null || true\n", cleanID, cleanID))
	}
	for _, ca := range cfg.CustomApps {
		if ca.ID != "" && ca.ComposeYAML != "" {
			sum := sha256.Sum256([]byte(strings.TrimSpace(ca.ComposeYAML)))
			hashHex := hex.EncodeToString(sum[:])
			bundleBuilder.WriteString(fmt.Sprintf("mkdir -p /DATA/AppData/%s\n", ca.ID))
			bundleBuilder.WriteString(fmt.Sprintf("cat << 'CUSTEOF_%s' > /DATA/AppData/%s/docker-compose.yml\n%s\nCUSTEOF_%s\n", ca.ID, ca.ID, strings.TrimRight(ca.ComposeYAML, "\r\n"), ca.ID))
			bundleBuilder.WriteString(fmt.Sprintf("printf '%%s\\n' %q > /DATA/AppData/%s/.compose_sha256\n", hashHex, ca.ID))
		}
	}
	// Also bundle custom system configs, custom Samba shares, custom systemd units, and custom udev rules into instant_apps_bundle.sh
	// so even when mounted from flat root FAT16 CIDATA, every custom user workflow is unpacked automatically.
	for customPath, customContent := range cfg.CustomConfigFiles {
		safeName := strings.ReplaceAll(strings.Trim(customPath, "/\\"), "/", "__")
		if safeName != "" {
			bundleBuilder.WriteString(fmt.Sprintf("cat << 'SYSCFGEOF_%s' > /opt/instant-server/custom_system_configs/%s\n%s\nSYSCFGEOF_%s\n", safeName, safeName, strings.TrimRight(customContent, "\r\n"), safeName))
		}
	}
	for unitName, unitContent := range cfg.CustomSystemdUnits {
		safeUnit := filepath.Base(strings.TrimSpace(unitName))
		if safeUnit != "" && safeUnit != "." && safeUnit != ".." {
			tag := reSlugClean.ReplaceAllString(safeUnit, "_")
			bundleBuilder.WriteString(fmt.Sprintf("cat << 'UNITEOF_%s' > /opt/instant-server/custom_systemd/%s\n%s\nUNITEOF_%s\n", tag, safeUnit, strings.TrimRight(unitContent, "\r\n"), tag))
		}
	}
	for ruleName, ruleContent := range cfg.CustomUdevRules {
		safeRule := filepath.Base(strings.TrimSpace(ruleName))
		if safeRule != "" && safeRule != "." && safeRule != ".." {
			tag := reSlugClean.ReplaceAllString(safeRule, "_")
			bundleBuilder.WriteString(fmt.Sprintf("cat << 'UDEVEOF_%s' > /opt/instant-server/custom_udev/%s\n%s\nUDEVEOF_%s\n", tag, safeRule, strings.TrimRight(ruleContent, "\r\n"), tag))
		}
	}
	if len(cfg.CustomSambaShares) > 0 {
		var smbBuilder strings.Builder
		for _, sh := range cfg.CustomSambaShares {
			name := strings.TrimSpace(sh.Name)
			path := strings.TrimSpace(sh.Path)
			if name == "" || path == "" {
				continue
			}
			ro := "no"
			if sh.ReadOnly {
				ro = "yes"
			}
			guest := "no"
			if sh.GuestOK {
				guest = "yes"
			}
			fruitLines := ""
			if sh.AppleFruit {
				fruitLines = "   vfs objects = catia fruit streams_xattr\n   fruit:aapl = yes\n   fruit:model = MacSamba\n   fruit:metadata = stream\n"
			}
			smbBuilder.WriteString(fmt.Sprintf("\n[%s]\n   comment = %s\n   path = %s\n   browseable = yes\n   read only = %s\n   guest ok = %s\n   create mask = 0664\n   directory mask = 0775\n%s", name, sh.Comment, path, ro, guest, fruitLines))
		}
		if smbBuilder.Len() > 0 {
			_ = writeItem("custom_samba_shares.conf", NormalizeShellScriptLF([]byte(smbBuilder.String())), 0644)
			bundleBuilder.WriteString(fmt.Sprintf("cat << 'SMBSHAREEOF' > /opt/instant-server/custom_samba_shares.conf\n%s\nSMBSHAREEOF\n", strings.TrimRight(smbBuilder.String(), "\r\n")))
		}
	}
	if err := writeItem("instant_apps_bundle.sh", NormalizeShellScriptLF([]byte(bundleBuilder.String())), 0755); err != nil {
		return nil, err
	}

	return writtenFiles, nil
}