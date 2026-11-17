// ============================================================================
// FILE: usb/cidata_fat.go
// PURPOSE: Pure-Go FAT16 ("CIDATA") and FAT32 ("UBUNTU-ISO" & "INSTANTUSB")
//
//	filesystem builders and ISO9660-to-FAT32 extractor.
//
// WHY THIS FILE EXISTS (For Beginners & Contributors):
//  1. Windows File Explorer cannot open or edit raw Linux ISO9660 partitions
//     on a USB stick, and Windows flags any FAT partition missing "." / ".."
//     directory entries or accurate FSInfo free-cluster counts with a
//     "Do you want to scan and fix USB?" popup.
//  2. This file solves both problems in pure Go:
//     - `ExtractISO9660ToFAT32Partition`: Extracts the Ubuntu Server ISO
//     into a clean, bootable, editable FAT32 partition ("UBUNTU-ISO") so
//     every ISO file is visible and editable right in Windows Explorer.
//     - `BuildCidataFAT16Image`: Creates a 100% spec-compliant FAT16 "CIDATA"
//     partition with valid "." and ".." entries and clean shutdown bits.
//     - `FormatRemainingUSBAsFAT32`: Formats the remaining USB space as
//     "INSTANTUSB" (FAT32) with exact FSInfo cluster counts, backup boot
//     sectors, `logs/`, and one-click next-boot launcher scripts.
//
// ============================================================================
package usb

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf16"
)

const (
	fatBytesPerSector    = 512
	fatSectorsPerCluster = 4     // 2 KB per cluster for FAT16 CIDATA
	fatTotalSectors      = 32880 // ~16.8 MB (> 4085 clusters required for FAT16)
	fatReservedSectors   = 4
	fatNumTables         = 2
	fatSectorsPerFAT     = 32
	fatRootDirEntries    = 512
	fatRootDirSectors    = (fatRootDirEntries * 32) / fatBytesPerSector

	// Standard DOS timestamp (2026-11-16 12:00:00) so Windows fastfat.sys
	// never complains about zeroed directory entry dates.
	dosDefaultDate = uint16(((2026 - 1980) << 9) | (11 << 5) | 16)
	dosDefaultTime = uint16((12 << 11) | (0 << 5) | 0)
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
	upper := strings.ToUpper(strings.Trim(name, ". "))
	base, ext := upper, ""
	if dot := strings.LastIndex(upper, "."); dot > 0 {
		base, ext = upper[:dot], upper[dot+1:]
	}
	keepAlnum := func(s string) string {
		var sb strings.Builder
		for _, r := range s {
			if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
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
	if len(cleanExt) > 3 {
		cleanExt = cleanExt[:3]
	}
	suffix := fmt.Sprintf("~%d", index)
	if index > 999 {
		suffix = fmt.Sprintf("~%03X", index&0xFFF)
	}
	if maxBase := 8 - len(suffix); maxBase > 0 {
		if len(cleanBase) > maxBase {
			cleanBase = cleanBase[:maxBase]
		}
	} else {
		cleanBase = cleanBase[:1]
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

// stampDOSDirEntry writes valid creation/write timestamps and cluster pointers on a 32-byte FAT entry.
func stampDOSDirEntry(ent *[32]byte, attr byte, cluster uint32, size uint32) {
	ent[11] = attr
	binary.LittleEndian.PutUint16(ent[14:16], dosDefaultTime)
	binary.LittleEndian.PutUint16(ent[16:18], dosDefaultDate)
	binary.LittleEndian.PutUint16(ent[18:20], dosDefaultDate)
	binary.LittleEndian.PutUint16(ent[20:22], uint16((cluster>>16)&0xFFFF))
	binary.LittleEndian.PutUint16(ent[22:24], dosDefaultTime)
	binary.LittleEndian.PutUint16(ent[24:26], dosDefaultDate)
	binary.LittleEndian.PutUint16(ent[26:28], uint16(cluster&0xFFFF))
	binary.LittleEndian.PutUint32(ent[28:32], size)
}

// makeDotEntry creates a 32-byte "." or ".." directory entry required by Windows fastfat.sys.
func makeDotEntry(name11 string, cluster uint32) [32]byte {
	var ent [32]byte
	copy(ent[0:11], []byte(name11))
	stampDOSDirEntry(&ent, 0x10, cluster, 0)
	return ent
}

// ============================================================================
// SECTION 1: PURE-GO FAT16 BUILDER FOR "CIDATA" PARTITION
// ============================================================================

// BuildCidataFAT16Image creates a clean FAT16 partition image labeled "CIDATA" from stageDir.
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

	// Step 1: Write FAT16 Boot Sector (BIOS Parameter Block) with Clean Shutdown Bit (img[37] = 0x00)
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
	img[36] = 0x80
	img[37] = 0x00 // BS_Reserved1 = 0x00 (Volume is 100% clean — prevents Windows chkdsk prompt)
	img[38] = 0x29
	binary.LittleEndian.PutUint32(img[39:43], 0x20261108)
	copy(img[43:54], []byte("CIDATA     "))
	copy(img[54:62], []byte("FAT16   "))
	img[510], img[511] = 0x55, 0xAA

	// Step 2: Initialize FAT #1 and FAT #2 tables with clean bit set on FAT[1] (0xFFFF)
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
	var volEnt [32]byte
	copy(volEnt[0:11], []byte("CIDATA     "))
	stampDOSDirEntry(&volEnt, 0x08, 0, 0)
	copy(img[dirCursor:dirCursor+32], volEnt[:])
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
			stampDOSDirEntry(&stdEnt, 0x20, uint32(startCluster), uint32(len(f.Data)))
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

// ============================================================================
// SECTION 2: ONE-CLICK NEXT-BOOT LAUNCHERS & "INSTANTUSB" FAT32 FORMATTER
// ============================================================================

// Windows one-click script placed on INSTANTUSB to boot into the USB on next restart with confirmation.
const windowsNextBootBatTemplate = "@echo off\r\n" +
	"setlocal EnableDelayedExpansion\r\n" +
	"title Instant Server - Boot This USB on Next Restart\r\n" +
	"net session >nul 2>&1\r\n" +
	"if %errorLevel% neq 0 (\r\n" +
	"    powershell -NoProfile -Command \"Start-Process -FilePath '%~f0' -Verb RunAs\"\r\n" +
	"    exit /b\r\n" +
	")\r\n" +
	"echo ============================================================================\r\n" +
	"echo  INSTANT SERVER - ONE-CLICK NEXT-BOOT USB LAUNCHER\r\n" +
	"echo ============================================================================\r\n" +
	"echo This will configure Windows UEFI firmware to boot from your Instant Server\r\n" +
	"echo USB drive on the next restart without pressing BIOS/Boot Menu keys.\r\n" +
	"echo.\r\n" +
	"if /i not \"%~1\"==\"-noconfirm\" (\r\n" +
	"    set /p CONFIRM=\"Restart now into Instant Server USB? (Y = Restart Now / S = Stage For Later / N = Cancel): \"\r\n" +
	"    if /i \"!CONFIRM!\"==\"N\" exit /b 0\r\n" +
	")\r\n" +
	"powershell -NoProfile -ExecutionPolicy Bypass -Command \"$out = bcdedit /enum firmware 2>$null | Out-String; $blocks = $out -split '(?=identifier\\s+\\{)'; $usbGuid = ''; foreach ($b in $blocks) { if ($b -match 'identifier\\s+(\\{[0-9a-fA-F-]+\\})' -and $b -match '(?i)(USB|Ubuntu|Removable|Flash|Instant|Linpus|UEFI:)') { $usbGuid = $Matches[1]; break } }; if (-not $usbGuid) { $efiDrv = Get-PSDrive -PSProvider FileSystem | Where-Object { Test-Path ($_.Root + 'EFI\\BOOT\\bootx64.efi') } | Select-Object -First 1; if ($efiDrv) { $part = $efiDrv.Name + ':'; $cp = bcdedit /copy '{bootmgr}' /d 'Instant Server USB' 2>$null | Out-String; if ($cp -match '(\\{[0-9a-fA-F-]+\\})') { $usbGuid = $Matches[1]; bcdedit /set $usbGuid device partition=$part | Out-Null; bcdedit /set $usbGuid path '\\EFI\\BOOT\\bootx64.efi' | Out-Null } } }; if ($usbGuid) { bcdedit /set '{fwbootmgr}' bootsequence $usbGuid | Out-Null; Write-Host ('Queued UEFI next-boot target: ' + $usbGuid) }\"\r\n" +
	"if /i \"!CONFIRM!\"==\"S\" (\r\n" +
	"    echo Next-boot USB target staged. Your PC will boot the USB the next time you restart.\r\n" +
	"    pause\r\n" +
	"    exit /b 0\r\n" +
	")\r\n" +
	"shutdown /r /t 3 /c \"Instant Server: Rebooting into USB installer\" || shutdown /r /fw /t 1\r\n"

// Linux one-click script placed on INSTANTUSB to boot into the USB on next restart via efibootmgr or kexec.
const linuxNextBootShTemplate = "#!/usr/bin/env bash\n" +
	"set -uo pipefail\n" +
	"if [ \"$(id -u)\" -ne 0 ]; then\n" +
	"    exec sudo bash \"$0\" \"$@\"\n" +
	"fi\n" +
	"if [ \"${1:-}\" != \"-y\" ]; then\n" +
	"    printf 'Configure next boot to Instant Server USB and restart now? [y/N]: '\n" +
	"    read -r ans\n" +
	"    [[ ! \"$ans\" =~ ^[Yy]$ ]] && exit 0\n" +
	"fi\n" +
	"if [ -d /sys/firmware/efi ] && command -v efibootmgr >/dev/null 2>&1; then\n" +
	"    USB_BOOTNUM=$(efibootmgr 2>/dev/null | grep -iE 'USB|Ubuntu|Linpus|Removable|UEFI:' | head -n1 | sed -n 's/^Boot\\([0-9A-Fa-f]\\{4\\}\\).*/\\1/p')\n" +
	"    if [ -n \"$USB_BOOTNUM\" ]; then\n" +
	"        efibootmgr -n \"$USB_BOOTNUM\" && systemctl reboot\n" +
	"        exit 0\n" +
	"    fi\n" +
	"fi\n" +
	"ISO_PART=$(blkid -L UBUNTU-ISO 2>/dev/null || blkid -t TYPE=iso9660 -o device 2>/dev/null | head -n1 || true)\n" +
	"if [ -n \"$ISO_PART\" ] && command -v kexec >/dev/null 2>&1; then\n" +
	"    mkdir -p /mnt/instant_usb_iso && mount -o ro \"$ISO_PART\" /mnt/instant_usb_iso 2>/dev/null || true\n" +
	"    if [ -f /mnt/instant_usb_iso/casper/vmlinuz ] && [ -f /mnt/instant_usb_iso/casper/initrd ]; then\n" +
	"        kexec -l /mnt/instant_usb_iso/casper/vmlinuz --initrd=/mnt/instant_usb_iso/casper/initrd --command-line=\"BOOT_IMAGE=/casper/vmlinuz autoinstall noprompt ---\" && systemctl kexec\n" +
	"        exit 0\n" +
	"    fi\n" +
	"fi\n" +
	"systemctl reboot --firmware-setup 2>/dev/null || reboot\n"

// FormatRemainingUSBAsFAT32 formats sectors [startLBA .. endLBA] on the USB drive
// as a 100% clean FAT32 partition labeled "INSTANTUSB" containing logs/ and next-boot launchers.
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
		secPerClus = 32
	}

	const reservedSectors uint32 = 32
	const numFATs uint32 = 2
	approxClusters := (totalSectors - reservedSectors) / secPerClus
	fatSectors := (((approxClusters+2)*4+511)/512 + 7) / 8 * 8
	if fatSectors < 128 {
		fatSectors = 128
	}
	dataSectors := totalSectors - reservedSectors - (numFATs * fatSectors)
	totalDataClusters := dataSectors / secPerClus
	const allocatedClusters uint32 = 5 // Clusters 2, 3, 4, 5, 6

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
		binary.LittleEndian.PutUint32(b[44:48], 2) // Root cluster = 2
		binary.LittleEndian.PutUint16(b[48:50], 1) // FSInfo sector = 1
		binary.LittleEndian.PutUint16(b[50:52], 6) // Backup boot sector = 6
		b[64] = 0x80
		b[65] = 0x00 // BS_Reserved1 = 0x00 (Volume is 100% clean)
		b[66] = 0x29
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
		if totalDataClusters > allocatedClusters {
			freeClus = totalDataClusters - allocatedClusters
		}
		binary.LittleEndian.PutUint32(fsi[488:492], freeClus)
		binary.LittleEndian.PutUint32(fsi[492:496], 2+allocatedClusters) // Next free cluster = 7
		fsi[510], fsi[511] = 0x55, 0xAA
	}
	writeFSInfo(resBuf[1*512 : 2*512])
	writeFSInfo(resBuf[7*512 : 8*512])

	if _, err := disk.WriteAt(resBuf, int64(startLBA*512)); err != nil {
		return err
	}

	// Zero the ENTIRE FAT1 and FAT2 tables on disk so no stale non-zero clusters trigger Windows chkdsk
	totalFATBytes := int(fatSectors * 512)
	fatFirstChunk := totalFATBytes
	if fatFirstChunk > 1024*1024 {
		fatFirstChunk = 1024 * 1024
	}
	fatBuf := AllocPageAlignedBuffer(fatFirstChunk, SectorAlignmentBytes)
	binary.LittleEndian.PutUint32(fatBuf[0:4], 0x0FFFFFF8)
	binary.LittleEndian.PutUint32(fatBuf[4:8], 0x0FFFFFFF) // Bit 27 = 1 (Clean shutdown)
	for clus := 2; clus <= 6; clus++ {
		binary.LittleEndian.PutUint32(fatBuf[clus*4:(clus+1)*4], 0x0FFFFFFF)
	}

	fat1StartLBA := startLBA + uint64(reservedSectors)
	fat2StartLBA := fat1StartLBA + uint64(fatSectors)
	for _, fatBaseLBA := range []uint64{fat1StartLBA, fat2StartLBA} {
		if err := writeAlignedChunks(disk, fatBuf, int64(fatBaseLBA*512)); err != nil {
			return err
		}
		if totalFATBytes > fatFirstChunk {
			zeroBuf := AllocPageAlignedBuffer(1024*1024, SectorAlignmentBytes)
			written := fatFirstChunk
			for written < totalFATBytes {
				step := totalFATBytes - written
				if step > len(zeroBuf) {
					step = len(zeroBuf)
				}
				if _, err := disk.WriteAt(zeroBuf[:step], int64(fatBaseLBA*512)+int64(written)); err != nil {
					return err
				}
				written += step
			}
		}
	}

	clusterBytes := int(secPerClus * 512)
	if clusterBytes < 4096 {
		clusterBytes = 4096
	}
	dataRegionStartLBA := fat2StartLBA + uint64(fatSectors)
	fiveClustersBuf := AllocPageAlignedBuffer(clusterBytes*5, SectorAlignmentBytes)

	var volEnt [32]byte
	copy(volEnt[0:11], []byte("INSTANTUSB "))
	stampDOSDirEntry(&volEnt, 0x08, 0, 0)
	copy(fiveClustersBuf[0:32], volEnt[:])
	cursor := 32

	appendRootEntry := func(longName string, idx int, attr byte, cluster uint32, size uint32) {
		shortName := makeShort83Name(longName, idx)
		for _, lfn := range buildLFNEntries(longName, shortName) {
			copy(fiveClustersBuf[cursor:cursor+32], lfn[:])
			cursor += 32
		}
		var stdEnt [32]byte
		copy(stdEnt[0:11], shortName[:])
		stampDOSDirEntry(&stdEnt, attr, cluster, size)
		copy(fiveClustersBuf[cursor:cursor+32], stdEnt[:])
		cursor += 32
	}

	// Cluster 3: logs/ folder (with mandatory "." and ".." entries)
	appendRootEntry("logs", 1, 0x10, 3, 0)
	logsOff := clusterBytes
	dotSelf := makeDotEntry(".          ", 3)
	dotParent := makeDotEntry("..         ", 0)
	copy(fiveClustersBuf[logsOff:logsOff+32], dotSelf[:])
	copy(fiveClustersBuf[logsOff+32:logsOff+64], dotParent[:])

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

// ============================================================================
// SECTION 3: ISO9660 TO BOOTABLE & EDITABLE FAT32 PARTITION EXTRACTOR
// ============================================================================
// Extracting the Ubuntu Server ISO into a standard FAT32 partition ("UBUNTU-ISO")
// makes the ISO partition visible, openable, and editable in Windows Explorer
// while booting natively via UEFI (\EFI\BOOT\BOOTx64.EFI -> grubx64.efi).

type isoFileExtent struct {
	Offset int64
	Length int64
}

type isoTreeNode struct {
	Name       string
	RelPath    string
	IsDir      bool
	Extents    []isoFileExtent
	TotalSize  int64
	Override   []byte
	Children   []*isoTreeNode
	Cluster    uint32
	NumCluster uint32
}

// HasExtractableISO9660Tree checks whether srcFile contains a standard ISO9660 Primary Volume Descriptor
// at sector 16 (byte offset 32768) with an extractable root directory tree.
func HasExtractableISO9660Tree(srcFile *os.File, totalBytes int64) bool {
	if totalBytes < 65536 {
		return false
	}
	var pvd [2048]byte
	if _, err := srcFile.ReadAt(pvd[:], 32768); err != nil {
		return false
	}
	if pvd[0] != 1 || string(pvd[1:6]) != "CD001" {
		return false
	}
	rootLBA := binary.LittleEndian.Uint32(pvd[156+2 : 156+6])
	rootLen := binary.LittleEndian.Uint32(pvd[156+10 : 156+14])
	return rootLBA > 0 && rootLen >= 34 && int64(rootLBA)*2048+int64(rootLen) <= totalBytes
}

// parseISO9660Directory recursively parses an ISO9660 + RockRidge directory tree from srcFile.
func parseISO9660Directory(srcFile *os.File, totalBytes int64, dirLBA uint32, dirLen uint32, parentRel string, depth int, visited map[uint32]bool) ([]*isoTreeNode, error) {
	if depth > 16 || dirLBA == 0 || dirLen == 0 || visited[dirLBA] {
		return nil, nil
	}
	visited[dirLBA] = true
	offset := int64(dirLBA) * 2048
	if offset+int64(dirLen) > totalBytes {
		return nil, nil
	}

	buf := make([]byte, dirLen)
	if _, err := srcFile.ReadAt(buf, offset); err != nil {
		return nil, err
	}

	var nodes []*isoTreeNode
	var pendingMulti *isoTreeNode
	pos := 0
	for pos < len(buf) {
		recLen := int(buf[pos])
		if recLen == 0 {
			// Advance to next 2048-byte ISO sector boundary
			pos = ((pos / 2048) + 1) * 2048
			continue
		}
		if pos+recLen > len(buf) || recLen < 34 {
			break
		}
		rec := buf[pos : pos+recLen]
		pos += recLen

		extentLBA := binary.LittleEndian.Uint32(rec[2:6])
		dataLen := binary.LittleEndian.Uint32(rec[10:14])
		flags := rec[25]
		nameLen := int(rec[32])
		if 33+nameLen > len(rec) {
			continue
		}
		rawNameBytes := rec[33 : 33+nameLen]
		if nameLen == 1 && (rawNameBytes[0] == 0x00 || rawNameBytes[0] == 0x01) {
			continue // Skip "." and ".."
		}

		// Parse RockRidge System Use Area (SUA) for long filenames (NM), symlinks (SL), and child links (CL)
		suaPos := 33 + nameLen
		if nameLen%2 == 0 {
			suaPos++
		}
		var rrName strings.Builder
		isSymlink := false
		isRelocated := false
		var childLinkLBA uint32
		for suaPos+4 <= len(rec) {
			s0, s1 := rec[suaPos], rec[suaPos+1]
			eLen := int(rec[suaPos+2])
			if eLen < 4 || suaPos+eLen > len(rec) {
				break
			}
			ent := rec[suaPos : suaPos+eLen]
			suaPos += eLen
			if s0 == 'N' && s1 == 'M' && eLen >= 5 {
				nmFlags := ent[4]
				if (nmFlags & 0x06) == 0 {
					rrName.Write(ent[5:])
				}
			} else if s0 == 'S' && s1 == 'L' {
				isSymlink = true
			} else if s0 == 'R' && s1 == 'E' {
				isRelocated = true
			} else if s0 == 'C' && s1 == 'L' && eLen >= 12 {
				childLinkLBA = binary.LittleEndian.Uint32(ent[4:8])
			}
		}

		if isSymlink || isRelocated {
			continue
		}

		name := strings.TrimSpace(rrName.String())
		if name == "" {
			name = string(rawNameBytes)
			if semi := strings.IndexByte(name, ';'); semi >= 0 {
				name = name[:semi]
			}
			name = strings.TrimSuffix(name, ".")
		}
		if name == "" || name == "." || name == ".." || name == ".rr_moved" {
			continue
		}

		isDir := (flags & 0x02) != 0
		if childLinkLBA > 0 {
			isDir = true
			extentLBA = childLinkLBA
			var dotHdr [34]byte
			if _, err := srcFile.ReadAt(dotHdr[:], int64(childLinkLBA)*2048); err == nil && dotHdr[0] >= 34 {
				dataLen = binary.LittleEndian.Uint32(dotHdr[10:14])
			}
		}

		relPath := name
		if parentRel != "" {
			relPath = parentRel + "/" + name
		}

		if !isDir && (flags&0x80) != 0 {
			if pendingMulti == nil {
				pendingMulti = &isoTreeNode{Name: name, RelPath: relPath, IsDir: false}
			}
			pendingMulti.Extents = append(pendingMulti.Extents, isoFileExtent{
				Offset: int64(extentLBA) * 2048,
				Length: int64(dataLen),
			})
			pendingMulti.TotalSize += int64(dataLen)
			continue
		}

		if pendingMulti != nil {
			pendingMulti.Extents = append(pendingMulti.Extents, isoFileExtent{
				Offset: int64(extentLBA) * 2048,
				Length: int64(dataLen),
			})
			pendingMulti.TotalSize += int64(dataLen)
			nodes = append(nodes, pendingMulti)
			pendingMulti = nil
			continue
		}

		node := &isoTreeNode{
			Name:      name,
			RelPath:   relPath,
			IsDir:     isDir,
			TotalSize: int64(dataLen),
		}
		if !isDir && dataLen > 0 {
			node.Extents = []isoFileExtent{{Offset: int64(extentLBA) * 2048, Length: int64(dataLen)}}
		}
		if isDir {
			children, err := parseISO9660Directory(srcFile, totalBytes, extentLBA, dataLen, relPath, depth+1, visited)
			if err != nil {
				return nil, err
			}
			node.Children = children
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// ExtractISO9660ToFAT32Partition extracts the ISO9660 filesystem in srcFile into a clean,
// bootable, Windows-editable FAT32 partition ("UBUNTU-ISO") starting at startLBA.
// Returns the last LBA of the UBUNTU-ISO partition, the number of GRUB patches applied, and any error.
func ExtractISO9660ToFAT32Partition(
	ctx context.Context,
	srcFile *os.File,
	totalISOBytes int64,
	disk RawDiskHandle,
	startLBA uint64,
	patchGrub bool,
	pauseGate *PauseGate,
	crcHasher hash.Hash32,
	shaHasher hash.Hash,
	onProgress func(FlashProgressEvent),
) (uint64, int, error) {
	var pvd [2048]byte
	if _, err := srcFile.ReadAt(pvd[:], 32768); err != nil {
		return 0, 0, err
	}
	rootLBA := binary.LittleEndian.Uint32(pvd[156+2 : 156+6])
	rootLen := binary.LittleEndian.Uint32(pvd[156+10 : 156+14])

	visited := make(map[uint32]bool)
	children, err := parseISO9660Directory(srcFile, totalISOBytes, rootLBA, rootLen, "", 0, visited)
	if err != nil {
		return 0, 0, err
	}
	rootNode := &isoTreeNode{Name: "", RelPath: "", IsDir: true, Children: children}

	// Patch GRUB configuration files in-memory and update md5sum.txt so casper-md5check passes 100%
	grubPatches := 0
	updatedMD5 := make(map[string]string)
	var md5Node *isoTreeNode

	var walkPrepare func(n *isoTreeNode)
	walkPrepare = func(n *isoTreeNode) {
		for _, ch := range n.Children {
			if ch.IsDir {
				walkPrepare(ch)
				continue
			}
			lowRel := strings.ToLower(ch.RelPath)
			if lowRel == "md5sum.txt" && ch.TotalSize > 0 && ch.TotalSize < 2*1024*1024 {
				md5Node = ch
			}
			if patchGrub && strings.HasSuffix(lowRel, ".cfg") && ch.TotalSize > 0 && ch.TotalSize < 512*1024 {
				raw := make([]byte, ch.TotalSize)
				if len(ch.Extents) > 0 {
					_, _ = srcFile.ReadAt(raw, ch.Extents[0].Offset)
				}
				pCount := PatchGrubKernelCmdlineInPlace(raw)
				if !bytes.Contains(raw, []byte("autoinstall")) && bytes.Contains(raw, []byte("/casper/vmlinuz")) {
					raw = bytes.ReplaceAll(raw, []byte("/casper/vmlinuz"), []byte("/casper/vmlinuz autoinstall noprompt"))
					pCount++
				}
				if pCount > 0 {
					grubPatches += pCount
					ch.Override = raw
					ch.TotalSize = int64(len(raw))
					sum := md5.Sum(raw)
					updatedMD5["./"+ch.RelPath] = hex.EncodeToString(sum[:])
				}
			}
		}
	}
	walkPrepare(rootNode)

	if md5Node != nil && len(updatedMD5) > 0 && len(md5Node.Extents) > 0 {
		rawMD5 := make([]byte, md5Node.TotalSize)
		if _, err := srcFile.ReadAt(rawMD5, md5Node.Extents[0].Offset); err == nil {
			lines := strings.Split(string(rawMD5), "\n")
			for i, line := range lines {
				for pathKey, newHex := range updatedMD5 {
					if strings.HasSuffix(strings.TrimSpace(line), pathKey) {
						lines[i] = newHex + "  " + pathKey
					}
				}
			}
			patchedMD5 := []byte(strings.Join(lines, "\n"))
			md5Node.Override = patchedMD5
			md5Node.TotalSize = int64(len(patchedMD5))
		}
	}

	// Choose cluster size (32 KB for real ISOs, 4 KB for small test images so FAT32 has >= 65525 clusters or fits)
	secPerClus := uint32(16) // 8 KB per cluster
	if totalISOBytes > 512*1024*1024 {
		secPerClus = 64 // 32 KB per cluster
	} else if totalISOBytes < 32*1024*1024 {
		secPerClus = 1 // 512 B per cluster for small synthetic ISO tests
	}
	clusterBytes := int(secPerClus * 512)

	// Ubuntu Server ISOs use RockRidge symlinks `/dists/stable` and `/dists/unstable`
	// pointing to the release codename directory (e.g. `/dists/noble` or `/dists/resolute`).
	// Because FAT32 does not support POSIX symlinks, we clone the `/dists/<codename>`
	// subtree into `/dists/stable` and `/dists/unstable` with independent FAT32 clusters
	// so both apt/curtin offline installs and Windows chkdsk validate 100% cleanly.
	for _, topDir := range rootNode.Children {
		if topDir.IsDir && strings.EqualFold(topDir.Name, "dists") && len(topDir.Children) > 0 {
			var baseDist *isoTreeNode
			existingNames := map[string]bool{}
			for _, sd := range topDir.Children {
				existingNames[strings.ToLower(sd.Name)] = true
				if sd.IsDir && baseDist == nil && !strings.EqualFold(sd.Name, "stable") && !strings.EqualFold(sd.Name, "unstable") {
					baseDist = sd
				}
			}
			if baseDist != nil {
				var cloneDistTree func(src *isoTreeNode, newName string, parentRel string) *isoTreeNode
				cloneDistTree = func(src *isoTreeNode, newName string, parentRel string) *isoTreeNode {
					rel := parentRel + "/" + newName
					dst := &isoTreeNode{
						Name:      newName,
						RelPath:   rel,
						IsDir:     src.IsDir,
						TotalSize: src.TotalSize,
						Extents:   append([]isoExtent(nil), src.Extents...),
						Override:  src.Override,
					}
					for _, ch := range src.Children {
						dst.Children = append(dst.Children, cloneDistTree(ch, ch.Name, rel))
					}
					return dst
				}
				for _, alias := range []string{"stable", "unstable"} {
					if !existingNames[alias] {
						topDir.Children = append(topDir.Children, cloneDistTree(baseDist, alias, topDir.RelPath))
					}
				}
			}
		}
	}

	// Assign cluster numbers to all directories and files
	nextCluster := uint32(2)
	var assignClusters func(dir *isoTreeNode)
	assignClusters = func(dir *isoTreeNode) {
		entryCount := 2 // "." and ".." (or volume label)
		for _, ch := range dir.Children {
			entryCount += 1 + ((len([]rune(ch.Name)) + 12) / 13)
		}
		dirBytesNeeded := (entryCount + 2) * 32
		dirClusters := uint32((dirBytesNeeded + clusterBytes - 1) / clusterBytes)
		if dirClusters == 0 {
			dirClusters = 1
		}
		dir.Cluster = nextCluster
		dir.NumCluster = dirClusters
		nextCluster += dirClusters

		for _, ch := range dir.Children {
			if ch.IsDir {
				assignClusters(ch)
			} else if ch.TotalSize > 0 {
				fClus := uint32((ch.TotalSize + int64(clusterBytes) - 1) / int64(clusterBytes))
				if fClus == 0 {
					fClus = 1
				}
				ch.Cluster = nextCluster
				ch.NumCluster = fClus
				nextCluster += fClus
			}
		}
	}
	assignClusters(rootNode)

	allocatedClusters := nextCluster - 2
	// Ensure at least 65536 total clusters so Windows fastfat.sys recognizes the volume strictly as FAT32
	totalDataClusters := allocatedClusters + 2048
	if totalDataClusters < 65600 {
		totalDataClusters = 65600
	}
	// Check physical disk size so small unit-test images still fit
	if diskEndBytes, seekErr := disk.Seek(0, io.SeekEnd); seekErr == nil && diskEndBytes > 0 {
		maxDiskSectors := uint64(diskEndBytes)/512 - startLBA - 40000
		if maxDiskSectors > 4096 && uint64(totalDataClusters*secPerClus) > maxDiskSectors {
			totalDataClusters = uint32(maxDiskSectors/uint64(secPerClus)) - 256
			if totalDataClusters < allocatedClusters+64 {
				totalDataClusters = allocatedClusters + 64
			}
		}
	}

	const reservedSectors uint32 = 32
	const numFATs uint32 = 2
	fatSectors := (((totalDataClusters+2)*4+511)/512 + 7) / 8 * 8
	if fatSectors < 128 {
		fatSectors = 128
	}
	totalPartSectors := reservedSectors + (numFATs * fatSectors) + (totalDataClusters * secPerClus)
	// Align partition size to 2048 sectors (1 MB)
	totalPartSectors = ((totalPartSectors + 2047) / 2048) * 2048
	totalDataClusters = (totalPartSectors - reservedSectors - (numFATs * fatSectors)) / secPerClus

	// 1. Write FAT32 Boot Sector & FSInfo for UBUNTU-ISO
	resBuf := AllocPageAlignedBuffer(int(reservedSectors*512), SectorAlignmentBytes)
	writeBoot := func(b []byte) {
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
		binary.LittleEndian.PutUint32(b[32:36], totalPartSectors)
		binary.LittleEndian.PutUint32(b[36:40], fatSectors)
		binary.LittleEndian.PutUint32(b[44:48], 2)
		binary.LittleEndian.PutUint16(b[48:50], 1)
		binary.LittleEndian.PutUint16(b[50:52], 6)
		b[64] = 0x80
		b[65] = 0x00 // Clean volume bit
		b[66] = 0x29
		binary.LittleEndian.PutUint32(b[67:71], 0x20260401)
		copy(b[71:82], []byte("UBUNTU-ISO "))
		copy(b[82:90], []byte("FAT32   "))
		b[510], b[511] = 0x55, 0xAA
	}
	writeBoot(resBuf[0:512])
	writeBoot(resBuf[6*512 : 7*512])

	writeFSI := func(fsi []byte) {
		binary.LittleEndian.PutUint32(fsi[0:4], 0x41615252)
		binary.LittleEndian.PutUint32(fsi[484:488], 0x61417272)
		freeClus := uint32(0)
		if totalDataClusters > allocatedClusters {
			freeClus = totalDataClusters - allocatedClusters
		}
		binary.LittleEndian.PutUint32(fsi[488:492], freeClus)
		binary.LittleEndian.PutUint32(fsi[492:496], nextCluster)
		fsi[510], fsi[511] = 0x55, 0xAA
	}
	writeFSI(resBuf[1*512 : 2*512])
	writeFSI(resBuf[7*512 : 8*512])

	if _, err := disk.WriteAt(resBuf, int64(startLBA*512)); err != nil {
		return 0, 0, err
	}

	// 2. Build and write FAT1 and FAT2 tables
	fatTable := make([]byte, int(fatSectors*512))
	binary.LittleEndian.PutUint32(fatTable[0:4], 0x0FFFFFF8)
	binary.LittleEndian.PutUint32(fatTable[4:8], 0x0FFFFFFF) // Clean shutdown bit = 1
	var markChain func(n *isoTreeNode)
	markChain = func(n *isoTreeNode) {
		if n.NumCluster > 0 {
			for c := uint32(0); c < n.NumCluster; c++ {
				cur := n.Cluster + c
				val := cur + 1
				if c == n.NumCluster-1 {
					val = 0x0FFFFFFF
				}
				if int(cur+1)*4 <= len(fatTable) {
					binary.LittleEndian.PutUint32(fatTable[cur*4:(cur+1)*4], val)
				}
			}
		}
		for _, ch := range n.Children {
			markChain(ch)
		}
	}
	markChain(rootNode)

	fat1LBA := startLBA + uint64(reservedSectors)
	fat2LBA := fat1LBA + uint64(fatSectors)
	if err := writeAlignedChunks(disk, fatTable, int64(fat1LBA*512)); err != nil {
		return 0, 0, err
	}
	if err := writeAlignedChunks(disk, fatTable, int64(fat2LBA*512)); err != nil {
		return 0, 0, err
	}

	// 3. Write all directory and file clusters into data region
	dataStartLBA := fat2LBA + uint64(fatSectors)
	clusterOffsetBytes := func(clus uint32) int64 {
		return int64(dataStartLBA*512) + int64(clus-2)*int64(clusterBytes)
	}

	var writeDirTree func(dir *isoTreeNode, parentCluster uint32) error
	writeDirTree = func(dir *isoTreeNode, parentCluster uint32) error {
		dirBuf := make([]byte, int(dir.NumCluster)*clusterBytes)
		cursor := 0
		if dir.Cluster == 2 {
			var volEnt [32]byte
			copy(volEnt[0:11], []byte("UBUNTU-ISO "))
			stampDOSDirEntry(&volEnt, 0x08, 0, 0)
			copy(dirBuf[0:32], volEnt[:])
			cursor = 32
		} else {
			dotSelf := makeDotEntry(".          ", dir.Cluster)
			pClus := parentCluster
			if pClus == 2 {
				pClus = 0
			}
			dotParent := makeDotEntry("..         ", pClus)
			copy(dirBuf[0:32], dotSelf[:])
			copy(dirBuf[32:64], dotParent[:])
			cursor = 64
		}

		for idx, ch := range dir.Children {
			shortName := makeShort83Name(ch.Name, idx+1)
			for _, lfn := range buildLFNEntries(ch.Name, shortName) {
				if cursor+32 <= len(dirBuf) {
					copy(dirBuf[cursor:cursor+32], lfn[:])
					cursor += 32
				}
			}
			if cursor+32 <= len(dirBuf) {
				var stdEnt [32]byte
				copy(stdEnt[0:11], shortName[:])
				attr := byte(0x20)
				size := uint32(ch.TotalSize)
				if ch.IsDir {
					attr = 0x10
					size = 0
				}
				stampDOSDirEntry(&stdEnt, attr, ch.Cluster, size)
				copy(dirBuf[cursor:cursor+32], stdEnt[:])
				cursor += 32
			}
		}

		if err := writeAlignedChunks(disk, dirBuf, clusterOffsetBytes(dir.Cluster)); err != nil {
			return err
		}
		for _, ch := range dir.Children {
			if ch.IsDir {
				if err := writeDirTree(ch, dir.Cluster); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := writeDirTree(rootNode, 0); err != nil {
		return 0, 0, err
	}

	// Stream all file contents into their allocated contiguous FAT32 clusters
	dmaBuf := AllocPageAlignedBuffer(DMAChunkSizeBytes, SectorAlignmentBytes)
	var writtenBytes int64
	startTime := time.Now()

	var writeFiles func(dir *isoTreeNode) error
	writeFiles = func(dir *isoTreeNode) error {
		for _, ch := range dir.Children {
			if ch.IsDir {
				if err := writeFiles(ch); err != nil {
					return err
				}
				continue
			}
			if ch.TotalSize == 0 || ch.Cluster < 2 {
				continue
			}
			destOffset := clusterOffsetBytes(ch.Cluster)
			if len(ch.Override) > 0 {
				_, _ = crcHasher.Write(ch.Override)
				_, _ = shaHasher.Write(ch.Override)
				if err := writeAlignedChunks(disk, ch.Override, destOffset); err != nil {
					return err
				}
				writtenBytes += int64(len(ch.Override))
				continue
			}
			curDest := destOffset
			for _, ext := range ch.Extents {
				rem := ext.Length
				srcOff := ext.Offset
				for rem > 0 {
					if pauseGate != nil {
						if err := pauseGate.WaitIfPaused(ctx); err != nil {
							return err
						}
					}
					select {
					case <-ctx.Done():
						return ctx.Err()
					default:
					}
					step := int64(len(dmaBuf))
					if rem < step {
						step = rem
					}
					n, rErr := srcFile.ReadAt(dmaBuf[:step], srcOff)
					if rErr != nil && rErr != io.EOF {
						return rErr
					}
					if n == 0 {
						break
					}
					valid := dmaBuf[:n]
					_, _ = crcHasher.Write(valid)
					_, _ = shaHasher.Write(valid)

					writeLen := n
					if writeLen%SectorAlignmentBytes != 0 {
						padded := ((writeLen + SectorAlignmentBytes - 1) / SectorAlignmentBytes) * SectorAlignmentBytes
						for z := writeLen; z < padded; z++ {
							dmaBuf[z] = 0
						}
						writeLen = padded
					}
					if _, wErr := disk.WriteAt(dmaBuf[:writeLen], curDest); wErr != nil {
						return wErr
					}
					srcOff += int64(n)
					curDest += int64(n)
					rem -= int64(n)
					writtenBytes += int64(n)

					if onProgress != nil {
						elapsed := time.Since(startTime).Seconds()
						if elapsed < 0.001 {
							elapsed = 0.001
						}
						mbps := (float64(writtenBytes) / 1048576.0) / elapsed
						pct := int((writtenBytes * 95) / totalISOBytes)
						if pct > 95 {
							pct = 95
						}
						onProgress(FlashProgressEvent{
							Stage:        "FLASHING",
							Percent:      pct,
							WrittenBytes: writtenBytes,
							TotalBytes:   totalISOBytes,
							SpeedMBps:    mbps,
							CRC32Hex:     fmt.Sprintf("%08X", crcHasher.Sum32()),
							GrubPatches:  grubPatches,
							Message:      fmt.Sprintf("Writing editable FAT32 ISO files (%s): %.1f / %.1f MB @ %.1f MB/s", ch.Name, float64(writtenBytes)/1048576.0, float64(totalISOBytes)/1048576.0, mbps),
						})
					}
				}
			}
		}
		return nil
	}
	if err := writeFiles(rootNode); err != nil {
		return 0, 0, err
	}

	endLBA := startLBA + uint64(totalPartSectors) - 1
	return endLBA, grubPatches, nil
}
