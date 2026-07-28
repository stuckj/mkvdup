package fuse

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func mode(v uint32) *uint32 { return &v }

// The original lost-update bug: two stores on one file, each with a long-lived
// in-memory copy loaded at startup. Before the locked read-modify-write, B's
// save wrote B's whole stale view and erased A's change.
func TestWithFileLock_ConcurrentStoresDoNotClobber(t *testing.T) {
	path := filepath.Join(t.TempDir(), "permissions.yaml")

	a := NewPermissionStore(path, DefaultPerms(), false)
	b := NewPermissionStore(path, DefaultPerms(), false)
	if err := a.Load(); err != nil {
		t.Fatal(err)
	}
	if err := b.Load(); err != nil {
		t.Fatal(err)
	}

	if err := a.SetFilePerms("A/one.mkv", nil, nil, mode(0640)); err != nil {
		t.Fatal(err)
	}
	// B never re-read; its write must still merge rather than replace.
	if err := b.SetFilePerms("B/two.mkv", nil, nil, mode(0600)); err != nil {
		t.Fatal(err)
	}

	fresh := NewPermissionStore(path, DefaultPerms(), false)
	if err := fresh.Load(); err != nil {
		t.Fatal(err)
	}
	if _, _, m := fresh.GetFilePerms("A/one.mkv"); m != 0640 {
		t.Errorf("A's override lost: mode = %#o, want 0640", m)
	}
	if _, _, m := fresh.GetFilePerms("B/two.mkv"); m != 0600 {
		t.Errorf("B's override lost: mode = %#o, want 0600", m)
	}
}

// Every mutation must survive concurrent writers.
func TestWithFileLock_ConcurrentMutationsAllLand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "permissions.yaml")

	const n = 50
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// Separate stores: each has its own stale in-memory view, which is
			// the situation multiple mount daemons are in.
			s := NewPermissionStore(path, DefaultPerms(), false)
			if err := s.SetFilePerms(fileKey(i), nil, nil, mode(0600)); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("SetFilePerms: %v", err)
	}

	fresh := NewPermissionStore(path, DefaultPerms(), false)
	if err := fresh.Load(); err != nil {
		t.Fatal(err)
	}
	missing := 0
	for i := range n {
		if _, _, m := fresh.GetFilePerms(fileKey(i)); m != 0600 {
			missing++
		}
	}
	if missing > 0 {
		t.Errorf("%d/%d mutations lost", missing, n)
	}
}

func fileKey(i int) string {
	return "f" + string(rune('a'+i/26)) + string(rune('a'+i%26)) + ".mkv"
}

// A reader must never observe a truncated or empty file. Before the atomic
// rename, Save() was O_TRUNC followed by a write, leaving exactly that window.
func TestWriteFileAtomic_ReaderNeverSeesPartialFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "permissions.yaml")
	s := NewPermissionStore(path, DefaultPerms(), false)
	if err := s.SetFilePerms("seed.mkv", nil, nil, mode(0644)); err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-done:
				return
			default:
			}
			w := NewPermissionStore(path, DefaultPerms(), false)
			_ = w.SetFilePerms(fileKey(i%50), nil, nil, mode(0600))
		}
	}()

	// Read concurrently; every read must parse and must never be empty.
	for range 300 {
		r := NewPermissionStore(path, DefaultPerms(), false)
		if err := r.Load(); err != nil {
			close(done)
			wg.Wait()
			t.Fatalf("reader saw an unparseable file: %v", err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if len(data) == 0 {
			close(done)
			wg.Wait()
			t.Fatal("reader saw a zero-length permissions file")
		}
		if !strings.Contains(string(data), "defaults:") {
			close(done)
			wg.Wait()
			t.Fatalf("reader saw a partial file:\n%s", data)
		}
	}
	close(done)
	wg.Wait()
}

// A failed write must leave the previous file intact. os.WriteFile truncates
// first, so a failure there could destroy good state; the temp+rename cannot.
func TestWriteFileAtomic_FailureLeavesPreviousFileIntact(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "permissions.yaml")

	good := []byte("defaults:\n  file_mode: 292\n")
	if err := writeFileAtomic(path, good, 0644); err != nil {
		t.Fatal(err)
	}

	// Make the directory read-only so creating the temp file fails.
	if err := os.Chmod(dir, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0700) })

	if err := writeFileAtomic(path, []byte("defaults:\n  file_mode: 384\n"), 0644); err == nil {
		t.Skip("write unexpectedly succeeded (running as root?)")
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("previous file is gone after a failed write: %v", err)
	}
	if string(after) != string(good) {
		t.Errorf("previous contents damaged by a failed write:\n%s", after)
	}
}

// The lock lives on a sidecar precisely because the rename swaps the inode.
func TestLockPath_IsSidecar(t *testing.T) {
	if got, want := lockPath("/var/lib/mkvdup/permissions.d/mnt-videos.yaml"),
		"/var/lib/mkvdup/permissions.d/mnt-videos.yaml.lock"; got != want {
		t.Errorf("lockPath = %q, want %q", got, want)
	}
}

// No temp files may survive a successful write.
func TestWriteFileAtomic_LeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "permissions.yaml")
	s := NewPermissionStore(path, DefaultPerms(), false)
	for i := range 5 {
		if err := s.SetFilePerms(fileKey(i), nil, nil, mode(0600)); err != nil {
			t.Fatal(err)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}
