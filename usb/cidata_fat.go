// usb/cidata_fat.go
package usb

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	// 32,896 sectors * 512 bytes = 16,842,752 bytes (~16.06 MB)
	// Guarantees > 4,096 data clusters (>= 4,085 clusters is required by the Microsoft FAT specification
	// and Linux kernel vfat driver to mount as strict FAT16 instead of FAT12).
	fatTotalSectors    = 32896
	fatBytesPerSector  = 512
	fatSectorsPerClust = 8 // 4,096 bytes per cluster
	fatReservedSectors = 8 // 4,096 bytes aligned
	fatNumTables       = 2
	fatSectorsPerTable = 16 // 16 * 512 = 8,192 bytes (holds 4,096 16-bit cluster entries)
	fatRootDirEntries  = 512
	fatRootDirSectors  = (fatRootDirEntries * 32) / fatBytesPerSector // 32 sectors (16,384 bytes)
	fatDataStartSector = fatReservedSectors + (fatNumTables * fatSectorsPerTable) + fatRootDirSectors
)

type fatStagedFile struct {
	Name string
	Data []byte
}

func vfatChecksum(shortName [11]byte) byte {
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
	dotIdx := strings.LastIndex(upper, ".")
	basePart := upper
	extPart := ""
	if dotIdx > 0 {
		basePart = upper[:dotIdx]
		extPart = upper[dotIdx+1:]
	}

	clean := func(s string) string {
		var b strings.Builder
		for _, r := range s {
			if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
				b.WriteRune(r)
			}
		}
		return b.String()
	}

	bClean := clean(basePart)
	eClean := clean(extPart)
	if len(bClean) == 0 {
		bClean = "FILE"
	}

	// If simple 8.3 and no hyphen/mixed-case/long name, use exact 8.3 short name
	isSimple83 := (dotIdx < 0 && len(name) <= 8 && bClean == upper) ||
		(dotIdx > 0 && len(basePart) <= 8 && len(extPart) <= 3 && bClean == basePart && eClean == extPart)

	if isSimple83 {
		for i := 0; i < len(bClean) && i < 8; i++ {
			out[i] = bClean[i]
		}
		for i := 0; i < len(eClean) && i < 3; i++ {
			out[8+i] = eClean[i]
		}
		return out
	}

	prefix := bClean
	if len(prefix) > 5 {
		prefix = prefix[:5]
	}
	suffix := "~" + string(rune('0'+(index%9)+1))
	if index >= 9 {
		suffix = "~" + string(rune('A'+((index-9)%26)))
	}
	combined := prefix + suffix
	for i := 0; i < len(combined) && i < 8; i++ {
		out[i] = combined[i]
	}
	for i := 0; i < len(eClean) && i < 3; i++ {
		out[8+i] = eClean[i]
	}
	return out
}

func buildLFNEntries(longName string, shortName [11]byte) [][32]byte {
	chk := vfatChecksum(shortName)
	utf16Chars := make([]uint16, 0, len(longName)+1)
	for _, r := range longName {
		utf16Chars = append(utf16Chars, uint16(r))
	}
	utf16Chars = append(utf16Chars, 0x0000)
	for len(utf16Chars)%13 != 0 {
		utf16Chars = append(utf16Chars, 0xFFFF)
	}

	numSlots := len(utf16Chars) / 13
	entries := make([][32]byte, 0, numSlots)
	byteOffsets := []int{1, 3, 5, 7, 9, 14, 16, 18, 20, 22, 24, 28, 30}

	for slot := numSlots; slot >= 1; slot-- {
		var ent [32]byte
		seq := byte(slot)
		if slot == numSlots {
			seq |= 0x40 // Last LFN entry marker
		}
		ent[0] = seq
		ent[11] = 0x0F // ATTR_LONG_NAME
		ent[12] = 0x00
		ent[13] = chk
		ent[26] = 0x00
		ent[27] = 0x00

		chunk := utf16Chars[(slot-1)*13 : slot*13]
		for i, code := range chunk {
			off := byteOffsets[i]
			binary.LittleEndian.PutUint16(ent[off:off+2], code)
		}
		entries = append(entries, ent)
	}
	return entries
}

// BuildCidataFAT16Image creates a standard-compliant FAT16 (vfat) partition image
// labeled "CIDATA" containing all root cloud-init files (user-data, meta-data,
// instant-server-env.conf, laptop_hardening.sh, server_dashboard.html, etc.)
// so Ubuntu subiquity / cloud-init NoCloud discovers and mounts it automatically on boot.
func BuildCidataFAT16Image(stageDir string) ([]byte, error) {
	entries, err := os.ReadDir(stageDir)
	if err != nil {
		return nil, err
	}

	var files []fatStagedFile
	var totalFileClusters int
	bytesPerCluster := fatSectorsPerClust * fatBytesPerSector
	for _, ent := range entries {
		if ent.IsDir() || strings.HasSuffix(ent.Name(), ".img") || strings.HasSuffix(ent.Name(), ".iso") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(stageDir, ent.Name()))
		if err != nil {
			return nil, err
		}
		files = append(files, fatStagedFile{
			Name: ent.Name(),
			Data: raw,
		})
		c := (len(raw) + bytesPerCluster - 1) / bytesPerCluster
		if c == 0 {
			c = 1
		}
		totalFileClusters += c
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name < files[j].Name
	})

	// Dynamically size the FAT16 volume so it is always >= 4100 clusters (strict FAT16)
	// and never overflows even when large custom payloads are staged.
	reqClusters := totalFileClusters + 64
	if reqClusters < 4100 {
		reqClusters = 4100
	}
	if reqClusters > 65520 {
		reqClusters = 65520
	}
	sectorsPerFAT := ((reqClusters + 2) * 2 + fatBytesPerSector - 1) / fatBytesPerSector
	if sectorsPerFAT < fatSectorsPerTable {
		sectorsPerFAT = fatSectorsPerTable
	}
	dataStartSector := fatReservedSectors + (fatNumTables * sectorsPerFAT) + fatRootDirSectors
	totalSectors := dataStartSector + reqClusters*fatSectorsPerClust
	totalSectors = ((totalSectors + 7) / 8) * 8 // 4096-byte page aligned

	imgSize := totalSectors * fatBytesPerSector
	img := make([]byte, imgSize)

	// 1. Format Boot Sector (BIOS Parameter Block) at Sector 0
	img[0] = 0xEB
	img[1] = 0x3C
	img[2] = 0x90
	copy(img[3:11], []byte("MSDOS5.0"))
	binary.LittleEndian.PutUint16(img[11:13], uint16(fatBytesPerSector))
	img[13] = uint8(fatSectorsPerClust)
	binary.LittleEndian.PutUint16(img[14:16], uint16(fatReservedSectors))
	img[16] = uint8(fatNumTables)
	binary.LittleEndian.PutUint16(img[17:19], uint16(fatRootDirEntries))
	if totalSectors <= 0xFFFF {
		binary.LittleEndian.PutUint16(img[19:21], uint16(totalSectors))
		binary.LittleEndian.PutUint32(img[32:36], 0)
	} else {
		binary.LittleEndian.PutUint16(img[19:21], 0)
		binary.LittleEndian.PutUint32(img[32:36], uint32(totalSectors))
	}
	img[21] = 0xF8 // Fixed / removable media descriptor
	binary.LittleEndian.PutUint16(img[22:24], uint16(sectorsPerFAT))
	binary.LittleEndian.PutUint16(img[24:26], 63)  // Sectors per track
	binary.LittleEndian.PutUint16(img[26:28], 255) // Heads
	img[36] = 0x80                                 // Drive number
	img[38] = 0x29                                 // Extended boot signature
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
		}
	}
	writeFATEntry(0, 0xFFF8)
	writeFATEntry(1, 0xFFFF)

	// 3. Initialize Root Directory with Volume Label entry ("CIDATA     ", ATTR_VOLUME_ID = 0x08)
	rootDirOffset := (fatReservedSectors + fatNumTables*sectorsPerFAT) * fatBytesPerSector
	dirCursor := rootDirOffset
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
		remaining := len(f.Data)
		if remaining > 0 {
			startCluster = nextCluster
			numClusters := (remaining + bytesPerCluster - 1) / bytesPerCluster
			for c := 0; c < numClusters; c++ {
				curCluster := nextCluster
				nextCluster++
				if c == numClusters-1 {
					writeFATEntry(curCluster, 0xFFFF) // End of cluster chain
				} else {
					writeFATEntry(curCluster, nextCluster)
				}
				dataOffset := (dataStartSector*fatBytesPerSector) + int(curCluster-2)*bytesPerCluster
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

// FormatRemainingUSBAsFAT32 formats sectors [startLBA .. endLBA] on the USB drive
// as a cross-platform FAT32 partition labeled "INSTANTUSB" and pre-creates
// INSTANT-SERVER-INSTALL.log in its root directory.
func FormatRemainingUSBAsFAT32(disk RawDiskHandle, startLBA uint64, endLBA uint64) error {
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
		secPerClus = 1 // 512 B clusters for compact test images (< 64 MB)
	} else if totalSectors < 16777216 {
		secPerClus = 16 // 8 KB clusters (< 8 GB)
	}

	const reservedSectors uint32 = 32
	const numFATs uint32 = 2
	approxClusters := (totalSectors - reservedSectors) / secPerClus
	fatSectors := ((approxClusters + 2) * 4 + 511) / 512
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
		binary.LittleEndian.PutUint16(b[24:26], 63)
		binary.LittleEndian.PutUint16(b[26:28], 255)
		binary.LittleEndian.PutUint32(b[28:32], uint32(startLBA))
		binary.LittleEndian.PutUint32(b[32:36], totalSectors)
		binary.LittleEndian.PutUint32(b[36:40], fatSectors)
		binary.LittleEndian.PutUint16(b[40:42], 0)
		binary.LittleEndian.PutUint16(b[42:44], 0)
		binary.LittleEndian.PutUint32(b[44:48], 2)
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
		if approxClusters > 4 {
			freeClus = approxClusters - 4
		}
		binary.LittleEndian.PutUint32(fsi[488:492], freeClus)
		binary.LittleEndian.PutUint32(fsi[492:496], 4)
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
	binary.LittleEndian.PutUint32(fatBuf[0:4], 0x0FFFFFF8)
	binary.LittleEndian.PutUint32(fatBuf[4:8], 0x0FFFFFFF)
	binary.LittleEndian.PutUint32(fatBuf[8:12], 0x0FFFFFFF)
	binary.LittleEndian.PutUint32(fatBuf[12:16], 0x0FFFFFFF)

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
	twoClustersBuf := AllocPageAlignedBuffer(clusterBytes*2, SectorAlignmentBytes)
	for i := range twoClustersBuf {
		twoClustersBuf[i] = 0
	}

	copy(twoClustersBuf[0:11], []byte("INSTANTUSB "))
	twoClustersBuf[11] = 0x08

	logFileName := "INSTANT-SERVER-INSTALL.log"
	logContent := []byte("================================================================================\n INSTANT SERVER USB STORAGE & LIVE INSTALLATION LOG\n All boot, Subiquity, Curtin, Netplan, and setup logs are saved here automatically.\n================================================================================\n")
	shortName := makeShort83Name(logFileName, 1)
	lfnEntries := buildLFNEntries(logFileName, shortName)
	cursor := 32
	for _, lfn := range lfnEntries {
		copy(twoClustersBuf[cursor:cursor+32], lfn[:])
		cursor += 32
	}
	var stdEnt [32]byte
	copy(stdEnt[0:11], shortName[:])
	stdEnt[11] = 0x20
	binary.LittleEndian.PutUint16(stdEnt[20:22], 0)
	binary.LittleEndian.PutUint16(stdEnt[26:28], 3)
	binary.LittleEndian.PutUint32(stdEnt[28:32], uint32(len(logContent)))
	copy(twoClustersBuf[cursor:cursor+32], stdEnt[:])

	copy(twoClustersBuf[clusterBytes:clusterBytes+len(logContent)], logContent)

	return writeAlignedChunks(disk, twoClustersBuf, int64(dataRegionStartLBA*512))
}