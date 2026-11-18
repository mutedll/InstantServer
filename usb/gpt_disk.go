// ============================================================================
// FILE: usb/gpt_disk.go
// PURPOSE: Builds RFC 4122 v4 unique GPT Disk/Partition GUIDs, Protective MBRs,
//
//	and Primary + Backup GPT tables for 2-partition and 3-partition USBs.
//
// ============================================================================
package usb

import (
	"crypto/rand"
	"encoding/binary"
	"hash/crc32"
	"io"
	"time"
)

var microsoftBasicDataGUID = [16]byte{
	0xA2, 0xA0, 0xD0, 0xEB,
	0xE5, 0xB9,
	0x33, 0x44,
	0x87, 0xC0,
	0x68, 0xB6, 0xB7, 0x26, 0x99, 0xC7,
}

var cidataUniquePartGUID = [16]byte{
	0x49, 0x4E, 0x53, 0x54,
	0x41, 0x4E,
	0x54, 0x53,
	0x43, 0x49,
	0x44, 0x41, 0x54, 0x41, 0x30, 0x31,
}

var instantUSBUniquePartGUID = [16]byte{
	0x49, 0x4E, 0x53, 0x54,
	0x41, 0x4E,
	0x54, 0x53,
	0x55, 0x53,
	0x42, 0x44, 0x41, 0x54, 0x41, 0x32,
}

func RelocateGPTAndAppendCidataPartition(disk RawDiskHandle, isoBytes uint64) error {
	return RelocateGPTAndWriteCidataPartition(disk, isoBytes, nil)
}

// generateRandomGUID produces a fresh RFC 4122 v4 UUID in mixed-endian 16-byte GPT binary format
// so every flashed USB stick and virtual VHD/VHDX disk gets a 100% unique Disk GUID and Partition GUIDs,
// preventing Windows Disk Management "signature collision with a disk that is online" errors.
func generateRandomGUID() [16]byte {
	var g [16]byte
	if _, err := rand.Read(g[:]); err != nil {
		ns := uint64(time.Now().UnixNano())
		binary.LittleEndian.PutUint64(g[0:8], ns)
		binary.LittleEndian.PutUint64(g[8:16], ns^0xA5A5F0F05A5A0F0F)
	}
	g[6] = (g[6] & 0x0F) | 0x40 // Version 4
	g[8] = (g[8] & 0x3F) | 0x80 // RFC 4122 Variant
	return g
}

func generateRandomMBRSignature() uint32 {
	var b [4]byte
	if _, err := rand.Read(b[:]); err == nil {
		sig := binary.LittleEndian.Uint32(b[:])
		if sig != 0 {
			return sig
		}
	}
	return uint32(time.Now().UnixNano()&0x7FFFFFFF) | 1
}

func writeStandardProtectiveMBR(mbrSector []byte, totalDiskSectors uint64) {
	// Assign a unique non-zero 32-bit MBR Disk Signature at offset 440..444 (0x1B8..0x1BB)
	// so Windows partmgr.sys never flags a duplicate MBR signature collision.
	if binary.LittleEndian.Uint32(mbrSector[440:444]) == 0 {
		binary.LittleEndian.PutUint32(mbrSector[440:444], generateRandomMBRSignature())
	}
	// Clear reserved bytes 444..446 and all 4 MBR partition entries at bytes 446..510
	for i := 444; i < 510; i++ {
		mbrSector[i] = 0
	}
	protSectors := totalDiskSectors - 1
	if protSectors > 0xFFFFFFFF {
		protSectors = 0xFFFFFFFF
	}
	mbrSector[446+0] = 0x00 // Non-bootable protective entry (UEFI spec 5.2.2)
	mbrSector[446+1] = 0x00
	mbrSector[446+2] = 0x02
	mbrSector[446+3] = 0x00
	mbrSector[446+4] = 0xEE // GPT Protective MBR type
	mbrSector[446+5] = 0xFF
	mbrSector[446+6] = 0xFF
	mbrSector[446+7] = 0xFF
	binary.LittleEndian.PutUint32(mbrSector[446+8:446+12], 1)
	binary.LittleEndian.PutUint32(mbrSector[446+12:446+16], uint32(protSectors))
	mbrSector[510] = 0x55
	mbrSector[511] = 0xAA
}

func RelocateGPTAndWriteCidataPartition(disk RawDiskHandle, isoBytes uint64, cidataFATImage []byte) error {
	tableBuf := AllocPageAlignedBuffer(8*SectorAlignmentBytes, SectorAlignmentBytes)
	if _, err := disk.ReadAt(tableBuf, 0); err != nil {
		return err
	}

	gptHdr := tableBuf[LPTSectorSize : LPTSectorSize*2]
	if string(gptHdr[0:8]) != "EFI PART" {
		return nil
	}

	origAlternateLBA := binary.LittleEndian.Uint64(gptHdr[32:40])

	cidataStartLBA := ((isoBytes + (1024*1024 - 1)) / (1024 * 1024)) * (1024 * 1024 / LPTSectorSize)
	cidataSectors := uint64(len(cidataFATImage)) / LPTSectorSize
	if cidataSectors == 0 {
		cidataSectors = uint64(fatTotalSectors)
	}
	cidataEndLBA := cidataStartLBA + cidataSectors - 1

	if len(cidataFATImage) > 0 {
		if err := writeAlignedChunks(disk, cidataFATImage, int64(cidataStartLBA*LPTSectorSize)); err != nil {
			return err
		}
	}

	partArray := tableBuf[LPTSectorSize*2:]
	numEntries := int(binary.LittleEndian.Uint32(gptHdr[80:84]))
	entrySize := int(binary.LittleEndian.Uint32(gptHdr[84:88]))
	if entrySize <= 0 || entrySize > 512 {
		entrySize = 128
		binary.LittleEndian.PutUint32(gptHdr[84:88], 128)
	}
	maxEntriesInBuf := len(partArray) / entrySize
	if numEntries <= 0 || numEntries > maxEntriesInBuf {
		numEntries = maxEntriesInBuf
		binary.LittleEndian.PutUint32(gptHdr[80:84], uint32(numEntries))
	}

	arrayBytesLen := numEntries * entrySize
	if arrayBytesLen > len(partArray) {
		arrayBytesLen = len(partArray)
	}
	arraySectors := uint64((arrayBytesLen + LPTSectorSize - 1) / LPTSectorSize)

	backupArrayLBA := ((cidataEndLBA + 1 + 7) / 8) * 8
	backupHeaderLBA := backupArrayLBA + arraySectors
	totalDiskSectors := backupHeaderLBA + 1

	if diskEndBytes, seekErr := disk.Seek(0, io.SeekEnd); seekErr == nil && diskEndBytes > 0 {
		physSectors := uint64(diskEndBytes) / LPTSectorSize
		if physSectors > cidataEndLBA+arraySectors+32 {
			totalDiskSectors = physSectors
			backupHeaderLBA = totalDiskSectors - 1
			backupArrayLBA = backupHeaderLBA - arraySectors
		}
	}
	lastUsableLBA := backupArrayLBA - 1

	if origAlternateLBA > 1 && origAlternateLBA < cidataStartLBA && backupHeaderLBA > origAlternateLBA {
		alignedOldLBA := (origAlternateLBA / 8) * 8
		zeroPage := AllocPageAlignedBuffer(SectorAlignmentBytes, SectorAlignmentBytes)
		if _, rErr := disk.ReadAt(zeroPage, int64(alignedOldLBA*LPTSectorSize)); rErr == nil {
			subOff := int((origAlternateLBA - alignedOldLBA) * LPTSectorSize)
			if subOff >= 0 && subOff+LPTSectorSize <= len(zeroPage) {
				for z := subOff; z < subOff+LPTSectorSize; z++ {
					zeroPage[z] = 0
				}
				_, _ = disk.WriteAt(zeroPage, int64(alignedOldLBA*LPTSectorSize))
			}
		}
	}

	var freeSlots []int
	for i := 0; i < numEntries && (i+1)*entrySize <= len(partArray); i++ {
		ent := partArray[i*entrySize : (i+1)*entrySize]
		allZero := true
		for _, b := range ent[:16] {
			if b != 0 {
				allZero = false
				break
			}
		}
		if allZero {
			freeSlots = append(freeSlots, i*entrySize)
			continue
		}
		// Keep existing partitions visible in File Explorer (clear hidden/read-only GPT attribute flags)
		binary.LittleEndian.PutUint64(ent[48:56], 0)
	}
	if len(freeSlots) == 0 {
		freeSlots = append(freeSlots, entrySize)
	}

	cidataSlot := freeSlots[0]
	if cidataSlot+128 <= len(partArray) {
		ent := partArray[cidataSlot : cidataSlot+128]
		for i := range ent {
			ent[i] = 0
		}
		cidataGUID := generateRandomGUID()
		_ = cidataUniquePartGUID
		copy(ent[0:16], microsoftBasicDataGUID[:])
		copy(ent[16:32], cidataGUID[:])
		binary.LittleEndian.PutUint64(ent[32:40], cidataStartLBA)
		binary.LittleEndian.PutUint64(ent[40:48], cidataEndLBA)
		binary.LittleEndian.PutUint64(ent[48:56], 0)
		for idx, ch := range []uint16{'C', 'I', 'D', 'A', 'T', 'A'} {
			binary.LittleEndian.PutUint16(ent[56+idx*2:58+idx*2], ch)
		}
	}

	dataStartLBA := ((cidataEndLBA + 1 + 2047) / 2048) * 2048
	dataEndLBA := (lastUsableLBA/2048)*2048 - 1
	if len(freeSlots) >= 2 && dataEndLBA > dataStartLBA+131072 {
		dataSlot := freeSlots[1]
		if dataSlot+128 <= len(partArray) {
			if fmtErr := FormatRemainingUSBAsFAT32(disk, dataStartLBA, dataEndLBA); fmtErr == nil {
				ent := partArray[dataSlot : dataSlot+128]
				for i := range ent {
					ent[i] = 0
				}
				instGUID := generateRandomGUID()
				_ = instantUSBUniquePartGUID
				copy(ent[0:16], microsoftBasicDataGUID[:])
				copy(ent[16:32], instGUID[:])
				binary.LittleEndian.PutUint64(ent[32:40], dataStartLBA)
				binary.LittleEndian.PutUint64(ent[40:48], dataEndLBA)
				binary.LittleEndian.PutUint64(ent[48:56], 0)
				for idx, ch := range []uint16{'I', 'N', 'S', 'T', 'A', 'N', 'T', 'U', 'S', 'B'} {
					binary.LittleEndian.PutUint16(ent[56+idx*2:58+idx*2], ch)
				}
			}
		}
	}

	binary.LittleEndian.PutUint64(gptHdr[24:32], 1)
	binary.LittleEndian.PutUint64(gptHdr[32:40], backupHeaderLBA)
	binary.LittleEndian.PutUint64(gptHdr[48:56], lastUsableLBA)
	relocDiskGUID := generateRandomGUID()
	copy(gptHdr[56:72], relocDiskGUID[:])

	partCRC := crc32.ChecksumIEEE(partArray[:arrayBytesLen])
	binary.LittleEndian.PutUint32(gptHdr[88:92], partCRC)

	hdrSize := int(binary.LittleEndian.Uint32(gptHdr[12:16]))
	if hdrSize < 92 || hdrSize > LPTSectorSize {
		hdrSize = 92
		binary.LittleEndian.PutUint32(gptHdr[12:16], 92)
	}
	gptHdr[16], gptHdr[17], gptHdr[18], gptHdr[19] = 0, 0, 0, 0
	hdrCRC := crc32.ChecksumIEEE(gptHdr[:hdrSize])
	binary.LittleEndian.PutUint32(gptHdr[16:20], hdrCRC)

	writeStandardProtectiveMBR(tableBuf[:LPTSectorSize], totalDiskSectors)

	if _, err := disk.WriteAt(tableBuf, 0); err != nil {
		return err
	}

	var backupHdr [LPTSectorSize]byte
	copy(backupHdr[:], gptHdr)
	binary.LittleEndian.PutUint64(backupHdr[24:32], backupHeaderLBA)
	binary.LittleEndian.PutUint64(backupHdr[32:40], 1)
	binary.LittleEndian.PutUint64(backupHdr[72:80], backupArrayLBA)
	backupHdr[16], backupHdr[17], backupHdr[18], backupHdr[19] = 0, 0, 0, 0
	backupHdrCRC := crc32.ChecksumIEEE(backupHdr[:hdrSize])
	binary.LittleEndian.PutUint32(backupHdr[16:20], backupHdrCRC)

	alignedStartLBA := (backupArrayLBA / 8) * 8
	endSpanSectors := (backupHeaderLBA + 1) - alignedStartLBA
	alignedBytes := int(((endSpanSectors + 7) / 8) * 8 * LPTSectorSize)
	backupWindow := AllocPageAlignedBuffer(alignedBytes, SectorAlignmentBytes)
	_, _ = disk.ReadAt(backupWindow, int64(alignedStartLBA*LPTSectorSize))

	arrOff := int((backupArrayLBA - alignedStartLBA) * LPTSectorSize)
	hdrOff := int((backupHeaderLBA - alignedStartLBA) * LPTSectorSize)
	if arrOff >= 0 && arrOff+arrayBytesLen <= len(backupWindow) {
		copy(backupWindow[arrOff:arrOff+arrayBytesLen], partArray[:arrayBytesLen])
	}
	if hdrOff >= 0 && hdrOff+LPTSectorSize <= len(backupWindow) {
		copy(backupWindow[hdrOff:hdrOff+LPTSectorSize], backupHdr[:])
	}

	writeLen := int(endSpanSectors * LPTSectorSize)
	if disk.RequiresPageAlignedTail() && (writeLen%SectorAlignmentBytes != 0) {
		writeLen = (writeLen / SectorAlignmentBytes) * SectorAlignmentBytes
		if writeLen < hdrOff+LPTSectorSize {
			writeLen = len(backupWindow)
		}
	}
	if _, err := disk.WriteAt(backupWindow[:writeLen], int64(alignedStartLBA*LPTSectorSize)); err != nil {
		return err
	}
	return disk.SyncAndFlush()
}

var ubuntuISOUniquePartGUID = [16]byte{
	0x49, 0x4E, 0x53, 0x54,
	0x41, 0x4E,
	0x54, 0x53,
	0x55, 0x42,
	0x55, 0x4E, 0x54, 0x55, 0x30, 0x30,
}

func putUTF16PartitionName(ent []byte, name string) {
	for idx, ch := range name {
		if 56+idx*2+2 <= 128 {
			binary.LittleEndian.PutUint16(ent[56+idx*2:58+idx*2], uint16(ch))
		}
	}
}

func writeThreePartitionGPT(disk RawDiskHandle, isoStartLBA, isoEndLBA, cidataStartLBA, cidataEndLBA uint64) error {
	const numEntries = 128
	const entrySize = 128
	const arrayBytesLen = numEntries * entrySize
	const arraySectors = uint64(arrayBytesLen / LPTSectorSize)

	backupArrayLBA := ((cidataEndLBA + 1 + 7) / 8) * 8
	backupHeaderLBA := backupArrayLBA + arraySectors
	totalDiskSectors := backupHeaderLBA + 1

	if diskEndBytes, seekErr := disk.Seek(0, io.SeekEnd); seekErr == nil && diskEndBytes > 0 {
		physSectors := uint64(diskEndBytes) / LPTSectorSize
		if physSectors > cidataEndLBA+arraySectors+32 {
			totalDiskSectors = physSectors
			backupHeaderLBA = totalDiskSectors - 1
			backupArrayLBA = backupHeaderLBA - arraySectors
		}
	}
	firstUsableLBA := uint64(34)
	lastUsableLBA := backupArrayLBA - 1

	tableBuf := AllocPageAlignedBuffer(8*SectorAlignmentBytes, SectorAlignmentBytes)
	writeStandardProtectiveMBR(tableBuf[:LPTSectorSize], totalDiskSectors)

	gptHdr := tableBuf[LPTSectorSize : LPTSectorSize*2]
	partArray := tableBuf[LPTSectorSize*2:]

	// Generate distinct random RFC 4122 v4 GUIDs for the Disk and each Partition so Windows
	// never triggers "The disk is offline because it has a signature collision".
	_ = ubuntuISOUniquePartGUID
	part1GUID := generateRandomGUID()
	part2GUID := generateRandomGUID()
	part3GUID := generateRandomGUID()
	diskGUID := generateRandomGUID()

	// Partition #1: UBUNTU-ISO (FAT32, Microsoft Basic Data — openable & editable in Explorer)
	ent0 := partArray[0:128]
	copy(ent0[0:16], microsoftBasicDataGUID[:])
	copy(ent0[16:32], part1GUID[:])
	binary.LittleEndian.PutUint64(ent0[32:40], isoStartLBA)
	binary.LittleEndian.PutUint64(ent0[40:48], isoEndLBA)
	binary.LittleEndian.PutUint64(ent0[48:56], 0)
	putUTF16PartitionName(ent0, "UBUNTU-ISO")

	// Partition #2: CIDATA (FAT16, Microsoft Basic Data — openable & editable in Explorer)
	ent1 := partArray[128:256]
	copy(ent1[0:16], microsoftBasicDataGUID[:])
	copy(ent1[16:32], part2GUID[:])
	binary.LittleEndian.PutUint64(ent1[32:40], cidataStartLBA)
	binary.LittleEndian.PutUint64(ent1[40:48], cidataEndLBA)
	binary.LittleEndian.PutUint64(ent1[48:56], 0)
	putUTF16PartitionName(ent1, "CIDATA")

	// Partition #3: INSTANTUSB (FAT32, Microsoft Basic Data — logs & next-boot launchers)
	dataStartLBA := ((cidataEndLBA + 1 + 2047) / 2048) * 2048
	dataEndLBA := (lastUsableLBA/2048)*2048 - 1
	if dataEndLBA > dataStartLBA+8192 {
		if fmtErr := FormatRemainingUSBAsFAT32(disk, dataStartLBA, dataEndLBA); fmtErr == nil {
			ent2 := partArray[256:384]
			copy(ent2[0:16], microsoftBasicDataGUID[:])
			copy(ent2[16:32], part3GUID[:])
			binary.LittleEndian.PutUint64(ent2[32:40], dataStartLBA)
			binary.LittleEndian.PutUint64(ent2[40:48], dataEndLBA)
			binary.LittleEndian.PutUint64(ent2[48:56], 0)
			putUTF16PartitionName(ent2, "INSTANTUSB")
		}
	}

	copy(gptHdr[0:8], []byte("EFI PART"))
	binary.LittleEndian.PutUint32(gptHdr[8:12], 0x00010000)
	binary.LittleEndian.PutUint32(gptHdr[12:16], 92)
	binary.LittleEndian.PutUint64(gptHdr[24:32], 1)
	binary.LittleEndian.PutUint64(gptHdr[32:40], backupHeaderLBA)
	binary.LittleEndian.PutUint64(gptHdr[40:48], firstUsableLBA)
	binary.LittleEndian.PutUint64(gptHdr[48:56], lastUsableLBA)
	copy(gptHdr[56:72], diskGUID[:])
	binary.LittleEndian.PutUint64(gptHdr[72:80], 2)
	binary.LittleEndian.PutUint32(gptHdr[80:84], numEntries)
	binary.LittleEndian.PutUint32(gptHdr[84:88], entrySize)
	partCRC := crc32.ChecksumIEEE(partArray[:arrayBytesLen])
	binary.LittleEndian.PutUint32(gptHdr[88:92], partCRC)
	hdrCRC := crc32.ChecksumIEEE(gptHdr[:92])
	binary.LittleEndian.PutUint32(gptHdr[16:20], hdrCRC)

	if _, err := disk.WriteAt(tableBuf, 0); err != nil {
		return err
	}

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
	if disk.RequiresPageAlignedTail() && (writeLen%SectorAlignmentBytes != 0) {
		writeLen = len(backupWindow)
	}
	_, err := disk.WriteAt(backupWindow[:writeLen], int64(alignedStartLBA*LPTSectorSize))
	return err
}

// ============================================================================
// SECTION 5: BOOTABLE USB VERIFICATION ENGINE ("Verify Bootable USB" button)
// ============================================================================
