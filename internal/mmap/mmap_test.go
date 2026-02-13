package mmap

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestOpen(t *testing.T) {
	// Create a test file with known content
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.bin")

	content := []byte("Hello, World! This is test data for mmap.")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Open with mmap
	f, err := Open(testFile)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer f.Close()

	// Verify size
	if f.Size() != int64(len(content)) {
		t.Errorf("Size mismatch: got %d, want %d", f.Size(), len(content))
	}

	// Verify Len
	if f.Len() != len(content) {
		t.Errorf("Len mismatch: got %d, want %d", f.Len(), len(content))
	}

	// Verify content via Data
	data := f.Data()
	if string(data) != string(content) {
		t.Errorf("Data mismatch: got %q, want %q", string(data), string(content))
	}
}

func TestOpen_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "empty.bin")

	// Create empty file
	if err := os.WriteFile(testFile, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Open with mmap
	f, err := Open(testFile)
	if err != nil {
		t.Fatalf("Open failed for empty file: %v", err)
	}
	defer f.Close()

	// Verify size is 0
	if f.Size() != 0 {
		t.Errorf("Size should be 0 for empty file, got %d", f.Size())
	}

	// Verify Len is 0
	if f.Len() != 0 {
		t.Errorf("Len should be 0 for empty file, got %d", f.Len())
	}

	// Verify Data is nil for empty file
	if f.Data() != nil {
		t.Errorf("Data should be nil for empty file")
	}
}

func TestOpen_NotExists(t *testing.T) {
	_, err := Open("/nonexistent/path/file.bin")
	if err == nil {
		t.Error("Open should fail for nonexistent file")
	}
}

func TestSlice(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.bin")

	content := []byte("0123456789ABCDEF")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	f, err := Open(testFile)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer f.Close()

	tests := []struct {
		name     string
		offset   int64
		size     int
		expected string
	}{
		{
			name:     "beginning",
			offset:   0,
			size:     5,
			expected: "01234",
		},
		{
			name:     "middle",
			offset:   5,
			size:     5,
			expected: "56789",
		},
		{
			name:     "end",
			offset:   10,
			size:     6,
			expected: "ABCDEF",
		},
		{
			name:     "partial at end",
			offset:   14,
			size:     10, // Request more than available
			expected: "EF",
		},
		{
			name:     "single byte",
			offset:   0,
			size:     1,
			expected: "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := f.Slice(tt.offset, tt.size)
			if string(result) != tt.expected {
				t.Errorf("Slice(%d, %d) = %q, want %q", tt.offset, tt.size, string(result), tt.expected)
			}
		})
	}
}

func TestSlice_OutOfBounds(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.bin")

	content := []byte("0123456789")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	f, err := Open(testFile)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer f.Close()

	// Offset at exactly file size should return nil
	result := f.Slice(int64(len(content)), 1)
	if result != nil {
		t.Errorf("Slice at file size should return nil, got %v", result)
	}

	// Offset beyond file size should return nil
	result = f.Slice(int64(len(content)+10), 1)
	if result != nil {
		t.Errorf("Slice beyond file size should return nil, got %v", result)
	}
}

func TestSlice_NegativeOffset(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.bin")

	content := []byte("0123456789")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	f, err := Open(testFile)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer f.Close()

	// Negative offset should return nil
	result := f.Slice(-1, 5)
	if result != nil {
		t.Errorf("Slice with negative offset should return nil, got %v", result)
	}
}

func TestAdvise(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.bin")

	// Create a file with some content
	content := make([]byte, 4096) // Page-sized
	for i := range content {
		content[i] = byte(i % 256)
	}
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	f, err := Open(testFile)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer f.Close()

	// Test MADV_SEQUENTIAL hint
	if err := f.Advise(unix.MADV_SEQUENTIAL); err != nil {
		t.Errorf("Advise(MADV_SEQUENTIAL) failed: %v", err)
	}

	// Test MADV_RANDOM hint
	if err := f.Advise(unix.MADV_RANDOM); err != nil {
		t.Errorf("Advise(MADV_RANDOM) failed: %v", err)
	}

	// Test MADV_NORMAL hint
	if err := f.Advise(unix.MADV_NORMAL); err != nil {
		t.Errorf("Advise(MADV_NORMAL) failed: %v", err)
	}
}

func TestAdvise_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "empty.bin")

	// Create empty file
	if err := os.WriteFile(testFile, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	f, err := Open(testFile)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer f.Close()

	// Advise on empty file should be a no-op and not error
	if err := f.Advise(unix.MADV_SEQUENTIAL); err != nil {
		t.Errorf("Advise on empty file should succeed, got: %v", err)
	}
}

func TestReadAt(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.bin")

	content := []byte("0123456789ABCDEF")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	f, err := Open(testFile)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer f.Close()

	tests := []struct {
		name    string
		bufSize int
		off     int64
		wantN   int
		wantErr error
		wantStr string
	}{
		{
			name:    "zero-length read",
			bufSize: 0,
			off:     0,
			wantN:   0,
			wantErr: nil,
		},
		{
			name:    "negative offset",
			bufSize: 4,
			off:     -1,
			wantN:   0,
			wantErr: os.ErrInvalid,
		},
		{
			name:    "offset at size (EOF)",
			bufSize: 4,
			off:     int64(len(content)),
			wantN:   0,
			wantErr: io.EOF,
		},
		{
			name:    "offset beyond size (EOF)",
			bufSize: 4,
			off:     int64(len(content) + 100),
			wantN:   0,
			wantErr: io.EOF,
		},
		{
			name:    "full read from start",
			bufSize: 5,
			off:     0,
			wantN:   5,
			wantErr: nil,
			wantStr: "01234",
		},
		{
			name:    "full read from middle",
			bufSize: 4,
			off:     10,
			wantN:   4,
			wantErr: nil,
			wantStr: "ABCD",
		},
		{
			name:    "partial read near end",
			bufSize: 10,
			off:     14,
			wantN:   2,
			wantErr: io.EOF,
			wantStr: "EF",
		},
		{
			name:    "single byte at last position",
			bufSize: 1,
			off:     15,
			wantN:   1,
			wantErr: nil,
			wantStr: "F",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, tt.bufSize)
			n, err := f.ReadAt(buf, tt.off)
			if n != tt.wantN {
				t.Errorf("ReadAt(%d, %d) n = %d, want %d", tt.bufSize, tt.off, n, tt.wantN)
			}
			if err != tt.wantErr {
				t.Errorf("ReadAt(%d, %d) err = %v, want %v", tt.bufSize, tt.off, err, tt.wantErr)
			}
			if tt.wantStr != "" && string(buf[:n]) != tt.wantStr {
				t.Errorf("ReadAt(%d, %d) data = %q, want %q", tt.bufSize, tt.off, string(buf[:n]), tt.wantStr)
			}
		})
	}
}

func TestClose(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.bin")

	content := []byte("test content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	f, err := Open(testFile)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// Verify data is accessible before close
	if f.Data() == nil {
		t.Error("Data should not be nil before close")
	}

	// Close the file
	if err := f.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// After close, data should be nil
	if f.Data() != nil {
		t.Error("Data should be nil after close")
	}
}

func TestClose_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.bin")

	content := []byte("test content")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	f, err := Open(testFile)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// First close
	if err := f.Close(); err != nil {
		t.Errorf("First Close failed: %v", err)
	}

	// Second close should be safe (idempotent)
	if err := f.Close(); err != nil {
		t.Errorf("Second Close should be safe, got: %v", err)
	}
}

func TestClose_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "empty.bin")

	// Create empty file
	if err := os.WriteFile(testFile, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	f, err := Open(testFile)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// Close empty file should work
	if err := f.Close(); err != nil {
		t.Errorf("Close empty file failed: %v", err)
	}
}
