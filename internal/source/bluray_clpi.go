package source

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// parseBlurayClipInfoCodecs parses a CLPI file's ProgramInfo section to extract
// codec information. CLPI files are small metadata files in BDMV/CLIPINF/ that
// authoritatively declare every elementary stream's codec type.
//
// CLPI header layout:
//
//	0x00-0x03: Type indicator ("HDMV")
//	0x04-0x07: Version string
//	0x08-0x0B: SequenceInfo start offset (4 bytes, big-endian)
//	0x0C-0x0F: ProgramInfo start offset (4 bytes, big-endian)
//
// ProgramInfo layout:
//
//	[0-3]  Section length (4 bytes, big-endian)
//	[4]    Reserved
//	[5]    Number of program sequences
//	Per sequence:
//	  [0-3]  SPN_program_sequence_start (4 bytes)
//	  [4-5]  program_map_PID (2 bytes)
//	  [6]    num_streams_in_ps (1 byte)
//	  [7]    num_groups (1 byte)
//	  Per stream:
//	    [0-1]  stream_PID (2 bytes)
//	    [2]    stream_coding_info_length (1 byte)
//	    [3]    stream_coding_type (1 byte) — same values as tsStreamTypeToCodecType
func parseBlurayClipInfoCodecs(data []byte) (*SourceCodecs, error) {
	if len(data) < 16 {
		return nil, fmt.Errorf("CLPI data too short (%d bytes)", len(data))
	}

	magic := string(data[0:4])
	if magic != "HDMV" {
		return nil, fmt.Errorf("not a CLPI file (magic: %q)", magic)
	}

	progInfoOffset := binary.BigEndian.Uint32(data[12:16])
	if progInfoOffset == 0 || int(progInfoOffset)+6 > len(data) {
		return nil, fmt.Errorf("invalid ProgramInfo offset: %d", progInfoOffset)
	}

	pi := data[progInfoOffset:]
	if len(pi) < 6 {
		return nil, fmt.Errorf("ProgramInfo section too short")
	}

	piLen := binary.BigEndian.Uint32(pi[0:4])
	if piLen == 0 {
		return nil, fmt.Errorf("empty ProgramInfo section")
	}

	// Cap the section to its declared length + header.
	pi = pi[:min(int(piLen)+4, len(pi))]

	numSeqs := int(pi[5])
	codecs := &SourceCodecs{}

	off := 6
	for range numSeqs {
		if off+8 > len(pi) {
			break
		}
		// SPN(4) + program_map_PID(2) + num_streams(1) + num_groups(1)
		//
		// num_groups (pi[off+7]) is not processed. The Blu-ray spec is proprietary
		// and the group entry format is undocumented. In practice num_groups is
		// always 0 on real discs, and no open-source parser (libbluray, MKVToolNix)
		// processes group entries either — the field is effectively reserved.
		numStreams := int(pi[off+6])
		off += 8

		for range numStreams {
			if off+3 > len(pi) {
				break
			}
			// stream_PID(2) + ci_len(1)
			ciLen := int(pi[off+2])

			if ciLen > 0 && off+3 < len(pi) {
				streamType := pi[off+3]
				ct := tsStreamTypeToCodecType(streamType)
				if ct != CodecUnknown {
					if IsVideoCodec(ct) {
						if !containsCodec(codecs.VideoCodecs, ct) {
							codecs.VideoCodecs = append(codecs.VideoCodecs, ct)
						}
					} else if IsAudioCodec(ct) {
						if !containsCodec(codecs.AudioCodecs, ct) {
							codecs.AudioCodecs = append(codecs.AudioCodecs, ct)
						}
					} else if IsSubtitleCodec(ct) {
						if !containsCodec(codecs.SubtitleCodecs, ct) {
							codecs.SubtitleCodecs = append(codecs.SubtitleCodecs, ct)
						}
					}
				}
			}

			off += 3 + ciLen
		}
	}

	return codecs, nil
}

// findCLPIsInISO navigates an ISO9660 filesystem to find CLPI files
// under BDMV/CLIPINF/. Returns the file extents or an error.
func findCLPIsInISO(f *os.File) ([]isoFileExtent, error) {
	rootLBA, rootLen, err := readISOPVDRoot(f)
	if err != nil {
		return nil, err
	}

	rootEntries, err := readISODirectory(f, rootLBA, rootLen)
	if err != nil {
		return nil, fmt.Errorf("read ISO root directory: %w", err)
	}

	bdmv, err := findISOEntry(rootEntries, "BDMV")
	if err != nil {
		return nil, fmt.Errorf("find BDMV directory: %w", err)
	}

	bdmvEntries, err := readISODirectory(f, uint32(bdmv.Offset/isoSectorSize), uint32(bdmv.Size))
	if err != nil {
		return nil, fmt.Errorf("read BDMV directory: %w", err)
	}

	clipinf, err := findISOEntry(bdmvEntries, "CLIPINF")
	if err != nil {
		return nil, fmt.Errorf("find CLIPINF directory: %w", err)
	}

	clipinfEntries, err := readISODirectory(f, uint32(clipinf.Offset/isoSectorSize), uint32(clipinf.Size))
	if err != nil {
		return nil, fmt.Errorf("read CLIPINF directory: %w", err)
	}

	var clpis []isoFileExtent
	for _, e := range clipinfEntries {
		if !e.IsDir && strings.HasSuffix(e.Name, ".CLPI") {
			clpis = append(clpis, e)
		}
	}

	if len(clpis) == 0 {
		return nil, fmt.Errorf("no CLPI files found in BDMV/CLIPINF/")
	}
	return clpis, nil
}

// findCLPIsInUDF navigates a UDF filesystem to find CLPI files under BDMV/CLIPINF/.
func findCLPIsInUDF(f *os.File) ([]isoFileExtent, error) {
	ctx, err := newUDFContext(f)
	if err != nil {
		return nil, err
	}

	rootFIDs, err := ctx.readDirectoryFromFE(ctx.rootFE)
	if err != nil {
		return nil, fmt.Errorf("read UDF root directory: %w", err)
	}

	bdmvFE, err := ctx.lookupDir(rootFIDs, "BDMV")
	if err != nil {
		return nil, fmt.Errorf("find BDMV: %w", err)
	}

	bdmvFIDs, err := ctx.readDirectoryFromFE(bdmvFE)
	if err != nil {
		return nil, fmt.Errorf("read BDMV directory: %w", err)
	}

	clipinfFE, err := ctx.lookupDir(bdmvFIDs, "CLIPINF")
	if err != nil {
		return nil, fmt.Errorf("find CLIPINF: %w", err)
	}

	clipinfFIDs, err := ctx.readDirectoryFromFE(clipinfFE)
	if err != nil {
		return nil, fmt.Errorf("read CLIPINF directory: %w", err)
	}

	var clpis []isoFileExtent
	for _, fid := range clipinfFIDs {
		if fid.IsDir || fid.IsParent {
			continue
		}
		name := strings.ToUpper(fid.Name)
		if !strings.HasSuffix(name, ".CLPI") {
			continue
		}

		fe, err := ctx.readFileEntryAt(fid.ICBLocation)
		if err != nil {
			continue
		}

		extents, err := ctx.resolveAllExtents(fe)
		if err != nil || len(extents) == 0 {
			continue
		}

		clpi := isoFileExtent{
			Name:   name,
			Offset: extents[0].ISOOffset,
			Size:   int64(fe.InfoLength),
			IsDir:  false,
		}
		if !extentsContiguous(extents) {
			clpi.Extents = extents
		}
		clpis = append(clpis, clpi)
	}

	if len(clpis) == 0 {
		return nil, fmt.Errorf("no CLPI files found in UDF BDMV/CLIPINF/")
	}
	return clpis, nil
}

// detectBlurayCodecsFromCLPIs reads CLPI files from within an ISO and returns
// the unioned codec information from all clip info files.
func detectBlurayCodecsFromCLPIs(f *os.File, clpis []isoFileExtent) (*SourceCodecs, error) {
	merged := &SourceCodecs{}
	var lastErr error
	anySuccess := false

	for _, clpi := range clpis {
		if clpi.Size <= 0 {
			continue
		}
		// Cap read size to prevent excessive allocation from malformed metadata.
		// Real CLPI files are ~64-78KB.
		const maxCLPISize = 8 * 1024 * 1024
		readSize := min(clpi.Size, maxCLPISize)
		data := make([]byte, readSize)
		n, err := f.ReadAt(data, clpi.Offset)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			lastErr = err
			continue
		}
		data = data[:n]

		codecs, err := parseBlurayClipInfoCodecs(data)
		if err != nil {
			lastErr = err
			continue
		}
		mergeSourceCodecs(merged, codecs)
		anySuccess = true
	}

	if !anySuccess {
		if lastErr != nil {
			return nil, fmt.Errorf("failed to parse any CLPI file: %w", lastErr)
		}
		return nil, fmt.Errorf("no valid CLPI files found")
	}
	return merged, nil
}

// detectBlurayCodecsFromCLPIDir detects codecs from CLPI files in an extracted
// Blu-ray directory structure (BDMV/CLIPINF/*.clpi).
func detectBlurayCodecsFromCLPIDir(sourceDir string) (*SourceCodecs, error) {
	clipinfDir := sourceDir
	// If sourceDir doesn't end with CLIPINF, try to find it
	if !strings.HasSuffix(strings.ToUpper(sourceDir), "CLIPINF") {
		// Look for BDMV/CLIPINF relative to sourceDir
		candidates := []string{
			filepath.Join(sourceDir, "BDMV", "CLIPINF"),
			filepath.Join(sourceDir, "bdmv", "clipinf"),
		}
		found := false
		for _, c := range candidates {
			if info, err := os.Stat(c); err == nil && info.IsDir() {
				clipinfDir = c
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("BDMV/CLIPINF directory not found in %s", sourceDir)
		}
	}

	entries, err := os.ReadDir(clipinfDir)
	if err != nil {
		return nil, fmt.Errorf("read CLIPINF directory: %w", err)
	}

	merged := &SourceCodecs{}
	var lastErr error
	anySuccess := false

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToUpper(entry.Name())
		if !strings.HasSuffix(name, ".CLPI") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(clipinfDir, entry.Name()))
		if err != nil {
			lastErr = err
			continue
		}

		codecs, err := parseBlurayClipInfoCodecs(data)
		if err != nil {
			lastErr = err
			continue
		}
		mergeSourceCodecs(merged, codecs)
		anySuccess = true
	}

	if !anySuccess {
		if lastErr != nil {
			return nil, fmt.Errorf("failed to parse any CLPI file: %w", lastErr)
		}
		return nil, fmt.Errorf("no valid CLPI files found")
	}
	return merged, nil
}
