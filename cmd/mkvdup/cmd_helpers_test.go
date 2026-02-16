package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cespare/xxhash/v2"
)

func TestFormatInt(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{99, "99"},
		{100, "100"},
		{999, "999"},
		{1000, "1,000"},
		{1234, "1,234"},
		{12345, "12,345"},
		{123456, "123,456"},
		{1234567, "1,234,567"},
		{1000000000, "1,000,000,000"},
		{3420000000, "3,420,000,000"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := formatInt(tt.input)
			if got != tt.expected {
				t.Errorf("formatInt(%d) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestCalculateFileChecksum(t *testing.T) {
	t.Run("normal file", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.bin")

		// Write known content
		content := []byte("Hello, World! This is test content for checksum verification.")
		if err := os.WriteFile(testFile, content, 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		// Calculate expected checksum
		expected := xxhash.Sum64(content)

		// Test the function
		checksum, err := calculateFileChecksum(testFile)
		if err != nil {
			t.Fatalf("calculateFileChecksum failed: %v", err)
		}
		if checksum != expected {
			t.Errorf("Checksum mismatch: got 0x%x, want 0x%x", checksum, expected)
		}
	})

	t.Run("empty file", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "empty.bin")

		// Create empty file
		if err := os.WriteFile(testFile, []byte{}, 0644); err != nil {
			t.Fatalf("Failed to write empty file: %v", err)
		}

		// Calculate expected checksum for empty data
		expected := xxhash.Sum64([]byte{})

		checksum, err := calculateFileChecksum(testFile)
		if err != nil {
			t.Fatalf("calculateFileChecksum failed for empty file: %v", err)
		}
		if checksum != expected {
			t.Errorf("Empty file checksum mismatch: got 0x%x, want 0x%x", checksum, expected)
		}
	})

	t.Run("file not found", func(t *testing.T) {
		tmpDir := t.TempDir()
		missingFile := filepath.Join(tmpDir, "nonexistent.bin")

		_, err := calculateFileChecksum(missingFile)
		if err == nil {
			t.Error("Expected error for nonexistent file")
		}
	})

	t.Run("large file", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "large.bin")

		// Create a 1MB file with pattern data
		size := 1024 * 1024
		content := make([]byte, size)
		for i := range content {
			content[i] = byte(i % 256)
		}
		if err := os.WriteFile(testFile, content, 0644); err != nil {
			t.Fatalf("Failed to write large file: %v", err)
		}

		expected := xxhash.Sum64(content)

		checksum, err := calculateFileChecksum(testFile)
		if err != nil {
			t.Fatalf("calculateFileChecksum failed for large file: %v", err)
		}
		if checksum != expected {
			t.Errorf("Large file checksum mismatch: got 0x%x, want 0x%x", checksum, expected)
		}
	})

	t.Run("consistency", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "consistent.bin")

		content := []byte("Test content for consistency check")
		if err := os.WriteFile(testFile, content, 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		// Calculate twice - should get same result
		checksum1, err := calculateFileChecksum(testFile)
		if err != nil {
			t.Fatalf("First checksum failed: %v", err)
		}

		checksum2, err := calculateFileChecksum(testFile)
		if err != nil {
			t.Fatalf("Second checksum failed: %v", err)
		}

		if checksum1 != checksum2 {
			t.Errorf("Checksums not consistent: got 0x%x and 0x%x", checksum1, checksum2)
		}
	})
}
