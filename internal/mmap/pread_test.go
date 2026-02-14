package mmap

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestOpenPread(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.dat")
	data := []byte("hello world, this is test data for pread")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	pf, err := OpenPread(path, 0)
	if err != nil {
		t.Fatalf("OpenPread: %v", err)
	}
	defer pf.Close()

	if pf.Size() != int64(len(data)) {
		t.Errorf("Size() = %d, want %d", pf.Size(), len(data))
	}
}

func TestOpenPread_NotFound(t *testing.T) {
	_, err := OpenPread("/nonexistent/file.dat", 0)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestPreadFile_ReadAt(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.dat")
	data := make([]byte, 1024)
	for i := range data {
		data[i] = byte(i % 256)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	pf, err := OpenPread(path, 0)
	if err != nil {
		t.Fatalf("OpenPread: %v", err)
	}
	defer pf.Close()

	tests := []struct {
		name    string
		off     int64
		size    int
		wantN   int
		wantErr error // nil means no error expected
	}{
		{"start", 0, 10, 10, nil},
		{"middle", 500, 20, 20, nil},
		{"end", 1020, 4, 4, nil},
		{"partial at end", 1020, 10, 4, io.EOF},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, tt.size)
			n, err := pf.ReadAt(buf, tt.off)
			if n != tt.wantN {
				t.Errorf("n = %d, want %d", n, tt.wantN)
			}
			if tt.wantErr != err {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
			// Verify data correctness
			for i := 0; i < n; i++ {
				expected := byte((int(tt.off) + i) % 256)
				if buf[i] != expected {
					t.Errorf("buf[%d] = %d, want %d", i, buf[i], expected)
					break
				}
			}
		})
	}
}

func TestPreadFile_ReadAt_EOF(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.dat")
	if err := os.WriteFile(path, []byte("hello"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	pf, err := OpenPread(path, 0)
	if err != nil {
		t.Fatalf("OpenPread: %v", err)
	}
	defer pf.Close()

	// Read beyond file
	buf := make([]byte, 10)
	n, err := pf.ReadAt(buf, 100)
	if n != 0 {
		t.Errorf("n = %d, want 0", n)
	}
	if err != io.EOF {
		t.Errorf("err = %v, want io.EOF", err)
	}
}

func TestPreadFile_Timeout(t *testing.T) {
	// We can't easily trigger a real timeout, but we can verify that the
	// timeout mechanism works for fast reads (no timeout should fire).
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.dat")
	data := []byte("quick read data")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	pf, err := OpenPread(path, 5*time.Second)
	if err != nil {
		t.Fatalf("OpenPread: %v", err)
	}
	defer pf.Close()

	buf := make([]byte, len(data))
	n, err := pf.ReadAt(buf, 0)
	if err != nil {
		t.Errorf("ReadAt with timeout: %v", err)
	}
	if n != len(data) {
		t.Errorf("n = %d, want %d", n, len(data))
	}
	if string(buf) != string(data) {
		t.Errorf("data = %q, want %q", buf, data)
	}
}

func TestPreadFile_ReadTimeoutError(t *testing.T) {
	err := &ReadTimeoutError{Path: "/mnt/nfs/test.dat", Timeout: 30 * time.Second}
	if err.Error() == "" {
		t.Error("expected non-empty error string")
	}
}

func TestPreadFile_ReadBackpressureError(t *testing.T) {
	err := &ReadBackpressureError{Path: "/mnt/nfs/test.dat"}
	msg := err.Error()
	if msg == "" {
		t.Error("expected non-empty error string")
	}
	if !strings.Contains(msg, "backpressure") {
		t.Errorf("error message should mention backpressure: %s", msg)
	}
}

func TestPreadFile_ReadAt_ZeroLength(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.dat")
	if err := os.WriteFile(path, []byte("data"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Test with timeout path (would spawn goroutine without the early return)
	pf, err := OpenPread(path, 5*time.Second)
	if err != nil {
		t.Fatalf("OpenPread: %v", err)
	}
	defer pf.Close()

	n, err := pf.ReadAt(nil, 0)
	if n != 0 || err != nil {
		t.Errorf("zero-length ReadAt: n=%d, err=%v; want 0, nil", n, err)
	}

	n, err = pf.ReadAt([]byte{}, 0)
	if n != 0 || err != nil {
		t.Errorf("empty-buf ReadAt: n=%d, err=%v; want 0, nil", n, err)
	}
}

func TestPreadFile_Reopen(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.dat")
	data := []byte("reopen test data content here")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	pf, err := OpenPread(path, 0)
	if err != nil {
		t.Fatalf("OpenPread: %v", err)
	}
	defer pf.Close()

	// Read before reopen
	buf := make([]byte, 5)
	n, err := pf.ReadAt(buf, 0)
	if err != nil || n != 5 {
		t.Fatalf("pre-reopen read: n=%d, err=%v", n, err)
	}

	// Force a reopen
	if err := pf.reopen(); err != nil {
		t.Fatalf("reopen: %v", err)
	}

	// Read after reopen should still work
	n, err = pf.ReadAt(buf, 0)
	if err != nil || n != 5 {
		t.Fatalf("post-reopen read: n=%d, err=%v", n, err)
	}
	if string(buf) != "reope" {
		t.Errorf("post-reopen data = %q, want %q", buf, "reope")
	}
}

func TestPreadFile_Reopen_SizeChanged(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.dat")
	data := []byte("original content")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	pf, err := OpenPread(path, 0)
	if err != nil {
		t.Fatalf("OpenPread: %v", err)
	}
	defer pf.Close()

	// Change file size
	if err := os.WriteFile(path, []byte("shorter"), 0644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}

	// Reopen should fail due to size change
	err = pf.reopen()
	if err == nil {
		t.Error("expected error for size change during reopen")
	}
}

func TestPreadFile_Close_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.dat")
	if err := os.WriteFile(path, []byte("data"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	pf, err := OpenPread(path, 0)
	if err != nil {
		t.Fatalf("OpenPread: %v", err)
	}

	if err := pf.Close(); err != nil {
		t.Errorf("first Close: %v", err)
	}
	if err := pf.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}

func TestIsRetryableError(t *testing.T) {
	// Retryable errnos (bare)
	for _, errno := range []unix.Errno{unix.ESTALE, unix.ETIMEDOUT, unix.ECONNRESET, unix.EIO} {
		if !isRetryableError(errno) {
			t.Errorf("%v should be retryable", errno)
		}
	}

	// Retryable errnos wrapped in *os.PathError (as os.File.ReadAt returns)
	for _, errno := range []unix.Errno{unix.ESTALE, unix.ETIMEDOUT, unix.ECONNRESET, unix.EIO} {
		wrapped := &os.PathError{Op: "read", Path: "/test", Err: errno}
		if !isRetryableError(wrapped) {
			t.Errorf("*os.PathError{%v} should be retryable", errno)
		}
	}

	// Non-retryable errors
	if isRetryableError(io.EOF) {
		t.Error("io.EOF should not be retryable")
	}
	if isRetryableError(os.ErrNotExist) {
		t.Error("ErrNotExist should not be retryable")
	}
	if isRetryableError(unix.ENOENT) {
		t.Error("ENOENT should not be retryable")
	}
	if isRetryableError(&os.PathError{Op: "read", Path: "/test", Err: unix.ENOENT}) {
		t.Error("*os.PathError{ENOENT} should not be retryable")
	}
}

// Verify PreadFile satisfies the SourceFile interface.
var _ SourceFile = (*PreadFile)(nil)

// Verify File satisfies the SourceFile and MmapData interfaces.
var _ SourceFile = (*File)(nil)
var _ MmapData = (*File)(nil)
