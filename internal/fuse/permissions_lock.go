package fuse

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
	"gopkg.in/yaml.v3"
)

const (
	// lockTimeout bounds how long a mutation waits for the file lock. A FUSE
	// request is a kernel request: blocking one indefinitely because another
	// process wedged holding the lock is worse than failing it, so the wait is
	// bounded and the caller gets an error.
	lockTimeout = 2 * time.Second

	// lockRetryInterval paces the non-blocking retry loop. Contention is rare
	// (per-mount files are unshared) and each hold is short, so polling costs
	// nothing in the common case.
	lockRetryInterval = 10 * time.Millisecond
)

// lockPath returns the sidecar lock file for a permissions file.
//
// The lock deliberately does *not* live on the permissions file itself:
// writeFileAtomic replaces that file by rename, which swaps the inode, and
// flock is per-inode. A lock held on the data file would be silently dropped by
// the very write it is meant to guard.
func lockPath(path string) string { return path + ".lock" }

// acquireFileLock takes an exclusive advisory lock on a sidecar file, waiting
// up to lockTimeout. The returned file must be closed to release the lock.
func acquireFileLock(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("create permissions directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("open lock file %s: %w", path, err)
	}

	deadline := time.Now().Add(lockTimeout)
	for {
		err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			return f, nil
		}
		if !errors.Is(err, unix.EWOULDBLOCK) {
			f.Close()
			return nil, fmt.Errorf("lock %s: %w", path, err)
		}
		if time.Now().After(deadline) {
			f.Close()
			return nil, fmt.Errorf("timed out after %s waiting for lock %s", lockTimeout, path)
		}
		time.Sleep(lockRetryInterval)
	}
}

// writeFileAtomic writes data to path via a temporary file and a rename, so a
// concurrent reader sees either the previous complete file or the new one and
// never a partial write.
//
// This also makes a failed write non-destructive: the previous file is left
// intact rather than truncated, which a plain os.WriteFile cannot promise.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create permissions directory: %w", err)
	}

	// Same directory as the target: rename is only atomic within a filesystem.
	tmp, err := os.CreateTemp(dir, ".permissions-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			tmp.Close()
			os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	// Flush before the rename, or a crash could leave the new name pointing at
	// an empty file -- exactly the failure this function exists to prevent.
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}
	committed = true

	// Persist the directory entry too. Best-effort: the rename has already
	// succeeded and the data is durable, so a failure here costs ordering
	// guarantees on a crash, not correctness now.
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		d.Close()
	}
	return nil
}

// readPermissionsFile parses the permissions file at path. A missing file is
// not an error: it yields an empty structure, which is what a first run sees.
func readPermissionsFile(path string) (permissionsFile, error) {
	var pf permissionsFile
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return pf, nil
		}
		return pf, fmt.Errorf("read permissions file: %w", err)
	}
	if err := yaml.Unmarshal(data, &pf); err != nil {
		return pf, fmt.Errorf("parse permissions file: %w", err)
	}
	return pf, nil
}

// withFileLock performs a read-modify-write of the permissions file under an
// exclusive lock, then refreshes the in-memory cache from the result.
//
// Reading inside the lock is the point. The previous implementation wrote its
// whole in-memory snapshot, which was loaded at startup and never refreshed --
// so any other writer's changes were silently erased. Applying a delta to the
// state actually on disk is what makes concurrent writers safe.
//
// Read paths (GetFilePerms and friends) deliberately do not take this lock:
// they serve from the in-memory cache, so FUSE getattr stays off the disk.
func (s *PermissionStore) withFileLock(apply func(pf *permissionsFile)) error {
	if s.path == "" {
		// Not persisted: the in-memory maps are the entire store.
		s.mu.Lock()
		defer s.mu.Unlock()
		pf := s.snapshotLocked()
		apply(&pf)
		s.adoptLocked(pf)
		return nil
	}

	lock, err := acquireFileLock(lockPath(s.path))
	if err != nil {
		return err
	}
	defer lock.Close()

	pf, err := readPermissionsFile(s.path)
	if err != nil {
		return err
	}

	s.mu.RLock()
	shared, mount := s.shared, s.mount
	if !shared {
		// Defaults come from this mount's --default-* flags, which the file does
		// not own. In shared mode another mount's defaults are in there, so they
		// are left exactly as read.
		pf.Defaults = toFileDefaults(s.defaults)
	}
	s.mu.RUnlock()

	if mount != "" && !shared {
		pf.Mount = mount
	}

	apply(&pf)

	data, err := yaml.Marshal(&pf)
	if err != nil {
		return fmt.Errorf("marshal permissions: %w", err)
	}

	// Skip a no-op write. chmod -R re-applying an identical mode is common, and
	// every skipped write is a lock hold and two fsyncs avoided.
	if existing, err := os.ReadFile(s.path); err == nil && string(existing) == string(data) {
		s.mu.Lock()
		s.adoptLocked(pf)
		s.mu.Unlock()
		return nil
	}

	if err := writeFileAtomic(s.path, data, 0644); err != nil {
		return err
	}

	s.mu.Lock()
	s.adoptLocked(pf)
	s.mu.Unlock()
	return nil
}

// snapshotLocked builds a deep copy of the in-memory state. s.mu must be held.
func (s *PermissionStore) snapshotLocked() permissionsFile {
	pf := permissionsFile{Defaults: toFileDefaults(s.defaults)}
	if s.mount != "" && !s.shared {
		pf.Mount = s.mount
	}
	if len(s.files) > 0 {
		pf.Files = make(map[string]*Perms, len(s.files))
		for k, v := range s.files {
			if v != nil {
				entry := *v
				pf.Files[k] = &entry
			}
		}
	}
	if len(s.dirs) > 0 {
		pf.Directories = make(map[string]*Perms, len(s.dirs))
		for k, v := range s.dirs {
			if v != nil {
				entry := *v
				pf.Directories[k] = &entry
			}
		}
	}
	return pf
}

// adoptLocked replaces the in-memory cache with the just-persisted state, so
// the cache cannot drift from the file. s.mu must be held.
func (s *PermissionStore) adoptLocked(pf permissionsFile) {
	s.files = make(map[string]*Perms, len(pf.Files))
	for k, v := range pf.Files {
		if v != nil {
			entry := *v
			s.files[k] = &entry
		}
	}
	s.dirs = make(map[string]*Perms, len(pf.Directories))
	for k, v := range pf.Directories {
		if v != nil {
			entry := *v
			s.dirs[k] = &entry
		}
	}
}

// entryFor returns the entry for path in m, creating it if absent.
func entryFor(m *map[string]*Perms, path string) *Perms {
	if *m == nil {
		*m = make(map[string]*Perms)
	}
	if p, ok := (*m)[path]; ok && p != nil {
		return p
	}
	p := &Perms{}
	(*m)[path] = p
	return p
}
