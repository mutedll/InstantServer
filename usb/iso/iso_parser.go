// ============================================================================
// FILE: usb/iso_extractor.go
// PURPOSE: Writes the parsed ISO9660 + CIDATA tree to a bootable, Windows-editable
//
//	FAT32 partition ("UBUNTU-ISO") and formats Partition #3 ("INSTANTUSB").
//
// ============================================================================
package iso

import (
	"context"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"hash"
	"io"
	"os"
	"strings"
	"time"
)

func ExtractISO9660ToFAT32Partition(
	ctx context.Context,
	srcFile *os.File,
	totalISOBytes int64,
	disk RawDiskHandle,
	startLBA uint64,
	cidataStageDir string,
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
				patchedRaw, pCount := PatchGrubConfigForAutoinstall(raw)
				if pCount > 0 {
					grubPatches += pCount
					ch.Override = patchedRaw
					ch.TotalSize = int64(len(patchedRaw))
					sum := md5.Sum(patchedRaw)
					updatedMD5["./"+ch.RelPath] = hex.EncodeToString(sum[:])
				}
			}
		}
	}
	walkPrepare(rootNode)
	injectStagedCidataIntoISOTree(rootNode, cidataStageDir, updatedMD5)

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

	secPerClus := uint32(16)
	if totalISOBytes > 512*1024*1024 {
		secPerClus = 64
	} else if totalISOBytes < 32*1024*1024 {
		secPerClus = 1
	}
	clusterBytes := int(secPerClus * 512)

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
					dst := &isoTreeNode{Name: newName, RelPath: rel, IsDir: src.IsDir, TotalSize: src.TotalSize, Extents: src.Extents, Override: src.Override}
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

	nextCluster := uint32(2)
	var assignClusters func(dir *isoTreeNode)
	assignClusters = func(dir *isoTreeNode) {
		entryCount := 2
		for _, ch := range dir.Children {
			entryCount += 1 + ((len([]rune(ch.Name)) + 12) / 13)
		}
		dirClusters := uint32(((entryCount+2)*32 + clusterBytes - 1) / clusterBytes)
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
	totalDataClusters := allocatedClusters + 2048
	if totalDataClusters < 65600 {
		totalDataClusters = 65600
	}
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
	totalPartSectors := ((reservedSectors + (numFATs * fatSectors) + (totalDataClusters * secPerClus) + 2047) / 2048) * 2048
	totalDataClusters = (totalPartSectors - reservedSectors - (numFATs * fatSectors)) / secPerClus

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
		b[64], b[65], b[66] = 0x80, 0x00, 0x29
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

	fatTable := make([]byte, int(fatSectors*512))
	binary.LittleEndian.PutUint32(fatTable[0:4], 0x0FFFFFF8)
	binary.LittleEndian.PutUint32(fatTable[4:8], 0x0FFFFFFF)
	var markChain func(n *isoTreeNode)
	markChain = func(n *isoTreeNode) {
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
		for _, ch := range n.Children {
			markChain(ch)
		}
	}
	markChain(rootNode)

	fat1LBA := startLBA + uint64(reservedSectors)
	fat2LBA := fat1LBA + uint64(fatSectors)
	if err := WriteAlignedChunks(disk, fatTable, int64(fat1LBA*512)); err != nil {
		return 0, 0, err
	}
	if err := WriteAlignedChunks(disk, fatTable, int64(fat2LBA*512)); err != nil {
		return 0, 0, err
	}

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
			StampDOSDirEntry(&volEnt, 0x08, 0, 0)
			copy(dirBuf[0:32], volEnt[:])
			cursor = 32
		} else {
			dotSelf := MakeDotEntry(".          ", dir.Cluster)
			pClus := parentCluster
			if pClus == 2 {
				pClus = 0
			}
			dotParent := MakeDotEntry("..         ", pClus)
			copy(dirBuf[0:32], dotSelf[:])
			copy(dirBuf[32:64], dotParent[:])
			cursor = 64
		}

		usedShort := make(map[[11]byte]bool)
		for idx, ch := range dir.Children {
			shortName := MakeClean83ShortName(ch.Name, idx+1, usedShort)
			for _, lfn := range BuildLFNEntries(ch.Name, shortName) {
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
				StampDOSDirEntry(&stdEnt, attr, ch.Cluster, size)
				copy(dirBuf[cursor:cursor+32], stdEnt[:])
				cursor += 32
			}
		}

		if err := WriteAlignedChunks(disk, dirBuf, clusterOffsetBytes(dir.Cluster)); err != nil {
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

	dmaBuf := AllocPageAlignedBuffer(DMAChunkSizeBytes, SectorAlignmentBytes)
	var writtenBytes int64
	startTime := time.Now()
	var lastProgressEmit time.Time
	var smoothedMBps float64

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
				if err := WriteAlignedChunks(disk, ch.Override, destOffset); err != nil {
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

					if onProgress != nil && (time.Since(lastProgressEmit) >= 120*time.Millisecond || writtenBytes >= totalISOBytes) {
						lastProgressEmit = time.Now()
						elapsed := time.Since(startTime).Seconds()
						if elapsed < 0.05 {
							elapsed = 0.05
						}
						instMBps := (float64(writtenBytes) / 1048576.0) / elapsed
						if smoothedMBps <= 0.1 {
							smoothedMBps = instMBps
						} else {
							smoothedMBps = smoothedMBps*0.80 + instMBps*0.20
						}
						stagePct := (float64(writtenBytes) * 95.0) / float64(totalISOBytes)
						if stagePct > 95.0 {
							stagePct = 95.0
						}
						etaSec := 0.0
						if remBytes := totalISOBytes - writtenBytes; smoothedMBps > 0.2 && remBytes > 0 && elapsed > 0.35 {
							etaSec = (float64(remBytes)/1048576.0)/smoothedMBps + 2.0
						}
						onProgress(FlashProgressEvent{
							Stage:        "FLASHING",
							Percent:      int(stagePct),
							StagePercent: stagePct,
							WrittenBytes: writtenBytes,
							TotalBytes:   totalISOBytes,
							SpeedMBps:    smoothedMBps,
							ETASeconds:   etaSec,
							CRC32Hex:     bytesToHexCRC(crcHasher.Sum32()),
							GrubPatches:  grubPatches,
							Message:      "Writing editable FAT32 ISO (" + ch.Name + ")",
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

	return startLBA + uint64(totalPartSectors) - 1, grubPatches, nil
}

func bytesToHexCRC(v uint32) string {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], v)
	return strings.ToUpper(hex.EncodeToString(b[:]))
}
