// usb/payload_builder.go
package usb

import (
	"crypto/sha512"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

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
	SelectedApps                []string               `json:"selected_apps"`
	CustomApps                  []CustomAppEntry       `json:"custom_apps"`
	MinecraftConfig             *MinecraftServerConfig `json:"minecraft_config,omitempty"`
	AppOverrides                map[string]AppOverride `json:"app_overrides,omitempty"`
	InstallMode                 string                 `json:"install_mode"`
	DualBootAllocationGB        int                    `json:"dual_boot_Allocation_gb"`
	ConfirmEraseMainDrive       bool                   `json:"confirm_erase_main_drive"`
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

	ctxHash := sha512.New()
	ctxHash.Write(key)
	ctxHash.Write(saltBytes)
	for cnt := keyLen; cnt > 64; cnt -= 64 {
		ctxHash.Write(altSum)
	}
	if keyLen > 0 {
		ctxHash.Write(altSum[:keyLen%64])
	}
	for cnt := keyLen; cnt > 0; cnt >>= 1 {
		if (cnt & 1) != 0 {
			ctxHash.Write(altSum)
		} else {
			ctxHash.Write(key)
		}
	}
	digestA := ctxHash.Sum(nil)

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
	b.Grow(120)
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

	// Build Netplan v2 dual-homing network configuration (Ethernet metric 100 + Wi-Fi failover metric 600 + USB tethering)
	ethSection := `      eth_lan:
        match:
          name: "en*"
        dhcp4: true
        dhcp4-overrides:
          route-metric: 100
        optional: true`
	if strings.TrimSpace(cfg.StaticIPCIDR) != "" && strings.TrimSpace(cfg.StaticGateway) != "" {
		dnsList := `"1.1.1.1", "9.9.9.9"`
		if len(cfg.StaticDNS) > 0 {
			dnsList = `"` + strings.Join(cfg.StaticDNS, `", "`) + `"`
		}
		ethSection = fmt.Sprintf(`      eth_lan:
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
		authLine := ""
		if safePSK != "" {
			authLine = fmt.Sprintf("\n            password: \"%s\"", safePSK)
		}
		wifiSection = fmt.Sprintf(`
    wifis:
      wlan_failover:
        match:
          name: "wl*"
        dhcp4: true
        dhcp4-overrides:
          route-metric: 600
        optional: true
        access-points:
          "%s":%s`, safeSSID, authLine)
	}

	var earlyCommandsBlock string
	var storageBlock string
	switch cfg.InstallMode {
	case "dual_boot":
		allocGB := cfg.DualBootAllocationGB
		if allocGB < 15 {
			allocGB = 45
		}
		earlyCommandsBlock = fmt.Sprintf(`  early-commands:
    - bash /cdrom/cidata/dynamic_dualboot_storage.sh %d 2>/dev/null || bash /cdrom/dynamic_dualboot_storage.sh %d 2>/dev/null || true
`, allocGB, allocGB)
		storageBlock = fmt.Sprintf(`  storage:
    config:
      - type: disk
        id: disk0
        ptable: gpt
        preserve: true
        grub_device: false
      - type: partition
        id: efi-part
        device: disk0
        size: 512M
        flag: boot
        preserve: true
        grub_device: true
      - type: format
        id: efi-fmt
        volume: efi-part
        fstype: fat32
        preserve: true
      - type: mount
        id: efi-mnt
        device: efi-fmt
        path: /boot/efi
      - type: partition
        id: instant-root
        device: disk0
        size: %dG
      - type: format
        id: instant-root-fmt
        volume: instant-root
        fstype: ext4
      - type: mount
        id: instant-root-mnt
        device: instant-root-fmt
        path: /`, allocGB)
	case "interactive_select":
		storageBlock = `  interactive-sections:
    - storage`
	case "repair_rescue_usb":
		storageBlock = `  # Live Repair & Rescue Mode: boots into diagnostic tools without formatting target drive`
	case "secondary_drive":
		storageBlock = fmt.Sprintf(`  storage:
    layout:
      name: %s
      match:
        path: /dev/sdb`, layout)
	default:
		storageBlock = fmt.Sprintf(`  storage:
    layout:
      name: %s
      match:
        size: largest`, layout)
	}

	return fmt.Sprintf(`#cloud-config
autoinstall:
  version: 1
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
    - mkdir -p /target/opt/instant-server
    - cp -rf /cdrom/cidata/* /target/opt/instant-server/ 2>/dev/null || cp -rf /cdrom/* /target/opt/instant-server/ 2>/dev/null || true
    - curtin in-target -- chmod +x /opt/instant-server/laptop_hardening.sh
    - curtin in-target -- bash /opt/instant-server/laptop_hardening.sh
`, host, user, hashedPass, sshKeysBlock, ethSection, usbTetherSection, wifiSection, earlyCommandsBlock, storageBlock)
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
INSTANT_ENABLE_TAILSCALE=%q
INSTANT_TAILSCALE_AUTHKEY=%q
INSTANT_TAILSCALE_MODE=%q
INSTANT_CLUSTER_MODE=%q
INSTANT_CLUSTER_POLICY=%q
INSTANT_CPU_GOVERNOR=%q
INSTANT_DISABLE_TURBO=%q
INSTANT_SELECTED_APPS=%q
`,
		host,
		user,
		bool01(cfg.IgnoreLidSwitch),
		strconv.Itoa(blankSec),
		strconv.Itoa(batStart),
		strconv.Itoa(batStop),
		strconv.Itoa(upsPct),
		bool01(cfg.DisableWifiPowerSave),
		bool01(enableCockpit),
		bool01(enableCasaOS),
		strconv.Itoa(dashPort),
		bool01(cfg.EnableTailscale),
		strings.TrimSpace(cfg.TailscaleAuthKey),
		tsMode,
		clusterMode,
		clusterPol,
		gov,
		bool01(cfg.DisableCPUTurboBoost),
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
	metaData := []byte(fmt.Sprintf("instance-id: %s-01\nlocal-hostname: %s\n", cfg.Hostname, cfg.Hostname))
	if err := writeItem("meta-data", metaData, 0644); err != nil {
		return nil, err
	}
	dualBootHelper := []byte(`#!/usr/bin/env bash
set -euo pipefail
REQ_GB="${1:-45}"
echo "[instant-server] Inspecting target disk for non-destructive ${REQ_GB}G dual-boot allocation..."
`)
	if err := writeItem("dynamic_dualboot_storage.sh", NormalizeShellScriptLF(dualBootHelper), 0755); err != nil {
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
	return writtenFiles, nil
}