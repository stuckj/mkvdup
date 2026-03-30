package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRelocateDedup_BasicMove(t *testing.T) {
	dir := t.TempDir()
	oldQuiet := quiet
	quiet = true
	defer func() { quiet = oldQuiet }()

	srcDir := filepath.Join(dir, "old")
	dstDir := filepath.Join(dir, "new")

	// Create a source directory that the sidecar points to.
	sourceMediaDir := filepath.Join(dir, "media")
	if err := os.MkdirAll(sourceMediaDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create fake .mkvdup file and sidecar.
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	dedupPath := filepath.Join(srcDir, "movie.mkvdup")
	if err := os.WriteFile(dedupPath, []byte("fake-dedup-data"), 0644); err != nil {
		t.Fatal(err)
	}

	relMedia, _ := filepath.Rel(srcDir, sourceMediaDir)
	writeTestYAML(t, dedupPath+".yaml", `name: "movie.mkv"
dedup_file: "movie.mkvdup"
source_dir: "`+relMedia+`"
`)

	if err := os.MkdirAll(dstDir, 0755); err != nil {
		t.Fatal(err)
	}

	newPath := filepath.Join(dstDir, "movie.mkvdup")
	if err := relocateDedup(dedupPath, newPath, false, false); err != nil {
		t.Fatalf("relocateDedup: %v", err)
	}

	// Old files should be gone.
	if _, err := os.Stat(dedupPath); !os.IsNotExist(err) {
		t.Error("old .mkvdup file should be removed")
	}
	if _, err := os.Stat(dedupPath + ".yaml"); !os.IsNotExist(err) {
		t.Error("old sidecar should be removed")
	}

	// New files should exist.
	if _, err := os.Stat(newPath); err != nil {
		t.Error("new .mkvdup file should exist")
	}

	data, err := os.ReadFile(newPath + ".yaml")
	if err != nil {
		t.Fatalf("read new sidecar: %v", err)
	}
	sidecar := string(data)

	// dedup_file should point to the new filename (same directory as sidecar).
	if !strings.Contains(sidecar, `"movie.mkvdup"`) {
		t.Errorf("sidecar dedup_file should reference new location %q, got:\n%s", "movie.mkvdup", sidecar)
	}

	// source_dir should be recalculated relative to new location.
	newRelMedia, _ := filepath.Rel(dstDir, sourceMediaDir)
	if !strings.Contains(sidecar, newRelMedia) {
		t.Errorf("sidecar should contain recalculated source_dir %q, got:\n%s", newRelMedia, sidecar)
	}
}

func TestRelocateDedup_MoveIntoDirectory(t *testing.T) {
	dir := t.TempDir()
	oldQuiet := quiet
	quiet = true
	defer func() { quiet = oldQuiet }()

	sourceMediaDir := filepath.Join(dir, "media")
	if err := os.MkdirAll(sourceMediaDir, 0755); err != nil {
		t.Fatal(err)
	}

	dedupPath := filepath.Join(dir, "movie.mkvdup")
	if err := os.WriteFile(dedupPath, []byte("fake-dedup-data"), 0644); err != nil {
		t.Fatal(err)
	}
	writeTestYAML(t, dedupPath+".yaml", `name: "movie.mkv"
dedup_file: "movie.mkvdup"
source_dir: "media"
`)

	dstDir := filepath.Join(dir, "subdir")
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Move into a directory (not a specific file path).
	if err := relocateDedup(dedupPath, dstDir, false, false); err != nil {
		t.Fatalf("relocateDedup: %v", err)
	}

	newPath := filepath.Join(dstDir, "movie.mkvdup")
	if _, err := os.Stat(newPath); err != nil {
		t.Error("new .mkvdup file should exist in destination directory")
	}
	if _, err := os.Stat(newPath + ".yaml"); err != nil {
		t.Error("new sidecar should exist in destination directory")
	}
}

func TestRelocateDedup_DryRun(t *testing.T) {
	dir := t.TempDir()
	oldQuiet := quiet
	quiet = true
	defer func() { quiet = oldQuiet }()

	sourceMediaDir := filepath.Join(dir, "media")
	if err := os.MkdirAll(sourceMediaDir, 0755); err != nil {
		t.Fatal(err)
	}

	dedupPath := filepath.Join(dir, "movie.mkvdup")
	if err := os.WriteFile(dedupPath, []byte("fake-dedup-data"), 0644); err != nil {
		t.Fatal(err)
	}
	writeTestYAML(t, dedupPath+".yaml", `name: "movie.mkv"
dedup_file: "movie.mkvdup"
source_dir: "media"
`)

	newPath := filepath.Join(dir, "new", "movie.mkvdup")

	if err := relocateDedup(dedupPath, newPath, false, true); err != nil {
		t.Fatalf("relocateDedup --dry-run: %v", err)
	}

	// Files should NOT have moved.
	if _, err := os.Stat(dedupPath); err != nil {
		t.Error("original .mkvdup file should still exist after dry run")
	}
	if _, err := os.Stat(newPath); !os.IsNotExist(err) {
		t.Error("destination should not exist after dry run")
	}
}

func TestRelocateDedup_DestinationExists(t *testing.T) {
	dir := t.TempDir()
	oldQuiet := quiet
	quiet = true
	defer func() { quiet = oldQuiet }()

	sourceMediaDir := filepath.Join(dir, "media")
	if err := os.MkdirAll(sourceMediaDir, 0755); err != nil {
		t.Fatal(err)
	}

	dedupPath := filepath.Join(dir, "movie.mkvdup")
	if err := os.WriteFile(dedupPath, []byte("fake-dedup-data"), 0644); err != nil {
		t.Fatal(err)
	}
	writeTestYAML(t, dedupPath+".yaml", `name: "movie.mkv"
dedup_file: "movie.mkvdup"
source_dir: "media"
`)

	newPath := filepath.Join(dir, "other.mkvdup")
	if err := os.WriteFile(newPath, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}

	// Without --force, should fail.
	err := relocateDedup(dedupPath, newPath, false, false)
	if err == nil {
		t.Fatal("expected error when destination exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error should mention 'already exists', got: %v", err)
	}

	// With --force, should succeed.
	if err := relocateDedup(dedupPath, newPath, true, false); err != nil {
		t.Fatalf("relocateDedup --force: %v", err)
	}
	if _, err := os.Stat(newPath); err != nil {
		t.Error("destination should exist after --force")
	}
}

func TestRelocateDedup_TrailingSlashCreatesDir(t *testing.T) {
	dir := t.TempDir()
	oldQuiet := quiet
	quiet = true
	defer func() { quiet = oldQuiet }()

	dedupPath := filepath.Join(dir, "movie.mkvdup")
	if err := os.WriteFile(dedupPath, []byte("fake-dedup-data"), 0644); err != nil {
		t.Fatal(err)
	}

	// Destination with trailing slash — directory doesn't exist yet.
	dstDir := filepath.Join(dir, "newdir") + "/"

	if err := relocateDedup(dedupPath, dstDir, false, false); err != nil {
		t.Fatalf("relocateDedup with trailing slash: %v", err)
	}

	newPath := filepath.Join(dir, "newdir", "movie.mkvdup")
	if _, err := os.Stat(newPath); err != nil {
		t.Error("file should be moved into the created directory")
	}
}

func TestRelocateDedup_NoSidecar(t *testing.T) {
	dir := t.TempDir()
	oldQuiet := quiet
	quiet = true
	defer func() { quiet = oldQuiet }()

	dedupPath := filepath.Join(dir, "movie.mkvdup")
	if err := os.WriteFile(dedupPath, []byte("fake-dedup-data"), 0644); err != nil {
		t.Fatal(err)
	}

	newPath := filepath.Join(dir, "new", "movie.mkvdup")

	// Should succeed even without a sidecar.
	if err := relocateDedup(dedupPath, newPath, false, false); err != nil {
		t.Fatalf("relocateDedup without sidecar: %v", err)
	}

	if _, err := os.Stat(newPath); err != nil {
		t.Error("new .mkvdup file should exist")
	}
	if _, err := os.Stat(newPath + ".yaml"); !os.IsNotExist(err) {
		t.Error("no sidecar should be created when none existed")
	}
}

func TestRelocateDedup_AbsolutePathsPreserved(t *testing.T) {
	dir := t.TempDir()
	oldQuiet := quiet
	quiet = true
	defer func() { quiet = oldQuiet }()

	sourceMediaDir := filepath.Join(dir, "media")
	if err := os.MkdirAll(sourceMediaDir, 0755); err != nil {
		t.Fatal(err)
	}

	srcDir := filepath.Join(dir, "old")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}

	dedupPath := filepath.Join(srcDir, "movie.mkvdup")
	if err := os.WriteFile(dedupPath, []byte("fake-dedup-data"), 0644); err != nil {
		t.Fatal(err)
	}
	// Use absolute paths in sidecar.
	writeTestYAML(t, dedupPath+".yaml", `name: "movie.mkv"
dedup_file: "`+dedupPath+`"
source_dir: "`+sourceMediaDir+`"
`)

	dstDir := filepath.Join(dir, "new")
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		t.Fatal(err)
	}
	newPath := filepath.Join(dstDir, "movie.mkvdup")

	if err := relocateDedup(dedupPath, newPath, false, false); err != nil {
		t.Fatalf("relocateDedup: %v", err)
	}

	data, err := os.ReadFile(newPath + ".yaml")
	if err != nil {
		t.Fatalf("read new sidecar: %v", err)
	}
	sidecar := string(data)

	// Absolute dedup_file should be updated to the new location.
	if !strings.Contains(sidecar, newPath) {
		t.Errorf("absolute dedup_file should be updated to new path %q, got:\n%s", newPath, sidecar)
	}
	// Absolute source_dir should be preserved unchanged.
	if !strings.Contains(sidecar, sourceMediaDir) {
		t.Errorf("absolute source_dir should be preserved, got:\n%s", sidecar)
	}
}

func TestRelocateDedup_DestSidecarConflictNoSourceSidecar(t *testing.T) {
	dir := t.TempDir()

	dedupPath := filepath.Join(dir, "movie.mkvdup")
	if err := os.WriteFile(dedupPath, []byte("fake-dedup-data"), 0644); err != nil {
		t.Fatal(err)
	}
	// No source sidecar.

	newPath := filepath.Join(dir, "other.mkvdup")
	// Destination sidecar exists but source has none.
	if err := os.WriteFile(newPath+".yaml", []byte("stale-sidecar"), 0644); err != nil {
		t.Fatal(err)
	}

	// Without --force, should error about existing destination sidecar.
	err := relocateDedup(dedupPath, newPath, false, false)
	if err == nil {
		t.Fatal("expected error when destination sidecar exists without --force")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error should mention 'already exists', got: %v", err)
	}
}

func TestRelocateDedup_SamePathError(t *testing.T) {
	dir := t.TempDir()

	dedupPath := filepath.Join(dir, "movie.mkvdup")
	if err := os.WriteFile(dedupPath, []byte("fake-dedup-data"), 0644); err != nil {
		t.Fatal(err)
	}

	err := relocateDedup(dedupPath, dedupPath, false, false)
	if err == nil {
		t.Fatal("expected error when source and destination are the same")
	}
	if !strings.Contains(err.Error(), "same") {
		t.Errorf("error should mention 'same', got: %v", err)
	}
}

func TestRelocateDedup_SourceNotFound(t *testing.T) {
	dir := t.TempDir()

	err := relocateDedup(filepath.Join(dir, "nonexistent.mkvdup"), filepath.Join(dir, "dst.mkvdup"), false, false)
	if err == nil {
		t.Fatal("expected error for nonexistent source")
	}
}

func TestRelocateDedup_ForceRemovesOrphanedSidecar(t *testing.T) {
	dir := t.TempDir()
	oldQuiet := quiet
	quiet = true
	defer func() { quiet = oldQuiet }()

	// Source has no sidecar.
	dedupPath := filepath.Join(dir, "movie.mkvdup")
	if err := os.WriteFile(dedupPath, []byte("fake-dedup-data"), 0644); err != nil {
		t.Fatal(err)
	}

	// Destination already has a dedup file AND a sidecar.
	newPath := filepath.Join(dir, "other.mkvdup")
	if err := os.WriteFile(newPath, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath+".yaml", []byte("orphan-sidecar"), 0644); err != nil {
		t.Fatal(err)
	}

	// With --force, should succeed and remove the orphaned destination sidecar.
	if err := relocateDedup(dedupPath, newPath, true, false); err != nil {
		t.Fatalf("relocateDedup --force: %v", err)
	}
	if _, err := os.Stat(newPath + ".yaml"); !os.IsNotExist(err) {
		t.Error("orphaned destination sidecar should be removed when source has no sidecar")
	}
}

func TestRecalcRelativePath(t *testing.T) {
	tests := []struct {
		name    string
		oldBase string
		newBase string
		path    string
		want    string
	}{
		{
			name:    "same directory",
			oldBase: "/a/b",
			newBase: "/a/b",
			path:    "file.txt",
			want:    "file.txt",
		},
		{
			name:    "moved deeper",
			oldBase: "/a",
			newBase: "/a/b",
			path:    "file.txt",
			want:    "../file.txt",
		},
		{
			name:    "moved shallower",
			oldBase: "/a/b",
			newBase: "/a",
			path:    "../file.txt",
			want:    "file.txt",
		},
		{
			name:    "absolute path unchanged",
			oldBase: "/a/b",
			newBase: "/c/d",
			path:    "/absolute/path",
			want:    "/absolute/path",
		},
		{
			name:    "parent reference",
			oldBase: "/a/b",
			newBase: "/c/d",
			path:    "../media",
			want:    "../../a/media",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := recalcRelativePath(tt.oldBase, tt.newBase, tt.path)
			if err != nil {
				t.Fatalf("recalcRelativePath: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
