package fuse

import (
	"fmt"
	"log"
	"os"
	"time"
)

const (
	// flushDebounce is how long an idle period must be before pending changes
	// are written. A chmod -R arrives as thousands of separate syscalls; without
	// coalescing each one is a lock, a read, a marshal and two fsyncs, which
	// makes a recursive chmod O(n^2) in bytes and 2n in fsyncs.
	flushDebounce = 250 * time.Millisecond

	// flushMaxDelay bounds how long a *continuous* stream of changes can defer
	// the write. Without it, a long chmod -R would never hit an idle window and
	// nothing would reach disk until it finished.
	flushMaxDelay = 2 * time.Second
)

// permKey identifies one entry in the permissions file. Directories and files
// occupy separate namespaces, so the flag is part of the identity.
type permKey struct {
	dir  bool
	path string
}

// markDirtyLocked records that path's in-memory entry differs from the file and
// schedules a write. s.mu must be held.
func (s *PermissionStore) markDirtyLocked(key permKey) {
	if s.path == "" {
		return // not persisted; the in-memory maps are the whole store
	}
	if s.pending == nil {
		s.pending = make(map[permKey]struct{})
	}
	s.pending[key] = struct{}{}
	s.scheduleFlushLocked()
}

// scheduleFlushLocked arms (or re-arms) the debounce timer. s.mu must be held.
func (s *PermissionStore) scheduleFlushLocked() {
	if s.closed {
		return
	}
	now := time.Now()
	if s.firstDirty.IsZero() {
		s.firstDirty = now
	}

	delay := flushDebounce
	// Never push the write past flushMaxDelay from the first pending change.
	if cap := s.firstDirty.Add(flushMaxDelay); now.Add(delay).After(cap) {
		delay = max(time.Until(cap), 0)
	}

	if s.flushTimer == nil {
		s.flushTimer = time.AfterFunc(delay, s.flushFromTimer)
		return
	}
	s.flushTimer.Reset(delay)
}

// flushFromTimer is the debounce timer's callback. Errors cannot be returned to
// anyone here, so they are logged and latched for the next mutation to report.
func (s *PermissionStore) flushFromTimer() {
	if err := s.Flush(); err != nil {
		log.Printf("Warning: failed to save permissions to %s: %v", s.path, err)
	}
}

// Flush writes any pending changes immediately, through the same path the
// debounce timer uses.
//
// This is deliberately the *only* way to force a write: there is no synchronous
// mode. A test that used one would be exercising a code path production never
// takes, which is worse than no test at all. Tests call this; so do the forced
// flush points -- unmount, shutdown, reload and stale-entry cleanup.
func (s *PermissionStore) Flush() error {
	s.mu.Lock()
	if len(s.pending) == 0 {
		err := s.flushErr
		s.flushErr = nil
		s.mu.Unlock()
		return err
	}

	// Take the pending set out wholesale. Mutations arriving during the write go
	// into the fresh map and are picked up by the next flush, so nothing is lost
	// to the gap between snapshotting and writing.
	flushing := s.pending
	s.pending = make(map[permKey]struct{})
	s.firstDirty = time.Time{}

	// Snapshot the values now, so the write does not need s.mu and readers are
	// never blocked on disk I/O.
	type snapshot struct {
		key   permKey
		perms *Perms // nil means the entry was removed
	}
	snaps := make([]snapshot, 0, len(flushing))
	for key := range flushing {
		src := s.files
		if key.dir {
			src = s.dirs
		}
		var entry *Perms
		if p, ok := src[key.path]; ok && p != nil {
			cp := *p
			entry = &cp
		}
		snaps = append(snaps, snapshot{key: key, perms: entry})
	}
	s.mu.Unlock()

	err := s.writeMerged(func(pf *permissionsFile) {
		for _, sn := range snaps {
			target := &pf.Files
			if sn.key.dir {
				target = &pf.Directories
			}
			if sn.perms == nil {
				delete(*target, sn.key.path)
				continue
			}
			if *target == nil {
				*target = make(map[string]*Perms)
			}
			cp := *sn.perms
			(*target)[sn.key.path] = &cp
		}
	})

	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		// Never drop a user's change because a write failed. Put the keys back
		// (union with anything newly dirtied) and latch the error so the next
		// mutation reports it.
		for key := range flushing {
			s.pending[key] = struct{}{}
		}
		s.flushErr = err
		return err
	}

	reported := s.flushErr
	s.flushErr = nil
	return reported
}

// latchedError returns and clears any error from a previous background flush,
// so a persistent failure still reaches the caller through a syscall rather
// than living only in the log.
//
// The cost of deferring the write is that the *first* failing chmod returns
// success; the next one reports it. For chmod -R that means the user sees EIO
// on the second file, which in practice is as good as the first.
func (s *PermissionStore) latchedError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.flushErr
	s.flushErr = nil
	return err
}

// Close flushes any pending changes and stops the timer. Safe to call twice.
func (s *PermissionStore) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	if s.flushTimer != nil {
		s.flushTimer.Stop()
		s.flushTimer = nil
	}
	s.mu.Unlock()

	return s.Flush()
}

// EnsureWritable checks at mount time that permission changes will be able to
// persist, by creating the state directory and the lock file.
//
// This exists because the debounced write reports failures late: without it, an
// unwritable state directory would let every chmod return success and surface
// only in the log. Failing loudly at mount is much easier to diagnose.
func (s *PermissionStore) EnsureWritable() error {
	if s.path == "" {
		return nil
	}
	lock, err := acquireFileLock(lockPath(s.path))
	if err != nil {
		return fmt.Errorf("permissions file %s is not writable: %w", s.path, err)
	}
	lock.Close()

	// The lock file proves the directory is writable; also confirm an existing
	// permissions file is not read-only.
	if f, err := os.OpenFile(s.path, os.O_WRONLY|os.O_APPEND, 0644); err == nil {
		f.Close()
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("permissions file %s is not writable: %w", s.path, err)
	}
	return nil
}
