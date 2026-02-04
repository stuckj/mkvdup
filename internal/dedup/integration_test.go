//go:build integration

package dedup_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/stuckj/mkvdup/internal/dedup"
	"github.com/stuckj/mkvdup/testdata"
)

// mkvdupBinary returns the path to the mkvdup binary, building it if necessary.
// The binary is built once per test run and cached.
var (
	builtBinary     string
	builtBinaryOnce sync.Once
	builtBinaryErr  error
)

func getMkvdupBinary(t testing.TB) string {
	t.Helper()
	builtBinaryOnce.Do(func() {
		// Build the binary in a temp directory
		tmpDir, err := os.MkdirTemp("", "mkvdup-build-*")
		if err != nil {
			builtBinaryErr = fmt.Errorf("create temp dir: %w", err)
			return
		}
		binaryName := "mkvdup"
		if runtime.GOOS == "windows" {
			binaryName = "mkvdup.exe"
		}
		builtBinary = filepath.Join(tmpDir, binaryName)

		// Find the module root (parent of internal/dedup)
		_, thisFile, _, _ := runtime.Caller(0)
		moduleRoot := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))

		cmd := exec.Command("go", "build", "-o", builtBinary, "./cmd/mkvdup")
		cmd.Dir = moduleRoot
		output, err := cmd.CombinedOutput()
		if err != nil {
			builtBinaryErr = fmt.Errorf("build mkvdup: %w\n%s", err, output)
			return
		}
	})
	if builtBinaryErr != nil {
		t.Fatalf("Failed to build mkvdup: %v", builtBinaryErr)
	}
	return builtBinary
}

// runMkvdup runs the mkvdup command with the given arguments.
func runMkvdup(t testing.TB, args ...string) (string, error) {
	t.Helper()
	binary := getMkvdupBinary(t)
	cmd := exec.Command(binary, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// TestFullDedupCycle tests the complete dedup -> reconstruct -> verify cycle
// using the Big Buck Bunny test data by running the actual mkvdup executable.
func TestFullDedupCycle(t *testing.T) {
	paths := testdata.SkipIfNotAvailable(t)

	t.Logf("Using ISO: %s", paths.ISOFile)
	t.Logf("Using MKV: %s", paths.MKVFile)

	// Create temp directory for output
	tmpDir, err := os.MkdirTemp("", "mkvdup-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dedupPath := filepath.Join(tmpDir, "test.mkvdup")

	// Run mkvdup create
	t.Log("Running mkvdup create...")
	output, err := runMkvdup(t, "create", "--quiet", "--non-interactive",
		paths.MKVFile, paths.ISODir, dedupPath)
	if err != nil {
		t.Fatalf("mkvdup create failed: %v\nOutput:\n%s", err, output)
	}
	t.Log(output)

	// Verify the dedup file was created
	dedupInfo, err := os.Stat(dedupPath)
	if err != nil {
		t.Fatalf("Failed to stat dedup file: %v", err)
	}
	mkvInfo, err := os.Stat(paths.MKVFile)
	if err != nil {
		t.Fatalf("Failed to stat MKV file: %v", err)
	}
	t.Logf("Dedup file: %d bytes (%.1f%% of original)",
		dedupInfo.Size(), float64(dedupInfo.Size())/float64(mkvInfo.Size())*100)

	// The create command already does internal verification.
	// Additional verification: use internal reader to verify byte-by-byte
	t.Log("Verifying reconstruction via internal reader...")
	verifyReconstruction(t, dedupPath, paths.ISODir, paths.MKVFile)

	// Summary
	t.Log("")
	t.Log("=== DVD Summary ===")
	t.Logf("Original MKV: %.2f MB", float64(mkvInfo.Size())/(1024*1024))
	t.Logf("Dedup file:   %.2f MB", float64(dedupInfo.Size())/(1024*1024))
	t.Logf("Space saved:  %.1f%%", (1-float64(dedupInfo.Size())/float64(mkvInfo.Size()))*100)
}

// TestFullDedupCycle_Bluray tests the complete dedup -> reconstruct -> verify cycle
// using Blu-ray (M2TS) source data created by remuxing the Big Buck Bunny MKV
// via ffmpeg. This exercises the V4 range map code path.
func TestFullDedupCycle_Bluray(t *testing.T) {
	paths := testdata.SkipIfNotAvailable(t)

	t.Logf("Using MKV: %s", paths.MKVFile)

	// Create temp directory for output
	tmpDir, err := os.MkdirTemp("", "mkvdup-bluray-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create Blu-ray source data via ffmpeg remux
	t.Log("Creating Blu-ray source data (ffmpeg remux)...")
	blurayDir := paths.CreateBlurayData(t, tmpDir)
	t.Logf("  Blu-ray dir: %s", blurayDir)

	dedupPath := filepath.Join(tmpDir, "bluray-test.mkvdup")

	// Run mkvdup create
	t.Log("Running mkvdup create...")
	output, err := runMkvdup(t, "create", "--quiet", "--non-interactive",
		paths.MKVFile, blurayDir, dedupPath)
	if err != nil {
		t.Fatalf("mkvdup create failed: %v\nOutput:\n%s", err, output)
	}
	t.Log(output)

	// Verify the dedup file was created
	dedupInfo, err := os.Stat(dedupPath)
	if err != nil {
		t.Fatalf("Failed to stat dedup file: %v", err)
	}
	mkvInfo, err := os.Stat(paths.MKVFile)
	if err != nil {
		t.Fatalf("Failed to stat MKV file: %v", err)
	}
	t.Logf("Dedup file: %d bytes (%.1f%% of original)",
		dedupInfo.Size(), float64(dedupInfo.Size())/float64(mkvInfo.Size())*100)

	// The create command already does internal verification.
	// Additional verification: use internal reader to verify byte-by-byte
	// This also verifies that HasRangeMaps() returns true for V4
	t.Log("Verifying reconstruction via internal reader...")
	reader := verifyReconstruction(t, dedupPath, blurayDir, paths.MKVFile)

	// V4-specific checks
	if !reader.HasRangeMaps() {
		t.Error("Expected V4 dedup file to have range maps")
	}

	// Summary
	t.Log("")
	t.Log("=== Blu-ray Summary ===")
	t.Logf("Original MKV: %.2f MB", float64(mkvInfo.Size())/(1024*1024))
	t.Logf("Dedup file:   %.2f MB", float64(dedupInfo.Size())/(1024*1024))
	t.Logf("Space saved:  %.1f%%", (1-float64(dedupInfo.Size())/float64(mkvInfo.Size()))*100)
}

// verifyReconstruction verifies byte-by-byte that the dedup file can reconstruct the original MKV.
// This uses the internal dedup.Reader to ensure the reader code path works correctly.
func verifyReconstruction(t testing.TB, dedupPath, sourceDir, originalPath string) *dedup.Reader {
	t.Helper()

	reader, err := dedup.NewReader(dedupPath, sourceDir)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	// Don't close here - return to caller for additional checks

	if err := reader.LoadSourceFiles(); err != nil {
		reader.Close()
		t.Fatalf("Failed to load source files: %v", err)
	}

	origInfo, err := os.Stat(originalPath)
	if err != nil {
		reader.Close()
		t.Fatalf("Failed to stat original file: %v", err)
	}

	if reader.OriginalSize() != origInfo.Size() {
		t.Errorf("Size mismatch: reader reports %d, original is %d",
			reader.OriginalSize(), origInfo.Size())
	}

	origFile, err := os.Open(originalPath)
	if err != nil {
		reader.Close()
		t.Fatalf("Failed to open original file: %v", err)
	}
	defer origFile.Close()

	const chunkSize = 1024 * 1024 // 1MB chunks
	origBuf := make([]byte, chunkSize)
	reconBuf := make([]byte, chunkSize)
	offset := int64(0)
	mismatches := 0

	for {
		nOrig, errOrig := origFile.Read(origBuf)
		if nOrig > 0 {
			nRecon, errRecon := reader.ReadAt(reconBuf[:nOrig], offset)
			if errRecon != nil && errRecon != io.EOF {
				t.Fatalf("Read error at offset %d: %v", offset, errRecon)
			}
			if nRecon != nOrig {
				t.Fatalf("Short read at offset %d: got %d, want %d", offset, nRecon, nOrig)
			}
			if !bytes.Equal(origBuf[:nOrig], reconBuf[:nOrig]) {
				mismatches++
				if mismatches <= 5 {
					for i := 0; i < nOrig; i++ {
						if origBuf[i] != reconBuf[i] {
							t.Errorf("Mismatch at offset %d: orig=%02x, recon=%02x",
								offset+int64(i), origBuf[i], reconBuf[i])
							break
						}
					}
				}
			}
			offset += int64(nOrig)
		}
		if errOrig == io.EOF {
			break
		}
		if errOrig != nil {
			t.Fatalf("Read error from original: %v", errOrig)
		}
	}

	if mismatches > 0 {
		if mismatches > 5 {
			t.Logf("  ... and %d more mismatches not shown", mismatches-5)
		}
		t.Errorf("Verification failed: %d chunk mismatches", mismatches)
	} else {
		t.Log("  Verification passed: reconstructed file matches original")
	}

	return reader
}

// TestConcurrentReaders ensures multiple independent readers can access the same dedup file concurrently
// without errors. Each goroutine uses its own dedup.Reader instance; this test does not validate
// internal thread-safety of sharing a single Reader across goroutines.
func TestConcurrentReaders(t *testing.T) {
	paths := testdata.SkipIfNotAvailable(t)

	// Create dedup file using the executable
	tmpDir, err := os.MkdirTemp("", "mkvdup-concurrent-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dedupPath := filepath.Join(tmpDir, "test.mkvdup")

	output, err := runMkvdup(t, "create", "--quiet", "--non-interactive",
		paths.MKVFile, paths.ISODir, dedupPath)
	if err != nil {
		t.Fatalf("mkvdup create failed: %v\nOutput:\n%s", err, output)
	}

	// Now test concurrent readers
	const numReaders = 4
	const readsPerReader = 10

	origInfo, err := os.Stat(paths.MKVFile)
	if err != nil {
		t.Fatalf("Failed to stat original file: %v", err)
	}
	fileSize := origInfo.Size()

	origFile, err := os.Open(paths.MKVFile)
	if err != nil {
		t.Fatalf("Failed to open original file: %v", err)
	}
	defer origFile.Close()

	var wg sync.WaitGroup
	errors := make(chan error, numReaders)

	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()

			reader, err := dedup.NewReader(dedupPath, paths.ISODir)
			if err != nil {
				errors <- fmt.Errorf("reader %d: create: %w", readerID, err)
				return
			}
			defer reader.Close()

			if err := reader.LoadSourceFiles(); err != nil {
				errors <- fmt.Errorf("reader %d: load source: %w", readerID, err)
				return
			}

			buf := make([]byte, 64*1024)
			for j := 0; j < readsPerReader; j++ {
				offset := (int64(readerID*readsPerReader+j) * 1024 * 1024) % fileSize
				if offset+int64(len(buf)) > fileSize {
					offset = fileSize - int64(len(buf))
				}
				if offset < 0 {
					offset = 0
				}

				n, err := reader.ReadAt(buf, offset)
				if err != nil && err != io.EOF {
					errors <- fmt.Errorf("reader %d read %d: %w", readerID, j, err)
					return
				}
				if n == 0 && err == nil {
					errors <- fmt.Errorf("reader %d read %d: zero bytes with no error", readerID, j)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}
}
