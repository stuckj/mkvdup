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

func TestEntry_Fields(t *testing.T) {
	// Test that Entry struct fields work as expected
	e := Entry{
		MkvOffset:        1000,
		Length:           500,
		Source:           1,
		SourceOffset:     2000,
		IsVideo:          true,
		AudioSubStreamID: 0x80,
	}

	if e.MkvOffset != 1000 {
		t.Errorf("MkvOffset: got %d, want 1000", e.MkvOffset)
	}
	if e.Length != 500 {
		t.Errorf("Length: got %d, want 500", e.Length)
	}
	if e.Source != 1 {
		t.Errorf("Source: got %d, want 1", e.Source)
	}
	if e.SourceOffset != 2000 {
		t.Errorf("SourceOffset: got %d, want 2000", e.SourceOffset)
	}
	if !e.IsVideo {
		t.Error("IsVideo: got false, want true")
	}
	if e.AudioSubStreamID != 0x80 {
		t.Errorf("AudioSubStreamID: got %x, want 0x80", e.AudioSubStreamID)
	}
}

func TestResult_Fields(t *testing.T) {
	// Test that Result struct fields work as expected
	r := Result{
		Entries:        []Entry{{MkvOffset: 0, Length: 100}},
		DeltaData:      []byte{1, 2, 3},
		MatchedBytes:   1000,
		UnmatchedBytes: 100,
		MatchedPackets: 50,
		TotalPackets:   60,
	}

	if len(r.Entries) != 1 {
		t.Errorf("Entries length: got %d, want 1", len(r.Entries))
	}
	if len(r.DeltaData) != 3 {
		t.Errorf("DeltaData length: got %d, want 3", len(r.DeltaData))
	}
	if r.MatchedBytes != 1000 {
		t.Errorf("MatchedBytes: got %d, want 1000", r.MatchedBytes)
	}
	if r.UnmatchedBytes != 100 {
		t.Errorf("UnmatchedBytes: got %d, want 100", r.UnmatchedBytes)
	}
	if r.MatchedPackets != 50 {
		t.Errorf("MatchedPackets: got %d, want 50", r.MatchedPackets)
	}
	if r.TotalPackets != 60 {
		t.Errorf("TotalPackets: got %d, want 60", r.TotalPackets)
	}
}

func TestProbeHash_Fields(t *testing.T) {
	// Test that ProbeHash struct fields work as expected
	h := ProbeHash{
		Hash:    0x123456789ABCDEF0,
		IsVideo: true,
	}

	if h.Hash != 0x123456789ABCDEF0 {
		t.Errorf("Hash: got %x, want 0x123456789ABCDEF0", h.Hash)
	}
	if !h.IsVideo {
		t.Error("IsVideo: got false, want true")
	}
}
