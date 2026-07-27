//go:build integration && nonroot

// FUSE integration tests that can only run as a NON-root user: they verify
// that the kernel denies access via default_permissions. Running as root would
// bypass every check. They live behind the `nonroot` build tag so that in a
// root context they are never scheduled, rather than scheduled and skipped
// (see #201).
package fuse_test

import (
	"os"
	"path/filepath"
	"testing"

	fuselib "github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	fusepkg "github.com/stuckj/mkvdup/internal/fuse"
)

// requireNonRoot fails rather than skips when the nonroot tag is built in a
// root context.
func requireNonRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Fatal("tests built with the 'nonroot' tag must NOT be run as root")
	}
}

func TestFUSE_PermissionDenied(t *testing.T) {
	skipIfFUSEUnavailable(t)
	_, configPath, _ := getSharedFixture(t)

	// This test requires running as non-root to test permission denial
	requireNonRoot(t)

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

	// Create FUSE filesystem with custom defaults that deny access
	// File owned by root with mode 0600 - non-root can't read
	root, err := fusepkg.NewMKVFSWithOptions([]string{configPath}, fusepkg.MKVFSOptions{
		Verbose:         false,
		PermissionsPath: permPath,
		Defaults: &fusepkg.Defaults{
			FileUID:  0,    // root
			FileGID:  0,    // root
			FileMode: 0600, // owner read/write only
			DirUID:   0,
			DirGID:   0,
			DirMode:  0755, // directories need to be accessible
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

	// Try to open the file - should fail with permission denied
	_, err = os.Open(filePath)
	if err == nil {
		t.Error("Expected permission denied error when opening root-owned file with mode 0600")
	} else if !os.IsPermission(err) {
		t.Errorf("Expected permission error, got: %v", err)
	} else {
		t.Logf("Got expected permission error: %v", err)
	}
}

func TestFUSE_PermissionAllowed_GroupAccess(t *testing.T) {
	skipIfFUSEUnavailable(t)
	_, configPath, _ := getSharedFixture(t)

	// Skip if running as root (root bypasses all permission checks)
	requireNonRoot(t)

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

	// Get current user's primary GID
	gid := uint32(os.Getgid())

	// Create FUSE filesystem with files owned by different user but same group
	// Using UID 0 (root) as a different user, current user's GID, group-readable
	root, err := fusepkg.NewMKVFSWithOptions([]string{configPath}, fusepkg.MKVFSOptions{
		Verbose:         false,
		PermissionsPath: permPath,
		Defaults: &fusepkg.Defaults{
			FileUID:  0,    // different owner (root)
			FileGID:  gid,  // current user's primary group
			FileMode: 0040, // group read only (no owner, no other)
			DirUID:   0,
			DirGID:   gid,
			DirMode:  0050, // group read+execute
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

	// User should be able to read file via primary group membership
	f, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("User should be able to read file via primary group (gid=%d) with mode 0040: %v", gid, err)
	}
	defer f.Close()

	buf := make([]byte, 1024)
	n, err := f.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	t.Logf("Successfully read %d bytes via primary group access (gid=%d)", n, gid)
}

func TestFUSE_PermissionAllowed_SupplementaryGroupAccess(t *testing.T) {
	skipIfFUSEUnavailable(t)
	_, configPath, _ := getSharedFixture(t)

	// Skip if running as root (root bypasses all permission checks)
	requireNonRoot(t)

	// Get supplementary groups
	groups, err := os.Getgroups()
	if err != nil {
		t.Fatalf("Failed to get supplementary groups: %v", err)
	}

	// Find a supplementary group that's different from primary GID
	primaryGid := os.Getgid()
	var supplementaryGid int = -1
	for _, g := range groups {
		if g != primaryGid {
			supplementaryGid = g
			break
		}
	}

	if supplementaryGid == -1 {
		t.Skip("No supplementary groups available (only primary GID)")
	}

	t.Logf("Testing access via supplementary group %d (primary gid=%d)", supplementaryGid, primaryGid)

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

	// Create FUSE filesystem with files owned by different user and supplementary group
	root, err := fusepkg.NewMKVFSWithOptions([]string{configPath}, fusepkg.MKVFSOptions{
		Verbose:         false,
		PermissionsPath: permPath,
		Defaults: &fusepkg.Defaults{
			FileUID:  0,                        // different owner (root)
			FileGID:  uint32(supplementaryGid), // supplementary group
			FileMode: 0040,                     // group read only
			DirUID:   0,
			DirGID:   uint32(supplementaryGid),
			DirMode:  0050, // group read+execute
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

	// User should be able to read file via supplementary group membership
	// This tests that the kernel's default_permissions properly checks supplementary groups
	f, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("User should be able to read file via supplementary group (gid=%d) with mode 0040: %v", supplementaryGid, err)
	}
	defer f.Close()

	buf := make([]byte, 1024)
	n, err := f.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	t.Logf("Successfully read %d bytes via supplementary group access (gid=%d)", n, supplementaryGid)
}

func TestFUSE_PermissionDenied_NotInGroup(t *testing.T) {
	skipIfFUSEUnavailable(t)
	_, configPath, _ := getSharedFixture(t)

	// Skip if running as root (root bypasses all permission checks)
	requireNonRoot(t)

	// Find a GID that the current user is NOT a member of
	groups, err := os.Getgroups()
	if err != nil {
		t.Fatalf("Failed to get groups: %v", err)
	}
	groupSet := make(map[int]bool)
	for _, g := range groups {
		groupSet[g] = true
	}

	// Try to find a GID we're not a member of (start from a high number)
	var nonMemberGid uint32 = 65534 // nobody group, likely not a member
	for gid := uint32(1000); gid < 65534; gid++ {
		if !groupSet[int(gid)] {
			nonMemberGid = gid
			break
		}
	}

	t.Logf("Testing access denied for group %d (user is not a member)", nonMemberGid)

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

	// Create FUSE filesystem with files owned by different user and a group we're not in.
	// Directories must allow "other" traverse (0755) so we can reach the file to test its permissions.
	// The file itself (mode 0040 = group read only) is what we're testing access denial on.
	root, err := fusepkg.NewMKVFSWithOptions([]string{configPath}, fusepkg.MKVFSOptions{
		Verbose:         false,
		PermissionsPath: permPath,
		Defaults: &fusepkg.Defaults{
			FileUID:  0,            // different owner (root)
			FileGID:  nonMemberGid, // group we're not a member of
			FileMode: 0040,         // group read only (no owner, no other)
			DirUID:   0,
			DirGID:   nonMemberGid,
			DirMode:  0755, // allow traverse so we can test file permissions
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

	// User should NOT be able to read file (not owner, not in group, no other permissions)
	_, err = os.Open(filePath)
	if err == nil {
		t.Error("Expected permission denied when user is not in file's group with mode 0040")
	} else if !os.IsPermission(err) {
		t.Errorf("Expected permission error, got: %v", err)
	} else {
		t.Logf("Got expected permission denied error: %v", err)
	}
}
