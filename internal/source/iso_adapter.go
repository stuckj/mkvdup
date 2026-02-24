package source

// isoM2TSAdapter wraps an MPEGTSParser to adjust FileOffset values for
// an M2TS region embedded within a Blu-ray ISO file. The parser operates
// on a sub-slice of the ISO data (the M2TS region), producing FileOffset
// values relative to the sub-slice. This adapter adds baseOffset to
// produce ISO-relative FileOffset values for range maps, while delegating
// ES-offset-based reads to the wrapped parser unchanged.
type isoM2TSAdapter struct {
	parser     *MPEGTSParser
	isoData    []byte // full ISO mmap data
	baseOffset int64  // M2TS region start offset within the ISO
}

// newISOAdapter creates an adapter for an M2TS region within an ISO.
func newISOAdapter(parser *MPEGTSParser, isoData []byte, baseOffset int64) *isoM2TSAdapter {
	return &isoM2TSAdapter{
		parser:     parser,
		isoData:    isoData,
		baseOffset: baseOffset,
	}
}

// --- esDataProvider interface (used by indexer) ---

// Data returns the full ISO data so indexESData() can do
// data[r.FileOffset:endOffset] with ISO-relative FileOffset values.
func (a *isoM2TSAdapter) Data() []byte {
	return a.isoData
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

// adjustRanges creates a copy of ranges with baseOffset added to FileOffset.
func (a *isoM2TSAdapter) adjustRanges(ranges []PESPayloadRange) []PESPayloadRange {
	if len(ranges) == 0 {
		return ranges
	}
	adjusted := make([]PESPayloadRange, len(ranges))
	for i, r := range ranges {
		adjusted[i] = PESPayloadRange{
			FileOffset: r.FileOffset + a.baseOffset,
			Size:       r.Size,
			ESOffset:   r.ESOffset,
		}
	}
	return adjusted
}

// adjustRawRanges creates a copy of raw ranges with baseOffset added to FileOffset.
func (a *isoM2TSAdapter) adjustRawRanges(ranges []RawRange) []RawRange {
	adjusted := make([]RawRange, len(ranges))
	for i, r := range ranges {
		adjusted[i] = RawRange{
			FileOffset: r.FileOffset + a.baseOffset,
			Size:       r.Size,
		}
	}
	return adjusted
}
