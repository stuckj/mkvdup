package matcher

import (
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"

	"github.com/cespare/xxhash/v2"
	"github.com/stuckj/mkvdup/internal/mkv"
	"github.com/stuckj/mkvdup/internal/source"
)

// generateDeterminismTestData creates synthetic MKV and source data for
// determinism testing. Uses the same data generation approach as the
// benchmark but returns paths and structures for Match() calls.
func generateDeterminismTestData(t *testing.T) (string, []mkv.Packet, []mkv.Track, *source.Index) {
	t.Helper()

	windowSize := 64
	rng := rand.New(rand.NewPCG(42, 0))

	numVideoFrames := 200
	numAudioFrames := 200

	type esChunk struct {
		data    []byte
		isVideo bool
		trackID uint64
	}

	chunks := make([]esChunk, 0, numVideoFrames+numAudioFrames)

	for i := range numVideoFrames {
		var size int
		r := rng.Float64()
		switch {
		case r < 0.05:
			size = 80*1024 + rng.IntN(110*1024)
		case r < 0.35:
			size = 30*1024 + rng.IntN(50*1024)
		case r < 0.70:
			size = 10*1024 + rng.IntN(20*1024)
		default:
			size = 1024 + rng.IntN(9*1024)
		}
		data := make([]byte, size)
		for j := range data {
			data[j] = byte(rng.IntN(256))
		}
		data[0] = 0x00
		data[1] = 0x00
		data[2] = 0x01
		data[3] = byte(0xE0 + (i % 16))
		chunks = append(chunks, esChunk{data: data, isVideo: true, trackID: 1})
	}

	for range numAudioFrames {
		size := 384 + rng.IntN(256)
		data := make([]byte, size)
		for j := range data {
			data[j] = byte(rng.IntN(256))
		}
		data[0] = 0x0B
		data[1] = 0x77
		chunks = append(chunks, esChunk{data: data, isVideo: false, trackID: 2})
	}

	rng.Shuffle(len(chunks), func(i, j int) {
		chunks[i], chunks[j] = chunks[j], chunks[i]
	})

	totalESSize := 0
	for _, c := range chunks {
		totalESSize += len(c.data)
	}

	pesOverhead := 24
	sourceSize := totalESSize + len(chunks)*pesOverhead
	sourceData := make([]byte, sourceSize)
	for i := range sourceData {
		val := byte((i*7)%200) + 50
		if val == 0x00 || val == 0x01 {
			val = 0x42
		}
		sourceData[i] = val
	}

	type chunkPlacement struct {
		chunk        esChunk
		sourceOffset int64
	}
	placements := make([]chunkPlacement, len(chunks))
	sourceOffset := int64(pesOverhead)
	for i, chunk := range chunks {
		copy(sourceData[sourceOffset:], chunk.data)
		placements[i] = chunkPlacement{chunk: chunk, sourceOffset: sourceOffset}
		sourceOffset += int64(len(chunk.data)) + int64(pesOverhead)
	}

	mkvHeaderSize := 8
	mkvSize := totalESSize*2 + len(chunks)*mkvHeaderSize*2
	mkvData := make([]byte, mkvSize)
	for i := range mkvData {
		val := byte((i*11)%200) + 50
		if val == 0x00 || val == 0x01 || val == 0x0B || val == 0x77 {
			val = 0x42
		}
		mkvData[i] = val
	}

	packets := make([]mkv.Packet, 0, len(chunks)*2)
	mkvOffset := int64(mkvHeaderSize)
	for i, placement := range placements {
		chunk := placement.chunk
		chunkData := chunk.data
		splitStrategy := rng.Float64()

		if splitStrategy < 0.25 && len(chunkData) > 4096 {
			splitPoint := len(chunkData)/3 + rng.IntN(len(chunkData)/3)
			copy(mkvData[mkvOffset:], chunkData[:splitPoint])
			packets = append(packets, mkv.Packet{
				Offset: mkvOffset, Size: int64(splitPoint),
				TrackNum: chunk.trackID, Keyframe: chunk.isVideo && i%30 == 0,
			})
			mkvOffset += int64(splitPoint) + int64(mkvHeaderSize)
			copy(mkvData[mkvOffset:], chunkData[splitPoint:])
			packets = append(packets, mkv.Packet{
				Offset: mkvOffset, Size: int64(len(chunkData) - splitPoint),
				TrackNum: chunk.trackID,
			})
			mkvOffset += int64(len(chunkData)-splitPoint) + int64(mkvHeaderSize)
		} else if splitStrategy < 0.40 && i < len(placements)-1 && chunk.isVideo {
			nextChunk := placements[i+1].chunk
			takeFromNext := min(len(nextChunk.data)/4, 1024)
			totalSize := len(chunkData) + takeFromNext
			copy(mkvData[mkvOffset:], chunkData)
			copy(mkvData[mkvOffset+int64(len(chunkData)):], nextChunk.data[:takeFromNext])
			packets = append(packets, mkv.Packet{
				Offset: mkvOffset, Size: int64(totalSize),
				TrackNum: chunk.trackID, Keyframe: chunk.isVideo && i%30 == 0,
			})
			mkvOffset += int64(totalSize) + int64(mkvHeaderSize)
		} else {
			copy(mkvData[mkvOffset:], chunkData)
			packets = append(packets, mkv.Packet{
				Offset: mkvOffset, Size: int64(len(chunkData)),
				TrackNum: chunk.trackID, Keyframe: chunk.isVideo && i%30 == 0,
			})
			mkvOffset += int64(len(chunkData)) + int64(mkvHeaderSize)
		}
	}

	idx := source.NewIndex("/test", source.TypeDVD, windowSize)
	idx.UsesESOffsets = false
	idx.Files = []source.File{{RelativePath: "source.vob", Size: int64(len(sourceData))}}
	idx.RawReaders = []source.RawReader{&sliceReader{data: sourceData}}

	for _, placement := range placements {
		if len(placement.chunk.data) >= windowSize {
			window := sourceData[placement.sourceOffset : placement.sourceOffset+int64(windowSize)]
			hash := xxhash.Sum64(window)
			idx.HashToLocations[hash] = append(idx.HashToLocations[hash], source.Location{
				FileIndex: 0, Offset: placement.sourceOffset, IsVideo: placement.chunk.isVideo,
			})
		}
	}
	idx.SortLocationsByOffset()

	tmpDir := t.TempDir()
	mkvPath := filepath.Join(tmpDir, "test.mkv")
	if err := os.WriteFile(mkvPath, mkvData[:mkvOffset], 0644); err != nil {
		t.Fatalf("write MKV: %v", err)
	}

	tracks := []mkv.Track{
		{Number: 1, Type: mkv.TrackTypeVideo, CodecID: "V_MPEG2"},
		{Number: 2, Type: mkv.TrackTypeAudio, CodecID: "A_AC3"},
	}

	return mkvPath, packets, tracks, idx
}

// TestMatchDeterminism verifies that matching the same inputs with different
// worker counts produces identical results every time.
//
// Note: This test uses UsesESOffsets=false (raw offsets), so the inter-batch
// edge sync path (which requires UsesESOffsets for tryLocalityMatch) is not
// exercised here. That path is validated by real-world Blu-ray tests.
func TestMatchDeterminism(t *testing.T) {
	mkvPath, packets, tracks, idx := generateDeterminismTestData(t)

	workerCounts := []int{1, 2, 4, 8}

	var referenceEntries []Entry
	var referenceMatchedBytes int64
	var referenceUnmatchedBytes int64
	var referenceMatchedPackets int

	for _, workers := range workerCounts {
		// Run twice with same worker count to verify intra-count determinism
		for run := 0; run < 2; run++ {
			m, err := NewMatcher(idx)
			if err != nil {
				t.Fatalf("NewMatcher: %v", err)
			}
			m.SetNumWorkers(workers)

			result, err := m.Match(mkvPath, packets, tracks, nil)
			if err != nil {
				m.Close()
				t.Fatalf("Match(workers=%d, run=%d): %v", workers, run, err)
			}

			if result.MatchedBytes == 0 {
				result.Close()
				m.Close()
				t.Fatalf("Match(workers=%d, run=%d): no bytes matched", workers, run)
			}

			if referenceEntries == nil {
				// First run: capture reference
				referenceEntries = result.Entries
				referenceMatchedBytes = result.MatchedBytes
				referenceUnmatchedBytes = result.UnmatchedBytes
				referenceMatchedPackets = result.MatchedPackets
				result.Close()
				m.Close()
				continue
			}

			// Compare with reference
			if result.MatchedBytes != referenceMatchedBytes {
				t.Errorf("workers=%d run=%d: MatchedBytes=%d, want %d",
					workers, run, result.MatchedBytes, referenceMatchedBytes)
			}
			if result.UnmatchedBytes != referenceUnmatchedBytes {
				t.Errorf("workers=%d run=%d: UnmatchedBytes=%d, want %d",
					workers, run, result.UnmatchedBytes, referenceUnmatchedBytes)
			}
			if result.MatchedPackets != referenceMatchedPackets {
				t.Errorf("workers=%d run=%d: MatchedPackets=%d, want %d",
					workers, run, result.MatchedPackets, referenceMatchedPackets)
			}
			if len(result.Entries) != len(referenceEntries) {
				t.Fatalf("workers=%d run=%d: len(Entries)=%d, want %d",
					workers, run, len(result.Entries), len(referenceEntries))
			}
			for i, got := range result.Entries {
				want := referenceEntries[i]
				if got != want {
					t.Errorf("workers=%d run=%d: Entries[%d] = %+v, want %+v",
						workers, run, i, got, want)
					break
				}
			}

			result.Close()
			m.Close()
		}
	}
}
