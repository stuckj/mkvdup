package source

import (
	"fmt"

	"github.com/cespare/xxhash/v2"
)

// esDataProvider is the interface needed by indexESData and indexAudioSubStream.
// Both MPEGPSParser and MPEGTSParser implement this, as well as isoM2TSAdapter.
type esDataProvider interface {
	Data() []byte
	DataSlice(off int64, size int) []byte
	DataSize() int64
	FilteredVideoRanges() []PESPayloadRange
	FilteredAudioRanges(subStreamID byte) []PESPayloadRange
	ReadESData(esOffset int64, size int, isVideo bool) ([]byte, error)
	ReadAudioSubStreamData(subStreamID byte, esOffset int64, size int) ([]byte, error)
}

// indexESData indexes the elementary stream data from an ES-aware parser.
// Uses zero-copy iteration through PES payload ranges.
func (idx *Indexer) indexESData(fileIndex uint16, parser esDataProvider, isVideo bool, esSize int64, progress func(int64)) error {
	ranges := parser.FilteredVideoRanges()
	if len(ranges) == 0 {
		return nil
	}

	dataSize := parser.DataSize()
	syncPointCount := 0
	var indexFastPath, indexSlowPath, indexSkipped int

	// Iterate through each PES payload range (zero-copy when within one region)
	for rangeIdx, r := range ranges {
		endOffset := r.FileOffset + int64(r.Size)
		if endOffset > dataSize {
			continue
		}
		rangeData := parser.DataSlice(r.FileOffset, r.Size)

		// Find NAL unit start positions (byte after 00 00 01)
		// Hashing from NAL header enables matching both Annex B and AVCC formats
		syncPoints := FindVideoNALStarts(rangeData)

		// Add each sync point to the index
		for _, offsetInRange := range syncPoints {
			syncESOffset := r.ESOffset + int64(offsetInRange)

			// Ensure we have enough data for the window
			if syncESOffset+int64(idx.windowSize) > esSize {
				continue
			}

			// Check if window fits within this range (zero-copy fast path)
			if offsetInRange+idx.windowSize <= len(rangeData) {
				window := rangeData[offsetInRange : offsetInRange+idx.windowSize]
				hash := xxhash.Sum64(window)

				idx.index.HashToLocations[hash] = append(idx.index.HashToLocations[hash], Location{
					FileIndex: fileIndex,
					Offset:    syncESOffset,
					IsVideo:   isVideo,
				})
				syncPointCount++
				indexFastPath++
			} else {
				// Window spans range boundary - use ReadESData (may copy)
				window, err := parser.ReadESData(syncESOffset, idx.windowSize, isVideo)
				if err != nil || len(window) < idx.windowSize {
					indexSkipped++
					continue
				}
				hash := xxhash.Sum64(window)

				idx.index.HashToLocations[hash] = append(idx.index.HashToLocations[hash], Location{
					FileIndex: fileIndex,
					Offset:    syncESOffset,
					IsVideo:   isVideo,
				})
				syncPointCount++
				indexSlowPath++
			}

		}

		// Report progress periodically
		if rangeIdx%10000 == 0 && progress != nil {
			progress(r.FileOffset)
		}
	}

	if idx.verboseWriter != nil {
		fmt.Fprintf(idx.verboseWriter, "  [indexESData] video=%v: %d NALs indexed (fast=%d, slow/cross-range=%d, skipped=%d)\n",
			isVideo, syncPointCount, indexFastPath, indexSlowPath, indexSkipped)
	}

	return nil
}

// syncPointFinder is a function that returns sync point offsets within data.
type syncPointFinder func(data []byte) []int

// indexAudioSubStream indexes a specific audio sub-stream.
func (idx *Indexer) indexAudioSubStream(fileIndex uint16, parser esDataProvider, subStreamID byte, esSize int64) error {
	return idx.indexSubStream(fileIndex, parser, subStreamID, esSize, FindAudioSyncPoints)
}

// indexSubStream indexes a specific sub-stream using the provided sync point finder.
// Uses zero-copy iteration through PES payload ranges.
func (idx *Indexer) indexSubStream(fileIndex uint16, parser esDataProvider, subStreamID byte, esSize int64, findSyncPoints syncPointFinder) error {
	ranges := parser.FilteredAudioRanges(subStreamID)
	if len(ranges) == 0 {
		return nil
	}

	dataSize := parser.DataSize()

	// Iterate through each PES payload range (zero-copy when within one region)
	for _, r := range ranges {
		endOffset := r.FileOffset + int64(r.Size)
		if endOffset > dataSize {
			continue
		}
		rangeData := parser.DataSlice(r.FileOffset, r.Size)

		// Find sync points in this range
		syncPoints := findSyncPoints(rangeData)

		// Add each sync point to the index
		for _, offsetInRange := range syncPoints {
			syncESOffset := r.ESOffset + int64(offsetInRange)

			// Ensure we have enough data for the window
			if syncESOffset+int64(idx.windowSize) > esSize {
				continue
			}

			// Check if window fits within this range (zero-copy fast path)
			if offsetInRange+idx.windowSize <= len(rangeData) {
				window := rangeData[offsetInRange : offsetInRange+idx.windowSize]
				hash := xxhash.Sum64(window)

				idx.index.HashToLocations[hash] = append(idx.index.HashToLocations[hash], Location{
					FileIndex:        fileIndex,
					Offset:           syncESOffset,
					IsVideo:          false,
					AudioSubStreamID: subStreamID,
				})
			} else {
				// Window spans range boundary - use ReadAudioSubStreamData (may copy)
				window, err := parser.ReadAudioSubStreamData(subStreamID, syncESOffset, idx.windowSize)
				if err != nil || len(window) < idx.windowSize {
					continue
				}
				hash := xxhash.Sum64(window)

				idx.index.HashToLocations[hash] = append(idx.index.HashToLocations[hash], Location{
					FileIndex:        fileIndex,
					Offset:           syncESOffset,
					IsVideo:          false,
					AudioSubStreamID: subStreamID,
				})
			}
		}
	}

	return nil
}
