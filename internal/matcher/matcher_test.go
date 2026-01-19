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
