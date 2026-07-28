package fuse

import (
	"os"
	"path/filepath"
	"testing"
)

// writeLegacy writes a legacy-format shared permissions file and points the
// legacy search path at it for the duration of the test.
func writeLegacy(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "permissions.yaml")
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	orig := legacyPathsForTest
	legacyPathsForTest = []string{path}
	t.Cleanup(func() { legacyPathsForTest = orig })
	return path
}

// A shared file holding entries for two different mounts. Each mount should
// take only its own.
const twoMountLegacy = `defaults:
  file_uid: 1000
  file_gid: 1000
files:
  "A/one.mkv":
    mode: 416
  "B/two.mkv":
    mode: 384
directories:
  "A":
    mode: 493
  "B":
    mode: 448
`

func TestSeedFromLegacy_CopiesOnlyThisMountsEntries(t *testing.T) {
	legacy := writeLegacy(t, twoMountLegacy)

	store := NewPermissionStore(filepath.Join(t.TempDir(), "mnt-a.yaml"), DefaultPerms(), false)
	copied, err := store.SeedFromLegacy(
		map[string]bool{"A/one.mkv": true},
		map[string]bool{"A": true},
	)
	if err != nil {
		t.Fatalf("SeedFromLegacy: %v", err)
	}
	if copied != 2 {
		t.Errorf("copied = %d, want 2 (one file + one dir)", copied)
	}

	if _, _, mode := store.GetFilePerms("A/one.mkv"); mode != 0640 {
		t.Errorf("A/one.mkv mode = %#o, want 0640", mode)
	}
	// Mount B's entry must not have come along: same key space, different mount.
	if _, _, mode := store.GetFilePerms("B/two.mkv"); mode == 0600 {
		t.Error("B/two.mkv leaked into mount A's store")
	}
	if _, _, mode := store.GetDirPerms("B"); mode == 0700 {
		t.Error("directory B leaked into mount A's store")
	}

	// defaults come across
	if uid, _, _ := store.GetFilePerms("anything"); uid != 1000 {
		t.Errorf("default file_uid = %d, want 1000 (seeded)", uid)
	}

	// The legacy file is a shared resource: other mounts still need it intact.
	after, err := os.ReadFile(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != twoMountLegacy {
		t.Errorf("legacy file was modified:\n%s", after)
	}
}

func TestSeedFromLegacy_EachMountGetsItsOwnSubset(t *testing.T) {
	writeLegacy(t, twoMountLegacy)
	dir := t.TempDir()

	a := NewPermissionStore(filepath.Join(dir, "mnt-a.yaml"), DefaultPerms(), false)
	if _, err := a.SeedFromLegacy(map[string]bool{"A/one.mkv": true}, nil); err != nil {
		t.Fatal(err)
	}
	b := NewPermissionStore(filepath.Join(dir, "mnt-b.yaml"), DefaultPerms(), false)
	if _, err := b.SeedFromLegacy(map[string]bool{"B/two.mkv": true}, nil); err != nil {
		t.Fatal(err)
	}

	// The scenario that used to destroy data: B's cleanup no longer sees A.
	if removed := b.CleanupStale(map[string]bool{"B/two.mkv": true}, nil); removed != 0 {
		t.Errorf("B's cleanup removed %d entries; it should never see A's", removed)
	}

	freshA := NewPermissionStore(a.path, DefaultPerms(), false)
	if err := freshA.Load(); err != nil {
		t.Fatal(err)
	}
	if _, _, mode := freshA.GetFilePerms("A/one.mkv"); mode != 0640 {
		t.Errorf("mount A lost its override: mode = %#o, want 0640", mode)
	}
}

func TestSeedFromLegacy_SkipsWhenFileAlreadyExists(t *testing.T) {
	writeLegacy(t, twoMountLegacy)

	path := filepath.Join(t.TempDir(), "mnt-a.yaml")
	// An existing file is authoritative, even an empty one.
	if err := os.WriteFile(path, []byte("files: {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	store := NewPermissionStore(path, DefaultPerms(), false)
	copied, err := store.SeedFromLegacy(map[string]bool{"A/one.mkv": true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if copied != 0 {
		t.Errorf("copied = %d, want 0: an existing per-mount file must not be re-seeded", copied)
	}
}

func TestSeedFromLegacy_NoLegacyFileIsNotAnError(t *testing.T) {
	orig := legacyPathsForTest
	legacyPathsForTest = []string{filepath.Join(t.TempDir(), "does-not-exist.yaml")}
	t.Cleanup(func() { legacyPathsForTest = orig })

	path := filepath.Join(t.TempDir(), "mnt-a.yaml")
	store := NewPermissionStore(path, DefaultPerms(), false)
	copied, err := store.SeedFromLegacy(map[string]bool{"A/one.mkv": true}, nil)
	if err != nil {
		t.Fatalf("fresh install should not error: %v", err)
	}
	if copied != 0 {
		t.Errorf("copied = %d, want 0", copied)
	}
	// Nothing to migrate means nothing written.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("no legacy file present, but a permissions file was created")
	}
}

func TestSeedFromLegacy_WritesFileEvenWhenNothingMatches(t *testing.T) {
	writeLegacy(t, twoMountLegacy)

	path := filepath.Join(t.TempDir(), "mnt-c.yaml")
	store := NewPermissionStore(path, DefaultPerms(), false)
	// A mount sharing no paths with the legacy file.
	copied, err := store.SeedFromLegacy(map[string]bool{"C/three.mkv": true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if copied != 0 {
		t.Errorf("copied = %d, want 0", copied)
	}
	// The file must still be created, or every start re-reads the legacy file.
	if _, err := os.Stat(path); err != nil {
		t.Errorf("permissions file not created after seeding: %v", err)
	}
}

func TestSeedFromLegacy_DoesNotSeedAFileFromItself(t *testing.T) {
	legacy := writeLegacy(t, twoMountLegacy)

	// --permissions-file pointing at the legacy path itself. It exists, so the
	// early return covers this; the guard matters if it is later removed.
	store := NewPermissionStore(legacy, DefaultPerms(), false)
	copied, err := store.SeedFromLegacy(map[string]bool{"A/one.mkv": true}, nil)
	if err != nil {
		t.Fatalf("SeedFromLegacy: %v", err)
	}
	if copied != 0 {
		t.Errorf("copied = %d, want 0", copied)
	}
}
