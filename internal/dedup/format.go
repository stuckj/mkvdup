// Package dedup provides reading and writing of .mkvdup deduplication files.
package dedup

import (
	"github.com/stuckj/mkvdup/internal/matcher"
	"github.com/stuckj/mkvdup/internal/source"
)

// File format constants
const (
	Magic   = "MKVDUP01"
	Version = 1
	// HeaderSize = Magic(8) + Version(4) + Flags(4) + OriginalSize(8) + OriginalChecksum(8) +
	//              SourceType(1) + UsesESOffsets(1) + SourceFileCount(2) + EntryCount(8) +
	//              DeltaOffset(8) + DeltaSize(8) = 60 bytes
	HeaderSize  = 60
	EntrySize   = 27 // Fixed entry size (25 + 2 for ES flags)
	FooterSize  = 24
	MagicSize   = 8
	VersionSize = 4
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
}

// Entry represents an index entry in the dedup file.
// This mirrors matcher.Entry but is specifically for serialization.
type Entry struct {
	MkvOffset        int64 // Start offset in the MKV file
	Length           int64 // Length of this region
	Source           uint8 // 0 = delta, 1+ = source file index + 1
	SourceOffset     int64 // Offset in source file (or ES offset)
	IsVideo          bool  // For ES-based sources
	AudioSubStreamID byte  // For ES-based audio sub-streams
}

// Footer represents the footer at the end of a .mkvdup file.
type Footer struct {
	IndexChecksum uint64  // xxhash of index section
	DeltaChecksum uint64  // xxhash of delta section
	Magic         [8]byte // "MKVDUP01" (for reverse scanning)
}

// File represents a complete dedup file structure for reconstruction.
type File struct {
	Header        Header
	SourceFiles   []SourceFile
	Entries       []Entry
	DeltaOffset   int64 // Offset to delta section in file
	UsesESOffsets bool
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
