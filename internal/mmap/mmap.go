// Package mmap provides zero-copy memory-mapped file access.
package mmap

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// SourceFile provides read access to a source file, either via mmap or pread.
type SourceFile interface {
	io.ReaderAt
	Size() int64
	Close() error
}

// MmapData provides zero-copy access to a memory-mapped file's data.
// Types implementing this interface allow callers to use direct slice access
// instead of copying through ReadAt.
type MmapData interface {
	Data() []byte
}

// File provides zero-copy access to a memory-mapped file.
// Unlike golang.org/x/exp/mmap, this exposes the raw []byte slice
// allowing direct access without copying data.
type File struct {
	data []byte
	size int64
}

// Open opens a file and memory-maps it for reading.
// The returned File provides zero-copy access to the file contents.
func Open(path string) (*File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}

	size := info.Size()
	if size == 0 {
		return &File{data: nil, size: 0}, nil
	}

	data, err := unix.Mmap(int(f.Fd()), 0, int(size), unix.PROT_READ, unix.MAP_SHARED)
	if err != nil {
		return nil, fmt.Errorf("mmap: %w", err)
	}

	return &File{data: data, size: size}, nil
}

// Data returns the raw byte slice for direct zero-copy access.
// The slice is valid until Close() is called.
func (m *File) Data() []byte {
	return m.data
}

// Size returns the size of the mapped file in bytes.
func (m *File) Size() int64 {
	return m.size
}

// Len returns the size of the mapped file as int (for compatibility).
func (m *File) Len() int {
	return int(m.size)
}

// Slice returns a sub-slice of the mapped data without copying.
// Returns nil if the range is out of bounds.
func (m *File) Slice(offset int64, size int) []byte {
	if offset < 0 || offset >= m.size {
		return nil
	}
	end := offset + int64(size)
	if end > m.size {
		end = m.size
	}
	return m.data[offset:end]
}

// Advise provides hints to the kernel about expected access patterns.
// Use MADV_DONTNEED to release pages (they'll be re-faulted when accessed).
// Use MADV_SEQUENTIAL to hint sequential access pattern.
func (m *File) Advise(advice int) error {
	if len(m.data) == 0 {
		return nil
	}
	return unix.Madvise(m.data, advice)
}

// ReadAt implements io.ReaderAt by copying from the mmap'd data.
func (m *File) ReadAt(p []byte, off int64) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if off < 0 || off >= m.size {
		return 0, io.EOF
	}
	n := copy(p, m.data[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

// Close unmaps the file from memory.
func (m *File) Close() error {
	if m.data == nil {
		return nil
	}

	if err := unix.Munmap(m.data); err != nil {
		return err
	}

	m.data = nil
	return nil
}
