// Package dedup provides reading and writing of .mkvdup deduplication files.
package dedup

import (
	"encoding/binary"

	"github.com/stuckj/mkvdup/internal/matcher"
	"github.com/stuckj/mkvdup/internal/source"
)

// File format constants
const (
	Magic   = "MKVDUP01"
	Version = 3 // v3: Source field expanded to uint16 for >256 source files
	// VersionRangeMap is the version for files with embedded range maps.
	// Entries use ES offsets; a range map section maps ES offsets to raw file offsets.
	VersionRangeMap uint32 = 4
	// VersionCreator is V3 with a creator version string after the header.
	VersionCreator uint32 = 5
	// VersionRangeMapCreator is V4 with a creator version string after the header.
	VersionRangeMapCreator uint32 = 6
	// VersionUsed is V5 with a per-source-file Used byte after the checksum.
	VersionUsed uint32 = 7
	// VersionRangeMapUsed is V6 with a per-source-file Used byte after the checksum.
	VersionRangeMapUsed uint32 = 8
	// HeaderSize = Magic(8) + Version(4) + Flags(4) + OriginalSize(8) + OriginalChecksum(8) +
	//              SourceType(1) + UsesESOffsets(1) + SourceFileCount(2) + EntryCount(8) +
	//              DeltaOffset(8) + DeltaSize(8) = 60 bytes
	HeaderSize           = 60
	EntrySize            = 28 // Fixed entry size: 8+8+2+8+1+1 = 28 bytes
	FooterSize           = 24
	FooterV4Size         = 32 // V4 footer adds RangeMapChecksum (8 bytes)
	MagicSize            = 8
	VersionSize          = 4
	MaxCreatorVersionLen = 4096 // Max bytes for creator version string (writer truncates, reader rejects)
)

// Source types
const (
	SourceTypeDVD    uint8 = 0
	SourceTypeBluray uint8 = 1
)

// Header represents the fixed header at the start of a .mkvdup file.
type Header struct {
	Magic            [8]byte // "MKVDUP01"
	Version          uint32  // File format version
	Flags            uint32  // Reserved for future use
	OriginalSize     int64   // Size of original MKV file
	OriginalChecksum uint64  // xxhash of original MKV file
	SourceType       uint8   // 0=DVD, 1=Blu-ray
	UsesESOffsets    uint8   // 1 if source uses ES offsets (MPEG-PS)
	SourceFileCount  uint16  // Number of source files
	EntryCount       uint64  // Number of index entries
	DeltaOffset      int64   // Offset to delta section
	DeltaSize        int64   // Size of delta section
}

// SourceFile represents a source file entry in the dedup file.
type SourceFile struct {
	RelativePath string // Path relative to source directory
	Size         int64  // File size
	Checksum     uint64 // xxhash of file
	Used         bool   // Whether this source file is referenced by any entry (V7/V8 only)
}

// Entry represents an index entry in the dedup file.
// This mirrors matcher.Entry but is specifically for serialization.
type Entry struct {
	MkvOffset        int64  // Start offset in the MKV file
	Length           int64  // Length of this region
	Source           uint16 // 0 = delta, 1+ = source file index + 1 (supports up to 65535 files)
	SourceOffset     int64  // Offset in source file (or ES offset)
	IsVideo          bool   // For ES-based sources
	AudioSubStreamID byte   // For ES-based audio sub-streams
}

// RawEntry matches the 28-byte on-disk entry format exactly.
// Uses byte arrays for int64 fields to handle unaligned access portably.
// This enables direct memory-mapped access without parsing into []Entry.
type RawEntry struct {
	MkvOffset        [8]byte // int64, little-endian
	Length           [8]byte // int64, little-endian
	Source           [2]byte // uint16, little-endian
	SourceOffset     [8]byte // int64, little-endian (unaligned at byte 18)
	ESFlags          uint8   // bit 0 = IsVideo
	AudioSubStreamID uint8
}

// ToEntry converts a RawEntry to an Entry by parsing the byte arrays.
func (r *RawEntry) ToEntry() Entry {
	return Entry{
		MkvOffset:        int64(binary.LittleEndian.Uint64(r.MkvOffset[:])),
		Length:           int64(binary.LittleEndian.Uint64(r.Length[:])),
		Source:           binary.LittleEndian.Uint16(r.Source[:]),
		SourceOffset:     int64(binary.LittleEndian.Uint64(r.SourceOffset[:])),
		IsVideo:          r.ESFlags&1 == 1,
		AudioSubStreamID: r.AudioSubStreamID,
	}
}

// Footer represents the footer at the end of a .mkvdup file.
type Footer struct {
	IndexChecksum    uint64  // xxhash of index section
	DeltaChecksum    uint64  // xxhash of delta section
	RangeMapChecksum uint64  // xxhash of range map section (V4 only; 0 for V3)
	Magic            [8]byte // "MKVDUP01" (for reverse scanning)
}

// File represents a complete dedup file structure for reconstruction.
// Note: Entries are accessed directly from mmap via Reader.getEntry(),
// not stored in this struct, to avoid large memory allocation.
type File struct {
	Header         Header
	SourceFiles    []SourceFile
	DeltaOffset    int64 // Offset to delta section in file
	UsesESOffsets  bool
	CreatorVersion string // Version of mkvdup that created this file (V5/V6 only)
	headerSize     int64  // Effective header size (60 for V3/V4, 60+2+len for V5-V8)
}

// creatorVersionSize returns the on-disk size of the creator version field.
func creatorVersionSize(v string) int64 {
	if v == "" {
		return 0
	}
	return 2 + int64(len(v))
}

// ToMatcherEntry converts a dedup Entry to a matcher Entry.
func (e *Entry) ToMatcherEntry() matcher.Entry {
	return matcher.Entry{
		MkvOffset:        e.MkvOffset,
		Length:           e.Length,
		Source:           e.Source,
		SourceOffset:     e.SourceOffset,
		IsVideo:          e.IsVideo,
		AudioSubStreamID: e.AudioSubStreamID,
	}
}

// FromMatcherEntry creates a dedup Entry from a matcher Entry.
func FromMatcherEntry(e matcher.Entry) Entry {
	return Entry{
		MkvOffset:        e.MkvOffset,
		Length:           e.Length,
		Source:           e.Source,
		SourceOffset:     e.SourceOffset,
		IsVideo:          e.IsVideo,
		AudioSubStreamID: e.AudioSubStreamID,
	}
}

// ToSourceFile converts source.File to dedup SourceFile.
func ToSourceFile(sf source.File) SourceFile {
	return SourceFile{
		RelativePath: sf.RelativePath,
		Size:         sf.Size,
		Checksum:     sf.Checksum,
	}
}
