package fuse

import (
	"context"
	"log"
	"sort"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

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
