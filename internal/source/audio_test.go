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

func TestAC3FrameSize(t *testing.T) {
	tests := []struct {
		name       string
		fscod      byte
		frmsizecod byte
		want       int
	}{
		// 48 kHz (fscod=0)
		{"48kHz/32kbps/even", 0, 0, 128},
		{"48kHz/32kbps/odd", 0, 1, 128},
		{"48kHz/64kbps", 0, 4, 192},
		{"48kHz/128kbps", 0, 8, 256},
		{"48kHz/192kbps", 0, 12, 384},
		{"48kHz/384kbps", 0, 20, 768},
		{"48kHz/448kbps", 0, 22, 896},
		{"48kHz/640kbps", 0, 36, 2560},

		// 44.1 kHz (fscod=1)
		{"44.1kHz/32kbps/even", 1, 0, 138},
		{"44.1kHz/32kbps/odd", 1, 1, 140},
		{"44.1kHz/384kbps", 1, 20, 834},
		{"44.1kHz/640kbps", 1, 36, 2786},

		// 32 kHz (fscod=2)
		{"32kHz/32kbps", 2, 0, 192},
		{"32kHz/384kbps", 2, 20, 1152},
		{"32kHz/640kbps", 2, 36, 3840},

		// Invalid inputs
		{"invalid fscod=3", 3, 0, 0},
		{"invalid frmsizecod=38", 0, 38, 0},
		{"invalid frmsizecod=255", 0, 255, 0},
		{"both invalid", 3, 38, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AC3FrameSize(tt.fscod, tt.frmsizecod)
			if got != tt.want {
				t.Errorf("AC3FrameSize(%d, %d) = %d, want %d", tt.fscod, tt.frmsizecod, got, tt.want)
			}
		})
	}
}

func TestAC3FrameSize_AllValid(t *testing.T) {
	// Verify all valid combinations return non-zero values
	for fscod := byte(0); fscod < 3; fscod++ {
		for frmsizecod := byte(0); frmsizecod < 38; frmsizecod++ {
			size := AC3FrameSize(fscod, frmsizecod)
			if size == 0 {
				t.Errorf("AC3FrameSize(%d, %d) = 0, want non-zero", fscod, frmsizecod)
			}
			if size%2 != 0 {
				t.Errorf("AC3FrameSize(%d, %d) = %d, want even (frame sizes are in 16-bit words)", fscod, frmsizecod, size)
			}
		}
	}
}

func TestDTSCoreFrameSize(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want int
	}{
		{
			name: "valid DTS frame, 96 bytes (minimum)",
			// frame_size_raw = 95, +1 = 96
			// byte5 = 0x00 (bits: nblks[0]=0, frame_size[13:7]=0000000)
			// byte6 = 0xBE (bits: frame_size[6:0]=1011111, amode[5]=0)
			// frame_size_raw = (0x00 & 0x7F) << 7 | (0xBE >> 1) = 0 | 95 = 95
			data: []byte{0x7F, 0xFE, 0x80, 0x01, 0x00, 0x00, 0xBE},
			want: 96,
		},
		{
			name: "valid DTS frame, 2048 bytes",
			// frame_size_raw = 2047, +1 = 2048
			// 2047 = 0x7FF → upper 7 bits = 0x0F, lower 7 bits = 0x7F
			// byte5 = (0x0F) = 0x0F (nblks[0]=0, frame_size[13:7]=0001111)
			// byte6 = (0x7F << 1) = 0xFE (frame_size[6:0]=1111111, amode[5]=0)
			data: []byte{0x7F, 0xFE, 0x80, 0x01, 0x00, 0x0F, 0xFE},
			want: 2048,
		},
		{
			name: "valid DTS frame, 16384 bytes (maximum)",
			// frame_size_raw = 16383 = 0x3FFF
			// upper 7 bits = 0x7F, lower 7 bits = 0x7F
			// byte5 = 0x7F, byte6 = 0xFE
			data: []byte{0x7F, 0xFE, 0x80, 0x01, 0x00, 0x7F, 0xFE},
			want: 16384,
		},
		{
			name: "too small (frame < 96 bytes)",
			// frame_size_raw = 0, +1 = 1 → below 96 minimum
			data: []byte{0x7F, 0xFE, 0x80, 0x01, 0x00, 0x00, 0x00},
			want: 0,
		},
		{
			name: "wrong sync word",
			data: []byte{0x7F, 0xFE, 0x80, 0x02, 0x00, 0x0F, 0xFE},
			want: 0,
		},
		{
			name: "data too short",
			data: []byte{0x7F, 0xFE, 0x80, 0x01, 0x00, 0x0F},
			want: 0,
		},
		{
			name: "nil data",
			data: nil,
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DTSCoreFrameSize(tt.data)
			if got != tt.want {
				t.Errorf("DTSCoreFrameSize() = %d, want %d", got, tt.want)
			}
		})
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
