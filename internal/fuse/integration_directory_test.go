//go:build integration

package fuse_test

import (
	"os"
	"path/filepath"
	"testing"

	fuselib "github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	fusepkg "github.com/stuckj/mkvdup/internal/fuse"
)

// TestFUSEMount_DirectoryStructure tests mounting with nested directory paths.
func TestFUSEMount_DirectoryStructure(t *testing.T) {
	skipIfFUSEUnavailable(t)
	getSharedFixture(t) // Verify fixture is available

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "mkvdup-fuse-dir-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create configs with different virtual paths (all point to same shared dedup file)
	config1 := copyConfigWithName(t, filepath.Join(tmpDir, "action"), "Movies/Action/test.mkv")
	config2 := copyConfigWithName(t, filepath.Join(tmpDir, "comedy"), "Movies/Comedy/funny.mkv")
	config3 := copyConfigWithName(t, filepath.Join(tmpDir, "root"), "root.mkv")

	// Create mount point
	mountPoint := filepath.Join(tmpDir, "mount")
	if err := os.Mkdir(mountPoint, 0755); err != nil {
		t.Fatalf("Failed to create mount point: %v", err)
	}

	// Create FUSE filesystem
	root, err := fusepkg.NewMKVFS([]string{config1, config2, config3}, false)
	if err != nil {
		t.Fatalf("Failed to create MKVFS: %v", err)
	}

	// Mount
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

	// Unmount when done
	defer func() {
		if err := server.Unmount(); err != nil {
			t.Logf("Warning: unmount failed: %v", err)
		}
	}()

	// Wait for mount to be ready
	server.WaitMount()

	// Test: List root directory - should have "Movies" dir and "root.mkv" file
	entries, err := os.ReadDir(mountPoint)
	if err != nil {
		t.Fatalf("Failed to read mount directory: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("Expected 2 entries at root (Movies dir + root.mkv), got %d", len(entries))
		for _, e := range entries {
			t.Logf("  - %s (dir=%v)", e.Name(), e.IsDir())
		}
	}

	// Check for Movies directory
	moviesPath := filepath.Join(mountPoint, "Movies")
	info, err := os.Stat(moviesPath)
	if err != nil {
		t.Fatalf("Failed to stat Movies directory: %v", err)
	}
	if !info.IsDir() {
		t.Error("Movies should be a directory")
	}

	// Test: List Movies directory - should have Action and Comedy
	moviesEntries, err := os.ReadDir(moviesPath)
	if err != nil {
		t.Fatalf("Failed to read Movies directory: %v", err)
	}

	if len(moviesEntries) != 2 {
		t.Errorf("Expected 2 subdirs in Movies (Action, Comedy), got %d", len(moviesEntries))
	}

	// Test: List Action directory - should have test.mkv
	actionPath := filepath.Join(mountPoint, "Movies", "Action")
	actionEntries, err := os.ReadDir(actionPath)
	if err != nil {
		t.Fatalf("Failed to read Action directory: %v", err)
	}

	if len(actionEntries) != 1 {
		t.Errorf("Expected 1 file in Action, got %d", len(actionEntries))
	}
	if len(actionEntries) > 0 && actionEntries[0].Name() != "test.mkv" {
		t.Errorf("Expected 'test.mkv' in Action, got %s", actionEntries[0].Name())
	}
}
