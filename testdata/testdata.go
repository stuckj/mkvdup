// Package testdata provides helpers for locating integration test data.
//
// Test data (Big Buck Bunny DVD ISO and MKV) is not stored in the repository.
// See README.md in this directory for setup instructions.
package testdata

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// Paths contains the resolved paths to test data files.
type Paths struct {
	Root      string // Base test data directory
	ISODir    string // Directory containing ISO file
	ISOFile   string // Path to the ISO file
	MKVDir    string // Directory containing MKV file(s)
	MKVFile   string // Path to the main MKV file
	Available bool   // True if all required files exist
}

// DefaultISOName is the expected ISO filename.
const DefaultISOName = "bbb-pal.iso"

// DefaultMKVPattern is the glob pattern for finding MKV files.
const DefaultMKVPattern = "*.mkv"

// Find locates the test data directory and checks for required files.
// It checks these locations in order:
//  1. $MKVDUP_TESTDATA environment variable
//  2. testdata/generated/ (relative to the testdata package, created by generate-test-data.sh)
//  3. ~/.cache/mkvdup/testdata/
//  4. /tmp/mkvdup-testdata/
//
// Returns Paths with Available=false if test data is not found.
func Find() Paths {
	var p Paths

	// Check environment variable first
	if envPath := os.Getenv("MKVDUP_TESTDATA"); envPath != "" {
		p.Root = envPath
		if checkPaths(&p) {
			return p
		}
	}

	// Check testdata/generated/ (local to the repo, created by generate-test-data.sh)
	// This is the preferred location for reproducible test data
	if localPath := findLocalTestdataDir(); localPath != "" {
		p.Root = localPath
		if checkPaths(&p) {
			return p
		}
	}

	// Check ~/.cache/mkvdup/testdata/
	if home, err := os.UserHomeDir(); err == nil {
		p.Root = filepath.Join(home, ".cache", "mkvdup", "testdata")
		if checkPaths(&p) {
			return p
		}
	}

	// Check /tmp/mkvdup-testdata/
	p.Root = "/tmp/mkvdup-testdata"
	if checkPaths(&p) {
		return p
	}

	// Not found - clear all paths
	p.Root = ""
	p.ISODir = ""
	p.ISOFile = ""
	p.MKVDir = ""
	p.MKVFile = ""
	p.Available = false
	return p
}

// checkPaths fills in the paths and returns true if all required files exist.
func checkPaths(p *Paths) bool {
	p.ISODir = filepath.Join(p.Root, "bigbuckbunny")
	p.MKVDir = filepath.Join(p.Root, "bigbuckbunny-mkv")

	// Check ISO file
	p.ISOFile = filepath.Join(p.ISODir, DefaultISOName)
	if _, err := os.Stat(p.ISOFile); err != nil {
		// Try NTSC variant
		p.ISOFile = filepath.Join(p.ISODir, "bbb-ntsc.iso")
		if _, err := os.Stat(p.ISOFile); err != nil {
			p.Available = false
			return false
		}
	}

	// Find MKV file (first match)
	matches, err := filepath.Glob(filepath.Join(p.MKVDir, DefaultMKVPattern))
	if err != nil || len(matches) == 0 {
		p.Available = false
		return false
	}
	p.MKVFile = matches[0]

	p.Available = true
	return true
}

// SkipIfNotAvailable calls t.Skip if test data is not available.
// Use this at the start of integration tests.
func SkipIfNotAvailable(t interface{ Skip(...interface{}) }) Paths {
	p := Find()
	if !p.Available {
		t.Skip("Test data not available. See testdata/README.md for setup instructions.")
	}
	return p
}

// CreateBlurayData creates a Blu-ray directory structure by remuxing the MKV
// file to M2TS format using ffmpeg (copy codec, no re-encoding). The directory
// is created under tmpDir and has the layout BDMV/STREAM/00001.m2ts that
// DetectType recognises as TypeBluray.
//
// The test is skipped if ffmpeg is not available.
func (p Paths) CreateBlurayData(t testing.TB, tmpDir string) string {
	t.Helper()

	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available, skipping Blu-ray test")
	}

	blurayRoot := filepath.Join(tmpDir, "bluray")
	streamDir := filepath.Join(blurayRoot, "BDMV", "STREAM")
	if err := os.MkdirAll(streamDir, 0755); err != nil {
		t.Fatalf("CreateBlurayData: mkdir: %v", err)
	}

	m2tsPath := filepath.Join(streamDir, "00001.m2ts")
	cmd := exec.Command("ffmpeg",
		"-loglevel", "error",
		"-i", p.MKVFile,
		"-c", "copy",
		"-f", "mpegts",
		"-y", // overwrite if exists
		m2tsPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("CreateBlurayData: ffmpeg remux failed: %v\n%s", err, output)
	}

	return blurayRoot
}

// findLocalTestdataDir returns the path to testdata/generated/ directory
// relative to this source file, or empty string if it cannot be determined.
func findLocalTestdataDir() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	// filename is the path to this file (testdata.go)
	// We want the "generated" subdirectory in the same directory
	dir := filepath.Dir(filename)
	return filepath.Join(dir, "generated")
}
