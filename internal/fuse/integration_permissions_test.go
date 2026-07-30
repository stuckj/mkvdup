//go:build integration

package fuse_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	fuselib "github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	fusepkg "github.com/stuckj/mkvdup/internal/fuse"
)

func TestFUSE_ChmodFile(t *testing.T) {
	skipIfFUSEUnavailable(t)
	_, configPath, _ := getSharedFixture(t)

	tmpDir, err := os.MkdirTemp("", "mkvdup-fuse-perm-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mountPoint := filepath.Join(tmpDir, "mount")
	if err := os.Mkdir(mountPoint, 0755); err != nil {
		t.Fatalf("Failed to create mount point: %v", err)
	}

	// Create permission store path
	permPath := filepath.Join(tmpDir, "permissions.yaml")

	// Get current user's UID/GID - files must be owned by current user to chmod
	currentUID := uint32(os.Getuid())
	currentGID := uint32(os.Getgid())

	// Create FUSE filesystem with permission store, owned by current user
	root, err := fusepkg.NewMKVFSWithOptions([]string{configPath}, fusepkg.MKVFSOptions{
		Verbose:         false,
		PermissionsPath: permPath,
		Defaults: &fusepkg.Defaults{
			FileUID:  currentUID,
			FileGID:  currentGID,
			FileMode: 0444,
			DirUID:   currentUID,
			DirGID:   currentGID,
			DirMode:  0555,
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

	// Get initial permissions
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}
	initialMode := info.Mode().Perm()
	t.Logf("Initial mode: %o", initialMode)

	// Change permissions to 0640
	newMode := os.FileMode(0640)
	if err := os.Chmod(filePath, newMode); err != nil {
		t.Fatalf("Failed to chmod: %v", err)
	}

	// Verify the change
	info, err = os.Stat(filePath)
	if err != nil {
		t.Fatalf("Failed to stat file after chmod: %v", err)
	}
	if info.Mode().Perm() != newMode {
		t.Errorf("Expected mode %o, got %o", newMode, info.Mode().Perm())
	}

	// Verify permissions file was created. Writes are coalesced, so this is not
	// instantaneous — poll rather than reading once.
	waitForFile(t, permPath, 10*time.Second)
}

func TestFUSE_ChmodDirectory(t *testing.T) {
	skipIfFUSEUnavailable(t)
	getSharedFixture(t) // Verify fixture is available

	tmpDir, err := os.MkdirTemp("", "mkvdup-fuse-perm-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create config with directory path in virtual name (reuses shared dedup file)
	configPath := copyConfigWithName(t, tmpDir, "Movies/test.mkv")

	mountPoint := filepath.Join(tmpDir, "mount")
	if err := os.Mkdir(mountPoint, 0755); err != nil {
		t.Fatalf("Failed to create mount point: %v", err)
	}

	permPath := filepath.Join(tmpDir, "permissions.yaml")

	// Get current user's UID/GID - directories must be owned by current user to chmod
	currentUID := uint32(os.Getuid())
	currentGID := uint32(os.Getgid())

	root, err := fusepkg.NewMKVFSWithOptions([]string{configPath}, fusepkg.MKVFSOptions{
		Verbose:         false,
		PermissionsPath: permPath,
		Defaults: &fusepkg.Defaults{
			FileUID:  currentUID,
			FileGID:  currentGID,
			FileMode: 0444,
			DirUID:   currentUID,
			DirGID:   currentGID,
			DirMode:  0555,
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

	dirPath := filepath.Join(mountPoint, "Movies")

	// Get initial permissions
	info, err := os.Stat(dirPath)
	if err != nil {
		t.Fatalf("Failed to stat directory: %v", err)
	}
	t.Logf("Initial dir mode: %o", info.Mode().Perm())

	// Change permissions to 0750
	newMode := os.FileMode(0750)
	if err := os.Chmod(dirPath, newMode); err != nil {
		t.Fatalf("Failed to chmod directory: %v", err)
	}

	// Verify the change
	info, err = os.Stat(dirPath)
	if err != nil {
		t.Fatalf("Failed to stat directory after chmod: %v", err)
	}
	if info.Mode().Perm() != newMode {
		t.Errorf("Expected mode %o, got %o", newMode, info.Mode().Perm())
	}
}

func TestFUSE_PermissionAllowed_OwnerAccess(t *testing.T) {
	skipIfFUSEUnavailable(t)
	_, configPath, _ := getSharedFixture(t)

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

	// Get current user's UID/GID
	uid := uint32(os.Getuid())
	gid := uint32(os.Getgid())

	// Create FUSE filesystem with files owned by current user, owner-only read
	root, err := fusepkg.NewMKVFSWithOptions([]string{configPath}, fusepkg.MKVFSOptions{
		Verbose:         false,
		PermissionsPath: permPath,
		Defaults: &fusepkg.Defaults{
			FileUID:  uid,
			FileGID:  gid,
			FileMode: 0400, // owner read only
			DirUID:   uid,
			DirGID:   gid,
			DirMode:  0700, // owner only
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

	// Owner should be able to read their own file
	f, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("Owner should be able to read own file with mode 0400: %v", err)
	}
	defer f.Close()

	buf := make([]byte, 1024)
	n, err := f.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	t.Logf("Owner successfully read %d bytes from file with mode 0400", n)
}
