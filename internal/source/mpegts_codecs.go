package source

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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

// detectBlurayCodecsFromFile parses the PMT from an M2TS file (or Blu-ray ISO)
// to detect codecs. For ISOs, it finds the largest M2TS file within the ISO
// and reads from that region.
func detectBlurayCodecsFromFile(path string) (*SourceCodecs, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	// Determine read offset: for ISOs, find the largest M2TS within
	readOffset := int64(0)
	if strings.HasSuffix(strings.ToLower(path), ".iso") {
		m2tsFiles, err := findBlurayM2TSInISO(path)
		if err != nil {
			return nil, fmt.Errorf("find M2TS in ISO: %w", err)
		}
		// Find the largest M2TS (most likely the main feature)
		var largest isoFileExtent
		for _, m := range m2tsFiles {
			if m.Size > largest.Size {
				largest = m
			}
		}
		if largest.Size == 0 {
			return nil, fmt.Errorf("no M2TS files found in Blu-ray ISO")
		}
		readOffset = largest.Offset
	}

	// Read 2MB from the M2TS data — enough to find PAT + PMT
	const scanSize = 2 * 1024 * 1024
	buf := make([]byte, scanSize)
	n, err := f.ReadAt(buf, readOffset)
	if err != nil && n == 0 {
		return nil, fmt.Errorf("read M2TS data: %w", err)
	}
	buf = buf[:n]

	// Need at least enough data for TS packet size detection (4 sync bytes at regular intervals)
	if len(buf) < 192*4 {
		return nil, fmt.Errorf("M2TS data too small to detect TS structure (%d bytes)", len(buf))
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
