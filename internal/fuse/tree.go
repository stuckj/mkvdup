package fuse

import (
	"path"
	"strings"
)

// BuildDirectoryTree creates a directory tree from files with path-containing names.
// Directories are auto-created for each path component.
// Files with names like "Movies/Action/film.mkv" will create the directory hierarchy.
func BuildDirectoryTree(files []*MKVFile, verbose bool, readerFactory ReaderFactory) *MKVFSDirNode {
	root := &MKVFSDirNode{
		name:          "",
		path:          "",
		files:         make(map[string]*MKVFile),
		subdirs:       make(map[string]*MKVFSDirNode),
		verbose:       verbose,
		readerFactory: readerFactory,
	}

	for _, file := range files {
		insertFile(root, file, verbose, readerFactory)
	}

	return root
}

// insertFile inserts a file into the directory tree, creating directories as needed.
func insertFile(root *MKVFSDirNode, file *MKVFile, verbose bool, readerFactory ReaderFactory) {
	// Clean and split the path
	cleanPath := path.Clean(file.Name)
	parts := strings.Split(cleanPath, "/")

	// Navigate/create directories for each path component except the last (filename)
	current := root
	for i := 0; i < len(parts)-1; i++ {
		dirName := parts[i]
		if dirName == "" || dirName == "." {
			continue
		}

		current.mu.Lock()
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
			}
			current.subdirs[dirName] = subdir
		}
		current.mu.Unlock()
		current = subdir
	}

	// Insert the file into the final directory
	fileName := parts[len(parts)-1]
	current.mu.Lock()
	current.files[fileName] = file
	current.mu.Unlock()
}
