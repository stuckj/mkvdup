package source

import "log"

// splitDTSHDCoreStreams detects DTS-HD audio streams that contain an embedded
// DTS core and extracts the core into a separate sub-stream. On Blu-ray,
// DTS-HD streams (PMT types 0x85/0x86) embed DTS core frames followed by
// extension data (ExSS: XBR, XLL, XXCh) in the same PID. Video extraction tools may
// extract either the full DTS-HD stream (A_DTS/LOSSLESS) or just the DTS core
// (A_DTS).
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

// detectActualDTSCoreSize reads the beginning of a DTS-HD stream's ES data
// to determine the actual core frame size. In DTS-HD MA/HRA streams, the FSIZE
// field in the DTS core header reports the full access unit size (core + extension),
// not just the core portion. This function finds the real core boundary by
// scanning for the ExSS sync word (64 58 20 25) or the next DTS core sync word.
//
// Returns the actual core frame size in bytes, or 0 if it cannot be determined.
func (p *MPEGTSParser) detectActualDTSCoreSize(ranges []PESPayloadRange) int {
	// Read up to 32KB of ES data — enough for several frames at any bitrate.
	const maxRead = 32 * 1024
	buf := make([]byte, 0, maxRead)
	for _, r := range ranges {
		if len(buf) >= maxRead {
			break
		}
		endOffset := r.FileOffset + int64(r.Size)
		if endOffset > p.size {
			continue
		}
		data := p.dataSlice(r.FileOffset, endOffset)
		remaining := maxRead - len(buf)
		if len(data) > remaining {
			data = data[:remaining]
		}
		buf = append(buf, data...)
	}

	// Find all DTS core sync positions to measure frame boundaries.
	var syncPositions []int
	for i := 0; i+6 < len(buf); i++ {
		if buf[i] == 0x7F && buf[i+1] == 0xFE &&
			buf[i+2] == 0x80 && buf[i+3] == 0x01 {
			if DTSCoreFrameSize(buf[i:i+7]) > 0 {
				syncPositions = append(syncPositions, i)
			}
		}
	}
	if len(syncPositions) == 0 {
		return 0
	}

	dtsSyncPos := syncPositions[0]

	// Find actual core boundary from the first frame by scanning for ExSS
	// sync or next DTS sync.
	coreSize := 0
	for i := dtsSyncPos + 7; i+3 < len(buf); i++ {
		// ExSS sync: 64 58 20 25
		if buf[i] == 0x64 && buf[i+1] == 0x58 &&
			buf[i+2] == 0x20 && buf[i+3] == 0x25 {
			coreSize = i - dtsSyncPos
			break
		}
		// Next DTS core sync: 7F FE 80 01 (validated)
		if buf[i] == 0x7F && buf[i+1] == 0xFE &&
			buf[i+2] == 0x80 && buf[i+3] == 0x01 {
			if i+6 < len(buf) && DTSCoreFrameSize(buf[i:i+7]) > 0 {
				coreSize = i - dtsSyncPos
				break
			}
		}
	}

	if coreSize == 0 {
		// Could not find boundary — fall back to FSIZE from header.
		return DTSCoreFrameSize(buf[dtsSyncPos : dtsSyncPos+7])
	}

	// Validate that all detected frames have a consistent core size.
	// DTS core on Blu-ray uses CBR, so frame sizes should be uniform.
	// If they vary, the single-size assumption in splitDTSHDCoreRanges
	// would produce incorrect ranges.
	for i := 1; i < len(syncPositions)-1; i++ {
		frameSize := syncPositions[i+1] - syncPositions[i]
		// In a DTS-HD stream, the distance between consecutive DTS syncs
		// is the full access unit (core + extension). But the distance
		// from a DTS sync to the next ExSS should equal coreSize.
		// We can't easily re-detect ExSS for each frame here, but we can
		// check that all access unit sizes are equal (implying consistent
		// core sizes within a uniform structure).
		if i == 1 {
			continue // need at least two intervals to compare
		}
		prevFrameSize := syncPositions[i] - syncPositions[i-1]
		if frameSize != prevFrameSize {
			log.Printf("mpegts: warning: DTS-HD stream has variable frame sizes (%d vs %d bytes); skipping core extraction", prevFrameSize, frameSize)
			return 0
		}
	}

	return coreSize
}

// splitDTSHDCoreRanges extracts DTS core frame ranges from a combined DTS-HD
// stream. It walks through PES payload ranges, identifies DTS core frames by
// their sync word, and collects only the core bytes (excluding DTS-HD extension
// data).
//
// In DTS-HD streams, the FSIZE header field reports the full access unit size
// (core + extension), not the core-only size. We detect the actual core size
// by scanning for the ExSS boundary in detectActualDTSCoreSize.
func (p *MPEGTSParser) splitDTSHDCoreRanges(ranges []PESPayloadRange) []PESPayloadRange {
	// Detect actual core frame size by scanning the stream.
	actualCoreSize := p.detectActualDTSCoreSize(ranges)
	if actualCoreSize <= 0 {
		return nil
	}

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
				// This range doesn't have enough bytes to complete the
				// 7-byte header. Abandon the speculative cross-range
				// header and fall through to normal scanning so these
				// bytes aren't silently skipped if the header turns out
				// to be invalid.
				if len(coreRanges) > 0 {
					last := coreRanges[len(coreRanges)-1]
					coreRanges = coreRanges[:len(coreRanges)-1]
					coreES -= int64(last.Size)
				}
				headerPendingRanges = nil
				headerBufLen = 0
				// Fall through to scanLoop below
			} else {
				copy(headerBuf[headerBufLen:], data[:need])
				if DTSCoreFrameSize(headerBuf[:7]) > 0 {
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
					// Use detected core size, not FSIZE. Subtract the 7 header
					// bytes already consumed (from buffer + current range).
					coreRemaining = actualCoreSize - 7
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
				if DTSCoreFrameSize(data[pos:pos+7]) > 0 {
					coreRemaining = actualCoreSize
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
			// Look for 0x7F (start of DTS sync word) near end of range.
			// We need up to 7 bytes (4-byte sync + 3 bytes) for DTSCoreFrameSize(),
			// so search the last 6 bytes in case the sync word starts at len(data)-6
			// or len(data)-5 and continues into the next range.
			checkStart := len(data) - 6
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
		}
	}

	return coreRanges
}
