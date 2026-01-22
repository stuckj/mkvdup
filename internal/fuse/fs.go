// Package fuse provides a FUSE filesystem for accessing deduplicated MKV files.
package fuse

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
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
}

// MKVFSNode represents a file node in the FUSE filesystem.
type MKVFSNode struct {
	fs.Inode
	file    *MKVFile
	verbose bool
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
}

// Ensure interfaces are implemented
var _ fs.InodeEmbedder = (*MKVFSRoot)(nil)
var _ fs.InodeEmbedder = (*MKVFSNode)(nil)
var _ fs.InodeEmbedder = (*MKVFSDirNode)(nil)
var _ fs.NodeReaddirer = (*MKVFSRoot)(nil)
var _ fs.NodeLookuper = (*MKVFSRoot)(nil)
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

// NewMKVFS creates a new MKVFS root from a list of config files.
// Set verbose=true to enable debug logging.
func NewMKVFS(configPaths []string, verbose bool) (*MKVFSRoot, error) {
	return NewMKVFSWithFactories(configPaths, verbose, &DefaultReaderFactory{}, &DefaultConfigReader{})
}

// NewMKVFSWithFactories creates a new MKVFS root with custom factories.
// This allows injecting mock implementations for testing.
func NewMKVFSWithFactories(configPaths []string, verbose bool, readerFactory ReaderFactory, configReader ConfigReader) (*MKVFSRoot, error) {
	root := &MKVFSRoot{
		files:         make(map[string]*MKVFile),
		verbose:       verbose,
		readerFactory: readerFactory,
		configReader:  configReader,
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
	root.rootDir = BuildDirectoryTree(fileList, verbose, readerFactory)

	if verbose {
		log.Printf("Directory tree built with %d root entries", len(root.rootDir.files)+len(root.rootDir.subdirs))
	}

	return root, nil
}

// Readdir implements fs.NodeReaddirer - lists files in the root directory.
// Delegates to the directory tree for hierarchical listing.
func (r *MKVFSRoot) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	if r.rootDir != nil {
		return r.rootDir.Readdir(ctx)
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
	if r.rootDir != nil {
		r.rootDir.mu.RLock()
		defer r.rootDir.mu.RUnlock()

		// Check subdirectories first
		if subdir, ok := r.rootDir.subdirs[name]; ok {
			if r.verbose {
				log.Printf("Lookup: found subdir %s at root", name)
			}

			now := time.Now()
			out.Mode = fuse.S_IFDIR | 0555
			out.Uid = 0
			out.Gid = 0
			out.Atime = uint64(now.Unix())
			out.Mtime = uint64(now.Unix())
			out.Ctime = uint64(now.Unix())
			out.Nlink = 2 + uint32(len(subdir.subdirs))

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

			now := time.Now()
			out.Size = uint64(file.Size)
			out.Mode = fuse.S_IFREG | 0444
			out.Atime = uint64(now.Unix())
			out.Mtime = uint64(now.Unix())
			out.Ctime = uint64(now.Unix())
			out.Nlink = 1

			node := &MKVFSNode{file: file, verbose: r.verbose}
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

	// Create a new file node
	node := &MKVFSNode{file: file, verbose: r.verbose}

	// Set attributes
	now := time.Now()
	out.Size = uint64(file.Size)
	out.Mode = fuse.S_IFREG | 0444
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
	out.Mode = fuse.S_IFREG | 0444
	out.Atime = uint64(now.Unix())
	out.Mtime = uint64(now.Unix())
	out.Ctime = uint64(now.Unix())
	out.Nlink = 1
	return 0
}

// Open implements fs.NodeOpener - opens a file for reading.
func (n *MKVFSNode) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
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

// --- MKVFSDirNode interface implementations ---

// Readdir implements fs.NodeReaddirer - lists files and subdirectories.
func (d *MKVFSDirNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.verbose {
		log.Printf("Readdir: %s (files=%d, subdirs=%d)", d.path, len(d.files), len(d.subdirs))
	}

	entries := make([]fuse.DirEntry, 0, len(d.files)+len(d.subdirs))

	// Add subdirectories first
	for name := range d.subdirs {
		if d.verbose {
			log.Printf("Readdir: adding subdir %s", name)
		}
		entries = append(entries, fuse.DirEntry{
			Name: name,
			Mode: fuse.S_IFDIR,
		})
	}

	// Add files
	for name := range d.files {
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
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Check subdirectories first
	if subdir, ok := d.subdirs[name]; ok {
		if d.verbose {
			log.Printf("Lookup: found subdir %s in %s", name, d.path)
		}

		now := time.Now()
		out.Mode = fuse.S_IFDIR | 0555
		out.Uid = 0
		out.Gid = 0
		out.Atime = uint64(now.Unix())
		out.Mtime = uint64(now.Unix())
		out.Ctime = uint64(now.Unix())
		out.Nlink = 2 + uint32(len(subdir.subdirs))

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

		now := time.Now()
		out.Size = uint64(file.Size)
		out.Mode = fuse.S_IFREG | 0444
		out.Atime = uint64(now.Unix())
		out.Mtime = uint64(now.Unix())
		out.Ctime = uint64(now.Unix())
		out.Nlink = 1

		node := &MKVFSNode{file: file, verbose: d.verbose}
		var inodePath string
		if d.path == "" {
			inodePath = name
		} else {
			inodePath = d.path + "/" + name
		}
		stable := fs.StableAttr{
			Mode: fuse.S_IFREG,
			Ino:  hashString(inodePath),
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
	out.Mode = fuse.S_IFDIR | 0555
	out.Uid = 0
	out.Gid = 0
	out.Atime = uint64(now.Unix())
	out.Mtime = uint64(now.Unix())
	out.Ctime = uint64(now.Unix())
	out.Nlink = 2 + uint32(len(d.subdirs))
	return 0
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
