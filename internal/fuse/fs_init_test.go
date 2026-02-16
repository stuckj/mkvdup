package fuse

import (
	"context"
	"errors"
	"testing"

	"github.com/hanwen/go-fuse/v2/fuse"
)

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
