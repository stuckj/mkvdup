package source

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// MPEG-PS start codes
const (
	PackStartCode      = 0x000001BA
	SystemHeaderCode   = 0x000001BB
	ProgramEndCode     = 0x000001B9
	PrivateStream1Code = 0x000001BD
	PrivateStream2Code = 0x000001BF
	PaddingStreamCode  = 0x000001BE
	VideoStreamMinCode = 0x000001E0
	VideoStreamMaxCode = 0x000001EF
	AudioStreamMinCode = 0x000001C0
	AudioStreamMaxCode = 0x000001DF
)

// PESPacket represents a parsed PES packet from an MPEG-PS stream.
type PESPacket struct {
	StreamID      byte  // Stream identifier (E0-EF = video, C0-DF = audio, BD = private)
	SubStreamID   byte  // Sub-stream ID for Private Stream 1 (0x80-0x87 = AC3, 0x88-0x8F = DTS)
	Offset        int64 // Offset of the PES packet start in the file
	HeaderSize    int   // Total header size (start code + length + PES header + private header)
	PayloadOffset int64 // Offset of the actual audio/video payload
	PayloadSize   int   // Size of the payload
	IsVideo       bool  // True if this is a video stream
	IsAudio       bool  // True if this is an audio stream
}

// PESPayloadRange represents a contiguous range of elementary stream payload data.
type PESPayloadRange struct {
	FileOffset int64 // Offset in the MPEG-PS file
	Size       int   // Size of this payload chunk
	ESOffset   int64 // Logical offset in the elementary stream
}

// MPEGPSParser parses MPEG Program Stream files to extract PES packet information.
type MPEGPSParser struct {
	data        []byte // Direct mmap'd data - zero-copy access
	size        int64
	packets     []PESPacket
	videoRanges []PESPayloadRange
	audioRanges []PESPayloadRange
	// Filtered ranges exclude user_data sections for MKV-compatible matching
	filteredVideoRanges []PESPayloadRange
	// Filtered audio ranges per sub-stream ID - separates interleaved audio tracks
	// Each sub-stream (0x80, 0x81, etc.) gets its own filtered range set
	filteredAudioBySubStream map[byte][]PESPayloadRange
	// audioSubStreams lists the sub-stream IDs in order of appearance
	audioSubStreams []byte
	filterUserData  bool
	// LPCM sub-stream tracking
	lpcmSubStreams map[byte]bool            // which sub-streams are LPCM
	lpcmInfo       map[byte]LPCMFrameHeader // parsed header per LPCM sub-stream
}

// NewMPEGPSParser creates a parser for the given memory-mapped data.
// The data slice should be from a zero-copy mmap (unix.Mmap).
func NewMPEGPSParser(data []byte) *MPEGPSParser {
	return &MPEGPSParser{
		data: data,
		size: int64(len(data)),
	}
}

// MPEGPSProgressFunc is called to report MPEG-PS parsing progress.
type MPEGPSProgressFunc func(processed, total int64)

// Parse scans the file and extracts all PES packet information.
func (p *MPEGPSParser) Parse() error {
	return p.ParseWithProgress(nil)
}

// ParseWithProgress scans the file with progress reporting.
func (p *MPEGPSParser) ParseWithProgress(progress MPEGPSProgressFunc) error {
	pos := int64(0)
	var videoESOffset, audioESOffset int64
	lastProgress := int64(0)

	// Pre-allocate slices to reduce reallocation churn
	// Estimate: average PES packet ~2KB, so ~size/2048 packets
	// We split roughly 60% video, 40% audio
	estimatedPackets := int(p.size / 2048)
	if estimatedPackets < 1000 {
		estimatedPackets = 1000
	}
	p.packets = make([]PESPacket, 0, estimatedPackets)
	p.videoRanges = make([]PESPayloadRange, 0, estimatedPackets*6/10)
	p.audioRanges = make([]PESPayloadRange, 0, estimatedPackets*4/10)

	for pos < p.size-4 {
		// Direct slice access - zero copy
		end := pos + 4*1024*1024 // Process in ~4MB logical chunks for progress
		if end > p.size {
			end = p.size
		}
		chunkData := p.data[pos:end]
		if len(chunkData) < 4 {
			break
		}

		// Scan for start codes within this chunk
		i := 0
		for i < len(chunkData)-4 {
			// Fast scan for 00 00 01 prefix
			if chunkData[i] != 0 {
				i++
				continue
			}
			if chunkData[i+1] != 0 {
				i += 2
				continue
			}
			if chunkData[i+2] != 1 {
				i++
				continue
			}

			// Found potential start code at pos + i
			startCodePos := pos + int64(i)
			startCode := uint32(0x00000100) | uint32(chunkData[i+3])

			advance := int64(1)

			switch {
			case startCode == PackStartCode:
				packSize, err := p.parsePackHeader(startCodePos)
				if err == nil {
					advance = int64(packSize)
				}

			case startCode == SystemHeaderCode:
				headerLen, err := p.parseSystemHeader(startCodePos)
				if err == nil {
					advance = int64(headerLen)
				}

			case startCode == ProgramEndCode:
				// End of program stream - but DVDs can have multiple programs
				// (menu, main feature, extras, etc.), so continue parsing
				advance = 4

			case startCode == PaddingStreamCode:
				length, err := p.readPESLength(startCodePos + 4)
				if err == nil {
					advance = 6 + int64(length)
				}

			case startCode == PrivateStream1Code:
				pkt, err := p.parsePESPacket(startCodePos, byte(startCode&0xFF))
				if err == nil {
					pkt.IsAudio = true
					p.packets = append(p.packets, pkt)
					p.audioRanges = append(p.audioRanges, PESPayloadRange{
						FileOffset: pkt.PayloadOffset,
						Size:       pkt.PayloadSize,
						ESOffset:   audioESOffset,
					})
					audioESOffset += int64(pkt.PayloadSize)
					advance = int64(pkt.HeaderSize + pkt.PayloadSize)
				}

			case startCode >= VideoStreamMinCode && startCode <= VideoStreamMaxCode:
				pkt, err := p.parsePESPacket(startCodePos, byte(startCode&0xFF))
				if err == nil {
					pkt.IsVideo = true
					p.packets = append(p.packets, pkt)
					p.videoRanges = append(p.videoRanges, PESPayloadRange{
						FileOffset: pkt.PayloadOffset,
						Size:       pkt.PayloadSize,
						ESOffset:   videoESOffset,
					})
					videoESOffset += int64(pkt.PayloadSize)
					advance = int64(pkt.HeaderSize + pkt.PayloadSize)
				}

			case startCode >= AudioStreamMinCode && startCode <= AudioStreamMaxCode:
				pkt, err := p.parsePESPacket(startCodePos, byte(startCode&0xFF))
				if err == nil {
					pkt.IsAudio = true
					p.packets = append(p.packets, pkt)
					p.audioRanges = append(p.audioRanges, PESPayloadRange{
						FileOffset: pkt.PayloadOffset,
						Size:       pkt.PayloadSize,
						ESOffset:   audioESOffset,
					})
					audioESOffset += int64(pkt.PayloadSize)
					advance = int64(pkt.HeaderSize + pkt.PayloadSize)
				}
			}

			// Move forward by the packet size (or 1 if unknown)
			newPos := startCodePos + advance
			i = int(newPos - pos)
		}

		// Move to next chunk, but back up slightly to catch start codes at boundaries
		pos += int64(len(chunkData)) - 3
		if pos < 0 {
			pos = 0
		}

		// Report progress
		if progress != nil && pos-lastProgress > 100*1024*1024 { // Every 100MB
			progress(pos, p.size)
			lastProgress = pos
		}
	}

	if progress != nil {
		progress(p.size, p.size)
	}

	// Build filtered video ranges that exclude user_data (B2) sections
	// This makes the ES compatible with what MKV tools produce
	if err := p.buildFilteredVideoRanges(); err != nil {
		return fmt.Errorf("build filtered video ranges: %w", err)
	}

	// Build filtered audio ranges that strip Private Stream 1 headers
	// (sub-stream ID and 2-byte pointer, keeping frame count byte)
	if err := p.buildFilteredAudioRanges(); err != nil {
		return fmt.Errorf("build filtered audio ranges: %w", err)
	}

	p.filterUserData = true

	return nil
}

// buildFilteredVideoRanges scans the video ES and creates ranges that exclude user_data sections.
// User_data (00 00 01 B2) is used for closed captions etc. and is stripped by MKV tools.
// Optimized to use bytes.IndexByte for fast scanning (uses SIMD on x86).
func (p *MPEGPSParser) buildFilteredVideoRanges() error {
	if len(p.videoRanges) == 0 {
		return nil
	}

	// Process each raw video range individually
	// This avoids complex chunk boundary handling
	// Pre-allocate with similar capacity to reduce reallocation
	filteredRanges := make([]PESPayloadRange, 0, len(p.videoRanges))
	var filteredESOffset int64

	for _, rawRange := range p.videoRanges {
		// Direct slice access - zero copy, no allocation
		endOffset := rawRange.FileOffset + int64(rawRange.Size)
		if endOffset > p.size {
			continue
		}
		data := p.data[rawRange.FileOffset:endOffset]

		// Scan for user_data sections within this PES payload
		// Use bytes.IndexByte to quickly find 0x01 bytes (SIMD optimized)
		i := 2 // Start at position 2 since we need at least 00 00 before 01
		rangeStart := 0
		for i < len(data)-1 {
			// Find next 0x01 byte
			idx := bytes.IndexByte(data[i:], 0x01)
			if idx < 0 {
				break
			}
			pos := i + idx

			// Check if this is a user_data start code (00 00 01 B2)
			if pos >= 2 && pos < len(data)-1 &&
				data[pos-1] == 0x00 && data[pos-2] == 0x00 && data[pos+1] == UserDataStartCode {
				// Found user_data - emit range before it
				startCodePos := pos - 2
				if startCodePos > rangeStart {
					filteredRanges = append(filteredRanges, PESPayloadRange{
						FileOffset: rawRange.FileOffset + int64(rangeStart),
						Size:       startCodePos - rangeStart,
						ESOffset:   filteredESOffset,
					})
					filteredESOffset += int64(startCodePos - rangeStart)
				}

				// Skip user_data section to next start code using fast scan
				i = pos + 2
				for i < len(data)-1 {
					idx := bytes.IndexByte(data[i:], 0x01)
					if idx < 0 {
						i = len(data)
						break
					}
					nextPos := i + idx
					if nextPos >= 2 && data[nextPos-1] == 0x00 && data[nextPos-2] == 0x00 {
						// Found next start code
						i = nextPos - 2
						break
					}
					i = nextPos + 1
				}
				rangeStart = i
			} else {
				i = pos + 1
			}
		}

		// Emit remaining data in this PES payload
		if rangeStart < len(data) {
			filteredRanges = append(filteredRanges, PESPayloadRange{
				FileOffset: rawRange.FileOffset + int64(rangeStart),
				Size:       len(data) - rangeStart,
				ESOffset:   filteredESOffset,
			})
			filteredESOffset += int64(len(data) - rangeStart)
		}
	}

	p.filteredVideoRanges = filteredRanges
	return nil
}

// buildFilteredAudioRanges creates ranges that strip Private Stream 1 headers
// and separates audio by sub-stream ID.
// DVD audio in Private Stream 1 has this structure:
//
//	Byte 0: sub-stream ID (0x80-0x87 = AC3, 0x88-0x8F = DTS, etc.)
//	Byte 1: number of audio frames
//	Bytes 2-3: first access unit pointer (offset to first audio frame)
//	Bytes 4+: audio data (for AC3/DTS)
//
// For LPCM sub-streams (0xA0-0xA7), there are 3 additional header bytes after the
// 4-byte PS header (emphasis/mute/frame_number, quant/samplerate/channels, DRC),
// so we strip 7 bytes total. The LPCM header is parsed once per sub-stream.
//
// We strip headers and keep only the raw audio data.
// Each sub-stream ID gets its own separate filtered ES to avoid interleaving issues.
func (p *MPEGPSParser) buildFilteredAudioRanges() error {
	if len(p.audioRanges) == 0 {
		return nil
	}

	// Map to track ranges per sub-stream
	rangesBySubStream := make(map[byte][]PESPayloadRange)
	esOffsetBySubStream := make(map[byte]int64)
	seenSubStreams := make(map[byte]bool)
	p.lpcmSubStreams = make(map[byte]bool)
	p.lpcmInfo = make(map[byte]LPCMFrameHeader)

	for _, rawRange := range p.audioRanges {
		if rawRange.Size < 4 {
			// Too small to have the header structure
			continue
		}

		// Direct slice access - zero copy
		if rawRange.FileOffset >= p.size {
			continue
		}
		subStreamID := p.data[rawRange.FileOffset]

		// Check if this is AC3, DTS, or LPCM
		isAC3 := subStreamID >= 0x80 && subStreamID <= 0x87
		isDTS := subStreamID >= 0x88 && subStreamID <= 0x8F
		isLPCM := subStreamID >= 0xA0 && subStreamID <= 0xA7

		if isAC3 || isDTS || isLPCM {
			// Track sub-stream order
			if !seenSubStreams[subStreamID] {
				seenSubStreams[subStreamID] = true
				p.audioSubStreams = append(p.audioSubStreams, subStreamID)
			}

			if isLPCM {
				// Strip 7 bytes: 4-byte PS header + 3-byte LPCM frame header
				if rawRange.Size > LPCMTotalHeaderSize {
					// Parse LPCM header on first packet to get bit depth
					if _, ok := p.lpcmInfo[subStreamID]; !ok {
						headerEnd := rawRange.FileOffset + 4 + LPCMHeaderSize
						if headerEnd > p.size {
							continue
						}
						headerData := p.data[rawRange.FileOffset+4 : headerEnd]
						info := ParseLPCMFrameHeader(headerData)
						p.lpcmInfo[subStreamID] = info
						// Only 16-bit LPCM is supported for byte-swap matching.
						// 20/24-bit uses grouped packing that changes data size
						// during transform, so it falls through to delta.
						if IsLPCM16Bit(info.Quantization) {
							p.lpcmSubStreams[subStreamID] = true
						}
					}
					esOffset := esOffsetBySubStream[subStreamID]
					rangesBySubStream[subStreamID] = append(rangesBySubStream[subStreamID], PESPayloadRange{
						FileOffset: rawRange.FileOffset + LPCMTotalHeaderSize,
						Size:       rawRange.Size - LPCMTotalHeaderSize,
						ESOffset:   esOffset,
					})
					esOffsetBySubStream[subStreamID] += int64(rawRange.Size - LPCMTotalHeaderSize)
				}
			} else {
				// Strip the entire 4-byte header, keep only raw audio data
				if rawRange.Size > 4 {
					esOffset := esOffsetBySubStream[subStreamID]
					rangesBySubStream[subStreamID] = append(rangesBySubStream[subStreamID], PESPayloadRange{
						FileOffset: rawRange.FileOffset + 4, // Skip header (1 + 1 + 2)
						Size:       rawRange.Size - 4,       // Rest is audio data
						ESOffset:   esOffset,
					})
					esOffsetBySubStream[subStreamID] += int64(rawRange.Size - 4)
				}
			}
		}
		// Skip unknown sub-stream types (like subtitles 0x20-0x3F)
	}

	p.filteredAudioBySubStream = rangesBySubStream
	return nil
}

// parsePackHeader parses an MPEG-2 pack header and returns its size.
func (p *MPEGPSParser) parsePackHeader(pos int64) (int, error) {
	// MPEG-2 pack header is 14 bytes minimum
	// Format: 00 00 01 BA + SCR (6 bytes) + mux_rate (3 bytes) + stuffing
	if pos+14 > p.size {
		return 0, fmt.Errorf("failed to read pack header")
	}
	buf := p.data[pos : pos+14]

	// Check if this is MPEG-2 (starts with 01) or MPEG-1 (starts with 0010)
	if buf[4]&0xC0 == 0x40 {
		// MPEG-2 pack header
		stuffingLen := int(buf[13] & 0x07)
		return 14 + stuffingLen, nil
	}

	// MPEG-1 pack header is 12 bytes
	return 12, nil
}

// parseSystemHeader parses a system header and returns its total size.
func (p *MPEGPSParser) parseSystemHeader(pos int64) (int, error) {
	length, err := p.readPESLength(pos + 4)
	if err != nil {
		return 0, err
	}
	return 6 + int(length), nil
}

// readPESLength reads the 2-byte PES packet length field.
func (p *MPEGPSParser) readPESLength(pos int64) (uint16, error) {
	if pos+2 > p.size {
		return 0, fmt.Errorf("failed to read PES length")
	}
	return binary.BigEndian.Uint16(p.data[pos : pos+2]), nil
}

// parsePESPacket parses a PES packet header and returns packet info.
func (p *MPEGPSParser) parsePESPacket(pos int64, streamID byte) (PESPacket, error) {
	pkt := PESPacket{
		StreamID: streamID,
		Offset:   pos,
	}

	// Read length field
	length, err := p.readPESLength(pos + 4)
	if err != nil {
		return pkt, err
	}

	// PES packet structure after start code + stream ID + length:
	// - 2 bits: '10'
	// - 2 bits: PES_scrambling_control
	// - 1 bit: PES_priority
	// - 1 bit: data_alignment_indicator
	// - 1 bit: copyright
	// - 1 bit: original_or_copy
	// - 2 bits: PTS_DTS_flags
	// - 1 bit: ESCR_flag
	// - 1 bit: ES_rate_flag
	// - 1 bit: DSM_trick_mode_flag
	// - 1 bit: additional_copy_info_flag
	// - 1 bit: PES_CRC_flag
	// - 1 bit: PES_extension_flag
	// - 8 bits: PES_header_data_length
	// Then optional fields based on flags

	// Direct slice access for PES header fields
	if pos+9 > p.size {
		return pkt, fmt.Errorf("failed to read PES header")
	}
	buf := p.data[pos+6 : pos+9]

	// Check for MPEG-2 PES (starts with 10)
	if buf[0]&0xC0 == 0x80 {
		// MPEG-2 PES header
		headerDataLen := int(buf[2])
		pkt.HeaderSize = 6 + 3 + headerDataLen // start code(4) + length(2) + flags(2) + header_len(1) + header_data
		pkt.PayloadOffset = pos + int64(pkt.HeaderSize)
		pkt.PayloadSize = int(length) - 3 - headerDataLen
	} else {
		// MPEG-1 PES header - simpler structure
		// Skip stuffing bytes (0xFF) and find actual header
		headerLen := 0
		offset := pos + 6
		for {
			if offset+int64(headerLen) >= p.size {
				return pkt, fmt.Errorf("failed to read PES header: offset out of range")
			}
			b := p.data[offset+int64(headerLen)]
			if b == 0xFF {
				headerLen++
				if headerLen > 16 { // Safety limit
					break
				}
				continue
			}
			if b&0xC0 == 0x40 {
				// STD buffer
				headerLen += 2
				continue
			}
			if b&0xF0 == 0x20 {
				// PTS only
				headerLen += 5
			} else if b&0xF0 == 0x30 {
				// PTS + DTS
				headerLen += 10
			} else if b == 0x0F {
				// No timestamps
				headerLen++
			}
			break
		}
		pkt.HeaderSize = 6 + headerLen
		pkt.PayloadOffset = pos + int64(pkt.HeaderSize)
		pkt.PayloadSize = int(length) - headerLen
	}

	if pkt.PayloadSize < 0 {
		pkt.PayloadSize = 0
	}

	return pkt, nil
}

// VideoRanges returns all video payload ranges found in the stream.
func (p *MPEGPSParser) VideoRanges() []PESPayloadRange {
	return p.videoRanges
}

// FilteredVideoRangesCount returns the number of filtered video ranges.
func (p *MPEGPSParser) FilteredVideoRangesCount() int {
	return len(p.filteredVideoRanges)
}

// RawVideoESSize returns the total size of raw (unfiltered) video ES.
func (p *MPEGPSParser) RawVideoESSize() int64 {
	if len(p.videoRanges) == 0 {
		return 0
	}
	last := p.videoRanges[len(p.videoRanges)-1]
	return last.ESOffset + int64(last.Size)
}

// AudioRanges returns all audio payload ranges found in the stream.
func (p *MPEGPSParser) AudioRanges() []PESPayloadRange {
	return p.audioRanges
}

// Packets returns all parsed PES packets.
func (p *MPEGPSParser) Packets() []PESPacket {
	return p.packets
}

// FileOffsetToESOffset converts a file offset within a payload to an ES offset.
// Returns -1 if the offset is not within a known payload range.
func (p *MPEGPSParser) FileOffsetToESOffset(fileOffset int64, isVideo bool) int64 {
	ranges := p.audioRanges
	if isVideo {
		ranges = p.videoRanges
	}

	for _, r := range ranges {
		if fileOffset >= r.FileOffset && fileOffset < r.FileOffset+int64(r.Size) {
			offsetInPayload := fileOffset - r.FileOffset
			return r.ESOffset + offsetInPayload
		}
	}
	return -1
}

// ESOffsetToFileOffset converts an ES offset to a file offset.
// Returns the file offset and payload remaining size, or -1 if not found.
func (p *MPEGPSParser) ESOffsetToFileOffset(esOffset int64, isVideo bool) (fileOffset int64, remaining int) {
	ranges := p.audioRanges
	if isVideo {
		ranges = p.videoRanges
	}

	for _, r := range ranges {
		if esOffset >= r.ESOffset && esOffset < r.ESOffset+int64(r.Size) {
			offsetInPayload := esOffset - r.ESOffset
			return r.FileOffset + offsetInPayload, r.Size - int(offsetInPayload)
		}
	}
	return -1, 0
}

// TotalESSize returns the total size of the elementary stream.
// For video, returns filtered ES size when filtering is enabled.
// For audio, this returns 0 - use AudioSubStreamESSize instead.
func (p *MPEGPSParser) TotalESSize(isVideo bool) int64 {
	if !isVideo {
		return 0
	}
	if p.filterUserData && len(p.filteredVideoRanges) > 0 {
		return totalESSizeFromRanges(p.filteredVideoRanges)
	}
	return totalESSizeFromRanges(p.videoRanges)
}

// AudioSubStreams returns the list of audio sub-stream IDs in order of appearance.
func (p *MPEGPSParser) AudioSubStreams() []byte {
	return p.audioSubStreams
}

// AudioSubStreamCount returns the number of audio sub-streams.
func (p *MPEGPSParser) AudioSubStreamCount() int {
	return len(p.audioSubStreams)
}

// AudioSubStreamESSize returns the total ES size for a specific audio sub-stream.
func (p *MPEGPSParser) AudioSubStreamESSize(subStreamID byte) int64 {
	return totalESSizeFromRanges(p.filteredAudioBySubStream[subStreamID])
}

// FilteredVideoRanges returns the filtered video payload ranges for zero-copy iteration.
// Returns the raw video ranges if filtering is not enabled.
func (p *MPEGPSParser) FilteredVideoRanges() []PESPayloadRange {
	if p.filterUserData && len(p.filteredVideoRanges) > 0 {
		return p.filteredVideoRanges
	}
	return p.videoRanges
}

// FilteredAudioRanges returns the filtered audio payload ranges for a specific sub-stream.
// Returns nil if the sub-stream doesn't exist.
func (p *MPEGPSParser) FilteredAudioRanges(subStreamID byte) []PESPayloadRange {
	return p.filteredAudioBySubStream[subStreamID]
}

// Data returns the raw mmap'd file data for zero-copy access.
func (p *MPEGPSParser) Data() []byte {
	return p.data
}

// DataSlice returns a sub-slice of the backing data at the given offset and size.
func (p *MPEGPSParser) DataSlice(off int64, size int) []byte {
	return p.data[off : off+int64(size)]
}

// DataSize returns the total size of the backing data.
func (p *MPEGPSParser) DataSize() int64 {
	return p.size
}

// ReadESByteWithHint reads a single byte from the ES stream, using a range hint
// to avoid binary search when reading sequentially. Returns the byte, the range
// index where it was found (for use as hint on next call), and success status.
// Pass rangeHint=-1 to force binary search.
func (p *MPEGPSParser) ReadESByteWithHint(esOffset int64, isVideo bool, rangeHint int) (byte, int, bool) {
	if !isVideo {
		// Audio doesn't use this method - it goes through sub-stream reader
		return 0, -1, false
	}
	var ranges []PESPayloadRange
	if p.filterUserData && len(p.filteredVideoRanges) > 0 {
		ranges = p.filteredVideoRanges
	} else {
		ranges = p.videoRanges
	}
	return readByteWithHint(p.data, nil, p.size, ranges, esOffset, rangeHint)
}

// ReadAudioByteWithHint reads a single byte from an audio sub-stream, using a range hint.
// For LPCM sub-streams (16-bit only), swaps even/odd byte positions to convert big-endian to little-endian.
func (p *MPEGPSParser) ReadAudioByteWithHint(subStreamID byte, esOffset int64, rangeHint int) (byte, int, bool) {
	if p.lpcmSubStreams[subStreamID] {
		// Swap even/odd byte position: XOR with 1
		swappedOffset := esOffset ^ 1
		return readByteWithHint(p.data, nil, p.size, p.filteredAudioBySubStream[subStreamID], swappedOffset, rangeHint)
	}
	return readByteWithHint(p.data, nil, p.size, p.filteredAudioBySubStream[subStreamID], esOffset, rangeHint)
}

// Video start codes that should be KEPT (not user_data)
const (
	UserDataStartCode = 0xB2 // This gets stripped by MKV tools
)

// RawRange represents a contiguous chunk of raw file data corresponding to
// part of an ES region. Used for converting ES offsets to raw file offsets.
type RawRange struct {
	FileOffset int64 // Offset in the raw file
	Size       int   // Size of this chunk
}

// RawRangesForESRegion returns the raw file ranges that contain the given ES region.
// For video streams only - audio should use RawRangesForAudioSubStream.
func (p *MPEGPSParser) RawRangesForESRegion(esOffset int64, size int, isVideo bool) ([]RawRange, error) {
	if !isVideo {
		return nil, fmt.Errorf("audio uses per-sub-stream methods, use RawRangesForAudioSubStream")
	}
	var ranges []PESPayloadRange
	if p.filterUserData && len(p.filteredVideoRanges) > 0 {
		ranges = p.filteredVideoRanges
	} else {
		ranges = p.videoRanges
	}
	return rawRangesFromPESRanges(ranges, esOffset, size)
}

// RawRangesForAudioSubStream returns the raw file ranges for audio data from a specific sub-stream.
func (p *MPEGPSParser) RawRangesForAudioSubStream(subStreamID byte, esOffset int64, size int) ([]RawRange, error) {
	ranges, ok := p.filteredAudioBySubStream[subStreamID]
	if !ok {
		return nil, fmt.Errorf("audio sub-stream 0x%02X not found", subStreamID)
	}
	return rawRangesFromPESRanges(ranges, esOffset, size)
}

// ReadESData reads elementary stream data at the given ES offset.
// For video, returns FILTERED ES data (excludes user_data sections).
// For audio, returns error - use ReadAudioSubStreamData instead.
func (p *MPEGPSParser) ReadESData(esOffset int64, size int, isVideo bool) ([]byte, error) {
	if !isVideo {
		return nil, fmt.Errorf("audio uses per-sub-stream methods, use ReadAudioSubStreamData")
	}
	var ranges []PESPayloadRange
	if p.filterUserData && len(p.filteredVideoRanges) > 0 {
		ranges = p.filteredVideoRanges
	} else {
		ranges = p.videoRanges
	}
	return readFromRanges(p.data, nil, p.size, ranges, esOffset, size)
}

// ReadAudioSubStreamData reads audio data from a specific sub-stream.
// For LPCM sub-streams, the data is byte-swapped to match MKV little-endian format.
// Handles alignment: if esOffset is odd, reads from the pair-aligned offset,
// swaps, and returns only the requested portion.
func (p *MPEGPSParser) ReadAudioSubStreamData(subStreamID byte, esOffset int64, size int) ([]byte, error) {
	ranges, ok := p.filteredAudioBySubStream[subStreamID]
	if !ok {
		return nil, fmt.Errorf("audio sub-stream 0x%02X not found", subStreamID)
	}

	if !p.lpcmSubStreams[subStreamID] {
		return readFromRanges(p.data, nil, p.size, ranges, esOffset, size)
	}

	// LPCM 16-bit forward transform (DVD big-endian → MKV little-endian).
	// Byte-swap pairs are aligned to the ES start (pairs at offsets 0-1, 2-3, ...).
	// If esOffset is odd, we must read one extra byte before to complete the pair.
	alignedOffset := esOffset
	trimFront := 0
	if esOffset%2 == 1 {
		alignedOffset = esOffset - 1
		trimFront = 1
	}
	alignedSize := size + trimFront
	// If alignedSize is odd, extend by 1 to complete the trailing pair
	// (if data is available).
	trimBack := 0
	if alignedSize%2 == 1 {
		alignedSize++
		trimBack = 1
	}

	data, err := readFromRanges(p.data, nil, p.size, ranges, alignedOffset, alignedSize)
	if err != nil {
		// If extending caused an out-of-range error, retry without the trailing extension
		if trimBack > 0 {
			alignedSize--
			trimBack = 0
			data, err = readFromRanges(p.data, nil, p.size, ranges, alignedOffset, alignedSize)
		}
		if err != nil {
			return nil, err
		}
	}

	// readFromRanges may return a zero-copy mmap slice, so clone first
	result := make([]byte, len(data))
	copy(result, data)
	TransformLPCM16BE(result)

	// Trim to the originally requested range
	start := trimFront
	end := start + size
	if end > len(result) {
		end = len(result)
	}
	return result[start:end], nil
}

// IsLPCMSubStream returns true if the given sub-stream ID is an LPCM sub-stream.
func (p *MPEGPSParser) IsLPCMSubStream(subStreamID byte) bool {
	return p.lpcmSubStreams[subStreamID]
}
