package dedup

import (
	"os"
	"testing"
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

	// Corrupt a byte in the index section (after header and source files)
	// Header is 56 bytes, source file section is ~30 bytes, so corrupt around offset 100
	_, err = f.Seek(100, 0)
	if err != nil {
		t.Fatalf("Failed to seek: %v", err)
	}
	f.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF}) // Corrupt 4 bytes
	f.Close()

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
