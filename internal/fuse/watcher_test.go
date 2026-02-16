package fuse

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/stuckj/mkvdup/internal/dedup"
)

// mockReaderWithSources extends mockReader with SourceFileInfo support.
type mockReaderWithSources struct {
	mockReader
	sources []SourceFileInfo
}

func (m *mockReaderWithSources) SourceFileInfo() []SourceFileInfo {
	return m.sources
}

// mockReaderFactoryWithSources creates mockReaderWithSources instances.
type mockReaderFactoryWithSources struct {
	readers map[string]*mockReaderWithSources
}

func (f *mockReaderFactoryWithSources) NewReaderLazy(dedupPath, sourceDir string) (ReaderInitializer, error) {
	if reader, ok := f.readers[dedupPath]; ok {
		return reader, nil
	}
	return nil, fmt.Errorf("reader not found for path: %s", dedupPath)
}

// newTestWatcher creates a SourceWatcher for testing. The caller should call
// sw.watcher.Close() in a defer or cleanup to release the inotify fd.
func newTestWatcher(t *testing.T, action string) (*SourceWatcher, *logCapture) {
	t.Helper()
	lc := &logCapture{}
	sw, err := NewSourceWatcher(action, 0, nil, lc.logFn)
	if err != nil {
		t.Fatalf("NewSourceWatcher(%q): %v", action, err)
	}
	t.Cleanup(func() { sw.watcher.Close() })
	return sw, lc
}

// logCapture captures log messages for test assertions.
type logCapture struct {
	mu       sync.Mutex
	messages []string
}

func (lc *logCapture) logFn(format string, args ...interface{}) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.messages = append(lc.messages, fmt.Sprintf(format, args...))
}

func (lc *logCapture) contains(t *testing.T, substr string) bool {
	t.Helper()
	lc.mu.Lock()
	defer lc.mu.Unlock()
	for _, msg := range lc.messages {
		if strings.Contains(msg, substr) {
			return true
		}
	}
	return false
}

// isDisabled returns the disabled state of an MKVFile, thread-safely.
func isDisabled(f *MKVFile) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.disabled
}

func TestSourceWatcher_WarnAction(t *testing.T) {
	sw, lc := newTestWatcher(t, "warn")

	file := &MKVFile{Name: "movie.mkv"}
	srcPath := "/some/source/VIDEO_TS/VTS_01_1.VOB"

	sw.mu.Lock()
	sw.reverse[srcPath] = []*MKVFile{file}
	sw.checksums[srcPath] = 0xdeadbeef
	sw.sizes[srcPath] = 1024
	sw.handleChangeLocked(srcPath)
	sw.mu.Unlock()

	// Verify a warning was logged.
	if !lc.contains(t, "WARNING") {
		t.Error("expected log message containing 'WARNING'")
	}
	if !lc.contains(t, srcPath) {
		t.Errorf("expected log message containing source path %s", srcPath)
	}

	// File should NOT be disabled.
	if isDisabled(file) {
		t.Error("file should not be disabled with 'warn' action")
	}
}

func TestSourceWatcher_DisableAction(t *testing.T) {
	sw, lc := newTestWatcher(t, "disable")

	file := &MKVFile{Name: "movie.mkv"}
	srcPath := "/some/source/VIDEO_TS/VTS_01_1.VOB"

	sw.mu.Lock()
	sw.reverse[srcPath] = []*MKVFile{file}
	sw.checksums[srcPath] = 0xdeadbeef
	sw.sizes[srcPath] = 1024
	sw.handleChangeLocked(srcPath)
	sw.mu.Unlock()

	// Verify log message was generated.
	if !lc.contains(t, "disabling") {
		t.Error("expected log message containing 'disabling'")
	}

	// File should be disabled.
	if !isDisabled(file) {
		t.Error("file should be disabled with 'disable' action")
	}
}

func TestSourceWatcher_ChecksumAction_SizeChanged(t *testing.T) {
	sw, lc := newTestWatcher(t, "checksum")

	// Create a real temp file so os.Stat succeeds in handleChangeLocked.
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "source.vob")
	content := []byte("hello world")
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	file := &MKVFile{Name: "movie.mkv"}

	sw.mu.Lock()
	sw.reverse[tmpFile] = []*MKVFile{file}
	sw.checksums[tmpFile] = 0xdeadbeef
	// Set an expected size that differs from the actual file size.
	sw.sizes[tmpFile] = int64(len(content)) + 999
	sw.handleChangeLocked(tmpFile)
	sw.mu.Unlock()

	// File should be disabled immediately due to size mismatch.
	if !isDisabled(file) {
		t.Error("file should be disabled when source file size changed")
	}

	if !lc.contains(t, "size changed") {
		t.Error("expected log message about size change")
	}
}

func TestSourceWatcher_ChecksumAction_SizeMatch_ChecksumOK(t *testing.T) {
	sw, lc := newTestWatcher(t, "checksum")

	// Create a real temp file.
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "source.vob")
	content := []byte("the quick brown fox jumps over the lazy dog")
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	// Compute the real xxhash checksum of the content.
	h := xxhash.New()
	h.Write(content)
	realChecksum := h.Sum64()

	file := &MKVFile{Name: "movie.mkv"}

	sw.mu.Lock()
	sw.reverse[tmpFile] = []*MKVFile{file}
	sw.checksums[tmpFile] = realChecksum
	sw.sizes[tmpFile] = int64(len(content))
	sw.handleChangeLocked(tmpFile)
	sw.mu.Unlock()

	// File should NOT be disabled yet (queued for checksum verification).
	if isDisabled(file) {
		t.Error("file should not be disabled before checksum verification")
	}

	// Start the checksum worker to process the queued request.
	sw.wg.Add(1)
	go sw.checksumWorker()

	// Wait for verification to complete by observing the expected log message.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if lc.contains(t, "checksum verified OK") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Stop the worker.
	close(sw.stopCh)
	sw.wg.Wait()

	if !lc.contains(t, "checksum verified OK") {
		t.Fatal("timed out waiting for checksum verification")
	}

	// File should still NOT be disabled since checksum matched.
	if isDisabled(file) {
		t.Error("file should not be disabled when checksum matches")
	}
}

func TestSourceWatcher_ChecksumAction_SizeMatch_ChecksumMismatch(t *testing.T) {
	sw, lc := newTestWatcher(t, "checksum")

	// Create a real temp file.
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "source.vob")
	content := []byte("some source file content for testing")
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	file := &MKVFile{Name: "movie.mkv"}

	sw.mu.Lock()
	sw.reverse[tmpFile] = []*MKVFile{file}
	// Set a WRONG checksum but correct size.
	sw.checksums[tmpFile] = 0xbadbadbadbadbad
	sw.sizes[tmpFile] = int64(len(content))
	sw.handleChangeLocked(tmpFile)
	sw.mu.Unlock()

	// File should NOT be disabled yet (queued for checksum verification).
	if isDisabled(file) {
		t.Error("file should not be disabled before checksum verification")
	}

	// Start the checksum worker.
	sw.wg.Add(1)
	go sw.checksumWorker()

	// Wait for verification to complete by observing the expected log message.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if lc.contains(t, "checksum mismatch") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Stop the worker.
	close(sw.stopCh)
	sw.wg.Wait()

	if !lc.contains(t, "checksum mismatch") {
		t.Fatal("timed out waiting for checksum mismatch detection")
	}

	// File should be disabled due to checksum mismatch.
	if !isDisabled(file) {
		t.Error("file should be disabled when checksum mismatches")
	}
}

func TestSourceWatcher_ChecksumAction_FileMissing(t *testing.T) {
	sw, lc := newTestWatcher(t, "checksum")

	// Use a path that does not exist.
	missingPath := filepath.Join(t.TempDir(), "nonexistent.vob")

	file := &MKVFile{Name: "movie.mkv"}

	sw.mu.Lock()
	sw.reverse[missingPath] = []*MKVFile{file}
	sw.checksums[missingPath] = 0xdeadbeef
	sw.sizes[missingPath] = 1024
	sw.handleChangeLocked(missingPath)
	sw.mu.Unlock()

	// File should be disabled immediately since the source file is missing.
	if !isDisabled(file) {
		t.Error("file should be disabled when source file is missing")
	}

	if !lc.contains(t, "missing") {
		t.Error("expected log message about missing source file")
	}
}

func TestSourceWatcher_ChecksumDedup(t *testing.T) {
	sw, _ := newTestWatcher(t, "checksum")

	// Create a real temp file.
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "source.vob")
	content := []byte("dedup test content")
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	file := &MKVFile{Name: "movie.mkv"}

	sw.mu.Lock()
	sw.reverse[tmpFile] = []*MKVFile{file}
	sw.checksums[tmpFile] = 0xdeadbeef
	sw.sizes[tmpFile] = int64(len(content))

	// Call handleChangeLocked twice for the same path.
	sw.handleChangeLocked(tmpFile)
	sw.handleChangeLocked(tmpFile)
	sw.mu.Unlock()

	// Only one request should be in the channel due to dedup.
	if got := len(sw.checksumCh); got != 1 {
		t.Errorf("expected 1 item in checksumCh, got %d", got)
	}

	// Verify the pending flag is set.
	sw.mu.RLock()
	pending := sw.checksumPending[tmpFile]
	sw.mu.RUnlock()
	if !pending {
		t.Error("expected checksumPending to be true for the path")
	}
}

func TestSourceWatcher_Update(t *testing.T) {
	sw, _ := newTestWatcher(t, "warn")

	// Use a temp directory as the source dir so that inotify watch setup
	// works (local filesystem, directory exists).
	sourceDir := t.TempDir()

	// Create the source file so that the watcher can watch its directory.
	relPath := "VIDEO_TS/VTS_01_1.VOB"
	absPath := filepath.Join(sourceDir, relPath)
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(absPath, []byte("dummy"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	file1 := &MKVFile{
		Name:      "movie1.mkv",
		DedupPath: "/data/movie1.dedup",
		SourceDir: sourceDir,
	}
	file2 := &MKVFile{
		Name:      "movie2.mkv",
		DedupPath: "/data/movie2.dedup",
		SourceDir: sourceDir,
	}

	factory := &mockReaderFactoryWithSources{
		readers: map[string]*mockReaderWithSources{
			"/data/movie1.dedup": {
				mockReader: mockReader{originalSize: 100},
				sources: []SourceFileInfo{
					{RelativePath: relPath, Size: 5, Checksum: 0xabc},
				},
			},
			"/data/movie2.dedup": {
				mockReader: mockReader{originalSize: 200},
				sources: []SourceFileInfo{
					{RelativePath: relPath, Size: 5, Checksum: 0xabc},
				},
			},
		},
	}

	files := map[string]*MKVFile{
		"movie1.mkv": file1,
		"movie2.mkv": file2,
	}

	sw.Update(files, factory)

	// Verify the reverse map was built correctly.
	sw.mu.RLock()
	defer sw.mu.RUnlock()

	affected, ok := sw.reverse[absPath]
	if !ok {
		t.Fatalf("expected reverse map entry for %s", absPath)
	}
	if len(affected) != 2 {
		t.Errorf("expected 2 affected files for %s, got %d", absPath, len(affected))
	}

	// Verify both file1 and file2 are in the affected list.
	foundFile1, foundFile2 := false, false
	for _, f := range affected {
		if f == file1 {
			foundFile1 = true
		}
		if f == file2 {
			foundFile2 = true
		}
	}
	if !foundFile1 {
		t.Error("expected file1 (movie1.mkv) in affected list")
	}
	if !foundFile2 {
		t.Error("expected file2 (movie2.mkv) in affected list")
	}

	// Verify checksum and size were stored.
	if sw.checksums[absPath] != 0xabc {
		t.Errorf("expected checksum 0xabc, got %#x", sw.checksums[absPath])
	}
	if sw.sizes[absPath] != 5 {
		t.Errorf("expected size 5, got %d", sw.sizes[absPath])
	}
}

func TestSourceWatcher_DisableAction_WithNotifier(t *testing.T) {
	tmpDir := t.TempDir()
	marker := filepath.Join(tmpDir, "notifier-marker.txt")

	errCmd := &dedup.ErrorCommandConfig{
		Command: dedup.CommandValue{
			IsShell: true,
			Args:    []string{"echo '%source% %event%' > " + marker},
		},
		Timeout:       5 * time.Second,
		BatchInterval: 50 * time.Millisecond,
	}

	lc := &logCapture{}
	sw, err := NewSourceWatcher("disable", 0, errCmd, lc.logFn)
	if err != nil {
		t.Fatalf("NewSourceWatcher: %v", err)
	}
	t.Cleanup(func() {
		sw.notifier.Stop()
		sw.watcher.Close()
	})

	file := &MKVFile{Name: "movie.mkv"}
	srcPath := "/some/source/VIDEO_TS/VTS_01_1.VOB"

	sw.mu.Lock()
	sw.reverse[srcPath] = []*MKVFile{file}
	sw.checksums[srcPath] = 0xdeadbeef
	sw.sizes[srcPath] = 1024
	sw.handleChangeLocked(srcPath)
	sw.mu.Unlock()

	// File should be disabled.
	if !isDisabled(file) {
		t.Error("file should be disabled with 'disable' action")
	}

	// Wait for the notifier batch interval to flush and the command to execute
	// by polling for the marker file with a deadline to avoid flakiness on slow CI.
	var data []byte
	deadline := time.Now().Add(5 * time.Second)
	for {
		var readErr error
		data, readErr = os.ReadFile(marker)
		if readErr == nil {
			break
		}
		if !os.IsNotExist(readErr) {
			t.Fatalf("error reading notifier marker file: %v", readErr)
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for notifier marker file to be created")
		}
		time.Sleep(10 * time.Millisecond)
	}

	content := strings.TrimSpace(string(data))
	if !strings.Contains(content, srcPath) {
		t.Errorf("marker missing source path, got: %q", content)
	}
	if !strings.Contains(content, "changed") {
		t.Errorf("marker missing event type, got: %q", content)
	}
}
