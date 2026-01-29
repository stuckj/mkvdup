package matcher

import (
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

	hashes := ExtractProbeHashes(data, true, windowSize)
	if len(hashes) == 0 {
		t.Fatal("Expected at least one hash from video data with start code")
	}

	// All hashes should be marked as video
	for _, h := range hashes {
		if !h.IsVideo {
			t.Errorf("Expected IsVideo=true, got false")
		}
	}

	// Verify the hash is computed from the sync point
	expectedHash := xxhash.Sum64(data[10 : 10+windowSize])
	found := false
	for _, h := range hashes {
		if h.Hash == expectedHash {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected hash computed from sync point at offset 10")
	}
}

func TestExtractProbeHashes_Audio(t *testing.T) {
	windowSize := 64

	// Create data with AC3 sync word (0x0B77)
	data := make([]byte, 128)
	// Place AC3 sync word at offset 20
	data[20] = 0x0B
	data[21] = 0x77

	hashes := ExtractProbeHashes(data, false, windowSize)
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
	hashes := ExtractProbeHashes(data, true, windowSize)
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

	hashes := ExtractProbeHashes(data, true, windowSize)
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

	hashes := ExtractProbeHashes(data, true, windowSize)
	if len(hashes) < 2 {
		t.Errorf("Expected at least 2 hashes for 2 sync points, got %d", len(hashes))
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

	hashes := ExtractProbeHashes(data, true, windowSize)
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

	hashes := ExtractProbeHashes(data, false, windowSize)
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

	hashes := ExtractProbeHashes(data, false, windowSize)
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

			hashes := ExtractProbeHashes(data, true, windowSize)
			if len(hashes) < tt.wantLen {
				t.Errorf("Expected at least %d hash(es), got %d", tt.wantLen, len(hashes))
			}
		})
	}
}

func TestExtractProbeHashes_EmptyData(t *testing.T) {
	hashes := ExtractProbeHashes([]byte{}, true, 64)
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
	hashes := ExtractProbeHashes(data, false, windowSize)
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
