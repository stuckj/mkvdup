//go:build integration

package fuse_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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
	paths := testdata.SkipIfNotAvailable(t)

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "mkvdup-fuse-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create dedup file
	_, configPath := createTestDedupFile(t, paths, tmpDir)

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
	paths := testdata.SkipIfNotAvailable(t)

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "mkvdup-fuse-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create dedup file
	_, configPath := createTestDedupFile(t, paths, tmpDir)

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
	originalFile, err := os.Open(paths.MKVFile)
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
	paths := testdata.SkipIfNotAvailable(t)

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "mkvdup-fuse-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create dedup file
	_, configPath := createTestDedupFile(t, paths, tmpDir)

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
	originalInfo, err := os.Stat(paths.MKVFile)
	if err != nil {
		t.Fatalf("Failed to stat original file: %v", err)
	}

	if virtualInfo.Size() != originalInfo.Size() {
		t.Errorf("Size mismatch: virtual=%d, original=%d", virtualInfo.Size(), originalInfo.Size())
	}
}

func TestFUSEChecksum_Integration(t *testing.T) {
	skipIfFUSEUnavailable(t)
	paths := testdata.SkipIfNotAvailable(t)

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "mkvdup-fuse-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create dedup file
	_, configPath := createTestDedupFile(t, paths, tmpDir)

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
	originalFile, err := os.Open(paths.MKVFile)
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
	paths := testdata.SkipIfNotAvailable(t)

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "mkvdup-fuse-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create dedup file
	_, configPath := createTestDedupFile(t, paths, tmpDir)

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
	paths := testdata.SkipIfNotAvailable(t)

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "mkvdup-adapter-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create dedup file
	dedupPath, _ := createTestDedupFile(t, paths, tmpDir)

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
	paths := testdata.SkipIfNotAvailable(t)

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "mkvdup-mkvfs-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create dedup file and config
	_, configPath := createTestDedupFile(t, paths, tmpDir)

	// Test NewMKVFS
	root, err := fusepkg.NewMKVFS([]string{configPath}, true)
	if err != nil {
		t.Fatalf("Failed to create MKVFS: %v", err)
	}

	// Test Readdir (use root credentials for permission checks)
	ctx := fusepkg.ContextWithCaller(context.Background(), 0, 0)
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

// testDedupData holds pre-computed data for creating multiple dedup files efficiently.
// This avoids repeating expensive indexing/matching operations.
type testDedupData struct {
	paths        testdata.Paths
	mkvSize      int64
	mkvChecksum  uint64
	sourceType   source.Type
	sourceFiles  []source.File
	result       *matcher.Result
	esConverters []source.ESRangeConverter
	index        *source.Index
	matcher      *matcher.Matcher
}

// prepareTestDedupData performs expensive indexing and matching once.
// Caller must call cleanup() when done.
func prepareTestDedupData(t *testing.T, paths testdata.Paths) (*testDedupData, func()) {
	t.Helper()

	// Parse MKV
	parser, err := mkv.NewParser(paths.MKVFile)
	if err != nil {
		t.Fatalf("Failed to create MKV parser: %v", err)
	}

	if err := parser.Parse(nil); err != nil {
		parser.Close()
		t.Fatalf("Failed to parse MKV: %v", err)
	}

	// Index source
	srcIndexer, err := source.NewIndexer(paths.ISODir, source.DefaultWindowSize)
	if err != nil {
		parser.Close()
		t.Fatalf("Failed to create indexer: %v", err)
	}

	if err := srcIndexer.Build(nil); err != nil {
		parser.Close()
		t.Fatalf("Failed to build index: %v", err)
	}
	index := srcIndexer.Index()

	// Match packets
	m, err := matcher.NewMatcher(index)
	if err != nil {
		index.Close()
		parser.Close()
		t.Fatalf("Failed to create matcher: %v", err)
	}

	result, err := m.Match(paths.MKVFile, parser.Packets(), parser.Tracks(), nil)
	if err != nil {
		m.Close()
		index.Close()
		parser.Close()
		t.Fatalf("Failed to match: %v", err)
	}

	// Get MKV file info and checksum
	mkvInfo, err := os.Stat(paths.MKVFile)
	if err != nil {
		m.Close()
		index.Close()
		parser.Close()
		t.Fatalf("Failed to stat MKV: %v", err)
	}

	mkvFile, err := os.Open(paths.MKVFile)
	if err != nil {
		m.Close()
		index.Close()
		parser.Close()
		t.Fatalf("Failed to open MKV: %v", err)
	}
	h := xxhash.New()
	if _, err := io.Copy(h, mkvFile); err != nil {
		mkvFile.Close()
		m.Close()
		index.Close()
		parser.Close()
		t.Fatalf("Failed to checksum MKV: %v", err)
	}
	mkvChecksum := h.Sum64()
	mkvFile.Close()

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

	data := &testDedupData{
		paths:        paths,
		mkvSize:      mkvInfo.Size(),
		mkvChecksum:  mkvChecksum,
		sourceType:   srcIndexer.SourceType(),
		sourceFiles:  index.Files,
		result:       result,
		esConverters: esConverters,
		index:        index,
		matcher:      m,
	}

	cleanup := func() {
		m.Close()
		index.Close()
		parser.Close()
	}

	return data, cleanup
}

// writeTestDedupFile creates a dedup file using pre-computed data.
func (d *testDedupData) writeTestDedupFile(t *testing.T, tmpDir, name, dedupName string) string {
	t.Helper()

	dedupPath := filepath.Join(tmpDir, dedupName)
	configPath := filepath.Join(tmpDir, dedupName+".yaml")

	writer, err := dedup.NewWriter(dedupPath)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	writer.SetHeader(d.mkvSize, d.mkvChecksum, d.sourceType)
	writer.SetSourceFiles(d.sourceFiles)

	if err := writer.SetMatchResult(d.result, d.esConverters); err != nil {
		t.Fatalf("Failed to set match result: %v", err)
	}

	if err := writer.Write(); err != nil {
		t.Fatalf("Failed to write dedup file: %v", err)
	}

	if err := dedup.WriteConfig(configPath, name, dedupName, d.paths.ISODir); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	return configPath
}

// createTestDedupFileWithName is a convenience wrapper for tests that only need one dedup file.
// For tests creating multiple dedup files, use prepareTestDedupData + writeTestDedupFile instead.
func createTestDedupFileWithName(t *testing.T, paths testdata.Paths, tmpDir, name, dedupName string) string {
	t.Helper()
	data, cleanup := prepareTestDedupData(t, paths)
	defer cleanup()
	return data.writeTestDedupFile(t, tmpDir, name, dedupName)
}

// TestFUSEMount_DirectoryStructure tests mounting with nested directory paths.
func TestFUSEMount_DirectoryStructure(t *testing.T) {
	skipIfFUSEUnavailable(t)
	paths := testdata.SkipIfNotAvailable(t)

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "mkvdup-fuse-dir-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Prepare test data once (expensive indexing/matching)
	data, cleanup := prepareTestDedupData(t, paths)
	defer cleanup()

	// Create dedup files with directory paths in names (fast - reuses prepared data)
	config1 := data.writeTestDedupFile(t, tmpDir, "Movies/Action/test.mkv", "action.mkvdup")
	config2 := data.writeTestDedupFile(t, tmpDir, "Movies/Comedy/funny.mkv", "comedy.mkvdup")
	config3 := data.writeTestDedupFile(t, tmpDir, "root.mkv", "root.mkvdup")

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
	paths := testdata.SkipIfNotAvailable(t)

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "mkvdup-fuse-subdir-read-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create dedup file with directory path
	configPath := createTestDedupFileWithName(t, paths, tmpDir, "Movies/Action/test.mkv", "test.mkvdup")

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
	paths := testdata.SkipIfNotAvailable(t)

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "mkvdup-fuse-readonly-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create dedup file with directory structure
	configPath := createTestDedupFileWithName(t, paths, tmpDir, "Movies/test.mkv", "test.mkvdup")

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
