package fuse

import (
	"fmt"
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
		watcher:   watcher,
		reverse:   make(map[string][]*MKVFile),
		checksums: make(map[string]uint64),
		sizes:     make(map[string]int64),
		pollFiles: make(map[string]time.Time),
		action:    action,
		logFn:     logFn,
		stopCh:    make(chan struct{}),
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

	// Start poller if there are any poll files
	sw.mu.RLock()
	hasPollFiles := len(sw.pollFiles) > 0
	sw.mu.RUnlock()
	if hasPollFiles {
		sw.wg.Add(1)
		go sw.pollLoop()
	}
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
	sw.mu.RLock()
	defer sw.mu.RUnlock()
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
		// Run checksum verification in a goroutine to avoid blocking the watcher
		expectedChecksum := sw.checksums[absPath]
		expectedSize := sw.sizes[absPath]
		affectedCopy := make([]*MKVFile, len(affected))
		copy(affectedCopy, affected)
		go sw.verifyChecksum(absPath, expectedChecksum, expectedSize, affectedCopy)
	}
}

// verifyChecksum re-hashes a source file and disables affected virtual files
// if the checksum doesn't match.
func (sw *SourceWatcher) verifyChecksum(absPath string, expectedChecksum uint64, expectedSize int64, affected []*MKVFile) {
	names := make([]string, len(affected))
	for i, f := range affected {
		names[i] = f.Name
	}

	// Quick size check first
	info, err := os.Stat(absPath)
	if err != nil {
		sw.logFn("source-watch: checksum: cannot stat %s: %v — disabling %v", absPath, err, names)
		for _, f := range affected {
			f.Disable()
		}
		return
	}
	if info.Size() != expectedSize {
		sw.logFn("source-watch: checksum: size mismatch for %s (got %d, expected %d) — disabling %v",
			absPath, info.Size(), expectedSize, names)
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
		// Re-enable files that may have been disabled by a prior event
		// (e.g., rapid modify-then-restore where the first event disabled
		// due to size mismatch but the file is now back to its original state).
		for _, f := range affected {
			f.Enable()
		}
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
