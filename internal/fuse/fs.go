// Package fuse provides a FUSE filesystem for accessing deduplicated MKV files.
package fuse

import (
	"sync"
	"sync/atomic"

	"github.com/hanwen/go-fuse/v2/fs"
)

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

	// Factory for lazy initialization (injected from root)
	readerFactory ReaderFactory
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
