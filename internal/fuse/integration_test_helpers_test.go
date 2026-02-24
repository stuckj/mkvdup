//go:build integration

package fuse_test

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/cespare/xxhash/v2"
	"github.com/stuckj/mkvdup/internal/dedup"
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
	skipReason           string // Non-empty if tests should be skipped
)

// TestMain sets up shared test fixtures before running tests.
// This creates the dedup file ONCE, which is then reused by all tests.
func TestMain(m *testing.M) {
	// Find test data
	sharedTestPaths = testdata.Find()
	if !sharedTestPaths.Available {
		skipReason = "Test data not available (set MKVDUP_TESTDATA)"
		log.Printf("WARNING: %s - tests will be skipped", skipReason)
		os.Exit(m.Run()) // Run tests so they show as skipped
	}

	// Check FUSE availability
	if reason := checkFUSEAvailability(); reason != "" {
		skipReason = reason
		log.Printf("WARNING: %s - tests will be skipped", skipReason)
		os.Exit(m.Run()) // Run tests so they show as skipped
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
	if skipReason != "" {
		t.Skip(skipReason)
	}
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

// checkFUSEAvailability checks whether FUSE is available on the system.
// Returns a non-empty reason string if FUSE is unavailable, or empty string if available.
func checkFUSEAvailability() string {
	if _, err := os.Stat("/dev/fuse"); os.IsNotExist(err) {
		return "FUSE not available: /dev/fuse does not exist"
	}
	if _, err := exec.LookPath("fusermount"); err != nil {
		if _, err := exec.LookPath("fusermount3"); err != nil {
			return "FUSE not available: fusermount not found in PATH"
		}
	}
	return ""
}

// skipIfFUSEUnavailable skips the test if FUSE is not available.
func skipIfFUSEUnavailable(t *testing.T) {
	if reason := checkFUSEAvailability(); reason != "" {
		t.Skip(reason)
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
