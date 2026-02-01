package source

import (
	"fmt"
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
	default:
		return CodecUnknown
	}
}

// SourceCodecs describes the codecs found in a source media.
type SourceCodecs struct {
	VideoCodecs []CodecType
	AudioCodecs []CodecType
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
