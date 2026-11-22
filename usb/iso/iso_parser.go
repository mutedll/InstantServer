// ============================================================================
// FILE: usb/iso/iso_parser.go
// PURPOSE: Parses ISO9660 + RockRidge Continuation Area (CE) directories, stages
//
//	Windows-editable FAT32 partition ("UBUNTU-ISO") with full RockRidge
//	Continuation Area (CE) parsing, embedded /autoinstall.yaml + /cidata/*
//	seed files, and MD5-verified GRUB autoinstall kernel command lines.
//
// ============================================================================
package iso

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

type isoFileExtent struct {
	Offset int64
	Length int64
}

type isoTreeNode struct {
	Name       string
	RelPath    string
	IsDir      bool
	Extents    []isoFileExtent
	TotalSize  int64
	Override   []byte
	Children   []*isoTreeNode
	Cluster    uint32
	NumCluster uint32
}

func HasExtractableISO9660Tree(srcFile *os.File, totalBytes int64) bool {
	if totalBytes < 65536 {
		return false
	}
	var pvd [2048]byte
	if _, err := srcFile.ReadAt(pvd[:], 32768); err != nil {
		return false
	}
	if pvd[0] != 1 || string(pvd[1:6]) != "CD001" {
		return false
	}
	rootLBA := binary.LittleEndian.Uint32(pvd[156+2 : 156+6])
	rootLen := binary.LittleEndian.Uint32(pvd[156+10 : 156+14])
	return rootLBA > 0 && rootLen >= 34 && int64(rootLBA)*2048+int64(rootLen) <= totalBytes
}

// parseRockRidgeSUA parses RockRidge System Use Area entries, following any
// Continuation Area (CE) pointers so long filenames in /casper/ are never lost.
func parseRockRidgeSUA(srcFile *os.File, totalBytes int64, sua []byte, rrName *strings.Builder, isSymlink *bool, isRelocated *bool, childLinkLBA *uint32, depth int) {
	if depth > 4 {
		return
	}
	pos := 0
	for pos+4 <= len(sua) {
		s0, s1 := sua[pos], sua[pos+1]
		eLen := int(sua[pos+2])
		if eLen < 4 || pos+eLen > len(sua) {
			break
		}
		ent := sua[pos : pos+eLen]
		pos += eLen
		if s0 == 'N' && s1 == 'M' && eLen >= 5 {
			if (ent[4] & 0x06) == 0 {
				rrName.Write(ent[5:])
			}
		} else if s0 == 'S' && s1 == 'L' {
			*isSymlink = true
		} else if s0 == 'R' && s1 == 'E' {
			*isRelocated = true
		} else if s0 == 'C' && s1 == 'L' && eLen >= 12 {
			*childLinkLBA = binary.LittleEndian.Uint32(ent[4:8])
		} else if s0 == 'C' && s1 == 'E' && eLen >= 28 {
			ceLBA := binary.LittleEndian.Uint32(ent[4:8])
			ceOff := binary.LittleEndian.Uint32(ent[12:16])
			ceLen := binary.LittleEndian.Uint32(ent[20:24])
			absOff := int64(ceLBA)*2048 + int64(ceOff)
			if ceLen >= 4 && ceLen <= 4096 && absOff > 0 && absOff+int64(ceLen) <= totalBytes {
				ceBuf := make([]byte, ceLen)
				if _, err := srcFile.ReadAt(ceBuf, absOff); err == nil {
					parseRockRidgeSUA(srcFile, totalBytes, ceBuf, rrName, isSymlink, isRelocated, childLinkLBA, depth+1)
				}
			}
		}
	}
}

func parseISO9660Directory(srcFile *os.File, totalBytes int64, dirLBA uint32, dirLen uint32, parentRel string, depth int, visited map[uint32]bool) ([]*isoTreeNode, error) {
	if depth > 16 || dirLBA == 0 || dirLen == 0 || visited[dirLBA] {
		return nil, nil
	}
	visited[dirLBA] = true
	offset := int64(dirLBA) * 2048
	if offset+int64(dirLen) > totalBytes {
		return nil, nil
	}

	buf := make([]byte, dirLen)
	if _, err := srcFile.ReadAt(buf, offset); err != nil {
		return nil, err
	}

	var nodes []*isoTreeNode
	var pendingMulti *isoTreeNode
	pos := 0
	for pos < len(buf) {
		recLen := int(buf[pos])
		if recLen == 0 {
			pos = ((pos / 2048) + 1) * 2048
			continue
		}
		if pos+recLen > len(buf) || recLen < 34 {
			break
		}
		rec := buf[pos : pos+recLen]
		pos += recLen

		extentLBA := binary.LittleEndian.Uint32(rec[2:6])
		dataLen := binary.LittleEndian.Uint32(rec[10:14])
		flags := rec[25]
		nameLen := int(rec[32])
		if 33+nameLen > len(rec) {
			continue
		}
		rawNameBytes := rec[33 : 33+nameLen]
		if nameLen == 1 && (rawNameBytes[0] == 0x00 || rawNameBytes[0] == 0x01) {
			continue
		}

		suaPos := 33 + nameLen
		if nameLen%2 == 0 {
			suaPos++
		}
		var rrName strings.Builder
		isSymlink := false
		isRelocated := false
		var childLinkLBA uint32
		if suaPos < len(rec) {
			parseRockRidgeSUA(srcFile, totalBytes, rec[suaPos:], &rrName, &isSymlink, &isRelocated, &childLinkLBA, 0)
		}
		if isSymlink || isRelocated {
			continue
		}

		name := strings.TrimSpace(rrName.String())
		if name == "" {
			name = string(rawNameBytes)
			if semi := strings.IndexByte(name, ';'); semi >= 0 {
				name = name[:semi]
			}
			name = strings.TrimSuffix(name, ".")
			if parentRel == "" && strings.EqualFold(name, "_DISK") {
				name = ".disk"
			}
		}
		if name == "" || name == "." || name == ".." || name == ".rr_moved" {
			continue
		}

		isDir := (flags & 0x02) != 0
		if childLinkLBA > 0 {
			isDir = true
			extentLBA = childLinkLBA
			var dotHdr [34]byte
			if _, err := srcFile.ReadAt(dotHdr[:], int64(childLinkLBA)*2048); err == nil && dotHdr[0] >= 34 {
				dataLen = binary.LittleEndian.Uint32(dotHdr[10:14])
			}
		}

		relPath := name
		if parentRel != "" {
			relPath = parentRel + "/" + name
		}

		if !isDir && (flags&0x80) != 0 {
			if pendingMulti == nil {
				pendingMulti = &isoTreeNode{Name: name, RelPath: relPath, IsDir: false}
			}
			pendingMulti.Extents = append(pendingMulti.Extents, isoFileExtent{Offset: int64(extentLBA) * 2048, Length: int64(dataLen)})
			pendingMulti.TotalSize += int64(dataLen)
			continue
		}
		if pendingMulti != nil {
			pendingMulti.Extents = append(pendingMulti.Extents, isoFileExtent{Offset: int64(extentLBA) * 2048, Length: int64(dataLen)})
			pendingMulti.TotalSize += int64(dataLen)
			nodes = append(nodes, pendingMulti)
			pendingMulti = nil
			continue
		}

		node := &isoTreeNode{Name: name, RelPath: relPath, IsDir: isDir, TotalSize: int64(dataLen)}
		if !isDir && dataLen > 0 {
			node.Extents = []isoFileExtent{{Offset: int64(extentLBA) * 2048, Length: int64(dataLen)}}
		}
		if isDir {
			children, err := parseISO9660Directory(srcFile, totalBytes, extentLBA, dataLen, relPath, depth+1, visited)
			if err != nil {
				return nil, err
			}
			node.Children = children
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// PatchGrubConfigForAutoinstall cleanly injects unattended autoinstall kernel
// parameters into any GRUB .cfg file without corrupting menuentry syntax.
func PatchGrubConfigForAutoinstall(raw []byte) ([]byte, int) {
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	patches := 0
	const kernelArgs = "autoinstall noprompt noeject fsck.mode=skip ds=nocloud\\;s=/cdrom/cidata/"

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "set timeout=") && trimmed != "set timeout=1" {
			lines[i] = "set timeout=1"
			patches++
			continue
		}
		if (strings.HasPrefix(trimmed, "linux") || strings.HasPrefix(trimmed, "linuxefi")) && strings.Contains(line, "/casper/") {
			if !strings.Contains(line, "autoinstall") {
				if strings.Contains(line, "---") {
					lines[i] = strings.Replace(line, "---", kernelArgs+" ---", 1)
				} else {
					lines[i] = line + " " + kernelArgs + " ---"
				}
				patches++
			}
		}
	}
	if patches == 0 {
		pCount := PatchGrubKernelCmdlineInPlace(raw)
		return raw, pCount
	}
	return []byte(strings.Join(lines, "\n")), patches
}

func injectStagedCidataIntoISOTree(rootNode *isoTreeNode, cidataStageDir string, updatedMD5 map[string]string) {
	if cidataStageDir == "" {
		return
	}
	entries, err := os.ReadDir(cidataStageDir)
	if err != nil {
		return
	}
	cidataDir := &isoTreeNode{Name: "cidata", RelPath: "cidata", IsDir: true}
	existingRoot := make(map[string]bool)
	for _, ch := range rootNode.Children {
		existingRoot[strings.ToLower(ch.Name)] = true
	}

	var userDataBytes []byte
	for _, ent := range entries {
		if ent.IsDir() || strings.HasSuffix(ent.Name(), ".img") || strings.HasSuffix(ent.Name(), ".iso") {
			continue
		}
		raw, rErr := os.ReadFile(filepath.Join(cidataStageDir, ent.Name()))
		if rErr != nil || len(raw) == 0 {
			continue
		}
		cidataDir.Children = append(cidataDir.Children, &isoTreeNode{
			Name:      ent.Name(),
			RelPath:   "cidata/" + ent.Name(),
			IsDir:     false,
			TotalSize: int64(len(raw)),
			Override:  raw,
		})
		if ent.Name() == "user-data" {
			userDataBytes = raw
		}
		if (ent.Name() == "user-data" || ent.Name() == "meta-data") && !existingRoot[ent.Name()] {
			rootNode.Children = append(rootNode.Children, &isoTreeNode{
				Name:      ent.Name(),
				RelPath:   ent.Name(),
				IsDir:     false,
				TotalSize: int64(len(raw)),
				Override:  raw,
			})
			existingRoot[ent.Name()] = true
		}
	}
	if len(cidataDir.Children) > 0 && !existingRoot["cidata"] {
		rootNode.Children = append(rootNode.Children, cidataDir)
	}
	if len(userDataBytes) > 0 && !existingRoot["autoinstall.yaml"] {
		// Per Canonical Subiquity 24.04+ docs, on-media /cdrom/autoinstall.yaml must NOT
		// carry a #cloud-config header (only cloud-init user-data uses #cloud-config).
		aiText := strings.TrimPrefix(strings.ReplaceAll(string(userDataBytes), "\r\n", "\n"), "#cloud-config\n")
		aiBytes := []byte(strings.TrimLeft(aiText, "\r\n"))
		rootNode.Children = append(rootNode.Children, &isoTreeNode{
			Name:      "autoinstall.yaml",
			RelPath:   "autoinstall.yaml",
			IsDir:     false,
			TotalSize: int64(len(aiBytes)),
			Override:  aiBytes,
		})
		sum := md5.Sum(aiBytes)
		updatedMD5["./autoinstall.yaml"] = hex.EncodeToString(sum[:])
	}
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
