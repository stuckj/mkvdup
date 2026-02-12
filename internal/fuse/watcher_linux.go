//go:build linux

package fuse

import "golang.org/x/sys/unix"

// Filesystem type constants for network FS detection.
const (
	nfsSuperMagic   = 0x6969
	cifsMagicNum    = 0xFF534D42
	smb2MagicNum    = 0xFE534D42
	afsSuper        = 0x5346414F
	ncpfsSuperMagic = 0x564C
)

// isNetworkFS checks if the given path is on a network filesystem.
func isNetworkFS(path string) bool {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		// Can't determine — assume local
		return false
	}

	switch stat.Type {
	case nfsSuperMagic, cifsMagicNum, smb2MagicNum, afsSuper, ncpfsSuperMagic:
		return true
	}
	return false
}
