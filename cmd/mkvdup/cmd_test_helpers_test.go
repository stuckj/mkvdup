package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/cespare/xxhash/v2"
	"github.com/stuckj/mkvdup/internal/dedup"
	"github.com/stuckj/mkvdup/internal/matcher"
	"github.com/stuckj/mkvdup/internal/source"
)

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

// createExtractableDedup creates a dedup file where all data is in the delta
// section, so extractDedup can reconstruct it without real source media.
func createExtractableDedup(t *testing.T, dedupPath, sourceDir string, originalData []byte) {
	t.Helper()

	srcFile := filepath.Join(sourceDir, "test.vob")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	srcContent := []byte("source data")
	if err := os.WriteFile(srcFile, srcContent, 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	writer, err := dedup.NewWriter(dedupPath)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}

	checksum := xxhash.Sum64(originalData)
	writer.SetHeader(int64(len(originalData)), checksum, source.TypeDVD)
	writer.SetSourceFiles([]source.File{
		{RelativePath: "test.vob", Size: int64(len(srcContent)), Checksum: xxhash.Sum64(srcContent)},
	})

	result := &matcher.Result{
		Entries: []matcher.Entry{
			{MkvOffset: 0, Length: int64(len(originalData)), Source: 0, SourceOffset: 0},
		},
		DeltaData:      originalData,
		UnmatchedBytes: int64(len(originalData)),
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

// captureStderr captures stderr output from f.
func captureStderr(t *testing.T, f func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { os.Stderr = old }()
	os.Stderr = w
	f()
	w.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}
