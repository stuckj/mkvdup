package source

import "sort"

// isoM2TSAdapter wraps an MPEGTSParser to adjust FileOffset values for
// an M2TS region embedded within a Blu-ray ISO file. The parser operates
// on a sub-slice of the ISO data (the M2TS region), producing FileOffset
// values relative to the sub-slice. This adapter adds baseOffset to
// produce ISO-relative FileOffset values for range maps, while delegating
// ES-offset-based reads to the wrapped parser unchanged.
//
// For multi-extent UDF files where the M2TS data is non-contiguous in the
// ISO, the adapter uses a multiRegionData to provide a virtual contiguous
// view over the mmap sub-slices, and maps logical offsets to physical ISO
// offsets via an extent table.
type isoM2TSAdapter struct {
	parser     *MPEGTSParser
	isoData    []byte // full ISO mmap data (used by Data() for contiguous case)
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
// isoData is the full ISO mmap data (for DataSize bounds checking).
func newISOAdapterMultiExtent(parser *MPEGTSParser, mr *multiRegionData, extents []isoPhysicalRange, isoData []byte) *isoM2TSAdapter {
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
		isoData:   isoData,
		mr:        mr,
		extentMap: em,
	}
}

// --- esDataProvider interface (used by indexer) ---

// Data returns the backing data buffer. For contiguous files, this is the
// full ISO mmap (FileOffset values are ISO-relative). For multi-extent files,
// returns nil — use DataSlice instead.
func (a *isoM2TSAdapter) Data() []byte {
	if a.mr != nil {
		return nil
	}
	return a.isoData
}

// DataSlice returns a sub-slice of the backing data at the given offset and size.
// Offsets are ISO-relative (from adjustRanges). Works for both contiguous and
// multi-extent files.
func (a *isoM2TSAdapter) DataSlice(off int64, size int) []byte {
	if a.mr != nil {
		// Multi-extent: convert ISO-relative offset to assembled-relative,
		// then resolve through the virtual contiguous view.
		logicalOff := a.isoToLogical(off)
		return a.mr.Slice(logicalOff, logicalOff+int64(size))
	}
	// Contiguous: offset is ISO-relative (adjusted by adjustRanges)
	return a.isoData[off : off+int64(size)]
}

// DataSize returns the size of the backing ISO data.
// Used for bounds-checking ISO-relative offsets from adjustRanges.
func (a *isoM2TSAdapter) DataSize() int64 {
	return int64(len(a.isoData))
}

func (a *isoM2TSAdapter) FilteredVideoRanges() []PESPayloadRange {
	return a.adjustRanges(a.parser.FilteredVideoRanges())
}

func (a *isoM2TSAdapter) FilteredAudioRanges(subStreamID byte) []PESPayloadRange {
	return a.adjustRanges(a.parser.FilteredAudioRanges(subStreamID))
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

// adjustRanges creates a copy of ranges with FileOffset converted to ISO-relative.
// For contiguous files, adds baseOffset. For multi-extent files, converts
// assembled-relative offsets to ISO-relative via logicalToISO.
// ISO-relative offsets are needed for range maps stored in the dedup file
// (reconstruction reads from the ISO at these offsets).
func (a *isoM2TSAdapter) adjustRanges(ranges []PESPayloadRange) []PESPayloadRange {
	if len(ranges) == 0 {
		return ranges
	}
	adjusted := make([]PESPayloadRange, len(ranges))
	if a.mr != nil {
		// Multi-extent: convert assembled-relative to ISO-relative
		for i, r := range ranges {
			adjusted[i] = PESPayloadRange{
				FileOffset: a.logicalToISO(r.FileOffset),
				Size:       r.Size,
				ESOffset:   r.ESOffset,
			}
		}
		return adjusted
	}
	for i, r := range ranges {
		adjusted[i] = PESPayloadRange{
			FileOffset: r.FileOffset + a.baseOffset,
			Size:       r.Size,
			ESOffset:   r.ESOffset,
		}
	}
	return adjusted
}

// adjustRawRanges creates a copy of raw ranges with offsets adjusted.
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

// isoToLogical converts a physical ISO offset to the corresponding logical
// offset in the assembled data. Used by DataSlice to convert ISO-relative
// offsets (from adjustRanges) back to assembled-relative for multiRegionData.
func (a *isoM2TSAdapter) isoToLogical(isoOff int64) int64 {
	for _, e := range a.extentMap {
		if isoOff >= e.ISOOffset && isoOff < e.ISOOffset+e.Length {
			return e.LogicalStart + (isoOff - e.ISOOffset)
		}
	}
	return 0
}

// logicalToISO converts a logical offset in the assembled data to the
// corresponding physical ISO offset using the extent map.
func (a *isoM2TSAdapter) logicalToISO(logicalOff int64) int64 {
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
