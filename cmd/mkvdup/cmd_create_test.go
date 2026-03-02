package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func TestHandleVerifyResult_Cleanup(t *testing.T) {
	dir := t.TempDir()

	// Create dummy dedup and config files to simulate post-write state
	dedupPath := filepath.Join(dir, "test.mkvdup")
	configPath := dedupPath + ".yaml"
	failedPath := dedupPath + ".failed"

	if err := os.WriteFile(dedupPath, []byte("dedup data"), 0644); err != nil {
		t.Fatalf("write dedup file: %v", err)
	}
	if err := os.WriteFile(configPath, []byte("name: test\n"), 0644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	// Mock verification to fail
	oldVerify := verifyReconstructionFunc
	verifyReconstructionFunc = func(_, _, _ string, _ *source.Index, _ string) error {
		return fmt.Errorf("simulated verification failure")
	}
	defer func() { verifyReconstructionFunc = oldVerify }()

	result := &createResult{OutputPath: dedupPath}

	// Call the real production helper
	captureStderr(t, func() {
		newPath := handleVerifyResult(dedupPath, "/fake/source", "/fake/mkv", nil, "Verifying...", result)

		// outputPath should be updated to .failed
		if newPath != failedPath {
			t.Errorf("expected outputPath %q, got %q", failedPath, newPath)
		}
	})

	// Verify: config file removed
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Error("config file should have been removed")
	}
	// Verify: original dedup file gone
	if _, err := os.Stat(dedupPath); !os.IsNotExist(err) {
		t.Error("original dedup file should not exist after rename")
	}
	// Verify: .failed file exists
	if _, err := os.Stat(failedPath); err != nil {
		t.Error("failed file should exist after rename")
	}
	// Verify: result fields set correctly
	if result.VerifyErr == nil {
		t.Error("VerifyErr should be set")
	}
	if result.OutputPath != failedPath {
		t.Errorf("result.OutputPath = %q, want %q", result.OutputPath, failedPath)
	}
}

func TestHandleVerifyResult_Success(t *testing.T) {
	// When verification succeeds, nothing should change
	oldVerify := verifyReconstructionFunc
	verifyReconstructionFunc = func(_, _, _ string, _ *source.Index, _ string) error {
		return nil
	}
	defer func() { verifyReconstructionFunc = oldVerify }()

	result := &createResult{OutputPath: "/data/test.mkvdup"}

	newPath := handleVerifyResult("/data/test.mkvdup", "/fake/source", "/fake/mkv", nil, "Verifying...", result)

	if newPath != "/data/test.mkvdup" {
		t.Errorf("expected unchanged outputPath, got %q", newPath)
	}
	if result.VerifyErr != nil {
		t.Errorf("VerifyErr should be nil on success, got %v", result.VerifyErr)
	}
	if result.OutputPath != "/data/test.mkvdup" {
		t.Errorf("OutputPath should be unchanged, got %q", result.OutputPath)
	}
}

func TestHandleVerifyResult_OverwritesExistingFailed(t *testing.T) {
	dir := t.TempDir()

	dedupPath := filepath.Join(dir, "test.mkvdup")
	failedPath := dedupPath + ".failed"

	// Create both dedup and a pre-existing .failed file
	if err := os.WriteFile(dedupPath, []byte("new dedup data"), 0644); err != nil {
		t.Fatalf("write dedup file: %v", err)
	}
	if err := os.WriteFile(failedPath, []byte("old failed data"), 0644); err != nil {
		t.Fatalf("write failed file: %v", err)
	}

	oldVerify := verifyReconstructionFunc
	verifyReconstructionFunc = func(_, _, _ string, _ *source.Index, _ string) error {
		return fmt.Errorf("simulated failure")
	}
	defer func() { verifyReconstructionFunc = oldVerify }()

	result := &createResult{OutputPath: dedupPath}

	captureStderr(t, func() {
		handleVerifyResult(dedupPath, "/fake/source", "/fake/mkv", nil, "Verifying...", result)
	})

	// .failed should contain the new data, not the old
	data, err := os.ReadFile(failedPath)
	if err != nil {
		t.Fatalf("read failed file: %v", err)
	}
	if string(data) != "new dedup data" {
		t.Errorf("expected .failed to contain new data, got %q", string(data))
	}
}

func TestVerifyFailureResult(t *testing.T) {
	// Test that a result with VerifyErr is not counted as a success
	// using the same predicate as printBatchSummary and the exit-code logic.
	failedResult := &createResult{
		MkvPath:   "/data/test.mkv",
		VerifyErr: fmt.Errorf("data mismatch at offset 1024"),
	}
	successResult := &createResult{
		MkvPath: "/data/ok.mkv",
	}
	results := []*createResult{failedResult, successResult}

	successes := 0
	for _, r := range results {
		if !r.Skipped && r.Err == nil && r.VerifyErr == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Errorf("expected 1 success, got %d", successes)
	}

	// VerifyErr should be distinct from Err
	if failedResult.Err != nil {
		t.Error("Err should be nil when only VerifyErr is set")
	}
}

func TestReportCodecMismatches_SkipAction(t *testing.T) {
	mismatches := []source.CodecMismatch{
		{
			TrackType:    "video",
			MKVCodecID:   "V_MPEG4/ISO/AVC",
			MKVCodecType: source.CodecH264Video,
			SourceCodecs: []source.CodecType{source.CodecMPEG2Video},
		},
	}

	stderr := captureStderr(t, func() {
		err := reportCodecMismatches(mismatches, codecMismatchSkip)
		if err != nil {
			t.Errorf("expected no error for skip action, got: %v", err)
		}
	})

	if !strings.Contains(stderr, "WARNING: Codec mismatch detected") {
		t.Error("expected mismatch warning in stderr")
	}
	if !strings.Contains(stderr, "Skipping (--skip-codec-mismatch)") {
		t.Errorf("expected skip message in stderr, got:\n%s", stderr)
	}
	if strings.Contains(stderr, "Continuing") {
		t.Error("skip action should not print 'Continuing'")
	}
}

func TestReportCodecMismatches_ContinueAction(t *testing.T) {
	mismatches := []source.CodecMismatch{
		{
			TrackType:    "video",
			MKVCodecID:   "V_MPEG4/ISO/AVC",
			MKVCodecType: source.CodecH264Video,
			SourceCodecs: []source.CodecType{source.CodecMPEG2Video},
		},
	}

	stderr := captureStderr(t, func() {
		err := reportCodecMismatches(mismatches, codecMismatchContinue)
		if err != nil {
			t.Errorf("expected no error for continue action, got: %v", err)
		}
	})

	if !strings.Contains(stderr, "WARNING: Codec mismatch detected") {
		t.Error("expected mismatch warning in stderr")
	}
	if !strings.Contains(stderr, "Continuing (non-interactive mode)") {
		t.Errorf("expected continue message in stderr, got:\n%s", stderr)
	}
	if strings.Contains(stderr, "Skipping") {
		t.Error("continue action should not print 'Skipping'")
	}
}

func TestReportCodecMismatches_NoMismatches(t *testing.T) {
	stderr := captureStderr(t, func() {
		err := reportCodecMismatches(nil, codecMismatchSkip)
		if err != nil {
			t.Errorf("expected no error for empty mismatches, got: %v", err)
		}
	})

	if stderr != "" {
		t.Errorf("expected no output for empty mismatches, got: %q", stderr)
	}
}
