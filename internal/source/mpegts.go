package source

import "fmt"

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

// IsLPCMSubStream always returns false for MPEG-TS (LPCM is DVD-only).
func (p *MPEGTSParser) IsLPCMSubStream(_ byte) bool {
	return false
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

// Ensure MPEGTSParser implements the required interfaces at compile time.
var (
	_ ESReader         = (*MPEGTSParser)(nil)
	_ ESRangeConverter = (*MPEGTSParser)(nil)
)
