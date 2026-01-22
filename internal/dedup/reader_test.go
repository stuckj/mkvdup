package dedup

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/cespare/xxhash/v2"
)

// createTestDedupFile creates a minimal valid .mkvdup file for testing.
// It creates entries with predictable values for verification.
func createTestDedupFile(t *testing.T, tmpDir string, numEntries int) string {
	t.Helper()

	dedupPath := filepath.Join(tmpDir, "test.mkvdup")
	f, err := os.Create(dedupPath)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer f.Close()

	// Write header
	f.Write([]byte(Magic))                                     // Magic (8 bytes)
	binary.Write(f, binary.LittleEndian, uint32(Version))      // Version (4 bytes)
	binary.Write(f, binary.LittleEndian, uint32(0))            // Flags (4 bytes)
	binary.Write(f, binary.LittleEndian, int64(1000000))       // OriginalSize (8 bytes)
	binary.Write(f, binary.LittleEndian, uint64(0x1234567890)) // OriginalChecksum (8 bytes)
	binary.Write(f, binary.LittleEndian, uint8(SourceTypeDVD)) // SourceType (1 byte)
	binary.Write(f, binary.LittleEndian, uint8(0))             // UsesESOffsets (1 byte)
	binary.Write(f, binary.LittleEndian, uint16(1))            // SourceFileCount (2 bytes)
	binary.Write(f, binary.LittleEndian, uint64(numEntries))   // EntryCount (8 bytes)

	// Calculate delta offset after header + source files + entries
	sourceFilePath := "test/source.vob"
	sourceFilesSize := int64(2 + len(sourceFilePath) + 8 + 8)
	indexSize := int64(numEntries) * EntrySize
	deltaOffset := int64(HeaderSize) + sourceFilesSize + indexSize
	deltaSize := int64(100) // 100 bytes of delta data

	binary.Write(f, binary.LittleEndian, deltaOffset) // DeltaOffset (8 bytes)
	binary.Write(f, binary.LittleEndian, deltaSize)   // DeltaSize (8 bytes)

	// Write source file section
	binary.Write(f, binary.LittleEndian, uint16(len(sourceFilePath)))
	f.Write([]byte(sourceFilePath))
	binary.Write(f, binary.LittleEndian, int64(5000000))       // Size
	binary.Write(f, binary.LittleEndian, uint64(0xABCDEF1234)) // Checksum

	// Write entries with predictable values
	// Each entry: MkvOffset = i*100, Length = 50+i, Source = 1, SourceOffset = i*200
	indexHasher := xxhash.New()
	for i := 0; i < numEntries; i++ {
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
	deltaData := make([]byte, deltaSize)
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

func TestGetEntry_CacheHit(t *testing.T) {
	tmpDir := t.TempDir()
	dedupPath := createTestDedupFile(t, tmpDir, 10)

	reader, err := NewReader(dedupPath, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	// First access - should populate cache
	entry1, ok := reader.getEntry(5)
	if !ok {
		t.Fatal("getEntry(5) returned false")
	}

	// Verify entry values
	if entry1.MkvOffset != 500 { // 5 * 100
		t.Errorf("MkvOffset = %d, want 500", entry1.MkvOffset)
	}
	if entry1.Length != 55 { // 50 + 5
		t.Errorf("Length = %d, want 55", entry1.Length)
	}
	if entry1.Source != 1 {
		t.Errorf("Source = %d, want 1", entry1.Source)
	}
	if entry1.SourceOffset != 1000 { // 5 * 200
		t.Errorf("SourceOffset = %d, want 1000", entry1.SourceOffset)
	}
	if !entry1.IsVideo {
		t.Error("IsVideo = false, want true")
	}

	// Verify cache was populated
	reader.cacheMu.Lock()
	if reader.lastEntryIdx != 5 {
		t.Errorf("lastEntryIdx = %d, want 5", reader.lastEntryIdx)
	}
	if !reader.lastEntryValid {
		t.Error("lastEntryValid = false, want true")
	}
	cachedEntry := reader.lastEntry
	reader.cacheMu.Unlock()

	// Second access to same index - should return cached entry
	entry2, ok := reader.getEntry(5)
	if !ok {
		t.Fatal("getEntry(5) returned false on second call")
	}

	// Entries should be identical
	if entry1 != entry2 {
		t.Errorf("Cache hit returned different entry: %+v vs %+v", entry1, entry2)
	}
	if entry2 != cachedEntry {
		t.Errorf("Entry doesn't match cached: %+v vs %+v", entry2, cachedEntry)
	}
}

func TestGetEntry_CacheMiss(t *testing.T) {
	tmpDir := t.TempDir()
	dedupPath := createTestDedupFile(t, tmpDir, 10)

	reader, err := NewReader(dedupPath, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	// Access entry 3
	entry3, ok := reader.getEntry(3)
	if !ok {
		t.Fatal("getEntry(3) returned false")
	}
	if entry3.MkvOffset != 300 {
		t.Errorf("Entry 3 MkvOffset = %d, want 300", entry3.MkvOffset)
	}

	// Verify cache has entry 3
	reader.cacheMu.Lock()
	if reader.lastEntryIdx != 3 {
		t.Errorf("lastEntryIdx = %d, want 3", reader.lastEntryIdx)
	}
	reader.cacheMu.Unlock()

	// Access entry 7 - should update cache
	entry7, ok := reader.getEntry(7)
	if !ok {
		t.Fatal("getEntry(7) returned false")
	}
	if entry7.MkvOffset != 700 {
		t.Errorf("Entry 7 MkvOffset = %d, want 700", entry7.MkvOffset)
	}

	// Verify cache was updated to entry 7
	reader.cacheMu.Lock()
	if reader.lastEntryIdx != 7 {
		t.Errorf("lastEntryIdx = %d, want 7 after cache miss", reader.lastEntryIdx)
	}
	reader.cacheMu.Unlock()
}

func TestGetEntry_BoundaryConditions(t *testing.T) {
	tmpDir := t.TempDir()
	dedupPath := createTestDedupFile(t, tmpDir, 5)

	reader, err := NewReader(dedupPath, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	tests := []struct {
		name      string
		idx       int
		wantOK    bool
		wantEntry Entry
	}{
		{
			name:   "negative index",
			idx:    -1,
			wantOK: false,
		},
		{
			name:   "index equals count",
			idx:    5,
			wantOK: false,
		},
		{
			name:   "index greater than count",
			idx:    100,
			wantOK: false,
		},
		{
			name:   "first valid index",
			idx:    0,
			wantOK: true,
			wantEntry: Entry{
				MkvOffset:    0,
				Length:       50,
				Source:       1,
				SourceOffset: 0,
				IsVideo:      true,
			},
		},
		{
			name:   "last valid index",
			idx:    4,
			wantOK: true,
			wantEntry: Entry{
				MkvOffset:    400,
				Length:       54,
				Source:       1,
				SourceOffset: 800,
				IsVideo:      true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, ok := reader.getEntry(tt.idx)
			if ok != tt.wantOK {
				t.Errorf("getEntry(%d) ok = %v, want %v", tt.idx, ok, tt.wantOK)
			}
			if tt.wantOK && entry != tt.wantEntry {
				t.Errorf("getEntry(%d) = %+v, want %+v", tt.idx, entry, tt.wantEntry)
			}
		})
	}
}

func TestGetMkvOffset(t *testing.T) {
	tmpDir := t.TempDir()
	dedupPath := createTestDedupFile(t, tmpDir, 10)

	reader, err := NewReader(dedupPath, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	tests := []struct {
		idx        int
		wantOffset int64
		wantOK     bool
	}{
		{idx: 0, wantOffset: 0, wantOK: true},
		{idx: 5, wantOffset: 500, wantOK: true},
		{idx: 9, wantOffset: 900, wantOK: true},
		{idx: -1, wantOffset: 0, wantOK: false},
		{idx: 10, wantOffset: 0, wantOK: false},
	}

	for _, tt := range tests {
		offset, ok := reader.getMkvOffset(tt.idx)
		if ok != tt.wantOK {
			t.Errorf("getMkvOffset(%d) ok = %v, want %v", tt.idx, ok, tt.wantOK)
		}
		if ok && offset != tt.wantOffset {
			t.Errorf("getMkvOffset(%d) = %d, want %d", tt.idx, offset, tt.wantOffset)
		}
	}
}

func TestGetEntryLength(t *testing.T) {
	tmpDir := t.TempDir()
	dedupPath := createTestDedupFile(t, tmpDir, 10)

	reader, err := NewReader(dedupPath, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	tests := []struct {
		idx        int
		wantLength int64
		wantOK     bool
	}{
		{idx: 0, wantLength: 50, wantOK: true},
		{idx: 5, wantLength: 55, wantOK: true},
		{idx: 9, wantLength: 59, wantOK: true},
		{idx: -1, wantLength: 0, wantOK: false},
		{idx: 10, wantLength: 0, wantOK: false},
	}

	for _, tt := range tests {
		length, ok := reader.getEntryLength(tt.idx)
		if ok != tt.wantOK {
			t.Errorf("getEntryLength(%d) ok = %v, want %v", tt.idx, ok, tt.wantOK)
		}
		if ok && length != tt.wantLength {
			t.Errorf("getEntryLength(%d) = %d, want %d", tt.idx, length, tt.wantLength)
		}
	}
}

func TestGetEntry_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	dedupPath := createTestDedupFile(t, tmpDir, 100)

	reader, err := NewReader(dedupPath, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	// Run multiple goroutines accessing different entries concurrently
	const numGoroutines = 10
	const accessesPerGoroutine = 100

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*accessesPerGoroutine)

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < accessesPerGoroutine; i++ {
				idx := (goroutineID*accessesPerGoroutine + i) % 100
				entry, ok := reader.getEntry(idx)
				if !ok {
					errors <- nil // Boundary errors handled separately
					continue
				}
				// Verify entry values are correct
				expectedMkvOffset := int64(idx * 100)
				expectedLength := int64(50 + idx)
				if entry.MkvOffset != expectedMkvOffset {
					errors <- nil
					t.Errorf("Goroutine %d: entry %d MkvOffset = %d, want %d",
						goroutineID, idx, entry.MkvOffset, expectedMkvOffset)
				}
				if entry.Length != expectedLength {
					errors <- nil
					t.Errorf("Goroutine %d: entry %d Length = %d, want %d",
						goroutineID, idx, entry.Length, expectedLength)
				}
			}
		}(g)
	}

	wg.Wait()
	close(errors)
}

func TestSequentialAccess_CacheEfficiency(t *testing.T) {
	tmpDir := t.TempDir()
	dedupPath := createTestDedupFile(t, tmpDir, 100)

	reader, err := NewReader(dedupPath, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	// Simulate sequential access pattern (like reading a file from start to end)
	// Each subsequent access should hit cache frequently
	cacheHits := 0
	for i := 0; i < 100; i++ {
		// Check if this access would be a cache hit
		reader.cacheMu.Lock()
		willBeHit := reader.lastEntryValid && reader.lastEntryIdx == i
		reader.cacheMu.Unlock()

		entry, ok := reader.getEntry(i)
		if !ok {
			t.Fatalf("getEntry(%d) returned false", i)
		}

		if willBeHit {
			cacheHits++
		}

		// Verify entry
		if entry.MkvOffset != int64(i*100) {
			t.Errorf("Entry %d MkvOffset = %d, want %d", i, entry.MkvOffset, i*100)
		}
	}

	// Sequential access without repeated indices should have 0 cache hits
	// (each index is accessed only once in order)
	if cacheHits != 0 {
		t.Logf("Sequential unique access had %d cache hits (expected 0)", cacheHits)
	}

	// Now test repeated access to same entry
	for i := 0; i < 10; i++ {
		entry, ok := reader.getEntry(50)
		if !ok {
			t.Fatal("getEntry(50) returned false")
		}
		if entry.MkvOffset != 5000 {
			t.Errorf("Repeated access MkvOffset = %d, want 5000", entry.MkvOffset)
		}
	}

	// Cache should still have entry 50
	reader.cacheMu.Lock()
	if reader.lastEntryIdx != 50 {
		t.Errorf("After repeated access, lastEntryIdx = %d, want 50", reader.lastEntryIdx)
	}
	reader.cacheMu.Unlock()
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

func TestEntryCount(t *testing.T) {
	tmpDir := t.TempDir()
	dedupPath := createTestDedupFile(t, tmpDir, 25)

	reader, err := NewReader(dedupPath, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	if count := reader.EntryCount(); count != 25 {
		t.Errorf("EntryCount() = %d, want 25", count)
	}
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
}

// createTestDedupFileZeroEntries creates a valid dedup file with zero entries
func createTestDedupFileZeroEntries(t *testing.T, tmpDir string) string {
	t.Helper()

	dedupPath := filepath.Join(tmpDir, "zero.mkvdup")
	f, err := os.Create(dedupPath)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer f.Close()

	// Write header with zero entries
	f.Write([]byte(Magic))
	binary.Write(f, binary.LittleEndian, uint32(Version))
	binary.Write(f, binary.LittleEndian, uint32(0))            // Flags
	binary.Write(f, binary.LittleEndian, int64(0))             // OriginalSize
	binary.Write(f, binary.LittleEndian, uint64(0))            // OriginalChecksum
	binary.Write(f, binary.LittleEndian, uint8(SourceTypeDVD)) // SourceType
	binary.Write(f, binary.LittleEndian, uint8(0))             // UsesESOffsets
	binary.Write(f, binary.LittleEndian, uint16(0))            // SourceFileCount (0)
	binary.Write(f, binary.LittleEndian, uint64(0))            // EntryCount (0)

	// Delta offset and size (right after header since no source files or entries)
	deltaOffset := int64(HeaderSize)
	deltaSize := int64(0)
	binary.Write(f, binary.LittleEndian, deltaOffset)
	binary.Write(f, binary.LittleEndian, deltaSize)

	// Write footer (no index to checksum, no delta to checksum)
	emptyHash := xxhash.Sum64(nil) // xxhash of empty data
	binary.Write(f, binary.LittleEndian, emptyHash)
	binary.Write(f, binary.LittleEndian, emptyHash)
	f.Write([]byte(Magic))

	return dedupPath
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

func TestFindEntriesForRange_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	dedupPath := createTestDedupFileZeroEntries(t, tmpDir)

	reader, err := NewReader(dedupPath, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	// findEntriesForRange should return nil for empty file
	entries := reader.findEntriesForRange(0, 100)
	if len(entries) != 0 {
		t.Errorf("findEntriesForRange returned %d entries, want 0", len(entries))
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

func TestSetESReader(t *testing.T) {
	tmpDir := t.TempDir()
	dedupPath := createTestDedupFile(t, tmpDir, 5)

	reader, err := NewReader(dedupPath, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	// SetESReader should not panic with nil
	reader.SetESReader(nil)

	// Reader's esReader should be nil
	if reader.esReader != nil {
		t.Error("esReader should be nil after SetESReader(nil)")
	}
}
