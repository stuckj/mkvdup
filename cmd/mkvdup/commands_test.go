package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/stuckj/mkvdup/internal/dedup"
	"github.com/stuckj/mkvdup/internal/matcher"
	"github.com/stuckj/mkvdup/internal/mkv"
	"github.com/stuckj/mkvdup/internal/source"
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

// writeTestYAML writes a YAML config file with the given content.
func writeTestYAML(t *testing.T, path, content string) {
	t.Helper()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// createTestDedupFile creates a minimal valid .mkvdup file for testing.
func createTestDedupFile(t *testing.T, dedupPath, sourceDir string) {
	t.Helper()

	// Create a source file for the dedup to reference
	srcFile := filepath.Join(sourceDir, "test.vob")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	if err := os.WriteFile(srcFile, []byte("source data"), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	writer, err := dedup.NewWriter(dedupPath)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}

	writer.SetHeader(100, 0x1234, source.TypeDVD)
	srcContent := []byte("source data")
	writer.SetSourceFiles([]source.File{
		{RelativePath: "test.vob", Size: int64(len(srcContent)), Checksum: xxhash.Sum64(srcContent)},
	})

	// Set an empty match result (no entries, no delta)
	result := &matcher.Result{
		Entries:        nil,
		DeltaData:      []byte("delta"),
		MatchedBytes:   50,
		UnmatchedBytes: 50,
		MatchedPackets: 1,
		TotalPackets:   1,
	}
	if err := writer.SetMatchResult(result, nil); err != nil {
		writer.Close()
		t.Fatalf("SetMatchResult: %v", err)
	}
	if err := writer.Write(); err != nil {
		writer.Close()
		t.Fatalf("Write: %v", err)
	}
	writer.Close()
}

func TestValidateConfigs_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "source")
	dedupPath := filepath.Join(dir, "movie.mkvdup")

	createTestDedupFile(t, dedupPath, sourceDir)
	writeTestYAML(t, filepath.Join(dir, "config.yaml"), fmt.Sprintf(`name: "movie.mkv"
dedup_file: %q
source_dir: %q
`, dedupPath, sourceDir))

	exitCode := validateConfigs([]string{filepath.Join(dir, "config.yaml")}, false, false, false)
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
}

func TestValidateConfigs_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	writeTestYAML(t, filepath.Join(dir, "bad.yaml"), `name: "test.mkv"
dedup_file: "test.mkvdup"
source_dir: [invalid yaml`)

	exitCode := validateConfigs([]string{filepath.Join(dir, "bad.yaml")}, false, false, false)
	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
}

func TestValidateConfigs_MissingFields(t *testing.T) {
	dir := t.TempDir()
	writeTestYAML(t, filepath.Join(dir, "partial.yaml"), `name: "test.mkv"
dedup_file: "/tmp/test.mkvdup"
`)

	exitCode := validateConfigs([]string{filepath.Join(dir, "partial.yaml")}, false, false, false)
	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
}

func TestValidateConfigs_MissingDedupFile(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "source")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeTestYAML(t, filepath.Join(dir, "config.yaml"), fmt.Sprintf(`name: "movie.mkv"
dedup_file: %q
source_dir: %q
`, filepath.Join(dir, "nonexistent.mkvdup"), sourceDir))

	exitCode := validateConfigs([]string{filepath.Join(dir, "config.yaml")}, false, false, false)
	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
}

func TestValidateConfigs_MissingSourceDir(t *testing.T) {
	dir := t.TempDir()
	dedupPath := filepath.Join(dir, "movie.mkvdup")
	sourceDir := filepath.Join(dir, "source")

	// Create dedup file but not source dir
	createTestDedupFile(t, dedupPath, sourceDir)
	os.RemoveAll(sourceDir) // Remove the source dir created by helper

	writeTestYAML(t, filepath.Join(dir, "config.yaml"), fmt.Sprintf(`name: "movie.mkv"
dedup_file: %q
source_dir: %q
`, dedupPath, filepath.Join(dir, "nonexistent_source")))

	exitCode := validateConfigs([]string{filepath.Join(dir, "config.yaml")}, false, false, false)
	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
}

func TestValidateConfigs_SourceDirIsFile(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "source")
	dedupPath := filepath.Join(dir, "movie.mkvdup")

	createTestDedupFile(t, dedupPath, sourceDir)
	os.RemoveAll(sourceDir)
	// Create a file where source dir should be
	fakeSrcPath := filepath.Join(dir, "fake_source")
	if err := os.WriteFile(fakeSrcPath, []byte("not a dir"), 0644); err != nil {
		t.Fatalf("write fake source: %v", err)
	}

	writeTestYAML(t, filepath.Join(dir, "config.yaml"), fmt.Sprintf(`name: "movie.mkv"
dedup_file: %q
source_dir: %q
`, dedupPath, fakeSrcPath))

	exitCode := validateConfigs([]string{filepath.Join(dir, "config.yaml")}, false, false, false)
	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
}

func TestValidateConfigs_DuplicateNames(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "source")
	dedupPath1 := filepath.Join(dir, "movie1.mkvdup")
	dedupPath2 := filepath.Join(dir, "movie2.mkvdup")

	createTestDedupFile(t, dedupPath1, sourceDir)
	createTestDedupFile(t, dedupPath2, sourceDir)

	writeTestYAML(t, filepath.Join(dir, "config1.yaml"), fmt.Sprintf(`name: "movie.mkv"
dedup_file: %q
source_dir: %q
`, dedupPath1, sourceDir))

	writeTestYAML(t, filepath.Join(dir, "config2.yaml"), fmt.Sprintf(`name: "movie.mkv"
dedup_file: %q
source_dir: %q
`, dedupPath2, sourceDir))

	// Without strict: warnings are OK, exit 0
	exitCode := validateConfigs([]string{
		filepath.Join(dir, "config1.yaml"),
		filepath.Join(dir, "config2.yaml"),
	}, false, false, false)
	if exitCode != 0 {
		t.Errorf("expected exit code 0 without strict, got %d", exitCode)
	}
}

func TestValidateConfigs_DuplicateNamesStrict(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "source")
	dedupPath1 := filepath.Join(dir, "movie1.mkvdup")
	dedupPath2 := filepath.Join(dir, "movie2.mkvdup")

	createTestDedupFile(t, dedupPath1, sourceDir)
	createTestDedupFile(t, dedupPath2, sourceDir)

	writeTestYAML(t, filepath.Join(dir, "config1.yaml"), fmt.Sprintf(`name: "movie.mkv"
dedup_file: %q
source_dir: %q
`, dedupPath1, sourceDir))

	writeTestYAML(t, filepath.Join(dir, "config2.yaml"), fmt.Sprintf(`name: "movie.mkv"
dedup_file: %q
source_dir: %q
`, dedupPath2, sourceDir))

	// With strict: warnings become errors, exit 1
	exitCode := validateConfigs([]string{
		filepath.Join(dir, "config1.yaml"),
		filepath.Join(dir, "config2.yaml"),
	}, false, false, true)
	if exitCode != 1 {
		t.Errorf("expected exit code 1 with strict, got %d", exitCode)
	}
}

func TestValidateConfigs_PathConflict(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "source")
	dedupPath1 := filepath.Join(dir, "movie1.mkvdup")
	dedupPath2 := filepath.Join(dir, "movie2.mkvdup")

	createTestDedupFile(t, dedupPath1, sourceDir)
	createTestDedupFile(t, dedupPath2, sourceDir)

	// First config uses "Movies" as a file name
	writeTestYAML(t, filepath.Join(dir, "config1.yaml"), fmt.Sprintf(`name: "Movies"
dedup_file: %q
source_dir: %q
`, dedupPath1, sourceDir))

	// Second config uses "Movies" as a directory component
	writeTestYAML(t, filepath.Join(dir, "config2.yaml"), fmt.Sprintf(`name: "Movies/Action/video.mkv"
dedup_file: %q
source_dir: %q
`, dedupPath2, sourceDir))

	exitCode := validateConfigs([]string{
		filepath.Join(dir, "config1.yaml"),
		filepath.Join(dir, "config2.yaml"),
	}, false, false, false)
	// Conflict is a warning, exit 0 without strict
	if exitCode != 0 {
		t.Errorf("expected exit code 0 without strict, got %d", exitCode)
	}
}

func TestValidateConfigs_InvalidPathDotDot(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "source")
	dedupPath := filepath.Join(dir, "movie.mkvdup")

	createTestDedupFile(t, dedupPath, sourceDir)

	writeTestYAML(t, filepath.Join(dir, "config.yaml"), fmt.Sprintf(`name: "../escape.mkv"
dedup_file: %q
source_dir: %q
`, dedupPath, sourceDir))

	exitCode := validateConfigs([]string{filepath.Join(dir, "config.yaml")}, false, false, false)
	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
}

func TestValidateConfigs_EmptyName(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "source")
	dedupPath := filepath.Join(dir, "movie.mkvdup")

	createTestDedupFile(t, dedupPath, sourceDir)

	// Create a config with virtual_files that has an empty-ish name
	writeTestYAML(t, filepath.Join(dir, "config.yaml"), fmt.Sprintf(`virtual_files:
  - name: "/"
    dedup_file: %q
    source_dir: %q
`, dedupPath, sourceDir))

	exitCode := validateConfigs([]string{filepath.Join(dir, "config.yaml")}, false, false, false)
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for empty name, got %d", exitCode)
	}
}

func TestValidateConfigs_ConfigDir(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "source")
	confDir := filepath.Join(dir, "configs")

	dedupPath1 := filepath.Join(dir, "m1.mkvdup")
	dedupPath2 := filepath.Join(dir, "m2.mkvdup")
	createTestDedupFile(t, dedupPath1, sourceDir)
	createTestDedupFile(t, dedupPath2, sourceDir)

	writeTestYAML(t, filepath.Join(confDir, "a.yaml"), fmt.Sprintf(`name: "a.mkv"
dedup_file: %q
source_dir: %q
`, dedupPath1, sourceDir))

	writeTestYAML(t, filepath.Join(confDir, "b.yaml"), fmt.Sprintf(`name: "b.mkv"
dedup_file: %q
source_dir: %q
`, dedupPath2, sourceDir))

	exitCode := validateConfigs([]string{confDir}, true, false, false)
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
}

func TestValidateConfigs_IncludesAndVirtualFiles(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "source")
	dedupPath1 := filepath.Join(dir, "m1.mkvdup")
	dedupPath2 := filepath.Join(dir, "m2.mkvdup")
	createTestDedupFile(t, dedupPath1, sourceDir)
	createTestDedupFile(t, dedupPath2, sourceDir)

	// Child config
	writeTestYAML(t, filepath.Join(dir, "child.mkvdup.yaml"), fmt.Sprintf(`name: "child.mkv"
dedup_file: %q
source_dir: %q
`, dedupPath1, sourceDir))

	// Parent config with includes and virtual_files
	writeTestYAML(t, filepath.Join(dir, "parent.yaml"), fmt.Sprintf(`includes:
  - %q
virtual_files:
  - name: "inline.mkv"
    dedup_file: %q
    source_dir: %q
`, filepath.Join(dir, "child.mkvdup.yaml"), dedupPath2, sourceDir))

	exitCode := validateConfigs([]string{filepath.Join(dir, "parent.yaml")}, false, false, false)
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
}

func TestValidateConfigs_CycleDetection(t *testing.T) {
	dir := t.TempDir()

	// Create two configs that include each other
	writeTestYAML(t, filepath.Join(dir, "a.yaml"), fmt.Sprintf(`includes:
  - %q
`, filepath.Join(dir, "b.yaml")))

	writeTestYAML(t, filepath.Join(dir, "b.yaml"), fmt.Sprintf(`includes:
  - %q
`, filepath.Join(dir, "a.yaml")))

	// Should not hang — cycle detection handles it
	exitCode := validateConfigs([]string{filepath.Join(dir, "a.yaml")}, false, false, false)
	// No entries, no errors — just empty result
	if exitCode != 0 {
		t.Errorf("expected exit code 0 (no entries), got %d", exitCode)
	}
}

func TestValidateConfigs_DeepValid(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "source")
	dedupPath := filepath.Join(dir, "movie.mkvdup")

	createTestDedupFile(t, dedupPath, sourceDir)
	writeTestYAML(t, filepath.Join(dir, "config.yaml"), fmt.Sprintf(`name: "movie.mkv"
dedup_file: %q
source_dir: %q
`, dedupPath, sourceDir))

	exitCode := validateConfigs([]string{filepath.Join(dir, "config.yaml")}, false, true, false)
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
}

func TestValidateConfigs_DeepCorrupt(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "source")
	dedupPath := filepath.Join(dir, "movie.mkvdup")

	createTestDedupFile(t, dedupPath, sourceDir)

	// Corrupt the dedup file by overwriting bytes in the middle
	data, err := os.ReadFile(dedupPath)
	if err != nil {
		t.Fatal(err)
	}
	// Corrupt near the footer area (checksum will mismatch)
	if len(data) > 30 {
		for i := len(data) - 30; i < len(data)-24; i++ {
			data[i] ^= 0xFF
		}
	}
	if err := os.WriteFile(dedupPath, data, 0644); err != nil {
		t.Fatalf("write corrupt dedup: %v", err)
	}

	writeTestYAML(t, filepath.Join(dir, "config.yaml"), fmt.Sprintf(`name: "movie.mkv"
dedup_file: %q
source_dir: %q
`, dedupPath, sourceDir))

	exitCode := validateConfigs([]string{filepath.Join(dir, "config.yaml")}, false, true, false)
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for corrupt dedup, got %d", exitCode)
	}
}

func TestCleanVirtualPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"movie.mkv", "movie.mkv"},
		{"Movies/Action/movie.mkv", "Movies/Action/movie.mkv"},
		{"/leading/slash.mkv", "leading/slash.mkv"},
		{"./relative/path.mkv", "relative/path.mkv"},
		{"foo//bar.mkv", "foo/bar.mkv"},
		{"trailing/", "trailing"},
		{"", ""},
		{"/", ""},
		{".", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := cleanVirtualPath(tt.input)
			if got != tt.expected {
				t.Errorf("cleanVirtualPath(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestExpandConfigDir(t *testing.T) {
	dir := t.TempDir()

	// Create some yaml files
	if err := os.WriteFile(filepath.Join(dir, "a.yaml"), []byte(""), 0644); err != nil {
		t.Fatalf("write a.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.yml"), []byte(""), 0644); err != nil {
		t.Fatalf("write b.yml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "c.txt"), []byte(""), 0644); err != nil {
		t.Fatalf("write c.txt: %v", err)
	}

	paths, err := expandConfigDir(dir)
	if err != nil {
		t.Fatalf("expandConfigDir: %v", err)
	}
	if len(paths) != 2 {
		t.Errorf("expected 2 yaml files, got %d: %v", len(paths), paths)
	}
}

func TestExpandConfigDir_Empty(t *testing.T) {
	dir := t.TempDir()

	_, err := expandConfigDir(dir)
	if err == nil {
		t.Error("expected error for empty directory")
	}
}

func TestExpandConfigDir_NotExist(t *testing.T) {
	_, err := expandConfigDir("/nonexistent/path")
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

// --- check command tests ---

func TestCheckDedup_Valid(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "source")
	dedupPath := filepath.Join(dir, "movie.mkvdup")

	createTestDedupFile(t, dedupPath, sourceDir)

	if err := checkDedup(dedupPath, sourceDir, false); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestCheckDedup_ValidWithChecksums(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "source")
	dedupPath := filepath.Join(dir, "movie.mkvdup")

	createTestDedupFile(t, dedupPath, sourceDir)

	if err := checkDedup(dedupPath, sourceDir, true); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestCheckDedup_InvalidDedupFile(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "source")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	err := checkDedup(filepath.Join(dir, "nonexistent.mkvdup"), sourceDir, false)
	if err == nil {
		t.Error("expected error for nonexistent dedup file")
	}
}

func TestCheckDedup_CorruptDedupFile(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "source")
	dedupPath := filepath.Join(dir, "movie.mkvdup")

	createTestDedupFile(t, dedupPath, sourceDir)

	// Corrupt the dedup file by flipping bytes in the delta section
	// (just before the 24-byte footer at the end)
	data, err := os.ReadFile(dedupPath)
	if err != nil {
		t.Fatalf("read dedup: %v", err)
	}
	// The footer is the last 24 bytes; corrupt delta data just before it
	if len(data) > 30 {
		idx := len(data) - 25
		data[idx] ^= 0xFF
	}
	if err := os.WriteFile(dedupPath, data, 0644); err != nil {
		t.Fatalf("write corrupt dedup: %v", err)
	}

	err = checkDedup(dedupPath, sourceDir, false)
	if err == nil {
		t.Error("expected error for corrupt dedup file")
	}
}

func TestCheckDedup_MissingSourceFile(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "source")
	dedupPath := filepath.Join(dir, "movie.mkvdup")

	createTestDedupFile(t, dedupPath, sourceDir)

	// Delete the source file
	if err := os.Remove(filepath.Join(sourceDir, "test.vob")); err != nil {
		t.Fatalf("remove source: %v", err)
	}

	err := checkDedup(dedupPath, sourceDir, false)
	if err == nil {
		t.Error("expected error for missing source file")
	}
}

func TestCheckDedup_WrongSourceSize(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "source")
	dedupPath := filepath.Join(dir, "movie.mkvdup")

	createTestDedupFile(t, dedupPath, sourceDir)

	// Truncate the source file
	if err := os.WriteFile(filepath.Join(sourceDir, "test.vob"), []byte("short"), 0644); err != nil {
		t.Fatalf("write truncated source: %v", err)
	}

	err := checkDedup(dedupPath, sourceDir, false)
	if err == nil {
		t.Error("expected error for wrong source size")
	}
}

func TestCheckDedup_WrongSourceChecksum(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "source")
	dedupPath := filepath.Join(dir, "movie.mkvdup")

	createTestDedupFile(t, dedupPath, sourceDir)

	// Overwrite source with different content of the same length
	if err := os.WriteFile(filepath.Join(sourceDir, "test.vob"), []byte("source XXXX"), 0644); err != nil {
		t.Fatalf("write modified source: %v", err)
	}

	err := checkDedup(dedupPath, sourceDir, true)
	if err == nil {
		t.Error("expected error for wrong source checksum")
	}
}

func TestCheckDedup_SkipsChecksumOnSizeError(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "source")
	dedupPath := filepath.Join(dir, "movie.mkvdup")

	createTestDedupFile(t, dedupPath, sourceDir)

	// Truncate the source file (wrong size)
	if err := os.WriteFile(filepath.Join(sourceDir, "test.vob"), []byte("short"), 0644); err != nil {
		t.Fatalf("write truncated source: %v", err)
	}

	// Even with sourceChecksums=true, should report size error (not checksum error)
	err := checkDedup(dedupPath, sourceDir, true)
	if err == nil {
		t.Error("expected error for wrong source size")
	}
}

// --- batch-create command tests ---

func TestCreateBatch_InvalidManifest(t *testing.T) {
	err := createBatch("/nonexistent/batch.yaml")
	if err == nil {
		t.Error("expected error for nonexistent manifest")
	}
}

func TestCreateBatch_BadSourceDir(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "batch.yaml")
	writeTestYAML(t, manifestPath, `source_dir: /nonexistent/source/dir
files:
  - mkv: /nonexistent/ep1.mkv
`)

	err := createBatch(manifestPath)
	if err == nil {
		t.Error("expected error for nonexistent source directory")
	}
}

func TestCreateBatch_EmptyManifest(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "batch.yaml")
	writeTestYAML(t, manifestPath, `source_dir: /tmp
files: []
`)

	err := createBatch(manifestPath)
	if err == nil {
		t.Error("expected error for empty files list")
	}
}

func TestPrintBatchSummary(t *testing.T) {
	results := []*createResult{
		{
			MkvPath:    "/data/ep1.mkv",
			OutputPath: "/data/ep1.mkvdup",
			Savings:    98.5,
		},
		{
			MkvPath: "/data/ep2.mkv",
			Err:     fmt.Errorf("file not found"),
		},
		{
			MkvPath:    "/data/ep3.mkv",
			OutputPath: "/data/ep3.mkvdup",
			Savings:    97.2,
		},
	}

	// Just verify it doesn't panic
	printBatchSummary(results, 5*time.Second, time.Now().Add(-10*time.Second))
}
