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
	"github.com/stuckj/mkvdup/internal/dedup"
	"github.com/stuckj/mkvdup/internal/source"
)

// MKVFile represents a virtual MKV file backed by a dedup file.
type MKVFile struct {
	Name      string
	DedupPath string
	SourceDir string
	Size      int64
	reader    *dedup.Reader
	index     *source.Index
	mu        sync.RWMutex
}

// MKVFSRoot is the root node of the FUSE filesystem.
type MKVFSRoot struct {
	fs.Inode
	files   map[string]*MKVFile
	mu      sync.RWMutex
	verbose bool
}

// MKVFSNode represents a file node in the FUSE filesystem.
type MKVFSNode struct {
	fs.Inode
	file    *MKVFile
	verbose bool
}

// Ensure interfaces are implemented
var _ fs.InodeEmbedder = (*MKVFSRoot)(nil)
var _ fs.InodeEmbedder = (*MKVFSNode)(nil)
var _ fs.NodeReaddirer = (*MKVFSRoot)(nil)
var _ fs.NodeLookuper = (*MKVFSRoot)(nil)
var _ fs.NodeOpener = (*MKVFSNode)(nil)
var _ fs.NodeReader = (*MKVFSNode)(nil)
var _ fs.NodeGetattrer = (*MKVFSNode)(nil)

// NewMKVFS creates a new MKVFS root from a list of config files.
// Set verbose=true to enable debug logging.
func NewMKVFS(configPaths []string, verbose bool) (*MKVFSRoot, error) {
	root := &MKVFSRoot{
		files:   make(map[string]*MKVFile),
		verbose: verbose,
	}

	if verbose {
		log.Printf("Creating MKVFS with %d config files", len(configPaths))
	}

	for _, configPath := range configPaths {
		if verbose {
			log.Printf("Reading config: %s", configPath)
		}
		config, err := dedup.ReadConfig(configPath)
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
		reader, err := dedup.NewReaderLazy(dedupPath, sourceDir)
		if err != nil {
			if verbose {
				log.Printf("Failed to open dedup file: %v", err)
			}
			return nil, fmt.Errorf("open dedup file %s: %w", dedupPath, err)
		}

		mkvFile := &MKVFile{
			Name:      config.Name,
			DedupPath: dedupPath,
			SourceDir: sourceDir,
			Size:      reader.OriginalSize(),
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

	return root, nil
}

// Readdir implements fs.NodeReaddirer - lists files in the root directory.
func (r *MKVFSRoot) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.verbose {
		log.Printf("Readdir: listing %d files", len(r.files))
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

// Lookup implements fs.NodeLookuper - looks up a file by name.
func (r *MKVFSRoot) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
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

	// Open dedup file with lazy loading
	reader, err := dedup.NewReaderLazy(n.file.DedupPath, n.file.SourceDir)
	if err != nil {
		return fmt.Errorf("open dedup file: %w", err)
	}

	// Check if this is an ES-based source
	if reader.UsesESOffsets() {
		// Create indexer to get ES reader
		indexer, err := source.NewIndexer(n.file.SourceDir, source.DefaultWindowSize)
		if err != nil {
			reader.Close()
			return fmt.Errorf("create indexer: %w", err)
		}
		if err := indexer.Build(nil); err != nil {
			reader.Close()
			return fmt.Errorf("build index: %w", err)
		}
		n.file.index = indexer.Index()

		if len(n.file.index.ESReaders) > 0 {
			reader.SetESReader(n.file.index.ESReaders[0])
		}
	} else {
		// Load source files for raw access
		if err := reader.LoadSourceFiles(); err != nil {
			reader.Close()
			return fmt.Errorf("load source files: %w", err)
		}
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
	if f.index != nil {
		f.index.Close()
		f.index = nil
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
