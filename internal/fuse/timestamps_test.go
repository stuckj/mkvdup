package fuse

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/hanwen/go-fuse/v2/fuse"
)

// writeFileWithMtime creates a file at path with the given content and mtime.
func writeFileWithMtime(t *testing.T, path string, mtime time.Time) {
	t.Helper()
	if err := os.WriteFile(path, []byte("dedup"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
}

// --- MKVFile.DerivedMtime / RefreshDerivedMtime ---

func TestMKVFile_DerivedMtime(t *testing.T) {
	dir := t.TempDir()
	dedupPath := filepath.Join(dir, "v.mkvdup")
	want := time.Unix(1_600_000_000, 0)
	writeFileWithMtime(t, dedupPath, want)

	f := &MKVFile{Name: "v.mkv", DedupPath: dedupPath}

	got := f.DerivedMtime()
	if got.Unix() != want.Unix() {
		t.Errorf("DerivedMtime = %v, want %v", got.Unix(), want.Unix())
	}

	// Second call returns the cached value even if the file changes on disk.
	writeFileWithMtime(t, dedupPath, time.Unix(1_700_000_000, 0))
	if cached := f.DerivedMtime(); cached.Unix() != want.Unix() {
		t.Errorf("DerivedMtime (cached) = %v, want %v", cached.Unix(), want.Unix())
	}
}

func TestMKVFile_DerivedMtime_MissingFileFallback(t *testing.T) {
	f := &MKVFile{Name: "gone.mkv", DedupPath: filepath.Join(t.TempDir(), "does-not-exist.mkvdup")}
	if got := f.DerivedMtime(); got.Unix() != fsStartTime.Unix() {
		t.Errorf("DerivedMtime fallback = %v, want fsStartTime %v", got.Unix(), fsStartTime.Unix())
	}
}

func TestMKVFile_RefreshDerivedMtime(t *testing.T) {
	dir := t.TempDir()
	dedupPath := filepath.Join(dir, "v.mkvdup")
	writeFileWithMtime(t, dedupPath, time.Unix(1_600_000_000, 0))

	f := &MKVFile{Name: "v.mkv", DedupPath: dedupPath}
	_ = f.DerivedMtime() // prime the cache

	// No change → returns false.
	if changed := f.RefreshDerivedMtime(); changed {
		t.Error("RefreshDerivedMtime reported a change when mtime was unchanged")
	}

	// Change the mtime → returns true and updates the cache.
	newMtime := time.Unix(1_650_000_000, 0)
	if err := os.Chtimes(dedupPath, newMtime, newMtime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	if changed := f.RefreshDerivedMtime(); !changed {
		t.Error("RefreshDerivedMtime did not report a change after mtime updated")
	}
	if got := f.DerivedMtime(); got.Unix() != newMtime.Unix() {
		t.Errorf("DerivedMtime after refresh = %v, want %v", got.Unix(), newMtime.Unix())
	}
}

// --- PermissionStore mtime overrides ---

func TestPermissionStore_Mtime_LoadSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "permissions.yaml")
	store := NewPermissionStore(path, DefaultPerms(), false)

	fileMtime := int64(1_600_000_000)
	if err := store.SetFileMtime("video.mkv", &fileMtime); err != nil {
		t.Fatalf("SetFileMtime: %v", err)
	}
	dirMtime := int64(1_500_000_000)
	if err := store.SetDirMtime("Movies", &dirMtime); err != nil {
		t.Fatalf("SetDirMtime: %v", err)
	}

	store2 := NewPermissionStore(path, DefaultPerms(), false)
	if err := store2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := store2.GetFileMtimeOverride("video.mkv"); got == nil || *got != fileMtime {
		t.Errorf("GetFileMtimeOverride = %v, want %d", got, fileMtime)
	}
	if got := store2.GetDirMtimeOverride("Movies"); got == nil || *got != dirMtime {
		t.Errorf("GetDirMtimeOverride = %v, want %d", got, dirMtime)
	}
}

func TestPermissionStore_Mtime_PreservesPerms(t *testing.T) {
	store := NewPermissionStore("", DefaultPerms(), false)

	uid, mode := uint32(1000), uint32(0640)
	if err := store.SetFilePerms("v.mkv", &uid, nil, &mode); err != nil {
		t.Fatalf("SetFilePerms: %v", err)
	}
	mtime := int64(1_600_000_000)
	if err := store.SetFileMtime("v.mkv", &mtime); err != nil {
		t.Fatalf("SetFileMtime: %v", err)
	}

	// Setting mtime must not clobber uid/mode.
	gotUID, _, gotMode := store.GetFilePerms("v.mkv")
	if gotUID != uid || gotMode != mode {
		t.Errorf("perms after SetFileMtime = (uid=%d, mode=%o), want (uid=%d, mode=%o)", gotUID, gotMode, uid, mode)
	}
	if got := store.GetFileMtimeOverride("v.mkv"); got == nil || *got != mtime {
		t.Errorf("mtime override = %v, want %d", got, mtime)
	}

	// Setting perms again must not clobber the mtime override.
	newMode := uint32(0600)
	if err := store.SetFilePerms("v.mkv", nil, nil, &newMode); err != nil {
		t.Fatalf("SetFilePerms 2: %v", err)
	}
	if got := store.GetFileMtimeOverride("v.mkv"); got == nil || *got != mtime {
		t.Errorf("mtime override after chmod = %v, want %d", got, mtime)
	}
}

func TestPermissionStore_Mtime_Clear(t *testing.T) {
	store := NewPermissionStore("", DefaultPerms(), false)
	mtime := int64(1_600_000_000)
	if err := store.SetFileMtime("v.mkv", &mtime); err != nil {
		t.Fatalf("SetFileMtime: %v", err)
	}
	if err := store.SetFileMtime("v.mkv", nil); err != nil {
		t.Fatalf("SetFileMtime clear: %v", err)
	}
	if got := store.GetFileMtimeOverride("v.mkv"); got != nil {
		t.Errorf("mtime override after clear = %v, want nil", got)
	}
}

// --- Node Getattr timestamps ---

func TestMKVFSNode_Getattr_DerivedMtime(t *testing.T) {
	dir := t.TempDir()
	dedupPath := filepath.Join(dir, "v.mkvdup")
	want := time.Unix(1_600_000_000, 0)
	writeFileWithMtime(t, dedupPath, want)

	store := NewPermissionStore("", DefaultPerms(), false)
	file := &MKVFile{Name: "v.mkv", DedupPath: dedupPath, Size: 100}
	node := &MKVFSNode{file: file, path: "v.mkv", permStore: store}

	var out fuse.AttrOut
	if errno := node.Getattr(context.Background(), nil, &out); errno != 0 {
		t.Fatalf("Getattr errno %d", errno)
	}
	if out.Mtime != uint64(want.Unix()) {
		t.Errorf("Mtime = %d, want %d", out.Mtime, want.Unix())
	}
	// atime and ctime mirror mtime; nanoseconds are zeroed.
	if out.Atime != out.Mtime || out.Ctime != out.Mtime {
		t.Errorf("atime/ctime (%d/%d) should equal mtime %d", out.Atime, out.Ctime, out.Mtime)
	}
	if out.Mtimensec != 0 || out.Atimensec != 0 || out.Ctimensec != 0 {
		t.Errorf("nanosecond fields not zeroed: %d/%d/%d", out.Atimensec, out.Mtimensec, out.Ctimensec)
	}

	// A repeated stat is stable (not time.Now()).
	var out2 fuse.AttrOut
	_ = node.Getattr(context.Background(), nil, &out2)
	if out2.Mtime != out.Mtime {
		t.Errorf("Mtime not stable across stats: %d vs %d", out2.Mtime, out.Mtime)
	}
}

func TestMKVFSNode_Getattr_MtimeOverride(t *testing.T) {
	dir := t.TempDir()
	dedupPath := filepath.Join(dir, "v.mkvdup")
	writeFileWithMtime(t, dedupPath, time.Unix(1_600_000_000, 0))

	store := NewPermissionStore("", DefaultPerms(), false)
	override := int64(1_234_567_890)
	if err := store.SetFileMtime("v.mkv", &override); err != nil {
		t.Fatalf("SetFileMtime: %v", err)
	}

	file := &MKVFile{Name: "v.mkv", DedupPath: dedupPath, Size: 100}
	node := &MKVFSNode{file: file, path: "v.mkv", permStore: store}

	var out fuse.AttrOut
	_ = node.Getattr(context.Background(), nil, &out)
	if out.Mtime != uint64(override) {
		t.Errorf("Mtime = %d, want override %d", out.Mtime, override)
	}
}

// --- Node Setattr utimes ---

func TestMKVFSNode_Setattr_Mtime(t *testing.T) {
	store := NewPermissionStore("", DefaultPerms(), false)
	file := &MKVFile{Name: "v.mkv", Size: 100}
	node := &MKVFSNode{file: file, path: "v.mkv", permStore: store}

	want := int64(1_600_000_000)
	in := &fuse.SetAttrIn{}
	in.Valid = fuse.FATTR_MTIME
	in.Mtime = uint64(want)

	ctx := ContextWithCaller(context.Background(), 0, 0) // root
	var out fuse.AttrOut
	if errno := node.Setattr(ctx, nil, in, &out); errno != 0 {
		t.Fatalf("Setattr errno %d", errno)
	}
	if got := store.GetFileMtimeOverride("v.mkv"); got == nil || *got != want {
		t.Errorf("stored mtime = %v, want %d", got, want)
	}
	if out.Mtime != uint64(want) {
		t.Errorf("returned Mtime = %d, want %d", out.Mtime, want)
	}
}

func TestMKVFSNode_Setattr_MtimeNow(t *testing.T) {
	store := NewPermissionStore("", DefaultPerms(), false)
	node := &MKVFSNode{file: &MKVFile{Name: "v.mkv"}, path: "v.mkv", permStore: store}

	before := time.Now().Unix()
	in := &fuse.SetAttrIn{}
	in.Valid = fuse.FATTR_MTIME | fuse.FATTR_MTIME_NOW

	ctx := ContextWithCaller(context.Background(), 0, 0)
	var out fuse.AttrOut
	if errno := node.Setattr(ctx, nil, in, &out); errno != 0 {
		t.Fatalf("Setattr errno %d", errno)
	}
	got := store.GetFileMtimeOverride("v.mkv")
	if got == nil || *got < before || *got > time.Now().Unix()+1 {
		t.Errorf("stored mtime = %v, want ~now (>= %d)", got, before)
	}
}

func TestMKVFSNode_Setattr_AtimeOnlyIsNoOp(t *testing.T) {
	store := NewPermissionStore("", DefaultPerms(), false)
	node := &MKVFSNode{file: &MKVFile{Name: "v.mkv"}, path: "v.mkv", permStore: store}

	in := &fuse.SetAttrIn{}
	in.Valid = fuse.FATTR_ATIME
	in.Atime = 12345

	ctx := ContextWithCaller(context.Background(), 0, 0)
	var out fuse.AttrOut
	if errno := node.Setattr(ctx, nil, in, &out); errno != 0 {
		t.Fatalf("Setattr (atime only) errno %d, want 0", errno)
	}
	if got := store.GetFileMtimeOverride("v.mkv"); got != nil {
		t.Errorf("atime-only touch persisted an mtime override %v, want nil", got)
	}
}

func TestMKVFSNode_Setattr_MtimeDenied(t *testing.T) {
	store := NewPermissionStore("", DefaultPerms(), false)
	owner := uint32(1000)
	_ = store.SetFilePerms("v.mkv", &owner, nil, nil)
	node := &MKVFSNode{file: &MKVFile{Name: "v.mkv"}, path: "v.mkv", permStore: store}

	in := &fuse.SetAttrIn{}
	in.Valid = fuse.FATTR_MTIME
	in.Mtime = 1_600_000_000

	// Caller is neither root nor the owner.
	ctx := ContextWithCaller(context.Background(), 2000, 2000)
	var out fuse.AttrOut
	if errno := node.Setattr(ctx, nil, in, &out); errno != syscall.EPERM {
		t.Errorf("Setattr = %v, want EPERM", errno)
	}
	if got := store.GetFileMtimeOverride("v.mkv"); got != nil {
		t.Errorf("denied touch persisted an override %v, want nil", got)
	}
}

func TestMKVFSNode_Setattr_SizeStillRejected(t *testing.T) {
	store := NewPermissionStore("", DefaultPerms(), false)
	node := &MKVFSNode{file: &MKVFile{Name: "v.mkv", Size: 100}, path: "v.mkv", permStore: store}

	in := &fuse.SetAttrIn{}
	in.Valid = fuse.FATTR_SIZE
	in.Size = 0

	ctx := ContextWithCaller(context.Background(), 0, 0)
	var out fuse.AttrOut
	if errno := node.Setattr(ctx, nil, in, &out); errno != syscall.EROFS {
		t.Errorf("Setattr(size) = %v, want EROFS", errno)
	}
}

// --- Directory timestamps ---

func TestMKVFSDirNode_Getattr_StableTime(t *testing.T) {
	store := NewPermissionStore("", DefaultPerms(), false)
	d := &MKVFSDirNode{name: "Movies", path: "Movies", permStore: store,
		files: map[string]*MKVFile{}, subdirs: map[string]*MKVFSDirNode{}}

	var out fuse.AttrOut
	if errno := d.Getattr(context.Background(), nil, &out); errno != 0 {
		t.Fatalf("Getattr errno %d", errno)
	}
	if out.Mtime != uint64(fsStartTime.Unix()) {
		t.Errorf("dir Mtime = %d, want fsStartTime %d", out.Mtime, fsStartTime.Unix())
	}
}

func TestMKVFSDirNode_Setattr_Mtime(t *testing.T) {
	store := NewPermissionStore("", DefaultPerms(), false)
	d := &MKVFSDirNode{name: "Movies", path: "Movies", permStore: store,
		files: map[string]*MKVFile{}, subdirs: map[string]*MKVFSDirNode{}}

	want := int64(1_600_000_000)
	in := &fuse.SetAttrIn{}
	in.Valid = fuse.FATTR_MTIME
	in.Mtime = uint64(want)

	ctx := ContextWithCaller(context.Background(), 0, 0)
	var out fuse.AttrOut
	if errno := d.Setattr(ctx, nil, in, &out); errno != 0 {
		t.Fatalf("Setattr errno %d", errno)
	}
	if got := store.GetDirMtimeOverride("Movies"); got == nil || *got != want {
		t.Errorf("stored dir mtime = %v, want %d", got, want)
	}
	if out.Mtime != uint64(want) {
		t.Errorf("returned dir Mtime = %d, want %d", out.Mtime, want)
	}
}

// --- Watcher: dedup-file tracking ---

func TestSourceWatcher_Update_BuildsDedupReverse(t *testing.T) {
	sw, err := NewSourceWatcher("warn", 0, nil, nil)
	if err != nil {
		t.Fatalf("NewSourceWatcher: %v", err)
	}
	defer sw.watcher.Close()

	dir := t.TempDir()
	dedupPath := filepath.Join(dir, "a.mkvdup")
	if err := os.WriteFile(dedupPath, []byte("x"), 0644); err != nil {
		t.Fatalf("write dedup: %v", err)
	}
	file := &MKVFile{Name: "a.mkv", DedupPath: dedupPath, SourceDir: dir}
	factory := &mockReaderFactory{readers: map[string]*mockReader{dedupPath: {}}}

	sw.Update(map[string]*MKVFile{"a.mkv": file}, factory)

	sw.mu.RLock()
	affected := sw.dedupReverse[filepath.Clean(dedupPath)]
	sw.mu.RUnlock()
	if len(affected) != 1 || affected[0] != file {
		t.Errorf("dedupReverse[%q] = %v, want [%p]", dedupPath, affected, file)
	}
}

func TestSourceWatcher_RefreshDedupMtime(t *testing.T) {
	sw, err := NewSourceWatcher("warn", 0, nil, nil)
	if err != nil {
		t.Fatalf("NewSourceWatcher: %v", err)
	}
	defer sw.watcher.Close()

	dir := t.TempDir()
	dedupPath := filepath.Join(dir, "a.mkvdup")
	writeFileWithMtime(t, dedupPath, time.Unix(1_600_000_000, 0))

	file := &MKVFile{Name: "a.mkv", DedupPath: dedupPath}
	_ = file.DerivedMtime() // prime cache

	sw.mu.Lock()
	sw.dedupReverse[filepath.Clean(dedupPath)] = []*MKVFile{file}
	sw.mu.Unlock()

	var invalidated []string
	sw.SetAttrInvalidator(func(p string) { invalidated = append(invalidated, p) })

	// Change the dedup file's mtime, then refresh.
	newMtime := time.Unix(1_650_000_000, 0)
	if err := os.Chtimes(dedupPath, newMtime, newMtime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	sw.refreshDedupMtime(dedupPath)

	if got := file.DerivedMtime(); got.Unix() != newMtime.Unix() {
		t.Errorf("derived mtime = %v, want %v", got.Unix(), newMtime.Unix())
	}
	if len(invalidated) != 1 || invalidated[0] != "a.mkv" {
		t.Errorf("invalidated = %v, want [a.mkv]", invalidated)
	}

	// A second refresh with no change fires nothing.
	invalidated = nil
	sw.refreshDedupMtime(dedupPath)
	if len(invalidated) != 0 {
		t.Errorf("invalidated on no-change = %v, want empty", invalidated)
	}
}

func TestSourceWatcher_RefreshDedupMtime_UntrackedNoOp(t *testing.T) {
	sw, err := NewSourceWatcher("warn", 0, nil, nil)
	if err != nil {
		t.Fatalf("NewSourceWatcher: %v", err)
	}
	defer sw.watcher.Close()

	var invalidated []string
	sw.SetAttrInvalidator(func(p string) { invalidated = append(invalidated, p) })
	// Path not in dedupReverse — must be a no-op, no panic.
	sw.refreshDedupMtime("/nonexistent/x.mkvdup")
	if len(invalidated) != 0 {
		t.Errorf("invalidated = %v, want empty for untracked path", invalidated)
	}
}
