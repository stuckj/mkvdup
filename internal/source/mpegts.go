package source

import (
	"bytes"
	"fmt"
	"log"
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
	data        []byte           // mmap'd file data (zero-copy); nil when using multiRegion
	multiRegion *multiRegionData // non-nil for multi-extent UDF files
	size        int64
	packetSize  int // 192 (M2TS) or 188 (standard TS)
	tsOffset    int // offset from packet start to TS sync byte (4 for M2TS, 0 for TS)

	// Stream PIDs from PMT
	videoPID   uint16
	audioPIDs  []uint16  // ordered by PMT appearance
	videoCodec CodecType // for user_data filtering decision

	// PES payload ranges (one entry per TS payload chunk for tracked PIDs)
	videoRanges         []PESPayloadRange
	filteredVideoRanges []PESPayloadRange // excludes user_data for MPEG-2 only
	audioBySubStream    map[byte][]PESPayloadRange

	// Audio PID → sub-stream ID mapping
	audioSubStreams []byte             // sequential IDs: 0, 1, 2, ...
	pidToSubStream  map[uint16]byte    // PID → sub-stream ID
	subStreamToPID  map[byte]uint16    // sub-stream ID → PID
	subStreamCodec  map[byte]CodecType // codec type per sub-stream

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
		subStreamCodec:   make(map[byte]CodecType),
	}
}

// NewMPEGTSParserMultiRegion creates a parser for non-contiguous M2TS data
// from a multi-extent UDF file. The multiRegionData provides a virtual
// contiguous view over multiple mmap sub-slices.
func NewMPEGTSParserMultiRegion(mr *multiRegionData) *MPEGTSParser {
	return &MPEGTSParser{
		multiRegion:      mr,
		size:             mr.Len(),
		audioBySubStream: make(map[byte][]PESPayloadRange),
		pidToSubStream:   make(map[uint16]byte),
		subStreamToPID:   make(map[byte]uint16),
		subStreamCodec:   make(map[byte]CodecType),
	}
}

// dataSlice returns a sub-slice of the parser's data source.
// Uses multiRegion when available, otherwise direct slice of p.data.
func (p *MPEGTSParser) dataSlice(off, end int64) []byte {
	if p.multiRegion != nil {
		return p.multiRegion.Slice(off, end)
	}
	return p.data[off:end]
}

// MPEGTSProgressFunc is called to report MPEG-TS parsing progress.
type MPEGTSProgressFunc func(processed, total int64)

// Parse scans the file and extracts all PES payload ranges.
func (p *MPEGTSParser) Parse() error {
	return p.ParseWithProgress(nil)
}

// ParseWithProgress scans the M2TS file with progress reporting.
func (p *MPEGTSParser) ParseWithProgress(progress MPEGTSProgressFunc) error {
	if p.multiRegion != nil {
		return p.parseMultiRegion(progress)
	}

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

	// Step 3: Scan packets and build PES ranges
	ss := p.initScanState()
	p.scanPackets(p.data, startOffset, 0, ss, progress)

	if progress != nil {
		progress(p.size, p.size)
	}

	return p.finalizeParse()
}

// parseMultiRegion handles parsing when data comes from multiple non-contiguous
// mmap regions. Processes each region sequentially, handling TS packets that
// straddle region boundaries via a small carryover buffer.
func (p *MPEGTSParser) parseMultiRegion(progress MPEGTSProgressFunc) error {
	mr := p.multiRegion
	if len(mr.regions) == 0 {
		return fmt.Errorf("no regions in multi-region data")
	}

	// Step 1: Detect TS packet size from first region
	firstRegion := mr.regions[0].data
	detectLen := 192 * 16
	if detectLen > len(firstRegion) {
		detectLen = len(firstRegion)
	}
	packetSize, startOffset := detectTSPacketSize(firstRegion[:detectLen])
	if packetSize == 0 {
		return fmt.Errorf("cannot detect TS packet size")
	}
	p.packetSize = packetSize
	if packetSize == 192 {
		p.tsOffset = 4
	}

	// Step 2: Parse PAT/PMT from first region
	scanLen := 2 * 1024 * 1024
	if scanLen > len(firstRegion) {
		scanLen = len(firstRegion)
	}
	if err := p.parsePATandPMT(firstRegion[:scanLen], startOffset); err != nil {
		return fmt.Errorf("parse PAT/PMT: %w", err)
	}

	// Step 3: Scan packets across all regions
	ss := p.initScanState()

	var carryover []byte
	for i, reg := range mr.regions {
		chunk := reg.data
		logicalBase := reg.logicalStart
		chunkStart := 0

		if i == 0 {
			// First region: skip to the initial start offset
			chunkStart = startOffset
		}

		// Handle carryover from previous region boundary
		if len(carryover) > 0 {
			needed := p.packetSize - len(carryover)
			if needed <= len(chunk) {
				// Assemble the straddling packet and process it
				bridgePkt := make([]byte, p.packetSize)
				copy(bridgePkt, carryover)
				copy(bridgePkt[len(carryover):], chunk[:needed])
				bridgeBase := logicalBase - int64(len(carryover))
				p.scanPackets(bridgePkt, 0, bridgeBase, ss, nil)
				chunkStart = needed
				carryover = nil
			} else {
				// Region too small to complete the packet — accumulate and continue
				carryover = append(carryover, chunk...)
				continue
			}
		}

		// Process complete packets in this region
		available := len(chunk) - chunkStart
		nComplete := (available / p.packetSize) * p.packetSize
		if nComplete > 0 {
			p.scanPackets(chunk[chunkStart:chunkStart+nComplete], 0, logicalBase+int64(chunkStart), ss, progress)
		}

		// Save any remainder for the next region
		remainder := available - nComplete
		if remainder > 0 {
			carryover = make([]byte, remainder)
			copy(carryover, chunk[chunkStart+nComplete:])
		}
	}

	if len(carryover) > 0 {
		log.Printf("mpegts: warning: discarding %d carryover bytes at end of multi-region data (incomplete TS packet)", len(carryover))
	}

	if progress != nil {
		progress(p.size, p.size)
	}

	return p.finalizeParse()
}

// pesState tracks PES header parsing state across TS packets.
type pesState struct {
	headerBytesRemaining int
}

// scanState holds mutable state for the packet scanning loop.
type scanState struct {
	trackedPIDs    map[uint16]bool
	pesStates      map[uint16]*pesState
	videoESOffset  int64
	audioESOffsets map[byte]int64
	lastProgress   int64
}

// initScanState sets up PID tracking and PES state for scanning.
func (p *MPEGTSParser) initScanState() *scanState {
	if p.videoPID == 0 && len(p.audioPIDs) == 0 {
		return nil
	}

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

	pesStates := make(map[uint16]*pesState)
	for pid := range trackedPIDs {
		pesStates[pid] = &pesState{}
	}

	return &scanState{
		trackedPIDs:    trackedPIDs,
		pesStates:      pesStates,
		audioESOffsets: make(map[byte]int64),
	}
}

// scanPackets processes TS packets in a data buffer, recording PES payload ranges.
// logicalBase is added to all FileOffset values to produce logical (assembled) offsets.
func (p *MPEGTSParser) scanPackets(data []byte, startPos int, logicalBase int64, ss *scanState, progress MPEGTSProgressFunc) {
	if ss == nil {
		return
	}

	for pos := startPos; pos+p.packetSize <= len(data); pos += p.packetSize {
		tsStart := pos + p.tsOffset
		if tsStart >= len(data) || data[tsStart] != 0x47 {
			continue
		}

		pid := uint16(data[tsStart+1]&0x1F)<<8 | uint16(data[tsStart+2])
		if !ss.trackedPIDs[pid] {
			continue
		}

		pusi := data[tsStart+1]&0x40 != 0
		adaptFieldCtrl := (data[tsStart+3] >> 4) & 0x03

		// Find payload start
		payloadOff := tsStart + 4
		switch adaptFieldCtrl {
		case 0x01: // payload only
		case 0x03: // adaptation field + payload
			if payloadOff < pos+p.packetSize {
				adaptLen := int(data[payloadOff])
				payloadOff += 1 + adaptLen
			}
		default: // 0x02 = adaptation only, 0x00 = reserved
			continue
		}

		payloadEnd := pos + p.packetSize
		if payloadEnd > len(data) {
			payloadEnd = len(data)
		}
		if payloadOff >= payloadEnd {
			continue
		}

		payload := data[payloadOff:payloadEnd]
		state := ss.pesStates[pid]

		// File offset in the logical (assembled) coordinate space
		logPayloadOff := logicalBase + int64(payloadOff)

		if pusi {
			// New PES packet starts here
			if len(payload) < 9 || payload[0] != 0 || payload[1] != 0 || payload[2] != 1 {
				continue
			}
			pesHeaderDataLen := int(payload[8])
			pesHeaderSize := 9 + pesHeaderDataLen

			if pesHeaderSize >= len(payload) {
				state.headerBytesRemaining = pesHeaderSize - len(payload)
				continue
			}

			esPayload := payload[pesHeaderSize:]
			fileOffset := logPayloadOff + int64(pesHeaderSize)

			if pid == p.videoPID {
				p.videoRanges = append(p.videoRanges, PESPayloadRange{
					FileOffset: fileOffset,
					Size:       len(esPayload),
					ESOffset:   ss.videoESOffset,
				})
				ss.videoESOffset += int64(len(esPayload))
			} else {
				subID := p.pidToSubStream[pid]
				p.audioBySubStream[subID] = append(p.audioBySubStream[subID], PESPayloadRange{
					FileOffset: fileOffset,
					Size:       len(esPayload),
					ESOffset:   ss.audioESOffsets[subID],
				})
				ss.audioESOffsets[subID] += int64(len(esPayload))
			}
			state.headerBytesRemaining = 0
		} else {
			// Continuation packet
			esPayload := payload
			fileOffset := logPayloadOff

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
					ESOffset:   ss.videoESOffset,
				})
				ss.videoESOffset += int64(len(esPayload))
			} else {
				subID := p.pidToSubStream[pid]
				p.audioBySubStream[subID] = append(p.audioBySubStream[subID], PESPayloadRange{
					FileOffset: fileOffset,
					Size:       len(esPayload),
					ESOffset:   ss.audioESOffsets[subID],
				})
				ss.audioESOffsets[subID] += int64(len(esPayload))
			}
		}

		// Report progress
		logPos := logicalBase + int64(pos)
		if progress != nil && logPos-ss.lastProgress > 100*1024*1024 {
			progress(logPos, p.size)
			ss.lastProgress = logPos
		}
	}
}

// finalizeParse performs post-scan processing: video range filtering and
// TrueHD+AC3 stream splitting. Shared by contiguous and multi-region paths.
func (p *MPEGTSParser) finalizeParse() error {
	if p.videoPID == 0 && len(p.audioPIDs) == 0 {
		return fmt.Errorf("no video or audio PIDs found in PMT")
	}

	if err := p.buildFilteredVideoRanges(); err != nil {
		return fmt.Errorf("build filtered video ranges: %w", err)
	}

	p.filterUserData = true
	p.splitTrueHDAC3Streams()
	p.splitDTSHDCoreStreams()

	return nil
}

// parsePATandPMT finds the PAT and PMT in the first portion of the file
// and extracts video/audio PIDs and stream types.
func (p *MPEGTSParser) parsePATandPMT(data []byte, startOffset int) error {
	// Find PAT (PID 0) and extract PMT PID
	patSection, err := p.reassemblePSISection(data, startOffset, 0, 0x00)
	if err != nil {
		return fmt.Errorf("reassemble PAT: %w", err)
	}

	pmtPID := uint16(0)
	if len(patSection) >= 8 {
		sectionLen := int(patSection[1]&0x0F)<<8 | int(patSection[2])
		progsEnd := 3 + sectionLen - 4 // section_length counts from byte 3; subtract 4 for CRC
		if progsEnd > len(patSection) {
			progsEnd = len(patSection)
		}
		for j := 8; j+4 <= progsEnd; j += 4 {
			progNum := uint16(patSection[j])<<8 | uint16(patSection[j+1])
			if progNum == 0 {
				continue
			}
			pmtPID = uint16(patSection[j+2]&0x1F)<<8 | uint16(patSection[j+3])
			break
		}
	}

	if pmtPID == 0 {
		return fmt.Errorf("PMT PID not found in PAT")
	}

	// Find PMT and extract stream types.
	// PMT sections can span multiple TS packets, so we must reassemble.
	pmtSection, err := p.reassemblePSISection(data, startOffset, pmtPID, 0x02)
	if err != nil {
		return fmt.Errorf("reassemble PMT: %w", err)
	}

	if len(pmtSection) >= 12 {
		progInfoLen := int(pmtSection[10]&0x0F)<<8 | int(pmtSection[11])
		streamsStart := 12 + progInfoLen
		sectionLen := int(pmtSection[1]&0x0F)<<8 | int(pmtSection[2])
		streamsEnd := 3 + sectionLen - 4 // exclude CRC32

		if streamsEnd > len(pmtSection) {
			streamsEnd = len(pmtSection)
		}

		var subStreamSeq byte
		for j := streamsStart; j+5 <= streamsEnd; {
			streamType := pmtSection[j]
			esPID := uint16(pmtSection[j+1]&0x1F)<<8 | uint16(pmtSection[j+2])
			esInfoLen := int(pmtSection[j+3]&0x0F)<<8 | int(pmtSection[j+4])

			ct := tsStreamTypeToCodecType(streamType)
			if ct != CodecUnknown {
				if IsVideoCodec(ct) && p.videoPID == 0 {
					p.videoPID = esPID
					p.videoCodec = ct
				} else if IsAudioCodec(ct) || IsSubtitleCodec(ct) {
					p.audioPIDs = append(p.audioPIDs, esPID)
					p.pidToSubStream[esPID] = subStreamSeq
					p.subStreamToPID[subStreamSeq] = esPID
					p.subStreamCodec[subStreamSeq] = ct
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
	}

	return nil
}

// buildFilteredVideoRanges creates filtered video ranges.
// For MPEG-2 video, this excludes user_data (00 00 01 B2) sections.
// For H.264/H.265, filtered ranges are the same as raw ranges (no filtering needed).
// reassemblePSISection collects a complete PSI section (PAT, PMT, etc.) from
// one or more TS packets. PSI sections can span multiple TS packets when the
// section is larger than a single TS payload (~170 bytes). This happens on
// Blu-ray discs with many audio and subtitle streams in the PMT.
func (p *MPEGTSParser) reassemblePSISection(data []byte, startOffset int, targetPID uint16, tableID byte) ([]byte, error) {
	var section []byte
	sectionLen := -1
	collecting := false

	for i := startOffset; i+p.packetSize <= len(data); i += p.packetSize {
		tsStart := i + p.tsOffset
		if tsStart+188 > len(data) || data[tsStart] != 0x47 {
			continue
		}

		pid := uint16(data[tsStart+1]&0x1F)<<8 | uint16(data[tsStart+2])
		if pid != targetPID {
			continue
		}

		pusi := data[tsStart+1]&0x40 != 0
		adaptFieldCtrl := (data[tsStart+3] >> 4) & 0x03
		hdrLen := 4
		switch adaptFieldCtrl {
		case 0x02, 0x03:
			if tsStart+4 >= len(data) {
				continue
			}
			hdrLen = 5 + int(data[tsStart+4])
		case 0x01:
		default:
			continue
		}
		if tsStart+hdrLen >= tsStart+188 {
			continue
		}

		payload := data[tsStart+hdrLen : tsStart+188]

		if pusi {
			// PUSI packet: skip pointer field, find section start
			pointerField := int(payload[0])
			sectionStart := 1 + pointerField
			if sectionStart >= len(payload) {
				continue
			}
			payload = payload[sectionStart:]
			if len(payload) < 3 || payload[0] != tableID {
				continue
			}

			sectionLen = 3 + (int(payload[1]&0x0F)<<8 | int(payload[2]))
			section = make([]byte, 0, sectionLen)
			collecting = true

			// Append what we have from this packet
			n := len(payload)
			if n > sectionLen {
				n = sectionLen
			}
			section = append(section, payload[:n]...)
		} else if collecting {
			// Continuation packet
			remaining := sectionLen - len(section)
			n := len(payload)
			if n > remaining {
				n = remaining
			}
			section = append(section, payload[:n]...)
		}

		if collecting && len(section) >= sectionLen {
			return section, nil
		}
	}

	if collecting {
		return nil, fmt.Errorf("truncated PSI section for table ID 0x%02X on PID 0x%04X: got %d of %d bytes", tableID, targetPID, len(section), sectionLen)
	}
	return nil, fmt.Errorf("PSI section with table ID 0x%02X not found on PID 0x%04X", tableID, targetPID)
}

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
		data := p.dataSlice(rawRange.FileOffset, endOffset)

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
	return readFromRanges(p.data, p.multiRegion, p.size, ranges, esOffset, size)
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

// SubtitleSubStreams returns the sub-stream IDs that carry subtitle data (e.g., PGS).
func (p *MPEGTSParser) SubtitleSubStreams() []byte {
	var ids []byte
	for _, id := range p.audioSubStreams {
		if IsSubtitleCodec(p.subStreamCodec[id]) {
			ids = append(ids, id)
		}
	}
	return ids
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
	return readFromRanges(p.data, p.multiRegion, p.size, ranges, esOffset, size)
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
	return readByteWithHint(p.data, p.multiRegion, p.size, ranges, esOffset, rangeHint)
}

// ReadAudioByteWithHint reads a single byte from an audio sub-stream with a range hint.
func (p *MPEGTSParser) ReadAudioByteWithHint(subStreamID byte, esOffset int64, rangeHint int) (byte, int, bool) {
	return readByteWithHint(p.data, p.multiRegion, p.size, p.audioBySubStream[subStreamID], esOffset, rangeHint)
}

// --- Accessors for indexer ---

// Data returns the raw mmap'd file data for zero-copy access.
// Returns nil when using multi-region data; use DataSlice instead.
func (p *MPEGTSParser) Data() []byte {
	return p.data
}

// DataSlice returns a sub-slice of the backing data at the given offset and size.
// Works for both contiguous and multi-region data.
func (p *MPEGTSParser) DataSlice(off int64, size int) []byte {
	if p.multiRegion != nil {
		return p.multiRegion.Slice(off, off+int64(size))
	}
	return p.data[off : off+int64(size)]
}

// DataSize returns the total size of the backing data.
func (p *MPEGTSParser) DataSize() int64 {
	return p.size
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

// splitTrueHDAC3Streams detects combined TrueHD+AC3 audio streams and splits
// them into separate sub-streams. On Blu-ray, TrueHD streams (PMT type 0x83)
// interleave an AC3 compatibility core in the same PID. MakeMKV splits these
// into separate MKV tracks, so we must split them here to match.
func (p *MPEGTSParser) splitTrueHDAC3Streams() {
	for _, subID := range p.audioSubStreams {
		if p.subStreamCodec[subID] != CodecTrueHDAudio {
			continue
		}
		ranges := p.audioBySubStream[subID]
		if len(ranges) == 0 {
			continue
		}

		// Check if this stream actually has interleaved AC3
		if !p.detectCombinedTrueHDAC3(ranges) {
			continue
		}

		// Split the combined ranges
		ac3Ranges, truehdRanges := p.splitCombinedAudioRanges(ranges)
		if len(ac3Ranges) == 0 {
			continue
		}

		// Merge adjacent ranges to reduce count
		ac3Ranges = mergeAdjacentRanges(ac3Ranges)
		truehdRanges = mergeAdjacentRanges(truehdRanges)

		// Replace original sub-stream with TrueHD-only ranges
		p.audioBySubStream[subID] = truehdRanges

		// Add AC3 as a new sub-stream
		newSubID := byte(len(p.audioSubStreams))
		p.audioBySubStream[newSubID] = ac3Ranges
		p.subStreamCodec[newSubID] = CodecAC3Audio
		p.audioSubStreams = append(p.audioSubStreams, newSubID)

	}
}

// detectCombinedTrueHDAC3 checks if a TrueHD audio stream contains interleaved
// AC3 frames by scanning the first few KB of ES data for both sync patterns.
func (p *MPEGTSParser) detectCombinedTrueHDAC3(ranges []PESPayloadRange) bool {
	// Read up to 16KB of ES data to check for both patterns
	hasAC3 := false
	hasTrueHD := false
	bytesChecked := 0
	const maxCheck = 16 * 1024

	for _, r := range ranges {
		if bytesChecked >= maxCheck {
			break
		}
		endOffset := r.FileOffset + int64(r.Size)
		if endOffset > p.size {
			continue
		}
		data := p.dataSlice(r.FileOffset, endOffset)
		// Clamp to remaining check budget
		remaining := maxCheck - bytesChecked
		if remaining < len(data) {
			data = data[:remaining]
		}
		for i := 0; i < len(data)-1; i++ {
			if data[i] == 0x0B && data[i+1] == 0x77 {
				hasAC3 = true
			}
			if i+3 < len(data) &&
				data[i] == 0xF8 && data[i+1] == 0x72 &&
				data[i+2] == 0x6F && data[i+3] == 0xBA {
				hasTrueHD = true
			}
			if hasAC3 && hasTrueHD {
				return true
			}
		}
		bytesChecked += len(data)
	}
	return false
}

// splitCombinedAudioRanges splits PES payload ranges of a combined TrueHD+AC3
// stream into separate AC3 and TrueHD ranges. It walks through the ranges,
// parsing AC3 frame headers to determine frame sizes, and assigns each byte
// to either the AC3 or TrueHD output.
func (p *MPEGTSParser) splitCombinedAudioRanges(ranges []PESPayloadRange) (ac3Ranges, truehdRanges []PESPayloadRange) {
	var ac3ES, truehdES int64 // cumulative ES offsets for output streams
	ac3Remaining := 0         // bytes remaining in current AC3 frame

	// Buffer for AC3 header detection across range boundaries.
	// We need bytes 0-1 (sync word 0B77) and byte 4 (fscod+frmsizecod).
	var headerBuf [5]byte
	headerBufLen := 0
	// Ranges from intermediate short ranges that contributed to headerBuf
	// but haven't been committed to either output yet.
	var headerPendingRanges []PESPayloadRange

	for _, r := range ranges {
		endOffset := r.FileOffset + int64(r.Size)
		if endOffset > p.size {
			continue
		}
		data := p.dataSlice(r.FileOffset, endOffset)
		pos := 0

		// Handle header bytes buffered from previous range.
		// The buffered bytes were trimmed from the previous range's TrueHD output,
		// so we must classify them here (as AC3 or TrueHD).
		if headerBufLen > 0 && ac3Remaining == 0 {
			need := 5 - headerBufLen
			if need > len(data) {
				// Still not enough data to complete header check.
				// Buffer the bytes without committing to either output —
				// we can't classify until we have the full 5-byte header.
				copy(headerBuf[headerBufLen:], data)
				headerBufLen += len(data)
				headerPendingRanges = append(headerPendingRanges, r)
				continue
			}
			copy(headerBuf[headerBufLen:], data[:need])
			if headerBuf[0] == 0x0B && headerBuf[1] == 0x77 {
				fscod := (headerBuf[4] >> 6) & 0x03
				frmsizecod := headerBuf[4] & 0x3F
				frameSize := AC3FrameSize(fscod, frmsizecod)
				if frameSize > 0 {
					// Valid AC3 frame header spanning range boundary.
					// The initial bytes from the first range were already
					// added to AC3 optimistically when buffered.
					// Add any intermediate pending ranges to AC3 too.
					for _, pr := range headerPendingRanges {
						ac3Ranges = append(ac3Ranges, PESPayloadRange{
							FileOffset: pr.FileOffset,
							Size:       pr.Size,
							ESOffset:   ac3ES,
						})
						ac3ES += int64(pr.Size)
					}
					headerPendingRanges = nil
					// Now add the header-completion bytes from this range to AC3.
					ac3Ranges = append(ac3Ranges, PESPayloadRange{
						FileOffset: r.FileOffset,
						Size:       need,
						ESOffset:   ac3ES,
					})
					ac3ES += int64(need)
					ac3Remaining = frameSize - 5 // remaining frame bytes after 5-byte header
					pos = need
					headerBufLen = 0
					// Fall through to normal scan which will consume ac3Remaining
					goto scanLoop
				}
			}
			// Not a valid AC3 header. The buffered bytes from the first range
			// were added to AC3 ranges optimistically; re-attribute them
			// to TrueHD by adjusting ES offsets.
			if len(ac3Ranges) > 0 {
				last := ac3Ranges[len(ac3Ranges)-1]
				ac3Ranges = ac3Ranges[:len(ac3Ranges)-1]
				ac3ES -= int64(last.Size)
				truehdRanges = append(truehdRanges, PESPayloadRange{
					FileOffset: last.FileOffset,
					Size:       last.Size,
					ESOffset:   truehdES,
				})
				truehdES += int64(last.Size)
			}
			// Re-attribute any intermediate pending ranges to TrueHD.
			for _, pr := range headerPendingRanges {
				truehdRanges = append(truehdRanges, PESPayloadRange{
					FileOffset: pr.FileOffset,
					Size:       pr.Size,
					ESOffset:   truehdES,
				})
				truehdES += int64(pr.Size)
			}
			headerPendingRanges = nil
			headerBufLen = 0
			// Fall through to normal processing for the rest of this range
		}

	scanLoop:
		for pos < len(data) {
			if ac3Remaining > 0 {
				// Inside an AC3 frame - consume bytes
				consume := ac3Remaining
				if consume > len(data)-pos {
					consume = len(data) - pos
				}
				ac3Ranges = append(ac3Ranges, PESPayloadRange{
					FileOffset: r.FileOffset + int64(pos),
					Size:       consume,
					ESOffset:   ac3ES,
				})
				ac3ES += int64(consume)
				ac3Remaining -= consume
				pos += consume
				continue
			}

			// Look for AC3 sync word (need 5 bytes: 2-byte sync + byte 4 for frame size)
			if pos+4 < len(data) && data[pos] == 0x0B && data[pos+1] == 0x77 {
				fscod := (data[pos+4] >> 6) & 0x03
				frmsizecod := data[pos+4] & 0x3F
				frameSize := AC3FrameSize(fscod, frmsizecod)
				if frameSize > 0 {
					ac3Remaining = frameSize
					continue // will be consumed in ac3Remaining branch
				}
			}

			// TrueHD data - scan forward to next AC3 sync word or end of range
			start := pos
			pos++
			for pos < len(data) {
				if pos+4 < len(data) && data[pos] == 0x0B && data[pos+1] == 0x77 {
					fscod := (data[pos+4] >> 6) & 0x03
					frmsizecod := data[pos+4] & 0x3F
					if AC3FrameSize(fscod, frmsizecod) > 0 {
						break
					}
				}
				pos++
			}
			if pos > start {
				truehdRanges = append(truehdRanges, PESPayloadRange{
					FileOffset: r.FileOffset + int64(start),
					Size:       pos - start,
					ESOffset:   truehdES,
				})
				truehdES += int64(pos - start)
			}
		}

		// After processing all bytes in this range, check if trailing bytes
		// could be a partial AC3 header for cross-range detection.
		// Only relevant when not inside an AC3 frame.
		if ac3Remaining == 0 && len(truehdRanges) > 0 {
			last := &truehdRanges[len(truehdRanges)-1]
			lastEnd := last.FileOffset + int64(last.Size)
			rangeEnd := r.FileOffset + int64(r.Size)
			if lastEnd == rangeEnd && last.Size > 0 {
				// TrueHD range extends to end of PES range. Check if last
				// 1-4 bytes could start an AC3 header (contain 0x0B).
				checkStart := last.Size - 4
				if checkStart < 0 {
					checkStart = 0
				}
				tailData := p.dataSlice(last.FileOffset, lastEnd)
				bufStart := -1
				for j := len(tailData) - 1; j >= checkStart; j-- {
					if tailData[j] == 0x0B {
						bufStart = j
						break
					}
				}
				if bufStart >= 0 {
					tailLen := len(tailData) - bufStart
					copy(headerBuf[:], tailData[bufStart:])
					headerBufLen = tailLen
					// Trim TrueHD range and add trimmed bytes to AC3 optimistically
					last.Size -= tailLen
					truehdES -= int64(tailLen)
					if last.Size == 0 {
						truehdRanges = truehdRanges[:len(truehdRanges)-1]
					}
					ac3Ranges = append(ac3Ranges, PESPayloadRange{
						FileOffset: rangeEnd - int64(tailLen),
						Size:       tailLen,
						ESOffset:   ac3ES,
					})
					ac3ES += int64(tailLen)
				}
			}
		}
	}

	// If we ended with buffered bytes, they weren't AC3 — re-attribute to TrueHD
	if headerBufLen > 0 {
		if len(ac3Ranges) > 0 {
			last := ac3Ranges[len(ac3Ranges)-1]
			ac3Ranges = ac3Ranges[:len(ac3Ranges)-1]
			ac3ES -= int64(last.Size)
			truehdRanges = append(truehdRanges, PESPayloadRange{
				FileOffset: last.FileOffset,
				Size:       last.Size,
				ESOffset:   truehdES,
			})
			truehdES += int64(last.Size)
		}
		for _, pr := range headerPendingRanges {
			truehdRanges = append(truehdRanges, PESPayloadRange{
				FileOffset: pr.FileOffset,
				Size:       pr.Size,
				ESOffset:   truehdES,
			})
			truehdES += int64(pr.Size)
		}
	}

	return ac3Ranges, truehdRanges
}

// mergeAdjacentRanges merges consecutive PESPayloadRange entries that are
// contiguous in both file offset and ES offset.
func mergeAdjacentRanges(ranges []PESPayloadRange) []PESPayloadRange {
	if len(ranges) <= 1 {
		return ranges
	}
	merged := make([]PESPayloadRange, 0, len(ranges)/2)
	merged = append(merged, ranges[0])
	for i := 1; i < len(ranges); i++ {
		last := &merged[len(merged)-1]
		r := ranges[i]
		if r.FileOffset == last.FileOffset+int64(last.Size) &&
			r.ESOffset == last.ESOffset+int64(last.Size) {
			last.Size += r.Size
		} else {
			merged = append(merged, r)
		}
	}
	return merged
}

// splitDTSHDCoreStreams detects DTS-HD audio streams that contain an embedded
// DTS core and extracts the core into a separate sub-stream. On Blu-ray,
// DTS-HD streams (PMT types 0x85/0x86) embed DTS core frames followed by
// extension data (ExSS: XBR, XLL, XXCh) in the same PID. MakeMKV may extract
// either the full DTS-HD stream (A_DTS/LOSSLESS) or just the DTS core (A_DTS).
//
// Unlike TrueHD+AC3 where the original is replaced, here we keep the original
// combined sub-stream (for A_DTS/LOSSLESS matching) and add a new core-only
// sub-stream (for A_DTS matching).
func (p *MPEGTSParser) splitDTSHDCoreStreams() {
	for _, subID := range p.audioSubStreams {
		if p.subStreamCodec[subID] != CodecDTSHDAudio {
			continue
		}
		ranges := p.audioBySubStream[subID]
		if len(ranges) == 0 {
			continue
		}

		// Check if this stream actually has both DTS core and DTS-HD extension
		if !p.detectCombinedDTSHDCore(ranges) {
			continue
		}

		// Split out the DTS core ranges
		coreRanges := p.splitDTSHDCoreRanges(ranges)
		if len(coreRanges) == 0 {
			continue
		}

		coreRanges = mergeAdjacentRanges(coreRanges)

		// Keep original combined sub-stream for A_DTS/LOSSLESS matching.
		// Add DTS core as a new sub-stream for A_DTS matching.
		newSubID := byte(len(p.audioSubStreams))
		p.audioBySubStream[newSubID] = coreRanges
		p.subStreamCodec[newSubID] = CodecDTSAudio
		p.audioSubStreams = append(p.audioSubStreams, newSubID)
	}
}

// detectCombinedDTSHDCore checks if a DTS-HD audio stream contains both
// DTS core frames and DTS-HD extension (ExSS) frames by scanning the first
// few KB of ES data for both sync patterns.
func (p *MPEGTSParser) detectCombinedDTSHDCore(ranges []PESPayloadRange) bool {
	hasDTSCore := false
	hasDTSHDExSS := false
	bytesChecked := 0
	const maxCheck = 16 * 1024

	for _, r := range ranges {
		if bytesChecked >= maxCheck {
			break
		}
		endOffset := r.FileOffset + int64(r.Size)
		if endOffset > p.size {
			continue
		}
		data := p.dataSlice(r.FileOffset, endOffset)
		remaining := maxCheck - bytesChecked
		if remaining < len(data) {
			data = data[:remaining]
		}
		for i := 0; i < len(data)-3; i++ {
			// DTS core sync: 7F FE 80 01
			if data[i] == 0x7F && data[i+1] == 0xFE &&
				data[i+2] == 0x80 && data[i+3] == 0x01 {
				hasDTSCore = true
			}
			// DTS-HD ExSS sync: 64 58 20 25
			if data[i] == 0x64 && data[i+1] == 0x58 &&
				data[i+2] == 0x20 && data[i+3] == 0x25 {
				hasDTSHDExSS = true
			}
			if hasDTSCore && hasDTSHDExSS {
				return true
			}
		}
		bytesChecked += len(data)
	}
	return false
}

// splitDTSHDCoreRanges extracts DTS core frame ranges from a combined DTS-HD
// stream. It walks through PES payload ranges, parsing DTS core frame headers
// to determine frame sizes, and collects only the DTS core bytes.
func (p *MPEGTSParser) splitDTSHDCoreRanges(ranges []PESPayloadRange) []PESPayloadRange {
	var coreRanges []PESPayloadRange
	var coreES int64   // cumulative ES offset for core output
	coreRemaining := 0 // bytes remaining in current DTS core frame

	// Buffer for DTS core header detection across range boundaries.
	// We need bytes 0-6: 4-byte sync word + 3 bytes for frame size field.
	var headerBuf [7]byte
	headerBufLen := 0
	var headerPendingRanges []PESPayloadRange

	for _, r := range ranges {
		endOffset := r.FileOffset + int64(r.Size)
		if endOffset > p.size {
			continue
		}
		data := p.dataSlice(r.FileOffset, endOffset)
		pos := 0

		// Handle header bytes buffered from previous range
		if headerBufLen > 0 && coreRemaining == 0 {
			need := 7 - headerBufLen
			if need > len(data) {
				copy(headerBuf[headerBufLen:], data)
				headerBufLen += len(data)
				headerPendingRanges = append(headerPendingRanges, r)
				continue
			}
			copy(headerBuf[headerBufLen:], data[:need])
			frameSize := DTSCoreFrameSize(headerBuf[:7])
			if frameSize > 0 {
				// Valid DTS core frame spanning range boundary.
				// Add any intermediate pending ranges to core.
				for _, pr := range headerPendingRanges {
					coreRanges = append(coreRanges, PESPayloadRange{
						FileOffset: pr.FileOffset,
						Size:       pr.Size,
						ESOffset:   coreES,
					})
					coreES += int64(pr.Size)
				}
				headerPendingRanges = nil
				coreRanges = append(coreRanges, PESPayloadRange{
					FileOffset: r.FileOffset,
					Size:       need,
					ESOffset:   coreES,
				})
				coreES += int64(need)
				coreRemaining = frameSize - 7
				pos = need
				headerBufLen = 0
				goto scanLoop
			}
			// Not a valid DTS core header — discard buffered bytes (they're extension data).
			// Re-attribute the optimistic core range back (remove it).
			if len(coreRanges) > 0 {
				last := coreRanges[len(coreRanges)-1]
				coreRanges = coreRanges[:len(coreRanges)-1]
				coreES -= int64(last.Size)
			}
			headerPendingRanges = nil
			headerBufLen = 0
		}

	scanLoop:
		for pos < len(data) {
			if coreRemaining > 0 {
				// Inside a DTS core frame — consume bytes
				consume := coreRemaining
				if consume > len(data)-pos {
					consume = len(data) - pos
				}
				coreRanges = append(coreRanges, PESPayloadRange{
					FileOffset: r.FileOffset + int64(pos),
					Size:       consume,
					ESOffset:   coreES,
				})
				coreES += int64(consume)
				coreRemaining -= consume
				pos += consume
				continue
			}

			// Look for DTS core sync word (need 7 bytes: 4-byte sync + 3 for frame size)
			if pos+6 < len(data) &&
				data[pos] == 0x7F && data[pos+1] == 0xFE &&
				data[pos+2] == 0x80 && data[pos+3] == 0x01 {
				frameSize := DTSCoreFrameSize(data[pos : pos+7])
				if frameSize > 0 {
					coreRemaining = frameSize
					continue // will be consumed in coreRemaining branch
				}
			}

			// Not DTS core data (extension or other) — skip forward to next
			// potential DTS core sync word or end of range
			pos++
			for pos < len(data) {
				if pos+6 < len(data) &&
					data[pos] == 0x7F && data[pos+1] == 0xFE &&
					data[pos+2] == 0x80 && data[pos+3] == 0x01 {
					if DTSCoreFrameSize(data[pos:pos+7]) > 0 {
						break
					}
				}
				pos++
			}
		}

		// After processing, check if trailing bytes could be a partial DTS core header
		if coreRemaining == 0 && len(data) > 0 {
			// Look for 0x7F (start of DTS sync word) near end of range
			checkStart := len(data) - 4
			if checkStart < 0 {
				checkStart = 0
			}
			bufStart := -1
			for j := len(data) - 1; j >= checkStart; j-- {
				if data[j] == 0x7F {
					bufStart = j
					break
				}
			}
			if bufStart >= 0 {
				tailLen := len(data) - bufStart
				copy(headerBuf[:], data[bufStart:])
				headerBufLen = tailLen
				// Add trimmed bytes to core optimistically
				coreRanges = append(coreRanges, PESPayloadRange{
					FileOffset: r.FileOffset + int64(bufStart),
					Size:       tailLen,
					ESOffset:   coreES,
				})
				coreES += int64(tailLen)
			}
		}
	}

	// If we ended with buffered bytes, they weren't a valid DTS core header — remove
	if headerBufLen > 0 {
		if len(coreRanges) > 0 {
			last := coreRanges[len(coreRanges)-1]
			coreRanges = coreRanges[:len(coreRanges)-1]
			coreES -= int64(last.Size)
			_ = last // suppress unused warning
		}
	}

	return coreRanges
}

// Ensure MPEGTSParser implements the required interfaces at compile time.
var (
	_ ESReader         = (*MPEGTSParser)(nil)
	_ ESRangeConverter = (*MPEGTSParser)(nil)
)
