package fuse

import (
	"context"
	"strconv"
	"syscall"
	"testing"

	"github.com/hanwen/go-fuse/v2/fuse"
)

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
