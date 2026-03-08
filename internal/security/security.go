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

// Geteuid returns the effective user ID. Exported for testing.
var Geteuid = os.Geteuid

// CheckFileOwnership validates that a file is root-owned and not
// group-writable or world-writable. Returns nil if safe, or an error
// describing the violation. Only checks when running as root (euid == 0).
func CheckFileOwnership(path string) error {
	if Geteuid() != 0 {
		return nil
	}

	info, err := os.Lstat(path)
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
	joined := filepath.Join(sourceDir, relPath)
	if Geteuid() != 0 {
		return joined, nil
	}

	// Canonicalize sourceDir
	canonicalDir, err := filepath.EvalSymlinks(sourceDir)
	if err != nil {
		return "", fmt.Errorf("security: resolve source dir %s: %w", sourceDir, err)
	}

	// Canonicalize the full path
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

// CheckDirectory validates that a directory is root-owned and not
// group-writable or world-writable. Returns nil if safe. Only checks
// when running as root (euid == 0).
func CheckDirectory(dir string) error {
	return CheckFileOwnership(dir)
}
