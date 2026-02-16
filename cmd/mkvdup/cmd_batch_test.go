package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stuckj/mkvdup/internal/dedup"
)

func TestGroupBySource(t *testing.T) {
	t.Run("single source", func(t *testing.T) {
		files := []dedup.BatchManifestFile{
			{MKV: "a.mkv", SourceDir: "/src/disc1"},
			{MKV: "b.mkv", SourceDir: "/src/disc1"},
			{MKV: "c.mkv", SourceDir: "/src/disc1"},
		}
		groups := groupBySource(files)
		if len(groups) != 1 {
			t.Fatalf("got %d groups, want 1", len(groups))
		}
		if groups[0].sourceDir != "/src/disc1" {
			t.Errorf("sourceDir = %q, want %q", groups[0].sourceDir, "/src/disc1")
		}
		if !slices.Equal(groups[0].indices, []int{0, 1, 2}) {
			t.Errorf("indices = %v, want [0 1 2]", groups[0].indices)
		}
	})

	t.Run("multiple sources in order", func(t *testing.T) {
		files := []dedup.BatchManifestFile{
			{MKV: "a.mkv", SourceDir: "/src/disc1"},
			{MKV: "b.mkv", SourceDir: "/src/disc2"},
			{MKV: "c.mkv", SourceDir: "/src/disc3"},
		}
		groups := groupBySource(files)
		if len(groups) != 3 {
			t.Fatalf("got %d groups, want 3", len(groups))
		}
		for i, want := range []string{"/src/disc1", "/src/disc2", "/src/disc3"} {
			if groups[i].sourceDir != want {
				t.Errorf("groups[%d].sourceDir = %q, want %q", i, groups[i].sourceDir, want)
			}
			if !slices.Equal(groups[i].indices, []int{i}) {
				t.Errorf("groups[%d].indices = %v, want [%d]", i, groups[i].indices, i)
			}
		}
	})

	t.Run("non-consecutive same source merged", func(t *testing.T) {
		files := []dedup.BatchManifestFile{
			{MKV: "a.mkv", SourceDir: "/src/disc1"}, // 0
			{MKV: "b.mkv", SourceDir: "/src/disc2"}, // 1
			{MKV: "c.mkv", SourceDir: "/src/disc1"}, // 2
			{MKV: "d.mkv", SourceDir: "/src/disc3"}, // 3
			{MKV: "e.mkv", SourceDir: "/src/disc2"}, // 4
		}
		groups := groupBySource(files)
		if len(groups) != 3 {
			t.Fatalf("got %d groups, want 3", len(groups))
		}
		// Groups in first-seen order
		if groups[0].sourceDir != "/src/disc1" || !slices.Equal(groups[0].indices, []int{0, 2}) {
			t.Errorf("group 0: sourceDir=%q indices=%v, want disc1 [0,2]", groups[0].sourceDir, groups[0].indices)
		}
		if groups[1].sourceDir != "/src/disc2" || !slices.Equal(groups[1].indices, []int{1, 4}) {
			t.Errorf("group 1: sourceDir=%q indices=%v, want disc2 [1,4]", groups[1].sourceDir, groups[1].indices)
		}
		if groups[2].sourceDir != "/src/disc3" || !slices.Equal(groups[2].indices, []int{3}) {
			t.Errorf("group 2: sourceDir=%q indices=%v, want disc3 [3]", groups[2].sourceDir, groups[2].indices)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		groups := groupBySource(nil)
		if len(groups) != 0 {
			t.Errorf("got %d groups, want 0", len(groups))
		}
	})
}

// --- batch-create command tests ---

func TestCreateBatch_InvalidManifest(t *testing.T) {
	err := createBatch("/nonexistent/batch.yaml", 75.0, false)
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

	err := createBatch(manifestPath, 75.0, false)
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

	err := createBatch(manifestPath, 75.0, false)
	if err == nil {
		t.Error("expected error for empty files list")
	}
}

func TestCreateBatch_PerFileSourceDir(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "batch.yaml")
	writeTestYAML(t, manifestPath, fmt.Sprintf(`files:
  - mkv: %s/ep1.mkv
    output: %s/ep1.mkvdup
    source_dir: %s/source1
  - mkv: %s/ep2.mkv
    output: %s/ep2.mkvdup
    source_dir: %s/source2
`, dir, dir, dir, dir, dir, dir))

	// Create the source directories (empty — indexing will fail)
	os.MkdirAll(filepath.Join(dir, "source1"), 0755)
	os.MkdirAll(filepath.Join(dir, "source2"), 0755)

	err := createBatch(manifestPath, 75.0, false)
	// Should fail (sources exist but have no media to index)
	if err == nil {
		t.Error("expected error for empty source directories")
	}
}

func TestCreateBatch_MultiSource_ContinuesOnIndexFailure(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "batch.yaml")
	writeTestYAML(t, manifestPath, fmt.Sprintf(`files:
  - mkv: %s/ep1.mkv
    output: %s/ep1.mkvdup
    source_dir: %s/source1
  - mkv: %s/ep2.mkv
    output: %s/ep2.mkvdup
    source_dir: %s/source2
`, dir, dir, dir, dir, dir, dir))

	// Create both source dirs (empty — indexing will fail for both)
	os.MkdirAll(filepath.Join(dir, "source1"), 0755)
	os.MkdirAll(filepath.Join(dir, "source2"), 0755)

	// Capture stderr to verify both sources were attempted
	stderrOutput := captureStderr(t, func() {
		createBatch(manifestPath, 75.0, false)
	})

	// Both source directories should appear in error output
	if !strings.Contains(stderrOutput, "source1") {
		t.Errorf("stderr missing source1 error, got: %s", stderrOutput)
	}
	if !strings.Contains(stderrOutput, "source2") {
		t.Errorf("stderr missing source2 error, got: %s", stderrOutput)
	}
}

func TestCreateBatch_MultiSource_OutputHeaders(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "batch.yaml")
	writeTestYAML(t, manifestPath, fmt.Sprintf(`files:
  - mkv: %s/ep1.mkv
    output: %s/ep1.mkvdup
    source_dir: %s/source1
  - mkv: %s/ep2.mkv
    output: %s/ep2.mkvdup
    source_dir: %s/source2
`, dir, dir, dir, dir, dir, dir))

	os.MkdirAll(filepath.Join(dir, "source1"), 0755)
	os.MkdirAll(filepath.Join(dir, "source2"), 0755)

	output := captureStdout(t, func() {
		createBatch(manifestPath, 75.0, false)
	})

	// Should show multi-source header
	if !strings.Contains(output, "2 sources") {
		t.Errorf("output missing '2 sources' header, got: %s", output)
	}
	// Should show source group separators
	if !strings.Contains(output, "Source 1/2") {
		t.Errorf("output missing 'Source 1/2' separator, got: %s", output)
	}
	if !strings.Contains(output, "Source 2/2") {
		t.Errorf("output missing 'Source 2/2' separator, got: %s", output)
	}
}

func TestCreateBatch_SingleSource_NoGroupHeaders(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "batch.yaml")
	writeTestYAML(t, manifestPath, fmt.Sprintf(`source_dir: %s/source
files:
  - mkv: %s/ep1.mkv
    output: %s/ep1.mkvdup
  - mkv: %s/ep2.mkv
    output: %s/ep2.mkvdup
`, dir, dir, dir, dir, dir))

	os.MkdirAll(filepath.Join(dir, "source"), 0755)

	output := captureStdout(t, func() {
		createBatch(manifestPath, 75.0, false)
	})

	// Single source should NOT show source group separators
	if strings.Contains(output, "Source 1/") {
		t.Errorf("single-source output should not have source separators, got: %s", output)
	}
	// Should show the source path directly
	if !strings.Contains(output, "source") {
		t.Errorf("output missing source path, got: %s", output)
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
	printBatchSummary(results, 5*time.Second, time.Now().Add(-10*time.Second), 75.0)
}

func TestPrintBatchSummary_LowSavingsWarning(t *testing.T) {
	results := []*createResult{
		{
			MkvPath:    "/data/ep1.mkv",
			OutputPath: "/data/ep1.mkvdup",
			Savings:    98.5,
		},
		{
			MkvPath:    "/data/ep2.mkv",
			OutputPath: "/data/ep2.mkvdup",
			Savings:    40.0,
		},
	}

	output := captureStdout(t, func() {
		printBatchSummary(results, 5*time.Second, time.Now().Add(-10*time.Second), 75.0)
	})

	if !strings.Contains(output, "WARNING") {
		t.Error("expected WARNING in output for low savings file")
	}
	if !strings.Contains(output, "ep2.mkv") {
		t.Error("expected ep2.mkv listed in low savings warning")
	}
	if !strings.Contains(output, "1 file(s)") {
		t.Error("expected '1 file(s)' in warning count")
	}
}

func TestPrintBatchSummary_QuietSuppressesWarning(t *testing.T) {
	results := []*createResult{
		{
			MkvPath:    "/data/ep1.mkv",
			OutputPath: "/data/ep1.mkvdup",
			Savings:    40.0,
		},
	}

	oldQuiet := quiet
	quiet = true
	defer func() { quiet = oldQuiet }()

	output := captureStdout(t, func() {
		printBatchSummary(results, 5*time.Second, time.Now().Add(-10*time.Second), 75.0)
	})

	if strings.Contains(output, "WARNING") {
		t.Error("expected no WARNING when quiet=true")
	}
}

func TestPrintBatchSummary_CustomThreshold(t *testing.T) {
	results := []*createResult{
		{
			MkvPath:    "/data/ep1.mkv",
			OutputPath: "/data/ep1.mkvdup",
			Savings:    80.0,
		},
	}

	// With default threshold (75), no warning
	output := captureStdout(t, func() {
		printBatchSummary(results, 5*time.Second, time.Now().Add(-10*time.Second), 75.0)
	})
	if strings.Contains(output, "WARNING") {
		t.Error("expected no WARNING with 80% savings and 75% threshold")
	}

	// With higher threshold (90), warning should appear
	output = captureStdout(t, func() {
		printBatchSummary(results, 5*time.Second, time.Now().Add(-10*time.Second), 90.0)
	})
	if !strings.Contains(output, "WARNING") {
		t.Error("expected WARNING with 80% savings and 90% threshold")
	}
	if !strings.Contains(output, "below 90%") {
		t.Error("expected 'below 90%' in warning message")
	}
}

func TestPrintBatchSummary_SkippedFiles(t *testing.T) {
	results := []*createResult{
		{
			MkvPath:    "/data/ep1.mkv",
			OutputPath: "/data/ep1.mkvdup",
			Savings:    98.5,
		},
		{
			MkvPath: "/data/ep2.mkv",
			Skipped: true,
		},
		{
			MkvPath: "/data/ep3.mkv",
			Err:     fmt.Errorf("write failed"),
		},
		{
			MkvPath:    "/data/ep4.mkv",
			OutputPath: "/data/ep4.mkvdup",
			Savings:    96.0,
		},
	}

	output := captureStdout(t, func() {
		printBatchSummary(results, 5*time.Second, time.Now().Add(-10*time.Second), 75.0)
	})

	if !strings.Contains(output, "SKIP  ep2.mkv: codec mismatch") {
		t.Error("expected SKIP line for ep2.mkv")
	}
	if !strings.Contains(output, "OK    ep1.mkv") {
		t.Error("expected OK line for ep1.mkv")
	}
	if !strings.Contains(output, "OK    ep4.mkv") {
		t.Error("expected OK line for ep4.mkv")
	}
	if !strings.Contains(output, "Succeeded: 2/4 (1 skipped)") {
		t.Errorf("expected 'Succeeded: 2/4 (1 skipped)' in output, got:\n%s", output)
	}
}

func TestPrintBatchSummary_NoSkippedFiles(t *testing.T) {
	results := []*createResult{
		{
			MkvPath:    "/data/ep1.mkv",
			OutputPath: "/data/ep1.mkvdup",
			Savings:    98.5,
		},
	}

	output := captureStdout(t, func() {
		printBatchSummary(results, 5*time.Second, time.Now().Add(-10*time.Second), 75.0)
	})

	if strings.Contains(output, "skipped") {
		t.Error("expected no 'skipped' in output when no files were skipped")
	}
	if !strings.Contains(output, "Succeeded: 1/1") {
		t.Errorf("expected 'Succeeded: 1/1' in output, got:\n%s", output)
	}
}
