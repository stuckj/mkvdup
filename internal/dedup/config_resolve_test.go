package dedup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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

	configs, _, err := ResolveConfigs([]string{cfgPath})
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

	configs, _, err := ResolveConfigs([]string{mainPath})
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

	configs, _, err := ResolveConfigs([]string{mainPath})
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

	configs, _, err := ResolveConfigs([]string{cfgPath})
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

	configs, _, err := ResolveConfigs([]string{mainPath})
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

	configs, _, err := ResolveConfigs([]string{aPath})
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

	configs, _, err := ResolveConfigs([]string{cfgPath})
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

	configs, _, err := ResolveConfigs([]string{filepath.Join(dir, "parent.yaml")})
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

	configs, _, err := ResolveConfigs([]string{cfgPath})
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

	_, _, err := ResolveConfigs([]string{mainPath})
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

	_, _, err := ResolveConfigs([]string{cfgPath})
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

	_, _, err := ResolveConfigs([]string{cfgPath})
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

	configs, _, err := ResolveConfigs([]string{cfgPath})
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

	configs, _, err := ResolveConfigs([]string{cfgPath})
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

	configs, _, err := ResolveConfigs([]string{aPath, bPath})
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
	_, _, err := ResolveConfigs([]string{"/nonexistent/config.yaml"})
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}

// --- on_error_command tests ---

func TestResolveConfigs_OnErrorCommand_ListForm(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "cfg.yaml")
	writeYAML(t, cfgPath, `on_error_command:
  command: ["echo", "%source%", "%event%"]
`)

	_, errCmd, err := ResolveConfigs([]string{cfgPath})
	if err != nil {
		t.Fatalf("ResolveConfigs: %v", err)
	}
	if errCmd == nil {
		t.Fatal("expected non-nil ErrorCommandConfig")
	}
	if errCmd.Command.IsShell {
		t.Error("expected IsShell=false for list form")
	}
	if len(errCmd.Command.Args) != 3 {
		t.Fatalf("expected 3 args, got %d", len(errCmd.Command.Args))
	}
	if errCmd.Command.Args[0] != "echo" || errCmd.Command.Args[1] != "%source%" || errCmd.Command.Args[2] != "%event%" {
		t.Errorf("Args = %v, want [echo %%source%% %%event%%]", errCmd.Command.Args)
	}
	// Defaults should be applied
	if errCmd.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", errCmd.Timeout)
	}
	if errCmd.BatchInterval != 5*time.Second {
		t.Errorf("BatchInterval = %v, want 5s", errCmd.BatchInterval)
	}
}

func TestResolveConfigs_OnErrorCommand_StringForm(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "cfg.yaml")
	writeYAML(t, cfgPath, `on_error_command:
  command: "echo %source% %event%"
`)

	_, errCmd, err := ResolveConfigs([]string{cfgPath})
	if err != nil {
		t.Fatalf("ResolveConfigs: %v", err)
	}
	if errCmd == nil {
		t.Fatal("expected non-nil ErrorCommandConfig")
	}
	if !errCmd.Command.IsShell {
		t.Error("expected IsShell=true for string form")
	}
	if len(errCmd.Command.Args) != 1 {
		t.Fatalf("expected 1 arg, got %d", len(errCmd.Command.Args))
	}
	if errCmd.Command.Args[0] != "echo %source% %event%" {
		t.Errorf("Args[0] = %q, want %q", errCmd.Command.Args[0], "echo %source% %event%")
	}
}

func TestResolveConfigs_OnErrorCommand_CustomTimeouts(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "cfg.yaml")
	writeYAML(t, cfgPath, `on_error_command:
  command: ["echo", "test"]
  timeout: 10s
  batch_interval: 2s
`)

	_, errCmd, err := ResolveConfigs([]string{cfgPath})
	if err != nil {
		t.Fatalf("ResolveConfigs: %v", err)
	}
	if errCmd == nil {
		t.Fatal("expected non-nil ErrorCommandConfig")
	}
	if errCmd.Timeout != 10*time.Second {
		t.Errorf("Timeout = %v, want 10s", errCmd.Timeout)
	}
	if errCmd.BatchInterval != 2*time.Second {
		t.Errorf("BatchInterval = %v, want 2s", errCmd.BatchInterval)
	}
}

func TestResolveConfigs_OnErrorCommand_FirstWins(t *testing.T) {
	dir := t.TempDir()

	cfg1Path := filepath.Join(dir, "first.yaml")
	writeYAML(t, cfg1Path, `on_error_command:
  command: ["first-cmd"]
  timeout: 10s
`)

	cfg2Path := filepath.Join(dir, "second.yaml")
	writeYAML(t, cfg2Path, `on_error_command:
  command: ["second-cmd"]
  timeout: 20s
`)

	_, errCmd, err := ResolveConfigs([]string{cfg1Path, cfg2Path})
	if err != nil {
		t.Fatalf("ResolveConfigs: %v", err)
	}
	if errCmd == nil {
		t.Fatal("expected non-nil ErrorCommandConfig")
	}
	// First config should win
	if errCmd.Command.Args[0] != "first-cmd" {
		t.Errorf("Command.Args[0] = %q, want %q", errCmd.Command.Args[0], "first-cmd")
	}
	if errCmd.Timeout != 10*time.Second {
		t.Errorf("Timeout = %v, want 10s (from first config)", errCmd.Timeout)
	}
}

func TestResolveConfigs_OnErrorCommand_FirstWins_ViaInclude(t *testing.T) {
	dir := t.TempDir()

	// Child config with its own on_error_command
	childPath := filepath.Join(dir, "child.yaml")
	writeYAML(t, childPath, `on_error_command:
  command: ["child-cmd"]
  timeout: 20s
`)

	// Parent config with on_error_command AND an include of the child
	parentPath := filepath.Join(dir, "parent.yaml")
	writeYAML(t, parentPath, fmt.Sprintf(`on_error_command:
  command: ["parent-cmd"]
  timeout: 10s
includes:
  - "%s"
`, childPath))

	_, errCmd, err := ResolveConfigs([]string{parentPath})
	if err != nil {
		t.Fatalf("ResolveConfigs: %v", err)
	}
	if errCmd == nil {
		t.Fatal("expected non-nil ErrorCommandConfig")
	}
	// Parent's on_error_command is encountered first (depth-first), so it wins
	if errCmd.Command.Args[0] != "parent-cmd" {
		t.Errorf("Command.Args[0] = %q, want %q", errCmd.Command.Args[0], "parent-cmd")
	}
	if errCmd.Timeout != 10*time.Second {
		t.Errorf("Timeout = %v, want 10s (from parent config)", errCmd.Timeout)
	}
}

func TestResolveConfigs_OnErrorCommand_NilWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "cfg.yaml")
	writeYAML(t, cfgPath, `name: "movie.mkv"
dedup_file: "/data/movie.mkvdup"
source_dir: "/data/source"
`)

	_, errCmd, err := ResolveConfigs([]string{cfgPath})
	if err != nil {
		t.Fatalf("ResolveConfigs: %v", err)
	}
	if errCmd != nil {
		t.Errorf("expected nil ErrorCommandConfig, got %+v", errCmd)
	}
}

func TestResolveConfigs_OnErrorCommand_MissingCommand(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "cfg.yaml")
	writeYAML(t, cfgPath, `name: "movie.mkv"
dedup_file: "/data/movie.mkvdup"
source_dir: "/data/source"
on_error_command:
  timeout: 10s
`)

	_, _, err := ResolveConfigs([]string{cfgPath})
	if err == nil {
		t.Fatal("expected error for on_error_command with missing command")
	}
	if !strings.Contains(err.Error(), "missing command") {
		t.Errorf("error = %q, want it to contain 'missing command'", err)
	}
}
