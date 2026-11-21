// ============================================================================
// FILE: usb/payload/payload_types.go
// PURPOSE: Profile configuration structs, pure-Go SHA-512 crypt ($6$) password
//
//	hashing, profile validation, and custom Docker/GitHub URL parser.
//
// ============================================================================
package payload

import (
	"crypto/sha512"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/mutedll/InstantServer/usb/catalog"
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
	ServerName                  string                 `json:"server_name,omitempty"`
	ServerAddressHost           string                 `json:"server_address_host,omitempty"`
	ServerAddressDomain         string                 `json:"server_address_domain,omitempty"`
	LogRetentionDays            int                    `json:"log_retention_days,omitempty"`
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
	SkipBeginnerChecks          bool                   `json:"skip_beginner_checks,omitempty"`
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

type MinecraftServerConfig = catalog.MinecraftServerConfig
type AppOverride = catalog.AppOverride

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
	ReSlugClean    = regexp.MustCompile(`[^a-z0-9_-]+`)
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
	slug := ReSlugClean.ReplaceAllString(strings.ToLower(baseName), "-")
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
