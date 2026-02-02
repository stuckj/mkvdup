package daemon

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestIsChild(t *testing.T) {
	// Save original value
	orig := os.Getenv(childEnvVar)
	defer os.Setenv(childEnvVar, orig)

	// Test not set
	os.Unsetenv(childEnvVar)
	if IsChild() {
		t.Error("IsChild() should return false when env var not set")
	}

	// Test set to empty
	os.Setenv(childEnvVar, "")
	if IsChild() {
		t.Error("IsChild() should return false when env var is empty")
	}

	// Test set to wrong value
	os.Setenv(childEnvVar, "0")
	if IsChild() {
		t.Error("IsChild() should return false when env var is '0'")
	}

	// Test set correctly
	os.Setenv(childEnvVar, "1")
	if !IsChild() {
		t.Error("IsChild() should return true when env var is '1'")
	}
}

func TestWriteAndRemovePidFile(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")

	// Test write
	pid := 12345
	if err := WritePidFile(pidFile, pid); err != nil {
		t.Fatalf("WritePidFile failed: %v", err)
	}

	// Verify content
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	expected := strconv.Itoa(pid) + "\n"
	if string(data) != expected {
		t.Errorf("PID file content = %q, want %q", string(data), expected)
	}

	// Verify permissions
	info, err := os.Stat(pidFile)
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0644 {
		t.Errorf("PID file permissions = %o, want 0644", perm)
	}

	// Test remove
	if err := RemovePidFile(pidFile); err != nil {
		t.Fatalf("RemovePidFile failed: %v", err)
	}
	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Error("PID file should not exist after removal")
	}
}

func TestRemovePidFileNotExists(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "nonexistent.pid")

	err := RemovePidFile(pidFile)
	if err == nil {
		t.Error("RemovePidFile should fail for non-existent file")
	}
	if !os.IsNotExist(err) {
		t.Errorf("RemovePidFile error should be IsNotExist, got: %v", err)
	}
}

func TestGetReadyPipeFdNotSet(t *testing.T) {
	// Save original value
	orig := os.Getenv(readyPipeFdEnvVar)
	defer os.Setenv(readyPipeFdEnvVar, orig)

	os.Unsetenv(readyPipeFdEnvVar)
	_, err := getReadyPipeFd()
	if err == nil {
		t.Error("getReadyPipeFd should fail when env var not set")
	}
}

func TestGetReadyPipeFdInvalid(t *testing.T) {
	// Save original value
	orig := os.Getenv(readyPipeFdEnvVar)
	defer os.Setenv(readyPipeFdEnvVar, orig)

	os.Setenv(readyPipeFdEnvVar, "not-a-number")
	_, err := getReadyPipeFd()
	if err == nil {
		t.Error("getReadyPipeFd should fail with invalid fd")
	}
}

func TestGetReadyPipeFdValid(t *testing.T) {
	// Save original value
	orig := os.Getenv(readyPipeFdEnvVar)
	defer os.Setenv(readyPipeFdEnvVar, orig)

	os.Setenv(readyPipeFdEnvVar, "3")
	fd, err := getReadyPipeFd()
	if err != nil {
		t.Errorf("getReadyPipeFd failed: %v", err)
	}
	if fd != 3 {
		t.Errorf("getReadyPipeFd = %d, want 3", fd)
	}
}

func TestNotifyReadyNotChild(t *testing.T) {
	// Save original value
	orig := os.Getenv(readyPipeFdEnvVar)
	defer os.Setenv(readyPipeFdEnvVar, orig)

	os.Unsetenv(readyPipeFdEnvVar)
	err := NotifyReady()
	if err == nil {
		t.Error("NotifyReady should fail when not running as child")
	}
}

func TestNotifyErrorNotChild(t *testing.T) {
	// Save original value
	orig := os.Getenv(readyPipeFdEnvVar)
	defer os.Setenv(readyPipeFdEnvVar, orig)

	os.Unsetenv(readyPipeFdEnvVar)
	err := NotifyError(os.ErrNotExist)
	if err == nil {
		t.Error("NotifyError should fail when not running as child")
	}
}

func TestStatusConstants(t *testing.T) {
	// Verify status constants are distinct
	if statusReady == statusError {
		t.Error("statusReady and statusError should be different")
	}
}

func TestNotifyReady_WithPipe(t *testing.T) {
	// Create a pipe
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	defer readPipe.Close()

	// Save original env and set up for test
	origFd := os.Getenv(readyPipeFdEnvVar)
	defer os.Setenv(readyPipeFdEnvVar, origFd)

	// Set the env var to point to our write pipe's fd
	os.Setenv(readyPipeFdEnvVar, strconv.Itoa(int(writePipe.Fd())))

	// Call NotifyReady
	err = NotifyReady()
	if err != nil {
		t.Errorf("NotifyReady failed: %v", err)
	}

	// Read from the pipe and verify the status
	status := make([]byte, 1)
	n, err := readPipe.Read(status)
	if err != nil {
		t.Fatalf("Failed to read from pipe: %v", err)
	}
	if n != 1 {
		t.Errorf("Expected to read 1 byte, got %d", n)
	}
	if status[0] != statusReady {
		t.Errorf("Expected status %d (statusReady), got %d", statusReady, status[0])
	}
}

func TestNotifyError_WithPipe(t *testing.T) {
	// Create a pipe
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	defer readPipe.Close()

	// Save original env and set up for test
	origFd := os.Getenv(readyPipeFdEnvVar)
	defer os.Setenv(readyPipeFdEnvVar, origFd)

	// Set the env var to point to our write pipe's fd
	os.Setenv(readyPipeFdEnvVar, strconv.Itoa(int(writePipe.Fd())))

	// Call NotifyError
	testErr := os.ErrNotExist
	err = NotifyError(testErr)
	if err != nil {
		t.Errorf("NotifyError failed: %v", err)
	}

	// Read from the pipe and verify the status
	status := make([]byte, 1)
	n, err := readPipe.Read(status)
	if err != nil {
		t.Fatalf("Failed to read status from pipe: %v", err)
	}
	if n != 1 {
		t.Errorf("Expected to read 1 byte for status, got %d", n)
	}
	if status[0] != statusError {
		t.Errorf("Expected status %d (statusError), got %d", statusError, status[0])
	}
}

func TestNotifyError_Message(t *testing.T) {
	// Create a pipe
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	defer readPipe.Close()

	// Save original env and set up for test
	origFd := os.Getenv(readyPipeFdEnvVar)
	defer os.Setenv(readyPipeFdEnvVar, origFd)

	// Set the env var to point to our write pipe's fd
	os.Setenv(readyPipeFdEnvVar, strconv.Itoa(int(writePipe.Fd())))

	// Call NotifyError with a custom error message
	customErr := fmt.Errorf("custom test error message")
	err = NotifyError(customErr)
	if err != nil {
		t.Errorf("NotifyError failed: %v", err)
	}

	// Close write end so we can read all data
	writePipe.Close()

	// Read status byte
	status := make([]byte, 1)
	n, err := readPipe.Read(status)
	if err != nil {
		t.Fatalf("Failed to read status from pipe: %v", err)
	}
	if n != 1 || status[0] != statusError {
		t.Errorf("Expected status byte %d, got %d", statusError, status[0])
	}

	// Read error message
	msgBuf := make([]byte, 256)
	n, err = readPipe.Read(msgBuf)
	if err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("Failed to read message from pipe: %v", err)
	}
	msg := string(msgBuf[:n])
	if msg != customErr.Error() {
		t.Errorf("Error message mismatch: got %q, want %q", msg, customErr.Error())
	}
}

func TestWritePidFile_InvalidPath(t *testing.T) {
	// Try to write to a path that doesn't exist
	err := WritePidFile("/nonexistent/directory/test.pid", 12345)
	if err == nil {
		t.Error("WritePidFile should fail for invalid path")
	}
}

func TestReadPidFile(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")

	if err := WritePidFile(pidFile, 12345); err != nil {
		t.Fatalf("WritePidFile failed: %v", err)
	}

	pid, err := ReadPidFile(pidFile)
	if err != nil {
		t.Fatalf("ReadPidFile failed: %v", err)
	}
	if pid != 12345 {
		t.Errorf("ReadPidFile = %d, want 12345", pid)
	}
}

func TestReadPidFile_NotFound(t *testing.T) {
	_, err := ReadPidFile("/nonexistent/test.pid")
	if err == nil {
		t.Error("ReadPidFile should fail for nonexistent file")
	}
}

func TestReadPidFile_InvalidContent(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "bad.pid")
	if err := os.WriteFile(pidFile, []byte("not-a-number\n"), 0644); err != nil {
		t.Fatalf("failed to write test pid file: %v", err)
	}

	_, err := ReadPidFile(pidFile)
	if err == nil {
		t.Error("ReadPidFile should fail for non-numeric content")
	}
}

func TestReadPidFile_NegativePid(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "neg.pid")
	if err := os.WriteFile(pidFile, []byte("-1\n"), 0644); err != nil {
		t.Fatalf("failed to write test pid file: %v", err)
	}

	_, err := ReadPidFile(pidFile)
	if err == nil {
		t.Error("ReadPidFile should fail for negative PID")
	}
}

func TestReadPidFile_ZeroPid(t *testing.T) {
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "zero.pid")
	if err := os.WriteFile(pidFile, []byte("0\n"), 0644); err != nil {
		t.Fatalf("failed to write test pid file: %v", err)
	}

	_, err := ReadPidFile(pidFile)
	if err == nil {
		t.Error("ReadPidFile should fail for zero PID")
	}
}
