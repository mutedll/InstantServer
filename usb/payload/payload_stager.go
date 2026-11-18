// ============================================================================
// FILE: usb/payload/payload_stager.go
// PURPOSE: Stages Cloud-Init user-data, meta-data, helper scripts, custom
//
//	configs, systemd units, udev rules, and Docker Compose app stacks
//	into the CIDATA staging directory.
//
// ============================================================================
package payload

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mutedll/InstantServer/usb/catalog"
)

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

	// 2. Stage early-boot USB logging & error-dumping helper scripts
	earlyScripts := []struct {
		name    string
		content string
	}{
		{"start_usb_logger.sh", scriptStartUSBLoggerSh},
		{"usb_log_daemon.sh", scriptUSBLogDaemonSh},
		{"dump_error_logs_to_usb.sh", scriptDumpErrorLogsSh},
		{"select_target_disk.sh", scriptSelectTargetDiskSh},
		{"dynamic_dualboot_storage.sh", ScriptDynamicDualBootStorageSh},
		{"reinstall_preserve_data.sh", scriptReinstallPreserveDataSh},
		{"offline_updater_apply.sh", scriptOfflineUpdaterApplySh},
	}
	for _, es := range earlyScripts {
		if err := writeItem(es.name, NormalizeShellScriptLF([]byte(es.content)), 0755); err != nil {
			return nil, err
		}
	}

	// 3. Stage standalone Python & secondary Bash helper scripts
	sanitizePyScript := LoadBundledScript("autoinstall_config.py", DefaultAutoinstallConfigPy)
	if err := writeItem("autoinstall_config.py", NormalizeShellScriptLF(sanitizePyScript), 0755); err != nil {
		return nil, err
	}
	dualbootPyScript := LoadBundledScript("dualboot_storage.py", DefaultDualBootStoragePy)
	if err := writeItem("dualboot_storage.py", NormalizeShellScriptLF(dualbootPyScript), 0755); err != nil {
		return nil, err
	}
	dashboardDaemonPy := LoadBundledScript("dashboard_server.py", DefaultDashboardServerPy)
	if err := writeItem("dashboard_server.py", NormalizeShellScriptLF(dashboardDaemonPy), 0755); err != nil {
		return nil, err
	}
	laptopAppsSetupSh := LoadBundledScript("apps_setup.sh", "#!/usr/bin/env bash\nexit 0\n")
	if err := writeItem("apps_setup.sh", NormalizeShellScriptLF(laptopAppsSetupSh), 0755); err != nil {
		return nil, err
	}

	// 4. Stage emergency repair & password-reset helper script
	repairResetScript := buildEmergencyRepairScript(cfg)
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
	dashboardSubAssets := []string{
		"dash_base.css",
		"dash_components.css",
		"dash_catalog.js",
		"dash_state.js",
		"dash_views_apps.js",
		"dash_views_system.js",
		"dash_views_tools.js",
		"dash_events.js",
		"dash_main.js",
	}
	for _, assetName := range dashboardSubAssets {
		for _, cand := range []string{
			filepath.Join("src", "instant_server", "ui", "dashboard", assetName),
			filepath.Join("..", "src", "instant_server", "ui", "dashboard", assetName),
			filepath.Join("..", "..", "src", "instant_server", "ui", "dashboard", assetName),
			filepath.Join("src", "instant_server", "ui", assetName),
			filepath.Join("..", "src", "instant_server", "ui", assetName),
			assetName,
			filepath.Join("..", assetName),
		} {
			if rawAsset, err := os.ReadFile(cand); err == nil && len(rawAsset) > 0 {
				_ = writeItem(assetName, rawAsset, 0644)
				break
			}
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
		composeYAML := catalog.BuildVerifiedAppComposeWithOverride(cleanID, "", 0, cfg.MinecraftConfig, override)
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
		composeYAML := catalog.BuildVerifiedAppComposeWithOverride(cleanID, "", 0, cfg.MinecraftConfig, override)
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
	for customPath, customContent := range cfg.CustomConfigFiles {
		safeName := strings.ReplaceAll(strings.Trim(customPath, "/\\"), "/", "__")
		if safeName != "" {
			bundleBuilder.WriteString(fmt.Sprintf("cat << 'SYSCFGEOF_%s' > /opt/instant-server/custom_system_configs/%s\n%s\nSYSCFGEOF_%s\n", safeName, safeName, strings.TrimRight(customContent, "\r\n"), safeName))
		}
	}
	for unitName, unitContent := range cfg.CustomSystemdUnits {
		safeUnit := filepath.Base(strings.TrimSpace(unitName))
		if safeUnit != "" && safeUnit != "." && safeUnit != ".." {
			tag := ReSlugClean.ReplaceAllString(safeUnit, "_")
			bundleBuilder.WriteString(fmt.Sprintf("cat << 'UNITEOF_%s' > /opt/instant-server/custom_systemd/%s\n%s\nUNITEOF_%s\n", tag, safeUnit, strings.TrimRight(unitContent, "\r\n"), tag))
		}
	}
	for ruleName, ruleContent := range cfg.CustomUdevRules {
		safeRule := filepath.Base(strings.TrimSpace(ruleName))
		if safeRule != "" && safeRule != "." && safeRule != ".." {
			tag := ReSlugClean.ReplaceAllString(safeRule, "_")
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
