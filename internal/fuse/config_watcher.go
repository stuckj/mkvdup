package fuse

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// configDebounceDelay is the time to wait after the last config file event
// before triggering the action. This coalesces rapid changes from editors
// that write to a temp file and then rename (atomic save).
const configDebounceDelay = 500 * time.Millisecond

// ConfigWatcher monitors config files for changes and either logs a warning
// or triggers a reload callback. It uses inotify for local filesystems and
// falls back to polling for network filesystems (NFS, CIFS/SMB).
type ConfigWatcher struct {
	watcher *fsnotify.Watcher

	// configFiles is the set of absolute config file paths being watched.
	configFiles map[string]bool

	// pollFiles maps absolute config file paths to their last known mtime
	// for directories that use polling instead of inotify.
	pollFiles map[string]time.Time

	action       string // "reload" or "warn"
	reloadFn     func()
	logFn        func(string, ...interface{})
	pollInterval time.Duration

	mu     sync.Mutex
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewConfigWatcher creates a new config file watcher with the given action.
// action must be "reload" or "warn".
// If pollInterval <= 0, defaultPollInterval is used.
// The watcher is not started until Start() is called.
func NewConfigWatcher(action string, pollInterval time.Duration, reloadFn func(), logFn func(string, ...interface{})) (*ConfigWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	if logFn == nil {
		logFn = func(string, ...interface{}) {}
	}

	if pollInterval <= 0 {
		pollInterval = defaultPollInterval
	}

	return &ConfigWatcher{
		watcher:      watcher,
		configFiles:  make(map[string]bool),
		pollFiles:    make(map[string]time.Time),
		action:       action,
		reloadFn:     reloadFn,
		logFn:        logFn,
		pollInterval: pollInterval,
		stopCh:       make(chan struct{}),
	}, nil
}

// Update replaces the set of watched config files. It removes old watches
// and sets up new ones. Called on mount and after reload.
func (cw *ConfigWatcher) Update(configPaths []string) {
	// Build new file set and directory set.
	newFiles := make(map[string]bool, len(configPaths))
	watchDirs := make(map[string]bool)
	for _, p := range configPaths {
		abs, err := filepath.Abs(p)
		if err != nil {
			cw.logFn("config-watch: warning: cannot resolve %s: %v", p, err)
			continue
		}
		newFiles[abs] = true
		watchDirs[filepath.Dir(abs)] = true
	}

	cw.mu.Lock()
	// Remove old inotify watches.
	oldDirs := make(map[string]bool)
	for f := range cw.configFiles {
		oldDirs[filepath.Dir(f)] = true
	}
	cw.configFiles = newFiles
	cw.pollFiles = make(map[string]time.Time)
	cw.mu.Unlock()

	// Remove old watches (fsnotify methods are thread-safe).
	for dir := range oldDirs {
		cw.watcher.Remove(dir)
	}

	// Precompute files per directory for efficient poll setup.
	pathsByDir := make(map[string][]string)
	for f := range newFiles {
		dir := filepath.Dir(f)
		pathsByDir[dir] = append(pathsByDir[dir], f)
	}

	// Set up new watches.
	newPollFiles := make(map[string]time.Time)
	for dir := range watchDirs {
		if isNetworkFS(dir) {
			cw.logFn("config-watch: %s is on a network filesystem, using polling", dir)
			for _, absPath := range pathsByDir[dir] {
				if info, err := os.Stat(absPath); err == nil {
					newPollFiles[absPath] = info.ModTime()
				} else {
					newPollFiles[absPath] = time.Time{}
				}
			}
		} else {
			if err := cw.watcher.Add(dir); err != nil {
				cw.logFn("config-watch: warning: cannot watch %s: %v", dir, err)
			}
		}
	}

	if len(newPollFiles) > 0 {
		cw.mu.Lock()
		cw.pollFiles = newPollFiles
		cw.mu.Unlock()
	}

	cw.logFn("config-watch: monitoring %d config files in %d directories (action=%s)",
		len(newFiles), len(watchDirs), cw.action)
}

// Start begins the event processing loops. Must be called after Update().
func (cw *ConfigWatcher) Start() {
	cw.wg.Add(1)
	go cw.eventLoop()

	cw.wg.Add(1)
	go cw.pollLoop()
}

// Stop stops the watcher and waits for goroutines to exit.
func (cw *ConfigWatcher) Stop() {
	close(cw.stopCh)
	cw.watcher.Close()
	cw.wg.Wait()
}

// eventLoop processes fsnotify events with debouncing.
func (cw *ConfigWatcher) eventLoop() {
	defer cw.wg.Done()

	// Single timer reused across events. Starts stopped; Reset activates it.
	debounceTimer := time.NewTimer(0)
	if !debounceTimer.Stop() {
		<-debounceTimer.C
	}
	defer debounceTimer.Stop()

	for {
		select {
		case event, ok := <-cw.watcher.Events:
			if !ok {
				return
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Remove) == 0 {
				continue
			}
			// Check if this event is for a tracked config file.
			cw.mu.Lock()
			tracked := cw.configFiles[event.Name]
			cw.mu.Unlock()
			if !tracked {
				continue
			}
			// Reset debounce timer — drain channel if Stop reports
			// the timer already fired to prevent a stale tick.
			if !debounceTimer.Stop() {
				select {
				case <-debounceTimer.C:
				default:
				}
			}
			debounceTimer.Reset(configDebounceDelay)

		case <-debounceTimer.C:
			cw.triggerAction()

		case err, ok := <-cw.watcher.Errors:
			if !ok {
				return
			}
			cw.logFn("config-watch: watcher error: %v", err)

		case <-cw.stopCh:
			return
		}
	}
}

// pollLoop periodically checks config files on network filesystems.
func (cw *ConfigWatcher) pollLoop() {
	defer cw.wg.Done()

	ticker := time.NewTicker(cw.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cw.pollCheck()
		case <-cw.stopCh:
			return
		}
	}
}

// pollCheck stats all poll-monitored config files and triggers action if
// any have changed.
func (cw *ConfigWatcher) pollCheck() {
	type polledFile struct {
		path      string
		lastMtime time.Time
	}
	cw.mu.Lock()
	snapshot := make([]polledFile, 0, len(cw.pollFiles))
	for absPath, lastMtime := range cw.pollFiles {
		snapshot = append(snapshot, polledFile{path: absPath, lastMtime: lastMtime})
	}
	cw.mu.Unlock()

	type mtimeUpdate struct {
		path     string
		newMtime time.Time
	}
	var updates []mtimeUpdate
	changed := false
	for _, pf := range snapshot {
		info, err := os.Stat(pf.path)
		if err != nil {
			cw.logFn("config-watch: poll: cannot stat %s: %v", pf.path, err)
			changed = true
			continue
		}
		if !info.ModTime().Equal(pf.lastMtime) {
			updates = append(updates, mtimeUpdate{path: pf.path, newMtime: info.ModTime()})
			changed = true
		}
	}

	if len(updates) > 0 {
		cw.mu.Lock()
		for _, u := range updates {
			if _, ok := cw.pollFiles[u.path]; ok {
				cw.pollFiles[u.path] = u.newMtime
			}
		}
		cw.mu.Unlock()
	}

	if changed {
		cw.triggerAction()
	}
}

// triggerAction executes the configured action (warn or reload).
func (cw *ConfigWatcher) triggerAction() {
	switch cw.action {
	case "warn":
		cw.logFn("config-watch: config file changed (action=warn)")
	case "reload":
		cw.logFn("config-watch: config file changed, triggering reload")
		cw.reloadFn()
	}
}
