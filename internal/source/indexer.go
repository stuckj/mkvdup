package source

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cespare/xxhash/v2"
	"github.com/stuckj/mkvdup/internal/mmap"
	"golang.org/x/sys/unix"
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
	verbose        bool // Enable diagnostic output
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

// SetVerbose enables or disables diagnostic output during indexing.
func (idx *Indexer) SetVerbose(v bool) {
	idx.verbose = v
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

// esDataProvider is the interface needed by indexESData and indexAudioSubStream.
// Both MPEGPSParser and MPEGTSParser implement this.
type esDataProvider interface {
	Data() []byte
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

	data := parser.Data() // Get mmap'd data for direct access
	syncPointCount := 0
	var indexFastPath, indexSlowPath, indexSkipped int

	// Iterate through each PES payload range (zero-copy)
	for rangeIdx, r := range ranges {
		// Direct slice access into mmap'd data - no copy!
		endOffset := r.FileOffset + int64(r.Size)
		if endOffset > int64(len(data)) {
			continue
		}
		rangeData := data[r.FileOffset:endOffset]

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

	if idx.verbose {
		fmt.Fprintf(os.Stderr, "  [indexESData] video=%v: %d NALs indexed (fast=%d, slow/cross-range=%d, skipped=%d)\n",
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

	data := parser.Data() // Get mmap'd data for direct access

	// Iterate through each PES payload range (zero-copy)
	for _, r := range ranges {
		// Direct slice access into mmap'd data - no copy!
		endOffset := r.FileOffset + int64(r.Size)
		if endOffset > int64(len(data)) {
			continue
		}
		rangeData := data[r.FileOffset:endOffset]

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

// indexRawFile processes a raw file (for non-DVD, non-Blu-ray formats).
// Processes the file in a single pass: computes checksum and indexes sync points
// together in chunks, releasing mmap pages as they're processed.
func (idx *Indexer) indexRawFile(fileIndex uint16, path string, size int64, progress func(int64)) (uint64, error) {
	mmapFile, err := mmap.Open(path)
	if err != nil {
		return 0, fmt.Errorf("mmap open: %w", err)
	}
	idx.index.RawReaders = append(idx.index.RawReaders, &mmapRawReader{mmapFile: mmapFile})

	mmapFile.Advise(unix.MADV_SEQUENTIAL)
	data := mmapFile.Data()

	return idx.indexRawFileData(fileIndex, mmapFile, data, size, progress)
}

// indexM2TSFile processes a Blu-ray M2TS file using ES-aware indexing.
// It parses the MPEG-TS structure to extract elementary stream data and
// indexes sync points within the continuous ES, matching what MKV files contain.
func (idx *Indexer) indexM2TSFile(fileIndex uint16, path string, size int64, progress func(int64)) (uint64, error) {
	mmapFile, err := mmap.Open(path)
	if err != nil {
		return 0, fmt.Errorf("mmap open: %w", err)
	}
	// Note: Don't close mmapFile - it's stored in MmapFiles for later use
	idx.index.MmapFiles = append(idx.index.MmapFiles, mmapFile)

	mmapFile.Advise(unix.MADV_SEQUENTIAL)

	// Phase 1: Parse MPEG-TS structure (0% → 33%)
	parser := NewMPEGTSParser(mmapFile.Data())

	if err := parser.ParseWithProgress(func(processed, total int64) {
		if progress != nil {
			progress(processed / 3)
		}
	}); err != nil {
		return 0, fmt.Errorf("parse MPEG-TS: %w", err)
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
	subtitleIDs := parser.SubtitleSubStreams()
	subtitleSet := make(map[byte]bool, len(subtitleIDs))
	for _, id := range subtitleIDs {
		subtitleSet[id] = true
	}
	for _, subStreamID := range parser.AudioSubStreams() {
		if subtitleSet[subStreamID] {
			continue // indexed below with subtitle-specific sync points
		}
		subStreamSize := parser.AudioSubStreamESSize(subStreamID)
		if subStreamSize > 0 {
			if err := idx.indexAudioSubStream(fileIndex, parser, subStreamID, subStreamSize); err != nil {
				return 0, fmt.Errorf("index audio sub-stream %d: %w", subStreamID, err)
			}
		}
	}

	// Index subtitle sub-streams with PGS sync point detection
	for _, subStreamID := range subtitleIDs {
		subStreamSize := parser.AudioSubStreamESSize(subStreamID)
		if subStreamSize > 0 {
			if err := idx.indexSubStream(fileIndex, parser, subStreamID, subStreamSize, FindPGSSyncPoints); err != nil {
				return 0, fmt.Errorf("index subtitle sub-stream %d: %w", subStreamID, err)
			}
		}
	}

	if progress != nil {
		progress(size)
	}

	return checksum, nil
}

// indexBlurayISOFile processes a Blu-ray ISO file by finding M2TS regions
// within the ISO9660 filesystem and indexing each as a separate source file entry.
// Returns the number of source file entries created and the ISO checksum.
func (idx *Indexer) indexBlurayISOFile(startFileIndex uint16, path, relPath string, size int64, progress func(int64)) (int, uint64, error) {
	// Find M2TS file extents within the ISO
	m2tsFiles, err := findBlurayM2TSInISO(path)
	if err != nil {
		return 0, 0, fmt.Errorf("find M2TS in ISO: %w", err)
	}
	if len(m2tsFiles) == 0 {
		return 0, 0, fmt.Errorf("no M2TS files found in Blu-ray ISO")
	}

	// Memory-map the entire ISO
	mmapFile, err := mmap.Open(path)
	if err != nil {
		return 0, 0, fmt.Errorf("mmap open: %w", err)
	}
	// Don't close — stored in MmapFiles for later use
	idx.index.MmapFiles = append(idx.index.MmapFiles, mmapFile)

	mmapFile.Advise(unix.MADV_SEQUENTIAL)
	isoData := mmapFile.Data()

	// Phase 1: Parse all M2TS regions (0% → 33%)
	type parsedM2TS struct {
		adapter *isoM2TSAdapter
		extent  isoFileExtent
	}
	var parsed []parsedM2TS

	for _, m2ts := range m2tsFiles {
		var m2tsSlice []byte
		var adapter *isoM2TSAdapter

		if m2ts.Extents != nil {
			// Multi-extent UDF file: assemble contiguous data from extents
			m2tsSlice = make([]byte, 0, m2ts.Size)
			for _, ext := range m2ts.Extents {
				endOff := ext.ISOOffset + ext.Length
				if endOff > int64(len(isoData)) {
					break
				}
				m2tsSlice = append(m2tsSlice, isoData[ext.ISOOffset:endOff]...)
			}
			if int64(len(m2tsSlice)) != m2ts.Size {
				if idx.verbose {
					fmt.Fprintf(os.Stderr, "  [indexBlurayISO] skipping %s: assembled %d bytes, expected %d\n",
						m2ts.Name, len(m2tsSlice), m2ts.Size)
				}
				continue
			}

			parser := NewMPEGTSParser(m2tsSlice)
			if err := parser.ParseWithProgress(nil); err != nil {
				if idx.verbose {
					fmt.Fprintf(os.Stderr, "  [indexBlurayISO] skipping %s: %v\n", m2ts.Name, err)
				}
				continue
			}
			adapter = newISOAdapterMultiExtent(parser, isoData, m2tsSlice, m2ts.Extents)
		} else {
			// Contiguous file: use sub-slice of mmap'd ISO
			endOffset := m2ts.Offset + m2ts.Size
			if endOffset > int64(len(isoData)) {
				if idx.verbose {
					fmt.Fprintf(os.Stderr, "  [indexBlurayISO] skipping %s: extent beyond ISO bounds (%d + %d > %d)\n",
						m2ts.Name, m2ts.Offset, m2ts.Size, len(isoData))
				}
				continue
			}

			m2tsSlice = isoData[m2ts.Offset:endOffset]
			parser := NewMPEGTSParser(m2tsSlice)
			if err := parser.ParseWithProgress(nil); err != nil {
				if idx.verbose {
					fmt.Fprintf(os.Stderr, "  [indexBlurayISO] skipping %s: %v\n", m2ts.Name, err)
				}
				continue
			}
			adapter = newISOAdapter(parser, isoData, m2ts.Offset)
		}

		parsed = append(parsed, parsedM2TS{adapter: adapter, extent: m2ts})
	}

	if len(parsed) == 0 {
		return 0, 0, fmt.Errorf("no valid M2TS streams found in Blu-ray ISO")
	}

	if progress != nil {
		progress(size / 3)
	}

	// Phase 2: Checksum the full ISO (33% → 66%)
	checksum := checksumWithProgress(isoData, func(processed int64) {
		if progress != nil {
			progress(size/3 + processed/3)
		}
	})

	// Phase 3: Index ES data from all M2TS regions (66% → 100%)
	entriesCreated := 0
	for _, p := range parsed {
		fileIndex := startFileIndex + uint16(entriesCreated)
		adapter := p.adapter

		// Store adapter as ESReader for this source file entry
		idx.index.ESReaders = append(idx.index.ESReaders, adapter)

		// Index video ES
		videoESSize := adapter.TotalESSize(true)
		if videoESSize > 0 {
			if err := idx.indexESData(fileIndex, adapter, true, videoESSize, nil); err != nil {
				return 0, 0, fmt.Errorf("index video ES for %s: %w", p.extent.Name, err)
			}
		}

		// Index audio sub-streams
		subtitleIDs := adapter.parser.SubtitleSubStreams()
		subtitleSet := make(map[byte]bool, len(subtitleIDs))
		for _, id := range subtitleIDs {
			subtitleSet[id] = true
		}
		for _, subStreamID := range adapter.AudioSubStreams() {
			if subtitleSet[subStreamID] {
				continue
			}
			subStreamSize := adapter.AudioSubStreamESSize(subStreamID)
			if subStreamSize > 0 {
				if err := idx.indexAudioSubStream(fileIndex, adapter, subStreamID, subStreamSize); err != nil {
					return 0, 0, fmt.Errorf("index audio sub-stream %d for %s: %w", subStreamID, p.extent.Name, err)
				}
			}
		}

		// Index subtitle sub-streams
		for _, subStreamID := range subtitleIDs {
			subStreamSize := adapter.AudioSubStreamESSize(subStreamID)
			if subStreamSize > 0 {
				if err := idx.indexSubStream(fileIndex, adapter, subStreamID, subStreamSize, FindPGSSyncPoints); err != nil {
					return 0, 0, fmt.Errorf("index subtitle sub-stream %d for %s: %w", subStreamID, p.extent.Name, err)
				}
			}
		}

		// Add source file entry — all entries share the same ISO path, size, checksum
		idx.index.Files = append(idx.index.Files, File{
			RelativePath: relPath,
			Size:         size,
			Checksum:     checksum,
		})

		entriesCreated++
	}

	if progress != nil {
		progress(size)
	}

	return entriesCreated, checksum, nil
}

// indexRawFileData is the core of indexRawFile operating on already-opened mmap data.
// Used as a fallback when M2TS packet structure cannot be detected.
func (idx *Indexer) indexRawFileData(fileIndex uint16, mmapFile *mmap.File, data []byte, size int64, progress func(int64)) (uint64, error) {
	hasher := xxhash.New()
	const chunkSize = 64 * 1024 * 1024
	const overlap = 3
	pageSize := unix.Getpagesize()
	checksumPos := 0

	for chunkStart := 0; chunkStart < len(data); {
		chunkEnd := chunkStart + chunkSize
		if chunkEnd > len(data) {
			chunkEnd = len(data)
		}

		chunk := data[chunkStart:chunkEnd]

		if chunkEnd > checksumPos {
			hasher.Write(data[checksumPos:chunkEnd])
			checksumPos = chunkEnd
		}

		videoOffsets := FindVideoNALStartsInRange(chunk, chunkStart)
		audioOffsets := FindAudioSyncPointsInRange(chunk, chunkStart)

		for _, offset := range videoOffsets {
			if offset+idx.windowSize > len(data) {
				continue
			}
			window := data[offset : offset+idx.windowSize]
			hash := xxhash.Sum64(window)
			idx.index.HashToLocations[hash] = append(idx.index.HashToLocations[hash], Location{
				FileIndex: fileIndex,
				Offset:    int64(offset),
			})
		}

		for _, offset := range audioOffsets {
			if offset+idx.windowSize > len(data) {
				continue
			}
			window := data[offset : offset+idx.windowSize]
			hash := xxhash.Sum64(window)
			idx.index.HashToLocations[hash] = append(idx.index.HashToLocations[hash], Location{
				FileIndex: fileIndex,
				Offset:    int64(offset),
			})
		}

		if progress != nil {
			progress(int64(chunkEnd))
		}

		releaseUpTo := (chunkStart / pageSize) * pageSize
		if releaseUpTo > 0 {
			unix.Madvise(data[:releaseUpTo], unix.MADV_DONTNEED)
		}

		if chunkEnd >= len(data) {
			break
		}
		chunkStart = chunkEnd - overlap
	}

	checksum := hasher.Sum64()
	mmapFile.Advise(unix.MADV_RANDOM)
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
