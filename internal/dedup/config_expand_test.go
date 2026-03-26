package dedup

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestResolveIncludePaths_DirectConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "movie.mkvdup.yaml")
	writeYAML(t, cfgPath, `name: "movie.mkv"
dedup_file: "/data/movie.mkvdup"
source_dir: "/data/source"
`)

	files, err := ResolveIncludePaths([]string{cfgPath})
	if err != nil {
		t.Fatalf("ResolveIncludePaths: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("got %d files, want 1", len(files))
	}
}

func TestResolveIncludePaths_IncludesGlob(t *testing.T) {
	dir := t.TempDir()

	// Create two config files.
	writeYAML(t, filepath.Join(dir, "configs", "a.mkvdup.yaml"), `name: "a.mkv"
dedup_file: "/data/a.mkvdup"
source_dir: "/data/source"
`)
	writeYAML(t, filepath.Join(dir, "configs", "b.mkvdup.yaml"), `name: "b.mkv"
dedup_file: "/data/b.mkvdup"
source_dir: "/data/source"
`)

	// Create a parent config with includes glob.
	mainPath := filepath.Join(dir, "main.yaml")
	writeYAML(t, mainPath, fmt.Sprintf(`includes:
  - "%s/configs/*.mkvdup.yaml"
`, dir))

	files, err := ResolveIncludePaths([]string{mainPath})
	if err != nil {
		t.Fatalf("ResolveIncludePaths: %v", err)
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

func TestResolveIncludePaths_RecursiveGlob(t *testing.T) {
	dir := t.TempDir()

	writeYAML(t, filepath.Join(dir, "movies", "movie1", "movie1.mkvdup.yaml"), `name: "movie1.mkv"
dedup_file: "/data/movie1.mkvdup"
source_dir: "/data/source"
`)
	writeYAML(t, filepath.Join(dir, "movies", "movie2", "movie2.mkvdup.yaml"), `name: "movie2.mkv"
dedup_file: "/data/movie2.mkvdup"
source_dir: "/data/source"
`)

	mainPath := filepath.Join(dir, "main.yaml")
	writeYAML(t, mainPath, fmt.Sprintf(`includes:
  - "%s/movies/**/*.mkvdup.yaml"
`, dir))

	files, err := ResolveIncludePaths([]string{mainPath})
	if err != nil {
		t.Fatalf("ResolveIncludePaths: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2: %v", len(files), files)
	}
}

func TestResolveIncludePaths_Deduplicates(t *testing.T) {
	dir := t.TempDir()

	cfgPath := filepath.Join(dir, "movie.mkvdup.yaml")
	writeYAML(t, cfgPath, `name: "movie.mkv"
dedup_file: "/data/movie.mkvdup"
source_dir: "/data/source"
`)

	// Two includes that both match the same file.
	mainPath := filepath.Join(dir, "main.yaml")
	writeYAML(t, mainPath, fmt.Sprintf(`includes:
  - "%s/*.mkvdup.yaml"
  - "%s/movie.mkvdup.yaml"
`, dir, dir))

	files, err := ResolveIncludePaths([]string{mainPath})
	if err != nil {
		t.Fatalf("ResolveIncludePaths: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("got %d files, want 1 (dedup failed): %v", len(files), files)
	}
}

func TestResolveIncludePaths_NoMatches(t *testing.T) {
	dir := t.TempDir()

	mainPath := filepath.Join(dir, "main.yaml")
	writeYAML(t, mainPath, fmt.Sprintf(`includes:
  - "%s/nonexistent/**/*.mkvdup.yaml"
`, dir))

	files, err := ResolveIncludePaths([]string{mainPath})
	if err != nil {
		t.Fatalf("ResolveIncludePaths: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("got %d files, want 0", len(files))
	}
}

func TestResolveIncludePaths_IncludesOnly(t *testing.T) {
	dir := t.TempDir()

	// A config with only includes (no name/dedup_file/source_dir) should
	// not include itself in the output.
	writeYAML(t, filepath.Join(dir, "a.mkvdup.yaml"), `name: "a.mkv"
dedup_file: "/data/a.mkvdup"
source_dir: "/data/source"
`)

	mainPath := filepath.Join(dir, "main.yaml")
	writeYAML(t, mainPath, fmt.Sprintf(`includes:
  - "%s/a.mkvdup.yaml"
`, dir))

	files, err := ResolveIncludePaths([]string{mainPath})
	if err != nil {
		t.Fatalf("ResolveIncludePaths: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("got %d files, want 1: %v", len(files), files)
	}
	// Should be the included file, not main.yaml
	if filepath.Base(files[0]) != "a.mkvdup.yaml" {
		t.Errorf("expected a.mkvdup.yaml, got %s", files[0])
	}
}

func TestResolveIncludePaths_MultipleInputPaths(t *testing.T) {
	dir := t.TempDir()

	aPath := filepath.Join(dir, "a.mkvdup.yaml")
	writeYAML(t, aPath, `name: "a.mkv"
dedup_file: "/data/a.mkvdup"
source_dir: "/data/source"
`)
	bPath := filepath.Join(dir, "b.mkvdup.yaml")
	writeYAML(t, bPath, `name: "b.mkv"
dedup_file: "/data/b.mkvdup"
source_dir: "/data/source"
`)

	files, err := ResolveIncludePaths([]string{aPath, bPath})
	if err != nil {
		t.Fatalf("ResolveIncludePaths: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2", len(files))
	}
}
