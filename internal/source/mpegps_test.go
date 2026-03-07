package source

import (
	"testing"
)

func TestRawRangesForESRegion_SingleRange(t *testing.T) {
	// Create parser with mock filtered video ranges
	p := &MPEGPSParser{
		filterUserData: true,
		filteredVideoRanges: []PESPayloadRange{
			{FileOffset: 1000, Size: 500, ESOffset: 0},
			{FileOffset: 2000, Size: 500, ESOffset: 500},
			{FileOffset: 3000, Size: 500, ESOffset: 1000},
		},
	}

	// Request a region that fits entirely in the first range
	ranges, err := p.RawRangesForESRegion(100, 200, true)
	if err != nil {
		t.Fatalf("RawRangesForESRegion() error = %v", err)
	}

	if len(ranges) != 1 {
		t.Fatalf("expected 1 range, got %d", len(ranges))
	}

	// ES offset 100 in first range (ESOffset=0) should map to FileOffset 1000 + 100 = 1100
	if ranges[0].FileOffset != 1100 {
		t.Errorf("FileOffset = %d, want 1100", ranges[0].FileOffset)
	}
	if ranges[0].Size != 200 {
		t.Errorf("Size = %d, want 200", ranges[0].Size)
	}
}

func TestRawRangesForESRegion_MultiRange(t *testing.T) {
	// Create parser with mock filtered video ranges
	p := &MPEGPSParser{
		filterUserData: true,
		filteredVideoRanges: []PESPayloadRange{
			{FileOffset: 1000, Size: 500, ESOffset: 0},
			{FileOffset: 2000, Size: 500, ESOffset: 500},
			{FileOffset: 3000, Size: 500, ESOffset: 1000},
		},
	}

	// Request a region that spans from first range into second
	// ES offset 400 to 400+300=700, which spans [0,500) and [500,1000)
	ranges, err := p.RawRangesForESRegion(400, 300, true)
	if err != nil {
		t.Fatalf("RawRangesForESRegion() error = %v", err)
	}

	if len(ranges) != 2 {
		t.Fatalf("expected 2 ranges, got %d", len(ranges))
	}

	// First chunk: ES 400-500 (100 bytes) at FileOffset 1400
	if ranges[0].FileOffset != 1400 {
		t.Errorf("ranges[0].FileOffset = %d, want 1400", ranges[0].FileOffset)
	}
	if ranges[0].Size != 100 {
		t.Errorf("ranges[0].Size = %d, want 100", ranges[0].Size)
	}

	// Second chunk: ES 500-700 (200 bytes) at FileOffset 2000
	if ranges[1].FileOffset != 2000 {
		t.Errorf("ranges[1].FileOffset = %d, want 2000", ranges[1].FileOffset)
	}
	if ranges[1].Size != 200 {
		t.Errorf("ranges[1].Size = %d, want 200", ranges[1].Size)
	}
}

func TestRawRangesForESRegion_SpansThreeRanges(t *testing.T) {
	p := &MPEGPSParser{
		filterUserData: true,
		filteredVideoRanges: []PESPayloadRange{
			{FileOffset: 1000, Size: 100, ESOffset: 0},
			{FileOffset: 2000, Size: 100, ESOffset: 100},
			{FileOffset: 3000, Size: 100, ESOffset: 200},
		},
	}

	// Request entire ES (0-300)
	ranges, err := p.RawRangesForESRegion(0, 300, true)
	if err != nil {
		t.Fatalf("RawRangesForESRegion() error = %v", err)
	}

	if len(ranges) != 3 {
		t.Fatalf("expected 3 ranges, got %d", len(ranges))
	}

	// Verify each range
	expected := []RawRange{
		{FileOffset: 1000, Size: 100},
		{FileOffset: 2000, Size: 100},
		{FileOffset: 3000, Size: 100},
	}
	for i, exp := range expected {
		if ranges[i].FileOffset != exp.FileOffset || ranges[i].Size != exp.Size {
			t.Errorf("ranges[%d] = {%d, %d}, want {%d, %d}",
				i, ranges[i].FileOffset, ranges[i].Size, exp.FileOffset, exp.Size)
		}
	}
}

func TestRawRangesForESRegion_ESOffsetNotFound(t *testing.T) {
	p := &MPEGPSParser{
		filterUserData: true,
		filteredVideoRanges: []PESPayloadRange{
			{FileOffset: 1000, Size: 500, ESOffset: 0},
		},
	}

	// Request beyond available ES
	_, err := p.RawRangesForESRegion(1000, 100, true)
	if err == nil {
		t.Error("expected error for ES offset not found")
	}
}

func TestRawRangesForESRegion_AudioError(t *testing.T) {
	p := &MPEGPSParser{}

	// Calling with isVideo=false should error
	_, err := p.RawRangesForESRegion(0, 100, false)
	if err == nil {
		t.Error("expected error for audio stream")
	}
}

func TestRawRangesForAudioSubStream_SingleRange(t *testing.T) {
	p := &MPEGPSParser{
		filteredAudioBySubStream: map[byte][]PESPayloadRange{
			0x80: {
				{FileOffset: 5000, Size: 1000, ESOffset: 0},
				{FileOffset: 7000, Size: 1000, ESOffset: 1000},
			},
		},
	}

	// Request region in first range
	ranges, err := p.RawRangesForAudioSubStream(0x80, 100, 500)
	if err != nil {
		t.Fatalf("RawRangesForAudioSubStream() error = %v", err)
	}

	if len(ranges) != 1 {
		t.Fatalf("expected 1 range, got %d", len(ranges))
	}

	if ranges[0].FileOffset != 5100 {
		t.Errorf("FileOffset = %d, want 5100", ranges[0].FileOffset)
	}
	if ranges[0].Size != 500 {
		t.Errorf("Size = %d, want 500", ranges[0].Size)
	}
}

func TestRawRangesForAudioSubStream_MultiRange(t *testing.T) {
	p := &MPEGPSParser{
		filteredAudioBySubStream: map[byte][]PESPayloadRange{
			0x80: {
				{FileOffset: 5000, Size: 1000, ESOffset: 0},
				{FileOffset: 7000, Size: 1000, ESOffset: 1000},
			},
		},
	}

	// Request region spanning both ranges
	ranges, err := p.RawRangesForAudioSubStream(0x80, 800, 500)
	if err != nil {
		t.Fatalf("RawRangesForAudioSubStream() error = %v", err)
	}

	if len(ranges) != 2 {
		t.Fatalf("expected 2 ranges, got %d", len(ranges))
	}

	// First chunk: ES 800-1000 (200 bytes) at FileOffset 5800
	if ranges[0].FileOffset != 5800 {
		t.Errorf("ranges[0].FileOffset = %d, want 5800", ranges[0].FileOffset)
	}
	if ranges[0].Size != 200 {
		t.Errorf("ranges[0].Size = %d, want 200", ranges[0].Size)
	}

	// Second chunk: ES 1000-1300 (300 bytes) at FileOffset 7000
	if ranges[1].FileOffset != 7000 {
		t.Errorf("ranges[1].FileOffset = %d, want 7000", ranges[1].FileOffset)
	}
	if ranges[1].Size != 300 {
		t.Errorf("ranges[1].Size = %d, want 300", ranges[1].Size)
	}
}

func TestRawRangesForAudioSubStream_SubStreamNotFound(t *testing.T) {
	p := &MPEGPSParser{
		filteredAudioBySubStream: map[byte][]PESPayloadRange{
			0x80: {{FileOffset: 5000, Size: 1000, ESOffset: 0}},
		},
	}

	// Request non-existent sub-stream
	_, err := p.RawRangesForAudioSubStream(0x81, 0, 100)
	if err == nil {
		t.Error("expected error for sub-stream not found")
	}
}

func TestRawRangesForESRegion_EmptyRanges(t *testing.T) {
	p := &MPEGPSParser{
		filterUserData:      true,
		filteredVideoRanges: []PESPayloadRange{},
	}

	_, err := p.RawRangesForESRegion(0, 100, true)
	if err == nil {
		t.Error("expected error for empty ranges")
	}
}

func TestReadESByteWithHint_VideoHintValid(t *testing.T) {
	// Create mock data
	data := make([]byte, 5000)
	for i := range data {
		data[i] = byte(i % 256)
	}

	p := &MPEGPSParser{
		data:           data,
		size:           int64(len(data)),
		filterUserData: true,
		filteredVideoRanges: []PESPayloadRange{
			{FileOffset: 1000, Size: 500, ESOffset: 0},
			{FileOffset: 2000, Size: 500, ESOffset: 500},
			{FileOffset: 3000, Size: 500, ESOffset: 1000},
		},
	}

	// Read with correct hint (range 0)
	b, newHint, ok := p.ReadESByteWithHint(100, true, 0)
	if !ok {
		t.Fatal("ReadESByteWithHint failed")
	}
	if newHint != 0 {
		t.Errorf("expected hint 0, got %d", newHint)
	}
	// ES offset 100 maps to file offset 1100
	expected := data[1100]
	if b != expected {
		t.Errorf("got byte %d, want %d", b, expected)
	}
}

func TestReadESByteWithHint_VideoHintCrossesBoundary(t *testing.T) {
	data := make([]byte, 5000)
	for i := range data {
		data[i] = byte(i % 256)
	}

	p := &MPEGPSParser{
		data:           data,
		size:           int64(len(data)),
		filterUserData: true,
		filteredVideoRanges: []PESPayloadRange{
			{FileOffset: 1000, Size: 500, ESOffset: 0},
			{FileOffset: 2000, Size: 500, ESOffset: 500},
			{FileOffset: 3000, Size: 500, ESOffset: 1000},
		},
	}

	// Read at boundary with old hint - should find adjacent range
	b, newHint, ok := p.ReadESByteWithHint(500, true, 0)
	if !ok {
		t.Fatal("ReadESByteWithHint failed")
	}
	if newHint != 1 {
		t.Errorf("expected hint 1 (adjacent range), got %d", newHint)
	}
	// ES offset 500 maps to file offset 2000 (start of second range)
	expected := data[2000]
	if b != expected {
		t.Errorf("got byte %d, want %d", b, expected)
	}
}

func TestReadESByteWithHint_FallbackToBinarySearch(t *testing.T) {
	data := make([]byte, 5000)
	for i := range data {
		data[i] = byte(i % 256)
	}

	p := &MPEGPSParser{
		data:           data,
		size:           int64(len(data)),
		filterUserData: true,
		filteredVideoRanges: []PESPayloadRange{
			{FileOffset: 1000, Size: 500, ESOffset: 0},
			{FileOffset: 2000, Size: 500, ESOffset: 500},
			{FileOffset: 3000, Size: 500, ESOffset: 1000},
		},
	}

	// Read with invalid hint (-1) - should fall back to binary search
	b, newHint, ok := p.ReadESByteWithHint(1200, true, -1)
	if !ok {
		t.Fatal("ReadESByteWithHint failed")
	}
	if newHint != 2 {
		t.Errorf("expected hint 2 (from binary search), got %d", newHint)
	}
	// ES offset 1200 is in range 2: 1000 + (1200-1000) = FileOffset 3200
	expected := data[3200]
	if b != expected {
		t.Errorf("got byte %d, want %d", b, expected)
	}
}

func TestReadAudioByteWithHint_Valid(t *testing.T) {
	data := make([]byte, 10000)
	for i := range data {
		data[i] = byte(i % 256)
	}

	p := &MPEGPSParser{
		data: data,
		size: int64(len(data)),
		filteredAudioBySubStream: map[byte][]PESPayloadRange{
			0x80: {
				{FileOffset: 5000, Size: 1000, ESOffset: 0},
				{FileOffset: 7000, Size: 1000, ESOffset: 1000},
			},
		},
	}

	// Read with correct hint
	b, newHint, ok := p.ReadAudioByteWithHint(0x80, 500, 0)
	if !ok {
		t.Fatal("ReadAudioByteWithHint failed")
	}
	if newHint != 0 {
		t.Errorf("expected hint 0, got %d", newHint)
	}
	// ES offset 500 maps to file offset 5500
	expected := data[5500]
	if b != expected {
		t.Errorf("got byte %d, want %d", b, expected)
	}
}

func TestBuildFilteredAudioRanges_MPEG1Audio(t *testing.T) {
	// Simulate MPEG-1 audio PES payload: raw MP2 frame data starting with FF Fx sync
	// No sub-stream header — payload IS the audio data
	mp2Frame := make([]byte, 200)
	mp2Frame[0] = 0xFF
	mp2Frame[1] = 0xFC // MPEG1 Layer 2 sync

	// Build a minimal data buffer with the MP2 payload at a known offset
	data := make([]byte, 1000)
	copy(data[100:300], mp2Frame)
	copy(data[400:600], mp2Frame)

	p := &MPEGPSParser{
		data: data,
		size: int64(len(data)),
		audioRanges: []PESPayloadRange{
			{FileOffset: 100, Size: 200, ESOffset: 0},
			{FileOffset: 400, Size: 200, ESOffset: 200},
		},
		audioRangeStreamIDs: []byte{0xC0, 0xC0},
	}

	if err := p.buildFilteredAudioRanges(); err != nil {
		t.Fatalf("buildFilteredAudioRanges() error = %v", err)
	}

	// Should have one sub-stream with ID 0xC0
	if len(p.audioSubStreams) != 1 {
		t.Fatalf("audioSubStreams = %v, want [0xC0]", p.audioSubStreams)
	}
	if p.audioSubStreams[0] != 0xC0 {
		t.Errorf("audioSubStreams[0] = 0x%02X, want 0xC0", p.audioSubStreams[0])
	}

	// Filtered ranges should have the full payload (no header stripping)
	ranges := p.FilteredAudioRanges(0xC0)
	if len(ranges) != 2 {
		t.Fatalf("filtered ranges count = %d, want 2", len(ranges))
	}

	// First range: full payload, no header stripped
	if ranges[0].FileOffset != 100 {
		t.Errorf("ranges[0].FileOffset = %d, want 100", ranges[0].FileOffset)
	}
	if ranges[0].Size != 200 {
		t.Errorf("ranges[0].Size = %d, want 200", ranges[0].Size)
	}
	if ranges[0].ESOffset != 0 {
		t.Errorf("ranges[0].ESOffset = %d, want 0", ranges[0].ESOffset)
	}

	// Second range: ES offset should be contiguous
	if ranges[1].FileOffset != 400 {
		t.Errorf("ranges[1].FileOffset = %d, want 400", ranges[1].FileOffset)
	}
	if ranges[1].ESOffset != 200 {
		t.Errorf("ranges[1].ESOffset = %d, want 200", ranges[1].ESOffset)
	}

	// Total ES size should be 400 (200 + 200, no header bytes stripped)
	esSize := p.AudioSubStreamESSize(0xC0)
	if esSize != 400 {
		t.Errorf("AudioSubStreamESSize(0xC0) = %d, want 400", esSize)
	}
}

func TestBuildFilteredAudioRanges_MixedPS1AndMPEG1(t *testing.T) {
	// Simulate a DVD with both AC3 (Private Stream 1) and MP2 (MPEG-1 audio)
	data := make([]byte, 2000)

	// AC3 packet in Private Stream 1: sub-stream header at payload start
	data[100] = 0x80 // sub-stream ID (AC3)
	data[101] = 0x01 // frame count
	data[102] = 0x00 // AU pointer high
	data[103] = 0x01 // AU pointer low
	// AC3 data follows at offset 104

	// MP2 packet: raw audio data, no sub-stream header
	data[500] = 0xFF
	data[501] = 0xFC

	p := &MPEGPSParser{
		data: data,
		size: int64(len(data)),
		audioRanges: []PESPayloadRange{
			{FileOffset: 100, Size: 200, ESOffset: 0},   // PS1 with AC3
			{FileOffset: 500, Size: 200, ESOffset: 200}, // MPEG-1 audio
		},
		audioRangeStreamIDs: []byte{0xBD, 0xC0},
	}

	if err := p.buildFilteredAudioRanges(); err != nil {
		t.Fatalf("buildFilteredAudioRanges() error = %v", err)
	}

	// Should have two sub-streams: 0x80 (AC3) and 0xC0 (MP2)
	if len(p.audioSubStreams) != 2 {
		t.Fatalf("audioSubStreams = %v, want [0x80, 0xC0]", p.audioSubStreams)
	}

	// AC3 ranges should have 4-byte header stripped
	ac3Ranges := p.FilteredAudioRanges(0x80)
	if len(ac3Ranges) != 1 {
		t.Fatalf("AC3 ranges count = %d, want 1", len(ac3Ranges))
	}
	if ac3Ranges[0].FileOffset != 104 {
		t.Errorf("AC3 range FileOffset = %d, want 104 (after 4-byte header)", ac3Ranges[0].FileOffset)
	}
	if ac3Ranges[0].Size != 196 {
		t.Errorf("AC3 range Size = %d, want 196", ac3Ranges[0].Size)
	}

	// MP2 ranges should have NO header stripped
	mp2Ranges := p.FilteredAudioRanges(0xC0)
	if len(mp2Ranges) != 1 {
		t.Fatalf("MP2 ranges count = %d, want 1", len(mp2Ranges))
	}
	if mp2Ranges[0].FileOffset != 500 {
		t.Errorf("MP2 range FileOffset = %d, want 500 (no header to strip)", mp2Ranges[0].FileOffset)
	}
	if mp2Ranges[0].Size != 200 {
		t.Errorf("MP2 range Size = %d, want 200", mp2Ranges[0].Size)
	}
}

func TestBuildFilteredAudioRanges_MultipleMPEG1Streams(t *testing.T) {
	// Two different MPEG-1 audio streams (0xC0 and 0xC1)
	data := make([]byte, 1000)

	p := &MPEGPSParser{
		data: data,
		size: int64(len(data)),
		audioRanges: []PESPayloadRange{
			{FileOffset: 100, Size: 100, ESOffset: 0},
			{FileOffset: 300, Size: 100, ESOffset: 100},
			{FileOffset: 500, Size: 100, ESOffset: 200},
		},
		audioRangeStreamIDs: []byte{0xC0, 0xC1, 0xC0},
	}

	if err := p.buildFilteredAudioRanges(); err != nil {
		t.Fatalf("buildFilteredAudioRanges() error = %v", err)
	}

	// Should have two sub-streams
	if len(p.audioSubStreams) != 2 {
		t.Fatalf("audioSubStreams = %v, want [0xC0, 0xC1]", p.audioSubStreams)
	}

	// Stream 0xC0 should have two ranges with correct ES offsets
	c0Ranges := p.FilteredAudioRanges(0xC0)
	if len(c0Ranges) != 2 {
		t.Fatalf("0xC0 ranges count = %d, want 2", len(c0Ranges))
	}
	if c0Ranges[0].ESOffset != 0 {
		t.Errorf("0xC0 ranges[0].ESOffset = %d, want 0", c0Ranges[0].ESOffset)
	}
	if c0Ranges[1].ESOffset != 100 {
		t.Errorf("0xC0 ranges[1].ESOffset = %d, want 100", c0Ranges[1].ESOffset)
	}

	// Stream 0xC1 should have one range with ESOffset 0
	c1Ranges := p.FilteredAudioRanges(0xC1)
	if len(c1Ranges) != 1 {
		t.Fatalf("0xC1 ranges count = %d, want 1", len(c1Ranges))
	}
	if c1Ranges[0].ESOffset != 0 {
		t.Errorf("0xC1 ranges[0].ESOffset = %d, want 0", c1Ranges[0].ESOffset)
	}
}

func TestParseWithProgress_MPEG1AudioStreamIDs(t *testing.T) {
	// Build a minimal MPEG-PS buffer with an MPEG-1 audio PES packet
	// PES header: 00 00 01 C0 <length_hi> <length_lo> <flags...> <payload>
	data := make([]byte, 300)

	// Pack header at offset 0: 00 00 01 BA + MPEG-2 format
	data[0] = 0x00
	data[1] = 0x00
	data[2] = 0x01
	data[3] = 0xBA
	data[4] = 0x44 // MPEG-2 marker (01xx xxxx)
	// Fill enough for 14-byte pack header
	data[13] = 0x00 // 0 stuffing bytes

	// MPEG-1 audio PES packet at offset 14: 00 00 01 C0
	pesStart := 14
	data[pesStart] = 0x00
	data[pesStart+1] = 0x00
	data[pesStart+2] = 0x01
	data[pesStart+3] = 0xC0 // MPEG-1 audio stream
	// PES length: payload = 50 bytes, MPEG-2 PES header = 3 bytes, total after length = 53
	pesPayloadSize := 50
	pesLength := pesPayloadSize + 3 // flags(2) + header_data_len(1) + header_data(0)
	data[pesStart+4] = byte(pesLength >> 8)
	data[pesStart+5] = byte(pesLength)
	// MPEG-2 PES flags: 10 000000 = 0x80, 00 000000 = 0x00
	data[pesStart+6] = 0x80
	data[pesStart+7] = 0x00
	data[pesStart+8] = 0x00 // header_data_length = 0

	// Payload starts at pesStart+9
	payloadStart := pesStart + 9
	data[payloadStart] = 0xFF   // MP2 sync byte 1
	data[payloadStart+1] = 0xFC // MP2 sync byte 2

	p := NewMPEGPSParser(data)
	if err := p.Parse(); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Should have recorded MPEG-1 audio sub-stream
	subs := p.AudioSubStreams()
	if len(subs) != 1 {
		t.Fatalf("AudioSubStreams() = %v, want [0xC0]", subs)
	}
	if subs[0] != 0xC0 {
		t.Errorf("AudioSubStreams()[0] = 0x%02X, want 0xC0", subs[0])
	}

	// Filtered ranges should exist and cover the payload
	ranges := p.FilteredAudioRanges(0xC0)
	if len(ranges) != 1 {
		t.Fatalf("FilteredAudioRanges(0xC0) count = %d, want 1", len(ranges))
	}
	if ranges[0].FileOffset != int64(payloadStart) {
		t.Errorf("range FileOffset = %d, want %d", ranges[0].FileOffset, payloadStart)
	}
	if ranges[0].Size != pesPayloadSize {
		t.Errorf("range Size = %d, want %d", ranges[0].Size, pesPayloadSize)
	}

	// ES size should equal payload size (no header stripping)
	esSize := p.AudioSubStreamESSize(0xC0)
	if esSize != int64(pesPayloadSize) {
		t.Errorf("AudioSubStreamESSize(0xC0) = %d, want %d", esSize, pesPayloadSize)
	}

	// Not an LPCM sub-stream
	if p.IsLPCMSubStream(0xC0) {
		t.Error("MPEG-1 audio should not be LPCM")
	}
}

func TestReadESByteWithHint_OutOfBounds(t *testing.T) {
	data := make([]byte, 5000)
	p := &MPEGPSParser{
		data:           data,
		size:           int64(len(data)),
		filterUserData: true,
		filteredVideoRanges: []PESPayloadRange{
			{FileOffset: 1000, Size: 500, ESOffset: 0},
		},
	}

	// Read beyond the range
	_, _, ok := p.ReadESByteWithHint(1000, true, 0)
	if ok {
		t.Error("expected failure for out of bounds ES offset")
	}
}
