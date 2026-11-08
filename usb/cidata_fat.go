// usb/cidata_fat.go
package usb

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	fatBytesPerSector    = 512
	fatSectorsPerCluster = 8 // 4096 bytes per cluster
	fatBytesPerCluster   = fatBytesPerSector * fatSectorsPerCluster
	fatReservedSectors   = 1
	fatNumTables         = 2
	fatRootDirEntries    = 512
	fatRootDirSectors    = (fatRootDirEntries * 32) / fatBytesPerSector // 32 sectors
	fatTotalClusters     = 4100                                         // > 4085 clusters guarantees strict FAT16 detection in Linux vfat/blkid
	fatSectorsPerTable   = 32                                           // 32 * 512 = 16,384 bytes (8,192 FAT16 entries)
	fatDataStartSector   = fatReservedSectors + (fatNumTables * fatSectorsPerTable) + fatRootDirSectors
	fatTotalSectors      = fatDataStartSector + (fatTotalClusters * fatSectorsPerCluster)
	fatTotalImageBytes   = fatTotalSectors * fatBytesPerSector // ~16.8 MB default FAT16 volume
)

type fatStagedFile struct {
	RelName string
	Data    []byte
}

func computeVFATChecksum(shortName [11]byte) byte {
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
	if dot := strings.LastIndexByte(upper, '.'); dot > 0 {
		base = upper[:dot]
		ext = upper[dot+1:]
	}
	cleanChars := func(s string) string {
		var b strings.Builder
		for _, r := range s {
			if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
				b.WriteRune(r)
			}
		}
		return b.String()
	}
	cBase := cleanChars(base)
	cExt := cleanChars(ext)
	if cBase == "" {
		cBase = "FILE"
	}
	if name == strings.ToLower(name) && len(base) <= 8 && len(ext) <= 3 && !strings.Contains(name, "-") && len(cBase) == len(base) && len(cExt) == len(ext) {
		for i := 0; i < len(cBase) && i < 8; i++ {
			out[i] = cBase[i]
		}
	} else {
		suffix := fmt.Sprintf("~%d", index)
		maxPrefix := 8 - len(suffix)
		if len(cBase) < maxPrefix {
			maxPrefix = len(cBase)
		}
		for i := 0; i < maxPrefix; i++ {
			out[i] = cBase[i]
		}
		for i := 0; i < len(suffix); i++ {
			out[maxPrefix+i] = suffix[i]
		}
	}
	for i := 0; i < len(cExt) && i < 3; i++ {
		out[8+i] = cExt[i]
	}
	return out
}

func buildLFNEntries(longName string, shortName [11]byte) [][32]byte {
	chk := computeVFATChecksum(shortName)
	utf16Chars := make([]uint16, 0, len(longName)+1)
	for _, r := range longName {
		utf16Chars = append(utf16Chars, uint16(r))
	}
	utf16Chars = append(utf16Chars, 0x0000)
	for len(utf16Chars)%13 != 0 {
		utf16Chars = append(utf16Chars, 0xFFFF)
	}

	numChunks := len(utf16Chars) / 13
	entries := make([][32]byte, 0, numChunks)
	offsets := [13]int{1, 3, 5, 7, 9, 14, 16, 18, 20, 22, 24, 28, 30}

	for seq := numChunks; seq >= 1; seq-- {
		var ent [32]byte
		ord := byte(seq)
		if seq == numChunks {
			ord |= 0x40
		}
		ent[0] = ord
		ent[11] = 0x0F // ATTR_LONG_NAME
		ent[12] = 0x00
		ent[13] = chk
		ent[26] = 0x00
		ent[27] = 0x00
		chunk := utf16Chars[(seq-1)*13 : seq*13]
		for j := 0; j < 13; j++ {
			binary.LittleEndian.PutUint16(ent[offsets[j]:offsets[j]+2], chunk[j])
		}
		entries = append(entries, ent)
	}
	return entries
}

func shouldSkipNonCidataFile(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	for _, ext := range []string{".iso", ".img", ".dmg", ".part", ".exe", ".bin"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
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
	totalRequiredClusters := 0
	for _, ent := range entries {
		if ent.IsDir() || shouldSkipNonCidataFile(ent.Name()) {
			continue
		}
		raw, rErr := os.ReadFile(filepath.Join(stageDir, ent.Name()))
		if rErr == nil {
			files = append(files, fatStagedFile{RelName: ent.Name(), Data: raw})
			if len(raw) > 0 {
				totalRequiredClusters += (len(raw) + fatBytesPerCluster - 1) / fatBytesPerCluster
			}
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].RelName < files[j].RelName })

	// Dynamically size the FAT16 volume so it is always >= 4100 clusters (strict FAT16)
	// and never overflows even when large custom payloads are staged.
	totalClusters := fatTotalClusters
	if totalRequiredClusters+64 > totalClusters {
		totalClusters = totalRequiredClusters + 64
	}
	if totalClusters > 65520 {
		totalClusters = 65520
	}
	sectorsPerTable := ((totalClusters+2)*2 + fatBytesPerSector - 1) / fatBytesPerSector
	if sectorsPerTable < fatSectorsPerTable {
		sectorsPerTable = fatSectorsPerTable
	}
	dataStartSector := fatReservedSectors + (fatNumTables * sectorsPerTable) + fatRootDirSectors
	totalSectors := dataStartSector + (totalClusters * fatSectorsPerCluster)
	totalImageBytes := totalSectors * fatBytesPerSector

	img := make([]byte, totalImageBytes)

	// 1. Format Boot Sector (BIOS Parameter Block) at Sector 0
	boot := img[0:fatBytesPerSector]
	boot[0], boot[1], boot[2] = 0xEB, 0x3C, 0x90
	copy(boot[3:11], []byte("MSDOS5.0"))
	binary.LittleEndian.PutUint16(boot[11:13], fatBytesPerSector)
	boot[13] = fatSectorsPerCluster
	binary.LittleEndian.PutUint16(boot[14:16], fatReservedSectors)
	boot[16] = fatNumTables
	binary.LittleEndian.PutUint16(boot[17:19], fatRootDirEntries)
	binary.LittleEndian.PutUint16(boot[19:21], 0) // TotalSectors16 = 0 (use TotalSectors32)
	boot[21] = 0xF8                               // Fixed/removable media descriptor
	binary.LittleEndian.PutUint16(boot[22:24], uint16(sectorsPerTable))
	binary.LittleEndian.PutUint16(boot[24:26], 63)  // Sectors per track
	binary.LittleEndian.PutUint16(boot[26:28], 255) // Heads
	binary.LittleEndian.PutUint32(boot[28:32], 0)   // Hidden sectors
	binary.LittleEndian.PutUint32(boot[32:36], uint32(totalSectors))
	boot[36] = 0x80 // Drive number
	boot[38] = 0x29 // Extended boot signature
	binary.LittleEndian.PutUint32(boot[39:43], 0x20261108)
	copy(boot[43:54], []byte("CIDATA     "))
	copy(boot[54:62], []byte("FAT16   "))
	boot[510] = 0x55
	boot[511] = 0xAA

	// 2. Initialize FAT #1 and FAT #2 reserved cluster entries 0 and 1
	fat1Offset := fatReservedSectors * fatBytesPerSector
	fat2Offset := (fatReservedSectors + sectorsPerTable) * fatBytesPerSector
	setFATEntry := func(cluster uint16, val uint16) {
		pos := int(cluster) * 2
		if fat1Offset+pos+2 <= len(img) && fat2Offset+pos+2 <= len(img) {
			binary.LittleEndian.PutUint16(img[fat1Offset+pos:fat1Offset+pos+2], val)
			binary.LittleEndian.PutUint16(img[fat2Offset+pos:fat2Offset+pos+2], val)
		}
	}
	setFATEntry(0, 0xFFF8)
	setFATEntry(1, 0xFFFF)

	// 3. Initialize Root Directory with Volume Label entry ("CIDATA     ", ATTR_VOLUME_ID = 0x08)
	rootDirOffset := (fatReservedSectors + fatNumTables*sectorsPerTable) * fatBytesPerSector
	dirCursor := rootDirOffset
	writeDirEntry := func(entry [32]byte) {
		if dirCursor+32 <= rootDirOffset+(fatRootDirSectors*fatBytesPerSector) {
			copy(img[dirCursor:dirCursor+32], entry[:])
			dirCursor += 32
		}
	}

	var volEntry [32]byte
	copy(volEntry[0:11], []byte("CIDATA     "))
	volEntry[11] = 0x08 // ATTR_VOLUME_ID
	writeDirEntry(volEntry)

	// 4. Write staged files into FAT16 data clusters
	nextCluster := uint16(2)
	for idx, f := range files {
		shortName := makeShort83Name(f.RelName, idx+1)
		for _, lfn := range buildLFNEntries(f.RelName, shortName) {
			writeDirEntry(lfn)
		}

		firstCluster := uint16(0)
		remaining := len(f.Data)
		if remaining > 0 {
			firstCluster = nextCluster
			numClusters := (remaining + fatBytesPerCluster - 1) / fatBytesPerCluster
			for c := 0; c < numClusters; c++ {
				curCluster := nextCluster
				nextCluster++
				if c == numClusters-1 {
					setFATEntry(curCluster, 0xFFFF)
				} else {
					setFATEntry(curCluster, curCluster+1)
				}
				dataOffset := (dataStartSector * fatBytesPerSector) + int(curCluster-2)*fatBytesPerCluster
				startByte := c * fatBytesPerCluster
				endByte := startByte + fatBytesPerCluster
				if endByte > len(f.Data) {
					endByte = len(f.Data)
				}
				chunkLen := endByte - startByte
				if dataOffset >= 0 && dataOffset+chunkLen <= len(img) {
					copy(img[dataOffset:dataOffset+chunkLen], f.Data[startByte:endByte])
				}
			}
		}

		var stdEnt [32]byte
		copy(stdEnt[0:11], shortName[:])
		stdEnt[11] = 0x20 // ATTR_ARCHIVE
		binary.LittleEndian.PutUint16(stdEnt[26:28], firstCluster)
		binary.LittleEndian.PutUint32(stdEnt[28:32], uint32(len(f.Data)))
		writeDirEntry(stdEnt)
	}

	return img, nil
}
