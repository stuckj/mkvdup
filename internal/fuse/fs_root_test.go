package fuse

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"syscall"
	"testing"

	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/stuckj/mkvdup/internal/dedup"
)

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

	// Pointer identity should be preserved (cached inodes see updates in place)
	if root.files["movie.mkv"] != oldFile {
		t.Error("expected flat map to preserve original *MKVFile pointer")
	}
	if treeFile != oldFile {
		t.Error("expected tree to preserve original *MKVFile pointer")
	}

	// Old reader should have been closed since dedup path changed
	if !reader1.closed {
		t.Error("expected old reader to be closed after mapping change")
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

func TestMKVFSRoot_Reload_MoveToSubdirAndBack(t *testing.T) {
	factory := &mockReaderFactory{
		readers: map[string]*mockReader{
			"/data/m1.dedup": {data: []byte("m1"), originalSize: 100},
		},
	}

	// Start with file in subdirectory
	initial := []dedup.Config{
		{Name: "Zootopia/movie.mkv", DedupFile: "/data/m1.dedup", SourceDir: "/src"},
	}
	root, err := NewMKVFSFromConfigs(initial, false, factory, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := root.rootDir.subdirs["Zootopia"]; !ok {
		t.Fatal("expected Zootopia dir initially")
	}

	// First reload: move to root level (remove subdirectory)
	reload1 := []dedup.Config{
		{Name: "movie.mkv", DedupFile: "/data/m1.dedup", SourceDir: "/src"},
	}
	if err := root.Reload(reload1, nil); err != nil {
		t.Fatalf("first reload: %v", err)
	}
	if _, ok := root.rootDir.subdirs["Zootopia"]; ok {
		t.Error("Zootopia dir should be removed after first reload")
	}
	if _, ok := root.rootDir.files["movie.mkv"]; !ok {
		t.Fatal("expected movie.mkv at root after first reload")
	}

	// Second reload: move back to subdirectory (re-add subdirectory).
	// This is the crash scenario: mergeDirectoryTree inserts a new
	// MKVFSDirNode with an uninitialized fs.Inode. findParentInode must
	// return nil for the uninitialized parent to prevent notification panics.
	reload2 := []dedup.Config{
		{Name: "Zootopia/movie.mkv", DedupFile: "/data/m1.dedup", SourceDir: "/src"},
	}
	if err := root.Reload(reload2, nil); err != nil {
		t.Fatalf("second reload: %v", err)
	}
	if _, ok := root.rootDir.subdirs["Zootopia"]; !ok {
		t.Error("expected Zootopia dir after second reload")
	}
	if _, ok := root.rootDir.files["movie.mkv"]; ok {
		t.Error("movie.mkv should not be at root after second reload")
	}

	// Verify findParentInode returns nil for the uninitialized Zootopia
	// directory (its fs.Inode was never registered via NewPersistentInode)
	parentInode, _ := root.findParentInode("Zootopia/movie.mkv")
	if parentInode != nil {
		t.Error("findParentInode should return nil for uninitialized parent dir")
	}
}

func TestMKVFSRoot_FindParentInode_UninitializedDir(t *testing.T) {
	factory := &mockReaderFactory{
		readers: map[string]*mockReader{
			"/data/m1.dedup": {data: []byte("m1"), originalSize: 100},
		},
	}

	// Start with a file at root level
	initial := []dedup.Config{
		{Name: "movie.mkv", DedupFile: "/data/m1.dedup", SourceDir: "/src"},
	}
	root, err := NewMKVFSFromConfigs(initial, false, factory, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Reload to add a file in a new subdirectory
	reload := []dedup.Config{
		{Name: "Movies/Action/film.mkv", DedupFile: "/data/m1.dedup", SourceDir: "/src"},
	}
	if err := root.Reload(reload, nil); err != nil {
		t.Fatalf("reload: %v", err)
	}

	// Both "Movies" and "Action" are new directories with uninitialized inodes.
	// findParentInode should return nil for paths under them.
	parentInode, _ := root.findParentInode("Movies/Action/film.mkv")
	if parentInode != nil {
		t.Error("findParentInode should return nil for deeply nested uninitialized dirs")
	}

	// Root-level files should still resolve (root inode is always initialized
	// by go-fuse during mount, but in tests the embedded Inode is a zero value
	// too — this test verifies the directory tree walk, not root resolution)
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

func TestMKVFile_Reload_ClearsDisabled(t *testing.T) {
	factory := &mockReaderFactory{
		readers: map[string]*mockReader{
			"/data/m1.dedup": {data: []byte("m1"), originalSize: 100},
		},
	}

	initial := []dedup.Config{
		{Name: "movie.mkv", DedupFile: "/data/m1.dedup", SourceDir: "/src"},
	}
	root, err := NewMKVFSFromConfigs(initial, false, factory, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Disable the file
	file := root.files["movie.mkv"]
	file.Disable()

	file.mu.RLock()
	if !file.disabled {
		t.Fatal("expected file to be disabled")
	}
	file.mu.RUnlock()

	// Reload with same config — should clear disabled
	if err := root.Reload(initial, nil); err != nil {
		t.Fatal(err)
	}

	file.mu.RLock()
	disabled := file.disabled
	file.mu.RUnlock()

	if disabled {
		t.Error("expected disabled to be cleared after reload")
	}
}
