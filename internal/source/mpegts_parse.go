package source

import (
	"bytes"
	"fmt"
	"log"
)

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
