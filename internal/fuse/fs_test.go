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

// --- Directory Tree Tests ---

func TestBuildDirectoryTree(t *testing.T) {
	files := []*MKVFile{
		{Name: "Movies/Action/Matrix.mkv", Size: 100},
		{Name: "Movies/Action/JohnWick.mkv", Size: 200},
		{Name: "Movies/Comedy/Hangover.mkv", Size: 150},
		{Name: "root.mkv", Size: 50},
	}

	tree := BuildDirectoryTree(files, false, nil)

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

	tree := BuildDirectoryTree(files, false, nil)

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

	tree := BuildDirectoryTree(files, false, nil)

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
		name := "Category" + string(rune('A'+category)) + "/Sub" + itoa(subcategory) + "/movie" + itoa(fileNum) + ".mkv"
		files[i] = &MKVFile{
			Name: name,
			Size: int64(i * 100),
		}
	}

	tree := BuildDirectoryTree(files, false, nil)

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

// itoa converts int to string without importing strconv
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var result []byte
	for i > 0 {
		result = append([]byte{byte('0' + i%10)}, result...)
		i /= 10
	}
	return string(result)
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

	stream, errno := dir.Readdir(context.Background())
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
				Name:      "Movies/Action/Matrix.mkv",
				DedupFile: "/data/matrix.dedup",
				SourceDir: "/data/source",
			},
			"/configs/comedy.yaml": {
				Name:      "Movies/Comedy/Hangover.mkv",
				DedupFile: "/data/hangover.dedup",
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
			"/data/matrix.dedup":     {data: []byte("matrix"), originalSize: 6},
			"/data/hangover.dedup":   {data: []byte("hangover"), originalSize: 8},
			"/data/standalone.dedup": {data: []byte("standalone"), originalSize: 10},
		},
	}

	root, err := NewMKVFSWithFactories(
		[]string{"/configs/action.yaml", "/configs/comedy.yaml", "/configs/root.yaml"},
		false,
		readerFactory,
		configReader,
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

	// Check Action has Matrix
	action, ok := movies.subdirs["Action"]
	if !ok {
		t.Fatal("expected Action subdirectory")
	}
	if _, ok := action.files["Matrix.mkv"]; !ok {
		t.Error("expected Matrix.mkv in Action directory")
	}

	// Check Comedy has Hangover
	comedy, ok := movies.subdirs["Comedy"]
	if !ok {
		t.Fatal("expected Comedy subdirectory")
	}
	if _, ok := comedy.files["Hangover.mkv"]; !ok {
		t.Error("expected Hangover.mkv in Comedy directory")
	}
}
