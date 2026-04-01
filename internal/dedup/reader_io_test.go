package dedup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stuckj/mkvdup/internal/bitshift"
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

func TestReadAt_BitShiftTransform(t *testing.T) {
	dir := t.TempDir()

	// Create source data: a fake NAL with known bytes.
	// The reader will apply bit-shift to source bytes to reconstruct MKV bytes.
	srcData := make([]byte, 200)
	for i := range srcData {
		srcData[i] = byte(i * 7 & 0xFF) // deterministic pattern
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

	// Compute what the MKV bytes should be for a shift of 3, starting at source offset 10.
	// Pre-divergence: 5 bytes copied verbatim from source.
	// Post-divergence: 90 bytes bit-shifted from source offset 15.
	shift := uint8(3)
	preDivLen := 5
	postDivLen := 90
	totalLen := preDivLen + postDivLen

	// Build expected MKV data
	expectedMKV := make([]byte, totalLen)
	copy(expectedMKV[:preDivLen], srcData[10:10+preDivLen])
	bitshift.Apply(srcData[10+preDivLen:10+preDivLen+postDivLen+1], shift, expectedMKV[preDivLen:])

	// Write dedup file with two entries:
	// 1. Delta entry for pre-divergence bytes (5 bytes)
	// 2. Bit-shifted source entry for post-divergence bytes (90 bytes)
	dedupPath := writeTestDedupFile(t, dir, writeTestOptions{
		originalSize:     int64(totalLen),
		originalChecksum: 0xAAAA,
		sourceType:       source.TypeDVD,
		creatorVersion:   "test-bitshift",
		sourceFiles: []source.File{
			{RelativePath: "source.vob", Size: int64(len(srcData)), Checksum: 0xBBBB},
		},
		result: &matcher.Result{
			Entries: []matcher.Entry{
				{MkvOffset: 0, Length: int64(preDivLen), Source: 0, SourceOffset: 0},
				{MkvOffset: int64(preDivLen), Length: int64(postDivLen), Source: 1, SourceOffset: int64(10 + preDivLen), IsVideo: true, BitShiftAmount: shift},
			},
			DeltaData:      expectedMKV[:preDivLen],
			MatchedBytes:   int64(postDivLen),
			UnmatchedBytes: int64(preDivLen),
			TotalPackets:   1,
		},
	})

	// Open reader with source dir
	r, err := NewReader(dedupPath, srcDir)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer r.Close()

	// Load source files for ReadAt to work
	if err := r.LoadSourceFiles(); err != nil {
		t.Fatalf("LoadSourceFiles: %v", err)
	}

	// Full read
	buf := make([]byte, totalLen)
	n, err := r.ReadAt(buf, 0)
	if err != nil {
		t.Fatalf("ReadAt full: %v", err)
	}
	if n != totalLen {
		t.Fatalf("ReadAt full: got %d bytes, want %d", n, totalLen)
	}
	for i := range expectedMKV {
		if buf[i] != expectedMKV[i] {
			t.Errorf("byte %d: got %02x, want %02x", i, buf[i], expectedMKV[i])
			break
		}
	}

	// Partial read: start mid-way through the bit-shifted entry
	partialOff := int64(preDivLen + 10)
	partialLen := 20
	partialBuf := make([]byte, partialLen)
	n, err = r.ReadAt(partialBuf, partialOff)
	if err != nil {
		t.Fatalf("ReadAt partial: %v", err)
	}
	if n != partialLen {
		t.Fatalf("ReadAt partial: got %d bytes, want %d", n, partialLen)
	}
	for i := range partialBuf {
		if partialBuf[i] != expectedMKV[int(partialOff)+i] {
			t.Errorf("partial byte %d: got %02x, want %02x", i, partialBuf[i], expectedMKV[int(partialOff)+i])
			break
		}
	}
}
