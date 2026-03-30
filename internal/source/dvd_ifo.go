package source

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

// parseDVDIFOCodecs parses a VTS_xx_0.IFO file's VTS_MAT structure to extract
// video and audio codec information. The IFO file authoritatively declares
// every stream in the title set, unlike PES scanning which can miss streams
// that appear later in the VOB data.
//
// VTS_MAT layout (relevant offsets):
//
//	0x000-0x00B: "DVDVIDEO-VTS" identifier
//	0x200-0x201: VTS video attributes (2 bytes)
//	0x202-0x203: Number of VTS audio streams (2 bytes, big-endian)
//	0x204-0x243: VTS audio stream attributes (8 bytes each, max 8)
func parseDVDIFOCodecs(data []byte) (*SourceCodecs, error) {
	if len(data) < 0x244 {
		return nil, fmt.Errorf("IFO data too short (%d bytes)", len(data))
	}

	magic := string(data[0:12])
	if magic != "DVDVIDEO-VTS" {
		return nil, fmt.Errorf("not a VTS IFO file (magic: %q)", magic)
	}

	codecs := &SourceCodecs{}

	// Video attributes at offset 0x200 (2 bytes, big-endian).
	// Bits 15-14: video compression mode (0=MPEG-1, 1=MPEG-2).
	videoAttr := binary.BigEndian.Uint16(data[0x200:0x202])
	switch (videoAttr >> 14) & 0x03 {
	case 0:
		codecs.VideoCodecs = append(codecs.VideoCodecs, CodecMPEG1Video)
	case 1:
		codecs.VideoCodecs = append(codecs.VideoCodecs, CodecMPEG2Video)
	}

	// Audio stream count at offset 0x202 (2 bytes, big-endian).
	numAudio := int(binary.BigEndian.Uint16(data[0x202:0x204]))
	numAudio = min(numAudio, 8)

	// Audio attributes at offset 0x204 (8 bytes each).
	// Byte 0, bits 7-5: audio coding mode.
	for i := 0; i < numAudio; i++ {
		off := 0x204 + i*8
		// Skip all-zero entries (unused slots)
		if data[off] == 0 && data[off+1] == 0 {
			continue
		}
		codingMode := (data[off] >> 5) & 0x07
		var ct CodecType
		switch codingMode {
		case 0:
			ct = CodecAC3Audio
		case 2, 3:
			ct = CodecMPEGAudio // MPEG-1 and MPEG-2ext
		case 4:
			ct = CodecLPCMAudio
		case 6:
			ct = CodecDTSAudio
		default:
			continue
		}
		if !containsCodec(codecs.AudioCodecs, ct) {
			codecs.AudioCodecs = append(codecs.AudioCodecs, ct)
		}
	}

	return codecs, nil
}

// findIFOsInISO navigates an ISO9660 filesystem to find VTS IFO files
// (VTS_xx_0.IFO) under the VIDEO_TS directory. Returns nil if navigation fails.
func findIFOsInISO(f *os.File) []isoFileExtent {
	rootLBA, rootLen, err := readISOPVDRoot(f)
	if err != nil {
		return nil
	}

	rootEntries, err := readISODirectory(f, rootLBA, rootLen)
	if err != nil {
		return nil
	}

	videoTS, err := findISOEntry(rootEntries, "VIDEO_TS")
	if err != nil {
		return nil
	}

	vtsEntries, err := readISODirectory(f, uint32(videoTS.Offset/isoSectorSize), uint32(videoTS.Size))
	if err != nil {
		return nil
	}

	var ifos []isoFileExtent
	for _, e := range vtsEntries {
		if e.IsDir {
			continue
		}
		name := e.Name
		// Match VTS_xx_0.IFO pattern (e.g., VTS_01_0.IFO)
		if strings.HasPrefix(name, "VTS_") && strings.HasSuffix(name, ".IFO") &&
			len(name) == 12 && name[7] == '0' {
			ifos = append(ifos, e)
		}
	}
	return ifos
}

// findIFOsInUDF navigates a UDF filesystem to find VTS IFO files under VIDEO_TS.
func findIFOsInUDF(f *os.File) ([]isoFileExtent, error) {
	ctx, err := newUDFContext(f)
	if err != nil {
		return nil, err
	}

	rootFIDs, err := ctx.readDirectoryFromFE(ctx.rootFE)
	if err != nil {
		return nil, fmt.Errorf("read UDF root directory: %w", err)
	}

	vtsFE, err := ctx.lookupDir(rootFIDs, "VIDEO_TS")
	if err != nil {
		return nil, fmt.Errorf("find VIDEO_TS: %w", err)
	}

	vtsFIDs, err := ctx.readDirectoryFromFE(vtsFE)
	if err != nil {
		return nil, fmt.Errorf("read VIDEO_TS directory: %w", err)
	}

	var ifos []isoFileExtent
	for _, fid := range vtsFIDs {
		if fid.IsDir || fid.IsParent {
			continue
		}
		name := strings.ToUpper(fid.Name)
		if !strings.HasPrefix(name, "VTS_") || !strings.HasSuffix(name, ".IFO") {
			continue
		}
		if len(name) != 12 || name[7] != '0' {
			continue
		}

		fe, err := ctx.readFileEntryAt(fid.ICBLocation)
		if err != nil || fe.InfoLength == 0 {
			continue
		}

		extents, err := ctx.resolveAllExtents(fe)
		if err != nil || len(extents) == 0 {
			continue
		}

		ifo := isoFileExtent{
			Name:   name,
			Offset: extents[0].ISOOffset,
			Size:   int64(fe.InfoLength),
			IsDir:  false,
		}
		if !extentsContiguous(extents) {
			ifo.Extents = extents
		}
		ifos = append(ifos, ifo)
	}

	if len(ifos) == 0 {
		return nil, fmt.Errorf("no VTS IFO files found in UDF VIDEO_TS/")
	}
	return ifos, nil
}

// cellAddrEntry represents one entry in the IFO Cell Address Table (C_ADT).
// Sector values are relative to the start of the VTS title VOBs.
type cellAddrEntry struct {
	VOBId       uint16
	CellId      uint8
	StartSector uint32 // relative to VTS VOBs start
	LastSector  uint32 // inclusive, relative to VTS VOBs start
}

// vtsCellInfo holds the cell address table and VOB start sector for a VTS.
type vtsCellInfo struct {
	VTSVobsSector uint32 // absolute sector of VTS title VOBs in the ISO
	Cells         []cellAddrEntry
}

// parseDVDIFOCells parses a VTS IFO file to extract the Cell Address Table.
// The C_ADT maps sector ranges within the VTS VOBs to (VOB ID, Cell ID) pairs,
// defining which portions of the interleaved VOB data belong to which cell.
//
// VTS_MAT layout (relevant offsets):
//
//	0x084: VTS title VOBs start sector (4 bytes BE, relative to IFO start)
//	0x0E0: VTS_C_ADT sector offset (4 bytes BE, relative to IFO start)
func parseDVDIFOCells(ifoData []byte, ifoAbsSector uint32) (*vtsCellInfo, error) {
	if len(ifoData) < 0xE4 {
		return nil, fmt.Errorf("IFO data too short for C_ADT pointer (%d bytes)", len(ifoData))
	}

	magic := string(ifoData[:12])
	if magic != "DVDVIDEO-VTS" {
		return nil, fmt.Errorf("not a VTS IFO file (magic: %q)", magic)
	}

	vtsVobsSectorRel := binary.BigEndian.Uint32(ifoData[0x84:0x88])
	cadtSectorRel := binary.BigEndian.Uint32(ifoData[0xE0:0xE4])
	if cadtSectorRel == 0 {
		return nil, fmt.Errorf("no C_ADT in this VTS IFO")
	}

	// C_ADT is at cadtSectorRel * 2048 bytes from the IFO start
	cadtOffset := int(cadtSectorRel) * isoSectorSize
	if cadtOffset+8 > len(ifoData) {
		return nil, fmt.Errorf("C_ADT offset %d beyond IFO data (%d bytes)", cadtOffset, len(ifoData))
	}
	cadtData := ifoData[cadtOffset:]

	// C_ADT header:
	//   0-1: nr_of_vobs (uint16 BE)
	//   2-3: reserved
	//   4-7: last_byte (uint32 BE, inclusive)
	lastByte := binary.BigEndian.Uint32(cadtData[4:8])
	numEntries := (int(lastByte) + 1 - 8) / 12
	if numEntries <= 0 || numEntries > 10000 {
		return nil, fmt.Errorf("invalid C_ADT entry count: %d", numEntries)
	}

	info := &vtsCellInfo{
		VTSVobsSector: ifoAbsSector + vtsVobsSectorRel,
		Cells:         make([]cellAddrEntry, 0, numEntries),
	}

	for i := range numEntries {
		off := 8 + i*12
		if off+12 > len(cadtData) {
			break
		}
		info.Cells = append(info.Cells, cellAddrEntry{
			VOBId:       binary.BigEndian.Uint16(cadtData[off : off+2]),
			CellId:      cadtData[off+2],
			StartSector: binary.BigEndian.Uint32(cadtData[off+4 : off+8]),
			LastSector:  binary.BigEndian.Uint32(cadtData[off+8 : off+12]),
		})
	}

	return info, nil
}

// findVTSCellInfo finds and parses the IFO file for the VTS that contains the given
// VOB file offset. Returns nil if the IFO cannot be found or parsed.
// The isoData is the full mmap'd ISO; ifoFinder locates IFO file extents.
func findVTSCellInfo(isoData []byte, vobAbsSector uint32) *vtsCellInfo {
	isoSize := int64(len(isoData))

	// Find IFO files by navigating ISO9660 structure
	// We reuse the same ISO navigation as findIFOsInISO but work from mmap'd data
	if isoSize < 17*isoSectorSize+isoSectorSize {
		return nil
	}

	pvd := isoData[16*isoSectorSize : 17*isoSectorSize]
	if pvd[0] != 1 || string(pvd[1:6]) != "CD001" {
		return nil
	}

	rootDirRecord := pvd[156:]
	if len(rootDirRecord) < 34 {
		return nil
	}
	rootExtent := uint32(rootDirRecord[2]) | uint32(rootDirRecord[3])<<8 | uint32(rootDirRecord[4])<<16 | uint32(rootDirRecord[5])<<24
	rootDataLen := uint32(rootDirRecord[10]) | uint32(rootDirRecord[11])<<8 | uint32(rootDirRecord[12])<<16 | uint32(rootDirRecord[13])<<24

	rootStart := int64(rootExtent) * isoSectorSize
	rootEnd := rootStart + int64(rootDataLen)
	if rootEnd > isoSize {
		return nil
	}
	rootDir := isoData[rootStart:rootEnd]

	// Find VIDEO_TS directory
	var vtsDirExtent, vtsDirLen uint32
	for offset := 0; offset < len(rootDir); {
		recLen := int(rootDir[offset])
		if recLen == 0 {
			nextSector := ((offset / isoSectorSize) + 1) * isoSectorSize
			if nextSector >= len(rootDir) {
				break
			}
			offset = nextSector
			continue
		}
		if offset+33 > len(rootDir) {
			break
		}
		nameLen := int(rootDir[offset+32])
		if offset+33+nameLen > len(rootDir) {
			break
		}
		name := strings.ToUpper(string(rootDir[offset+33 : offset+33+nameLen]))
		if idx := strings.Index(name, ";"); idx >= 0 {
			name = name[:idx]
		}
		name = strings.TrimSuffix(name, ".")
		if name == "VIDEO_TS" {
			vtsDirExtent = uint32(rootDir[offset+2]) | uint32(rootDir[offset+3])<<8 | uint32(rootDir[offset+4])<<16 | uint32(rootDir[offset+5])<<24
			vtsDirLen = uint32(rootDir[offset+10]) | uint32(rootDir[offset+11])<<8 | uint32(rootDir[offset+12])<<16 | uint32(rootDir[offset+13])<<24
		}
		offset += recLen
	}
	if vtsDirExtent == 0 {
		return nil
	}

	vtsDirStart := int64(vtsDirExtent) * isoSectorSize
	vtsDirEnd := vtsDirStart + int64(vtsDirLen)
	if vtsDirEnd > isoSize {
		return nil
	}
	vtsDir := isoData[vtsDirStart:vtsDirEnd]

	// Find all VTS IFO files and check which VTS contains our VOB sector
	for offset := 0; offset < len(vtsDir); {
		recLen := int(vtsDir[offset])
		if recLen == 0 {
			nextSector := ((offset / isoSectorSize) + 1) * isoSectorSize
			if nextSector >= len(vtsDir) {
				break
			}
			offset = nextSector
			continue
		}
		if offset+33 > len(vtsDir) {
			break
		}
		nameLen := int(vtsDir[offset+32])
		if offset+33+nameLen > len(vtsDir) {
			break
		}
		name := strings.ToUpper(string(vtsDir[offset+33 : offset+33+nameLen]))
		if idx := strings.Index(name, ";"); idx >= 0 {
			name = name[:idx]
		}
		name = strings.TrimSuffix(name, ".")

		if len(name) == 12 && strings.HasPrefix(name, "VTS_") && name[7] == '0' && strings.HasSuffix(name, ".IFO") {
			ifoSector := uint32(vtsDir[offset+2]) | uint32(vtsDir[offset+3])<<8 | uint32(vtsDir[offset+4])<<16 | uint32(vtsDir[offset+5])<<24
			ifoSize := uint32(vtsDir[offset+10]) | uint32(vtsDir[offset+11])<<8 | uint32(vtsDir[offset+12])<<16 | uint32(vtsDir[offset+13])<<24

			ifoStart := int64(ifoSector) * isoSectorSize
			ifoEnd := ifoStart + int64(ifoSize)
			if ifoEnd > isoSize {
				offset += recLen
				continue
			}

			info, err := parseDVDIFOCells(isoData[ifoStart:ifoEnd], ifoSector)
			if err != nil {
				offset += recLen
				continue
			}

			// Check if the VOB sector falls within this VTS's VOB range
			if len(info.Cells) > 0 {
				lastCell := info.Cells[len(info.Cells)-1]
				vtsVobEnd := info.VTSVobsSector + lastCell.LastSector
				if vobAbsSector >= info.VTSVobsSector && vobAbsSector <= vtsVobEnd {
					return info
				}
			}
		}
		offset += recLen
	}

	return nil
}

// detectDVDCodecsFromIFOs reads IFO files from within an ISO and returns
// the unioned codec information from all title sets.
func detectDVDCodecsFromIFOs(f *os.File, ifos []isoFileExtent) (*SourceCodecs, error) {
	merged := &SourceCodecs{}
	var lastErr error
	anySuccess := false

	for _, ifo := range ifos {
		// We only need the first 0x244 bytes for VTS_MAT parsing.
		const maxIFOReadSize int64 = 0x244
		data, err := readISOFileExtent(f, ifo, maxIFOReadSize)
		if err != nil {
			lastErr = err
			continue
		}

		codecs, err := parseDVDIFOCodecs(data)
		if err != nil {
			lastErr = err
			continue
		}
		mergeSourceCodecs(merged, codecs)
		anySuccess = true
	}

	if !anySuccess {
		if lastErr != nil {
			return nil, fmt.Errorf("failed to parse any VTS IFO: %w", lastErr)
		}
		return nil, fmt.Errorf("no valid VTS IFO files found")
	}
	return merged, nil
}
