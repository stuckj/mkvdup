//go:build integration

package fuse_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/stuckj/mkvdup/internal/dedup"
	"github.com/stuckj/mkvdup/internal/matcher"
	"github.com/stuckj/mkvdup/internal/source"
)

// watchBinary returns the path to the mkvdup binary, building it if necessary.
var (
	watchBinaryPath string
	watchBinaryOnce sync.Once
	watchBinaryErr  error
)

func getWatchTestBinary(t testing.TB) string {
	t.Helper()
	watchBinaryOnce.Do(func() {
		tmpDir, err := os.MkdirTemp("", "mkvdup-watch-build-*")
		if err != nil {
			watchBinaryErr = fmt.Errorf("create temp dir: %w", err)
			return
		}
		binaryName := "mkvdup"
		if runtime.GOOS == "windows" {
			binaryName = "mkvdup.exe"
		}
		watchBinaryPath = filepath.Join(tmpDir, binaryName)

		_, thisFile, _, _ := runtime.Caller(0)
		moduleRoot := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))

		cmd := exec.Command("go", "build", "-o", watchBinaryPath, "./cmd/mkvdup")
		cmd.Dir = moduleRoot
		output, err := cmd.CombinedOutput()
		if err != nil {
			watchBinaryErr = fmt.Errorf("build mkvdup: %w\n%s", err, output)
			return
		}
	})
	if watchBinaryErr != nil {
		t.Fatalf("Failed to build mkvdup: %v", watchBinaryErr)
	}
	return watchBinaryPath
}

// watchFixture contains paths for a synthetic source-watcher test setup.
type watchFixture struct {
	TmpDir     string // Root temp directory (cleaned up by t.TempDir)
	SourceDir  string // Directory containing source file
	SourcePath string // Path to source file
	SourceData []byte // Original source file content (for restoration)
	MKVData    []byte // Original "MKV" data (for read verification)
	DedupPath  string // Path to .mkvdup file
	ConfigPath string // Path to .mkvdup.yaml config
	MountPoint string // FUSE mount point directory
}

// createWatchFixture creates a lightweight synthetic test fixture.
// The dedup file is all-delta (no source matches), so reads succeed
// without real source data. The source file is declared in the header
// so the watcher monitors it via inotify.
func createWatchFixture(t *testing.T) watchFixture {
	t.Helper()

	tmpDir := t.TempDir()

	// Create source directory with a small file
	sourceDir := filepath.Join(tmpDir, "source")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("Failed to create source dir: %v", err)
	}

	sourceData := make([]byte, 4096)
	for i := range sourceData {
		sourceData[i] = byte((i*13 + 5) % 251)
	}
	sourcePath := filepath.Join(sourceDir, "data.bin")
	if err := os.WriteFile(sourcePath, sourceData, 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	// Create "MKV" data (arbitrary content, stored as delta)
	mkvData := make([]byte, 4096)
	for i := range mkvData {
		mkvData[i] = byte((i*7 + 3) % 251)
	}

	// Create dedup file with all-delta entries
	dedupPath := filepath.Join(tmpDir, "test.mkvdup")
	writer, err := dedup.NewWriter(dedupPath)
	if err != nil {
		t.Fatalf("Failed to create dedup writer: %v", err)
	}

	writer.SetHeader(int64(len(mkvData)), xxhash.Sum64(mkvData), source.TypeDVD)
	writer.SetSourceFiles([]source.File{{
		RelativePath: "data.bin",
		Size:         int64(len(sourceData)),
		Checksum:     xxhash.Sum64(sourceData),
	}})

	if err := writer.SetMatchResult(&matcher.Result{
		Entries: []matcher.Entry{{
			MkvOffset: 0,
			Length:    int64(len(mkvData)),
			Source:    0, // delta
		}},
		DeltaData:      mkvData,
		UnmatchedBytes: int64(len(mkvData)),
	}, nil); err != nil {
		t.Fatalf("Failed to set match result: %v", err)
	}

	if err := writer.Write(); err != nil {
		t.Fatalf("Failed to write dedup file: %v", err)
	}
	writer.Close()

	// Create config
	configPath := filepath.Join(tmpDir, "test.mkvdup.yaml")
	if err := dedup.WriteConfig(configPath, "test.mkv", dedupPath, sourceDir); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Create mount point
	mountPoint := filepath.Join(tmpDir, "mount")
	if err := os.Mkdir(mountPoint, 0755); err != nil {
		t.Fatalf("Failed to create mount point: %v", err)
	}

	return watchFixture{
		TmpDir:     tmpDir,
		SourceDir:  sourceDir,
		SourcePath: sourcePath,
		SourceData: sourceData,
		MKVData:    mkvData,
		DedupPath:  dedupPath,
		ConfigPath: configPath,
		MountPoint: mountPoint,
	}
}

// startMount starts the mkvdup binary in foreground mode and waits for the
// mount to become ready. It registers t.Cleanup to unmount and stop the process.
func startMount(t *testing.T, binary, mountPoint string, configs []string, extraArgs ...string) *exec.Cmd {
	t.Helper()

	args := []string{"mount", "-f"}
	args = append(args, extraArgs...)
	args = append(args, mountPoint)
	args = append(args, configs...)

	cmd := exec.Command(binary, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start mkvdup mount: %v", err)
	}

	// Register cleanup to ensure unmount even on test failure
	t.Cleanup(func() {
		// Try fusermount first for clean unmount
		fusermount := "fusermount"
		if p, err := exec.LookPath("fusermount3"); err == nil {
			fusermount = p
		}
		exec.Command(fusermount, "-u", mountPoint).Run()

		// Then signal the process
		if cmd.Process != nil {
			cmd.Process.Signal(syscall.SIGTERM)
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				cmd.Process.Kill()
				<-done
			}
		}
	})

	// Wait for mount to be ready
	virtualFile := filepath.Join(mountPoint, "test.mkv")
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(virtualFile); err == nil {
			return cmd
		}
		// Check if process exited prematurely (Signal(0) fails if dead)
		if cmd.Process.Signal(syscall.Signal(0)) != nil {
			t.Fatalf("mkvdup mount exited prematurely")
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("Mount did not become ready within 10 seconds")
	return nil
}

// waitForEIO polls the given path until Open() returns EIO or the timeout expires.
func waitForEIO(path string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		f, err := os.Open(path)
		if err != nil {
			if errors.Is(err, syscall.EIO) {
				return true
			}
		} else {
			f.Close()
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// waitForReadable polls the given path until Open() succeeds or the timeout expires.
func waitForReadable(path string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		f, err := os.Open(path)
		if err == nil {
			f.Close()
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// readVirtualFile reads the entire virtual file and returns its contents.
func readVirtualFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read virtual file: %v", err)
	}
	return data
}

// TestSourceWatch_NoSourceWatch verifies that --no-source-watch prevents
// the watcher from disabling files when source files change.
func TestSourceWatch_NoSourceWatch(t *testing.T) {
	skipIfFUSEUnavailable(t)
	binary := getWatchTestBinary(t)
	fix := createWatchFixture(t)

	startMount(t, binary, fix.MountPoint, []string{fix.ConfigPath}, "--no-source-watch")

	virtualPath := filepath.Join(fix.MountPoint, "test.mkv")

	// Verify file is readable
	data := readVirtualFile(t, virtualPath)
	if !bytes.Equal(data, fix.MKVData) {
		t.Fatal("Virtual file content doesn't match expected data")
	}

	// Truncate source file (drastic change)
	if err := os.Truncate(fix.SourcePath, 0); err != nil {
		t.Fatalf("Failed to truncate source file: %v", err)
	}

	// Wait a moment for any potential inotify events
	time.Sleep(500 * time.Millisecond)

	// File should still be readable (no watcher active)
	data = readVirtualFile(t, virtualPath)
	if !bytes.Equal(data, fix.MKVData) {
		t.Fatal("Virtual file content changed after source modification with --no-source-watch")
	}
}

// TestSourceWatch_WarnMode verifies that --on-source-change=warn only logs
// a warning without disabling the virtual file.
func TestSourceWatch_WarnMode(t *testing.T) {
	skipIfFUSEUnavailable(t)
	binary := getWatchTestBinary(t)
	fix := createWatchFixture(t)

	startMount(t, binary, fix.MountPoint, []string{fix.ConfigPath}, "--on-source-change", "warn")

	virtualPath := filepath.Join(fix.MountPoint, "test.mkv")

	// Verify file is readable
	data := readVirtualFile(t, virtualPath)
	if !bytes.Equal(data, fix.MKVData) {
		t.Fatal("Virtual file content doesn't match expected data")
	}

	// Truncate source file
	if err := os.Truncate(fix.SourcePath, 0); err != nil {
		t.Fatalf("Failed to truncate source file: %v", err)
	}

	// Wait for watcher to process the event
	time.Sleep(500 * time.Millisecond)

	// File should still be readable (warn mode only logs)
	data = readVirtualFile(t, virtualPath)
	if !bytes.Equal(data, fix.MKVData) {
		t.Fatal("Virtual file content changed after source modification with warn mode")
	}
}

// TestSourceWatch_DisableMode verifies that --on-source-change=disable
// immediately disables virtual files when source files change.
func TestSourceWatch_DisableMode(t *testing.T) {
	skipIfFUSEUnavailable(t)
	binary := getWatchTestBinary(t)
	fix := createWatchFixture(t)

	startMount(t, binary, fix.MountPoint, []string{fix.ConfigPath}, "--on-source-change", "disable")

	virtualPath := filepath.Join(fix.MountPoint, "test.mkv")

	// Verify file is readable
	data := readVirtualFile(t, virtualPath)
	if !bytes.Equal(data, fix.MKVData) {
		t.Fatal("Virtual file content doesn't match expected data")
	}

	// Truncate source file
	if err := os.Truncate(fix.SourcePath, 0); err != nil {
		t.Fatalf("Failed to truncate source file: %v", err)
	}

	// Wait for EIO
	if !waitForEIO(virtualPath, 5*time.Second) {
		t.Fatal("Expected EIO after source file truncation in disable mode")
	}
}

// TestSourceWatch_Checksum_Touch verifies that touching a source file
// (timestamp-only change) does NOT disable the virtual file when using
// checksum mode, because the checksum matches.
func TestSourceWatch_Checksum_Touch(t *testing.T) {
	skipIfFUSEUnavailable(t)
	binary := getWatchTestBinary(t)
	fix := createWatchFixture(t)

	// Default mode is checksum
	startMount(t, binary, fix.MountPoint, []string{fix.ConfigPath})

	virtualPath := filepath.Join(fix.MountPoint, "test.mkv")

	// Verify file is readable
	data := readVirtualFile(t, virtualPath)
	if !bytes.Equal(data, fix.MKVData) {
		t.Fatal("Virtual file content doesn't match expected data")
	}

	// Touch the source file (changes mtime, not content)
	now := time.Now()
	if err := os.Chtimes(fix.SourcePath, now, now); err != nil {
		t.Fatalf("Failed to touch source file: %v", err)
	}

	// Wait for checksum verification to complete
	time.Sleep(2 * time.Second)

	// File should still be readable (checksum matches)
	data = readVirtualFile(t, virtualPath)
	if !bytes.Equal(data, fix.MKVData) {
		t.Fatal("Virtual file was disabled after touch (checksum should have matched)")
	}
}

// TestSourceWatch_Checksum_SizeChange verifies that changing the source
// file's size immediately disables the virtual file (no checksum needed).
func TestSourceWatch_Checksum_SizeChange(t *testing.T) {
	skipIfFUSEUnavailable(t)
	binary := getWatchTestBinary(t)
	fix := createWatchFixture(t)

	startMount(t, binary, fix.MountPoint, []string{fix.ConfigPath})

	virtualPath := filepath.Join(fix.MountPoint, "test.mkv")

	// Verify file is readable
	data := readVirtualFile(t, virtualPath)
	if !bytes.Equal(data, fix.MKVData) {
		t.Fatal("Virtual file content doesn't match expected data")
	}

	// Append a byte to the source file (size change)
	f, err := os.OpenFile(fix.SourcePath, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		t.Fatalf("Failed to open source file for append: %v", err)
	}
	if _, err := f.Write([]byte{0xFF}); err != nil {
		f.Close()
		t.Fatalf("Failed to append to source file: %v", err)
	}
	f.Close()

	// Wait for EIO (should be immediate — size change skips checksum)
	if !waitForEIO(virtualPath, 5*time.Second) {
		t.Fatal("Expected EIO after source file size change in checksum mode")
	}
}

// TestSourceWatch_Checksum_ContentChange verifies that modifying source
// file content (same size) triggers checksum verification, and the file
// is disabled when the checksum mismatches.
func TestSourceWatch_Checksum_ContentChange(t *testing.T) {
	skipIfFUSEUnavailable(t)
	binary := getWatchTestBinary(t)
	fix := createWatchFixture(t)

	startMount(t, binary, fix.MountPoint, []string{fix.ConfigPath})

	virtualPath := filepath.Join(fix.MountPoint, "test.mkv")

	// Verify file is readable
	data := readVirtualFile(t, virtualPath)
	if !bytes.Equal(data, fix.MKVData) {
		t.Fatal("Virtual file content doesn't match expected data")
	}

	// Flip a byte in the source file (same size, different content)
	modified := make([]byte, len(fix.SourceData))
	copy(modified, fix.SourceData)
	modified[1000] ^= 0xFF
	if err := os.WriteFile(fix.SourcePath, modified, 0644); err != nil {
		t.Fatalf("Failed to write modified source file: %v", err)
	}

	// Wait for EIO (after checksum mismatch detection)
	if !waitForEIO(virtualPath, 5*time.Second) {
		t.Fatal("Expected EIO after source file content change (checksum mismatch)")
	}
}

// TestSourceWatch_Reload_Recovery verifies that sending SIGHUP to the
// mount process re-enables a disabled virtual file.
func TestSourceWatch_Reload_Recovery(t *testing.T) {
	skipIfFUSEUnavailable(t)
	binary := getWatchTestBinary(t)
	fix := createWatchFixture(t)

	cmd := startMount(t, binary, fix.MountPoint, []string{fix.ConfigPath}, "--on-source-change", "disable")

	virtualPath := filepath.Join(fix.MountPoint, "test.mkv")

	// Verify file is readable
	data := readVirtualFile(t, virtualPath)
	if !bytes.Equal(data, fix.MKVData) {
		t.Fatal("Virtual file content doesn't match expected data")
	}

	// Truncate source file to trigger disable
	if err := os.Truncate(fix.SourcePath, 0); err != nil {
		t.Fatalf("Failed to truncate source file: %v", err)
	}

	// Wait for EIO
	if !waitForEIO(virtualPath, 5*time.Second) {
		t.Fatal("Expected EIO after source file truncation")
	}

	// Restore source file to original content
	if err := os.WriteFile(fix.SourcePath, fix.SourceData, 0644); err != nil {
		t.Fatalf("Failed to restore source file: %v", err)
	}

	// Send SIGHUP to trigger reload (re-enables disabled files)
	if err := cmd.Process.Signal(syscall.SIGHUP); err != nil {
		t.Fatalf("Failed to send SIGHUP: %v", err)
	}

	// Wait for file to become readable again
	if !waitForReadable(virtualPath, 5*time.Second) {
		t.Fatal("Expected file to become readable again after SIGHUP reload")
	}

	// Verify content is correct
	data = readVirtualFile(t, virtualPath)
	if !bytes.Equal(data, fix.MKVData) {
		t.Fatal("Virtual file content doesn't match after reload recovery")
	}
}
