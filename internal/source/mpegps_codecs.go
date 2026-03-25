package source

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// detectDVDCodecs extracts codec information from an already-indexed DVD source.
// The MPEG-PS parser has already identified video and audio streams during indexing.
func detectDVDCodecs(index *Index) (*SourceCodecs, error) {
	codecs := &SourceCodecs{}

	for _, esReader := range index.ESReaders {
		parser, ok := esReader.(*MPEGPSParser)
		if !ok {
			continue
		}

		// Video: DVD is MPEG-2 if video ranges exist
		if parser.TotalESSize(true) > 0 {
			if !containsCodec(codecs.VideoCodecs, CodecMPEG2Video) {
				codecs.VideoCodecs = append(codecs.VideoCodecs, CodecMPEG2Video)
			}
		}

		// Audio from sub-streams (Private Stream 1 and MPEG-1 audio)
		for _, subStreamID := range parser.AudioSubStreams() {
			var ct CodecType
			switch {
			case subStreamID >= 0x80 && subStreamID <= 0x87:
				ct = CodecAC3Audio
			case subStreamID >= 0x88 && subStreamID <= 0x8F:
				ct = CodecDTSAudio
			case subStreamID >= 0xA0 && subStreamID <= 0xA7:
				ct = CodecLPCMAudio
			case subStreamID >= 0xC0 && subStreamID <= 0xDF:
				ct = CodecMPEGAudio
			default:
				continue
			}
			if !containsCodec(codecs.AudioCodecs, ct) {
				codecs.AudioCodecs = append(codecs.AudioCodecs, ct)
			}
		}
	}

	return codecs, nil
}

// detectDVDCodecsFromFile detects codecs from a DVD ISO by parsing VTS IFO
// metadata files. IFO files authoritatively declare every stream in each title
// set, unlike PES scanning which can miss audio streams that appear later in
// the VOB data. Falls back to PES scanning if IFO parsing fails.
func detectDVDCodecsFromFile(path string) (*SourceCodecs, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open ISO file: %w", err)
	}
	defer f.Close()

	// Try IFO-based detection first (ISO9660, then UDF).
	if ifos := findIFOsInISO(f); len(ifos) > 0 {
		codecs, err := detectDVDCodecsFromIFOs(f, ifos)
		if err == nil {
			return codecs, nil
		}
	}
	if ifos, err := findIFOsInUDF(f); err == nil && len(ifos) > 0 {
		codecs, err := detectDVDCodecsFromIFOs(f, ifos)
		if err == nil {
			return codecs, nil
		}
	}

	// Fallback: scan PES data from VOBs.
	return detectDVDCodecsFromFilePES(f)
}

// detectDVDCodecsFromFilePES scans PES start codes in VOB data to detect codecs.
// This is the legacy approach, kept as a fallback for ISOs where IFO parsing fails.
func detectDVDCodecsFromFilePES(f *os.File) (*SourceCodecs, error) {
	vobs := findContentVOBs(f)
	if len(vobs) == 0 {
		return scanDVDRegion(f, 0) // fallback: scan from start of ISO
	}

	merged := &SourceCodecs{}
	var lastErr error
	anySuccess := false
	for _, v := range significantFiles(vobs) {
		codecs, err := scanDVDRegion(f, v.Offset)
		if err != nil {
			lastErr = err
			continue
		}
		mergeSourceCodecs(merged, codecs)
		anySuccess = true
	}
	if !anySuccess {
		// Fall back to scanning from start of ISO
		fallback, err := scanDVDRegion(f, 0)
		if err == nil {
			return fallback, nil
		}
		if lastErr != nil {
			return nil, fmt.Errorf("failed to scan any DVD VOBs: %w", lastErr)
		}
		return nil, err
	}
	return merged, nil
}

// scanDVDRegion reads 4MB from the given offset and scans for MPEG-PS codecs.
func scanDVDRegion(f *os.File, offset int64) (*SourceCodecs, error) {
	const scanSize = 4 * 1024 * 1024
	buf := make([]byte, scanSize)
	n, err := f.ReadAt(buf, offset)
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		buf = buf[:n]
	} else if err != nil {
		return nil, fmt.Errorf("read %s at offset %d: %w", f.Name(), offset, err)
	}
	if n == 0 {
		return nil, fmt.Errorf("no data at offset %d in %s", offset, f.Name())
	}
	return scanPESCodecs(buf)
}

// scanPESCodecs scans a byte buffer for MPEG-PS PES headers and extracts codec information.
func scanPESCodecs(buf []byte) (*SourceCodecs, error) {
	codecs := &SourceCodecs{}

	// Scan for PES start codes: 0x00 0x00 0x01 <stream_id>
	for i := 0; i+3 < len(buf); i++ {
		if buf[i] != 0x00 || buf[i+1] != 0x00 || buf[i+2] != 0x01 {
			continue
		}
		streamID := buf[i+3]

		switch {
		case streamID >= 0xE0 && streamID <= 0xEF:
			// Video stream — DVD is MPEG-2
			if !containsCodec(codecs.VideoCodecs, CodecMPEG2Video) {
				codecs.VideoCodecs = append(codecs.VideoCodecs, CodecMPEG2Video)
			}

		case streamID == 0xBD:
			// Private Stream 1 — contains AC3, DTS, LPCM sub-streams
			// Parse the PES header to get to the sub-stream ID
			if i+9 < len(buf) {
				pesHeaderLen := int(buf[i+8])
				subStreamOffset := i + 9 + pesHeaderLen
				if subStreamOffset < len(buf) {
					subStreamID := buf[subStreamOffset]
					var ct CodecType
					switch {
					case subStreamID >= 0x80 && subStreamID <= 0x87:
						ct = CodecAC3Audio
					case subStreamID >= 0x88 && subStreamID <= 0x8F:
						ct = CodecDTSAudio
					case subStreamID >= 0xA0 && subStreamID <= 0xA7:
						ct = CodecLPCMAudio
					}
					if ct != CodecUnknown && !containsCodec(codecs.AudioCodecs, ct) {
						codecs.AudioCodecs = append(codecs.AudioCodecs, ct)
					}
				}
			}

		case streamID >= 0xC0 && streamID <= 0xDF:
			// MPEG audio stream
			if !containsCodec(codecs.AudioCodecs, CodecMPEGAudio) {
				codecs.AudioCodecs = append(codecs.AudioCodecs, CodecMPEGAudio)
			}
		}
	}

	if len(codecs.VideoCodecs) == 0 && len(codecs.AudioCodecs) == 0 {
		return nil, fmt.Errorf("no DVD codecs detected in scanned region")
	}

	return codecs, nil
}

// findContentVOBs navigates the ISO9660 filesystem to find all content VOBs
// (VTS_xx_1.VOB, the first part of each title set). Returns nil if navigation
// fails, signaling the caller to fall back to scanning from the ISO start.
// Uses readISOPVDRoot/readISODirectory/findISOEntry from iso.go.
func findContentVOBs(f *os.File) []isoFileExtent {
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

	// Collect VTS_xx_1.VOB entries — the first content VOB of each title set.
	// VTS_xx_0.VOB is navigation-only, and VTS_xx_2+ are continuations with
	// the same audio layout, so only _1 from each title set is needed.
	var vobs []isoFileExtent
	for _, e := range vtsEntries {
		if e.IsDir {
			continue
		}
		if strings.HasPrefix(e.Name, "VTS_") && strings.HasSuffix(e.Name, ".VOB") {
			if len(e.Name) == 12 && e.Name[7] == '1' {
				vobs = append(vobs, e)
			}
		}
	}
	return vobs
}
