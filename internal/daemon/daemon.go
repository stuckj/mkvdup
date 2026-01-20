// Package daemon provides daemonization support for mkvdup FUSE mount.
//
// It uses a re-exec pattern where the parent process spawns a child with
// an environment variable marker. The child signals readiness to the parent
// via a pipe, allowing the parent to return success/failure appropriately.
package daemon

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

// childEnvVar is the environment variable that marks a child daemon process.
const childEnvVar = "MKVDUP_DAEMON_CHILD"

// readyPipeFdEnvVar is the environment variable containing the pipe fd for signaling.
const readyPipeFdEnvVar = "MKVDUP_READY_PIPE_FD"

// Status codes sent from child to parent via the ready pipe.
const (
	statusReady byte = 0 // Mount successful
	statusError byte = 1 // Mount failed
)

// IsChild returns true if the current process is a daemon child.
func IsChild() bool {
	return os.Getenv(childEnvVar) == "1"
}

// Daemonize spawns the current executable as a background daemon.
// It waits for the child to signal readiness or error via a pipe.
// Returns nil on success (child signaled ready) or error on failure.
// The timeout specifies how long to wait for the child to signal.
func Daemonize(pidFile string, timeout time.Duration) error {
	// Create pipe for child to signal readiness
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("create pipe: %w", err)
	}
	defer readPipe.Close()

	// Build command with same arguments
	cmd := exec.Command(os.Args[0], os.Args[1:]...)

	// Set up environment
	cmd.Env = append(os.Environ(),
		childEnvVar+"=1",
		readyPipeFdEnvVar+"=3", // fd 3 is after stdin/stdout/stderr
	)

	// Pass write end of pipe to child as fd 3
	cmd.ExtraFiles = []*os.File{writePipe}

	// Detach from terminal
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // Create new session
	}

	// Start child process
	if err := cmd.Start(); err != nil {
		writePipe.Close()
		return fmt.Errorf("start daemon: %w", err)
	}

	// Close write end in parent (child has it)
	writePipe.Close()

	// Wait for child to signal with timeout
	resultChan := make(chan error, 1)
	go func() {
		status := make([]byte, 1)
		n, err := readPipe.Read(status)
		if err != nil {
			if errors.Is(err, io.EOF) {
				resultChan <- fmt.Errorf("daemon child exited unexpectedly")
			} else {
				resultChan <- fmt.Errorf("read from child: %w", err)
			}
			return
		}
		if n != 1 {
			resultChan <- fmt.Errorf("unexpected read size from child: %d", n)
			return
		}

		if status[0] == statusReady {
			resultChan <- nil
		} else {
			// Read error message
			errMsg := make([]byte, 1024)
			n, _ := readPipe.Read(errMsg)
			if n > 0 {
				resultChan <- fmt.Errorf("daemon failed: %s", string(errMsg[:n]))
			} else {
				resultChan <- fmt.Errorf("daemon failed with unknown error")
			}
		}
	}()

	select {
	case err := <-resultChan:
		if err != nil {
			// Try to clean up the child
			cmd.Process.Kill()
			return err
		}
		// Success - child is running and mount is ready
		if pidFile != "" {
			// Write PID file from parent since child may not have permission
			if err := WritePidFile(pidFile, cmd.Process.Pid); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to write pid file: %v\n", err)
			}
		}
		return nil
	case <-time.After(timeout):
		cmd.Process.Kill()
		return fmt.Errorf("daemon startup timed out after %v", timeout)
	}
}

// NotifyReady signals to the parent that the mount is ready.
// This should be called by the child after the FUSE mount is ready.
func NotifyReady() error {
	fd, err := getReadyPipeFd()
	if err != nil {
		return err
	}

	pipe := os.NewFile(fd, "ready-pipe")
	if pipe == nil {
		return fmt.Errorf("invalid pipe fd")
	}
	defer pipe.Close()

	_, err = pipe.Write([]byte{statusReady})
	return err
}

// NotifyError signals to the parent that the mount failed.
// This should be called by the child if an error occurs during startup.
func NotifyError(mountErr error) error {
	fd, err := getReadyPipeFd()
	if err != nil {
		return err
	}

	pipe := os.NewFile(fd, "ready-pipe")
	if pipe == nil {
		return fmt.Errorf("invalid pipe fd")
	}
	defer pipe.Close()

	// Write error status followed by error message
	_, err = pipe.Write([]byte{statusError})
	if err != nil {
		return err
	}
	_, err = pipe.Write([]byte(mountErr.Error()))
	return err
}

// getReadyPipeFd returns the file descriptor for the ready pipe.
func getReadyPipeFd() (uintptr, error) {
	fdStr := os.Getenv(readyPipeFdEnvVar)
	if fdStr == "" {
		return 0, fmt.Errorf("not running as daemon child")
	}
	fd, err := strconv.ParseUint(fdStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid pipe fd: %w", err)
	}
	return uintptr(fd), nil
}

// Detach closes stdin, stdout, and stderr to fully detach from the terminal.
// This should be called by the child after signaling ready.
func Detach() {
	// Redirect standard file descriptors to /dev/null
	devNull, err := os.OpenFile("/dev/null", os.O_RDWR, 0)
	if err != nil {
		return
	}

	// Replace stdin, stdout, stderr with /dev/null
	syscall.Dup2(int(devNull.Fd()), int(os.Stdin.Fd()))
	syscall.Dup2(int(devNull.Fd()), int(os.Stdout.Fd()))
	syscall.Dup2(int(devNull.Fd()), int(os.Stderr.Fd()))
	devNull.Close()
}

// WritePidFile writes the given PID to a file.
func WritePidFile(path string, pid int) error {
	return os.WriteFile(path, []byte(strconv.Itoa(pid)+"\n"), 0644)
}

// RemovePidFile removes the PID file at the given path.
func RemovePidFile(path string) error {
	return os.Remove(path)
}
