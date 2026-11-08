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

	// Verify Minecraft Studio Compose generation with Geyser+Floodgate crossplay, Aikar flags, Offline mode, Modrinth, and RCON backup sidecar
	mcCfg := DefaultMinecraftConfig()
	mcCfg.ServerType = "GEYSER_FLOODGATE"
	mcCfg.MemoryHeap = "6G"
	mcCfg.OnlineMode = false
	mcCfg.OpsList = "nihal,admin"
	mcCfg.ModrinthProjects = "worldedit,lithium"
	mcYAML := BuildMinecraftComposeYAML(mcCfg)
	for _, token := range []string{
		`TYPE: "PAPER"`,
		`MEMORY: "6G"`,
		`USE_AIKAR_FLAGS: "true"`,
		`ONLINE_MODE: "false"`,
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
	isoPath := filepath.Join(stageDir, "boot.iso")
	usbPath := filepath.Join(stageDir, "usb_disk.img")
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
