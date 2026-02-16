package dedup

import (
	"sync"
	"testing"
)

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

func TestBlockIndex_Built(t *testing.T) {
	tmpDir := t.TempDir()
	dedupPath := createTestDedupFile(t, tmpDir, 100)

	reader, err := NewReader(dedupPath, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	// Block index should be built
	if reader.blockIndex == nil {
		t.Fatal("blockIndex is nil after NewReader")
	}

	// All entries should point to valid indices
	for i, idx := range reader.blockIndex {
		if idx < 0 || int(idx) >= reader.entryCount {
			t.Errorf("blockIndex[%d] = %d, out of range [0, %d)", i, idx, reader.entryCount)
		}
	}
}

func TestBlockIndex_LookupCorrectness(t *testing.T) {
	tmpDir := t.TempDir()
	// Test entries: MkvOffset=i*100, Length=50+i (from createTestDedupFile)
	dedupPath := createTestDedupFile(t, tmpDir, 100)

	reader, err := NewReader(dedupPath, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	tests := []struct {
		name      string
		offset    int64
		length    int64
		wantFirst int64 // Expected MkvOffset of first returned entry
	}{
		{name: "start of file", offset: 0, length: 10, wantFirst: 0},
		{name: "exact entry start", offset: 500, length: 10, wantFirst: 500},
		{name: "mid-entry", offset: 505, length: 10, wantFirst: 500},
		// Entry 98: MkvOffset=9800, Length=148 covers [9800,9948), so offset 9900 is within it
		{name: "near end", offset: 9900, length: 10, wantFirst: 9800},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear cache to force block index path
			reader.cacheMu.Lock()
			reader.lastEntryValid = false
			reader.cacheMu.Unlock()

			entries := reader.findEntriesForRange(tt.offset, tt.length)
			if len(entries) == 0 {
				t.Fatalf("findEntriesForRange(%d, %d) returned no entries", tt.offset, tt.length)
			}
			if entries[0].MkvOffset != tt.wantFirst {
				t.Errorf("first entry MkvOffset = %d, want %d", entries[0].MkvOffset, tt.wantFirst)
			}
		})
	}
}

func TestBlockIndex_ZeroEntries(t *testing.T) {
	tmpDir := t.TempDir()
	dedupPath := createTestDedupFileZeroEntries(t, tmpDir)

	reader, err := NewReader(dedupPath, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	// Block index should be nil for zero entries
	if reader.blockIndex != nil {
		t.Error("blockIndex should be nil for zero entries")
	}
}

func TestBlockIndex_SingleEntry(t *testing.T) {
	tmpDir := t.TempDir()
	dedupPath := createTestDedupFile(t, tmpDir, 1)

	reader, err := NewReader(dedupPath, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	if reader.blockIndex == nil {
		t.Fatal("blockIndex is nil for single-entry file")
	}

	// All block index entries should point to index 0
	for i, idx := range reader.blockIndex {
		if idx != 0 {
			t.Errorf("blockIndex[%d] = %d, want 0", i, idx)
		}
	}
}

func TestGetEntry_PublicAPI(t *testing.T) {
	tmpDir := t.TempDir()
	numEntries := 5
	dedupPath := createTestDedupFile(t, tmpDir, numEntries)

	reader, err := NewReader(dedupPath, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	// Valid index should return true
	entry, ok := reader.GetEntry(0)
	if !ok {
		t.Fatal("GetEntry(0) returned false, want true")
	}
	if entry.MkvOffset < 0 {
		t.Errorf("GetEntry(0).MkvOffset = %d, want >= 0", entry.MkvOffset)
	}

	// Last valid index
	_, ok = reader.GetEntry(numEntries - 1)
	if !ok {
		t.Fatalf("GetEntry(%d) returned false, want true", numEntries-1)
	}

	// Out of range should return false
	_, ok = reader.GetEntry(numEntries)
	if ok {
		t.Error("GetEntry(numEntries) returned true, want false")
	}

	_, ok = reader.GetEntry(-1)
	if ok {
		t.Error("GetEntry(-1) returned true, want false")
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
