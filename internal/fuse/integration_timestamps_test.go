//go:build integration

package fuse_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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
