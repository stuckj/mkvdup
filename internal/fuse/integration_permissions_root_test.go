//go:build integration && rootonly

// FUSE integration tests that can only run as root. They live behind the
// `rootonly` build tag so that in a non-root context they are never scheduled,
// rather than scheduled and skipped -- a skipped test is indistinguishable
// from a passing one in a green build (see #201).
package fuse_test

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	fuselib "github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	fusepkg "github.com/stuckj/mkvdup/internal/fuse"
)

// requireRoot fails rather than skips when the rootonly tag is built outside a
// root context, so the gap this tag exists to close cannot silently reappear.
func requireRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Fatal("tests built with the 'rootonly' tag must be run as root")
	}
}

func TestFUSE_ChownFile(t *testing.T) {
	skipIfFUSEUnavailable(t)
	_, configPath, _ := getSharedFixture(t)

	// chown requires root
	requireRoot(t)

	tmpDir, err := os.MkdirTemp("", "mkvdup-fuse-perm-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mountPoint := filepath.Join(tmpDir, "mount")
	if err := os.Mkdir(mountPoint, 0755); err != nil {
		t.Fatalf("Failed to create mount point: %v", err)
	}

	permPath := filepath.Join(tmpDir, "permissions.yaml")

	root, err := fusepkg.NewMKVFSWithOptions([]string{configPath}, fusepkg.MKVFSOptions{
		Verbose:         false,
		PermissionsPath: permPath,
	})
	if err != nil {
		t.Fatalf("Failed to create MKVFS: %v", err)
	}

	server, err := fuselib.Mount(mountPoint, root, &fuselib.Options{
		MountOptions: fuse.MountOptions{
			AllowOther: false,
			Debug:      false,
			Options:    []string{"default_permissions"},
		},
	})
	if err != nil {
		t.Fatalf("Failed to mount: %v", err)
	}

	defer func() {
		if err := server.Unmount(); err != nil {
			t.Logf("Warning: unmount failed: %v", err)
		}
	}()

	server.WaitMount()

	filePath := filepath.Join(mountPoint, "test.mkv")

	// Change ownership to 1000:1000
	if err := os.Chown(filePath, 1000, 1000); err != nil {
		t.Fatalf("Failed to chown: %v", err)
	}

	// Verify the change via stat
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Failed to stat file after chown: %v", err)
	}

	// Get uid/gid from stat
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatal("Failed to get syscall.Stat_t from FileInfo")
	}

	if stat.Uid != 1000 {
		t.Errorf("Expected UID 1000, got %d", stat.Uid)
	}
	if stat.Gid != 1000 {
		t.Errorf("Expected GID 1000, got %d", stat.Gid)
	}
}

func TestFUSE_RootBypassesPermissions(t *testing.T) {
	skipIfFUSEUnavailable(t)
	_, configPath, _ := getSharedFixture(t)

	// This test requires root
	requireRoot(t)

	tmpDir, err := os.MkdirTemp("", "mkvdup-fuse-perm-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mountPoint := filepath.Join(tmpDir, "mount")
	if err := os.Mkdir(mountPoint, 0755); err != nil {
		t.Fatalf("Failed to create mount point: %v", err)
	}

	permPath := filepath.Join(tmpDir, "permissions.yaml")

	// Create FUSE filesystem with files that have NO permissions (mode 0000)
	root, err := fusepkg.NewMKVFSWithOptions([]string{configPath}, fusepkg.MKVFSOptions{
		Verbose:         false,
		PermissionsPath: permPath,
		Defaults: &fusepkg.Defaults{
			FileUID:  1000, // not root
			FileGID:  1000, // not root
			FileMode: 0000, // no permissions at all
			DirUID:   1000,
			DirGID:   1000,
			DirMode:  0000, // no permissions at all
		},
	})
	if err != nil {
		t.Fatalf("Failed to create MKVFS: %v", err)
	}

	server, err := fuselib.Mount(mountPoint, root, &fuselib.Options{
		MountOptions: fuse.MountOptions{
			AllowOther: false,
			Debug:      false,
			Options:    []string{"default_permissions"},
		},
	})
	if err != nil {
		t.Fatalf("Failed to mount: %v", err)
	}

	defer func() {
		if err := server.Unmount(); err != nil {
			t.Logf("Warning: unmount failed: %v", err)
		}
	}()

	server.WaitMount()

	filePath := filepath.Join(mountPoint, "test.mkv")

	// Root should be able to read file even with mode 0000
	f, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("Root should bypass all permission checks, but got: %v", err)
	}
	defer f.Close()

	buf := make([]byte, 1024)
	n, err := f.Read(buf)
	if err != nil {
		t.Fatalf("Root failed to read file: %v", err)
	}
	t.Logf("Root successfully read %d bytes from file with mode 0000", n)
}
