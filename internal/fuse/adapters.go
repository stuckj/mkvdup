package fuse

import (
	"fmt"

	"github.com/stuckj/mkvdup/internal/dedup"
	"github.com/stuckj/mkvdup/internal/source"
)

// Ensure adapters implement interfaces
var _ ReaderInitializer = (*dedupReaderAdapter)(nil)
var _ ReaderFactory = (*DefaultReaderFactory)(nil)
var _ ConfigReader = (*DefaultConfigReader)(nil)

// dedupReaderAdapter wraps dedup.Reader to implement ReaderInitializer interface.
type dedupReaderAdapter struct {
	reader *dedup.Reader
	// index stores the source index for cleanup when using ES offsets.
	// This is nil when using raw source files.
	index *source.Index
}

func (a *dedupReaderAdapter) OriginalSize() int64 {
	return a.reader.OriginalSize()
}

func (a *dedupReaderAdapter) UsesESOffsets() bool {
	return a.reader.UsesESOffsets()
}

func (a *dedupReaderAdapter) InitializeForReading(sourceDir string) error {
	if a.reader.UsesESOffsets() && !a.reader.HasRangeMaps() {
		// V1/V3: ES offsets without range maps — need full ES reader
		indexer, err := source.NewIndexer(sourceDir, source.DefaultWindowSize)
		if err != nil {
			return fmt.Errorf("create indexer: %w", err)
		}
		if err := indexer.Build(nil); err != nil {
			return fmt.Errorf("build index: %w", err)
		}
		index := indexer.Index()
		if len(index.ESReaders) > 0 {
			a.reader.SetESReader(index.ESReaders[0])
		}
		// Store index for cleanup in Close()
		a.index = index
	} else {
		// V4 range maps or raw offsets: just need source files mmap'd.
		// Range maps handle ES-to-raw translation at read time.
		if err := a.reader.LoadSourceFiles(); err != nil {
			return fmt.Errorf("load source files: %w", err)
		}
	}
	return nil
}

func (a *dedupReaderAdapter) ReadAt(p []byte, off int64) (n int, err error) {
	return a.reader.ReadAt(p, off)
}

func (a *dedupReaderAdapter) Close() error {
	var errs []error
	if err := a.reader.Close(); err != nil {
		errs = append(errs, err)
	}
	if a.index != nil {
		if err := a.index.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// DefaultReaderFactory is the default implementation of ReaderFactory.
type DefaultReaderFactory struct{}

func (f *DefaultReaderFactory) NewReaderLazy(dedupPath, sourceDir string) (ReaderInitializer, error) {
	reader, err := dedup.NewReaderLazy(dedupPath, sourceDir)
	if err != nil {
		return nil, err
	}
	return &dedupReaderAdapter{reader: reader}, nil
}

// DefaultConfigReader is the default implementation of ConfigReader.
type DefaultConfigReader struct{}

func (r *DefaultConfigReader) ReadConfig(path string) (*Config, error) {
	config, err := dedup.ReadConfig(path)
	if err != nil {
		return nil, err
	}
	return &Config{
		Name:      config.Name,
		DedupFile: config.DedupFile,
		SourceDir: config.SourceDir,
	}, nil
}
