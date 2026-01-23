package fuse

import (
	"log"
	"path"
	"strings"
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
