// usb/flasher_test.go
package usb

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPageAlignedDMABufferAllocation(t *testing.T) {
	for _, size := range []int{4096, 65536, DMAChunkSizeBytes} {
		buf := AllocPageAlignedBuffer(size, SectorAlignmentBytes)
		if len(buf) != size {
			t.Fatalf("expected aligned buffer length %d, got %d", size, len(buf))
		}
		if !IsPointerPageAligned(buf, SectorAlignmentBytes) {
			t.Fatalf("allocated buffer of size %d is not 4096-byte page aligned", size)
		}
	}
}

func TestStreamISOToRawDiskAndTailPadding(t *testing.T) {
	tmpDir := t.TempDir()
	isoPath := filepath.Join(tmpDir, "synthetic_ubuntu.iso")
	targetPath := filepath.Join(tmpDir, "virtual_usb.img")

	// Create an intentionally unaligned ISO (8192 + 1337 bytes) with a GPT header at LBA 1 and GRUB cmdline
	isoSize := 8192 + 1337
	isoData := make([]byte, isoSize)
	copy(isoData[512:520], []byte("EFI PART"))
	binary.LittleEndian.PutUint32(isoData[512+12:512+16], 92)
	copy(isoData[2048:], []byte("menuentry 'Install Ubuntu' {\n  linux /casper/vmlinuz quiet splash ---\n}\n"))

	if err := os.WriteFile(isoPath, isoData, 0644); err != nil {
		t.Fatalf("failed writing synthetic ISO: %v", err)
	}
	if err := os.WriteFile(targetPath, make([]byte, 64*1024), 0644); err != nil {
		t.Fatalf("failed creating virtual USB image: %v", err)
	}

	var events []FlashProgressEvent
	pg := NewPauseGate()
	res, err := StreamISOToRawDisk(
		context.Background(),
		isoPath,
		targetPath,
		true,
		true,
		pg,
		func(ev FlashProgressEvent) {
			events = append(events, ev)
		},
	)
	if err != nil {
		t.Fatalf("StreamISOToRawDisk failed: %v", err)
	}
	if res == nil || res.Percent != 100 {
		t.Fatalf("expected 100%% completion, got %+v", res)
	}
	if res.GrubPatches < 1 {
		t.Fatalf("expected at least 1 in-stream GRUB autoinstall patch, got %d", res.GrubPatches)
	}
	if len(res.SHA256Hex) != 64 {
		t.Fatalf("expected 64-char SHA-256 digest, got %q", res.SHA256Hex)
	}

	writtenDisk, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("failed reading virtual USB target: %v", err)
	}
	if !bytes.Contains(writtenDisk, []byte("autoinstall  ---")) {
		t.Fatalf("expected written USB image to contain patched 'autoinstall  ---' kernel cmdline")
	}
}

func TestPauseResumeAndCancelGate(t *testing.T) {
	pg := NewPauseGate()
	if pg.IsPaused() {
		t.Fatalf("expected new PauseGate to start unpaused")
	}
	pg.Pause()
	if !pg.IsPaused() {
		t.Fatalf("expected PauseGate to report paused after Pause()")
	}

	resumed := make(chan error, 1)
	go func() {
		resumed <- pg.WaitIfPaused(context.Background())
	}()

	time.Sleep(30 * time.Millisecond)
	pg.Resume()

	select {
	case err := <-resumed:
		if err != nil {
			t.Fatalf("expected nil error after Resume(), got %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("WaitIfPaused did not unblock after Resume()")
	}
}

func TestTripleLockSafetyValidation(t *testing.T) {
	passedSys, reasonsSys := EvaluateTripleLockSafety("nvme", false, 512.0, []string{"C:\\"}, `\\.\PhysicalDrive0`)
	if passedSys || len(reasonsSys) == 0 {
		t.Fatalf("expected system NVMe disk 0 to be blocked by Triple-Lock")
	}

	passedMacSys, reasonsMacSys := EvaluateTripleLockSafety("usb", true, 256.0, []string{"/System/Volumes/Data"}, "/dev/disk1")
	if passedMacSys || len(reasonsMacSys) == 0 {
		t.Fatalf("expected macOS APFS synthesized disk1 /System/Volumes/Data to be blocked by Triple-Lock")
	}

	passedUSB, reasonsUSB := EvaluateTripleLockSafety("usb", true, 32.0, []string{"D:\\"}, `\\.\PhysicalDrive2`)
	if !passedUSB || len(reasonsUSB) != 0 {
		t.Fatalf("expected valid 32GB removable USB drive to pass Triple-Lock, got reasons: %v", reasonsUSB)
	}

	// Verify attached VHD / VHDX virtual drives (where Windows reports Fixed hard disk media / removable=false) pass Triple-Lock
	passedVHD, reasonsVHD := EvaluateTripleLockSafety("virtual", false, 32.0, []string{"E:\\"}, `\\.\PhysicalDrive1`)
	if !passedVHD || len(reasonsVHD) != 0 {
		t.Fatalf("expected mounted VHD/VHDX virtual drive on PhysicalDrive1 to pass Triple-Lock, got reasons: %v", reasonsVHD)
	}

	// Verify direct .vhdx / .vhd / .vmdk file paths pass Triple-Lock
	passedVHDXFile, reasonsVHDXFile := EvaluateTripleLockSafety("sata", false, 32.0, nil, `D:\VMs\instant-server-test.vhdx`)
	if !passedVHDXFile || len(reasonsVHDXFile) != 0 {
		t.Fatalf("expected .vhdx virtual disk file path to pass Triple-Lock, got reasons: %v", reasonsVHDXFile)
	}

	// Verify PhysicalDrive0 / C:\ is STILL blocked even if bus is reported as virtual (e.g. native VHD boot host OS)
	passedSysVHD, reasonsSysVHD := EvaluateTripleLockSafety("virtual", true, 128.0, []string{"C:\\"}, `\\.\PhysicalDrive0`)
	if passedSysVHD || len(reasonsSysVHD) == 0 {
		t.Fatalf("expected system boot drive PhysicalDrive0/C: to remain blocked even when bus is virtual")
	}
}

func TestAutoinstallAndDualBoot64TBValidation(t *testing.T) {
	validCfg := ServerProfileConfig{
		Hostname:             "instant-server",
		Username:             "admin",
		InstallMode:          "dual_boot",
		DualBootAllocationGB: 4096, // 4 TB
		UbuntuRelease:        "26.04",
		StorageLayout:        "direct",
	}
	if err := ValidateProfileConfig(validCfg); err != nil {
		t.Fatalf("expected 4TB dual-boot config to be valid, got: %v", err)
	}

	invalidCfg := validCfg
	invalidCfg.DualBootAllocationGB = 70000 // > 64 TB
	if err := ValidateProfileConfig(invalidCfg); err == nil {
		t.Fatalf("expected >64TB dual-boot allocation to be rejected")
	}

	wipeBlockedCfg := validCfg
	wipeBlockedCfg.ClusterMode = "usb_multi_node_auto"
	wipeBlockedCfg.ClusterStoragePolicy = "wipe_secondary_expand"
	wipeBlockedCfg.ClusterConfirmWipeSecondary = ""
	if err := ValidateProfileConfig(wipeBlockedCfg); err == nil {
		t.Fatalf("expected secondary server wipe without WIPE token to be blocked")
	}

	wipeConfirmedCfg := wipeBlockedCfg
	wipeConfirmedCfg.ClusterConfirmWipeSecondary = "WIPE-SECONDARY"
	if err := ValidateProfileConfig(wipeConfirmedCfg); err != nil {
		t.Fatalf("expected confirmed secondary wipe to pass validation: %v", err)
	}

	tmpStage := t.TempDir()
	crlfScript := []byte("#!/usr/bin/env bash\r\necho 'hello'\r\n")
	files, err := StageCIDATAPayload(tmpStage, validCfg, crlfScript, []byte("<html></html>"))
	if err != nil || len(files) < 4 {
		t.Fatalf("StageCIDATAPayload failed: %v (files=%v)", err, files)
	}
	stagedSh, _ := os.ReadFile(filepath.Join(tmpStage, "laptop_hardening.sh"))
	if bytes.Contains(stagedSh, []byte("\r")) {
		t.Fatalf("expected staged laptop_hardening.sh to have all CR (\\r) stripped")
	}
}

func TestCustomWebAppParser(t *testing.T) {
	runEntry := ParseCustomAppInput("docker run -d -p 8888:80 --name custom-wiki nginx:alpine", "", 0)
	if runEntry.Port != 8888 || !strings.Contains(runEntry.Image, "nginx:alpine") {
		t.Fatalf("unexpected docker run parse result: %+v", runEntry)
	}

	ghEntry := ParseCustomAppInput("https://github.com/louislam/uptime-kuma", "", 0)
	if !strings.Contains(ghEntry.Image, "ghcr.io/louislam/uptime-kuma:latest") {
		t.Fatalf("unexpected GitHub URL parse result: %+v", ghEntry)
	}
}

func TestVerifiedAppCatalogAndMinecraftStudio(t *testing.T) {
	// Verify Nextcloud maps host 8081 to internal 80 (not 8081:8081)
	ncYAML := BuildVerifiedAppComposeYAML("nextcloud", "", 0, nil)
	if !strings.Contains(ncYAML, `"8081:80"`) {
		t.Fatalf("expected Nextcloud compose to map 8081:80, got:\n%s", ncYAML)
	}

	// Verify Pi-hole includes 53/tcp, 53/udp, 8085:80, and NET_ADMIN capability
	piholeYAML := BuildVerifiedAppComposeYAML("pihole", "", 0, nil)
	if !strings.Contains(piholeYAML, `"53:53/udp"`) || !strings.Contains(piholeYAML, "NET_ADMIN") || !strings.Contains(piholeYAML, `"8085:80"`) {
		t.Fatalf("expected Pi-hole compose to include 53/udp, 8085:80, and NET_ADMIN, got:\n%s", piholeYAML)
	}

	// Verify WireGuard (wg-easy) includes 51820:51820/udp, NET_ADMIN, SYS_MODULE, and ipv4 forwarding sysctl
	wgYAML := BuildVerifiedAppComposeYAML("wg_easy", "", 0, nil)
	if !strings.Contains(wgYAML, `"51820:51820/udp"`) || !strings.Contains(wgYAML, "SYS_MODULE") || !strings.Contains(wgYAML, "net.ipv4.ip_forward=1") {
		t.Fatalf("expected wg-easy compose to include 51820/udp, SYS_MODULE, and ip_forward sysctl, got:\n%s", wgYAML)
	}

	// Verify Ollama + Open WebUI maps host 3005 to internal 8080
	ollamaYAML := BuildVerifiedAppComposeYAML("ollama_webui", "", 0, nil)
	if !strings.Contains(ollamaYAML, `"3005:8080"`) {
		t.Fatalf("expected Ollama + Open WebUI compose to map 3005:8080, got:\n%s", ollamaYAML)
	}

	// Verify Minecraft Studio Compose generation with 26.3 default, WHITE_LIST: false, Geyser+Floodgate crossplay, Aikar flags, Offline mode, Modrinth, and RCON backup sidecar
	mcCfg := DefaultMinecraftConfig()
	if mcCfg.Version != "26.3" {
		t.Fatalf("expected DefaultMinecraftConfig Version to be 26.3, got %q", mcCfg.Version)
	}
	mcCfg.ServerType = "GEYSER_FLOODGATE"
	mcCfg.MemoryHeap = "6G"
	mcCfg.OnlineMode = false
	mcCfg.OpsList = "nihal,admin"
	mcCfg.ModrinthProjects = "worldedit,lithium"
	mcYAML := BuildMinecraftComposeYAML(mcCfg)
	for _, token := range []string{
		`TYPE: "PAPER"`,
		`VERSION: "26.3"`,
		`MEMORY: "6G"`,
		`USE_AIKAR_FLAGS: "true"`,
		`ONLINE_MODE: "false"`,
		`WHITE_LIST: "false"`,
		`OPS: "nihal,admin"`,
		`MODRINTH_PROJECTS: "worldedit,lithium,geyser,floodgate"`,
		`"19132:19132/udp"`,
		`image: itzg/mc-backup:latest`,
	} {
		if !strings.Contains(mcYAML, token) {
			t.Fatalf("expected Minecraft Studio YAML to contain %q, got:\n%s", token, mcYAML)
		}
	}
}

func TestSha512CryptAndAutoinstallChrootSafety(t *testing.T) {
	hash1 := Sha512Crypt("MySuperSecretPass123!", "instantserverlts")
	if !strings.HasPrefix(hash1, "$6$instantserverlts$") {
		t.Fatalf("expected valid $6$instantserverlts$ prefix, got %q", hash1)
	}
	if len(hash1) != len("$6$instantserverlts$")+86 {
		t.Fatalf("expected 86-char SHA-512 crypt body, got len=%d (%q)", len(hash1), hash1)
	}
	// Deterministic check
	hash2 := Sha512Crypt("MySuperSecretPass123!", "instantserverlts")
	if hash1 != hash2 {
		t.Fatalf("Sha512Crypt is not deterministic: %q != %q", hash1, hash2)
	}

	cfg := ServerProfileConfig{
		Hostname:             "homelab-pro",
		Username:             "nihal",
		Password:             "MySuperSecretPass123!",
		WifiSSID:             "HomeFiber5G",
		WifiPassword:         "WifiSecret99!",
		EnableUSBTethering:   true,
		InstallMode:          "dual_boot",
		DualBootAllocationGB: 1024,
		SelectedApps:         []string{"minecraft", "nextcloud", "pihole"},
		AppOverrides: map[string]AppOverride{
			"minecraft": {
				HostPort:    25565,
				MemoryLimit: "6g",
				ConfigFiles: map[string]string{
					"server.properties": "motd=Custom Server Properties\nonline-mode=false\n",
				},
			},
		},
	}

	userData := GenerateAutoinstallUserData(cfg)
	if strings.Contains(userData, "reset-partition: true") {
		t.Fatalf("expected user-data NOT to include OEM reset-partition: true")
	}
	for _, mustContain := range []string{
		hash1,
		`"HomeFiber5G":`,
		`password: "WifiSecret99!"`,
		`route-metric: 100`,
		`route-metric: 600`,
		`mkdir -p /target/opt/instant-server`,
		`curtin in-target -- bash /opt/instant-server/laptop_hardening.sh`,
	} {
		if !strings.Contains(userData, mustContain) {
			t.Fatalf("expected user-data to contain %q, got:\n%s", mustContain, userData)
		}
	}

	stageDir := t.TempDir()
	written, err := StageCIDATAPayload(stageDir, cfg, []byte("#!/usr/bin/env bash\n"), []byte("<html></html>"))
	if err != nil {
		t.Fatalf("StageCIDATAPayload failed: %v", err)
	}
	if len(written) < 7 {
		t.Fatalf("expected >= 7 staged files (including env.conf, app compose, and custom config_files), got %v", written)
	}
	srvPropsPath := filepath.Join(stageDir, "casaos_apps", "minecraft", "config_files", "server.properties")
	srvPropsBytes, err := os.ReadFile(srvPropsPath)
	if err != nil || !strings.Contains(string(srvPropsBytes), "online-mode=false") {
		t.Fatalf("expected staged server.properties override at %s, err=%v", srvPropsPath, err)
	}
}

func TestBuildCidataFAT16ImageAndGPTInjection(t *testing.T) {
	stageDir := t.TempDir()
	diskDir := t.TempDir()
	cfg := ServerProfileConfig{
		Hostname:             "instant-server",
		Username:             "admin",
		Password:             "SecretPass123!",
		InstallMode:          "dual_boot",
		DualBootAllocationGB: 64,
		SelectedApps:         []string{"jellyfin", "pihole", "immich", "paperless"},
	}
	_, err := StageCIDATAPayload(stageDir, cfg, []byte("#!/usr/bin/env bash\necho ok\n"), []byte("<html></html>"))
	if err != nil {
		t.Fatalf("StageCIDATAPayload failed: %v", err)
	}

	fatImg, err := BuildCidataFAT16Image(stageDir)
	if err != nil {
		t.Fatalf("BuildCidataFAT16Image failed: %v", err)
	}
	// Check BIOS Parameter Block signature 0x55 0xAA and volume label "CIDATA     "
	if fatImg[510] != 0x55 || fatImg[511] != 0xAA {
		t.Fatalf("missing 0x55AA boot sector signature in FAT16 image")
	}
	if string(fatImg[43:54]) != "CIDATA     " || string(fatImg[54:62]) != "FAT16   " {
		t.Fatalf("unexpected FAT16 BPB label/type: %q / %q", string(fatImg[43:54]), string(fatImg[54:62]))
	}
	if !bytes.Contains(fatImg, []byte("#cloud-config")) {
		t.Fatalf("expected FAT16 image data region to contain #cloud-config from user-data")
	}

	// Stream synthetic ISO + CIDATA FAT16 image into virtual USB disk and verify GPT Partition #2 injection
	isoPath := filepath.Join(diskDir, "boot.iso")
	usbPath := filepath.Join(diskDir, "usb_disk.img")
	isoBytes := make([]byte, 64*1024)
	copy(isoBytes[512:520], []byte("EFI PART"))
	binary.LittleEndian.PutUint32(isoBytes[512+12:512+16], 92)
	binary.LittleEndian.PutUint32(isoBytes[512+80:512+84], 128)
	binary.LittleEndian.PutUint32(isoBytes[512+84:512+88], 128)
	_ = os.WriteFile(isoPath, isoBytes, 0644)
	_ = os.WriteFile(usbPath, make([]byte, 20*1024*1024), 0644)

	_, err = StreamISOAndCidataToRawDisk(context.Background(), isoPath, usbPath, stageDir, true, true, nil, nil)
	if err != nil {
		t.Fatalf("StreamISOAndCidataToRawDisk failed: %v", err)
	}

	usbRaw, err := os.ReadFile(usbPath)
	if err != nil {
		t.Fatalf("failed reading usb_disk.img: %v", err)
	}
	// Verify CIDATA FAT16 partition was written at 1MB boundary (offset 1048576)
	if string(usbRaw[1048576+43:1048576+54]) != "CIDATA     " {
		t.Fatalf("expected CIDATA FAT16 BPB at 1MB boundary on USB disk, got %q", string(usbRaw[1048576+43:1048576+54]))
	}
}

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
}