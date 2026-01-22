package fuse

import (
	"context"
	"errors"
	"io"
	"sort"
	"syscall"
	"testing"

	"github.com/hanwen/go-fuse/v2/fuse"
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

func TestNewMKVFSWithFactories(t *testing.T) {
	testData := []byte("test MKV data content")

	configReader := &mockConfigReader{
		configs: map[string]*Config{
			"/configs/movie.yaml": {
				Name:      "movie.mkv",
				DedupFile: "/data/movie.dedup",
				SourceDir: "/data/source",
			},
		},
	}

	readerFactory := &mockReaderFactory{
		readers: map[string]*mockReader{
			"/data/movie.dedup": {
				data:         testData,
				originalSize: int64(len(testData)),
			},
		},
	}

	root, err := NewMKVFSWithFactories(
		[]string{"/configs/movie.yaml"},
		false,
		readerFactory,
		configReader,
	)
	if err != nil {
		t.Fatalf("NewMKVFSWithFactories failed: %v", err)
	}

	if len(root.files) != 1 {
		t.Errorf("expected 1 file, got %d", len(root.files))
	}

	file, ok := root.files["movie.mkv"]
	if !ok {
		t.Fatal("expected movie.mkv in files")
	}

	if file.Size != int64(len(testData)) {
		t.Errorf("expected size %d, got %d", len(testData), file.Size)
	}
}

func TestNewMKVFSWithFactories_ConfigError(t *testing.T) {
	configReader := &mockConfigReader{
		err: errors.New("config read error"),
	}

	readerFactory := &mockReaderFactory{}

	_, err := NewMKVFSWithFactories(
		[]string{"/configs/movie.yaml"},
		false,
		readerFactory,
		configReader,
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, configReader.err) && err.Error() != "read config /configs/movie.yaml: config read error" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNewMKVFSWithFactories_ReaderError(t *testing.T) {
	configReader := &mockConfigReader{
		configs: map[string]*Config{
			"/configs/movie.yaml": {
				Name:      "movie.mkv",
				DedupFile: "/data/movie.dedup",
				SourceDir: "/data/source",
			},
		},
	}

	readerFactory := &mockReaderFactory{
		err: errors.New("reader open error"),
	}

	_, err := NewMKVFSWithFactories(
		[]string{"/configs/movie.yaml"},
		false,
		readerFactory,
		configReader,
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMKVFSRoot_Readdir(t *testing.T) {
	root := &MKVFSRoot{
		files: map[string]*MKVFile{
			"movie1.mkv": {Name: "movie1.mkv", Size: 1000},
			"movie2.mkv": {Name: "movie2.mkv", Size: 2000},
		},
	}

	stream, errno := root.Readdir(context.Background())
	if errno != 0 {
		t.Fatalf("Readdir returned errno %d", errno)
	}

	var entries []string
	for stream.HasNext() {
		entry, _ := stream.Next()
		entries = append(entries, entry.Name)
	}

	sort.Strings(entries)
	expected := []string{"movie1.mkv", "movie2.mkv"}
	if len(entries) != len(expected) {
		t.Fatalf("expected %d entries, got %d", len(expected), len(entries))
	}
	for i, name := range expected {
		if entries[i] != name {
			t.Errorf("entry %d: expected %s, got %s", i, name, entries[i])
		}
	}
}

func TestMKVFSRoot_Lookup_Found(t *testing.T) {
	// Note: Full Lookup test requires FUSE infrastructure (NewInode).
	// Here we test that the file is found and attributes are set correctly.
	// Integration tests cover the full Lookup flow with a real mount.
	root := &MKVFSRoot{
		files: map[string]*MKVFile{
			"movie.mkv": {Name: "movie.mkv", Size: 12345},
		},
	}

	// Test that file exists in root
	root.mu.RLock()
	file, ok := root.files["movie.mkv"]
	root.mu.RUnlock()

	if !ok {
		t.Fatal("expected movie.mkv in files")
	}
	if file.Size != 12345 {
		t.Errorf("expected size 12345, got %d", file.Size)
	}
}

func TestMKVFSRoot_Lookup_NotFound(t *testing.T) {
	root := &MKVFSRoot{
		files: map[string]*MKVFile{},
	}

	var out fuse.EntryOut
	_, errno := root.Lookup(context.Background(), "nonexistent.mkv", &out)
	if errno != syscall.ENOENT {
		t.Errorf("expected ENOENT, got %d", errno)
	}
}

func TestMKVFSNode_Getattr(t *testing.T) {
	file := &MKVFile{
		Name: "movie.mkv",
		Size: 54321,
	}
	node := &MKVFSNode{file: file}

	var out fuse.AttrOut
	errno := node.Getattr(context.Background(), nil, &out)
	if errno != 0 {
		t.Fatalf("Getattr returned errno %d", errno)
	}

	if out.Size != 54321 {
		t.Errorf("expected size 54321, got %d", out.Size)
	}
	if out.Nlink != 1 {
		t.Errorf("expected nlink 1, got %d", out.Nlink)
	}
}

func TestMKVFSNode_Read(t *testing.T) {
	testData := []byte("Hello, FUSE filesystem!")
	mockRdr := &mockReader{
		data:         testData,
		originalSize: int64(len(testData)),
	}

	file := &MKVFile{
		Name:   "test.mkv",
		Size:   int64(len(testData)),
		reader: mockRdr,
	}
	node := &MKVFSNode{file: file}

	// Read full content
	buf := make([]byte, len(testData))
	result, errno := node.Read(context.Background(), nil, buf, 0)
	if errno != 0 {
		t.Fatalf("Read returned errno %d", errno)
	}

	data, _ := result.Bytes(buf)
	if string(data) != string(testData) {
		t.Errorf("expected %q, got %q", testData, data)
	}
}

func TestMKVFSNode_Read_Partial(t *testing.T) {
	testData := []byte("Hello, FUSE filesystem!")
	mockRdr := &mockReader{
		data:         testData,
		originalSize: int64(len(testData)),
	}

	file := &MKVFile{
		Name:   "test.mkv",
		Size:   int64(len(testData)),
		reader: mockRdr,
	}
	node := &MKVFSNode{file: file}

	// Read from offset
	buf := make([]byte, 5)
	result, errno := node.Read(context.Background(), nil, buf, 7)
	if errno != 0 {
		t.Fatalf("Read returned errno %d", errno)
	}

	data, _ := result.Bytes(buf)
	if string(data) != "FUSE " {
		t.Errorf("expected %q, got %q", "FUSE ", data)
	}
}

func TestMKVFSNode_Read_BeyondEOF(t *testing.T) {
	testData := []byte("short")
	mockRdr := &mockReader{
		data:         testData,
		originalSize: int64(len(testData)),
	}

	file := &MKVFile{
		Name:   "test.mkv",
		Size:   int64(len(testData)),
		reader: mockRdr,
	}
	node := &MKVFSNode{file: file}

	// Read at offset beyond file size
	buf := make([]byte, 10)
	result, errno := node.Read(context.Background(), nil, buf, 100)
	if errno != 0 {
		t.Fatalf("Read returned errno %d", errno)
	}

	data, _ := result.Bytes(buf)
	if len(data) != 0 {
		t.Errorf("expected empty data, got %d bytes", len(data))
	}
}

func TestMKVFSNode_Read_NoReader(t *testing.T) {
	file := &MKVFile{
		Name:   "test.mkv",
		Size:   100,
		reader: nil, // No reader initialized
	}
	node := &MKVFSNode{file: file}

	buf := make([]byte, 10)
	_, errno := node.Read(context.Background(), nil, buf, 0)
	if errno != syscall.EIO {
		t.Errorf("expected EIO, got %d", errno)
	}
}

func TestMKVFSNode_Open(t *testing.T) {
	testData := []byte("test data")
	mockRdr := &mockReader{
		data:         testData,
		originalSize: int64(len(testData)),
	}

	readerFactory := &mockReaderFactory{
		readers: map[string]*mockReader{
			"/path/to/movie.dedup": mockRdr,
		},
	}

	file := &MKVFile{
		Name:          "movie.mkv",
		DedupPath:     "/path/to/movie.dedup",
		SourceDir:     "/path/to/source",
		Size:          int64(len(testData)),
		readerFactory: readerFactory,
	}
	node := &MKVFSNode{file: file}

	_, _, errno := node.Open(context.Background(), 0)
	if errno != 0 {
		t.Fatalf("Open returned errno %d", errno)
	}

	// Verify reader was initialized
	if file.reader == nil {
		t.Error("expected reader to be initialized")
	}
}

func TestMKVFSNode_Open_InitError(t *testing.T) {
	mockRdr := &mockReader{
		originalSize: 100,
		initErr:      errors.New("initialization failed"),
	}

	readerFactory := &mockReaderFactory{
		readers: map[string]*mockReader{
			"/path/to/movie.dedup": mockRdr,
		},
	}

	file := &MKVFile{
		Name:          "movie.mkv",
		DedupPath:     "/path/to/movie.dedup",
		SourceDir:     "/path/to/source",
		Size:          100,
		readerFactory: readerFactory,
	}
	node := &MKVFSNode{file: file}

	_, _, errno := node.Open(context.Background(), 0)
	if errno != syscall.EIO {
		t.Errorf("expected EIO, got %d", errno)
	}
}

func TestMKVFile_Close(t *testing.T) {
	mockRdr := &mockReader{}
	file := &MKVFile{
		Name:   "test.mkv",
		reader: mockRdr,
	}

	file.Close()

	if !mockRdr.closed {
		t.Error("expected reader to be closed")
	}
	if file.reader != nil {
		t.Error("expected reader to be nil after close")
	}
}

func TestMKVFile_Close_NoReader(t *testing.T) {
	file := &MKVFile{
		Name:   "test.mkv",
		reader: nil,
	}

	// Should not panic
	file.Close()
}

func TestHashString(t *testing.T) {
	// Test that same string produces same hash
	h1 := hashString("movie.mkv")
	h2 := hashString("movie.mkv")
	if h1 != h2 {
		t.Error("same string should produce same hash")
	}

	// Test that different strings produce different hashes
	h3 := hashString("other.mkv")
	if h1 == h3 {
		t.Error("different strings should produce different hashes")
	}

	// Test empty string doesn't panic
	_ = hashString("")
}

func TestNewMKVFSWithFactories_RelativePaths(t *testing.T) {
	testData := []byte("test data")

	configReader := &mockConfigReader{
		configs: map[string]*Config{
			"/base/configs/movie.yaml": {
				Name:      "movie.mkv",
				DedupFile: "../data/movie.dedup", // Relative path
				SourceDir: "../source",          // Relative path
			},
		},
	}

	readerFactory := &mockReaderFactory{
		readers: map[string]*mockReader{
			"/base/data/movie.dedup": {
				data:         testData,
				originalSize: int64(len(testData)),
			},
		},
	}

	root, err := NewMKVFSWithFactories(
		[]string{"/base/configs/movie.yaml"},
		false,
		readerFactory,
		configReader,
	)
	if err != nil {
		t.Fatalf("NewMKVFSWithFactories failed: %v", err)
	}

	file := root.files["movie.mkv"]
	if file.DedupPath != "/base/data/movie.dedup" {
		t.Errorf("expected resolved dedup path /base/data/movie.dedup, got %s", file.DedupPath)
	}
	if file.SourceDir != "/base/source" {
		t.Errorf("expected resolved source dir /base/source, got %s", file.SourceDir)
	}
}

func TestNewMKVFSWithFactories_AbsolutePaths(t *testing.T) {
	testData := []byte("test data")

	configReader := &mockConfigReader{
		configs: map[string]*Config{
			"/configs/movie.yaml": {
				Name:      "movie.mkv",
				DedupFile: "/absolute/path/movie.dedup", // Absolute path
				SourceDir: "/absolute/source",          // Absolute path
			},
		},
	}

	readerFactory := &mockReaderFactory{
		readers: map[string]*mockReader{
			"/absolute/path/movie.dedup": {
				data:         testData,
				originalSize: int64(len(testData)),
			},
		},
	}

	root, err := NewMKVFSWithFactories(
		[]string{"/configs/movie.yaml"},
		false,
		readerFactory,
		configReader,
	)
	if err != nil {
		t.Fatalf("NewMKVFSWithFactories failed: %v", err)
	}

	file := root.files["movie.mkv"]
	if file.DedupPath != "/absolute/path/movie.dedup" {
		t.Errorf("expected absolute dedup path, got %s", file.DedupPath)
	}
	if file.SourceDir != "/absolute/source" {
		t.Errorf("expected absolute source dir, got %s", file.SourceDir)
	}
}

func TestNewMKVFSWithFactories_MultipleConfigs(t *testing.T) {
	testData1 := []byte("movie 1 data")
	testData2 := []byte("movie 2 data longer")

	configReader := &mockConfigReader{
		configs: map[string]*Config{
			"/configs/movie1.yaml": {
				Name:      "movie1.mkv",
				DedupFile: "/data/movie1.dedup",
				SourceDir: "/data/source1",
			},
			"/configs/movie2.yaml": {
				Name:      "movie2.mkv",
				DedupFile: "/data/movie2.dedup",
				SourceDir: "/data/source2",
			},
		},
	}

	readerFactory := &mockReaderFactory{
		readers: map[string]*mockReader{
			"/data/movie1.dedup": {
				data:         testData1,
				originalSize: int64(len(testData1)),
			},
			"/data/movie2.dedup": {
				data:         testData2,
				originalSize: int64(len(testData2)),
			},
		},
	}

	root, err := NewMKVFSWithFactories(
		[]string{"/configs/movie1.yaml", "/configs/movie2.yaml"},
		false,
		readerFactory,
		configReader,
	)
	if err != nil {
		t.Fatalf("NewMKVFSWithFactories failed: %v", err)
	}

	if len(root.files) != 2 {
		t.Errorf("expected 2 files, got %d", len(root.files))
	}

	if root.files["movie1.mkv"].Size != int64(len(testData1)) {
		t.Errorf("movie1 size mismatch")
	}
	if root.files["movie2.mkv"].Size != int64(len(testData2)) {
		t.Errorf("movie2 size mismatch")
	}
}

func TestEnsureReader_AlreadyInitialized(t *testing.T) {
	existingReader := &mockReader{
		data:         []byte("existing"),
		originalSize: 8,
	}

	file := &MKVFile{
		Name:   "test.mkv",
		reader: existingReader,
	}
	node := &MKVFSNode{file: file}

	err := node.ensureReader()
	if err != nil {
		t.Fatalf("ensureReader failed: %v", err)
	}

	// Should still be the same reader
	if file.reader != existingReader {
		t.Error("reader should not have been replaced")
	}
}
