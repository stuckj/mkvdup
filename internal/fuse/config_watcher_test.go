package fuse

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// waitForCondition polls a condition function until it returns true or the
// deadline is reached. Returns true if the condition was met.
func waitForCondition(deadline time.Time, check func() bool) bool {
	for time.Now().Before(deadline) {
		if check() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return check()
}

func TestConfigWatcher_WarnAction(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "test.yaml")
	if err := os.WriteFile(cfgFile, []byte("initial"), 0644); err != nil {
		t.Fatal(err)
	}

	var warned atomic.Int32
	logFn := func(format string, args ...interface{}) {
		// Count warn log messages
		warned.Add(1)
	}

	cw, err := NewConfigWatcher("warn", time.Second, nil, logFn)
	if err != nil {
		t.Fatal(err)
	}
	cw.Update([]string{cfgFile})
	cw.Start()
	defer cw.Stop()

	// Modify the config file
	if err := os.WriteFile(cfgFile, []byte("modified"), 0644); err != nil {
		t.Fatal(err)
	}

	// Wait for debounce + processing with deadline
	deadline := time.Now().Add(5 * time.Second)
	if !waitForCondition(deadline, func() bool { return warned.Load() >= 2 }) {
		t.Errorf("expected at least 2 log messages (update + warn), got %d", warned.Load())
	}
}

func TestConfigWatcher_ReloadAction(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "test.yaml")
	if err := os.WriteFile(cfgFile, []byte("initial"), 0644); err != nil {
		t.Fatal(err)
	}

	var reloaded atomic.Int32
	reloadFn := func() {
		reloaded.Add(1)
	}
	logFn := func(format string, args ...interface{}) {}

	cw, err := NewConfigWatcher("reload", time.Second, reloadFn, logFn)
	if err != nil {
		t.Fatal(err)
	}
	cw.Update([]string{cfgFile})
	cw.Start()
	defer cw.Stop()

	// Modify the config file
	if err := os.WriteFile(cfgFile, []byte("modified"), 0644); err != nil {
		t.Fatal(err)
	}

	// Wait for reload with deadline
	deadline := time.Now().Add(5 * time.Second)
	if !waitForCondition(deadline, func() bool { return reloaded.Load() >= 1 }) {
		t.Errorf("expected 1 reload, got %d", reloaded.Load())
	}
}

func TestConfigWatcher_Debounce(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "test.yaml")
	if err := os.WriteFile(cfgFile, []byte("v1"), 0644); err != nil {
		t.Fatal(err)
	}

	var reloaded atomic.Int32
	reloadFn := func() {
		reloaded.Add(1)
	}
	logFn := func(format string, args ...interface{}) {}

	cw, err := NewConfigWatcher("reload", time.Second, reloadFn, logFn)
	if err != nil {
		t.Fatal(err)
	}
	cw.Update([]string{cfgFile})
	cw.Start()
	defer cw.Stop()

	// Rapid successive writes (simulating editor save)
	for i := 0; i < 5; i++ {
		if err := os.WriteFile(cfgFile, []byte("v"+string(rune('2'+i))), 0644); err != nil {
			t.Fatal(err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Wait for the single debounced reload
	deadline := time.Now().Add(5 * time.Second)
	if !waitForCondition(deadline, func() bool { return reloaded.Load() >= 1 }) {
		t.Errorf("expected 1 reload (debounced), got %d", reloaded.Load())
	}

	// Give extra time to confirm no additional reloads fire
	time.Sleep(configDebounceDelay + 100*time.Millisecond)
	if reloaded.Load() != 1 {
		t.Errorf("expected exactly 1 reload (debounced), got %d", reloaded.Load())
	}
}

func TestConfigWatcher_IgnoresUnrelatedFiles(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "tracked.yaml")
	otherFile := filepath.Join(dir, "untracked.txt")
	if err := os.WriteFile(cfgFile, []byte("config"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(otherFile, []byte("other"), 0644); err != nil {
		t.Fatal(err)
	}

	var reloaded atomic.Int32
	reloadFn := func() {
		reloaded.Add(1)
	}
	logFn := func(format string, args ...interface{}) {}

	cw, err := NewConfigWatcher("reload", time.Second, reloadFn, logFn)
	if err != nil {
		t.Fatal(err)
	}
	cw.Update([]string{cfgFile})
	cw.Start()
	defer cw.Stop()

	// Modify unrelated file
	if err := os.WriteFile(otherFile, []byte("modified"), 0644); err != nil {
		t.Fatal(err)
	}

	// Wait past the debounce window to confirm no reload fires
	time.Sleep(configDebounceDelay + 200*time.Millisecond)

	if reloaded.Load() != 0 {
		t.Errorf("expected 0 reloads for unrelated file change, got %d", reloaded.Load())
	}
}

func TestConfigWatcher_UpdateReplacesPaths(t *testing.T) {
	dir := t.TempDir()
	cfg1 := filepath.Join(dir, "a.yaml")
	cfg2 := filepath.Join(dir, "b.yaml")
	if err := os.WriteFile(cfg1, []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg2, []byte("b"), 0644); err != nil {
		t.Fatal(err)
	}

	var reloaded atomic.Int32
	reloadFn := func() {
		reloaded.Add(1)
	}
	logFn := func(format string, args ...interface{}) {}

	cw, err := NewConfigWatcher("reload", time.Second, reloadFn, logFn)
	if err != nil {
		t.Fatal(err)
	}
	// Start watching cfg1
	cw.Update([]string{cfg1})
	cw.Start()
	defer cw.Stop()

	// Switch to watching cfg2 only
	cw.Update([]string{cfg2})

	// Modify old config (cfg1) - should not trigger reload
	if err := os.WriteFile(cfg1, []byte("a-modified"), 0644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(configDebounceDelay + 200*time.Millisecond)
	if reloaded.Load() != 0 {
		t.Errorf("expected 0 reloads for old config file, got %d", reloaded.Load())
	}

	// Modify new config (cfg2) - should trigger reload
	if err := os.WriteFile(cfg2, []byte("b-modified"), 0644); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	if !waitForCondition(deadline, func() bool { return reloaded.Load() >= 1 }) {
		t.Errorf("expected 1 reload for new config file, got %d", reloaded.Load())
	}
}
