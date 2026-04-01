package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

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
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("check destination: %w", err)
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
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("check destination %s: %w", absDst, err)
		}
		// Always check for existing destination sidecar, even if source has none,
		// to avoid leaving stale/mismatched sidecars.
		if _, err := os.Stat(sidecarDst); err == nil {
			return fmt.Errorf("destination sidecar %s already exists (use --force to overwrite)", sidecarDst)
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

		// Recalculate relative paths in virtual_files entries
		if err := recalcVirtualFiles(root, srcDir, dstDir); err != nil {
			return fmt.Errorf("recalculate virtual_files paths: %w", err)
		}

		// Recalculate relative include patterns
		recalcIncludes(root, srcDir, dstDir)

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

	// Move the .mkvdup file (supports cross-filesystem moves)
	if err := moveFile(absSrc, absDst); err != nil {
		return fmt.Errorf("move dedup file: %w", err)
	}

	// With --force and no source sidecar, clean up any orphaned destination
	// sidecar now that the dedup move has succeeded.
	if force && !hasSidecar {
		if _, err := os.Stat(sidecarDst); err == nil {
			if err := osRemove(sidecarDst); err != nil {
				printWarn("Warning: could not remove orphaned sidecar %s: %v\n", sidecarDst, err)
			}
		}
	}

	// Write updated sidecar atomically, then remove old one.
	// If sidecar write fails, rollback the dedup move.
	if hasSidecar {
		if err := writeFileAtomic(sidecarDst, updatedSidecar, 0644); err != nil {
			if rbErr := moveFile(absDst, absSrc); rbErr != nil {
				printWarn("Warning: failed to rollback dedup move: %v\n", rbErr)
			}
			return fmt.Errorf("write sidecar: %w", err)
		}
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

// writeFileAtomic writes data to dst via a temp file + rename, ensuring
// no partially written file is left at dst on failure. The temp file is
// cleaned up automatically on any error.
func writeFileAtomic(dst string, data []byte, perm os.FileMode) error {
	tmpFile, err := os.CreateTemp(filepath.Dir(dst), ".mkvdup-relocate-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	success := false
	defer func() {
		if !success {
			_ = osRemove(tmpPath)
		}
	}()

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return err
	}
	if err := osRename(tmpPath, dst); err != nil {
		return err
	}
	success = true
	return nil
}

// moveFile moves a file from src to dst. It tries os.Rename first for
// efficiency; if that fails with EXDEV (cross-device), it falls back to
// copy + remove.
func moveFile(src, dst string) error {
	err := osRename(src, dst)
	if err == nil {
		return nil
	}
	if !errors.Is(err, syscall.EXDEV) {
		return err
	}

	// Cross-filesystem: copy then remove source.
	if err := copyFile(src, dst); err != nil {
		return fmt.Errorf("cross-device copy: %w", err)
	}
	if err := osRemove(src); err != nil {
		return fmt.Errorf("remove source after cross-device copy: %w", err)
	}
	return nil
}

// copyFile copies a file from src to dst, preserving permissions.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		dstFile.Close()
		_ = osRemove(dst)
		return err
	}
	if err := dstFile.Close(); err != nil {
		_ = osRemove(dst)
		return err
	}
	return nil
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

// recalcVirtualFiles recalculates relative dedup_file and source_dir paths
// in virtual_files entries (a YAML sequence of mappings).
func recalcVirtualFiles(root *yaml.Node, srcDir, dstDir string) error {
	vfNode := yamlNodeByKey(root, "virtual_files")
	if vfNode == nil || vfNode.Kind != yaml.SequenceNode {
		return nil
	}
	for i, entry := range vfNode.Content {
		if entry.Kind != yaml.MappingNode {
			continue
		}
		// Recalculate dedup_file (points to a static file, not being moved)
		if old := yamlNodeValue(entry, "dedup_file"); old != "" {
			recalced, err := recalcRelativePath(srcDir, dstDir, old)
			if err != nil {
				return fmt.Errorf("virtual_files[%d].dedup_file: %w", i, err)
			}
			setYAMLNodeValue(entry, "dedup_file", recalced)
		}
		// Recalculate source_dir
		if old := yamlNodeValue(entry, "source_dir"); old != "" {
			recalced, err := recalcRelativePath(srcDir, dstDir, old)
			if err != nil {
				return fmt.Errorf("virtual_files[%d].source_dir: %w", i, err)
			}
			setYAMLNodeValue(entry, "source_dir", recalced)
		}
	}
	return nil
}

// recalcIncludes recalculates relative include glob patterns in the sidecar.
func recalcIncludes(root *yaml.Node, srcDir, dstDir string) {
	inclNode := yamlNodeByKey(root, "includes")
	if inclNode == nil || inclNode.Kind != yaml.SequenceNode {
		return
	}
	for _, entry := range inclNode.Content {
		if entry.Kind != yaml.ScalarNode || filepath.IsAbs(entry.Value) {
			continue
		}
		// Recalculate the relative portion. Glob patterns may contain
		// wildcards, but the directory prefix is what needs adjusting.
		// recalcRelativePath works on the path as-is since filepath.Rel
		// handles non-existent paths fine.
		recalced, err := recalcRelativePath(srcDir, dstDir, entry.Value)
		if err == nil {
			entry.Value = recalced
		}
	}
}

// yamlNodeByKey returns the value node for a key in a YAML mapping node.
// Returns nil if the key is not found.
func yamlNodeByKey(mapping *yaml.Node, key string) *yaml.Node {
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

