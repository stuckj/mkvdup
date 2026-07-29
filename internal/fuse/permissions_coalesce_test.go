package fuse

import (
	"os"
	"path/filepath"
	"testing"
)

// A burst of changes must not produce a write per change. Rather than counting
// writes through a hook that only tests would use, this asserts the observable
// consequence: nothing has reached disk yet.
func TestCoalesce_BurstDefersTheWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "permissions.yaml")
	s := NewPermissionStore(path, DefaultPerms(), false)

	for i := range 100 {
		if err := s.SetFilePerms(fileKey(i), nil, nil, mode(0600)); err != nil {
			t.Fatal(err)
		}
	}

	// 100 mutations, no file: they were coalesced rather than written one by one.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected no write yet during a burst, but the file exists (err=%v)", err)
	}

	flush(t, s)

	fresh := NewPermissionStore(path, DefaultPerms(), false)
	if err := fresh.Load(); err != nil {
		t.Fatal(err)
	}
	for i := range 100 {
		if _, _, m := fresh.GetFilePerms(fileKey(i)); m != 0600 {
			t.Fatalf("%s missing after flush", fileKey(i))
		}
	}
}

// The sharpest edge in the design: Load replaces the in-memory maps, so a
// pending change must be written first or it disappears. Load does that itself,
// so no caller can forget.
func TestCoalesce_LoadFlushesPendingFirst(t *testing.T) {
	path := filepath.Join(t.TempDir(), "permissions.yaml")
	s := NewPermissionStore(path, DefaultPerms(), false)

	if err := s.SetFilePerms("one.mkv", nil, nil, mode(0640)); err != nil {
		t.Fatal(err)
	}
	// Simulates a SIGHUP reload arriving inside the debounce window.
	if err := s.Load(); err != nil {
		t.Fatal(err)
	}

	if _, _, m := s.GetFilePerms("one.mkv"); m != 0640 {
		t.Errorf("pending change lost across a reload: mode = %#o, want 0640", m)
	}
	// And it really is on disk, not just still in memory.
	fresh := NewPermissionStore(path, DefaultPerms(), false)
	if err := fresh.Load(); err != nil {
		t.Fatal(err)
	}
	if _, _, m := fresh.GetFilePerms("one.mkv"); m != 0640 {
		t.Errorf("pending change never reached disk: mode = %#o, want 0640", m)
	}
}

// Close is the unmount/shutdown path: it must stop the timer and drain.
func TestCoalesce_CloseFlushes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "permissions.yaml")
	s := NewPermissionStore(path, DefaultPerms(), false)

	if err := s.SetFilePerms("one.mkv", nil, nil, mode(0640)); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("second Close should be a no-op: %v", err)
	}

	fresh := NewPermissionStore(path, DefaultPerms(), false)
	if err := fresh.Load(); err != nil {
		t.Fatal(err)
	}
	if _, _, m := fresh.GetFilePerms("one.mkv"); m != 0640 {
		t.Errorf("Close did not persist the pending change: mode = %#o", m)
	}
}

// Deferring the write means the failing chmod itself returns success. The
// failure must still reach the caller, on the next operation.
func TestCoalesce_FailureIsLatchedAndRetried(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "state")
	if err := os.MkdirAll(sub, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(sub, "permissions.yaml")
	s := NewPermissionStore(path, DefaultPerms(), false)

	if err := s.SetFilePerms("one.mkv", nil, nil, mode(0640)); err != nil {
		t.Fatal(err)
	}

	// Make the write fail.
	if err := os.Chmod(sub, 0500); err != nil {
		t.Fatal(err)
	}
	if err := s.Flush(); err == nil {
		if os.Geteuid() == 0 {
			t.Skip("running as root: a read-only directory does not block writes")
		}
		t.Fatal("expected the flush to fail")
	}

	// The next mutation reports the latched error rather than swallowing it.
	if err := s.SetFilePerms("two.mkv", nil, nil, mode(0600)); err == nil {
		t.Error("a failed flush was not reported to the next caller")
	}

	// Nothing was dropped: once writable again, both changes land.
	if err := os.Chmod(sub, 0700); err != nil {
		t.Fatal(err)
	}
	flush(t, s)

	fresh := NewPermissionStore(path, DefaultPerms(), false)
	if err := fresh.Load(); err != nil {
		t.Fatal(err)
	}
	if _, _, m := fresh.GetFilePerms("one.mkv"); m != 0640 {
		t.Errorf("change from the failed flush was dropped: mode = %#o, want 0640", m)
	}
	if _, _, m := fresh.GetFilePerms("two.mkv"); m != 0600 {
		t.Errorf("later change lost: mode = %#o, want 0600", m)
	}
}

// EnsureWritable is what turns a misconfigured state directory into a loud
// failure at mount instead of a quiet one in the log much later.
func TestEnsureWritable(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "state")
	if err := os.MkdirAll(sub, 0700); err != nil {
		t.Fatal(err)
	}

	s := NewPermissionStore(filepath.Join(sub, "permissions.yaml"), DefaultPerms(), false)
	if err := s.EnsureWritable(); err != nil {
		t.Fatalf("writable directory reported as unwritable: %v", err)
	}

	if err := os.Chmod(sub, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(sub, 0700) })

	bad := NewPermissionStore(filepath.Join(sub, "nested", "permissions.yaml"), DefaultPerms(), false)
	if err := bad.EnsureWritable(); err == nil && os.Geteuid() != 0 {
		t.Error("unwritable state directory was not detected at mount time")
	}
}

// A store with no path keeps working entirely in memory.
func TestCoalesce_NoPathIsMemoryOnly(t *testing.T) {
	s := NewPermissionStore("", DefaultPerms(), false)
	if err := s.SetFilePerms("one.mkv", nil, nil, mode(0640)); err != nil {
		t.Fatal(err)
	}
	if _, _, m := s.GetFilePerms("one.mkv"); m != 0640 {
		t.Errorf("mode = %#o, want 0640", m)
	}
	if err := s.Flush(); err != nil {
		t.Errorf("Flush on an unpersisted store: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Errorf("Close on an unpersisted store: %v", err)
	}
}
