package fuse

import (
	"context"
	"fmt"
	"log"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

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

	// Check if file was disabled due to source file change
	n.file.mu.RLock()
	disabled := n.file.disabled
	n.file.mu.RUnlock()
	if disabled {
		if n.verbose {
			log.Printf("Open: %s: source file changed, file disabled", n.file.Name)
		}
		return nil, 0, syscall.EIO
	}

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

	if n.file.disabled {
		if n.verbose {
			log.Printf("Read error: %s: source file changed, file disabled", n.file.Name)
		}
		return nil, syscall.EIO
	}

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

// Disable marks the file as disabled (source changed). Subsequent reads
// return EIO. Closes any active reader. Thread-safe.
func (f *MKVFile) Disable() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.disabled = true
	if f.reader != nil {
		f.reader.Close()
		f.reader = nil
	}
}

// Enable re-enables a previously disabled file (e.g., after checksum
// verification confirms the source is OK). The reader will be lazily
// re-initialized on next Open.
func (f *MKVFile) Enable() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.disabled = false
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

// updateFrom copies data fields from src into f. If the underlying dedup file
// changed, any active reader is closed since it's no longer valid.
// The caller must hold f.mu (write lock).
func (f *MKVFile) updateFrom(src *MKVFile) {
	// Close reader if the underlying file changed — it's no longer valid
	if f.reader != nil && (f.DedupPath != src.DedupPath || f.SourceDir != src.SourceDir) {
		f.reader.Close()
		f.reader = nil
	}
	f.Name = src.Name
	f.DedupPath = src.DedupPath
	f.SourceDir = src.SourceDir
	f.Size = src.Size
	f.readerFactory = src.readerFactory
	// Reset disabled flag — reload re-validates source files
	f.disabled = false
}

// hashString creates a stable inode number from a string.
func hashString(s string) uint64 {
	var h uint64 = 5381
	for _, c := range s {
		h = ((h << 5) + h) + uint64(c)
	}
	return h
}
