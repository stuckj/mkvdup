package source

// cellSegmentAdapter wraps an MPEGPSParser to provide access to a single
// cell segment's elementary stream data. Each adapter views a subset of the
// parser's PES payload ranges (the VOBUs belonging to one cell segment) with
// ES offsets renumbered from 0.
//
// This enables correct matching on multi-episode DVDs where cells from
// different titles are interleaved in the VOB files. Each cell segment
// gets its own source file entry and ESReader, so the matcher treats them
// as independent streams — just like multiple M2TS regions in a Blu-ray ISO.
type cellSegmentAdapter struct {
	parser *MPEGPSParser // backing parser with raw mmap'd data

	// Per-segment ranges with ES offsets starting at 0
	videoRanges              []PESPayloadRange
	filteredVideoRanges      []PESPayloadRange
	audioRanges              []PESPayloadRange
	audioRangeStreamIDs      []byte
	filteredAudioBySubStream map[byte][]PESPayloadRange
	audioSubStreams          []byte
	lpcmSubStreams           map[byte]bool
	lpcmInfo                 map[byte]LPCMFrameHeader
}

// newCellSegmentAdapter creates an adapter for a specific cell segment of the parser.
// It extracts the segment's video/audio ranges, rebuilds filtered ranges with
// segment-local ES offsets, and copies LPCM metadata from the parser.
func newCellSegmentAdapter(parser *MPEGPSParser, segIdx int) *cellSegmentAdapter {
	a := &cellSegmentAdapter{
		parser:         parser,
		lpcmSubStreams: make(map[byte]bool),
		lpcmInfo:       make(map[byte]LPCMFrameHeader),
	}

	// Copy LPCM metadata from parser
	for id, isLPCM := range parser.lpcmSubStreams {
		a.lpcmSubStreams[id] = isLPCM
	}
	for id, info := range parser.lpcmInfo {
		a.lpcmInfo[id] = info
	}

	// Extract segment's raw video/audio ranges (rebased to ES offset 0)
	a.videoRanges = parser.CellSegmentVideoRanges(segIdx)
	a.audioRanges, a.audioRangeStreamIDs = parser.CellSegmentAudioRanges(segIdx)

	// Build filtered video ranges (exclude user_data sections)
	a.filteredVideoRanges = buildFilteredVideoRangesFromData(parser.data, parser.size, a.videoRanges)

	// Build filtered audio ranges per sub-stream
	a.buildFilteredAudioRanges()

	return a
}

// buildFilteredAudioRanges creates per-sub-stream filtered audio ranges for this segment.
// Same logic as MPEGPSParser.buildFilteredAudioRanges but operating on the segment's subset.
func (a *cellSegmentAdapter) buildFilteredAudioRanges() {
	if len(a.audioRanges) == 0 {
		return
	}

	rangesBySubStream := make(map[byte][]PESPayloadRange)
	esOffsetBySubStream := make(map[byte]int64)
	seenSubStreams := make(map[byte]bool)

	for i, rawRange := range a.audioRanges {
		if rawRange.FileOffset >= a.parser.size {
			continue
		}

		pesStreamID := a.audioRangeStreamIDs[i]

		// MPEG-1 audio streams (0xC0-0xDF): payload is raw MP2 data, no sub-stream header
		if pesStreamID >= 0xC0 && pesStreamID <= 0xDF {
			if rawRange.Size <= 0 {
				continue
			}
			if !seenSubStreams[pesStreamID] {
				seenSubStreams[pesStreamID] = true
				a.audioSubStreams = append(a.audioSubStreams, pesStreamID)
			}
			esOffset := esOffsetBySubStream[pesStreamID]
			rangesBySubStream[pesStreamID] = append(rangesBySubStream[pesStreamID], PESPayloadRange{
				FileOffset: rawRange.FileOffset,
				Size:       rawRange.Size,
				ESOffset:   esOffset,
			})
			esOffsetBySubStream[pesStreamID] += int64(rawRange.Size)
			continue
		}

		// Private Stream 1 (0xBD): has sub-stream header
		if rawRange.Size < 4 {
			continue
		}

		subStreamID := a.parser.data[rawRange.FileOffset]

		isAC3 := subStreamID >= 0x80 && subStreamID <= 0x87
		isDTS := subStreamID >= 0x88 && subStreamID <= 0x8F
		isLPCM := subStreamID >= 0xA0 && subStreamID <= 0xA7

		if isAC3 || isDTS || isLPCM {
			if !seenSubStreams[subStreamID] {
				seenSubStreams[subStreamID] = true
				a.audioSubStreams = append(a.audioSubStreams, subStreamID)
			}

			if isLPCM {
				if rawRange.Size > LPCMTotalHeaderSize {
					esOffset := esOffsetBySubStream[subStreamID]
					rangesBySubStream[subStreamID] = append(rangesBySubStream[subStreamID], PESPayloadRange{
						FileOffset: rawRange.FileOffset + LPCMTotalHeaderSize,
						Size:       rawRange.Size - LPCMTotalHeaderSize,
						ESOffset:   esOffset,
					})
					esOffsetBySubStream[subStreamID] += int64(rawRange.Size - LPCMTotalHeaderSize)
				}
			} else {
				if rawRange.Size > 4 {
					esOffset := esOffsetBySubStream[subStreamID]
					rangesBySubStream[subStreamID] = append(rangesBySubStream[subStreamID], PESPayloadRange{
						FileOffset: rawRange.FileOffset + 4,
						Size:       rawRange.Size - 4,
						ESOffset:   esOffset,
					})
					esOffsetBySubStream[subStreamID] += int64(rawRange.Size - 4)
				}
			}
		}
	}

	a.filteredAudioBySubStream = rangesBySubStream
}

// --- esDataProvider interface (used by indexer) ---

func (a *cellSegmentAdapter) Data() []byte {
	return a.parser.data
}

func (a *cellSegmentAdapter) DataSlice(off int64, size int) []byte {
	return a.parser.data[off : off+int64(size)]
}

func (a *cellSegmentAdapter) DataSize() int64 {
	return a.parser.size
}

func (a *cellSegmentAdapter) FilteredVideoRanges() []PESPayloadRange {
	if len(a.filteredVideoRanges) > 0 {
		return a.filteredVideoRanges
	}
	return a.videoRanges
}

func (a *cellSegmentAdapter) FilteredAudioRanges(subStreamID byte) []PESPayloadRange {
	return a.filteredAudioBySubStream[subStreamID]
}

func (a *cellSegmentAdapter) ReadESData(esOffset int64, size int, isVideo bool) ([]byte, error) {
	if !isVideo {
		return nil, errAudioUsesSubStream
	}
	return readFromRanges(a.parser.data, nil, a.parser.size, a.FilteredVideoRanges(), esOffset, size)
}

func (a *cellSegmentAdapter) ReadAudioSubStreamData(subStreamID byte, esOffset int64, size int) ([]byte, error) {
	ranges := a.filteredAudioBySubStream[subStreamID]
	if ranges == nil {
		return nil, errSubStreamNotFound(subStreamID)
	}

	if !a.lpcmSubStreams[subStreamID] {
		return readFromRanges(a.parser.data, nil, a.parser.size, ranges, esOffset, size)
	}

	// LPCM 16-bit forward transform (same as MPEGPSParser.ReadAudioSubStreamData)
	alignedOffset := esOffset
	trimFront := 0
	if esOffset%2 == 1 {
		alignedOffset = esOffset - 1
		trimFront = 1
	}
	alignedSize := size + trimFront
	trimBack := 0
	if alignedSize%2 == 1 {
		alignedSize++
		trimBack = 1
	}

	data, err := readFromRanges(a.parser.data, nil, a.parser.size, ranges, alignedOffset, alignedSize)
	if err != nil {
		if trimBack > 0 {
			alignedSize--
			trimBack = 0
			data, err = readFromRanges(a.parser.data, nil, a.parser.size, ranges, alignedOffset, alignedSize)
		}
		if err != nil {
			return nil, err
		}
	}

	result := make([]byte, len(data))
	copy(result, data)
	TransformLPCM16BE(result)

	start := trimFront
	end := start + size
	if end > len(result) {
		end = len(result)
	}
	return result[start:end], nil
}

func (a *cellSegmentAdapter) IsLPCMSubStream(subStreamID byte) bool {
	return a.lpcmSubStreams[subStreamID]
}

// --- ESReader interface (used by matcher/reconstruction) ---

func (a *cellSegmentAdapter) ESOffsetToFileOffset(esOffset int64, isVideo bool) (fileOffset int64, remaining int) {
	var ranges []PESPayloadRange
	if isVideo {
		ranges = a.FilteredVideoRanges()
	} else {
		// Audio uses per-sub-stream methods; this fallback uses video ranges
		ranges = a.FilteredVideoRanges()
	}
	for _, r := range ranges {
		if esOffset >= r.ESOffset && esOffset < r.ESOffset+int64(r.Size) {
			offsetInPayload := esOffset - r.ESOffset
			return r.FileOffset + offsetInPayload, r.Size - int(offsetInPayload)
		}
	}
	return -1, 0
}

func (a *cellSegmentAdapter) TotalESSize(isVideo bool) int64 {
	if !isVideo {
		return 0
	}
	return totalESSizeFromRanges(a.FilteredVideoRanges())
}

func (a *cellSegmentAdapter) AudioSubStreams() []byte {
	return a.audioSubStreams
}

func (a *cellSegmentAdapter) AudioSubStreamESSize(subStreamID byte) int64 {
	return totalESSizeFromRanges(a.filteredAudioBySubStream[subStreamID])
}

// --- hintedESReader interface (used by matcher expand) ---

func (a *cellSegmentAdapter) ReadESByteWithHint(esOffset int64, isVideo bool, rangeHint int) (byte, int, bool) {
	if !isVideo {
		return 0, -1, false
	}
	return readByteWithHint(a.parser.data, nil, a.parser.size, a.FilteredVideoRanges(), esOffset, rangeHint)
}

func (a *cellSegmentAdapter) ReadAudioByteWithHint(subStreamID byte, esOffset int64, rangeHint int) (byte, int, bool) {
	if a.lpcmSubStreams[subStreamID] {
		swappedOffset := esOffset ^ 1
		return readByteWithHint(a.parser.data, nil, a.parser.size, a.filteredAudioBySubStream[subStreamID], swappedOffset, rangeHint)
	}
	return readByteWithHint(a.parser.data, nil, a.parser.size, a.filteredAudioBySubStream[subStreamID], esOffset, rangeHint)
}

// --- PESRangeProvider interface (used for range map creation) ---
// FilteredVideoRanges, FilteredAudioRanges, and AudioSubStreams already defined above.

// --- ESRangeConverter interface (for V3 format) ---

func (a *cellSegmentAdapter) RawRangesForESRegion(esOffset int64, size int, isVideo bool) ([]RawRange, error) {
	if !isVideo {
		return nil, errAudioUsesSubStream
	}
	return rawRangesFromPESRanges(a.FilteredVideoRanges(), esOffset, size)
}

func (a *cellSegmentAdapter) RawRangesForAudioSubStream(subStreamID byte, esOffset int64, size int) ([]RawRange, error) {
	ranges := a.filteredAudioBySubStream[subStreamID]
	if ranges == nil {
		return nil, errSubStreamNotFound(subStreamID)
	}
	return rawRangesFromPESRanges(ranges, esOffset, size)
}
