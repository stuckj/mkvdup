package source

import (
	"encoding/binary"
	"fmt"
	"io"
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

// detectDVDCodecsFromIFOs reads IFO files from within an ISO and returns
// the unioned codec information from all title sets.
func detectDVDCodecsFromIFOs(f *os.File, ifos []isoFileExtent) (*SourceCodecs, error) {
	merged := &SourceCodecs{}
	var lastErr error
	anySuccess := false

	for _, ifo := range ifos {
		if ifo.Size <= 0 {
			continue
		}
		// We only need the first 0x244 bytes for VTS_MAT parsing, so cap the
		// read to avoid excessive allocation from malformed metadata.
		const maxIFOReadSize int64 = 0x244
		readSize := min(ifo.Size, maxIFOReadSize)
		data := make([]byte, readSize)
		n, err := f.ReadAt(data, ifo.Offset)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			lastErr = err
			continue
		}
		data = data[:n]

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
