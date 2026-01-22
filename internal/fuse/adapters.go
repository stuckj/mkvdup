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
	reader    *dedup.Reader
	sourceDir string
}

func (a *dedupReaderAdapter) OriginalSize() int64 {
	return a.reader.OriginalSize()
}

func (a *dedupReaderAdapter) UsesESOffsets() bool {
	return a.reader.UsesESOffsets()
}

func (a *dedupReaderAdapter) InitializeForReading(sourceDir string) error {
	if a.reader.UsesESOffsets() {
		// Create indexer to get ES reader
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
		// Note: We're not storing index here since the adapter is just for reading
		// In a production scenario, we'd want to track this for cleanup
	} else {
		// Load source files for raw access
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
	return a.reader.Close()
}

// DefaultReaderFactory is the default implementation of ReaderFactory.
type DefaultReaderFactory struct{}

func (f *DefaultReaderFactory) NewReaderLazy(dedupPath, sourceDir string) (ReaderInitializer, error) {
	reader, err := dedup.NewReaderLazy(dedupPath, sourceDir)
	if err != nil {
		return nil, err
	}
	return &dedupReaderAdapter{reader: reader, sourceDir: sourceDir}, nil
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
