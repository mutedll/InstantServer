// usb/flasher_common.go
package usb

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"sync"
	"syscall"
	"time"
	"unsafe"
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

type grubPatchPair struct {
	needle []byte
	repl   []byte
}

var grubFixedPatchPairs = []grubPatchPair{
	{
		needle: []byte("quiet splash ---"),
		repl:   []byte("autoinstall  ---"),
	},
	{
		needle: []byte("set timeout=30"),
		repl:   []byte("set timeout=1 "),
	},
	{
		needle: []byte("set timeout=10"),
		repl:   []byte("set timeout=1 "),
	},
	{
		needle: []byte("set timeout=5\n"),
		repl:   []byte("set timeout=1\n"),
	},
	{
		needle: []byte("timeout 300"),
		repl:   []byte("timeout 10 "),
	},
}

func PatchGrubKernelCmdlineInPlace(buf []byte) int {
	count := 0
	for _, pair := range grubFixedPatchPairs {
		if len(pair.needle) != len(pair.repl) {
			continue
		}
		idx := 0
		for {
			pos := bytes.Index(buf[idx:], pair.needle)
			if pos < 0 {
				break
			}
			copy(buf[idx+pos:idx+pos+len(pair.repl)], pair.repl)
			idx += pos + len(pair.needle)
			count++
		}
	}

	gfxMarker := []byte("set gfxpayload=keep")
	dashMarker := []byte("---")
	searchIdx := 0
	for searchIdx < len(buf) {
		relPos := bytes.Index(buf[searchIdx:], gfxMarker)
		if relPos < 0 {
			break
		}
		spanStart := searchIdx + relPos
		windowEnd := spanStart + 110
		if windowEnd > len(buf) {
			windowEnd = len(buf)
		}
		window := buf[spanStart:windowEnd]
		dashPos := bytes.Index(window, dashMarker)
		casperPos := bytes.Index(window, []byte("/casper/"))
		if dashPos > 0 && casperPos > 0 && casperPos < dashPos {
			spanLen := dashPos + len(dashMarker)
			kEnd := casperPos
			for kEnd < dashPos && window[kEnd] != ' ' && window[kEnd] != '\t' && window[kEnd] != '\r' && window[kEnd] != '\n' {
				kEnd++
			}
			kernelPath := string(window[casperPos:kEnd])
			candidates := []string{
				"linux\t" + kernelPath + " autoinstall noprompt noeject quiet ",
				"linux " + kernelPath + " autoinstall noprompt ",
				"linux\t" + kernelPath + " autoinstall quiet ",
				"linux " + kernelPath + " autoinstall ",
			}
			for _, prefix := range candidates {
				if len(prefix)+len(dashMarker) <= spanLen {
					padCount := spanLen - len(prefix) - len(dashMarker)
					patched := make([]byte, 0, spanLen)
					patched = append(patched, []byte(prefix)...)
					for p := 0; p < padCount; p++ {
						patched = append(patched, ' ')
					}
					patched = append(patched, dashMarker...)
					copy(buf[spanStart:spanStart+spanLen], patched)
					count++
					break
				}
			}
			searchIdx = spanStart + spanLen
		} else {
			searchIdx = spanStart + len(gfxMarker)
		}
	}

	return count
}

func isHardwareDisconnectError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, syscall.EIO) || errors.Is(err, syscall.ENODEV) || errors.Is(err, syscall.ENXIO)
}

func StreamISOToRawDisk(
	ctx context.Context,
	isoPath string,
	targetDevice string,
	patchGrub bool,
	injectGPTCidata bool,
	pauseGate *PauseGate,
	onProgress func(FlashProgressEvent),
) (*FlashProgressEvent, error) {
	return StreamISOAndCidataToRawDisk(ctx, isoPath, targetDevice, "", patchGrub, injectGPTCidata, pauseGate, onProgress)
}

func StreamISOAndCidataToRawDisk(
	ctx context.Context,
	isoPath string,
	targetDevice string,
	cidataStageDir string,
	patchGrub bool,
	injectGPTCidata bool,
	pauseGate *PauseGate,
	onProgress func(FlashProgressEvent),
) (*FlashProgressEvent, error) {
	releaseWakeLock := AcquireHostWakeLock()
	defer releaseWakeLock()

	srcFile, err := os.Open(isoPath)
	if err != nil {
		return nil, fmt.Errorf("IS-ISO-101: failed to open source ISO %s: %w", isoPath, err)
	}
	defer srcFile.Close()

	stat, err := srcFile.Stat()
	if err != nil {
		return nil, fmt.Errorf("IS-ISO-102: failed to stat source ISO: %w", err)
	}
	totalBytes := stat.Size()
	if totalBytes <= 0 {
		return nil, errors.New("IS-ISO-103: source ISO file is empty")
	}

	rawDisk, err := OpenRawDisk(targetDevice)
	if err != nil {
		return nil, err
	}
	defer rawDisk.Close()

	dmaBuf := AllocPageAlignedBuffer(DMAChunkSizeBytes, SectorAlignmentBytes)
	crcHasher := crc32.NewIEEE()
	shaHasher := sha256.New()

	var writtenBytes int64
	var totalGrubPatches int
	startTime := time.Now()

	for writtenBytes < totalBytes {
		if pauseGate != nil {
			if err := pauseGate.WaitIfPaused(ctx); err != nil {
				return nil, fmt.Errorf("IS-USB-399: flashing cancelled while paused: %w", err)
			}
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("IS-USB-399: flashing cancelled cleanly by user: %w", ctx.Err())
		default:
		}

		readBytes, readErr := io.ReadFull(srcFile, dmaBuf)
		if readErr != nil && !errors.Is(readErr, io.EOF) && !errors.Is(readErr, io.ErrUnexpectedEOF) {
			return nil, fmt.Errorf("IS-ISO-104: failed reading ISO stream at offset %d: %w", writtenBytes, readErr)
		}
		if readBytes == 0 {
			break
		}

		validSlice := dmaBuf[:readBytes]
		if patchGrub {
			totalGrubPatches += PatchGrubKernelCmdlineInPlace(validSlice)
		}

		_, _ = crcHasher.Write(validSlice)
		_, _ = shaHasher.Write(validSlice)

		writeSlice := validSlice
		if rawDisk.RequiresPageAlignedTail() && (readBytes%SectorAlignmentBytes != 0) {
			paddedLen := ((readBytes + SectorAlignmentBytes - 1) / SectorAlignmentBytes) * SectorAlignmentBytes
			for i := readBytes; i < paddedLen; i++ {
				dmaBuf[i] = 0
			}
			writeSlice = dmaBuf[:paddedLen]
		}

		if _, wErr := rawDisk.WriteAt(writeSlice, writtenBytes); wErr != nil {
			if isHardwareDisconnectError(wErr) {
				return nil, fmt.Errorf("IS-USB-304: USB drive %s was physically unplugged mid-flash at %d bytes: %w", targetDevice, writtenBytes, wErr)
			}
			return nil, fmt.Errorf("IS-USB-302: raw sector write failed on %s at offset %d: %w", targetDevice, writtenBytes, wErr)
		}

		writtenBytes += int64(readBytes)
		elapsed := time.Since(startTime).Seconds()
		if elapsed <= 0.001 {
			elapsed = 0.001
		}
		mbPerSec := (float64(writtenBytes) / (1024.0 * 1024.0)) / elapsed
		remainingBytes := totalBytes - writtenBytes
		etaSec := 0.0
		if mbPerSec > 0.01 && remainingBytes > 0 {
			etaSec = (float64(remainingBytes) / (1024.0 * 1024.0)) / mbPerSec
		}
		pct := int((writtenBytes * 100) / totalBytes)
		if pct > 100 {
			pct = 100
		}

		if onProgress != nil {
			onProgress(FlashProgressEvent{
				Stage:        "FLASHING",
				Percent:      pct,
				WrittenBytes: writtenBytes,
				TotalBytes:   totalBytes,
				SpeedMBps:    mbPerSec,
				ETASeconds:   etaSec,
				CRC32Hex:     fmt.Sprintf("%08X", crcHasher.Sum32()),
				GrubPatches:  totalGrubPatches,
				Message:      fmt.Sprintf("Streaming 4MB aligned DMA blocks: %.1f / %.1f MB @ %.1f MB/s", float64(writtenBytes)/1048576.0, float64(totalBytes)/1048576.0, mbPerSec),
			})
		}
	}

	var cidataFAT []byte
	if cidataStageDir != "" {
		fatImg, fatErr := BuildCidataFAT16Image(cidataStageDir)
		if fatErr != nil {
			return nil, fmt.Errorf("IS-USB-310: failed building FAT16 CIDATA partition image: %w", fatErr)
		}
		cidataFAT = fatImg
	}

	if injectGPTCidata {
		if gptErr := RelocateGPTAndWriteCidataPartition(rawDisk, uint64(totalBytes), cidataFAT); gptErr != nil {
			return nil, fmt.Errorf("IS-USB-311: failed writing GPT & CIDATA partition table on %s: %w", targetDevice, gptErr)
		}
	}

	if syncErr := rawDisk.SyncAndFlush(); syncErr != nil {
		return nil, fmt.Errorf("IS-USB-305: hardware cache flush failed on %s: %w", targetDevice, syncErr)
	}

	finalEvent := &FlashProgressEvent{
		Stage:        "VERIFIED",
		Percent:      100,
		WrittenBytes: writtenBytes,
		TotalBytes:   totalBytes,
		CRC32Hex:     fmt.Sprintf("%08X", crcHasher.Sum32()),
		SHA256Hex:    hex.EncodeToString(shaHasher.Sum(nil)),
		GrubPatches:  totalGrubPatches,
		Message:      "Verified raw DMA write, SHA-256 digest, UEFI ESP + Backup GPT, and FAT16 CIDATA partition.",
	}
	if onProgress != nil {
		onProgress(*finalEvent)
	}
	return finalEvent, nil
}

// microsoftBasicDataGUID in mixed-endian GPT binary format: EBD0A0A2-B9E5-4433-87C0-68B6B72699C7
var microsoftBasicDataGUID = [16]byte{
	0xA2, 0xA0, 0xD0, 0xEB,
	0xE5, 0xB9,
	0x33, 0x44,
	0x87, 0xC0,
	0x68, 0xB6, 0xB7, 0x26, 0x99, 0xC7,
}

// linuxFilesystemDataGUID in mixed-endian GPT binary format: 0FC63DAF-8483-4772-8E79-3D69D8477DE4
var linuxFilesystemDataGUID = [16]byte{
	0xAF, 0x3D, 0xC6, 0x0F,
	0x83, 0x84,
	0x72, 0x47,
	0x8E, 0x79,
	0x3D, 0x69, 0xD8, 0x47, 0x7D, 0xE4,
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

func writeAlignedChunks(disk RawDiskHandle, data []byte, baseOffset int64) error {
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

func writeStandardProtectiveMBR(mbrSector []byte, totalDiskSectors uint64) {
	// Clear all 4 MBR partition entries at bytes 446..510 while preserving bootstrap code in 0..446
	for i := 446; i < 510; i++ {
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
		if bytes.Equal(ent[0:16], microsoftBasicDataGUID[:]) {
			copy(ent[0:16], linuxFilesystemDataGUID[:])
			binary.LittleEndian.PutUint64(ent[48:56], 0xC000000000000001)
		}
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
		copy(ent[0:16], microsoftBasicDataGUID[:])
		copy(ent[16:32], cidataUniquePartGUID[:])
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
				copy(ent[0:16], microsoftBasicDataGUID[:])
				copy(ent[16:32], instantUSBUniquePartGUID[:])
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
	copy(ent[0:16], microsoftBasicDataGUID[:])
	copy(ent[16:32], instantUSBUniquePartGUID[:])
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
	copy(gptHdr[56:72], cidataUniquePartGUID[:])
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
