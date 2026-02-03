package source

import (
	"bytes"
	"fmt"
)

// MPEGTSParser parses MPEG Transport Stream (M2TS) files to extract elementary
// stream data. This is the Blu-ray equivalent of MPEGPSParser for DVDs.
//
// M2TS files use 192-byte packets: 4-byte timestamp + 188-byte TS packet.
// Each TS packet carries a fragment of a PES packet, identified by PID.
// PES packets span multiple TS packets and contain the actual codec data.
//
// The parser builds PES payload range tables that map ES offsets to raw file
// offsets, enabling the matcher to work with continuous ES data while the
// underlying file has TS headers interleaved.
type MPEGTSParser struct {
	data       []byte // mmap'd file data (zero-copy)
	size       int64
	packetSize int // 192 (M2TS) or 188 (standard TS)
	tsOffset   int // offset from packet start to TS sync byte (4 for M2TS, 0 for TS)

	// Stream PIDs from PMT
	videoPID   uint16
	audioPIDs  []uint16  // ordered by PMT appearance
	videoCodec CodecType // for user_data filtering decision

	// PES payload ranges (one entry per TS payload chunk for tracked PIDs)
	videoRanges         []PESPayloadRange
	filteredVideoRanges []PESPayloadRange // excludes user_data for MPEG-2 only
	audioBySubStream    map[byte][]PESPayloadRange

	// Audio PID → sub-stream ID mapping
	audioSubStreams []byte            // sequential IDs: 0, 1, 2, ...
	pidToSubStream  map[uint16]byte   // PID → sub-stream ID
	subStreamToPID  map[byte]uint16   // sub-stream ID → PID

	filterUserData bool
}

// NewMPEGTSParser creates a parser for the given memory-mapped M2TS data.
func NewMPEGTSParser(data []byte) *MPEGTSParser {
	return &MPEGTSParser{
		data:             data,
		size:             int64(len(data)),
		audioBySubStream: make(map[byte][]PESPayloadRange),
		pidToSubStream:   make(map[uint16]byte),
		subStreamToPID:   make(map[byte]uint16),
	}
}

// MPEGTSProgressFunc is called to report MPEG-TS parsing progress.
type MPEGTSProgressFunc func(processed, total int64)

// Parse scans the file and extracts all PES payload ranges.
func (p *MPEGTSParser) Parse() error {
	return p.ParseWithProgress(nil)
}

// ParseWithProgress scans the M2TS file with progress reporting.
func (p *MPEGTSParser) ParseWithProgress(progress MPEGTSProgressFunc) error {
	// Step 1: Detect TS packet size
	detectLen := 192 * 16
	if detectLen > len(p.data) {
		detectLen = len(p.data)
	}
	packetSize, startOffset := detectTSPacketSize(p.data[:detectLen])
	if packetSize == 0 {
		return fmt.Errorf("cannot detect TS packet size")
	}
	p.packetSize = packetSize
	if packetSize == 192 {
		p.tsOffset = 4
	}

	// Step 2: Parse PAT/PMT to find stream PIDs
	scanLen := 2 * 1024 * 1024
	if scanLen > len(p.data) {
		scanLen = len(p.data)
	}
	if err := p.parsePATandPMT(p.data[:scanLen], startOffset); err != nil {
		return fmt.Errorf("parse PAT/PMT: %w", err)
	}

	if p.videoPID == 0 && len(p.audioPIDs) == 0 {
		return fmt.Errorf("no video or audio PIDs found in PMT")
	}

	// Build PID lookup set for fast checking
	trackedPIDs := make(map[uint16]bool)
	if p.videoPID != 0 {
		trackedPIDs[p.videoPID] = true
	}
	for _, pid := range p.audioPIDs {
		trackedPIDs[pid] = true
	}

	// Pre-allocate range slices
	estimatedPackets := int(p.size) / p.packetSize
	if p.videoPID != 0 {
		p.videoRanges = make([]PESPayloadRange, 0, estimatedPackets*7/10)
	}
	for _, pid := range p.audioPIDs {
		subID := p.pidToSubStream[pid]
		p.audioBySubStream[subID] = make([]PESPayloadRange, 0, estimatedPackets/10/len(p.audioPIDs))
	}

	// Step 3: Single-pass packet iteration
	var videoESOffset int64
	audioESOffsets := make(map[byte]int64) // per sub-stream ID

	// Track whether we're inside a PES header (PES headers can span the
	// remainder of the PUSI packet's payload)
	type pesState struct {
		headerBytesRemaining int // PES header bytes still to skip in next continuation packet
	}
	pesStates := make(map[uint16]*pesState)
	for pid := range trackedPIDs {
		pesStates[pid] = &pesState{}
	}

	lastProgress := int64(0)

	for pos := startOffset; pos+p.packetSize <= len(p.data); pos += p.packetSize {
		tsStart := pos + p.tsOffset
		if tsStart >= len(p.data) || p.data[tsStart] != 0x47 {
			continue
		}

		pid := uint16(p.data[tsStart+1]&0x1F)<<8 | uint16(p.data[tsStart+2])
		if !trackedPIDs[pid] {
			continue
		}

		pusi := p.data[tsStart+1]&0x40 != 0
		adaptFieldCtrl := (p.data[tsStart+3] >> 4) & 0x03

		// Find payload start
		payloadOff := tsStart + 4
		switch adaptFieldCtrl {
		case 0x01: // payload only
		case 0x03: // adaptation field + payload
			if payloadOff < pos+p.packetSize {
				adaptLen := int(p.data[payloadOff])
				payloadOff += 1 + adaptLen
			}
		default: // 0x02 = adaptation only, 0x00 = reserved
			continue
		}

		payloadEnd := pos + p.packetSize
		if payloadEnd > len(p.data) {
			payloadEnd = len(p.data)
		}
		if payloadOff >= payloadEnd {
			continue
		}

		payload := p.data[payloadOff:payloadEnd]
		state := pesStates[pid]

		if pusi {
			// New PES packet starts here
			// Parse PES header: 00 00 01 XX LL LL [flags...] HD [header_data]
			if len(payload) < 9 || payload[0] != 0 || payload[1] != 0 || payload[2] != 1 {
				// Not a valid PES start — skip
				continue
			}
			// PES header data length at byte 8
			pesHeaderDataLen := int(payload[8])
			pesHeaderSize := 9 + pesHeaderDataLen // 3 (start code) + 1 (stream_id) + 2 (length) + 2 (flags) + 1 (hdr_data_len) + hdr_data

			if pesHeaderSize >= len(payload) {
				// PES header spans beyond this packet — skip all payload,
				// record remaining header bytes for next packet
				state.headerBytesRemaining = pesHeaderSize - len(payload)
				continue
			}

			// Payload data starts after PES header
			esPayload := payload[pesHeaderSize:]
			fileOffset := int64(payloadOff) + int64(pesHeaderSize)

			if pid == p.videoPID {
				p.videoRanges = append(p.videoRanges, PESPayloadRange{
					FileOffset: fileOffset,
					Size:       len(esPayload),
					ESOffset:   videoESOffset,
				})
				videoESOffset += int64(len(esPayload))
			} else {
				subID := p.pidToSubStream[pid]
				p.audioBySubStream[subID] = append(p.audioBySubStream[subID], PESPayloadRange{
					FileOffset: fileOffset,
					Size:       len(esPayload),
					ESOffset:   audioESOffsets[subID],
				})
				audioESOffsets[subID] += int64(len(esPayload))
			}
			state.headerBytesRemaining = 0
		} else {
			// Continuation packet
			esPayload := payload
			fileOffset := int64(payloadOff)

			// Skip remaining PES header bytes if we're still in the header
			if state.headerBytesRemaining > 0 {
				if state.headerBytesRemaining >= len(esPayload) {
					state.headerBytesRemaining -= len(esPayload)
					continue
				}
				esPayload = esPayload[state.headerBytesRemaining:]
				fileOffset += int64(state.headerBytesRemaining)
				state.headerBytesRemaining = 0
			}

			if len(esPayload) == 0 {
				continue
			}

			if pid == p.videoPID {
				p.videoRanges = append(p.videoRanges, PESPayloadRange{
					FileOffset: fileOffset,
					Size:       len(esPayload),
					ESOffset:   videoESOffset,
				})
				videoESOffset += int64(len(esPayload))
			} else {
				subID := p.pidToSubStream[pid]
				p.audioBySubStream[subID] = append(p.audioBySubStream[subID], PESPayloadRange{
					FileOffset: fileOffset,
					Size:       len(esPayload),
					ESOffset:   audioESOffsets[subID],
				})
				audioESOffsets[subID] += int64(len(esPayload))
			}
		}

		// Report progress
		if progress != nil && int64(pos)-lastProgress > 100*1024*1024 {
			progress(int64(pos), p.size)
			lastProgress = int64(pos)
		}
	}

	if progress != nil {
		progress(p.size, p.size)
	}

	// Step 4: Build filtered video ranges
	if err := p.buildFilteredVideoRanges(); err != nil {
		return fmt.Errorf("build filtered video ranges: %w", err)
	}

	p.filterUserData = true

	return nil
}

// parsePATandPMT finds the PAT and PMT in the first portion of the file
// and extracts video/audio PIDs and stream types.
func (p *MPEGTSParser) parsePATandPMT(data []byte, startOffset int) error {
	// Find PAT (PID 0) to get PMT PID
	pmtPID := uint16(0)
	for i := startOffset; i+p.packetSize <= len(data); i += p.packetSize {
		tsStart := i + p.tsOffset
		if tsStart+188 > len(data) || data[tsStart] != 0x47 {
			continue
		}

		pid := uint16(data[tsStart+1]&0x1F)<<8 | uint16(data[tsStart+2])
		if pid != 0 {
			continue
		}

		pusi := data[tsStart+1]&0x40 != 0
		if !pusi {
			continue
		}

		// Parse payload
		adaptFieldCtrl := (data[tsStart+3] >> 4) & 0x03
		hdrLen := 4
		switch adaptFieldCtrl {
		case 0x02, 0x03:
			if tsStart+4 >= len(data) {
				continue
			}
			adaptLen := int(data[tsStart+4])
			hdrLen = 5 + adaptLen
		case 0x01:
		default:
			continue
		}
		if tsStart+hdrLen >= tsStart+188 {
			continue
		}

		// Skip pointer field
		pointerField := int(data[tsStart+hdrLen])
		hdrLen += 1 + pointerField
		if tsStart+hdrLen+8 > tsStart+188 {
			continue
		}

		payload := data[tsStart+hdrLen : tsStart+188]
		if len(payload) < 12 || payload[0] != 0x00 {
			continue
		}

		sectionLen := int(payload[1]&0x0F)<<8 | int(payload[2])
		if sectionLen < 9 {
			continue
		}

		progsEnd := 8 + sectionLen - 4
		if progsEnd > len(payload) {
			progsEnd = len(payload) - 4
		}

		for j := 8; j+4 <= progsEnd; j += 4 {
			progNum := uint16(payload[j])<<8 | uint16(payload[j+1])
			if progNum == 0 {
				continue
			}
			pmtPID = uint16(payload[j+2]&0x1F)<<8 | uint16(payload[j+3])
			break
		}
		break
	}

	if pmtPID == 0 {
		return fmt.Errorf("PMT PID not found in PAT")
	}

	// Find PMT and extract stream types
	for i := startOffset; i+p.packetSize <= len(data); i += p.packetSize {
		tsStart := i + p.tsOffset
		if tsStart+188 > len(data) || data[tsStart] != 0x47 {
			continue
		}

		pid := uint16(data[tsStart+1]&0x1F)<<8 | uint16(data[tsStart+2])
		if pid != pmtPID {
			continue
		}

		pusi := data[tsStart+1]&0x40 != 0
		if !pusi {
			continue
		}

		adaptFieldCtrl := (data[tsStart+3] >> 4) & 0x03
		hdrLen := 4
		switch adaptFieldCtrl {
		case 0x02, 0x03:
			if tsStart+4 >= len(data) {
				continue
			}
			adaptLen := int(data[tsStart+4])
			hdrLen = 5 + adaptLen
		case 0x01:
		default:
			continue
		}
		if tsStart+hdrLen >= tsStart+188 {
			continue
		}

		pointerField := int(data[tsStart+hdrLen])
		hdrLen += 1 + pointerField
		if tsStart+hdrLen+12 > tsStart+188 {
			continue
		}

		payload := data[tsStart+hdrLen : tsStart+188]
		if len(payload) < 12 || payload[0] != 0x02 {
			continue
		}

		sectionLen := int(payload[1]&0x0F)<<8 | int(payload[2])
		if sectionLen < 13 {
			continue
		}

		progInfoLen := int(payload[10]&0x0F)<<8 | int(payload[11])
		streamsStart := 12 + progInfoLen
		streamsEnd := 3 + sectionLen - 4
		if streamsEnd > len(payload) {
			streamsEnd = len(payload) - 4
		}
		if streamsStart > streamsEnd {
			continue
		}

		var subStreamSeq byte
		for j := streamsStart; j+5 <= streamsEnd; {
			streamType := payload[j]
			esPID := uint16(payload[j+1]&0x1F)<<8 | uint16(payload[j+2])
			esInfoLen := int(payload[j+3]&0x0F)<<8 | int(payload[j+4])

			ct := tsStreamTypeToCodecType(streamType)
			if ct != CodecUnknown {
				if IsVideoCodec(ct) && p.videoPID == 0 {
					p.videoPID = esPID
					p.videoCodec = ct
				} else if IsAudioCodec(ct) {
					p.audioPIDs = append(p.audioPIDs, esPID)
					p.pidToSubStream[esPID] = subStreamSeq
					p.subStreamToPID[subStreamSeq] = esPID
					p.audioSubStreams = append(p.audioSubStreams, subStreamSeq)
					subStreamSeq++
				}
			}

			next := j + 5 + esInfoLen
			if next < j || next > streamsEnd {
				break
			}
			j = next
		}
		break
	}

	return nil
}

// buildFilteredVideoRanges creates filtered video ranges.
// For MPEG-2 video, this excludes user_data (00 00 01 B2) sections.
// For H.264/H.265, filtered ranges are the same as raw ranges (no filtering needed).
func (p *MPEGTSParser) buildFilteredVideoRanges() error {
	if len(p.videoRanges) == 0 {
		return nil
	}

	// Only MPEG-2 needs user_data filtering
	if p.videoCodec != CodecMPEG2Video {
		// For H.264/H.265/etc, no filtering needed — use raw ranges directly
		p.filteredVideoRanges = p.videoRanges
		return nil
	}

	// MPEG-2: scan for user_data sections and exclude them
	// Same algorithm as MPEGPSParser.buildFilteredVideoRanges
	filteredRanges := make([]PESPayloadRange, 0, len(p.videoRanges))
	var filteredESOffset int64

	for _, rawRange := range p.videoRanges {
		endOffset := rawRange.FileOffset + int64(rawRange.Size)
		if endOffset > p.size {
			continue
		}
		data := p.data[rawRange.FileOffset:endOffset]

		i := 2
		rangeStart := 0
		for i < len(data)-1 {
			idx := bytes.IndexByte(data[i:], 0x01)
			if idx < 0 {
				break
			}
			pos := i + idx

			if pos >= 2 && pos < len(data)-1 &&
				data[pos-1] == 0x00 && data[pos-2] == 0x00 && data[pos+1] == UserDataStartCode {
				startCodePos := pos - 2
				if startCodePos > rangeStart {
					filteredRanges = append(filteredRanges, PESPayloadRange{
						FileOffset: rawRange.FileOffset + int64(rangeStart),
						Size:       startCodePos - rangeStart,
						ESOffset:   filteredESOffset,
					})
					filteredESOffset += int64(startCodePos - rangeStart)
				}

				i = pos + 2
				for i < len(data)-1 {
					idx := bytes.IndexByte(data[i:], 0x01)
					if idx < 0 {
						i = len(data)
						break
					}
					nextPos := i + idx
					if nextPos >= 2 && data[nextPos-1] == 0x00 && data[nextPos-2] == 0x00 {
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

// --- ESReader interface implementation ---

// ReadESData reads elementary stream data at the given ES offset.
func (p *MPEGTSParser) ReadESData(esOffset int64, size int, isVideo bool) ([]byte, error) {
	if !isVideo {
		return nil, fmt.Errorf("audio uses per-sub-stream methods, use ReadAudioSubStreamData")
	}
	ranges := p.filteredVideoRanges
	if len(ranges) == 0 {
		ranges = p.videoRanges
	}
	return readFromRanges(p.data, p.size, ranges, esOffset, size)
}

// ESOffsetToFileOffset converts an ES offset to a file offset and remaining bytes.
func (p *MPEGTSParser) ESOffsetToFileOffset(esOffset int64, isVideo bool) (fileOffset int64, remaining int) {
	var ranges []PESPayloadRange
	if isVideo {
		ranges = p.filteredVideoRanges
		if len(ranges) == 0 {
			ranges = p.videoRanges
		}
	} else {
		return -1, 0
	}

	idx := binarySearchRanges(ranges, esOffset)
	if idx < 0 {
		return -1, 0
	}
	r := ranges[idx]
	offsetInPayload := esOffset - r.ESOffset
	return r.FileOffset + offsetInPayload, r.Size - int(offsetInPayload)
}

// TotalESSize returns the total size of the elementary stream.
func (p *MPEGTSParser) TotalESSize(isVideo bool) int64 {
	if !isVideo {
		return 0
	}
	if p.filterUserData && len(p.filteredVideoRanges) > 0 {
		return totalESSizeFromRanges(p.filteredVideoRanges)
	}
	return totalESSizeFromRanges(p.videoRanges)
}

// AudioSubStreams returns the list of audio sub-stream IDs.
func (p *MPEGTSParser) AudioSubStreams() []byte {
	return p.audioSubStreams
}

// AudioSubStreamESSize returns the ES size for a specific audio sub-stream.
func (p *MPEGTSParser) AudioSubStreamESSize(subStreamID byte) int64 {
	return totalESSizeFromRanges(p.audioBySubStream[subStreamID])
}

// ReadAudioSubStreamData reads audio data from a specific sub-stream.
func (p *MPEGTSParser) ReadAudioSubStreamData(subStreamID byte, esOffset int64, size int) ([]byte, error) {
	ranges, ok := p.audioBySubStream[subStreamID]
	if !ok {
		return nil, fmt.Errorf("audio sub-stream %d not found", subStreamID)
	}
	return readFromRanges(p.data, p.size, ranges, esOffset, size)
}

// --- ESRangeConverter interface implementation ---

// RawRangesForESRegion returns the raw file ranges for a video ES region.
func (p *MPEGTSParser) RawRangesForESRegion(esOffset int64, size int, isVideo bool) ([]RawRange, error) {
	if !isVideo {
		return nil, fmt.Errorf("audio uses per-sub-stream methods, use RawRangesForAudioSubStream")
	}
	ranges := p.filteredVideoRanges
	if len(ranges) == 0 {
		ranges = p.videoRanges
	}
	return rawRangesFromPESRanges(ranges, esOffset, size)
}

// RawRangesForAudioSubStream returns the raw file ranges for audio data from a specific sub-stream.
func (p *MPEGTSParser) RawRangesForAudioSubStream(subStreamID byte, esOffset int64, size int) ([]RawRange, error) {
	ranges, ok := p.audioBySubStream[subStreamID]
	if !ok {
		return nil, fmt.Errorf("audio sub-stream %d not found", subStreamID)
	}
	return rawRangesFromPESRanges(ranges, esOffset, size)
}

// --- Hint-based reading for matcher hot path ---

// ReadESByteWithHint reads a single byte from the ES stream with a range hint.
func (p *MPEGTSParser) ReadESByteWithHint(esOffset int64, isVideo bool, rangeHint int) (byte, int, bool) {
	if !isVideo {
		return 0, -1, false
	}
	ranges := p.filteredVideoRanges
	if len(ranges) == 0 {
		ranges = p.videoRanges
	}
	return readByteWithHint(p.data, p.size, ranges, esOffset, rangeHint)
}

// ReadAudioByteWithHint reads a single byte from an audio sub-stream with a range hint.
func (p *MPEGTSParser) ReadAudioByteWithHint(subStreamID byte, esOffset int64, rangeHint int) (byte, int, bool) {
	return readByteWithHint(p.data, p.size, p.audioBySubStream[subStreamID], esOffset, rangeHint)
}

// --- Accessors for indexer ---

// Data returns the raw mmap'd file data for zero-copy access.
func (p *MPEGTSParser) Data() []byte {
	return p.data
}

// FilteredVideoRanges returns the filtered video payload ranges.
func (p *MPEGTSParser) FilteredVideoRanges() []PESPayloadRange {
	if p.filterUserData && len(p.filteredVideoRanges) > 0 {
		return p.filteredVideoRanges
	}
	return p.videoRanges
}

// FilteredAudioRanges returns the audio payload ranges for a specific sub-stream.
func (p *MPEGTSParser) FilteredAudioRanges(subStreamID byte) []PESPayloadRange {
	return p.audioBySubStream[subStreamID]
}

// RawVideoESSize returns the total size of raw (unfiltered) video ES.
func (p *MPEGTSParser) RawVideoESSize() int64 {
	return totalESSizeFromRanges(p.videoRanges)
}

// FilteredVideoRangesCount returns the number of filtered video ranges.
func (p *MPEGTSParser) FilteredVideoRangesCount() int {
	return len(p.filteredVideoRanges)
}

// AudioSubStreamCount returns the number of audio sub-streams.
func (p *MPEGTSParser) AudioSubStreamCount() int {
	return len(p.audioSubStreams)
}

// VideoPID returns the video PID detected from the PMT.
func (p *MPEGTSParser) VideoPID() uint16 {
	return p.videoPID
}

// AudioPIDs returns the audio PIDs detected from the PMT.
func (p *MPEGTSParser) AudioPIDs() []uint16 {
	return p.audioPIDs
}

// VideoCodec returns the video codec type detected from the PMT.
func (p *MPEGTSParser) VideoCodec() CodecType {
	return p.videoCodec
}

// Ensure MPEGTSParser implements the required interfaces at compile time.
var (
	_ ESReader         = (*MPEGTSParser)(nil)
	_ ESRangeConverter = (*MPEGTSParser)(nil)
)

