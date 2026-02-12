package mmap

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

// ReadTimeoutError is returned when a pread operation exceeds the configured timeout.
type ReadTimeoutError struct {
	Path    string
	Timeout time.Duration
}

func (e *ReadTimeoutError) Error() string {
	return fmt.Sprintf("pread timeout after %s: %s", e.Timeout, e.Path)
}

// ReadBackpressureError is returned when all inflight read slots are occupied,
// indicating the network FS is likely stalled. This is distinct from
// ReadTimeoutError, which indicates a single read exceeded its deadline.
type ReadBackpressureError struct {
	Path string
}

func (e *ReadBackpressureError) Error() string {
	return fmt.Sprintf("pread backpressure: all %d inflight slots occupied: %s", maxInflight, e.Path)
}

// maxInflight is the maximum number of concurrent in-flight read goroutines
// per PreadFile. This bounds memory/goroutine accumulation when an NFS mount
// is stalled and reads are timing out repeatedly.
const maxInflight = 16

// PreadFile provides pread(2)-based read access to a source file, with retry
// and stale handle recovery. This is used for source files on network
// filesystems (NFS, CIFS/SMB) where mmap is unsafe due to SIGBUS on
// page fault failures.
type PreadFile struct {
	mu       sync.RWMutex // protects file; RLock for reads, Lock for reopen/close
	file     *os.File
	path     string
	size     int64
	timeout  time.Duration // 0 = no timeout
	inflight chan struct{} // semaphore bounding concurrent timeout goroutines
}

// OpenPread opens a file for pread-based access.
func OpenPread(path string, timeout time.Duration) (*PreadFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}

	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("stat file: %w", err)
	}

	return &PreadFile{
		file:     f,
		path:     path,
		size:     info.Size(),
		timeout:  timeout,
		inflight: make(chan struct{}, maxInflight),
	}, nil
}

// Size returns the size of the file.
func (p *PreadFile) Size() int64 {
	return p.size
}

// ReadAt reads len(buf) bytes from the file starting at byte offset off.
// If timeout is configured and the read takes too long, it returns a
// ReadTimeoutError. The underlying goroutine may continue until the kernel
// completes the I/O, but the caller is unblocked. The goroutine reads into
// a private buffer to prevent it from writing to buf after the caller has
// moved on. A per-file semaphore bounds the number of in-flight goroutines
// to prevent unbounded accumulation under a stalled NFS mount.
func (p *PreadFile) ReadAt(buf []byte, off int64) (int, error) {
	if len(buf) == 0 {
		return 0, nil
	}

	if p.timeout <= 0 {
		return p.readAtWithRetry(buf, off)
	}

	// Acquire an inflight slot (non-blocking). If all slots are occupied
	// the NFS mount is likely stalled — fail fast instead of spawning
	// more goroutines.
	select {
	case p.inflight <- struct{}{}:
	default:
		return 0, &ReadBackpressureError{Path: p.path}
	}

	type result struct {
		n   int
		err error
	}
	// Read into a private buffer so an abandoned goroutine (after timeout)
	// cannot write into buf while it is being reused by the caller.
	tmp := make([]byte, len(buf))
	ch := make(chan result, 1)
	go func() {
		defer func() { <-p.inflight }()
		n, err := p.readAtWithRetry(tmp, off)
		ch <- result{n, err}
	}()

	timer := time.NewTimer(p.timeout)
	defer timer.Stop()

	select {
	case r := <-ch:
		copy(buf[:r.n], tmp[:r.n])
		return r.n, r.err
	case <-timer.C:
		return 0, &ReadTimeoutError{Path: p.path, Timeout: p.timeout}
	}
}

// readAtWithRetry performs a pread with one retry on retryable errors,
// reopening the file descriptor if needed. The read lock is held across
// each ReadAt call to prevent reopen/Close from closing the fd mid-read.
func (p *PreadFile) readAtWithRetry(buf []byte, off int64) (int, error) {
	n, err := p.lockedReadAt(buf, off)
	if err != nil && err != io.EOF && isRetryableError(err) {
		if reopenErr := p.reopen(); reopenErr != nil {
			return n, fmt.Errorf("pread retry failed (reopen: %w, original: %w)", reopenErr, err)
		}
		n, err = p.lockedReadAt(buf, off)
	}
	return n, err
}

// lockedReadAt performs a single ReadAt under the read lock.
func (p *PreadFile) lockedReadAt(buf []byte, off int64) (int, error) {
	p.mu.RLock()
	f := p.file
	if f == nil {
		p.mu.RUnlock()
		return 0, os.ErrClosed
	}
	n, err := f.ReadAt(buf, off)
	p.mu.RUnlock()
	return n, err
}

// reopen closes the current fd and opens a new one.
// The file size is verified to be unchanged.
func (p *PreadFile) reopen() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.file == nil {
		return os.ErrClosed
	}

	oldFile := p.file

	newFile, err := os.Open(p.path)
	if err != nil {
		return fmt.Errorf("reopen: %w", err)
	}

	info, err := newFile.Stat()
	if err != nil {
		newFile.Close()
		return fmt.Errorf("reopen stat: %w", err)
	}

	if info.Size() != p.size {
		newFile.Close()
		return fmt.Errorf("reopen: size changed (%d → %d)", p.size, info.Size())
	}

	p.file = newFile
	oldFile.Close()
	return nil
}

// Close closes the underlying file.
func (p *PreadFile) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.file != nil {
		err := p.file.Close()
		p.file = nil
		return err
	}
	return nil
}

// isRetryableError checks if an error is a transient network FS error
// that may succeed on retry (possibly after reopening the fd).
func isRetryableError(err error) bool {
	var errno unix.Errno
	if errors.As(err, &errno) {
		switch errno {
		case unix.ESTALE, unix.ETIMEDOUT, unix.ECONNRESET, unix.EIO:
			return true
		}
	}
	return false
}
