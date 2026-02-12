package fuse

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/fsnotify/fsnotify"
)

// Default poll interval for network filesystems where inotify doesn't work.
const defaultPollInterval = 60 * time.Second

// checksumRequest is a queued checksum verification job.
type checksumRequest struct {
	absPath          string
	expectedChecksum uint64
	expectedSize     int64
	affected         []*MKVFile
	gen              uint64 // generation stamp; stale requests are skipped
}

// SourceWatcher monitors source files for changes and takes action when
// modifications are detected. It uses inotify for local filesystems and
// falls back to polling for network filesystems (NFS, CIFS/SMB).
type SourceWatcher struct {
	watcher *fsnotify.Watcher

	// reverse maps absolute source file paths to the virtual files that use them.
	reverse map[string][]*MKVFile

	// checksums maps absolute source file paths to expected xxhash values.
	checksums map[string]uint64

	// sizes maps absolute source file paths to expected file sizes.
	sizes map[string]int64

	// pollFiles maps absolute source file paths to their last known mtime
	// for directories that use polling instead of inotify.
	pollFiles map[string]time.Time

	action string // "warn", "disable", "checksum"
	logFn  func(string, ...interface{})
	mu     sync.RWMutex

	// checksumCh queues checksum verification requests so they run
	// sequentially in a single worker goroutine, avoiding I/O storms
	// when many source files change at once.
	checksumCh chan checksumRequest

	// checksumPending tracks source paths with a queued checksum request,
	// preventing duplicate queue entries for the same file. The worker
	// clears the flag when it starts processing, so new events that arrive
	// during verification are still queued.
	checksumPending map[string]bool

	// updateGen is incremented on each Update() call. Checksum requests
	// carry the generation they were created in; the worker skips requests
	// whose generation doesn't match, preventing stale verifications from
	// a previous config from disabling files after a reload.
	updateGen uint64

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewSourceWatcher creates a new source file watcher with the given action.
// The watcher is not started until Start() is called.
func NewSourceWatcher(action string, logFn func(string, ...interface{})) (*SourceWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create fsnotify watcher: %w", err)
	}

	if logFn == nil {
		logFn = func(string, ...interface{}) {}
	}

	return &SourceWatcher{
		watcher:         watcher,
		reverse:         make(map[string][]*MKVFile),
		checksums:       make(map[string]uint64),
		sizes:           make(map[string]int64),
		pollFiles:       make(map[string]time.Time),
		action:          action,
		logFn:           logFn,
		checksumCh:      make(chan checksumRequest, 256),
		checksumPending: make(map[string]bool),
		stopCh:          make(chan struct{}),
	}, nil
}

// Update rebuilds the watcher's source file mappings from the current file set.
// It removes old watches and sets up new ones. Called on mount and after reload.
//
// For each MKVFile, the readerFactory is used to read the dedup file header
// (lazy read, no full initialization) to get the source file list.
//
// The method minimizes lock hold time: maps are built without the lock,
// swapped in briefly under the lock, and then inotify watches and os.Stat
// calls happen without the lock.
func (sw *SourceWatcher) Update(files map[string]*MKVFile, readerFactory ReaderFactory) {
	// Phase 1: Build new maps without holding the lock. This involves
	// I/O (reading dedup headers) that should not block event handling.
	newReverse := make(map[string][]*MKVFile)
	newChecksums := make(map[string]uint64)
	newSizes := make(map[string]int64)
	watchDirs := make(map[string]bool)

	for _, file := range files {
		reader, err := readerFactory.NewReaderLazy(file.DedupPath, file.SourceDir)
		if err != nil {
			sw.logFn("source-watch: warning: cannot read dedup header for %s: %v", file.Name, err)
			continue
		}
		sourceFiles := reader.SourceFileInfo()
		reader.Close()

		cleanSourceDir := filepath.Clean(file.SourceDir)
		if cleanSourceDir[len(cleanSourceDir)-1] != filepath.Separator {
			cleanSourceDir += string(filepath.Separator)
		}
		for _, sf := range sourceFiles {
			absPath := filepath.Clean(filepath.Join(file.SourceDir, sf.RelativePath))
			if !strings.HasPrefix(absPath, cleanSourceDir) {
				sw.logFn("source-watch: warning: skipping source file with path traversal: %s", sf.RelativePath)
				continue
			}
			newReverse[absPath] = append(newReverse[absPath], file)
			newChecksums[absPath] = sf.Checksum
			newSizes[absPath] = sf.Size
			watchDirs[filepath.Dir(absPath)] = true
		}
	}

	// Phase 2: Swap maps and drain stale checksum queue under the lock.
	sw.mu.Lock()
	oldDirs := sw.watchedDirs()

	// Drain any stale checksum requests from a previous configuration.
drain:
	for {
		select {
		case <-sw.checksumCh:
		default:
			break drain
		}
	}
	sw.checksumPending = make(map[string]bool)
	sw.updateGen++

	sw.reverse = newReverse
	sw.checksums = newChecksums
	sw.sizes = newSizes
	sw.pollFiles = make(map[string]time.Time)
	sw.mu.Unlock()

	// Phase 3: Update inotify watches without the lock.
	// fsnotify.Watcher methods are thread-safe.
	for dir := range oldDirs {
		sw.watcher.Remove(dir)
	}

	// Precompute files per directory so polling setup is O(files), not O(dirs×files).
	pathsByDir := make(map[string][]string)
	for absPath := range newReverse {
		dir := filepath.Dir(absPath)
		pathsByDir[dir] = append(pathsByDir[dir], absPath)
	}

	newPollFiles := make(map[string]time.Time)
	for dir := range watchDirs {
		if isNetworkFS(dir) {
			sw.logFn("source-watch: %s is on a network filesystem, using polling", dir)
			for _, absPath := range pathsByDir[dir] {
				if info, err := os.Stat(absPath); err == nil {
					newPollFiles[absPath] = info.ModTime()
				} else {
					// File currently missing/unavailable — use zero mtime so
					// pollCheck detects it appearing (or triggers handleChange
					// via its stat-error path).
					newPollFiles[absPath] = time.Time{}
				}
			}
		} else {
			if err := sw.watcher.Add(dir); err != nil {
				sw.logFn("source-watch: warning: cannot watch %s: %v", dir, err)
			}
		}
	}

	// Phase 4: Set poll files under the lock.
	if len(newPollFiles) > 0 {
		sw.mu.Lock()
		sw.pollFiles = newPollFiles
		sw.mu.Unlock()
	}

	sw.logFn("source-watch: monitoring %d source files in %d directories (action=%s)",
		len(newReverse), len(watchDirs), sw.action)
}

// watchedDirs returns the set of currently watched directories.
func (sw *SourceWatcher) watchedDirs() map[string]bool {
	dirs := make(map[string]bool)
	for path := range sw.reverse {
		dirs[filepath.Dir(path)] = true
	}
	return dirs
}

// Start begins the event processing loop. Must be called after Update().
func (sw *SourceWatcher) Start() {
	sw.wg.Add(1)
	go sw.eventLoop()

	// Start checksum worker (single goroutine processes queue sequentially)
	if sw.action == "checksum" {
		sw.wg.Add(1)
		go sw.checksumWorker()
	}

	// Always start poller — it no-ops when pollFiles is empty, but must
	// be running so that network FS dirs added via Update() after reload
	// are polled without requiring a restart.
	sw.wg.Add(1)
	go sw.pollLoop()
}

// Stop stops the watcher and waits for goroutines to exit.
func (sw *SourceWatcher) Stop() {
	close(sw.stopCh)
	sw.watcher.Close()
	sw.wg.Wait()
}

// eventLoop processes fsnotify events.
func (sw *SourceWatcher) eventLoop() {
	defer sw.wg.Done()

	for {
		select {
		case event, ok := <-sw.watcher.Events:
			if !ok {
				return
			}
			// React to writes, creates (overwrites), renames, and removals
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Remove) == 0 {
				continue
			}
			sw.handleChange(event.Name)

		case err, ok := <-sw.watcher.Errors:
			if !ok {
				return
			}
			sw.logFn("source-watch: watcher error: %v", err)

		case <-sw.stopCh:
			return
		}
	}
}

// pollLoop periodically checks files on network filesystems for changes.
func (sw *SourceWatcher) pollLoop() {
	defer sw.wg.Done()

	ticker := time.NewTicker(defaultPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sw.pollCheck()
		case <-sw.stopCh:
			return
		}
	}
}

// pollCheck stats all poll-monitored files and triggers handleChange for
// any that have a different mtime than recorded. It snapshots the poll set
// under a read lock, performs os.Stat calls without the lock (network FS
// stats can block), then updates mtimes and processes changes.
func (sw *SourceWatcher) pollCheck() {
	// Snapshot under read lock so os.Stat doesn't block event handling.
	type polledFile struct {
		path      string
		lastMtime time.Time
	}
	sw.mu.RLock()
	snapshot := make([]polledFile, 0, len(sw.pollFiles))
	for absPath, lastMtime := range sw.pollFiles {
		snapshot = append(snapshot, polledFile{path: absPath, lastMtime: lastMtime})
	}
	sw.mu.RUnlock()

	// Stat without holding the lock.
	type mtimeUpdate struct {
		path     string
		newMtime time.Time
	}
	var (
		updates      []mtimeUpdate
		changedPaths []string
	)
	for _, pf := range snapshot {
		info, err := os.Stat(pf.path)
		if err != nil {
			sw.logFn("source-watch: poll: cannot stat %s: %v", pf.path, err)
			changedPaths = append(changedPaths, pf.path)
			continue
		}
		if !info.ModTime().Equal(pf.lastMtime) {
			updates = append(updates, mtimeUpdate{path: pf.path, newMtime: info.ModTime()})
			changedPaths = append(changedPaths, pf.path)
		}
	}

	// Update stored mtimes under the lock.
	if len(updates) > 0 {
		sw.mu.Lock()
		for _, u := range updates {
			if _, ok := sw.pollFiles[u.path]; ok {
				sw.pollFiles[u.path] = u.newMtime
			}
		}
		sw.mu.Unlock()
	}

	// Process changes — handleChange acquires the lock per-path.
	for _, absPath := range changedPaths {
		sw.handleChange(absPath)
	}
}

// handleChange processes a source file change event.
func (sw *SourceWatcher) handleChange(absPath string) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	sw.handleChangeLocked(absPath)
}

// handleChangeLocked processes a source file change. Caller must hold sw.mu.
func (sw *SourceWatcher) handleChangeLocked(absPath string) {
	affected, ok := sw.reverse[absPath]
	if !ok {
		return // Not a tracked source file
	}

	names := make([]string, len(affected))
	for i, f := range affected {
		names[i] = f.Name
	}

	switch sw.action {
	case "warn":
		sw.logFn("source-watch: WARNING: source file changed: %s (affects: %v)", absPath, names)

	case "disable":
		sw.logFn("source-watch: source file changed, disabling: %s (affects: %v)", absPath, names)
		for _, f := range affected {
			f.Disable()
		}

	case "checksum":
		// Stat the source file to distinguish size changes from
		// timestamp-only changes (e.g., touch).
		info, err := os.Stat(absPath)
		if err != nil {
			// File disappeared — disable immediately
			sw.logFn("source-watch: source file missing, disabling: %s (affects: %v)", absPath, names)
			for _, f := range affected {
				f.Disable()
			}
			return
		}

		expectedSize := sw.sizes[absPath]
		if info.Size() != expectedSize {
			// Size changed — definitely corrupted, disable immediately
			sw.logFn("source-watch: source file size changed (%d → %d), disabling: %s (affects: %v)",
				expectedSize, info.Size(), absPath, names)
			for _, f := range affected {
				f.Disable()
			}
			return
		}

		// Size matches — verify checksum in background. File remains
		// accessible during verification; only disabled on mismatch.
		if sw.checksumPending[absPath] {
			return // Already queued
		}
		sw.logFn("source-watch: source file modified, verifying checksum: %s (affects: %v)", absPath, names)
		affectedCopy := make([]*MKVFile, len(affected))
		copy(affectedCopy, affected)
		select {
		case sw.checksumCh <- checksumRequest{
			absPath:          absPath,
			expectedChecksum: sw.checksums[absPath],
			expectedSize:     expectedSize,
			affected:         affectedCopy,
			gen:              sw.updateGen,
		}:
			sw.checksumPending[absPath] = true
		default:
			// Queue full — disable as a safety measure
			sw.logFn("source-watch: checksum queue full, disabling: %s (affects: %v)", absPath, names)
			for _, f := range affected {
				f.Disable()
			}
		}
	}
}

// checksumWorker processes checksum verification requests sequentially.
// Only one goroutine runs this, ensuring that bulk source changes don't
// spawn hundreds of parallel I/O-heavy hash operations.
func (sw *SourceWatcher) checksumWorker() {
	defer sw.wg.Done()

	for {
		select {
		case req := <-sw.checksumCh:
			// Clear pending flag so new events for this path get queued.
			// This must happen before verification so that changes during
			// hashing trigger a fresh verification.
			sw.mu.Lock()
			delete(sw.checksumPending, req.absPath)
			stale := req.gen != sw.updateGen
			sw.mu.Unlock()

			if stale {
				continue // Config was reloaded; skip stale request
			}
			sw.verifyChecksum(req.absPath, req.expectedChecksum, req.expectedSize, req.affected, req.gen)
		case <-sw.stopCh:
			return
		}
	}
}

// verifyChecksum re-hashes a source file in the background. Files remain
// accessible during verification. If the checksum mismatches, affected
// virtual files are disabled (recoverable via SIGHUP reload or a
// subsequent successful checksum). The gen parameter is checked before
// disabling or enabling so that a reload during verification prevents
// stale results from affecting files in the new configuration.
func (sw *SourceWatcher) verifyChecksum(absPath string, expectedChecksum uint64, expectedSize int64, affected []*MKVFile, gen uint64) {
	names := make([]string, len(affected))
	for i, f := range affected {
		names[i] = f.Name
	}

	// disableIfCurrent disables affected files only if the watcher
	// generation hasn't changed (i.e., no reload occurred during verification).
	disableIfCurrent := func() {
		sw.mu.RLock()
		stale := gen != sw.updateGen
		sw.mu.RUnlock()
		if stale {
			sw.logFn("source-watch: checksum: skipping disable for %s (config reloaded during verification)", absPath)
			return
		}
		for _, f := range affected {
			f.Disable()
		}
	}

	// Re-check size — it may have changed since the event was queued
	info, err := os.Stat(absPath)
	if err != nil {
		sw.logFn("source-watch: checksum: cannot stat %s: %v — disabling %v", absPath, err, names)
		disableIfCurrent()
		return
	}
	if info.Size() != expectedSize {
		sw.logFn("source-watch: checksum: size changed for %s (%d → %d) — disabling %v",
			absPath, expectedSize, info.Size(), names)
		disableIfCurrent()
		return
	}

	// Full xxhash checksum
	f, err := os.Open(absPath)
	if err != nil {
		sw.logFn("source-watch: checksum: cannot open %s: %v — disabling %v", absPath, err, names)
		disableIfCurrent()
		return
	}
	defer f.Close()

	h := xxhash.New()
	buf := make([]byte, 1<<20) // 1MB buffer
	for {
		// Check for shutdown between reads so large-file hashing
		// doesn't block Stop() indefinitely.
		select {
		case <-sw.stopCh:
			return
		default:
		}

		n, readErr := f.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
		}
		if readErr != nil {
			if readErr != io.EOF {
				sw.logFn("source-watch: checksum: read error for %s: %v — disabling %v", absPath, readErr, names)
				disableIfCurrent()
				return
			}
			break
		}
	}
	actualChecksum := h.Sum64()

	if actualChecksum != expectedChecksum {
		sw.logFn("source-watch: checksum mismatch for %s (got %016x, expected %016x) — disabling %v",
			absPath, actualChecksum, expectedChecksum, names)
		disableIfCurrent()
	} else {
		// Re-enable affected files so transient issues (e.g., network
		// glitches) auto-recover without requiring admin SIGHUP.
		//
		// NOTE: a virtual file can depend on multiple source files. A
		// passing checksum for one source could re-enable a file whose
		// other source is still bad. This is a known limitation; the
		// common case (single source per MKV) is handled correctly, and
		// SIGHUP is available as a fallback for multi-source edge cases.
		sw.mu.RLock()
		stale := gen != sw.updateGen
		sw.mu.RUnlock()
		if stale {
			sw.logFn("source-watch: checksum: skipping re-enable for %s (config reloaded during verification)", absPath)
			return
		}
		sw.logFn("source-watch: checksum verified OK for %s — re-enabling %v", absPath, names)
		for _, f := range affected {
			f.Enable()
		}
	}
}
