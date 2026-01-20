package daemon

import (
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
