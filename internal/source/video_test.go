package source

import (
	"testing"
)

func TestFindVideoStartCodes(t *testing.T) {
	// FindVideoStartCodes finds all 00 00 01 XX patterns (any video start code)
	tests := []struct {
		name     string
		data     []byte
		expected []int
	}{
		{
			name:     "empty data",
			data:     []byte{},
			expected: nil,
		},
		{
			name:     "too short",
			data:     []byte{0x00, 0x00, 0x00},
			expected: nil,
		},
		{
			name:     "single start code at beginning",
			data:     []byte{0x00, 0x00, 0x01, 0xB3, 0xFF},
			expected: []int{0},
		},
		{
			name:     "start code in middle",
			data:     []byte{0xFF, 0xFF, 0x00, 0x00, 0x01, 0x00, 0xFF},
			expected: []int{2},
		},
		{
			name:     "multiple start codes",
			data:     []byte{0x00, 0x00, 0x01, 0xB3, 0x00, 0x00, 0x01, 0x00},
			expected: []int{0, 4},
		},
		{
			name:     "slice header also indexed",
			data:     []byte{0x00, 0x00, 0x01, 0x01, 0xFF},
			expected: []int{0},
		},
		{
			name:     "H.264 NAL units also indexed",
			data:     []byte{0x00, 0x00, 0x01, 0x67, 0x00, 0x00, 0x01, 0x68},
			expected: []int{0, 4},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FindVideoStartCodes(tt.data)
			if !intSliceEqual(result, tt.expected) {
				t.Errorf("FindVideoStartCodes() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFindVideoStartCodesInRange(t *testing.T) {
	// Start codes at offset 0 and offset 5
	data := []byte{0x00, 0x00, 0x01, 0xB3, 0xFF, 0x00, 0x00, 0x01, 0x00, 0xFF}
	startOffset := 1000

	result := FindVideoStartCodesInRange(data, startOffset)
	expected := []int{1000, 1005}

	if !intSliceEqual(result, expected) {
		t.Errorf("FindVideoStartCodesInRange() = %v, want %v", result, expected)
	}
}

func intSliceEqual(a, b []int) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestFindVideoNALStarts(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected []int
	}{
		{
			name:     "empty data",
			data:     []byte{},
			expected: nil,
		},
		{
			name:     "too short",
			data:     []byte{0x00, 0x00, 0x01},
			expected: nil,
		},
		{
			name: "single NAL at beginning",
			// 00 00 01 B3 FF → NAL header at position 3
			data:     []byte{0x00, 0x00, 0x01, 0xB3, 0xFF},
			expected: []int{3},
		},
		{
			name: "NAL in middle",
			// FF FF 00 00 01 67 FF → NAL header at position 5
			data:     []byte{0xFF, 0xFF, 0x00, 0x00, 0x01, 0x67, 0xFF},
			expected: []int{5},
		},
		{
			name: "multiple NALs",
			// 00 00 01 B3 00 00 01 00 → NAL headers at 3 and 7
			data:     []byte{0x00, 0x00, 0x01, 0xB3, 0x00, 0x00, 0x01, 0x00, 0xFF},
			expected: []int{3, 7},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FindVideoNALStarts(tt.data)
			if !intSliceEqual(result, tt.expected) {
				t.Errorf("FindVideoNALStarts() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFindVideoNALStartsInRange(t *testing.T) {
	// 00 00 01 B3 FF 00 00 01 00 FF → NAL headers at local positions 3 and 8
	data := []byte{0x00, 0x00, 0x01, 0xB3, 0xFF, 0x00, 0x00, 0x01, 0x00, 0xFF}
	startOffset := 1000

	result := FindVideoNALStartsInRange(data, startOffset)
	expected := []int{1003, 1008}

	if !intSliceEqual(result, expected) {
		t.Errorf("FindVideoNALStartsInRange() = %v, want %v", result, expected)
	}
}

func TestFindAVCCNALStarts(t *testing.T) {
	tests := []struct {
		name          string
		data          []byte
		nalLengthSize int
		expected      []int
	}{
		{
			name:          "empty data",
			data:          []byte{},
			nalLengthSize: 4,
			expected:      nil,
		},
		{
			name:          "invalid length size",
			data:          []byte{0x00, 0x00, 0x00, 0x05, 0x67, 0x01, 0x02, 0x03, 0x04},
			nalLengthSize: 0,
			expected:      nil,
		},
		{
			name: "single NAL 4-byte length",
			// [00 00 00 05][67 01 02 03 04] → NAL header at position 4
			data:          []byte{0x00, 0x00, 0x00, 0x05, 0x67, 0x01, 0x02, 0x03, 0x04},
			nalLengthSize: 4,
			expected:      []int{4},
		},
		{
			name: "two NALs 4-byte length",
			// [00 00 00 02][67 01] [00 00 00 03][68 01 02] → NAL headers at 4 and 10
			data:          []byte{0x00, 0x00, 0x00, 0x02, 0x67, 0x01, 0x00, 0x00, 0x00, 0x03, 0x68, 0x01, 0x02},
			nalLengthSize: 4,
			expected:      []int{4, 10},
		},
		{
			name: "single NAL 2-byte length",
			// [00 05][67 01 02 03 04] → NAL header at position 2
			data:          []byte{0x00, 0x05, 0x67, 0x01, 0x02, 0x03, 0x04},
			nalLengthSize: 2,
			expected:      []int{2},
		},
		{
			name: "single NAL 1-byte length",
			// [05][67 01 02 03 04] → NAL header at position 1
			data:          []byte{0x05, 0x67, 0x01, 0x02, 0x03, 0x04},
			nalLengthSize: 1,
			expected:      []int{1},
		},
		{
			name: "zero length stops parsing",
			// [00 00 00 00] → zero length, should stop
			data:          []byte{0x00, 0x00, 0x00, 0x00, 0x67, 0x01, 0x02, 0x03},
			nalLengthSize: 4,
			expected:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FindAVCCNALStarts(tt.data, tt.nalLengthSize)
			if !intSliceEqual(result, tt.expected) {
				t.Errorf("FindAVCCNALStarts() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func BenchmarkFindVideoStartCodes(b *testing.B) {
	// Create test data with start codes scattered throughout
	data := make([]byte, 1024*1024) // 1MB
	// Add start codes (00 00 01 00) every ~1000 bytes
	for i := 0; i < len(data)-4; i += 1000 {
		data[i] = 0x00
		data[i+1] = 0x00
		data[i+2] = 0x01
		data[i+3] = 0x00
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FindVideoStartCodes(data)
	}
}
