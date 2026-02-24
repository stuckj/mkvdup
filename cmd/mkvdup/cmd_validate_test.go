package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateConfigs_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "source")
	dedupPath := filepath.Join(dir, "movie.mkvdup")

	createTestDedupFile(t, dedupPath, sourceDir)
	writeTestYAML(t, filepath.Join(dir, "config.yaml"), fmt.Sprintf(`name: "movie.mkv"
dedup_file: %q
source_dir: %q
`, dedupPath, sourceDir))

	exitCode := validateConfigs([]string{filepath.Join(dir, "config.yaml")}, false, false, false)
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
}

func TestValidateConfigs_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	writeTestYAML(t, filepath.Join(dir, "bad.yaml"), `name: "test.mkv"
dedup_file: "test.mkvdup"
source_dir: [invalid yaml`)

	exitCode := validateConfigs([]string{filepath.Join(dir, "bad.yaml")}, false, false, false)
	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
}

func TestValidateConfigs_MissingFields(t *testing.T) {
	dir := t.TempDir()
	writeTestYAML(t, filepath.Join(dir, "partial.yaml"), `name: "test.mkv"
dedup_file: "/tmp/test.mkvdup"
`)

	exitCode := validateConfigs([]string{filepath.Join(dir, "partial.yaml")}, false, false, false)
	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
}

func TestValidateConfigs_MissingDedupFile(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "source")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeTestYAML(t, filepath.Join(dir, "config.yaml"), fmt.Sprintf(`name: "movie.mkv"
dedup_file: %q
source_dir: %q
`, filepath.Join(dir, "nonexistent.mkvdup"), sourceDir))

	exitCode := validateConfigs([]string{filepath.Join(dir, "config.yaml")}, false, false, false)
	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
}

func TestValidateConfigs_MissingSourceDir(t *testing.T) {
	dir := t.TempDir()
	dedupPath := filepath.Join(dir, "movie.mkvdup")
	sourceDir := filepath.Join(dir, "source")

	// Create dedup file but not source dir
	createTestDedupFile(t, dedupPath, sourceDir)
	os.RemoveAll(sourceDir) // Remove the source dir created by helper

	writeTestYAML(t, filepath.Join(dir, "config.yaml"), fmt.Sprintf(`name: "movie.mkv"
dedup_file: %q
source_dir: %q
`, dedupPath, filepath.Join(dir, "nonexistent_source")))

	exitCode := validateConfigs([]string{filepath.Join(dir, "config.yaml")}, false, false, false)
	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
}

func TestValidateConfigs_SourceDirIsFile(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "source")
	dedupPath := filepath.Join(dir, "movie.mkvdup")

	createTestDedupFile(t, dedupPath, sourceDir)
	os.RemoveAll(sourceDir)
	// Create a file where source dir should be
	fakeSrcPath := filepath.Join(dir, "fake_source")
	if err := os.WriteFile(fakeSrcPath, []byte("not a dir"), 0644); err != nil {
		t.Fatalf("write fake source: %v", err)
	}

	writeTestYAML(t, filepath.Join(dir, "config.yaml"), fmt.Sprintf(`name: "movie.mkv"
dedup_file: %q
source_dir: %q
`, dedupPath, fakeSrcPath))

	exitCode := validateConfigs([]string{filepath.Join(dir, "config.yaml")}, false, false, false)
	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
}

func TestValidateConfigs_DuplicateNames(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "source")
	dedupPath1 := filepath.Join(dir, "movie1.mkvdup")
	dedupPath2 := filepath.Join(dir, "movie2.mkvdup")

	createTestDedupFile(t, dedupPath1, sourceDir)
	createTestDedupFile(t, dedupPath2, sourceDir)

	writeTestYAML(t, filepath.Join(dir, "config1.yaml"), fmt.Sprintf(`name: "movie.mkv"
dedup_file: %q
source_dir: %q
`, dedupPath1, sourceDir))

	writeTestYAML(t, filepath.Join(dir, "config2.yaml"), fmt.Sprintf(`name: "movie.mkv"
dedup_file: %q
source_dir: %q
`, dedupPath2, sourceDir))

	// Without strict: warnings are OK, exit 0
	exitCode := validateConfigs([]string{
		filepath.Join(dir, "config1.yaml"),
		filepath.Join(dir, "config2.yaml"),
	}, false, false, false)
	if exitCode != 0 {
		t.Errorf("expected exit code 0 without strict, got %d", exitCode)
	}
}

func TestValidateConfigs_DuplicateNamesStrict(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "source")
	dedupPath1 := filepath.Join(dir, "movie1.mkvdup")
	dedupPath2 := filepath.Join(dir, "movie2.mkvdup")

	createTestDedupFile(t, dedupPath1, sourceDir)
	createTestDedupFile(t, dedupPath2, sourceDir)

	writeTestYAML(t, filepath.Join(dir, "config1.yaml"), fmt.Sprintf(`name: "movie.mkv"
dedup_file: %q
source_dir: %q
`, dedupPath1, sourceDir))

	writeTestYAML(t, filepath.Join(dir, "config2.yaml"), fmt.Sprintf(`name: "movie.mkv"
dedup_file: %q
source_dir: %q
`, dedupPath2, sourceDir))

	// With strict: warnings become errors, exit 1
	exitCode := validateConfigs([]string{
		filepath.Join(dir, "config1.yaml"),
		filepath.Join(dir, "config2.yaml"),
	}, false, false, true)
	if exitCode != 1 {
		t.Errorf("expected exit code 1 with strict, got %d", exitCode)
	}
}

func TestValidateConfigs_PathConflict(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "source")
	dedupPath1 := filepath.Join(dir, "movie1.mkvdup")
	dedupPath2 := filepath.Join(dir, "movie2.mkvdup")

	createTestDedupFile(t, dedupPath1, sourceDir)
	createTestDedupFile(t, dedupPath2, sourceDir)

	// First config uses "Movies" as a file name
	writeTestYAML(t, filepath.Join(dir, "config1.yaml"), fmt.Sprintf(`name: "Movies"
dedup_file: %q
source_dir: %q
`, dedupPath1, sourceDir))

	// Second config uses "Movies" as a directory component
	writeTestYAML(t, filepath.Join(dir, "config2.yaml"), fmt.Sprintf(`name: "Movies/Action/video.mkv"
dedup_file: %q
source_dir: %q
`, dedupPath2, sourceDir))

	exitCode := validateConfigs([]string{
		filepath.Join(dir, "config1.yaml"),
		filepath.Join(dir, "config2.yaml"),
	}, false, false, false)
	// Conflict is a warning, exit 0 without strict
	if exitCode != 0 {
		t.Errorf("expected exit code 0 without strict, got %d", exitCode)
	}
}

func TestValidateConfigs_InvalidPathDotDot(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "source")
	dedupPath := filepath.Join(dir, "movie.mkvdup")

	createTestDedupFile(t, dedupPath, sourceDir)

	writeTestYAML(t, filepath.Join(dir, "config.yaml"), fmt.Sprintf(`name: "../escape.mkv"
dedup_file: %q
source_dir: %q
`, dedupPath, sourceDir))

	exitCode := validateConfigs([]string{filepath.Join(dir, "config.yaml")}, false, false, false)
	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
}

func TestValidateConfigs_EmptyName(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "source")
	dedupPath := filepath.Join(dir, "movie.mkvdup")

	createTestDedupFile(t, dedupPath, sourceDir)

	// Create a config with virtual_files that has an empty-ish name
	writeTestYAML(t, filepath.Join(dir, "config.yaml"), fmt.Sprintf(`virtual_files:
  - name: "/"
    dedup_file: %q
    source_dir: %q
`, dedupPath, sourceDir))

	exitCode := validateConfigs([]string{filepath.Join(dir, "config.yaml")}, false, false, false)
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for empty name, got %d", exitCode)
	}
}

func TestValidateConfigs_ConfigDir(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "source")
	confDir := filepath.Join(dir, "configs")

	dedupPath1 := filepath.Join(dir, "m1.mkvdup")
	dedupPath2 := filepath.Join(dir, "m2.mkvdup")
	createTestDedupFile(t, dedupPath1, sourceDir)
	createTestDedupFile(t, dedupPath2, sourceDir)

	writeTestYAML(t, filepath.Join(confDir, "a.yaml"), fmt.Sprintf(`name: "a.mkv"
dedup_file: %q
source_dir: %q
`, dedupPath1, sourceDir))

	writeTestYAML(t, filepath.Join(confDir, "b.yaml"), fmt.Sprintf(`name: "b.mkv"
dedup_file: %q
source_dir: %q
`, dedupPath2, sourceDir))

	exitCode := validateConfigs([]string{confDir}, true, false, false)
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
}

func TestValidateConfigs_IncludesAndVirtualFiles(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "source")
	dedupPath1 := filepath.Join(dir, "m1.mkvdup")
	dedupPath2 := filepath.Join(dir, "m2.mkvdup")
	createTestDedupFile(t, dedupPath1, sourceDir)
	createTestDedupFile(t, dedupPath2, sourceDir)

	// Child config
	writeTestYAML(t, filepath.Join(dir, "child.mkvdup.yaml"), fmt.Sprintf(`name: "child.mkv"
dedup_file: %q
source_dir: %q
`, dedupPath1, sourceDir))

	// Parent config with includes and virtual_files
	writeTestYAML(t, filepath.Join(dir, "parent.yaml"), fmt.Sprintf(`includes:
  - %q
virtual_files:
  - name: "inline.mkv"
    dedup_file: %q
    source_dir: %q
`, filepath.Join(dir, "child.mkvdup.yaml"), dedupPath2, sourceDir))

	exitCode := validateConfigs([]string{filepath.Join(dir, "parent.yaml")}, false, false, false)
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
}

func TestValidateConfigs_CycleDetection(t *testing.T) {
	dir := t.TempDir()

	// Create two configs that include each other
	writeTestYAML(t, filepath.Join(dir, "a.yaml"), fmt.Sprintf(`includes:
  - %q
`, filepath.Join(dir, "b.yaml")))

	writeTestYAML(t, filepath.Join(dir, "b.yaml"), fmt.Sprintf(`includes:
  - %q
`, filepath.Join(dir, "a.yaml")))

	// Should not hang — cycle detection handles it
	exitCode := validateConfigs([]string{filepath.Join(dir, "a.yaml")}, false, false, false)
	// No entries, no errors — just empty result
	if exitCode != 0 {
		t.Errorf("expected exit code 0 (no entries), got %d", exitCode)
	}
}

func TestValidateConfigs_DeepValid(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "source")
	dedupPath := filepath.Join(dir, "movie.mkvdup")

	createTestDedupFile(t, dedupPath, sourceDir)
	writeTestYAML(t, filepath.Join(dir, "config.yaml"), fmt.Sprintf(`name: "movie.mkv"
dedup_file: %q
source_dir: %q
`, dedupPath, sourceDir))

	exitCode := validateConfigs([]string{filepath.Join(dir, "config.yaml")}, false, true, false)
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
}

func TestValidateConfigs_DeepCorrupt(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "source")
	dedupPath := filepath.Join(dir, "movie.mkvdup")

	createTestDedupFile(t, dedupPath, sourceDir)

	// Corrupt the dedup file by overwriting bytes in the middle
	data, err := os.ReadFile(dedupPath)
	if err != nil {
		t.Fatal(err)
	}
	// Corrupt near the footer area (checksum will mismatch)
	if len(data) > 30 {
		for i := len(data) - 30; i < len(data)-24; i++ {
			data[i] ^= 0xFF
		}
	}
	if err := os.WriteFile(dedupPath, data, 0644); err != nil {
		t.Fatalf("write corrupt dedup: %v", err)
	}

	writeTestYAML(t, filepath.Join(dir, "config.yaml"), fmt.Sprintf(`name: "movie.mkv"
dedup_file: %q
source_dir: %q
`, dedupPath, sourceDir))

	exitCode := validateConfigs([]string{filepath.Join(dir, "config.yaml")}, false, true, false)
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for corrupt dedup, got %d", exitCode)
	}
}

func TestCleanVirtualPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"movie.mkv", "movie.mkv"},
		{"Movies/Action/movie.mkv", "Movies/Action/movie.mkv"},
		{"/leading/slash.mkv", "leading/slash.mkv"},
		{"./relative/path.mkv", "relative/path.mkv"},
		{"foo//bar.mkv", "foo/bar.mkv"},
		{"trailing/", "trailing"},
		{"", ""},
		{"/", ""},
		{".", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := cleanVirtualPath(tt.input)
			if got != tt.expected {
				t.Errorf("cleanVirtualPath(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestExpandConfigDir(t *testing.T) {
	dir := t.TempDir()

	// Create some yaml files
	if err := os.WriteFile(filepath.Join(dir, "a.yaml"), []byte(""), 0644); err != nil {
		t.Fatalf("write a.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.yml"), []byte(""), 0644); err != nil {
		t.Fatalf("write b.yml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "c.txt"), []byte(""), 0644); err != nil {
		t.Fatalf("write c.txt: %v", err)
	}

	paths, err := expandConfigDir(dir)
	if err != nil {
		t.Fatalf("expandConfigDir: %v", err)
	}
	if len(paths) != 2 {
		t.Errorf("expected 2 yaml files, got %d: %v", len(paths), paths)
	}
}

func TestExpandConfigDir_Empty(t *testing.T) {
	dir := t.TempDir()

	_, err := expandConfigDir(dir)
	if err == nil {
		t.Error("expected error for empty directory")
	}
}

func TestExpandConfigDir_NotExist(t *testing.T) {
	_, err := expandConfigDir("/nonexistent/path")
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}
