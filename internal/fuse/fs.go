// Package fuse provides a FUSE filesystem for accessing deduplicated MKV files.
package fuse

import (
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// fsStartTime is a last-resort fallback timestamp for a directory node that
// somehow carries no mtime of its own (zero value). Real directory mtimes are
// stamped by BuildDirectoryTree at the moment the tree is built — mount time
// for the initial build — because every virtual entry comes into existence
// then. A directory's mtime thereafter reflects when an entry was last added to
// or removed from it; see mergeDirectoryTree.
//
// Note this is captured at package init, which precedes the mount by however
// long startup takes, so it must not be used as the mount baseline itself.
var fsStartTime = time.Now()

// MKVFile represents a virtual MKV file backed by a dedup file.
type MKVFile struct {
	Name      string
	DedupPath string
	SourceDir string
	Size      int64
	reader    DedupReader
	mu        sync.RWMutex

	// disabled is set when a source file change is detected and the
	// configured action is "disable" or "checksum" (with mismatch).
	// When true, Open/Read return EIO. Reset to false on reload.
	disabled bool

	// derivedMtime caches the virtual file's modification time, derived from
	// the dedup (.mkvdup) file's mtime. Computed lazily on first stat and
	// refreshed by the source watcher when the dedup file changes. Guarded by mu.
	derivedMtime time.Time
	derivedSet   bool

	// Factory for lazy initialization (injected from root)
	readerFactory ReaderFactory
}

// statMtime returns the mtime of the file at path, or fsStartTime if it cannot
// be stat'd (a stable fallback so timestamps don't flap on transient errors).
func statMtime(path string) time.Time {
	if info, err := os.Stat(path); err == nil {
		return info.ModTime()
	}
	return fsStartTime
}

// DerivedMtime returns the virtual file's modification time, derived from the
// dedup file's mtime. The value is computed lazily on first call and cached.
func (f *MKVFile) DerivedMtime() time.Time {
	f.mu.RLock()
	if f.derivedSet {
		m := f.derivedMtime
		f.mu.RUnlock()
		return m
	}
	f.mu.RUnlock()

	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.derivedSet {
		f.derivedMtime = statMtime(f.DedupPath)
		f.derivedSet = true
	}
	return f.derivedMtime
}

// RefreshDerivedMtime re-stats the dedup file and updates the cached derived
// mtime. It returns true if the value changed. Used by the source watcher when
// a dedup file's timestamp changes so the new mtime becomes visible.
func (f *MKVFile) RefreshDerivedMtime() bool {
	// Snapshot the path under the lock: this runs on the watcher goroutine and
	// DedupPath is rewritten by updateFrom during a reload, so reading it
	// unlocked would be a data race. The stat itself stays outside the lock.
	f.mu.RLock()
	path := f.DedupPath
	f.mu.RUnlock()

	newMtime := statMtime(path)

	f.mu.Lock()
	defer f.mu.Unlock()

	// A reload may have swapped the dedup file while we were stat'ing. The value
	// we just read belongs to the old file, so discard it rather than caching it
	// against the new path — updateFrom already cleared derivedSet, and the next
	// Getattr or refresh will derive correctly from the new path.
	if f.DedupPath != path {
		return false
	}

	// No value cached yet means nothing has ever been reported to the kernel,
	// so there is nothing to invalidate: record the baseline and report "no
	// change". Without this, the first poll after mount or after a reload
	// (which clears the cache via updateFrom) would claim the mtime changed and
	// trigger a pointless invalidation plus a misleading log line.
	if !f.derivedSet {
		f.derivedMtime = newMtime
		f.derivedSet = true
		return false
	}

	if f.derivedMtime.Equal(newMtime) {
		return false
	}
	f.derivedMtime = newMtime
	return true
}

// MKVFSRoot is the root node of the FUSE filesystem.
type MKVFSRoot struct {
	fs.Inode

	// Directory tree for hierarchical file organization
	rootDir *MKVFSDirNode

	// Flat map for O(1) lookup by full path (kept for backwards compatibility)
	files map[string]*MKVFile

	mu      sync.RWMutex
	verbose bool

	// mounted is set to true after fs.Mount() succeeds. FUSE kernel
	// notifications (NotifyDelete, NotifyEntry, NotifyContent) are only
	// safe to call when the filesystem is mounted — the go-fuse bridge
	// is nil before mount, causing panics.
	mounted atomic.Bool

	// Factories for dependency injection (allows mocking in tests)
	readerFactory ReaderFactory
	configReader  ConfigReader

	// Permission store for chmod/chown support
	permStore *PermissionStore
}

// MKVFSNode represents a file node in the FUSE filesystem.
type MKVFSNode struct {
	fs.Inode
	file      *MKVFile
	path      string // full path for permission lookups
	verbose   bool
	permStore *PermissionStore
}

// MKVFSDirNode represents a directory node in the FUSE filesystem.
type MKVFSDirNode struct {
	fs.Inode
	name    string                   // basename (e.g., "Action")
	path    string                   // full path from root (e.g., "Movies/Action")
	files   map[string]*MKVFile      // files directly in this directory
	subdirs map[string]*MKVFSDirNode // child directories
	mu      sync.RWMutex
	verbose bool

	// mtime is the directory's modification time. It starts at mount time and
	// advances only when a direct child (file or subdirectory) is added or
	// removed, matching POSIX directory semantics. Guarded by mu.
	mtime time.Time

	// Factory for creating file nodes (injected from root)
	readerFactory ReaderFactory

	// Permission store for chmod/chown support
	permStore *PermissionStore
}

// Ensure interfaces are implemented
var _ fs.InodeEmbedder = (*MKVFSRoot)(nil)
var _ fs.InodeEmbedder = (*MKVFSNode)(nil)
var _ fs.InodeEmbedder = (*MKVFSDirNode)(nil)
var _ fs.NodeReaddirer = (*MKVFSRoot)(nil)
var _ fs.NodeLookuper = (*MKVFSRoot)(nil)
var _ fs.NodeGetattrer = (*MKVFSRoot)(nil)
var _ fs.NodeReaddirer = (*MKVFSDirNode)(nil)
var _ fs.NodeLookuper = (*MKVFSDirNode)(nil)
var _ fs.NodeGetattrer = (*MKVFSDirNode)(nil)
var _ fs.NodeMkdirer = (*MKVFSDirNode)(nil)
var _ fs.NodeRmdirer = (*MKVFSDirNode)(nil)
var _ fs.NodeUnlinker = (*MKVFSDirNode)(nil)
var _ fs.NodeCreater = (*MKVFSDirNode)(nil)
var _ fs.NodeOpener = (*MKVFSNode)(nil)
var _ fs.NodeReader = (*MKVFSNode)(nil)
var _ fs.NodeGetattrer = (*MKVFSNode)(nil)
var _ fs.NodeSetattrer = (*MKVFSNode)(nil)
var _ fs.NodeSetattrer = (*MKVFSDirNode)(nil)

// getFilePerms returns file permissions from the store, or defaults if store is nil.
func getFilePerms(store *PermissionStore, path string) (uid, gid, mode uint32) {
	if store != nil {
		return store.GetFilePerms(path)
	}
	return 0, 0, 0444
}

// getDirPerms returns directory permissions from the store, or defaults if store is nil.
func getDirPerms(store *PermissionStore, path string) (uid, gid, mode uint32) {
	if store != nil {
		return store.GetDirPerms(path)
	}
	return 0, 0, 0555
}

// fileTimes returns the (atime, mtime, ctime) in Unix seconds for a virtual
// file. mtime is the dedup file's derived mtime, or the permissions-store
// override if one is set. atime and ctime mirror mtime: atime is deliberately
// not tracked (avoids read-time cost), and ctime has no independent meaning on
// this synthetic filesystem.
func fileTimes(store *PermissionStore, path string, f *MKVFile) (atime, mtime, ctime uint64) {
	m := f.DerivedMtime().Unix()
	if store != nil {
		if override := store.GetFileMtimeOverride(path); override != nil {
			m = *override
		}
	}
	um := uint64(m)
	return um, um, um
}

// dirTimes returns the (atime, mtime, ctime) in Unix seconds for a virtual
// directory. dirMtime is the node's own mtime (mount time, advanced when a
// direct child is added or removed); an explicit permissions-store override
// takes precedence. Callers read dirMtime while holding the node's lock and
// pass it in, since this helper must not acquire node locks itself.
func dirTimes(store *PermissionStore, path string, dirMtime time.Time) (atime, mtime, ctime uint64) {
	if dirMtime.IsZero() {
		dirMtime = fsStartTime
	}
	m := dirMtime.Unix()
	if store != nil {
		if override := store.GetDirMtimeOverride(path); override != nil {
			m = *override
		}
	}
	um := uint64(m)
	return um, um, um
}

// applyTimes sets the atime/mtime/ctime (in seconds) on a fuse.Attr and zeroes
// the nanosecond fields. Shared by all Getattr and Lookup handlers so file and
// directory nodes report timestamps consistently.
func applyTimes(attr *fuse.Attr, atime, mtime, ctime uint64) {
	attr.Atime = atime
	attr.Mtime = mtime
	attr.Ctime = ctime
	attr.Atimensec = 0
	attr.Mtimensec = 0
	attr.Ctimensec = 0
}
