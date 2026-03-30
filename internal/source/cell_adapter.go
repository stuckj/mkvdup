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
	filteredVideoRanges      []PESPayloadRange
	filteredAudioBySubStream map[byte][]PESPayloadRange
	audioSubStreams          []byte
	lpcmSubStreams           map[byte]bool
}

// newCellSegmentAdapter creates an adapter for a specific cell segment of the parser.
// It extracts the segment's video/audio ranges, rebuilds filtered ranges with
// segment-local ES offsets, and inherits LPCM metadata from the parser.
func newCellSegmentAdapter(parser *MPEGPSParser, segIdx int) *cellSegmentAdapter {
	// Extract segment's raw video/audio ranges (rebased to ES offset 0)
	videoRanges := parser.CellSegmentVideoRanges(segIdx)
	audioRanges, audioRangeStreamIDs := parser.CellSegmentAudioRanges(segIdx)

	// Build filtered video ranges (exclude user_data sections)
	filteredVideoRanges := buildFilteredVideoRangesFromData(parser.data, parser.size, videoRanges)

	// Build filtered audio ranges per sub-stream (reuse parser's LPCM detection)
	audioResult := buildFilteredAudioRangesFromData(
		parser.data, parser.size,
		audioRanges, audioRangeStreamIDs,
		parser.lpcmSubStreams,
	)

	return &cellSegmentAdapter{
		parser:                   parser,
		filteredVideoRanges:      filteredVideoRanges,
		filteredAudioBySubStream: audioResult.RangesBySubStream,
		audioSubStreams:          audioResult.SubStreams,
		lpcmSubStreams:           audioResult.LPCMSubStreams,
	}
}

// Parser returns the underlying MPEGPSParser. Used by codec detection
// which needs access to parser-level stream metadata.
func (a *cellSegmentAdapter) Parser() *MPEGPSParser {
	return a.parser
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
	return nil
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

	return readLPCMSubStreamData(a.parser.data, a.parser.size, ranges, esOffset, size)
}

func (a *cellSegmentAdapter) IsLPCMSubStream(subStreamID byte) bool {
	return a.lpcmSubStreams[subStreamID]
}

// --- ESReader interface (used by matcher/reconstruction) ---

func (a *cellSegmentAdapter) ESOffsetToFileOffset(esOffset int64, isVideo bool) (fileOffset int64, remaining int) {
	if !isVideo {
		// Audio uses per-sub-stream methods; generic ES offset lookup
		// is not meaningful for audio.
		return -1, 0
	}
	for _, r := range a.FilteredVideoRanges() {
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
