// ============================================================================
// FILE: usb/autoinstall_test.go
// PURPOSE: Unit tests for multi-service compose generation, GRUB autoinstall
//
//	patching, non-destructive Dual-Boot storage guarantees, clean FAT32
//	USB reset, and ISO9660-to-FAT32 extraction with /cdrom/autoinstall.yaml.
//
// ============================================================================
package usb

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mutedll/InstantServer/usb/payload"
)

func TestMultiServiceImmichPaperlessAndV6Migrations(t *testing.T) {
	immichYAML := BuildVerifiedAppComposeWithOverride("immich", "", 0, nil, nil)
	for _, tok := range []string{"immich_server:", "immich_machine_learning:", "immich_redis:", "immich_postgres:", "DB_HOSTNAME=immich_postgres"} {
		if !strings.Contains(immichYAML, tok) {
			t.Fatalf("expected multi-container Immich YAML to contain %q, got:\n%s", tok, immichYAML)
		}
	}

	paperlessYAML := BuildVerifiedAppComposeWithOverride("paperless", "", 0, nil, nil)
	for _, tok := range []string{"paperless:", "paperless_redis:", "PAPERLESS_REDIS=redis://paperless_redis:6379"} {
		if !strings.Contains(paperlessYAML, tok) {
			t.Fatalf("expected multi-container Paperless-ngx YAML to contain %q, got:\n%s", tok, paperlessYAML)
		}
	}

	piholeYAML := BuildVerifiedAppComposeWithOverride("pihole", "", 0, nil, nil)
	if !strings.Contains(piholeYAML, "FTLCONF_dns_listeningMode=all") || !strings.Contains(piholeYAML, "FTLCONF_misc_etc_dnsmasq_d=true") {
		t.Fatalf("expected Pi-hole v6 FTLCONF_ variables in YAML, got:\n%s", piholeYAML)
	}

	wgYAML := BuildVerifiedAppComposeWithOverride("wg_easy", "", 0, nil, nil)
	if !strings.Contains(wgYAML, "ghcr.io/wg-easy/wg-easy:15") {
		t.Fatalf("expected wg-easy v15 image tag, got:\n%s", wgYAML)
	}

	stirlingYAML := BuildVerifiedAppComposeWithOverride("stirlingpdf", "", 0, nil, nil)
	if !strings.Contains(stirlingYAML, "stirlingtools/stirling-pdf:latest") {
		t.Fatalf("expected stirlingtools/stirling-pdf:latest image, got:\n%s", stirlingYAML)
	}

	qbitYAML := BuildVerifiedAppComposeWithOverride("qbittorrent", "", 0, nil, nil)
	if !strings.Contains(qbitYAML, "WEBUI_PORT=8090") || !strings.Contains(qbitYAML, `"6881:6881/udp"`) {
		t.Fatalf("expected qBittorrent WEBUI_PORT=8090 and 6881/udp, got:\n%s", qbitYAML)
	}
}

func TestUbuntuLiveServerGrubPatchAndHybridMBRCidata(t *testing.T) {
	// Test official Ubuntu 24.04 / 26.04 Live Server grub.cfg pattern (without 'splash')
	orig := "set timeout=30\nmenuentry \"Try or Install Ubuntu Server\" {\n\tset gfxpayload=keep\n\tlinux\t/casper/vmlinuz  ---\n\tinitrd\t/casper/initrd\n}\n"
	buf := []byte(orig)
	patches := PatchGrubKernelCmdlineInPlace(buf)
	if patches < 2 {
		t.Fatalf("expected at least 2 patches (timeout + Live Server linux cmdline), got %d", patches)
	}
	if len(buf) != len(orig) {
		t.Fatalf("expected exact in-place byte length %d, got %d", len(orig), len(buf))
	}
	if !bytes.Contains(buf, []byte("set timeout=1 ")) {
		t.Fatalf("expected set timeout=1  in patched grub.cfg, got:\n%s", string(buf))
	}
	if !bytes.Contains(buf, []byte("/casper/vmlinuz autoinstall")) || !bytes.Contains(buf, []byte("---")) {
		t.Fatalf("expected patched Live Server autoinstall kernel line, got:\n%s", string(buf))
	}
}

func TestReinstallModesOfflineUpdaterAndCustomWorkflows(t *testing.T) {
	// 1. Reinstall Without Retaining Data MUST block unless ConfirmWipeServerData == "WIPE-SERVER-DATA"
	wipeBlocked := ServerProfileConfig{
		Hostname:              "instant-server",
		Username:              "admin",
		InstallMode:           "reinstall_wipe_data",
		ConfirmWipeServerData: "",
	}
	if err := ValidateProfileConfig(wipeBlocked); err == nil {
		t.Fatalf("expected reinstall_wipe_data without confirmation token to be blocked")
	}

	wipeConfirmed := wipeBlocked
	wipeConfirmed.ConfirmWipeServerData = "WIPE-SERVER-DATA"
	if err := ValidateProfileConfig(wipeConfirmed); err != nil {
		t.Fatalf("expected confirmed reinstall_wipe_data to pass validation, got: %v", err)
	}

	// 2. Reinstall While Retaining Server Data + Universal Custom Server Workflows (Custom Configs, Samba Shares, Systemd, Udev, Apt, Post-Install Script)
	keepCfg := ServerProfileConfig{
		Hostname:          "instant-server",
		Username:          "admin",
		Password:          "StrongPass123!",
		InstallMode:       "reinstall_keep_data",
		StorageLayout:     "lvm",
		CustomAptPackages: []string{"ffmpeg", "rsync"},
		CustomSambaShares: []CustomSambaShare{
			{Name: "CustomVault", Path: "/DATA/CustomVault", Comment: "User Custom NAS Share", AppleFruit: true},
		},
		CustomSystemdUnits: map[string]string{
			"custom-sync.service": "[Unit]\nDescription=Custom Sync Job\n[Service]\nExecStart=/usr/bin/true\n",
		},
		CustomUdevRules: map[string]string{
			"99-custom-usb.rules": "ACTION==\"add\", SUBSYSTEM==\"usb\", RUN+=\"/usr/local/bin/custom-usb-trigger.sh\"\n",
		},
		CustomConfigFiles: map[string]string{
			"/etc/custom-workflow.conf": "SYNC_INTERVAL=300\n",
		},
		CustomPostInstallScript: "mkdir -p /DATA/CustomVault\n",
	}
	if err := ValidateProfileConfig(keepCfg); err != nil {
		t.Fatalf("expected reinstall_keep_data config to be valid, got: %v", err)
	}

	userData := GenerateAutoinstallUserData(keepCfg)
	for _, must := range []string{
		"systemctl mask sleep.target suspend.target hibernate.target hybrid-sleep.target",
		"reinstall_preserve_data.sh backup",
		"reinstall_preserve_data.sh restore",
		"sizing-policy: all",
	} {
		if !strings.Contains(userData, must) {
			t.Fatalf("expected reinstall_keep_data user-data to contain %q, got:\n%s", must, userData)
		}
	}

	stageDir := t.TempDir()
	written, err := StageCIDATAPayload(stageDir, keepCfg, []byte("#!/usr/bin/env bash\n"), []byte("<html></html>"))
	if err != nil {
		t.Fatalf("StageCIDATAPayload failed for keepCfg: %v", err)
	}
	for _, expectedFile := range []string{
		"autoinstall_config.py",
		"dashboard_server.py",
		"reinstall_preserve_data.sh",
		"offline_updater_apply.sh",
		"emergency_repair_reset.sh",
		"custom_post_install.sh",
		"custom_samba_shares.conf",
	} {
		found := false
		for _, w := range written {
			if w == expectedFile {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected %q in staged CIDATA files, got %v", expectedFile, written)
		}
	}
}

func TestResetUSBToCleanFAT32(t *testing.T) {
	tmpDir := t.TempDir()
	usbPath := filepath.Join(tmpDir, "write_protected_usb.img")
	img := make([]byte, 16*1024*1024)
	// Simulate an ISO9660 CD001 primary volume descriptor at offset 32768
	copy(img[32768:32774], []byte("\x01CD001"))
	if err := os.WriteFile(usbPath, img, 0644); err != nil {
		t.Fatalf("failed creating synthetic USB image: %v", err)
	}

	if err := ResetUSBToCleanFAT32(usbPath); err != nil {
		t.Fatalf("ResetUSBToCleanFAT32 failed: %v", err)
	}

	after, err := os.ReadFile(usbPath)
	if err != nil {
		t.Fatalf("failed reading reset USB image: %v", err)
	}
	if bytes.Equal(after[32768:32774], []byte("\x01CD001")) {
		t.Fatalf("expected ISO9660 CD001 descriptor at offset 32768 to be wiped")
	}
	if string(after[512:520]) != "EFI PART" {
		t.Fatalf("expected clean EFI PART GPT header at LBA 1 after reset")
	}
	if after[446+4] != 0xEE || after[510] != 0x55 || after[511] != 0xAA {
		t.Fatalf("expected standard 0xEE Protective MBR at LBA 0 after reset")
	}
	// Verify FAT32 boot sector at 1MB boundary (offset 1048576)
	if string(after[1048576+82:1048576+90]) != "FAT32   " {
		t.Fatalf("expected FAT32 boot sector at 1MB offset (LBA 2048), got %q", string(after[1048576+82:1048576+90]))
	}
	// Verify one-click next-boot USB launchers and log file are pre-created on INSTANTUSB
	if !bytes.Contains(after[1048576:], []byte("Boot-USB-Next-Restart-Windows.bat")) && !bytes.Contains(after[1048576:], []byte("bcdedit /enum firmware")) {
		t.Fatalf("expected Windows one-click next-boot launcher on INSTANTUSB FAT32 partition")
	}
	if !bytes.Contains(after[1048576:], []byte("efibootmgr -n")) {
		t.Fatalf("expected Linux one-click next-boot launcher on INSTANTUSB FAT32 partition")
	}
}

func TestExtractISO9660ToEditableFAT32AndVerifyUSB(t *testing.T) {
	tmpDir := t.TempDir()
	stageDir := filepath.Join(tmpDir, "cidata")
	_ = os.MkdirAll(stageDir, 0755)

	cfg := ServerProfileConfig{
		Hostname:    "instant-server",
		Username:    "admin",
		Password:    "SecretPass123!",
		InstallMode: "interactive_gui_disk",
	}
	if err := ValidateProfileConfig(cfg); err != nil {
		t.Fatalf("expected interactive_gui_disk mode to be valid, got: %v", err)
	}
	if _, err := StageCIDATAPayload(stageDir, cfg, []byte("#!/usr/bin/env bash\\necho ok\\n"), []byte("<html></html>")); err != nil {
		t.Fatalf("StageCIDATAPayload failed: %v", err)
	}

	// Build a synthetic ISO9660 image with Primary Volume Descriptor at sector 16 (offset 32768),
	// Root Directory at sector 18, and grub.cfg at sector 19
	isoPath := filepath.Join(tmpDir, "ubuntu_editable.iso")
	isoBytes := make([]byte, 128*1024)
	pvd := isoBytes[32768 : 32768+2048]
	pvd[0] = 1
	copy(pvd[1:6], []byte("CD001"))
	pvd[6] = 1
	// Root directory record at PVD+156 pointing to LBA 18 (offset 36864), length 2048
	pvd[156+0] = 34
	binary.LittleEndian.PutUint32(pvd[156+2:156+6], 18)
	binary.LittleEndian.PutUint32(pvd[156+10:156+14], 2048)
	pvd[156+25] = 0x02

	// Root directory at LBA 18 (offset 36864): "." entry, ".." entry, and "grub.cfg;1" file entry at LBA 19
	rootDir := isoBytes[18*2048 : 19*2048]
	rootDir[0] = 34
	binary.LittleEndian.PutUint32(rootDir[2:6], 18)
	binary.LittleEndian.PutUint32(rootDir[10:14], 2048)
	rootDir[25] = 0x02
	rootDir[32] = 1
	rootDir[33] = 0

	rootDir[34+0] = 34
	binary.LittleEndian.PutUint32(rootDir[34+2:34+6], 18)
	binary.LittleEndian.PutUint32(rootDir[34+10:34+14], 2048)
	rootDir[34+25] = 0x02
	rootDir[34+32] = 1
	rootDir[34+33] = 1

	grubContent := []byte("set timeout=30\nmenuentry \"Install Ubuntu Server\" {\n\tset gfxpayload=keep\n\tlinux\t/casper/vmlinuz  ---\n\tinitrd\t/casper/initrd\n}\n")
	copy(isoBytes[19*2048:], grubContent)

	fileRec := rootDir[68:]
	fileName := "GRUB.CFG;1"
	fileRec[0] = byte(33 + len(fileName) + 1)
	binary.LittleEndian.PutUint32(fileRec[2:6], 19)
	binary.LittleEndian.PutUint32(fileRec[10:14], uint32(len(grubContent)))
	fileRec[25] = 0x00
	fileRec[32] = byte(len(fileName))
	copy(fileRec[33:], []byte(fileName))

	if err := os.WriteFile(isoPath, isoBytes, 0644); err != nil {
		t.Fatalf("failed writing synthetic ISO9660: %v", err)
	}

	usbPath := filepath.Join(tmpDir, "editable_usb.img")
	if err := os.WriteFile(usbPath, make([]byte, 80*1024*1024), 0644); err != nil {
		t.Fatalf("failed creating synthetic USB image: %v", err)
	}

	res, err := StreamISOAndCidataToRawDisk(context.Background(), isoPath, usbPath, stageDir, true, true, nil, nil)
	if err != nil {
		t.Fatalf("StreamISOAndCidataToRawDisk (editable FAT32 mode) failed: %v", err)
	}
	if res.GrubPatches < 1 {
		t.Fatalf("expected GRUB autoinstall patch in extracted FAT32 ISO partition, got %d", res.GrubPatches)
	}

	report, err := VerifyBootableUSB(usbPath)
	if err != nil {
		t.Fatalf("VerifyBootableUSB failed: %v", err)
	}
	if !report.Passed || len(report.Partitions) < 2 {
		t.Fatalf("expected VerifyBootableUSB to pass with >=2 partitions, got: %+v", report)
	}
}

func TestNonDestructiveDualBootNeverWipesDisk(t *testing.T) {
	forbidden := []string{
		"wipefs",
		"sgdisk --zap-all",
		"dd if=/dev/zero",
		"lvremove",
		"vgremove",
		"pvremove",
	}
	for _, f := range forbidden {
		if strings.Contains(payload.ScriptDynamicDualBootStorageSh, f) {
			t.Fatalf("CRITICAL SAFETY VIOLATION: payload.ScriptDynamicDualBootStorageSh must never contain destructive command %q", f)
		}
	}
	dualBootPy := string(payload.LoadBundledScript("dualboot_storage.py", payload.DefaultDualBootStoragePy))
	for _, f := range forbidden {
		if strings.Contains(dualBootPy, f) {
			t.Fatalf("CRITICAL SAFETY VIOLATION: dualboot_storage.py must never contain destructive command %q", f)
		}
	}
	if !strings.Contains(dualBootPy, `"preserve": True`) {
		t.Fatalf("expected dualboot_storage.py to set preserve: True on disk and existing partitions")
	}
}
