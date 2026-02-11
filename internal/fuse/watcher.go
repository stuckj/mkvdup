package fuse

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/fsnotify/fsnotify"
	"golang.org/x/sys/unix"
)

// Filesystem type constants for network FS detection.
const (
	nfsSuperMagic   = 0x6969
	cifsMagicNum    = 0xFF534D42
	smb2MagicNum    = 0xFE534D42
	afsSuper        = 0x5346414F
	ncpfsSuperMagic = 0x564C
)

// Default poll interval for network filesystems where inotify doesn't work.
const defaultPollInterval = 60 * time.Second

// checksumRequest is a queued checksum verification job.
type checksumRequest struct {
	absPath          string
	expectedChecksum uint64
	expectedSize     int64
	affected         []*MKVFile
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
func (sw *SourceWatcher) Update(files map[string]*MKVFile, readerFactory ReaderFactory) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	// Remove existing watches
	for dir := range sw.watchedDirs() {
		sw.watcher.Remove(dir)
	}

	// Reset mappings
	sw.reverse = make(map[string][]*MKVFile)
	sw.checksums = make(map[string]uint64)
	sw.sizes = make(map[string]int64)
	sw.pollFiles = make(map[string]time.Time)

	// Build reverse mapping from source files to virtual files
	watchDirs := make(map[string]bool)
	for _, file := range files {
		reader, err := readerFactory.NewReaderLazy(file.DedupPath, file.SourceDir)
		if err != nil {
			sw.logFn("source-watch: warning: cannot read dedup header for %s: %v", file.Name, err)
			continue
		}
		sourceFiles := reader.SourceFileInfo()
		reader.Close()

		for _, sf := range sourceFiles {
			absPath := filepath.Join(file.SourceDir, sf.RelativePath)
			sw.reverse[absPath] = append(sw.reverse[absPath], file)
			sw.checksums[absPath] = sf.Checksum
			sw.sizes[absPath] = sf.Size
			watchDirs[filepath.Dir(absPath)] = true
		}
	}

	// Set up watches on source directories
	for dir := range watchDirs {
		if isNetworkFS(dir) {
			sw.logFn("source-watch: %s is on a network filesystem, using polling", dir)
			// Initialize poll state for files in this directory
			for absPath := range sw.reverse {
				if filepath.Dir(absPath) == dir {
					if info, err := os.Stat(absPath); err == nil {
						sw.pollFiles[absPath] = info.ModTime()
					}
				}
			}
		} else {
			if err := sw.watcher.Add(dir); err != nil {
				sw.logFn("source-watch: warning: cannot watch %s: %v", dir, err)
			}
		}
	}

	sw.logFn("source-watch: monitoring %d source files in %d directories (action=%s)",
		len(sw.reverse), len(watchDirs), sw.action)
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
			// Only react to writes, creates (overwrites), and renames
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
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
// any that have a different mtime than recorded.
func (sw *SourceWatcher) pollCheck() {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	for absPath, lastMtime := range sw.pollFiles {
		info, err := os.Stat(absPath)
		if err != nil {
			// File disappeared — treat as change
			sw.logFn("source-watch: poll: cannot stat %s: %v", absPath, err)
			sw.handleChangeLocked(absPath)
			continue
		}
		if !info.ModTime().Equal(lastMtime) {
			sw.pollFiles[absPath] = info.ModTime()
			sw.handleChangeLocked(absPath)
		}
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
			sw.mu.Unlock()

			sw.verifyChecksum(req.absPath, req.expectedChecksum, req.expectedSize, req.affected)
		case <-sw.stopCh:
			return
		}
	}
}

// verifyChecksum re-hashes a source file in the background. Files remain
// accessible during verification. If the checksum mismatches, affected
// virtual files are disabled (recoverable via SIGHUP reload). If it
// matches, no action is taken.
func (sw *SourceWatcher) verifyChecksum(absPath string, expectedChecksum uint64, expectedSize int64, affected []*MKVFile) {
	names := make([]string, len(affected))
	for i, f := range affected {
		names[i] = f.Name
	}

	// Re-check size — it may have changed since the event was queued
	info, err := os.Stat(absPath)
	if err != nil {
		sw.logFn("source-watch: checksum: cannot stat %s: %v — disabling %v", absPath, err, names)
		for _, f := range affected {
			f.Disable()
		}
		return
	}
	if info.Size() != expectedSize {
		sw.logFn("source-watch: checksum: size changed for %s (%d → %d) — disabling %v",
			absPath, expectedSize, info.Size(), names)
		for _, f := range affected {
			f.Disable()
		}
		return
	}

	// Full xxhash checksum
	f, err := os.Open(absPath)
	if err != nil {
		sw.logFn("source-watch: checksum: cannot open %s: %v — disabling %v", absPath, err, names)
		for _, f := range affected {
			f.Disable()
		}
		return
	}
	defer f.Close()

	h := xxhash.New()
	buf := make([]byte, 1<<20) // 1MB buffer
	for {
		n, readErr := f.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
		}
		if readErr != nil {
			if readErr != io.EOF {
				sw.logFn("source-watch: checksum: read error for %s: %v — disabling %v", absPath, readErr, names)
				for _, af := range affected {
					af.Disable()
				}
				return
			}
			break
		}
	}
	actualChecksum := h.Sum64()

	if actualChecksum != expectedChecksum {
		sw.logFn("source-watch: checksum mismatch for %s (got %016x, expected %016x) — disabling %v",
			absPath, actualChecksum, expectedChecksum, names)
		for _, f := range affected {
			f.Disable()
		}
	} else {
		sw.logFn("source-watch: checksum verified OK for %s", absPath)
	}
}

// isNetworkFS checks if the given path is on a network filesystem.
func isNetworkFS(path string) bool {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		// Can't determine — assume local
		return false
	}

	switch stat.Type {
	case nfsSuperMagic, cifsMagicNum, smb2MagicNum, afsSuper, ncpfsSuperMagic:
		return true
	}
	return false
}
