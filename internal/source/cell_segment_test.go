package source

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// buildTestNAVPack writes a minimal NAV pack (PCI + DSI) into data at the given offset.
// Returns the number of bytes written.
func buildTestNAVPack(data []byte, offset int, ilvuCategory uint16) int {
	pos := offset

	// Pack header: 00 00 01 BA + MPEG-2 format (14 bytes)
	data[pos] = 0x00
	data[pos+1] = 0x00
	data[pos+2] = 0x01
	data[pos+3] = 0xBA
	data[pos+4] = 0x44 // MPEG-2 marker
	data[pos+13] = 0x00
	pos += 14

	// PCI packet: 00 00 01 BF + length 980
	data[pos] = 0x00
	data[pos+1] = 0x00
	data[pos+2] = 0x01
	data[pos+3] = 0xBF
	binary.BigEndian.PutUint16(data[pos+4:pos+6], 980)
	pos += 6 + 980

	// DSI packet: 00 00 01 BF + length 1018
	data[pos] = 0x00
	data[pos+1] = 0x00
	data[pos+2] = 0x01
	data[pos+3] = 0xBF
	binary.BigEndian.PutUint16(data[pos+4:pos+6], 1018)
	// SML_PBI.category at payload offset 24
	payloadStart := pos + 6
	binary.BigEndian.PutUint16(data[payloadStart+24:payloadStart+26], ilvuCategory)
	pos += 6 + 1018

	return pos - offset
}

// buildTestVideoPES writes a minimal video PES packet into data at the given offset.
// Returns the number of bytes written.
func buildTestVideoPES(data []byte, offset int, payloadSize int) int {
	pos := offset

	// PES header: 00 00 01 E0
	data[pos] = 0x00
	data[pos+1] = 0x00
	data[pos+2] = 0x01
	data[pos+3] = 0xE0
	pesLength := payloadSize + 3 // flags(2) + header_data_len(1)
	binary.BigEndian.PutUint16(data[pos+4:pos+6], uint16(pesLength))
	data[pos+6] = 0x80 // MPEG-2 PES marker
	data[pos+7] = 0x00 // no PTS/DTS
	data[pos+8] = 0x00 // header_data_length = 0
	pos += 9

	// Fill payload with identifiable pattern
	for i := 0; i < payloadSize; i++ {
		data[pos+i] = byte((offset + i) & 0xFF)
	}
	pos += payloadSize

	return pos - offset
}

func TestBuildCellSegments_NoNAVPacks(t *testing.T) {
	p := &MPEGPSParser{}
	p.buildCellSegments()

	if p.CellSegmentCount() != 0 {
		t.Errorf("CellSegmentCount() = %d, want 0", p.CellSegmentCount())
	}
}

func TestBuildCellSegments_NoInterleaving(t *testing.T) {
	// VOBUs with category=0 (non-interleaved)
	p := &MPEGPSParser{
		vobus: []vobuInfo{
			{ilvuCategory: 0, videoRangeStart: 0, audioRangeStart: 0},
			{ilvuCategory: 0, videoRangeStart: 2, audioRangeStart: 1},
			{ilvuCategory: 0, videoRangeStart: 4, audioRangeStart: 2},
		},
	}
	p.buildCellSegments()

	if p.CellSegmentCount() != 0 {
		t.Errorf("CellSegmentCount() = %d, want 0 (no interleaving)", p.CellSegmentCount())
	}
}

func TestBuildCellSegments_SimpleInterleaving(t *testing.T) {
	// Two interleaved cells: Cell A (start+end), Cell B (start+end)
	p := &MPEGPSParser{
		vobus: []vobuInfo{
			{ilvuCategory: ilvuStartBit | ilvuEndBit, videoRangeStart: 0, audioRangeStart: 0}, // Cell A ILVU
			{ilvuCategory: ilvuStartBit | ilvuEndBit, videoRangeStart: 2, audioRangeStart: 1}, // Cell B ILVU
			{ilvuCategory: ilvuStartBit | ilvuEndBit, videoRangeStart: 4, audioRangeStart: 2}, // Cell A ILVU
			{ilvuCategory: ilvuStartBit | ilvuEndBit, videoRangeStart: 6, audioRangeStart: 3}, // Cell B ILVU
		},
		videoRanges: make([]PESPayloadRange, 8),
		audioRanges: make([]PESPayloadRange, 4),
	}
	p.buildCellSegments()

	if p.CellSegmentCount() != 4 {
		t.Fatalf("CellSegmentCount() = %d, want 4", p.CellSegmentCount())
	}

	// Each segment should contain exactly one VOBU
	for i := 0; i < 4; i++ {
		seg := p.cellSegments[i]
		if seg.vobuStart != i || seg.vobuEnd != i+1 {
			t.Errorf("segment %d: vobus [%d, %d), want [%d, %d)", i, seg.vobuStart, seg.vobuEnd, i, i+1)
		}
	}
}

func TestBuildCellSegments_MultiVOBUInterleaving(t *testing.T) {
	// Cell A has 3 VOBUs, Cell B has 2 VOBUs, then Cell A again
	p := &MPEGPSParser{
		vobus: []vobuInfo{
			{ilvuCategory: ilvuStartBit, videoRangeStart: 0, audioRangeStart: 0},  // Cell A start
			{ilvuCategory: 0x4000, videoRangeStart: 2, audioRangeStart: 1},        // Cell A mid (interleaved, no start/end)
			{ilvuCategory: ilvuEndBit, videoRangeStart: 4, audioRangeStart: 2},    // Cell A end
			{ilvuCategory: ilvuStartBit, videoRangeStart: 6, audioRangeStart: 3},  // Cell B start
			{ilvuCategory: ilvuEndBit, videoRangeStart: 8, audioRangeStart: 4},    // Cell B end
			{ilvuCategory: ilvuStartBit, videoRangeStart: 10, audioRangeStart: 5}, // Cell A start
			{ilvuCategory: ilvuEndBit, videoRangeStart: 12, audioRangeStart: 6},   // Cell A end
		},
		videoRanges: make([]PESPayloadRange, 14),
		audioRanges: make([]PESPayloadRange, 7),
	}
	p.buildCellSegments()

	if p.CellSegmentCount() != 3 {
		t.Fatalf("CellSegmentCount() = %d, want 3", p.CellSegmentCount())
	}

	// Segment 0: VOBUs 0-2 (Cell A)
	if p.cellSegments[0].vobuStart != 0 || p.cellSegments[0].vobuEnd != 3 {
		t.Errorf("segment 0: vobus [%d, %d), want [0, 3)", p.cellSegments[0].vobuStart, p.cellSegments[0].vobuEnd)
	}
	// Segment 1: VOBUs 3-4 (Cell B)
	if p.cellSegments[1].vobuStart != 3 || p.cellSegments[1].vobuEnd != 5 {
		t.Errorf("segment 1: vobus [%d, %d), want [3, 5)", p.cellSegments[1].vobuStart, p.cellSegments[1].vobuEnd)
	}
	// Segment 2: VOBUs 5-6 (Cell A)
	if p.cellSegments[2].vobuStart != 5 || p.cellSegments[2].vobuEnd != 7 {
		t.Errorf("segment 2: vobus [%d, %d), want [5, 7)", p.cellSegments[2].vobuStart, p.cellSegments[2].vobuEnd)
	}
}

func TestBuildCellSegments_NonInterleavedBeforeAndAfter(t *testing.T) {
	// Non-interleaved VOBUs, then interleaved block, then non-interleaved
	p := &MPEGPSParser{
		vobus: []vobuInfo{
			{ilvuCategory: 0, videoRangeStart: 0, audioRangeStart: 0},                         // non-interleaved
			{ilvuCategory: 0, videoRangeStart: 1, audioRangeStart: 1},                         // non-interleaved
			{ilvuCategory: ilvuStartBit | ilvuEndBit, videoRangeStart: 2, audioRangeStart: 2}, // Cell A
			{ilvuCategory: ilvuStartBit | ilvuEndBit, videoRangeStart: 3, audioRangeStart: 3}, // Cell B
			{ilvuCategory: 0, videoRangeStart: 4, audioRangeStart: 4},                         // non-interleaved
		},
		videoRanges: make([]PESPayloadRange, 5),
		audioRanges: make([]PESPayloadRange, 5),
	}
	p.buildCellSegments()

	if p.CellSegmentCount() != 4 {
		t.Fatalf("CellSegmentCount() = %d, want 4", p.CellSegmentCount())
	}

	// Segment 0: non-interleaved VOBUs 0-1
	if p.cellSegments[0].vobuStart != 0 || p.cellSegments[0].vobuEnd != 2 {
		t.Errorf("segment 0: [%d, %d), want [0, 2)", p.cellSegments[0].vobuStart, p.cellSegments[0].vobuEnd)
	}
	// Segment 1: Cell A VOBU 2
	if p.cellSegments[1].vobuStart != 2 || p.cellSegments[1].vobuEnd != 3 {
		t.Errorf("segment 1: [%d, %d), want [2, 3)", p.cellSegments[1].vobuStart, p.cellSegments[1].vobuEnd)
	}
	// Segment 2: Cell B VOBU 3
	if p.cellSegments[2].vobuStart != 3 || p.cellSegments[2].vobuEnd != 4 {
		t.Errorf("segment 2: [%d, %d), want [3, 4)", p.cellSegments[2].vobuStart, p.cellSegments[2].vobuEnd)
	}
	// Segment 3: non-interleaved VOBU 4
	if p.cellSegments[3].vobuStart != 4 || p.cellSegments[3].vobuEnd != 5 {
		t.Errorf("segment 3: [%d, %d), want [4, 5)", p.cellSegments[3].vobuStart, p.cellSegments[3].vobuEnd)
	}
}

func TestRebaseRanges(t *testing.T) {
	ranges := []PESPayloadRange{
		{FileOffset: 1000, Size: 200, ESOffset: 5000},
		{FileOffset: 2000, Size: 300, ESOffset: 5200},
		{FileOffset: 3000, Size: 100, ESOffset: 5500},
	}

	rebased := rebaseRanges(ranges)
	if len(rebased) != 3 {
		t.Fatalf("rebaseRanges() returned %d ranges, want 3", len(rebased))
	}

	// ES offsets should be renumbered from 0
	if rebased[0].ESOffset != 0 {
		t.Errorf("rebased[0].ESOffset = %d, want 0", rebased[0].ESOffset)
	}
	if rebased[1].ESOffset != 200 {
		t.Errorf("rebased[1].ESOffset = %d, want 200", rebased[1].ESOffset)
	}
	if rebased[2].ESOffset != 500 {
		t.Errorf("rebased[2].ESOffset = %d, want 500", rebased[2].ESOffset)
	}

	// File offsets and sizes should be preserved
	if rebased[0].FileOffset != 1000 || rebased[0].Size != 200 {
		t.Errorf("rebased[0] = {%d, %d}, want {1000, 200}", rebased[0].FileOffset, rebased[0].Size)
	}
}

func TestRebaseRanges_Empty(t *testing.T) {
	if rebased := rebaseRanges(nil); rebased != nil {
		t.Errorf("rebaseRanges(nil) = %v, want nil", rebased)
	}
}

func TestCellSegmentVideoRanges(t *testing.T) {
	p := &MPEGPSParser{
		videoRanges: []PESPayloadRange{
			{FileOffset: 100, Size: 50, ESOffset: 0},
			{FileOffset: 200, Size: 50, ESOffset: 50},
			{FileOffset: 300, Size: 50, ESOffset: 100},
			{FileOffset: 400, Size: 50, ESOffset: 150},
		},
		vobus: []vobuInfo{
			{ilvuCategory: ilvuStartBit | ilvuEndBit, videoRangeStart: 0, audioRangeStart: 0},
			{ilvuCategory: ilvuStartBit | ilvuEndBit, videoRangeStart: 2, audioRangeStart: 0},
		},
		cellSegments: []cellSegment{
			{vobuStart: 0, vobuEnd: 1},
			{vobuStart: 1, vobuEnd: 2},
		},
	}

	// Segment 0: video ranges 0-1 (VOBU 0's ranges)
	seg0 := p.CellSegmentVideoRanges(0)
	if len(seg0) != 2 {
		t.Fatalf("segment 0: %d ranges, want 2", len(seg0))
	}
	if seg0[0].FileOffset != 100 || seg0[0].ESOffset != 0 {
		t.Errorf("seg0[0] = {FileOffset:%d, ESOffset:%d}, want {100, 0}", seg0[0].FileOffset, seg0[0].ESOffset)
	}
	if seg0[1].FileOffset != 200 || seg0[1].ESOffset != 50 {
		t.Errorf("seg0[1] = {FileOffset:%d, ESOffset:%d}, want {200, 50}", seg0[1].FileOffset, seg0[1].ESOffset)
	}

	// Segment 1: video ranges 2-3 (VOBU 1's ranges), rebased to 0
	seg1 := p.CellSegmentVideoRanges(1)
	if len(seg1) != 2 {
		t.Fatalf("segment 1: %d ranges, want 2", len(seg1))
	}
	if seg1[0].FileOffset != 300 || seg1[0].ESOffset != 0 {
		t.Errorf("seg1[0] = {FileOffset:%d, ESOffset:%d}, want {300, 0}", seg1[0].FileOffset, seg1[0].ESOffset)
	}
	if seg1[1].FileOffset != 400 || seg1[1].ESOffset != 50 {
		t.Errorf("seg1[1] = {FileOffset:%d, ESOffset:%d}, want {400, 50}", seg1[1].FileOffset, seg1[1].ESOffset)
	}
}

func TestCellSegmentVideoRanges_InvalidIndex(t *testing.T) {
	p := &MPEGPSParser{}
	if ranges := p.CellSegmentVideoRanges(-1); ranges != nil {
		t.Error("expected nil for negative index")
	}
	if ranges := p.CellSegmentVideoRanges(0); ranges != nil {
		t.Error("expected nil for index beyond segments")
	}
}

func TestParseWithProgress_DetectsNAVPacks(t *testing.T) {
	// Build a buffer with two NAV packs (non-interleaved) and video PES packets
	const bufSize = 8192
	data := make([]byte, bufSize)
	pos := 0

	// NAV pack 1 (no interleaving)
	pos += buildTestNAVPack(data, pos, 0)
	// Video PES 1
	pos += buildTestVideoPES(data, pos, 100)

	// NAV pack 2 (no interleaving)
	pos += buildTestNAVPack(data, pos, 0)
	// Video PES 2
	pos += buildTestVideoPES(data, pos, 100)

	p := NewMPEGPSParser(data[:pos])
	if err := p.Parse(); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Should detect 2 VOBUs
	if len(p.vobus) != 2 {
		t.Errorf("detected %d VOBUs, want 2", len(p.vobus))
	}

	// No interleaving → 0 segments
	if p.CellSegmentCount() != 0 {
		t.Errorf("CellSegmentCount() = %d, want 0 (no interleaving)", p.CellSegmentCount())
	}
}

func TestParseWithProgress_DetectsILVU(t *testing.T) {
	// Build a buffer with interleaved NAV packs
	const bufSize = 16384
	data := make([]byte, bufSize)
	pos := 0

	// Cell A VOBU (start+end)
	pos += buildTestNAVPack(data, pos, ilvuStartBit|ilvuEndBit)
	pos += buildTestVideoPES(data, pos, 100)

	// Cell B VOBU (start+end)
	pos += buildTestNAVPack(data, pos, ilvuStartBit|ilvuEndBit)
	pos += buildTestVideoPES(data, pos, 100)

	// Cell A VOBU (start+end)
	pos += buildTestNAVPack(data, pos, ilvuStartBit|ilvuEndBit)
	pos += buildTestVideoPES(data, pos, 100)

	p := NewMPEGPSParser(data[:pos])
	if err := p.Parse(); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(p.vobus) != 3 {
		t.Fatalf("detected %d VOBUs, want 3", len(p.vobus))
	}

	// Each VOBU should have ilvuCategory set
	for i, v := range p.vobus {
		if v.ilvuCategory != ilvuStartBit|ilvuEndBit {
			t.Errorf("vobu[%d].ilvuCategory = 0x%04X, want 0x%04X", i, v.ilvuCategory, ilvuStartBit|ilvuEndBit)
		}
	}

	// 3 interleaved VOBUs → 3 segments
	if p.CellSegmentCount() != 3 {
		t.Errorf("CellSegmentCount() = %d, want 3", p.CellSegmentCount())
	}
}

func TestCellSegmentAdapter_VideoES(t *testing.T) {
	// Set up a parser with 2 segments, each with 1 VOBU and 1 video range
	data := make([]byte, 2000)
	// Fill segment 0's video data at offset 100
	for i := 0; i < 200; i++ {
		data[100+i] = 0xAA
	}
	// Fill segment 1's video data at offset 500
	for i := 0; i < 200; i++ {
		data[500+i] = 0xBB
	}

	p := &MPEGPSParser{
		data: data,
		size: int64(len(data)),
		videoRanges: []PESPayloadRange{
			{FileOffset: 100, Size: 200, ESOffset: 0},
			{FileOffset: 500, Size: 200, ESOffset: 200},
		},
		vobus: []vobuInfo{
			{ilvuCategory: ilvuStartBit | ilvuEndBit, videoRangeStart: 0, audioRangeStart: 0},
			{ilvuCategory: ilvuStartBit | ilvuEndBit, videoRangeStart: 1, audioRangeStart: 0},
		},
		cellSegments: []cellSegment{
			{vobuStart: 0, vobuEnd: 1},
			{vobuStart: 1, vobuEnd: 2},
		},
		lpcmSubStreams: map[byte]bool{},
	}

	// Create adapter for segment 1
	adapter := newCellSegmentAdapter(p, 1)

	// Video ES size should be 200 (just segment 1's data)
	if esSize := adapter.TotalESSize(true); esSize != 200 {
		t.Errorf("TotalESSize(true) = %d, want 200", esSize)
	}

	// Audio ES size should be 0
	if esSize := adapter.TotalESSize(false); esSize != 0 {
		t.Errorf("TotalESSize(false) = %d, want 0", esSize)
	}

	// Read segment 1's data (should be 0xBB)
	esData, err := adapter.ReadESData(0, 10, true)
	if err != nil {
		t.Fatalf("ReadESData() error = %v", err)
	}
	for i, b := range esData {
		if b != 0xBB {
			t.Errorf("ReadESData byte %d = 0x%02X, want 0xBB", i, b)
			break
		}
	}

	// ReadESByteWithHint should work
	b, hint, ok := adapter.ReadESByteWithHint(0, true, -1)
	if !ok || b != 0xBB {
		t.Errorf("ReadESByteWithHint(0) = {0x%02X, %d, %v}, want {0xBB, _, true}", b, hint, ok)
	}

	// ESOffsetToFileOffset should map correctly
	fileOff, remaining := adapter.ESOffsetToFileOffset(10, true)
	if fileOff != 510 || remaining != 190 {
		t.Errorf("ESOffsetToFileOffset(10, true) = {%d, %d}, want {510, 190}", fileOff, remaining)
	}

	// ESOffsetToFileOffset for audio should return -1
	fileOff, _ = adapter.ESOffsetToFileOffset(0, false)
	if fileOff != -1 {
		t.Errorf("ESOffsetToFileOffset(0, false) = %d, want -1", fileOff)
	}
}

func TestCellSegmentAdapter_Parser(t *testing.T) {
	p := &MPEGPSParser{
		data:           make([]byte, 100),
		size:           100,
		lpcmSubStreams: map[byte]bool{},
		cellSegments:   []cellSegment{{vobuStart: 0, vobuEnd: 0}},
		vobus:          []vobuInfo{{videoRangeStart: 0, audioRangeStart: 0}},
	}

	adapter := newCellSegmentAdapter(p, 0)
	if adapter.Parser() != p {
		t.Error("Parser() should return the underlying parser")
	}
}

func TestDetectDVDCodecs_WithCellSegmentAdapter(t *testing.T) {
	// Create an index with a cellSegmentAdapter ESReader
	data := make([]byte, 1000)
	// AC3 audio sub-stream header
	data[100] = 0x80

	p := &MPEGPSParser{
		data: data,
		size: int64(len(data)),
		videoRanges: []PESPayloadRange{
			{FileOffset: 0, Size: 50, ESOffset: 0},
		},
		filteredVideoRanges: []PESPayloadRange{
			{FileOffset: 0, Size: 50, ESOffset: 0},
		},
		filterUserData:  true,
		audioSubStreams: []byte{0x80},
		filteredAudioBySubStream: map[byte][]PESPayloadRange{
			0x80: {{FileOffset: 104, Size: 96, ESOffset: 0}},
		},
		lpcmSubStreams: map[byte]bool{},
		cellSegments:   []cellSegment{{vobuStart: 0, vobuEnd: 1}},
		vobus:          []vobuInfo{{videoRangeStart: 0, audioRangeStart: 0}},
	}

	adapter := newCellSegmentAdapter(p, 0)

	index := &Index{
		SourceType: TypeDVD,
		ESReaders:  []ESReader{adapter},
	}

	codecs, err := DetectSourceCodecs(index)
	if err != nil {
		t.Fatalf("DetectSourceCodecs() error = %v", err)
	}

	// Should detect MPEG-2 video (from parser's video ranges)
	if !containsCodec(codecs.VideoCodecs, CodecMPEG2Video) {
		t.Error("expected MPEG-2 video codec")
	}

	// Should detect AC3 audio (from parser's audio sub-streams)
	if !containsCodec(codecs.AudioCodecs, CodecAC3Audio) {
		t.Error("expected AC3 audio codec")
	}
}

func TestIndexMPEGPSFile_InterleavedSegments(t *testing.T) {
	// Build a synthetic MPEG-PS file with 4 interleaved VOBUs (2 cells x 2 ILVUs).
	// Each VOBU has: NAV pack + video PES packet with identifiable payload.
	const bufSize = 32768
	data := make([]byte, bufSize)
	pos := 0

	// VOBU 0: Cell A, ILVU start+end
	pos += buildTestNAVPack(data, pos, ilvuStartBit|ilvuEndBit)
	pos += buildTestVideoPES(data, pos, 200)

	// VOBU 1: Cell B, ILVU start+end
	pos += buildTestNAVPack(data, pos, ilvuStartBit|ilvuEndBit)
	pos += buildTestVideoPES(data, pos, 200)

	// VOBU 2: Cell A, ILVU start+end
	pos += buildTestNAVPack(data, pos, ilvuStartBit|ilvuEndBit)
	pos += buildTestVideoPES(data, pos, 200)

	// VOBU 3: Cell B, ILVU start+end
	pos += buildTestNAVPack(data, pos, ilvuStartBit|ilvuEndBit)
	pos += buildTestVideoPES(data, pos, 200)

	// Write to a temp file (indexMPEGPSFile mmaps the file)
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.vob")
	if err := os.WriteFile(tmpFile, data[:pos], 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	// Create an indexer and call indexMPEGPSFile directly
	idx := &Indexer{
		windowSize: DefaultWindowSize,
		index:      NewIndex(tmpDir, TypeDVD, DefaultWindowSize),
	}
	idx.index.UsesESOffsets = true

	n, checksum, err := idx.indexMPEGPSFile(0, tmpFile, int64(pos), nil)
	if err != nil {
		t.Fatalf("indexMPEGPSFile() error = %v", err)
	}

	// Should produce 4 segments (one per VOBU)
	if n != 4 {
		t.Errorf("indexMPEGPSFile() returned %d entries, want 4", n)
	}

	// Checksum should be non-zero
	if checksum == 0 {
		t.Error("expected non-zero checksum")
	}

	// ESReaders count should match segment count
	if len(idx.index.ESReaders) != n {
		t.Errorf("len(ESReaders) = %d, want %d", len(idx.index.ESReaders), n)
	}

	// Each ESReader should be a cellSegmentAdapter
	for i, reader := range idx.index.ESReaders {
		if _, ok := reader.(*cellSegmentAdapter); !ok {
			t.Errorf("ESReaders[%d] is %T, want *cellSegmentAdapter", i, reader)
		}
	}

	// Verify locations reference valid FileIndex values (0..n-1)
	for hash, locs := range idx.index.HashToLocations {
		for _, loc := range locs {
			if int(loc.FileIndex) >= n {
				t.Errorf("hash 0x%X: location FileIndex=%d exceeds segment count %d", hash, loc.FileIndex, n)
			}
		}
	}

	// Clean up mmap files
	idx.index.Close()
}

func TestIndexMPEGPSFile_NonInterleaved(t *testing.T) {
	// Build a synthetic MPEG-PS file with non-interleaved VOBUs.
	// Should produce exactly 1 entry (existing behavior).
	const bufSize = 16384
	data := make([]byte, bufSize)
	pos := 0

	// Two non-interleaved VOBUs
	pos += buildTestNAVPack(data, pos, 0)
	pos += buildTestVideoPES(data, pos, 200)
	pos += buildTestNAVPack(data, pos, 0)
	pos += buildTestVideoPES(data, pos, 200)

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.vob")
	if err := os.WriteFile(tmpFile, data[:pos], 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	idx := &Indexer{
		windowSize: DefaultWindowSize,
		index:      NewIndex(tmpDir, TypeDVD, DefaultWindowSize),
	}
	idx.index.UsesESOffsets = true

	n, _, err := idx.indexMPEGPSFile(0, tmpFile, int64(pos), nil)
	if err != nil {
		t.Fatalf("indexMPEGPSFile() error = %v", err)
	}

	// Non-interleaved: exactly 1 entry
	if n != 1 {
		t.Errorf("indexMPEGPSFile() returned %d entries, want 1", n)
	}

	// ESReader should be *MPEGPSParser (not adapter)
	if len(idx.index.ESReaders) != 1 {
		t.Fatalf("len(ESReaders) = %d, want 1", len(idx.index.ESReaders))
	}
	if _, ok := idx.index.ESReaders[0].(*MPEGPSParser); !ok {
		t.Errorf("ESReaders[0] is %T, want *MPEGPSParser", idx.index.ESReaders[0])
	}

	// Files and ESReaders should be aligned
	if len(idx.index.ESReaders) != 1 {
		t.Error("ESReaders count should be 1 for non-interleaved")
	}

	idx.index.Close()
}
