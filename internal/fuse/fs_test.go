package fuse

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/stuckj/mkvdup/internal/dedup"
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
		nil,
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
		nil,
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
		nil,
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

	ctx := ContextWithCaller(context.Background(), 0, 0) // root
	stream, errno := root.Readdir(ctx)
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

	ctx := ContextWithCaller(context.Background(), 0, 0) // root
	var out fuse.EntryOut
	_, errno := root.Lookup(ctx, "nonexistent.mkv", &out)
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
	ctx := ContextWithCaller(context.Background(), 0, 0) // root
	buf := make([]byte, len(testData))
	result, errno := node.Read(ctx, nil, buf, 0)
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
	ctx := ContextWithCaller(context.Background(), 0, 0) // root
	buf := make([]byte, 5)
	result, errno := node.Read(ctx, nil, buf, 7)
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
	ctx := ContextWithCaller(context.Background(), 0, 0) // root
	buf := make([]byte, 10)
	result, errno := node.Read(ctx, nil, buf, 100)
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

	ctx := ContextWithCaller(context.Background(), 0, 0) // root
	buf := make([]byte, 10)
	_, errno := node.Read(ctx, nil, buf, 0)
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

	ctx := ContextWithCaller(context.Background(), 0, 0) // root
	_, _, errno := node.Open(ctx, 0)
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

	ctx := ContextWithCaller(context.Background(), 0, 0) // root
	_, _, errno := node.Open(ctx, 0)
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
				SourceDir: "../source",           // Relative path
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
		nil,
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
				SourceDir: "/absolute/source",           // Absolute path
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
		nil,
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
		nil,
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

// --- Directory Tree Tests ---

func TestBuildDirectoryTree(t *testing.T) {
	files := []*MKVFile{
		{Name: "Movies/Action/Video1.mkv", Size: 100},
		{Name: "Movies/Action/Video2.mkv", Size: 200},
		{Name: "Movies/Comedy/Video3.mkv", Size: 150},
		{Name: "root.mkv", Size: 50},
	}

	tree := BuildDirectoryTree(files, false, nil, nil)

	// Check root level
	if len(tree.files) != 1 {
		t.Errorf("expected 1 file at root, got %d", len(tree.files))
	}
	if _, ok := tree.files["root.mkv"]; !ok {
		t.Error("expected root.mkv at root level")
	}

	if len(tree.subdirs) != 1 {
		t.Errorf("expected 1 subdir at root, got %d", len(tree.subdirs))
	}

	// Check Movies directory
	movies, ok := tree.subdirs["Movies"]
	if !ok {
		t.Fatal("expected Movies subdirectory")
	}
	if movies.name != "Movies" {
		t.Errorf("expected name 'Movies', got '%s'", movies.name)
	}
	if movies.path != "Movies" {
		t.Errorf("expected path 'Movies', got '%s'", movies.path)
	}
	if len(movies.subdirs) != 2 {
		t.Errorf("expected 2 subdirs in Movies, got %d", len(movies.subdirs))
	}

	// Check Action directory
	action, ok := movies.subdirs["Action"]
	if !ok {
		t.Fatal("expected Action subdirectory")
	}
	if action.path != "Movies/Action" {
		t.Errorf("expected path 'Movies/Action', got '%s'", action.path)
	}
	if len(action.files) != 2 {
		t.Errorf("expected 2 files in Action, got %d", len(action.files))
	}

	// Check Comedy directory
	comedy, ok := movies.subdirs["Comedy"]
	if !ok {
		t.Fatal("expected Comedy subdirectory")
	}
	if len(comedy.files) != 1 {
		t.Errorf("expected 1 file in Comedy, got %d", len(comedy.files))
	}
}

func TestBuildDirectoryTree_RootFiles(t *testing.T) {
	files := []*MKVFile{
		{Name: "movie1.mkv", Size: 100},
		{Name: "movie2.mkv", Size: 200},
		{Name: "movie3.mkv", Size: 300},
	}

	tree := BuildDirectoryTree(files, false, nil, nil)

	if len(tree.files) != 3 {
		t.Errorf("expected 3 files at root, got %d", len(tree.files))
	}
	if len(tree.subdirs) != 0 {
		t.Errorf("expected 0 subdirs at root, got %d", len(tree.subdirs))
	}
}

func TestBuildDirectoryTree_DeepNesting(t *testing.T) {
	files := []*MKVFile{
		{Name: "a/b/c/d/e/f/deep.mkv", Size: 100},
	}

	tree := BuildDirectoryTree(files, false, nil, nil)

	// Navigate to the deepest level
	current := tree
	expectedPath := []string{"a", "b", "c", "d", "e", "f"}
	for i, dir := range expectedPath {
		if len(current.subdirs) != 1 {
			t.Errorf("expected 1 subdir at level %d, got %d", i, len(current.subdirs))
		}
		next, ok := current.subdirs[dir]
		if !ok {
			t.Fatalf("expected subdir '%s' at level %d", dir, i)
		}
		current = next
	}

	// Check the file is at the deepest level
	if len(current.files) != 1 {
		t.Errorf("expected 1 file at deepest level, got %d", len(current.files))
	}
	if _, ok := current.files["deep.mkv"]; !ok {
		t.Error("expected deep.mkv at deepest level")
	}
}

func TestBuildDirectoryTree_LargeScale(t *testing.T) {
	// Create 10,000 files across various directories
	files := make([]*MKVFile, 10000)
	for i := range files {
		category := i % 10
		subcategory := (i / 10) % 100
		fileNum := i
		name := "Category" + string(rune('A'+category)) + "/Sub" + strconv.Itoa(subcategory) + "/movie" + strconv.Itoa(fileNum) + ".mkv"
		files[i] = &MKVFile{
			Name: name,
			Size: int64(i * 100),
		}
	}

	tree := BuildDirectoryTree(files, false, nil, nil)

	// Verify structure - should have 10 category directories (A-J)
	if len(tree.subdirs) != 10 {
		t.Errorf("expected 10 category subdirs, got %d", len(tree.subdirs))
	}

	// Count total files
	totalFiles := countFiles(tree)
	if totalFiles != 10000 {
		t.Errorf("expected 10000 total files, got %d", totalFiles)
	}
}

func countFiles(dir *MKVFSDirNode) int {
	count := len(dir.files)
	for _, subdir := range dir.subdirs {
		count += countFiles(subdir)
	}
	return count
}

func TestMKVFSDirNode_Readdir(t *testing.T) {
	dir := &MKVFSDirNode{
		name: "test",
		path: "test",
		files: map[string]*MKVFile{
			"file1.mkv": {Name: "test/file1.mkv", Size: 100},
			"file2.mkv": {Name: "test/file2.mkv", Size: 200},
		},
		subdirs: map[string]*MKVFSDirNode{
			"subdir1": {name: "subdir1", path: "test/subdir1"},
			"subdir2": {name: "subdir2", path: "test/subdir2"},
		},
	}

	ctx := ContextWithCaller(context.Background(), 0, 0) // root
	stream, errno := dir.Readdir(ctx)
	if errno != 0 {
		t.Fatalf("Readdir returned error: %v", errno)
	}

	// Collect all entries
	var entries []fuse.DirEntry
	for stream.HasNext() {
		entry, _ := stream.Next()
		entries = append(entries, entry)
	}

	if len(entries) != 4 {
		t.Errorf("expected 4 entries, got %d", len(entries))
	}

	// Check we have 2 directories and 2 files
	dirs := 0
	regularFiles := 0
	for _, e := range entries {
		if e.Mode&fuse.S_IFDIR != 0 {
			dirs++
		} else {
			regularFiles++
		}
	}

	if dirs != 2 {
		t.Errorf("expected 2 directories, got %d", dirs)
	}
	if regularFiles != 2 {
		t.Errorf("expected 2 files, got %d", regularFiles)
	}
}

func TestMKVFSDirNode_Getattr(t *testing.T) {
	dir := &MKVFSDirNode{
		name: "test",
		path: "test",
		subdirs: map[string]*MKVFSDirNode{
			"sub1": {},
			"sub2": {},
			"sub3": {},
		},
	}

	var out fuse.AttrOut
	errno := dir.Getattr(context.Background(), nil, &out)
	if errno != 0 {
		t.Fatalf("Getattr returned error: %v", errno)
	}

	// Check mode is directory with 0555 permissions
	expectedMode := uint32(fuse.S_IFDIR | 0555)
	if out.Mode != expectedMode {
		t.Errorf("expected mode %o, got %o", expectedMode, out.Mode)
	}

	// nlink should be 2 + number of subdirs
	expectedNlink := uint32(2 + 3)
	if out.Nlink != expectedNlink {
		t.Errorf("expected nlink %d, got %d", expectedNlink, out.Nlink)
	}
}

func TestMKVFSDirNode_Mkdir_EROFS(t *testing.T) {
	dir := &MKVFSDirNode{name: "test", path: "test"}

	var out fuse.EntryOut
	_, errno := dir.Mkdir(context.Background(), "newdir", 0755, &out)
	if errno != syscall.EROFS {
		t.Errorf("expected EROFS, got %v", errno)
	}
}

func TestMKVFSDirNode_Rmdir_EROFS(t *testing.T) {
	dir := &MKVFSDirNode{name: "test", path: "test"}

	errno := dir.Rmdir(context.Background(), "somedir")
	if errno != syscall.EROFS {
		t.Errorf("expected EROFS, got %v", errno)
	}
}

func TestMKVFSDirNode_Unlink_EROFS(t *testing.T) {
	dir := &MKVFSDirNode{name: "test", path: "test"}

	errno := dir.Unlink(context.Background(), "somefile")
	if errno != syscall.EROFS {
		t.Errorf("expected EROFS, got %v", errno)
	}
}

func TestMKVFSDirNode_Create_EROFS(t *testing.T) {
	dir := &MKVFSDirNode{name: "test", path: "test"}

	var out fuse.EntryOut
	_, _, _, errno := dir.Create(context.Background(), "newfile", 0, 0644, &out)
	if errno != syscall.EROFS {
		t.Errorf("expected EROFS, got %v", errno)
	}
}

func TestMKVFSWithDirectories(t *testing.T) {
	// Test that files with paths are organized into directories
	configReader := &mockConfigReader{
		configs: map[string]*Config{
			"/configs/action.yaml": {
				Name:      "Movies/Action/Video1.mkv",
				DedupFile: "/data/video1.dedup",
				SourceDir: "/data/source",
			},
			"/configs/comedy.yaml": {
				Name:      "Movies/Comedy/Video2.mkv",
				DedupFile: "/data/video2.dedup",
				SourceDir: "/data/source",
			},
			"/configs/root.yaml": {
				Name:      "standalone.mkv",
				DedupFile: "/data/standalone.dedup",
				SourceDir: "/data/source",
			},
		},
	}

	readerFactory := &mockReaderFactory{
		readers: map[string]*mockReader{
			"/data/video1.dedup":     {data: []byte("video1"), originalSize: 6},
			"/data/video2.dedup":     {data: []byte("video2"), originalSize: 6},
			"/data/standalone.dedup": {data: []byte("standalone"), originalSize: 10},
		},
	}

	root, err := NewMKVFSWithFactories(
		[]string{"/configs/action.yaml", "/configs/comedy.yaml", "/configs/root.yaml"},
		false,
		readerFactory,
		configReader,
		nil,
	)
	if err != nil {
		t.Fatalf("NewMKVFSWithFactories failed: %v", err)
	}

	// Check the directory tree structure
	if root.rootDir == nil {
		t.Fatal("rootDir should not be nil")
	}

	// Root should have 1 file and 1 directory
	if len(root.rootDir.files) != 1 {
		t.Errorf("expected 1 file at root, got %d", len(root.rootDir.files))
	}
	if len(root.rootDir.subdirs) != 1 {
		t.Errorf("expected 1 subdir at root, got %d", len(root.rootDir.subdirs))
	}

	// Check Movies directory
	movies, ok := root.rootDir.subdirs["Movies"]
	if !ok {
		t.Fatal("expected Movies subdirectory")
	}
	if len(movies.subdirs) != 2 {
		t.Errorf("expected 2 subdirs in Movies (Action, Comedy), got %d", len(movies.subdirs))
	}

	// Check Action has Video1
	action, ok := movies.subdirs["Action"]
	if !ok {
		t.Fatal("expected Action subdirectory")
	}
	if _, ok := action.files["Video1.mkv"]; !ok {
		t.Error("expected Video1.mkv in Action directory")
	}

	// Check Comedy has Video2
	comedy, ok := movies.subdirs["Comedy"]
	if !ok {
		t.Fatal("expected Comedy subdirectory")
	}
	if _, ok := comedy.files["Video2.mkv"]; !ok {
		t.Error("expected Video2.mkv in Comedy directory")
	}
}

// --- Edge Case Tests for Directory Tree Building ---

func TestBuildDirectoryTree_EmptyName(t *testing.T) {
	// Empty names should be skipped
	files := []*MKVFile{
		{Name: "", Size: 100},
		{Name: "valid.mkv", Size: 200},
	}

	tree := BuildDirectoryTree(files, false, nil, nil)

	// Only valid.mkv should be in the tree
	if len(tree.files) != 1 {
		t.Errorf("expected 1 file, got %d", len(tree.files))
	}
	if _, ok := tree.files["valid.mkv"]; !ok {
		t.Error("expected valid.mkv in tree")
	}
}

func TestBuildDirectoryTree_AbsolutePath(t *testing.T) {
	// Leading slashes should be stripped (absolute paths become relative)
	files := []*MKVFile{
		{Name: "/Movies/test.mkv", Size: 100},
	}

	tree := BuildDirectoryTree(files, false, nil, nil)

	// Should create Movies directory (leading slash stripped)
	if len(tree.subdirs) != 1 {
		t.Errorf("expected 1 subdir, got %d", len(tree.subdirs))
	}
	movies, ok := tree.subdirs["Movies"]
	if !ok {
		t.Fatal("expected Movies subdirectory")
	}
	if _, ok := movies.files["test.mkv"]; !ok {
		t.Error("expected test.mkv in Movies directory")
	}
}

func TestBuildDirectoryTree_DotDotPath(t *testing.T) {
	// Paths with ".." components should be rejected (security)
	files := []*MKVFile{
		{Name: "Movies/../etc/passwd", Size: 100},
		{Name: "valid.mkv", Size: 200},
	}

	tree := BuildDirectoryTree(files, false, nil, nil)

	// Only valid.mkv should be in the tree (malicious path rejected)
	if len(tree.files) != 1 {
		t.Errorf("expected 1 file, got %d", len(tree.files))
	}
	if _, ok := tree.files["valid.mkv"]; !ok {
		t.Error("expected valid.mkv in tree")
	}
	if len(tree.subdirs) != 0 {
		t.Errorf("expected 0 subdirs, got %d", len(tree.subdirs))
	}
}

func TestBuildDirectoryTree_DuplicatePaths(t *testing.T) {
	// Duplicate paths: later file wins
	files := []*MKVFile{
		{Name: "movie.mkv", DedupPath: "/first.dedup", Size: 100},
		{Name: "movie.mkv", DedupPath: "/second.dedup", Size: 200},
	}

	tree := BuildDirectoryTree(files, false, nil, nil)

	// Should have one file with the second dedup path
	if len(tree.files) != 1 {
		t.Errorf("expected 1 file, got %d", len(tree.files))
	}
	file, ok := tree.files["movie.mkv"]
	if !ok {
		t.Fatal("expected movie.mkv in tree")
	}
	if file.DedupPath != "/second.dedup" {
		t.Errorf("expected second.dedup, got %s", file.DedupPath)
	}
	if file.Size != 200 {
		t.Errorf("expected size 200, got %d", file.Size)
	}
}

func TestBuildDirectoryTree_FileDirectoryCollision_DirWins(t *testing.T) {
	// When a directory exists with the same name as a file, directory wins
	// Create directory first via a file in it, then try to add file with same name
	files := []*MKVFile{
		{Name: "Movies/Action/test.mkv", Size: 100}, // Creates Movies directory
		{Name: "Movies", Size: 200},                 // Tries to create file named "Movies"
	}

	tree := BuildDirectoryTree(files, false, nil, nil)

	// Movies should be a directory, not a file
	if len(tree.subdirs) != 1 {
		t.Errorf("expected 1 subdir, got %d", len(tree.subdirs))
	}
	if _, ok := tree.subdirs["Movies"]; !ok {
		t.Fatal("expected Movies to be a directory")
	}
	if _, ok := tree.files["Movies"]; ok {
		t.Error("Movies should not exist as a file")
	}
}

func TestBuildDirectoryTree_PathComponentCollision(t *testing.T) {
	// When trying to use a file name as a path component, the file is skipped
	files := []*MKVFile{
		{Name: "Movies.mkv", Size: 100},                 // Creates file "Movies.mkv"
		{Name: "Movies.mkv/Action/test.mkv", Size: 200}, // Tries to use "Movies.mkv" as directory
	}

	tree := BuildDirectoryTree(files, false, nil, nil)

	// Only Movies.mkv file should exist
	if len(tree.files) != 1 {
		t.Errorf("expected 1 file, got %d", len(tree.files))
	}
	if _, ok := tree.files["Movies.mkv"]; !ok {
		t.Error("expected Movies.mkv as file")
	}
	// No directories should be created
	if len(tree.subdirs) != 0 {
		t.Errorf("expected 0 subdirs, got %d", len(tree.subdirs))
	}
}

func TestBuildDirectoryTree_MultipleSlashes(t *testing.T) {
	// Multiple consecutive slashes should be normalized
	files := []*MKVFile{
		{Name: "Movies//Action///test.mkv", Size: 100},
	}

	tree := BuildDirectoryTree(files, false, nil, nil)

	// Should create proper hierarchy despite multiple slashes
	movies, ok := tree.subdirs["Movies"]
	if !ok {
		t.Fatal("expected Movies subdirectory")
	}
	action, ok := movies.subdirs["Action"]
	if !ok {
		t.Fatal("expected Action subdirectory")
	}
	if _, ok := action.files["test.mkv"]; !ok {
		t.Error("expected test.mkv in Action directory")
	}
}

func TestBuildDirectoryTree_DotPath(t *testing.T) {
	// Current directory (.) components should be filtered
	files := []*MKVFile{
		{Name: "./Movies/./test.mkv", Size: 100},
	}

	tree := BuildDirectoryTree(files, false, nil, nil)

	// Should create Movies directory (. components stripped)
	movies, ok := tree.subdirs["Movies"]
	if !ok {
		t.Fatal("expected Movies subdirectory")
	}
	if _, ok := movies.files["test.mkv"]; !ok {
		t.Error("expected test.mkv in Movies directory")
	}
}

func TestBuildDirectoryTree_TrailingSlash(t *testing.T) {
	// Trailing slashes are stripped by path.Clean, so "Movies/" becomes "Movies"
	files := []*MKVFile{
		{Name: "Movies/", Size: 100},
		{Name: "valid.mkv", Size: 200},
	}

	tree := BuildDirectoryTree(files, false, nil, nil)

	// Both files should be in the tree ("Movies/" becomes "Movies")
	if len(tree.files) != 2 {
		t.Errorf("expected 2 files, got %d", len(tree.files))
	}
	if _, ok := tree.files["valid.mkv"]; !ok {
		t.Error("expected valid.mkv in tree")
	}
	if _, ok := tree.files["Movies"]; !ok {
		t.Error("expected Movies in tree (from 'Movies/')")
	}
}

func TestMKVFSDirNode_Readdir_Sorted(t *testing.T) {
	// Verify that Readdir returns entries in sorted order
	dir := &MKVFSDirNode{
		name: "test",
		path: "test",
		files: map[string]*MKVFile{
			"zebra.mkv": {Name: "test/zebra.mkv", Size: 100},
			"apple.mkv": {Name: "test/apple.mkv", Size: 200},
			"mango.mkv": {Name: "test/mango.mkv", Size: 150},
		},
		subdirs: map[string]*MKVFSDirNode{
			"zoo":    {name: "zoo", path: "test/zoo"},
			"alpha":  {name: "alpha", path: "test/alpha"},
			"middle": {name: "middle", path: "test/middle"},
		},
	}

	ctx := ContextWithCaller(context.Background(), 0, 0) // root
	stream, errno := dir.Readdir(ctx)
	if errno != 0 {
		t.Fatalf("Readdir returned error: %v", errno)
	}

	// Collect all entries in order
	var entries []string
	for stream.HasNext() {
		entry, _ := stream.Next()
		entries = append(entries, entry.Name)
	}

	// Directories should come first (sorted), then files (sorted)
	expected := []string{"alpha", "middle", "zoo", "apple.mkv", "mango.mkv", "zebra.mkv"}
	if len(entries) != len(expected) {
		t.Fatalf("expected %d entries, got %d", len(expected), len(entries))
	}
	for i, name := range expected {
		if entries[i] != name {
			t.Errorf("entry %d: expected %s, got %s", i, name, entries[i])
		}
	}
}

// --- Permission Store Integration Tests ---

func TestMKVFSNode_Getattr_WithPermStore(t *testing.T) {
	// Create permission store with custom defaults
	defaults := Defaults{
		FileUID:  1000,
		FileGID:  1001,
		FileMode: 0640,
		DirUID:   1000,
		DirGID:   1001,
		DirMode:  0750,
	}
	store := NewPermissionStore("", defaults, false)

	// Set custom permissions for a specific file
	uid := uint32(2000)
	mode := uint32(0600)
	_ = store.SetFilePerms("test/video.mkv", &uid, nil, &mode)

	file := &MKVFile{
		Name: "video.mkv",
		Size: 54321,
	}
	node := &MKVFSNode{file: file, path: "test/video.mkv", permStore: store}

	var out fuse.AttrOut
	errno := node.Getattr(context.Background(), nil, &out)
	if errno != 0 {
		t.Fatalf("Getattr returned errno %d", errno)
	}

	// Should use custom UID from store
	if out.Uid != 2000 {
		t.Errorf("expected UID 2000, got %d", out.Uid)
	}
	// GID should fall back to default (not overridden)
	if out.Gid != 1001 {
		t.Errorf("expected GID 1001 (default), got %d", out.Gid)
	}
	// Mode should use custom mode
	expectedMode := uint32(fuse.S_IFREG | 0600)
	if out.Mode != expectedMode {
		t.Errorf("expected mode %o, got %o", expectedMode, out.Mode)
	}
}

func TestMKVFSNode_Getattr_WithPermStore_Defaults(t *testing.T) {
	// Create permission store with custom defaults
	defaults := Defaults{
		FileUID:  1000,
		FileGID:  1001,
		FileMode: 0640,
		DirUID:   1000,
		DirGID:   1001,
		DirMode:  0750,
	}
	store := NewPermissionStore("", defaults, false)

	file := &MKVFile{
		Name: "other.mkv",
		Size: 12345,
	}
	// File with no custom permissions set
	node := &MKVFSNode{file: file, path: "other.mkv", permStore: store}

	var out fuse.AttrOut
	errno := node.Getattr(context.Background(), nil, &out)
	if errno != 0 {
		t.Fatalf("Getattr returned errno %d", errno)
	}

	// Should use defaults
	if out.Uid != 1000 {
		t.Errorf("expected UID 1000, got %d", out.Uid)
	}
	if out.Gid != 1001 {
		t.Errorf("expected GID 1001, got %d", out.Gid)
	}
	expectedMode := uint32(fuse.S_IFREG | 0640)
	if out.Mode != expectedMode {
		t.Errorf("expected mode %o, got %o", expectedMode, out.Mode)
	}
}

func TestMKVFSDirNode_Getattr_WithPermStore(t *testing.T) {
	defaults := Defaults{
		FileUID:  1000,
		FileGID:  1001,
		FileMode: 0640,
		DirUID:   1000,
		DirGID:   1001,
		DirMode:  0750,
	}
	store := NewPermissionStore("", defaults, false)

	// Set custom permissions for directory
	mode := uint32(0755)
	_ = store.SetDirPerms("Movies/Action", nil, nil, &mode)

	dir := &MKVFSDirNode{
		name:      "Action",
		path:      "Movies/Action",
		subdirs:   make(map[string]*MKVFSDirNode),
		files:     make(map[string]*MKVFile),
		permStore: store,
	}

	var out fuse.AttrOut
	errno := dir.Getattr(context.Background(), nil, &out)
	if errno != 0 {
		t.Fatalf("Getattr returned errno %d", errno)
	}

	// Should use defaults for UID/GID, custom for mode
	if out.Uid != 1000 {
		t.Errorf("expected UID 1000, got %d", out.Uid)
	}
	if out.Gid != 1001 {
		t.Errorf("expected GID 1001, got %d", out.Gid)
	}
	expectedMode := uint32(fuse.S_IFDIR | 0755)
	if out.Mode != expectedMode {
		t.Errorf("expected mode %o, got %o", expectedMode, out.Mode)
	}
}

func TestMKVFSNode_Setattr_NoPermStore(t *testing.T) {
	file := &MKVFile{Name: "test.mkv", Size: 100}
	node := &MKVFSNode{file: file, path: "test.mkv", permStore: nil}

	in := &fuse.SetAttrIn{}
	in.Valid = fuse.FATTR_MODE
	in.Mode = 0644

	ctx := ContextWithCaller(context.Background(), 0, 0) // root
	var out fuse.AttrOut
	errno := node.Setattr(ctx, nil, in, &out)

	// Should return EROFS when no permission store
	if errno != syscall.EROFS {
		t.Errorf("expected EROFS, got %d", errno)
	}
}

func TestMKVFSNode_Setattr_Chmod(t *testing.T) {
	store := NewPermissionStore("", DefaultPerms(), false)

	file := &MKVFile{Name: "test.mkv", Size: 100}
	node := &MKVFSNode{file: file, path: "test.mkv", permStore: store}

	in := &fuse.SetAttrIn{}
	in.Valid = fuse.FATTR_MODE
	in.Mode = 0644

	ctx := ContextWithCaller(context.Background(), 0, 0) // root
	var out fuse.AttrOut
	errno := node.Setattr(ctx, nil, in, &out)
	if errno != 0 {
		t.Fatalf("Setattr returned errno %d", errno)
	}

	// Verify mode was updated
	expectedMode := uint32(fuse.S_IFREG | 0644)
	if out.Mode != expectedMode {
		t.Errorf("expected mode %o, got %o", expectedMode, out.Mode)
	}

	// Verify it persisted in the store
	_, _, mode := store.GetFilePerms("test.mkv")
	if mode != 0644 {
		t.Errorf("expected stored mode 0644, got %o", mode)
	}
}

func TestMKVFSNode_Setattr_Chown(t *testing.T) {
	store := NewPermissionStore("", DefaultPerms(), false)

	file := &MKVFile{Name: "test.mkv", Size: 100}
	node := &MKVFSNode{file: file, path: "test.mkv", permStore: store}

	in := &fuse.SetAttrIn{}
	in.Valid = fuse.FATTR_UID | fuse.FATTR_GID
	in.Uid = 1000
	in.Gid = 1001

	ctx := ContextWithCaller(context.Background(), 0, 0) // root
	var out fuse.AttrOut
	errno := node.Setattr(ctx, nil, in, &out)
	if errno != 0 {
		t.Fatalf("Setattr returned errno %d", errno)
	}

	// Verify uid/gid were updated
	if out.Uid != 1000 {
		t.Errorf("expected UID 1000, got %d", out.Uid)
	}
	if out.Gid != 1001 {
		t.Errorf("expected GID 1001, got %d", out.Gid)
	}

	// Verify it persisted in the store
	uid, gid, _ := store.GetFilePerms("test.mkv")
	if uid != 1000 {
		t.Errorf("expected stored UID 1000, got %d", uid)
	}
	if gid != 1001 {
		t.Errorf("expected stored GID 1001, got %d", gid)
	}
}

func TestMKVFSDirNode_Setattr_NoPermStore(t *testing.T) {
	dir := &MKVFSDirNode{
		name:      "test",
		path:      "test",
		subdirs:   make(map[string]*MKVFSDirNode),
		files:     make(map[string]*MKVFile),
		permStore: nil,
	}

	in := &fuse.SetAttrIn{}
	in.Valid = fuse.FATTR_MODE
	in.Mode = 0755

	ctx := ContextWithCaller(context.Background(), 0, 0) // root
	var out fuse.AttrOut
	errno := dir.Setattr(ctx, nil, in, &out)

	// Should return EROFS when no permission store
	if errno != syscall.EROFS {
		t.Errorf("expected EROFS, got %d", errno)
	}
}

func TestMKVFSDirNode_Setattr_Chmod(t *testing.T) {
	store := NewPermissionStore("", DefaultPerms(), false)

	dir := &MKVFSDirNode{
		name:      "test",
		path:      "Movies/Action",
		subdirs:   make(map[string]*MKVFSDirNode),
		files:     make(map[string]*MKVFile),
		permStore: store,
	}

	in := &fuse.SetAttrIn{}
	in.Valid = fuse.FATTR_MODE
	in.Mode = 0755

	ctx := ContextWithCaller(context.Background(), 0, 0) // root
	var out fuse.AttrOut
	errno := dir.Setattr(ctx, nil, in, &out)
	if errno != 0 {
		t.Fatalf("Setattr returned errno %d", errno)
	}

	// Verify mode was updated
	expectedMode := uint32(fuse.S_IFDIR | 0755)
	if out.Mode != expectedMode {
		t.Errorf("expected mode %o, got %o", expectedMode, out.Mode)
	}

	// Verify it persisted in the store
	_, _, mode := store.GetDirPerms("Movies/Action")
	if mode != 0755 {
		t.Errorf("expected stored mode 0755, got %o", mode)
	}
}

func TestMKVFSDirNode_Setattr_Chown(t *testing.T) {
	store := NewPermissionStore("", DefaultPerms(), false)

	dir := &MKVFSDirNode{
		name:      "Action",
		path:      "Movies/Action",
		subdirs:   make(map[string]*MKVFSDirNode),
		files:     make(map[string]*MKVFile),
		permStore: store,
	}

	in := &fuse.SetAttrIn{}
	in.Valid = fuse.FATTR_UID | fuse.FATTR_GID
	in.Uid = 1000
	in.Gid = 1001

	ctx := ContextWithCaller(context.Background(), 0, 0) // root
	var out fuse.AttrOut
	errno := dir.Setattr(ctx, nil, in, &out)
	if errno != 0 {
		t.Fatalf("Setattr returned errno %d", errno)
	}

	// Verify UID/GID were updated in the response
	if out.Uid != 1000 {
		t.Errorf("expected UID 1000, got %d", out.Uid)
	}
	if out.Gid != 1001 {
		t.Errorf("expected GID 1001, got %d", out.Gid)
	}

	// Verify it persisted in the store
	uid, gid, _ := store.GetDirPerms("Movies/Action")
	if uid != 1000 {
		t.Errorf("expected stored UID 1000, got %d", uid)
	}
	if gid != 1001 {
		t.Errorf("expected stored GID 1001, got %d", gid)
	}
}

// --- Permission Enforcement Tests ---
// Note: Read/write/execute access checks are now handled by the kernel via the
// default_permissions mount option. These tests cover ownership changes (chown/chmod)
// which are still implemented in our code.

func TestMKVFSNode_Setattr_ChownUIDDenied(t *testing.T) {
	store := NewPermissionStore("", DefaultPerms(), false)

	uid := uint32(1000)
	_ = store.SetFilePerms("test.mkv", &uid, nil, nil)

	node := &MKVFSNode{
		file:      &MKVFile{Name: "test.mkv", Size: 1000},
		path:      "test.mkv",
		permStore: store,
	}

	// Owner (non-root) trying to chown to different user - should get EPERM
	ctx := ContextWithCaller(context.Background(), 1000, 1000)
	in := &fuse.SetAttrIn{}
	in.Valid = fuse.FATTR_UID
	in.Uid = 2000
	var out fuse.AttrOut

	errno := node.Setattr(ctx, nil, in, &out)
	if errno != syscall.EPERM {
		t.Errorf("Setattr() = %v, want EPERM", errno)
	}
}

func TestMKVFSNode_Setattr_RootCanChownUID(t *testing.T) {
	store := NewPermissionStore("", DefaultPerms(), false)

	uid := uint32(1000)
	_ = store.SetFilePerms("test.mkv", &uid, nil, nil)

	node := &MKVFSNode{
		file:      &MKVFile{Name: "test.mkv", Size: 1000},
		path:      "test.mkv",
		permStore: store,
	}

	// Root can chown UID
	ctx := ContextWithCaller(context.Background(), 0, 0)
	in := &fuse.SetAttrIn{}
	in.Valid = fuse.FATTR_UID
	in.Uid = 2000
	var out fuse.AttrOut

	errno := node.Setattr(ctx, nil, in, &out)
	if errno != 0 {
		t.Errorf("Setattr() = %v, want 0", errno)
	}

	// Verify the change
	gotUID, _, _ := store.GetFilePerms("test.mkv")
	if gotUID != 2000 {
		t.Errorf("UID = %d, want 2000", gotUID)
	}
}

func TestMKVFSNode_Setattr_OwnerCanChownGID(t *testing.T) {
	// Mock group membership: uid 1000 is member of groups 1000 (primary), 100
	origFunc := groupMembershipFunc
	groupMembershipFunc = func(uid, primaryGID, targetGID uint32) bool {
		if targetGID == primaryGID {
			return true
		}
		return uid == 1000 && targetGID == 100
	}
	t.Cleanup(func() { groupMembershipFunc = origFunc })

	store := NewPermissionStore("", DefaultPerms(), false)

	uid := uint32(1000)
	gid := uint32(2000) // File starts with GID 2000
	_ = store.SetFilePerms("test.mkv", &uid, &gid, nil)

	node := &MKVFSNode{
		file:      &MKVFile{Name: "test.mkv", Size: 1000},
		path:      "test.mkv",
		permStore: store,
	}

	// Owner can change GID to their primary GID (Unix semantics)
	ctx := ContextWithCaller(context.Background(), 1000, 1000)
	in := &fuse.SetAttrIn{}
	in.Valid = fuse.FATTR_GID
	in.Gid = 1000 // Change to caller's primary GID
	var out fuse.AttrOut

	errno := node.Setattr(ctx, nil, in, &out)
	if errno != 0 {
		t.Errorf("Setattr() = %v, want 0", errno)
	}

	// Verify the change
	_, gotGID, _ := store.GetFilePerms("test.mkv")
	if gotGID != 1000 {
		t.Errorf("GID = %d, want 1000", gotGID)
	}
}

func TestMKVFSNode_Setattr_OwnerCanChownGIDToSupplementary(t *testing.T) {
	// Mock group membership: uid 1000 is member of groups 1000 (primary), 100
	origFunc := groupMembershipFunc
	groupMembershipFunc = func(uid, primaryGID, targetGID uint32) bool {
		if targetGID == primaryGID {
			return true
		}
		return uid == 1000 && targetGID == 100
	}
	t.Cleanup(func() { groupMembershipFunc = origFunc })

	store := NewPermissionStore("", DefaultPerms(), false)

	uid := uint32(1000)
	_ = store.SetFilePerms("test.mkv", &uid, nil, nil)

	node := &MKVFSNode{
		file:      &MKVFile{Name: "test.mkv", Size: 1000},
		path:      "test.mkv",
		permStore: store,
	}

	// Owner can change GID to a supplementary group
	ctx := ContextWithCaller(context.Background(), 1000, 1000)
	in := &fuse.SetAttrIn{}
	in.Valid = fuse.FATTR_GID
	in.Gid = 100 // Supplementary group
	var out fuse.AttrOut

	errno := node.Setattr(ctx, nil, in, &out)
	if errno != 0 {
		t.Errorf("Setattr() = %v, want 0", errno)
	}

	// Verify the change
	_, gotGID, _ := store.GetFilePerms("test.mkv")
	if gotGID != 100 {
		t.Errorf("GID = %d, want 100", gotGID)
	}
}

func TestMKVFSNode_Setattr_OwnerCannotChownGIDToNonMemberGroup(t *testing.T) {
	// Mock group membership: uid 1000 is member of groups 1000 (primary), 100
	origFunc := groupMembershipFunc
	groupMembershipFunc = func(uid, primaryGID, targetGID uint32) bool {
		if targetGID == primaryGID {
			return true
		}
		return uid == 1000 && targetGID == 100
	}
	t.Cleanup(func() { groupMembershipFunc = origFunc })

	store := NewPermissionStore("", DefaultPerms(), false)

	uid := uint32(1000)
	_ = store.SetFilePerms("test.mkv", &uid, nil, nil)

	node := &MKVFSNode{
		file:      &MKVFile{Name: "test.mkv", Size: 1000},
		path:      "test.mkv",
		permStore: store,
	}

	// Owner cannot change GID to a group they don't belong to
	ctx := ContextWithCaller(context.Background(), 1000, 1000)
	in := &fuse.SetAttrIn{}
	in.Valid = fuse.FATTR_GID
	in.Gid = 2000 // Not a member of this group
	var out fuse.AttrOut

	errno := node.Setattr(ctx, nil, in, &out)
	if errno != syscall.EPERM {
		t.Errorf("Setattr() = %v, want EPERM", errno)
	}
}

func TestMKVFSNode_Setattr_NonOwnerCannotChownGID(t *testing.T) {
	// Mock group membership: no supplementary groups for uid 2000
	origFunc := groupMembershipFunc
	groupMembershipFunc = func(uid, primaryGID, targetGID uint32) bool {
		return targetGID == primaryGID
	}
	t.Cleanup(func() { groupMembershipFunc = origFunc })

	store := NewPermissionStore("", DefaultPerms(), false)

	uid := uint32(1000)
	_ = store.SetFilePerms("test.mkv", &uid, nil, nil)

	node := &MKVFSNode{
		file:      &MKVFile{Name: "test.mkv", Size: 1000},
		path:      "test.mkv",
		permStore: store,
	}

	// Non-owner cannot change GID
	ctx := ContextWithCaller(context.Background(), 2000, 2000)
	in := &fuse.SetAttrIn{}
	in.Valid = fuse.FATTR_GID
	in.Gid = 3000
	var out fuse.AttrOut

	errno := node.Setattr(ctx, nil, in, &out)
	if errno != syscall.EPERM {
		t.Errorf("Setattr() = %v, want EPERM", errno)
	}
}

func TestMKVFSNode_Setattr_ChmodDenied(t *testing.T) {
	store := NewPermissionStore("", DefaultPerms(), false)

	uid := uint32(1000)
	_ = store.SetFilePerms("test.mkv", &uid, nil, nil)

	node := &MKVFSNode{
		file:      &MKVFile{Name: "test.mkv", Size: 1000},
		path:      "test.mkv",
		permStore: store,
	}

	// Non-owner trying to chmod - should get EPERM
	ctx := ContextWithCaller(context.Background(), 2000, 2000)
	in := &fuse.SetAttrIn{}
	in.Valid = fuse.FATTR_MODE
	in.Mode = 0777
	var out fuse.AttrOut

	errno := node.Setattr(ctx, nil, in, &out)
	if errno != syscall.EPERM {
		t.Errorf("Setattr() = %v, want EPERM", errno)
	}
}

func TestMKVFSNode_Setattr_OwnerCanChmod(t *testing.T) {
	store := NewPermissionStore("", DefaultPerms(), false)

	uid := uint32(1000)
	_ = store.SetFilePerms("test.mkv", &uid, nil, nil)

	node := &MKVFSNode{
		file:      &MKVFile{Name: "test.mkv", Size: 1000},
		path:      "test.mkv",
		permStore: store,
	}

	// Owner can chmod
	ctx := ContextWithCaller(context.Background(), 1000, 1000)
	in := &fuse.SetAttrIn{}
	in.Valid = fuse.FATTR_MODE
	in.Mode = 0640
	var out fuse.AttrOut

	errno := node.Setattr(ctx, nil, in, &out)
	if errno != 0 {
		t.Errorf("Setattr() = %v, want 0", errno)
	}

	// Verify the change
	_, _, gotMode := store.GetFilePerms("test.mkv")
	if gotMode != 0640 {
		t.Errorf("Mode = %o, want 0640", gotMode)
	}
}

func TestMKVFSDirNode_Setattr_ChownDenied(t *testing.T) {
	store := NewPermissionStore("", DefaultPerms(), false)

	uid := uint32(1000)
	_ = store.SetDirPerms("Movies", &uid, nil, nil)

	dir := &MKVFSDirNode{
		path:      "Movies",
		subdirs:   map[string]*MKVFSDirNode{},
		permStore: store,
	}

	// Non-owner trying to chown - should get EPERM
	ctx := ContextWithCaller(context.Background(), 2000, 2000)
	in := &fuse.SetAttrIn{}
	in.Valid = fuse.FATTR_UID
	in.Uid = 3000
	var out fuse.AttrOut

	errno := dir.Setattr(ctx, nil, in, &out)
	if errno != syscall.EPERM {
		t.Errorf("Setattr() = %v, want EPERM", errno)
	}
}

func TestMKVFSDirNode_Setattr_RootCanChown(t *testing.T) {
	store := NewPermissionStore("", DefaultPerms(), false)

	uid := uint32(1000)
	_ = store.SetDirPerms("Movies", &uid, nil, nil)

	dir := &MKVFSDirNode{
		path:      "Movies",
		subdirs:   map[string]*MKVFSDirNode{},
		permStore: store,
	}

	// Root can chown
	ctx := ContextWithCaller(context.Background(), 0, 0)
	in := &fuse.SetAttrIn{}
	in.Valid = fuse.FATTR_UID | fuse.FATTR_GID
	in.Uid = 2000
	in.Gid = 2000
	var out fuse.AttrOut

	errno := dir.Setattr(ctx, nil, in, &out)
	if errno != 0 {
		t.Errorf("Setattr() = %v, want 0", errno)
	}

	// Verify the change
	gotUID, gotGID, _ := store.GetDirPerms("Movies")
	if gotUID != 2000 {
		t.Errorf("UID = %d, want 2000", gotUID)
	}
	if gotGID != 2000 {
		t.Errorf("GID = %d, want 2000", gotGID)
	}
}

func TestMKVFSDirNode_Setattr_ChmodDenied(t *testing.T) {
	store := NewPermissionStore("", DefaultPerms(), false)

	uid := uint32(1000)
	_ = store.SetDirPerms("Movies", &uid, nil, nil)

	dir := &MKVFSDirNode{
		path:      "Movies",
		permStore: store,
	}

	// Non-owner trying to chmod - should get EPERM
	ctx := ContextWithCaller(context.Background(), 2000, 2000)
	in := &fuse.SetAttrIn{}
	in.Valid = fuse.FATTR_MODE
	in.Mode = 0777
	var out fuse.AttrOut

	errno := dir.Setattr(ctx, nil, in, &out)
	if errno != syscall.EPERM {
		t.Errorf("Setattr() = %v, want EPERM", errno)
	}
}

func TestBuildDirectoryTree_WithPermStore(t *testing.T) {
	// All existing BuildDirectoryTree tests pass nil for permStore.
	// This test verifies that a real PermissionStore is propagated to nodes.
	defaults := Defaults{
		FileUID:  1000,
		FileGID:  1000,
		FileMode: 0644,
		DirUID:   1000,
		DirGID:   1000,
		DirMode:  0755,
	}
	store := NewPermissionStore("", defaults, false)

	files := []*MKVFile{
		{Name: "Movies/Action/video.mkv", Size: 100},
		{Name: "root.mkv", Size: 50},
	}
	tree := BuildDirectoryTree(files, false, nil, store)

	// Verify permStore is set on root directory
	if tree.permStore != store {
		t.Error("root dir permStore not set")
	}

	// Verify permStore is propagated to subdirectories
	movies, ok := tree.subdirs["Movies"]
	if !ok {
		t.Fatal("expected Movies subdirectory")
	}
	if movies.permStore != store {
		t.Error("Movies dir permStore not set")
	}
	action, ok := movies.subdirs["Action"]
	if !ok {
		t.Fatal("expected Action subdirectory")
	}
	if action.permStore != store {
		t.Error("Action dir permStore not set")
	}

	// Verify Getattr on directory returns UID/GID from store defaults
	var out fuse.AttrOut
	errno := movies.Getattr(context.Background(), nil, &out)
	if errno != 0 {
		t.Fatalf("Getattr returned errno %d", errno)
	}
	if out.Uid != 1000 {
		t.Errorf("dir UID = %d, want 1000", out.Uid)
	}
	if out.Gid != 1000 {
		t.Errorf("dir GID = %d, want 1000", out.Gid)
	}
	expectedMode := uint32(fuse.S_IFDIR | 0755)
	if out.Mode != expectedMode {
		t.Errorf("dir mode = %o, want %o", out.Mode, expectedMode)
	}
}

func TestNewMKVFSWithFactories_WithPermStore(t *testing.T) {
	testData := []byte("test MKV data content")

	defaults := Defaults{
		FileUID:  1000,
		FileGID:  1000,
		FileMode: 0644,
		DirUID:   1000,
		DirGID:   1000,
		DirMode:  0755,
	}
	store := NewPermissionStore("", defaults, false)

	configReader := &mockConfigReader{
		configs: map[string]*Config{
			"/configs/movie.yaml": {
				Name:      "Movies/movie.mkv",
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
		store,
	)
	if err != nil {
		t.Fatalf("NewMKVFSWithFactories failed: %v", err)
	}

	// Verify rootDir has permStore set
	if root.rootDir == nil {
		t.Fatal("rootDir should not be nil")
	}
	if root.rootDir.permStore != store {
		t.Error("rootDir permStore not set")
	}

	// Verify subdirectory has permStore
	movies, ok := root.rootDir.subdirs["Movies"]
	if !ok {
		t.Fatal("expected Movies subdirectory")
	}
	if movies.permStore != store {
		t.Error("Movies dir permStore not set")
	}

	// Verify directory Getattr returns UID/GID from store defaults
	var out fuse.AttrOut
	errno := movies.Getattr(context.Background(), nil, &out)
	if errno != 0 {
		t.Fatalf("Getattr returned errno %d", errno)
	}
	if out.Uid != 1000 {
		t.Errorf("dir UID = %d, want 1000", out.Uid)
	}
	if out.Gid != 1000 {
		t.Errorf("dir GID = %d, want 1000", out.Gid)
	}
}

// --- Reload tests ---

func TestMKVFSRoot_Reload_AddFile(t *testing.T) {
	factory := &mockReaderFactory{
		readers: map[string]*mockReader{
			"/data/m1.dedup": {data: []byte("m1"), originalSize: 100},
			"/data/m2.dedup": {data: []byte("m2"), originalSize: 200},
		},
	}

	initial := []dedup.Config{
		{Name: "movie1.mkv", DedupFile: "/data/m1.dedup", SourceDir: "/src"},
	}
	root, err := NewMKVFSFromConfigs(initial, false, factory, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(root.files) != 1 {
		t.Fatalf("expected 1 file initially, got %d", len(root.files))
	}

	// Reload with two files
	updated := []dedup.Config{
		{Name: "movie1.mkv", DedupFile: "/data/m1.dedup", SourceDir: "/src"},
		{Name: "movie2.mkv", DedupFile: "/data/m2.dedup", SourceDir: "/src"},
	}
	if err := root.Reload(updated, nil); err != nil {
		t.Fatal(err)
	}
	if len(root.files) != 2 {
		t.Errorf("expected 2 files after reload, got %d", len(root.files))
	}
	if _, ok := root.rootDir.files["movie2.mkv"]; !ok {
		t.Error("expected movie2.mkv in root dir after reload")
	}
}

func TestMKVFSRoot_Reload_RemoveFile(t *testing.T) {
	factory := &mockReaderFactory{
		readers: map[string]*mockReader{
			"/data/m1.dedup": {data: []byte("m1"), originalSize: 100},
			"/data/m2.dedup": {data: []byte("m2"), originalSize: 200},
		},
	}

	initial := []dedup.Config{
		{Name: "movie1.mkv", DedupFile: "/data/m1.dedup", SourceDir: "/src"},
		{Name: "movie2.mkv", DedupFile: "/data/m2.dedup", SourceDir: "/src"},
	}
	root, err := NewMKVFSFromConfigs(initial, false, factory, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(root.files) != 2 {
		t.Fatalf("expected 2 files initially, got %d", len(root.files))
	}

	// Reload with only one file
	updated := []dedup.Config{
		{Name: "movie1.mkv", DedupFile: "/data/m1.dedup", SourceDir: "/src"},
	}
	if err := root.Reload(updated, nil); err != nil {
		t.Fatal(err)
	}
	if len(root.files) != 1 {
		t.Errorf("expected 1 file after reload, got %d", len(root.files))
	}
	if _, ok := root.rootDir.files["movie2.mkv"]; ok {
		t.Error("movie2.mkv should have been removed")
	}
}

func TestMKVFSRoot_Reload_MappingChangedWithActiveReader(t *testing.T) {
	reader1 := &mockReader{data: []byte("original"), originalSize: 100}
	factory := &mockReaderFactory{
		readers: map[string]*mockReader{
			"/data/m1.dedup":     reader1,
			"/data/m1_new.dedup": {data: []byte("updated"), originalSize: 200},
		},
	}

	initial := []dedup.Config{
		{Name: "movie.mkv", DedupFile: "/data/m1.dedup", SourceDir: "/src"},
	}
	root, err := NewMKVFSFromConfigs(initial, false, factory, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Simulate an active reader on the file
	oldFile := root.files["movie.mkv"]
	oldFile.mu.Lock()
	oldFile.reader = reader1
	oldFile.mu.Unlock()

	// Reload with changed mapping
	updated := []dedup.Config{
		{Name: "movie.mkv", DedupFile: "/data/m1_new.dedup", SourceDir: "/src2"},
	}
	if err := root.Reload(updated, nil); err != nil {
		t.Fatal(err)
	}

	// Both tree and flat map should have the new mapping
	treeFile := root.rootDir.files["movie.mkv"]
	if treeFile.DedupPath != "/data/m1_new.dedup" {
		t.Errorf("expected tree file to have new dedup path, got %s", treeFile.DedupPath)
	}
	if root.files["movie.mkv"].DedupPath != "/data/m1_new.dedup" {
		t.Error("expected flat files map to have new mapping")
	}
}

func TestMKVFSRoot_Reload_NewDirectory(t *testing.T) {
	factory := &mockReaderFactory{
		readers: map[string]*mockReader{
			"/data/m1.dedup": {data: []byte("m1"), originalSize: 100},
			"/data/m2.dedup": {data: []byte("m2"), originalSize: 200},
		},
	}

	initial := []dedup.Config{
		{Name: "movie1.mkv", DedupFile: "/data/m1.dedup", SourceDir: "/src"},
	}
	root, err := NewMKVFSFromConfigs(initial, false, factory, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := root.rootDir.subdirs["Movies"]; ok {
		t.Fatal("Movies dir should not exist initially")
	}

	// Reload with file in subdirectory
	updated := []dedup.Config{
		{Name: "movie1.mkv", DedupFile: "/data/m1.dedup", SourceDir: "/src"},
		{Name: "Movies/movie2.mkv", DedupFile: "/data/m2.dedup", SourceDir: "/src"},
	}
	if err := root.Reload(updated, nil); err != nil {
		t.Fatal(err)
	}

	movies, ok := root.rootDir.subdirs["Movies"]
	if !ok {
		t.Fatal("expected Movies subdirectory after reload")
	}
	if _, ok := movies.files["movie2.mkv"]; !ok {
		t.Error("expected movie2.mkv in Movies dir")
	}
}

func TestMKVFSRoot_Reload_RemoveDirectory(t *testing.T) {
	factory := &mockReaderFactory{
		readers: map[string]*mockReader{
			"/data/m1.dedup": {data: []byte("m1"), originalSize: 100},
			"/data/m2.dedup": {data: []byte("m2"), originalSize: 200},
		},
	}

	initial := []dedup.Config{
		{Name: "root.mkv", DedupFile: "/data/m1.dedup", SourceDir: "/src"},
		{Name: "Movies/movie.mkv", DedupFile: "/data/m2.dedup", SourceDir: "/src"},
	}
	root, err := NewMKVFSFromConfigs(initial, false, factory, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := root.rootDir.subdirs["Movies"]; !ok {
		t.Fatal("expected Movies dir initially")
	}

	// Reload without the subdir file
	updated := []dedup.Config{
		{Name: "root.mkv", DedupFile: "/data/m1.dedup", SourceDir: "/src"},
	}
	if err := root.Reload(updated, nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := root.rootDir.subdirs["Movies"]; ok {
		t.Error("Movies dir should have been removed after reload")
	}
}

func TestMKVFSRoot_Reload_SkipsBadConfigs(t *testing.T) {
	factory := &mockReaderFactory{
		readers: map[string]*mockReader{
			"/data/m1.dedup": {data: []byte("m1"), originalSize: 100},
		},
	}

	initial := []dedup.Config{
		{Name: "movie1.mkv", DedupFile: "/data/m1.dedup", SourceDir: "/src"},
	}
	root, err := NewMKVFSFromConfigs(initial, false, factory, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Reload with a config that has a bad dedup file
	var logMessages []string
	logFn := func(format string, args ...interface{}) {
		logMessages = append(logMessages, fmt.Sprintf(format, args...))
	}

	updated := []dedup.Config{
		{Name: "movie1.mkv", DedupFile: "/data/m1.dedup", SourceDir: "/src"},
		{Name: "bad.mkv", DedupFile: "/data/nonexistent.dedup", SourceDir: "/src"},
	}
	if err := root.Reload(updated, logFn); err != nil {
		t.Fatal(err)
	}

	// Should have 1 file (bad config skipped)
	if len(root.files) != 1 {
		t.Errorf("expected 1 file after reload, got %d", len(root.files))
	}

	// Should have logged a warning
	foundWarning := false
	for _, msg := range logMessages {
		if strings.Contains(msg, "skipping") && strings.Contains(msg, "bad.mkv") {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Error("expected warning about skipped bad config")
	}
}

// --- mergeDirectoryTree tests ---

func TestMergeDirectoryTree_AddFiles(t *testing.T) {
	existing := &MKVFSDirNode{
		files:   map[string]*MKVFile{"a.mkv": {Name: "a.mkv"}},
		subdirs: make(map[string]*MKVFSDirNode),
	}
	newTree := &MKVFSDirNode{
		files:   map[string]*MKVFile{"a.mkv": {Name: "a.mkv"}, "b.mkv": {Name: "b.mkv"}},
		subdirs: make(map[string]*MKVFSDirNode),
	}

	mergeDirectoryTree(existing, newTree)

	if len(existing.files) != 2 {
		t.Errorf("expected 2 files, got %d", len(existing.files))
	}
	if _, ok := existing.files["b.mkv"]; !ok {
		t.Error("expected b.mkv after merge")
	}
}

func TestMergeDirectoryTree_RemoveFiles(t *testing.T) {
	existing := &MKVFSDirNode{
		files:   map[string]*MKVFile{"a.mkv": {Name: "a.mkv"}, "b.mkv": {Name: "b.mkv"}},
		subdirs: make(map[string]*MKVFSDirNode),
	}
	newTree := &MKVFSDirNode{
		files:   map[string]*MKVFile{"a.mkv": {Name: "a.mkv"}},
		subdirs: make(map[string]*MKVFSDirNode),
	}

	mergeDirectoryTree(existing, newTree)

	if len(existing.files) != 1 {
		t.Errorf("expected 1 file, got %d", len(existing.files))
	}
	if _, ok := existing.files["b.mkv"]; ok {
		t.Error("b.mkv should have been removed")
	}
}

func TestMergeDirectoryTree_AddSubdir(t *testing.T) {
	existing := &MKVFSDirNode{
		files:   make(map[string]*MKVFile),
		subdirs: make(map[string]*MKVFSDirNode),
	}
	newTree := &MKVFSDirNode{
		files: make(map[string]*MKVFile),
		subdirs: map[string]*MKVFSDirNode{
			"Movies": {
				name:    "Movies",
				path:    "Movies",
				files:   map[string]*MKVFile{"film.mkv": {Name: "Movies/film.mkv"}},
				subdirs: make(map[string]*MKVFSDirNode),
			},
		},
	}

	mergeDirectoryTree(existing, newTree)

	if _, ok := existing.subdirs["Movies"]; !ok {
		t.Error("expected Movies subdir after merge")
	}
}

func TestMergeDirectoryTree_RemoveSubdir(t *testing.T) {
	existing := &MKVFSDirNode{
		files: make(map[string]*MKVFile),
		subdirs: map[string]*MKVFSDirNode{
			"Movies": {
				name:    "Movies",
				path:    "Movies",
				files:   map[string]*MKVFile{"film.mkv": {Name: "Movies/film.mkv"}},
				subdirs: make(map[string]*MKVFSDirNode),
			},
		},
	}
	newTree := &MKVFSDirNode{
		files:   make(map[string]*MKVFile),
		subdirs: make(map[string]*MKVFSDirNode),
	}

	mergeDirectoryTree(existing, newTree)

	if _, ok := existing.subdirs["Movies"]; ok {
		t.Error("Movies subdir should have been removed")
	}
}

func TestMergeDirectoryTree_RecursiveMerge(t *testing.T) {
	existingMovies := &MKVFSDirNode{
		name:    "Movies",
		path:    "Movies",
		files:   map[string]*MKVFile{"old.mkv": {Name: "Movies/old.mkv"}},
		subdirs: make(map[string]*MKVFSDirNode),
	}
	existing := &MKVFSDirNode{
		files:   make(map[string]*MKVFile),
		subdirs: map[string]*MKVFSDirNode{"Movies": existingMovies},
	}

	newTree := &MKVFSDirNode{
		files: make(map[string]*MKVFile),
		subdirs: map[string]*MKVFSDirNode{
			"Movies": {
				name:    "Movies",
				path:    "Movies",
				files:   map[string]*MKVFile{"new.mkv": {Name: "Movies/new.mkv"}},
				subdirs: make(map[string]*MKVFSDirNode),
			},
		},
	}

	mergeDirectoryTree(existing, newTree)

	// The existing Movies node should be the SAME object (not replaced)
	if existing.subdirs["Movies"] != existingMovies {
		t.Error("existing Movies node should be preserved (same pointer)")
	}
	// But its files should be updated
	if _, ok := existingMovies.files["new.mkv"]; !ok {
		t.Error("expected new.mkv in Movies after merge")
	}
	if _, ok := existingMovies.files["old.mkv"]; ok {
		t.Error("old.mkv should have been removed from Movies")
	}
}
