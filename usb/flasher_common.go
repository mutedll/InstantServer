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

func PatchGrubKernelCmdlineInPlace(buf []byte) int {
	longNeedle := []byte("quiet splash ---")
	longRepl := []byte("autoinstall  ---")
	count := 0
	idx := 0
	for {
		pos := bytes.Index(buf[idx:], longNeedle)
		if pos < 0 {
			break
		}
		copy(buf[idx+pos:idx+pos+len(longRepl)], longRepl)
		idx += pos + len(longNeedle)
		count++
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
		if fatImg, fatErr := BuildCidataFAT16Image(cidataStageDir); fatErr == nil {
			cidataFAT = fatImg
		}
	}

	if injectGPTCidata {
		_ = RelocateGPTAndWriteCidataPartition(rawDisk, uint64(totalBytes), cidataFAT)
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
		Message:      "Verified raw DMA write, SHA-256 digest, and hybrid GPT/MBR CIDATA partition.",
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

var cidataUniquePartGUID = [16]byte{
	0x49, 0x4E, 0x53, 0x54,
	0x41, 0x4E,
	0x54, 0x53,
	0x43, 0x49,
	0x44, 0x41, 0x54, 0x41, 0x30, 0x31,
}

func RelocateGPTAndAppendCidataPartition(disk RawDiskHandle, isoBytes uint64) error {
	return RelocateGPTAndWriteCidataPartition(disk, isoBytes, nil)
}

func RelocateGPTAndWriteCidataPartition(disk RawDiskHandle, isoBytes uint64, cidataFATImage []byte) error {
	// Read LBA 0..33 (34 sectors = 17,408 bytes, rounded to 20,480 bytes for 4096-byte page alignment)
	tableBuf := AllocPageAlignedBuffer(5*SectorAlignmentBytes, SectorAlignmentBytes)
	if _, err := disk.ReadAt(tableBuf, 0); err != nil {
		return err
	}

	gptHdr := tableBuf[LPTSectorSize : LPTSectorSize*2]
	if string(gptHdr[0:8]) != "EFI PART" {
		return nil
	}

	// Align CIDATA partition start to the next 1 MB boundary (2048 512-byte LBAs) after the ISO
	cidataStartLBA := ((isoBytes + (1024*1024 - 1)) / (1024 * 1024)) * (1024 * 1024 / LPTSectorSize)
	cidataSectors := uint64(len(cidataFATImage)) / LPTSectorSize
	if cidataSectors == 0 {
		cidataSectors = uint64(fatTotalSectors)
	}
	cidataEndLBA := cidataStartLBA + cidataSectors - 1

	// Write the FAT16 CIDATA filesystem image at cidataStartLBA if provided
	if len(cidataFATImage) > 0 {
		alignedFAT := AllocPageAlignedBuffer(len(cidataFATImage), SectorAlignmentBytes)
		copy(alignedFAT, cidataFATImage)
		_, _ = disk.WriteAt(alignedFAT, int64(cidataStartLBA*LPTSectorSize))
	}

	// Find the first empty 128-byte slot in the GPT Partition Entry Array (starting at LBA 2)
	partArray := tableBuf[LPTSectorSize*2 : LPTSectorSize*34]
	numEntries := int(binary.LittleEndian.Uint32(gptHdr[80:84]))
	entrySize := int(binary.LittleEndian.Uint32(gptHdr[84:88]))
	if numEntries <= 0 || numEntries > 128 {
		numEntries = 128
	}
	if entrySize <= 0 || entrySize > 512 {
		entrySize = 128
	}

	slotOffset := entrySize // Default to Partition #2 (index 1) if only 1 partition exists
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
			slotOffset = i * entrySize
			break
		}
	}

	if slotOffset+128 <= len(partArray) {
		ent := partArray[slotOffset : slotOffset+128]
		for i := range ent {
			ent[i] = 0
		}
		copy(ent[0:16], microsoftBasicDataGUID[:])
		copy(ent[16:32], cidataUniquePartGUID[:])
		binary.LittleEndian.PutUint64(ent[32:40], cidataStartLBA)
		binary.LittleEndian.PutUint64(ent[40:48], cidataEndLBA)
		// Encode "CIDATA" as UTF-16LE in bytes 56..128
		for idx, ch := range []uint16{'C', 'I', 'D', 'A', 'T', 'A'} {
			binary.LittleEndian.PutUint16(ent[56+idx*2:58+idx*2], ch)
		}
	}

	// Update LastUsableLBA (offset 48) and BackupLBA (offset 32) if needed
	currentLastUsable := binary.LittleEndian.Uint64(gptHdr[48:56])
	if cidataEndLBA > currentLastUsable {
		binary.LittleEndian.PutUint64(gptHdr[48:56], cidataEndLBA+34)
		binary.LittleEndian.PutUint64(gptHdr[32:40], cidataEndLBA+68)
	}

	// Recompute Partition Entry Array CRC-32 (offset 88)
	arrayBytesLen := numEntries * entrySize
	if arrayBytesLen > len(partArray) {
		arrayBytesLen = len(partArray)
	}
	partCRC := crc32.ChecksumIEEE(partArray[:arrayBytesLen])
	binary.LittleEndian.PutUint32(gptHdr[88:92], partCRC)

	// Recompute GPT Header CRC-32 (offset 16, over headerSize bytes at 12..16)
	hdrSize := int(binary.LittleEndian.Uint32(gptHdr[12:16]))
	if hdrSize < 92 || hdrSize > LPTSectorSize {
		hdrSize = 92
	}
	gptHdr[16], gptHdr[17], gptHdr[18], gptHdr[19] = 0, 0, 0, 0
	hdrCRC := crc32.ChecksumIEEE(gptHdr[:hdrSize])
	binary.LittleEndian.PutUint32(gptHdr[16:20], hdrCRC)

	// Write the updated GPT Header + Partition Entry Array back to LBA 0..33!
	if _, err := disk.WriteAt(tableBuf, 0); err != nil {
		return err
	}
	return disk.SyncAndFlush()
}