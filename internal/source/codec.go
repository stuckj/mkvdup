package source

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/stuckj/mkvdup/internal/mkv"
)

// CodecType represents a broad codec family.
type CodecType int

// Codec type constants.
const (
	CodecUnknown CodecType = iota
	CodecMPEG1Video
	CodecMPEG2Video
	CodecH264Video
	CodecH265Video
	CodecVC1Video
	CodecAC3Audio
	CodecEAC3Audio
	CodecDTSAudio
	CodecDTSHDAudio
	CodecTrueHDAudio
	CodecLPCMAudio
	CodecMPEGAudio
	CodecAACaudio
	CodecFLACAudio
	CodecOpusAudio
	CodecPGSSubtitle
)

// CodecTypeName returns a human-readable name for a codec type.
func CodecTypeName(ct CodecType) string {
	switch ct {
	case CodecMPEG1Video:
		return "MPEG-1"
	case CodecMPEG2Video:
		return "MPEG-2"
	case CodecH264Video:
		return "H.264"
	case CodecH265Video:
		return "H.265"
	case CodecVC1Video:
		return "VC-1"
	case CodecAC3Audio:
		return "AC3"
	case CodecEAC3Audio:
		return "E-AC3"
	case CodecDTSAudio:
		return "DTS"
	case CodecDTSHDAudio:
		return "DTS-HD"
	case CodecTrueHDAudio:
		return "TrueHD"
	case CodecLPCMAudio:
		return "LPCM"
	case CodecMPEGAudio:
		return "MPEG Audio"
	case CodecAACaudio:
		return "AAC"
	case CodecFLACAudio:
		return "FLAC"
	case CodecOpusAudio:
		return "Opus"
	case CodecPGSSubtitle:
		return "PGS"
	default:
		return "Unknown"
	}
}

// IsVideoCodec returns true if the codec type is a video codec.
func IsVideoCodec(ct CodecType) bool {
	switch ct {
	case CodecMPEG1Video, CodecMPEG2Video, CodecH264Video, CodecH265Video, CodecVC1Video:
		return true
	}
	return false
}

// IsSubtitleCodec returns true if the codec type is a subtitle codec.
func IsSubtitleCodec(ct CodecType) bool {
	return ct == CodecPGSSubtitle
}

// IsAudioCodec returns true if the codec type is an audio codec.
func IsAudioCodec(ct CodecType) bool {
	switch ct {
	case CodecAC3Audio, CodecEAC3Audio, CodecDTSAudio, CodecDTSHDAudio,
		CodecTrueHDAudio, CodecLPCMAudio, CodecMPEGAudio, CodecAACaudio,
		CodecFLACAudio, CodecOpusAudio:
		return true
	}
	return false
}

// MKVCodecToType maps an MKV CodecID string to a CodecType.
func MKVCodecToType(codecID string) CodecType {
	switch {
	case codecID == "V_MPEG1":
		return CodecMPEG1Video
	case codecID == "V_MPEG2":
		return CodecMPEG2Video
	case codecID == "V_MPEG4/ISO/AVC":
		return CodecH264Video
	case codecID == "V_MPEGH/ISO/HEVC":
		return CodecH265Video
	case codecID == "V_MS/VFW/FOURCC":
		// Could be VC-1 or other; can't determine without codec private data
		return CodecUnknown
	case codecID == "A_AC3":
		return CodecAC3Audio
	case codecID == "A_EAC3":
		return CodecEAC3Audio
	case codecID == "A_DTS":
		return CodecDTSAudio
	case strings.HasPrefix(codecID, "A_DTS/"):
		// A_DTS/EXPRESS, A_DTS/LOSSLESS, etc.
		return CodecDTSHDAudio
	case codecID == "A_TRUEHD":
		return CodecTrueHDAudio
	case strings.HasPrefix(codecID, "A_PCM/"):
		// A_PCM/INT/LIT, A_PCM/INT/BIG, A_PCM/FLOAT/IEEE
		return CodecLPCMAudio
	case strings.HasPrefix(codecID, "A_MPEG/"):
		// A_MPEG/L2, A_MPEG/L3
		return CodecMPEGAudio
	case strings.HasPrefix(codecID, "A_AAC"):
		// A_AAC, A_AAC/MPEG2/MAIN, etc.
		return CodecAACaudio
	case codecID == "A_FLAC":
		return CodecFLACAudio
	case codecID == "A_OPUS":
		return CodecOpusAudio
	case codecID == "S_HDMV/PGS":
		return CodecPGSSubtitle
	default:
		return CodecUnknown
	}
}

// SourceCodecs describes the codecs found in a source media.
type SourceCodecs struct {
	VideoCodecs    []CodecType
	AudioCodecs    []CodecType
	SubtitleCodecs []CodecType
}

// CodecMismatch describes a detected codec mismatch between MKV and source.
type CodecMismatch struct {
	TrackType    string      // "video" or "audio"
	MKVCodecID   string      // e.g. "V_MPEG4/ISO/AVC"
	MKVCodecType CodecType   // resolved codec type
	SourceCodecs []CodecType // codecs found in source for this track type
}

// DetectSourceCodecs determines what codecs are present in the source media.
// For DVD sources, it extracts codec info from the already-parsed MPEG-PS data.
// For Blu-ray sources, it performs a lightweight PMT scan of the first M2TS file.
func DetectSourceCodecs(index *Index) (*SourceCodecs, error) {
	switch index.SourceType {
	case TypeDVD:
		return detectDVDCodecs(index)
	case TypeBluray:
		return detectBlurayCodecs(index)
	default:
		return nil, fmt.Errorf("unknown source type")
	}
}

// DetectSourceCodecsFromDir performs a lightweight codec detection from a source
// directory without building the full hash index. This allows codec compatibility
// checks to run before the expensive indexing step.
func DetectSourceCodecsFromDir(sourceDir string) (*SourceCodecs, error) {
	sourceType, err := DetectType(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("detect source type: %w", err)
	}

	files, err := EnumerateMediaFiles(sourceDir, sourceType)
	if err != nil {
		return nil, fmt.Errorf("enumerate files: %w", err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no media files found in %s", sourceDir)
	}

	// Find the largest file (most likely the main feature)
	var largestFile string
	var largestSize int64
	for _, f := range files {
		fullPath := filepath.Join(sourceDir, f)
		info, err := os.Stat(fullPath)
		if err != nil {
			continue
		}
		if info.Size() > largestSize {
			largestSize = info.Size()
			largestFile = f
		}
	}
	if largestFile == "" {
		return nil, fmt.Errorf("no accessible media files found")
	}

	fullPath := filepath.Join(sourceDir, largestFile)

	switch sourceType {
	case TypeBluray:
		return detectBlurayCodecsFromFile(fullPath)
	case TypeDVD:
		return detectDVDCodecsFromFile(fullPath)
	default:
		return nil, fmt.Errorf("unknown source type")
	}
}

// detectDVDCodecsFromFile performs a lightweight scan of an ISO file to detect
// MPEG-PS stream types without building the full MPEG-PS index.
func detectDVDCodecsFromFile(path string) (*SourceCodecs, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open ISO file: %w", err)
	}
	defer f.Close()

	// Find the main content VOB inside the ISO to avoid scanning menu VOBs
	// (which may have different audio codecs than the main feature).
	scanOffset := findMainVOBOffset(f)

	const scanSize = 4 * 1024 * 1024
	buf := make([]byte, scanSize)
	n, err := f.ReadAt(buf, scanOffset)
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		buf = buf[:n]
	} else if err != nil {
		return nil, fmt.Errorf("read ISO file: %w", err)
	}
	if n == 0 {
		return nil, fmt.Errorf("no data at scan offset %d in %s", scanOffset, path)
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

// findMainVOBOffset navigates the ISO9660 filesystem to find the byte offset of
// the main content VOB (VTS_xx_1.VOB or larger). Returns 0 if navigation fails,
// falling back to scanning from the ISO start.
func findMainVOBOffset(f *os.File) int64 {
	const sectorSize = 2048

	// Read the primary volume descriptor at sector 16
	pvd := make([]byte, sectorSize)
	if _, err := f.ReadAt(pvd, 16*sectorSize); err != nil {
		return 0
	}
	if pvd[0] != 1 || string(pvd[1:6]) != "CD001" {
		return 0
	}

	// Root directory record at offset 156
	rootDirRecord := pvd[156:]
	if len(rootDirRecord) < 34 {
		return 0
	}
	rootExtent := uint32(rootDirRecord[2]) | uint32(rootDirRecord[3])<<8 |
		uint32(rootDirRecord[4])<<16 | uint32(rootDirRecord[5])<<24
	rootDataLen := uint32(rootDirRecord[10]) | uint32(rootDirRecord[11])<<8 |
		uint32(rootDirRecord[12])<<16 | uint32(rootDirRecord[13])<<24
	if rootDataLen > 16*1024 {
		rootDataLen = 16 * 1024
	}

	// Read root directory
	rootDir := make([]byte, rootDataLen)
	if _, err := f.ReadAt(rootDir, int64(rootExtent)*sectorSize); err != nil {
		return 0
	}

	// Find VIDEO_TS directory entry
	videoTSExtent, videoTSLen := findDirEntry(rootDir, "VIDEO_TS", sectorSize)
	if videoTSExtent == 0 {
		return 0
	}

	// Read VIDEO_TS directory
	if videoTSLen > 64*1024 {
		videoTSLen = 64 * 1024
	}
	videoTSDir := make([]byte, videoTSLen)
	if _, err := f.ReadAt(videoTSDir, int64(videoTSExtent)*sectorSize); err != nil {
		return 0
	}

	// Find the largest VTS content VOB (VTS_xx_1.VOB through VTS_xx_9.VOB).
	// These contain the main feature; VTS_xx_0.VOB is navigation-only.
	var bestExtent uint32
	var bestSize uint32
	offset := 0
	for offset < len(videoTSDir) {
		recLen := int(videoTSDir[offset])
		if recLen == 0 {
			nextSector := ((offset / sectorSize) + 1) * sectorSize
			if nextSector >= len(videoTSDir) {
				break
			}
			offset = nextSector
			continue
		}
		if offset+33 > len(videoTSDir) {
			break
		}
		nameLen := int(videoTSDir[offset+32])
		if offset+33+nameLen > len(videoTSDir) || offset+recLen > len(videoTSDir) {
			break
		}

		name := strings.ToUpper(string(videoTSDir[offset+33 : offset+33+nameLen]))
		if idx := strings.Index(name, ";"); idx >= 0 {
			name = name[:idx]
		}

		// Match VTS_xx_N.VOB where N >= 1 (content VOBs, not navigation)
		if strings.HasPrefix(name, "VTS_") && strings.HasSuffix(name, ".VOB") {
			// Check digit before .VOB: VTS_01_1.VOB → name[7] is the content number
			if len(name) == 12 && name[7] >= '1' && name[7] <= '9' {
				extent := uint32(videoTSDir[offset+2]) | uint32(videoTSDir[offset+3])<<8 |
					uint32(videoTSDir[offset+4])<<16 | uint32(videoTSDir[offset+5])<<24
				size := uint32(videoTSDir[offset+10]) | uint32(videoTSDir[offset+11])<<8 |
					uint32(videoTSDir[offset+12])<<16 | uint32(videoTSDir[offset+13])<<24
				if size > bestSize {
					bestSize = size
					bestExtent = extent
				}
			}
		}

		offset += recLen
	}

	if bestExtent > 0 {
		return int64(bestExtent) * sectorSize
	}
	return 0
}

// findDirEntry searches an ISO9660 directory for a named entry and returns its
// extent location and data length. Returns (0, 0) if not found.
func findDirEntry(dirData []byte, targetName string, sectorSize int) (uint32, uint32) {
	targetName = strings.ToUpper(targetName)
	offset := 0
	for offset < len(dirData) {
		recLen := int(dirData[offset])
		if recLen == 0 {
			nextSector := ((offset / sectorSize) + 1) * sectorSize
			if nextSector >= len(dirData) {
				break
			}
			offset = nextSector
			continue
		}
		if offset+33 > len(dirData) {
			break
		}
		nameLen := int(dirData[offset+32])
		if offset+33+nameLen > len(dirData) || offset+recLen > len(dirData) {
			break
		}

		name := strings.ToUpper(string(dirData[offset+33 : offset+33+nameLen]))
		if idx := strings.Index(name, ";"); idx >= 0 {
			name = name[:idx]
		}
		name = strings.TrimSuffix(name, ".")

		if name == targetName {
			extent := uint32(dirData[offset+2]) | uint32(dirData[offset+3])<<8 |
				uint32(dirData[offset+4])<<16 | uint32(dirData[offset+5])<<24
			dataLen := uint32(dirData[offset+10]) | uint32(dirData[offset+11])<<8 |
				uint32(dirData[offset+12])<<16 | uint32(dirData[offset+13])<<24
			return extent, dataLen
		}

		offset += recLen
	}
	return 0, 0
}

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

		// Audio from Private Stream 1 sub-streams
		for _, subStreamID := range parser.AudioSubStreams() {
			var ct CodecType
			switch {
			case subStreamID >= 0x80 && subStreamID <= 0x87:
				ct = CodecAC3Audio
			case subStreamID >= 0x88 && subStreamID <= 0x8F:
				ct = CodecDTSAudio
			case subStreamID >= 0xA0 && subStreamID <= 0xA7:
				ct = CodecLPCMAudio
			default:
				continue
			}
			if !containsCodec(codecs.AudioCodecs, ct) {
				codecs.AudioCodecs = append(codecs.AudioCodecs, ct)
			}
		}

		// MPEG audio streams (stream IDs 0xC0-0xDF)
		for _, pkt := range parser.Packets() {
			if pkt.StreamID >= 0xC0 && pkt.StreamID <= 0xDF {
				if !containsCodec(codecs.AudioCodecs, CodecMPEGAudio) {
					codecs.AudioCodecs = append(codecs.AudioCodecs, CodecMPEGAudio)
				}
				break // Only need to find one
			}
		}
	}

	return codecs, nil
}

// detectBlurayCodecs performs a lightweight scan of the first M2TS file
// to detect codecs via the MPEG-TS Program Map Table (PMT).
func detectBlurayCodecs(index *Index) (*SourceCodecs, error) {
	if len(index.Files) == 0 {
		return nil, fmt.Errorf("no source files in index")
	}

	// Find the largest M2TS file (most likely the main feature)
	var largestFile string
	var largestSize int64
	for _, f := range index.Files {
		if f.Size > largestSize {
			largestSize = f.Size
			largestFile = f.RelativePath
		}
	}

	if largestFile == "" {
		return nil, fmt.Errorf("no valid M2TS files found")
	}

	fullPath := filepath.Join(index.SourceDir, largestFile)
	return detectBlurayCodecsFromFile(fullPath)
}

// detectBlurayCodecsFromFile parses the PMT from an M2TS file to detect codecs.
func detectBlurayCodecsFromFile(path string) (*SourceCodecs, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open M2TS file: %w", err)
	}
	defer f.Close()

	// Read first 2MB — enough to find PAT + PMT
	const scanSize = 2 * 1024 * 1024
	buf := make([]byte, scanSize)
	n, err := f.Read(buf)
	if err != nil && n == 0 {
		return nil, fmt.Errorf("read M2TS file: %w", err)
	}
	buf = buf[:n]

	// Need at least enough data for TS packet size detection (4 sync bytes at regular intervals)
	if len(buf) < 192*4 {
		return nil, fmt.Errorf("M2TS file too small to detect TS structure (%d bytes)", len(buf))
	}

	return parseTSCodecs(buf)
}

// parseTSCodecs scans MPEG-TS data to find the PAT and PMT and extract stream types.
func parseTSCodecs(data []byte) (*SourceCodecs, error) {
	// Detect TS packet size: 188 (standard) or 192 (M2TS with 4-byte timestamp)
	packetSize, offset := detectTSPacketSize(data)
	if packetSize == 0 {
		return nil, fmt.Errorf("cannot detect TS packet size")
	}

	// Step 1: Find PAT (PID 0x0000) to get PMT PID
	pmtPID := uint16(0)
	for i := offset; i+packetSize <= len(data); i += packetSize {
		tsOffset := i
		if packetSize == 192 {
			tsOffset += 4 // Skip 4-byte M2TS timestamp
		}
		if tsOffset+188 > len(data) {
			break
		}
		pkt := data[tsOffset : tsOffset+188]
		if pkt[0] != 0x47 {
			continue // Not a valid TS sync byte
		}

		pid := uint16(pkt[1]&0x1F)<<8 | uint16(pkt[2])
		if pid != 0x0000 {
			continue
		}

		// PAT found — parse it
		payloadStart := pkt[1]&0x40 != 0
		if !payloadStart {
			continue
		}

		// Skip adaptation field if present
		adaptationFieldControl := (pkt[3] >> 4) & 0x03
		headerLen := 4
		switch adaptationFieldControl {
		case 0x02, 0x03: // Adaptation field present
			adaptLen := int(pkt[4])
			if adaptLen > 183 {
				continue
			}
			headerLen = 5 + adaptLen
		case 0x01: // Payload only, no adaptation field
		default: // 0x00 is reserved/invalid
			continue
		}
		if headerLen >= 188 {
			continue
		}

		// Skip pointer field
		pointerField := int(pkt[headerLen])
		headerLen += 1 + pointerField
		if headerLen+8 > 188 {
			continue
		}

		payload := pkt[headerLen:]
		// PAT: table_id(1) + flags+length(2) + tsid(2) + version(1) + section(1) + last_section(1)
		// then 4 bytes per program: program_number(2) + PMT_PID(2)
		if len(payload) < 12 {
			continue
		}
		if payload[0] != 0x00 { // table_id for PAT
			continue
		}

		sectionLength := int(payload[1]&0x0F)<<8 | int(payload[2])
		if sectionLength < 9 {
			continue
		}

		// Programs start at offset 8, each is 4 bytes
		programsEnd := 8 + sectionLength - 4 // -4 for CRC
		if programsEnd > len(payload) {
			programsEnd = len(payload) - 4
		}

		for j := 8; j+4 <= programsEnd; j += 4 {
			progNum := uint16(payload[j])<<8 | uint16(payload[j+1])
			if progNum == 0 {
				continue // Network PID, skip
			}
			pmtPID = uint16(payload[j+2]&0x1F)<<8 | uint16(payload[j+3])
			break // Use the first program
		}
		break
	}

	if pmtPID == 0 {
		return nil, fmt.Errorf("PMT PID not found in PAT")
	}

	// Step 2: Find PMT and extract stream types
	codecs := &SourceCodecs{}
	for i := offset; i+packetSize <= len(data); i += packetSize {
		tsOffset := i
		if packetSize == 192 {
			tsOffset += 4
		}
		if tsOffset+188 > len(data) {
			break
		}
		pkt := data[tsOffset : tsOffset+188]
		if pkt[0] != 0x47 {
			continue
		}

		pid := uint16(pkt[1]&0x1F)<<8 | uint16(pkt[2])
		if pid != pmtPID {
			continue
		}

		payloadStart := pkt[1]&0x40 != 0
		if !payloadStart {
			continue
		}

		// Skip adaptation field
		adaptationFieldControl := (pkt[3] >> 4) & 0x03
		headerLen := 4
		switch adaptationFieldControl {
		case 0x02, 0x03: // Adaptation field present
			adaptLen := int(pkt[4])
			if adaptLen > 183 {
				continue
			}
			headerLen = 5 + adaptLen
		case 0x01: // Payload only, no adaptation field
		default: // 0x00 is reserved/invalid
			continue
		}
		if headerLen >= 188 {
			continue
		}

		// Skip pointer field
		pointerField := int(pkt[headerLen])
		headerLen += 1 + pointerField
		if headerLen+12 > 188 {
			continue
		}

		payload := pkt[headerLen:]
		if len(payload) < 12 || payload[0] != 0x02 { // table_id for PMT
			continue
		}

		sectionLength := int(payload[1]&0x0F)<<8 | int(payload[2])
		if sectionLength < 13 {
			continue
		}

		// Program info length at offset 10
		progInfoLen := int(payload[10]&0x0F)<<8 | int(payload[11])

		// Stream descriptors start after program info
		streamsStart := 12 + progInfoLen
		streamsEnd := 3 + sectionLength - 4 // section starts at byte 3, -4 for CRC
		if streamsEnd > len(payload) {
			streamsEnd = len(payload) - 4
		}
		if streamsStart > streamsEnd {
			continue
		}

		for j := streamsStart; j+5 <= streamsEnd; {
			streamType := payload[j]
			esInfoLen := int(payload[j+3]&0x0F)<<8 | int(payload[j+4])

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

			next := j + 5 + esInfoLen
			if next < j || next > streamsEnd {
				break
			}
			j = next
		}

		break // Found and parsed PMT
	}

	return codecs, nil
}

// tsStreamTypeToCodecType maps MPEG-TS stream type values to CodecType.
func tsStreamTypeToCodecType(streamType byte) CodecType {
	switch streamType {
	case 0x01:
		return CodecMPEG1Video
	case 0x02:
		return CodecMPEG2Video
	case 0x1B:
		return CodecH264Video
	case 0x24:
		return CodecH265Video
	case 0xEA:
		return CodecVC1Video
	case 0x03, 0x04:
		return CodecMPEGAudio
	case 0x0F:
		return CodecAACaudio
	case 0x80:
		return CodecLPCMAudio
	case 0x81:
		return CodecAC3Audio
	case 0x82:
		return CodecDTSAudio
	case 0x83:
		return CodecTrueHDAudio
	case 0x84:
		return CodecEAC3Audio
	case 0x85, 0x86:
		return CodecDTSHDAudio
	case 0x90:
		return CodecPGSSubtitle
	default:
		return CodecUnknown
	}
}

// detectTSPacketSize determines TS packet size (188 or 192) and the offset to
// the first sync byte. Returns (0, 0) if no valid TS structure is found.
func detectTSPacketSize(data []byte) (int, int) {
	// Try both M2TS (192-byte packets) and standard TS (188-byte packets)
	for _, size := range []int{192, 188} {
		for startOffset := 0; startOffset < size && startOffset+size*3 < len(data); startOffset++ {
			syncOffset := startOffset
			if size == 192 {
				syncOffset += 4 // M2TS timestamp prefix
			}
			if syncOffset >= len(data) || data[syncOffset] != 0x47 {
				continue
			}
			// Verify 3 consecutive sync bytes
			valid := true
			for k := 1; k <= 3; k++ {
				nextSync := startOffset + k*size
				if size == 192 {
					nextSync += 4
				}
				if nextSync >= len(data) || data[nextSync] != 0x47 {
					valid = false
					break
				}
			}
			if valid {
				return size, startOffset
			}
		}
	}
	return 0, 0
}

// CheckCodecCompatibility compares MKV track codecs against source codecs.
// Returns nil if all codecs are compatible, or a list of mismatches.
func CheckCodecCompatibility(tracks []mkv.Track, sourceCodecs *SourceCodecs) []CodecMismatch {
	var mismatches []CodecMismatch

	for _, track := range tracks {
		ct := MKVCodecToType(track.CodecID)
		if ct == CodecUnknown {
			continue // Skip unknown codecs — no false alarms
		}

		if track.Type == mkv.TrackTypeVideo && IsVideoCodec(ct) {
			if len(sourceCodecs.VideoCodecs) == 0 {
				continue // No source video info available
			}
			if !codecFamilyMatch(ct, sourceCodecs.VideoCodecs) {
				mismatches = append(mismatches, CodecMismatch{
					TrackType:    "video",
					MKVCodecID:   track.CodecID,
					MKVCodecType: ct,
					SourceCodecs: sourceCodecs.VideoCodecs,
				})
			}
		} else if track.Type == mkv.TrackTypeAudio && IsAudioCodec(ct) {
			if len(sourceCodecs.AudioCodecs) == 0 {
				continue // No source audio info available
			}
			if !codecFamilyMatch(ct, sourceCodecs.AudioCodecs) {
				mismatches = append(mismatches, CodecMismatch{
					TrackType:    "audio",
					MKVCodecID:   track.CodecID,
					MKVCodecType: ct,
					SourceCodecs: sourceCodecs.AudioCodecs,
				})
			}
		} else if track.Type == mkv.TrackTypeSubtitle && IsSubtitleCodec(ct) {
			if len(sourceCodecs.SubtitleCodecs) == 0 {
				continue // No source subtitle info available
			}
			if !codecFamilyMatch(ct, sourceCodecs.SubtitleCodecs) {
				mismatches = append(mismatches, CodecMismatch{
					TrackType:    "subtitle",
					MKVCodecID:   track.CodecID,
					MKVCodecType: ct,
					SourceCodecs: sourceCodecs.SubtitleCodecs,
				})
			}
		}
	}

	return mismatches
}

// codecFamilyMatch checks if a codec type is compatible with any codec in the list.
// Uses family-based matching (e.g., DTS is compatible with DTS-HD).
func codecFamilyMatch(ct CodecType, sourceCodecs []CodecType) bool {
	family := codecFamily(ct)
	for _, sc := range sourceCodecs {
		if codecFamily(sc) == family {
			return true
		}
	}
	return false
}

// codecFamily returns the codec family for family-based matching.
// Related codecs map to the same family value.
func codecFamily(ct CodecType) int {
	switch ct {
	case CodecMPEG1Video, CodecMPEG2Video:
		return 1
	case CodecH264Video:
		return 2
	case CodecH265Video:
		return 3
	case CodecVC1Video:
		return 4
	case CodecAC3Audio, CodecEAC3Audio:
		return 10
	case CodecDTSAudio, CodecDTSHDAudio:
		return 11
	case CodecTrueHDAudio:
		return 12
	case CodecLPCMAudio:
		return 13
	case CodecMPEGAudio:
		return 14
	case CodecAACaudio:
		return 15
	case CodecFLACAudio:
		return 16
	case CodecOpusAudio:
		return 17
	case CodecPGSSubtitle:
		return 20
	default:
		return 0
	}
}

// containsCodec checks if a codec type is already in the list.
func containsCodec(codecs []CodecType, ct CodecType) bool {
	for _, c := range codecs {
		if c == ct {
			return true
		}
	}
	return false
}
