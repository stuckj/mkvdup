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
