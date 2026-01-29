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

	// Block index for O(1) entry lookup on cache miss.
	// Maps block_number (MKV offset / blockSize) → entry index.
	// Built once in initEntryAccess; immutable after that (no mutex needed).
	blockIndex []int32

	// Last-entry cache for O(1) sequential read lookup
	// Protected by cacheMu for concurrent access safety
	cacheMu        sync.Mutex
	lastEntryIdx   int   // Index of last accessed entry (-1 if none)
	lastEntry      Entry // The cached parsed entry
	lastEntryValid bool  // Whether lastEntry is valid
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
		r.indexStart = int64(HeaderSize) + r.calculateSourceFilesSize()
		r.entryCount = int(r.file.Header.EntryCount)

		// Validate mmap has enough data for all entries
		requiredSize := r.indexStart + int64(r.entryCount)*EntrySize
		if int64(r.dedupMmap.Size()) < requiredSize {
			r.entriesErr = fmt.Errorf("mmap too small: need %d, have %d",
				requiredSize, r.dedupMmap.Size())
			return
		}

		// Build block index for O(1) random access lookup
		r.buildBlockIndex()
	})
	return r.entriesErr
}

// buildBlockIndex creates a mapping from block numbers to entry indices.
// Each block represents a fixed-size range of MKV offsets. The index maps
// block_number → the entry index whose region covers or precedes that block's
// start offset. This enables O(1) lookup for random access patterns.
//
// Algorithm: single pass over all entries, filling block slots as we go.
// Time: O(entryCount + blockCount), Space: O(blockCount).
func (r *Reader) buildBlockIndex() {
	originalSize := r.file.Header.OriginalSize
	if originalSize <= 0 || r.entryCount == 0 {
		return
	}

	blockCount := int((originalSize + blockSize - 1) / blockSize)
	r.blockIndex = make([]int32, blockCount)

	entryIdx := 0
	for b := range blockCount {
		blockStart := int64(b) * blockSize
		// Advance entryIdx to the last entry whose MkvOffset <= blockStart
		for entryIdx+1 < r.entryCount {
			nextOffset, ok := r.getMkvOffset(entryIdx + 1)
			if !ok || nextOffset > blockStart {
				break
			}
			entryIdx++
		}
		r.blockIndex[b] = int32(entryIdx)
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

	// Find entries that cover this range
	entries := r.findEntriesForRange(offset, int64(remaining))

	for _, entry := range entries {
		if remaining <= 0 {
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

		// Read data from appropriate source
		var data []byte
		var err error

		if entry.Source == 0 {
			// Read from delta section
			data, err = r.readDelta(sourceOffset, readLen)
		} else if r.file.UsesESOffsets && r.esReader != nil {
			// Read from ES
			if entry.IsVideo {
				data, err = r.esReader.ReadESData(sourceOffset, readLen, true)
			} else {
				data, err = r.esReader.ReadAudioSubStreamData(entry.AudioSubStreamID, sourceOffset, readLen)
			}
		} else {
			// Read from raw source file
			fileIndex := int(entry.Source - 1)
			data, err = r.readSource(fileIndex, sourceOffset, readLen)
		}

		if err != nil {
			return totalRead, fmt.Errorf("read at offset %d: %w", readStart, err)
		}

		// Copy to output buffer - use original offset to calculate buffer position
		bufOffset := int(readStart - originalOffset)
		copy(buf[bufOffset:], data)
		totalRead += len(data)
		remaining -= len(data)
		offset = readEnd
	}

	if totalRead == 0 && len(buf) > 0 {
		return 0, io.EOF
	}

	return totalRead, nil
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
		lo = int(r.blockIndex[blockNum])

		// Upper bound: start of next block's entries (or entryCount)
		if blockNum+1 < len(r.blockIndex) {
			// Search up to 1 past the next block's start entry to handle
			// entries that span block boundaries
			hi = int(r.blockIndex[blockNum+1]) + 1
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
	// Support current version (3) only. Older versions must be recreated.
	if file.Header.Version != Version {
		if file.Header.Version == 1 {
			return nil, fmt.Errorf("unsupported version 1 (uses ES offsets); please recreate with 'mkvdup create'")
		}
		if file.Header.Version == 2 {
			return nil, fmt.Errorf("unsupported version 2 (uses uint8 source index); please recreate with 'mkvdup create'")
		}
		return nil, fmt.Errorf("unsupported version: %d (expected %d)", file.Header.Version, Version)
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

	f, err := os.Open(r.dedupPath)
	if err != nil {
		return fmt.Errorf("open dedup file: %w", err)
	}
	defer f.Close()

	// Get file size
	stat, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat dedup file: %w", err)
	}

	// Read footer
	footerOffset := stat.Size() - FooterSize
	if _, err := f.Seek(footerOffset, 0); err != nil {
		return fmt.Errorf("seek to footer: %w", err)
	}

	var footer Footer
	if err := binary.Read(f, binary.LittleEndian, &footer.IndexChecksum); err != nil {
		return fmt.Errorf("read index checksum: %w", err)
	}
	if err := binary.Read(f, binary.LittleEndian, &footer.DeltaChecksum); err != nil {
		return fmt.Errorf("read delta checksum: %w", err)
	}
	footerMagic := make([]byte, MagicSize)
	if _, err := io.ReadFull(f, footerMagic); err != nil {
		return fmt.Errorf("read footer magic: %w", err)
	}
	if string(footerMagic) != Magic {
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
		"source_file_count": len(r.file.SourceFiles),
		"entry_count":       r.entryCount,
		"delta_size":        r.file.Header.DeltaSize,
	}
	if err != nil {
		info["error"] = err.Error()
	}
	return info
}
