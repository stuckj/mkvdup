package fuse

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/stuckj/mkvdup/internal/dedup"
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

func TestMKVFile_RefreshDerivedMtime_FirstDeriveIsNotAChange(t *testing.T) {
	dir := t.TempDir()
	dedupPath := filepath.Join(dir, "v.mkvdup")
	want := time.Unix(1_600_000_000, 0)
	writeFileWithMtime(t, dedupPath, want)

	// Cache never populated (as after mount, or after updateFrom clears it on
	// reload). Nothing has been reported to the kernel, so there is nothing to
	// invalidate and this must not report a change.
	f := &MKVFile{Name: "v.mkv", DedupPath: dedupPath}
	if changed := f.RefreshDerivedMtime(); changed {
		t.Error("first refresh reported a change; nothing was cached to change from")
	}
	if got := f.DerivedMtime(); got.Unix() != want.Unix() {
		t.Errorf("baseline not recorded: DerivedMtime = %d, want %d", got.Unix(), want.Unix())
	}

	// A genuine change after the baseline is still reported.
	newMtime := time.Unix(1_650_000_000, 0)
	if err := os.Chtimes(dedupPath, newMtime, newMtime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	if changed := f.RefreshDerivedMtime(); !changed {
		t.Error("real mtime change after baseline was not reported")
	}
}

// TestMKVFile_RefreshDerivedMtime_RaceWithReload exercises the real-world
// pairing that made this a bug: the watcher goroutine refreshing a file's
// derived mtime while a config reload rewrites DedupPath underneath it. Run
// under -race, an unlocked read of DedupPath here is reported as a data race.
func TestMKVFile_RefreshDerivedMtime_RaceWithReload(t *testing.T) {
	dir := t.TempDir()
	pathA := filepath.Join(dir, "a.mkvdup")
	pathB := filepath.Join(dir, "b.mkvdup")
	writeFileWithMtime(t, pathA, time.Unix(1_600_000_000, 0))
	writeFileWithMtime(t, pathB, time.Unix(1_650_000_000, 0))

	f := &MKVFile{Name: "v.mkv", DedupPath: pathA}

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Watcher goroutine: refresh in a tight loop.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				f.RefreshDerivedMtime()
			}
		}
	}()

	// Reload goroutine: swap the dedup path back and forth, exactly as
	// mergeDirectoryTree/Reload do (updateFrom under the write lock).
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := range 300 {
			src := &MKVFile{Name: "v.mkv", DedupPath: pathA}
			if i%2 == 1 {
				src.DedupPath = pathB
			}
			f.mu.Lock()
			f.updateFrom(src)
			f.mu.Unlock()
		}
	}()

	time.Sleep(100 * time.Millisecond)
	close(stop)
	wg.Wait()

	// Whichever path won the last write, the cached mtime must correspond to it
	// and never be a value carried over from the other file.
	f.mu.RLock()
	gotPath, gotMtime, wasSet := f.DedupPath, f.derivedMtime, f.derivedSet
	f.mu.RUnlock()
	if wasSet {
		want := statMtime(gotPath)
		if !gotMtime.Equal(want) {
			t.Errorf("cached mtime %v does not match current DedupPath %s (want %v)", gotMtime, gotPath, want)
		}
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

	flush(t, store)

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
	d := &MKVFSDirNode{name: "Movies", path: "Movies", permStore: store, mtime: fsStartTime,
		files: map[string]*MKVFile{}, subdirs: map[string]*MKVFSDirNode{}}

	var out fuse.AttrOut
	if errno := d.Getattr(context.Background(), nil, &out); errno != 0 {
		t.Fatalf("Getattr errno %d", errno)
	}
	if out.Mtime != uint64(fsStartTime.Unix()) {
		t.Errorf("dir Mtime = %d, want mount time %d", out.Mtime, fsStartTime.Unix())
	}
}

// --- Directory mtime follows POSIX add/remove semantics ---

// dirMtimeOf returns the mtime a directory reports via Getattr.
func dirMtimeOf(t *testing.T, d *MKVFSDirNode) uint64 {
	t.Helper()
	var out fuse.AttrOut
	if errno := d.Getattr(context.Background(), nil, &out); errno != 0 {
		t.Fatalf("Getattr errno %d", errno)
	}
	return out.Mtime
}

func TestDirMtime_BuiltTreeUsesBuildTime(t *testing.T) {
	// Directories are stamped when the tree is built (mount time for the real
	// filesystem), not at package init — so assert the value falls inside the
	// build window rather than matching fsStartTime exactly.
	before := time.Now().Add(-2 * time.Second).Unix()
	f := &MKVFile{Name: "Movies/Action/a.mkv"}
	root := BuildDirectoryTree([]*MKVFile{f}, false, nil, nil)
	after := time.Now().Add(2 * time.Second).Unix()

	movies := root.subdirs["Movies"]
	action := movies.subdirs["Action"]
	for name, d := range map[string]*MKVFSDirNode{"root": root, "Movies": movies, "Action": action} {
		got := int64(dirMtimeOf(t, d))
		if got < before || got > after {
			t.Errorf("%s mtime = %d, want within build window [%d, %d]", name, got, before, after)
		}
	}
}

func TestDirMtime_AddUpdatesOnlyImmediateParent(t *testing.T) {
	// Start with Movies/Action/a.mkv
	root := BuildDirectoryTree([]*MKVFile{{Name: "Movies/Action/a.mkv"}}, false, nil, nil)
	movies := root.subdirs["Movies"]
	action := movies.subdirs["Action"]

	rootBefore := dirMtimeOf(t, root)
	moviesBefore := dirMtimeOf(t, movies)
	actionBefore := dirMtimeOf(t, action)

	// Add a sibling file inside Movies/Action.
	newTree := BuildDirectoryTree([]*MKVFile{
		{Name: "Movies/Action/a.mkv"},
		{Name: "Movies/Action/b.mkv"},
	}, false, nil, nil)
	mergeDirectoryTreeAt(root, newTree, time.Unix(1_700_000_000, 0))

	// Only the directory that directly gained the entry advances.
	if got := dirMtimeOf(t, action); got != 1_700_000_000 {
		t.Errorf("Action mtime = %d, want 1700000000 (gained a child)", got)
	}
	if got := dirMtimeOf(t, movies); got != moviesBefore {
		t.Errorf("Movies mtime = %d, want unchanged %d (no direct child change)", got, moviesBefore)
	}
	if got := dirMtimeOf(t, root); got != rootBefore {
		t.Errorf("root mtime = %d, want unchanged %d (no direct child change)", got, rootBefore)
	}
	if actionBefore == 1_700_000_000 {
		t.Fatal("test setup: Action mtime already equalled the merge time")
	}
}

func TestDirMtime_RemoveUpdatesParent(t *testing.T) {
	root := BuildDirectoryTree([]*MKVFile{
		{Name: "Movies/Action/a.mkv"},
		{Name: "Movies/Action/b.mkv"},
	}, false, nil, nil)
	action := root.subdirs["Movies"].subdirs["Action"]

	newTree := BuildDirectoryTree([]*MKVFile{{Name: "Movies/Action/a.mkv"}}, false, nil, nil)
	mergeDirectoryTreeAt(root, newTree, time.Unix(1_700_000_500, 0))

	if got := dirMtimeOf(t, action); got != 1_700_000_500 {
		t.Errorf("Action mtime = %d, want 1700000500 (lost a child)", got)
	}
}

func TestDirMtime_ContentChangeDoesNotTouchParent(t *testing.T) {
	// Same file set, but the file's dedup path changes — an in-place content
	// change, not an add/remove. POSIX: the directory mtime must not move.
	root := BuildDirectoryTree([]*MKVFile{{Name: "a.mkv", DedupPath: "/old.mkvdup"}}, false, nil, nil)
	before := dirMtimeOf(t, root)

	newTree := BuildDirectoryTree([]*MKVFile{{Name: "a.mkv", DedupPath: "/new.mkvdup"}}, false, nil, nil)
	mergeDirectoryTreeAt(root, newTree, time.Unix(1_700_001_000, 0))

	if got := dirMtimeOf(t, root); got != before {
		t.Errorf("root mtime = %d, want unchanged %d (content change is not add/remove)", got, before)
	}
}

func TestDirMtime_NewSubtreeStamped(t *testing.T) {
	root := BuildDirectoryTree([]*MKVFile{{Name: "a.mkv"}}, false, nil, nil)

	newTree := BuildDirectoryTree([]*MKVFile{
		{Name: "a.mkv"},
		{Name: "New/Deep/b.mkv"},
	}, false, nil, nil)
	mergeDirectoryTreeAt(root, newTree, time.Unix(1_700_002_000, 0))

	// Root gained "New"; the whole new subtree was created at merge time.
	if got := dirMtimeOf(t, root); got != 1_700_002_000 {
		t.Errorf("root mtime = %d, want 1700002000", got)
	}
	newDir := root.subdirs["New"]
	if got := dirMtimeOf(t, newDir); got != 1_700_002_000 {
		t.Errorf("New mtime = %d, want 1700002000", got)
	}
	if got := dirMtimeOf(t, newDir.subdirs["Deep"]); got != 1_700_002_000 {
		t.Errorf("New/Deep mtime = %d, want 1700002000", got)
	}
}

func TestDirMtime_OverrideWins(t *testing.T) {
	store := NewPermissionStore("", DefaultPerms(), false)
	override := int64(1_234_567_890)
	if err := store.SetDirMtime("Movies", &override); err != nil {
		t.Fatalf("SetDirMtime: %v", err)
	}

	root := BuildDirectoryTree([]*MKVFile{{Name: "Movies/a.mkv"}}, false, nil, store)
	movies := root.subdirs["Movies"]

	// Even after an add/remove bumps the node's own mtime, the override wins.
	newTree := BuildDirectoryTree([]*MKVFile{
		{Name: "Movies/a.mkv"},
		{Name: "Movies/b.mkv"},
	}, false, nil, store)
	mergeDirectoryTreeAt(root, newTree, time.Unix(1_700_003_000, 0))

	if got := dirMtimeOf(t, movies); got != uint64(override) {
		t.Errorf("Movies mtime = %d, want override %d", got, override)
	}
}

// --- Directory mtime through the real Reload() path ---
// These mirror the SIGHUP integration tests but run without a FUSE mount.
// They compare time.Time values rather than the second-granularity values
// Getattr reports, since a reload here happens within the same second as mount.

// nodeMtime reads a directory node's mtime under its lock.
func nodeMtime(d *MKVFSDirNode) time.Time {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.mtime
}

func reloadDirFixture(t *testing.T) (*MKVFSRoot, *MKVFSDirNode, *MKVFSDirNode) {
	t.Helper()
	factory := &mockReaderFactory{
		readers: map[string]*mockReader{
			"/data/a.dedup": {originalSize: 100},
			"/data/b.dedup": {originalSize: 200},
		},
	}
	root, err := NewMKVFSFromConfigs([]dedup.Config{
		{Name: "Movies/Action/a.mkv", DedupFile: "/data/a.dedup", SourceDir: "/src"},
	}, false, factory, nil)
	if err != nil {
		t.Fatalf("NewMKVFSFromConfigs: %v", err)
	}
	movies := root.rootDir.subdirs["Movies"]
	if movies == nil {
		t.Fatal("Movies directory not created")
	}
	action := movies.subdirs["Action"]
	if action == nil {
		t.Fatal("Movies/Action directory not created")
	}
	return root, movies, action
}

func TestMKVFSRoot_Reload_DirMtime_AddUpdatesOnlyParent(t *testing.T) {
	root, movies, action := reloadDirFixture(t)

	rootBefore := nodeMtime(root.rootDir)
	moviesBefore := nodeMtime(movies)
	actionBefore := nodeMtime(action)

	if err := root.Reload([]dedup.Config{
		{Name: "Movies/Action/a.mkv", DedupFile: "/data/a.dedup", SourceDir: "/src"},
		{Name: "Movies/Action/b.mkv", DedupFile: "/data/b.dedup", SourceDir: "/src"},
	}, nil); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	if !nodeMtime(action).After(actionBefore) {
		t.Errorf("Movies/Action mtime did not advance after gaining a child (%v)", nodeMtime(action))
	}
	if !nodeMtime(movies).Equal(moviesBefore) {
		t.Errorf("Movies mtime changed to %v, want unchanged %v", nodeMtime(movies), moviesBefore)
	}
	if !nodeMtime(root.rootDir).Equal(rootBefore) {
		t.Errorf("root mtime changed to %v, want unchanged %v", nodeMtime(root.rootDir), rootBefore)
	}
}

func TestMKVFSRoot_Reload_DirMtime_RemoveUpdatesParent(t *testing.T) {
	root, movies, action := reloadDirFixture(t)

	// Add, then remove, so we isolate the removal.
	if err := root.Reload([]dedup.Config{
		{Name: "Movies/Action/a.mkv", DedupFile: "/data/a.dedup", SourceDir: "/src"},
		{Name: "Movies/Action/b.mkv", DedupFile: "/data/b.dedup", SourceDir: "/src"},
	}, nil); err != nil {
		t.Fatalf("Reload (add): %v", err)
	}
	moviesBefore := nodeMtime(movies)
	afterAdd := nodeMtime(action)

	if err := root.Reload([]dedup.Config{
		{Name: "Movies/Action/a.mkv", DedupFile: "/data/a.dedup", SourceDir: "/src"},
	}, nil); err != nil {
		t.Fatalf("Reload (remove): %v", err)
	}

	if !nodeMtime(action).After(afterAdd) {
		t.Errorf("Movies/Action mtime did not advance after losing a child (%v)", nodeMtime(action))
	}
	if !nodeMtime(movies).Equal(moviesBefore) {
		t.Errorf("Movies mtime changed to %v, want unchanged %v", nodeMtime(movies), moviesBefore)
	}
}

func TestMKVFSRoot_Reload_DirMtime_NoOpReloadKeepsMtime(t *testing.T) {
	root, movies, action := reloadDirFixture(t)

	rootBefore := nodeMtime(root.rootDir)
	moviesBefore := nodeMtime(movies)
	actionBefore := nodeMtime(action)

	// Reload with an identical file set — nothing added or removed.
	if err := root.Reload([]dedup.Config{
		{Name: "Movies/Action/a.mkv", DedupFile: "/data/a.dedup", SourceDir: "/src"},
	}, nil); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	if !nodeMtime(action).Equal(actionBefore) {
		t.Errorf("Movies/Action mtime changed on no-op reload: %v, want %v", nodeMtime(action), actionBefore)
	}
	if !nodeMtime(movies).Equal(moviesBefore) {
		t.Errorf("Movies mtime changed on no-op reload: %v, want %v", nodeMtime(movies), moviesBefore)
	}
	if !nodeMtime(root.rootDir).Equal(rootBefore) {
		t.Errorf("root mtime changed on no-op reload: %v, want %v", nodeMtime(root.rootDir), rootBefore)
	}
}

func TestMKVFSRoot_Reload_DirMtime_NewDirectorySubtreeStamped(t *testing.T) {
	root, _, _ := reloadDirFixture(t)
	rootBefore := nodeMtime(root.rootDir)

	if err := root.Reload([]dedup.Config{
		{Name: "Movies/Action/a.mkv", DedupFile: "/data/a.dedup", SourceDir: "/src"},
		{Name: "New/Deep/b.mkv", DedupFile: "/data/b.dedup", SourceDir: "/src"},
	}, nil); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	// Root directly gained "New", so root advances.
	if !nodeMtime(root.rootDir).After(rootBefore) {
		t.Errorf("root mtime did not advance after gaining a subdirectory (%v)", nodeMtime(root.rootDir))
	}
	newDir := root.rootDir.subdirs["New"]
	if newDir == nil {
		t.Fatal("New directory not created")
	}
	deep := newDir.subdirs["Deep"]
	if deep == nil {
		t.Fatal("New/Deep directory not created")
	}
	// The whole newly created subtree is stamped, not left at mount time.
	for name, d := range map[string]*MKVFSDirNode{"New": newDir, "New/Deep": deep} {
		if !nodeMtime(d).After(rootBefore) {
			t.Errorf("%s mtime = %v, want later than mount time %v", name, nodeMtime(d), rootBefore)
		}
	}
}

// --- Clearing an override must not leave an empty entry ---

func TestPermissionStore_ClearMtime_RemovesEmptyEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "permissions.yaml")
	store := NewPermissionStore(path, DefaultPerms(), false)

	mtime := int64(1_600_000_000)
	if err := store.SetFileMtime("v.mkv", &mtime); err != nil {
		t.Fatalf("SetFileMtime: %v", err)
	}
	if err := store.SetFileMtime("v.mkv", nil); err != nil {
		t.Fatalf("clear SetFileMtime: %v", err)
	}
	flush(t, store)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read permissions: %v", err)
	}
	if strings.Contains(string(data), "v.mkv") {
		t.Errorf("cleared entry still persisted:\n%s", data)
	}
}

func TestPermissionStore_ClearMtime_KeepsEntryWithPerms(t *testing.T) {
	store := NewPermissionStore("", DefaultPerms(), false)

	mode := uint32(0640)
	if err := store.SetFilePerms("v.mkv", nil, nil, &mode); err != nil {
		t.Fatalf("SetFilePerms: %v", err)
	}
	mtime := int64(1_600_000_000)
	if err := store.SetFileMtime("v.mkv", &mtime); err != nil {
		t.Fatalf("SetFileMtime: %v", err)
	}
	if err := store.SetFileMtime("v.mkv", nil); err != nil {
		t.Fatalf("clear SetFileMtime: %v", err)
	}

	// mtime gone, but the mode override must survive.
	if got := store.GetFileMtimeOverride("v.mkv"); got != nil {
		t.Errorf("mtime override = %v, want nil", got)
	}
	if _, _, gotMode := store.GetFilePerms("v.mkv"); gotMode != mode {
		t.Errorf("mode = %o, want %o (entry must not be dropped)", gotMode, mode)
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
