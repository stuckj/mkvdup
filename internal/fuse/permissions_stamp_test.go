package fuse

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A store that knows its mountpoint stamps the file, so a later reader can tell
// whose keys these are.
func TestStamp_WrittenOnSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mnt-videos.yaml")
	s := NewPermissionStore(path, DefaultPerms(), false)
	s.SetMountIdentity("/mnt/videos")

	if err := s.SetFilePerms("one.mkv", nil, nil, mode(0640)); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "mount: /mnt/videos") {
		t.Errorf("file not stamped with its mountpoint:\n%s", data)
	}
}

// No mountpoint (tests, programmatic use) means no stamp and no checking, so
// existing callers behave exactly as before.
func TestStamp_OmittedWithoutIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "permissions.yaml")
	s := NewPermissionStore(path, DefaultPerms(), false)

	if err := s.SetFilePerms("one.mkv", nil, nil, mode(0640)); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "mount:") {
		t.Errorf("unexpected stamp when no identity was set:\n%s", data)
	}
}

// The case isolation cannot reach: two mounts deliberately pointed at one
// explicit --permissions-file. The second must not prune the first's entries.
func TestStamp_SharedFileDisablesCleanup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shared.yaml")

	a := NewPermissionStore(path, DefaultPerms(), false)
	a.SetMountIdentity("/mnt/videos")
	if err := a.SetFilePerms("A/one.mkv", nil, nil, mode(0640)); err != nil {
		t.Fatal(err)
	}

	// A different mount opens the same file.
	b := NewPermissionStore(path, DefaultPerms(), false)
	b.SetMountIdentity("/mnt/movies")
	if err := b.Load(); err != nil {
		t.Fatal(err)
	}
	if !b.sharedMode() {
		t.Fatal("store did not detect that the file belongs to another mount")
	}

	// B's tree contains none of A's paths. Without the stamp this would delete
	// every one of them.
	if removed := b.CleanupStale(map[string]bool{"B/two.mkv": true}, nil); removed != 0 {
		t.Errorf("cleanup removed %d entries from a shared file; want 0", removed)
	}

	fresh := NewPermissionStore(path, DefaultPerms(), false)
	if err := fresh.Load(); err != nil {
		t.Fatal(err)
	}
	if _, _, m := fresh.GetFilePerms("A/one.mkv"); m != 0640 {
		t.Errorf("A's override was destroyed by B: mode = %#o, want 0640", m)
	}
}

// In shared mode this mount's defaults must not overwrite the file's.
func TestStamp_SharedFileDoesNotPersistDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shared.yaml")

	a := NewPermissionStore(path, Defaults{FileUID: 1000, FileGID: 1000, FileMode: 0444, DirMode: 0555}, false)
	a.SetMountIdentity("/mnt/videos")
	if err := a.SetFilePerms("A/one.mkv", nil, nil, mode(0640)); err != nil {
		t.Fatal(err)
	}

	b := NewPermissionStore(path, Defaults{FileUID: 4242, FileGID: 4242, FileMode: 0400, DirMode: 0500}, false)
	b.SetMountIdentity("/mnt/movies")
	if err := b.Load(); err != nil {
		t.Fatal(err)
	}
	if err := b.SetFilePerms("B/two.mkv", nil, nil, mode(0600)); err != nil {
		t.Fatal(err)
	}

	onDisk, err := readPermissionsFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if onDisk.Defaults.FileUID == 4242 {
		t.Error("shared mode persisted this mount's defaults over the file's")
	}
	// The stamp must survive, or the sharing becomes invisible next time.
	if onDisk.Mount != "/mnt/videos" {
		t.Errorf("stamp = %q, want /mnt/videos preserved", onDisk.Mount)
	}
}

// Same mountpoint reopening its own file is not shared.
func TestStamp_SameMountIsNotShared(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mnt-videos.yaml")

	a := NewPermissionStore(path, DefaultPerms(), false)
	a.SetMountIdentity("/mnt/videos")
	if err := a.SetFilePerms("one.mkv", nil, nil, mode(0640)); err != nil {
		t.Fatal(err)
	}

	b := NewPermissionStore(path, DefaultPerms(), false)
	b.SetMountIdentity("/mnt/videos")
	if err := b.Load(); err != nil {
		t.Fatal(err)
	}
	if b.sharedMode() {
		t.Error("a mount reopening its own file was treated as shared")
	}
	if removed := b.CleanupStale(map[string]bool{"one.mkv": true}, nil); removed != 0 {
		t.Errorf("removed %d valid entries", removed)
	}
}

// A legacy file has no stamp; the mount adopts it and stamps on next write.
func TestStamp_UnstampedFileIsAdopted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "permissions.yaml")
	if err := os.WriteFile(path, []byte("files:\n  \"one.mkv\":\n    mode: 416\n"), 0644); err != nil {
		t.Fatal(err)
	}

	s := NewPermissionStore(path, DefaultPerms(), false)
	s.SetMountIdentity("/mnt/videos")
	if err := s.Load(); err != nil {
		t.Fatal(err)
	}
	if s.sharedMode() {
		t.Fatal("an unstamped file should be adopted, not treated as shared")
	}

	if err := s.SetFilePerms("two.mkv", nil, nil, mode(0600)); err != nil {
		t.Fatal(err)
	}
	onDisk, err := readPermissionsFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if onDisk.Mount != "/mnt/videos" {
		t.Errorf("stamp = %q, want /mnt/videos after adoption", onDisk.Mount)
	}
	if _, ok := onDisk.Files["one.mkv"]; !ok {
		t.Error("pre-existing entry lost during adoption")
	}
}
