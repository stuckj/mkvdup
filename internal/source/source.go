// Package source provides functionality for indexing source media files (DVD ISOs, Blu-ray directories).
package source

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/stuckj/mkvdup/internal/mmap"
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
// ISOs are inspected to determine if they contain DVD (VIDEO_TS) or Blu-ray (BDMV) content.
func DetectType(dir string) (Type, error) {
	// Check for ISO files
	isos, err := filepath.Glob(filepath.Join(dir, "*.iso"))
	if err != nil {
		return 0, err
	}

	// Also check for ISO in subdirectory (common structure)
	subIsos, err := filepath.Glob(filepath.Join(dir, "*", "*.iso"))
	if err != nil {
		return 0, err
	}
	isos = append(isos, subIsos...)

	// If we found ISOs, inspect them to determine type
	if len(isos) > 0 {
		// Check the first ISO to determine type
		isoType, err := detectISOType(isos[0])
		if err != nil {
			// If we can't read the ISO, default to DVD (legacy behavior)
			return TypeDVD, nil
		}
		return isoType, nil
	}

	// Check for Blu-ray directory structure
	m2ts, err := filepath.Glob(filepath.Join(dir, "BDMV", "STREAM", "*.m2ts"))
	if err != nil {
		return 0, err
	}
	if len(m2ts) > 0 {
		return TypeBluray, nil
	}

	return 0, ErrUnknownSourceType
}

// detectISOType examines an ISO file to determine if it's a DVD or Blu-ray.
// DVDs have VIDEO_TS directory, Blu-rays have BDMV directory.
// Uses minimal reads to avoid loading the entire ISO into memory.
func detectISOType(isoPath string) (Type, error) {
	f, err := os.Open(isoPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	// ISO9660 primary volume descriptor is at sector 16 (2048 bytes per sector)
	// The root directory record is embedded in the volume descriptor at offset 156.
	const sectorSize = 2048
	const pvdOffset = 16 * sectorSize

	// Read the primary volume descriptor
	pvd := make([]byte, sectorSize)
	if _, err := f.ReadAt(pvd, pvdOffset); err != nil {
		return 0, err
	}

	// Check volume descriptor type (byte 0) and signature "CD001" (bytes 1-5)
	if pvd[0] != 1 || string(pvd[1:6]) != "CD001" {
		// No ISO9660 PVD. Check for UDF (Blu-ray ISOs from CloneBD).
		if isUDFImage(f) {
			return detectUDFISOType(f)
		}
		return TypeDVD, nil
	}

	// Root directory record is at offset 156, length at byte 0 of the record
	rootDirRecord := pvd[156:]
	if len(rootDirRecord) < 34 {
		return TypeDVD, nil
	}

	// Extract root directory extent location (bytes 2-5, little-endian)
	rootExtent := uint32(rootDirRecord[2]) | uint32(rootDirRecord[3])<<8 |
		uint32(rootDirRecord[4])<<16 | uint32(rootDirRecord[5])<<24
	// Extract root directory data length (bytes 10-13, little-endian)
	rootDataLen := uint32(rootDirRecord[10]) | uint32(rootDirRecord[11])<<8 |
		uint32(rootDirRecord[12])<<16 | uint32(rootDirRecord[13])<<24

	// Read the root directory
	// Limit to first 16KB to avoid reading huge directories
	if rootDataLen > 16*1024 {
		rootDataLen = 16 * 1024
	}
	rootDir := make([]byte, rootDataLen)
	if _, err := f.ReadAt(rootDir, int64(rootExtent)*sectorSize); err != nil {
		return 0, err
	}

	// Parse directory entries looking for VIDEO_TS or BDMV
	hasBDMV := false
	hasVideoTS := false

	offset := 0
	for offset < len(rootDir) {
		recLen := int(rootDir[offset])
		if recLen == 0 {
			// Move to next sector boundary
			nextSector := ((offset / sectorSize) + 1) * sectorSize
			if nextSector >= len(rootDir) {
				break
			}
			offset = nextSector
			continue
		}
		if offset+recLen > len(rootDir) {
			break
		}

		// Name length is at offset 32
		if offset+33 > len(rootDir) {
			break
		}
		nameLen := int(rootDir[offset+32])
		if offset+33+nameLen > len(rootDir) {
			break
		}

		// Extract and check the filename
		name := strings.ToUpper(string(rootDir[offset+33 : offset+33+nameLen]))
		// Strip version number (;1) if present
		if idx := strings.Index(name, ";"); idx >= 0 {
			name = name[:idx]
		}
		// Strip trailing dot if present
		name = strings.TrimSuffix(name, ".")

		if name == "BDMV" {
			hasBDMV = true
		}
		if name == "VIDEO_TS" {
			hasVideoTS = true
		}

		offset += recLen
	}

	// Blu-ray takes precedence if both are present
	if hasBDMV {
		return TypeBluray, nil
	}
	if hasVideoTS {
		return TypeDVD, nil
	}

	// Default to DVD for unrecognized ISOs
	return TypeDVD, nil
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

// ESRangeConverter provides an interface for converting ES offsets to raw file offsets.
// This is used during dedup file creation to convert ES-based entries to raw-offset entries.
type ESRangeConverter interface {
	// RawRangesForESRegion returns the raw file ranges that contain the given ES region.
	// Each returned range represents a contiguous chunk of raw file data.
	// The sum of all returned range sizes equals the requested ES region size.
	// For video streams only - audio should use RawRangesForAudioSubStream.
	RawRangesForESRegion(esOffset int64, size int, isVideo bool) ([]RawRange, error)
	// RawRangesForAudioSubStream returns the raw file ranges for audio data from a specific sub-stream.
	RawRangesForAudioSubStream(subStreamID byte, esOffset int64, size int) ([]RawRange, error)
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

// PESRangeProvider provides access to PES payload ranges for building range maps.
// Both MPEGPSParser and MPEGTSParser implement this.
type PESRangeProvider interface {
	FilteredVideoRanges() []PESPayloadRange
	FilteredAudioRanges(subStreamID byte) []PESPayloadRange
	AudioSubStreams() []byte
}

// RawReader provides an interface for reading raw file data.
type RawReader interface {
	ReadAt(buf []byte, offset int64) (int, error)
	// Slice returns a zero-copy slice of the underlying data.
	// Returns nil if offset is out of range.
	Slice(offset int64, size int) []byte
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

	// MmapFiles holds the mmap file handles for proper cleanup.
	// These back the ESReaders for MPEG-PS files.
	MmapFiles []*mmap.File

	// UsesESOffsets indicates whether Location.Offset values are ES offsets
	// rather than raw file offsets. True for DVD (MPEG-PS) sources.
	UsesESOffsets bool

	// sortOnce ensures SortLocationsByOffset runs only once.
	sortOnce sync.Once
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

// SortLocationsByOffset sorts all location slices by (FileIndex, Offset).
// This is a one-time cost at match setup time that enables binary search
// for nearby locations during matching. Must be called before concurrent access.
func (idx *Index) SortLocationsByOffset() {
	idx.sortOnce.Do(func() {
		for hash, locs := range idx.HashToLocations {
			if len(locs) > 1 {
				sort.Slice(locs, func(i, j int) bool {
					if locs[i].FileIndex != locs[j].FileIndex {
						return locs[i].FileIndex < locs[j].FileIndex
					}
					return locs[i].Offset < locs[j].Offset
				})
				idx.HashToLocations[hash] = locs
			}
		}
	})
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
		// Look for m2ts files in BDMV/STREAM (extracted Blu-ray)
		m2ts, err := filepath.Glob(filepath.Join(dir, "BDMV", "STREAM", "*.m2ts"))
		if err != nil {
			return nil, err
		}
		files = append(files, m2ts...)

		// If no extracted M2TS files, look for Blu-ray ISOs
		if len(files) == 0 {
			isos, err := filepath.Glob(filepath.Join(dir, "*.iso"))
			if err != nil {
				return nil, err
			}
			files = append(files, isos...)

			// Also check subdirectory (same pattern as DVD)
			isos, err = filepath.Glob(filepath.Join(dir, "*", "*.iso"))
			if err != nil {
				return nil, err
			}
			files = append(files, isos...)
		}
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
// Note: This copies data. Prefer RawSlice for zero-copy access.
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

// RawSlice returns a zero-copy slice of raw data at the given location.
// Returns nil if the location is out of range.
func (idx *Index) RawSlice(loc Location, size int) []byte {
	if int(loc.FileIndex) >= len(idx.RawReaders) || idx.RawReaders[loc.FileIndex] == nil {
		return nil
	}
	return idx.RawReaders[loc.FileIndex].Slice(loc.Offset, size)
}
