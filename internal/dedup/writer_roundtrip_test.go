package dedup

import (
	"bytes"
	"testing"

	"github.com/stuckj/mkvdup/internal/matcher"
	"github.com/stuckj/mkvdup/internal/source"
)

func TestWriter_RoundTrip_DeltaOnly(t *testing.T) {
	dir := t.TempDir()
	deltaData := bytes.Repeat([]byte{0xAB}, 100)

	path := writeTestDedupFile(t, dir, writeTestOptions{
		originalSize:     100,
		originalChecksum: 0x1234,
		sourceType:       source.TypeDVD,
		result: &matcher.Result{
			Entries: []matcher.Entry{
				{MkvOffset: 0, Length: 100, Source: 0, SourceOffset: 0},
			},
			DeltaData:      deltaData,
			UnmatchedBytes: 100,
			TotalPackets:   1,
		},
	})

	r, err := NewReaderLazy(path, dir)
	if err != nil {
		t.Fatalf("NewReaderLazy: %v", err)
	}
	defer r.Close()

	info := r.Info()
	if got := info["entry_count"].(int); got != 1 {
		t.Errorf("entry_count = %d, want 1", got)
	}
	if got := info["delta_size"].(int64); got != 100 {
		t.Errorf("delta_size = %d, want 100", got)
	}
	if got := info["original_size"].(int64); got != 100 {
		t.Errorf("original_size = %d, want 100", got)
	}

	entry, ok := r.getEntry(0)
	if !ok {
		t.Fatal("getEntry(0) returned false")
	}
	if entry.MkvOffset != 0 {
		t.Errorf("MkvOffset = %d, want 0", entry.MkvOffset)
	}
	if entry.Length != 100 {
		t.Errorf("Length = %d, want 100", entry.Length)
	}
	if entry.Source != 0 {
		t.Errorf("Source = %d, want 0", entry.Source)
	}
	if entry.SourceOffset != 0 {
		t.Errorf("SourceOffset = %d, want 0", entry.SourceOffset)
	}
}

func TestWriter_RoundTrip_SourceOnly(t *testing.T) {
	dir := t.TempDir()

	path := writeTestDedupFile(t, dir, writeTestOptions{
		originalSize:     500,
		originalChecksum: 0x5678,
		sourceType:       source.TypeBluray,
		result: &matcher.Result{
			Entries: []matcher.Entry{
				{MkvOffset: 0, Length: 500, Source: 1, SourceOffset: 1000},
			},
			MatchedBytes:   500,
			MatchedPackets: 1,
			TotalPackets:   1,
		},
	})

	r, err := NewReaderLazy(path, dir)
	if err != nil {
		t.Fatalf("NewReaderLazy: %v", err)
	}
	defer r.Close()

	info := r.Info()
	if got := info["entry_count"].(int); got != 1 {
		t.Errorf("entry_count = %d, want 1", got)
	}
	if got := info["delta_size"].(int64); got != 0 {
		t.Errorf("delta_size = %d, want 0", got)
	}

	entry, ok := r.getEntry(0)
	if !ok {
		t.Fatal("getEntry(0) returned false")
	}
	if entry.MkvOffset != 0 {
		t.Errorf("MkvOffset = %d, want 0", entry.MkvOffset)
	}
	if entry.Length != 500 {
		t.Errorf("Length = %d, want 500", entry.Length)
	}
	if entry.Source != 1 {
		t.Errorf("Source = %d, want 1", entry.Source)
	}
	if entry.SourceOffset != 1000 {
		t.Errorf("SourceOffset = %d, want 1000", entry.SourceOffset)
	}
}

func TestWriter_RoundTrip_Mixed(t *testing.T) {
	dir := t.TempDir()
	// Delta regions: [0,50) and [150,200) = 100 bytes total
	deltaData := bytes.Repeat([]byte{0xCD}, 100)

	path := writeTestDedupFile(t, dir, writeTestOptions{
		originalSize:     200,
		originalChecksum: 0xAAAA,
		sourceType:       source.TypeDVD,
		result: &matcher.Result{
			Entries: []matcher.Entry{
				{MkvOffset: 0, Length: 50, Source: 0, SourceOffset: 0},
				{MkvOffset: 50, Length: 100, Source: 1, SourceOffset: 2000},
				{MkvOffset: 150, Length: 50, Source: 0, SourceOffset: 50},
			},
			DeltaData:      deltaData,
			MatchedBytes:   100,
			UnmatchedBytes: 100,
			MatchedPackets: 1,
			TotalPackets:   3,
		},
	})

	r, err := NewReaderLazy(path, dir)
	if err != nil {
		t.Fatalf("NewReaderLazy: %v", err)
	}
	defer r.Close()

	if got := r.EntryCount(); got != 3 {
		t.Fatalf("EntryCount = %d, want 3", got)
	}

	info := r.Info()
	if got := info["delta_size"].(int64); got != 100 {
		t.Errorf("delta_size = %d, want 100", got)
	}

	// Entry 0: delta [0,50)
	e0, ok := r.getEntry(0)
	if !ok {
		t.Fatal("getEntry(0) returned false")
	}
	if e0.MkvOffset != 0 || e0.Length != 50 || e0.Source != 0 || e0.SourceOffset != 0 {
		t.Errorf("entry 0 = %+v, want MkvOffset=0, Length=50, Source=0, SourceOffset=0", e0)
	}

	// Entry 1: source [50,150)
	e1, ok := r.getEntry(1)
	if !ok {
		t.Fatal("getEntry(1) returned false")
	}
	if e1.MkvOffset != 50 || e1.Length != 100 || e1.Source != 1 || e1.SourceOffset != 2000 {
		t.Errorf("entry 1 = %+v, want MkvOffset=50, Length=100, Source=1, SourceOffset=2000", e1)
	}

	// Entry 2: delta [150,200)
	e2, ok := r.getEntry(2)
	if !ok {
		t.Fatal("getEntry(2) returned false")
	}
	if e2.MkvOffset != 150 || e2.Length != 50 || e2.Source != 0 || e2.SourceOffset != 50 {
		t.Errorf("entry 2 = %+v, want MkvOffset=150, Length=50, Source=0, SourceOffset=50", e2)
	}
}

func TestWriter_RoundTrip_SourceFiles(t *testing.T) {
	dir := t.TempDir()

	sourceFiles := []source.File{
		{RelativePath: "VIDEO_TS/VTS_01_1.VOB", Size: 1073741824, Checksum: 0x1111111111111111},
		{RelativePath: "VIDEO_TS/VTS_01_2.VOB", Size: 1073741825, Checksum: 0x2222222222222222},
		{RelativePath: "VIDEO_TS/VTS_01_3.VOB", Size: 536870912, Checksum: 0x3333333333333333},
	}

	path := writeTestDedupFile(t, dir, writeTestOptions{
		originalSize:     100,
		originalChecksum: 0xBBBB,
		sourceType:       source.TypeDVD,
		sourceFiles:      sourceFiles,
		result: &matcher.Result{
			Entries: []matcher.Entry{
				{MkvOffset: 0, Length: 100, Source: 0, SourceOffset: 0},
			},
			DeltaData: bytes.Repeat([]byte{0x00}, 100),
		},
	})

	r, err := NewReaderLazy(path, dir)
	if err != nil {
		t.Fatalf("NewReaderLazy: %v", err)
	}
	defer r.Close()

	gotFiles := r.SourceFiles()
	if len(gotFiles) != 3 {
		t.Fatalf("SourceFiles count = %d, want 3", len(gotFiles))
	}

	for i, want := range sourceFiles {
		got := gotFiles[i]
		if got.RelativePath != want.RelativePath {
			t.Errorf("SourceFiles[%d].RelativePath = %q, want %q", i, got.RelativePath, want.RelativePath)
		}
		if got.Size != want.Size {
			t.Errorf("SourceFiles[%d].Size = %d, want %d", i, got.Size, want.Size)
		}
		if got.Checksum != want.Checksum {
			t.Errorf("SourceFiles[%d].Checksum = %d, want %d", i, got.Checksum, want.Checksum)
		}
	}
}

func TestWriter_RoundTrip_SourceTypes(t *testing.T) {
	tests := []struct {
		name       string
		sourceType source.Type
		wantType   uint8
	}{
		{"DVD", source.TypeDVD, SourceTypeDVD},
		{"Bluray", source.TypeBluray, SourceTypeBluray},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeTestDedupFile(t, dir, writeTestOptions{
				originalSize:     100,
				originalChecksum: 0xCCCC,
				sourceType:       tt.sourceType,
				result: &matcher.Result{
					Entries: []matcher.Entry{
						{MkvOffset: 0, Length: 100, Source: 0, SourceOffset: 0},
					},
					DeltaData: bytes.Repeat([]byte{0x00}, 100),
				},
			})

			r, err := NewReaderLazy(path, dir)
			if err != nil {
				t.Fatalf("NewReaderLazy: %v", err)
			}
			defer r.Close()

			info := r.Info()
			if got := info["source_type"].(uint8); got != tt.wantType {
				t.Errorf("source_type = %d, want %d", got, tt.wantType)
			}
		})
	}
}

func TestWriter_RoundTrip_LargeEntryCount(t *testing.T) {
	dir := t.TempDir()

	const numEntries = 1000
	entries := make([]matcher.Entry, numEntries)
	for i := range numEntries {
		entries[i] = matcher.Entry{
			MkvOffset:    int64(i) * 100,
			Length:       100,
			Source:       1,
			SourceOffset: int64(i) * 100,
		}
	}

	path := writeTestDedupFile(t, dir, writeTestOptions{
		originalSize:     int64(numEntries) * 100,
		originalChecksum: 0xDDDD,
		sourceType:       source.TypeBluray,
		result: &matcher.Result{
			Entries:      entries,
			MatchedBytes: int64(numEntries) * 100,
		},
	})

	r, err := NewReaderLazy(path, dir)
	if err != nil {
		t.Fatalf("NewReaderLazy: %v", err)
	}
	defer r.Close()

	if got := r.EntryCount(); got != numEntries {
		t.Errorf("EntryCount = %d, want %d", got, numEntries)
	}
}

func TestWriter_RoundTrip_VerifyIntegrity(t *testing.T) {
	dir := t.TempDir()
	deltaData := bytes.Repeat([]byte{0xEF}, 256)

	path := writeTestDedupFile(t, dir, writeTestOptions{
		originalSize:     1000,
		originalChecksum: 0xEEEE,
		sourceType:       source.TypeDVD,
		result: &matcher.Result{
			Entries: []matcher.Entry{
				{MkvOffset: 0, Length: 256, Source: 0, SourceOffset: 0},
				{MkvOffset: 256, Length: 744, Source: 1, SourceOffset: 0},
			},
			DeltaData:      deltaData,
			MatchedBytes:   744,
			UnmatchedBytes: 256,
		},
	})

	r, err := NewReaderLazy(path, dir)
	if err != nil {
		t.Fatalf("NewReaderLazy: %v", err)
	}
	defer r.Close()

	if err := r.VerifyIntegrity(); err != nil {
		t.Errorf("VerifyIntegrity failed: %v", err)
	}
}

func TestWriter_RoundTrip_OriginalSizeAndChecksum(t *testing.T) {
	dir := t.TempDir()

	const wantSize int64 = 12345678
	const wantChecksum uint64 = 0xDEADBEEFCAFE

	path := writeTestDedupFile(t, dir, writeTestOptions{
		originalSize:     wantSize,
		originalChecksum: wantChecksum,
		sourceType:       source.TypeBluray,
		result: &matcher.Result{
			Entries: []matcher.Entry{
				{MkvOffset: 0, Length: wantSize, Source: 0, SourceOffset: 0},
			},
			DeltaData: bytes.Repeat([]byte{0x00}, int(wantSize)),
		},
	})

	r, err := NewReaderLazy(path, dir)
	if err != nil {
		t.Fatalf("NewReaderLazy: %v", err)
	}
	defer r.Close()

	info := r.Info()
	if got := info["original_size"].(int64); got != wantSize {
		t.Errorf("original_size = %d, want %d", got, wantSize)
	}
	if got := info["original_checksum"].(uint64); got != wantChecksum {
		t.Errorf("original_checksum = %#x, want %#x", got, wantChecksum)
	}
}

func TestWriter_RoundTrip_V7_CreatorVersion(t *testing.T) {
	dir := t.TempDir()
	deltaData := bytes.Repeat([]byte{0xAB}, 100)

	path := writeTestDedupFile(t, dir, writeTestOptions{
		originalSize:     100,
		originalChecksum: 0x1234,
		sourceType:       source.TypeDVD,
		creatorVersion:   "mkvdup 0.9.0-canary.13",
		result: &matcher.Result{
			Entries: []matcher.Entry{
				{MkvOffset: 0, Length: 100, Source: 0, SourceOffset: 0},
			},
			DeltaData:      deltaData,
			UnmatchedBytes: 100,
			TotalPackets:   1,
		},
	})

	r, err := NewReaderLazy(path, dir)
	if err != nil {
		t.Fatalf("NewReaderLazy: %v", err)
	}
	defer r.Close()

	info := r.Info()
	if got := info["version"].(uint32); got != VersionUsed {
		t.Errorf("version = %d, want %d", got, VersionUsed)
	}
	if got := info["creator_version"].(string); got != "mkvdup 0.9.0-canary.13" {
		t.Errorf("creator_version = %q, want %q", got, "mkvdup 0.9.0-canary.13")
	}
	if got := info["entry_count"].(int); got != 1 {
		t.Errorf("entry_count = %d, want 1", got)
	}

	// Verify entries are still readable
	entry, ok := r.getEntry(0)
	if !ok {
		t.Fatal("getEntry(0) returned false")
	}
	if entry.MkvOffset != 0 || entry.Length != 100 || entry.Source != 0 {
		t.Errorf("entry = %+v, want MkvOffset=0, Length=100, Source=0", entry)
	}

	// Verify integrity
	if err := r.VerifyIntegrity(); err != nil {
		t.Errorf("VerifyIntegrity failed: %v", err)
	}
}

func TestWriter_RoundTrip_V3_NoCreatorVersion(t *testing.T) {
	dir := t.TempDir()
	deltaData := bytes.Repeat([]byte{0xAB}, 100)

	// No creatorVersion set — should still produce V5 (writer always does)
	// But the reader should handle V3 files with no creator version
	path := writeTestDedupFile(t, dir, writeTestOptions{
		originalSize:     100,
		originalChecksum: 0x1234,
		sourceType:       source.TypeDVD,
		result: &matcher.Result{
			Entries: []matcher.Entry{
				{MkvOffset: 0, Length: 100, Source: 0, SourceOffset: 0},
			},
			DeltaData:      deltaData,
			UnmatchedBytes: 100,
			TotalPackets:   1,
		},
	})

	r, err := NewReaderLazy(path, dir)
	if err != nil {
		t.Fatalf("NewReaderLazy: %v", err)
	}
	defer r.Close()

	info := r.Info()
	// Writer without SetCreatorVersion still produces V5, but with empty string
	if got := info["creator_version"].(string); got != "" {
		t.Errorf("creator_version = %q, want empty", got)
	}
}

func TestWriter_RoundTrip_V5_VerifyIntegrity(t *testing.T) {
	dir := t.TempDir()
	deltaData := bytes.Repeat([]byte{0xEF}, 256)

	path := writeTestDedupFile(t, dir, writeTestOptions{
		originalSize:     1000,
		originalChecksum: 0xEEEE,
		sourceType:       source.TypeDVD,
		creatorVersion:   "mkvdup test-version",
		result: &matcher.Result{
			Entries: []matcher.Entry{
				{MkvOffset: 0, Length: 256, Source: 0, SourceOffset: 0},
				{MkvOffset: 256, Length: 744, Source: 1, SourceOffset: 0},
			},
			DeltaData:      deltaData,
			MatchedBytes:   744,
			UnmatchedBytes: 256,
		},
	})

	r, err := NewReaderLazy(path, dir)
	if err != nil {
		t.Fatalf("NewReaderLazy: %v", err)
	}
	defer r.Close()

	if err := r.VerifyIntegrity(); err != nil {
		t.Errorf("VerifyIntegrity failed: %v", err)
	}
}

func TestWriter_RoundTrip_V5_LargeEntryCount(t *testing.T) {
	dir := t.TempDir()

	const numEntries = 1000
	entries := make([]matcher.Entry, numEntries)
	for i := range numEntries {
		entries[i] = matcher.Entry{
			MkvOffset:    int64(i) * 100,
			Length:       100,
			Source:       1,
			SourceOffset: int64(i) * 100,
		}
	}

	path := writeTestDedupFile(t, dir, writeTestOptions{
		originalSize:     int64(numEntries) * 100,
		originalChecksum: 0xDDDD,
		sourceType:       source.TypeBluray,
		creatorVersion:   "mkvdup 1.0.0",
		result: &matcher.Result{
			Entries:      entries,
			MatchedBytes: int64(numEntries) * 100,
		},
	})

	r, err := NewReaderLazy(path, dir)
	if err != nil {
		t.Fatalf("NewReaderLazy: %v", err)
	}
	defer r.Close()

	if got := r.EntryCount(); got != numEntries {
		t.Errorf("EntryCount = %d, want %d", got, numEntries)
	}

	// Spot check a few entries
	for _, idx := range []int{0, 500, 999} {
		entry, ok := r.getEntry(idx)
		if !ok {
			t.Errorf("getEntry(%d) returned false", idx)
			continue
		}
		if entry.MkvOffset != int64(idx)*100 {
			t.Errorf("entry %d MkvOffset = %d, want %d", idx, entry.MkvOffset, int64(idx)*100)
		}
	}
}
