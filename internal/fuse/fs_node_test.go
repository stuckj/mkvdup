package fuse

import (
	"context"
	"errors"
	"syscall"
	"testing"

	"github.com/hanwen/go-fuse/v2/fuse"
)

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

// --- Disable/Enable Tests ---

func TestMKVFile_Disable(t *testing.T) {
	mockRdr := &mockReader{
		data:         []byte("test data"),
		originalSize: 9,
	}
	file := &MKVFile{
		Name:   "test.mkv",
		Size:   9,
		reader: mockRdr,
	}

	file.Disable()

	file.mu.RLock()
	disabled := file.disabled
	reader := file.reader
	file.mu.RUnlock()

	if !disabled {
		t.Error("expected file to be disabled")
	}
	if reader != nil {
		t.Error("expected reader to be nil after disable")
	}
	if !mockRdr.closed {
		t.Error("expected reader to be closed after disable")
	}
}

func TestMKVFile_Disable_NoReader(t *testing.T) {
	file := &MKVFile{
		Name: "test.mkv",
		Size: 100,
	}

	// Should not panic
	file.Disable()

	file.mu.RLock()
	disabled := file.disabled
	file.mu.RUnlock()

	if !disabled {
		t.Error("expected file to be disabled")
	}
}

func TestMKVFile_Enable(t *testing.T) {
	file := &MKVFile{
		Name:     "test.mkv",
		Size:     100,
		disabled: true,
	}

	file.Enable()

	file.mu.RLock()
	disabled := file.disabled
	file.mu.RUnlock()

	if disabled {
		t.Error("expected file to be enabled")
	}
}

func TestMKVFSNode_Open_Disabled(t *testing.T) {
	file := &MKVFile{
		Name:     "test.mkv",
		Size:     100,
		disabled: true,
	}
	node := &MKVFSNode{file: file}

	ctx := ContextWithCaller(context.Background(), 0, 0)
	_, _, errno := node.Open(ctx, 0)
	if errno != syscall.EIO {
		t.Errorf("expected EIO for disabled file, got %d", errno)
	}
}

func TestMKVFSNode_Read_Disabled(t *testing.T) {
	file := &MKVFile{
		Name:     "test.mkv",
		Size:     100,
		disabled: true,
		reader:   &mockReader{data: []byte("should not read")},
	}
	node := &MKVFSNode{file: file}

	ctx := ContextWithCaller(context.Background(), 0, 0)
	buf := make([]byte, 10)
	_, errno := node.Read(ctx, nil, buf, 0)
	if errno != syscall.EIO {
		t.Errorf("expected EIO for disabled file, got %d", errno)
	}
}

func TestMKVFile_DisableThenEnable_ReadsWork(t *testing.T) {
	testData := []byte("Hello, FUSE!")
	mockRdr := &mockReader{
		data:         testData,
		originalSize: int64(len(testData)),
	}

	readerFactory := &mockReaderFactory{
		readers: map[string]*mockReader{
			"/path/to/movie.dedup": {
				data:         testData,
				originalSize: int64(len(testData)),
			},
		},
	}

	file := &MKVFile{
		Name:          "test.mkv",
		DedupPath:     "/path/to/movie.dedup",
		SourceDir:     "/path/to/source",
		Size:          int64(len(testData)),
		reader:        mockRdr,
		readerFactory: readerFactory,
	}
	node := &MKVFSNode{file: file}

	// Disable
	file.Disable()
	ctx := ContextWithCaller(context.Background(), 0, 0)
	_, _, errno := node.Open(ctx, 0)
	if errno != syscall.EIO {
		t.Fatalf("expected EIO while disabled, got %d", errno)
	}

	// Enable
	file.Enable()
	_, _, errno = node.Open(ctx, 0)
	if errno != 0 {
		t.Fatalf("expected successful open after enable, got %d", errno)
	}

	// Read should work
	buf := make([]byte, len(testData))
	result, errno := node.Read(ctx, nil, buf, 0)
	if errno != 0 {
		t.Fatalf("expected successful read after enable, got %d", errno)
	}
	data, _ := result.Bytes(buf)
	if string(data) != string(testData) {
		t.Errorf("expected %q, got %q", testData, data)
	}
}
