// Package security provides file ownership and path confinement checks
// for FUSE mounts running as root.
package security

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// fileStatFunc is a package-level var for os.Stat, allowing test injection.
var fileStatFunc = os.Stat

// Geteuid returns the effective user ID. Exported for testing.
var Geteuid = os.Geteuid

// CheckFileOwnership validates that a file is root-owned and not
// group-writable or world-writable. Returns nil if safe, or an error
// describing the violation. Only checks when running as root (euid == 0).
// The path is resolved via EvalSymlinks before checking.
func CheckFileOwnership(path string) error {
	if Geteuid() != 0 {
		return nil
	}

	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", path, err)
	}

	return checkOwnership(resolved)
}

// CheckFileOwnershipResolved is like CheckFileOwnership but skips symlink
// resolution, assuming the caller already canonicalized the path.
// Only checks when running as root (euid == 0).
func CheckFileOwnershipResolved(path string) error {
	if Geteuid() != 0 {
		return nil
	}
	return checkOwnership(path)
}

// checkOwnership performs the actual ownership and permission checks on
// an already-resolved path.
func checkOwnership(path string) error {
	info, err := fileStatFunc(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("cannot get ownership info for %s", path)
	}

	if stat.Uid != 0 {
		return fmt.Errorf("security: %s is owned by uid %d, not root", path, stat.Uid)
	}

	mode := info.Mode()
	if mode&0020 != 0 {
		return fmt.Errorf("security: %s is group-writable (%04o)", path, mode.Perm())
	}
	if mode&0002 != 0 {
		return fmt.Errorf("security: %s is world-writable (%04o)", path, mode.Perm())
	}

	return nil
}

// CheckPathConfinement resolves sourceDir + relPath, canonicalizes via
// EvalSymlinks, and verifies the result stays within sourceDir. Returns
// the canonical path or an error. Only checks when running as root.
//
// When not running as root, returns the simple joined path without
// canonicalization (preserving existing behavior).
func CheckPathConfinement(sourceDir, relPath string) (string, error) {
	// Reject absolute paths regardless of euid — filepath.Join would
	// silently drop sourceDir for absolute relPath, allowing escape.
	if filepath.IsAbs(relPath) {
		return "", fmt.Errorf("security: absolute source path %q not allowed", relPath)
	}

	if Geteuid() != 0 {
		// Non-root: return cleaned join without canonicalization.
		// Absolute relPath is already rejected above, so Join always
		// prepends sourceDir. Note that Join cleans ".." components,
		// but confinement is not enforced in non-root mode.
		return filepath.Join(sourceDir, relPath), nil
	}

	// Canonicalize sourceDir
	canonicalDir, err := filepath.EvalSymlinks(sourceDir)
	if err != nil {
		return "", fmt.Errorf("security: resolve source dir %s: %w", sourceDir, err)
	}

	// Canonicalize the full path
	joined := filepath.Join(sourceDir, relPath)
	canonical, err := filepath.EvalSymlinks(joined)
	if err != nil {
		return "", fmt.Errorf("security: resolve source path %s: %w", joined, err)
	}

	// Use trailing separator to prevent prefix attacks
	// (e.g., /data/source-evil matching /data/source)
	if !strings.HasPrefix(canonical+"/", canonicalDir+"/") {
		return "", fmt.Errorf("security: source path %s escapes source dir %s (resolved to %s)", relPath, sourceDir, canonical)
	}

	return canonical, nil
}

// CheckDirectory validates that a path is a directory, is root-owned,
// and is not group-writable or world-writable. Returns nil if safe.
// Only checks when running as root (euid == 0).
// The path is resolved via EvalSymlinks before checking.
func CheckDirectory(dir string) error {
	if Geteuid() != 0 {
		return nil
	}

	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", dir, err)
	}

	return checkDirectory(resolved)
}

// CheckDirectoryResolved is like CheckDirectory but skips symlink
// resolution, assuming the caller already canonicalized the path.
// Only checks when running as root (euid == 0).
func CheckDirectoryResolved(dir string) error {
	if Geteuid() != 0 {
		return nil
	}
	return checkDirectory(dir)
}

// checkDirectory performs ownership and directory checks on an
// already-resolved path.
func checkDirectory(dir string) error {
	if err := checkOwnership(dir); err != nil {
		return err
	}

	info, err := fileStatFunc(dir)
	if err != nil {
		return fmt.Errorf("stat %s: %w", dir, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("security: %s is not a directory", dir)
	}

	return nil
}
