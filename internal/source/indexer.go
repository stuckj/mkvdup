package source

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

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
	useRawIndexing bool      // Force raw file indexing even for DVDs
	verboseWriter  io.Writer // Destination for diagnostic output (nil = disabled)
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

// SetVerboseWriter sets the destination for diagnostic output during indexing.
// Pass nil to disable verbose output.
func (idx *Indexer) SetVerboseWriter(w io.Writer) {
	idx.verboseWriter = w
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

	// For DVDs (MPEG-PS) and Blu-rays (MPEG-TS), use ES-based indexing
	// so the matcher works with continuous ES data.
	// Raw indexing is available as fallback for DVDs.
	if idx.sourceType == TypeDVD && !idx.useRawIndexing {
		idx.index.UsesESOffsets = true
	} else if idx.sourceType == TypeBluray {
		idx.index.UsesESOffsets = true
	}

	var processedSize int64

	// Process each file
	// fileIndex tracks the next available index for source file entries.
	// Most files produce one entry, but Blu-ray ISOs produce one per M2TS region.
	fileIndex := 0
	for _, relPath := range files {
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
		} else if idx.sourceType == TypeBluray && isISOFile(relPath) {
			// Blu-ray ISO: one ISO may contain multiple M2TS regions,
			// each producing a separate source file entry.
			var n int
			n, _, err = idx.indexBlurayISOFile(uint16(fileIndex), fullPath, relPath, size, func(fileProcessed int64) {
				if progress != nil {
					progress(processedSize+fileProcessed, totalSize)
				}
			})
			if err != nil {
				return fmt.Errorf("index file %s: %w", relPath, err)
			}
			// indexBlurayISOFile already added source file entries
			fileIndex += n
			processedSize += size
			continue
		} else if idx.sourceType == TypeBluray {
			checksum, err = idx.indexM2TSFile(uint16(fileIndex), fullPath, size, func(fileProcessed int64) {
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

		fileIndex++
		processedSize += size
	}

	return nil
}

// isISOFile returns true if the path has an .iso extension.
func isISOFile(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".iso")
}

// checksumWithProgress computes xxhash checksum of data in chunks, calling
// progress with the number of bytes processed so far after each chunk.
func checksumWithProgress(data []byte, progress func(int64)) uint64 {
	hasher := xxhash.New()
	const chunkSize = 16 * 1024 * 1024 // 16MB chunks
	for offset := 0; offset < len(data); offset += chunkSize {
		end := offset + chunkSize
		if end > len(data) {
			end = len(data)
		}
		hasher.Write(data[offset:end])
		if progress != nil {
			progress(int64(end))
		}
	}
	return hasher.Sum64()
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

	// Phase 1: Parse MPEG-PS structure (0% → 33%)
	if err := parser.ParseWithProgress(func(processed, total int64) {
		if progress != nil {
			progress(processed / 3)
		}
	}); err != nil {
		return 0, fmt.Errorf("parse MPEG-PS: %w", err)
	}

	// Store parser for later use by matcher
	idx.index.ESReaders = append(idx.index.ESReaders, parser)

	// Phase 2: Checksum (33% → 66%)
	checksum := checksumWithProgress(mmapFile.Data(), func(processed int64) {
		if progress != nil {
			progress(size/3 + processed/3)
		}
	})

	// Phase 3: Index ES data (66% → 100%)
	videoESSize := parser.TotalESSize(true)
	if videoESSize > 0 {
		indexProgress := func(fileOffset int64) {
			if progress != nil {
				progress(2*size/3 + fileOffset/3)
			}
		}
		if err := idx.indexESData(fileIndex, parser, true, videoESSize, indexProgress); err != nil {
			return 0, fmt.Errorf("index video ES: %w", err)
		}
	}

	// Index each audio sub-stream separately
	audioSubStreams := parser.AudioSubStreams()
	for _, subStreamID := range audioSubStreams {
		subStreamSize := parser.AudioSubStreamESSize(subStreamID)
		if subStreamSize > 0 {
			if parser.IsLPCMSubStream(subStreamID) {
				// LPCM has no natural sync patterns; use fixed-interval sync points.
				// The indexer forces the slow path (ReadAudioSubStreamData) for LPCM
				// so the data goes through the byte-swap transform.
				if err := idx.indexSubStream(fileIndex, parser, subStreamID, subStreamSize, FindLPCMSyncPoints); err != nil {
					return 0, fmt.Errorf("index LPCM sub-stream 0x%02X: %w", subStreamID, err)
				}
			} else {
				if err := idx.indexAudioSubStream(fileIndex, parser, subStreamID, subStreamSize); err != nil {
					return 0, fmt.Errorf("index audio sub-stream 0x%02X: %w", subStreamID, err)
				}
			}
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
