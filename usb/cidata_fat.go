// usb/cidata_fat.go
// -----------------------------------------------------------------------------
// Pure-Go FAT16 ("CIDATA") and FAT32 ("INSTANTUSB") filesystem image builders.
//
// Why this file exists:
//   - Cloud-init on Ubuntu Server Live ISOs automatically looks for a FAT volume
//     labeled "CIDATA" containing user-data, meta-data, and helper scripts.
//   - The remaining space on the USB stick is formatted as a normal FAT32 drive
//     labeled "INSTANTUSB" containing:
//     1. logs/                         (folder for live installer debug logs)
//     2. INSTANT-SERVER-INSTALL.log    (main live installation log)
//     3. Boot-USB-Next-Restart-Windows.bat (one-click Windows next-boot USB runner)
//     4. boot-usb-next-restart-linux.sh    (one-click Linux next-boot USB runner)
//
// -----------------------------------------------------------------------------
package usb

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf16"
)

const (
	fatBytesPerSector    = 512
	fatSectorsPerCluster = 4     // 2,048 bytes (2 KB) per cluster
	fatTotalSectors      = 32880 // > 16 MB (4,096 data clusters => valid FAT16)
	fatReservedSectors   = 4
	fatNumTables         = 2
	fatSectorsPerFAT     = 32
	fatRootDirEntries    = 512
	fatRootDirSectors    = (fatRootDirEntries * 32) / fatBytesPerSector // 32 sectors
)

type fatStagedFile struct {
	Name string
	Data []byte
	Name string
	Data []byte
}

func computeLFNChecksum(shortName [11]byte) byte {
	var sum byte
	for i := 0; i < 11; i++ {
		sum = ((sum & 1) << 7) + (sum >> 1) + shortName[i]
	}
	return sum
}

func makeShort83Name(name string, index int) [11]byte {
	var out [11]byte
	for i := range out {
		out[i] = ' '
	}
	upper := strings.ToUpper(name)
	ext := ""
	base := upper
	if dot := strings.LastIndex(upper, "."); dot > 0 {
		base = upper[:dot]
		ext = upper[dot+1:]
	}
	cleanChars := func(s string) string {
		var sb strings.Builder
		for _, r := range s {
			if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
				sb.WriteRune(r)
			}
		}
		return sb.String()
	}
	cleanBase := cleanChars(base)
	cleanExt := cleanChars(ext)
	if cleanBase == "" {
		cleanBase = "FILE"
	}
	suffix := fmt.Sprintf("~%d", index)
	maxBase := 8 - len(suffix)
	if len(cleanBase) > maxBase {
		cleanBase = cleanBase[:maxBase]
	}
	shortBase := cleanBase + suffix
	for i := 0; i < len(shortBase) && i < 8; i++ {
		out[i] = shortBase[i]
	}
	for i := 0; i < len(cleanExt) && i < 3; i++ {
		out[8+i] = cleanExt[i]
	}
	return out
}

func buildLFNEntries(longName string, shortName [11]byte) [][32]byte {
	utf16Chars := utf16.Encode([]rune(longName))
	utf16Chars = append(utf16Chars, 0x0000)
	for len(utf16Chars)%13 != 0 {
		utf16Chars = append(utf16Chars, 0xFFFF)
	}
	numEntries := len(utf16Chars) / 13
	chk := computeLFNChecksum(shortName)
	var entries [][32]byte

	// VFAT LFN entries are stored in reverse order (N|0x40 down to 1) immediately before the 8.3 entry
	for seq := numEntries; seq >= 1; seq-- {
		var ent [32]byte
		ord := byte(seq)
		if seq == numEntries {
			ord |= 0x40
		}
		ent[0] = seq
		ent[11] = 0x0F // ATTR_LONG_NAME
		ent[12] = 0x00
		ent[13] = chk
		ent[26] = 0x00
		ent[27] = 0x00

		chunk := utf16Chars[(seq-1)*13 : seq*13]
		offsets := [13]int{1, 3, 5, 7, 9, 14, 16, 18, 20, 22, 24, 28, 30}
		for i, codeUnit := range chunk {
			binary.LittleEndian.PutUint16(ent[offsets[i]:offsets[i]+2], codeUnit)
		}
		entries = append(entries, ent)
	}
	return entries
}

// BuildCidataFAT16Image packs all files in stageDir into a self-contained
// FAT16 filesystem image with Volume Label "CIDATA" so Ubuntu Live Server
// NoCloud / Subiquity autoinstall discovers `/user-data` and `/meta-data`
// automatically on any UEFI or Legacy BIOS machine.
func BuildCidataFAT16Image(stageDir string) ([]byte, error) {
	entries, err := os.ReadDir(stageDir)
	if err != nil {
		return nil, err
	}


	var files []fatStagedFile
	for _, ent := range entries {
		if ent.IsDir() || strings.HasSuffix(ent.Name(), ".img") || strings.HasSuffix(ent.Name(), ".iso") {
		if ent.IsDir() || strings.HasSuffix(ent.Name(), ".img") || strings.HasSuffix(ent.Name(), ".iso") {
			continue
		}
		raw, rErr := os.ReadFile(filepath.Join(stageDir, ent.Name()))
		if rErr != nil {
			return nil, rErr
		}
		files = append(files, fatStagedFile{
			Name: ent.Name(),
			Data: raw,
		})
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name < files[j].Name
	})

	bytesPerCluster := fatBytesPerSector * fatSectorsPerCluster
	neededDataClusters := 2
	for _, f := range files {
		c := (len(f.Data) + bytesPerCluster - 1) / bytesPerCluster
		if c == 0 {
			c = 1
		}
		neededDataClusters += c
	}

	totalSectors := fatTotalSectors
	sectorsPerFAT := fatSectorsPerFAT
	dataStartSector := fatReservedSectors + (fatNumTables * sectorsPerFAT) + fatRootDirSectors
	availClusters := (totalSectors - dataStartSector) / fatSectorsPerCluster
	if neededDataClusters+64 > availClusters {
		targetClusters := neededDataClusters + 128
		if targetClusters < 4100 {
			targetClusters = 4100
		}
		sectorsPerFAT = ((targetClusters * 2) + fatBytesPerSector - 1) / fatBytesPerSector
		if sectorsPerFAT < fatSectorsPerFAT {
			sectorsPerFAT = fatSectorsPerFAT
		}
		dataStartSector = fatReservedSectors + (fatNumTables * sectorsPerFAT) + fatRootDirSectors
		totalSectors = dataStartSector + (targetClusters * fatSectorsPerCluster)
		if totalSectors%8 != 0 {
			totalSectors = ((totalSectors + 7) / 8) * 8
		}
	}

	totalBytes := totalSectors * fatBytesPerSector
	img := make([]byte, totalBytes)

	// 1. Write FAT16 BIOS Parameter Block (BPB) in Sector 0
	img[0] = 0xEB
	img[1] = 0x3C
	img[2] = 0x90
	copy(img[3:11], []byte("MSDOS5.0"))
	binary.LittleEndian.PutUint16(img[11:13], fatBytesPerSector)
	img[13] = fatSectorsPerCluster
	binary.LittleEndian.PutUint16(img[14:16], fatReservedSectors)
	img[16] = fatNumTables
	binary.LittleEndian.PutUint16(img[17:19], fatRootDirEntries)
	if totalSectors <= 65535 {
		binary.LittleEndian.PutUint16(img[19:21], uint16(totalSectors))
		binary.LittleEndian.PutUint32(img[32:36], 0)
	} else {
		binary.LittleEndian.PutUint16(img[19:21], 0)
		binary.LittleEndian.PutUint32(img[32:36], uint32(totalSectors))
	}
	img[21] = 0xF8 // Fixed media descriptor
	binary.LittleEndian.PutUint16(img[22:24], uint16(sectorsPerFAT))
	binary.LittleEndian.PutUint16(img[24:26], 63)  // Sectors per track
	binary.LittleEndian.PutUint16(img[26:28], 255) // Heads
	binary.LittleEndian.PutUint32(img[28:32], 0)   // Hidden sectors

	img[36] = 0x80 // Drive number
	img[38] = 0x29 // Extended boot signature
	binary.LittleEndian.PutUint32(img[39:43], 0x20261108)
	copy(img[43:54], []byte("CIDATA     "))
	copy(img[54:62], []byte("FAT16   "))
	img[510] = 0x55
	img[511] = 0xAA

	// 2. Initialize FAT #1 and FAT #2 reserved cluster entries 0 and 1
	fat1Offset := fatReservedSectors * fatBytesPerSector
	fat2Offset := (fatReservedSectors + sectorsPerFAT) * fatBytesPerSector
	writeFATEntry := func(cluster uint16, val uint16) {
		off := int(cluster) * 2
		if fat1Offset+off+2 <= len(img) && fat2Offset+off+2 <= len(img) {
			binary.LittleEndian.PutUint16(img[fat1Offset+off:fat1Offset+off+2], val)
			binary.LittleEndian.PutUint16(img[fat2Offset+off:fat2Offset+off+2], val)
	fat2Offset := (fatReservedSectors + sectorsPerFAT) * fatBytesPerSector
	writeFATEntry := func(cluster uint16, val uint16) {
		off := int(cluster) * 2
		if fat1Offset+off+2 <= len(img) && fat2Offset+off+2 <= len(img) {
			binary.LittleEndian.PutUint16(img[fat1Offset+off:fat1Offset+off+2], val)
			binary.LittleEndian.PutUint16(img[fat2Offset+off:fat2Offset+off+2], val)
		}
	}
	writeFATEntry(0, 0xFFF8)
	writeFATEntry(1, 0xFFFF)
	writeFATEntry(0, 0xFFF8)
	writeFATEntry(1, 0xFFFF)

	// 3. Initialize Root Directory with Volume Label entry ("CIDATA     ", ATTR_VOLUME_ID = 0x08)
	rootDirOffset := (fatReservedSectors + fatNumTables*sectorsPerFAT) * fatBytesPerSector
	rootDirOffset := (fatReservedSectors + fatNumTables*sectorsPerFAT) * fatBytesPerSector
	dirCursor := rootDirOffset
	copy(img[dirCursor:dirCursor+11], []byte("CIDATA     "))
	img[dirCursor+11] = 0x08 // Volume Label attribute
	dirCursor += 32
	copy(img[dirCursor:dirCursor+11], []byte("CIDATA     "))
	img[dirCursor+11] = 0x08 // Volume Label attribute
	dirCursor += 32

	// 4. Write staged files into FAT16 data clusters
	nextCluster := uint16(2)


	for idx, f := range files {
		shortName := makeShort83Name(f.Name, idx+1)
		lfnList := buildLFNEntries(f.Name, shortName)
		for _, lfn := range lfnList {
			if dirCursor+32 <= rootDirOffset+(fatRootDirSectors*fatBytesPerSector) {
				copy(img[dirCursor:dirCursor+32], lfn[:])
				dirCursor += 32
			}
		}

		startCluster := uint16(0)
		shortName := makeShort83Name(f.Name, idx+1)
		lfnList := buildLFNEntries(f.Name, shortName)
		for _, lfn := range lfnList {
			if dirCursor+32 <= rootDirOffset+(fatRootDirSectors*fatBytesPerSector) {
				copy(img[dirCursor:dirCursor+32], lfn[:])
				dirCursor += 32
			}
		}

		startCluster := uint16(0)
		remaining := len(f.Data)
		if remaining > 0 {
			startCluster = nextCluster
			numClusters := (remaining + bytesPerCluster - 1) / bytesPerCluster
			startCluster = nextCluster
			numClusters := (remaining + bytesPerCluster - 1) / bytesPerCluster
			for c := 0; c < numClusters; c++ {
				curCluster := nextCluster
				nextCluster++
				if c == numClusters-1 {
					writeFATEntry(curCluster, 0xFFFF) // End of cluster chain
					writeFATEntry(curCluster, 0xFFFF) // End of cluster chain
				} else {
					writeFATEntry(curCluster, nextCluster)
					writeFATEntry(curCluster, nextCluster)
				}
				dataOffset := (dataStartSector * fatBytesPerSector) + int(curCluster-2)*bytesPerCluster
				startByte := c * bytesPerCluster
				endByte := startByte + bytesPerCluster
				if endByte > len(f.Data) {
					endByte = len(f.Data)
				}
				if dataOffset >= 0 && dataOffset+(endByte-startByte) <= len(img) {
					copy(img[dataOffset:dataOffset+(endByte-startByte)], f.Data[startByte:endByte])
				if dataOffset >= 0 && dataOffset+(endByte-startByte) <= len(img) {
					copy(img[dataOffset:dataOffset+(endByte-startByte)], f.Data[startByte:endByte])
				}
			}
		}

		if dirCursor+32 <= rootDirOffset+(fatRootDirSectors*fatBytesPerSector) {
			var stdEnt [32]byte
			copy(stdEnt[0:11], shortName[:])
			stdEnt[11] = 0x20 // ATTR_ARCHIVE
			if f.Name == strings.ToLower(f.Name) {
				stdEnt[12] = 0x18
			}
			binary.LittleEndian.PutUint16(stdEnt[26:28], startCluster)
			binary.LittleEndian.PutUint32(stdEnt[28:32], uint32(len(f.Data)))
			copy(img[dirCursor:dirCursor+32], stdEnt[:])
			dirCursor += 32
		}
		if dirCursor+32 <= rootDirOffset+(fatRootDirSectors*fatBytesPerSector) {
			var stdEnt [32]byte
			copy(stdEnt[0:11], shortName[:])
			stdEnt[11] = 0x20 // ATTR_ARCHIVE
			if f.Name == strings.ToLower(f.Name) {
				stdEnt[12] = 0x18
			}
			binary.LittleEndian.PutUint16(stdEnt[26:28], startCluster)
			binary.LittleEndian.PutUint32(stdEnt[28:32], uint32(len(f.Data)))
			copy(img[dirCursor:dirCursor+32], stdEnt[:])
			dirCursor += 32
		}
	}

	return img, nil
}

func BuildFAT16CidataVolume(stageDir string, _ string) ([]byte, error) {
	return BuildCidataFAT16Image(stageDir)
}

func buildWindowsNextBootLauncherBat() []byte {
	script := strings.Join([]string{
		"@echo off",
		"title Instant Server - Boot This USB on Next Restart",
		"net session >nul 2>&1",
		"if %errorLevel% neq 0 (",
		"    echo Requesting Administrator privileges to configure UEFI next-boot USB target...",
		"    powershell -NoProfile -Command \"Start-Process -FilePath '%~f0' -Verb RunAs\"",
		"    exit /b",
		")",
		"echo ============================================================================",
		"echo  INSTANT SERVER - ONE-CLICK BOOT INTO USB ON NEXT RESTART (NO BIOS MENU)",
		"echo ============================================================================",
		"powershell -NoProfile -ExecutionPolicy Bypass -Command ^",
		"  \"$out = bcdedit /enum firmware 2>$null | Out-String; \" ^",
		"  \"$blocks = $out -split '(?=identifier\\s+\\{)'; \" ^",
		"  \"$usbGuid = ''; \" ^",
		"  \"foreach ($b in $blocks) { \" ^",
		"  \"  if ($b -match 'identifier\\s+(\\{[0-9a-fA-F-]+\\})' -and $b -match '(?i)(USB|Ubuntu|Removable|Flash|Instant)') { \" ^",
		"  \"    $usbGuid = $Matches[1]; break \" ^",
		"  \"  } \" ^",
		"  \"}; \" ^",
		"  \"if ($usbGuid) { \" ^",
		"  \"  Write-Host ('Found UEFI USB Boot Entry: ' + $usbGuid + '. Setting one-time bootsequence...'); \" ^",
		"  \"  bcdedit /set '{fwbootmgr}' bootsequence $usbGuid | Out-Null; \" ^",
		"  \"  Write-Host 'Rebooting directly into Instant Server USB in 3 seconds...'; \" ^",
		"  \"  shutdown /r /t 3 /c 'Instant Server: Rebooting directly into USB installer' \" ^",
		"  \"} else { \" ^",
		"  \"  Write-Host 'Rebooting into UEFI Firmware Boot Selector...'; \" ^",
		"  \"  shutdown /r /fw /t 1 \" ^",
		"  \"}\"",
		"",
	}, "\r\n")
	return []byte(script)
}

func buildLinuxNextBootLauncherSh() []byte {
	script := strings.Join([]string{
		"#!/usr/bin/env bash",
		"# ============================================================================",
		"# Instant Server - One-Click Boot Into USB on Next Restart (Linux)",
		"# Boots directly into this Instant Server USB without entering BIOS/Boot Menu",
		"# using UEFI BootNext (efibootmgr) or direct kernel boot (kexec).",
		"# ============================================================================",
		"set -uo pipefail",
		"if [ \"$(id -u)\" -ne 0 ]; then",
		"    echo \"Requesting root privileges to configure next-boot USB target...\"",
		"    exec sudo bash \"$0\" \"$@\"",
		"fi",
		"",
		"# 1. Try UEFI BootNext (efibootmgr) if booted in UEFI mode",
		"if [ -d /sys/firmware/efi ] && command -v efibootmgr >/dev/null 2>&1; then",
		"    USB_BOOTNUM=$(efibootmgr 2>/dev/null | grep -iE 'USB|Ubuntu|Linpus|Removable|UEFI:' | head -n1 | sed -n 's/^Boot\\([0-9A-Fa-f]\\{4\\}\\).*/\\1/p')",
		"    if [ -n \"$USB_BOOTNUM\" ]; then",
		"        echo \"[Instant Server] Setting UEFI BootNext to Boot${USB_BOOTNUM} and rebooting into USB...\"",
		"        efibootmgr -n \"$USB_BOOTNUM\" && systemctl reboot",
		"        exit 0",
		"    fi",
		"fi",
		"",
		"# 2. Fallback for Legacy BIOS or unlisted UEFI USB: direct kernel kexec from USB ISO partition",
		"ISO_PART=$(blkid -t TYPE=iso9660 -o device 2>/dev/null | head -n1 || true)",
		"if [ -n \"$ISO_PART\" ] && command -v kexec >/dev/null 2>&1; then",
		"    mkdir -p /mnt/instant_usb_iso",
		"    mount -o ro \"$ISO_PART\" /mnt/instant_usb_iso 2>/dev/null || true",
		"    if [ -f /mnt/instant_usb_iso/casper/vmlinuz ] && [ -f /mnt/instant_usb_iso/casper/initrd ]; then",
		"        echo \"[Instant Server] Loading USB kernel directly via kexec (zero BIOS menu)...\"",
		"        kexec -l /mnt/instant_usb_iso/casper/vmlinuz --initrd=/mnt/instant_usb_iso/casper/initrd --command-line=\"BOOT_IMAGE=/casper/vmlinuz autoinstall noprompt ---\" && systemctl kexec",
		"        exit 0",
		"    fi",
		"fi",
		"",
		"echo \"[Instant Server] Rebooting system (use UEFI firmware setup if prompted)...\"",
		"systemctl reboot --firmware-setup 2>/dev/null || reboot",
		"",
	}, "\n")
	return []byte(script)
}

// FormatRemainingUSBAsFAT32 formats sectors [startLBA .. endLBA] on the USB drive
// as a cross-platform FAT32 partition labeled "INSTANTUSB" and pre-creates:
//   - logs/                              (directory for detailed installer logs)
//   - INSTANT-SERVER-INSTALL.log         (main live installation log)
//   - Boot-USB-Next-Restart-Windows.bat  (one-click Windows next-boot USB launcher)
//   - boot-usb-next-restart-linux.sh     (one-click Linux next-boot USB launcher)
func FormatRemainingUSBAsFAT32(disk RawDiskHandle, startLBA uint64, endLBA uint64) error {
	if endLBA <= startLBA+4096 {
	if endLBA <= startLBA+4096 {
		return nil
	}
	totalSectors64 := endLBA - startLBA + 1
	if totalSectors64 > 0xFFFFFFFF {
		totalSectors64 = 0xFFFFFFFF
	}
	totalSectors := uint32(totalSectors64)

	var secPerClus uint32 = 64 // 32 KB clusters (up to 64 GB)
	if totalSectors > 134217728 {
		secPerClus = 128 // 64 KB clusters (> 64 GB)
	} else if totalSectors < 131072 {
		secPerClus = 8 // 4 KB clusters for compact images (< 64 MB) so clusterBytes == 4096
	} else if totalSectors < 16777216 {
		secPerClus = 16 // 8 KB clusters (< 8 GB)
	}

	const reservedSectors uint32 = 32
	const numFATs uint32 = 2
	approxClusters := (totalSectors - reservedSectors) / secPerClus
	fatSectors := ((approxClusters+2)*4 + 511) / 512
	fatSectors = ((fatSectors + 7) / 8) * 8
	if fatSectors < 128 {
		fatSectors = 128
	}

	resBuf := AllocPageAlignedBuffer(int(reservedSectors*512), SectorAlignmentBytes)
	for i := range resBuf {
		resBuf[i] = 0
	}

	writeBootSector := func(b []byte) {
		b[0], b[1], b[2] = 0xEB, 0x58, 0x90
		copy(b[3:11], []byte("MSDOS5.0"))
		binary.LittleEndian.PutUint16(b[11:13], 512)
		b[13] = byte(secPerClus)
		binary.LittleEndian.PutUint16(b[14:16], uint16(reservedSectors))
		b[16] = byte(numFATs)
		binary.LittleEndian.PutUint16(b[17:19], 0)
		binary.LittleEndian.PutUint16(b[19:21], 0)
		b[21] = 0xF8
		binary.LittleEndian.PutUint16(b[22:24], 0)
		binary.LittleEndian.PutUint16(b[17:19], 0)
		binary.LittleEndian.PutUint16(b[19:21], 0)
		b[21] = 0xF8
		binary.LittleEndian.PutUint16(b[22:24], 0)
		binary.LittleEndian.PutUint16(b[24:26], 63)
		binary.LittleEndian.PutUint16(b[26:28], 255)
		binary.LittleEndian.PutUint32(b[28:32], uint32(startLBA))
		binary.LittleEndian.PutUint32(b[32:36], totalSectors)
		binary.LittleEndian.PutUint32(b[36:40], fatSectors)
		binary.LittleEndian.PutUint16(b[40:42], 0)
		binary.LittleEndian.PutUint16(b[42:44], 0)
		binary.LittleEndian.PutUint32(b[44:48], 2) // Root directory starts at Cluster 2
		binary.LittleEndian.PutUint16(b[48:50], 1)
		binary.LittleEndian.PutUint16(b[50:52], 6)
		b[64] = 0x80
		b[66] = 0x29
		binary.LittleEndian.PutUint32(b[67:71], 0x20261112)
		copy(b[71:82], []byte("INSTANTUSB "))
		copy(b[82:90], []byte("FAT32   "))
		b[510] = 0x55
		b[511] = 0xAA
	}
	writeBootSector(resBuf[0:512])
	writeBootSector(resBuf[6*512 : 7*512])

	writeFSInfo := func(fsi []byte) {
		binary.LittleEndian.PutUint32(fsi[0:4], 0x41615252)
		binary.LittleEndian.PutUint32(fsi[484:488], 0x61417272)
		freeClus := uint32(0xFFFFFFFF)
		if approxClusters > 8 {
			freeClus = approxClusters - 8
		}
		binary.LittleEndian.PutUint32(fsi[488:492], freeClus)
		binary.LittleEndian.PutUint32(fsi[492:496], 7) // Next free cluster after 2..6
		fsi[510] = 0x55
		fsi[511] = 0xAA
	}
	writeFSInfo(resBuf[1*512 : 2*512])
	writeFSInfo(resBuf[7*512 : 8*512])

	if _, err := disk.WriteAt(resBuf, int64(startLBA*512)); err != nil {
		return err
	}

	fatInitBytes := int(fatSectors * 512)
	if fatInitBytes > 4*1024*1024 {
		fatInitBytes = 4 * 1024 * 1024
	}
	fatBuf := AllocPageAlignedBuffer(fatInitBytes, SectorAlignmentBytes)
	for i := range fatBuf {
		fatBuf[i] = 0
	}
	// Mark Clusters 0, 1 (reserved) and 2, 3, 4, 5, 6 as allocated end-of-chain
	binary.LittleEndian.PutUint32(fatBuf[0:4], 0x0FFFFFF8)
	for clus := 1; clus <= 6; clus++ {
		binary.LittleEndian.PutUint32(fatBuf[clus*4:(clus+1)*4], 0x0FFFFFFF)
	}

	fat1StartLBA := startLBA + uint64(reservedSectors)
	fat2StartLBA := fat1StartLBA + uint64(fatSectors)
	if err := writeAlignedChunks(disk, fatBuf, int64(fat1StartLBA*512)); err != nil {
		return err
	}
	if err := writeAlignedChunks(disk, fatBuf, int64(fat2StartLBA*512)); err != nil {
		return err
	}

	clusterBytes := int(secPerClus * 512)
	if clusterBytes < 4096 {
		clusterBytes = 4096
	}
	if clusterBytes < 4096 {
		clusterBytes = 4096
	}
	dataRegionStartLBA := fat2StartLBA + uint64(fatSectors)

	// Allocate 5 contiguous clusters (Clusters 2, 3, 4, 5, 6):
	//   Cluster 2: Root Directory
	//   Cluster 3: logs/ subdirectory
	//   Cluster 4: INSTANT-SERVER-INSTALL.log
	//   Cluster 5: Boot-USB-Next-Restart-Windows.bat
	//   Cluster 6: boot-usb-next-restart-linux.sh
	fiveClustersBuf := AllocPageAlignedBuffer(clusterBytes*5, SectorAlignmentBytes)
	for i := range fiveClustersBuf {
		fiveClustersBuf[i] = 0
	}

	// Cluster 2: Volume Label "INSTANTUSB "
	copy(fiveClustersBuf[0:11], []byte("INSTANTUSB "))
	fiveClustersBuf[11] = 0x08
	cursor := 32

	appendRootEntry := func(longName string, idx int, attr byte, cluster uint16, size uint32) {
		shortName := makeShort83Name(longName, idx)
		for _, lfn := range buildLFNEntries(longName, shortName) {
			copy(fiveClustersBuf[cursor:cursor+32], lfn[:])
			cursor += 32
		}
		var stdEnt [32]byte
		copy(stdEnt[0:11], shortName[:])
		stdEnt[11] = attr
		binary.LittleEndian.PutUint16(stdEnt[20:22], 0)
		binary.LittleEndian.PutUint16(stdEnt[26:28], cluster)
		binary.LittleEndian.PutUint32(stdEnt[28:32], size)
		copy(fiveClustersBuf[cursor:cursor+32], stdEnt[:])
		cursor += 32
	}

	// 1. Subdirectory "logs" at Cluster 3
	appendRootEntry("logs", 1, 0x10, 3, 0)
	logsClusterOff := clusterBytes * 1
	copy(fiveClustersBuf[logsClusterOff+0:logsClusterOff+11], []byte(".          "))
	fiveClustersBuf[logsClusterOff+11] = 0x10
	binary.LittleEndian.PutUint16(fiveClustersBuf[logsClusterOff+26:logsClusterOff+28], 3)
	copy(fiveClustersBuf[logsClusterOff+32:logsClusterOff+43], []byte("..         "))
	fiveClustersBuf[logsClusterOff+43] = 0x10
	binary.LittleEndian.PutUint16(fiveClustersBuf[logsClusterOff+58:logsClusterOff+60], 0)

	// 2. File "INSTANT-SERVER-INSTALL.log" at Cluster 4
	logContent := []byte("================================================================================\n INSTANT SERVER USB STORAGE & LIVE INSTALLATION LOG\n All boot, Subiquity, Curtin, Netplan, and setup logs are saved here automatically.\n Detailed debug logs are also saved inside the logs/ folder on this USB drive.\n================================================================================\n")
	appendRootEntry("INSTANT-SERVER-INSTALL.log", 2, 0x20, 4, uint32(len(logContent)))
	copy(fiveClustersBuf[clusterBytes*2:clusterBytes*2+len(logContent)], logContent)

	// 3. File "Boot-USB-Next-Restart-Windows.bat" at Cluster 5
	winBatContent := buildWindowsNextBootLauncherBat()
	appendRootEntry("Boot-USB-Next-Restart-Windows.bat", 3, 0x20, 5, uint32(len(winBatContent)))
	copy(fiveClustersBuf[clusterBytes*3:clusterBytes*3+len(winBatContent)], winBatContent)

	// 4. File "boot-usb-next-restart-linux.sh" at Cluster 6
	linShContent := buildLinuxNextBootLauncherSh()
	appendRootEntry("boot-usb-next-restart-linux.sh", 4, 0x20, 6, uint32(len(linShContent)))
	copy(fiveClustersBuf[clusterBytes*4:clusterBytes*4+len(linShContent)], linShContent)

	return writeAlignedChunks(disk, fiveClustersBuf, int64(dataRegionStartLBA*512))
}