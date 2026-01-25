//go:build integration

package fuse_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/cespare/xxhash/v2"
	fuselib "github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/stuckj/mkvdup/internal/dedup"
	fusepkg "github.com/stuckj/mkvdup/internal/fuse"
	"github.com/stuckj/mkvdup/internal/matcher"
	"github.com/stuckj/mkvdup/internal/mkv"
	"github.com/stuckj/mkvdup/internal/source"
	"github.com/stuckj/mkvdup/testdata"
)

// Shared test fixture created once by TestMain.
// Tests that need a dedup file should use these instead of creating their own.
var (
	sharedTestPaths      testdata.Paths
	sharedDedupPath      string // Path to the shared .mkvdup file
	sharedConfigPath     string // Path to the shared .mkvdup.yaml config
	sharedTmpDir         string // Temp directory containing the shared files
	sharedFixtureCreated bool   // True if fixture was successfully created
)

// TestMain sets up shared test fixtures before running tests.
// This creates the dedup file ONCE, which is then reused by all tests.
func TestMain(m *testing.M) {
	// Find test data
	sharedTestPaths = testdata.Find()
	if !sharedTestPaths.Available {
		log.Println("Test data not available. Skipping integration tests.")
		os.Exit(0)
	}

	// Check FUSE availability
	if _, err := os.Stat("/dev/fuse"); os.IsNotExist(err) {
		log.Println("FUSE not available: /dev/fuse does not exist")
		os.Exit(0)
	}

	// Create temp directory for shared test files
	var err error
	sharedTmpDir, err = os.MkdirTemp("", "mkvdup-integration-*")
	if err != nil {
		log.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create the shared dedup file (this is the expensive operation done once)
	sharedDedupPath, sharedConfigPath, err = createSharedDedupFile(sharedTestPaths, sharedTmpDir)
	if err != nil {
		log.Printf("Failed to create shared dedup file: %v", err)
		os.RemoveAll(sharedTmpDir)
		os.Exit(1)
	}
	sharedFixtureCreated = true

	// Run tests
	code := m.Run()

	// Cleanup
	os.RemoveAll(sharedTmpDir)

	os.Exit(code)
}

// createSharedDedupFile creates a dedup file that can be reused by all tests.
// This performs the expensive indexing/matching once.
func createSharedDedupFile(paths testdata.Paths, tmpDir string) (dedupPath, configPath string, err error) {
	dedupPath = filepath.Join(tmpDir, "shared.mkvdup")
	configPath = filepath.Join(tmpDir, "shared.mkvdup.yaml")

	// Parse MKV
	parser, err := mkv.NewParser(paths.MKVFile)
	if err != nil {
		return "", "", fmt.Errorf("create MKV parser: %w", err)
	}
	defer parser.Close()

	if err := parser.Parse(nil); err != nil {
		return "", "", fmt.Errorf("parse MKV: %w", err)
	}

	// Index source
	srcIndexer, err := source.NewIndexer(paths.ISODir, source.DefaultWindowSize)
	if err != nil {
		return "", "", fmt.Errorf("create indexer: %w", err)
	}

	if err := srcIndexer.Build(nil); err != nil {
		return "", "", fmt.Errorf("build index: %w", err)
	}
	index := srcIndexer.Index()
	defer index.Close()

	// Match packets
	m, err := matcher.NewMatcher(index)
	if err != nil {
		return "", "", fmt.Errorf("create matcher: %w", err)
	}
	defer m.Close()

	result, err := m.Match(paths.MKVFile, parser.Packets(), parser.Tracks(), nil)
	if err != nil {
		return "", "", fmt.Errorf("match: %w", err)
	}

	// Get MKV file info and checksum
	mkvInfo, err := os.Stat(paths.MKVFile)
	if err != nil {
		return "", "", fmt.Errorf("stat MKV: %w", err)
	}

	mkvFile, err := os.Open(paths.MKVFile)
	if err != nil {
		return "", "", fmt.Errorf("open MKV: %w", err)
	}
	h := xxhash.New()
	if _, err := io.Copy(h, mkvFile); err != nil {
		mkvFile.Close()
		return "", "", fmt.Errorf("checksum MKV: %w", err)
	}
	mkvChecksum := h.Sum64()
	mkvFile.Close()

	// Convert ES offsets to raw offsets if needed
	var esConverters []source.ESRangeConverter
	if index.UsesESOffsets && len(index.ESReaders) > 0 {
		esConverters = make([]source.ESRangeConverter, len(index.ESReaders))
		for i, r := range index.ESReaders {
			if converter, ok := r.(source.ESRangeConverter); ok {
				esConverters[i] = converter
			}
		}
	}

	// Create dedup file using Writer API
	writer, err := dedup.NewWriter(dedupPath)
	if err != nil {
		return "", "", fmt.Errorf("create writer: %w", err)
	}
	defer writer.Close()

	writer.SetHeader(mkvInfo.Size(), mkvChecksum, srcIndexer.SourceType())
	writer.SetSourceFiles(index.Files)

	if err := writer.SetMatchResult(result, esConverters); err != nil {
		return "", "", fmt.Errorf("set match result: %w", err)
	}

	if err := writer.Write(); err != nil {
		return "", "", fmt.Errorf("write dedup file: %w", err)
	}

	// Write config
	if err := dedup.WriteConfig(configPath, "test.mkv", filepath.Base(dedupPath), paths.ISODir); err != nil {
		return "", "", fmt.Errorf("write config: %w", err)
	}

	log.Printf("Created shared dedup file: %s", dedupPath)
	return dedupPath, configPath, nil
}

// getSharedFixture returns the shared test fixture paths.
// Call this at the start of tests that need the dedup file.
func getSharedFixture(t *testing.T) (dedupPath, configPath string, paths testdata.Paths) {
	t.Helper()
	if !sharedFixtureCreated {
		t.Fatal("Shared test fixture not available")
	}
	return sharedDedupPath, sharedConfigPath, sharedTestPaths
}

// copyConfigWithName copies the shared config to a new location with a different virtual file name.
// This allows tests to mount the same dedup file with different paths (e.g., "Movies/test.mkv").
func copyConfigWithName(t *testing.T, tmpDir, virtualName string) string {
	t.Helper()
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}
	configPath := filepath.Join(tmpDir, "test.mkvdup.yaml")
	if err := dedup.WriteConfig(configPath, virtualName, sharedDedupPath, sharedTestPaths.ISODir); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	return configPath
}

// skipIfFUSEUnavailable skips the test if FUSE is not available.
func skipIfFUSEUnavailable(t *testing.T) {
	// Check if /dev/fuse exists
	if _, err := os.Stat("/dev/fuse"); os.IsNotExist(err) {
		t.Skip("FUSE not available: /dev/fuse does not exist")
	}

	// Check if fusermount is available in PATH (more portable than hardcoded paths)
	if _, err := exec.LookPath("fusermount"); err != nil {
		if _, err := exec.LookPath("fusermount3"); err != nil {
			t.Skip("FUSE not available: fusermount not found in PATH")
		}
	}
}

// createTestDedupFile creates a dedup file from the test data.
// Returns the path to the dedup file and the config path.
//
// NOTE: Most tests should use getSharedFixture() instead to avoid expensive
// repeated indexing/matching. This function is kept for tests that specifically
// need to test dedup file creation itself.
func createTestDedupFile(t *testing.T, paths testdata.Paths, tmpDir string) (string, string) {
	t.Helper()

	dedupPath := filepath.Join(tmpDir, "test.mkvdup")
	configPath := filepath.Join(tmpDir, "test.mkvdup.yaml")

	// Parse MKV
	parser, err := mkv.NewParser(paths.MKVFile)
	if err != nil {
		t.Fatalf("Failed to create MKV parser: %v", err)
	}
	defer parser.Close()

	if err := parser.Parse(nil); err != nil {
		t.Fatalf("Failed to parse MKV: %v", err)
	}

	// Index source
	srcIndexer, err := source.NewIndexer(paths.ISODir, source.DefaultWindowSize)
	if err != nil {
		t.Fatalf("Failed to create indexer: %v", err)
	}

	if err := srcIndexer.Build(nil); err != nil {
		t.Fatalf("Failed to build index: %v", err)
	}
	index := srcIndexer.Index()
	defer index.Close()

	// Match packets
	m, err := matcher.NewMatcher(index)
	if err != nil {
		t.Fatalf("Failed to create matcher: %v", err)
	}
	defer m.Close()

	result, err := m.Match(paths.MKVFile, parser.Packets(), parser.Tracks(), nil)
	if err != nil {
		t.Fatalf("Failed to match: %v", err)
	}

	// Get MKV file info and checksum
	mkvInfo, err := os.Stat(paths.MKVFile)
	if err != nil {
		t.Fatalf("Failed to stat MKV: %v", err)
	}

	mkvFile, err := os.Open(paths.MKVFile)
	if err != nil {
		t.Fatalf("Failed to open MKV: %v", err)
	}
	h := xxhash.New()
	if _, err := io.Copy(h, mkvFile); err != nil {
		mkvFile.Close()
		t.Fatalf("Failed to checksum MKV: %v", err)
	}
	mkvChecksum := h.Sum64()
	mkvFile.Close()

	// Write dedup file
	writer, err := dedup.NewWriter(dedupPath)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	writer.SetHeader(mkvInfo.Size(), mkvChecksum, srcIndexer.SourceType())
	writer.SetSourceFiles(index.Files)

	// Convert ES offsets to raw offsets if we have ES readers (DVD sources)
	var esConverters []source.ESRangeConverter
	if index.UsesESOffsets && len(index.ESReaders) > 0 {
		esConverters = make([]source.ESRangeConverter, len(index.ESReaders))
		for i, r := range index.ESReaders {
			if converter, ok := r.(source.ESRangeConverter); ok {
				esConverters[i] = converter
			}
		}
	}

	if err := writer.SetMatchResult(result, esConverters); err != nil {
		t.Fatalf("Failed to set match result: %v", err)
	}

	if err := writer.Write(); err != nil {
		t.Fatalf("Failed to write dedup file: %v", err)
	}

	// Write config
	if err := dedup.WriteConfig(configPath, "test.mkv", "test.mkvdup", paths.ISODir); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	return dedupPath, configPath
}

func TestFUSEMount_Integration(t *testing.T) {
	skipIfFUSEUnavailable(t)
	_, configPath, _ := getSharedFixture(t)

	// Create temp directory for mount point
	tmpDir, err := os.MkdirTemp("", "mkvdup-fuse-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create mount point
	mountPoint := filepath.Join(tmpDir, "mount")
	if err := os.Mkdir(mountPoint, 0755); err != nil {
		t.Fatalf("Failed to create mount point: %v", err)
	}

	// Create FUSE filesystem
	root, err := fusepkg.NewMKVFS([]string{configPath}, false)
	if err != nil {
		t.Fatalf("Failed to create MKVFS: %v", err)
	}

	// Mount
	server, err := fuselib.Mount(mountPoint, root, &fuselib.Options{
		MountOptions: fuse.MountOptions{
			AllowOther: false,
			Debug:      false,
			Options:    []string{"default_permissions"},
		},
	})
	if err != nil {
		t.Fatalf("Failed to mount: %v", err)
	}

	// Unmount when done
	defer func() {
		if err := server.Unmount(); err != nil {
			t.Logf("Warning: unmount failed: %v", err)
		}
	}()

	// Wait for mount to be ready
	server.WaitMount()

	// Test: List files
	entries, err := os.ReadDir(mountPoint)
	if err != nil {
		t.Fatalf("Failed to read mount directory: %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("Expected 1 file, got %d", len(entries))
	}

	if len(entries) > 0 && entries[0].Name() != "test.mkv" {
		t.Errorf("Expected file name 'test.mkv', got %s", entries[0].Name())
	}
}

func TestFUSERead_Integration(t *testing.T) {
	skipIfFUSEUnavailable(t)
	_, configPath, testPaths := getSharedFixture(t)

	// Create temp directory for mount point
	tmpDir, err := os.MkdirTemp("", "mkvdup-fuse-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create mount point
	mountPoint := filepath.Join(tmpDir, "mount")
	if err := os.Mkdir(mountPoint, 0755); err != nil {
		t.Fatalf("Failed to create mount point: %v", err)
	}

	// Create FUSE filesystem
	root, err := fusepkg.NewMKVFS([]string{configPath}, false)
	if err != nil {
		t.Fatalf("Failed to create MKVFS: %v", err)
	}

	// Mount
	server, err := fuselib.Mount(mountPoint, root, &fuselib.Options{
		MountOptions: fuse.MountOptions{
			AllowOther: false,
			Debug:      false,
			Options:    []string{"default_permissions"},
		},
	})
	if err != nil {
		t.Fatalf("Failed to mount: %v", err)
	}
	defer func() {
		if err := server.Unmount(); err != nil {
			t.Logf("Warning: unmount failed: %v", err)
		}
	}()

	server.WaitMount()

	// Open the virtual MKV file
	virtualPath := filepath.Join(mountPoint, "test.mkv")
	virtualFile, err := os.Open(virtualPath)
	if err != nil {
		t.Fatalf("Failed to open virtual file: %v", err)
	}
	defer virtualFile.Close()

	// Open the original MKV file
	originalFile, err := os.Open(testPaths.MKVFile)
	if err != nil {
		t.Fatalf("Failed to open original file: %v", err)
	}
	defer originalFile.Close()

	// Compare first 64KB
	const compareSize = 64 * 1024
	virtualBuf := make([]byte, compareSize)
	originalBuf := make([]byte, compareSize)

	vn, err := io.ReadFull(virtualFile, virtualBuf)
	if err != nil && err != io.ErrUnexpectedEOF {
		t.Fatalf("Failed to read virtual file: %v", err)
	}

	on, err := io.ReadFull(originalFile, originalBuf)
	if err != nil && err != io.ErrUnexpectedEOF {
		t.Fatalf("Failed to read original file: %v", err)
	}

	if vn != on {
		t.Errorf("Read different amounts: virtual=%d, original=%d", vn, on)
	}

	if !bytes.Equal(virtualBuf[:vn], originalBuf[:on]) {
		t.Error("First 64KB of virtual file does not match original")
		// Find first difference
		for i := 0; i < vn && i < on; i++ {
			if virtualBuf[i] != originalBuf[i] {
				t.Errorf("First difference at offset %d: virtual=0x%02x, original=0x%02x",
					i, virtualBuf[i], originalBuf[i])
				break
			}
		}
	}
}

func TestFUSEFileSize_Integration(t *testing.T) {
	skipIfFUSEUnavailable(t)
	_, configPath, testPaths := getSharedFixture(t)

	// Create temp directory for mount point
	tmpDir, err := os.MkdirTemp("", "mkvdup-fuse-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create mount point
	mountPoint := filepath.Join(tmpDir, "mount")
	if err := os.Mkdir(mountPoint, 0755); err != nil {
		t.Fatalf("Failed to create mount point: %v", err)
	}

	// Create FUSE filesystem
	root, err := fusepkg.NewMKVFS([]string{configPath}, false)
	if err != nil {
		t.Fatalf("Failed to create MKVFS: %v", err)
	}

	// Mount
	server, err := fuselib.Mount(mountPoint, root, &fuselib.Options{
		MountOptions: fuse.MountOptions{
			AllowOther: false,
			Debug:      false,
			Options:    []string{"default_permissions"},
		},
	})
	if err != nil {
		t.Fatalf("Failed to mount: %v", err)
	}
	defer func() {
		if err := server.Unmount(); err != nil {
			t.Logf("Warning: unmount failed: %v", err)
		}
	}()

	server.WaitMount()

	// Get virtual file stats
	virtualPath := filepath.Join(mountPoint, "test.mkv")
	virtualInfo, err := os.Stat(virtualPath)
	if err != nil {
		t.Fatalf("Failed to stat virtual file: %v", err)
	}

	// Get original file stats
	originalInfo, err := os.Stat(testPaths.MKVFile)
	if err != nil {
		t.Fatalf("Failed to stat original file: %v", err)
	}

	if virtualInfo.Size() != originalInfo.Size() {
		t.Errorf("Size mismatch: virtual=%d, original=%d", virtualInfo.Size(), originalInfo.Size())
	}
}

func TestFUSEChecksum_Integration(t *testing.T) {
	skipIfFUSEUnavailable(t)
	_, configPath, testPaths := getSharedFixture(t)

	// Create temp directory for mount point
	tmpDir, err := os.MkdirTemp("", "mkvdup-fuse-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create mount point
	mountPoint := filepath.Join(tmpDir, "mount")
	if err := os.Mkdir(mountPoint, 0755); err != nil {
		t.Fatalf("Failed to create mount point: %v", err)
	}

	// Create FUSE filesystem
	root, err := fusepkg.NewMKVFS([]string{configPath}, false)
	if err != nil {
		t.Fatalf("Failed to create MKVFS: %v", err)
	}

	// Mount
	server, err := fuselib.Mount(mountPoint, root, &fuselib.Options{
		MountOptions: fuse.MountOptions{
			AllowOther: false,
			Debug:      false,
			Options:    []string{"default_permissions"},
		},
	})
	if err != nil {
		t.Fatalf("Failed to mount: %v", err)
	}
	defer func() {
		if err := server.Unmount(); err != nil {
			t.Logf("Warning: unmount failed: %v", err)
		}
	}()

	server.WaitMount()

	// Calculate checksum of virtual file
	virtualPath := filepath.Join(mountPoint, "test.mkv")
	virtualFile, err := os.Open(virtualPath)
	if err != nil {
		t.Fatalf("Failed to open virtual file: %v", err)
	}

	h1 := xxhash.New()
	if _, err := io.Copy(h1, virtualFile); err != nil {
		virtualFile.Close()
		t.Fatalf("Failed to checksum virtual file: %v", err)
	}
	virtualFile.Close()
	virtualChecksum := h1.Sum64()

	// Calculate checksum of original file
	originalFile, err := os.Open(testPaths.MKVFile)
	if err != nil {
		t.Fatalf("Failed to open original file: %v", err)
	}

	h2 := xxhash.New()
	if _, err := io.Copy(h2, originalFile); err != nil {
		originalFile.Close()
		t.Fatalf("Failed to checksum original file: %v", err)
	}
	originalFile.Close()
	originalChecksum := h2.Sum64()

	if virtualChecksum != originalChecksum {
		t.Errorf("Checksum mismatch: virtual=0x%016x, original=0x%016x",
			virtualChecksum, originalChecksum)
	} else {
		t.Logf("Checksums match: 0x%016x", virtualChecksum)
	}
}

func TestFUSEMountUnmount_Integration(t *testing.T) {
	skipIfFUSEUnavailable(t)
	_, configPath, _ := getSharedFixture(t)

	// Create temp directory for mount point
	tmpDir, err := os.MkdirTemp("", "mkvdup-fuse-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create mount point
	mountPoint := filepath.Join(tmpDir, "mount")
	if err := os.Mkdir(mountPoint, 0755); err != nil {
		t.Fatalf("Failed to create mount point: %v", err)
	}

	// Test multiple mount/unmount cycles
	for i := 0; i < 3; i++ {
		t.Logf("Mount cycle %d", i+1)

		root, err := fusepkg.NewMKVFS([]string{configPath}, false)
		if err != nil {
			t.Fatalf("Cycle %d: Failed to create MKVFS: %v", i+1, err)
		}

		server, err := fuselib.Mount(mountPoint, root, &fuselib.Options{
			MountOptions: fuse.MountOptions{
				AllowOther: false,
				Debug:      false,
				Options:    []string{"default_permissions"},
			},
		})
		if err != nil {
			t.Fatalf("Cycle %d: Failed to mount: %v", i+1, err)
		}

		server.WaitMount()

		// Quick read test
		entries, err := os.ReadDir(mountPoint)
		if err != nil {
			t.Fatalf("Cycle %d: Failed to read directory: %v", i+1, err)
		}
		if len(entries) != 1 {
			t.Errorf("Cycle %d: Expected 1 file, got %d", i+1, len(entries))
		}

		if err := server.Unmount(); err != nil {
			t.Fatalf("Cycle %d: Failed to unmount: %v", i+1, err)
		}

		// Small delay to ensure unmount completes
		time.Sleep(100 * time.Millisecond)
	}
}

// TestAdapters_Integration tests the adapter implementations with real data.
func TestAdapters_Integration(t *testing.T) {
	dedupPath, _, paths := getSharedFixture(t)

	// Test DefaultReaderFactory
	factory := &fusepkg.DefaultReaderFactory{}
	reader, err := factory.NewReaderLazy(dedupPath, paths.ISODir)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	// Check size matches original
	originalInfo, err := os.Stat(paths.MKVFile)
	if err != nil {
		t.Fatalf("Failed to stat original: %v", err)
	}

	if reader.OriginalSize() != originalInfo.Size() {
		t.Errorf("Size mismatch: reader=%d, original=%d", reader.OriginalSize(), originalInfo.Size())
	}

	// Initialize for reading
	if err := reader.InitializeForReading(paths.ISODir); err != nil {
		t.Fatalf("Failed to initialize reader: %v", err)
	}

	// Read and compare first 4KB
	buf := make([]byte, 4096)
	n, err := reader.ReadAt(buf, 0)
	if err != nil && err != io.EOF {
		t.Fatalf("Failed to read: %v", err)
	}

	// Read same from original
	originalFile, err := os.Open(paths.MKVFile)
	if err != nil {
		t.Fatalf("Failed to open original: %v", err)
	}
	defer originalFile.Close()

	originalBuf := make([]byte, 4096)
	on, err := originalFile.Read(originalBuf)
	if err != nil && err != io.EOF {
		t.Fatalf("Failed to read original: %v", err)
	}

	if n != on {
		t.Errorf("Read different amounts: reader=%d, original=%d", n, on)
	}

	if !bytes.Equal(buf[:n], originalBuf[:on]) {
		t.Error("Data mismatch in first 4KB")
	}
}

// TestDefaultConfigReader_Integration tests the config reader with a real config.
func TestDefaultConfigReader_Integration(t *testing.T) {
	paths := testdata.SkipIfNotAvailable(t)

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "mkvdup-config-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a config file
	configPath := filepath.Join(tmpDir, "test.yaml")
	if err := dedup.WriteConfig(configPath, "movie.mkv", "movie.mkvdup", paths.ISODir); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Test DefaultConfigReader
	reader := &fusepkg.DefaultConfigReader{}
	config, err := reader.ReadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	if config.Name != "movie.mkv" {
		t.Errorf("Expected name 'movie.mkv', got %s", config.Name)
	}
	if config.DedupFile != "movie.mkvdup" {
		t.Errorf("Expected dedup file 'movie.mkvdup', got %s", config.DedupFile)
	}
	if config.SourceDir != paths.ISODir {
		t.Errorf("Expected source dir %s, got %s", paths.ISODir, config.SourceDir)
	}
}

// TestNewMKVFS_Integration tests creating MKVFS with real data (without mount).
func TestNewMKVFS_Integration(t *testing.T) {
	_, configPath, _ := getSharedFixture(t)

	// Test NewMKVFS
	root, err := fusepkg.NewMKVFS([]string{configPath}, true)
	if err != nil {
		t.Fatalf("Failed to create MKVFS: %v", err)
	}

	// Test Readdir (no caller context needed - kernel handles permissions via default_permissions)
	ctx := context.Background()
	stream, errno := root.Readdir(ctx)
	if errno != 0 {
		t.Fatalf("Readdir failed with errno %d", errno)
	}

	var names []string
	for stream.HasNext() {
		entry, _ := stream.Next()
		names = append(names, entry.Name)
	}

	if len(names) != 1 {
		t.Errorf("Expected 1 file, got %d", len(names))
	}
	if len(names) > 0 && names[0] != "test.mkv" {
		t.Errorf("Expected 'test.mkv', got %s", names[0])
	}
}

// TestFUSEMount_DirectoryStructure tests mounting with nested directory paths.
func TestFUSEMount_DirectoryStructure(t *testing.T) {
	skipIfFUSEUnavailable(t)
	getSharedFixture(t) // Verify fixture is available

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "mkvdup-fuse-dir-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create configs with different virtual paths (all point to same shared dedup file)
	config1 := copyConfigWithName(t, filepath.Join(tmpDir, "action"), "Movies/Action/test.mkv")
	config2 := copyConfigWithName(t, filepath.Join(tmpDir, "comedy"), "Movies/Comedy/funny.mkv")
	config3 := copyConfigWithName(t, filepath.Join(tmpDir, "root"), "root.mkv")

	// Create mount point
	mountPoint := filepath.Join(tmpDir, "mount")
	if err := os.Mkdir(mountPoint, 0755); err != nil {
		t.Fatalf("Failed to create mount point: %v", err)
	}

	// Create FUSE filesystem
	root, err := fusepkg.NewMKVFS([]string{config1, config2, config3}, false)
	if err != nil {
		t.Fatalf("Failed to create MKVFS: %v", err)
	}

	// Mount
	server, err := fuselib.Mount(mountPoint, root, &fuselib.Options{
		MountOptions: fuse.MountOptions{
			AllowOther: false,
			Debug:      false,
			Options:    []string{"default_permissions"},
		},
	})
	if err != nil {
		t.Fatalf("Failed to mount: %v", err)
	}

	// Unmount when done
	defer func() {
		if err := server.Unmount(); err != nil {
			t.Logf("Warning: unmount failed: %v", err)
		}
	}()

	// Wait for mount to be ready
	server.WaitMount()

	// Test: List root directory - should have "Movies" dir and "root.mkv" file
	entries, err := os.ReadDir(mountPoint)
	if err != nil {
		t.Fatalf("Failed to read mount directory: %v", err)
	}

	if len(entries) != 2 {
		t.Errorf("Expected 2 entries at root (Movies dir + root.mkv), got %d", len(entries))
		for _, e := range entries {
			t.Logf("  - %s (dir=%v)", e.Name(), e.IsDir())
		}
	}

	// Check for Movies directory
	moviesPath := filepath.Join(mountPoint, "Movies")
	info, err := os.Stat(moviesPath)
	if err != nil {
		t.Fatalf("Failed to stat Movies directory: %v", err)
	}
	if !info.IsDir() {
		t.Error("Movies should be a directory")
	}

	// Test: List Movies directory - should have Action and Comedy
	moviesEntries, err := os.ReadDir(moviesPath)
	if err != nil {
		t.Fatalf("Failed to read Movies directory: %v", err)
	}

	if len(moviesEntries) != 2 {
		t.Errorf("Expected 2 subdirs in Movies (Action, Comedy), got %d", len(moviesEntries))
	}

	// Test: List Action directory - should have test.mkv
	actionPath := filepath.Join(mountPoint, "Movies", "Action")
	actionEntries, err := os.ReadDir(actionPath)
	if err != nil {
		t.Fatalf("Failed to read Action directory: %v", err)
	}

	if len(actionEntries) != 1 {
		t.Errorf("Expected 1 file in Action, got %d", len(actionEntries))
	}
	if len(actionEntries) > 0 && actionEntries[0].Name() != "test.mkv" {
		t.Errorf("Expected 'test.mkv' in Action, got %s", actionEntries[0].Name())
	}
}

// TestFUSE_ReadFileInSubdirectory tests reading a file through a directory path.
func TestFUSE_ReadFileInSubdirectory(t *testing.T) {
	skipIfFUSEUnavailable(t)
	getSharedFixture(t) // Verify fixture is available

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "mkvdup-fuse-subdir-read-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create config with directory path (uses shared dedup file)
	configPath := copyConfigWithName(t, tmpDir, "Movies/Action/test.mkv")

	// Create mount point
	mountPoint := filepath.Join(tmpDir, "mount")
	if err := os.Mkdir(mountPoint, 0755); err != nil {
		t.Fatalf("Failed to create mount point: %v", err)
	}

	// Create and mount FUSE filesystem
	root, err := fusepkg.NewMKVFS([]string{configPath}, false)
	if err != nil {
		t.Fatalf("Failed to create MKVFS: %v", err)
	}

	server, err := fuselib.Mount(mountPoint, root, &fuselib.Options{
		MountOptions: fuse.MountOptions{
			AllowOther: false,
			Debug:      false,
			Options:    []string{"default_permissions"},
		},
	})
	if err != nil {
		t.Fatalf("Failed to mount: %v", err)
	}

	defer func() {
		if err := server.Unmount(); err != nil {
			t.Logf("Warning: unmount failed: %v", err)
		}
	}()

	server.WaitMount()

	// Read file through directory path
	filePath := filepath.Join(mountPoint, "Movies", "Action", "test.mkv")
	f, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("Failed to open file in subdirectory: %v", err)
	}
	defer f.Close()

	// Read first 1KB
	buf := make([]byte, 1024)
	n, err := f.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if n != 1024 {
		t.Errorf("Expected to read 1024 bytes, got %d", n)
	}

	// Verify it's valid MKV data (starts with EBML header)
	if !bytes.HasPrefix(buf, []byte{0x1A, 0x45, 0xDF, 0xA3}) {
		t.Error("File content doesn't start with EBML header")
	}
}

// TestFUSE_ReadOnlyOperations tests that write operations return EROFS.
func TestFUSE_ReadOnlyOperations(t *testing.T) {
	skipIfFUSEUnavailable(t)
	getSharedFixture(t) // Verify fixture is available

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "mkvdup-fuse-readonly-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create config with directory structure (uses shared dedup file)
	configPath := copyConfigWithName(t, tmpDir, "Movies/test.mkv")

	// Create mount point
	mountPoint := filepath.Join(tmpDir, "mount")
	if err := os.Mkdir(mountPoint, 0755); err != nil {
		t.Fatalf("Failed to create mount point: %v", err)
	}

	// Create and mount FUSE filesystem
	root, err := fusepkg.NewMKVFS([]string{configPath}, false)
	if err != nil {
		t.Fatalf("Failed to create MKVFS: %v", err)
	}

	server, err := fuselib.Mount(mountPoint, root, &fuselib.Options{
		MountOptions: fuse.MountOptions{
			AllowOther: false,
			Debug:      false,
			Options:    []string{"default_permissions"},
		},
	})
	if err != nil {
		t.Fatalf("Failed to mount: %v", err)
	}

	defer func() {
		if err := server.Unmount(); err != nil {
			t.Logf("Warning: unmount failed: %v", err)
		}
	}()

	server.WaitMount()

	moviesDir := filepath.Join(mountPoint, "Movies")

	// Test mkdir should fail with EROFS
	err = os.Mkdir(filepath.Join(moviesDir, "NewDir"), 0755)
	if err == nil {
		t.Error("Expected mkdir to fail with EROFS")
	}

	// Test creating a file should fail with EROFS
	_, err = os.Create(filepath.Join(moviesDir, "newfile.txt"))
	if err == nil {
		t.Error("Expected file creation to fail with EROFS")
	}

	// Test removing a file should fail with EROFS
	err = os.Remove(filepath.Join(moviesDir, "test.mkv"))
	if err == nil {
		t.Error("Expected file removal to fail with EROFS")
	}
}

func TestFUSE_ChmodFile(t *testing.T) {
	skipIfFUSEUnavailable(t)
	_, configPath, _ := getSharedFixture(t)

	tmpDir, err := os.MkdirTemp("", "mkvdup-fuse-perm-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mountPoint := filepath.Join(tmpDir, "mount")
	if err := os.Mkdir(mountPoint, 0755); err != nil {
		t.Fatalf("Failed to create mount point: %v", err)
	}

	// Create permission store path
	permPath := filepath.Join(tmpDir, "permissions.yaml")

	// Get current user's UID/GID - files must be owned by current user to chmod
	currentUID := uint32(os.Getuid())
	currentGID := uint32(os.Getgid())

	// Create FUSE filesystem with permission store, owned by current user
	root, err := fusepkg.NewMKVFSWithOptions([]string{configPath}, fusepkg.MKVFSOptions{
		Verbose:         false,
		PermissionsPath: permPath,
		Defaults: &fusepkg.Defaults{
			FileUID:  currentUID,
			FileGID:  currentGID,
			FileMode: 0444,
			DirUID:   currentUID,
			DirGID:   currentGID,
			DirMode:  0555,
		},
	})
	if err != nil {
		t.Fatalf("Failed to create MKVFS: %v", err)
	}

	server, err := fuselib.Mount(mountPoint, root, &fuselib.Options{
		MountOptions: fuse.MountOptions{
			AllowOther: false,
			Debug:      false,
			Options:    []string{"default_permissions"},
		},
	})
	if err != nil {
		t.Fatalf("Failed to mount: %v", err)
	}

	defer func() {
		if err := server.Unmount(); err != nil {
			t.Logf("Warning: unmount failed: %v", err)
		}
	}()

	server.WaitMount()

	filePath := filepath.Join(mountPoint, "test.mkv")

	// Get initial permissions
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}
	initialMode := info.Mode().Perm()
	t.Logf("Initial mode: %o", initialMode)

	// Change permissions to 0640
	newMode := os.FileMode(0640)
	if err := os.Chmod(filePath, newMode); err != nil {
		t.Fatalf("Failed to chmod: %v", err)
	}

	// Verify the change
	info, err = os.Stat(filePath)
	if err != nil {
		t.Fatalf("Failed to stat file after chmod: %v", err)
	}
	if info.Mode().Perm() != newMode {
		t.Errorf("Expected mode %o, got %o", newMode, info.Mode().Perm())
	}

	// Verify permissions file was created
	if _, err := os.Stat(permPath); os.IsNotExist(err) {
		t.Error("Permissions file was not created")
	}
}

func TestFUSE_ChmodDirectory(t *testing.T) {
	skipIfFUSEUnavailable(t)
	getSharedFixture(t) // Verify fixture is available

	tmpDir, err := os.MkdirTemp("", "mkvdup-fuse-perm-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create config with directory path in virtual name (reuses shared dedup file)
	configPath := copyConfigWithName(t, tmpDir, "Movies/test.mkv")

	mountPoint := filepath.Join(tmpDir, "mount")
	if err := os.Mkdir(mountPoint, 0755); err != nil {
		t.Fatalf("Failed to create mount point: %v", err)
	}

	permPath := filepath.Join(tmpDir, "permissions.yaml")

	// Get current user's UID/GID - directories must be owned by current user to chmod
	currentUID := uint32(os.Getuid())
	currentGID := uint32(os.Getgid())

	root, err := fusepkg.NewMKVFSWithOptions([]string{configPath}, fusepkg.MKVFSOptions{
		Verbose:         false,
		PermissionsPath: permPath,
		Defaults: &fusepkg.Defaults{
			FileUID:  currentUID,
			FileGID:  currentGID,
			FileMode: 0444,
			DirUID:   currentUID,
			DirGID:   currentGID,
			DirMode:  0555,
		},
	})
	if err != nil {
		t.Fatalf("Failed to create MKVFS: %v", err)
	}

	server, err := fuselib.Mount(mountPoint, root, &fuselib.Options{
		MountOptions: fuse.MountOptions{
			AllowOther: false,
			Debug:      false,
			Options:    []string{"default_permissions"},
		},
	})
	if err != nil {
		t.Fatalf("Failed to mount: %v", err)
	}

	defer func() {
		if err := server.Unmount(); err != nil {
			t.Logf("Warning: unmount failed: %v", err)
		}
	}()

	server.WaitMount()

	dirPath := filepath.Join(mountPoint, "Movies")

	// Get initial permissions
	info, err := os.Stat(dirPath)
	if err != nil {
		t.Fatalf("Failed to stat directory: %v", err)
	}
	t.Logf("Initial dir mode: %o", info.Mode().Perm())

	// Change permissions to 0750
	newMode := os.FileMode(0750)
	if err := os.Chmod(dirPath, newMode); err != nil {
		t.Fatalf("Failed to chmod directory: %v", err)
	}

	// Verify the change
	info, err = os.Stat(dirPath)
	if err != nil {
		t.Fatalf("Failed to stat directory after chmod: %v", err)
	}
	if info.Mode().Perm() != newMode {
		t.Errorf("Expected mode %o, got %o", newMode, info.Mode().Perm())
	}
}

func TestFUSE_ChownFile(t *testing.T) {
	skipIfFUSEUnavailable(t)
	_, configPath, _ := getSharedFixture(t)

	// chown requires root
	if os.Geteuid() != 0 {
		t.Skip("chown requires root privileges")
	}

	tmpDir, err := os.MkdirTemp("", "mkvdup-fuse-perm-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mountPoint := filepath.Join(tmpDir, "mount")
	if err := os.Mkdir(mountPoint, 0755); err != nil {
		t.Fatalf("Failed to create mount point: %v", err)
	}

	permPath := filepath.Join(tmpDir, "permissions.yaml")

	root, err := fusepkg.NewMKVFSWithOptions([]string{configPath}, fusepkg.MKVFSOptions{
		Verbose:         false,
		PermissionsPath: permPath,
	})
	if err != nil {
		t.Fatalf("Failed to create MKVFS: %v", err)
	}

	server, err := fuselib.Mount(mountPoint, root, &fuselib.Options{
		MountOptions: fuse.MountOptions{
			AllowOther: false,
			Debug:      false,
			Options:    []string{"default_permissions"},
		},
	})
	if err != nil {
		t.Fatalf("Failed to mount: %v", err)
	}

	defer func() {
		if err := server.Unmount(); err != nil {
			t.Logf("Warning: unmount failed: %v", err)
		}
	}()

	server.WaitMount()

	filePath := filepath.Join(mountPoint, "test.mkv")

	// Change ownership to 1000:1000
	if err := os.Chown(filePath, 1000, 1000); err != nil {
		t.Fatalf("Failed to chown: %v", err)
	}

	// Verify the change via stat
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Failed to stat file after chown: %v", err)
	}

	// Get uid/gid from stat
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatal("Failed to get syscall.Stat_t from FileInfo")
	}

	if stat.Uid != 1000 {
		t.Errorf("Expected UID 1000, got %d", stat.Uid)
	}
	if stat.Gid != 1000 {
		t.Errorf("Expected GID 1000, got %d", stat.Gid)
	}
}

func TestFUSE_PermissionDenied(t *testing.T) {
	skipIfFUSEUnavailable(t)
	_, configPath, _ := getSharedFixture(t)

	// This test requires running as non-root to test permission denial
	if os.Geteuid() == 0 {
		t.Skip("Permission denial test requires non-root user")
	}

	tmpDir, err := os.MkdirTemp("", "mkvdup-fuse-perm-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mountPoint := filepath.Join(tmpDir, "mount")
	if err := os.Mkdir(mountPoint, 0755); err != nil {
		t.Fatalf("Failed to create mount point: %v", err)
	}

	permPath := filepath.Join(tmpDir, "permissions.yaml")

	// Create FUSE filesystem with custom defaults that deny access
	// File owned by root with mode 0600 - non-root can't read
	root, err := fusepkg.NewMKVFSWithOptions([]string{configPath}, fusepkg.MKVFSOptions{
		Verbose:         false,
		PermissionsPath: permPath,
		Defaults: &fusepkg.Defaults{
			FileUID:  0,    // root
			FileGID:  0,    // root
			FileMode: 0600, // owner read/write only
			DirUID:   0,
			DirGID:   0,
			DirMode:  0755, // directories need to be accessible
		},
	})
	if err != nil {
		t.Fatalf("Failed to create MKVFS: %v", err)
	}

	server, err := fuselib.Mount(mountPoint, root, &fuselib.Options{
		MountOptions: fuse.MountOptions{
			AllowOther: false,
			Debug:      false,
			Options:    []string{"default_permissions"},
		},
	})
	if err != nil {
		t.Fatalf("Failed to mount: %v", err)
	}

	defer func() {
		if err := server.Unmount(); err != nil {
			t.Logf("Warning: unmount failed: %v", err)
		}
	}()

	server.WaitMount()

	filePath := filepath.Join(mountPoint, "test.mkv")

	// Try to open the file - should fail with permission denied
	_, err = os.Open(filePath)
	if err == nil {
		t.Error("Expected permission denied error when opening root-owned file with mode 0600")
	} else if !os.IsPermission(err) {
		t.Errorf("Expected permission error, got: %v", err)
	} else {
		t.Logf("Got expected permission error: %v", err)
	}
}

func TestFUSE_PermissionAllowed_OwnerAccess(t *testing.T) {
	skipIfFUSEUnavailable(t)
	_, configPath, _ := getSharedFixture(t)

	tmpDir, err := os.MkdirTemp("", "mkvdup-fuse-perm-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mountPoint := filepath.Join(tmpDir, "mount")
	if err := os.Mkdir(mountPoint, 0755); err != nil {
		t.Fatalf("Failed to create mount point: %v", err)
	}

	permPath := filepath.Join(tmpDir, "permissions.yaml")

	// Get current user's UID/GID
	uid := uint32(os.Getuid())
	gid := uint32(os.Getgid())

	// Create FUSE filesystem with files owned by current user, owner-only read
	root, err := fusepkg.NewMKVFSWithOptions([]string{configPath}, fusepkg.MKVFSOptions{
		Verbose:         false,
		PermissionsPath: permPath,
		Defaults: &fusepkg.Defaults{
			FileUID:  uid,
			FileGID:  gid,
			FileMode: 0400, // owner read only
			DirUID:   uid,
			DirGID:   gid,
			DirMode:  0700, // owner only
		},
	})
	if err != nil {
		t.Fatalf("Failed to create MKVFS: %v", err)
	}

	server, err := fuselib.Mount(mountPoint, root, &fuselib.Options{
		MountOptions: fuse.MountOptions{
			AllowOther: false,
			Debug:      false,
			Options:    []string{"default_permissions"},
		},
	})
	if err != nil {
		t.Fatalf("Failed to mount: %v", err)
	}

	defer func() {
		if err := server.Unmount(); err != nil {
			t.Logf("Warning: unmount failed: %v", err)
		}
	}()

	server.WaitMount()

	filePath := filepath.Join(mountPoint, "test.mkv")

	// Owner should be able to read their own file
	f, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("Owner should be able to read own file with mode 0400: %v", err)
	}
	defer f.Close()

	buf := make([]byte, 1024)
	n, err := f.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	t.Logf("Owner successfully read %d bytes from file with mode 0400", n)
}

func TestFUSE_PermissionAllowed_GroupAccess(t *testing.T) {
	skipIfFUSEUnavailable(t)
	_, configPath, _ := getSharedFixture(t)

	// Skip if running as root (root bypasses all permission checks)
	if os.Geteuid() == 0 {
		t.Skip("Group access test requires non-root user")
	}

	tmpDir, err := os.MkdirTemp("", "mkvdup-fuse-perm-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mountPoint := filepath.Join(tmpDir, "mount")
	if err := os.Mkdir(mountPoint, 0755); err != nil {
		t.Fatalf("Failed to create mount point: %v", err)
	}

	permPath := filepath.Join(tmpDir, "permissions.yaml")

	// Get current user's primary GID
	gid := uint32(os.Getgid())

	// Create FUSE filesystem with files owned by different user but same group
	// Using UID 0 (root) as a different user, current user's GID, group-readable
	root, err := fusepkg.NewMKVFSWithOptions([]string{configPath}, fusepkg.MKVFSOptions{
		Verbose:         false,
		PermissionsPath: permPath,
		Defaults: &fusepkg.Defaults{
			FileUID:  0,    // different owner (root)
			FileGID:  gid,  // current user's primary group
			FileMode: 0040, // group read only (no owner, no other)
			DirUID:   0,
			DirGID:   gid,
			DirMode:  0050, // group read+execute
		},
	})
	if err != nil {
		t.Fatalf("Failed to create MKVFS: %v", err)
	}

	server, err := fuselib.Mount(mountPoint, root, &fuselib.Options{
		MountOptions: fuse.MountOptions{
			AllowOther: false,
			Debug:      false,
			Options:    []string{"default_permissions"},
		},
	})
	if err != nil {
		t.Fatalf("Failed to mount: %v", err)
	}

	defer func() {
		if err := server.Unmount(); err != nil {
			t.Logf("Warning: unmount failed: %v", err)
		}
	}()

	server.WaitMount()

	filePath := filepath.Join(mountPoint, "test.mkv")

	// User should be able to read file via primary group membership
	f, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("User should be able to read file via primary group (gid=%d) with mode 0040: %v", gid, err)
	}
	defer f.Close()

	buf := make([]byte, 1024)
	n, err := f.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	t.Logf("Successfully read %d bytes via primary group access (gid=%d)", n, gid)
}

func TestFUSE_PermissionAllowed_SupplementaryGroupAccess(t *testing.T) {
	skipIfFUSEUnavailable(t)
	_, configPath, _ := getSharedFixture(t)

	// Skip if running as root (root bypasses all permission checks)
	if os.Geteuid() == 0 {
		t.Skip("Supplementary group access test requires non-root user")
	}

	// Get supplementary groups
	groups, err := os.Getgroups()
	if err != nil {
		t.Fatalf("Failed to get supplementary groups: %v", err)
	}

	// Find a supplementary group that's different from primary GID
	primaryGid := os.Getgid()
	var supplementaryGid int = -1
	for _, g := range groups {
		if g != primaryGid {
			supplementaryGid = g
			break
		}
	}

	if supplementaryGid == -1 {
		t.Skip("No supplementary groups available (only primary GID)")
	}

	t.Logf("Testing access via supplementary group %d (primary gid=%d)", supplementaryGid, primaryGid)

	tmpDir, err := os.MkdirTemp("", "mkvdup-fuse-perm-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mountPoint := filepath.Join(tmpDir, "mount")
	if err := os.Mkdir(mountPoint, 0755); err != nil {
		t.Fatalf("Failed to create mount point: %v", err)
	}

	permPath := filepath.Join(tmpDir, "permissions.yaml")

	// Create FUSE filesystem with files owned by different user and supplementary group
	root, err := fusepkg.NewMKVFSWithOptions([]string{configPath}, fusepkg.MKVFSOptions{
		Verbose:         false,
		PermissionsPath: permPath,
		Defaults: &fusepkg.Defaults{
			FileUID:  0,                        // different owner (root)
			FileGID:  uint32(supplementaryGid), // supplementary group
			FileMode: 0040,                     // group read only
			DirUID:   0,
			DirGID:   uint32(supplementaryGid),
			DirMode:  0050, // group read+execute
		},
	})
	if err != nil {
		t.Fatalf("Failed to create MKVFS: %v", err)
	}

	server, err := fuselib.Mount(mountPoint, root, &fuselib.Options{
		MountOptions: fuse.MountOptions{
			AllowOther: false,
			Debug:      false,
			Options:    []string{"default_permissions"},
		},
	})
	if err != nil {
		t.Fatalf("Failed to mount: %v", err)
	}

	defer func() {
		if err := server.Unmount(); err != nil {
			t.Logf("Warning: unmount failed: %v", err)
		}
	}()

	server.WaitMount()

	filePath := filepath.Join(mountPoint, "test.mkv")

	// User should be able to read file via supplementary group membership
	// This tests that the kernel's default_permissions properly checks supplementary groups
	f, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("User should be able to read file via supplementary group (gid=%d) with mode 0040: %v", supplementaryGid, err)
	}
	defer f.Close()

	buf := make([]byte, 1024)
	n, err := f.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	t.Logf("Successfully read %d bytes via supplementary group access (gid=%d)", n, supplementaryGid)
}

func TestFUSE_PermissionDenied_NotInGroup(t *testing.T) {
	skipIfFUSEUnavailable(t)
	_, configPath, _ := getSharedFixture(t)

	// Skip if running as root (root bypasses all permission checks)
	if os.Geteuid() == 0 {
		t.Skip("Permission denial test requires non-root user")
	}

	// Find a GID that the current user is NOT a member of
	groups, err := os.Getgroups()
	if err != nil {
		t.Fatalf("Failed to get groups: %v", err)
	}
	groupSet := make(map[int]bool)
	for _, g := range groups {
		groupSet[g] = true
	}

	// Try to find a GID we're not a member of (start from a high number)
	var nonMemberGid uint32 = 65534 // nobody group, likely not a member
	for gid := uint32(1000); gid < 65534; gid++ {
		if !groupSet[int(gid)] {
			nonMemberGid = gid
			break
		}
	}

	t.Logf("Testing access denied for group %d (user is not a member)", nonMemberGid)

	tmpDir, err := os.MkdirTemp("", "mkvdup-fuse-perm-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mountPoint := filepath.Join(tmpDir, "mount")
	if err := os.Mkdir(mountPoint, 0755); err != nil {
		t.Fatalf("Failed to create mount point: %v", err)
	}

	permPath := filepath.Join(tmpDir, "permissions.yaml")

	// Create FUSE filesystem with files owned by different user and a group we're not in
	root, err := fusepkg.NewMKVFSWithOptions([]string{configPath}, fusepkg.MKVFSOptions{
		Verbose:         false,
		PermissionsPath: permPath,
		Defaults: &fusepkg.Defaults{
			FileUID:  0,            // different owner (root)
			FileGID:  nonMemberGid, // group we're not a member of
			FileMode: 0040,         // group read only (no owner, no other)
			DirUID:   0,
			DirGID:   nonMemberGid,
			DirMode:  0750, // owner+group can access
		},
	})
	if err != nil {
		t.Fatalf("Failed to create MKVFS: %v", err)
	}

	server, err := fuselib.Mount(mountPoint, root, &fuselib.Options{
		MountOptions: fuse.MountOptions{
			AllowOther: false,
			Debug:      false,
			Options:    []string{"default_permissions"},
		},
	})
	if err != nil {
		t.Fatalf("Failed to mount: %v", err)
	}

	defer func() {
		if err := server.Unmount(); err != nil {
			t.Logf("Warning: unmount failed: %v", err)
		}
	}()

	server.WaitMount()

	filePath := filepath.Join(mountPoint, "test.mkv")

	// User should NOT be able to read file (not owner, not in group, no other permissions)
	_, err = os.Open(filePath)
	if err == nil {
		t.Error("Expected permission denied when user is not in file's group with mode 0040")
	} else if !os.IsPermission(err) {
		t.Errorf("Expected permission error, got: %v", err)
	} else {
		t.Logf("Got expected permission denied error: %v", err)
	}
}

func TestFUSE_RootBypassesPermissions(t *testing.T) {
	skipIfFUSEUnavailable(t)
	_, configPath, _ := getSharedFixture(t)

	// This test requires root
	if os.Geteuid() != 0 {
		t.Skip("Root bypass test requires root privileges")
	}

	tmpDir, err := os.MkdirTemp("", "mkvdup-fuse-perm-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mountPoint := filepath.Join(tmpDir, "mount")
	if err := os.Mkdir(mountPoint, 0755); err != nil {
		t.Fatalf("Failed to create mount point: %v", err)
	}

	permPath := filepath.Join(tmpDir, "permissions.yaml")

	// Create FUSE filesystem with files that have NO permissions (mode 0000)
	root, err := fusepkg.NewMKVFSWithOptions([]string{configPath}, fusepkg.MKVFSOptions{
		Verbose:         false,
		PermissionsPath: permPath,
		Defaults: &fusepkg.Defaults{
			FileUID:  1000, // not root
			FileGID:  1000, // not root
			FileMode: 0000, // no permissions at all
			DirUID:   1000,
			DirGID:   1000,
			DirMode:  0000, // no permissions at all
		},
	})
	if err != nil {
		t.Fatalf("Failed to create MKVFS: %v", err)
	}

	server, err := fuselib.Mount(mountPoint, root, &fuselib.Options{
		MountOptions: fuse.MountOptions{
			AllowOther: false,
			Debug:      false,
			Options:    []string{"default_permissions"},
		},
	})
	if err != nil {
		t.Fatalf("Failed to mount: %v", err)
	}

	defer func() {
		if err := server.Unmount(); err != nil {
			t.Logf("Warning: unmount failed: %v", err)
		}
	}()

	server.WaitMount()

	filePath := filepath.Join(mountPoint, "test.mkv")

	// Root should be able to read file even with mode 0000
	f, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("Root should bypass all permission checks, but got: %v", err)
	}
	defer f.Close()

	buf := make([]byte, 1024)
	n, err := f.Read(buf)
	if err != nil {
		t.Fatalf("Root failed to read file: %v", err)
	}
	t.Logf("Root successfully read %d bytes from file with mode 0000", n)
}
