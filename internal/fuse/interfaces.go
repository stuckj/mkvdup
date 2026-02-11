// Package fuse provides a FUSE filesystem for accessing deduplicated MKV files.
package fuse

// DedupReader is an interface for reading deduplicated MKV files.
// This allows mocking the dedup.Reader in tests.
type DedupReader interface {
	// OriginalSize returns the original size of the MKV file.
	OriginalSize() int64

	// ReadAt reads data at the specified offset.
	ReadAt(p []byte, off int64) (n int, err error)

	// Close closes the reader.
	Close() error
}

// SourceFileInfo contains metadata about a source file referenced by a dedup file.
// This is read from the dedup file header (available without full reader initialization).
type SourceFileInfo struct {
	RelativePath string // Path relative to source directory
	Size         int64  // Expected file size
	Checksum     uint64 // Expected xxhash checksum
}

// ReaderInitializer is an interface for initializing readers with source data.
// This is separate from DedupReader to allow simpler mocking for basic read tests.
type ReaderInitializer interface {
	DedupReader

	// UsesESOffsets returns true if the dedup file uses ES offsets.
	UsesESOffsets() bool

	// InitializeForReading prepares the reader for reading.
	// For ES-based sources, this sets up the ES reader.
	// For raw sources, this loads source files.
	InitializeForReading(sourceDir string) error

	// SourceFileInfo returns metadata about source files referenced by the
	// dedup file. Available from the header without full initialization.
	SourceFileInfo() []SourceFileInfo
}

// ReaderFactory creates DedupReader instances.
// This allows mocking reader creation in tests.
type ReaderFactory interface {
	// NewReaderLazy creates a new lazy-loading reader.
	NewReaderLazy(dedupPath, sourceDir string) (ReaderInitializer, error)
}

// ConfigReader reads dedup configuration files.
// This allows mocking config reading in tests.
type ConfigReader interface {
	// ReadConfig reads a dedup configuration file.
	ReadConfig(path string) (*Config, error)
}

// Config represents a dedup configuration.
type Config struct {
	Name      string
	DedupFile string
	SourceDir string
}
