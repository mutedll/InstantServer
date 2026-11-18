// ============================================================================
// FILE: usb/iso/cidata_fat.go
// PURPOSE: Pure-Go FAT16 ("CIDATA") and FAT32 ("INSTANTUSB") filesystem
//
//	builders with spec-compliant VFAT Long File Names, clean 8.3 short
//	names, "." / ".." directory entries, and clean shutdown bits.
//
// ============================================================================
package iso

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode/utf16"
	"unsafe"
)

const (
	fatBytesPerSector    = 512
	fatSectorsPerCluster = 4 // 2 KB per cluster for FAT16 CIDATA
	FatTotalSectors      = 32880
	fatTotalSectors      = FatTotalSectors // ~16.8 MB (> 4085 clusters required for FAT16)
	fatReservedSectors   = 4
	fatNumTables         = 2
	fatSectorsPerFAT     = 32
	fatRootDirEntries    = 512
	fatRootDirSectors    = (fatRootDirEntries * 32) / fatBytesPerSector

	dosDefaultDate = uint16(((2026 - 1980) << 9) | (11 << 5) | 16)
	dosDefaultTime = uint16(12 << 11) // 12:00:00
)

const (
	SectorAlignmentBytes = 4096
	DMAChunkSizeBytes    = 4 * 1024 * 1024
	LPTSectorSize        = 512
)

type RawDiskHandle interface {
	io.ReaderAt
	io.WriterAt
	io.ReadWriteSeeker
	io.Closer
	SyncAndFlush() error
	DevicePath() string
	RequiresPageAlignedTail() bool
}

type FlashProgressEvent struct {
	Stage        string  `json:"stage"`
	Percent      int     `json:"pct"`
	StagePercent float64 `json:"stage_pct"`
	WrittenBytes int64   `json:"written_bytes"`
	TotalBytes   int64   `json:"total_bytes"`
	SpeedMBps    float64 `json:"mb_s"`
	ETASeconds   float64 `json:"eta_sec"`
	CRC32Hex     string  `json:"crc32"`
	SHA256Hex    string  `json:"sha256,omitempty"`
	GrubPatches  int     `json:"grub_patches,omitempty"`
	ErrorCode    string  `json:"error_code,omitempty"`
	Message      string  `json:"message"`
}

type PauseGate struct {
	mu     sync.Mutex
	cond   *sync.Cond
	paused bool
}

func NewPauseGate() *PauseGate {
	pg := &PauseGate{}
	pg.cond = sync.NewCond(&pg.mu)
	return pg
}

func (pg *PauseGate) Pause() {
	pg.mu.Lock()
	pg.paused = true
	pg.mu.Unlock()
}

func (pg *PauseGate) Resume() {
	pg.mu.Lock()
	pg.paused = false
	pg.cond.Broadcast()
	pg.mu.Unlock()
}

func (pg *PauseGate) IsPaused() bool {
	pg.mu.Lock()
	defer pg.mu.Unlock()
	return pg.paused
}

func (pg *PauseGate) WaitIfPaused(ctx context.Context) error {
	pg.mu.Lock()
	defer pg.mu.Unlock()
	for pg.paused {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		pg.cond.Wait()
	}
	return ctx.Err()
}

func AllocPageAlignedBuffer(size int, align int) []byte {
	if align <= 0 {
		align = SectorAlignmentBytes
	}
	if size%align != 0 {
		size = ((size + align - 1) / align) * align
	}
	raw := make([]byte, size+align)
	baseAddr := uintptr(unsafe.Pointer(&raw[0]))
	remainder := baseAddr % uintptr(align)
	offset := 0
	if remainder != 0 {
		offset = align - int(remainder)
	}
	return raw[offset : offset+size]
}

func IsPointerPageAligned(buf []byte, align int) bool {
	if len(buf) == 0 {
		return false
	}
	return uintptr(unsafe.Pointer(&buf[0]))%uintptr(align) == 0
}

func WriteAlignedChunks(disk RawDiskHandle, data []byte, baseOffset int64) error {
	const chunkStep = 1024 * 1024
	chunkBuf := AllocPageAlignedBuffer(chunkStep, SectorAlignmentBytes)
	written := 0
	for written < len(data) {
		rem := len(data) - written
		n := rem
		if n > chunkStep {
			n = chunkStep
		}
		alignedN := ((n + SectorAlignmentBytes - 1) / SectorAlignmentBytes) * SectorAlignmentBytes
		for i := 0; i < alignedN; i++ {
			chunkBuf[i] = 0
		}
		copy(chunkBuf[:n], data[written:written+n])
		if _, err := disk.WriteAt(chunkBuf[:alignedN], baseOffset+int64(written)); err != nil {
			return err
		}
		written += n
	}
	return nil
}

type fatStagedFile struct {
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

// MakeClean83ShortName generates a spec-compliant DOS 8.3 short name.
// If the filename fits cleanly in 8.3 ASCII (e.g., "EFI", "BOOT", "GRUB.CFG",
// "VMLINUZ", "INITRD") and does not collide in `used`, it uses the exact 8.3
// name without appending "~1" so UEFI firmware and GRUB fat.mod resolve paths natively.
func MakeClean83ShortName(name string, index int, used map[[11]byte]bool) [11]byte {
	var out [11]byte
	for i := range out {
		out[i] = ' '
	}
	startsWithDot := strings.HasPrefix(name, ".")
	upper := strings.ToUpper(strings.Trim(name, ". "))
	base, ext := upper, ""
	hasExtraDots := false
	if dot := strings.LastIndex(upper, "."); dot > 0 {
		base, ext = upper[:dot], upper[dot+1:]
		if strings.Contains(base, ".") {
			hasExtraDots = true
		}
	}
	keepValid83 := func(s string) (string, bool) {
		var sb strings.Builder
		lossy := false
		for _, r := range s {
			if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
				sb.WriteRune(r)
			} else {
				lossy = true
			}
		}
		return sb.String(), lossy
	}
	cleanBase, lossyBase := keepValid83(base)
	cleanExt, lossyExt := keepValid83(ext)
	if cleanBase == "" {
		cleanBase = "FILE"
		lossyBase = true
	}

	needsTilde := startsWithDot || hasExtraDots || lossyBase || lossyExt || len(cleanBase) > 8 || len(cleanExt) > 3
	if len(cleanExt) > 3 {
		cleanExt = cleanExt[:3]
	}

	if !needsTilde {
		copy(out[0:8], []byte(cleanBase))
		copy(out[8:11], []byte(cleanExt))
		if used == nil || !used[out] {
			if used != nil {
				used[out] = true
			}
			return out
		}
	}

	for attempt := index; attempt < index+4096; attempt++ {
		var cand [11]byte
		for i := range cand {
			cand[i] = ' '
		}
		suffix := fmt.Sprintf("~%d", attempt)
		if attempt > 999 {
			suffix = fmt.Sprintf("~%03X", attempt&0xFFF)
		}
		b := cleanBase
		if maxBase := 8 - len(suffix); maxBase > 0 {
			if len(b) > maxBase {
				b = b[:maxBase]
			}
		} else {
			b = b[:1]
		}
		copy(cand[0:8], []byte(b+suffix))
		copy(cand[8:11], []byte(cleanExt))
		if used == nil || !used[cand] {
			if used != nil {
				used[cand] = true
			}
			return cand
		}
	}
	return out
}

// BuildLFNEntries builds 32-byte VFAT Long File Name directory entries in reverse sequence order.
func BuildLFNEntries(longName string, shortName [11]byte) [][32]byte {
	u16 := utf16.Encode([]rune(longName))
	if len(u16)%13 != 0 || len(u16) == 0 {
		u16 = append(u16, 0x0000)
		for len(u16)%13 != 0 {
			u16 = append(u16, 0xFFFF)
		}
	}
	numEntries := len(u16) / 13
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
		chunk := u16[(seq-1)*13 : seq*13]
		for i, codeUnit := range chunk {
			binary.LittleEndian.PutUint16(ent[offsets[i]:offsets[i]+2], codeUnit)
		}
		entries = append(entries, ent)
	}
	return entries
}

func StampDOSDirEntry(ent *[32]byte, attr byte, cluster uint32, size uint32) {
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

func MakeDotEntry(name11 string, cluster uint32) [32]byte {
	var ent [32]byte
	copy(ent[0:11], []byte(name11))
	StampDOSDirEntry(&ent, 0x10, cluster, 0)
	return ent
}

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
	img[37] = 0x00 // Clean shutdown bit
	img[38] = 0x29
	binary.LittleEndian.PutUint32(img[39:43], 0x20261108)
	copy(img[43:54], []byte("CIDATA     "))
	copy(img[54:62], []byte("FAT16   "))
	img[510], img[511] = 0x55, 0xAA

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

	rootDirOffset := (fatReservedSectors + fatNumTables*sectorsPerFAT) * fatBytesPerSector
	rootDirLimit := rootDirOffset + (fatRootDirSectors * fatBytesPerSector)
	dirCursor := rootDirOffset
	var volEnt [32]byte
	copy(volEnt[0:11], []byte("CIDATA     "))
	StampDOSDirEntry(&volEnt, 0x08, 0, 0)
	copy(img[dirCursor:dirCursor+32], volEnt[:])
	dirCursor += 32

	usedShort := make(map[[11]byte]bool)
	nextCluster := uint16(2)
	for idx, f := range files {
		shortName := MakeClean83ShortName(f.Name, idx+1, usedShort)
		for _, lfn := range BuildLFNEntries(f.Name, shortName) {
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
			StampDOSDirEntry(&stdEnt, 0x20, uint32(startCluster), uint32(len(f.Data)))
			copy(img[dirCursor:dirCursor+32], stdEnt[:])
			dirCursor += 32
		}
	}

	return img, nil
}

func BuildFAT16CidataVolume(stageDir string, _ string) ([]byte, error) {
	return BuildCidataFAT16Image(stageDir)
}
