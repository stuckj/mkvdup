package matcher

import (
	"math/rand/v2"
	"sort"
	"testing"

	"github.com/cespare/xxhash/v2"
	"github.com/stuckj/mkvdup/internal/source"
)

// createSyntheticPacketData creates packet data with sync points for benchmarking.
func createSyntheticPacketData(size int, isVideo bool, syncInterval int) []byte {
	data := make([]byte, size)
	rng := rand.New(rand.NewPCG(42, 0))
	for i := range data {
		data[i] = byte(rng.IntN(256))
	}

	if isVideo {
		// Insert video start codes (00 00 01 XX)
		for i := 0; i+4 <= size; i += syncInterval {
			data[i] = 0x00
			data[i+1] = 0x00
			data[i+2] = 0x01
			data[i+3] = 0xE0
		}
	} else {
		// Insert AC3 sync (0B 77)
		for i := 0; i+2 <= size; i += syncInterval {
			data[i] = 0x0B
			data[i+1] = 0x77
		}
	}

	return data
}

// BenchmarkExtractProbeHashes benchmarks hash extraction from packet data.
func BenchmarkExtractProbeHashes(b *testing.B) {
	testCases := []struct {
		name     string
		size     int
		isVideo  bool
		interval int
	}{
		{"Video_1KB", 1024, true, 256},
		{"Video_4KB", 4096, true, 512},
		{"Audio_1KB", 1024, false, 256},
		{"Audio_4KB", 4096, false, 512},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			data := createSyntheticPacketData(tc.size, tc.isVideo, tc.interval)
			windowSize := 64

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				hashes := ExtractProbeHashes(data, tc.isVideo, windowSize)
				if len(hashes) == 0 {
					b.Fatal("expected to find hashes")
				}
			}
		})
	}
}

// BenchmarkExtractProbeHashes_NoSync benchmarks hash extraction when no sync points exist.
func BenchmarkExtractProbeHashes_NoSync(b *testing.B) {
	// Create data without sync points
	data := make([]byte, 4096)
	rng := rand.New(rand.NewPCG(42, 0))
	for i := range data {
		// Avoid creating accidental sync patterns
		val := byte(rng.IntN(256))
		if val == 0x00 || val == 0x0B || val == 0x77 || val == 0xFF || val == 0x7F {
			val = 0x42
		}
		data[i] = val
	}
	windowSize := 64

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		hashes := ExtractProbeHashes(data, true, windowSize)
		if len(hashes) == 0 {
			b.Fatal("expected to find at least one hash")
		}
	}
}

// BenchmarkHashLookup benchmarks the hash lookup in a source index.
func BenchmarkHashLookup(b *testing.B) {
	// Create a source index with synthetic hashes
	idx := &source.Index{
		WindowSize:      64,
		HashToLocations: make(map[uint64][]source.Location),
		SourceDir:       "/test",
	}

	// Populate with 10000 hashes
	rng := rand.New(rand.NewPCG(42, 0))
	for range 10000 {
		hash := rng.Uint64()
		idx.HashToLocations[hash] = []source.Location{
			{FileIndex: 0, Offset: rng.Int64N(1000000)},
		}
	}

	// Create some hashes we'll look up (half exist, half don't)
	lookupHashes := make([]uint64, 1000)
	for i := range lookupHashes {
		if i%2 == 0 {
			// Pick an existing hash
			for h := range idx.HashToLocations {
				lookupHashes[i] = h
				break
			}
		} else {
			// Generate a random hash (likely doesn't exist)
			lookupHashes[i] = rng.Uint64()
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		for _, h := range lookupHashes {
			_ = idx.Lookup(h)
		}
	}
}

// BenchmarkMergeRegions benchmarks region merging logic.
// This tests the algorithm used after parallel matching completes.
func BenchmarkMergeRegions(b *testing.B) {
	testCases := []struct {
		name       string
		numRegions int
		overlap    float64 // Fraction of regions that overlap
	}{
		{"100_NoOverlap", 100, 0.0},
		{"100_50pctOverlap", 100, 0.5},
		{"1000_NoOverlap", 1000, 0.0},
		{"1000_50pctOverlap", 1000, 0.5},
		{"10000_10pctOverlap", 10000, 0.1},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			// Generate regions with specified overlap characteristics
			regions := generateTestRegions(tc.numRegions, tc.overlap)

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				// Copy regions since merge modifies in place
				workingRegions := make([]matchedRegion, len(regions))
				copy(workingRegions, regions)

				// Simulate the merge algorithm
				mergeTestRegions(workingRegions)
			}
		})
	}
}

// generateTestRegions creates test regions with specified overlap.
func generateTestRegions(count int, overlapFraction float64) []matchedRegion {
	regions := make([]matchedRegion, count)
	rng := rand.New(rand.NewPCG(42, 0))

	baseOffset := int64(0)
	avgRegionSize := int64(10000) // 10KB average

	for i := range regions {
		size := avgRegionSize + rng.Int64N(5000) - 2500 // 7.5KB to 12.5KB

		// Decide if this region should overlap with previous
		if i > 0 && rng.Float64() < overlapFraction {
			// Overlap: start within the previous region
			prevEnd := regions[i-1].mkvEnd
			overlapAmount := rng.Int64N(size/2) + 1
			baseOffset = prevEnd - overlapAmount
		}

		regions[i] = matchedRegion{
			mkvStart:  baseOffset,
			mkvEnd:    baseOffset + size,
			fileIndex: 0,
			srcOffset: rng.Int64N(1000000),
		}

		baseOffset += size + rng.Int64N(1000) // Gap between regions
	}

	return regions
}

// mergeTestRegions implements the same algorithm as Matcher.mergeRegions for benchmarking.
func mergeTestRegions(regions []matchedRegion) []matchedRegion {
	if len(regions) == 0 {
		return regions
	}

	// Sort by start offset
	sort.Slice(regions, func(i, j int) bool {
		return regions[i].mkvStart < regions[j].mkvStart
	})

	// Merge overlapping regions
	merged := []matchedRegion{regions[0]}
	for i := 1; i < len(regions); i++ {
		curr := regions[i]
		last := &merged[len(merged)-1]

		if curr.mkvStart <= last.mkvEnd {
			if curr.mkvEnd > last.mkvEnd {
				if (curr.mkvEnd - curr.mkvStart) > (last.mkvEnd - last.mkvStart) {
					*last = curr
				} else {
					last.mkvEnd = curr.mkvEnd
				}
			}
		} else {
			merged = append(merged, curr)
		}
	}

	return merged
}

// BenchmarkXXHash benchmarks the xxhash function used for packet hashing.
func BenchmarkXXHash(b *testing.B) {
	testCases := []struct {
		name string
		size int
	}{
		{"64B", 64},
		{"256B", 256},
		{"1KB", 1024},
		{"4KB", 4096},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			data := make([]byte, tc.size)
			rng := rand.New(rand.NewPCG(42, 0))
			for i := range data {
				data[i] = byte(rng.IntN(256))
			}

			b.ReportAllocs()
			b.SetBytes(int64(tc.size))
			b.ResetTimer()
			for b.Loop() {
				_ = xxhash.Sum64(data)
			}
		})
	}
}
