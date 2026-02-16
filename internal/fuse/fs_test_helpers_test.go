package fuse

import (
	"errors"
	"io"
)

// mockReader implements ReaderInitializer for testing.
type mockReader struct {
	data          []byte
	originalSize  int64
	usesESOffsets bool
	initErr       error
	readErr       error
	closed        bool
}

func (m *mockReader) OriginalSize() int64 {
	return m.originalSize
}

func (m *mockReader) UsesESOffsets() bool {
	return m.usesESOffsets
}

func (m *mockReader) InitializeForReading(sourceDir string) error {
	return m.initErr
}

func (m *mockReader) ReadAt(p []byte, off int64) (n int, err error) {
	if m.readErr != nil {
		return 0, m.readErr
	}
	if off >= int64(len(m.data)) {
		return 0, io.EOF
	}
	n = copy(p, m.data[off:])
	if off+int64(n) >= int64(len(m.data)) {
		return n, io.EOF
	}
	return n, nil
}

func (m *mockReader) SourceFileInfo() []SourceFileInfo {
	return nil
}

func (m *mockReader) Close() error {
	m.closed = true
	return nil
}

// mockReaderFactory implements ReaderFactory for testing.
type mockReaderFactory struct {
	readers map[string]*mockReader
	err     error
}

func (f *mockReaderFactory) NewReaderLazy(dedupPath, sourceDir string) (ReaderInitializer, error) {
	if f.err != nil {
		return nil, f.err
	}
	if reader, ok := f.readers[dedupPath]; ok {
		return reader, nil
	}
	return nil, errors.New("reader not found for path: " + dedupPath)
}

// mockConfigReader implements ConfigReader for testing.
type mockConfigReader struct {
	configs map[string]*Config
	err     error
}

func (r *mockConfigReader) ReadConfig(path string) (*Config, error) {
	if r.err != nil {
		return nil, r.err
	}
	if config, ok := r.configs[path]; ok {
		return config, nil
	}
	return nil, errors.New("config not found: " + path)
}
