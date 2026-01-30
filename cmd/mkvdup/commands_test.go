package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cespare/xxhash/v2"
	"github.com/stuckj/mkvdup/internal/mkv"
)

// TestSamplePackets tests the samplePackets function which selects
// packets distributed across a file using stratified sampling.
func TestSamplePackets(t *testing.T) {
	// Create test packets with distinct offsets for identification
	makePackets := func(n int) []mkv.Packet {
		packets := make([]mkv.Packet, n)
		for i := 0; i < n; i++ {
			packets[i] = mkv.Packet{
				Offset:    int64(i * 1000),
				Size:      100,
				TrackNum:  1,
				Timestamp: int64(i * 40), // 40ms per frame (25fps)
				Keyframe:  i%12 == 0,     // Keyframe every 12 frames
			}
		}
		return packets
	}

	t.Run("fewer packets than requested", func(t *testing.T) {
		packets := makePackets(5)
		result := samplePackets(packets, 10)
		if len(result) != 5 {
			t.Errorf("Expected all 5 packets when requesting 10, got %d", len(result))
		}
		// Should return original slice
		for i, p := range result {
			if p.Offset != packets[i].Offset {
				t.Errorf("Packet %d mismatch: got offset %d, want %d", i, p.Offset, packets[i].Offset)
			}
		}
	})

	t.Run("equal packets and requested", func(t *testing.T) {
		packets := makePackets(10)
		result := samplePackets(packets, 10)
		if len(result) != 10 {
			t.Errorf("Expected 10 packets, got %d", len(result))
		}
	})

	t.Run("normal sampling from large set", func(t *testing.T) {
		packets := makePackets(1000)
		result := samplePackets(packets, 100)

		// Should get approximately 100 samples (may be slightly less due to step rounding)
		if len(result) < 90 || len(result) > 100 {
			t.Errorf("Expected ~100 samples, got %d", len(result))
		}

		// Verify samples are in order (increasing offsets)
		for i := 1; i < len(result); i++ {
			if result[i].Offset <= result[i-1].Offset {
				t.Errorf("Samples not in order: offset[%d]=%d <= offset[%d]=%d",
					i, result[i].Offset, i-1, result[i-1].Offset)
			}
		}
	})

	t.Run("distribution check", func(t *testing.T) {
		packets := makePackets(1000)
		result := samplePackets(packets, 100)

		// Count samples from different regions
		// First 10% = offsets 0-99999 (packets 0-99)
		// Middle 80% = offsets 100000-899999 (packets 100-899)
		// Last 10% = offsets 900000+ (packets 900-999)
		var early, mid, late int
		for _, p := range result {
			idx := int(p.Offset / 1000)
			if idx < 100 {
				early++
			} else if idx >= 900 {
				late++
			} else {
				mid++
			}
		}

		// Expect roughly 25% early, 50% mid, 25% late
		// Allow for some variance due to step rounding
		t.Logf("Distribution: early=%d, mid=%d, late=%d", early, mid, late)
		if early < 15 || early > 35 {
			t.Errorf("Expected ~25 early samples, got %d", early)
		}
		if mid < 35 || mid > 60 {
			t.Errorf("Expected ~50 mid samples, got %d", mid)
		}
		if late < 15 || late > 35 {
			t.Errorf("Expected ~25 late samples, got %d", late)
		}
	})

	t.Run("single packet", func(t *testing.T) {
		packets := makePackets(1)
		result := samplePackets(packets, 10)
		if len(result) != 1 {
			t.Errorf("Expected 1 packet, got %d", len(result))
		}
	})

	t.Run("empty packets", func(t *testing.T) {
		packets := makePackets(0)
		result := samplePackets(packets, 10)
		if len(result) != 0 {
			t.Errorf("Expected 0 packets for empty input, got %d", len(result))
		}
	})

	t.Run("request single sample", func(t *testing.T) {
		packets := makePackets(100)
		result := samplePackets(packets, 1)
		// With n=1: earlyCount=0, lateCount=0, midCount=1
		// So we get 1 sample from the middle section
		if len(result) != 1 {
			t.Errorf("Expected 1 sample, got %d", len(result))
		}
	})

	t.Run("small packet count edge cases", func(t *testing.T) {
		// Test with various small packet counts
		for _, count := range []int{2, 3, 5, 9, 10, 11, 20} {
			packets := makePackets(count)
			result := samplePackets(packets, 8)
			if count <= 8 {
				if len(result) != count {
					t.Errorf("With %d packets requesting 8: expected %d, got %d",
						count, count, len(result))
				}
			} else {
				if len(result) > 8 {
					t.Errorf("With %d packets requesting 8: got %d (more than requested)",
						count, len(result))
				}
			}
		}
	})
}

func TestCalculateFileChecksum(t *testing.T) {
	t.Run("normal file", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.bin")

		// Write known content
		content := []byte("Hello, World! This is test content for checksum verification.")
		if err := os.WriteFile(testFile, content, 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		// Calculate expected checksum
		expected := xxhash.Sum64(content)

		// Test the function
		checksum, err := calculateFileChecksum(testFile)
		if err != nil {
			t.Fatalf("calculateFileChecksum failed: %v", err)
		}
		if checksum != expected {
			t.Errorf("Checksum mismatch: got 0x%x, want 0x%x", checksum, expected)
		}
	})

	t.Run("empty file", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "empty.bin")

		// Create empty file
		if err := os.WriteFile(testFile, []byte{}, 0644); err != nil {
			t.Fatalf("Failed to write empty file: %v", err)
		}

		// Calculate expected checksum for empty data
		expected := xxhash.Sum64([]byte{})

		checksum, err := calculateFileChecksum(testFile)
		if err != nil {
			t.Fatalf("calculateFileChecksum failed for empty file: %v", err)
		}
		if checksum != expected {
			t.Errorf("Empty file checksum mismatch: got 0x%x, want 0x%x", checksum, expected)
		}
	})

	t.Run("file not found", func(t *testing.T) {
		tmpDir := t.TempDir()
		missingFile := filepath.Join(tmpDir, "nonexistent.bin")

		_, err := calculateFileChecksum(missingFile)
		if err == nil {
			t.Error("Expected error for nonexistent file")
		}
	})

	t.Run("large file", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "large.bin")

		// Create a 1MB file with pattern data
		size := 1024 * 1024
		content := make([]byte, size)
		for i := range content {
			content[i] = byte(i % 256)
		}
		if err := os.WriteFile(testFile, content, 0644); err != nil {
			t.Fatalf("Failed to write large file: %v", err)
		}

		expected := xxhash.Sum64(content)

		checksum, err := calculateFileChecksum(testFile)
		if err != nil {
			t.Fatalf("calculateFileChecksum failed for large file: %v", err)
		}
		if checksum != expected {
			t.Errorf("Large file checksum mismatch: got 0x%x, want 0x%x", checksum, expected)
		}
	})

	t.Run("consistency", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "consistent.bin")

		content := []byte("Test content for consistency check")
		if err := os.WriteFile(testFile, content, 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		// Calculate twice - should get same result
		checksum1, err := calculateFileChecksum(testFile)
		if err != nil {
			t.Fatalf("First checksum failed: %v", err)
		}

		checksum2, err := calculateFileChecksum(testFile)
		if err != nil {
			t.Fatalf("Second checksum failed: %v", err)
		}

		if checksum1 != checksum2 {
			t.Errorf("Checksums not consistent: got 0x%x and 0x%x", checksum1, checksum2)
		}
	})
}

func TestProbeResult(t *testing.T) {
	// Test that ProbeResult struct has expected fields and behavior
	result := ProbeResult{
		SourcePath:   "/path/to/source",
		MatchCount:   85,
		TotalSamples: 100,
		MatchPercent: 85.0,
	}

	if result.SourcePath != "/path/to/source" {
		t.Errorf("SourcePath = %q, want %q", result.SourcePath, "/path/to/source")
	}
	if result.MatchCount != 85 {
		t.Errorf("MatchCount = %d, want 85", result.MatchCount)
	}
	if result.TotalSamples != 100 {
		t.Errorf("TotalSamples = %d, want 100", result.TotalSamples)
	}
	if result.MatchPercent != 85.0 {
		t.Errorf("MatchPercent = %f, want 85.0", result.MatchPercent)
	}
}

func TestProbeResult_ZeroValues(t *testing.T) {
	// Test zero value behavior
	var result ProbeResult

	if result.SourcePath != "" {
		t.Errorf("Zero SourcePath = %q, want empty", result.SourcePath)
	}
	if result.MatchCount != 0 {
		t.Errorf("Zero MatchCount = %d, want 0", result.MatchCount)
	}
	if result.TotalSamples != 0 {
		t.Errorf("Zero TotalSamples = %d, want 0", result.TotalSamples)
	}
	if result.MatchPercent != 0.0 {
		t.Errorf("Zero MatchPercent = %f, want 0.0", result.MatchPercent)
	}
}

func TestSamplePackets_RequestZero(t *testing.T) {
	// Edge case: requesting 0 samples
	packets := make([]mkv.Packet, 100)
	for i := range packets {
		packets[i] = mkv.Packet{Offset: int64(i * 1000)}
	}

	result := samplePackets(packets, 0)
	// When n=0, earlyCount=0, lateCount=0, midCount=0
	// So we get 0 samples
	if len(result) != 0 {
		t.Errorf("Expected 0 samples when requesting 0, got %d", len(result))
	}
}

func TestSamplePackets_RequestTwo(t *testing.T) {
	// Edge case: requesting 2 samples (minimal distribution)
	packets := make([]mkv.Packet, 100)
	for i := range packets {
		packets[i] = mkv.Packet{Offset: int64(i * 1000)}
	}

	result := samplePackets(packets, 2)
	// n=2: earlyCount=0, lateCount=0, midCount=2
	if len(result) > 2 {
		t.Errorf("Expected at most 2 samples, got %d", len(result))
	}
	if len(result) == 0 {
		t.Error("Expected at least some samples, got 0")
	}
}

func TestSamplePackets_RequestFour(t *testing.T) {
	// Edge case: requesting 4 samples (1 early, 2 mid, 1 late)
	packets := make([]mkv.Packet, 100)
	for i := range packets {
		packets[i] = mkv.Packet{Offset: int64(i * 1000)}
	}

	result := samplePackets(packets, 4)
	// n=4: earlyCount=1, lateCount=1, midCount=2
	if len(result) > 4 {
		t.Errorf("Expected at most 4 samples, got %d", len(result))
	}
	t.Logf("Got %d samples for n=4 request", len(result))
}

func TestFormatInt(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{99, "99"},
		{100, "100"},
		{999, "999"},
		{1000, "1,000"},
		{1234, "1,234"},
		{12345, "12,345"},
		{123456, "123,456"},
		{1234567, "1,234,567"},
		{1000000000, "1,000,000,000"},
		{3420000000, "3,420,000,000"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := formatInt(tt.input)
			if got != tt.expected {
				t.Errorf("formatInt(%d) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
