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
// stream into separate AC3 and TrueHD ranges using AU-aware parsing.
//
// The interleaved stream alternates between AC3 frames and TrueHD access units
// at unit boundaries. At each boundary, the parser checks for the AC3 sync word
// (0B 77) to identify AC3 frames, or reads the TrueHD AU length header to
// determine the AU size. This avoids false-positive AC3 detection inside TrueHD
// AU data, which the previous byte-scan approach was susceptible to.
func (p *MPEGTSParser) splitCombinedAudioRanges(ranges []PESPayloadRange) (ac3Ranges, truehdRanges []PESPayloadRange) {
	var ac3ES, truehdES int64
	ac3Remaining := 0    // bytes remaining in current AC3 frame
	truehdRemaining := 0 // bytes remaining in current TrueHD AU

	// Cross-boundary header buffer. At unit boundaries, we need 2 bytes
	// to determine type (AC3 vs TrueHD), or 5 bytes if starting with
	// AC3 sync 0B 77 (to read fscod+frmsizecod at byte 4).
	var headerBuf [5]byte
	headerBufLen := 0

	type pendingRange struct {
		fileOffset int64
		size       int
	}
	var pendingRanges []pendingRange

	for _, r := range ranges {
		endOffset := r.FileOffset + int64(r.Size)
		if endOffset > p.size {
			continue
		}
		data := p.dataSlice(r.FileOffset, endOffset)
		pos := 0

		// Resolve buffered header bytes from previous range
		if headerBufLen > 0 && ac3Remaining == 0 && truehdRemaining == 0 {
			// Determine how many total bytes we need
			needTotal := 2
			if headerBufLen >= 2 && headerBuf[0] == 0x0B && headerBuf[1] == 0x77 {
				needTotal = 5
			}
			need := needTotal - headerBufLen
			available := len(data) - pos
			if need > available {
				// Still not enough data — buffer more
				copy(headerBuf[headerBufLen:], data[pos:])
				headerBufLen += available
				pendingRanges = append(pendingRanges, pendingRange{r.FileOffset + int64(pos), available})
				continue
			}

			copy(headerBuf[headerBufLen:], data[pos:pos+need])
			consumedFromCurrent := need
			headerBufLen += need

			// Re-check: we may now have 0B 77 and need more bytes
			if headerBufLen >= 2 && headerBuf[0] == 0x0B && headerBuf[1] == 0x77 && headerBufLen < 5 {
				moreNeed := 5 - headerBufLen
				moreAvail := len(data) - pos - consumedFromCurrent
				if moreNeed > moreAvail {
					// Still not enough for full AC3 header
					copy(headerBuf[headerBufLen:], data[pos+consumedFromCurrent:])
					pendingRanges = append(pendingRanges, pendingRange{r.FileOffset + int64(pos), consumedFromCurrent + moreAvail})
					headerBufLen += moreAvail
					continue
				}
				copy(headerBuf[headerBufLen:], data[pos+consumedFromCurrent:pos+consumedFromCurrent+moreNeed])
				consumedFromCurrent += moreNeed
				headerBufLen += moreNeed
			}

			// Classify the unit
			isAC3 := false
			unitSize := 0

			if headerBuf[0] == 0x0B && headerBuf[1] == 0x77 && headerBufLen >= 5 {
				fscod := (headerBuf[4] >> 6) & 0x03
				frmsizecod := headerBuf[4] & 0x3F
				frameSize := AC3FrameSize(fscod, frmsizecod)
				if frameSize > 0 {
					isAC3 = true
					unitSize = frameSize
				}
			}

			if !isAC3 {
				auLen := ParseTrueHDAULength(headerBuf[:2])
				if auLen >= 4 {
					unitSize = auLen
				}
			}

			if unitSize > 0 {
				// Attribute pending ranges + consumed bytes from current range
				if isAC3 {
					for _, pr := range pendingRanges {
						ac3Ranges = append(ac3Ranges, PESPayloadRange{
							FileOffset: pr.fileOffset,
							Size:       pr.size,
							ESOffset:   ac3ES,
						})
						ac3ES += int64(pr.size)
					}
					if consumedFromCurrent > 0 {
						ac3Ranges = append(ac3Ranges, PESPayloadRange{
							FileOffset: r.FileOffset + int64(pos),
							Size:       consumedFromCurrent,
							ESOffset:   ac3ES,
						})
						ac3ES += int64(consumedFromCurrent)
					}
					ac3Remaining = unitSize - headerBufLen
				} else {
					for _, pr := range pendingRanges {
						truehdRanges = append(truehdRanges, PESPayloadRange{
							FileOffset: pr.fileOffset,
							Size:       pr.size,
							ESOffset:   truehdES,
						})
						truehdES += int64(pr.size)
					}
					if consumedFromCurrent > 0 {
						truehdRanges = append(truehdRanges, PESPayloadRange{
							FileOffset: r.FileOffset + int64(pos),
							Size:       consumedFromCurrent,
							ESOffset:   truehdES,
						})
						truehdES += int64(consumedFromCurrent)
					}
					truehdRemaining = unitSize - headerBufLen
				}
			} else {
				// Unrecognized — attribute all buffered bytes to TrueHD
				for _, pr := range pendingRanges {
					truehdRanges = append(truehdRanges, PESPayloadRange{
						FileOffset: pr.fileOffset,
						Size:       pr.size,
						ESOffset:   truehdES,
					})
					truehdES += int64(pr.size)
				}
				if consumedFromCurrent > 0 {
					truehdRanges = append(truehdRanges, PESPayloadRange{
						FileOffset: r.FileOffset + int64(pos),
						Size:       consumedFromCurrent,
						ESOffset:   truehdES,
					})
					truehdES += int64(consumedFromCurrent)
				}
			}

			pos += consumedFromCurrent
			headerBufLen = 0
			pendingRanges = nil
		}

		for pos < len(data) {
			if ac3Remaining > 0 {
				consume := min(ac3Remaining, len(data)-pos)
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

			if truehdRemaining > 0 {
				consume := min(truehdRemaining, len(data)-pos)
				truehdRanges = append(truehdRanges, PESPayloadRange{
					FileOffset: r.FileOffset + int64(pos),
					Size:       consume,
					ESOffset:   truehdES,
				})
				truehdES += int64(consume)
				truehdRemaining -= consume
				pos += consume
				continue
			}

			// At unit boundary — determine type
			available := len(data) - pos

			// Need at least 2 bytes to determine type
			if available < 2 {
				copy(headerBuf[:], data[pos:])
				headerBufLen = available
				pendingRanges = []pendingRange{{r.FileOffset + int64(pos), available}}
				pos = len(data)
				continue
			}

			// Check for AC3 sync word
			if data[pos] == 0x0B && data[pos+1] == 0x77 {
				if available < 5 {
					// Need more bytes for AC3 header
					copy(headerBuf[:], data[pos:pos+available])
					headerBufLen = available
					pendingRanges = []pendingRange{{r.FileOffset + int64(pos), available}}
					pos = len(data)
					continue
				}

				fscod := (data[pos+4] >> 6) & 0x03
				frmsizecod := data[pos+4] & 0x3F
				frameSize := AC3FrameSize(fscod, frmsizecod)
				if frameSize > 0 {
					ac3Remaining = frameSize
					continue
				}
			}

			// TrueHD AU: parse length from first 2 bytes
			auLen := ParseTrueHDAULength(data[pos:])
			if auLen >= 4 {
				truehdRemaining = auLen
				continue
			}

			// Unrecognized — consume byte-by-byte as TrueHD
			truehdRanges = append(truehdRanges, PESPayloadRange{
				FileOffset: r.FileOffset + int64(pos),
				Size:       1,
				ESOffset:   truehdES,
			})
			truehdES++
			pos++
		}
	}

	// Attribute remaining buffered bytes to TrueHD
	if headerBufLen > 0 {
		for _, pr := range pendingRanges {
			truehdRanges = append(truehdRanges, PESPayloadRange{
				FileOffset: pr.fileOffset,
				Size:       pr.size,
				ESOffset:   truehdES,
			})
			truehdES += int64(pr.size)
		}
	}

	return ac3Ranges, truehdRanges
}
