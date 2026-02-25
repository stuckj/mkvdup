package source

import "sort"

// isoM2TSAdapter wraps an MPEGTSParser to provide ISO-level integration
// for an M2TS region embedded within a Blu-ray ISO file. The parser operates
// on a sub-slice (contiguous) or virtual contiguous view (multi-extent) of
// the ISO data, producing FileOffset values relative to that view.
//
// The adapter handles two offset domains:
//   - Parser-relative: used by FilteredVideoRanges (zero-copy from parser),
//     DataSlice (adds baseOffset / resolves via multiRegionData internally),
//     and all ES-offset-based reads.
//   - ISO-relative: used by range maps stored in the dedup file. The
//     FileOffsetConverter method provides the conversion function, applied
//     lazily during range map encoding to avoid copying range arrays.
type isoM2TSAdapter struct {
	parser     *MPEGTSParser
	isoData    []byte // full ISO mmap data (contiguous case: used by Data/DataSlice)
	baseOffset int64  // M2TS region start offset within the ISO

	// For non-contiguous multi-extent files:
	mr        *multiRegionData // virtual contiguous view over mmap sub-slices
	extentMap []extentMapEntry // maps logical offset → ISO offset
}

// extentMapEntry maps a range of logical (assembled) offsets to physical ISO offsets.
type extentMapEntry struct {
	LogicalStart int64 // start offset in assembled data
	ISOOffset    int64 // corresponding offset in the ISO file
	Length       int64 // length of this extent
}

// newISOAdapter creates an adapter for an M2TS region within an ISO.
func newISOAdapter(parser *MPEGTSParser, isoData []byte, baseOffset int64) *isoM2TSAdapter {
	return &isoM2TSAdapter{
		parser:     parser,
		isoData:    isoData,
		baseOffset: baseOffset,
	}
}

// newISOAdapterMultiExtent creates an adapter for a non-contiguous M2TS region.
// mr provides a virtual contiguous view over the mmap sub-slices.
// extents describes the physical layout in the ISO.
func newISOAdapterMultiExtent(parser *MPEGTSParser, mr *multiRegionData, extents []isoPhysicalRange) *isoM2TSAdapter {
	// Build the extent map with cumulative logical offsets
	em := make([]extentMapEntry, len(extents))
	logicalOff := int64(0)
	for i, ext := range extents {
		em[i] = extentMapEntry{
			LogicalStart: logicalOff,
			ISOOffset:    ext.ISOOffset,
			Length:       ext.Length,
		}
		logicalOff += ext.Length
	}
	return &isoM2TSAdapter{
		parser:    parser,
		mr:        mr,
		extentMap: em,
	}
}

// --- esDataProvider interface (used by indexer) ---

// Data returns the backing data buffer. For contiguous files, this is the
// full ISO mmap. For multi-extent files, returns nil — use DataSlice instead.
func (a *isoM2TSAdapter) Data() []byte {
	if a.mr != nil {
		return nil
	}
	return a.isoData
}

// DataSlice returns a sub-slice of the backing data at the given offset and size.
// Offsets are parser-relative (from FilteredVideoRanges). The adapter handles
// the mapping to ISO data internally.
func (a *isoM2TSAdapter) DataSlice(off int64, size int) []byte {
	if a.mr != nil {
		// Multi-extent: parser-relative = assembled-relative, resolve via mr
		return a.mr.Slice(off, off+int64(size))
	}
	// Contiguous: parser-relative + baseOffset = ISO-relative
	return a.isoData[off+a.baseOffset : off+a.baseOffset+int64(size)]
}

// DataSize returns the parser's data size (for bounds checking parser-relative offsets).
func (a *isoM2TSAdapter) DataSize() int64 {
	return a.parser.DataSize()
}

// FilteredVideoRanges returns the parser's filtered video ranges (zero-copy).
// FileOffset values are parser-relative. Use FileOffsetConverter to get
// ISO-relative offsets for range map encoding.
func (a *isoM2TSAdapter) FilteredVideoRanges() []PESPayloadRange {
	return a.parser.FilteredVideoRanges()
}

// FilteredAudioRanges returns the parser's filtered audio ranges (zero-copy).
func (a *isoM2TSAdapter) FilteredAudioRanges(subStreamID byte) []PESPayloadRange {
	return a.parser.FilteredAudioRanges(subStreamID)
}

func (a *isoM2TSAdapter) ReadESData(esOffset int64, size int, isVideo bool) ([]byte, error) {
	return a.parser.ReadESData(esOffset, size, isVideo)
}

func (a *isoM2TSAdapter) ReadAudioSubStreamData(subStreamID byte, esOffset int64, size int) ([]byte, error) {
	return a.parser.ReadAudioSubStreamData(subStreamID, esOffset, size)
}

// --- ESReader interface (used by matcher/reconstruction) ---

func (a *isoM2TSAdapter) ESOffsetToFileOffset(esOffset int64, isVideo bool) (fileOffset int64, remaining int) {
	fOff, rem := a.parser.ESOffsetToFileOffset(esOffset, isVideo)
	if a.mr != nil {
		// Multi-extent: parser offset is assembled-relative,
		// convert to ISO-relative for range maps / reconstruction.
		return a.logicalToISO(fOff), rem
	}
	return fOff + a.baseOffset, rem
}

func (a *isoM2TSAdapter) TotalESSize(isVideo bool) int64 {
	return a.parser.TotalESSize(isVideo)
}

func (a *isoM2TSAdapter) AudioSubStreams() []byte {
	return a.parser.AudioSubStreams()
}

func (a *isoM2TSAdapter) AudioSubStreamESSize(subStreamID byte) int64 {
	return a.parser.AudioSubStreamESSize(subStreamID)
}

// --- PESRangeProvider interface (used for range map creation) ---
// FilteredVideoRanges and FilteredAudioRanges already defined above.
// AudioSubStreams already defined above.

// --- FileOffsetAdjuster interface ---

// FileOffsetConverter returns a function that converts parser-relative
// FileOffset values to ISO-relative offsets for range map storage.
func (a *isoM2TSAdapter) FileOffsetConverter() func(int64) int64 {
	if a.mr != nil {
		return a.logicalToISO
	}
	baseOff := a.baseOffset
	return func(off int64) int64 { return off + baseOff }
}

// --- hintedESReader interface (used by matcher expand) ---

func (a *isoM2TSAdapter) ReadESByteWithHint(esOffset int64, isVideo bool, rangeHint int) (byte, int, bool) {
	return a.parser.ReadESByteWithHint(esOffset, isVideo, rangeHint)
}

func (a *isoM2TSAdapter) ReadAudioByteWithHint(subStreamID byte, esOffset int64, rangeHint int) (byte, int, bool) {
	return a.parser.ReadAudioByteWithHint(subStreamID, esOffset, rangeHint)
}

// --- ESRangeConverter interface (for V3 format — adds baseOffset to raw ranges) ---

func (a *isoM2TSAdapter) RawRangesForESRegion(esOffset int64, size int, isVideo bool) ([]RawRange, error) {
	ranges, err := a.parser.RawRangesForESRegion(esOffset, size, isVideo)
	if err != nil {
		return nil, err
	}
	return a.adjustRawRanges(ranges), nil
}

func (a *isoM2TSAdapter) RawRangesForAudioSubStream(subStreamID byte, esOffset int64, size int) ([]RawRange, error) {
	ranges, err := a.parser.RawRangesForAudioSubStream(subStreamID, esOffset, size)
	if err != nil {
		return nil, err
	}
	return a.adjustRawRanges(ranges), nil
}

// --- Internal helpers ---

// adjustRawRanges creates a copy of raw ranges with offsets adjusted.
// Raw ranges are small (per-match, not per-packet) so copying is fine.
func (a *isoM2TSAdapter) adjustRawRanges(ranges []RawRange) []RawRange {
	if a.mr != nil {
		// Multi-extent: convert assembled-relative offsets to ISO-relative
		// for range maps stored in the dedup file.
		return a.mapRawRangesToISO(ranges)
	}
	adjusted := make([]RawRange, len(ranges))
	for i, r := range ranges {
		adjusted[i] = RawRange{
			FileOffset: r.FileOffset + a.baseOffset,
			Size:       r.Size,
		}
	}
	return adjusted
}

// logicalToISO converts a logical offset in the assembled data to the
// corresponding physical ISO offset using the extent map.
func (a *isoM2TSAdapter) logicalToISO(logicalOff int64) int64 {
	if len(a.extentMap) == 0 {
		return logicalOff
	}
	// Binary search for the extent containing this offset
	idx := sort.Search(len(a.extentMap), func(i int) bool {
		return a.extentMap[i].LogicalStart+a.extentMap[i].Length > logicalOff
	})
	if idx >= len(a.extentMap) {
		// Shouldn't happen — fall back to last extent
		idx = len(a.extentMap) - 1
	}
	e := a.extentMap[idx]
	return e.ISOOffset + (logicalOff - e.LogicalStart)
}

// mapRawRangesToISO converts assembled-relative raw ranges to ISO-relative ranges.
// A single assembled range may span an extent boundary, so it may be split into
// multiple ISO ranges.
func (a *isoM2TSAdapter) mapRawRangesToISO(ranges []RawRange) []RawRange {
	var result []RawRange
	for _, r := range ranges {
		remaining := int64(r.Size)
		logOff := r.FileOffset
		for remaining > 0 {
			idx := sort.Search(len(a.extentMap), func(i int) bool {
				return a.extentMap[i].LogicalStart+a.extentMap[i].Length > logOff
			})
			if idx >= len(a.extentMap) {
				break
			}
			e := a.extentMap[idx]
			offsetInExtent := logOff - e.LogicalStart
			available := e.Length - offsetInExtent
			chunk := remaining
			if chunk > available {
				chunk = available
			}
			result = append(result, RawRange{
				FileOffset: e.ISOOffset + offsetInExtent,
				Size:       int(chunk),
			})
			logOff += chunk
			remaining -= chunk
		}
	}
	return result
}
