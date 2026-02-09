package dedup

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stuckj/mkvdup/internal/matcher"
	"github.com/stuckj/mkvdup/internal/source"
)

// writeTestOptions configures the helper that writes a dedup file.
type writeTestOptions struct {
	originalSize     int64
	originalChecksum uint64
	sourceType       source.Type
	sourceFiles      []source.File
	result           *matcher.Result
	esConverters     []source.ESRangeConverter
	creatorVersion   string
}

// writeTestDedupFile creates a dedup file using the Writer API and returns the path.
func writeTestDedupFile(t *testing.T, dir string, opts writeTestOptions) string {
	t.Helper()
	path := filepath.Join(dir, "test.mkvdup")
	w, err := NewWriter(path)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	defer w.Close()

	w.SetHeader(opts.originalSize, opts.originalChecksum, opts.sourceType)
	if opts.creatorVersion != "" {
		w.SetCreatorVersion(opts.creatorVersion)
	}
	if len(opts.sourceFiles) > 0 {
		w.SetSourceFiles(opts.sourceFiles)
	}
	if opts.result != nil {
		if err := w.SetMatchResult(opts.result, opts.esConverters); err != nil {
			t.Fatalf("SetMatchResult: %v", err)
		}
	}
	if err := w.Write(); err != nil {
		t.Fatalf("Write: %v", err)
	}
	return path
}

// mockESConverter implements source.ESRangeConverter for testing.
type mockESConverter struct {
	videoRanges     []source.RawRange
	audioRanges     []source.RawRange
	videoErr        error
	audioErr        error
	videoCalled     bool
	audioCalled     bool
	lastESOffset    int64
	lastSize        int
	lastIsVideo     bool
	lastSubStreamID byte
}

func (m *mockESConverter) RawRangesForESRegion(esOffset int64, size int, isVideo bool) ([]source.RawRange, error) {
	m.videoCalled = true
	m.lastESOffset = esOffset
	m.lastSize = size
	m.lastIsVideo = isVideo
	if m.videoErr != nil {
		return nil, m.videoErr
	}
	return m.videoRanges, nil
}

func (m *mockESConverter) RawRangesForAudioSubStream(subStreamID byte, esOffset int64, size int) ([]source.RawRange, error) {
	m.audioCalled = true
	m.lastSubStreamID = subStreamID
	m.lastESOffset = esOffset
	m.lastSize = size
	if m.audioErr != nil {
		return nil, m.audioErr
	}
	return m.audioRanges, nil
}

func TestNewWriter_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.mkvdup")
	w, err := NewWriter(path)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestNewWriter_InvalidPath(t *testing.T) {
	_, err := NewWriter("/nonexistent/dir/test.mkvdup")
	if err == nil {
		t.Fatal("expected error for invalid path, got nil")
	}
}

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

func TestWriter_WriteWithProgress(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.mkvdup")

	w, err := NewWriter(path)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	defer w.Close()

	w.SetHeader(1000, 0xFFFF, source.TypeDVD)

	deltaData := bytes.Repeat([]byte{0xAA}, 500)
	result := &matcher.Result{
		Entries: []matcher.Entry{
			{MkvOffset: 0, Length: 500, Source: 0, SourceOffset: 0},
			{MkvOffset: 500, Length: 500, Source: 1, SourceOffset: 0},
		},
		DeltaData:      deltaData,
		MatchedBytes:   500,
		UnmatchedBytes: 500,
	}
	if err := w.SetMatchResult(result, nil); err != nil {
		t.Fatalf("SetMatchResult: %v", err)
	}

	var callCount atomic.Int64
	var lastWritten atomic.Int64
	var lastTotal atomic.Int64

	err = w.WriteWithProgress(func(written, total int64) {
		callCount.Add(1)
		lastWritten.Store(written)
		lastTotal.Store(total)
	})
	if err != nil {
		t.Fatalf("WriteWithProgress: %v", err)
	}

	if callCount.Load() < 1 {
		t.Error("progress callback was never called")
	}
	if lastWritten.Load() <= 0 {
		t.Errorf("last written = %d, want > 0", lastWritten.Load())
	}
	if lastTotal.Load() <= 0 {
		t.Errorf("last total = %d, want > 0", lastTotal.Load())
	}
}

func TestWriter_Close_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.mkvdup")

	w, err := NewWriter(path)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}

	// First close should succeed.
	if err := w.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}

	// Second close should not panic. The underlying os.File.Close will return
	// an error for double close, but the Writer should handle it gracefully.
	// We just verify no panic occurs; an error is acceptable.
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("second Close panicked: %v", r)
			}
		}()
		w.Close() //nolint:errcheck
	}()
}

func TestWriter_ConvertESToRawOffsets_DeltaPassthrough(t *testing.T) {
	dir := t.TempDir()
	deltaData := bytes.Repeat([]byte{0xDD}, 200)

	converter := &mockESConverter{
		videoRanges: []source.RawRange{{FileOffset: 5000, Size: 200}},
	}

	path := writeTestDedupFile(t, dir, writeTestOptions{
		originalSize:     200,
		originalChecksum: 0x1111,
		sourceType:       source.TypeDVD,
		result: &matcher.Result{
			Entries: []matcher.Entry{
				{MkvOffset: 0, Length: 200, Source: 0, SourceOffset: 0},
			},
			DeltaData:      deltaData,
			UnmatchedBytes: 200,
		},
		esConverters: []source.ESRangeConverter{converter},
	})

	r, err := NewReaderLazy(path, dir)
	if err != nil {
		t.Fatalf("NewReaderLazy: %v", err)
	}
	defer r.Close()

	if got := r.EntryCount(); got != 1 {
		t.Fatalf("EntryCount = %d, want 1", got)
	}

	entry, ok := r.getEntry(0)
	if !ok {
		t.Fatal("getEntry(0) returned false")
	}
	// Delta entry (Source=0) should pass through unchanged.
	if entry.Source != 0 {
		t.Errorf("Source = %d, want 0", entry.Source)
	}
	if entry.MkvOffset != 0 {
		t.Errorf("MkvOffset = %d, want 0", entry.MkvOffset)
	}
	if entry.Length != 200 {
		t.Errorf("Length = %d, want 200", entry.Length)
	}
	if entry.SourceOffset != 0 {
		t.Errorf("SourceOffset = %d, want 0", entry.SourceOffset)
	}

	// The converter should not have been called for a delta entry.
	if converter.videoCalled {
		t.Error("converter.RawRangesForESRegion was called for a delta entry")
	}
	if converter.audioCalled {
		t.Error("converter.RawRangesForAudioSubStream was called for a delta entry")
	}
}

func TestWriter_ConvertESToRawOffsets_VideoSplit(t *testing.T) {
	dir := t.TempDir()

	// The converter splits a single video ES entry into 2 raw ranges.
	converter := &mockESConverter{
		videoRanges: []source.RawRange{
			{FileOffset: 10000, Size: 300},
			{FileOffset: 20000, Size: 200},
		},
	}

	path := writeTestDedupFile(t, dir, writeTestOptions{
		originalSize:     500,
		originalChecksum: 0x2222,
		sourceType:       source.TypeDVD,
		result: &matcher.Result{
			Entries: []matcher.Entry{
				{MkvOffset: 0, Length: 500, Source: 1, SourceOffset: 5000, IsVideo: true},
			},
			MatchedBytes: 500,
		},
		// Source index in entry is 1 (1-based), so converter index is 0.
		esConverters: []source.ESRangeConverter{converter},
	})

	r, err := NewReaderLazy(path, dir)
	if err != nil {
		t.Fatalf("NewReaderLazy: %v", err)
	}
	defer r.Close()

	// The single entry should have been split into 2.
	if got := r.EntryCount(); got != 2 {
		t.Fatalf("EntryCount = %d, want 2", got)
	}

	// First split entry: MkvOffset=0, Length=300, SourceOffset=10000
	e0, ok := r.getEntry(0)
	if !ok {
		t.Fatal("getEntry(0) returned false")
	}
	if e0.MkvOffset != 0 {
		t.Errorf("entry 0 MkvOffset = %d, want 0", e0.MkvOffset)
	}
	if e0.Length != 300 {
		t.Errorf("entry 0 Length = %d, want 300", e0.Length)
	}
	if e0.Source != 1 {
		t.Errorf("entry 0 Source = %d, want 1", e0.Source)
	}
	if e0.SourceOffset != 10000 {
		t.Errorf("entry 0 SourceOffset = %d, want 10000", e0.SourceOffset)
	}
	if !e0.IsVideo {
		t.Error("entry 0 IsVideo = false, want true")
	}

	// Second split entry: MkvOffset=300, Length=200, SourceOffset=20000
	e1, ok := r.getEntry(1)
	if !ok {
		t.Fatal("getEntry(1) returned false")
	}
	if e1.MkvOffset != 300 {
		t.Errorf("entry 1 MkvOffset = %d, want 300", e1.MkvOffset)
	}
	if e1.Length != 200 {
		t.Errorf("entry 1 Length = %d, want 200", e1.Length)
	}
	if e1.Source != 1 {
		t.Errorf("entry 1 Source = %d, want 1", e1.Source)
	}
	if e1.SourceOffset != 20000 {
		t.Errorf("entry 1 SourceOffset = %d, want 20000", e1.SourceOffset)
	}
	if !e1.IsVideo {
		t.Error("entry 1 IsVideo = false, want true")
	}
}

func TestWriter_ConvertESToRawOffsets_Error(t *testing.T) {
	converter := &mockESConverter{
		videoErr: fmt.Errorf("simulated conversion failure"),
	}

	w, err := NewWriter(filepath.Join(t.TempDir(), "test.mkvdup"))
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	defer w.Close()

	w.SetHeader(500, 0x3333, source.TypeDVD)

	result := &matcher.Result{
		Entries: []matcher.Entry{
			{MkvOffset: 0, Length: 500, Source: 1, SourceOffset: 5000, IsVideo: true},
		},
		MatchedBytes: 500,
	}

	err = w.SetMatchResult(result, []source.ESRangeConverter{converter})
	if err == nil {
		t.Fatal("expected error from SetMatchResult, got nil")
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

func TestWriter_UsedFlags_SomeUnused(t *testing.T) {
	dir := t.TempDir()

	// Create source files: 3 files, but only files 0 and 2 are referenced
	sourceFiles := []source.File{
		{RelativePath: "VIDEO_TS/VTS_01_1.VOB", Size: 1000, Checksum: 0x111},
		{RelativePath: "VIDEO_TS/VTS_02_1.VOB", Size: 2000, Checksum: 0x222},
		{RelativePath: "VIDEO_TS/VTS_03_1.VOB", Size: 3000, Checksum: 0x333},
	}

	path := writeTestDedupFile(t, dir, writeTestOptions{
		originalSize:     500,
		originalChecksum: 0xABCD,
		sourceType:       source.TypeDVD,
		creatorVersion:   "mkvdup test",
		sourceFiles:      sourceFiles,
		result: &matcher.Result{
			Entries: []matcher.Entry{
				{MkvOffset: 0, Length: 200, Source: 1, SourceOffset: 0},   // file 0 (used)
				{MkvOffset: 200, Length: 300, Source: 3, SourceOffset: 0}, // file 2 (used)
			},
			TotalPackets: 2,
		},
	})

	r, err := NewReaderLazy(path, dir)
	if err != nil {
		t.Fatalf("NewReaderLazy: %v", err)
	}
	defer r.Close()

	if !r.HasSourceUsedFlags() {
		t.Fatal("HasSourceUsedFlags should be true for V7")
	}

	sfs := r.SourceFiles()
	if len(sfs) != 3 {
		t.Fatalf("SourceFiles count = %d, want 3", len(sfs))
	}
	if !sfs[0].Used {
		t.Error("source file 0 should be used")
	}
	if sfs[1].Used {
		t.Error("source file 1 should be unused")
	}
	if !sfs[2].Used {
		t.Error("source file 2 should be used")
	}

	if err := r.VerifyIntegrity(); err != nil {
		t.Errorf("VerifyIntegrity failed: %v", err)
	}
}

func TestWriter_UsedFlags_AllDelta(t *testing.T) {
	dir := t.TempDir()
	deltaData := bytes.Repeat([]byte{0xCD}, 100)

	sourceFiles := []source.File{
		{RelativePath: "VIDEO_TS/VTS_01_1.VOB", Size: 1000, Checksum: 0x111},
	}

	path := writeTestDedupFile(t, dir, writeTestOptions{
		originalSize:     100,
		originalChecksum: 0x1234,
		sourceType:       source.TypeDVD,
		creatorVersion:   "mkvdup test",
		sourceFiles:      sourceFiles,
		result: &matcher.Result{
			Entries: []matcher.Entry{
				{MkvOffset: 0, Length: 100, Source: 0, SourceOffset: 0}, // all delta
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

	sfs := r.SourceFiles()
	if sfs[0].Used {
		t.Error("source file 0 should be unused (all delta)")
	}
}

func TestWriter_UsedFlags_V3_NoFlags(t *testing.T) {
	dir := t.TempDir()
	deltaData := bytes.Repeat([]byte{0xAB}, 100)

	// V3: no creator version
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

	if r.HasSourceUsedFlags() {
		t.Error("HasSourceUsedFlags should be false for V3")
	}
}
