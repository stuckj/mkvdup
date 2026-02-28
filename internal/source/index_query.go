package source

import (
	"fmt"

	"github.com/cespare/xxhash/v2"
	"golang.org/x/sys/unix"
)

// Lookup finds locations in the source that match the given hash.
func (idx *Index) Lookup(hash uint64) []Location {
	return idx.HashToLocations[hash]
}

// ReadESDataAt reads ES data at the given location.
// For sources that use ES offsets, this handles the translation.
// For audio locations, uses the sub-stream ID from the location.
func (idx *Index) ReadESDataAt(loc Location, size int) ([]byte, error) {
	if int(loc.FileIndex) >= len(idx.ESReaders) || idx.ESReaders[loc.FileIndex] == nil {
		// No ES reader - this shouldn't happen for ES-based indexes
		return nil, fmt.Errorf("no ES reader for file %d", loc.FileIndex)
	}
	if loc.IsVideo {
		return idx.ESReaders[loc.FileIndex].ReadESData(loc.Offset, size, true)
	}
	// For audio, use the sub-stream specific reader
	return idx.ESReaders[loc.FileIndex].ReadAudioSubStreamData(loc.AudioSubStreamID, loc.Offset, size)
}

// hintedESReader is the interface for hint-based byte reading.
// Both MPEGPSParser and MPEGTSParser implement this.
type hintedESReader interface {
	ReadESByteWithHint(esOffset int64, isVideo bool, rangeHint int) (byte, int, bool)
	ReadAudioByteWithHint(subStreamID byte, esOffset int64, rangeHint int) (byte, int, bool)
}

// ReadESByteWithHint reads a single byte from the ES stream, using a range hint
// to avoid binary search when reading sequentially. Returns the byte, the new range
// hint for the next call, and success status. Pass rangeHint=-1 to force binary search.
// This is optimized for the expandMatch hot path where we read bytes sequentially.
func (idx *Index) ReadESByteWithHint(loc Location, rangeHint int) (byte, int, bool) {
	if int(loc.FileIndex) >= len(idx.ESReaders) || idx.ESReaders[loc.FileIndex] == nil {
		return 0, -1, false
	}

	// Try hint-based reading (fast path for MPEGPSParser and MPEGTSParser)
	if hinted, ok := idx.ESReaders[loc.FileIndex].(hintedESReader); ok {
		if loc.IsVideo {
			return hinted.ReadESByteWithHint(loc.Offset, true, rangeHint)
		}
		return hinted.ReadAudioByteWithHint(loc.AudioSubStreamID, loc.Offset, rangeHint)
	}

	// Fallback: use ReadESData (allocates, but works for any ESReader)
	var data []byte
	var err error
	if loc.IsVideo {
		data, err = idx.ESReaders[loc.FileIndex].ReadESData(loc.Offset, 1, true)
	} else {
		data, err = idx.ESReaders[loc.FileIndex].ReadAudioSubStreamData(loc.AudioSubStreamID, loc.Offset, 1)
	}
	if err != nil || len(data) == 0 {
		return 0, -1, false
	}
	return data[0], -1, true
}

// ComputeHash calculates the xxhash of the given data.
func ComputeHash(data []byte) uint64 {
	return xxhash.Sum64(data)
}

// AdviseForMatching sets madvise hints on source mmap'd files before matching.
// For raw-indexed sources (Blu-ray with raw offsets), sets MADV_SEQUENTIAL since
// locality-aware matching produces largely sequential access.
// For ES-indexed sources (DVD MPEG-PS, Blu-ray M2TS with ES offsets), the ES reader
// translates ES offsets to scattered positions in the container file, so MADV_SEQUENTIAL
// would hurt. Uses MADV_NORMAL (default adaptive readahead) instead.
func (idx *Index) AdviseForMatching() {
	if idx.UsesESOffsets {
		// ES-based: access pattern in the raw file is not sequential
		// (ES offsets map to scattered PES packets). Use normal adaptive readahead.
		for _, mmapFile := range idx.MmapFiles {
			if mmapFile != nil {
				mmapFile.Advise(unix.MADV_NORMAL)
			}
		}
	} else {
		// Raw-indexed: locality-aware matching produces sequential access
		for _, reader := range idx.RawReaders {
			if rr, ok := reader.(*mmapRawReader); ok {
				rr.mmapFile.Advise(unix.MADV_SEQUENTIAL)
			}
		}
	}
}

// Close releases resources held by the index.
func (idx *Index) Close() error {
	// Close all mmap files (these back the ESReaders and RawReaders)
	for _, mmapFile := range idx.MmapFiles {
		if mmapFile != nil {
			mmapFile.Close()
		}
	}
	// Close all raw readers (which also close their mmap files)
	for _, reader := range idx.RawReaders {
		if reader != nil {
			reader.Close()
		}
	}
	return nil
}
