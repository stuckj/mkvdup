package dedup

import (
	"fmt"
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

// writeYAML is a test helper that writes content to a file.
func writeYAML(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestResolveConfigs_BasicConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "movie.mkvdup.yaml")
	writeYAML(t, cfgPath, `name: "movie.mkv"
dedup_file: "/data/movie.mkvdup"
source_dir: "/data/source"
`)

	configs, err := ResolveConfigs([]string{cfgPath})
	if err != nil {
		t.Fatalf("ResolveConfigs: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("got %d configs, want 1", len(configs))
	}
	if configs[0].Name != "movie.mkv" {
		t.Errorf("Name = %q, want %q", configs[0].Name, "movie.mkv")
	}
	if configs[0].DedupFile != "/data/movie.mkvdup" {
		t.Errorf("DedupFile = %q, want %q", configs[0].DedupFile, "/data/movie.mkvdup")
	}
	if configs[0].SourceDir != "/data/source" {
		t.Errorf("SourceDir = %q, want %q", configs[0].SourceDir, "/data/source")
	}
}

func TestResolveConfigs_IncludesSimpleGlob(t *testing.T) {
	dir := t.TempDir()

	// Create two config files in a subdirectory.
	subDir := filepath.Join(dir, "configs")
	writeYAML(t, filepath.Join(subDir, "a.mkvdup.yaml"), `name: "a.mkv"
dedup_file: "/data/a.mkvdup"
source_dir: "/data/source"
`)
	writeYAML(t, filepath.Join(subDir, "b.mkvdup.yaml"), `name: "b.mkv"
dedup_file: "/data/b.mkvdup"
source_dir: "/data/source"
`)

	// Create a parent config that includes them.
	mainPath := filepath.Join(dir, "main.yaml")
	writeYAML(t, mainPath, fmt.Sprintf(`includes:
  - "%s/*.mkvdup.yaml"
`, subDir))

	configs, err := ResolveConfigs([]string{mainPath})
	if err != nil {
		t.Fatalf("ResolveConfigs: %v", err)
	}
	if len(configs) != 2 {
		t.Fatalf("got %d configs, want 2", len(configs))
	}

	names := []string{configs[0].Name, configs[1].Name}
	if names[0] != "a.mkv" || names[1] != "b.mkv" {
		t.Errorf("names = %v, want [a.mkv b.mkv]", names)
	}
}

func TestResolveConfigs_IncludesRecursiveGlob(t *testing.T) {
	dir := t.TempDir()

	// Create config files in nested directories.
	writeYAML(t, filepath.Join(dir, "sub", "deep", "movie.mkvdup.yaml"), `name: "deep.mkv"
dedup_file: "/data/deep.mkvdup"
source_dir: "/data/source"
`)

	mainPath := filepath.Join(dir, "main.yaml")
	writeYAML(t, mainPath, fmt.Sprintf(`includes:
  - "%s/**/*.mkvdup.yaml"
`, dir))

	configs, err := ResolveConfigs([]string{mainPath})
	if err != nil {
		t.Fatalf("ResolveConfigs: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("got %d configs, want 1", len(configs))
	}
	if configs[0].Name != "deep.mkv" {
		t.Errorf("Name = %q, want %q", configs[0].Name, "deep.mkv")
	}
}

func TestResolveConfigs_VirtualFiles(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "vf.yaml")
	writeYAML(t, cfgPath, `virtual_files:
  - name: "Movies/movie1.mkv"
    dedup_file: "/data/movie1.mkvdup"
    source_dir: "/data/source1"
  - name: "Movies/movie2.mkv"
    dedup_file: "/data/movie2.mkvdup"
    source_dir: "/data/source2"
`)

	configs, err := ResolveConfigs([]string{cfgPath})
	if err != nil {
		t.Fatalf("ResolveConfigs: %v", err)
	}
	if len(configs) != 2 {
		t.Fatalf("got %d configs, want 2", len(configs))
	}
	if configs[0].Name != "Movies/movie1.mkv" {
		t.Errorf("configs[0].Name = %q, want %q", configs[0].Name, "Movies/movie1.mkv")
	}
	if configs[1].Name != "Movies/movie2.mkv" {
		t.Errorf("configs[1].Name = %q, want %q", configs[1].Name, "Movies/movie2.mkv")
	}
}

func TestResolveConfigs_Combination(t *testing.T) {
	dir := t.TempDir()

	// An included config.
	subDir := filepath.Join(dir, "configs")
	writeYAML(t, filepath.Join(subDir, "included.mkvdup.yaml"), `name: "included.mkv"
dedup_file: "/data/included.mkvdup"
source_dir: "/data/source"
`)

	// A config with top-level fields, includes, and virtual_files.
	mainPath := filepath.Join(dir, "main.yaml")
	writeYAML(t, mainPath, fmt.Sprintf(`name: "top.mkv"
dedup_file: "/data/top.mkvdup"
source_dir: "/data/source"
includes:
  - "%s/*.mkvdup.yaml"
virtual_files:
  - name: "vf.mkv"
    dedup_file: "/data/vf.mkvdup"
    source_dir: "/data/source"
`, subDir))

	configs, err := ResolveConfigs([]string{mainPath})
	if err != nil {
		t.Fatalf("ResolveConfigs: %v", err)
	}
	// Should have: top-level (1) + included (1) + virtual_files (1) = 3
	if len(configs) != 3 {
		t.Fatalf("got %d configs, want 3", len(configs))
	}
	if configs[0].Name != "top.mkv" {
		t.Errorf("configs[0].Name = %q, want %q", configs[0].Name, "top.mkv")
	}
	if configs[1].Name != "included.mkv" {
		t.Errorf("configs[1].Name = %q, want %q", configs[1].Name, "included.mkv")
	}
	if configs[2].Name != "vf.mkv" {
		t.Errorf("configs[2].Name = %q, want %q", configs[2].Name, "vf.mkv")
	}
}

func TestResolveConfigs_CycleDetection(t *testing.T) {
	dir := t.TempDir()

	aPath := filepath.Join(dir, "a.yaml")
	bPath := filepath.Join(dir, "b.yaml")

	// A includes B, B includes A.
	writeYAML(t, aPath, fmt.Sprintf(`name: "a.mkv"
dedup_file: "/data/a.mkvdup"
source_dir: "/data/source"
includes:
  - "%s"
`, bPath))
	writeYAML(t, bPath, fmt.Sprintf(`name: "b.mkv"
dedup_file: "/data/b.mkvdup"
source_dir: "/data/source"
includes:
  - "%s"
`, aPath))

	configs, err := ResolveConfigs([]string{aPath})
	if err != nil {
		t.Fatalf("ResolveConfigs: %v", err)
	}
	// Should resolve without infinite loop; both A and B included once.
	if len(configs) != 2 {
		t.Fatalf("got %d configs, want 2", len(configs))
	}
}

func TestResolveConfigs_RelativePaths(t *testing.T) {
	dir := t.TempDir()

	// Config with relative dedup_file and source_dir.
	cfgPath := filepath.Join(dir, "configs", "rel.yaml")
	writeYAML(t, cfgPath, `name: "rel.mkv"
dedup_file: "../data/rel.mkvdup"
source_dir: "../sources/dvd"
`)

	configs, err := ResolveConfigs([]string{cfgPath})
	if err != nil {
		t.Fatalf("ResolveConfigs: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("got %d configs, want 1", len(configs))
	}

	// Relative paths should be resolved against the config file's directory.
	wantDedup := filepath.Join(dir, "data", "rel.mkvdup")
	wantSource := filepath.Join(dir, "sources", "dvd")
	if configs[0].DedupFile != wantDedup {
		t.Errorf("DedupFile = %q, want %q", configs[0].DedupFile, wantDedup)
	}
	if configs[0].SourceDir != wantSource {
		t.Errorf("SourceDir = %q, want %q", configs[0].SourceDir, wantSource)
	}
}

func TestResolveConfigs_RelativeInclude(t *testing.T) {
	dir := t.TempDir()

	// Config in a subdirectory with a relative include pattern.
	writeYAML(t, filepath.Join(dir, "sub", "child.mkvdup.yaml"), `name: "child.mkv"
dedup_file: "/data/child.mkvdup"
source_dir: "/data/source"
`)
	writeYAML(t, filepath.Join(dir, "parent.yaml"), `includes:
  - "sub/*.mkvdup.yaml"
`)

	configs, err := ResolveConfigs([]string{filepath.Join(dir, "parent.yaml")})
	if err != nil {
		t.Fatalf("ResolveConfigs: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("got %d configs, want 1", len(configs))
	}
	if configs[0].Name != "child.mkv" {
		t.Errorf("Name = %q, want %q", configs[0].Name, "child.mkv")
	}
}

func TestResolveConfigs_NoMatchesNotError(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "empty.yaml")
	writeYAML(t, cfgPath, `includes:
  - "/nonexistent/path/*.mkvdup.yaml"
`)

	configs, err := ResolveConfigs([]string{cfgPath})
	if err != nil {
		t.Fatalf("ResolveConfigs: %v", err)
	}
	if len(configs) != 0 {
		t.Errorf("got %d configs, want 0", len(configs))
	}
}

func TestResolveConfigs_InvalidIncludedConfig(t *testing.T) {
	dir := t.TempDir()

	// An included config with invalid YAML.
	writeYAML(t, filepath.Join(dir, "bad.mkvdup.yaml"), "invalid: yaml: content: [")

	mainPath := filepath.Join(dir, "main.yaml")
	writeYAML(t, mainPath, fmt.Sprintf(`includes:
  - "%s/*.mkvdup.yaml"
`, dir))

	_, err := ResolveConfigs([]string{mainPath})
	if err == nil {
		t.Fatal("expected error for invalid included config, got nil")
	}
}

func TestResolveConfigs_PartialTopLevelFields(t *testing.T) {
	dir := t.TempDir()

	// Config with only some top-level fields (name but not dedup_file/source_dir).
	cfgPath := filepath.Join(dir, "partial.yaml")
	writeYAML(t, cfgPath, `name: "movie.mkv"
`)

	_, err := ResolveConfigs([]string{cfgPath})
	if err == nil {
		t.Fatal("expected error for partial top-level fields, got nil")
	}
	if !strings.Contains(err.Error(), "must all be set") {
		t.Errorf("error = %q, want error about partial fields", err.Error())
	}
}

func TestResolveConfigs_VirtualFilesMissingFields(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "bad_vf.yaml")
	writeYAML(t, cfgPath, `virtual_files:
  - name: "movie.mkv"
`)

	_, err := ResolveConfigs([]string{cfgPath})
	if err == nil {
		t.Fatal("expected error for virtual_files with missing fields, got nil")
	}
	if !strings.Contains(err.Error(), "missing required fields") {
		t.Errorf("error = %q, want error about missing required fields", err.Error())
	}
}

func TestResolveConfigs_EmptyConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "empty.yaml")
	writeYAML(t, cfgPath, "# just a comment\n")

	configs, err := ResolveConfigs([]string{cfgPath})
	if err != nil {
		t.Fatalf("ResolveConfigs: %v", err)
	}
	if len(configs) != 0 {
		t.Errorf("got %d configs, want 0", len(configs))
	}
}

func TestResolveConfigs_VirtualFilesRelativePaths(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "configs", "vf_rel.yaml")
	writeYAML(t, cfgPath, `virtual_files:
  - name: "movie.mkv"
    dedup_file: "../data/movie.mkvdup"
    source_dir: "../sources/dvd"
`)

	configs, err := ResolveConfigs([]string{cfgPath})
	if err != nil {
		t.Fatalf("ResolveConfigs: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("got %d configs, want 1", len(configs))
	}

	wantDedup := filepath.Join(dir, "data", "movie.mkvdup")
	wantSource := filepath.Join(dir, "sources", "dvd")
	if configs[0].DedupFile != wantDedup {
		t.Errorf("DedupFile = %q, want %q", configs[0].DedupFile, wantDedup)
	}
	if configs[0].SourceDir != wantSource {
		t.Errorf("SourceDir = %q, want %q", configs[0].SourceDir, wantSource)
	}
}

func TestResolveConfigs_MultipleInputPaths(t *testing.T) {
	dir := t.TempDir()

	aPath := filepath.Join(dir, "a.yaml")
	bPath := filepath.Join(dir, "b.yaml")
	writeYAML(t, aPath, `name: "a.mkv"
dedup_file: "/data/a.mkvdup"
source_dir: "/data/source"
`)
	writeYAML(t, bPath, `name: "b.mkv"
dedup_file: "/data/b.mkvdup"
source_dir: "/data/source"
`)

	configs, err := ResolveConfigs([]string{aPath, bPath})
	if err != nil {
		t.Fatalf("ResolveConfigs: %v", err)
	}
	if len(configs) != 2 {
		t.Fatalf("got %d configs, want 2", len(configs))
	}
	if configs[0].Name != "a.mkv" || configs[1].Name != "b.mkv" {
		t.Errorf("names = [%q, %q], want [a.mkv, b.mkv]", configs[0].Name, configs[1].Name)
	}
}

func TestResolveConfigs_FileNotFound(t *testing.T) {
	_, err := ResolveConfigs([]string{"/nonexistent/config.yaml"})
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}

// --- Batch manifest tests ---

func TestReadBatchManifest_Valid(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "batch.yaml")
	writeYAML(t, manifestPath, `source_dir: /data/source
files:
  - mkv: /data/ep1.mkv
    output: /data/ep1.mkvdup
    name: "Show/S01/E01.mkv"
  - mkv: /data/ep2.mkv
    output: /data/ep2.mkvdup
    name: "Show/S01/E02.mkv"
`)

	m, err := ReadBatchManifest(manifestPath)
	if err != nil {
		t.Fatalf("ReadBatchManifest: %v", err)
	}
	if m.SourceDir != "/data/source" {
		t.Errorf("SourceDir = %q, want %q", m.SourceDir, "/data/source")
	}
	if len(m.Files) != 2 {
		t.Fatalf("got %d files, want 2", len(m.Files))
	}
	if m.Files[0].MKV != "/data/ep1.mkv" {
		t.Errorf("Files[0].MKV = %q, want %q", m.Files[0].MKV, "/data/ep1.mkv")
	}
	if m.Files[0].Output != "/data/ep1.mkvdup" {
		t.Errorf("Files[0].Output = %q, want %q", m.Files[0].Output, "/data/ep1.mkvdup")
	}
	if m.Files[0].Name != "Show/S01/E01.mkv" {
		t.Errorf("Files[0].Name = %q, want %q", m.Files[0].Name, "Show/S01/E01.mkv")
	}
}

func TestReadBatchManifest_Defaults(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "batch.yaml")
	writeYAML(t, manifestPath, `source_dir: /data/source
files:
  - mkv: /data/ep1.mkv
    output: /data/ep1.mkvdup
  - mkv: /data/ep2.mkv
    output: /data/ep2.mkvdup
`)

	m, err := ReadBatchManifest(manifestPath)
	if err != nil {
		t.Fatalf("ReadBatchManifest: %v", err)
	}
	if m.Files[0].Output != "/data/ep1.mkvdup" {
		t.Errorf("Files[0].Output = %q, want %q", m.Files[0].Output, "/data/ep1.mkvdup")
	}
	// name defaults to basename of mkv
	if m.Files[0].Name != "ep1.mkv" {
		t.Errorf("Files[0].Name = %q, want %q", m.Files[0].Name, "ep1.mkv")
	}
	if m.Files[1].Output != "/data/ep2.mkvdup" {
		t.Errorf("Files[1].Output = %q, want %q", m.Files[1].Output, "/data/ep2.mkvdup")
	}
	if m.Files[1].Name != "ep2.mkv" {
		t.Errorf("Files[1].Name = %q, want %q", m.Files[1].Name, "ep2.mkv")
	}
}

func TestReadBatchManifest_MissingOutput(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "batch.yaml")
	writeYAML(t, manifestPath, `source_dir: /data/source
files:
  - mkv: /data/ep1.mkv
`)

	_, err := ReadBatchManifest(manifestPath)
	if err == nil {
		t.Fatal("expected error for missing output field, got nil")
	}
	if !strings.Contains(err.Error(), "missing required 'output' field") {
		t.Errorf("error = %q, want to contain 'missing required output field'", err.Error())
	}
}

func TestReadBatchManifest_MKVExtensionAutoAdded(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "batch.yaml")
	writeYAML(t, manifestPath, `source_dir: /data/source
files:
  - mkv: /data/ep1.mkv
    output: /data/ep1.mkvdup
    name: "Show/S01/Episode 1"
  - mkv: /data/ep2.mkv
    output: /data/ep2.mkvdup
    name: "Episode 2.mkv"
  - mkv: /data/ep3.mkv
    output: /data/ep3.mkvdup
`)

	m, err := ReadBatchManifest(manifestPath)
	if err != nil {
		t.Fatalf("ReadBatchManifest: %v", err)
	}
	// Name without .mkv gets it auto-added
	if m.Files[0].Name != "Show/S01/Episode 1.mkv" {
		t.Errorf("Files[0].Name = %q, want %q", m.Files[0].Name, "Show/S01/Episode 1.mkv")
	}
	// Name already with .mkv stays unchanged
	if m.Files[1].Name != "Episode 2.mkv" {
		t.Errorf("Files[1].Name = %q, want %q", m.Files[1].Name, "Episode 2.mkv")
	}
	// Default name from basename already has .mkv
	if m.Files[2].Name != "ep3.mkv" {
		t.Errorf("Files[2].Name = %q, want %q", m.Files[2].Name, "ep3.mkv")
	}
}

func TestReadBatchManifest_RelativePaths(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "manifests")
	manifestPath := filepath.Join(subDir, "batch.yaml")
	writeYAML(t, manifestPath, `source_dir: ../sources/disc1
files:
  - mkv: ../mkvs/ep1.mkv
    output: ../output/ep1.mkvdup
  - mkv: ../mkvs/ep2.mkv
    output: ../output/ep2.mkvdup
`)

	m, err := ReadBatchManifest(manifestPath)
	if err != nil {
		t.Fatalf("ReadBatchManifest: %v", err)
	}

	wantSource := filepath.Join(dir, "sources", "disc1")
	if m.SourceDir != wantSource {
		t.Errorf("SourceDir = %q, want %q", m.SourceDir, wantSource)
	}

	wantMKV := filepath.Join(dir, "mkvs", "ep1.mkv")
	if m.Files[0].MKV != wantMKV {
		t.Errorf("Files[0].MKV = %q, want %q", m.Files[0].MKV, wantMKV)
	}

	wantOutput := filepath.Join(dir, "output", "ep1.mkvdup")
	if m.Files[0].Output != wantOutput {
		t.Errorf("Files[0].Output = %q, want %q", m.Files[0].Output, wantOutput)
	}

	// Second file: relative mkv and output
	wantMKV2 := filepath.Join(dir, "mkvs", "ep2.mkv")
	if m.Files[1].MKV != wantMKV2 {
		t.Errorf("Files[1].MKV = %q, want %q", m.Files[1].MKV, wantMKV2)
	}
	wantOutput2 := filepath.Join(dir, "output", "ep2.mkvdup")
	if m.Files[1].Output != wantOutput2 {
		t.Errorf("Files[1].Output = %q, want %q", m.Files[1].Output, wantOutput2)
	}
}

func TestReadBatchManifest_MissingSourceDir(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "batch.yaml")
	// No top-level source_dir and no per-file source_dir
	writeYAML(t, manifestPath, `files:
  - mkv: /data/ep1.mkv
    output: /data/ep1.mkvdup
`)

	_, err := ReadBatchManifest(manifestPath)
	if err == nil {
		t.Fatal("expected error for missing source_dir, got nil")
	}
	if !strings.Contains(err.Error(), "source_dir") {
		t.Errorf("error = %q, want mention of source_dir", err.Error())
	}
}

func TestReadBatchManifest_PerFileSourceDir(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "batch.yaml")
	// No top-level source_dir; each file has its own
	writeYAML(t, manifestPath, `files:
  - mkv: /data/ep1.mkv
    output: /data/ep1.mkvdup
    source_dir: /source/disc1
  - mkv: /data/ep2.mkv
    output: /data/ep2.mkvdup
    source_dir: /source/disc2
`)

	m, err := ReadBatchManifest(manifestPath)
	if err != nil {
		t.Fatalf("ReadBatchManifest: %v", err)
	}
	if m.SourceDir != "" {
		t.Errorf("SourceDir = %q, want empty", m.SourceDir)
	}
	if m.Files[0].SourceDir != "/source/disc1" {
		t.Errorf("Files[0].SourceDir = %q, want %q", m.Files[0].SourceDir, "/source/disc1")
	}
	if m.Files[1].SourceDir != "/source/disc2" {
		t.Errorf("Files[1].SourceDir = %q, want %q", m.Files[1].SourceDir, "/source/disc2")
	}
}

func TestReadBatchManifest_MixedSourceDir(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "batch.yaml")
	// Top-level default; one file overrides
	writeYAML(t, manifestPath, `source_dir: /source/default
files:
  - mkv: /data/ep1.mkv
    output: /data/ep1.mkvdup
  - mkv: /data/ep2.mkv
    output: /data/ep2.mkvdup
    source_dir: /source/override
`)

	m, err := ReadBatchManifest(manifestPath)
	if err != nil {
		t.Fatalf("ReadBatchManifest: %v", err)
	}
	// File without per-file source_dir inherits top-level
	if m.Files[0].SourceDir != "/source/default" {
		t.Errorf("Files[0].SourceDir = %q, want %q", m.Files[0].SourceDir, "/source/default")
	}
	// File with per-file source_dir overrides top-level
	if m.Files[1].SourceDir != "/source/override" {
		t.Errorf("Files[1].SourceDir = %q, want %q", m.Files[1].SourceDir, "/source/override")
	}
}

func TestReadBatchManifest_PerFileSourceDir_Relative(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "manifests")
	manifestPath := filepath.Join(subDir, "batch.yaml")
	writeYAML(t, manifestPath, `files:
  - mkv: /data/ep1.mkv
    output: /data/ep1.mkvdup
    source_dir: ../sources/disc1
  - mkv: /data/ep2.mkv
    output: /data/ep2.mkvdup
    source_dir: ../sources/disc2
`)

	m, err := ReadBatchManifest(manifestPath)
	if err != nil {
		t.Fatalf("ReadBatchManifest: %v", err)
	}

	wantSource1 := filepath.Join(dir, "sources", "disc1")
	wantSource2 := filepath.Join(dir, "sources", "disc2")
	if m.Files[0].SourceDir != wantSource1 {
		t.Errorf("Files[0].SourceDir = %q, want %q", m.Files[0].SourceDir, wantSource1)
	}
	if m.Files[1].SourceDir != wantSource2 {
		t.Errorf("Files[1].SourceDir = %q, want %q", m.Files[1].SourceDir, wantSource2)
	}
}

func TestReadBatchManifest_NoSourceDir_PartialFiles(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "batch.yaml")
	// No top-level; first file has source_dir, second doesn't
	writeYAML(t, manifestPath, `files:
  - mkv: /data/ep1.mkv
    output: /data/ep1.mkvdup
    source_dir: /source/disc1
  - mkv: /data/ep2.mkv
    output: /data/ep2.mkvdup
`)

	_, err := ReadBatchManifest(manifestPath)
	if err == nil {
		t.Fatal("expected error for file without source_dir, got nil")
	}
	if !strings.Contains(err.Error(), "files[1]") {
		t.Errorf("error = %q, want mention of files[1]", err.Error())
	}
	if !strings.Contains(err.Error(), "source_dir") {
		t.Errorf("error = %q, want mention of source_dir", err.Error())
	}
}

func TestReadBatchManifest_EmptyFiles(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "batch.yaml")
	writeYAML(t, manifestPath, `source_dir: /data/source
files: []
`)

	_, err := ReadBatchManifest(manifestPath)
	if err == nil {
		t.Fatal("expected error for empty files list, got nil")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error = %q, want mention of empty", err.Error())
	}
}

func TestReadBatchManifest_MissingMKV(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "batch.yaml")
	writeYAML(t, manifestPath, `source_dir: /data/source
files:
  - output: /data/ep1.mkvdup
`)

	_, err := ReadBatchManifest(manifestPath)
	if err == nil {
		t.Fatal("expected error for missing mkv field, got nil")
	}
	if !strings.Contains(err.Error(), "mkv") {
		t.Errorf("error = %q, want mention of mkv", err.Error())
	}
}

func TestReadBatchManifest_FileNotFound(t *testing.T) {
	_, err := ReadBatchManifest("/nonexistent/batch.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent manifest, got nil")
	}
}
