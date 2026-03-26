package dedup

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestReadExpandConfig_Basic(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "expand.yaml")
	writeYAML(t, cfgPath, `sources:
  - path: /data/isos/dvds
    pattern: "**/*.mkvdup.yaml"
`)
	cfg, err := ReadExpandConfig(cfgPath)
	if err != nil {
		t.Fatalf("ReadExpandConfig: %v", err)
	}
	if len(cfg.Sources) != 1 {
		t.Fatalf("got %d sources, want 1", len(cfg.Sources))
	}
	if cfg.Sources[0].Path != "/data/isos/dvds" {
		t.Errorf("Path = %q, want %q", cfg.Sources[0].Path, "/data/isos/dvds")
	}
	if cfg.Sources[0].Pattern != "**/*.mkvdup.yaml" {
		t.Errorf("Pattern = %q, want %q", cfg.Sources[0].Pattern, "**/*.mkvdup.yaml")
	}
}

func TestReadExpandConfig_RelativePath(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "expand.yaml")
	writeYAML(t, cfgPath, `sources:
  - path: ./isos
    pattern: "*.yaml"
`)
	cfg, err := ReadExpandConfig(cfgPath)
	if err != nil {
		t.Fatalf("ReadExpandConfig: %v", err)
	}
	want := filepath.Join(dir, "isos")
	if cfg.Sources[0].Path != want {
		t.Errorf("Path = %q, want %q", cfg.Sources[0].Path, want)
	}
}

func TestReadExpandConfig_EmptySources(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "expand.yaml")
	writeYAML(t, cfgPath, `sources: []`)
	_, err := ReadExpandConfig(cfgPath)
	if err == nil {
		t.Fatal("expected error for empty sources")
	}
}

func TestReadExpandConfig_MissingPath(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "expand.yaml")
	writeYAML(t, cfgPath, `sources:
  - pattern: "*.yaml"
`)
	_, err := ReadExpandConfig(cfgPath)
	if err == nil {
		t.Fatal("expected error for missing path")
	}
}

func TestReadExpandConfig_MissingPattern(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "expand.yaml")
	writeYAML(t, cfgPath, `sources:
  - path: /data
`)
	_, err := ReadExpandConfig(cfgPath)
	if err == nil {
		t.Fatal("expected error for missing pattern")
	}
}

func TestReadExpandConfig_AbsolutePattern(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "expand.yaml")
	writeYAML(t, cfgPath, `sources:
  - path: /data
    pattern: "/absolute/path/*.yaml"
`)
	_, err := ReadExpandConfig(cfgPath)
	if err == nil {
		t.Fatal("expected error for absolute pattern")
	}
}

func TestResolveExpandConfig_MatchesFiles(t *testing.T) {
	dir := t.TempDir()

	// Create some .mkvdup.yaml files in a directory tree.
	writeYAML(t, filepath.Join(dir, "movies", "movie1", "movie1.mkvdup.yaml"), "name: movie1")
	writeYAML(t, filepath.Join(dir, "movies", "movie2", "movie2.mkvdup.yaml"), "name: movie2")
	// Non-matching file
	writeYAML(t, filepath.Join(dir, "movies", "other.txt"), "not a yaml")

	cfg := &ExpandConfig{
		Sources: []ExpandSource{
			{Path: dir, Pattern: "**/*.mkvdup.yaml"},
		},
	}

	files, err := ResolveExpandConfig(cfg)
	if err != nil {
		t.Fatalf("ResolveExpandConfig: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2: %v", len(files), files)
	}

	// Results should be sorted.
	for i := 1; i < len(files); i++ {
		if files[i] < files[i-1] {
			t.Errorf("files not sorted: %v", files)
			break
		}
	}
}

func TestResolveExpandConfig_Deduplicates(t *testing.T) {
	dir := t.TempDir()

	writeYAML(t, filepath.Join(dir, "sub", "test.mkvdup.yaml"), "name: test")

	cfg := &ExpandConfig{
		Sources: []ExpandSource{
			{Path: dir, Pattern: "**/*.mkvdup.yaml"},
			{Path: filepath.Join(dir, "sub"), Pattern: "*.mkvdup.yaml"},
		},
	}

	files, err := ResolveExpandConfig(cfg)
	if err != nil {
		t.Fatalf("ResolveExpandConfig: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("got %d files, want 1 (dedup failed): %v", len(files), files)
	}
}

func TestResolveExpandConfig_NoMatches(t *testing.T) {
	dir := t.TempDir()

	cfg := &ExpandConfig{
		Sources: []ExpandSource{
			{Path: dir, Pattern: "**/*.mkvdup.yaml"},
		},
	}

	files, err := ResolveExpandConfig(cfg)
	if err != nil {
		t.Fatalf("ResolveExpandConfig: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("got %d files, want 0", len(files))
	}
}

func TestResolveExpandConfig_MultipleSources(t *testing.T) {
	dir := t.TempDir()

	writeYAML(t, filepath.Join(dir, "dvds", "a.mkvdup.yaml"), "name: a")
	writeYAML(t, filepath.Join(dir, "blurays", "b.mkvdup.yaml"), "name: b")

	cfg := &ExpandConfig{
		Sources: []ExpandSource{
			{Path: filepath.Join(dir, "dvds"), Pattern: "*.mkvdup.yaml"},
			{Path: filepath.Join(dir, "blurays"), Pattern: "*.mkvdup.yaml"},
		},
	}

	files, err := ResolveExpandConfig(cfg)
	if err != nil {
		t.Fatalf("ResolveExpandConfig: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2", len(files))
	}
}

func TestReadExpandConfig_MultipleSources(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "expand.yaml")
	writeYAML(t, cfgPath, fmt.Sprintf(`sources:
  - path: %s/dvds
    pattern: "**/*.mkvdup.yaml"
  - path: %s/blurays
    pattern: "**/*.mkvdup.yaml"
`, dir, dir))

	cfg, err := ReadExpandConfig(cfgPath)
	if err != nil {
		t.Fatalf("ReadExpandConfig: %v", err)
	}
	if len(cfg.Sources) != 2 {
		t.Fatalf("got %d sources, want 2", len(cfg.Sources))
	}
}
