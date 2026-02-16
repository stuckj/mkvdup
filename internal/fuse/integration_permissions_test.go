//go:build integration

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

	// Verify permissions file was created
	if _, err := os.Stat(permPath); os.IsNotExist(err) {
		t.Error("Permissions file was not created")
	}
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

func TestFUSE_ChownFile(t *testing.T) {
	skipIfFUSEUnavailable(t)
	_, configPath, _ := getSharedFixture(t)

	// chown requires root
	if os.Geteuid() != 0 {
		t.Skip("chown requires root privileges")
	}

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

func TestFUSE_PermissionDenied(t *testing.T) {
	skipIfFUSEUnavailable(t)
	_, configPath, _ := getSharedFixture(t)

	// This test requires running as non-root to test permission denial
	if os.Geteuid() == 0 {
		t.Skip("Permission denial test requires non-root user")
	}

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

func TestFUSE_PermissionAllowed_GroupAccess(t *testing.T) {
	skipIfFUSEUnavailable(t)
	_, configPath, _ := getSharedFixture(t)

	// Skip if running as root (root bypasses all permission checks)
	if os.Geteuid() == 0 {
		t.Skip("Group access test requires non-root user")
	}

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
	if os.Geteuid() == 0 {
		t.Skip("Supplementary group access test requires non-root user")
	}

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
	if os.Geteuid() == 0 {
		t.Skip("Permission denial test requires non-root user")
	}

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

func TestFUSE_RootBypassesPermissions(t *testing.T) {
	skipIfFUSEUnavailable(t)
	_, configPath, _ := getSharedFixture(t)

	// This test requires root
	if os.Geteuid() != 0 {
		t.Skip("Root bypass test requires root privileges")
	}

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
