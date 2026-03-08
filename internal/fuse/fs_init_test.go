package fuse

import (
	"fmt"
	"testing"

	"github.com/stuckj/mkvdup/internal/dedup"
)

func TestReadConfigHeaders_ParallelPath(t *testing.T) {
	// Create >4 configs to exercise the parallel branch
	const n = 10
	readers := make(map[string]*mockReader)
	configs := make([]dedup.Config, n)
	for i := range n {
		path := fmt.Sprintf("/dedup/%d.mkvdup", i)
		readers[path] = &mockReader{originalSize: int64(1000 + i)}
		configs[i] = dedup.Config{
			Name:      fmt.Sprintf("file%d.mkv", i),
			DedupFile: path,
			SourceDir: fmt.Sprintf("/source/%d", i),
		}
	}

	factory := &mockReaderFactory{readers: readers}
	results, err := readConfigHeaders(configs, factory, false)
	if err != nil {
		t.Fatalf("readConfigHeaders: %v", err)
	}

	if len(results) != n {
		t.Fatalf("expected %d results, got %d", n, len(results))
	}
	for i, r := range results {
		if r == nil {
			t.Fatalf("result[%d] is nil", i)
		}
		wantSize := int64(1000 + i)
		if r.Size != wantSize {
			t.Errorf("result[%d].Size = %d, want %d", i, r.Size, wantSize)
		}
		if r.Name != configs[i].Name {
			t.Errorf("result[%d].Name = %q, want %q", i, r.Name, configs[i].Name)
		}
	}
}

func TestReadConfigHeaders_ParallelError(t *testing.T) {
	// Create >4 configs where one will fail — must not deadlock
	const n = 10
	readers := make(map[string]*mockReader)
	configs := make([]dedup.Config, n)
	for i := range n {
		path := fmt.Sprintf("/dedup/%d.mkvdup", i)
		// Don't add reader for index 5 — it will fail
		if i != 5 {
			readers[path] = &mockReader{originalSize: int64(1000 + i)}
		}
		configs[i] = dedup.Config{
			Name:      fmt.Sprintf("file%d.mkv", i),
			DedupFile: path,
			SourceDir: fmt.Sprintf("/source/%d", i),
		}
	}

	factory := &mockReaderFactory{readers: readers}
	_, err := readConfigHeaders(configs, factory, false)
	if err == nil {
		t.Fatal("expected error when one config fails, got nil")
	}
	t.Logf("got expected error: %v", err)
}

func TestReadConfigHeaders_SequentialPath(t *testing.T) {
	// <=4 configs uses the sequential path
	const n = 3
	readers := make(map[string]*mockReader)
	configs := make([]dedup.Config, n)
	for i := range n {
		path := fmt.Sprintf("/dedup/%d.mkvdup", i)
		readers[path] = &mockReader{originalSize: int64(2000 + i)}
		configs[i] = dedup.Config{
			Name:      fmt.Sprintf("file%d.mkv", i),
			DedupFile: path,
			SourceDir: fmt.Sprintf("/source/%d", i),
		}
	}

	factory := &mockReaderFactory{readers: readers}
	results, err := readConfigHeaders(configs, factory, false)
	if err != nil {
		t.Fatalf("readConfigHeaders: %v", err)
	}

	if len(results) != n {
		t.Fatalf("expected %d results, got %d", n, len(results))
	}
	for i, r := range results {
		if r == nil {
			t.Fatalf("result[%d] is nil", i)
		}
		wantSize := int64(2000 + i)
		if r.Size != wantSize {
			t.Errorf("result[%d].Size = %d, want %d", i, r.Size, wantSize)
		}
	}
}

func TestReadConfigHeaders_AllFail(t *testing.T) {
	// All configs fail — should return first error promptly (no deadlock)
	const n = 10
	configs := make([]dedup.Config, n)
	for i := range n {
		configs[i] = dedup.Config{
			Name:      fmt.Sprintf("file%d.mkv", i),
			DedupFile: fmt.Sprintf("/dedup/%d.mkvdup", i),
			SourceDir: fmt.Sprintf("/source/%d", i),
		}
	}

	factory := &mockReaderFactory{err: fmt.Errorf("connection refused")}
	_, err := readConfigHeaders(configs, factory, false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
