package source

import (
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/cespare/xxhash/v2"
	"github.com/stuckj/mkvdup/internal/mmap"
)

const (
	// DefaultWindowSize is the default number of bytes to hash at each sync point
	DefaultWindowSize = 64

	// MinWindowSize is the minimum allowed window size
	MinWindowSize = 32

	// MaxWindowSize is the maximum allowed window size
	MaxWindowSize = 4096
)

// Indexer builds a hash index from source media files.
type Indexer struct {
	sourceDir      string
	sourceType     Type
	windowSize     int
	index          *Index
	useRawIndexing bool // Force raw file indexing even for DVDs
}

// NewIndexer creates a new Indexer for the given source directory.
func NewIndexer(sourceDir string, windowSize int) (*Indexer, error) {
	return NewIndexerWithOptions(sourceDir, windowSize, false)
}

// NewIndexerWithOptions creates a new Indexer with additional options.
// useRawIndexing forces raw file indexing even for DVDs (useful for finding
// content from any title/stream in the ISO).
func NewIndexerWithOptions(sourceDir string, windowSize int, useRawIndexing bool) (*Indexer, error) {
	sourceType, err := DetectType(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("detect source type: %w", err)
	}

	if windowSize < MinWindowSize {
		windowSize = MinWindowSize
	}
	if windowSize > MaxWindowSize {
		windowSize = MaxWindowSize
	}

	return &Indexer{
		sourceDir:      sourceDir,
		sourceType:     sourceType,
		windowSize:     windowSize,
		index:          NewIndex(sourceDir, sourceType, windowSize),
		useRawIndexing: useRawIndexing,
	}, nil
}

// SourceType returns the detected source type.
func (idx *Indexer) SourceType() Type {
	return idx.sourceType
}

// SourceDir returns the source directory path.
func (idx *Indexer) SourceDir() string {
	return idx.sourceDir
}

// ProgressFunc is called during indexing to report progress.
// processed is the number of bytes processed so far, total is the total bytes to process.
type ProgressFunc func(processed, total int64)

// Build scans all media files and builds the hash index.
// If progress is non-nil, it will be called periodically to report progress.
func (idx *Indexer) Build(progress ProgressFunc) error {
	files, err := EnumerateMediaFiles(idx.sourceDir, idx.sourceType)
	if err != nil {
		return fmt.Errorf("enumerate media files: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no media files found in %s", idx.sourceDir)
	}

	// Calculate total size for progress reporting
	var totalSize int64
	for _, relPath := range files {
		fullPath := filepath.Join(idx.sourceDir, relPath)
		size, err := GetFileInfo(fullPath)
		if err != nil {
			return fmt.Errorf("get file info for %s: %w", relPath, err)
		}
		totalSize += size
	}

	// Pre-allocate hash map to reduce reallocation
	// Estimate: ~1 sync point per 2KB of data on average
	estimatedSyncPoints := int(totalSize / 2048)
	if estimatedSyncPoints < 10000 {
		estimatedSyncPoints = 10000
	}
	idx.index.HashToLocations = make(map[uint64][]Location, estimatedSyncPoints)

	// For DVDs (MPEG-PS), we can use ES-based indexing or raw indexing
	// Raw indexing is more reliable as it finds content from any title/stream
	if idx.sourceType == TypeDVD && !idx.useRawIndexing {
		idx.index.UsesESOffsets = true
	}

	var processedSize int64

	// Process each file
	for fileIndex, relPath := range files {
		fullPath := filepath.Join(idx.sourceDir, relPath)

		size, err := GetFileInfo(fullPath)
		if err != nil {
			return fmt.Errorf("get file info for %s: %w", relPath, err)
		}

		var checksum uint64
		if idx.sourceType == TypeDVD && !idx.useRawIndexing {
			checksum, err = idx.indexMPEGPSFile(uint16(fileIndex), fullPath, size, func(fileProcessed int64) {
				if progress != nil {
					progress(processedSize+fileProcessed, totalSize)
				}
			})
		} else {
			checksum, err = idx.indexRawFile(uint16(fileIndex), fullPath, size, func(fileProcessed int64) {
				if progress != nil {
					progress(processedSize+fileProcessed, totalSize)
				}
			})
		}
		if err != nil {
			return fmt.Errorf("index file %s: %w", relPath, err)
		}

		idx.index.Files = append(idx.index.Files, File{
			RelativePath: relPath,
			Size:         size,
			Checksum:     checksum,
		})

		processedSize += size
	}

	return nil
}

// indexMPEGPSFile processes an MPEG-PS file (DVD ISO) using ES-aware indexing.
// It extracts the elementary stream data and indexes sync points within it.
func (idx *Indexer) indexMPEGPSFile(fileIndex uint16, path string, size int64, progress func(int64)) (uint64, error) {
	// Memory-map the file with zero-copy access
	mmapFile, err := mmap.Open(path)
	if err != nil {
		return 0, fmt.Errorf("mmap open: %w", err)
	}
	// Note: Don't close mmapFile - it's stored in MmapFiles for later use

	// Store the mmap file for cleanup
	idx.index.MmapFiles = append(idx.index.MmapFiles, mmapFile)

	// Parse MPEG-PS structure with progress reporting using zero-copy data
	parser := NewMPEGPSParser(mmapFile.Data())

	if err := parser.ParseWithProgress(func(processed, total int64) {
		if progress != nil {
			// Report parsing progress (first 50% of the indexing)
			progress(processed / 2)
		}
	}); err != nil {
		return 0, fmt.Errorf("parse MPEG-PS: %w", err)
	}

	// Store parser for later use by matcher
	idx.index.ESReaders = append(idx.index.ESReaders, parser)

	// Calculate file checksum using zero-copy data (no allocation needed)
	checksum := xxhash.Sum64(mmapFile.Data())

	// Index video ES
	videoESSize := parser.TotalESSize(true)
	if videoESSize > 0 {
		if err := idx.indexESData(fileIndex, parser, true, videoESSize, progress); err != nil {
			return 0, fmt.Errorf("index video ES: %w", err)
		}
	}

	// Index each audio sub-stream separately
	audioSubStreams := parser.AudioSubStreams()
	for _, subStreamID := range audioSubStreams {
		subStreamSize := parser.AudioSubStreamESSize(subStreamID)
		if subStreamSize > 0 {
			if err := idx.indexAudioSubStream(fileIndex, parser, subStreamID, subStreamSize); err != nil {
				return 0, fmt.Errorf("index audio sub-stream 0x%02X: %w", subStreamID, err)
			}
		}
	}

	if progress != nil {
		progress(size)
	}

	return checksum, nil
}

// indexESData indexes the elementary stream data from an MPEG-PS parser.
// Uses zero-copy iteration through PES payload ranges.
func (idx *Indexer) indexESData(fileIndex uint16, parser *MPEGPSParser, isVideo bool, esSize int64, progress func(int64)) error {
	ranges := parser.FilteredVideoRanges()
	if len(ranges) == 0 {
		return nil
	}

	data := parser.Data() // Get mmap'd data for direct access
	syncPointCount := 0

	// Iterate through each PES payload range (zero-copy)
	for rangeIdx, r := range ranges {
		// Direct slice access into mmap'd data - no copy!
		endOffset := r.FileOffset + int64(r.Size)
		if endOffset > int64(len(data)) {
			continue
		}
		rangeData := data[r.FileOffset:endOffset]

		// Find sync points in this range
		syncPoints := FindVideoStartCodes(rangeData)

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
			} else {
				// Window spans range boundary - use ReadESData (may copy)
				window, err := parser.ReadESData(syncESOffset, idx.windowSize, isVideo)
				if err != nil || len(window) < idx.windowSize {
					continue
				}
				hash := xxhash.Sum64(window)

				idx.index.HashToLocations[hash] = append(idx.index.HashToLocations[hash], Location{
					FileIndex: fileIndex,
					Offset:    syncESOffset,
					IsVideo:   isVideo,
				})
				syncPointCount++
			}
		}

		// Report progress periodically and force GC to prevent memory buildup
		if rangeIdx%1000 == 0 {
			if progress != nil {
				progress(r.FileOffset)
			}
			// Force GC periodically to clean up temporary allocations
			if rangeIdx%10000 == 0 {
				runtime.GC()
			}
		}
	}

	return nil
}

// indexAudioSubStream indexes a specific audio sub-stream.
// Uses zero-copy iteration through PES payload ranges.
func (idx *Indexer) indexAudioSubStream(fileIndex uint16, parser *MPEGPSParser, subStreamID byte, esSize int64) error {
	ranges := parser.FilteredAudioRanges(subStreamID)
	if len(ranges) == 0 {
		return nil
	}

	data := parser.Data() // Get mmap'd data for direct access

	// Iterate through each PES payload range (zero-copy)
	for _, r := range ranges {
		// Direct slice access into mmap'd data - no copy!
		endOffset := r.FileOffset + int64(r.Size)
		if endOffset > int64(len(data)) {
			continue
		}
		rangeData := data[r.FileOffset:endOffset]

		// Find audio sync points in this range
		syncPoints := FindAudioSyncPoints(rangeData)

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

// mmapRawReader wraps mmap.File to implement RawReader interface.
type mmapRawReader struct {
	mmapFile *mmap.File
}

func (r *mmapRawReader) ReadAt(buf []byte, offset int64) (int, error) {
	data := r.mmapFile.Slice(offset, len(buf))
	if data == nil {
		return 0, fmt.Errorf("offset out of range")
	}
	copy(buf, data)
	return len(data), nil
}

// Slice returns a zero-copy slice of the underlying mmap'd data.
func (r *mmapRawReader) Slice(offset int64, size int) []byte {
	return r.mmapFile.Slice(offset, size)
}

func (r *mmapRawReader) Len() int {
	return r.mmapFile.Len()
}

func (r *mmapRawReader) Close() error {
	return r.mmapFile.Close()
}

// indexRawFile processes a raw file (for Blu-ray M2TS or other formats).
// This is the original indexing approach.
func (idx *Indexer) indexRawFile(fileIndex uint16, path string, size int64, progress func(int64)) (uint64, error) {
	// Memory-map the file with zero-copy access
	mmapFile, err := mmap.Open(path)
	if err != nil {
		return 0, fmt.Errorf("mmap open: %w", err)
	}
	// Don't close the mmapFile - keep it for later use by matcher
	idx.index.RawReaders = append(idx.index.RawReaders, &mmapRawReader{mmapFile: mmapFile})

	// Zero-copy access to file data
	data := mmapFile.Data()

	// Calculate file checksum (zero-copy - no allocation)
	checksum := xxhash.Sum64(data)

	// Find all sync points
	syncPoints := FindAllSyncPoints(data)

	// Add each sync point to the index
	for i, offset := range syncPoints {
		// Ensure we have enough data for the window
		if int64(offset)+int64(idx.windowSize) > size {
			continue
		}

		// Hash the window at this offset (zero-copy slice)
		window := data[offset : offset+idx.windowSize]
		hash := xxhash.Sum64(window)

		// Add to index
		idx.index.HashToLocations[hash] = append(idx.index.HashToLocations[hash], Location{
			FileIndex: fileIndex,
			Offset:    int64(offset),
		})

		// Report progress periodically (every 10000 sync points)
		if progress != nil && i%10000 == 0 {
			progress(int64(offset))
		}
	}

	if progress != nil {
		progress(size)
	}

	return checksum, nil
}

// Index returns the built index. Must call Build first.
func (idx *Indexer) Index() *Index {
	return idx.index
}

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

// ReadESByteWithHint reads a single byte from the ES stream, using a range hint
// to avoid binary search when reading sequentially. Returns the byte, the new range
// hint for the next call, and success status. Pass rangeHint=-1 to force binary search.
// This is optimized for the expandMatch hot path where we read bytes sequentially.
func (idx *Index) ReadESByteWithHint(loc Location, rangeHint int) (byte, int, bool) {
	if int(loc.FileIndex) >= len(idx.ESReaders) || idx.ESReaders[loc.FileIndex] == nil {
		return 0, -1, false
	}

	// Type-assert to MPEGPSParser to access hint-based methods
	parser, ok := idx.ESReaders[loc.FileIndex].(*MPEGPSParser)
	if !ok {
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

	// Use hint-based reading for MPEGPSParser
	if loc.IsVideo {
		return parser.ReadESByteWithHint(loc.Offset, true, rangeHint)
	}
	return parser.ReadAudioByteWithHint(loc.AudioSubStreamID, loc.Offset, rangeHint)
}

// ComputeHash calculates the xxhash of the given data.
func ComputeHash(data []byte) uint64 {
	return xxhash.Sum64(data)
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
