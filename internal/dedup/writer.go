package dedup

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/cespare/xxhash/v2"
	"github.com/stuckj/mkvdup/internal/matcher"
	"github.com/stuckj/mkvdup/internal/source"
)

// Writer creates .mkvdup files.
type Writer struct {
	file           *os.File
	header         Header
	sourceFiles    []SourceFile
	entries        []Entry
	deltaData      []byte               // In-memory delta (for tests / small files)
	deltaFile      *matcher.DeltaWriter // File-backed delta (for large files)
	rangeMaps      []RangeMapData       // V4/V6: per-source-file range maps (nil for V3/V5)
	rangeMapBuf    []byte               // Pre-encoded range map section (set by EncodeRangeMaps)
	creatorVersion string               // Version string to embed in the file
}

// NewWriter creates a new dedup file writer.
func NewWriter(path string) (*Writer, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create file: %w", err)
	}
	return &Writer{file: f}, nil
}

// SetCreatorVersion sets the version string to embed in the file.
// When set, the writer produces V7 (or V8 if range maps are also set).
func (w *Writer) SetCreatorVersion(v string) {
	if len(v) > MaxCreatorVersionLen {
		v = v[:MaxCreatorVersionLen]
	}
	w.creatorVersion = v
}

// SetHeader sets the header information.
func (w *Writer) SetHeader(originalSize int64, originalChecksum uint64, sourceType source.Type) {
	copy(w.header.Magic[:], Magic)
	w.header.Version = Version // Default V3; upgraded to V7/V8 in resolveVersion()
	w.header.Flags = 0
	w.header.OriginalSize = originalSize
	w.header.OriginalChecksum = originalChecksum
	w.header.UsesESOffsets = 0 // v2 always uses raw offsets

	switch sourceType {
	case source.TypeDVD:
		w.header.SourceType = SourceTypeDVD
	case source.TypeBluray:
		w.header.SourceType = SourceTypeBluray
	}
}

// SetSourceFiles sets the source file list.
func (w *Writer) SetSourceFiles(files []source.File) {
	w.sourceFiles = make([]SourceFile, len(files))
	for i, sf := range files {
		w.sourceFiles[i] = ToSourceFile(sf)
	}
	w.header.SourceFileCount = uint16(len(files))
}

// SetRangeMaps sets the range map data for V4 format.
// When range maps are set, ES-offset entries are preserved (not converted to raw offsets)
// and a range map section is written to the dedup file for mapping ES offsets to
// raw file positions at read time.
func (w *Writer) SetRangeMaps(rangeMaps []RangeMapData) {
	w.rangeMaps = rangeMaps
	w.header.Version = VersionRangeMap // Default V4; upgraded to V8 in resolveVersion()
	w.header.UsesESOffsets = 1
}

// resolveVersion sets the final file version based on configured features.
func (w *Writer) resolveVersion() {
	if w.rangeMaps != nil {
		if w.creatorVersion != "" {
			w.header.Version = VersionRangeMapUsed // V8
		} else {
			w.header.Version = VersionRangeMap // V4
		}
	} else {
		if w.creatorVersion != "" {
			w.header.Version = VersionUsed // V7
		} else {
			w.header.Version = Version // V3
		}
	}
}

// computeUsedFlags scans entries and marks which source files are referenced.
func (w *Writer) computeUsedFlags() {
	for i := range w.sourceFiles {
		w.sourceFiles[i].Used = false
	}
	for _, e := range w.entries {
		if e.Source > 0 {
			idx := int(e.Source - 1)
			if idx < len(w.sourceFiles) {
				w.sourceFiles[idx].Used = true
			}
		}
	}
}

// EncodeRangeMaps pre-encodes the range map section. Call this before
// WriteWithProgress to avoid a CPU-intensive encoding phase with no progress
// output. Returns the encoded size. If range maps are nil, this is a no-op.
func (w *Writer) EncodeRangeMaps() (int64, error) {
	if w.rangeMaps == nil {
		return 0, nil
	}
	buf, err := encodeRangeMapSection(w.rangeMaps)
	if err != nil {
		return 0, fmt.Errorf("encode range maps: %w", err)
	}
	w.rangeMapBuf = buf
	return int64(len(buf)), nil
}

// SetMatchResult sets the match result (entries and delta).
// If esConverters is provided and non-empty, ES-offset entries will be converted
// to raw-offset entries, potentially splitting entries that span multiple ranges.
func (w *Writer) SetMatchResult(result *matcher.Result, esConverters []source.ESRangeConverter) error {
	// Convert matcher entries to dedup entries
	entries := make([]Entry, len(result.Entries))
	for i, e := range result.Entries {
		entries[i] = FromMatcherEntry(e)
	}

	// Convert ES offsets to raw offsets if we have converters.
	// Skip conversion for V4 (range maps handle the mapping at read time).
	if len(esConverters) > 0 && w.rangeMaps == nil {
		var err error
		entries, err = w.convertESToRawOffsets(entries, esConverters)
		if err != nil {
			return fmt.Errorf("convert ES to raw offsets: %w", err)
		}
	}

	w.entries = entries
	w.header.EntryCount = uint64(len(w.entries))

	if result.DeltaFile != nil {
		w.deltaFile = result.DeltaFile
		w.header.DeltaSize = result.DeltaFile.Size()
	} else {
		w.deltaData = result.DeltaData
		w.header.DeltaSize = int64(len(result.DeltaData))
	}
	return nil
}

// convertESToRawOffsets converts ES-offset entries to raw-offset entries.
// Entries that span multiple PES payload ranges are split into multiple entries.
func (w *Writer) convertESToRawOffsets(entries []Entry, esConverters []source.ESRangeConverter) ([]Entry, error) {
	// Pre-allocate with ~2x capacity since entries typically expand to multiple raw ranges
	result := make([]Entry, 0, len(entries)*2)

	for _, entry := range entries {
		if entry.Source == 0 {
			// Delta entry - no conversion needed
			result = append(result, entry)
			continue
		}

		// Get the ES converter for this source file
		fileIndex := int(entry.Source - 1)
		if fileIndex >= len(esConverters) || esConverters[fileIndex] == nil {
			// No converter available - assume raw offsets already
			result = append(result, entry)
			continue
		}
		converter := esConverters[fileIndex]

		// Get raw ranges for this ES region
		var rawRanges []source.RawRange
		var err error
		if entry.IsVideo {
			rawRanges, err = converter.RawRangesForESRegion(entry.SourceOffset, int(entry.Length), true)
		} else {
			rawRanges, err = converter.RawRangesForAudioSubStream(entry.AudioSubStreamID, entry.SourceOffset, int(entry.Length))
		}
		if err != nil {
			return nil, fmt.Errorf("convert entry at MKV offset %d: %w", entry.MkvOffset, err)
		}

		// Create one entry per raw range
		mkvOffset := entry.MkvOffset
		for _, rr := range rawRanges {
			result = append(result, Entry{
				MkvOffset:        mkvOffset,
				Length:           int64(rr.Size),
				Source:           entry.Source,
				SourceOffset:     rr.FileOffset, // Raw file offset!
				IsVideo:          entry.IsVideo,
				AudioSubStreamID: entry.AudioSubStreamID,
			})
			mkvOffset += int64(rr.Size)
		}
	}

	return result, nil
}

// WriteProgressFunc is called to report write progress.
type WriteProgressFunc func(written, total int64)

// Write writes the dedup file.
func (w *Writer) Write() error {
	return w.WriteWithProgress(nil)
}

// WriteWithProgress writes the dedup file with progress reporting.
func (w *Writer) WriteWithProgress(progress WriteProgressFunc) error {
	// Determine final file version based on configured features.
	w.resolveVersion()
	w.computeUsedFlags()

	// Use pre-encoded range maps if available (from EncodeRangeMaps),
	// otherwise encode now.
	rangeMapBuf := w.rangeMapBuf
	if rangeMapBuf == nil && w.rangeMaps != nil {
		var err error
		rangeMapBuf, err = encodeRangeMapSection(w.rangeMaps)
		if err != nil {
			return fmt.Errorf("encode range maps: %w", err)
		}
	}

	// Calculate offsets and total size
	sourceFilesSize := w.calculateSourceFilesSize()
	cvSize := creatorVersionSize(w.creatorVersion)
	indexSize := int64(len(w.entries)) * EntrySize
	deltaOffset := int64(HeaderSize) + cvSize + sourceFilesSize + indexSize
	w.header.DeltaOffset = deltaOffset

	footerSize := int64(FooterSize)
	if rangeMapBuf != nil {
		footerSize = FooterV4Size
	}

	totalSize := deltaOffset + w.header.DeltaSize + int64(len(rangeMapBuf)) + footerSize
	var written int64

	// Write header (includes creator version for V5/V6)
	if err := w.writeHeader(); err != nil {
		return fmt.Errorf("write header: %w", err)
	}
	written += int64(HeaderSize) + cvSize

	// Write source files section
	if err := w.writeSourceFiles(); err != nil {
		return fmt.Errorf("write source files: %w", err)
	}
	written += sourceFilesSize

	// Write index entries and calculate checksum
	indexChecksum, err := w.writeEntriesWithProgress(progress, &written, totalSize)
	if err != nil {
		return fmt.Errorf("write entries: %w", err)
	}

	// Write delta data and calculate checksum
	deltaChecksum, err := w.writeDeltaWithProgress(progress, &written, totalSize)
	if err != nil {
		return fmt.Errorf("write delta: %w", err)
	}

	// Write range map section (V4 only)
	var rangeMapChecksum uint64
	if rangeMapBuf != nil {
		rangeMapChecksum, err = writeRangeMapSection(w.file, rangeMapBuf)
		if err != nil {
			return fmt.Errorf("write range map: %w", err)
		}
		written += int64(len(rangeMapBuf))
		if progress != nil {
			progress(written, totalSize)
		}
	}

	// Write footer
	if err := w.writeFooter(indexChecksum, deltaChecksum, rangeMapChecksum); err != nil {
		return fmt.Errorf("write footer: %w", err)
	}

	if progress != nil {
		progress(totalSize, totalSize)
	}

	return nil
}

// Close closes the writer.
func (w *Writer) Close() error {
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

func (w *Writer) calculateSourceFilesSize() int64 {
	var size int64
	hasUsed := w.header.Version == VersionUsed || w.header.Version == VersionRangeMapUsed
	for _, sf := range w.sourceFiles {
		// PathLen (2) + Path (variable) + Size (8) + Checksum (8) [+ Used (1)]
		size += 2 + int64(len(sf.RelativePath)) + 8 + 8
		if hasUsed {
			size += 1
		}
	}
	return size
}

func (w *Writer) writeHeader() error {
	// Write magic
	if _, err := w.file.Write([]byte(Magic)); err != nil {
		return err
	}

	// Write version
	if err := binary.Write(w.file, binary.LittleEndian, w.header.Version); err != nil {
		return err
	}

	// Write flags
	if err := binary.Write(w.file, binary.LittleEndian, w.header.Flags); err != nil {
		return err
	}

	// Write original size
	if err := binary.Write(w.file, binary.LittleEndian, w.header.OriginalSize); err != nil {
		return err
	}

	// Write original checksum
	if err := binary.Write(w.file, binary.LittleEndian, w.header.OriginalChecksum); err != nil {
		return err
	}

	// Write source type
	if err := binary.Write(w.file, binary.LittleEndian, w.header.SourceType); err != nil {
		return err
	}

	// Write uses ES offsets flag
	if err := binary.Write(w.file, binary.LittleEndian, w.header.UsesESOffsets); err != nil {
		return err
	}

	// Write source file count
	if err := binary.Write(w.file, binary.LittleEndian, w.header.SourceFileCount); err != nil {
		return err
	}

	// Write entry count
	if err := binary.Write(w.file, binary.LittleEndian, w.header.EntryCount); err != nil {
		return err
	}

	// Write delta offset
	if err := binary.Write(w.file, binary.LittleEndian, w.header.DeltaOffset); err != nil {
		return err
	}

	// Write delta size
	if err := binary.Write(w.file, binary.LittleEndian, w.header.DeltaSize); err != nil {
		return err
	}

	// Write creator version string (V5/V6)
	if w.creatorVersion != "" {
		versionLen := uint16(len(w.creatorVersion))
		if err := binary.Write(w.file, binary.LittleEndian, versionLen); err != nil {
			return err
		}
		if _, err := w.file.Write([]byte(w.creatorVersion)); err != nil {
			return err
		}
	}

	return nil
}

func (w *Writer) writeSourceFiles() error {
	hasUsed := w.header.Version == VersionUsed || w.header.Version == VersionRangeMapUsed
	for _, sf := range w.sourceFiles {
		// Write path length
		pathLen := uint16(len(sf.RelativePath))
		if err := binary.Write(w.file, binary.LittleEndian, pathLen); err != nil {
			return err
		}

		// Write path
		if _, err := w.file.Write([]byte(sf.RelativePath)); err != nil {
			return err
		}

		// Write size
		if err := binary.Write(w.file, binary.LittleEndian, sf.Size); err != nil {
			return err
		}

		// Write checksum
		if err := binary.Write(w.file, binary.LittleEndian, sf.Checksum); err != nil {
			return err
		}

		// Write used flag (V7/V8)
		if hasUsed {
			var used uint8
			if sf.Used {
				used = 1
			}
			if err := binary.Write(w.file, binary.LittleEndian, used); err != nil {
				return err
			}
		}
	}
	return nil
}

func (w *Writer) writeEntriesWithProgress(progress WriteProgressFunc, written *int64, total int64) (uint64, error) {
	hasher := xxhash.New()
	// Use buffered writer to batch syscalls (64KB buffer)
	bufWriter := bufio.NewWriterSize(w.file, 64*1024)
	writer := io.MultiWriter(bufWriter, hasher)

	entryCount := len(w.entries)
	lastProgress := 0

	// Reusable buffer for entry serialization (allocation-free per entry)
	var entryBuf [EntrySize]byte

	for i, entry := range w.entries {
		// Serialize entry to buffer using allocation-free Put* functions
		binary.LittleEndian.PutUint64(entryBuf[0:8], uint64(entry.MkvOffset))
		binary.LittleEndian.PutUint64(entryBuf[8:16], uint64(entry.Length))
		binary.LittleEndian.PutUint16(entryBuf[16:18], entry.Source)
		binary.LittleEndian.PutUint64(entryBuf[18:26], uint64(entry.SourceOffset))

		// ES flags byte: bit 0 = IsVideo, bit 1 = IsLPCM
		var esFlags uint8
		if entry.IsVideo {
			esFlags |= 1
		}
		if entry.IsLPCM {
			esFlags |= 2
		}
		entryBuf[26] = esFlags
		entryBuf[27] = entry.AudioSubStreamID

		// Single write per entry
		if _, err := writer.Write(entryBuf[:]); err != nil {
			return 0, err
		}

		*written += EntrySize

		// Report progress every 1% or 10000 entries
		if progress != nil && entryCount > 0 {
			pct := (i + 1) * 100 / entryCount
			if pct > lastProgress || (i+1)%10000 == 0 {
				progress(*written, total)
				lastProgress = pct
			}
		}
	}

	// Flush buffered writer
	if err := bufWriter.Flush(); err != nil {
		return 0, err
	}

	return hasher.Sum64(), nil
}

func (w *Writer) writeDeltaWithProgress(progress WriteProgressFunc, written *int64, total int64) (uint64, error) {
	hasher := xxhash.New()
	const chunkSize = 64 * 1024 // 64KB chunks
	lastProgress := 0

	if w.deltaFile != nil {
		// Read from temp file and write to output
		f := w.deltaFile.File()
		if _, err := f.Seek(0, 0); err != nil {
			return 0, fmt.Errorf("seek delta file: %w", err)
		}

		buf := make([]byte, chunkSize)
		for {
			n, err := f.Read(buf)
			if n > 0 {
				chunk := buf[:n]
				if _, werr := w.file.Write(chunk); werr != nil {
					return 0, werr
				}
				hasher.Write(chunk)
				*written += int64(n)

				if progress != nil && w.header.DeltaSize > 0 {
					pct := int((*written * 100) / total)
					if pct > lastProgress {
						progress(*written, total)
						lastProgress = pct
					}
				}
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				return 0, err
			}
		}
	} else {
		// In-memory path (for tests / small files)
		data := w.deltaData
		for len(data) > 0 {
			chunk := data
			if len(chunk) > chunkSize {
				chunk = data[:chunkSize]
			}
			data = data[len(chunk):]

			if _, err := w.file.Write(chunk); err != nil {
				return 0, err
			}
			hasher.Write(chunk)
			*written += int64(len(chunk))

			if progress != nil && w.header.DeltaSize > 0 {
				pct := int((*written * 100) / total)
				if pct > lastProgress {
					progress(*written, total)
					lastProgress = pct
				}
			}
		}
	}

	return hasher.Sum64(), nil
}

func (w *Writer) writeFooter(indexChecksum, deltaChecksum, rangeMapChecksum uint64) error {
	// Write index checksum
	if err := binary.Write(w.file, binary.LittleEndian, indexChecksum); err != nil {
		return err
	}

	// Write delta checksum
	if err := binary.Write(w.file, binary.LittleEndian, deltaChecksum); err != nil {
		return err
	}

	// Write range map checksum (V4 only)
	if w.rangeMaps != nil {
		if err := binary.Write(w.file, binary.LittleEndian, rangeMapChecksum); err != nil {
			return err
		}
	}

	// Write magic
	if _, err := w.file.Write([]byte(Magic)); err != nil {
		return err
	}

	return nil
}
