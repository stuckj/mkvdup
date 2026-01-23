package dedup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.mkvdup.yaml")

	err := WriteConfig(configPath, "Test Movie", "/path/to/dedup.mkvdup", "/path/to/source")
	if err != nil {
		t.Fatalf("WriteConfig failed: %v", err)
	}

	// Verify file was created
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	content := string(data)

	// Verify content contains expected fields
	if !strings.Contains(content, `name: "Test Movie"`) {
		t.Errorf("Config missing name field, got: %s", content)
	}
	if !strings.Contains(content, `dedup_file: "/path/to/dedup.mkvdup"`) {
		t.Errorf("Config missing dedup_file field, got: %s", content)
	}
	if !strings.Contains(content, `source_dir: "/path/to/source"`) {
		t.Errorf("Config missing source_dir field, got: %s", content)
	}
}

func TestWriteConfig_SpecialChars(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.mkvdup.yaml")

	// Test with special characters in name (colons, parentheses, etc.)
	specialName := "Movie: The Sequel (2024) - Director's Cut"
	err := WriteConfig(configPath, specialName, "/path/to/file", "/path/to/source")
	if err != nil {
		t.Fatalf("WriteConfig failed: %v", err)
	}

	// Read back and verify
	config, err := ReadConfig(configPath)
	if err != nil {
		t.Fatalf("ReadConfig failed: %v", err)
	}

	if config.Name != specialName {
		t.Errorf("Name mismatch: got %q, want %q", config.Name, specialName)
	}
}

func TestReadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.mkvdup.yaml")

	// Write a valid config
	err := WriteConfig(configPath, "Test Movie", "/path/to/dedup.mkvdup", "/path/to/source")
	if err != nil {
		t.Fatalf("WriteConfig failed: %v", err)
	}

	// Read it back
	config, err := ReadConfig(configPath)
	if err != nil {
		t.Fatalf("ReadConfig failed: %v", err)
	}

	if config.Name != "Test Movie" {
		t.Errorf("Name mismatch: got %q, want %q", config.Name, "Test Movie")
	}
	if config.DedupFile != "/path/to/dedup.mkvdup" {
		t.Errorf("DedupFile mismatch: got %q, want %q", config.DedupFile, "/path/to/dedup.mkvdup")
	}
	if config.SourceDir != "/path/to/source" {
		t.Errorf("SourceDir mismatch: got %q, want %q", config.SourceDir, "/path/to/source")
	}
}

func TestReadConfig_MissingFields(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "missing name",
			content: "dedup_file: \"/path/to/file\"\nsource_dir: \"/path/to/source\"\n",
		},
		{
			name:    "missing dedup_file",
			content: "name: \"Test\"\nsource_dir: \"/path/to/source\"\n",
		},
		{
			name:    "missing source_dir",
			content: "name: \"Test\"\ndedup_file: \"/path/to/file\"\n",
		},
		{
			name:    "empty file",
			content: "",
		},
		{
			name:    "only comments",
			content: "# This is a comment\n# Another comment\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "test.mkvdup.yaml")

			err := os.WriteFile(configPath, []byte(tt.content), 0644)
			if err != nil {
				t.Fatalf("Failed to write test config: %v", err)
			}

			_, err = ReadConfig(configPath)
			if err == nil {
				t.Error("ReadConfig should have failed for config with missing fields")
			}
		})
	}
}

func TestReadConfig_FileNotFound(t *testing.T) {
	_, err := ReadConfig("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("ReadConfig should have failed for nonexistent file")
	}
}

func TestReadConfig_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.mkvdup.yaml")

	// Write invalid YAML
	err := os.WriteFile(configPath, []byte("invalid: yaml: content: ["), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	_, err = ReadConfig(configPath)
	if err == nil {
		t.Error("ReadConfig should have failed for invalid YAML")
	}
}

func TestReadConfig_UnquotedValues(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.mkvdup.yaml")

	// YAML supports unquoted values - test that yaml.v3 handles them
	content := `name: Test Movie
dedup_file: /path/to/dedup.mkvdup
source_dir: /path/to/source
`
	err := os.WriteFile(configPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	config, err := ReadConfig(configPath)
	if err != nil {
		t.Fatalf("ReadConfig failed: %v", err)
	}

	if config.Name != "Test Movie" {
		t.Errorf("Name mismatch: got %q, want %q", config.Name, "Test Movie")
	}
	if config.DedupFile != "/path/to/dedup.mkvdup" {
		t.Errorf("DedupFile mismatch: got %q, want %q", config.DedupFile, "/path/to/dedup.mkvdup")
	}
	if config.SourceDir != "/path/to/source" {
		t.Errorf("SourceDir mismatch: got %q, want %q", config.SourceDir, "/path/to/source")
	}
}
