//go:build darwin

package fuse

import "golang.org/x/sys/unix"

// isNetworkFS checks if the given path is on a network filesystem.
func isNetworkFS(path string) bool {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		// Can't determine — assume local
		return false
	}

	// On macOS, Fstypename is a [16]byte containing the FS type name.
	n := 0
	for n < len(stat.Fstypename) && stat.Fstypename[n] != 0 {
		n++
	}
	fstype := string(stat.Fstypename[:n])

	switch fstype {
	case "nfs", "smbfs", "afpfs", "webdav":
		return true
	}
	return false
}
