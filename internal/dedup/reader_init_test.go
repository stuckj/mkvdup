package dedup

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cespare/xxhash/v2"
)

// testDedupFileOptions configures how createTestDedupFileWithOptions creates the test file.
type testDedupFileOptions struct {
	filename         string
	numEntries       int
	originalSize     int64
	originalChecksum uint64
	includeSource    bool
	deltaSize        int64
}

// defaultTestDedupOptions returns the default options for creating a test dedup file.
func defaultTestDedupOptions() testDedupFileOptions {
	return testDedupFileOptions{
		filename:         "test.mkvdup",
		numEntries:       10,
		originalSize:     1000000,
		originalChecksum: 0x1234567890,
		includeSource:    true,
		deltaSize:        100,
	}
}

// createTestDedupFile creates a minimal valid .mkvdup file for testing.
// It creates entries with predictable values for verification.
func createTestDedupFile(t *testing.T, tmpDir string, numEntries int) string {
	t.Helper()
	opts := defaultTestDedupOptions()
	opts.numEntries = numEntries
	return createTestDedupFileWithOptions(t, tmpDir, opts)
}

// createTestDedupFileWithOptions creates a test dedup file with custom options.
func createTestDedupFileWithOptions(t *testing.T, tmpDir string, opts testDedupFileOptions) string {
	t.Helper()

	dedupPath := filepath.Join(tmpDir, opts.filename)
	f, err := os.Create(dedupPath)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer f.Close()

	// Determine source file count and size
	var sourceFileCount uint16
	var sourceFilesSize int64
	sourceFilePath := "test/source.vob"
	if opts.includeSource {
		sourceFileCount = 1
		sourceFilesSize = int64(2 + len(sourceFilePath) + 8 + 8)
	}

	// Calculate delta offset after header + source files + entries
	indexSize := int64(opts.numEntries) * EntrySize
	deltaOffset := int64(HeaderSize) + sourceFilesSize + indexSize

	// Write header
	f.Write([]byte(Magic))                                        // Magic (8 bytes)
	binary.Write(f, binary.LittleEndian, uint32(Version))         // Version (4 bytes)
	binary.Write(f, binary.LittleEndian, uint32(0))               // Flags (4 bytes)
	binary.Write(f, binary.LittleEndian, opts.originalSize)       // OriginalSize (8 bytes)
	binary.Write(f, binary.LittleEndian, opts.originalChecksum)   // OriginalChecksum (8 bytes)
	binary.Write(f, binary.LittleEndian, uint8(SourceTypeDVD))    // SourceType (1 byte)
	binary.Write(f, binary.LittleEndian, uint8(0))                // UsesESOffsets (1 byte)
	binary.Write(f, binary.LittleEndian, sourceFileCount)         // SourceFileCount (2 bytes)
	binary.Write(f, binary.LittleEndian, uint64(opts.numEntries)) // EntryCount (8 bytes)
	binary.Write(f, binary.LittleEndian, deltaOffset)             // DeltaOffset (8 bytes)
	binary.Write(f, binary.LittleEndian, opts.deltaSize)          // DeltaSize (8 bytes)

	// Write source file section if included
	if opts.includeSource {
		binary.Write(f, binary.LittleEndian, uint16(len(sourceFilePath)))
		f.Write([]byte(sourceFilePath))
		binary.Write(f, binary.LittleEndian, int64(5000000))       // Size
		binary.Write(f, binary.LittleEndian, uint64(0xABCDEF1234)) // Checksum
	}

	// Write entries with predictable values
	// Each entry: MkvOffset = i*100, Length = 50+i, Source = 1, SourceOffset = i*200
	indexHasher := xxhash.New()
	for i := 0; i < opts.numEntries; i++ {
		entryBuf := make([]byte, EntrySize)
		binary.LittleEndian.PutUint64(entryBuf[0:8], uint64(i*100))   // MkvOffset
		binary.LittleEndian.PutUint64(entryBuf[8:16], uint64(50+i))   // Length
		binary.LittleEndian.PutUint16(entryBuf[16:18], uint16(1))     // Source
		binary.LittleEndian.PutUint64(entryBuf[18:26], uint64(i*200)) // SourceOffset
		entryBuf[26] = 1                                              // ESFlags (IsVideo = true)
		entryBuf[27] = 0                                              // AudioSubStreamID

		f.Write(entryBuf)
		indexHasher.Write(entryBuf)
	}
	indexChecksum := indexHasher.Sum64()

	// Write delta data
	deltaData := make([]byte, opts.deltaSize)
	for i := range deltaData {
		deltaData[i] = byte(i % 256)
	}
	f.Write(deltaData)
	deltaChecksum := xxhash.Sum64(deltaData)

	// Write footer
	binary.Write(f, binary.LittleEndian, indexChecksum) // IndexChecksum (8 bytes)
	binary.Write(f, binary.LittleEndian, deltaChecksum) // DeltaChecksum (8 bytes)
	f.Write([]byte(Magic))                              // Magic (8 bytes)

	return dedupPath
}

// createTestDedupFileZeroEntries creates a valid dedup file with zero entries
func createTestDedupFileZeroEntries(t *testing.T, tmpDir string) string {
	t.Helper()
	opts := testDedupFileOptions{
		filename:         "zero.mkvdup",
		numEntries:       0,
		originalSize:     0,
		originalChecksum: 0,
		includeSource:    false,
		deltaSize:        0,
	}
	return createTestDedupFileWithOptions(t, tmpDir, opts)
}

func TestNewReader_FileNotFound(t *testing.T) {
	_, err := NewReader("/nonexistent/path/file.mkvdup", "/tmp")
	if err == nil {
		t.Error("NewReader should fail for nonexistent file")
	}
}

func TestNewReaderLazy_FileNotFound(t *testing.T) {
	_, err := NewReaderLazy("/nonexistent/path/file.mkvdup", "/tmp")
	if err == nil {
		t.Error("NewReaderLazy should fail for nonexistent file")
	}
}

func TestNewReader_InvalidMagic(t *testing.T) {
	tmpDir := t.TempDir()
	dedupPath := filepath.Join(tmpDir, "bad.mkvdup")

	// Write file with invalid magic
	f, err := os.Create(dedupPath)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	f.Write([]byte("BADMAGIC")) // Wrong magic
	f.Close()

	_, err = NewReader(dedupPath, tmpDir)
	if err == nil {
		t.Error("NewReader should fail for invalid magic")
	}
}

func TestNewReader_InvalidVersion(t *testing.T) {
	tmpDir := t.TempDir()
	dedupPath := filepath.Join(tmpDir, "bad.mkvdup")

	f, err := os.Create(dedupPath)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Write valid magic but unsupported version
	f.Write([]byte(Magic))
	binary.Write(f, binary.LittleEndian, uint32(99)) // Unsupported version
	f.Close()

	_, err = NewReader(dedupPath, tmpDir)
	if err == nil {
		t.Error("NewReader should fail for unsupported version")
	}
	if err != nil && !strings.Contains(err.Error(), "expected 3-8") {
		t.Errorf("Error should mention expected versions 3-8: %v", err)
	}
}

func TestNewReader_Version1Error(t *testing.T) {
	tmpDir := t.TempDir()
	dedupPath := filepath.Join(tmpDir, "v1.mkvdup")

	f, err := os.Create(dedupPath)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Write valid magic but version 1
	f.Write([]byte(Magic))
	binary.Write(f, binary.LittleEndian, uint32(1)) // Version 1
	f.Close()

	_, err = NewReader(dedupPath, tmpDir)
	if err == nil {
		t.Error("NewReader should fail for version 1")
	}
	// Check error message suggests recreating
	if err != nil && !strings.Contains(err.Error(), "recreate") {
		t.Errorf("Error should suggest recreating: %v", err)
	}
}

func TestNewReader_Version2Error(t *testing.T) {
	tmpDir := t.TempDir()
	dedupPath := filepath.Join(tmpDir, "v2.mkvdup")

	f, err := os.Create(dedupPath)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Write valid magic but version 2
	f.Write([]byte(Magic))
	binary.Write(f, binary.LittleEndian, uint32(2)) // Version 2
	f.Close()

	_, err = NewReader(dedupPath, tmpDir)
	if err == nil {
		t.Error("NewReader should fail for version 2")
	}
}

func TestNewReader_TruncatedHeader(t *testing.T) {
	tmpDir := t.TempDir()
	dedupPath := filepath.Join(tmpDir, "truncated.mkvdup")

	f, err := os.Create(dedupPath)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Write only partial header
	f.Write([]byte(Magic))
	binary.Write(f, binary.LittleEndian, uint32(Version))
	// Missing rest of header
	f.Close()

	_, err = NewReader(dedupPath, tmpDir)
	if err == nil {
		t.Error("NewReader should fail for truncated header")
	}
}

func TestLoadSourceFiles_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	dedupPath := createTestDedupFile(t, tmpDir, 5)

	reader, err := NewReader(dedupPath, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	// Try to load source files that don't exist
	err = reader.LoadSourceFiles()
	if err == nil {
		t.Error("LoadSourceFiles should fail when source files don't exist")
	}
}

func TestInitEntryAccess(t *testing.T) {
	tmpDir := t.TempDir()
	dedupPath := createTestDedupFile(t, tmpDir, 10)

	// Use NewReaderLazy to avoid immediate initialization
	reader, err := NewReaderLazy(dedupPath, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	// Before InitEntryAccess, entryCount should be 0
	if reader.entryCount != 0 {
		t.Errorf("Before init, entryCount = %d, want 0", reader.entryCount)
	}

	// Call InitEntryAccess explicitly
	if err := reader.InitEntryAccess(); err != nil {
		t.Fatalf("InitEntryAccess failed: %v", err)
	}

	// After InitEntryAccess, entryCount should be set
	if reader.entryCount != 10 {
		t.Errorf("After init, entryCount = %d, want 10", reader.entryCount)
	}

	// Calling again should be safe (idempotent via sync.Once)
	if err := reader.InitEntryAccess(); err != nil {
		t.Errorf("Second InitEntryAccess failed: %v", err)
	}
}

func TestZeroEntries(t *testing.T) {
	tmpDir := t.TempDir()
	dedupPath := createTestDedupFileZeroEntries(t, tmpDir)

	reader, err := NewReader(dedupPath, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	if count := reader.EntryCount(); count != 0 {
		t.Errorf("EntryCount() = %d, want 0", count)
	}

	// Reading from empty file should return EOF
	buf := make([]byte, 100)
	_, err = reader.ReadAt(buf, 0)
	if err == nil {
		t.Error("ReadAt should return error for empty file")
	}
}

func TestClose_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	dedupPath := createTestDedupFile(t, tmpDir, 5)

	reader, err := NewReader(dedupPath, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}

	// Close multiple times should not panic
	if err := reader.Close(); err != nil {
		t.Errorf("First Close() failed: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Errorf("Second Close() failed: %v", err)
	}
}
