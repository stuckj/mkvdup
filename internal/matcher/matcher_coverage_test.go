package matcher

import (
	"testing"

	"github.com/stuckj/mkvdup/internal/source"
)

// isRangeCoveredGlobal checks if a range is covered in the global coverage bitmap.
// Test-only helper for validating markChunksCovered writes.
func (m *Matcher) isRangeCoveredGlobal(offset, size int64) bool {
	startChunk := offset / coverageChunkSize
	endChunk := (offset + size - 1) / coverageChunkSize

	m.coverageMu.RLock()
	defer m.coverageMu.RUnlock()

	for chunk := startChunk; chunk <= endChunk; chunk++ {
		wordIdx := chunk / 64
		bitIdx := uint(chunk % 64)
		if wordIdx >= int64(len(m.coveredChunks)) {
			return false
		}
		if m.coveredChunks[wordIdx]&(1<<bitIdx) == 0 {
			return false
		}
	}
	return true
}

func TestCoverageBitmap(t *testing.T) {
	// Create a matcher with minimal setup
	idx := source.NewIndex("/test", source.TypeDVD, 64)
	m, _ := NewMatcher(idx)

	// Simulate MKV size of 100KB
	m.mkvSize = 100 * 1024

	// Initialize coverage bitmap (normally done in Match)
	numChunks := (m.mkvSize + coverageChunkSize - 1) / coverageChunkSize
	m.coveredChunks = make([]uint64, (numChunks+63)/64)

	// Test marking chunks covered
	// Mark region from 8192 to 16384 (exactly 2 chunks at 4KB granularity)
	m.markChunksCovered(8192, 16384)

	// Chunks 2 and 3 should be fully covered (chunk 2 = [8192, 12288), chunk 3 = [12288, 16384))
	// isRangeCoveredGlobal checks if ALL chunks in the range are covered
	if !m.isRangeCoveredGlobal(8192, 4096) {
		t.Error("Range [8192, 12288) should be covered")
	}
	if !m.isRangeCoveredGlobal(12288, 4096) {
		t.Error("Range [12288, 16384) should be covered")
	}

	// Range spanning chunks 2-3 should be covered
	if !m.isRangeCoveredGlobal(8192, 8192) {
		t.Error("Range [8192, 16384) should be covered")
	}

	// Chunk 1 (before the marked region) should not be covered
	if m.isRangeCoveredGlobal(4096, 4096) {
		t.Error("Range [4096, 8192) should NOT be covered")
	}

	// Chunk 4 (after the marked region) should not be covered
	if m.isRangeCoveredGlobal(16384, 4096) {
		t.Error("Range [16384, 20480) should NOT be covered")
	}
}

func TestCoverageBitmap_PartialChunks(t *testing.T) {
	idx := source.NewIndex("/test", source.TypeDVD, 64)
	m, _ := NewMatcher(idx)

	m.mkvSize = 100 * 1024
	numChunks := (m.mkvSize + coverageChunkSize - 1) / coverageChunkSize
	m.coveredChunks = make([]uint64, (numChunks+63)/64)

	// Mark a region that doesn't fully cover any chunks
	// Region [5000, 7000) is within chunk 1 [4096, 8192) but doesn't fully contain it
	m.markChunksCovered(5000, 7000)

	// No chunks should be marked as covered since the region
	// doesn't fully contain any chunk
	if m.isRangeCoveredGlobal(4096, 4096) {
		t.Error("Partial coverage should not mark chunk as covered")
	}
}

func TestCoverageBitmap_MarkMultipleChunks(t *testing.T) {
	idx := source.NewIndex("/test", source.TypeDVD, 64)
	m, _ := NewMatcher(idx)

	m.mkvSize = 100 * 1024
	numChunks := (m.mkvSize + coverageChunkSize - 1) / coverageChunkSize
	m.coveredChunks = make([]uint64, (numChunks+63)/64)

	// Mark region {0, 40960} which is exactly 10 chunks (0 through 9)
	m.markChunksCovered(0, 40960)

	// Verify each individual chunk in the range is covered
	for i := int64(0); i < 10; i++ {
		offset := i * coverageChunkSize
		if !m.isRangeCoveredGlobal(offset, coverageChunkSize) {
			t.Errorf("chunk %d at offset %d should be covered", i, offset)
		}
	}

	// Chunk 10 should not be covered
	if m.isRangeCoveredGlobal(40960, coverageChunkSize) {
		t.Error("chunk 10 at offset 40960 should NOT be covered")
	}
}

func TestCoverageBitmap_EmptyBitmapUncovered(t *testing.T) {
	idx := source.NewIndex("/test", source.TypeDVD, 64)
	m, _ := NewMatcher(idx)

	m.mkvSize = 100 * 1024
	numChunks := (m.mkvSize + coverageChunkSize - 1) / coverageChunkSize
	m.coveredChunks = make([]uint64, (numChunks+63)/64)

	// Fresh bitmap with no marks - first chunk should not be covered
	if m.isRangeCoveredGlobal(0, coverageChunkSize) {
		t.Error("fresh bitmap should report range as uncovered")
	}
}
