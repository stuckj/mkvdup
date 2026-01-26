package matcher

import (
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/cespare/xxhash/v2"
	"github.com/stuckj/mkvdup/internal/mkv"
	"github.com/stuckj/mkvdup/internal/source"
)

// sliceReader implements source.RawReader for in-memory data.
type sliceReader struct {
	data []byte
}

func (r *sliceReader) ReadAt(buf []byte, offset int64) (int, error) {
	if offset >= int64(len(r.data)) {
		return 0, nil
	}
	n := copy(buf, r.data[offset:])
	return n, nil
}

func (r *sliceReader) Slice(offset int64, size int) []byte {
	if offset < 0 || offset >= int64(len(r.data)) {
		return nil
	}
	end := offset + int64(size)
	if end > int64(len(r.data)) {
		end = int64(len(r.data))
	}
	return r.data[offset:end]
}

func (r *sliceReader) Len() int {
	return len(r.data)
}

func (r *sliceReader) Close() error {
	return nil
}

// matchBenchmarkData holds pre-generated data for matching benchmarks.
// This is generated once and reused across benchmark iterations.
type matchBenchmarkData struct {
	sourceData []byte        // Raw source file data
	mkvData    []byte        // MKV file data (overlaps with source)
	mkvPath    string        // Path to temp MKV file
	packets    []mkv.Packet  // MKV packets pointing into mkvData
	tracks     []mkv.Track   // MKV tracks
	index      *source.Index // Source index with hashes
	windowSize int
}

// esChunk represents a single ES data chunk with metadata.
type esChunk struct {
	data    []byte
	isVideo bool
	trackID uint64
}

// generateMatchBenchmarkData creates realistic test data modeled after Big Buck Bunny:
// - Video packets: 5KB to 100KB (I-frames up to 190KB, P/B-frames smaller)
// - Audio packets: 300-600 bytes (AC3 frames ~400-500 bytes)
// - Source contains ES data at various offsets (simulating PES structure in VOB)
// - MKV contains the SAME ES bytes but with different packet boundaries
// - Packets don't align with source chunks (exercises expansion logic)
func generateMatchBenchmarkData(b *testing.B, numVideoFrames, numAudioFrames int) *matchBenchmarkData {
	b.Helper()

	windowSize := 64
	rng := rand.New(rand.NewPCG(42, 0))

	// Generate ES chunks with realistic sizes based on Big Buck Bunny analysis:
	// - Video: min 1KB, max 190KB, median ~11KB, many in 10-50KB range
	// - Audio: 300-600 bytes (AC3 frames)
	chunks := make([]esChunk, 0, numVideoFrames+numAudioFrames)

	// Generate video frames with realistic size distribution
	for i := range numVideoFrames {
		var size int
		r := rng.Float64()
		switch {
		case r < 0.05: // 5% I-frames: 80-190KB
			size = 80*1024 + rng.IntN(110*1024)
		case r < 0.35: // 30% large P-frames: 30-80KB
			size = 30*1024 + rng.IntN(50*1024)
		case r < 0.70: // 35% medium frames: 10-30KB
			size = 10*1024 + rng.IntN(20*1024)
		default: // 30% small B-frames: 1-10KB
			size = 1024 + rng.IntN(9*1024)
		}

		data := make([]byte, size)
		for j := range data {
			data[j] = byte(rng.IntN(256))
		}
		// Insert video start code at beginning (00 00 01 Ex for video)
		data[0] = 0x00
		data[1] = 0x00
		data[2] = 0x01
		data[3] = byte(0xE0 + (i % 16))

		chunks = append(chunks, esChunk{data: data, isVideo: true, trackID: 1})
	}

	// Generate audio frames with realistic AC3 sizes
	for range numAudioFrames {
		// AC3 frames: typically 384-640 bytes depending on bitrate
		size := 384 + rng.IntN(256)
		data := make([]byte, size)
		for j := range data {
			data[j] = byte(rng.IntN(256))
		}
		// Insert AC3 sync word at beginning
		data[0] = 0x0B
		data[1] = 0x77

		chunks = append(chunks, esChunk{data: data, isVideo: false, trackID: 2})
	}

	// Interleave video and audio chunks (simulating real stream order)
	// In DVD, audio and video are interleaved in PES packets
	rng.Shuffle(len(chunks), func(i, j int) {
		chunks[i], chunks[j] = chunks[j], chunks[i]
	})

	// Calculate total ES size
	totalESSize := 0
	for _, c := range chunks {
		totalESSize += len(c.data)
	}

	// Create source data: ES chunks with PES header gaps between them
	// PES headers in MPEG-PS are typically 9-14 bytes, but we use larger gaps
	// to simulate the pack headers (14 bytes) + PES headers
	pesOverhead := 24 // Pack header + PES header overhead
	sourceSize := totalESSize + len(chunks)*pesOverhead
	sourceData := make([]byte, sourceSize)

	// Fill with non-matching PES header bytes (avoid accidental sync patterns)
	for i := range sourceData {
		val := byte((i*7)%200) + 50
		// Avoid creating accidental start codes
		if val == 0x00 || val == 0x01 {
			val = 0x42
		}
		sourceData[i] = val
	}

	// Place ES chunks in source and track their positions
	type chunkPlacement struct {
		chunk        esChunk
		sourceOffset int64
	}
	placements := make([]chunkPlacement, len(chunks))

	sourceOffset := int64(pesOverhead)
	for i, chunk := range chunks {
		copy(sourceData[sourceOffset:], chunk.data)
		placements[i] = chunkPlacement{
			chunk:        chunk,
			sourceOffset: sourceOffset,
		}
		sourceOffset += int64(len(chunk.data)) + int64(pesOverhead)
	}

	// Create MKV data: same ES bytes but different packet boundaries
	// MKV SimpleBlock header is ~4-8 bytes typically
	mkvHeaderSize := 8
	// Account for: split packets (extra headers), combine overlaps (extra 1KB per combine)
	// Use 2x total ES size to be safe since we may duplicate some data in combine cases
	mkvSize := totalESSize*2 + len(chunks)*mkvHeaderSize*2
	mkvData := make([]byte, mkvSize)

	// Fill with non-matching header data
	for i := range mkvData {
		val := byte((i*11)%200) + 50
		if val == 0x00 || val == 0x01 || val == 0x0B || val == 0x77 {
			val = 0x42
		}
		mkvData[i] = val
	}

	// Create MKV packets - sometimes split ES chunks, sometimes combine them
	packets := make([]mkv.Packet, 0, len(chunks)*2)
	mkvOffset := int64(mkvHeaderSize)

	for i, placement := range placements {
		chunk := placement.chunk
		chunkData := chunk.data

		// Decide how to handle this chunk:
		// - 60% of time: one packet per ES chunk (normal case)
		// - 25% of time: split large chunks into 2 packets
		// - 15% of time: combine with part of next chunk (if video)
		splitStrategy := rng.Float64()

		if splitStrategy < 0.25 && len(chunkData) > 4096 {
			// Split into 2 packets at a random point
			splitPoint := len(chunkData)/3 + rng.IntN(len(chunkData)/3)

			// First packet
			copy(mkvData[mkvOffset:], chunkData[:splitPoint])
			packets = append(packets, mkv.Packet{
				Offset:   mkvOffset,
				Size:     int64(splitPoint),
				TrackNum: chunk.trackID,
				Keyframe: chunk.isVideo && i%30 == 0,
			})
			mkvOffset += int64(splitPoint) + int64(mkvHeaderSize)

			// Second packet
			copy(mkvData[mkvOffset:], chunkData[splitPoint:])
			packets = append(packets, mkv.Packet{
				Offset:   mkvOffset,
				Size:     int64(len(chunkData) - splitPoint),
				TrackNum: chunk.trackID,
				Keyframe: false,
			})
			mkvOffset += int64(len(chunkData)-splitPoint) + int64(mkvHeaderSize)

		} else if splitStrategy < 0.40 && i < len(placements)-1 && chunk.isVideo {
			// Combine with part of next chunk (only for video)
			nextChunk := placements[i+1].chunk
			takeFromNext := min(len(nextChunk.data)/4, 1024) // Take up to 1KB from next

			totalSize := len(chunkData) + takeFromNext
			copy(mkvData[mkvOffset:], chunkData)
			copy(mkvData[mkvOffset+int64(len(chunkData)):], nextChunk.data[:takeFromNext])

			packets = append(packets, mkv.Packet{
				Offset:   mkvOffset,
				Size:     int64(totalSize),
				TrackNum: chunk.trackID,
				Keyframe: chunk.isVideo && i%30 == 0,
			})
			mkvOffset += int64(totalSize) + int64(mkvHeaderSize)

		} else {
			// Normal: one packet per chunk
			copy(mkvData[mkvOffset:], chunkData)
			packets = append(packets, mkv.Packet{
				Offset:   mkvOffset,
				Size:     int64(len(chunkData)),
				TrackNum: chunk.trackID,
				Keyframe: chunk.isVideo && i%30 == 0,
			})
			mkvOffset += int64(len(chunkData)) + int64(mkvHeaderSize)
		}
	}

	// Build source index with hashes at sync points
	idx := source.NewIndex("/benchmark", source.TypeDVD, windowSize)
	idx.UsesESOffsets = false // Using raw offsets
	idx.Files = []source.File{
		{RelativePath: "source.vob", Size: int64(len(sourceData))},
	}
	idx.RawReaders = []source.RawReader{&sliceReader{data: sourceData}}

	// Index hashes at sync points in source
	for _, placement := range placements {
		if len(placement.chunk.data) >= windowSize {
			window := sourceData[placement.sourceOffset : placement.sourceOffset+int64(windowSize)]
			hash := xxhash.Sum64(window)
			idx.HashToLocations[hash] = append(idx.HashToLocations[hash], source.Location{
				FileIndex: 0,
				Offset:    placement.sourceOffset,
				IsVideo:   placement.chunk.isVideo,
			})
		}
	}

	// Write MKV to temp file (Matcher.Match requires file path for mmap)
	tmpDir := b.TempDir()
	mkvPath := filepath.Join(tmpDir, "benchmark.mkv")
	if err := os.WriteFile(mkvPath, mkvData[:mkvOffset], 0644); err != nil {
		b.Fatalf("Failed to write MKV file: %v", err)
	}

	tracks := []mkv.Track{
		{Number: 1, Type: mkv.TrackTypeVideo, CodecID: "V_MPEG2"},
		{Number: 2, Type: mkv.TrackTypeAudio, CodecID: "A_AC3"},
	}

	return &matchBenchmarkData{
		sourceData: sourceData,
		mkvData:    mkvData[:mkvOffset],
		mkvPath:    mkvPath,
		packets:    packets,
		tracks:     tracks,
		index:      idx,
		windowSize: windowSize,
	}
}

// BenchmarkMatch_EndToEnd benchmarks the full matching pipeline with realistic data.
// This exercises the critical path: sync detection, hash lookup, verification, and expansion.
func BenchmarkMatch_EndToEnd(b *testing.B) {
	testCases := []struct {
		name           string
		numVideoFrames int
		numAudioFrames int
	}{
		// Small: ~5 seconds of video at 25fps, matching audio frames
		{"Small_125V_125A", 125, 125},
		// Medium: ~20 seconds of video
		{"Medium_500V_500A", 500, 500},
		// Large: ~40 seconds of video
		{"Large_1000V_1000A", 1000, 1000},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			// Generate data once before benchmarking
			data := generateMatchBenchmarkData(b, tc.numVideoFrames, tc.numAudioFrames)

			b.ResetTimer()
			b.ReportAllocs()
			b.SetBytes(int64(len(data.mkvData)))

			for b.Loop() {
				matcher, err := NewMatcher(data.index)
				if err != nil {
					b.Fatalf("NewMatcher failed: %v", err)
				}
				matcher.SetNumWorkers(1) // Single worker for consistent benchmarks

				result, err := matcher.Match(data.mkvPath, data.packets, data.tracks, nil)
				if err != nil {
					matcher.Close()
					b.Fatalf("Match failed: %v", err)
				}

				// Verify we actually matched something
				if result.MatchedBytes == 0 {
					matcher.Close()
					b.Fatalf("No bytes matched - test data may be misconfigured")
				}

				matcher.Close()
			}
		})
	}
}

// BenchmarkMatch_Parallel benchmarks matching with multiple workers.
func BenchmarkMatch_Parallel(b *testing.B) {
	// Generate medium-sized data (500 video frames, 500 audio frames)
	data := generateMatchBenchmarkData(b, 500, 500)

	workerCounts := []int{1, 2, 4, 8}

	for _, workers := range workerCounts {
		b.Run(fmt.Sprintf("Workers_%d", workers), func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			b.SetBytes(int64(len(data.mkvData)))

			for b.Loop() {
				matcher, err := NewMatcher(data.index)
				if err != nil {
					b.Fatalf("NewMatcher failed: %v", err)
				}
				matcher.SetNumWorkers(workers)

				result, err := matcher.Match(data.mkvPath, data.packets, data.tracks, nil)
				if err != nil {
					matcher.Close()
					b.Fatalf("Match failed: %v", err)
				}

				if result.MatchedBytes == 0 {
					matcher.Close()
					b.Fatalf("No bytes matched")
				}

				matcher.Close()
			}
		})
	}
}

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
		SourceDir:       "/benchmark",
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
