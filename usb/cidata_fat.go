// ============================================================================
// FILE: usb/cidata_fat.go
// PURPOSE: Builds two FAT filesystems in pure Go (zero external OS tools needed):
//  1. `BuildCidataFAT16Image`: Creates the 16 MB `CIDATA` FAT16 partition image
//     containing `user-data`, `meta-data`, `laptop_hardening.sh`, and app compose files.
//  2. `FormatRemainingUSBAsFAT32`: Formats the remaining free space on the USB
//     (or the entire USB when resetting) as a normal `INSTANTUSB` FAT32 drive
//     pre-populated with:
//     - `logs/INSTANT-SERVER-INSTALL.log`
//     - `Boot-USB-Next-Restart-Windows.bat` (one-click Windows UEFI next-boot)
//     - `boot-usb-next-restart-linux.sh`    (one-click Linux UEFI/kexec next-boot)
//
// ============================================================================
// usb/cidata_fat.go
// =============================================================================
// Pure-Go FAT16 ("CIDATA") and FAT32 ("INSTANTUSB") Partition Builders
// -----------------------------------------------------------------------------
// Study Guide:
//  1. BuildCidataFAT16Image(stageDir):
//     Packs cloud-init files (user-data, meta-data, helper scripts) into a
//     16 MB FAT16 partition image labeled "CIDATA". Ubuntu Server's installer
//     automatically mounts "CIDATA" at boot to run unattended autoinstall.
//  2. FormatRemainingUSBAsFAT32(disk, startLBA, endLBA):
//     Formats the remaining empty space on the USB drive as a normal FAT32
//     partition labeled "INSTANTUSB" and pre-creates:
//     - logs/                              (folder for installer debug logs)
//     - INSTANT-SERVER-INSTALL.log         (main live installation log)
//     - Boot-USB-Next-Restart-Windows.bat  (one-click Windows next-boot tool)
//     - boot-usb-next-restart-linux.sh     (one-click Linux next-boot tool)
//
// =============================================================================
package usb

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf16"
)

const (
	fatBytesPerSector    = 512
	fatSectorsPerCluster = 4     // 2 KB per cluster
	fatTotalSectors      = 32880 // ~16.8 MB (> 4085 clusters required for FAT16)
	fatReservedSectors   = 4
	fatNumTables         = 2
	fatSectorsPerFAT     = 32
	fatRootDirEntries    = 512
	fatRootDirSectors    = (fatRootDirEntries * 32) / fatBytesPerSector
)

type fatStagedFile struct {
	Name string
	Data []byte
}

// computeLFNChecksum calculates the VFAT checksum linking Long File Name entries to an 8.3 entry.
func computeLFNChecksum(shortName [11]byte) byte {
	var sum byte
	for i := 0; i < 11; i++ {
		sum = ((sum & 1) << 7) + (sum >> 1) + shortName[i]
	}
	return sum
}

// makeShort83Name generates a unique DOS 8.3 short name (e.g. "USER_D~1   ") for a filename.
func makeShort83Name(name string, index int) [11]byte {
	var out [11]byte
	for i := range out {
		out[i] = ' '
	}
	upper := strings.ToUpper(name)
	base, ext := upper, ""
	if dot := strings.LastIndex(upper, "."); dot > 0 {
		base, ext = upper[:dot], upper[dot+1:]
	}
	keepAlnum := func(s string) string {
		var sb strings.Builder
		for _, r := range s {
			if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
				sb.WriteRune(r)
			}
		}
		return sb.String()
	}
	cleanBase := keepAlnum(base)
	cleanExt := keepAlnum(ext)
	if cleanBase == "" {
		cleanBase = "FILE"
	}
	suffix := fmt.Sprintf("~%d", index)
	if maxBase := 8 - len(suffix); len(cleanBase) > maxBase {
		cleanBase = cleanBase[:maxBase]
	}
	shortBase := cleanBase + suffix
	copy(out[0:8], []byte(shortBase))
	copy(out[8:11], []byte(cleanExt))
	return out
}

// buildLFNEntries builds 32-byte VFAT Long File Name directory entries in reverse sequence order.
func buildLFNEntries(longName string, shortName [11]byte) [][32]byte {
	utf16Chars := append(utf16.Encode([]rune(longName)), 0x0000)
	for len(utf16Chars)%13 != 0 {
		utf16Chars = append(utf16Chars, 0xFFFF)
	}
	numEntries := len(utf16Chars) / 13
	chk := computeLFNChecksum(shortName)
	offsets := [13]int{1, 3, 5, 7, 9, 14, 16, 18, 20, 22, 24, 28, 30}

	var entries [][32]byte
	for seq := numEntries; seq >= 1; seq-- {
		var ent [32]byte
		ent[0] = byte(seq)
		if seq == numEntries {
			ent[0] |= 0x40
		}
		ent[11] = 0x0F // ATTR_LONG_NAME
		ent[13] = chk
		chunk := utf16Chars[(seq-1)*13 : seq*13]
		for i, codeUnit := range chunk {
			binary.LittleEndian.PutUint16(ent[offsets[i]:offsets[i]+2], codeUnit)
		}
		entries = append(entries, ent)
	}
	return entries
}

// BuildCidataFAT16Image creates a FAT16 partition image labeled "CIDATA" from stageDir.
func BuildCidataFAT16Image(stageDir string) ([]byte, error) {
	entries, err := os.ReadDir(stageDir)
	if err != nil {
		return nil, err
	}

	var files []fatStagedFile
	for _, ent := range entries {
		if ent.IsDir() || strings.HasSuffix(ent.Name(), ".img") || strings.HasSuffix(ent.Name(), ".iso") {
			continue
		}
		raw, rErr := os.ReadFile(filepath.Join(stageDir, ent.Name()))
		if rErr != nil {
			return nil, rErr
		}
		files = append(files, fatStagedFile{Name: ent.Name(), Data: raw})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })

	bytesPerCluster := fatBytesPerSector * fatSectorsPerCluster
	neededClusters := 2
	for _, f := range files {
		c := (len(f.Data) + bytesPerCluster - 1) / bytesPerCluster
		if c == 0 {
			c = 1
		}
		neededClusters += c
	}

	totalSectors := fatTotalSectors
	sectorsPerFAT := fatSectorsPerFAT
	dataStartSector := fatReservedSectors + (fatNumTables * sectorsPerFAT) + fatRootDirSectors
	if avail := (totalSectors - dataStartSector) / fatSectorsPerCluster; neededClusters+64 > avail {
		targetClusters := neededClusters + 128
		if targetClusters < 4100 {
			targetClusters = 4100
		}
		sectorsPerFAT = ((targetClusters * 2) + fatBytesPerSector - 1) / fatBytesPerSector
		if sectorsPerFAT < fatSectorsPerFAT {
			sectorsPerFAT = fatSectorsPerFAT
		}
		dataStartSector = fatReservedSectors + (fatNumTables * sectorsPerFAT) + fatRootDirSectors
		totalSectors = ((dataStartSector + targetClusters*fatSectorsPerCluster + 7) / 8) * 8
	}

	img := make([]byte, totalSectors*fatBytesPerSector)

	// Step 1: Write FAT16 Boot Sector (BIOS Parameter Block)
	img[0], img[1], img[2] = 0xEB, 0x3C, 0x90
	copy(img[3:11], []byte("MSDOS5.0"))
	binary.LittleEndian.PutUint16(img[11:13], fatBytesPerSector)
	img[13] = fatSectorsPerCluster
	binary.LittleEndian.PutUint16(img[14:16], fatReservedSectors)
	img[16] = fatNumTables
	binary.LittleEndian.PutUint16(img[17:19], fatRootDirEntries)
	if totalSectors <= 65535 {
		binary.LittleEndian.PutUint16(img[19:21], uint16(totalSectors))
	} else {
		binary.LittleEndian.PutUint32(img[32:36], uint32(totalSectors))
	}
	img[21] = 0xF8
	binary.LittleEndian.PutUint16(img[22:24], uint16(sectorsPerFAT))
	binary.LittleEndian.PutUint16(img[24:26], 63)
	binary.LittleEndian.PutUint16(img[26:28], 255)
	img[36], img[38] = 0x80, 0x29
	binary.LittleEndian.PutUint32(img[39:43], 0x20261108)
	copy(img[43:54], []byte("CIDATA     "))
	copy(img[54:62], []byte("FAT16   "))
	img[510], img[511] = 0x55, 0xAA

	// Step 2: Initialize FAT #1 and FAT #2 tables
	fat1Offset := fatReservedSectors * fatBytesPerSector
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

	// Step 3: Write Root Directory volume label and file entries
	rootDirOffset := (fatReservedSectors + fatNumTables*sectorsPerFAT) * fatBytesPerSector
	rootDirLimit := rootDirOffset + (fatRootDirSectors * fatBytesPerSector)
	dirCursor := rootDirOffset
	copy(img[dirCursor:dirCursor+11], []byte("CIDATA     "))
	img[dirCursor+11] = 0x08
	dirCursor += 32

	nextCluster := uint16(2)
	for idx, f := range files {
		shortName := makeShort83Name(f.Name, idx+1)
		for _, lfn := range buildLFNEntries(f.Name, shortName) {
			if dirCursor+32 <= rootDirLimit {
				copy(img[dirCursor:dirCursor+32], lfn[:])
				dirCursor += 32
			}
		}

		startCluster := uint16(0)
		if len(f.Data) > 0 {
			startCluster = nextCluster
			numClusters := (len(f.Data) + bytesPerCluster - 1) / bytesPerCluster
			for c := 0; c < numClusters; c++ {
				curCluster := nextCluster
				nextCluster++
				if c == numClusters-1 {
					writeFATEntry(curCluster, 0xFFFF)
				} else {
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
				}
			}
		}

		if dirCursor+32 <= rootDirLimit {
			var stdEnt [32]byte
			copy(stdEnt[0:11], shortName[:])
			stdEnt[11] = 0x20
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

// BuildFAT16CidataVolume is an alias for BuildCidataFAT16Image.
func BuildFAT16CidataVolume(stageDir string, _ string) ([]byte, error) {
	return BuildCidataFAT16Image(stageDir)
}

// Windows one-click script placed on INSTANTUSB to boot into the USB on next restart without entering BIOS.
const windowsNextBootBatTemplate = "@echo off\r\n" +
	"title Instant Server - Boot This USB on Next Restart\r\n" +
	"net session >nul 2>&1\r\n" +
	"if %errorLevel% neq 0 (\r\n" +
	"    powershell -NoProfile -Command \"Start-Process -FilePath '%~f0' -Verb RunAs\"\r\n" +
	"    exit /b\r\n" +
	")\r\n" +
	"echo Configuring Windows UEFI firmware to boot Instant Server USB on next restart...\r\n" +
	"powershell -NoProfile -ExecutionPolicy Bypass -Command \"$out = bcdedit /enum firmware 2>$null | Out-String; $blocks = $out -split '(?=identifier\\s+\\{)'; $usbGuid = ''; foreach ($b in $blocks) { if ($b -match 'identifier\\s+(\\{[0-9a-fA-F-]+\\})' -and $b -match '(?i)(USB|Ubuntu|Removable|Flash|Instant)') { $usbGuid = $Matches[1]; break } }; if ($usbGuid) { bcdedit /set '{fwbootmgr}' bootsequence $usbGuid | Out-Null; shutdown /r /t 3 /c 'Instant Server: Rebooting directly into USB installer' } else { shutdown /r /fw /t 1 }\"\r\n"

// Linux one-click script placed on INSTANTUSB to boot into the USB on next restart via efibootmgr or kexec.
const linuxNextBootShTemplate = "#!/usr/bin/env bash\n" +
	"set -uo pipefail\n" +
	"if [ \"$(id -u)\" -ne 0 ]; then\n" +
	"    exec sudo bash \"$0\" \"$@\"\n" +
	"fi\n" +
	"if [ -d /sys/firmware/efi ] && command -v efibootmgr >/dev/null 2>&1; then\n" +
	"    USB_BOOTNUM=$(efibootmgr 2>/dev/null | grep -iE 'USB|Ubuntu|Linpus|Removable|UEFI:' | head -n1 | sed -n 's/^Boot\\([0-9A-Fa-f]\\{4\\}\\).*/\\1/p')\n" +
	"    if [ -n \"$USB_BOOTNUM\" ]; then\n" +
	"        efibootmgr -n \"$USB_BOOTNUM\" && systemctl reboot\n" +
	"        exit 0\n" +
	"    fi\n" +
	"fi\n" +
	"ISO_PART=$(blkid -t TYPE=iso9660 -o device 2>/dev/null | head -n1 || true)\n" +
	"if [ -n \"$ISO_PART\" ] && command -v kexec >/dev/null 2>&1; then\n" +
	"    mkdir -p /mnt/instant_usb_iso && mount -o ro \"$ISO_PART\" /mnt/instant_usb_iso 2>/dev/null || true\n" +
	"    if [ -f /mnt/instant_usb_iso/casper/vmlinuz ] && [ -f /mnt/instant_usb_iso/casper/initrd ]; then\n" +
	"        kexec -l /mnt/instant_usb_iso/casper/vmlinuz --initrd=/mnt/instant_usb_iso/casper/initrd --command-line=\"BOOT_IMAGE=/casper/vmlinuz autoinstall noprompt ---\" && systemctl kexec\n" +
	"        exit 0\n" +
	"    fi\n" +
	"fi\n" +
	"systemctl reboot --firmware-setup 2>/dev/null || reboot\n"

// FormatRemainingUSBAsFAT32 formats sectors [startLBA .. endLBA] on the USB drive
// as a FAT32 partition labeled "INSTANTUSB" containing logs/ and one-click next-boot launchers.
func FormatRemainingUSBAsFAT32(disk RawDiskHandle, startLBA uint64, endLBA uint64) error {
	if endLBA <= startLBA+4096 {
		return nil
	}
	totalSectors64 := endLBA - startLBA + 1
	if totalSectors64 > 0xFFFFFFFF {
		totalSectors64 = 0xFFFFFFFF
	}
	totalSectors := uint32(totalSectors64)

	var secPerClus uint32 = 64
	if totalSectors > 134217728 {
		secPerClus = 128
	} else if totalSectors < 131072 {
		secPerClus = 8
	} else if totalSectors < 16777216 {
		secPerClus = 16
	}

	const reservedSectors uint32 = 32
	const numFATs uint32 = 2
	approxClusters := (totalSectors - reservedSectors) / secPerClus
	fatSectors := (((approxClusters+2)*4+511)/512 + 7) / 8 * 8
	if fatSectors < 128 {
		fatSectors = 128
	}

	resBuf := AllocPageAlignedBuffer(int(reservedSectors*512), SectorAlignmentBytes)
	writeBootSector := func(b []byte) {
		b[0], b[1], b[2] = 0xEB, 0x58, 0x90
		copy(b[3:11], []byte("MSDOS5.0"))
		binary.LittleEndian.PutUint16(b[11:13], 512)
		b[13] = byte(secPerClus)
		binary.LittleEndian.PutUint16(b[14:16], uint16(reservedSectors))
		b[16] = byte(numFATs)
		b[21] = 0xF8
		binary.LittleEndian.PutUint16(b[24:26], 63)
		binary.LittleEndian.PutUint16(b[26:28], 255)
		binary.LittleEndian.PutUint32(b[28:32], uint32(startLBA))
		binary.LittleEndian.PutUint32(b[32:36], totalSectors)
		binary.LittleEndian.PutUint32(b[36:40], fatSectors)
		binary.LittleEndian.PutUint32(b[44:48], 2)
		binary.LittleEndian.PutUint16(b[48:50], 1)
		binary.LittleEndian.PutUint16(b[50:52], 6)
		b[64], b[66] = 0x80, 0x29
		binary.LittleEndian.PutUint32(b[67:71], 0x20261112)
		copy(b[71:82], []byte("INSTANTUSB "))
		copy(b[82:90], []byte("FAT32   "))
		b[510], b[511] = 0x55, 0xAA
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
		binary.LittleEndian.PutUint32(fsi[492:496], 7)
		fsi[510], fsi[511] = 0x55, 0xAA
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
	dataRegionStartLBA := fat2StartLBA + uint64(fatSectors)
	fiveClustersBuf := AllocPageAlignedBuffer(clusterBytes*5, SectorAlignmentBytes)

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
		binary.LittleEndian.PutUint16(stdEnt[26:28], cluster)
		binary.LittleEndian.PutUint32(stdEnt[28:32], size)
		copy(fiveClustersBuf[cursor:cursor+32], stdEnt[:])
		cursor += 32
	}

	// Cluster 3: logs/ folder
	appendRootEntry("logs", 1, 0x10, 3, 0)
	logsOff := clusterBytes
	copy(fiveClustersBuf[logsOff:logsOff+11], []byte(".          "))
	fiveClustersBuf[logsOff+11] = 0x10
	binary.LittleEndian.PutUint16(fiveClustersBuf[logsOff+26:logsOff+28], 3)
	copy(fiveClustersBuf[logsOff+32:logsOff+43], []byte("..         "))
	fiveClustersBuf[logsOff+43] = 0x10

	// Cluster 4: INSTANT-SERVER-INSTALL.log
	logContent := []byte("================================================================================\n INSTANT SERVER USB STORAGE & LIVE INSTALLATION LOG\n All boot, Subiquity, Curtin, Netplan, and setup logs are saved here automatically.\n================================================================================\n")
	appendRootEntry("INSTANT-SERVER-INSTALL.log", 2, 0x20, 4, uint32(len(logContent)))
	copy(fiveClustersBuf[clusterBytes*2:clusterBytes*2+len(logContent)], logContent)

	// Cluster 5: Boot-USB-Next-Restart-Windows.bat
	winBat := []byte(windowsNextBootBatTemplate)
	appendRootEntry("Boot-USB-Next-Restart-Windows.bat", 3, 0x20, 5, uint32(len(winBat)))
	copy(fiveClustersBuf[clusterBytes*3:clusterBytes*3+len(winBat)], winBat)

	// Cluster 6: boot-usb-next-restart-linux.sh
	linSh := []byte(linuxNextBootShTemplate)
	appendRootEntry("boot-usb-next-restart-linux.sh", 4, 0x20, 6, uint32(len(linSh)))
	copy(fiveClustersBuf[clusterBytes*4:clusterBytes*4+len(linSh)], linSh)

	return writeAlignedChunks(disk, fiveClustersBuf, int64(dataRegionStartLBA*512))
}
