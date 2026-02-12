//go:build integration

package fuse_test

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	fusepkg "github.com/stuckj/mkvdup/internal/fuse"
)

// TestNFSPreadFallback_Integration tests that source files on NFS are read
// via the pread path instead of mmap. It sets up a local NFS loopback mount,
// verifies isNetworkFS detection, and exercises the full read path through
// the adapter layer.
func TestNFSPreadFallback_Integration(t *testing.T) {
	// Requires root for NFS setup
	if os.Geteuid() != 0 {
		t.Skip("NFS integration test requires root privileges")
	}

	// Check for nfs-kernel-server
	if _, err := exec.LookPath("exportfs"); err != nil {
		t.Skip("NFS integration test requires nfs-kernel-server (exportfs not found)")
	}

	// Check that rpcbind and nfs-server are running
	if err := exec.Command("systemctl", "is-active", "--quiet", "nfs-server").Run(); err != nil {
		// Try to start them
		if err := exec.Command("systemctl", "start", "rpcbind").Run(); err != nil {
			t.Skipf("Failed to start rpcbind: %v", err)
		}
		if err := exec.Command("systemctl", "start", "nfs-server").Run(); err != nil {
			t.Skipf("Failed to start nfs-server: %v", err)
		}
	}

	// Need test data
	dedupPath, _, testPaths := getSharedFixture(t)

	// Create temp dirs for NFS export and mount point
	exportDir := t.TempDir()
	mountDir := t.TempDir()

	// Copy source files to the export directory, preserving directory structure
	copySourceDir(t, testPaths.ISODir, exportDir)

	// Export the directory via NFS
	exportLine := exportDir + " localhost(ro,no_subtree_check,no_root_squash,insecure)"
	if err := os.MkdirAll("/etc/exports.d", 0755); err != nil {
		t.Fatalf("Failed to create exports.d directory: %v", err)
	}
	if err := os.WriteFile("/etc/exports.d/mkvdup-test.exports", []byte(exportLine+"\n"), 0644); err != nil {
		t.Fatalf("Failed to write exports file: %v", err)
	}
	t.Cleanup(func() {
		os.Remove("/etc/exports.d/mkvdup-test.exports")
		exec.Command("exportfs", "-ra").Run()
	})

	// Re-export
	if out, err := exec.Command("exportfs", "-ra").CombinedOutput(); err != nil {
		t.Fatalf("exportfs -ra failed: %v\n%s", err, out)
	}

	// Mount via NFS loopback
	if out, err := exec.Command("mount", "-t", "nfs", "localhost:"+exportDir, mountDir).CombinedOutput(); err != nil {
		t.Fatalf("NFS mount failed: %v\n%s", err, out)
	}
	t.Cleanup(func() {
		exec.Command("umount", "-f", mountDir).Run()
	})

	// Verify isNetworkFS detects the NFS mount
	if !fusepkg.IsNetworkFS(mountDir) {
		t.Fatal("Expected isNetworkFS to return true for NFS mount")
	}

	// Test the pread path through the adapter
	factory := &fusepkg.DefaultReaderFactory{ReadTimeout: 30 * time.Second}
	reader, err := factory.NewReaderLazy(dedupPath, mountDir)
	if err != nil {
		t.Fatalf("Failed to create reader with NFS source: %v", err)
	}
	defer reader.Close()

	// InitializeForReading should choose the pread path for NFS
	if err := reader.InitializeForReading(mountDir); err != nil {
		t.Fatalf("Failed to initialize reader (pread path): %v", err)
	}

	// Read first 4KB from the pread-backed reader
	buf := make([]byte, 4096)
	n, err := reader.ReadAt(buf, 0)
	if err != nil && err != io.EOF {
		t.Fatalf("ReadAt failed: %v", err)
	}

	// Compare against a reader using the local (mmap) path
	localFactory := &fusepkg.DefaultReaderFactory{}
	localReader, err := localFactory.NewReaderLazy(dedupPath, testPaths.ISODir)
	if err != nil {
		t.Fatalf("Failed to create local reader: %v", err)
	}
	defer localReader.Close()

	if err := localReader.InitializeForReading(testPaths.ISODir); err != nil {
		t.Fatalf("Failed to initialize local reader: %v", err)
	}

	localBuf := make([]byte, 4096)
	localN, err := localReader.ReadAt(localBuf, 0)
	if err != nil && err != io.EOF {
		t.Fatalf("Local ReadAt failed: %v", err)
	}

	if n != localN {
		t.Errorf("Read different amounts: NFS=%d, local=%d", n, localN)
	}

	if !bytes.Equal(buf[:n], localBuf[:localN]) {
		t.Error("Data mismatch between NFS (pread) and local (mmap) read paths")
		for i := 0; i < n && i < localN; i++ {
			if buf[i] != localBuf[i] {
				t.Errorf("First difference at offset %d: NFS=0x%02x, local=0x%02x", i, buf[i], localBuf[i])
				break
			}
		}
	}

	t.Logf("NFS pread path: successfully read %d bytes matching local mmap path", n)
}

// copySourceDir recursively copies source files from src to dst.
func copySourceDir(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()
		dstFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		defer dstFile.Close()
		_, err = io.Copy(dstFile, srcFile)
		return err
	})
	if err != nil {
		t.Fatalf("Failed to copy source dir: %v", err)
	}
}
