package fuse

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// appName is the directory name used under the state directories below. It is
// deliberately the project name rather than the running binary's name: a canary
// build reads the same virtual filesystems as the stable build and should see
// the same overrides.
const appName = "mkvdup"

// permissionsDirName holds the per-mount permissions files. One file per
// mountpoint, so two mounts can never share a key space (see EscapeMountpoint).
const permissionsDirName = "permissions.d"

// legacyPermissionsPaths are the pre-per-mount locations. They are read as a
// one-time seed for a new per-mount file and are never written again, so an
// existing installation keeps its overrides and any other mount can still seed
// from the same file. Order is the historical search order.
func legacyPermissionsPaths() []string {
	paths := []string{filepath.Join("/etc", appName, "permissions.yaml")}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".config", appName, "permissions.yaml"))
	}
	return paths
}

// StateDir returns the directory holding auto-managed daemon state.
//
// This is state, not configuration: it is written by the daemon in response to
// chmod/chown/touch, not edited by hand as a matter of course. Root therefore
// uses /var/lib rather than /etc, and non-root uses XDG_STATE_HOME rather than
// ~/.config. Non-root always gets a user-writable path so that saving a
// permission change cannot fail with EACCES.
func StateDir() string {
	if os.Geteuid() == 0 {
		return filepath.Join("/var/lib", appName)
	}
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" && filepath.IsAbs(xdg) {
		return filepath.Join(xdg, appName)
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "state", appName)
	}
	// No home directory: fall back to the system path. Unusual for non-root,
	// and it may not be writable, but it is better than an empty path.
	return filepath.Join("/var/lib", appName)
}

// CanonicalMountpoint resolves a mountpoint to the stable identity used to name
// its permissions file. Call it before mounting, while the path is still an
// ordinary directory: resolving symlinks through a live FUSE mount would ask
// the filesystem about itself.
//
// A path that cannot be resolved (it does not exist yet, say) falls back to a
// cleaned absolute path rather than failing — a usable identity matters more
// than a perfect one.
func CanonicalMountpoint(mountpoint string) string {
	if mountpoint == "" {
		return ""
	}
	abs, err := filepath.Abs(mountpoint)
	if err != nil {
		return filepath.Clean(mountpoint)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return filepath.Clean(resolved)
	}
	return filepath.Clean(abs)
}

// EscapeMountpoint converts an absolute path into a single filename component,
// using systemd's path-escaping rules: leading and trailing slashes are
// dropped, remaining slashes become '-', and anything outside [A-Za-z0-9_.]
// becomes \xNN. A literal '-' is escaped to \x2d so the mapping is injective —
// without that, /a-b and /a/b would collide.
//
// The result is readable and greppable ("/mnt/videos" -> "mnt-videos"), which
// matters because an admin looking for the file that backs a given mount should
// be able to find it by eye.
func EscapeMountpoint(path string) string {
	trimmed := strings.Trim(filepath.Clean(path), "/")
	if trimmed == "" {
		// The root directory has no components; systemd spells this "-".
		return "-"
	}

	var b strings.Builder
	for i := 0; i < len(trimmed); i++ {
		c := trimmed[i]
		switch {
		case c == '/':
			b.WriteByte('-')
		case c == '_' || (c == '.' && i > 0):
			// A leading '.' is escaped so the file is never hidden.
			b.WriteByte(c)
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			b.WriteByte(c)
		default:
			// Covers '-', a leading '.', spaces, and every non-ASCII byte.
			fmt.Fprintf(&b, `\x%02x`, c)
		}
	}
	return b.String()
}

// ResolvePermissionsPath determines which permissions file a mount uses.
//
// Priority:
//  1. explicitPath (--permissions-file / permissions_file=), used verbatim
//  2. <state dir>/permissions.d/<escaped mountpoint>.yaml
//  3. <state dir>/permissions.yaml, when no mountpoint is known
//
// Case 2 is what makes concurrent mounts safe: override keys are virtual paths
// relative to the mount root, so they are only unique *within* a mount. Giving
// each mountpoint its own file keeps those key spaces from overlapping, and
// keeps one mount's stale-entry cleanup from deleting another's entries.
//
// Case 3 exists for callers with no mountpoint (tests, programmatic use). It
// reproduces the historical single-file behaviour, sharing and all.
func ResolvePermissionsPath(explicitPath, mountpoint string) string {
	if explicitPath != "" {
		return explicitPath
	}

	stateDir := StateDir()
	canonical := CanonicalMountpoint(mountpoint)
	if canonical == "" {
		return filepath.Join(stateDir, "permissions.yaml")
	}
	return filepath.Join(stateDir, permissionsDirName, EscapeMountpoint(canonical)+".yaml")
}
