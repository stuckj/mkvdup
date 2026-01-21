package dedup

import (
	"os"
	"path/filepath"
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
	if !contains(content, `name: "Test Movie"`) {
		t.Errorf("Config missing name field, got: %s", content)
	}
	if !contains(content, `dedup_file: "/path/to/dedup.mkvdup"`) {
		t.Errorf("Config missing dedup_file field, got: %s", content)
	}
	if !contains(content, `source_dir: "/path/to/source"`) {
		t.Errorf("Config missing source_dir field, got: %s", content)
	}
}

func TestWriteConfig_SpecialChars(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.mkvdup.yaml")

	// Test with special characters in name (colons, parentheses, etc.)
	// Note: The simple YAML parser doesn't handle escaped quotes properly,
	// so we test with characters that don't require escaping.
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

func TestParseYAMLValue(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		key      string
		expected string
	}{
		{
			name:     "simple value",
			content:  "name: \"Test Movie\"\n",
			key:      "name",
			expected: "Test Movie",
		},
		{
			name:     "value with spaces",
			content:  "name: \"Movie With Spaces\"\n",
			key:      "name",
			expected: "Movie With Spaces",
		},
		{
			name:     "value with path",
			content:  "source_dir: \"/path/to/source\"\n",
			key:      "source_dir",
			expected: "/path/to/source",
		},
		{
			name:     "key not found",
			content:  "other_key: \"value\"\n",
			key:      "name",
			expected: "",
		},
		{
			name:     "multiple lines",
			content:  "name: \"First\"\ndedup_file: \"/path\"\nsource_dir: \"/source\"\n",
			key:      "dedup_file",
			expected: "/path",
		},
		{
			name:     "key in comment should not match",
			content:  "# name: \"Comment\"\nname: \"Actual\"\n",
			key:      "name",
			expected: "Actual",
		},
		{
			name:     "key as substring should not match",
			content:  "full_name: \"Wrong\"\nname: \"Right\"\n",
			key:      "name",
			expected: "Right",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseYAMLValue(tt.content, tt.key)
			if result != tt.expected {
				t.Errorf("parseYAMLValue(%q, %q) = %q, want %q", tt.content, tt.key, result, tt.expected)
			}
		})
	}
}

func TestParseYAMLValue_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		key      string
		expected string
	}{
		{
			name:     "empty content",
			content:  "",
			key:      "name",
			expected: "",
		},
		{
			name:     "no closing quote",
			content:  "name: \"Unclosed\n",
			key:      "name",
			expected: "",
		},
		{
			name:     "key without colon space quote",
			content:  "name:value\n",
			key:      "name",
			expected: "",
		},
		{
			name:     "key with different format",
			content:  "name = \"value\"\n",
			key:      "name",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseYAMLValue(tt.content, tt.key)
			if result != tt.expected {
				t.Errorf("parseYAMLValue(%q, %q) = %q, want %q", tt.content, tt.key, result, tt.expected)
			}
		})
	}
}

func TestIndexOf(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected int
	}{
		{
			name:     "found at beginning",
			s:        "hello world",
			substr:   "hello",
			expected: 0,
		},
		{
			name:     "found in middle",
			s:        "hello world",
			substr:   "wor",
			expected: 6,
		},
		{
			name:     "found at end",
			s:        "hello world",
			substr:   "world",
			expected: 6,
		},
		{
			name:     "not found",
			s:        "hello world",
			substr:   "xyz",
			expected: -1,
		},
		{
			name:     "empty substr",
			s:        "hello",
			substr:   "",
			expected: 0,
		},
		{
			name:     "substr longer than s",
			s:        "hi",
			substr:   "hello",
			expected: -1,
		},
		{
			name:     "exact match",
			s:        "hello",
			substr:   "hello",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := indexOf(tt.s, tt.substr)
			if result != tt.expected {
				t.Errorf("indexOf(%q, %q) = %d, want %d", tt.s, tt.substr, result, tt.expected)
			}
		})
	}
}

// contains is a helper function for string contains check
func contains(s, substr string) bool {
	return indexOf(s, substr) >= 0
}
