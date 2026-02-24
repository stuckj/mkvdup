package matcher

import (
	"testing"

	"github.com/cespare/xxhash/v2"
)

func TestExtractProbeHashes_Video(t *testing.T) {
	windowSize := 64

	// Create data with video start code (00 00 01)
	data := make([]byte, 128)
	// Place a video start code at offset 10
	data[10] = 0x00
	data[11] = 0x00
	data[12] = 0x01
	data[13] = 0xB3 // Sequence header start code

	hashes := ExtractProbeHashes(data, true, windowSize, 0)
	if len(hashes) == 0 {
		t.Fatal("Expected at least one hash from video data with start code")
	}

	// All hashes should be marked as video
	for _, h := range hashes {
		if !h.IsVideo {
			t.Errorf("Expected IsVideo=true, got false")
		}
	}

	// Verify the hash is computed from the NAL header byte (offset 13, after 00 00 01)
	expectedHash := xxhash.Sum64(data[13 : 13+windowSize])
	found := false
	for _, h := range hashes {
		if h.Hash == expectedHash {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected hash computed from NAL header at offset 13")
	}
}

func TestExtractProbeHashes_Audio(t *testing.T) {
	windowSize := 64

	// Create data with AC3 sync word (0x0B77)
	data := make([]byte, 128)
	// Place AC3 sync word at offset 20
	data[20] = 0x0B
	data[21] = 0x77

	hashes := ExtractProbeHashes(data, false, windowSize, 0)
	if len(hashes) == 0 {
		t.Fatal("Expected at least one hash from audio data with sync word")
	}

	// All hashes should be marked as audio
	for _, h := range hashes {
		if h.IsVideo {
			t.Errorf("Expected IsVideo=false, got true")
		}
	}

	// Verify the hash is computed from the sync point
	expectedHash := xxhash.Sum64(data[20 : 20+windowSize])
	found := false
	for _, h := range hashes {
		if h.Hash == expectedHash {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected hash computed from sync point at offset 20")
	}
}

func TestExtractProbeHashes_NoSyncPoint(t *testing.T) {
	windowSize := 64

	// Create data without any sync points
	data := make([]byte, 128)
	for i := range data {
		data[i] = byte(i % 256)
	}

	// Should still return a hash from the start of data
	hashes := ExtractProbeHashes(data, true, windowSize, 0)
	if len(hashes) == 0 {
		t.Fatal("Expected at least one hash even without sync points")
	}

	// The hash should be from the start of data
	expectedHash := xxhash.Sum64(data[:windowSize])
	if hashes[0].Hash != expectedHash {
		t.Errorf("Expected hash from data start when no sync points found")
	}
}

func TestExtractProbeHashes_TooSmall(t *testing.T) {
	windowSize := 64

	// Data smaller than window size
	data := make([]byte, windowSize-1)

	hashes := ExtractProbeHashes(data, true, windowSize, 0)
	if hashes != nil {
		t.Errorf("Expected nil for data smaller than window size, got %d hashes", len(hashes))
	}
}

func TestExtractProbeHashes_MultipleSyncPoints(t *testing.T) {
	windowSize := 64

	// Create data with multiple video start codes
	data := make([]byte, 256)
	// Start code at offset 0
	data[0] = 0x00
	data[1] = 0x00
	data[2] = 0x01
	// Start code at offset 100
	data[100] = 0x00
	data[101] = 0x00
	data[102] = 0x01

	hashes := ExtractProbeHashes(data, true, windowSize, 0)
	if len(hashes) < 2 {
		t.Errorf("Expected at least 2 hashes for 2 sync points, got %d", len(hashes))
	}
}

func TestExtractProbeHashes_AVCC(t *testing.T) {
	windowSize := 64

	// Build AVCC formatted data: [4-byte length][NAL unit data]
	// NAL unit 1: length=80, data starts at offset 4
	data := make([]byte, 256)
	data[0] = 0x00
	data[1] = 0x00
	data[2] = 0x00
	data[3] = 80 // NAL length = 80
	for i := 4; i < 84; i++ {
		data[i] = byte(i) // NAL data
	}
	// NAL unit 2: length=80, data starts at offset 88
	data[84] = 0x00
	data[85] = 0x00
	data[86] = 0x00
	data[87] = 80
	for i := 88; i < 168; i++ {
		data[i] = byte(i)
	}

	hashes := ExtractProbeHashes(data, true, windowSize, 4)
	if len(hashes) < 2 {
		t.Fatalf("Expected at least 2 hashes from 2 AVCC NAL units, got %d", len(hashes))
	}

	// First hash should be from offset 4 (first NAL header)
	expectedHash1 := xxhash.Sum64(data[4 : 4+windowSize])
	if hashes[0].Hash != expectedHash1 {
		t.Errorf("First hash should be from NAL header at offset 4")
	}

	// Second hash should be from offset 88 (second NAL header)
	expectedHash2 := xxhash.Sum64(data[88 : 88+windowSize])
	if hashes[1].Hash != expectedHash2 {
		t.Errorf("Second hash should be from NAL header at offset 88")
	}

	// Both should be video
	for _, h := range hashes {
		if !h.IsVideo {
			t.Error("AVCC hashes should be marked as video")
		}
	}
}

func TestDetectNALLengthSize(t *testing.T) {
	tests := []struct {
		name         string
		codecID      string
		codecPrivate []byte
		expected     int
	}{
		{
			name:     "MPEG-2 is Annex B",
			codecID:  "V_MPEG2",
			expected: 0,
		},
		{
			name:    "H.264 with valid AVCC",
			codecID: "V_MPEG4/ISO/AVC",
			// AVCDecoderConfigurationRecord: version=1, profile=100, compat=0, level=40, NALLengthSize=4 (0x03+1)
			codecPrivate: []byte{0x01, 0x64, 0x00, 0x28, 0xFF, 0xE1, 0x00},
			expected:     4,
		},
		{
			name:    "H.264 NALLengthSize=2",
			codecID: "V_MPEG4/ISO/AVC",
			// byte 4: 0xFD = 1111_1101, &0x03 = 01, +1 = 2
			codecPrivate: []byte{0x01, 0x64, 0x00, 0x28, 0xFD, 0xE1, 0x00},
			expected:     2,
		},
		{
			name:     "H.264 no CodecPrivate defaults to 4",
			codecID:  "V_MPEG4/ISO/AVC",
			expected: 4,
		},
		{
			name:    "H.265 with valid HVCC",
			codecID: "V_MPEGH/ISO/HEVC",
			codecPrivate: func() []byte {
				b := make([]byte, 23)
				b[0] = 1     // configurationVersion must be 1
				b[21] = 0xFC // upper 6 bits = 111111 (reserved), lower 2 bits = 00, size = 1
				return b
			}(),
			expected: 1,
		},
		{
			name:     "H.265 no CodecPrivate defaults to 4",
			codecID:  "V_MPEGH/ISO/HEVC",
			expected: 4,
		},
		{
			name:     "Audio codec returns 0",
			codecID:  "A_AC3",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectNALLengthSize(tt.codecID, tt.codecPrivate)
			if result != tt.expected {
				t.Errorf("detectNALLengthSize(%q) = %d, want %d", tt.codecID, result, tt.expected)
			}
		})
	}
}

func TestExtractProbeHashes_ExactWindowSize(t *testing.T) {
	windowSize := 64

	// Data exactly equal to window size
	data := make([]byte, windowSize)
	for i := range data {
		data[i] = byte(i)
	}

	hashes := ExtractProbeHashes(data, true, windowSize, 0)
	if len(hashes) == 0 {
		t.Fatal("Expected at least one hash for data equal to window size")
	}

	// Should get hash from data start (no sync point)
	expectedHash := xxhash.Sum64(data)
	if hashes[0].Hash != expectedHash {
		t.Errorf("Expected hash from data start")
	}
}

func TestExtractProbeHashes_DifferentSyncPoints(t *testing.T) {
	windowSize := 64
	data := make([]byte, 128)

	// Test DTS sync word (0x7FFE)
	data[10] = 0x7F
	data[11] = 0xFE

	hashes := ExtractProbeHashes(data, false, windowSize, 0)
	if len(hashes) == 0 {
		t.Fatal("Expected at least one hash from DTS data")
	}
	// Should be marked as audio
	for _, h := range hashes {
		if h.IsVideo {
			t.Error("DTS hashes should be marked as audio")
		}
	}
}

func TestExtractProbeHashes_MPASync(t *testing.T) {
	windowSize := 64
	data := make([]byte, 128)

	// Test MP3/MPA sync word (0xFF followed by 0xFx or 0xEx)
	data[5] = 0xFF
	data[6] = 0xFB // Valid MP3 frame sync

	hashes := ExtractProbeHashes(data, false, windowSize, 0)
	if len(hashes) == 0 {
		t.Fatal("Expected at least one hash from MPA data")
	}
}

func TestExtractProbeHashes_VideoStartCodesTypes(t *testing.T) {
	windowSize := 64

	// Test different video start codes
	tests := []struct {
		name    string
		code    byte
		wantLen int
	}{
		{"Sequence header (0xB3)", 0xB3, 1},
		{"Picture (0x00)", 0x00, 1},
		{"GOP (0xB8)", 0xB8, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := make([]byte, 128)
			data[10] = 0x00
			data[11] = 0x00
			data[12] = 0x01
			data[13] = tt.code

			hashes := ExtractProbeHashes(data, true, windowSize, 0)
			if len(hashes) < tt.wantLen {
				t.Errorf("Expected at least %d hash(es), got %d", tt.wantLen, len(hashes))
			}
		})
	}
}

func TestExtractProbeHashes_EmptyData(t *testing.T) {
	hashes := ExtractProbeHashes([]byte{}, true, 64, 0)
	if hashes != nil {
		t.Errorf("Expected nil for empty data, got %d hashes", len(hashes))
	}
}

func TestExtractProbeHashes_AudioWithNoSyncPoint(t *testing.T) {
	windowSize := 64
	data := make([]byte, 128)

	// Fill with random-ish data (no sync points)
	for i := range data {
		data[i] = byte((i * 7) % 256)
	}

	// Should still return a hash from start
	hashes := ExtractProbeHashes(data, false, windowSize, 0)
	if len(hashes) == 0 {
		t.Fatal("Expected at least one hash")
	}

	// Hash should be from data start
	expectedHash := xxhash.Sum64(data[:windowSize])
	if hashes[0].Hash != expectedHash {
		t.Errorf("Expected hash from data start when no sync point")
	}
}
