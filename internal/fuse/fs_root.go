package fuse

import (
	"context"
	"log"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/stuckj/mkvdup/internal/dedup"
)

// reloadNotification captures a pending FUSE kernel notification to emit
// after all locks are released (go-fuse notifications must not be called
// while holding filesystem locks, as the kernel may call back into the FS).
type reloadNotification struct {
	parent   *fs.Inode
	child    *fs.Inode // non-nil for deletions (if kernel had cached the inode)
	name     string
	isDelete bool
}

// findParentInode walks the directory tree to find the parent inode for a
// given file path (e.g., "Movies/Action/film.mkv"). Returns the parent's
// go-fuse Inode and the basename, or (nil, "") if the parent directory
// doesn't exist in the tree.
//
// For root-level files (no directory component), returns r.Inode.
// Caller must NOT hold directory locks — this method acquires them.
func (r *MKVFSRoot) findParentInode(filePath string) (*fs.Inode, string) {
	cleaned := path.Clean(filePath)
	parts := strings.Split(cleaned, "/")
	// Filter empty parts (handles leading slashes)
	valid := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" && p != "." {
			valid = append(valid, p)
		}
	}
	if len(valid) == 0 {
		return nil, ""
	}

	basename := valid[len(valid)-1]
	dirParts := valid[:len(valid)-1]

	if len(dirParts) == 0 {
		// File is at root level — parent is the root inode
		return &r.Inode, basename
	}

	// Walk directory tree to find parent
	current := r.rootDir
	for _, part := range dirParts {
		current.mu.RLock()
		subdir, ok := current.subdirs[part]
		current.mu.RUnlock()
		if !ok {
			return nil, ""
		}
		current = subdir
	}

	// Newly created directories from mergeDirectoryTree have uninitialized
	// fs.Inode (never registered with go-fuse via NewPersistentInode).
	// The kernel doesn't know about them, so notifications would panic.
	// Return nil — the kernel will discover the directory via Lookup.
	if current.Inode.StableAttr().Ino == 0 {
		return nil, ""
	}

	return &current.Inode, basename
}

// markAncestorDirs walks from inode up to (and including) the root,
// adding each ancestor to changedDirs so their readdir caches are
// invalidated. This is necessary because a file addition or removal
// in a deeply nested virtual directory may cause intermediate
// directories to be created or removed by the tree merge.
func markAncestorDirs(inode *fs.Inode, changedDirs map[*fs.Inode]bool) {
	for node := inode; ; {
		_, ancestor := node.Parent()
		if ancestor == nil {
			break
		}
		if changedDirs[ancestor] {
			break // already marked — ancestors above must be too
		}
		changedDirs[ancestor] = true
		node = ancestor
	}
}

// Reload updates the filesystem with new configs. It updates existing MKVFile
// objects in place to preserve pointer identity for cached FUSE inodes, and
// merges the directory tree structure (required because go-fuse caches
// persistent inode objects by inode number).
//
// After the merge, FUSE kernel notifications are emitted:
//   - NotifyDelete for removed files (sends IN_DELETE to inotify watchers)
//   - NotifyEntry for added files (invalidates kernel dentry cache)
//   - NotifyContent on changed directories (invalidates readdir cache)
//
// Note: The FUSE protocol has no NOTIFY_CREATE, so added files don't
// generate proactive inotify events. Media servers should use periodic
// scanning in addition to inotify watching.
//
// Semantics:
//   - New files become immediately visible
//   - Removed files disappear from listings
//   - Modified mappings update existing MKVFile objects in place; active readers
//     are closed if the underlying dedup path changed (re-opened lazily on next read)
//   - Permissions are reloaded from disk and stale entries cleaned up
//     (cleanup is skipped if permission reload fails, to avoid overwriting
//     a temporarily unreadable permissions file)
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
		if existing, ok := newFiles[config.Name]; ok {
			logFn("warning: duplicate name %q (dedup: %s replaced by %s)", config.Name, existing.DedupPath, mkvFile.DedupPath)
		}
		newFiles[config.Name] = mkvFile
	}

	// Snapshot old file names for change detection
	r.mu.RLock()
	oldFileNames := make(map[string]bool, len(r.files))
	for name := range r.files {
		oldFileNames[name] = true
	}
	r.mu.RUnlock()

	// Before merge: capture child inodes for files being removed. We need
	// these for NotifyDelete (sends IN_DELETE inotify event), and the child
	// inode won't be reachable after tree merge removes it. We do NOT
	// capture parent inodes here because the merge may delete parent
	// directories, leaving stale inode pointers that crash go-fuse.
	deletedChildren := make(map[string]*fs.Inode) // filePath → child inode
	for name := range oldFileNames {
		if _, inNew := newFiles[name]; !inNew {
			parentInode, basename := r.findParentInode(name)
			if parentInode != nil {
				if child := parentInode.GetChild(basename); child != nil {
					deletedChildren[name] = child
				}
			}
		}
	}

	// Build new directory tree
	fileList := make([]*MKVFile, 0, len(newFiles))
	for _, f := range newFiles {
		fileList = append(fileList, f)
	}
	newTree := BuildDirectoryTree(fileList, r.verbose, r.readerFactory, r.permStore)

	// Update flat files map in place (preserves pointer identity for cached inodes)
	r.mu.Lock()
	for name := range r.files {
		if _, inNew := newFiles[name]; !inNew {
			delete(r.files, name)
		}
	}
	for name, newFile := range newFiles {
		if existingFile, ok := r.files[name]; ok {
			existingFile.mu.Lock()
			existingFile.updateFrom(newFile)
			existingFile.mu.Unlock()
		} else {
			r.files[name] = newFile
		}
	}
	r.mu.Unlock()

	// Merge new tree into existing tree in place
	mergeDirectoryTree(r.rootDir, newTree)

	// After merge: capture all notifications using the post-merge tree.
	// Parent inodes are now resolved against the live tree, so we never
	// reference deleted directory inodes. If a parent directory was removed
	// by the merge, findParentInode returns nil and we skip the notification
	// — the directory removal already invalidates its children in the kernel.
	var notifications []reloadNotification
	changedDirs := make(map[*fs.Inode]bool)
	for name := range oldFileNames {
		if _, inNew := newFiles[name]; !inNew {
			parentInode, basename := r.findParentInode(name)
			if parentInode != nil {
				notifications = append(notifications, reloadNotification{
					parent:   parentInode,
					child:    deletedChildren[name],
					name:     basename,
					isDelete: true,
				})
				changedDirs[parentInode] = true
				markAncestorDirs(parentInode, changedDirs)
			}
		}
	}
	for name := range newFiles {
		if !oldFileNames[name] {
			parentInode, basename := r.findParentInode(name)
			if parentInode != nil {
				notifications = append(notifications, reloadNotification{
					parent:   parentInode,
					name:     basename,
					isDelete: false,
				})
				changedDirs[parentInode] = true
				markAncestorDirs(parentInode, changedDirs)
			}
		}
	}

	// Reload permissions and clean up stale entries
	if r.permStore != nil {
		if err := r.permStore.Load(); err != nil {
			logFn("warning: failed to reload permissions: %v", err)
		} else {
			validFiles, validDirs := r.collectValidPaths()
			removed := r.permStore.CleanupStale(validFiles, validDirs)
			if removed > 0 {
				logFn("cleaned up %d stale permission entries", removed)
				if err := r.permStore.Save(); err != nil {
					logFn("warning: failed to save permissions after cleanup: %v", err)
				}
			}
		}
	}

	logFn("reload complete: %d files", len(newFiles))

	// Emit FUSE kernel notifications. Must be called after all filesystem
	// locks are released — go-fuse may call back into the FS during
	// notification processing, which would deadlock if locks were held.
	r.emitReloadNotifications(notifications, changedDirs, logFn)

	return nil
}

// Files returns a snapshot of the current file set. Used by SourceWatcher
// to build reverse mappings from source files to virtual files. Returns a
// defensive copy to avoid data races with concurrent Reload() calls.
func (r *MKVFSRoot) Files() map[string]*MKVFile {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]*MKVFile, len(r.files))
	for k, v := range r.files {
		out[k] = v
	}
	return out
}

// SetMounted marks the filesystem as mounted, enabling FUSE kernel
// notifications during config reload. Must be called after fs.Mount()
// succeeds.
func (r *MKVFSRoot) SetMounted() {
	r.mounted.Store(true)
}

// emitReloadNotifications sends FUSE kernel notifications for files that
// were added or removed during a config reload.
func (r *MKVFSRoot) emitReloadNotifications(notifications []reloadNotification, changedDirs map[*fs.Inode]bool, logFn func(string, ...interface{})) {
	if len(notifications) == 0 || !r.mounted.Load() {
		return
	}

	var deleted, invalidated int
	for _, n := range notifications {
		if n.isDelete {
			if n.child != nil {
				// NotifyDelete sends a real IN_DELETE inotify event
				if errno := n.parent.NotifyDelete(n.name, n.child); errno == 0 {
					deleted++
				}
			} else {
				// Child inode was never cached by kernel — just invalidate entry
				if errno := n.parent.NotifyEntry(n.name); errno == 0 {
					invalidated++
				}
			}
		} else {
			// NotifyEntry invalidates the kernel's dentry cache so the
			// new file is visible on next lookup/readdir.
			if errno := n.parent.NotifyEntry(n.name); errno == 0 {
				invalidated++
			}
		}
	}

	// Invalidate readdir cache for all directories that had changes.
	// Skip uninitialized inodes (Ino==0) as a safety net — these should
	// not appear here after the findParentInode fix, but guard anyway.
	for dirInode := range changedDirs {
		if dirInode.StableAttr().Ino != 0 {
			dirInode.NotifyContent(0, 0)
		}
	}

	if deleted > 0 || invalidated > 0 {
		logFn("kernel notifications: %d deleted, %d invalidated, %d dirs", deleted, invalidated, len(changedDirs))
	}
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

// Getattr implements fs.NodeGetattrer - returns attributes for the root directory.
// This ensures the root directory uses permissions from the permission store,
// consistent with all subdirectories.
func (r *MKVFSRoot) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
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
		r.rootDir.mu.RLock()
		out.Nlink += uint32(len(r.rootDir.subdirs))
		r.rootDir.mu.RUnlock()
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
