package dedup

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"

	"github.com/cespare/xxhash/v2"
	"golang.org/x/exp/mmap"
)

// Reader reads .mkvdup files and provides data reconstruction.
type Reader struct {
	file        *File
	dedupMmap   *mmap.ReaderAt
	dedupPath   string
	sourceDir   string
	sourceMmaps []*mmap.ReaderAt
	esReader    ESReader  // For ES-based sources
	entriesOnce sync.Once // For lazy loading entries
	entriesErr  error     // Error from lazy loading
}

// ESReader interface for reading ES data from MPEG-PS sources.
type ESReader interface {
	ReadESData(esOffset int64, size int, isVideo bool) ([]byte, error)
	ReadAudioSubStreamData(subStreamID byte, esOffset int64, size int) ([]byte, error)
}

// NewReader opens a dedup file for reading.
// This parses the full file including all entries - use NewReaderLazy for faster initialization.
func NewReader(dedupPath, sourceDir string) (*Reader, error) {
	f, err := os.Open(dedupPath)
	if err != nil {
		return nil, fmt.Errorf("open dedup file: %w", err)
	}
	defer f.Close()

	file, err := parseFile(f)
	if err != nil {
		return nil, fmt.Errorf("parse dedup file: %w", err)
	}

	// Memory-map the dedup file
	dedupMmap, err := mmap.Open(dedupPath)
	if err != nil {
		return nil, fmt.Errorf("mmap dedup file: %w", err)
	}

	return &Reader{
		file:      file,
		dedupMmap: dedupMmap,
		dedupPath: dedupPath,
		sourceDir: sourceDir,
	}, nil
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
		file:      file,
		dedupMmap: dedupMmap,
		dedupPath: dedupPath,
		sourceDir: sourceDir,
	}, nil
}

// SetESReader sets the ES reader for ES-based sources.
func (r *Reader) SetESReader(esReader ESReader) {
	r.esReader = esReader
}

// LoadSourceFiles memory-maps all source files.
func (r *Reader) LoadSourceFiles() error {
	r.sourceMmaps = make([]*mmap.ReaderAt, len(r.file.SourceFiles))
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

// loadEntries loads entries from the mmap (for lazy loading).
func (r *Reader) loadEntries() error {
	r.entriesOnce.Do(func() {
		if r.file.Entries != nil {
			return // Already loaded
		}

		// Calculate index start offset
		indexStart := int64(HeaderSize) + r.calculateSourceFilesSize()
		entryCount := int(r.file.Header.EntryCount)

		// Read all entries from mmap
		indexSize := int64(entryCount) * EntrySize
		indexData := make([]byte, indexSize)
		n, err := r.dedupMmap.ReadAt(indexData, indexStart)
		if err != nil && n < int(indexSize) {
			r.entriesErr = fmt.Errorf("read entries: %w", err)
			return
		}

		// Parse entries
		r.file.Entries = make([]Entry, entryCount)
		buf := bytes.NewReader(indexData)
		for i := 0; i < entryCount; i++ {
			if err := binary.Read(buf, binary.LittleEndian, &r.file.Entries[i].MkvOffset); err != nil {
				r.entriesErr = fmt.Errorf("read entry MkvOffset: %w", err)
				return
			}
			if err := binary.Read(buf, binary.LittleEndian, &r.file.Entries[i].Length); err != nil {
				r.entriesErr = fmt.Errorf("read entry Length: %w", err)
				return
			}
			if err := binary.Read(buf, binary.LittleEndian, &r.file.Entries[i].Source); err != nil {
				r.entriesErr = fmt.Errorf("read entry Source: %w", err)
				return
			}
			if err := binary.Read(buf, binary.LittleEndian, &r.file.Entries[i].SourceOffset); err != nil {
				r.entriesErr = fmt.Errorf("read entry SourceOffset: %w", err)
				return
			}

			var esFlags uint8
			if err := binary.Read(buf, binary.LittleEndian, &esFlags); err != nil {
				r.entriesErr = fmt.Errorf("read entry esFlags: %w", err)
				return
			}
			r.file.Entries[i].IsVideo = esFlags&1 == 1

			if err := binary.Read(buf, binary.LittleEndian, &r.file.Entries[i].AudioSubStreamID); err != nil {
				r.entriesErr = fmt.Errorf("read entry AudioSubStreamID: %w", err)
				return
			}
		}
	})
	return r.entriesErr
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
func (r *Reader) EntryCount() int {
	return len(r.file.Entries)
}

// UsesESOffsets returns true if this dedup file uses ES offsets.
func (r *Reader) UsesESOffsets() bool {
	return r.file.UsesESOffsets
}

// ReadAt reads reconstructed MKV data at the given offset.
func (r *Reader) ReadAt(buf []byte, offset int64) (int, error) {
	// Load entries on first access (lazy loading)
	if err := r.loadEntries(); err != nil {
		return 0, fmt.Errorf("load entries: %w", err)
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
	// Binary search for first entry that could contain offset
	entries := r.file.Entries
	idx := sort.Search(len(entries), func(i int) bool {
		return entries[i].MkvOffset+entries[i].Length > offset
	})

	var result []Entry
	endOffset := offset + length
	for i := idx; i < len(entries) && entries[i].MkvOffset < endOffset; i++ {
		result = append(result, entries[i])
	}
	return result
}

func (r *Reader) readDelta(offset int64, size int) ([]byte, error) {
	fileOffset := r.file.DeltaOffset + offset
	data := make([]byte, size)
	n, err := r.dedupMmap.ReadAt(data, fileOffset)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return data[:n], nil
}

func (r *Reader) readSource(fileIndex int, offset int64, size int) ([]byte, error) {
	if fileIndex < 0 || fileIndex >= len(r.sourceMmaps) {
		return nil, fmt.Errorf("invalid file index: %d", fileIndex)
	}
	if r.sourceMmaps[fileIndex] == nil {
		return nil, fmt.Errorf("source file %d not loaded", fileIndex)
	}

	data := make([]byte, size)
	n, err := r.sourceMmaps[fileIndex].ReadAt(data, offset)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return data[:n], nil
}

// parseFile parses a dedup file from a reader.
func parseFile(r io.Reader) (*File, error) {
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
	if file.Header.Version != Version {
		return nil, fmt.Errorf("unsupported version: %d", file.Header.Version)
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

	// Read entries
	file.Entries = make([]Entry, file.Header.EntryCount)
	for i := range file.Entries {
		if err := binary.Read(r, binary.LittleEndian, &file.Entries[i].MkvOffset); err != nil {
			return nil, fmt.Errorf("read entry MkvOffset: %w", err)
		}
		if err := binary.Read(r, binary.LittleEndian, &file.Entries[i].Length); err != nil {
			return nil, fmt.Errorf("read entry Length: %w", err)
		}
		if err := binary.Read(r, binary.LittleEndian, &file.Entries[i].Source); err != nil {
			return nil, fmt.Errorf("read entry Source: %w", err)
		}
		if err := binary.Read(r, binary.LittleEndian, &file.Entries[i].SourceOffset); err != nil {
			return nil, fmt.Errorf("read entry SourceOffset: %w", err)
		}

		var esFlags uint8
		if err := binary.Read(r, binary.LittleEndian, &esFlags); err != nil {
			return nil, fmt.Errorf("read entry esFlags: %w", err)
		}
		file.Entries[i].IsVideo = esFlags&1 == 1

		if err := binary.Read(r, binary.LittleEndian, &file.Entries[i].AudioSubStreamID); err != nil {
			return nil, fmt.Errorf("read entry AudioSubStreamID: %w", err)
		}
	}

	return file, nil
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
	if file.Header.Version != Version {
		return nil, fmt.Errorf("unsupported version: %d", file.Header.Version)
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

	// Don't read entries - they will be loaded lazily
	// Entries is nil, indicating they need to be loaded
	file.Entries = nil

	return file, nil
}

// VerifyIntegrity verifies the dedup file checksums.
func (r *Reader) VerifyIntegrity() error {
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

	// Calculate and verify index checksum
	indexStart := HeaderSize + r.calculateSourceFilesSize()
	indexSize := int64(len(r.file.Entries)) * EntrySize
	indexData := make([]byte, indexSize)
	if _, err := r.dedupMmap.ReadAt(indexData, indexStart); err != nil {
		return fmt.Errorf("read index for checksum: %w", err)
	}
	if xxhash.Sum64(indexData) != footer.IndexChecksum {
		return fmt.Errorf("index checksum mismatch")
	}

	// Calculate and verify delta checksum
	deltaData := make([]byte, r.file.Header.DeltaSize)
	if _, err := r.dedupMmap.ReadAt(deltaData, r.file.DeltaOffset); err != nil {
		return fmt.Errorf("read delta for checksum: %w", err)
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
func (r *Reader) Info() map[string]interface{} {
	return map[string]interface{}{
		"original_size":     r.file.Header.OriginalSize,
		"original_checksum": r.file.Header.OriginalChecksum,
		"source_type":       r.file.Header.SourceType,
		"uses_es_offsets":   r.file.UsesESOffsets,
		"source_file_count": len(r.file.SourceFiles),
		"entry_count":       len(r.file.Entries),
		"delta_size":        r.file.Header.DeltaSize,
	}
}
