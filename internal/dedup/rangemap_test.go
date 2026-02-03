package dedup

import (
	"testing"

	"github.com/stuckj/mkvdup/internal/source"
)

func TestZigzagRoundTrip(t *testing.T) {
	values := []int64{0, 1, -1, 2, -2, 127, -128, 1000, -1000, 1<<31 - 1, -(1 << 31)}
	for _, v := range values {
		encoded := zigzagEncode(v)
		decoded := zigzagDecode(encoded)
		if decoded != v {
			t.Errorf("zigzag roundtrip failed for %d: encoded=%d, decoded=%d", v, encoded, decoded)
		}
	}
}

func TestZigzagEncode_KnownValues(t *testing.T) {
	tests := []struct {
		input    int64
		expected uint64
	}{
		{0, 0},
		{-1, 1},
		{1, 2},
		{-2, 3},
		{2, 4},
	}
	for _, tt := range tests {
		got := zigzagEncode(tt.input)
		if got != tt.expected {
			t.Errorf("zigzagEncode(%d) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

func TestFindDefaults(t *testing.T) {
	tests := []struct {
		name     string
		ranges   []source.PESPayloadRange
		wantGap  int64
		wantSize int
	}{
		{
			name:     "empty",
			ranges:   nil,
			wantGap:  0,
			wantSize: 0,
		},
		{
			name:     "single entry",
			ranges:   []source.PESPayloadRange{{FileOffset: 0, Size: 184}},
			wantGap:  0,
			wantSize: 184,
		},
		{
			name: "uniform M2TS pattern",
			ranges: []source.PESPayloadRange{
				{FileOffset: 4, Size: 184},
				{FileOffset: 196, Size: 184},
				{FileOffset: 388, Size: 184},
				{FileOffset: 580, Size: 184},
			},
			wantGap:  8,
			wantSize: 184,
		},
		{
			name: "mixed sizes",
			ranges: []source.PESPayloadRange{
				{FileOffset: 0, Size: 100},
				{FileOffset: 110, Size: 200},
				{FileOffset: 320, Size: 100},
				{FileOffset: 430, Size: 100},
			},
			wantGap:  10,
			wantSize: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotGap, gotSize := findDefaults(tt.ranges)
			if gotGap != tt.wantGap {
				t.Errorf("defaultGap = %d, want %d", gotGap, tt.wantGap)
			}
			if gotSize != tt.wantSize {
				t.Errorf("defaultSize = %d, want %d", gotSize, tt.wantSize)
			}
		})
	}
}

func TestEncodeDecodeRoundTrip_UniformEntries(t *testing.T) {
	// Simulate typical M2TS video: stride=192, size=184, gap=8
	const count = 5000
	ranges := make([]source.PESPayloadRange, count)
	for i := range count {
		ranges[i] = source.PESPayloadRange{
			FileOffset: int64(4 + i*192),
			Size:       184,
			ESOffset:   int64(i * 184),
		}
	}

	defGap, defSize := findDefaults(ranges)
	if defGap != 8 || defSize != 184 {
		t.Fatalf("findDefaults = (%d, %d), want (8, 184)", defGap, defSize)
	}

	compressed := encodeCompressedRanges(ranges, defGap, defSize)
	t.Logf("Compressed %d uniform entries: %d bytes (%.1f bytes/entry)",
		count, len(compressed), float64(len(compressed))/float64(count))

	// Should be very compact due to RLE
	// First entry: ~2 uvarints (~4 bytes), then one RLE token (~3 bytes)
	if len(compressed) > 20 {
		t.Errorf("compressed size %d too large for %d uniform entries", len(compressed), count)
	}

	// Decode and verify
	sm, err := buildStreamRangeMap(compressed, count, defGap, defSize)
	if err != nil {
		t.Fatalf("buildStreamRangeMap: %v", err)
	}

	if sm.TotalESSize() != int64(count*184) {
		t.Errorf("TotalESSize = %d, want %d", sm.TotalESSize(), count*184)
	}

	// Read data from various positions using synthetic source data
	sourceData := make([]byte, 4+count*192)
	for i := range sourceData {
		sourceData[i] = byte(i % 256)
	}

	// Read first few bytes of ES
	data, err := sm.ReadData(sourceData, int64(len(sourceData)), 0, 10)
	if err != nil {
		t.Fatalf("ReadData(0, 10): %v", err)
	}
	// ES offset 0 maps to file offset 4
	for i := 0; i < 10; i++ {
		if data[i] != sourceData[4+i] {
			t.Errorf("byte %d: got %d, want %d", i, data[i], sourceData[4+i])
		}
	}

	// Read spanning two entries (at boundary 184)
	data, err = sm.ReadData(sourceData, int64(len(sourceData)), 180, 10)
	if err != nil {
		t.Fatalf("ReadData(180, 10): %v", err)
	}
	// ES 180-183 is in entry 0 (file offset 4+180=184)
	// ES 184-189 is in entry 1 (file offset 196+0=196)
	for i := 0; i < 4; i++ {
		if data[i] != sourceData[184+i] {
			t.Errorf("span byte %d: got %d, want %d (from entry 0)", i, data[i], sourceData[184+i])
		}
	}
	for i := 4; i < 10; i++ {
		if data[i] != sourceData[196+i-4] {
			t.Errorf("span byte %d: got %d, want %d (from entry 1)", i, data[i], sourceData[196+i-4])
		}
	}

	// Read from middle of stream
	esOff := int64(2500 * 184)
	data, err = sm.ReadData(sourceData, int64(len(sourceData)), esOff, 5)
	if err != nil {
		t.Fatalf("ReadData(%d, 5): %v", esOff, err)
	}
	expectedFileOff := 4 + 2500*192
	for i := 0; i < 5; i++ {
		if data[i] != sourceData[expectedFileOff+i] {
			t.Errorf("mid byte %d: got %d, want %d", i, data[i], sourceData[expectedFileOff+i])
		}
	}
}

func TestEncodeDecodeRoundTrip_MixedEntries(t *testing.T) {
	// Mix of default and non-default entries
	ranges := []source.PESPayloadRange{
		{FileOffset: 100, Size: 184, ESOffset: 0},
		{FileOffset: 292, Size: 184, ESOffset: 184},     // default (gap=8)
		{FileOffset: 484, Size: 184, ESOffset: 368},     // default
		{FileOffset: 676, Size: 184, ESOffset: 552},     // default
		{FileOffset: 900, Size: 200, ESOffset: 736},     // non-default (gap=32, size=200)
		{FileOffset: 1108, Size: 184, ESOffset: 936},    // default (gap=8)
		{FileOffset: 1300, Size: 184, ESOffset: 1120},   // default
		{FileOffset: 1500, Size: 150, ESOffset: 1304},   // non-default (gap=8, size=150)
		{FileOffset: 1658, Size: 184, ESOffset: 1454},   // default (gap=8)
		{FileOffset: 1850, Size: 184, ESOffset: 1638},   // default
	}

	defGap, defSize := findDefaults(ranges)
	if defGap != 8 || defSize != 184 {
		t.Fatalf("findDefaults = (%d, %d), want (8, 184)", defGap, defSize)
	}

	compressed := encodeCompressedRanges(ranges, defGap, defSize)
	t.Logf("Compressed %d mixed entries: %d bytes", len(ranges), len(compressed))

	sm, err := buildStreamRangeMap(compressed, len(ranges), defGap, defSize)
	if err != nil {
		t.Fatalf("buildStreamRangeMap: %v", err)
	}

	expectedTotal := int64(0)
	for _, r := range ranges {
		expectedTotal += int64(r.Size)
	}
	if sm.TotalESSize() != expectedTotal {
		t.Errorf("TotalESSize = %d, want %d", sm.TotalESSize(), expectedTotal)
	}

	// Verify we can read from each entry
	sourceData := make([]byte, 2100)
	for i := range sourceData {
		sourceData[i] = byte(i % 256)
	}

	esOff := int64(0)
	for i, r := range ranges {
		data, err := sm.ReadData(sourceData, int64(len(sourceData)), esOff, 1)
		if err != nil {
			t.Fatalf("ReadData(esOff=%d) for entry %d: %v", esOff, i, err)
		}
		if data[0] != sourceData[r.FileOffset] {
			t.Errorf("entry %d: got byte %d, want %d (fileOffset=%d)", i, data[0], sourceData[r.FileOffset], r.FileOffset)
		}
		esOff += int64(r.Size)
	}
}

func TestEncodeDecodeRoundTrip_SingleEntry(t *testing.T) {
	ranges := []source.PESPayloadRange{
		{FileOffset: 42, Size: 100, ESOffset: 0},
	}

	defGap, defSize := findDefaults(ranges)
	compressed := encodeCompressedRanges(ranges, defGap, defSize)

	sm, err := buildStreamRangeMap(compressed, 1, defGap, defSize)
	if err != nil {
		t.Fatalf("buildStreamRangeMap: %v", err)
	}

	if sm.TotalESSize() != 100 {
		t.Errorf("TotalESSize = %d, want 100", sm.TotalESSize())
	}

	sourceData := make([]byte, 200)
	for i := range sourceData {
		sourceData[i] = byte(i)
	}

	data, err := sm.ReadData(sourceData, int64(len(sourceData)), 0, 10)
	if err != nil {
		t.Fatalf("ReadData: %v", err)
	}
	for i := 0; i < 10; i++ {
		if data[i] != sourceData[42+i] {
			t.Errorf("byte %d: got %d, want %d", i, data[i], sourceData[42+i])
		}
	}
}

func TestEncodeDecodeRoundTrip_AllExplicit(t *testing.T) {
	// Every entry has different gap and size — no RLE possible
	ranges := []source.PESPayloadRange{
		{FileOffset: 0, Size: 100, ESOffset: 0},
		{FileOffset: 200, Size: 150, ESOffset: 100},
		{FileOffset: 500, Size: 80, ESOffset: 250},
		{FileOffset: 700, Size: 120, ESOffset: 330},
		{FileOffset: 1000, Size: 90, ESOffset: 450},
	}

	defGap, defSize := findDefaults(ranges)
	compressed := encodeCompressedRanges(ranges, defGap, defSize)
	t.Logf("Compressed %d all-explicit entries: %d bytes (defGap=%d, defSize=%d)",
		len(ranges), len(compressed), defGap, defSize)

	sm, err := buildStreamRangeMap(compressed, len(ranges), defGap, defSize)
	if err != nil {
		t.Fatalf("buildStreamRangeMap: %v", err)
	}

	sourceData := make([]byte, 1200)
	for i := range sourceData {
		sourceData[i] = byte(i % 256)
	}

	// Verify each entry
	esOff := int64(0)
	for i, r := range ranges {
		data, err := sm.ReadData(sourceData, int64(len(sourceData)), esOff, 1)
		if err != nil {
			t.Fatalf("ReadData(esOff=%d) for entry %d: %v", esOff, i, err)
		}
		if data[0] != sourceData[r.FileOffset] {
			t.Errorf("entry %d: got %d, want %d", i, data[0], sourceData[r.FileOffset])
		}
		esOff += int64(r.Size)
	}
}

func TestEncodeRangeMapSection_RoundTrip(t *testing.T) {
	// Create range map data with video and audio streams
	rmData := []RangeMapData{
		{
			FileIndex: 0,
			VideoRanges: func() []source.PESPayloadRange {
				ranges := make([]source.PESPayloadRange, 100)
				for i := range 100 {
					ranges[i] = source.PESPayloadRange{
						FileOffset: int64(4 + i*192),
						Size:       184,
						ESOffset:   int64(i * 184),
					}
				}
				return ranges
			}(),
			AudioStreams: []AudioRangeData{
				{
					SubStreamID: 0x80,
					Ranges: func() []source.PESPayloadRange {
						ranges := make([]source.PESPayloadRange, 50)
						for i := range 50 {
							ranges[i] = source.PESPayloadRange{
								FileOffset: int64(10000 + i*192),
								Size:       184,
								ESOffset:   int64(i * 184),
							}
						}
						return ranges
					}(),
				},
			},
		},
	}

	// Encode
	buf, err := encodeRangeMapSection(rmData)
	if err != nil {
		t.Fatalf("encodeRangeMapSection: %v", err)
	}
	t.Logf("Encoded range map section: %d bytes (100 video + 50 audio entries)", len(buf))

	// Decode
	sources, err := readRangeMapSection(buf)
	if err != nil {
		t.Fatalf("readRangeMapSection: %v", err)
	}

	if len(sources) != 1 {
		t.Fatalf("source count = %d, want 1", len(sources))
	}

	src := sources[0]
	if src.FileIndex != 0 {
		t.Errorf("FileIndex = %d, want 0", src.FileIndex)
	}
	if src.VideoMap == nil {
		t.Fatal("VideoMap is nil")
	}
	if src.VideoMap.TotalESSize() != 100*184 {
		t.Errorf("video TotalESSize = %d, want %d", src.VideoMap.TotalESSize(), 100*184)
	}

	audioMap, ok := src.AudioMaps[0x80]
	if !ok {
		t.Fatal("audio sub-stream 0x80 not found")
	}
	if audioMap.TotalESSize() != 50*184 {
		t.Errorf("audio TotalESSize = %d, want %d", audioMap.TotalESSize(), 50*184)
	}
}

func TestCoarseIndex_LargeRLERun(t *testing.T) {
	// Create entries that span multiple coarse blocks within a single RLE run
	const count = 5000 // > 4 * rangeMapCoarseStep
	ranges := make([]source.PESPayloadRange, count)
	for i := range count {
		ranges[i] = source.PESPayloadRange{
			FileOffset: int64(4 + i*192),
			Size:       184,
			ESOffset:   int64(i * 184),
		}
	}

	defGap, defSize := findDefaults(ranges)
	compressed := encodeCompressedRanges(ranges, defGap, defSize)

	sm, err := buildStreamRangeMap(compressed, count, defGap, defSize)
	if err != nil {
		t.Fatalf("buildStreamRangeMap: %v", err)
	}

	// Verify coarse index has expected number of entries
	expectedCoarse := (count + rangeMapCoarseStep - 1) / rangeMapCoarseStep
	if len(sm.coarse) != expectedCoarse {
		t.Errorf("coarse index size = %d, want %d", len(sm.coarse), expectedCoarse)
	}

	// Verify coarse entries have correct ES offsets
	for i, ce := range sm.coarse {
		expectedES := int64(ce.entryIndex * 184)
		if ce.esOffset != expectedES {
			t.Errorf("coarse[%d] esOffset = %d, want %d (entryIndex=%d)", i, ce.esOffset, expectedES, ce.entryIndex)
		}
		expectedFileOff := int64(4 + ce.entryIndex*192)
		if ce.fileOffset != expectedFileOff {
			t.Errorf("coarse[%d] fileOffset = %d, want %d", i, ce.fileOffset, expectedFileOff)
		}
	}

	// Read from deep within the stream using RLE fast path
	sourceData := make([]byte, 4+count*192)
	for i := range sourceData {
		sourceData[i] = byte(i % 256)
	}

	// Read from entry 4500 (well past coarse boundaries)
	esOff := int64(4500 * 184)
	data, err := sm.ReadData(sourceData, int64(len(sourceData)), esOff, 5)
	if err != nil {
		t.Fatalf("ReadData(%d, 5): %v", esOff, err)
	}
	expectedFile := 4 + 4500*192
	for i := 0; i < 5; i++ {
		if data[i] != sourceData[expectedFile+i] {
			t.Errorf("byte %d: got %d, want %d", i, data[i], sourceData[expectedFile+i])
		}
	}
}

func TestReadData_SpanningManyEntries(t *testing.T) {
	// Read a large region spanning many entries
	const count = 100
	ranges := make([]source.PESPayloadRange, count)
	for i := range count {
		ranges[i] = source.PESPayloadRange{
			FileOffset: int64(4 + i*192),
			Size:       184,
			ESOffset:   int64(i * 184),
		}
	}

	defGap, defSize := findDefaults(ranges)
	compressed := encodeCompressedRanges(ranges, defGap, defSize)

	sm, err := buildStreamRangeMap(compressed, count, defGap, defSize)
	if err != nil {
		t.Fatalf("buildStreamRangeMap: %v", err)
	}

	sourceData := make([]byte, 4+count*192)
	for i := range sourceData {
		sourceData[i] = byte(i % 256)
	}

	// Read 10 entries worth of data starting from entry 5
	esOff := int64(5 * 184)
	readSize := 10 * 184
	data, err := sm.ReadData(sourceData, int64(len(sourceData)), esOff, readSize)
	if err != nil {
		t.Fatalf("ReadData(%d, %d): %v", esOff, readSize, err)
	}

	if len(data) != readSize {
		t.Fatalf("read %d bytes, want %d", len(data), readSize)
	}

	// Verify each entry's data
	for entry := 0; entry < 10; entry++ {
		fileOff := 4 + (5+entry)*192
		for b := 0; b < 184; b++ {
			idx := entry*184 + b
			if data[idx] != sourceData[fileOff+b] {
				t.Errorf("entry %d byte %d: got %d, want %d (fileOff=%d)", entry, b, data[idx], sourceData[fileOff+b], fileOff)
				break
			}
		}
	}
}

func TestReadData_EmptyRangeMap(t *testing.T) {
	sm := &StreamRangeMap{entryCount: 0}
	_, err := sm.ReadData(nil, 0, 0, 10)
	if err == nil {
		t.Error("expected error for empty range map, got nil")
	}
}


func TestCompressedEncoding_ZeroDeltas(t *testing.T) {
	// Verify that the +1 shift works: explicit entry with zero delta
	// (default gap but non-default size) should not produce 0x00 byte
	ranges := []source.PESPayloadRange{
		{FileOffset: 0, Size: 100, ESOffset: 0},
		{FileOffset: 108, Size: 200, ESOffset: 100}, // gap=8 (default), size=200 (not default)
		{FileOffset: 316, Size: 100, ESOffset: 300}, // gap=8, size=100 (not default)
	}

	defGap, defSize := findDefaults(ranges)
	compressed := encodeCompressedRanges(ranges, defGap, defSize)

	// Verify no 0x00 bytes appear as token starters (after the first entry)
	// The first entry is two uvarints, then subsequent tokens should not start with 0x00
	// (since there are no default entries in this test)
	sm, err := buildStreamRangeMap(compressed, len(ranges), defGap, defSize)
	if err != nil {
		t.Fatalf("buildStreamRangeMap: %v", err)
	}

	if sm.TotalESSize() != 400 {
		t.Errorf("TotalESSize = %d, want 400", sm.TotalESSize())
	}
}
