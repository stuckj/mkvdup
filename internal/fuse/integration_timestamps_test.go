//go:build integration

package fuse_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// writeVirtualFilesConfig writes a config exposing each of names as a virtual
// file backed by the fixture's dedup file. Names may contain directory
// components (e.g. "Movies/Action/a.mkv"), which the filesystem materializes
// as virtual directories.
func writeVirtualFilesConfig(t *testing.T, configPath string, fix watchFixture, names ...string) {
	t.Helper()
	var b strings.Builder
	b.WriteString("virtual_files:\n")
	for _, n := range names {
		fmt.Fprintf(&b, "  - name: %q\n    dedup_file: %q\n    source_dir: %q\n",
			n, fix.DedupPath, fix.SourceDir)
	}
	if err := os.WriteFile(configPath, []byte(b.String()), 0644); err != nil {
		t.Fatalf("write config %s: %v", configPath, err)
	}
}

// waitForPath polls until path exists (want=true) or is gone (want=false).
func waitForPath(path string, want bool, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		_, err := os.Stat(path)
		if (err == nil) == want {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// waitForMtimeAfter polls path until its mtime is newer than "after".
func waitForMtimeAfter(t *testing.T, path string, after int64, timeout time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if statMtimeUnix(t, path) > after {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// statMtime returns the mtime (truncated to seconds) of the file at path.
func statMtimeUnix(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.ModTime().Unix()
}

// TestFUSE_DerivedMtimeFromDedup verifies that a virtual file's mtime is
// derived from its .mkvdup dedup file's mtime (not time.Now()), and is stable
// across repeated stats.
func TestFUSE_DerivedMtimeFromDedup(t *testing.T) {
	skipIfFUSEUnavailable(t)
	binary := getWatchTestBinary(t)
	fix := createWatchFixture(t)

	// Pin the dedup file's mtime to a known value before mounting.
	want := time.Date(2021, 6, 1, 12, 0, 0, 0, time.UTC)
	if err := os.Chtimes(fix.DedupPath, want, want); err != nil {
		t.Fatalf("chtimes dedup: %v", err)
	}

	startMount(t, binary, fix.MountPoint, []string{fix.ConfigPath}, "--no-source-watch")

	virtualPath := filepath.Join(fix.MountPoint, "test.mkv")

	if got := statMtimeUnix(t, virtualPath); got != want.Unix() {
		t.Errorf("virtual file mtime = %d, want dedup mtime %d", got, want.Unix())
	}
	// Stable across a second stat (would differ if it were time.Now()).
	if got := statMtimeUnix(t, virtualPath); got != want.Unix() {
		t.Errorf("virtual file mtime not stable: %d, want %d", got, want.Unix())
	}
}

// TestFUSE_TouchPersistsMtime verifies that touching a virtual file (utimes)
// succeeds, is reflected on the next stat, and is persisted to permissions.yaml
// as an mtime override.
func TestFUSE_TouchPersistsMtime(t *testing.T) {
	skipIfFUSEUnavailable(t)
	binary := getWatchTestBinary(t)
	fix := createWatchFixture(t)

	permsPath := filepath.Join(fix.TmpDir, "permissions.yaml")
	startMount(t, binary, fix.MountPoint, []string{fix.ConfigPath},
		"--no-source-watch", "--permissions-file", permsPath)

	virtualPath := filepath.Join(fix.MountPoint, "test.mkv")

	want := time.Date(2019, 3, 4, 5, 6, 7, 0, time.UTC)
	if err := os.Chtimes(virtualPath, want, want); err != nil {
		t.Fatalf("touch virtual file: %v", err)
	}

	if got := statMtimeUnix(t, virtualPath); got != want.Unix() {
		t.Errorf("mtime after touch = %d, want %d", got, want.Unix())
	}

	// Persisted to the permissions file as an mtime override.
	data, err := os.ReadFile(permsPath)
	if err != nil {
		t.Fatalf("read permissions file: %v", err)
	}
	if !strings.Contains(string(data), "mtime:") {
		t.Errorf("permissions file missing mtime override:\n%s", data)
	}
}

// TestFUSE_DedupTouchRefreshesMtime verifies that when the dedup file's mtime
// changes while mounted, the source watcher refreshes the virtual file's
// derived mtime live (dedup-file watching, timestamp only).
func TestFUSE_DedupTouchRefreshesMtime(t *testing.T) {
	skipIfFUSEUnavailable(t)
	binary := getWatchTestBinary(t)
	fix := createWatchFixture(t)

	orig := time.Date(2021, 6, 1, 12, 0, 0, 0, time.UTC)
	if err := os.Chtimes(fix.DedupPath, orig, orig); err != nil {
		t.Fatalf("chtimes dedup: %v", err)
	}

	// Default action is checksum; source watch (and thus dedup watch) is on.
	startMount(t, binary, fix.MountPoint, []string{fix.ConfigPath})

	virtualPath := filepath.Join(fix.MountPoint, "test.mkv")

	// Prime the derived mtime cache and confirm the starting value.
	if got := statMtimeUnix(t, virtualPath); got != orig.Unix() {
		t.Fatalf("initial mtime = %d, want %d", got, orig.Unix())
	}

	// Touch the dedup file to a new mtime.
	updated := time.Date(2022, 9, 9, 9, 9, 9, 0, time.UTC)
	if err := os.Chtimes(fix.DedupPath, updated, updated); err != nil {
		t.Fatalf("chtimes dedup update: %v", err)
	}

	// The watcher should refresh the derived mtime + invalidate the attr cache.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if statMtimeUnix(t, virtualPath) == updated.Unix() {
			return // success
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Errorf("virtual file mtime did not refresh to %d within timeout (got %d)",
		updated.Unix(), statMtimeUnix(t, virtualPath))
}

// TestFUSE_Reload_DirectoryMtimeOnAddRemove verifies POSIX directory-mtime
// semantics across a SIGHUP config reload: adding or removing a virtual file
// updates the mtime of the directory that directly contained it, and does NOT
// propagate to parent directories.
func TestFUSE_Reload_DirectoryMtimeOnAddRemove(t *testing.T) {
	skipIfFUSEUnavailable(t)
	binary := getWatchTestBinary(t)
	fix := createWatchFixture(t)

	// "test.mkv" at the root keeps startMount's readiness check working and
	// doubles as a control: the root's entries never change in this test.
	configPath := filepath.Join(fix.TmpDir, "dirs.mkvdup.yaml")
	writeVirtualFilesConfig(t, configPath, fix, "test.mkv", "Movies/Action/a.mkv")

	// Only SIGHUP should trigger reloads, so config watching is off.
	cmd := startMount(t, binary, fix.MountPoint, []string{configPath},
		"--no-source-watch", "--no-config-watch")

	rootDir := fix.MountPoint
	moviesDir := filepath.Join(fix.MountPoint, "Movies")
	actionDir := filepath.Join(moviesDir, "Action")

	if _, err := os.Stat(filepath.Join(actionDir, "a.mkv")); err != nil {
		t.Fatalf("expected Movies/Action/a.mkv to exist: %v", err)
	}

	rootBefore := statMtimeUnix(t, rootDir)
	moviesBefore := statMtimeUnix(t, moviesDir)
	actionBefore := statMtimeUnix(t, actionDir)

	// Timestamps have one-second granularity here, so ensure the reload lands
	// in a later second than the mount.
	time.Sleep(1100 * time.Millisecond)

	// --- Add a file inside Movies/Action ---
	writeVirtualFilesConfig(t, configPath, fix, "test.mkv", "Movies/Action/a.mkv", "Movies/Action/b.mkv")
	if err := cmd.Process.Signal(syscall.SIGHUP); err != nil {
		t.Fatalf("send SIGHUP: %v", err)
	}
	if !waitForPath(filepath.Join(actionDir, "b.mkv"), true, 5*time.Second) {
		t.Fatal("b.mkv did not appear after SIGHUP reload")
	}
	if !waitForMtimeAfter(t, actionDir, actionBefore, 5*time.Second) {
		t.Errorf("Movies/Action mtime did not advance after gaining a child (still %d)",
			statMtimeUnix(t, actionDir))
	}
	// No propagation to ancestors — neither gained or lost an entry of its own.
	if got := statMtimeUnix(t, moviesDir); got != moviesBefore {
		t.Errorf("Movies mtime = %d, want unchanged %d (no direct child added)", got, moviesBefore)
	}
	if got := statMtimeUnix(t, rootDir); got != rootBefore {
		t.Errorf("root mtime = %d, want unchanged %d (no direct child added)", got, rootBefore)
	}

	actionAfterAdd := statMtimeUnix(t, actionDir)
	time.Sleep(1100 * time.Millisecond)

	// --- Remove that file again ---
	writeVirtualFilesConfig(t, configPath, fix, "test.mkv", "Movies/Action/a.mkv")
	if err := cmd.Process.Signal(syscall.SIGHUP); err != nil {
		t.Fatalf("send SIGHUP: %v", err)
	}
	if !waitForPath(filepath.Join(actionDir, "b.mkv"), false, 5*time.Second) {
		t.Fatal("b.mkv did not disappear after SIGHUP reload")
	}
	if !waitForMtimeAfter(t, actionDir, actionAfterAdd, 5*time.Second) {
		t.Errorf("Movies/Action mtime did not advance after losing a child (still %d)",
			statMtimeUnix(t, actionDir))
	}
	if got := statMtimeUnix(t, moviesDir); got != moviesBefore {
		t.Errorf("Movies mtime = %d, want unchanged %d (no direct child removed)", got, moviesBefore)
	}
	if got := statMtimeUnix(t, rootDir); got != rootBefore {
		t.Errorf("root mtime = %d, want unchanged %d (no direct child removed)", got, rootBefore)
	}
}

// TestFUSE_Reload_UnchangedConfigKeepsDirectoryMtime verifies that a reload
// which changes nothing leaves directory mtimes alone — a directory's mtime
// tracks entry add/remove, not reload events.
func TestFUSE_Reload_UnchangedConfigKeepsDirectoryMtime(t *testing.T) {
	skipIfFUSEUnavailable(t)
	binary := getWatchTestBinary(t)
	fix := createWatchFixture(t)

	configPath := filepath.Join(fix.TmpDir, "dirs.mkvdup.yaml")
	writeVirtualFilesConfig(t, configPath, fix, "test.mkv", "Movies/Action/a.mkv")

	cmd := startMount(t, binary, fix.MountPoint, []string{configPath},
		"--no-source-watch", "--no-config-watch")

	actionDir := filepath.Join(fix.MountPoint, "Movies", "Action")
	actionBefore := statMtimeUnix(t, actionDir)
	rootBefore := statMtimeUnix(t, fix.MountPoint)

	time.Sleep(1100 * time.Millisecond)

	// Reload with an identical file set.
	if err := cmd.Process.Signal(syscall.SIGHUP); err != nil {
		t.Fatalf("send SIGHUP: %v", err)
	}
	time.Sleep(2 * time.Second) // let the reload complete

	if got := statMtimeUnix(t, actionDir); got != actionBefore {
		t.Errorf("Movies/Action mtime = %d, want unchanged %d (no-op reload)", got, actionBefore)
	}
	if got := statMtimeUnix(t, fix.MountPoint); got != rootBefore {
		t.Errorf("root mtime = %d, want unchanged %d (no-op reload)", got, rootBefore)
	}
}

// TestSourceWatch_DisableMode_TouchDoesNotDisable verifies that a timestamp-only
// change to a SOURCE file does not disable the virtual file, even in disable
// mode. Catching fsnotify.Chmod for dedup-mtime refresh must not route
// source-file Chmod events into the integrity (disable) path.
func TestSourceWatch_DisableMode_TouchDoesNotDisable(t *testing.T) {
	skipIfFUSEUnavailable(t)
	binary := getWatchTestBinary(t)
	fix := createWatchFixture(t)

	startMount(t, binary, fix.MountPoint, []string{fix.ConfigPath}, "--on-source-change", "disable")

	virtualPath := filepath.Join(fix.MountPoint, "test.mkv")

	// Sanity: readable to start.
	_ = readVirtualFile(t, virtualPath)

	// Touch the SOURCE file (mtime only, content unchanged).
	now := time.Now()
	if err := os.Chtimes(fix.SourcePath, now, now); err != nil {
		t.Fatalf("touch source: %v", err)
	}

	// Give the watcher time to (incorrectly) react.
	time.Sleep(1 * time.Second)

	// Must NOT be disabled — a timestamp-only source change is not corruption.
	if f, err := os.Open(virtualPath); err != nil {
		t.Errorf("virtual file was disabled after source touch: %v", err)
	} else {
		f.Close()
	}
}
