package matcher

import (
	"bytes"
	"io"
	"testing"

	"github.com/cespare/xxhash/v2"
	"github.com/stuckj/mkvdup/internal/source"
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
				b[0] = 1    // configurationVersion must be 1
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

func TestNewMatcher(t *testing.T) {
	// Create a minimal source index
	idx := &source.Index{
		WindowSize:      64,
		HashToLocations: make(map[uint64][]source.Location),
		SourceDir:       "/test/dir",
		SourceType:      source.TypeDVD,
	}

	matcher, err := NewMatcher(idx)
	if err != nil {
		t.Fatalf("NewMatcher() error = %v", err)
	}
	defer matcher.Close()

	if matcher == nil {
		t.Fatal("NewMatcher() returned nil")
	}
	if matcher.windowSize != 64 {
		t.Errorf("windowSize = %d, want 64", matcher.windowSize)
	}
}

func TestMatcher_SetNumWorkers(t *testing.T) {
	idx := &source.Index{
		WindowSize:      64,
		HashToLocations: make(map[uint64][]source.Location),
	}

	matcher, err := NewMatcher(idx)
	if err != nil {
		t.Fatalf("NewMatcher() error = %v", err)
	}
	defer matcher.Close()

	// Set to specific value
	matcher.SetNumWorkers(4)
	if matcher.numWorkers != 4 {
		t.Errorf("numWorkers = %d, want 4", matcher.numWorkers)
	}

	// Set to zero should clamp to 1
	matcher.SetNumWorkers(0)
	if matcher.numWorkers != 1 {
		t.Errorf("numWorkers = %d, want 1 (clamped)", matcher.numWorkers)
	}

	// Set to negative should clamp to 1
	matcher.SetNumWorkers(-5)
	if matcher.numWorkers != 1 {
		t.Errorf("numWorkers = %d, want 1 (clamped)", matcher.numWorkers)
	}
}

func TestMatcher_Close(t *testing.T) {
	idx := &source.Index{
		WindowSize:      64,
		HashToLocations: make(map[uint64][]source.Location),
	}

	matcher, err := NewMatcher(idx)
	if err != nil {
		t.Fatalf("NewMatcher() error = %v", err)
	}

	// Close should not error
	if err := matcher.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Close again should be safe (idempotent)
	if err := matcher.Close(); err != nil {
		t.Errorf("Second Close() error = %v", err)
	}
}

func TestMaxExpansionBytes(t *testing.T) {
	// Verify MaxExpansionBytes is reasonable
	if MaxExpansionBytes <= 0 {
		t.Errorf("MaxExpansionBytes = %d, should be positive", MaxExpansionBytes)
	}
	// Should be at least 1MB for video keyframes
	if MaxExpansionBytes < 1024*1024 {
		t.Errorf("MaxExpansionBytes = %d, should be at least 1MB", MaxExpansionBytes)
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

func TestProbeHash_Fields(t *testing.T) {
	// Test that ProbeHash struct has expected fields
	h := ProbeHash{
		Hash:    12345,
		IsVideo: true,
	}

	if h.Hash != 12345 {
		t.Errorf("Hash = %d, want 12345", h.Hash)
	}
	if !h.IsVideo {
		t.Error("IsVideo should be true")
	}
}

func TestEntry_Fields(t *testing.T) {
	// Test that Entry struct has expected fields
	e := Entry{
		MkvOffset:        1000,
		Length:           500,
		Source:           1,
		SourceOffset:     2000,
		IsVideo:          true,
		AudioSubStreamID: 0x80,
	}

	if e.MkvOffset != 1000 {
		t.Errorf("MkvOffset = %d, want 1000", e.MkvOffset)
	}
	if e.Length != 500 {
		t.Errorf("Length = %d, want 500", e.Length)
	}
	if e.Source != 1 {
		t.Errorf("Source = %d, want 1", e.Source)
	}
	if e.SourceOffset != 2000 {
		t.Errorf("SourceOffset = %d, want 2000", e.SourceOffset)
	}
	if !e.IsVideo {
		t.Error("IsVideo should be true")
	}
	if e.AudioSubStreamID != 0x80 {
		t.Errorf("AudioSubStreamID = 0x%02X, want 0x80", e.AudioSubStreamID)
	}
}

func TestResult_Fields(t *testing.T) {
	// Test that Result struct has expected fields
	r := Result{
		Entries:        []Entry{{MkvOffset: 0, Length: 100}},
		DeltaData:      []byte{1, 2, 3},
		MatchedBytes:   1000,
		UnmatchedBytes: 100,
		MatchedPackets: 50,
		TotalPackets:   60,
	}

	if len(r.Entries) != 1 {
		t.Errorf("Entries len = %d, want 1", len(r.Entries))
	}
	if len(r.DeltaData) != 3 {
		t.Errorf("DeltaData len = %d, want 3", len(r.DeltaData))
	}
	if r.MatchedBytes != 1000 {
		t.Errorf("MatchedBytes = %d, want 1000", r.MatchedBytes)
	}
	if r.UnmatchedBytes != 100 {
		t.Errorf("UnmatchedBytes = %d, want 100", r.UnmatchedBytes)
	}
	if r.MatchedPackets != 50 {
		t.Errorf("MatchedPackets = %d, want 50", r.MatchedPackets)
	}
	if r.TotalPackets != 60 {
		t.Errorf("TotalPackets = %d, want 60", r.TotalPackets)
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

func TestCoverageBitmap(t *testing.T) {
	// Create a matcher with minimal setup
	idx := source.NewIndex("/test", source.TypeDVD, 64)
	m, _ := NewMatcher(idx)

	// Simulate MKV size of 100KB
	m.mkvSize = 100 * 1024

	// Initialize coverage bitmap (normally done in Match)
	numChunks := (m.mkvSize + coverageChunkSize - 1) / coverageChunkSize
	m.coveredChunks = make([]uint64, (numChunks+63)/64)

	// Test marking chunks covered
	// Mark region from 8192 to 16384 (exactly 2 chunks at 4KB granularity)
	m.markChunksCovered(8192, 16384)

	// Chunks 2 and 3 should be fully covered (chunk 2 = [8192, 12288), chunk 3 = [12288, 16384))
	// isRangeCoveredParallel checks if ALL chunks in the range are covered
	if !m.isRangeCoveredParallel(8192, 4096) {
		t.Error("Range [8192, 12288) should be covered")
	}
	if !m.isRangeCoveredParallel(12288, 4096) {
		t.Error("Range [12288, 16384) should be covered")
	}

	// Range spanning chunks 2-3 should be covered
	if !m.isRangeCoveredParallel(8192, 8192) {
		t.Error("Range [8192, 16384) should be covered")
	}

	// Chunk 1 (before the marked region) should not be covered
	if m.isRangeCoveredParallel(4096, 4096) {
		t.Error("Range [4096, 8192) should NOT be covered")
	}

	// Chunk 4 (after the marked region) should not be covered
	if m.isRangeCoveredParallel(16384, 4096) {
		t.Error("Range [16384, 20480) should NOT be covered")
	}
}

func TestCoverageBitmap_PartialChunks(t *testing.T) {
	idx := source.NewIndex("/test", source.TypeDVD, 64)
	m, _ := NewMatcher(idx)

	m.mkvSize = 100 * 1024
	numChunks := (m.mkvSize + coverageChunkSize - 1) / coverageChunkSize
	m.coveredChunks = make([]uint64, (numChunks+63)/64)

	// Mark a region that doesn't fully cover any chunks
	// Region [5000, 7000) is within chunk 1 [4096, 8192) but doesn't fully contain it
	m.markChunksCovered(5000, 7000)

	// No chunks should be marked as covered since the region
	// doesn't fully contain any chunk
	if m.isRangeCoveredParallel(4096, 4096) {
		t.Error("Partial coverage should not mark chunk as covered")
	}
}

func newTestMatcherWithRegions(mkvSize int64, data []byte, regions []matchedRegion) *Matcher {
	return &Matcher{
		mkvSize:        mkvSize,
		mkvData:        data,
		matchedRegions: regions,
	}
}

func TestMergeRegions(t *testing.T) {
	tests := []struct {
		name     string
		regions  []matchedRegion
		expected []matchedRegion
	}{
		{
			name:     "empty",
			regions:  nil,
			expected: nil,
		},
		{
			name: "single region",
			regions: []matchedRegion{
				{mkvStart: 0, mkvEnd: 100, fileIndex: 0, srcOffset: 0},
			},
			expected: []matchedRegion{
				{mkvStart: 0, mkvEnd: 100, fileIndex: 0, srcOffset: 0},
			},
		},
		{
			name: "non-overlapping",
			regions: []matchedRegion{
				{mkvStart: 0, mkvEnd: 100, fileIndex: 0, srcOffset: 0},
				{mkvStart: 200, mkvEnd: 300, fileIndex: 0, srcOffset: 0},
			},
			expected: []matchedRegion{
				{mkvStart: 0, mkvEnd: 100, fileIndex: 0, srcOffset: 0},
				{mkvStart: 200, mkvEnd: 300, fileIndex: 0, srcOffset: 0},
			},
		},
		{
			name: "adjacent touching same source",
			regions: []matchedRegion{
				{mkvStart: 0, mkvEnd: 100, fileIndex: 0, srcOffset: 0},
				{mkvStart: 100, mkvEnd: 200, fileIndex: 0, srcOffset: 100},
			},
			expected: []matchedRegion{
				// Adjacent regions are not overlapping (curr.mkvStart >= last.mkvEnd),
				// so they remain separate even with consistent source mapping.
				{mkvStart: 0, mkvEnd: 100, fileIndex: 0, srcOffset: 0},
				{mkvStart: 100, mkvEnd: 200, fileIndex: 0, srcOffset: 100},
			},
		},
		{
			name: "fully contained same source",
			regions: []matchedRegion{
				{mkvStart: 0, mkvEnd: 300, fileIndex: 0, srcOffset: 0},
				{mkvStart: 50, mkvEnd: 150, fileIndex: 0, srcOffset: 50},
			},
			expected: []matchedRegion{
				{mkvStart: 0, mkvEnd: 300, fileIndex: 0, srcOffset: 0},
			},
		},
		{
			name: "fully contained different source",
			regions: []matchedRegion{
				{mkvStart: 0, mkvEnd: 300, fileIndex: 0, srcOffset: 0},
				{mkvStart: 50, mkvEnd: 150, fileIndex: 1, srcOffset: 500},
			},
			expected: []matchedRegion{
				// curr is fully contained in last, so it's dropped
				{mkvStart: 0, mkvEnd: 300, fileIndex: 0, srcOffset: 0},
			},
		},
		{
			name: "overlap different file clips later region",
			regions: []matchedRegion{
				{mkvStart: 0, mkvEnd: 100, fileIndex: 0, srcOffset: 0},
				{mkvStart: 50, mkvEnd: 250, fileIndex: 1, srcOffset: 500},
			},
			expected: []matchedRegion{
				// Earlier region keeps priority, later region is clipped
				{mkvStart: 0, mkvEnd: 100, fileIndex: 0, srcOffset: 0},
				{mkvStart: 100, mkvEnd: 250, fileIndex: 1, srcOffset: 550},
			},
		},
		{
			name: "overlap same source consistent offsets merges",
			regions: []matchedRegion{
				{mkvStart: 0, mkvEnd: 200, fileIndex: 0, srcOffset: 1000},
				{mkvStart: 100, mkvEnd: 300, fileIndex: 0, srcOffset: 1100},
			},
			expected: []matchedRegion{
				// Same source, consistent mapping: merge into one
				{mkvStart: 0, mkvEnd: 300, fileIndex: 0, srcOffset: 1000},
			},
		},
		{
			name: "overlap different source clips shorter",
			regions: []matchedRegion{
				{mkvStart: 0, mkvEnd: 200, fileIndex: 0, srcOffset: 0},
				{mkvStart: 100, mkvEnd: 250, fileIndex: 1, srcOffset: 500},
			},
			expected: []matchedRegion{
				// Earlier region keeps priority, later region is clipped
				{mkvStart: 0, mkvEnd: 200, fileIndex: 0, srcOffset: 0},
				{mkvStart: 200, mkvEnd: 250, fileIndex: 1, srcOffset: 600},
			},
		},
		{
			name: "unsorted input same source consistent offsets",
			regions: []matchedRegion{
				{mkvStart: 200, mkvEnd: 300, fileIndex: 0, srcOffset: 200},
				{mkvStart: 0, mkvEnd: 150, fileIndex: 0, srcOffset: 0},
				{mkvStart: 100, mkvEnd: 250, fileIndex: 0, srcOffset: 100},
			},
			expected: []matchedRegion{
				// After sort: [0,150) src=0, [100,250) src=100, [200,300) src=200
				// [0,150) + [100,250): same source, consistent (0+100=100) → merge [0,250)
				// [0,250) + [200,300): same source, consistent (0+200=200) → merge [0,300)
				{mkvStart: 0, mkvEnd: 300, fileIndex: 0, srcOffset: 0},
			},
		},
		{
			name: "multiple non-overlapping",
			regions: []matchedRegion{
				{mkvStart: 0, mkvEnd: 50, fileIndex: 0, srcOffset: 0},
				{mkvStart: 100, mkvEnd: 150, fileIndex: 0, srcOffset: 0},
				{mkvStart: 200, mkvEnd: 250, fileIndex: 0, srcOffset: 0},
			},
			expected: []matchedRegion{
				{mkvStart: 0, mkvEnd: 50, fileIndex: 0, srcOffset: 0},
				{mkvStart: 100, mkvEnd: 150, fileIndex: 0, srcOffset: 0},
				{mkvStart: 200, mkvEnd: 250, fileIndex: 0, srcOffset: 0},
			},
		},
		{
			name: "source fields preserved",
			regions: []matchedRegion{
				{mkvStart: 0, mkvEnd: 100, fileIndex: 1, srcOffset: 500, isVideo: true, audioSubStreamID: 0x00},
				{mkvStart: 200, mkvEnd: 400, fileIndex: 3, srcOffset: 8000, isVideo: false, audioSubStreamID: 0x80},
			},
			expected: []matchedRegion{
				{mkvStart: 0, mkvEnd: 100, fileIndex: 1, srcOffset: 500, isVideo: true, audioSubStreamID: 0x00},
				{mkvStart: 200, mkvEnd: 400, fileIndex: 3, srcOffset: 8000, isVideo: false, audioSubStreamID: 0x80},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMatcherWithRegions(1000, nil, tt.regions)
			m.mergeRegions()

			if len(m.matchedRegions) != len(tt.expected) {
				t.Fatalf("got %d regions, want %d", len(m.matchedRegions), len(tt.expected))
			}

			for i, got := range m.matchedRegions {
				want := tt.expected[i]
				if got.mkvStart != want.mkvStart {
					t.Errorf("region[%d].mkvStart = %d, want %d", i, got.mkvStart, want.mkvStart)
				}
				if got.mkvEnd != want.mkvEnd {
					t.Errorf("region[%d].mkvEnd = %d, want %d", i, got.mkvEnd, want.mkvEnd)
				}
				if got.fileIndex != want.fileIndex {
					t.Errorf("region[%d].fileIndex = %d, want %d", i, got.fileIndex, want.fileIndex)
				}
				if got.srcOffset != want.srcOffset {
					t.Errorf("region[%d].srcOffset = %d, want %d", i, got.srcOffset, want.srcOffset)
				}
				if got.isVideo != want.isVideo {
					t.Errorf("region[%d].isVideo = %v, want %v", i, got.isVideo, want.isVideo)
				}
				if got.audioSubStreamID != want.audioSubStreamID {
					t.Errorf("region[%d].audioSubStreamID = 0x%02X, want 0x%02X", i, got.audioSubStreamID, want.audioSubStreamID)
				}
			}
		})
	}
}

func TestBuildEntries(t *testing.T) {
	tests := []struct {
		name           string
		mkvSize        int64
		regions        []matchedRegion
		wantEntryCount int
		wantEntries    []Entry
		wantDeltaBytes []byte // nil means skip delta content check
	}{
		{
			name:           "all delta",
			mkvSize:        100,
			regions:        nil,
			wantEntryCount: 1,
			wantEntries: []Entry{
				{MkvOffset: 0, Length: 100, Source: 0, SourceOffset: 0},
			},
		},
		{
			name:    "all matched",
			mkvSize: 100,
			regions: []matchedRegion{
				{mkvStart: 0, mkvEnd: 100, fileIndex: 2, srcOffset: 500},
			},
			wantEntryCount: 1,
			wantEntries: []Entry{
				{MkvOffset: 0, Length: 100, Source: 3, SourceOffset: 500},
			},
		},
		{
			name:    "gap match gap",
			mkvSize: 300,
			regions: []matchedRegion{
				{mkvStart: 100, mkvEnd: 200, fileIndex: 0, srcOffset: 1000},
			},
			wantEntryCount: 3,
			wantEntries: []Entry{
				{MkvOffset: 0, Length: 100, Source: 0, SourceOffset: 0},
				{MkvOffset: 100, Length: 100, Source: 1, SourceOffset: 1000},
				{MkvOffset: 200, Length: 100, Source: 0, SourceOffset: 100},
			},
		},
		{
			name:    "match at start",
			mkvSize: 200,
			regions: []matchedRegion{
				{mkvStart: 0, mkvEnd: 100, fileIndex: 1, srcOffset: 0},
			},
			wantEntryCount: 2,
			wantEntries: []Entry{
				{MkvOffset: 0, Length: 100, Source: 2, SourceOffset: 0},
				{MkvOffset: 100, Length: 100, Source: 0, SourceOffset: 0},
			},
		},
		{
			name:    "match at end",
			mkvSize: 200,
			regions: []matchedRegion{
				{mkvStart: 100, mkvEnd: 200, fileIndex: 0, srcOffset: 500},
			},
			wantEntryCount: 2,
			wantEntries: []Entry{
				{MkvOffset: 0, Length: 100, Source: 0, SourceOffset: 0},
				{MkvOffset: 100, Length: 100, Source: 1, SourceOffset: 500},
			},
		},
		{
			name:    "multiple regions",
			mkvSize: 500,
			regions: []matchedRegion{
				{mkvStart: 50, mkvEnd: 150, fileIndex: 0, srcOffset: 0},
				{mkvStart: 250, mkvEnd: 350, fileIndex: 0, srcOffset: 0},
			},
			wantEntryCount: 5,
			wantEntries: []Entry{
				{MkvOffset: 0, Length: 50, Source: 0, SourceOffset: 0},
				{MkvOffset: 50, Length: 100, Source: 1, SourceOffset: 0},
				{MkvOffset: 150, Length: 100, Source: 0, SourceOffset: 50},
				{MkvOffset: 250, Length: 100, Source: 1, SourceOffset: 0},
				{MkvOffset: 350, Length: 150, Source: 0, SourceOffset: 150},
			},
		},
		{
			name:    "source field propagation",
			mkvSize: 300,
			regions: []matchedRegion{
				{mkvStart: 100, mkvEnd: 200, fileIndex: 2, srcOffset: 5000, isVideo: true, audioSubStreamID: 0x80},
			},
			wantEntryCount: 3,
			wantEntries: []Entry{
				{MkvOffset: 0, Length: 100, Source: 0, SourceOffset: 0},
				{MkvOffset: 100, Length: 100, Source: 3, SourceOffset: 5000, IsVideo: true, AudioSubStreamID: 0x80},
				{MkvOffset: 200, Length: 100, Source: 0, SourceOffset: 100},
			},
		},
		{
			name:           "zero length file",
			mkvSize:        0,
			regions:        nil,
			wantEntryCount: 0,
			wantEntries:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mkvData := make([]byte, tt.mkvSize)
			for i := range mkvData {
				mkvData[i] = byte(i % 256)
			}
			m := newTestMatcherWithRegions(tt.mkvSize, mkvData, tt.regions)
			entries, deltaWriter, err := m.buildEntries()
			if err != nil {
				t.Fatalf("buildEntries error: %v", err)
			}
			defer deltaWriter.Close()

			if len(entries) != tt.wantEntryCount {
				t.Fatalf("got %d entries, want %d", len(entries), tt.wantEntryCount)
			}

			for i, got := range entries {
				if i >= len(tt.wantEntries) {
					break
				}
				want := tt.wantEntries[i]
				if got.MkvOffset != want.MkvOffset {
					t.Errorf("entry[%d].MkvOffset = %d, want %d", i, got.MkvOffset, want.MkvOffset)
				}
				if got.Length != want.Length {
					t.Errorf("entry[%d].Length = %d, want %d", i, got.Length, want.Length)
				}
				if got.Source != want.Source {
					t.Errorf("entry[%d].Source = %d, want %d", i, got.Source, want.Source)
				}
				if got.SourceOffset != want.SourceOffset {
					t.Errorf("entry[%d].SourceOffset = %d, want %d", i, got.SourceOffset, want.SourceOffset)
				}
				if got.IsVideo != want.IsVideo {
					t.Errorf("entry[%d].IsVideo = %v, want %v", i, got.IsVideo, want.IsVideo)
				}
				if got.AudioSubStreamID != want.AudioSubStreamID {
					t.Errorf("entry[%d].AudioSubStreamID = 0x%02X, want 0x%02X", i, got.AudioSubStreamID, want.AudioSubStreamID)
				}
			}

			// For "all matched", delta should be empty
			if tt.name == "all matched" && deltaWriter.Size() != 0 {
				t.Errorf("expected empty delta data for all matched, got %d bytes", deltaWriter.Size())
			}
		})
	}
}

func TestBuildEntries_DeltaDataContent(t *testing.T) {
	mkvSize := int64(50)
	mkvData := make([]byte, mkvSize)
	for i := range mkvData {
		mkvData[i] = byte(i % 256)
	}

	regions := []matchedRegion{
		{mkvStart: 20, mkvEnd: 30, fileIndex: 0, srcOffset: 0},
	}
	m := newTestMatcherWithRegions(mkvSize, mkvData, regions)
	_, deltaWriter, err := m.buildEntries()
	if err != nil {
		t.Fatalf("buildEntries error: %v", err)
	}
	defer deltaWriter.Close()

	// Read delta data back from temp file
	f := deltaWriter.File()
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatalf("seek error: %v", err)
	}
	deltaData := make([]byte, deltaWriter.Size())
	if _, err := io.ReadFull(f, deltaData); err != nil {
		t.Fatalf("read delta file: %v", err)
	}

	// Delta should be mkvData[0:20] + mkvData[30:50]
	expectedDelta := make([]byte, 0, 40)
	expectedDelta = append(expectedDelta, mkvData[0:20]...)
	expectedDelta = append(expectedDelta, mkvData[30:50]...)

	if !bytes.Equal(deltaData, expectedDelta) {
		t.Errorf("delta data mismatch: got %d bytes, want %d bytes", len(deltaData), len(expectedDelta))
		for i := 0; i < len(deltaData) && i < len(expectedDelta); i++ {
			if deltaData[i] != expectedDelta[i] {
				t.Errorf("first difference at byte %d: got 0x%02X, want 0x%02X", i, deltaData[i], expectedDelta[i])
				break
			}
		}
	}
}

func TestCoverageBitmap_MarkMultipleChunks(t *testing.T) {
	idx := source.NewIndex("/test", source.TypeDVD, 64)
	m, _ := NewMatcher(idx)

	m.mkvSize = 100 * 1024
	numChunks := (m.mkvSize + coverageChunkSize - 1) / coverageChunkSize
	m.coveredChunks = make([]uint64, (numChunks+63)/64)

	// Mark region {0, 40960} which is exactly 10 chunks (0 through 9)
	m.markChunksCovered(0, 40960)

	// Verify each individual chunk in the range is covered
	for i := int64(0); i < 10; i++ {
		offset := i * coverageChunkSize
		if !m.isRangeCoveredParallel(offset, coverageChunkSize) {
			t.Errorf("chunk %d at offset %d should be covered", i, offset)
		}
	}

	// Chunk 10 should not be covered
	if m.isRangeCoveredParallel(40960, coverageChunkSize) {
		t.Error("chunk 10 at offset 40960 should NOT be covered")
	}
}

func TestCoverageBitmap_EmptyBitmapUncovered(t *testing.T) {
	idx := source.NewIndex("/test", source.TypeDVD, 64)
	m, _ := NewMatcher(idx)

	m.mkvSize = 100 * 1024
	numChunks := (m.mkvSize + coverageChunkSize - 1) / coverageChunkSize
	m.coveredChunks = make([]uint64, (numChunks+63)/64)

	// Fresh bitmap with no marks - first chunk should not be covered
	if m.isRangeCoveredParallel(0, coverageChunkSize) {
		t.Error("fresh bitmap should report range as uncovered")
	}
}
