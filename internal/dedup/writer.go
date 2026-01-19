package dedup

import (
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
	file          *os.File
	header        Header
	sourceFiles   []SourceFile
	entries       []Entry
	deltaData     []byte
	usesESOffsets bool
}

// NewWriter creates a new dedup file writer.
func NewWriter(path string) (*Writer, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create file: %w", err)
	}
	return &Writer{file: f}, nil
}

// SetHeader sets the header information.
func (w *Writer) SetHeader(originalSize int64, originalChecksum uint64, sourceType source.Type, usesESOffsets bool) {
	copy(w.header.Magic[:], Magic)
	w.header.Version = Version
	w.header.Flags = 0
	w.header.OriginalSize = originalSize
	w.header.OriginalChecksum = originalChecksum
	w.usesESOffsets = usesESOffsets
	if usesESOffsets {
		w.header.UsesESOffsets = 1
	} else {
		w.header.UsesESOffsets = 0
	}

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

// SetMatchResult sets the match result (entries and delta).
func (w *Writer) SetMatchResult(result *matcher.Result) {
	w.entries = make([]Entry, len(result.Entries))
	for i, e := range result.Entries {
		w.entries[i] = FromMatcherEntry(e)
	}
	w.deltaData = result.DeltaData
	w.header.EntryCount = uint64(len(result.Entries))
	w.header.DeltaSize = int64(len(result.DeltaData))
}

// Write writes the dedup file.
func (w *Writer) Write() error {
	// Calculate offsets
	sourceFilesSize := w.calculateSourceFilesSize()
	indexSize := int64(len(w.entries)) * EntrySize
	deltaOffset := int64(HeaderSize) + sourceFilesSize + indexSize
	w.header.DeltaOffset = deltaOffset

	// Write header
	if err := w.writeHeader(); err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	// Write source files section
	if err := w.writeSourceFiles(); err != nil {
		return fmt.Errorf("write source files: %w", err)
	}

	// Write index entries and calculate checksum
	indexChecksum, err := w.writeEntries()
	if err != nil {
		return fmt.Errorf("write entries: %w", err)
	}

	// Write delta data and calculate checksum
	deltaChecksum, err := w.writeDelta()
	if err != nil {
		return fmt.Errorf("write delta: %w", err)
	}

	// Write footer
	if err := w.writeFooter(indexChecksum, deltaChecksum); err != nil {
		return fmt.Errorf("write footer: %w", err)
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
	for _, sf := range w.sourceFiles {
		// PathLen (2) + Path (variable) + Size (8) + Checksum (8)
		size += 2 + int64(len(sf.RelativePath)) + 8 + 8
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

	return nil
}

func (w *Writer) writeSourceFiles() error {
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
	}
	return nil
}

func (w *Writer) writeEntries() (uint64, error) {
	hasher := xxhash.New()
	writer := io.MultiWriter(w.file, hasher)

	for _, entry := range w.entries {
		// MkvOffset (8)
		if err := binary.Write(writer, binary.LittleEndian, entry.MkvOffset); err != nil {
			return 0, err
		}

		// Length (8)
		if err := binary.Write(writer, binary.LittleEndian, entry.Length); err != nil {
			return 0, err
		}

		// Source (1)
		if err := binary.Write(writer, binary.LittleEndian, entry.Source); err != nil {
			return 0, err
		}

		// SourceOffset (8)
		if err := binary.Write(writer, binary.LittleEndian, entry.SourceOffset); err != nil {
			return 0, err
		}

		// ES flags byte: bit 0 = IsVideo, bits 1-7 unused for video
		// For audio: bit 0 = 0 (audio), bits 1-7 unused here
		var esFlags uint8
		if entry.IsVideo {
			esFlags = 1
		}
		if err := binary.Write(writer, binary.LittleEndian, esFlags); err != nil {
			return 0, err
		}

		// AudioSubStreamID (1)
		if err := binary.Write(writer, binary.LittleEndian, entry.AudioSubStreamID); err != nil {
			return 0, err
		}
	}

	return hasher.Sum64(), nil
}

func (w *Writer) writeDelta() (uint64, error) {
	checksum := xxhash.Sum64(w.deltaData)
	if _, err := w.file.Write(w.deltaData); err != nil {
		return 0, err
	}
	return checksum, nil
}

func (w *Writer) writeFooter(indexChecksum, deltaChecksum uint64) error {
	// Write index checksum
	if err := binary.Write(w.file, binary.LittleEndian, indexChecksum); err != nil {
		return err
	}

	// Write delta checksum
	if err := binary.Write(w.file, binary.LittleEndian, deltaChecksum); err != nil {
		return err
	}

	// Write magic
	if _, err := w.file.Write([]byte(Magic)); err != nil {
		return err
	}

	return nil
}
