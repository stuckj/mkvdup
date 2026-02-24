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
