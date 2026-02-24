//go:build integration

package fuse_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/cespare/xxhash/v2"
	fuselib "github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	fusepkg "github.com/stuckj/mkvdup/internal/fuse"
)

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

// TestFUSE_ReadOnlyOperations tests that write operations fail on the read-only filesystem.
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

	// Helper to check if error is permission-related (EACCES or EROFS)
	isPermissionOrReadOnlyError := func(err error) bool {
		return os.IsPermission(err) || errors.Is(err, syscall.EROFS)
	}

	// Test mkdir should fail (EACCES from kernel permission check or EROFS from FUSE)
	err = os.Mkdir(filepath.Join(moviesDir, "NewDir"), 0755)
	if err == nil {
		t.Error("Expected mkdir to fail")
	} else if !isPermissionOrReadOnlyError(err) {
		t.Errorf("Expected permission or read-only error, got: %v", err)
	}

	// Test creating a file should fail (EACCES from kernel permission check or EROFS from FUSE)
	_, err = os.Create(filepath.Join(moviesDir, "newfile.txt"))
	if err == nil {
		t.Error("Expected file creation to fail")
	} else if !isPermissionOrReadOnlyError(err) {
		t.Errorf("Expected permission or read-only error, got: %v", err)
	}

	// Test removing a file should fail (EACCES from kernel permission check or EROFS from FUSE)
	err = os.Remove(filepath.Join(moviesDir, "test.mkv"))
	if err == nil {
		t.Error("Expected file removal to fail")
	} else if !isPermissionOrReadOnlyError(err) {
		t.Errorf("Expected permission or read-only error, got: %v", err)
	}
}
