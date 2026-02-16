package dedup

import (
	"path/filepath"
	"strings"
	"testing"
)

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
