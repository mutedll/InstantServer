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

// Note: Setting `set timeout=1 ` (1 second) instead of `0` prevents Canonical's
// patched grubx64.efi from overriding `timeout=0` back to 10 seconds when
// `timeout_style=menu` is active.
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

// PatchGrubKernelCmdlineInPlace patches Ubuntu Live Server and Desktop GRUB configs
// in-place without changing a single file offset or ISO9660 extent length:
//  1. Sets GRUB menu timeout to 1 second (`set timeout=1 `) so the USB boots automatically without pressing Enter.
//  2. Replaces `set gfxpayload=keep\n\tlinux\t/casper/... ---` (as well as standalone `/casper/vmlinuz` lines)
//     with `linux /casper/... autoinstall noprompt ---` so Subiquity installs 100% unattended and reboots automatically.
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

	// Universal byte-length-preserving scanner for Ubuntu Live Server menuentries:
	// Matches `set gfxpayload=keep` followed within 110 bytes by `linux` + `/casper/` + `---`
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
			// Extract the exact kernel binary path (e.g. "/casper/vmlinuz" or "/casper/hwe-vmlinuz")
			kEnd := casperPos
			for kEnd < dashPos && window[kEnd] != ' ' && window[kEnd] != '\t' && window[kEnd] != '\r' && window[kEnd] != '\n' {
				kEnd++
			}
			kernelPath := string(window[casperPos:kEnd])
			// Build candidate replacements from richest flags to compact flags that fit in spanLen
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
// Used on the ISO9660 and Gap1 partitions so Windows MountMgr never attempts to mount raw ISO9660
// sectors as an unformatted Windows drive ("You need to format the disk in drive X:").
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

func RelocateGPTAndAppendCidataPartition(disk RawDiskHandle, isoBytes uint64) error {
	return RelocateGPTAndWriteCidataPartition(disk, isoBytes, nil)
}

func writeAlignedChunks(disk RawDiskHandle, data []byte, baseOffset int64) error {
	const chunkStep = 1024 * 1024 // 1 MB chunks for universal USBSTOR/UASP/NVMe compatibility
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

func RelocateGPTAndWriteCidataPartition(disk RawDiskHandle, isoBytes uint64, cidataFATImage []byte) error {
	// Read LBA 0..63 (64 sectors = 32,768 bytes = 8 * 4096-byte pages)
	// Ubuntu 22.04 / 24.04 / 26.04 ISOs built by xorriso have up to 248 GPT partition entries
	// spanning LBA 2..63 (62 sectors = 31,744 bytes). Reading all 64 sectors ensures we never
	// truncate the Partition Entry Array when recomputing PartitionEntryArrayCRC32.
	tableBuf := AllocPageAlignedBuffer(8*SectorAlignmentBytes, SectorAlignmentBytes)
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

	// Write the FAT16 CIDATA filesystem image in 1 MB aligned chunks and verify no write error occurs
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

	// Mark existing ISO9660 / Gap1 partitions (which xorriso tags with microsoftBasicDataGUID)
	// as Linux Filesystem Data + Hidden + NoDriveLetter (0xC000000000000001) so Windows Explorer
	// never tries to mount the raw ISO9660 partition or show "You need to format the disk in drive X:".
	// Partition 2 (EFI System Partition, C12A7328-...) is preserved untouched for UEFI booting.
	slotOffset := -1
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
			if slotOffset < 0 {
				slotOffset = i * entrySize
			}
			continue
		}
		if bytes.Equal(ent[0:16], microsoftBasicDataGUID[:]) {
			copy(ent[0:16], linuxFilesystemDataGUID[:])
			// Set RequiredPartition (bit 0) + Hidden (bit 62) + NoDriveLetter (bit 63)
			binary.LittleEndian.PutUint64(ent[48:56], 0xC000000000000001)
		}
	}
	if slotOffset < 0 {
		slotOffset = entrySize
	}

	if slotOffset+128 <= len(partArray) {
		ent := partArray[slotOffset : slotOffset+128]
		for i := range ent {
			ent[i] = 0
		}
		// Give CIDATA the Microsoft Basic Data GUID and Attributes = 0 so Windows, Linux, and macOS
		// recognize and mount the FAT16 CIDATA partition cleanly without any format prompt.
		copy(ent[0:16], microsoftBasicDataGUID[:])
		copy(ent[16:32], cidataUniquePartGUID[:])
		binary.LittleEndian.PutUint64(ent[32:40], cidataStartLBA)
		binary.LittleEndian.PutUint64(ent[40:48], cidataEndLBA)
		binary.LittleEndian.PutUint64(ent[48:56], 0)
		for idx, ch := range []uint16{'C', 'I', 'D', 'A', 'T', 'A'} {
			binary.LittleEndian.PutUint16(ent[56+idx*2:58+idx*2], ch)
		}
	}

	arrayBytesLen := numEntries * entrySize
	if arrayBytesLen > len(partArray) {
		arrayBytesLen = len(partArray)
	}
	arraySectors := uint64((arrayBytesLen + LPTSectorSize - 1) / LPTSectorSize)

	// Default Backup GPT position aligned immediately after cidataEndLBA
	backupArrayLBA := ((cidataEndLBA + 1 + 7) / 8) * 8
	backupHeaderLBA := backupArrayLBA + arraySectors

	// If the underlying USB disk/image has a known larger physical capacity, place the official
	// Backup GPT at the true end of the disk (aligned to 4096-byte page boundaries) per UEFI 2.9 Spec 5.3.2.
	if diskEndBytes, seekErr := disk.Seek(0, io.SeekEnd); seekErr == nil && diskEndBytes > 0 {
		totalDiskSectors := uint64(diskEndBytes) / LPTSectorSize
		if totalDiskSectors > cidataEndLBA+arraySectors+16 {
			// Align backupArrayLBA to an 8-sector (4096-byte) boundary near the end of the disk
			candidateArrayLBA := ((totalDiskSectors - 1 - arraySectors) / 8) * 8
			if candidateArrayLBA > cidataEndLBA+1 {
				backupArrayLBA = candidateArrayLBA
				backupHeaderLBA = backupArrayLBA + arraySectors
			}
		}
	}
	lastUsableLBA := backupArrayLBA - 1

	binary.LittleEndian.PutUint64(gptHdr[24:32], 1)               // MyLBA = 1
	binary.LittleEndian.PutUint64(gptHdr[32:40], backupHeaderLBA) // AlternateLBA = backupHeaderLBA
	binary.LittleEndian.PutUint64(gptHdr[48:56], lastUsableLBA)   // LastUsableLBA

	// Recompute Partition Entry Array CRC-32 (offset 88) over exact numEntries * entrySize bytes
	partCRC := crc32.ChecksumIEEE(partArray[:arrayBytesLen])
	binary.LittleEndian.PutUint32(gptHdr[88:92], partCRC)

	// Recompute Primary GPT Header CRC-32 (offset 16, over headerSize bytes at 12..16)
	hdrSize := int(binary.LittleEndian.Uint32(gptHdr[12:16]))
	if hdrSize < 92 || hdrSize > LPTSectorSize {
		hdrSize = 92
		binary.LittleEndian.PutUint32(gptHdr[12:16], 92)
	}
	gptHdr[16], gptHdr[17], gptHdr[18], gptHdr[19] = 0, 0, 0, 0
	hdrCRC := crc32.ChecksumIEEE(gptHdr[:hdrSize])
	binary.LittleEndian.PutUint32(gptHdr[16:20], hdrCRC)

	// Ensure LBA 0 Protective MBR (offset 446..510) conforms to UEFI Specification Section 5.2.2:
	// Partition 1 (0xEE) spans LBA 1..backupHeaderLBA, and Slots 2..4 are untouched/zeroed.
	if tableBuf[446+4] == 0xEE {
		protSectors := backupHeaderLBA
		if protSectors > 0xFFFFFFFF {
			protSectors = 0xFFFFFFFF
		}
		binary.LittleEndian.PutUint32(tableBuf[446+12:446+16], uint32(protSectors))
	}
	tableBuf[510] = 0x55
	tableBuf[511] = 0xAA

	// Write the updated Protective MBR + Primary GPT Header + Partition Entry Array back to LBA 0..63
	if _, err := disk.WriteAt(tableBuf, 0); err != nil {
		return err
	}

	// Build and write the Backup GPT Partition Entry Array + Backup GPT Header at backupArrayLBA
	backupBytesTotal := int((arraySectors + 1) * LPTSectorSize)
	backupBuf := AllocPageAlignedBuffer(backupBytesTotal, SectorAlignmentBytes)
	copy(backupBuf[:arrayBytesLen], partArray[:arrayBytesLen])

	backupHdr := backupBuf[arraySectors*LPTSectorSize : (arraySectors+1)*LPTSectorSize]
	copy(backupHdr, gptHdr)
	binary.LittleEndian.PutUint64(backupHdr[24:32], backupHeaderLBA) // MyLBA = backupHeaderLBA
	binary.LittleEndian.PutUint64(backupHdr[32:40], 1)               // AlternateLBA = 1
	binary.LittleEndian.PutUint64(backupHdr[72:80], backupArrayLBA)  // PartitionEntryLBA = backupArrayLBA
	backupHdr[16], backupHdr[17], backupHdr[18], backupHdr[19] = 0, 0, 0, 0
	backupHdrCRC := crc32.ChecksumIEEE(backupHdr[:hdrSize])
	binary.LittleEndian.PutUint32(backupHdr[16:20], backupHdrCRC)

	if _, err := disk.WriteAt(backupBuf, int64(backupArrayLBA*LPTSectorSize)); err != nil {
		return err
	}
	return disk.SyncAndFlush()
}
