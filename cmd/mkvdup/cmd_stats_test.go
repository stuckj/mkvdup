package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestShowStats_SingleFile(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "source")
	dedupPath := filepath.Join(dir, "movie.mkvdup")

	createTestDedupFile(t, dedupPath, sourceDir)
	writeTestYAML(t, filepath.Join(dir, "config.yaml"), fmt.Sprintf(`name: "movie.mkv"
dedup_file: %q
source_dir: %q
`, dedupPath, sourceDir))

	output := captureStdout(t, func() {
		err := showStats([]string{filepath.Join(dir, "config.yaml")}, false)
		if err != nil {
			t.Errorf("showStats error: %v", err)
		}
	})

	// Check per-file stats appear
	for _, want := range []string{
		"movie.mkv",
		"Original size:",
		"Dedup file size:",
		"Space savings:",
		"Source type:",
		"DVD",
		"Source directory:",
		sourceDir,
		"Source files:",
		"Index entries:",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q\n\nFull output:\n%s", want, output)
		}
	}

	// No rollup for single file
	if strings.Contains(output, "Totals") {
		t.Errorf("single file should not have rollup section\n\nFull output:\n%s", output)
	}
}

func TestShowStats_MultipleFiles(t *testing.T) {
	dir := t.TempDir()
	sourceDir1 := filepath.Join(dir, "source1")
	sourceDir2 := filepath.Join(dir, "source2")
	dedupPath1 := filepath.Join(dir, "movie1.mkvdup")
	dedupPath2 := filepath.Join(dir, "movie2.mkvdup")

	createTestDedupFile(t, dedupPath1, sourceDir1)
	createTestDedupFile(t, dedupPath2, sourceDir2)
	writeTestYAML(t, filepath.Join(dir, "config1.yaml"), fmt.Sprintf(`name: "movie1.mkv"
dedup_file: %q
source_dir: %q
`, dedupPath1, sourceDir1))
	writeTestYAML(t, filepath.Join(dir, "config2.yaml"), fmt.Sprintf(`name: "movie2.mkv"
dedup_file: %q
source_dir: %q
`, dedupPath2, sourceDir2))

	output := captureStdout(t, func() {
		err := showStats([]string{
			filepath.Join(dir, "config1.yaml"),
			filepath.Join(dir, "config2.yaml"),
		}, false)
		if err != nil {
			t.Errorf("showStats error: %v", err)
		}
	})

	// Both files should appear
	if !strings.Contains(output, "movie1.mkv") {
		t.Errorf("output missing movie1.mkv\n\nFull output:\n%s", output)
	}
	if !strings.Contains(output, "movie2.mkv") {
		t.Errorf("output missing movie2.mkv\n\nFull output:\n%s", output)
	}

	// Rollup should appear for multiple files
	if !strings.Contains(output, "Totals (2 files):") {
		t.Errorf("output missing rollup section\n\nFull output:\n%s", output)
	}
	if !strings.Contains(output, "Unique sources:") {
		t.Errorf("output missing unique sources\n\nFull output:\n%s", output)
	}
}

func TestShowStats_ConfigDir(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "configs")
	sourceDir := filepath.Join(dir, "source")
	dedupPath := filepath.Join(dir, "movie.mkvdup")

	createTestDedupFile(t, dedupPath, sourceDir)
	writeTestYAML(t, filepath.Join(configDir, "movie.yaml"), fmt.Sprintf(`name: "movie.mkv"
dedup_file: %q
source_dir: %q
`, dedupPath, sourceDir))

	output := captureStdout(t, func() {
		err := showStats([]string{configDir}, true)
		if err != nil {
			t.Errorf("showStats error: %v", err)
		}
	})

	if !strings.Contains(output, "movie.mkv") {
		t.Errorf("output missing movie.mkv\n\nFull output:\n%s", output)
	}
}

func TestShowStats_InvalidDedupFile(t *testing.T) {
	dir := t.TempDir()
	writeTestYAML(t, filepath.Join(dir, "config.yaml"), fmt.Sprintf(`name: "movie.mkv"
dedup_file: %q
source_dir: %q
`, filepath.Join(dir, "nonexistent.mkvdup"), dir))

	stderr := captureStderr(t, func() {
		err := showStats([]string{filepath.Join(dir, "config.yaml")}, false)
		if err != nil {
			t.Errorf("showStats should not return error for individual file failures: %v", err)
		}
	})

	if !strings.Contains(stderr, "Error:") {
		t.Errorf("stderr should contain error for invalid dedup file\n\nStderr:\n%s", stderr)
	}
}

func TestShowStats_ContinuesOnError(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "source")
	dedupPath := filepath.Join(dir, "movie.mkvdup")

	createTestDedupFile(t, dedupPath, sourceDir)

	// First config: invalid (nonexistent dedup file)
	writeTestYAML(t, filepath.Join(dir, "bad.yaml"), fmt.Sprintf(`name: "bad.mkv"
dedup_file: %q
source_dir: %q
`, filepath.Join(dir, "nonexistent.mkvdup"), dir))

	// Second config: valid
	writeTestYAML(t, filepath.Join(dir, "good.yaml"), fmt.Sprintf(`name: "good.mkv"
dedup_file: %q
source_dir: %q
`, dedupPath, sourceDir))

	output := captureStdout(t, func() {
		stderr := captureStderr(t, func() {
			err := showStats([]string{
				filepath.Join(dir, "bad.yaml"),
				filepath.Join(dir, "good.yaml"),
			}, false)
			if err != nil {
				t.Errorf("showStats should not return error: %v", err)
			}
		})
		// Error for bad file should be on stderr
		if !strings.Contains(stderr, "bad.mkv") {
			t.Errorf("stderr should mention bad.mkv\n\nStderr:\n%s", stderr)
		}
	})

	// Good file should still appear
	if !strings.Contains(output, "good.mkv") {
		t.Errorf("output should contain good.mkv stats\n\nFull output:\n%s", output)
	}

	// Rollup should not appear with only 1 successful file
	if strings.Contains(output, "Totals") {
		t.Errorf("rollup should not appear with only 1 successful file\n\nFull output:\n%s", output)
	}
}
