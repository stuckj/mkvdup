package dedup

import (
	"encoding/binary"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"

	"github.com/cespare/xxhash/v2"
)

// createBenchmarkDedupFile creates a valid .mkvdup file with source files for benchmarking.
// Unlike createTestDedupFile, this creates actual source files that ReadAt can read from.
func createBenchmarkDedupFile(b *testing.B, tmpDir string, numEntries int) (*Reader, func()) {
	b.Helper()

	// Create source directory with a source file
	sourceDir := filepath.Join(tmpDir, "source")
	if err := os.MkdirAll(filepath.Join(sourceDir, "VIDEO_TS"), 0755); err != nil {
		b.Fatalf("Failed to create source dir: %v", err)
	}

	// Calculate source file size: each entry maps to source at SourceOffset = i*100
	// with Length = 100 (fixed for simplicity), so we need at least numEntries*100 + 100 bytes
	sourceFileSize := int64(numEntries*100 + 100)

	// Create source file with predictable data
	sourceFilePath := "VIDEO_TS/VTS_01_1.VOB"
	sourceFullPath := filepath.Join(sourceDir, sourceFilePath)
	sourceData := make([]byte, sourceFileSize)
	for i := range sourceData {
		sourceData[i] = byte(i % 256)
	}
	if err := os.WriteFile(sourceFullPath, sourceData, 0644); err != nil {
		b.Fatalf("Failed to write source file: %v", err)
	}
	sourceChecksum := xxhash.Sum64(sourceData)

	// Create dedup file
	dedupPath := filepath.Join(tmpDir, "test.mkvdup")
	f, err := os.Create(dedupPath)
	if err != nil {
		b.Fatalf("Failed to create dedup file: %v", err)
	}
	defer f.Close()

	// Calculate sizes
	sourceFilesSize := int64(2 + len(sourceFilePath) + 8 + 8) // PathLen + Path + Size + Checksum
	indexSize := int64(numEntries) * EntrySize
	deltaOffset := int64(HeaderSize) + sourceFilesSize + indexSize
	deltaSize := int64(1000) // 1KB delta

	// Original MKV size: entries cover offsets 0 to numEntries*100+99
	originalSize := int64(numEntries*100 + 100)

	// Write header
	f.Write([]byte(Magic))
	binary.Write(f, binary.LittleEndian, uint32(Version))
	binary.Write(f, binary.LittleEndian, uint32(0))                // Flags
	binary.Write(f, binary.LittleEndian, originalSize)             // OriginalSize
	binary.Write(f, binary.LittleEndian, uint64(0x1234567890ABCD)) // OriginalChecksum
	binary.Write(f, binary.LittleEndian, uint8(SourceTypeDVD))     // SourceType
	binary.Write(f, binary.LittleEndian, uint8(0))                 // UsesESOffsets
	binary.Write(f, binary.LittleEndian, uint16(1))                // SourceFileCount
	binary.Write(f, binary.LittleEndian, uint64(numEntries))       // EntryCount
	binary.Write(f, binary.LittleEndian, deltaOffset)              // DeltaOffset
	binary.Write(f, binary.LittleEndian, deltaSize)                // DeltaSize

	// Write source file section
	binary.Write(f, binary.LittleEndian, uint16(len(sourceFilePath)))
	f.Write([]byte(sourceFilePath))
	binary.Write(f, binary.LittleEndian, sourceFileSize)
	binary.Write(f, binary.LittleEndian, sourceChecksum)

	// Write entries: contiguous chunks mapping MKV offset i*100 -> source offset i*100
	// Each entry: Length=100, Source=1 (first source file)
	indexHasher := xxhash.New()
	for i := 0; i < numEntries; i++ {
		entryBuf := make([]byte, EntrySize)
		binary.LittleEndian.PutUint64(entryBuf[0:8], uint64(i*100))   // MkvOffset
		binary.LittleEndian.PutUint64(entryBuf[8:16], uint64(100))    // Length
		binary.LittleEndian.PutUint16(entryBuf[16:18], uint16(1))     // Source (1 = first source file)
		binary.LittleEndian.PutUint64(entryBuf[18:26], uint64(i*100)) // SourceOffset
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
	binary.Write(f, binary.LittleEndian, indexChecksum)
	binary.Write(f, binary.LittleEndian, deltaChecksum)
	f.Write([]byte(Magic))

	// Sync to catch any write errors
	if err := f.Sync(); err != nil {
		b.Fatalf("Failed to sync dedup file: %v", err)
	}

	// Create reader
	reader, err := NewReader(dedupPath, sourceDir)
	if err != nil {
		b.Fatalf("Failed to create reader: %v", err)
	}

	// Load source files for ReadAt to work
	if err := reader.LoadSourceFiles(); err != nil {
		reader.Close()
		b.Fatalf("Failed to load source files: %v", err)
	}

	cleanup := func() {
		reader.Close()
	}

	return reader, cleanup
}

// BenchmarkGetEntry_Sequential benchmarks sequential entry access.
// This tests the effectiveness of the single-entry cache.
func BenchmarkGetEntry_Sequential(b *testing.B) {
	tmpDir := b.TempDir()
	numEntries := 100000 // 100K entries for realistic benchmark
	reader, cleanup := createBenchmarkDedupFile(b, tmpDir, numEntries)
	defer cleanup()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		idx := i % numEntries
		entry, ok := reader.getEntry(idx)
		if !ok {
			b.Fatalf("getEntry(%d) failed", idx)
		}
		// Prevent compiler optimization
		if entry.MkvOffset < 0 {
			b.Fatal("unexpected negative offset")
		}
	}
}

// BenchmarkGetEntry_Random benchmarks random entry access.
// This tests binary search performance without cache benefits.
func BenchmarkGetEntry_Random(b *testing.B) {
	tmpDir := b.TempDir()
	numEntries := 100000
	reader, cleanup := createBenchmarkDedupFile(b, tmpDir, numEntries)
	defer cleanup()

	// Pre-generate random indices to avoid rand overhead in benchmark
	indices := make([]int, b.N)
	rng := rand.New(rand.NewPCG(42, 0))
	for i := range indices {
		indices[i] = rng.IntN(numEntries)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		entry, ok := reader.getEntry(indices[i])
		if !ok {
			b.Fatalf("getEntry(%d) failed", indices[i])
		}
		if entry.MkvOffset < 0 {
			b.Fatal("unexpected negative offset")
		}
	}
}

// BenchmarkGetMkvOffset benchmarks the optimized MkvOffset-only read.
// Used in binary search, should be faster than full getEntry.
func BenchmarkGetMkvOffset(b *testing.B) {
	tmpDir := b.TempDir()
	numEntries := 100000
	reader, cleanup := createBenchmarkDedupFile(b, tmpDir, numEntries)
	defer cleanup()

	indices := make([]int, b.N)
	rng := rand.New(rand.NewPCG(42, 0))
	for i := range indices {
		indices[i] = rng.IntN(numEntries)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		offset, ok := reader.getMkvOffset(indices[i])
		if !ok {
			b.Fatalf("getMkvOffset(%d) failed", indices[i])
		}
		if offset < 0 {
			b.Fatal("unexpected negative offset")
		}
	}
}

// BenchmarkReadAt_Sequential benchmarks sequential reads in 64KB chunks.
// This simulates a video player reading a file from start to end.
func BenchmarkReadAt_Sequential(b *testing.B) {
	tmpDir := b.TempDir()
	numEntries := 10000 // 10K entries = ~1MB file
	reader, cleanup := createBenchmarkDedupFile(b, tmpDir, numEntries)
	defer cleanup()

	const chunkSize = 64 * 1024 // 64KB chunks
	buf := make([]byte, chunkSize)
	fileSize := reader.file.Header.OriginalSize

	b.ResetTimer()
	b.ReportAllocs()
	b.SetBytes(chunkSize)

	for i := 0; i < b.N; i++ {
		offset := int64((i * chunkSize) % int(fileSize))
		// Ensure we don't read past end of file
		if offset+chunkSize > fileSize {
			offset = 0
		}
		n, err := reader.ReadAt(buf, offset)
		if err != nil && n == 0 {
			b.Fatalf("ReadAt failed at offset %d: %v", offset, err)
		}
	}
}

// BenchmarkReadAt_Random benchmarks random 64KB reads.
// This simulates a video player seeking to random positions.
func BenchmarkReadAt_Random(b *testing.B) {
	tmpDir := b.TempDir()
	numEntries := 10000
	reader, cleanup := createBenchmarkDedupFile(b, tmpDir, numEntries)
	defer cleanup()

	const chunkSize = 64 * 1024
	buf := make([]byte, chunkSize)
	fileSize := reader.file.Header.OriginalSize

	// Pre-generate random offsets
	offsets := make([]int64, b.N)
	rng := rand.New(rand.NewPCG(42, 0))
	maxOffset := fileSize - chunkSize
	if maxOffset < 0 {
		maxOffset = 0
	}
	for i := range offsets {
		offsets[i] = rng.Int64N(maxOffset + 1)
	}

	b.ResetTimer()
	b.ReportAllocs()
	b.SetBytes(chunkSize)

	for i := 0; i < b.N; i++ {
		n, err := reader.ReadAt(buf, offsets[i])
		if err != nil && n == 0 {
			b.Fatalf("ReadAt failed at offset %d: %v", offsets[i], err)
		}
	}
}

// BenchmarkReadAt_Small benchmarks small reads (typical for container parsing).
// MKV parsers often read small chunks when parsing headers.
func BenchmarkReadAt_Small(b *testing.B) {
	tmpDir := b.TempDir()
	numEntries := 10000
	reader, cleanup := createBenchmarkDedupFile(b, tmpDir, numEntries)
	defer cleanup()

	const chunkSize = 256 // Small reads
	buf := make([]byte, chunkSize)
	fileSize := reader.file.Header.OriginalSize

	offsets := make([]int64, b.N)
	rng := rand.New(rand.NewPCG(42, 0))
	maxOffset := fileSize - chunkSize
	if maxOffset < 0 {
		maxOffset = 0
	}
	for i := range offsets {
		offsets[i] = rng.Int64N(maxOffset + 1)
	}

	b.ResetTimer()
	b.ReportAllocs()
	b.SetBytes(chunkSize)

	for i := 0; i < b.N; i++ {
		n, err := reader.ReadAt(buf, offsets[i])
		if err != nil && n == 0 {
			b.Fatalf("ReadAt failed: %v", err)
		}
	}
}

// BenchmarkFindEntriesForRange benchmarks the binary search for entry ranges.
// This is the core lookup operation for ReadAt.
func BenchmarkFindEntriesForRange(b *testing.B) {
	tmpDir := b.TempDir()
	numEntries := 100000
	reader, cleanup := createBenchmarkDedupFile(b, tmpDir, numEntries)
	defer cleanup()

	// Pre-generate random ranges
	type rangeQuery struct {
		offset int64
		length int64
	}
	queries := make([]rangeQuery, b.N)
	rng := rand.New(rand.NewPCG(42, 0))
	fileSize := reader.file.Header.OriginalSize
	const queryLen int64 = 1000
	length := queryLen
	if fileSize < length {
		length = fileSize
	}
	maxOffset := fileSize - length
	if maxOffset < 0 {
		maxOffset = 0
	}
	for i := range queries {
		var offset int64
		if maxOffset > 0 {
			offset = rng.Int64N(maxOffset)
		}
		queries[i] = rangeQuery{offset: offset, length: length}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		entries := reader.findEntriesForRange(queries[i].offset, queries[i].length)
		if len(entries) == 0 {
			b.Fatal("findEntriesForRange returned no entries")
		}
	}
}

// BenchmarkNewReader benchmarks reader initialization time.
// This measures the overhead of opening a dedup file.
func BenchmarkNewReader(b *testing.B) {
	tmpDir := b.TempDir()

	// Create test file once
	numEntries := 100000
	dedupPath := filepath.Join(tmpDir, "test.mkvdup")
	sourceDir := filepath.Join(tmpDir, "source")

	// Use the existing createBenchmarkDedupFile to set up, then close it
	_, cleanup := createBenchmarkDedupFile(b, tmpDir, numEntries)
	cleanup()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		r, err := NewReader(dedupPath, sourceDir)
		if err != nil {
			b.Fatalf("NewReader failed: %v", err)
		}
		r.Close()
	}
}

// BenchmarkNewReaderLazy benchmarks lazy reader initialization.
// Should be faster than NewReader since entries aren't parsed.
func BenchmarkNewReaderLazy(b *testing.B) {
	tmpDir := b.TempDir()
	numEntries := 100000
	_, cleanup := createBenchmarkDedupFile(b, tmpDir, numEntries)
	cleanup()

	dedupPath := filepath.Join(tmpDir, "test.mkvdup")
	sourceDir := filepath.Join(tmpDir, "source")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		r, err := NewReaderLazy(dedupPath, sourceDir)
		if err != nil {
			b.Fatalf("NewReaderLazy failed: %v", err)
		}
		r.Close()
	}
}
