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
// This uses reassemblePSISection to correctly handle PMTs that span multiple TS
// packets (common on Blu-rays with many audio and subtitle streams).
func parseTSCodecs(data []byte) (*SourceCodecs, error) {
	// Detect TS packet size: 188 (standard) or 192 (M2TS with 4-byte timestamp)
	packetSize, startOffset := detectTSPacketSize(data)
	if packetSize == 0 {
		return nil, fmt.Errorf("cannot detect TS packet size")
	}
	tsOffset := 0
	if packetSize == 192 {
		tsOffset = 4
	}

	// Step 1: Find PAT (PID 0x0000) to get PMT PID
	patSection, err := reassemblePSISection(data, startOffset, packetSize, tsOffset, 0, 0x00)
	if err != nil {
		return nil, fmt.Errorf("find PAT: %w", err)
	}

	pmtPID := pmtPIDFromPAT(patSection)
	if pmtPID == 0 {
		return nil, fmt.Errorf("PMT PID not found in PAT")
	}

	// Step 2: Reassemble complete PMT section (may span multiple TS packets)
	pmtSection, err := reassemblePSISection(data, startOffset, packetSize, tsOffset, pmtPID, 0x02)
	if err != nil {
		return nil, fmt.Errorf("find PMT: %w", err)
	}

	// Step 3: Extract stream types from the reassembled PMT
	codecs := &SourceCodecs{}
	if len(pmtSection) >= 12 {
		progInfoLen := int(pmtSection[10]&0x0F)<<8 | int(pmtSection[11])
		streamsStart := 12 + progInfoLen
		sectionLen := int(pmtSection[1]&0x0F)<<8 | int(pmtSection[2])
		streamsEnd := 3 + sectionLen - 4 // exclude CRC32
		if streamsEnd > len(pmtSection) {
			streamsEnd = len(pmtSection)
		}

		for j := streamsStart; j+5 <= streamsEnd; {
			streamType := pmtSection[j]
			esInfoLen := int(pmtSection[j+3]&0x0F)<<8 | int(pmtSection[j+4])

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
