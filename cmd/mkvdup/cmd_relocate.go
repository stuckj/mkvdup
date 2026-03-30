package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// relocateDedup moves an .mkvdup file and its .mkvdup.yaml sidecar to a new
// location, recalculating relative paths in the sidecar so they resolve to
// the same absolute locations from the new position.
func relocateDedup(src, dst string, force, dryRun bool) error {
	// Resolve source to absolute path
	absSrc, err := filepath.Abs(src)
	if err != nil {
		return fmt.Errorf("resolve source path: %w", err)
	}

	// Verify source .mkvdup file exists
	srcInfo, err := os.Stat(absSrc)
	if err != nil {
		return fmt.Errorf("source file: %w", err)
	}
	if srcInfo.IsDir() {
		return fmt.Errorf("source %s is a directory, expected an .mkvdup file", absSrc)
	}

	// Determine sidecar path
	sidecarSrc := absSrc + ".yaml"
	hasSidecar := true
	if _, err := os.Stat(sidecarSrc); os.IsNotExist(err) {
		hasSidecar = false
	} else if err != nil {
		return fmt.Errorf("check sidecar: %w", err)
	}

	// Resolve destination
	absDst, err := filepath.Abs(dst)
	if err != nil {
		return fmt.Errorf("resolve destination path: %w", err)
	}

	// If destination is an existing directory, or an explicitly-directory path
	// (e.g. ends with a path separator, like "/new/location/"), move into it
	// with the same filename.
	dstInfo, err := os.Stat(absDst)
	isDirDst := false
	if err == nil && dstInfo.IsDir() {
		isDirDst = true
	} else if os.IsNotExist(err) && len(dst) > 0 && os.IsPathSeparator(dst[len(dst)-1]) {
		isDirDst = true
	}
	if isDirDst {
		absDst = filepath.Join(absDst, filepath.Base(absSrc))
	}

	// Don't relocate to the same path
	if absSrc == absDst {
		return fmt.Errorf("source and destination are the same: %s", absSrc)
	}

	sidecarDst := absDst + ".yaml"

	// Check destination doesn't already exist (unless --force)
	if !force {
		if _, err := os.Stat(absDst); err == nil {
			return fmt.Errorf("destination %s already exists (use --force to overwrite)", absDst)
		}
		// Always check for existing destination sidecar, even if source has none,
		// to avoid leaving stale/mismatched sidecars.
		if _, err := os.Stat(sidecarDst); err == nil {
			return fmt.Errorf("destination sidecar %s already exists (use --force to overwrite)", sidecarDst)
		}
	} else if !hasSidecar {
		// With --force, if the source has no sidecar but the destination does,
		// remove the stale destination sidecar so it doesn't become orphaned.
		if _, err := os.Stat(sidecarDst); err == nil {
			if !dryRun {
				if err := osRemove(sidecarDst); err != nil {
					return fmt.Errorf("remove stale destination sidecar %s: %w", sidecarDst, err)
				}
			}
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("check destination sidecar %s: %w", sidecarDst, err)
		}
	}

	// Read and update sidecar if it exists, preserving all YAML keys/comments
	var updatedSidecar []byte
	if hasSidecar {
		sidecarData, err := os.ReadFile(sidecarSrc)
		if err != nil {
			return fmt.Errorf("read sidecar: %w", err)
		}

		var doc yaml.Node
		if err := yaml.Unmarshal(sidecarData, &doc); err != nil {
			return fmt.Errorf("parse sidecar %s: %w", sidecarSrc, err)
		}
		if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
			return fmt.Errorf("sidecar %s: unexpected YAML structure", sidecarSrc)
		}
		root := doc.Content[0]
		if root.Kind != yaml.MappingNode {
			return fmt.Errorf("sidecar %s: expected YAML mapping, got %v", sidecarSrc, root.Kind)
		}

		// Extract current values for dedup_file and source_dir
		oldDedupFile := yamlNodeValue(root, "dedup_file")
		oldSourceDir := yamlNodeValue(root, "source_dir")
		if oldDedupFile == "" || oldSourceDir == "" {
			return fmt.Errorf("sidecar %s: missing required dedup_file or source_dir", sidecarSrc)
		}

		srcDir := filepath.Dir(absSrc)
		dstDir := filepath.Dir(absDst)

		// dedup_file should point to the new location (since the .mkvdup file
		// itself is being moved). Use the basename for relative paths (sidecar
		// and dedup file are always in the same directory), or the new absolute
		// path if the original was absolute.
		var newDedupFile string
		if filepath.IsAbs(oldDedupFile) {
			newDedupFile = absDst
		} else {
			newDedupFile = filepath.Base(absDst)
		}

		// source_dir points to a static location — recalculate relative to new position
		newSourceDir, err := recalcRelativePath(srcDir, dstDir, oldSourceDir)
		if err != nil {
			return fmt.Errorf("recalculate source_dir path: %w", err)
		}

		// Validate that source_dir is still reachable from the new location
		absSourceDir := resolveRelPath(dstDir, newSourceDir)
		sdInfo, err := os.Stat(absSourceDir)
		if err != nil {
			return fmt.Errorf("source directory not reachable from new location: %s → %s: %w", newSourceDir, absSourceDir, err)
		}
		if !sdInfo.IsDir() {
			return fmt.Errorf("source_dir is not a directory from new location: %s → %s", newSourceDir, absSourceDir)
		}

		// Update values in the YAML node tree (preserves all other keys/comments)
		setYAMLNodeValue(root, "dedup_file", newDedupFile)
		setYAMLNodeValue(root, "source_dir", newSourceDir)

		updatedSidecar, err = yaml.Marshal(&doc)
		if err != nil {
			return fmt.Errorf("marshal updated sidecar: %w", err)
		}
	}

	// Dry run: print what would happen and return
	if dryRun {
		printInfo("Would move:\n")
		printInfo("  %s → %s\n", absSrc, absDst)
		if hasSidecar {
			printInfo("  %s → %s\n", sidecarSrc, sidecarDst)
			printInfo("\nUpdated sidecar would contain:\n")
			printInfo("%s", string(updatedSidecar))
		}
		return nil
	}

	// Ensure destination directory exists
	dstDir := filepath.Dir(absDst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}

	// Move the .mkvdup file
	if err := osRename(absSrc, absDst); err != nil {
		return fmt.Errorf("move dedup file: %w", err)
	}

	// Write updated sidecar at destination (then remove old one).
	// Write to a unique temporary file first, then atomically move into
	// place to avoid leaving a partially written sidecar at sidecarDst.
	if hasSidecar {
		tmpFile, err := os.CreateTemp(filepath.Dir(sidecarDst), ".mkvdup-relocate-*.tmp")
		if err != nil {
			if rbErr := osRename(absDst, absSrc); rbErr != nil {
				return fmt.Errorf("create temp sidecar: %v (also failed to rollback dedup move: %w)", err, rbErr)
			}
			return fmt.Errorf("create temp sidecar: %w", err)
		}
		sidecarTmp := tmpFile.Name()

		if _, err := tmpFile.Write(updatedSidecar); err != nil {
			tmpFile.Close()
			_ = osRemove(sidecarTmp)
			if rbErr := osRename(absDst, absSrc); rbErr != nil {
				return fmt.Errorf("write sidecar: %v (also failed to rollback dedup move: %w)", err, rbErr)
			}
			return fmt.Errorf("write sidecar: %w", err)
		}
		if err := tmpFile.Close(); err != nil {
			_ = osRemove(sidecarTmp)
			if rbErr := osRename(absDst, absSrc); rbErr != nil {
				return fmt.Errorf("close temp sidecar: %v (also failed to rollback dedup move: %w)", err, rbErr)
			}
			return fmt.Errorf("close temp sidecar: %w", err)
		}

		if err := osRename(sidecarTmp, sidecarDst); err != nil {
			_ = osRemove(sidecarTmp)
			if rbErr := osRename(absDst, absSrc); rbErr != nil {
				return fmt.Errorf("move sidecar into place: %v (also failed to rollback dedup move: %w)", err, rbErr)
			}
			return fmt.Errorf("move sidecar into place: %w", err)
		}

		// Remove old sidecar (only if src and dst sidecars are different files)
		if sidecarSrc != sidecarDst {
			if err := osRemove(sidecarSrc); err != nil && !os.IsNotExist(err) {
				printWarn("Warning: could not remove old sidecar %s: %v\n", sidecarSrc, err)
			}
		}
	}

	printInfo("Moved:\n")
	printInfo("  %s → %s\n", absSrc, absDst)
	if hasSidecar {
		printInfo("  %s → %s\n", sidecarSrc, sidecarDst)
	}

	return nil
}

// recalcRelativePath takes a path (which may be relative to oldBase or absolute),
// resolves it to absolute, and returns it relative to newBase. If the original
// path was absolute, it is returned unchanged.
func recalcRelativePath(oldBase, newBase, path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}

	// Resolve to absolute using old base
	absPath := filepath.Join(oldBase, path)
	absPath = filepath.Clean(absPath)

	// Make relative to new base
	rel, err := filepath.Rel(newBase, absPath)
	if err != nil {
		return "", fmt.Errorf("make relative to %s: %w", newBase, err)
	}

	return rel, nil
}

// resolveRelPath resolves a path relative to baseDir. If already absolute, returns as-is.
func resolveRelPath(baseDir, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Clean(filepath.Join(baseDir, path))
}

// yamlNodeValue returns the string value for a key in a YAML mapping node.
// Returns "" if the key is not found.
func yamlNodeValue(mapping *yaml.Node, key string) string {
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1].Value
		}
	}
	return ""
}

// setYAMLNodeValue sets the string value for a key in a YAML mapping node.
func setYAMLNodeValue(mapping *yaml.Node, key, value string) {
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content[i+1].Value = value
			return
		}
	}
}

// printRelocateUsage prints the usage for the relocate command.
func printRelocateUsage() {
	fmt.Print(`Usage: mkvdup relocate [options] <source.mkvdup> <destination>

Move an .mkvdup file and its .mkvdup.yaml sidecar to a new location,
updating relative paths in the sidecar so they resolve to the same
absolute locations from the new position.

Arguments:
    <source.mkvdup>  Path to the .mkvdup file to move
    <destination>    Destination path (file or directory)

Options:
    --dry-run  Preview changes without moving files
    --force    Overwrite destination if it already exists

If <destination> is an existing directory, the file is moved into that
directory with its original filename. Otherwise, <destination> is used
as the new file path.

The .mkvdup.yaml sidecar (if present) is moved alongside the .mkvdup
file. The dedup_file path is updated to reference the new .mkvdup
location. The source_dir path is recalculated so it resolves to the
same absolute location from the new position (absolute source_dir
paths are preserved unchanged).

Before moving, the command validates that source directories referenced
by the sidecar would remain reachable from the new location. If not,
the move is refused.

Examples:
    mkvdup relocate movie.mkvdup /new/location/movie.mkvdup
    mkvdup relocate movie.mkvdup /new/location/
    mkvdup relocate --dry-run movie.mkvdup /new/location/
    mkvdup relocate --force movie.mkvdup /new/location/movie.mkvdup
`)
}
