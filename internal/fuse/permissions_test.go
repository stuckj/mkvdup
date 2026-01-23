package fuse

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestNewPermissionStore(t *testing.T) {
	defaults := DefaultPerms()
	store := NewPermissionStore("", defaults, false)

	if store == nil {
		t.Fatal("NewPermissionStore returned nil")
	}

	// Check defaults are set
	d := store.Defaults()
	if d.FileMode != 0444 {
		t.Errorf("Default file mode = %o, want %o", d.FileMode, 0444)
	}
	if d.DirMode != 0555 {
		t.Errorf("Default dir mode = %o, want %o", d.DirMode, 0555)
	}
}

func TestPermissionStore_LoadSave(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "permissions.yaml")

	defaults := DefaultPerms()
	store := NewPermissionStore(path, defaults, false)

	// Set some permissions
	uid := uint32(1000)
	gid := uint32(1001)
	mode := uint32(0644)

	err := store.SetFilePerms("video.mkv", &uid, &gid, &mode)
	if err != nil {
		t.Fatalf("SetFilePerms failed: %v", err)
	}

	dirMode := uint32(0755)
	err = store.SetDirPerms("Movies", nil, nil, &dirMode)
	if err != nil {
		t.Fatalf("SetDirPerms failed: %v", err)
	}

	// Create a new store and load
	store2 := NewPermissionStore(path, defaults, false)
	err = store2.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify file permissions
	gotUID, gotGID, gotMode := store2.GetFilePerms("video.mkv")
	if gotUID != uid || gotGID != gid || gotMode != mode {
		t.Errorf("GetFilePerms = (%d, %d, %o), want (%d, %d, %o)",
			gotUID, gotGID, gotMode, uid, gid, mode)
	}

	// Verify directory permissions (only mode was set)
	gotUID, gotGID, gotMode = store2.GetDirPerms("Movies")
	if gotMode != dirMode {
		t.Errorf("GetDirPerms mode = %o, want %o", gotMode, dirMode)
	}
	// UID/GID should be defaults
	if gotUID != defaults.DirUID || gotGID != defaults.DirGID {
		t.Errorf("GetDirPerms uid/gid = (%d, %d), want (%d, %d)",
			gotUID, gotGID, defaults.DirUID, defaults.DirGID)
	}
}

func TestPermissionStore_GetFilePerms_Default(t *testing.T) {
	defaults := Defaults{
		FileUID:  1000,
		FileGID:  1000,
		FileMode: 0444,
		DirUID:   1000,
		DirGID:   1000,
		DirMode:  0555,
	}
	store := NewPermissionStore("", defaults, false)

	uid, gid, mode := store.GetFilePerms("nonexistent.mkv")

	if uid != 1000 || gid != 1000 || mode != 0444 {
		t.Errorf("GetFilePerms = (%d, %d, %o), want (1000, 1000, 0444)",
			uid, gid, mode)
	}
}

func TestPermissionStore_GetFilePerms_Override(t *testing.T) {
	defaults := DefaultPerms()
	store := NewPermissionStore("", defaults, false)

	// Set partial override (only mode)
	mode := uint32(0640)
	_ = store.SetFilePerms("video.mkv", nil, nil, &mode)

	uid, gid, gotMode := store.GetFilePerms("video.mkv")

	// Mode should be overridden, uid/gid should be defaults
	if gotMode != mode {
		t.Errorf("mode = %o, want %o", gotMode, mode)
	}
	if uid != defaults.FileUID || gid != defaults.FileGID {
		t.Errorf("uid/gid = (%d, %d), want (%d, %d)",
			uid, gid, defaults.FileUID, defaults.FileGID)
	}
}

func TestPermissionStore_SetFilePerms(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "permissions.yaml")

	store := NewPermissionStore(path, DefaultPerms(), false)

	uid := uint32(1000)
	err := store.SetFilePerms("video.mkv", &uid, nil, nil)
	if err != nil {
		t.Fatalf("SetFilePerms failed: %v", err)
	}

	gotUID, _, _ := store.GetFilePerms("video.mkv")
	if gotUID != uid {
		t.Errorf("GetFilePerms uid = %d, want %d", gotUID, uid)
	}

	// Verify file was saved
	if _, err := os.Stat(path); err != nil {
		t.Errorf("Permissions file was not created: %v", err)
	}
}

func TestPermissionStore_GetDirPerms(t *testing.T) {
	defaults := Defaults{
		FileUID:  0,
		FileGID:  0,
		FileMode: 0444,
		DirUID:   1000,
		DirGID:   1001,
		DirMode:  0755,
	}
	store := NewPermissionStore("", defaults, false)

	uid, gid, mode := store.GetDirPerms("Movies")

	if uid != 1000 || gid != 1001 || mode != 0755 {
		t.Errorf("GetDirPerms = (%d, %d, %o), want (1000, 1001, 0755)",
			uid, gid, mode)
	}
}

func TestPermissionStore_SetDirPerms(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "permissions.yaml")

	store := NewPermissionStore(path, DefaultPerms(), false)

	mode := uint32(0755)
	err := store.SetDirPerms("Movies/Action", nil, nil, &mode)
	if err != nil {
		t.Fatalf("SetDirPerms failed: %v", err)
	}

	_, _, gotMode := store.GetDirPerms("Movies/Action")
	if gotMode != mode {
		t.Errorf("GetDirPerms mode = %o, want %o", gotMode, mode)
	}
}

func TestPermissionStore_CleanupStale(t *testing.T) {
	store := NewPermissionStore("", DefaultPerms(), false)

	// Add some permissions
	uid := uint32(1000)
	_ = store.SetFilePerms("valid.mkv", &uid, nil, nil)
	_ = store.SetFilePerms("stale.mkv", &uid, nil, nil)
	_ = store.SetDirPerms("ValidDir", &uid, nil, nil)
	_ = store.SetDirPerms("StaleDir", &uid, nil, nil)

	validFiles := map[string]bool{"valid.mkv": true}
	validDirs := map[string]bool{"ValidDir": true}

	removed := store.CleanupStale(validFiles, validDirs)

	if removed != 2 {
		t.Errorf("CleanupStale removed %d, want 2", removed)
	}

	// Verify stale entries are gone
	gotUID, _, _ := store.GetFilePerms("stale.mkv")
	if gotUID != 0 { // Default is 0
		t.Error("Stale file entry was not removed")
	}

	gotUID, _, _ = store.GetDirPerms("StaleDir")
	if gotUID != 0 { // Default is 0
		t.Error("Stale dir entry was not removed")
	}

	// Verify valid entries remain
	gotUID, _, _ = store.GetFilePerms("valid.mkv")
	if gotUID != uid {
		t.Error("Valid file entry was incorrectly removed")
	}

	gotUID, _, _ = store.GetDirPerms("ValidDir")
	if gotUID != uid {
		t.Error("Valid dir entry was incorrectly removed")
	}
}

func TestPermissionStore_Concurrency(t *testing.T) {
	store := NewPermissionStore("", DefaultPerms(), false)

	var wg sync.WaitGroup
	numGoroutines := 100

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			uid := uint32(i)
			_ = store.SetFilePerms("file.mkv", &uid, nil, nil)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, _ = store.GetFilePerms("file.mkv")
		}()
	}

	wg.Wait()

	// Should not panic or race
}

func TestPermissionStore_LoadNonexistent(t *testing.T) {
	store := NewPermissionStore("/nonexistent/path/permissions.yaml", DefaultPerms(), false)

	err := store.Load()
	if err != nil {
		t.Errorf("Load should not fail for nonexistent file: %v", err)
	}
}

func TestPermissionStore_RemoveFilePerms(t *testing.T) {
	store := NewPermissionStore("", DefaultPerms(), false)

	// Set permissions
	uid := uint32(1000)
	_ = store.SetFilePerms("video.mkv", &uid, nil, nil)

	// Verify it's set
	gotUID, _, _ := store.GetFilePerms("video.mkv")
	if gotUID != uid {
		t.Fatalf("SetFilePerms didn't work")
	}

	// Remove the entry
	_ = store.RemoveFilePerms("video.mkv")

	// Should now return defaults (entry should be deleted)
	gotUID, _, _ = store.GetFilePerms("video.mkv")
	if gotUID != 0 { // Default is 0
		t.Errorf("Entry was not removed, got uid=%d", gotUID)
	}
}

func TestPermissionStore_PartialUpdate(t *testing.T) {
	store := NewPermissionStore("", DefaultPerms(), false)

	// Set all three values
	uid := uint32(1000)
	gid := uint32(1001)
	mode := uint32(0640)
	_ = store.SetFilePerms("video.mkv", &uid, &gid, &mode)

	// Update only mode
	newMode := uint32(0644)
	_ = store.SetFilePerms("video.mkv", nil, nil, &newMode)

	// UID and GID should be preserved, mode should be updated
	gotUID, gotGID, gotMode := store.GetFilePerms("video.mkv")
	if gotUID != uid {
		t.Errorf("UID changed unexpectedly: got %d, want %d", gotUID, uid)
	}
	if gotGID != gid {
		t.Errorf("GID changed unexpectedly: got %d, want %d", gotGID, gid)
	}
	if gotMode != newMode {
		t.Errorf("Mode not updated: got %o, want %o", gotMode, newMode)
	}
}

func TestResolvePermissionsPath_Explicit(t *testing.T) {
	path := ResolvePermissionsPath("/custom/path/permissions.yaml")
	if path != "/custom/path/permissions.yaml" {
		t.Errorf("ResolvePermissionsPath with explicit path = %q, want /custom/path/permissions.yaml", path)
	}
}

func TestResolvePermissionsPath_Default(t *testing.T) {
	// When no file exists and no explicit path, should return based on euid
	path := ResolvePermissionsPath("")

	// Path should be either /etc/mkvdup/permissions.yaml (root) or ~/.config/mkvdup/permissions.yaml (non-root)
	if path != "/etc/mkvdup/permissions.yaml" {
		home, _ := os.UserHomeDir()
		expected := filepath.Join(home, ".config", "mkvdup", "permissions.yaml")
		if path != expected {
			t.Errorf("ResolvePermissionsPath() = %q, want %q or /etc/mkvdup/permissions.yaml", path, expected)
		}
	}
}

func TestPermissionStore_SaveCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	nestedPath := filepath.Join(tmpDir, "nested", "dir", "permissions.yaml")

	store := NewPermissionStore(nestedPath, DefaultPerms(), false)

	uid := uint32(1000)
	err := store.SetFilePerms("video.mkv", &uid, nil, nil)
	if err != nil {
		t.Fatalf("SetFilePerms failed: %v", err)
	}

	// Verify nested directory was created
	if _, err := os.Stat(filepath.Dir(nestedPath)); err != nil {
		t.Errorf("Nested directory was not created: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(nestedPath); err != nil {
		t.Errorf("Permissions file was not created: %v", err)
	}
}
