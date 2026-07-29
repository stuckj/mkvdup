package fuse

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func mode(v uint32) *uint32 { return &v }

// flush forces pending changes to disk.
//
// Writes are debounced, so a test that inspects the file (or reloads it through
// a second store) must force the write first. There is deliberately no
// synchronous mode to switch on: this is the same call the daemon makes at
// unmount, on SIGTERM and before a reload, so tests exercise the production
// write path rather than an alternative that only exists for them.
func flush(t *testing.T, s *PermissionStore) {
	t.Helper()
	if err := s.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
}

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
	flush(t, a)
	flush(t, b)

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
				return
			}
			if err := s.Flush(); err != nil {
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
	flush(t, s)

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
			_ = w.Flush()
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

// A failed write must not destroy what is already there, and must not leave a
// temp file behind. os.WriteFile truncates before writing, so a failure there
// could destroy good state; a temp+rename never touches the target until the
// rename, which either happens completely or not at all.
//
// The failure is forced through filesystem semantics rather than permissions:
// renaming a file over a non-empty directory fails for root too, whereas a
// read-only directory does not (CAP_DAC_OVERRIDE). This test therefore behaves
// identically as root and non-root, so it needs no build tag and never skips.
func TestWriteFileAtomic_FailureIsNonDestructive(t *testing.T) {
	dir := t.TempDir()

	// The target already exists, holding state a failed write must not damage.
	target := filepath.Join(dir, "permissions.yaml")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(target, "keep.txt")
	if err := os.WriteFile(keep, []byte("preexisting"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := writeFileAtomic(target, []byte("defaults:\n  file_mode: 0600\n"), 0644); err == nil {
		t.Fatal("expected the write to fail when the target cannot be replaced")
	}

	// Target untouched.
	if data, err := os.ReadFile(keep); err != nil || string(data) != "preexisting" {
		t.Errorf("existing state damaged by a failed write: data=%q err=%v", data, err)
	}

	// And no litter: the temp file must be cleaned up on the failure path.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("temp file left behind after a failed write: %s", e.Name())
		}
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
	flush(t, s)

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
