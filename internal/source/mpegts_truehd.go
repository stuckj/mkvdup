package source

// splitTrueHDAC3Streams detects combined TrueHD+AC3 audio streams and splits
// them into separate sub-streams. On Blu-ray, TrueHD streams (PMT type 0x83)
// interleave an AC3 compatibility core in the same PID. Video extraction tools split
// these into separate MKV tracks, so we must split them here to match.
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
