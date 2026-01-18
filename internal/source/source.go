// Package source provides functionality for indexing source media files (DVD ISOs, Blu-ray directories).
package source

import (
	"errors"
	"os"
	"path/filepath"
)

// Type represents the type of source media.
type Type int

// Source type constants.
const (
	TypeDVD    Type = iota // Contains .iso file
	TypeBluray             // Contains BDMV/STREAM/*.m2ts
)

func (t Type) String() string {
	switch t {
	case TypeDVD:
		return "DVD"
	case TypeBluray:
		return "Blu-ray"
	default:
		return "Unknown"
	}
}

// ErrUnknownSourceType is returned when the source directory type cannot be determined.
var ErrUnknownSourceType = errors.New("unknown source type: directory contains neither ISO nor BDMV structure")

// DetectType determines whether a directory contains a DVD ISO or Blu-ray structure.
func DetectType(dir string) (Type, error) {
	// Check for ISO file (DVD)
	isos, err := filepath.Glob(filepath.Join(dir, "*.iso"))
	if err != nil {
		return 0, err
	}
	if len(isos) > 0 {
		return TypeDVD, nil
	}

	// Also check for ISO in subdirectory (common structure)
	isos, err = filepath.Glob(filepath.Join(dir, "*", "*.iso"))
	if err != nil {
		return 0, err
	}
	if len(isos) > 0 {
		return TypeDVD, nil
	}

	// Check for Blu-ray structure
	m2ts, err := filepath.Glob(filepath.Join(dir, "BDMV", "STREAM", "*.m2ts"))
	if err != nil {
		return 0, err
	}
	if len(m2ts) > 0 {
		return TypeBluray, nil
	}

	return 0, ErrUnknownSourceType
}

// File represents a source file within the source directory.
type File struct {
	RelativePath string // Path relative to source directory
	Size         int64
	Checksum     uint64 // xxhash of file for integrity
}

// Location represents a position within a source file where a hash was found.
type Location struct {
	FileIndex        uint16 // Index into Files array
	Offset           int64  // Offset within that file (or ES offset for MPEG-PS)
	IsVideo          bool   // For ES-based indexes: true for video ES, false for audio ES
	AudioSubStreamID byte   // For audio in MPEG-PS: sub-stream ID (0x80-0x87 = AC3, etc.)
}

// ESReader provides an interface for reading elementary stream data from container files.
type ESReader interface {
	// ReadESData reads size bytes of ES data starting at esOffset.
	// The data is continuous ES data, with container headers stripped.
	// For video, this works as expected. For audio, use ReadAudioSubStreamData instead.
	ReadESData(esOffset int64, size int, isVideo bool) ([]byte, error)
	// ESOffsetToFileOffset converts an ES offset to a file offset and remaining bytes in that segment.
	ESOffsetToFileOffset(esOffset int64, isVideo bool) (fileOffset int64, remaining int)
	// TotalESSize returns the total size of the elementary stream.
	// For video, returns filtered video ES size. For audio, returns 0 - use AudioSubStreamESSize.
	TotalESSize(isVideo bool) int64
	// AudioSubStreams returns the list of audio sub-stream IDs in order of appearance.
	AudioSubStreams() []byte
	// AudioSubStreamESSize returns the ES size for a specific audio sub-stream.
	AudioSubStreamESSize(subStreamID byte) int64
	// ReadAudioSubStreamData reads audio data from a specific sub-stream.
	ReadAudioSubStreamData(subStreamID byte, esOffset int64, size int) ([]byte, error)
}

// RawReader provides an interface for reading raw file data.
type RawReader interface {
	ReadAt(buf []byte, offset int64) (int, error)
	Len() int
	Close() error
}

// Index holds the hash-to-location mapping for fast lookup of byte sequences.
type Index struct {
	// HashToLocations maps from xxhash to list of locations where that hash was found
	HashToLocations map[uint64][]Location

	// SourceDir is the path to the source directory
	SourceDir string

	// SourceType indicates whether this is DVD or Blu-ray
	SourceType Type

	// Files lists all media files in the source
	Files []File

	// WindowSize is the number of bytes used for hashing
	WindowSize int

	// ESReaders provides ES-aware reading for each file (nil for raw files)
	// For MPEG-PS files, this allows reading continuous ES data.
	ESReaders []ESReader

	// RawReaders provides raw file reading for each file.
	// Used when raw file indexing is enabled.
	RawReaders []RawReader

	// UsesESOffsets indicates whether Location.Offset values are ES offsets
	// rather than raw file offsets. True for DVD (MPEG-PS) sources.
	UsesESOffsets bool
}

// NewIndex creates a new empty Index for the given source directory.
func NewIndex(sourceDir string, sourceType Type, windowSize int) *Index {
	return &Index{
		HashToLocations: make(map[uint64][]Location),
		SourceDir:       sourceDir,
		SourceType:      sourceType,
		WindowSize:      windowSize,
	}
}

// EnumerateMediaFiles returns the list of media files to index based on source type.
func EnumerateMediaFiles(dir string, sourceType Type) ([]string, error) {
	var files []string

	switch sourceType {
	case TypeDVD:
		// Look for ISO files
		isos, err := filepath.Glob(filepath.Join(dir, "*.iso"))
		if err != nil {
			return nil, err
		}
		files = append(files, isos...)

		// Also check subdirectory
		isos, err = filepath.Glob(filepath.Join(dir, "*", "*.iso"))
		if err != nil {
			return nil, err
		}
		files = append(files, isos...)

	case TypeBluray:
		// Look for m2ts files in BDMV/STREAM
		m2ts, err := filepath.Glob(filepath.Join(dir, "BDMV", "STREAM", "*.m2ts"))
		if err != nil {
			return nil, err
		}
		files = append(files, m2ts...)
	}

	// Convert to relative paths
	relFiles := make([]string, 0, len(files))
	for _, f := range files {
		rel, err := filepath.Rel(dir, f)
		if err != nil {
			return nil, err
		}
		relFiles = append(relFiles, rel)
	}

	return relFiles, nil
}

// GetFileInfo returns size information for a file.
func GetFileInfo(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// ReadRawDataAt reads raw data from the source file at the given location.
// This is used for raw file indexing (non-ES mode).
func (idx *Index) ReadRawDataAt(loc Location, size int) ([]byte, error) {
	if int(loc.FileIndex) >= len(idx.RawReaders) || idx.RawReaders[loc.FileIndex] == nil {
		return nil, errors.New("no raw reader for file")
	}
	buf := make([]byte, size)
	n, err := idx.RawReaders[loc.FileIndex].ReadAt(buf, loc.Offset)
	if err != nil && n < size {
		return buf[:n], err
	}
	return buf[:n], nil
}
