package fuse

import (
	"log"
	"path"
	"strings"
	"time"
)

// BuildDirectoryTree creates a directory tree from files with path-containing names.
// Directories are auto-created for each path component.
// Files with names like "Movies/Action/film.mkv" will create the directory hierarchy.
//
// Path handling:
//   - Leading slashes are stripped (absolute paths become relative)
//   - Paths are cleaned (e.g., "foo//bar" becomes "foo/bar")
//   - Only forward slashes (/) are treated as path separators
//   - Paths containing ".." components are rejected
//   - Empty filenames are rejected
//
// Conflicts:
//   - Duplicate paths: later file wins, warning logged
//   - File/directory collision: directory wins, file skipped with warning
func BuildDirectoryTree(files []*MKVFile, verbose bool, readerFactory ReaderFactory, permStore *PermissionStore) *MKVFSDirNode {
	root := &MKVFSDirNode{
		name:          "",
		path:          "",
		files:         make(map[string]*MKVFile),
		subdirs:       make(map[string]*MKVFSDirNode),
		verbose:       verbose,
		readerFactory: readerFactory,
		permStore:     permStore,
		mtime:         fsStartTime,
	}

	for _, file := range files {
		insertFile(root, file, verbose, readerFactory, permStore)
	}

	return root
}

// insertFile inserts a file into the directory tree, creating directories as needed.
func insertFile(root *MKVFSDirNode, file *MKVFile, verbose bool, readerFactory ReaderFactory, permStore *PermissionStore) {
	// Validate: reject paths with ".." components (security)
	if strings.Contains(file.Name, "..") {
		log.Printf("Warning: skipping file with invalid path (contains '..'): %s", file.Name)
		return
	}

	// Clean and split the path
	cleanPath := path.Clean(file.Name)
	parts := strings.Split(cleanPath, "/")

	// Filter out empty parts (handles leading slashes and multiple slashes)
	validParts := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" && p != "." {
			validParts = append(validParts, p)
		}
	}

	// Validate: reject empty filenames
	if len(validParts) == 0 {
		log.Printf("Warning: skipping file with empty name: %q", file.Name)
		return
	}

	fileName := validParts[len(validParts)-1]
	if fileName == "" {
		log.Printf("Warning: skipping file with empty filename: %q", file.Name)
		return
	}

	// Navigate/create directories for each path component except the last (filename)
	current := root
	for i := 0; i < len(validParts)-1; i++ {
		dirName := validParts[i]

		current.mu.Lock()
		// Check for file/directory collision: if a file exists with this name, skip
		if _, fileExists := current.files[dirName]; fileExists {
			log.Printf("Warning: path component %q conflicts with existing file, skipping: %s", dirName, file.Name)
			current.mu.Unlock()
			return
		}

		subdir, exists := current.subdirs[dirName]
		if !exists {
			// Create new directory node
			var newPath string
			if current.path == "" {
				newPath = dirName
			} else {
				newPath = current.path + "/" + dirName
			}
			subdir = &MKVFSDirNode{
				name:          dirName,
				path:          newPath,
				files:         make(map[string]*MKVFile),
				subdirs:       make(map[string]*MKVFSDirNode),
				verbose:       verbose,
				readerFactory: readerFactory,
				permStore:     permStore,
				mtime:         fsStartTime,
			}
			current.subdirs[dirName] = subdir
		}
		current.mu.Unlock()
		current = subdir
	}

	// Insert the file into the final directory
	current.mu.Lock()
	defer current.mu.Unlock()

	// Check for file/directory collision: if a directory exists with this name, skip the file
	if _, dirExists := current.subdirs[fileName]; dirExists {
		log.Printf("Warning: file %q conflicts with existing directory, skipping", file.Name)
		return
	}

	// Check for duplicate: warn if overwriting
	if existing, exists := current.files[fileName]; exists {
		log.Printf("Warning: duplicate path %q, replacing %s with %s", file.Name, existing.DedupPath, file.DedupPath)
	}

	current.files[fileName] = file
}

// mergeDirectoryTree merges newTree's contents into existing's maps in place.
// This is necessary because go-fuse caches persistent inode objects by inode
// number — swapping the root directory won't affect already-cached inodes.
// Instead, we update existing MKVFSDirNode objects' files and subdirs maps
// so cached inodes see the new data.
//
// Directory mtimes follow POSIX semantics: a directory's mtime advances to the
// time of the merge only if a direct child (file or subdirectory) was added or
// removed. Directories whose contents merely changed in place — e.g. a file's
// dedup path or its derived mtime — keep their existing mtime, and the change
// is not propagated to ancestors.
func mergeDirectoryTree(existing, newTree *MKVFSDirNode) {
	mergeDirectoryTreeAt(existing, newTree, time.Now())
}

// mergeDirectoryTreeAt is mergeDirectoryTree with an explicit timestamp so a
// single reload stamps every affected directory identically.
func mergeDirectoryTreeAt(existing, newTree *MKVFSDirNode, now time.Time) {
	existing.mu.Lock()
	defer existing.mu.Unlock()

	// childrenChanged tracks whether an entry was added to or removed from this
	// directory, which is what POSIX says updates a directory's mtime.
	childrenChanged := false

	// Remove files that are no longer present
	for name := range existing.files {
		if _, inNew := newTree.files[name]; !inNew {
			delete(existing.files, name)
			childrenChanged = true
		}
	}

	// Add or update files (update in place to preserve pointer identity for cached inodes)
	for name, newFile := range newTree.files {
		if existingFile, ok := existing.files[name]; ok {
			existingFile.mu.Lock()
			existingFile.updateFrom(newFile)
			existingFile.mu.Unlock()
		} else {
			existing.files[name] = newFile
			childrenChanged = true
		}
	}

	// Remove subdirectories that are no longer present
	for name := range existing.subdirs {
		if _, inNew := newTree.subdirs[name]; !inNew {
			delete(existing.subdirs, name)
			childrenChanged = true
		}
	}

	// Add or recursively merge subdirectories
	for name, newSubdir := range newTree.subdirs {
		existingSubdir, exists := existing.subdirs[name]
		if !exists {
			// Freshly created directory: it and everything under it came into
			// existence now, so stamp the whole subtree.
			stampDirTree(newSubdir, now)
			existing.subdirs[name] = newSubdir
			childrenChanged = true
		} else {
			mergeDirectoryTreeAt(existingSubdir, newSubdir, now)
		}
	}

	if childrenChanged {
		existing.mtime = now
	}
}

// stampDirTree sets mtime on d and all of its descendant directories. Used for
// subtrees that are newly created during a reload, since every entry in them
// was added at that moment.
func stampDirTree(d *MKVFSDirNode, now time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.mtime = now
	for _, sub := range d.subdirs {
		stampDirTree(sub, now)
	}
}
