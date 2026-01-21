package dedup

import (
	"encoding/binary"
	"os"
	"path/filepath"
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
	f.Write([]byte(Magic))                                      // Magic (8 bytes)
	binary.Write(f, binary.LittleEndian, uint32(Version))       // Version (4 bytes)
	binary.Write(f, binary.LittleEndian, uint32(0))             // Flags (4 bytes)
	binary.Write(f, binary.LittleEndian, int64(1000000))        // OriginalSize (8 bytes)
	binary.Write(f, binary.LittleEndian, uint64(0x1234567890))  // OriginalChecksum (8 bytes)
	binary.Write(f, binary.LittleEndian, uint8(SourceTypeDVD))  // SourceType (1 byte)
	binary.Write(f, binary.LittleEndian, uint8(0))              // UsesESOffsets (1 byte)
	binary.Write(f, binary.LittleEndian, uint16(1))             // SourceFileCount (2 bytes)
	binary.Write(f, binary.LittleEndian, uint64(numEntries))    // EntryCount (8 bytes)

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
