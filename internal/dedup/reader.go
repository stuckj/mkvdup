package dedup

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"

	"github.com/cespare/xxhash/v2"
	"github.com/stuckj/mkvdup/internal/mmap"
	"golang.org/x/sys/unix"
)

// blockSize is the block size for the block index.
// Each block maps an MKV offset range to an entry index for O(1) lookup.
// 64KB balances memory overhead vs scan distance.
const blockSize = 64 * 1024

// Reader reads .mkvdup files and provides data reconstruction.
// Reader is safe for concurrent use from multiple goroutines.
type Reader struct {
	file        *File
	dedupMmap   *mmap.File
	dedupPath   string
	sourceDir   string
	sourceMmaps []*mmap.File
	esReader    ESReader  // For ES-based sources (v1 only, deprecated in v2)
	entriesOnce sync.Once // For lazy entry access initialization
	entriesErr  error     // Error from entry access initialization

	// Direct mmap access to entries (no []Entry allocation)
	indexStart int64 // Byte offset where entries begin in file
	entryCount int   // Number of entries

	// Block index for fast entry lookup on cache miss.
	// Maps block_number (MKV offset / blockSize) → entry index for O(1)
	// narrowing, followed by bounded binary search within the block range.
	// Built once in initEntryAccess; immutable after that (no mutex needed).
	blockIndex []int

	// Last-entry cache for O(1) sequential read lookup
	// Protected by cacheMu for concurrent access safety
	cacheMu        sync.Mutex
	lastEntryIdx   int   // Index of last accessed entry (-1 if none)
	lastEntry      Entry // The cached parsed entry
	lastEntryValid bool  // Whether lastEntry is valid

	// V4 range map data (maps ES offsets to raw file offsets)
	rangeMapsByFile map[int]*SourceRangeMaps // file index -> range maps
}

// ESReader interface for reading ES data from MPEG-PS sources.
type ESReader interface {
	ReadESData(esOffset int64, size int, isVideo bool) ([]byte, error)
	ReadAudioSubStreamData(subStreamID byte, esOffset int64, size int) ([]byte, error)
}

// NewReader opens a dedup file for reading with entry access initialized immediately.
// Use NewReaderLazy for faster initialization when entries can be accessed on first read.
func NewReader(dedupPath, sourceDir string) (*Reader, error) {
	r, err := NewReaderLazy(dedupPath, sourceDir)
	if err != nil {
		return nil, err
	}

	// Force immediate entry access initialization
	if err := r.initEntryAccess(); err != nil {
		r.Close()
		return nil, fmt.Errorf("init entry access: %w", err)
	}

	return r, nil
}

// NewReaderLazy opens a dedup file but only reads the header.
// Entries are loaded lazily on first Read. Use this for fast mount times with many files.
func NewReaderLazy(dedupPath, sourceDir string) (*Reader, error) {
	f, err := os.Open(dedupPath)
	if err != nil {
		return nil, fmt.Errorf("open dedup file: %w", err)
	}
	defer f.Close()

	file, err := parseHeaderOnly(f)
	if err != nil {
		return nil, fmt.Errorf("parse dedup header: %w", err)
	}

	// Memory-map the dedup file
	dedupMmap, err := mmap.Open(dedupPath)
	if err != nil {
		return nil, fmt.Errorf("mmap dedup file: %w", err)
	}

	return &Reader{
		file:         file,
		dedupMmap:    dedupMmap,
		dedupPath:    dedupPath,
		sourceDir:    sourceDir,
		lastEntryIdx: -1, // No entry cached yet
	}, nil
}

// SetESReader sets the ES reader for ES-based sources.
func (r *Reader) SetESReader(esReader ESReader) {
	r.esReader = esReader
}

// LoadSourceFiles memory-maps all source files.
func (r *Reader) LoadSourceFiles() error {
	r.sourceMmaps = make([]*mmap.File, len(r.file.SourceFiles))
	for i, sf := range r.file.SourceFiles {
		path := r.sourceDir + "/" + sf.RelativePath
		m, err := mmap.Open(path)
		if err != nil {
			// Clean up already opened files
			for j := 0; j < i; j++ {
				if r.sourceMmaps[j] != nil {
					r.sourceMmaps[j].Close()
				}
			}
			return fmt.Errorf("mmap source file %s: %w", sf.RelativePath, err)
		}
		// Hint sequential access so the kernel does aggressive readahead
		// instead of handling individual 4KB page faults.
		m.Advise(unix.MADV_SEQUENTIAL)
		r.sourceMmaps[i] = m
	}
	return nil
}

// Close releases all resources.
func (r *Reader) Close() error {
	if r.dedupMmap != nil {
		r.dedupMmap.Close()
	}
	for _, m := range r.sourceMmaps {
		if m != nil {
			m.Close()
		}
	}
	return nil
}

// initEntryAccess initializes direct mmap access to entries (no parsing into []Entry).
// This is called lazily on first entry access.
func (r *Reader) initEntryAccess() error {
	r.entriesOnce.Do(func() {
		// Calculate index start offset
		r.indexStart = r.file.headerSize + r.calculateSourceFilesSize()
		r.entryCount = int(r.file.Header.EntryCount)

		// Validate mmap has enough data for all entries
		requiredSize := r.indexStart + int64(r.entryCount)*EntrySize
		if int64(r.dedupMmap.Size()) < requiredSize {
			r.entriesErr = fmt.Errorf("mmap too small: need %d, have %d",
				requiredSize, r.dedupMmap.Size())
			return
		}

		// Build block index for fast random access lookup
		r.buildBlockIndex()

		// V4/V6: parse range map section
		if r.file.Header.Version == VersionRangeMap || r.file.Header.Version == VersionRangeMapCreator {
			if err := r.initRangeMaps(); err != nil {
				r.entriesErr = fmt.Errorf("init range maps: %w", err)
				return
			}
		}
	})
	return r.entriesErr
}

// initRangeMaps parses the range map section from the mmap'd dedup file.
func (r *Reader) initRangeMaps() error {
	// Range map section is between delta and footer
	rangeMapOffset := r.file.DeltaOffset + r.file.Header.DeltaSize
	fileSize := r.dedupMmap.Size()
	rangeMapSize := int(fileSize) - FooterV4Size - int(rangeMapOffset)

	if rangeMapSize <= 0 {
		return fmt.Errorf("no range map section found (offset %d, file size %d)", rangeMapOffset, fileSize)
	}

	data := r.dedupMmap.Slice(rangeMapOffset, rangeMapSize)
	if data == nil {
		return fmt.Errorf("range map slice out of bounds")
	}

	sources, err := readRangeMapSection(data)
	if err != nil {
		return fmt.Errorf("parse range map section: %w", err)
	}

	r.rangeMapsByFile = make(map[int]*SourceRangeMaps, len(sources))
	for i := range sources {
		r.rangeMapsByFile[int(sources[i].FileIndex)] = &sources[i]
	}

	return nil
}

// HasRangeMaps returns true if this dedup file uses V4/V6 range maps.
// This checks the header version (available immediately after NewReaderLazy)
// rather than the lazily-loaded range map data, so it's safe to call
// before the first ReadAt.
func (r *Reader) HasRangeMaps() bool {
	return r.file.Header.Version == VersionRangeMap || r.file.Header.Version == VersionRangeMapCreator
}

// buildBlockIndex creates a mapping from block numbers to entry indices.
// Each block represents a fixed-size range of MKV offsets. The index maps
// block_number → the entry index whose region covers or precedes that block's
// start offset. This narrows binary search from O(log N) over all entries
// to O(log B) within a single block's entries.
//
// Algorithm: single pass over all entries, filling block slots as we go.
// Time: O(entryCount + blockCount), Space: O(blockCount).
func (r *Reader) buildBlockIndex() {
	originalSize := r.file.Header.OriginalSize
	if originalSize <= 0 || r.entryCount == 0 {
		return
	}

	blockCount := int((originalSize + blockSize - 1) / blockSize)
	r.blockIndex = make([]int, blockCount)

	entryIdx := 0
	for b := range blockCount {
		blockStart := int64(b) * blockSize
		// Advance entryIdx to the last entry whose MkvOffset <= blockStart.
		// For block 0 (blockStart=0), this stays at 0 since no entry precedes it.
		for entryIdx+1 < r.entryCount {
			nextOffset, ok := r.getMkvOffset(entryIdx + 1)
			if !ok || nextOffset > blockStart {
				break
			}
			entryIdx++
		}
		r.blockIndex[b] = entryIdx
	}
}

// getEntry returns the entry at the given index by parsing from mmap.
// Uses cache for O(1) sequential access. Safe for concurrent use.
func (r *Reader) getEntry(idx int) (Entry, bool) {
	if idx < 0 || idx >= r.entryCount {
		return Entry{}, false
	}

	// Check cache first (with lock)
	r.cacheMu.Lock()
	if r.lastEntryValid && r.lastEntryIdx == idx {
		entry := r.lastEntry
		r.cacheMu.Unlock()
		return entry, true
	}
	r.cacheMu.Unlock()

	// Parse entry from mmap using RawEntry (no lock needed - mmap is read-only)
	offset := r.indexStart + int64(idx)*EntrySize
	data := r.dedupMmap.Slice(offset, EntrySize)
	if len(data) < EntrySize {
		return Entry{}, false
	}

	// Parse using RawEntry for portable unaligned access
	// Layout: MkvOffset(8) + Length(8) + Source(2) + SourceOffset(8) + ESFlags(1) + AudioSubStreamID(1) = 28
	var raw RawEntry
	copy(raw.MkvOffset[:], data[0:8])
	copy(raw.Length[:], data[8:16])
	copy(raw.Source[:], data[16:18])
	copy(raw.SourceOffset[:], data[18:26])
	raw.ESFlags = data[26]
	raw.AudioSubStreamID = data[27]

	entry := raw.ToEntry()

	// Update cache (with lock)
	r.cacheMu.Lock()
	r.lastEntryIdx = idx
	r.lastEntry = entry
	r.lastEntryValid = true
	r.cacheMu.Unlock()

	return entry, true
}

// getMkvOffset returns just the MkvOffset for entry at idx (for binary search).
// This avoids full entry parsing when only the offset is needed.
func (r *Reader) getMkvOffset(idx int) (int64, bool) {
	if idx < 0 || idx >= r.entryCount {
		return 0, false
	}

	offset := r.indexStart + int64(idx)*EntrySize
	data := r.dedupMmap.Slice(offset, 8) // Only read MkvOffset field (first 8 bytes)
	if len(data) < 8 {
		return 0, false
	}

	return int64(binary.LittleEndian.Uint64(data)), true
}

// getEntryLength returns just the Length for entry at idx (for binary search).
// This avoids full entry parsing when only offset and length are needed.
func (r *Reader) getEntryLength(idx int) (int64, bool) {
	if idx < 0 || idx >= r.entryCount {
		return 0, false
	}

	// Length is at offset 8 within each entry (after MkvOffset)
	offset := r.indexStart + int64(idx)*EntrySize + 8
	data := r.dedupMmap.Slice(offset, 8)
	if len(data) < 8 {
		return 0, false
	}

	return int64(binary.LittleEndian.Uint64(data)), true
}

// OriginalSize returns the size of the original MKV file.
func (r *Reader) OriginalSize() int64 {
	return r.file.Header.OriginalSize
}

// OriginalChecksum returns the checksum of the original MKV file.
func (r *Reader) OriginalChecksum() uint64 {
	return r.file.Header.OriginalChecksum
}

// SourceFiles returns the list of source files.
func (r *Reader) SourceFiles() []SourceFile {
	return r.file.SourceFiles
}

// EntryCount returns the number of index entries.
// Returns 0 if entry access initialization failed. Use InitEntryAccess() to check for errors.
func (r *Reader) EntryCount() int {
	r.initEntryAccess() // Ensure entryCount is initialized
	return r.entryCount
}

// GetEntry returns the entry at the given index.
// Returns false if the index is out of range or if entry access initialization failed.
func (r *Reader) GetEntry(idx int) (Entry, bool) {
	if err := r.initEntryAccess(); err != nil {
		return Entry{}, false
	}
	return r.getEntry(idx)
}

// InitEntryAccess explicitly initializes entry access and returns any error.
// This is useful when you need to check for initialization errors before calling
// methods like EntryCount() or Info() that silently return zero/empty on error.
func (r *Reader) InitEntryAccess() error {
	return r.initEntryAccess()
}

// UsesESOffsets returns true if this dedup file uses ES offsets.
func (r *Reader) UsesESOffsets() bool {
	return r.file.UsesESOffsets
}

// ReadAt reads reconstructed MKV data at the given offset.
func (r *Reader) ReadAt(buf []byte, offset int64) (int, error) {
	// Initialize entry access on first read (lazy initialization)
	if err := r.initEntryAccess(); err != nil {
		return 0, fmt.Errorf("init entry access: %w", err)
	}

	if offset >= r.file.Header.OriginalSize {
		return 0, io.EOF
	}

	totalRead := 0
	remaining := len(buf)
	originalOffset := offset // Preserve original offset for buffer position calculation

	// Limit read to file size
	if offset+int64(remaining) > r.file.Header.OriginalSize {
		remaining = int(r.file.Header.OriginalSize - offset)
	}

	endOffset := offset + int64(remaining)

	// Find starting entry index (zero-allocation inline lookup)
	startIdx := r.findStartEntry(offset)

	// Iterate entries directly — no []Entry allocation
	for i := startIdx; i < r.entryCount && remaining > 0; i++ {
		entry, ok := r.getEntry(i)
		if !ok || entry.MkvOffset >= endOffset {
			break
		}

		// Calculate overlap
		entryEnd := entry.MkvOffset + entry.Length
		readStart := offset
		if readStart < entry.MkvOffset {
			readStart = entry.MkvOffset
		}
		readEnd := offset + int64(remaining)
		if readEnd > entryEnd {
			readEnd = entryEnd
		}

		readLen := int(readEnd - readStart)
		if readLen <= 0 {
			continue
		}

		// Calculate offset within entry
		offsetInEntry := readStart - entry.MkvOffset
		sourceOffset := entry.SourceOffset + offsetInEntry

		// Calculate buffer position
		bufOffset := int(readStart - originalOffset)

		// Read data from appropriate source
		if entry.Source == 0 {
			// Read from delta section (zero-copy mmap slice)
			data, err := r.readDelta(sourceOffset, readLen)
			if err != nil {
				return totalRead, fmt.Errorf("read at offset %d: %w", readStart, err)
			}
			copy(buf[bufOffset:], data)
		} else if r.rangeMapsByFile != nil {
			// V4: Read via range map directly into output buffer (no allocation)
			fileIndex := int(entry.Source - 1)
			if err := r.readViaRangeMapInto(fileIndex, entry, sourceOffset, buf[bufOffset:bufOffset+readLen]); err != nil {
				return totalRead, fmt.Errorf("read at offset %d: %w", readStart, err)
			}
		} else if r.file.UsesESOffsets && r.esReader != nil {
			// V1: Read from ES via external reader
			var data []byte
			var err error
			if entry.IsVideo {
				data, err = r.esReader.ReadESData(sourceOffset, readLen, true)
			} else {
				data, err = r.esReader.ReadAudioSubStreamData(entry.AudioSubStreamID, sourceOffset, readLen)
			}
			if err != nil {
				return totalRead, fmt.Errorf("read at offset %d: %w", readStart, err)
			}
			copy(buf[bufOffset:], data)
		} else {
			// V3: Read from raw source file (zero-copy mmap slice)
			fileIndex := int(entry.Source - 1)
			data, err := r.readSource(fileIndex, sourceOffset, readLen)
			if err != nil {
				return totalRead, fmt.Errorf("read at offset %d: %w", readStart, err)
			}
			copy(buf[bufOffset:], data)
		}

		totalRead += readLen
		remaining -= readLen
		offset = readEnd
	}

	if totalRead == 0 && len(buf) > 0 {
		return 0, io.EOF
	}

	return totalRead, nil
}

// findStartEntry returns the index of the first entry whose range covers offset.
// Uses the entry cache for O(1) sequential access, block index for O(1) narrowing,
// then bounded binary search. Zero allocations.
func (r *Reader) findStartEntry(offset int64) int {
	// Fast path: check if offset is within cached entry
	r.cacheMu.Lock()
	if r.lastEntryValid && r.lastEntryIdx >= 0 && r.lastEntryIdx < r.entryCount {
		if offset >= r.lastEntry.MkvOffset && offset < r.lastEntry.MkvOffset+r.lastEntry.Length {
			idx := r.lastEntryIdx
			r.cacheMu.Unlock()
			return idx
		}
	}
	r.cacheMu.Unlock()

	// Use block index to narrow binary search range
	var lo, hi int
	if r.blockIndex != nil {
		blockNum := int(offset / blockSize)
		if blockNum >= len(r.blockIndex) {
			blockNum = len(r.blockIndex) - 1
		}
		lo = r.blockIndex[blockNum]

		if blockNum+1 < len(r.blockIndex) {
			hi = r.blockIndex[blockNum+1] + 1
			if hi > r.entryCount {
				hi = r.entryCount
			}
		} else {
			hi = r.entryCount
		}
	} else {
		lo = 0
		hi = r.entryCount
	}

	// Binary search within [lo, hi) for first entry whose range covers offset
	return lo + sort.Search(hi-lo, func(i int) bool {
		mkvOffset, ok := r.getMkvOffset(lo + i)
		if !ok {
			return true
		}
		entryLen, ok := r.getEntryLength(lo + i)
		if !ok {
			return true
		}
		return mkvOffset+entryLen > offset
	})
}

func (r *Reader) findEntriesForRange(offset, length int64) []Entry {
	if r.entryCount == 0 {
		return nil
	}

	endOffset := offset + length

	// Fast path: check if offset is within cached entry (with lock)
	r.cacheMu.Lock()
	if r.lastEntryValid && r.lastEntryIdx >= 0 && r.lastEntryIdx < r.entryCount {
		if offset >= r.lastEntry.MkvOffset && offset < r.lastEntry.MkvOffset+r.lastEntry.Length {
			// Cache hit - start from cached entry
			startIdx := r.lastEntryIdx
			r.cacheMu.Unlock()

			var result []Entry
			for i := startIdx; i < r.entryCount; i++ {
				entry, ok := r.getEntry(i)
				if !ok || entry.MkvOffset >= endOffset {
					break
				}
				result = append(result, entry)
			}
			return result
		}
	}
	r.cacheMu.Unlock()

	// Cache miss - use block index to narrow binary search range
	var lo, hi int
	if r.blockIndex != nil {
		blockNum := int(offset / blockSize)
		if blockNum >= len(r.blockIndex) {
			blockNum = len(r.blockIndex) - 1
		}
		lo = r.blockIndex[blockNum]

		// Upper bound: start of next block's entries (or entryCount)
		if blockNum+1 < len(r.blockIndex) {
			// Search up to 1 past the next block's start entry to handle
			// entries that span block boundaries
			hi = r.blockIndex[blockNum+1] + 1
			if hi > r.entryCount {
				hi = r.entryCount
			}
		} else {
			hi = r.entryCount
		}
	} else {
		lo = 0
		hi = r.entryCount
	}

	// Binary search within [lo, hi) for first entry whose range covers offset
	idx := lo + sort.Search(hi-lo, func(i int) bool {
		mkvOffset, ok := r.getMkvOffset(lo + i)
		if !ok {
			return true
		}
		entryLen, ok := r.getEntryLength(lo + i)
		if !ok {
			return true
		}
		return mkvOffset+entryLen > offset
	})

	var result []Entry
	for i := idx; i < r.entryCount; i++ {
		entry, ok := r.getEntry(i)
		if !ok || entry.MkvOffset >= endOffset {
			break
		}
		result = append(result, entry)
	}

	return result
}

func (r *Reader) readDelta(offset int64, size int) ([]byte, error) {
	fileOffset := r.file.DeltaOffset + offset
	// Zero-copy slice from mmap'd data
	data := r.dedupMmap.Slice(fileOffset, size)
	if data == nil {
		return nil, fmt.Errorf("delta offset out of range")
	}
	return data, nil
}

// readViaRangeMapInto reads via range map directly into dest, avoiding allocation.
func (r *Reader) readViaRangeMapInto(fileIndex int, entry Entry, sourceOffset int64, dest []byte) error {
	src, ok := r.rangeMapsByFile[fileIndex]
	if !ok {
		return fmt.Errorf("no range map for source file %d", fileIndex)
	}

	if fileIndex < 0 || fileIndex >= len(r.sourceMmaps) || r.sourceMmaps[fileIndex] == nil {
		return fmt.Errorf("source file %d not loaded for range map read", fileIndex)
	}

	sourceData := r.sourceMmaps[fileIndex].Data()
	sourceSize := r.sourceMmaps[fileIndex].Size()

	if entry.IsVideo {
		if src.VideoMap == nil {
			return fmt.Errorf("no video range map for source file %d", fileIndex)
		}
		_, err := src.VideoMap.ReadDataInto(sourceData, sourceSize, sourceOffset, dest)
		return err
	}

	audioMap, ok := src.AudioMaps[entry.AudioSubStreamID]
	if !ok {
		return fmt.Errorf("no audio sub-stream %d range map for source file %d", entry.AudioSubStreamID, fileIndex)
	}
	_, err := audioMap.ReadDataInto(sourceData, sourceSize, sourceOffset, dest)
	return err
}

func (r *Reader) readSource(fileIndex int, offset int64, size int) ([]byte, error) {
	if fileIndex < 0 || fileIndex >= len(r.sourceMmaps) {
		return nil, fmt.Errorf("invalid file index: %d", fileIndex)
	}
	if r.sourceMmaps[fileIndex] == nil {
		return nil, fmt.Errorf("source file %d not loaded", fileIndex)
	}

	// Zero-copy slice from mmap'd data
	data := r.sourceMmaps[fileIndex].Slice(offset, size)
	if data == nil {
		return nil, fmt.Errorf("source offset out of range")
	}
	return data, nil
}

// parseHeaderOnly parses just the header and source files (not entries) for fast initialization.
func parseHeaderOnly(r io.Reader) (*File, error) {
	file := &File{}

	// Read and verify magic
	magic := make([]byte, MagicSize)
	if _, err := io.ReadFull(r, magic); err != nil {
		return nil, fmt.Errorf("read magic: %w", err)
	}
	if string(magic) != Magic {
		return nil, fmt.Errorf("invalid magic: %s", magic)
	}
	copy(file.Header.Magic[:], magic)

	// Read version
	if err := binary.Read(r, binary.LittleEndian, &file.Header.Version); err != nil {
		return nil, fmt.Errorf("read version: %w", err)
	}
	// Support versions 3-6. Older versions must be recreated.
	switch file.Header.Version {
	case Version, VersionRangeMap, VersionCreator, VersionRangeMapCreator:
		// OK
	case 1:
		return nil, fmt.Errorf("unsupported version 1 (uses ES offsets); please recreate with 'mkvdup create'")
	case 2:
		return nil, fmt.Errorf("unsupported version 2 (uses uint8 source index); please recreate with 'mkvdup create'")
	default:
		return nil, fmt.Errorf("unsupported version: %d (expected 3-6)", file.Header.Version)
	}

	// Read flags
	if err := binary.Read(r, binary.LittleEndian, &file.Header.Flags); err != nil {
		return nil, fmt.Errorf("read flags: %w", err)
	}

	// Read original size
	if err := binary.Read(r, binary.LittleEndian, &file.Header.OriginalSize); err != nil {
		return nil, fmt.Errorf("read original size: %w", err)
	}

	// Read original checksum
	if err := binary.Read(r, binary.LittleEndian, &file.Header.OriginalChecksum); err != nil {
		return nil, fmt.Errorf("read original checksum: %w", err)
	}

	// Read source type
	if err := binary.Read(r, binary.LittleEndian, &file.Header.SourceType); err != nil {
		return nil, fmt.Errorf("read source type: %w", err)
	}

	// Read uses ES offsets flag
	if err := binary.Read(r, binary.LittleEndian, &file.Header.UsesESOffsets); err != nil {
		return nil, fmt.Errorf("read uses ES offsets: %w", err)
	}
	file.UsesESOffsets = file.Header.UsesESOffsets == 1

	// Read source file count
	if err := binary.Read(r, binary.LittleEndian, &file.Header.SourceFileCount); err != nil {
		return nil, fmt.Errorf("read source file count: %w", err)
	}

	// Read entry count
	if err := binary.Read(r, binary.LittleEndian, &file.Header.EntryCount); err != nil {
		return nil, fmt.Errorf("read entry count: %w", err)
	}

	// Read delta offset
	if err := binary.Read(r, binary.LittleEndian, &file.Header.DeltaOffset); err != nil {
		return nil, fmt.Errorf("read delta offset: %w", err)
	}
	file.DeltaOffset = file.Header.DeltaOffset

	// Read delta size
	if err := binary.Read(r, binary.LittleEndian, &file.Header.DeltaSize); err != nil {
		return nil, fmt.Errorf("read delta size: %w", err)
	}

	// Read creator version string (V5/V6 only)
	file.headerSize = int64(HeaderSize)
	if file.Header.Version == VersionCreator || file.Header.Version == VersionRangeMapCreator {
		var versionLen uint16
		if err := binary.Read(r, binary.LittleEndian, &versionLen); err != nil {
			return nil, fmt.Errorf("read creator version length: %w", err)
		}
		if versionLen > MaxCreatorVersionLen {
			return nil, fmt.Errorf("creator version length %d exceeds maximum (%d)", versionLen, MaxCreatorVersionLen)
		}
		if versionLen > 0 {
			versionBytes := make([]byte, versionLen)
			if _, err := io.ReadFull(r, versionBytes); err != nil {
				return nil, fmt.Errorf("read creator version: %w", err)
			}
			file.CreatorVersion = string(versionBytes)
		}
		file.headerSize = int64(HeaderSize) + 2 + int64(versionLen)
	}

	// Read source files
	file.SourceFiles = make([]SourceFile, file.Header.SourceFileCount)
	for i := range file.SourceFiles {
		var pathLen uint16
		if err := binary.Read(r, binary.LittleEndian, &pathLen); err != nil {
			return nil, fmt.Errorf("read path length: %w", err)
		}

		path := make([]byte, pathLen)
		if _, err := io.ReadFull(r, path); err != nil {
			return nil, fmt.Errorf("read path: %w", err)
		}
		file.SourceFiles[i].RelativePath = string(path)

		if err := binary.Read(r, binary.LittleEndian, &file.SourceFiles[i].Size); err != nil {
			return nil, fmt.Errorf("read file size: %w", err)
		}

		if err := binary.Read(r, binary.LittleEndian, &file.SourceFiles[i].Checksum); err != nil {
			return nil, fmt.Errorf("read file checksum: %w", err)
		}
	}

	// Entries are accessed directly from mmap via Reader.getEntry()
	return file, nil
}

// VerifyIntegrity verifies the dedup file checksums.
func (r *Reader) VerifyIntegrity() error {
	// Initialize entry access to get entryCount
	if err := r.initEntryAccess(); err != nil {
		return fmt.Errorf("init entry access: %w", err)
	}

	hasRangeMaps := r.file.Header.Version == VersionRangeMap || r.file.Header.Version == VersionRangeMapCreator
	footerSz := int64(FooterSize)
	if hasRangeMaps {
		footerSz = int64(FooterV4Size)
	}

	fileSize := r.dedupMmap.Size()

	// Read footer from mmap
	footerOffset := fileSize - footerSz
	footerData := r.dedupMmap.Slice(footerOffset, int(footerSz))
	if footerData == nil {
		return fmt.Errorf("footer slice out of range")
	}

	var footer Footer
	off := 0
	footer.IndexChecksum = binary.LittleEndian.Uint64(footerData[off : off+8])
	off += 8
	footer.DeltaChecksum = binary.LittleEndian.Uint64(footerData[off : off+8])
	off += 8
	if hasRangeMaps {
		footer.RangeMapChecksum = binary.LittleEndian.Uint64(footerData[off : off+8])
		off += 8
	}
	if string(footerData[off:off+MagicSize]) != Magic {
		return fmt.Errorf("invalid footer magic")
	}

	// Calculate and verify index checksum (zero-copy)
	indexSize := int(int64(r.entryCount) * EntrySize)
	indexData := r.dedupMmap.Slice(r.indexStart, indexSize)
	if indexData == nil {
		return fmt.Errorf("read index for checksum: slice out of range")
	}
	if xxhash.Sum64(indexData) != footer.IndexChecksum {
		return fmt.Errorf("index checksum mismatch")
	}

	// Calculate and verify delta checksum (zero-copy)
	deltaData := r.dedupMmap.Slice(r.file.DeltaOffset, int(r.file.Header.DeltaSize))
	if deltaData == nil {
		return fmt.Errorf("read delta for checksum: slice out of range")
	}
	if xxhash.Sum64(deltaData) != footer.DeltaChecksum {
		return fmt.Errorf("delta checksum mismatch")
	}

	// V4/V6: verify range map checksum
	if hasRangeMaps {
		rangeMapOffset := r.file.DeltaOffset + r.file.Header.DeltaSize
		rangeMapSize := int(footerOffset - rangeMapOffset)
		if rangeMapSize > 0 {
			rangeMapData := r.dedupMmap.Slice(rangeMapOffset, rangeMapSize)
			if rangeMapData == nil {
				return fmt.Errorf("read range map for checksum: slice out of range")
			}
			if xxhash.Sum64(rangeMapData) != footer.RangeMapChecksum {
				return fmt.Errorf("range map checksum mismatch")
			}
		}
	}

	return nil
}

func (r *Reader) calculateSourceFilesSize() int64 {
	var size int64
	for _, sf := range r.file.SourceFiles {
		size += 2 + int64(len(sf.RelativePath)) + 8 + 8
	}
	return size
}

// Info returns a summary of the dedup file.
// If entry access initialization failed, the "error" key will contain the error message
// and "entry_count" will be 0.
func (r *Reader) Info() map[string]any {
	err := r.initEntryAccess() // Ensure entryCount is initialized
	info := map[string]any{
		"version":           r.file.Header.Version,
		"original_size":     r.file.Header.OriginalSize,
		"original_checksum": r.file.Header.OriginalChecksum,
		"source_type":       r.file.Header.SourceType,
		"uses_es_offsets":   r.file.UsesESOffsets,
		"has_range_maps":    r.rangeMapsByFile != nil,
		"source_file_count": len(r.file.SourceFiles),
		"entry_count":       r.entryCount,
		"delta_size":        r.file.Header.DeltaSize,
		"creator_version":   r.file.CreatorVersion,
	}
	if err != nil {
		info["error"] = err.Error()
	}
	return info
}
