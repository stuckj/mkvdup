// Package fuse provides a FUSE filesystem for accessing deduplicated MKV files.
package fuse

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/stuckj/mkvdup/internal/dedup"
)

// MKVFile represents a virtual MKV file backed by a dedup file.
type MKVFile struct {
	Name      string
	DedupPath string
	SourceDir string
	Size      int64
	reader    DedupReader
	mu        sync.RWMutex

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

// NewMKVFS creates a new MKVFS root from a list of config files.
// Config files are resolved recursively (includes and virtual_files are expanded).
// Set verbose=true to enable debug logging.
func NewMKVFS(configPaths []string, verbose bool) (*MKVFSRoot, error) {
	configs, err := dedup.ResolveConfigs(configPaths)
	if err != nil {
		return nil, fmt.Errorf("resolve configs: %w", err)
	}
	return NewMKVFSFromConfigs(configs, verbose, &DefaultReaderFactory{}, nil)
}

// NewMKVFSWithPermissions creates a new MKVFS root with a permission store.
// Config files are resolved recursively (includes and virtual_files are expanded).
func NewMKVFSWithPermissions(configPaths []string, verbose bool, permStore *PermissionStore) (*MKVFSRoot, error) {
	configs, err := dedup.ResolveConfigs(configPaths)
	if err != nil {
		return nil, fmt.Errorf("resolve configs: %w", err)
	}
	return NewMKVFSFromConfigs(configs, verbose, &DefaultReaderFactory{}, permStore)
}

// MKVFSOptions contains options for creating an MKVFS filesystem.
type MKVFSOptions struct {
	Verbose         bool
	PermissionsPath string
	// Defaults holds the default permissions to use when a PermissionStore is configured.
	// If nil, DefaultPerms() is used. Set to a non-nil value to use specific defaults.
	// Note: explicit-zero defaults only work when provided programmatically here;
	// they are not persisted to or loaded from the permissions YAML file.
	Defaults *Defaults
}

// NewMKVFSWithOptions creates a new MKVFS root with the given options.
// Config files are resolved recursively (includes and virtual_files are expanded).
func NewMKVFSWithOptions(configPaths []string, opts MKVFSOptions) (*MKVFSRoot, error) {
	var permStore *PermissionStore
	if opts.PermissionsPath != "" {
		defaults := DefaultPerms()
		if opts.Defaults != nil {
			defaults = *opts.Defaults
		}
		permStore = NewPermissionStore(opts.PermissionsPath, defaults, opts.Verbose)
		if err := permStore.Load(); err != nil {
			return nil, fmt.Errorf("load permissions: %w", err)
		}
	}
	configs, err := dedup.ResolveConfigs(configPaths)
	if err != nil {
		return nil, fmt.Errorf("resolve configs: %w", err)
	}
	return NewMKVFSFromConfigs(configs, opts.Verbose, &DefaultReaderFactory{}, permStore)
}

// NewMKVFSWithFactories creates a new MKVFS root with custom factories.
// This allows injecting mock implementations for testing.
func NewMKVFSWithFactories(configPaths []string, verbose bool, readerFactory ReaderFactory, configReader ConfigReader, permStore *PermissionStore) (*MKVFSRoot, error) {
	root := &MKVFSRoot{
		files:         make(map[string]*MKVFile),
		verbose:       verbose,
		readerFactory: readerFactory,
		configReader:  configReader,
		permStore:     permStore,
	}

	if verbose {
		log.Printf("Creating MKVFS with %d config files", len(configPaths))
	}

	for _, configPath := range configPaths {
		if verbose {
			log.Printf("Reading config: %s", configPath)
		}
		config, err := root.configReader.ReadConfig(configPath)
		if err != nil {
			return nil, fmt.Errorf("read config %s: %w", configPath, err)
		}
		if verbose {
			log.Printf("Config: name=%s, dedup=%s, source=%s", config.Name, config.DedupFile, config.SourceDir)
		}

		// Resolve relative paths
		configDir := filepath.Dir(configPath)
		dedupPath := config.DedupFile
		if !filepath.IsAbs(dedupPath) {
			dedupPath = filepath.Join(configDir, dedupPath)
		}
		sourceDir := config.SourceDir
		if !filepath.IsAbs(sourceDir) {
			sourceDir = filepath.Join(configDir, sourceDir)
		}

		// Open dedup file to get size (lazy loading - only reads header)
		if verbose {
			log.Printf("Opening dedup file: %s", dedupPath)
		}
		reader, err := root.readerFactory.NewReaderLazy(dedupPath, sourceDir)
		if err != nil {
			if verbose {
				log.Printf("Failed to open dedup file: %v", err)
			}
			return nil, fmt.Errorf("open dedup file %s: %w", dedupPath, err)
		}

		mkvFile := &MKVFile{
			Name:          config.Name,
			DedupPath:     dedupPath,
			SourceDir:     sourceDir,
			Size:          reader.OriginalSize(),
			readerFactory: root.readerFactory,
		}

		// Don't keep reader open - we'll open it lazily
		reader.Close()

		root.files[config.Name] = mkvFile
		if verbose {
			log.Printf("Added file: %s (size=%d)", config.Name, mkvFile.Size)
		}
	}

	if verbose {
		log.Printf("Total files: %d", len(root.files))
	}

	// Build directory tree from collected files
	fileList := make([]*MKVFile, 0, len(root.files))
	for _, f := range root.files {
		fileList = append(fileList, f)
	}
	root.rootDir = BuildDirectoryTree(fileList, verbose, readerFactory, permStore)

	// Clean up stale permission entries if we have a permission store
	if permStore != nil {
		validFiles, validDirs := root.collectValidPaths()
		removed := permStore.CleanupStale(validFiles, validDirs)
		if removed > 0 {
			if verbose {
				log.Printf("Cleaned up %d stale permission entries", removed)
			}
			if err := permStore.Save(); err != nil {
				log.Printf("Warning: failed to save permissions after cleanup: %v", err)
			}
		}
	}

	if verbose {
		log.Printf("Directory tree built with %d root entries", len(root.rootDir.files)+len(root.rootDir.subdirs))
	}

	return root, nil
}

// NewMKVFSFromConfigs creates a new MKVFS root from already-resolved configs.
// Paths in configs must already be absolute (as returned by dedup.ResolveConfigs).
func NewMKVFSFromConfigs(configs []dedup.Config, verbose bool, readerFactory ReaderFactory, permStore *PermissionStore) (*MKVFSRoot, error) {
	root := &MKVFSRoot{
		files:         make(map[string]*MKVFile),
		verbose:       verbose,
		readerFactory: readerFactory,
		permStore:     permStore,
	}

	if verbose {
		log.Printf("Creating MKVFS with %d resolved configs", len(configs))
	}

	for _, config := range configs {
		if verbose {
			log.Printf("Config: name=%s, dedup=%s, source=%s", config.Name, config.DedupFile, config.SourceDir)
		}

		// Open dedup file to get size (lazy loading - only reads header)
		if verbose {
			log.Printf("Opening dedup file: %s", config.DedupFile)
		}
		reader, err := root.readerFactory.NewReaderLazy(config.DedupFile, config.SourceDir)
		if err != nil {
			if verbose {
				log.Printf("Failed to open dedup file: %v", err)
			}
			return nil, fmt.Errorf("open dedup file %s: %w", config.DedupFile, err)
		}

		mkvFile := &MKVFile{
			Name:          config.Name,
			DedupPath:     config.DedupFile,
			SourceDir:     config.SourceDir,
			Size:          reader.OriginalSize(),
			readerFactory: root.readerFactory,
		}

		// Don't keep reader open - we'll open it lazily
		reader.Close()

		root.files[config.Name] = mkvFile
		if verbose {
			log.Printf("Added file: %s (size=%d)", config.Name, mkvFile.Size)
		}
	}

	if verbose {
		log.Printf("Total files: %d", len(root.files))
	}

	// Build directory tree from collected files
	fileList := make([]*MKVFile, 0, len(root.files))
	for _, f := range root.files {
		fileList = append(fileList, f)
	}
	root.rootDir = BuildDirectoryTree(fileList, verbose, readerFactory, permStore)

	// Clean up stale permission entries if we have a permission store
	if permStore != nil {
		validFiles, validDirs := root.collectValidPaths()
		removed := permStore.CleanupStale(validFiles, validDirs)
		if removed > 0 {
			if verbose {
				log.Printf("Cleaned up %d stale permission entries", removed)
			}
			if err := permStore.Save(); err != nil {
				log.Printf("Warning: failed to save permissions after cleanup: %v", err)
			}
		}
	}

	if verbose {
		log.Printf("Directory tree built with %d root entries", len(root.rootDir.files)+len(root.rootDir.subdirs))
	}

	return root, nil
}

// Reload updates the filesystem with new configs. It merges the new file tree
// into the existing directory tree in-place (required because go-fuse caches
// persistent inode objects by inode number).
//
// Semantics:
//   - New files become immediately visible
//   - Removed files disappear from listings (active readers continue via held refs)
//   - Modified mappings: existing readers use old mapping until close
//   - Permissions are reloaded from disk and stale entries cleaned up
func (r *MKVFSRoot) Reload(configs []dedup.Config, logFn func(string, ...interface{})) error {
	if logFn == nil {
		logFn = func(string, ...interface{}) {}
	}

	// Build new file set from configs
	newFiles := make(map[string]*MKVFile)
	for _, config := range configs {
		reader, err := r.readerFactory.NewReaderLazy(config.DedupFile, config.SourceDir)
		if err != nil {
			logFn("warning: skipping %s: %v", config.Name, err)
			continue
		}

		mkvFile := &MKVFile{
			Name:          config.Name,
			DedupPath:     config.DedupFile,
			SourceDir:     config.SourceDir,
			Size:          reader.OriginalSize(),
			readerFactory: r.readerFactory,
		}
		reader.Close()
		newFiles[config.Name] = mkvFile
	}

	// Build new directory tree
	fileList := make([]*MKVFile, 0, len(newFiles))
	for _, f := range newFiles {
		fileList = append(fileList, f)
	}
	newTree := BuildDirectoryTree(fileList, r.verbose, r.readerFactory, r.permStore)

	// Update flat files map
	r.mu.Lock()
	r.files = newFiles
	r.mu.Unlock()

	// Merge new tree into existing tree in place
	mergeDirectoryTree(r.rootDir, newTree)

	// Reload permissions and clean up stale entries
	if r.permStore != nil {
		if err := r.permStore.Load(); err != nil {
			logFn("warning: failed to reload permissions: %v", err)
		}
		validFiles, validDirs := r.collectValidPaths()
		removed := r.permStore.CleanupStale(validFiles, validDirs)
		if removed > 0 {
			logFn("cleaned up %d stale permission entries", removed)
			if err := r.permStore.Save(); err != nil {
				logFn("warning: failed to save permissions after cleanup: %v", err)
			}
		}
	}

	logFn("reload complete: %d files", len(newFiles))
	return nil
}

// Getattr implements fs.NodeGetattrer - returns attributes for the root directory.
// This ensures the root directory uses permissions from the permission store,
// consistent with all subdirectories.
func (r *MKVFSRoot) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	r.mu.RLock()
	defer r.mu.RUnlock()

	now := time.Now()

	uid, gid, mode := getDirPerms(r.permStore, "")

	out.Mode = fuse.S_IFDIR | mode
	out.Uid = uid
	out.Gid = gid
	out.Atime = uint64(now.Unix())
	out.Mtime = uint64(now.Unix())
	out.Ctime = uint64(now.Unix())
	out.Nlink = 2
	if r.rootDir != nil {
		out.Nlink += uint32(len(r.rootDir.subdirs))
	}
	return 0
}

// Readdir implements fs.NodeReaddirer - lists files in the root directory.
// Delegates to the directory tree for hierarchical listing.
func (r *MKVFSRoot) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	// Permission checks are handled by the kernel via default_permissions mount option.
	// This properly checks supplementary groups and matches real filesystem behavior.

	if r.rootDir != nil {
		return r.rootDir.readdirInternal(ctx)
	}

	// Fallback to flat listing if no directory tree (shouldn't happen)
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.verbose {
		log.Printf("Readdir: listing %d files (flat)", len(r.files))
	}

	entries := make([]fuse.DirEntry, 0, len(r.files))
	for name := range r.files {
		if r.verbose {
			log.Printf("Readdir: adding %s", name)
		}
		entries = append(entries, fuse.DirEntry{
			Name: name,
			Mode: fuse.S_IFREG,
		})
	}
	return fs.NewListDirStream(entries), 0
}

// Lookup implements fs.NodeLookuper - looks up a file or directory by name.
// Uses the directory tree for hierarchical lookup.
func (r *MKVFSRoot) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	// Permission checks are handled by the kernel via default_permissions mount option.

	if r.rootDir != nil {
		r.rootDir.mu.RLock()
		defer r.rootDir.mu.RUnlock()

		// Check subdirectories first
		if subdir, ok := r.rootDir.subdirs[name]; ok {
			if r.verbose {
				log.Printf("Lookup: found subdir %s at root", name)
			}

			// Lock subdir to safely access its fields
			subdir.mu.RLock()
			subdirCount := len(subdir.subdirs)
			subdir.mu.RUnlock()

			uid, gid, mode := getDirPerms(r.permStore, subdir.path)

			now := time.Now()
			out.Mode = fuse.S_IFDIR | mode
			out.Uid = uid
			out.Gid = gid
			out.Atime = uint64(now.Unix())
			out.Mtime = uint64(now.Unix())
			out.Ctime = uint64(now.Unix())
			out.Nlink = 2 + uint32(subdirCount)

			stable := fs.StableAttr{
				Mode: fuse.S_IFDIR,
				Ino:  hashString(subdir.path),
			}
			child := r.NewPersistentInode(ctx, subdir, stable)
			return child, 0
		}

		// Check files
		if file, ok := r.rootDir.files[name]; ok {
			if r.verbose {
				log.Printf("Lookup: found file %s at root (size=%d)", name, file.Size)
			}

			uid, gid, mode := getFilePerms(r.permStore, name)

			now := time.Now()
			out.Size = uint64(file.Size)
			out.Mode = fuse.S_IFREG | mode
			out.Uid = uid
			out.Gid = gid
			out.Atime = uint64(now.Unix())
			out.Mtime = uint64(now.Unix())
			out.Ctime = uint64(now.Unix())
			out.Nlink = 1

			node := &MKVFSNode{file: file, path: name, verbose: r.verbose, permStore: r.permStore}
			stable := fs.StableAttr{
				Mode: fuse.S_IFREG,
				Ino:  hashString(name),
			}
			child := r.NewInode(ctx, node, stable)
			return child, 0
		}

		if r.verbose {
			log.Printf("Lookup: not found %s at root", name)
		}
		return nil, syscall.ENOENT
	}

	// Fallback to flat lookup if no directory tree (shouldn't happen)
	r.mu.RLock()
	file, ok := r.files[name]
	r.mu.RUnlock()

	if !ok {
		if r.verbose {
			log.Printf("Lookup: file not found: %s", name)
		}
		return nil, syscall.ENOENT
	}

	if r.verbose {
		log.Printf("Lookup: %s (size=%d)", name, file.Size)
	}

	uid, gid, mode := getFilePerms(r.permStore, name)

	// Create a new file node
	node := &MKVFSNode{file: file, path: name, verbose: r.verbose, permStore: r.permStore}

	// Set attributes
	now := time.Now()
	out.Size = uint64(file.Size)
	out.Mode = fuse.S_IFREG | mode
	out.Uid = uid
	out.Gid = gid
	out.Atime = uint64(now.Unix())
	out.Mtime = uint64(now.Unix())
	out.Ctime = uint64(now.Unix())

	// Create inode with stable ID based on filename
	stable := fs.StableAttr{
		Mode: fuse.S_IFREG,
		Ino:  hashString(name),
	}

	child := r.NewInode(ctx, node, stable)
	return child, 0
}

// Getattr implements fs.NodeGetattrer - returns file attributes.
func (n *MKVFSNode) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	now := time.Now()
	out.Size = uint64(n.file.Size)

	uid, gid, mode := getFilePerms(n.permStore, n.path)

	out.Mode = fuse.S_IFREG | mode
	out.Uid = uid
	out.Gid = gid
	out.Atime = uint64(now.Unix())
	out.Mtime = uint64(now.Unix())
	out.Ctime = uint64(now.Unix())
	out.Nlink = 1
	return 0
}

// Setattr implements fs.NodeSetattrer - handles chmod/chown on files.
func (n *MKVFSNode) Setattr(ctx context.Context, fh fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	if n.permStore == nil {
		// No permission store - can't change permissions
		return syscall.EROFS
	}

	// Only UID, GID, and mode changes are supported. All other setattr operations
	// (e.g. size truncation, atime/mtime updates) must fail on this read-only FS.
	supportedMask := uint32(fuse.FATTR_UID | fuse.FATTR_GID | fuse.FATTR_MODE)
	if in.Valid&^supportedMask != 0 {
		return syscall.EROFS
	}

	// Get current permissions and caller
	fileUID, fileGID, fileMode := getFilePerms(n.permStore, n.path)
	caller, ok := GetCaller(ctx)
	if !ok {
		return syscall.EACCES
	}

	var newUID, newGID, newMode *uint32

	// Check which fields are being changed
	if in.Valid&fuse.FATTR_UID != 0 {
		newUID = &in.Uid
	}
	if in.Valid&fuse.FATTR_GID != 0 {
		newGID = &in.Gid
	}
	if in.Valid&fuse.FATTR_MODE != 0 {
		mode := in.Mode & 0777 // Only permission bits
		newMode = &mode
	}

	// Normalize no-op changes to nil to avoid unnecessary disk writes
	if newUID != nil && *newUID == fileUID {
		newUID = nil
	}
	if newGID != nil && *newGID == fileGID {
		newGID = nil
	}
	if newMode != nil && *newMode == fileMode {
		newMode = nil
	}

	// Permission checks for chown
	if newUID != nil || newGID != nil {
		if errno := CheckChown(caller, fileUID, fileGID, newUID, newGID); errno != 0 {
			if n.verbose {
				log.Printf("Setattr: chown permission denied for %s (caller uid=%d)", n.path, caller.Uid)
			}
			return errno
		}
	}

	// Permission checks for chmod
	if newMode != nil {
		if errno := CheckChmod(caller, fileUID); errno != 0 {
			if n.verbose {
				log.Printf("Setattr: chmod permission denied for %s (caller uid=%d)", n.path, caller.Uid)
			}
			return errno
		}
	}

	// Update permission store
	if err := n.permStore.SetFilePerms(n.path, newUID, newGID, newMode); err != nil {
		if n.verbose {
			log.Printf("Setattr error: %s: %v", n.path, err)
		}
		return syscall.EIO
	}

	if n.verbose {
		log.Printf("Setattr: %s uid=%v gid=%v mode=%v", n.path, newUID, newGID, newMode)
	}

	// Return updated attributes
	return n.Getattr(ctx, fh, out)
}

// Open implements fs.NodeOpener - opens a file for reading.
func (n *MKVFSNode) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	// This is a read-only filesystem - reject any write access or operations
	// that would modify the filesystem. Note: O_RDONLY|O_APPEND is a valid
	// read-only open on Linux (positions at EOF), so we only check access mode.
	accMode := flags & syscall.O_ACCMODE
	if accMode != syscall.O_RDONLY || flags&(syscall.O_TRUNC|syscall.O_CREAT) != 0 {
		return nil, 0, syscall.EROFS
	}

	// Permission checks are handled by the kernel via default_permissions mount option.

	if n.verbose {
		log.Printf("Open: %s", n.file.Name)
	}
	// Initialize reader lazily if needed
	if err := n.ensureReader(); err != nil {
		if n.verbose {
			log.Printf("Open error: %s: %v", n.file.Name, err)
		}
		return nil, 0, syscall.EIO
	}
	return nil, fuse.FOPEN_KEEP_CACHE | fuse.FOPEN_CACHE_DIR, 0
}

// Read implements fs.NodeReader - reads data from the file.
func (n *MKVFSNode) Read(ctx context.Context, fh fs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	// Permission checks are handled by the kernel via default_permissions mount option.

	n.file.mu.RLock()
	defer n.file.mu.RUnlock()

	if n.file.reader == nil {
		// Reader not initialized
		if n.verbose {
			log.Printf("Read error: %s: reader not initialized", n.file.Name)
		}
		return nil, syscall.EIO
	}

	// Clamp read to file size
	if off >= n.file.Size {
		return fuse.ReadResultData(nil), 0
	}

	endOff := off + int64(len(dest))
	if endOff > n.file.Size {
		dest = dest[:n.file.Size-off]
	}

	// Read from dedup reader
	nRead, err := n.file.reader.ReadAt(dest, off)
	if err != nil && nRead == 0 {
		if n.verbose {
			log.Printf("Read error: %s at offset %d: %v", n.file.Name, off, err)
		}
		return nil, syscall.EIO
	}

	if n.verbose {
		log.Printf("Read: %s offset=%d len=%d read=%d", n.file.Name, off, len(dest), nRead)
	}

	return fuse.ReadResultData(dest[:nRead]), 0
}

// ensureReader ensures the dedup reader is initialized.
func (n *MKVFSNode) ensureReader() error {
	n.file.mu.Lock()
	defer n.file.mu.Unlock()

	if n.file.reader != nil {
		return nil
	}

	// Open dedup file with lazy loading using the factory
	reader, err := n.file.readerFactory.NewReaderLazy(n.file.DedupPath, n.file.SourceDir)
	if err != nil {
		return fmt.Errorf("open dedup file: %w", err)
	}

	// Initialize the reader for reading (handles ES vs raw internally)
	if err := reader.InitializeForReading(n.file.SourceDir); err != nil {
		reader.Close()
		return fmt.Errorf("initialize reader: %w", err)
	}

	n.file.reader = reader
	return nil
}

// Close cleans up the file's resources.
func (f *MKVFile) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.reader != nil {
		f.reader.Close()
		f.reader = nil
	}
}

// hashString creates a stable inode number from a string.
func hashString(s string) uint64 {
	var h uint64 = 5381
	for _, c := range s {
		h = ((h << 5) + h) + uint64(c)
	}
	return h
}

// collectValidPaths returns maps of all valid file and directory paths.
func (r *MKVFSRoot) collectValidPaths() (files, dirs map[string]bool) {
	files = make(map[string]bool)
	dirs = make(map[string]bool)

	if r.rootDir == nil {
		return files, dirs
	}

	r.collectPathsRecursive(r.rootDir, files, dirs)
	return files, dirs
}

func (r *MKVFSRoot) collectPathsRecursive(node *MKVFSDirNode, files, dirs map[string]bool) {
	node.mu.RLock()
	defer node.mu.RUnlock()

	// Add this directory (including root with empty path)
	dirs[node.path] = true

	// Add files
	for name := range node.files {
		var filePath string
		if node.path == "" {
			filePath = name
		} else {
			filePath = node.path + "/" + name
		}
		files[filePath] = true
	}

	// Recurse into subdirectories
	for _, subdir := range node.subdirs {
		r.collectPathsRecursive(subdir, files, dirs)
	}
}

// --- MKVFSDirNode interface implementations ---

// Readdir implements fs.NodeReaddirer - lists files and subdirectories.
func (d *MKVFSDirNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	// Permission checks are handled by the kernel via default_permissions mount option.
	return d.readdirInternal(ctx)
}

// readdirInternal performs the directory listing. It does not perform any permission
// checks itself (those are handled by the kernel via default_permissions) and is
// shared by both MKVFSRoot.Readdir and MKVFSDirNode.Readdir.
func (d *MKVFSDirNode) readdirInternal(ctx context.Context) (fs.DirStream, syscall.Errno) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.verbose {
		log.Printf("Readdir: %s (files=%d, subdirs=%d)", d.path, len(d.files), len(d.subdirs))
	}

	entries := make([]fuse.DirEntry, 0, len(d.files)+len(d.subdirs))

	// Collect and sort subdirectory names for deterministic ordering
	subdirNames := make([]string, 0, len(d.subdirs))
	for name := range d.subdirs {
		subdirNames = append(subdirNames, name)
	}
	sort.Strings(subdirNames)

	// Add subdirectories first (sorted)
	for _, name := range subdirNames {
		if d.verbose {
			log.Printf("Readdir: adding subdir %s", name)
		}
		entries = append(entries, fuse.DirEntry{
			Name: name,
			Mode: fuse.S_IFDIR,
		})
	}

	// Collect and sort file names for deterministic ordering
	fileNames := make([]string, 0, len(d.files))
	for name := range d.files {
		fileNames = append(fileNames, name)
	}
	sort.Strings(fileNames)

	// Add files (sorted)
	for _, name := range fileNames {
		if d.verbose {
			log.Printf("Readdir: adding file %s", name)
		}
		entries = append(entries, fuse.DirEntry{
			Name: name,
			Mode: fuse.S_IFREG,
		})
	}

	return fs.NewListDirStream(entries), 0
}

// Lookup implements fs.NodeLookuper - looks up a file or subdirectory by name.
func (d *MKVFSDirNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	// Permission checks are handled by the kernel via default_permissions mount option.

	d.mu.RLock()
	defer d.mu.RUnlock()

	// Check subdirectories first
	if subdir, ok := d.subdirs[name]; ok {
		if d.verbose {
			log.Printf("Lookup: found subdir %s in %s", name, d.path)
		}

		// Lock subdir to safely access its fields
		subdir.mu.RLock()
		subdirCount := len(subdir.subdirs)
		subdir.mu.RUnlock()

		uid, gid, mode := getDirPerms(d.permStore, subdir.path)

		now := time.Now()
		out.Mode = fuse.S_IFDIR | mode
		out.Uid = uid
		out.Gid = gid
		out.Atime = uint64(now.Unix())
		out.Mtime = uint64(now.Unix())
		out.Ctime = uint64(now.Unix())
		out.Nlink = 2 + uint32(subdirCount)

		stable := fs.StableAttr{
			Mode: fuse.S_IFDIR,
			Ino:  hashString(subdir.path),
		}
		child := d.NewPersistentInode(ctx, subdir, stable)
		return child, 0
	}

	// Check files
	if file, ok := d.files[name]; ok {
		if d.verbose {
			log.Printf("Lookup: found file %s in %s (size=%d)", name, d.path, file.Size)
		}

		var filePath string
		if d.path == "" {
			filePath = name
		} else {
			filePath = d.path + "/" + name
		}

		uid, gid, mode := getFilePerms(d.permStore, filePath)

		now := time.Now()
		out.Size = uint64(file.Size)
		out.Mode = fuse.S_IFREG | mode
		out.Uid = uid
		out.Gid = gid
		out.Atime = uint64(now.Unix())
		out.Mtime = uint64(now.Unix())
		out.Ctime = uint64(now.Unix())
		out.Nlink = 1

		node := &MKVFSNode{file: file, path: filePath, verbose: d.verbose, permStore: d.permStore}
		stable := fs.StableAttr{
			Mode: fuse.S_IFREG,
			Ino:  hashString(filePath),
		}
		child := d.NewInode(ctx, node, stable)
		return child, 0
	}

	if d.verbose {
		log.Printf("Lookup: not found %s in %s", name, d.path)
	}
	return nil, syscall.ENOENT
}

// Getattr implements fs.NodeGetattrer - returns directory attributes.
func (d *MKVFSDirNode) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	d.mu.RLock()
	defer d.mu.RUnlock()

	now := time.Now()

	uid, gid, mode := getDirPerms(d.permStore, d.path)

	out.Mode = fuse.S_IFDIR | mode
	out.Uid = uid
	out.Gid = gid
	out.Atime = uint64(now.Unix())
	out.Mtime = uint64(now.Unix())
	out.Ctime = uint64(now.Unix())
	out.Nlink = 2 + uint32(len(d.subdirs))
	return 0
}

// Setattr implements fs.NodeSetattrer - handles chmod/chown on directories.
func (d *MKVFSDirNode) Setattr(ctx context.Context, fh fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	if d.permStore == nil {
		// No permission store - can't change permissions
		return syscall.EROFS
	}

	// Only UID, GID, and mode changes are supported. All other setattr operations
	// (e.g. size truncation, atime/mtime updates) must fail on this read-only FS.
	supportedMask := uint32(fuse.FATTR_UID | fuse.FATTR_GID | fuse.FATTR_MODE)
	if in.Valid&^supportedMask != 0 {
		return syscall.EROFS
	}

	// Get current permissions and caller
	dirUID, dirGID, dirMode := getDirPerms(d.permStore, d.path)
	caller, ok := GetCaller(ctx)
	if !ok {
		return syscall.EACCES
	}

	var newUID, newGID, newMode *uint32

	// Check which fields are being changed
	if in.Valid&fuse.FATTR_UID != 0 {
		newUID = &in.Uid
	}
	if in.Valid&fuse.FATTR_GID != 0 {
		newGID = &in.Gid
	}
	if in.Valid&fuse.FATTR_MODE != 0 {
		mode := in.Mode & 0777 // Only permission bits
		newMode = &mode
	}

	// Normalize no-op changes to nil to avoid unnecessary disk writes
	if newUID != nil && *newUID == dirUID {
		newUID = nil
	}
	if newGID != nil && *newGID == dirGID {
		newGID = nil
	}
	if newMode != nil && *newMode == dirMode {
		newMode = nil
	}

	// Permission checks for chown
	if newUID != nil || newGID != nil {
		if errno := CheckChown(caller, dirUID, dirGID, newUID, newGID); errno != 0 {
			if d.verbose {
				log.Printf("Setattr: chown permission denied for %s (caller uid=%d)", d.path, caller.Uid)
			}
			return errno
		}
	}

	// Permission checks for chmod
	if newMode != nil {
		if errno := CheckChmod(caller, dirUID); errno != 0 {
			if d.verbose {
				log.Printf("Setattr: chmod permission denied for %s (caller uid=%d)", d.path, caller.Uid)
			}
			return errno
		}
	}

	// Update permission store
	if err := d.permStore.SetDirPerms(d.path, newUID, newGID, newMode); err != nil {
		if d.verbose {
			log.Printf("Setattr error: %s: %v", d.path, err)
		}
		return syscall.EIO
	}

	if d.verbose {
		log.Printf("Setattr: %s uid=%v gid=%v mode=%v", d.path, newUID, newGID, newMode)
	}

	// Return updated attributes
	return d.Getattr(ctx, fh, out)
}

// --- Read-only filesystem error handlers ---
// These return EROFS (Read-only file system) for write operations.

// Mkdir implements fs.NodeMkdirer - rejects directory creation.
func (d *MKVFSDirNode) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	if d.verbose {
		log.Printf("Mkdir: rejected (read-only) %s in %s", name, d.path)
	}
	return nil, syscall.EROFS
}

// Rmdir implements fs.NodeRmdirer - rejects directory removal.
func (d *MKVFSDirNode) Rmdir(ctx context.Context, name string) syscall.Errno {
	if d.verbose {
		log.Printf("Rmdir: rejected (read-only) %s in %s", name, d.path)
	}
	return syscall.EROFS
}

// Unlink implements fs.NodeUnlinker - rejects file deletion.
func (d *MKVFSDirNode) Unlink(ctx context.Context, name string) syscall.Errno {
	if d.verbose {
		log.Printf("Unlink: rejected (read-only) %s in %s", name, d.path)
	}
	return syscall.EROFS
}

// Create implements fs.NodeCreater - rejects file creation.
func (d *MKVFSDirNode) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (node *fs.Inode, fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	if d.verbose {
		log.Printf("Create: rejected (read-only) %s in %s", name, d.path)
	}
	return nil, nil, 0, syscall.EROFS
}
