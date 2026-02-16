//go:build integration

package fuse_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	fuselib "github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/stuckj/mkvdup/internal/dedup"
	fusepkg "github.com/stuckj/mkvdup/internal/fuse"
	"github.com/stuckj/mkvdup/testdata"
)

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
