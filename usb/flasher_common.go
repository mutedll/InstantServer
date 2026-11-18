// ============================================================================
// FILE: usb/flasher_common.go
// PURPOSE: Core cross-platform raw USB streaming engine, 3-partition FAT32
//
//	orchestrator, and unified package facade for usb/iso, usb/payload,
//	and usb/catalog.
//
// ============================================================================
package usb

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"os"
	"syscall"
	"time"

	"github.com/mutedll/InstantServer/usb/catalog"
	"github.com/mutedll/InstantServer/usb/iso"
	"github.com/mutedll/InstantServer/usb/payload"
)

const (
	SectorAlignmentBytes = iso.SectorAlignmentBytes
	DMAChunkSizeBytes    = iso.DMAChunkSizeBytes
	LPTSectorSize        = iso.LPTSectorSize
	fatTotalSectors      = iso.FatTotalSectors
)

type (
	RawDiskHandle         = iso.RawDiskHandle
	FlashProgressEvent    = iso.FlashProgressEvent
	PauseGate             = iso.PauseGate
	UbuntuReleaseMirror   = iso.UbuntuReleaseMirror
	ServerProfileConfig   = payload.ServerProfileConfig
	CustomAppEntry        = payload.CustomAppEntry
	CustomSambaShare      = payload.CustomSambaShare
	AppOverride           = catalog.AppOverride
	MinecraftServerConfig = catalog.MinecraftServerConfig
	AppSpec               = catalog.AppSpec
)

var (
	NewPauseGate                        = iso.NewPauseGate
	AllocPageAlignedBuffer              = iso.AllocPageAlignedBuffer
	IsPointerPageAligned                = iso.IsPointerPageAligned
	BuildCidataFAT16Image               = iso.BuildCidataFAT16Image
	HasExtractableISO9660Tree           = iso.HasExtractableISO9660Tree
	PatchGrubConfigForAutoinstall       = iso.PatchGrubConfigForAutoinstall
	PatchGrubKernelCmdlineInPlace       = iso.PatchGrubKernelCmdlineInPlace
	ExtractISO9660ToFAT32Partition      = iso.ExtractISO9660ToFAT32Partition
	OfficialUbuntuMirrors               = iso.OfficialUbuntuMirrors
	ComputeFileSHA256                   = iso.ComputeFileSHA256
	DownloadISOWithResume               = iso.DownloadISOWithResume
	ValidateProfileConfig               = payload.ValidateProfileConfig
	ParseCustomAppInput                 = payload.ParseCustomAppInput
	Sha512Crypt                         = payload.Sha512Crypt
	GenerateAutoinstallUserData         = payload.GenerateAutoinstallUserData
	GenerateTargetEnvConf               = payload.GenerateTargetEnvConf
	NormalizeShellScriptLF              = payload.NormalizeShellScriptLF
	StageCIDATAPayload                  = payload.StageCIDATAPayload
	VerifiedCoreAppSpecs                = catalog.VerifiedCoreAppSpecs
	DefaultMinecraftConfig              = catalog.DefaultMinecraftConfig
	BuildMinecraftComposeYAML           = catalog.BuildMinecraftComposeYAML
	BuildVerifiedAppComposeYAML         = catalog.BuildVerifiedAppComposeYAML
	BuildVerifiedAppComposeWithOverride = catalog.BuildVerifiedAppComposeWithOverride
)

func init() {
	iso.HostWakeLockHook = AcquireHostWakeLock
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

	// If source ISO contains a full ISO9660 directory tree, extract it into an openable,
	// editable FAT32 partition ("UBUNTU-ISO") alongside "CIDATA" (FAT16) and "INSTANTUSB" (FAT32).
	if injectGPTCidata && HasExtractableISO9660Tree(srcFile, totalBytes) {
		return streamISOAsEditableFAT32Partitions(ctx, srcFile, totalBytes, rawDisk, targetDevice, cidataStageDir, patchGrub, pauseGate, crcHasher, shaHasher, onProgress)
	}

	var writtenBytes int64
	var totalGrubPatches int
	var deferredHeader64K []byte
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

		if injectGPTCidata && writtenBytes == 0 && len(writeSlice) >= 65536 {
			deferredHeader64K = AllocPageAlignedBuffer(65536, SectorAlignmentBytes)
			copy(deferredHeader64K, writeSlice[:65536])
			for z := 0; z < 65536; z++ {
				writeSlice[z] = 0
			}
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
		stagePct := (float64(writtenBytes) * 100.0) / float64(totalBytes)
		if stagePct > 100.0 {
			stagePct = 100.0
		}
		pct := int(stagePct)

		if onProgress != nil {
			onProgress(FlashProgressEvent{
				Stage:        "FLASHING",
				Percent:      pct,
				StagePercent: stagePct,
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

	if len(deferredHeader64K) == 65536 {
		if _, hErr := rawDisk.WriteAt(deferredHeader64K, 0); hErr != nil {
			return nil, fmt.Errorf("IS-USB-302: failed writing deferred boot header on %s: %w", targetDevice, hErr)
		}
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
		StagePercent: 100.0,
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

func writeAlignedChunks(disk RawDiskHandle, data []byte, baseOffset int64) error {
	return iso.WriteAlignedChunks(disk, data, baseOffset)
}

func streamISOAsEditableFAT32Partitions(
	ctx context.Context,
	srcFile *os.File,
	totalBytes int64,
	rawDisk RawDiskHandle,
	targetDevice string,
	cidataStageDir string,
	patchGrub bool,
	pauseGate *PauseGate,
	crcHasher hash.Hash32,
	shaHasher hash.Hash,
	onProgress func(FlashProgressEvent),
) (*FlashProgressEvent, error) {
	// 1. Wipe first 1 MB so no whole-disk ISO9660 CD001 superblock remains at offset 32768
	zero1M := AllocPageAlignedBuffer(1024*1024, SectorAlignmentBytes)
	if _, err := rawDisk.WriteAt(zero1M, 0); err != nil {
		return nil, fmt.Errorf("IS-USB-302: failed clearing first 1MB on %s: %w", targetDevice, err)
	}

	// 2. Extract ISO9660 tree into Partition #1 ("UBUNTU-ISO", FAT32, starting at LBA 2048)
	isoStartLBA := uint64(2048)
	isoEndLBA, totalGrubPatches, err := ExtractISO9660ToFAT32Partition(
		ctx, srcFile, totalBytes, rawDisk, isoStartLBA, cidataStageDir, patchGrub, pauseGate, crcHasher, shaHasher, onProgress,
	)
	if err != nil {
		return nil, fmt.Errorf("IS-USB-302: failed building editable FAT32 ISO partition on %s: %w", targetDevice, err)
	}

	// 3. Build and write Partition #2 ("CIDATA", FAT16)
	var cidataFAT []byte
	if cidataStageDir != "" {
		fatImg, fatErr := BuildCidataFAT16Image(cidataStageDir)
		if fatErr != nil {
			return nil, fmt.Errorf("IS-USB-310: failed building FAT16 CIDATA partition image: %w", fatErr)
		}
		cidataFAT = fatImg
	}
	cidataStartLBA := ((isoEndLBA + 1 + 2047) / 2048) * 2048
	cidataSectors := uint64(len(cidataFAT)) / LPTSectorSize
	if cidataSectors == 0 {
		cidataSectors = uint64(fatTotalSectors)
	}
	cidataEndLBA := cidataStartLBA + cidataSectors - 1
	if len(cidataFAT) > 0 {
		if err := writeAlignedChunks(rawDisk, cidataFAT, int64(cidataStartLBA*LPTSectorSize)); err != nil {
			return nil, fmt.Errorf("IS-USB-311: failed writing CIDATA partition on %s: %w", targetDevice, err)
		}
	}

	// 4. Write Partition #3 ("INSTANTUSB", FAT32) and Primary + Backup GPT tables
	if err := writeThreePartitionGPT(rawDisk, isoStartLBA, isoEndLBA, cidataStartLBA, cidataEndLBA); err != nil {
		return nil, fmt.Errorf("IS-USB-311: failed writing 3-partition GPT table on %s: %w", targetDevice, err)
	}
	if err := rawDisk.SyncAndFlush(); err != nil {
		return nil, fmt.Errorf("IS-USB-305: hardware cache flush failed on %s: %w", targetDevice, err)
	}

	finalEvent := &FlashProgressEvent{
		Stage:        "VERIFIED",
		Percent:      100,
		StagePercent: 100.0,
		WrittenBytes: totalBytes,
		TotalBytes:   totalBytes,
		CRC32Hex:     fmt.Sprintf("%08X", crcHasher.Sum32()),
		SHA256Hex:    hex.EncodeToString(shaHasher.Sum(nil)),
		GrubPatches:  totalGrubPatches,
		Message:      "Verified editable FAT32 UBUNTU-ISO partition, FAT16 CIDATA partition, and FAT32 INSTANTUSB partition.",
	}
	if onProgress != nil {
		onProgress(*finalEvent)
	}
	return finalEvent, nil
}
