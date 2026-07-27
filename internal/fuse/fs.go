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

// fsStartTime is a stable reference timestamp used for virtual directories,
// which have no backing file on disk. It is captured once at process start so
// directory timestamps don't change on every stat.
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
	newMtime := statMtime(f.DedupPath) // stat without holding the lock

	f.mu.Lock()
	defer f.mu.Unlock()
	if f.derivedSet && f.derivedMtime.Equal(newMtime) {
		return false
	}
	f.derivedMtime = newMtime
	f.derivedSet = true
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
// directory. Directories have no backing file, so they default to a stable
// reference time (fsStartTime), overridable via the permissions store.
func dirTimes(store *PermissionStore, path string) (atime, mtime, ctime uint64) {
	m := fsStartTime.Unix()
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
