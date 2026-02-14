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
	mu         sync.Mutex // protects file and staleFiles
	file       *os.File
	path       string
	size       int64
	timeout    time.Duration // 0 = no timeout
	inflight   chan struct{} // semaphore bounding concurrent timeout goroutines
	staleFiles []*os.File    // old fds kept open until Close to avoid EBADF on in-flight reads
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
// reopening the file descriptor if needed. The mutex is only held briefly
// to copy the fd pointer — not during the pread syscall — so Close() and
// reopen() are never blocked by a stalled network read. Old fds from
// reopen are kept in staleFiles (not closed) to avoid EBADF on
// concurrent in-flight reads; they are cleaned up on Close().
func (p *PreadFile) readAtWithRetry(buf []byte, off int64) (int, error) {
	p.mu.Lock()
	f := p.file
	p.mu.Unlock()

	if f == nil {
		return 0, os.ErrClosed
	}

	n, err := f.ReadAt(buf, off)
	if err != nil && err != io.EOF && isRetryableError(err) {
		if reopenErr := p.reopen(); reopenErr != nil {
			return n, fmt.Errorf("pread retry failed (reopen: %w, original: %w)", reopenErr, err)
		}

		p.mu.Lock()
		f = p.file
		p.mu.Unlock()

		if f == nil {
			return 0, os.ErrClosed
		}
		n, err = f.ReadAt(buf, off)
	}
	return n, err
}

// reopen opens a new fd and swaps it in. The old fd is not closed
// immediately because in-flight goroutines may still hold a reference
// to it (copied under the mutex before the pread syscall). Old fds are
// collected in staleFiles and cleaned up on Close().
//
// Fd accumulation is bounded in practice: reopens only occur on transient
// network errors (ESTALE, ETIMEDOUT, etc.), which are rare. Even under
// a flaky mount, each reopen adds just one fd, well within default ulimits.
func (p *PreadFile) reopen() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.file == nil {
		return os.ErrClosed
	}

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

	p.staleFiles = append(p.staleFiles, p.file)
	p.file = newFile
	return nil
}

// Close closes the current file and any stale fds from previous reopens.
func (p *PreadFile) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var firstErr error
	if p.file != nil {
		firstErr = p.file.Close()
		p.file = nil
	}
	for _, f := range p.staleFiles {
		if err := f.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	p.staleFiles = nil
	return firstErr
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
