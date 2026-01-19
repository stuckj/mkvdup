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
