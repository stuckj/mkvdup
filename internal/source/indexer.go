package source

import (
	"fmt"
	"path/filepath"

	"github.com/cespare/xxhash/v2"
	"golang.org/x/exp/mmap"
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
	sourceDir  string
	sourceType Type
	windowSize int
	index      *Index
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
	// Memory-map the file
	reader, err := mmap.Open(path)
	if err != nil {
		return 0, fmt.Errorf("mmap open: %w", err)
	}
	// Note: Don't close the reader - it's stored in ESReaders for later use

	// Parse MPEG-PS structure with progress reporting
	parser := NewMPEGPSParser(reader)
	if err := parser.ParseWithProgress(func(processed, total int64) {
		if progress != nil {
			// Report parsing progress (first 50% of the indexing)
			progress(processed / 2)
		}
	}); err != nil {
		reader.Close()
		return 0, fmt.Errorf("parse MPEG-PS: %w", err)
	}

	// Store parser for later use by matcher
	idx.index.ESReaders = append(idx.index.ESReaders, parser)

	// Calculate file checksum (of the raw file for integrity)
	data := make([]byte, size)
	n, err := reader.ReadAt(data, 0)
	if err != nil && int64(n) != size {
		return 0, fmt.Errorf("read file for checksum: %w", err)
	}
	checksum := xxhash.Sum64(data)

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
func (idx *Indexer) indexESData(fileIndex uint16, parser *MPEGPSParser, isVideo bool, esSize int64, progress func(int64)) error {
	// Read ES data in chunks and find sync points
	const chunkSize = 4 * 1024 * 1024 // 4MB chunks

	var esOffset int64
	syncPointCount := 0

	for esOffset < esSize {
		readSize := chunkSize
		if esOffset+int64(readSize) > esSize {
			readSize = int(esSize - esOffset)
		}

		// Read chunk of ES data
		data, err := parser.ReadESData(esOffset, readSize, isVideo)
		if err != nil {
			return fmt.Errorf("read ES data at %d: %w", esOffset, err)
		}

		// Find sync points in this chunk
		var syncPoints []int
		if isVideo {
			syncPoints = FindVideoStartCodes(data)
		} else {
			syncPoints = FindAudioSyncPoints(data)
		}

		// Add each sync point to the index
		for _, offsetInChunk := range syncPoints {
			syncESOffset := esOffset + int64(offsetInChunk)

			// Ensure we have enough data for the window
			if syncESOffset+int64(idx.windowSize) > esSize {
				continue
			}

			// Use data from chunk directly if we have enough bytes available
			var window []byte
			if offsetInChunk+idx.windowSize <= len(data) {
				window = data[offsetInChunk : offsetInChunk+idx.windowSize]
			} else {
				// Window spans chunk boundary, need to read separately
				window, err = parser.ReadESData(syncESOffset, idx.windowSize, isVideo)
				if err != nil || len(window) < idx.windowSize {
					continue
				}
			}

			hash := xxhash.Sum64(window)

			// Add to index (storing ES offset and stream type)
			idx.index.HashToLocations[hash] = append(idx.index.HashToLocations[hash], Location{
				FileIndex: fileIndex,
				Offset:    syncESOffset,
				IsVideo:   isVideo,
			})

			syncPointCount++
		}

		esOffset += int64(readSize)

		// Report progress periodically
		if progress != nil && syncPointCount%10000 == 0 {
			// Convert ES offset back to approximate file position for progress
			fileOffset, _ := parser.ESOffsetToFileOffset(esOffset, isVideo)
			if fileOffset > 0 {
				progress(fileOffset)
			}
		}
	}

	return nil
}

// indexAudioSubStream indexes a specific audio sub-stream.
func (idx *Indexer) indexAudioSubStream(fileIndex uint16, parser *MPEGPSParser, subStreamID byte, esSize int64) error {
	const chunkSize = 4 * 1024 * 1024 // 4MB chunks

	var esOffset int64

	for esOffset < esSize {
		readSize := chunkSize
		if esOffset+int64(readSize) > esSize {
			readSize = int(esSize - esOffset)
		}

		// Read chunk of audio sub-stream data
		data, err := parser.ReadAudioSubStreamData(subStreamID, esOffset, readSize)
		if err != nil {
			return fmt.Errorf("read audio sub-stream data at %d: %w", esOffset, err)
		}

		// Find audio sync points in this chunk
		syncPoints := FindAudioSyncPoints(data)

		// Add each sync point to the index
		for _, offsetInChunk := range syncPoints {
			syncESOffset := esOffset + int64(offsetInChunk)

			// Ensure we have enough data for the window
			if syncESOffset+int64(idx.windowSize) > esSize {
				continue
			}

			// Use data from chunk directly if we have enough bytes available
			var window []byte
			if offsetInChunk+idx.windowSize <= len(data) {
				window = data[offsetInChunk : offsetInChunk+idx.windowSize]
			} else {
				// Window spans chunk boundary, need to read separately
				window, err = parser.ReadAudioSubStreamData(subStreamID, syncESOffset, idx.windowSize)
				if err != nil || len(window) < idx.windowSize {
					continue
				}
			}

			hash := xxhash.Sum64(window)

			// Add to index (storing ES offset, stream type, and sub-stream ID)
			idx.index.HashToLocations[hash] = append(idx.index.HashToLocations[hash], Location{
				FileIndex:        fileIndex,
				Offset:           syncESOffset,
				IsVideo:          false,
				AudioSubStreamID: subStreamID,
			})
		}

		esOffset += int64(readSize)
	}

	return nil
}

// mmapRawReader wraps mmap.ReaderAt to implement RawReader interface.
type mmapRawReader struct {
	reader *mmap.ReaderAt
}

func (r *mmapRawReader) ReadAt(buf []byte, offset int64) (int, error) {
	return r.reader.ReadAt(buf, offset)
}

func (r *mmapRawReader) Len() int {
	return r.reader.Len()
}

func (r *mmapRawReader) Close() error {
	return r.reader.Close()
}

// indexRawFile processes a raw file (for Blu-ray M2TS or other formats).
// This is the original indexing approach.
func (idx *Indexer) indexRawFile(fileIndex uint16, path string, size int64, progress func(int64)) (uint64, error) {
	// Memory-map the file
	reader, err := mmap.Open(path)
	if err != nil {
		return 0, fmt.Errorf("mmap open: %w", err)
	}
	// Don't close the reader - keep it for later use by matcher
	idx.index.RawReaders = append(idx.index.RawReaders, &mmapRawReader{reader: reader})

	// Read entire file for checksum and indexing
	data := make([]byte, size)
	n, err := reader.ReadAt(data, 0)
	if err != nil && int64(n) != size {
		return 0, fmt.Errorf("read file: %w (read %d of %d)", err, n, size)
	}

	// Calculate file checksum
	checksum := xxhash.Sum64(data)

	// Find all sync points
	syncPoints := FindAllSyncPoints(data)

	// Add each sync point to the index
	for i, offset := range syncPoints {
		// Ensure we have enough data for the window
		if int64(offset)+int64(idx.windowSize) > size {
			continue
		}

		// Hash the window at this offset
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

// ComputeHash calculates the xxhash of the given data.
func ComputeHash(data []byte) uint64 {
	return xxhash.Sum64(data)
}

// Close releases resources held by the index.
func (idx *Index) Close() error {
	// Close all memory-mapped files in ES readers
	for _, reader := range idx.ESReaders {
		if parser, ok := reader.(*MPEGPSParser); ok {
			if parser.reader != nil {
				parser.reader.Close()
			}
		}
	}
	// Close all raw readers
	for _, reader := range idx.RawReaders {
		if reader != nil {
			reader.Close()
		}
	}
	return nil
}
