//go:build integration && nonroot

// End-to-end coverage for where the permissions file lands and who owns it.
//
// These drive the real `mkvdup mount` binary with no --permissions-file, which
// is the gap the rest of the suite leaves: the other permission integration
// tests mount in-process and are *handed* a path, so they never exercise
// ResolvePermissionsPath or the wiring in cmd_mount.go. A mistake there --
// forgetting to pass the mountpoint, say -- would pass every unit test.
//
// nonroot because the state directory is chosen by euid: root uses /var/lib,
// which a test must not write to, while non-root honours XDG_STATE_HOME.
//
// NAMING IS LOAD-BEARING: the non-root CI job runs
//
//	go test -tags=integration,nonroot -run 'TestFUSE_Permission' ./internal/fuse/...
//
// The -run filter is needed because -tags=integration,nonroot also compiles in
// every plain `integration` file, which that job should not re-run. The
// consequence is that a test here whose name does not start with
// TestFUSE_Permission compiles, vets and is never executed -- invisibly, since
// a filtered-out test is not a *skipped* test and check-no-skips.py will not
// flag it. That is the failure mode of #201 wearing a different hat. Keep the
// prefix.
package fuse_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	fusepkg "github.com/stuckj/mkvdup/internal/fuse"
)

// waitForFileContaining polls until path exists and contains want.
//
// Permission writes are coalesced, so a change is not on disk the instant the
// chmod returns. Polling is the honest way to observe that from outside the
// process: the alternative -- a flag forcing synchronous writes -- would test a
// code path production never takes.
func waitForFileContaining(t *testing.T, path, want string, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var last string
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(path); err == nil {
			last = string(data)
			if strings.Contains(last, want) {
				return last
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if last == "" {
		t.Fatalf("timed out after %s waiting for %s to exist", timeout, path)
	}
	t.Fatalf("timed out after %s waiting for %q in %s:\n%s", timeout, want, path, last)
	return ""
}

// expectedPermissionsPath is where the daemon should put a mount's file.
func expectedPermissionsPath(t *testing.T, mountPoint string) string {
	t.Helper()
	canonical := fusepkg.CanonicalMountpoint(mountPoint)
	return filepath.Join(fusepkg.StateDir(), "permissions.d",
		fusepkg.EscapeMountpoint(canonical)+".yaml")
}

// A mount with no --permissions-file must derive its own path from the
// mountpoint, and stamp the file with which mount owns it.
func TestFUSE_PermissionsFileDerivedFromMountpoint(t *testing.T) {
	requireNonRoot(t)
	skipIfFUSEUnavailable(t)

	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	binary := getWatchTestBinary(t)
	fix := createWatchFixture(t)

	// Deliberately no --permissions-file: that is the code path under test.
	startMount(t, binary, fix.MountPoint, []string{fix.ConfigPath}, "--no-source-watch")

	virtualPath := filepath.Join(fix.MountPoint, "test.mkv")
	if err := os.Chmod(virtualPath, 0640); err != nil {
		t.Fatalf("chmod virtual file: %v", err)
	}

	permsPath := expectedPermissionsPath(t, fix.MountPoint)
	body := waitForFileContaining(t, permsPath, "test.mkv", 10*time.Second)

	if !strings.Contains(body, "mode: 0640") {
		t.Errorf("mode not persisted in octal:\n%s", body)
	}
	// The stamp proves cmd_mount.go passed the mountpoint through.
	if !strings.Contains(body, "mount: "+fusepkg.CanonicalMountpoint(fix.MountPoint)) {
		t.Errorf("file not stamped with this mountpoint:\n%s", body)
	}

	// And it is genuinely under permissions.d, not the old single-file location.
	if _, err := os.Stat(filepath.Join(stateHome, "mkvdup", "permissions.yaml")); err == nil {
		t.Error("daemon wrote the legacy single-file path instead of a per-mount file")
	}
}

// The regression test for the original bug, through two real daemons: each
// mount keeps its own state and neither deletes the other's.
func TestFUSE_PermissionsIsolatedBetweenMounts(t *testing.T) {
	requireNonRoot(t)
	skipIfFUSEUnavailable(t)

	t.Setenv("XDG_STATE_HOME", t.TempDir())

	binary := getWatchTestBinary(t)
	a := createWatchFixture(t)
	b := createWatchFixture(t)

	startMount(t, binary, a.MountPoint, []string{a.ConfigPath}, "--no-source-watch")
	startMount(t, binary, b.MountPoint, []string{b.ConfigPath}, "--no-source-watch")

	// Both mounts expose a file at the same *virtual* path, which is exactly the
	// key collision that made a shared file unsafe.
	if err := os.Chmod(filepath.Join(a.MountPoint, "test.mkv"), 0640); err != nil {
		t.Fatalf("chmod on mount A: %v", err)
	}
	if err := os.Chmod(filepath.Join(b.MountPoint, "test.mkv"), 0600); err != nil {
		t.Fatalf("chmod on mount B: %v", err)
	}

	pathA := expectedPermissionsPath(t, a.MountPoint)
	pathB := expectedPermissionsPath(t, b.MountPoint)
	if pathA == pathB {
		t.Fatalf("both mounts resolved to the same file: %s", pathA)
	}

	bodyA := waitForFileContaining(t, pathA, "mode: 0640", 10*time.Second)
	bodyB := waitForFileContaining(t, pathB, "mode: 0600", 10*time.Second)

	// Neither mount's mode leaked into the other's file.
	if strings.Contains(bodyA, "mode: 0600") {
		t.Errorf("mount B's override appeared in mount A's file:\n%s", bodyA)
	}
	if strings.Contains(bodyB, "mode: 0640") {
		t.Errorf("mount A's override appeared in mount B's file:\n%s", bodyB)
	}

	// Both survive: previously one mount's startup cleanup deleted the other's
	// entries outright.
	if got := statMode(t, filepath.Join(a.MountPoint, "test.mkv")); got != 0640 {
		t.Errorf("mount A mode = %#o, want 0640", got)
	}
	if got := statMode(t, filepath.Join(b.MountPoint, "test.mkv")); got != 0600 {
		t.Errorf("mount B mode = %#o, want 0600", got)
	}
}

// An existing installation's shared file must be migrated, not ignored, and
// must be left intact for any other mount that still needs to seed from it.
func TestFUSE_PermissionsSeededFromLegacyFile(t *testing.T) {
	requireNonRoot(t)
	skipIfFUSEUnavailable(t)

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	// The pre-existing shared file: one entry for this mount, one for a mount
	// that no longer exists.
	legacyDir := filepath.Join(home, ".config", "mkvdup")
	if err := os.MkdirAll(legacyDir, 0755); err != nil {
		t.Fatal(err)
	}
	legacyPath := filepath.Join(legacyDir, "permissions.yaml")
	legacyBody := "files:\n  \"test.mkv\":\n    mode: 0640\n  \"Other/gone.mkv\":\n    mode: 0600\n"
	if err := os.WriteFile(legacyPath, []byte(legacyBody), 0644); err != nil {
		t.Fatal(err)
	}

	binary := getWatchTestBinary(t)
	fix := createWatchFixture(t)
	startMount(t, binary, fix.MountPoint, []string{fix.ConfigPath}, "--no-source-watch")

	// The override from the legacy file is in effect immediately after mount.
	if got := statMode(t, filepath.Join(fix.MountPoint, "test.mkv")); got != 0640 {
		t.Errorf("legacy override not applied: mode = %#o, want 0640", got)
	}

	permsPath := expectedPermissionsPath(t, fix.MountPoint)
	body := waitForFileContaining(t, permsPath, "test.mkv", 10*time.Second)

	// Only this mount's entry was taken.
	if strings.Contains(body, "gone.mkv") {
		t.Errorf("seeded an entry that does not belong to this mount:\n%s", body)
	}

	// The legacy file is a shared resource and must survive untouched.
	after, err := os.ReadFile(legacyPath)
	if err != nil {
		t.Fatalf("legacy file disappeared: %v", err)
	}
	if string(after) != legacyBody {
		t.Errorf("legacy file was modified:\ngot:\n%s\nwant:\n%s", after, legacyBody)
	}
}

// statMode returns a path's permission bits.
func statMode(t *testing.T, path string) uint32 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return uint32(info.Mode().Perm())
}
