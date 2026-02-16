package source

import (
	"testing"
)

func TestMPEGTSParser_ReadESData(t *testing.T) {
	data := buildBasicM2TSData()
	p := NewMPEGTSParser(data)
	if err := p.Parse(); err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	tests := []struct {
		name     string
		offset   int64
		size     int
		expected []byte
	}{
		{"start", 0, 4, []byte{0, 1, 2, 3}},
		{"cross first boundary", 173, 4, []byte{173, 174, 175, 176}},
		{"in continuation", 200, 3, []byte{200, 201, 202}},
		{"cross second boundary", 357, 4, []byte{101, 102, 103, 104}},
		{"near end", 530, 4, []byte{18, 19, 20, 21}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := p.ReadESData(tt.offset, tt.size, true)
			if err != nil {
				t.Fatalf("ReadESData(%d, %d) error: %v", tt.offset, tt.size, err)
			}
			for i, b := range got {
				if b != tt.expected[i] {
					t.Errorf("byte[%d] = 0x%02X, want 0x%02X", i, b, tt.expected[i])
				}
			}
		})
	}

	// Audio read
	audioData, err := p.ReadAudioSubStreamData(0, 0, 4)
	if err != nil {
		t.Fatalf("ReadAudioSubStreamData() error: %v", err)
	}
	for i := 0; i < 4; i++ {
		exp := byte((0x80 + i) & 0xFF)
		if audioData[i] != exp {
			t.Errorf("audio byte[%d] = 0x%02X, want 0x%02X", i, audioData[i], exp)
		}
	}

	// ReadESData with isVideo=false should error
	if _, err := p.ReadESData(0, 1, false); err == nil {
		t.Error("expected error for ReadESData with isVideo=false")
	}
}

func TestMPEGTSParser_RawRanges(t *testing.T) {
	data := buildBasicM2TSData()
	p := NewMPEGTSParser(data)
	if err := p.Parse(); err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	// Single range
	ranges, err := p.RawRangesForESRegion(10, 50, true)
	if err != nil {
		t.Fatalf("RawRangesForESRegion() error: %v", err)
	}
	if len(ranges) != 1 {
		t.Fatalf("got %d ranges, want 1", len(ranges))
	}
	if ranges[0].FileOffset != 411 || ranges[0].Size != 50 {
		t.Errorf("range = {%d, %d}, want {411, 50}", ranges[0].FileOffset, ranges[0].Size)
	}

	// Cross boundary
	ranges, err = p.RawRangesForESRegion(170, 20, true)
	if err != nil {
		t.Fatalf("RawRangesForESRegion() error: %v", err)
	}
	if len(ranges) != 2 {
		t.Fatalf("got %d ranges, want 2", len(ranges))
	}
	// 5 bytes in range 0 (ES 170-174), 15 bytes in range 1 (ES 175-189)
	if ranges[0].FileOffset != 571 || ranges[0].Size != 5 {
		t.Errorf("range[0] = {%d, %d}, want {571, 5}", ranges[0].FileOffset, ranges[0].Size)
	}
	if ranges[1].FileOffset != 584 || ranges[1].Size != 15 {
		t.Errorf("range[1] = {%d, %d}, want {584, 15}", ranges[1].FileOffset, ranges[1].Size)
	}

	// Audio sub-stream range
	audioRanges, err := p.RawRangesForAudioSubStream(0, 10, 50)
	if err != nil {
		t.Fatalf("RawRangesForAudioSubStream() error: %v", err)
	}
	if len(audioRanges) != 1 {
		t.Fatalf("got %d audio ranges, want 1", len(audioRanges))
	}
	if audioRanges[0].FileOffset != 795 || audioRanges[0].Size != 50 {
		t.Errorf("audio range = {%d, %d}, want {795, 50}", audioRanges[0].FileOffset, audioRanges[0].Size)
	}

	// Error: isVideo=false
	if _, err := p.RawRangesForESRegion(0, 1, false); err == nil {
		t.Error("expected error for RawRangesForESRegion with isVideo=false")
	}

	// Error: unknown audio sub-stream
	if _, err := p.RawRangesForAudioSubStream(99, 0, 1); err == nil {
		t.Error("expected error for unknown audio sub-stream")
	}
}

func TestMPEGTSParser_HintedReading(t *testing.T) {
	data := buildBasicM2TSData()
	p := NewMPEGTSParser(data)
	if err := p.Parse(); err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	// Video: hint valid for range 0
	b, hint, ok := p.ReadESByteWithHint(100, true, 0)
	if !ok {
		t.Fatal("ReadESByteWithHint failed")
	}
	if hint != 0 {
		t.Errorf("hint = %d, want 0", hint)
	}
	if b != 100 {
		t.Errorf("byte = %d, want 100", b)
	}

	// Video: cross boundary from range 0 to range 1
	b, hint, ok = p.ReadESByteWithHint(175, true, 0)
	if !ok {
		t.Fatal("ReadESByteWithHint at boundary failed")
	}
	if hint != 1 {
		t.Errorf("hint = %d, want 1", hint)
	}
	if b != 175 {
		t.Errorf("byte = %d, want 175", b)
	}

	// Video: fallback to binary search (hint=-1)
	b, hint, ok = p.ReadESByteWithHint(400, true, -1)
	if !ok {
		t.Fatal("ReadESByteWithHint with binary search failed")
	}
	if hint != 2 {
		t.Errorf("hint = %d, want 2", hint)
	}
	// 400 & 0xFF = 144
	if b != 144 {
		t.Errorf("byte = %d, want 144", b)
	}

	// Video: out of bounds
	_, _, ok = p.ReadESByteWithHint(600, true, 0)
	if ok {
		t.Error("expected failure for out-of-bounds ES offset")
	}

	// Video: isVideo=false returns failure
	_, _, ok = p.ReadESByteWithHint(0, false, 0)
	if ok {
		t.Error("expected failure for isVideo=false")
	}

	// Audio hint reading
	b, hint, ok = p.ReadAudioByteWithHint(0, 10, 0)
	if !ok {
		t.Fatal("ReadAudioByteWithHint failed")
	}
	if hint != 0 {
		t.Errorf("audio hint = %d, want 0", hint)
	}
	// Audio ES byte 10 = (0x80 + 10) & 0xFF = 0x8A
	if b != 0x8A {
		t.Errorf("audio byte = 0x%02X, want 0x8A", b)
	}
}

func TestMPEGTSParser_ESOffsetToFileOffset(t *testing.T) {
	data := buildBasicM2TSData()
	p := NewMPEGTSParser(data)
	if err := p.Parse(); err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	// In first range
	fo, rem := p.ESOffsetToFileOffset(10, true)
	if fo != 411 || rem != 165 {
		t.Errorf("ESOffsetToFileOffset(10) = (%d, %d), want (411, 165)", fo, rem)
	}

	// Start of second range
	fo, rem = p.ESOffsetToFileOffset(175, true)
	if fo != 584 || rem != 184 {
		t.Errorf("ESOffsetToFileOffset(175) = (%d, %d), want (584, 184)", fo, rem)
	}

	// isVideo=false
	fo, _ = p.ESOffsetToFileOffset(0, false)
	if fo != -1 {
		t.Errorf("ESOffsetToFileOffset(false) = %d, want -1", fo)
	}
}
