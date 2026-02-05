package source

import (
	"testing"
)

func TestFindAudioSyncPoints_AC3(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected []int
	}{
		{
			name:     "AC3 sync at beginning",
			data:     []byte{0x0B, 0x77, 0x00, 0x00},
			expected: []int{0},
		},
		{
			name:     "AC3 sync in middle",
			data:     []byte{0xFF, 0x0B, 0x77, 0x00},
			expected: []int{1},
		},
		{
			name:     "multiple AC3 syncs",
			data:     []byte{0x0B, 0x77, 0x00, 0x0B, 0x77},
			expected: []int{0, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FindAudioSyncPoints(tt.data)
			if !intSliceEqual(result, tt.expected) {
				t.Errorf("FindAudioSyncPoints() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFindAudioSyncPoints_DTS(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected []int
	}{
		{
			name:     "DTS sync at beginning",
			data:     []byte{0x7F, 0xFE, 0x80, 0x01, 0x00},
			expected: []int{0},
		},
		{
			name:     "DTS sync in middle",
			data:     []byte{0xFF, 0x7F, 0xFE, 0x80, 0x01},
			expected: []int{1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FindAudioSyncPoints(tt.data)
			if !intSliceEqual(result, tt.expected) {
				t.Errorf("FindAudioSyncPoints() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFindAudioSyncPoints_TrueHD(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected []int
	}{
		{
			name:     "TrueHD sync at beginning",
			data:     []byte{0xF8, 0x72, 0x6F, 0xBA, 0x00},
			expected: []int{0},
		},
		{
			name:     "TrueHD sync in middle",
			data:     []byte{0x00, 0xF8, 0x72, 0x6F, 0xBA}, // Use 0x00 prefix to avoid MPEG sync
			expected: []int{1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FindAudioSyncPoints(tt.data)
			if !intSliceEqual(result, tt.expected) {
				t.Errorf("FindAudioSyncPoints() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFindAudioSyncPoints_MPEG(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected []int
	}{
		{
			name:     "MPEG audio sync FF F0",
			data:     []byte{0xFF, 0xF0, 0x00, 0x00},
			expected: []int{0},
		},
		{
			name:     "MPEG audio sync FF FB (MP3)",
			data:     []byte{0xFF, 0xFB, 0x90, 0x00},
			expected: []int{0},
		},
		{
			name:     "AAC ADTS sync FF F1",
			data:     []byte{0xFF, 0xF1, 0x00, 0x00},
			expected: []int{0},
		},
		{
			name:     "not MPEG sync (FF E0)",
			data:     []byte{0xFF, 0xE0, 0x00, 0x00},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FindAudioSyncPoints(tt.data)
			if !intSliceEqual(result, tt.expected) {
				t.Errorf("FindAudioSyncPoints() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFindAudioSyncPoints_Mixed(t *testing.T) {
	// Data with AC3, DTS, and MPEG syncs
	data := []byte{
		0x0B, 0x77, // AC3 at 0
		0x00, 0x00,
		0x7F, 0xFE, 0x80, 0x01, // DTS at 4
		0x00,
		0xFF, 0xFB, 0x90, // MPEG at 9 (byte 2 upper nibble 0x9 = valid bitrate)
	}

	result := FindAudioSyncPoints(data)
	expected := []int{0, 4, 9}

	if !intSliceEqual(result, expected) {
		t.Errorf("FindAudioSyncPoints() = %v, want %v", result, expected)
	}
}

func TestFindAudioSyncPoints_FFPaddingRejection(t *testing.T) {
	// A run of 0xFF bytes (MPEG-TS adaptation field padding) should NOT
	// produce sync points. Before the bitrate-index validation fix, every
	// consecutive byte pair in this run matched the MPEG audio pattern.
	data := make([]byte, 256)
	for i := range data {
		data[i] = 0xFF
	}

	result := FindAudioSyncPoints(data)
	if len(result) != 0 {
		t.Errorf("FindAudioSyncPoints() on 0xFF padding returned %d sync points, want 0", len(result))
	}
}

func TestFindAllSyncPoints(t *testing.T) {
	// Data with video start codes and audio syncs interleaved
	data := []byte{
		0x00, 0x00, 0x01, 0xB3, // Video at 0
		0x0B, 0x77, // AC3 at 4
		0x00, 0x00, 0x01, 0x00, // Video at 6
	}

	result := FindAllSyncPoints(data)
	expected := []int{0, 4, 6}

	if !intSliceEqual(result, expected) {
		t.Errorf("FindAllSyncPoints() = %v, want %v", result, expected)
	}
}

func TestFindAudioSyncPointsInRange(t *testing.T) {
	data := []byte{0x0B, 0x77, 0x00, 0x0B, 0x77}
	startOffset := 5000

	result := FindAudioSyncPointsInRange(data, startOffset)
	expected := []int{5000, 5003}

	if !intSliceEqual(result, expected) {
		t.Errorf("FindAudioSyncPointsInRange() = %v, want %v", result, expected)
	}
}

func BenchmarkFindAudioSyncPoints(b *testing.B) {
	// Create test data with some sync patterns scattered throughout
	data := make([]byte, 1024*1024) // 1MB
	// Add AC3 syncs every ~1000 bytes
	for i := 0; i < len(data)-2; i += 1000 {
		data[i] = 0x0B
		data[i+1] = 0x77
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FindAudioSyncPoints(data)
	}
}

func BenchmarkFindAllSyncPoints(b *testing.B) {
	// Create test data with mixed sync patterns
	data := make([]byte, 1024*1024) // 1MB
	// Add video start codes and audio syncs
	for i := 0; i < len(data)-4; i += 500 {
		if i%1000 == 0 {
			data[i] = 0x00
			data[i+1] = 0x00
			data[i+2] = 0x01
		} else {
			data[i] = 0x0B
			data[i+1] = 0x77
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FindAllSyncPoints(data)
	}
}
