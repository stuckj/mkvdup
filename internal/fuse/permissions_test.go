package fuse

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
)

// testCallerKey is used to inject caller credentials in tests.
type testCallerKeyType struct{}

var testCallerKey = testCallerKeyType{}

func init() {
	// Set up the test caller hook to allow ContextWithCaller to work
	testCallerHook = func(ctx context.Context) (CallerInfo, bool) {
		if caller, ok := ctx.Value(testCallerKey).(CallerInfo); ok {
			return caller, true
		}
		return CallerInfo{}, false
	}
}

// ContextWithCaller creates a context with injected caller credentials for testing.
func ContextWithCaller(ctx context.Context, uid, gid uint32) context.Context {
	return context.WithValue(ctx, testCallerKey, CallerInfo{Uid: uid, Gid: gid})
}

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

// --- Permission Checking Utility Tests ---

func TestCallerInfo_IsRoot(t *testing.T) {
	tests := []struct {
		name   string
		caller CallerInfo
		want   bool
	}{
		{"root user", CallerInfo{Uid: 0, Gid: 0}, true},
		{"root user with non-root group", CallerInfo{Uid: 0, Gid: 1000}, true},
		{"non-root user", CallerInfo{Uid: 1000, Gid: 1000}, false},
		{"user with gid 0", CallerInfo{Uid: 1000, Gid: 0}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.caller.IsRoot(); got != tt.want {
				t.Errorf("CallerInfo.IsRoot() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetCaller(t *testing.T) {
	tests := []struct {
		name    string
		ctx     context.Context
		wantUID uint32
		wantGID uint32
		wantOK  bool
	}{
		{"empty context fails closed", context.Background(), 0, 0, false},
		{"injected caller", ContextWithCaller(context.Background(), 1000, 1000), 1000, 1000, true},
		{"injected root", ContextWithCaller(context.Background(), 0, 0), 0, 0, true},
		{"different uid and gid", ContextWithCaller(context.Background(), 1000, 2000), 1000, 2000, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := GetCaller(tt.ctx)
			if ok != tt.wantOK {
				t.Errorf("GetCaller() ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && (got.Uid != tt.wantUID || got.Gid != tt.wantGID) {
				t.Errorf("GetCaller() = {Uid: %d, Gid: %d}, want {Uid: %d, Gid: %d}",
					got.Uid, got.Gid, tt.wantUID, tt.wantGID)
			}
		})
	}
}

func TestContextWithCaller(t *testing.T) {
	ctx := context.Background()
	ctxWithCaller := ContextWithCaller(ctx, 1234, 5678)

	caller, ok := GetCaller(ctxWithCaller)
	if !ok {
		t.Error("GetCaller returned ok=false for injected caller")
	}
	if caller.Uid != 1234 || caller.Gid != 5678 {
		t.Errorf("ContextWithCaller created wrong caller: got {%d, %d}, want {1234, 5678}",
			caller.Uid, caller.Gid)
	}

	// Original context should return ok=false (fail closed)
	_, ok = GetCaller(ctx)
	if ok {
		t.Error("Original context should return ok=false, got ok=true")
	}
}

func TestCheckAccess(t *testing.T) {
	tests := []struct {
		name    string
		caller  CallerInfo
		fileUID uint32
		fileGID uint32
		mode    uint32
		access  AccessMode
		want    syscall.Errno
	}{
		// Root bypass tests
		{"root bypasses mode 0000 read", CallerInfo{0, 0}, 1000, 1000, 0000, AccessRead, 0},
		{"root bypasses mode 0000 write", CallerInfo{0, 0}, 1000, 1000, 0000, AccessWrite, 0},
		{"root bypasses mode 0000 execute", CallerInfo{0, 0}, 1000, 1000, 0000, AccessExecute, 0},

		// Owner permission tests
		{"owner read allowed with 0400", CallerInfo{1000, 1000}, 1000, 1000, 0400, AccessRead, 0},
		{"owner read denied with 0300", CallerInfo{1000, 1000}, 1000, 1000, 0300, AccessRead, syscall.EACCES},
		{"owner write allowed with 0200", CallerInfo{1000, 1000}, 1000, 1000, 0200, AccessWrite, 0},
		{"owner write denied with 0500", CallerInfo{1000, 1000}, 1000, 1000, 0500, AccessWrite, syscall.EACCES},
		{"owner execute allowed with 0100", CallerInfo{1000, 1000}, 1000, 1000, 0100, AccessExecute, 0},
		{"owner execute denied with 0600", CallerInfo{1000, 1000}, 1000, 1000, 0600, AccessExecute, syscall.EACCES},
		{"owner full perms with 0700", CallerInfo{1000, 1000}, 1000, 1000, 0700, AccessRead, 0},

		// Group permission tests (caller UID differs from file UID, GID matches)
		{"group read allowed with 0040", CallerInfo{2000, 1000}, 1000, 1000, 0040, AccessRead, 0},
		{"group read denied with 0030", CallerInfo{2000, 1000}, 1000, 1000, 0030, AccessRead, syscall.EACCES},
		{"group write allowed with 0020", CallerInfo{2000, 1000}, 1000, 1000, 0020, AccessWrite, 0},
		{"group write denied with 0050", CallerInfo{2000, 1000}, 1000, 1000, 0050, AccessWrite, syscall.EACCES},
		{"group execute allowed with 0010", CallerInfo{2000, 1000}, 1000, 1000, 0010, AccessExecute, 0},
		{"group execute denied with 0060", CallerInfo{2000, 1000}, 1000, 1000, 0060, AccessExecute, syscall.EACCES},
		{"group inherits nothing from owner", CallerInfo{2000, 1000}, 1000, 1000, 0700, AccessRead, syscall.EACCES},

		// Other permission tests (caller UID and GID both differ)
		{"other read allowed with 0004", CallerInfo{3000, 3000}, 1000, 1000, 0004, AccessRead, 0},
		{"other read denied with 0770", CallerInfo{3000, 3000}, 1000, 1000, 0770, AccessRead, syscall.EACCES},
		{"other write allowed with 0002", CallerInfo{3000, 3000}, 1000, 1000, 0002, AccessWrite, 0},
		{"other write denied with 0775", CallerInfo{3000, 3000}, 1000, 1000, 0775, AccessWrite, syscall.EACCES},
		{"other execute allowed with 0001", CallerInfo{3000, 3000}, 1000, 1000, 0001, AccessExecute, 0},
		{"other execute denied with 0776", CallerInfo{3000, 3000}, 1000, 1000, 0776, AccessExecute, syscall.EACCES},
		{"other inherits nothing from group", CallerInfo{3000, 3000}, 1000, 1000, 0070, AccessRead, syscall.EACCES},

		// Priority tests (owner bits take precedence over group/other)
		{"owner checked before group", CallerInfo{1000, 1000}, 1000, 1000, 0470, AccessRead, 0},
		{"owner denied even with group allowed", CallerInfo{1000, 1000}, 1000, 1000, 0070, AccessRead, syscall.EACCES},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckAccess(tt.caller, tt.fileUID, tt.fileGID, tt.mode, tt.access)
			if got != tt.want {
				t.Errorf("CheckAccess() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckChown(t *testing.T) {
	uid := func(u uint32) *uint32 { return &u }
	gid := func(g uint32) *uint32 { return &g }

	tests := []struct {
		name    string
		caller  CallerInfo
		fileUID uint32
		fileGID uint32
		newUID  *uint32
		newGID  *uint32
		want    syscall.Errno
	}{
		// Root can do anything
		{"root can change UID", CallerInfo{0, 0}, 1000, 1000, uid(2000), nil, 0},
		{"root can change GID", CallerInfo{0, 0}, 1000, 1000, nil, gid(2000), 0},
		{"root can change both", CallerInfo{0, 0}, 1000, 1000, uid(2000), gid(2000), 0},

		// Non-root UID changes
		{"non-root cannot change UID to different user", CallerInfo{1000, 1000}, 1000, 1000, uid(2000), nil, syscall.EPERM},
		{"non-root can set UID to same value", CallerInfo{1000, 1000}, 1000, 1000, uid(1000), nil, 0},
		{"non-owner cannot change UID", CallerInfo{2000, 2000}, 1000, 1000, uid(2000), nil, syscall.EPERM},

		// Non-root GID changes - owner can only change to their own GID
		{"owner can change GID to own GID", CallerInfo{1000, 1000}, 1000, 2000, nil, gid(1000), 0},
		{"owner cannot change GID to arbitrary GID", CallerInfo{1000, 1000}, 1000, 1000, nil, gid(2000), syscall.EPERM},
		{"non-owner cannot change GID", CallerInfo{2000, 2000}, 1000, 1000, nil, gid(2000), syscall.EPERM},

		// No-op GID changes (setting to same value is always allowed)
		{"non-owner can set GID to same value", CallerInfo{2000, 2000}, 1000, 1000, nil, gid(1000), 0},
		{"anyone can set GID to same value", CallerInfo{3000, 3000}, 1000, 1000, nil, gid(1000), 0},

		// Combined UID+GID changes
		{"owner cannot change UID even with valid GID", CallerInfo{1000, 1000}, 1000, 1000, uid(2000), gid(1000), syscall.EPERM},

		// Nil values (no change requested)
		{"nil UID and GID always allowed", CallerInfo{2000, 2000}, 1000, 1000, nil, nil, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckChown(tt.caller, tt.fileUID, tt.fileGID, tt.newUID, tt.newGID)
			if got != tt.want {
				t.Errorf("CheckChown() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckChmod(t *testing.T) {
	tests := []struct {
		name    string
		caller  CallerInfo
		fileUID uint32
		want    syscall.Errno
	}{
		// Root can chmod anything
		{"root can chmod any file", CallerInfo{0, 0}, 1000, 0},
		{"root can chmod root-owned file", CallerInfo{0, 0}, 0, 0},

		// Owner can chmod
		{"owner can chmod own file", CallerInfo{1000, 1000}, 1000, 0},

		// Non-owner cannot chmod
		{"non-owner cannot chmod", CallerInfo{2000, 2000}, 1000, syscall.EPERM},
		{"same group but not owner cannot chmod", CallerInfo{2000, 1000}, 1000, syscall.EPERM},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckChmod(tt.caller, tt.fileUID)
			if got != tt.want {
				t.Errorf("CheckChmod() = %v, want %v", got, tt.want)
			}
		})
	}
}
