package dedup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stuckj/mkvdup/internal/matcher"
	"github.com/stuckj/mkvdup/internal/source"
)

func TestReadAt_BeyondFileEnd(t *testing.T) {
	tmpDir := t.TempDir()
	dedupPath := createTestDedupFile(t, tmpDir, 5)

	reader, err := NewReader(dedupPath, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	// Try to read beyond file end
	buf := make([]byte, 100)
	n, err := reader.ReadAt(buf, reader.OriginalSize()+1000)
	if err == nil {
		t.Error("ReadAt should return error when reading beyond file end")
	}
	if n != 0 {
		t.Errorf("ReadAt should return 0 bytes, got %d", n)
	}
}

func TestReadAt_AtExactEnd(t *testing.T) {
	tmpDir := t.TempDir()
	dedupPath := createTestDedupFile(t, tmpDir, 5)

	reader, err := NewReader(dedupPath, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	// Try to read at exact end of file
	buf := make([]byte, 100)
	_, err = reader.ReadAt(buf, reader.OriginalSize())
	if err == nil {
		t.Error("ReadAt should return EOF when reading at exact file end")
	}
}

func TestReadAt_EmptyBuffer(t *testing.T) {
	tmpDir := t.TempDir()
	dedupPath := createTestDedupFile(t, tmpDir, 5)

	reader, err := NewReader(dedupPath, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	// Read with empty buffer
	buf := make([]byte, 0)
	n, err := reader.ReadAt(buf, 0)
	if err != nil {
		t.Errorf("ReadAt with empty buffer should not error: %v", err)
	}
	if n != 0 {
		t.Errorf("ReadAt with empty buffer should return 0, got %d", n)
	}
}

func TestVerifyIntegrity_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	dedupPath := createTestDedupFile(t, tmpDir, 10)

	reader, err := NewReader(dedupPath, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	// Verify integrity should pass for valid file
	if err := reader.VerifyIntegrity(); err != nil {
		t.Errorf("VerifyIntegrity failed for valid file: %v", err)
	}
}

func TestVerifyIntegrity_CorruptedIndex(t *testing.T) {
	tmpDir := t.TempDir()
	dedupPath := createTestDedupFile(t, tmpDir, 10)

	// Corrupt the index section
	f, err := os.OpenFile(dedupPath, os.O_RDWR, 0644)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}

	// Corrupt bytes near the end of the file to reliably hit the index section
	// regardless of header/source-file section sizes
	fi, err := f.Stat()
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}
	corruptionOffset := fi.Size() - 4
	if corruptionOffset < 0 {
		t.Fatal("File too small to corrupt")
	}
	_, err = f.Seek(corruptionOffset, 0)
	if err != nil {
		t.Fatalf("Failed to seek: %v", err)
	}
	if _, err := f.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF}); err != nil {
		t.Fatalf("Failed to write corruption bytes: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Failed to close file: %v", err)
	}

	reader, err := NewReader(dedupPath, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	// Verify integrity should fail
	err = reader.VerifyIntegrity()
	if err == nil {
		t.Error("VerifyIntegrity should fail for corrupted index")
	}
}

func TestVerifyIntegrity_InvalidFooterMagic(t *testing.T) {
	tmpDir := t.TempDir()
	dedupPath := createTestDedupFile(t, tmpDir, 5)

	// Get file size and corrupt footer magic
	stat, err := os.Stat(dedupPath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	f, err := os.OpenFile(dedupPath, os.O_RDWR, 0644)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}

	// Footer magic is at the very end (last 8 bytes)
	_, err = f.Seek(stat.Size()-8, 0)
	if err != nil {
		t.Fatalf("Failed to seek: %v", err)
	}
	f.Write([]byte("BADMAGIC"))
	f.Close()

	reader, err := NewReader(dedupPath, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	err = reader.VerifyIntegrity()
	if err == nil {
		t.Error("VerifyIntegrity should fail for invalid footer magic")
	}
}

func TestOriginalSize(t *testing.T) {
	tmpDir := t.TempDir()
	dedupPath := createTestDedupFile(t, tmpDir, 5)

	reader, err := NewReader(dedupPath, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	if size := reader.OriginalSize(); size != 1000000 {
		t.Errorf("OriginalSize() = %d, want 1000000", size)
	}
}

func TestOriginalChecksum(t *testing.T) {
	tmpDir := t.TempDir()
	dedupPath := createTestDedupFile(t, tmpDir, 5)

	reader, err := NewReader(dedupPath, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	if checksum := reader.OriginalChecksum(); checksum != 0x1234567890 {
		t.Errorf("OriginalChecksum() = 0x%x, want 0x1234567890", checksum)
	}
}

func TestSourceFiles(t *testing.T) {
	tmpDir := t.TempDir()
	dedupPath := createTestDedupFile(t, tmpDir, 5)

	reader, err := NewReader(dedupPath, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	files := reader.SourceFiles()
	if len(files) != 1 {
		t.Fatalf("SourceFiles() len = %d, want 1", len(files))
	}
	if files[0].RelativePath != "test/source.vob" {
		t.Errorf("SourceFiles()[0].RelativePath = %q, want %q", files[0].RelativePath, "test/source.vob")
	}
}

func TestUsesESOffsets(t *testing.T) {
	tmpDir := t.TempDir()
	dedupPath := createTestDedupFile(t, tmpDir, 5)

	reader, err := NewReader(dedupPath, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	// Test file has UsesESOffsets = false
	if reader.UsesESOffsets() {
		t.Error("UsesESOffsets() = true, want false")
	}
}

func TestInfo(t *testing.T) {
	tmpDir := t.TempDir()
	dedupPath := createTestDedupFile(t, tmpDir, 10)

	reader, err := NewReader(dedupPath, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	info := reader.Info()

	// Verify key fields exist and have expected values
	if info["version"].(uint32) != Version {
		t.Errorf("info[version] = %v, want %v", info["version"], Version)
	}
	if info["original_size"].(int64) != 1000000 {
		t.Errorf("info[original_size] = %v, want 1000000", info["original_size"])
	}
	if info["entry_count"].(int) != 10 {
		t.Errorf("info[entry_count] = %v, want 10", info["entry_count"])
	}
	if info["source_file_count"].(int) != 1 {
		t.Errorf("info[source_file_count] = %v, want 1", info["source_file_count"])
	}
	// V3 file should have empty creator version
	if info["creator_version"].(string) != "" {
		t.Errorf("info[creator_version] = %v, want empty string", info["creator_version"])
	}
}

func TestReadAt_SourceBacked(t *testing.T) {
	dir := t.TempDir()

	// Create source data with a known pattern.
	srcData := make([]byte, 200)
	for i := range srcData {
		srcData[i] = byte(i * 7 & 0xFF)
	}

	// Create source file
	srcDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	srcPath := filepath.Join(srcDir, "source.vob")
	if err := os.WriteFile(srcPath, srcData, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Write dedup file with: delta (10 bytes) + source-matched (90 bytes)
	deltaBytes := srcData[:10]
	dedupPath := writeTestDedupFile(t, dir, writeTestOptions{
		originalSize:     100,
		originalChecksum: 0xAAAA,
		sourceType:       source.TypeDVD,
		creatorVersion:   "test-v1",
		sourceFiles: []source.File{
			{RelativePath: "source.vob", Size: int64(len(srcData)), Checksum: 0xBBBB},
		},
		result: &matcher.Result{
			Entries: []matcher.Entry{
				{MkvOffset: 0, Length: 10, Source: 0, SourceOffset: 0},
				{MkvOffset: 10, Length: 90, Source: 1, SourceOffset: 10, IsVideo: true},
			},
			DeltaData:      deltaBytes,
			MatchedBytes:   90,
			UnmatchedBytes: 10,
			TotalPackets:   1,
		},
	})

	r, err := NewReader(dedupPath, srcDir)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer r.Close()

	if err := r.LoadSourceFiles(); err != nil {
		t.Fatalf("LoadSourceFiles: %v", err)
	}

	// Full read
	buf := make([]byte, 100)
	n, err := r.ReadAt(buf, 0)
	if err != nil {
		t.Fatalf("ReadAt full: %v", err)
	}
	if n != 100 {
		t.Fatalf("ReadAt full: got %d bytes, want 100", n)
	}
	for i := 0; i < 100; i++ {
		if buf[i] != srcData[i] {
			t.Errorf("byte %d: got %02x, want %02x", i, buf[i], srcData[i])
			break
		}
	}

	// Partial read spanning delta and source entries
	partialBuf := make([]byte, 20)
	n, err = r.ReadAt(partialBuf, 5)
	if err != nil {
		t.Fatalf("ReadAt partial: %v", err)
	}
	if n != 20 {
		t.Fatalf("ReadAt partial: got %d bytes, want 20", n)
	}
	for i := range partialBuf {
		if partialBuf[i] != srcData[5+i] {
			t.Errorf("partial byte %d: got %02x, want %02x", i, partialBuf[i], srcData[5+i])
			break
		}
	}
}
