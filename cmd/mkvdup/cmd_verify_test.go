package main

import (
	"os"
	"path/filepath"
	"testing"
)

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

// --- extract command tests ---

func TestExtractDedup_Success(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "source")
	dedupPath := filepath.Join(dir, "movie.mkvdup")
	outputPath := filepath.Join(dir, "output.mkv")
	originalData := []byte("hello world this is test data for extraction")

	createExtractableDedup(t, dedupPath, sourceDir, originalData)

	if err := extractDedup(dedupPath, sourceDir, outputPath); err != nil {
		t.Fatalf("extractDedup: %v", err)
	}

	got, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(got) != string(originalData) {
		t.Errorf("extracted data = %q, want %q", got, originalData)
	}
}

func TestExtractDedup_CleansUpOnError(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "source")
	dedupPath := filepath.Join(dir, "movie.mkvdup")
	outputPath := filepath.Join(dir, "output.mkv")

	// Use the basic helper which creates a dedup with OriginalSize=100
	// but only 5 bytes of delta and no entries — ReadAt will fail.
	createTestDedupFile(t, dedupPath, sourceDir)

	err := extractDedup(dedupPath, sourceDir, outputPath)
	if err == nil {
		t.Fatal("expected error for unextractable dedup")
	}

	if _, statErr := os.Stat(outputPath); !os.IsNotExist(statErr) {
		t.Error("expected output file to be cleaned up after error")
	}
}
