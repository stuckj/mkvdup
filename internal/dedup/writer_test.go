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
