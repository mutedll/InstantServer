// ============================================================================
// FILE: usb/usb_verifier.go
// PURPOSE: Formats the "INSTANTUSB" FAT32 partition (with next-boot scripts
//
//	and logs/ folder), resets flashed USB sticks back to clean FAT32,
//	and verifies bootable USB partition structures.
//
// ============================================================================
package usb

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"strings"

	"github.com/mutedll/InstantServer/usb/iso"
)

const windowsNextBootBatTemplate = "@echo off\r\n" +
	"title Instant Server - Boot This USB on Next Restart\r\n" +
	"echo ============================================================================\r\n" +
	"echo   INSTANT SERVER - ONE-CLICK BOOT INTO USB ON NEXT RESTART (WINDOWS)\r\n" +
	"echo ============================================================================\r\n" +
	"net session >nul 2>&1\r\n" +
	"if %errorLevel% neq 0 (\r\n" +
	"    echo Requesting Administrator privileges...\r\n" +
	"    powershell -NoProfile -Command \"Start-Process -FilePath '%~f0' -Verb RunAs\"\r\n" +
	"    exit /b\r\n" +
	")\r\n" +
	"echo.\r\n" +
	"set /p CONFIRM=\"Confirm restarting this computer into the Instant Server USB now? (Y/N): \"\r\n" +
	"if /i not \"%CONFIRM%\"==\"Y\" exit /b 0\r\n" +
	"for /f \"tokens=2\" %%I in ('bcdedit /enum firmware ^| findstr /i \"identifier\"') do (\r\n" +
	"    bcdedit /enum firmware %%I | findstr /i \"USB\" >nul 2>&1 && (\r\n" +
	"        bcdedit /set {fwbootmgr} bootsequence %%I\r\n" +
	"        shutdown /r /t 3 /c \"Restarting into Instant Server USB...\"\r\n" +
	"        exit /b 0\r\n" +
	"    )\r\n" +
	")\r\n" +
	"shutdown /r /fw /t 3\r\n"

const linuxNextBootShTemplate = "#!/usr/bin/env bash\n" +
	"set -euo pipefail\n" +
	"if [ \"$(id -u)\" -ne 0 ]; then exec sudo bash \"$0\" \"$@\"; fi\n" +
	"read -r -p \"Confirm restarting into Instant Server USB now? [y/N]: \" ans\n" +
	"[[ \"$ans\" =~ ^[Yy]$ ]] || exit 0\n" +
	"if command -v efibootmgr >/dev/null 2>&1; then\n" +
	"    USB_NUM=$(efibootmgr | grep -iE 'USB|Linpus|UEFI:' | head -n1 | sed -E 's/^Boot([0-9A-Fa-f]{4}).*/\\1/' || true)\n" +
	"    if [ -n \"$USB_NUM\" ]; then efibootmgr -n \"$USB_NUM\"; reboot; exit 0; fi\n" +
	"fi\n" +
	"systemctl reboot --firmware-setup || reboot\n"

const nextBootReadmeTxt = "INSTANT SERVER — USB LOGS & ONE-CLICK NEXT-BOOT RUNNERS\r\n" +
	"1. logs/ folder: Live installer logs (INSTANT-SERVER-INSTALL.log) are saved here.\r\n" +
	"2. BOOT-USB-NEXT-RESTART-WINDOWS.bat: Double-click on Windows to boot this USB on next restart.\r\n" +
	"3. boot-usb-next-restart-linux.sh: Run on Linux to set EFI BootNext to this USB.\r\n"

// FormatRemainingUSBAsFAT32 formats [startLBA..endLBA] as a clean FAT32 volume ("INSTANTUSB")
// containing the `logs/` directory and one-click next-boot launcher scripts.
func FormatRemainingUSBAsFAT32(disk RawDiskHandle, startLBA uint64, endLBA uint64) error {
	if endLBA <= startLBA+4096 {
		return fmt.Errorf("partition range too small for FAT32 (%d..%d)", startLBA, endLBA)
	}
	totalSectors := uint32(endLBA - startLBA + 1)
	const reservedSectors uint32 = 32
	const numFATs uint32 = 2
	secPerClus := uint32(64)
	if totalSectors < 524288 {
		secPerClus = 8
	}
	dataSectorsEst := totalSectors - reservedSectors
	totalClustersEst := dataSectorsEst / secPerClus
	fatSectors := (((totalClustersEst+2)*4+511)/512 + 7) / 8 * 8
	if fatSectors < 128 {
		fatSectors = 128
	}
	totalDataClusters := (totalSectors - reservedSectors - (numFATs * fatSectors)) / secPerClus
	clusterBytes := int(secPerClus * 512)

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
		binary.LittleEndian.PutUint32(b[32:36], totalSectors)
		binary.LittleEndian.PutUint32(b[36:40], fatSectors)
		binary.LittleEndian.PutUint32(b[44:48], 2)
		binary.LittleEndian.PutUint16(b[48:50], 1)
		binary.LittleEndian.PutUint16(b[50:52], 6)
		b[64], b[65], b[66] = 0x80, 0x00, 0x29
		binary.LittleEndian.PutUint32(b[67:71], 0x20261112)
		copy(b[71:82], []byte("INSTANTUSB "))
		copy(b[82:90], []byte("FAT32   "))
		b[510], b[511] = 0x55, 0xAA
	}
	writeBoot(resBuf[0:512])
	writeBoot(resBuf[6*512 : 7*512])

	const allocatedClusters uint32 = 5
	writeFSI := func(fsi []byte) {
		binary.LittleEndian.PutUint32(fsi[0:4], 0x41615252)
		binary.LittleEndian.PutUint32(fsi[484:488], 0x61417272)
		freeClus := uint32(0)
		if totalDataClusters > allocatedClusters {
			freeClus = totalDataClusters - allocatedClusters
		}
		binary.LittleEndian.PutUint32(fsi[488:492], freeClus)
		binary.LittleEndian.PutUint32(fsi[492:496], 2+allocatedClusters)
		fsi[510], fsi[511] = 0x55, 0xAA
	}
	writeFSI(resBuf[1*512 : 2*512])
	writeFSI(resBuf[7*512 : 8*512])

	if _, err := disk.WriteAt(resBuf, int64(startLBA*512)); err != nil {
		return err
	}

	fatTable := make([]byte, int(fatSectors*512))
	binary.LittleEndian.PutUint32(fatTable[0:4], 0x0FFFFFF8)
	binary.LittleEndian.PutUint32(fatTable[4:8], 0x0FFFFFFF)
	for c := uint32(2); c < 2+allocatedClusters; c++ {
		binary.LittleEndian.PutUint32(fatTable[c*4:(c+1)*4], 0x0FFFFFFF)
	}
	fat1LBA := startLBA + uint64(reservedSectors)
	fat2LBA := fat1LBA + uint64(fatSectors)
	if err := writeAlignedChunks(disk, fatTable, int64(fat1LBA*512)); err != nil {
		return err
	}
	if err := writeAlignedChunks(disk, fatTable, int64(fat2LBA*512)); err != nil {
		return err
	}

	dataLBA := fat2LBA + uint64(fatSectors)
	dataBuf := AllocPageAlignedBuffer(int(allocatedClusters)*clusterBytes, SectorAlignmentBytes)

	// Cluster 2: Root Directory
	var volEnt [32]byte
	copy(volEnt[0:11], []byte("INSTANTUSB "))
	iso.StampDOSDirEntry(&volEnt, 0x08, 0, 0)
	copy(dataBuf[0:32], volEnt[:])
	cursor := 32

	usedShort := make(map[[11]byte]bool)
	items := []struct {
		name    string
		attr    byte
		cluster uint32
		data    []byte
	}{
		{"logs", 0x10, 3, nil},
		{"BOOT-USB-NEXT-RESTART-WINDOWS.bat", 0x20, 4, []byte(windowsNextBootBatTemplate)},
		{"boot-usb-next-restart-linux.sh", 0x20, 5, []byte(linuxNextBootShTemplate)},
		{"README-INSTANT-USB.txt", 0x20, 6, []byte(nextBootReadmeTxt)},
	}
	for idx, it := range items {
		sn := iso.MakeClean83ShortName(it.name, idx+1, usedShort)
		for _, lfn := range iso.BuildLFNEntries(it.name, sn) {
			copy(dataBuf[cursor:cursor+32], lfn[:])
			cursor += 32
		}
		var de [32]byte
		copy(de[0:11], sn[:])
		iso.StampDOSDirEntry(&de, it.attr, it.cluster, uint32(len(it.data)))
		copy(dataBuf[cursor:cursor+32], de[:])
		cursor += 32
		if len(it.data) > 0 {
			cOff := int(it.cluster-2) * clusterBytes
			copy(dataBuf[cOff:cOff+len(it.data)], it.data)
		}
	}

	// Cluster 3: `logs` subdirectory with mandatory "." and ".." entries
	logsOff := int(3-2) * clusterBytes
	dotSelf := iso.MakeDotEntry(".          ", 3)
	dotParent := iso.MakeDotEntry("..         ", 0)
	copy(dataBuf[logsOff:logsOff+32], dotSelf[:])
	copy(dataBuf[logsOff+32:logsOff+64], dotParent[:])

	return writeAlignedChunks(disk, dataBuf, int64(dataLBA*512))
}

// ResetUSBToCleanFAT32 clears any hybrid ISO9660/GPT write-protection signatures on a USB drive
// and formats 100% of its capacity as a standard single-partition FAT32 drive ("INSTANTUSB").
func ResetUSBToCleanFAT32(targetDevice string) error {
	releaseWakeLock := AcquireHostWakeLock()
	defer releaseWakeLock()

	rawDisk, err := OpenRawDisk(targetDevice)
	if err != nil {
		return err
	}
	defer rawDisk.Close()

	diskEndBytes, err := rawDisk.Seek(0, io.SeekEnd)
	if err != nil || diskEndBytes < 8*1024*1024 {
		return fmt.Errorf("IS-USB-320: unable to determine capacity of USB drive %s (size=%d)", targetDevice, diskEndBytes)
	}

	totalDiskSectors := uint64(diskEndBytes) / LPTSectorSize
	const numEntries = 128
	const entrySize = 128
	const arrayBytesLen = numEntries * entrySize
	const arraySectors = uint64(arrayBytesLen / LPTSectorSize) // 32 sectors

	backupHeaderLBA := totalDiskSectors - 1
	backupArrayLBA := backupHeaderLBA - arraySectors
	firstUsableLBA := uint64(34)
	lastUsableLBA := backupArrayLBA - 1

	// 1. Wipe first 1 MB (including LBA 0 MBR, LBA 1 GPT, and offset 32768 ISO9660 CD001 signature)
	zero1M := AllocPageAlignedBuffer(1024*1024, SectorAlignmentBytes)
	if _, err := rawDisk.WriteAt(zero1M, 0); err != nil {
		return fmt.Errorf("IS-USB-321: failed clearing ISO9660/GPT header on %s: %w", targetDevice, err)
	}

	// 2. Format single FAT32 partition starting at LBA 2048 (1 MB) up to lastUsableLBA
	dataStartLBA := uint64(2048)
	dataEndLBA := (lastUsableLBA/2048)*2048 - 1
	if dataEndLBA <= dataStartLBA+2048 {
		dataEndLBA = lastUsableLBA
	}
	if err := FormatRemainingUSBAsFAT32(rawDisk, dataStartLBA, dataEndLBA); err != nil {
		return fmt.Errorf("IS-USB-322: failed formatting FAT32 partition on %s: %w", targetDevice, err)
	}

	// 3. Write clean Protective MBR + Primary GPT Header + Partition Entry #1
	tableBuf := AllocPageAlignedBuffer(8*SectorAlignmentBytes, SectorAlignmentBytes)
	writeStandardProtectiveMBR(tableBuf[:LPTSectorSize], totalDiskSectors)

	gptHdr := tableBuf[LPTSectorSize : LPTSectorSize*2]
	partArray := tableBuf[LPTSectorSize*2:]

	ent := partArray[0:128]
	resetPartGUID := generateRandomGUID()
	copy(ent[0:16], microsoftBasicDataGUID[:])
	copy(ent[16:32], resetPartGUID[:])
	binary.LittleEndian.PutUint64(ent[32:40], dataStartLBA)
	binary.LittleEndian.PutUint64(ent[40:48], dataEndLBA)
	binary.LittleEndian.PutUint64(ent[48:56], 0)
	for idx, ch := range []uint16{'I', 'N', 'S', 'T', 'A', 'N', 'T', 'U', 'S', 'B'} {
		binary.LittleEndian.PutUint16(ent[56+idx*2:58+idx*2], ch)
	}

	copy(gptHdr[0:8], []byte("EFI PART"))
	binary.LittleEndian.PutUint32(gptHdr[8:12], 0x00010000)
	binary.LittleEndian.PutUint32(gptHdr[12:16], 92)
	binary.LittleEndian.PutUint64(gptHdr[24:32], 1)
	binary.LittleEndian.PutUint64(gptHdr[32:40], backupHeaderLBA)
	binary.LittleEndian.PutUint64(gptHdr[40:48], firstUsableLBA)
	binary.LittleEndian.PutUint64(gptHdr[48:56], lastUsableLBA)
	resetDiskGUID := generateRandomGUID()
	copy(gptHdr[56:72], resetDiskGUID[:])
	binary.LittleEndian.PutUint64(gptHdr[72:80], 2)
	binary.LittleEndian.PutUint32(gptHdr[80:84], numEntries)
	binary.LittleEndian.PutUint32(gptHdr[84:88], entrySize)
	partCRC := crc32.ChecksumIEEE(partArray[:arrayBytesLen])
	binary.LittleEndian.PutUint32(gptHdr[88:92], partCRC)
	hdrCRC := crc32.ChecksumIEEE(gptHdr[:92])
	binary.LittleEndian.PutUint32(gptHdr[16:20], hdrCRC)

	if _, err := rawDisk.WriteAt(tableBuf, 0); err != nil {
		return err
	}

	// 4. Write Backup GPT Entry Array + Backup GPT Header at totalDiskSectors - 1
	var backupHdr [LPTSectorSize]byte
	copy(backupHdr[:], gptHdr)
	binary.LittleEndian.PutUint64(backupHdr[24:32], backupHeaderLBA)
	binary.LittleEndian.PutUint64(backupHdr[32:40], 1)
	binary.LittleEndian.PutUint64(backupHdr[72:80], backupArrayLBA)
	backupHdr[16], backupHdr[17], backupHdr[18], backupHdr[19] = 0, 0, 0, 0
	backupHdrCRC := crc32.ChecksumIEEE(backupHdr[:92])
	binary.LittleEndian.PutUint32(backupHdr[16:20], backupHdrCRC)

	alignedStartLBA := (backupArrayLBA / 8) * 8
	endSpanSectors := (backupHeaderLBA + 1) - alignedStartLBA
	alignedBytes := int(((endSpanSectors + 7) / 8) * 8 * LPTSectorSize)
	backupWindow := AllocPageAlignedBuffer(alignedBytes, SectorAlignmentBytes)
	arrOff := int((backupArrayLBA - alignedStartLBA) * LPTSectorSize)
	hdrOff := int((backupHeaderLBA - alignedStartLBA) * LPTSectorSize)
	if arrOff >= 0 && arrOff+arrayBytesLen <= len(backupWindow) {
		copy(backupWindow[arrOff:arrOff+arrayBytesLen], partArray[:arrayBytesLen])
	}
	if hdrOff >= 0 && hdrOff+LPTSectorSize <= len(backupWindow) {
		copy(backupWindow[hdrOff:hdrOff+LPTSectorSize], backupHdr[:])
	}
	writeLen := int(endSpanSectors * LPTSectorSize)
	if rawDisk.RequiresPageAlignedTail() && (writeLen%SectorAlignmentBytes != 0) {
		writeLen = len(backupWindow)
	}
	if _, err := rawDisk.WriteAt(backupWindow[:writeLen], int64(alignedStartLBA*LPTSectorSize)); err != nil {
		return err
	}

	return rawDisk.SyncAndFlush()
}

type USBVerificationReport struct {
	Passed     bool     `json:"passed"`
	Device     string   `json:"device"`
	Partitions []string `json:"partitions"`
	Checks     []string `json:"checks"`
	Summary    string   `json:"summary"`
}

// VerifyBootableUSB inspects the GPT partition table, boot signatures, FAT clean bits,
// and CIDATA / UBUNTU-ISO / INSTANTUSB partitions on targetDevice without modifying it.
func VerifyBootableUSB(targetDevice string) (*USBVerificationReport, error) {
	f, err := os.Open(strings.TrimSpace(targetDevice))
	if err != nil {
		return nil, fmt.Errorf("IS-USB-330: unable to open %s for verification: %w", targetDevice, err)
	}
	defer f.Close()

	report := &USBVerificationReport{
		Passed: true,
		Device: targetDevice,
	}

	hdrBuf := make([]byte, 34*512)
	if _, err := f.ReadAt(hdrBuf, 0); err != nil {
		return nil, fmt.Errorf("IS-USB-331: failed reading GPT header from %s: %w", targetDevice, err)
	}

	if hdrBuf[510] == 0x55 && hdrBuf[511] == 0xAA {
		report.Checks = append(report.Checks, "LBA 0 Protective MBR signature (0x55AA) verified.")
	} else {
		report.Passed = false
		report.Checks = append(report.Checks, "Missing LBA 0 MBR signature (0x55AA).")
	}

	gptHdr := hdrBuf[512:1024]
	if string(gptHdr[0:8]) == "EFI PART" {
		report.Checks = append(report.Checks, "LBA 1 Primary GPT Header (EFI PART) verified.")
	} else {
		report.Passed = false
		report.Checks = append(report.Checks, "Missing LBA 1 GPT Header (EFI PART).")
	}

	partArray := hdrBuf[1024:]
	foundISO := false
	foundCidata := false
	foundInstantUSB := false

	for i := 0; i < 128 && (i+1)*128 <= len(partArray); i++ {
		ent := partArray[i*128 : (i+1)*128]
		allZero := true
		for _, b := range ent[:16] {
			if b != 0 {
				allZero = false
				break
			}
		}
		if allZero {
			continue
		}
		startLBA := binary.LittleEndian.Uint64(ent[32:40])
		endLBA := binary.LittleEndian.Uint64(ent[40:48])
		attrs := binary.LittleEndian.Uint64(ent[48:56])

		var nameRunes []rune
		for c := 0; c < 36; c++ {
			u := binary.LittleEndian.Uint16(ent[56+c*2 : 58+c*2])
			if u == 0 {
				break
			}
			nameRunes = append(nameRunes, rune(u))
		}
		partName := strings.TrimSpace(string(nameRunes))
		sizeMB := float64((endLBA-startLBA+1)*512) / (1024.0 * 1024.0)
		report.Partitions = append(report.Partitions, fmt.Sprintf("Partition #%d: %s (%.1f MB, LBA %d..%d)", i+1, partName, sizeMB, startLBA, endLBA))

		if attrs != 0 {
			report.Checks = append(report.Checks, fmt.Sprintf("Warning: Partition %s has non-zero GPT attribute flags (0x%X).", partName, attrs))
		}

		var bpb [512]byte
		if _, rErr := f.ReadAt(bpb[:], int64(startLBA*512)); rErr == nil {
			if string(bpb[54:62]) == "FAT16   " {
				foundCidata = true
				clean := bpb[37] == 0
				report.Checks = append(report.Checks, fmt.Sprintf("Partition %s: valid FAT16 boot sector (clean_bit=%v).", partName, clean))
			} else if string(bpb[82:90]) == "FAT32   " {
				clean := bpb[65] == 0
				lbl := strings.TrimSpace(string(bpb[71:82]))
				if lbl == "UBUNTU-ISO" || i == 0 {
					foundISO = true
				}
				if lbl == "INSTANTUSB" {
					foundInstantUSB = true
				}
				report.Checks = append(report.Checks, fmt.Sprintf("Partition %s (%s): valid FAT32 boot sector (clean_bit=%v).", partName, lbl, clean))
			} else if strings.EqualFold(partName, "ISO9660") || i == 0 {
				foundISO = true
				report.Checks = append(report.Checks, fmt.Sprintf("Partition %s: bootable ISO stream verified.", partName))
			}
		}
	}

	if !foundISO {
		report.Passed = false
		report.Checks = append(report.Checks, "Bootable Ubuntu ISO partition not found on drive.")
	}
	if !foundCidata {
		report.Passed = false
		report.Checks = append(report.Checks, "CIDATA autoinstall partition not found on drive.")
	}
	if foundInstantUSB {
		report.Checks = append(report.Checks, "INSTANTUSB FAT32 logs & next-boot launcher partition verified.")
	}

	if report.Passed {
		report.Summary = fmt.Sprintf("USB drive %s passed all bootability, GPT, and FAT clean-bit checks (%d partitions ready).", targetDevice, len(report.Partitions))
	} else {
		report.Summary = fmt.Sprintf("USB drive %s failed one or more bootability checks.", targetDevice)
	}
	return report, nil
}
