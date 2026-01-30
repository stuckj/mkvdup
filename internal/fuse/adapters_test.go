package fuse

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stuckj/mkvdup/internal/dedup"
	"github.com/stuckj/mkvdup/internal/matcher"
	"github.com/stuckj/mkvdup/internal/source"
)

// createTestDedupFile creates a minimal .mkvdup file in dir with the given
// originalSize and returns its path. The file contains a single delta entry
// covering the full original size.
func createTestDedupFile(t *testing.T, dir string, originalSize int64) string {
	t.Helper()
	path := filepath.Join(dir, "test.mkvdup")
	w, err := dedup.NewWriter(path)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	defer w.Close()

	w.SetHeader(originalSize, 0x12345678, source.TypeDVD)
	w.SetSourceFiles([]source.File{
		{RelativePath: "test.iso", Size: 1000, Checksum: 0xABCD},
	})
	result := &matcher.Result{
		Entries: []matcher.Entry{
			{MkvOffset: 0, Length: originalSize, Source: 0, SourceOffset: 0},
		},
		DeltaData: make([]byte, originalSize),
	}
	if err := w.SetMatchResult(result, nil); err != nil {
		t.Fatalf("SetMatchResult: %v", err)
	}
	if err := w.Write(); err != nil {
		t.Fatalf("Write: %v", err)
	}
	return path
}

func TestDedupReaderAdapter_OriginalSize(t *testing.T) {
	dir := t.TempDir()
	const wantSize int64 = 12345

	path := createTestDedupFile(t, dir, wantSize)

	reader, err := dedup.NewReaderLazy(path, dir)
	if err != nil {
		t.Fatalf("NewReaderLazy: %v", err)
	}

	adapter := &dedupReaderAdapter{reader: reader}
	defer adapter.Close()

	if got := adapter.OriginalSize(); got != wantSize {
		t.Errorf("OriginalSize() = %d, want %d", got, wantSize)
	}
}

func TestDedupReaderAdapter_UsesESOffsets(t *testing.T) {
	dir := t.TempDir()

	path := createTestDedupFile(t, dir, 100)

	reader, err := dedup.NewReaderLazy(path, dir)
	if err != nil {
		t.Fatalf("NewReaderLazy: %v", err)
	}

	adapter := &dedupReaderAdapter{reader: reader}
	defer adapter.Close()

	// v2/v3 dedup files do not use ES offsets by default.
	if got := adapter.UsesESOffsets(); got {
		t.Errorf("UsesESOffsets() = %v, want false", got)
	}
}

func TestDedupReaderAdapter_Close_NoIndex(t *testing.T) {
	dir := t.TempDir()

	path := createTestDedupFile(t, dir, 100)

	reader, err := dedup.NewReaderLazy(path, dir)
	if err != nil {
		t.Fatalf("NewReaderLazy: %v", err)
	}

	adapter := &dedupReaderAdapter{reader: reader, index: nil}

	if err := adapter.Close(); err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}

func TestDedupReaderAdapter_Close_WithIndex(t *testing.T) {
	dir := t.TempDir()

	path := createTestDedupFile(t, dir, 100)

	reader, err := dedup.NewReaderLazy(path, dir)
	if err != nil {
		t.Fatalf("NewReaderLazy: %v", err)
	}

	index := source.NewIndex(dir, source.TypeDVD, 64)

	adapter := &dedupReaderAdapter{reader: reader, index: index}

	if err := adapter.Close(); err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}

func TestDefaultReaderFactory_NewReaderLazy_Success(t *testing.T) {
	dir := t.TempDir()
	const wantSize int64 = 9999

	path := createTestDedupFile(t, dir, wantSize)

	factory := &DefaultReaderFactory{}
	ri, err := factory.NewReaderLazy(path, "/fake/source")
	if err != nil {
		t.Fatalf("NewReaderLazy: %v", err)
	}
	defer ri.Close()

	if ri == nil {
		t.Fatal("NewReaderLazy returned nil ReaderInitializer")
	}
	if got := ri.OriginalSize(); got != wantSize {
		t.Errorf("OriginalSize() = %d, want %d", got, wantSize)
	}
}

func TestDefaultReaderFactory_NewReaderLazy_NotFound(t *testing.T) {
	factory := &DefaultReaderFactory{}
	_, err := factory.NewReaderLazy("/nonexistent/path/test.mkvdup", "/fake/source")
	if err == nil {
		t.Fatal("expected error for nonexistent path, got nil")
	}
}

func TestDefaultConfigReader_ReadConfig_Success(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "test.mkvdup.yaml")

	wantName := "My Movie"
	wantDedupFile := "/data/movies/movie.mkvdup"
	wantSourceDir := "/mnt/dvd"

	if err := dedup.WriteConfig(configPath, wantName, wantDedupFile, wantSourceDir); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}

	cr := &DefaultConfigReader{}
	cfg, err := cr.ReadConfig(configPath)
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}

	if cfg.Name != wantName {
		t.Errorf("Name = %q, want %q", cfg.Name, wantName)
	}
	if cfg.DedupFile != wantDedupFile {
		t.Errorf("DedupFile = %q, want %q", cfg.DedupFile, wantDedupFile)
	}
	if cfg.SourceDir != wantSourceDir {
		t.Errorf("SourceDir = %q, want %q", cfg.SourceDir, wantSourceDir)
	}
}

func TestDefaultConfigReader_ReadConfig_NotFound(t *testing.T) {
	cr := &DefaultConfigReader{}
	_, err := cr.ReadConfig("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent path, got nil")
	}
}

func TestDefaultConfigReader_ReadConfig_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "bad.yaml")

	if err := os.WriteFile(configPath, []byte("not: valid: yaml: [[["), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cr := &DefaultConfigReader{}
	_, err := cr.ReadConfig(configPath)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}
