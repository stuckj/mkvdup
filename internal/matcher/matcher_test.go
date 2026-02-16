package matcher

import (
	"bytes"
	"io"
	"testing"

	"github.com/stuckj/mkvdup/internal/source"
)

func TestNewMatcher(t *testing.T) {
	// Create a minimal source index
	idx := &source.Index{
		WindowSize:      64,
		HashToLocations: make(map[uint64][]source.Location),
		SourceDir:       "/test/dir",
		SourceType:      source.TypeDVD,
	}

	matcher, err := NewMatcher(idx)
	if err != nil {
		t.Fatalf("NewMatcher() error = %v", err)
	}
	defer matcher.Close()

	if matcher == nil {
		t.Fatal("NewMatcher() returned nil")
	}
	if matcher.windowSize != 64 {
		t.Errorf("windowSize = %d, want 64", matcher.windowSize)
	}
}

func TestMatcher_SetNumWorkers(t *testing.T) {
	idx := &source.Index{
		WindowSize:      64,
		HashToLocations: make(map[uint64][]source.Location),
	}

	matcher, err := NewMatcher(idx)
	if err != nil {
		t.Fatalf("NewMatcher() error = %v", err)
	}
	defer matcher.Close()

	// Set to specific value
	matcher.SetNumWorkers(4)
	if matcher.numWorkers != 4 {
		t.Errorf("numWorkers = %d, want 4", matcher.numWorkers)
	}

	// Set to zero should clamp to 1
	matcher.SetNumWorkers(0)
	if matcher.numWorkers != 1 {
		t.Errorf("numWorkers = %d, want 1 (clamped)", matcher.numWorkers)
	}

	// Set to negative should clamp to 1
	matcher.SetNumWorkers(-5)
	if matcher.numWorkers != 1 {
		t.Errorf("numWorkers = %d, want 1 (clamped)", matcher.numWorkers)
	}
}

func TestMatcher_Close(t *testing.T) {
	idx := &source.Index{
		WindowSize:      64,
		HashToLocations: make(map[uint64][]source.Location),
	}

	matcher, err := NewMatcher(idx)
	if err != nil {
		t.Fatalf("NewMatcher() error = %v", err)
	}

	// Close should not error
	if err := matcher.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Close again should be safe (idempotent)
	if err := matcher.Close(); err != nil {
		t.Errorf("Second Close() error = %v", err)
	}
}

func TestMaxExpansionBytes(t *testing.T) {
	// Verify MaxExpansionBytes is reasonable
	if MaxExpansionBytes <= 0 {
		t.Errorf("MaxExpansionBytes = %d, should be positive", MaxExpansionBytes)
	}
	// Should be at least 1MB for video keyframes
	if MaxExpansionBytes < 1024*1024 {
		t.Errorf("MaxExpansionBytes = %d, should be at least 1MB", MaxExpansionBytes)
	}
}

func TestProbeHash_Fields(t *testing.T) {
	// Test that ProbeHash struct has expected fields
	h := ProbeHash{
		Hash:    12345,
		IsVideo: true,
	}

	if h.Hash != 12345 {
		t.Errorf("Hash = %d, want 12345", h.Hash)
	}
	if !h.IsVideo {
		t.Error("IsVideo should be true")
	}
}

func TestEntry_Fields(t *testing.T) {
	// Test that Entry struct has expected fields
	e := Entry{
		MkvOffset:        1000,
		Length:           500,
		Source:           1,
		SourceOffset:     2000,
		IsVideo:          true,
		AudioSubStreamID: 0x80,
	}

	if e.MkvOffset != 1000 {
		t.Errorf("MkvOffset = %d, want 1000", e.MkvOffset)
	}
	if e.Length != 500 {
		t.Errorf("Length = %d, want 500", e.Length)
	}
	if e.Source != 1 {
		t.Errorf("Source = %d, want 1", e.Source)
	}
	if e.SourceOffset != 2000 {
		t.Errorf("SourceOffset = %d, want 2000", e.SourceOffset)
	}
	if !e.IsVideo {
		t.Error("IsVideo should be true")
	}
	if e.AudioSubStreamID != 0x80 {
		t.Errorf("AudioSubStreamID = 0x%02X, want 0x80", e.AudioSubStreamID)
	}
}

func TestResult_Fields(t *testing.T) {
	// Test that Result struct has expected fields
	r := Result{
		Entries:        []Entry{{MkvOffset: 0, Length: 100}},
		DeltaData:      []byte{1, 2, 3},
		MatchedBytes:   1000,
		UnmatchedBytes: 100,
		MatchedPackets: 50,
		TotalPackets:   60,
	}

	if len(r.Entries) != 1 {
		t.Errorf("Entries len = %d, want 1", len(r.Entries))
	}
	if len(r.DeltaData) != 3 {
		t.Errorf("DeltaData len = %d, want 3", len(r.DeltaData))
	}
	if r.MatchedBytes != 1000 {
		t.Errorf("MatchedBytes = %d, want 1000", r.MatchedBytes)
	}
	if r.UnmatchedBytes != 100 {
		t.Errorf("UnmatchedBytes = %d, want 100", r.UnmatchedBytes)
	}
	if r.MatchedPackets != 50 {
		t.Errorf("MatchedPackets = %d, want 50", r.MatchedPackets)
	}
	if r.TotalPackets != 60 {
		t.Errorf("TotalPackets = %d, want 60", r.TotalPackets)
	}
}

func newTestMatcherWithRegions(mkvSize int64, data []byte, regions []matchedRegion) *Matcher {
	return &Matcher{
		mkvSize:        mkvSize,
		mkvData:        data,
		matchedRegions: regions,
	}
}

func TestMergeRegions(t *testing.T) {
	tests := []struct {
		name     string
		regions  []matchedRegion
		expected []matchedRegion
	}{
		{
			name:     "empty",
			regions:  nil,
			expected: nil,
		},
		{
			name: "single region",
			regions: []matchedRegion{
				{mkvStart: 0, mkvEnd: 100, fileIndex: 0, srcOffset: 0},
			},
			expected: []matchedRegion{
				{mkvStart: 0, mkvEnd: 100, fileIndex: 0, srcOffset: 0},
			},
		},
		{
			name: "non-overlapping",
			regions: []matchedRegion{
				{mkvStart: 0, mkvEnd: 100, fileIndex: 0, srcOffset: 0},
				{mkvStart: 200, mkvEnd: 300, fileIndex: 0, srcOffset: 0},
			},
			expected: []matchedRegion{
				{mkvStart: 0, mkvEnd: 100, fileIndex: 0, srcOffset: 0},
				{mkvStart: 200, mkvEnd: 300, fileIndex: 0, srcOffset: 0},
			},
		},
		{
			name: "adjacent touching same source",
			regions: []matchedRegion{
				{mkvStart: 0, mkvEnd: 100, fileIndex: 0, srcOffset: 0},
				{mkvStart: 100, mkvEnd: 200, fileIndex: 0, srcOffset: 100},
			},
			expected: []matchedRegion{
				// Adjacent regions are not overlapping (curr.mkvStart >= last.mkvEnd),
				// so they remain separate even with consistent source mapping.
				{mkvStart: 0, mkvEnd: 100, fileIndex: 0, srcOffset: 0},
				{mkvStart: 100, mkvEnd: 200, fileIndex: 0, srcOffset: 100},
			},
		},
		{
			name: "fully contained same source",
			regions: []matchedRegion{
				{mkvStart: 0, mkvEnd: 300, fileIndex: 0, srcOffset: 0},
				{mkvStart: 50, mkvEnd: 150, fileIndex: 0, srcOffset: 50},
			},
			expected: []matchedRegion{
				{mkvStart: 0, mkvEnd: 300, fileIndex: 0, srcOffset: 0},
			},
		},
		{
			name: "fully contained different source",
			regions: []matchedRegion{
				{mkvStart: 0, mkvEnd: 300, fileIndex: 0, srcOffset: 0},
				{mkvStart: 50, mkvEnd: 150, fileIndex: 1, srcOffset: 500},
			},
			expected: []matchedRegion{
				// curr is fully contained in last, so it's dropped
				{mkvStart: 0, mkvEnd: 300, fileIndex: 0, srcOffset: 0},
			},
		},
		{
			name: "overlap different file clips later region",
			regions: []matchedRegion{
				{mkvStart: 0, mkvEnd: 100, fileIndex: 0, srcOffset: 0},
				{mkvStart: 50, mkvEnd: 250, fileIndex: 1, srcOffset: 500},
			},
			expected: []matchedRegion{
				// Earlier region keeps priority, later region is clipped
				{mkvStart: 0, mkvEnd: 100, fileIndex: 0, srcOffset: 0},
				{mkvStart: 100, mkvEnd: 250, fileIndex: 1, srcOffset: 550},
			},
		},
		{
			name: "overlap same source consistent offsets merges",
			regions: []matchedRegion{
				{mkvStart: 0, mkvEnd: 200, fileIndex: 0, srcOffset: 1000},
				{mkvStart: 100, mkvEnd: 300, fileIndex: 0, srcOffset: 1100},
			},
			expected: []matchedRegion{
				// Same source, consistent mapping: merge into one
				{mkvStart: 0, mkvEnd: 300, fileIndex: 0, srcOffset: 1000},
			},
		},
		{
			name: "overlap different source clips shorter",
			regions: []matchedRegion{
				{mkvStart: 0, mkvEnd: 200, fileIndex: 0, srcOffset: 0},
				{mkvStart: 100, mkvEnd: 250, fileIndex: 1, srcOffset: 500},
			},
			expected: []matchedRegion{
				// Earlier region keeps priority, later region is clipped
				{mkvStart: 0, mkvEnd: 200, fileIndex: 0, srcOffset: 0},
				{mkvStart: 200, mkvEnd: 250, fileIndex: 1, srcOffset: 600},
			},
		},
		{
			name: "unsorted input same source consistent offsets",
			regions: []matchedRegion{
				{mkvStart: 200, mkvEnd: 300, fileIndex: 0, srcOffset: 200},
				{mkvStart: 0, mkvEnd: 150, fileIndex: 0, srcOffset: 0},
				{mkvStart: 100, mkvEnd: 250, fileIndex: 0, srcOffset: 100},
			},
			expected: []matchedRegion{
				// After sort: [0,150) src=0, [100,250) src=100, [200,300) src=200
				// [0,150) + [100,250): same source, consistent (0+100=100) → merge [0,250)
				// [0,250) + [200,300): same source, consistent (0+200=200) → merge [0,300)
				{mkvStart: 0, mkvEnd: 300, fileIndex: 0, srcOffset: 0},
			},
		},
		{
			name: "multiple non-overlapping",
			regions: []matchedRegion{
				{mkvStart: 0, mkvEnd: 50, fileIndex: 0, srcOffset: 0},
				{mkvStart: 100, mkvEnd: 150, fileIndex: 0, srcOffset: 0},
				{mkvStart: 200, mkvEnd: 250, fileIndex: 0, srcOffset: 0},
			},
			expected: []matchedRegion{
				{mkvStart: 0, mkvEnd: 50, fileIndex: 0, srcOffset: 0},
				{mkvStart: 100, mkvEnd: 150, fileIndex: 0, srcOffset: 0},
				{mkvStart: 200, mkvEnd: 250, fileIndex: 0, srcOffset: 0},
			},
		},
		{
			name: "source fields preserved",
			regions: []matchedRegion{
				{mkvStart: 0, mkvEnd: 100, fileIndex: 1, srcOffset: 500, isVideo: true, audioSubStreamID: 0x00},
				{mkvStart: 200, mkvEnd: 400, fileIndex: 3, srcOffset: 8000, isVideo: false, audioSubStreamID: 0x80},
			},
			expected: []matchedRegion{
				{mkvStart: 0, mkvEnd: 100, fileIndex: 1, srcOffset: 500, isVideo: true, audioSubStreamID: 0x00},
				{mkvStart: 200, mkvEnd: 400, fileIndex: 3, srcOffset: 8000, isVideo: false, audioSubStreamID: 0x80},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMatcherWithRegions(1000, nil, tt.regions)
			m.mergeRegions()

			if len(m.matchedRegions) != len(tt.expected) {
				t.Fatalf("got %d regions, want %d", len(m.matchedRegions), len(tt.expected))
			}

			for i, got := range m.matchedRegions {
				want := tt.expected[i]
				if got.mkvStart != want.mkvStart {
					t.Errorf("region[%d].mkvStart = %d, want %d", i, got.mkvStart, want.mkvStart)
				}
				if got.mkvEnd != want.mkvEnd {
					t.Errorf("region[%d].mkvEnd = %d, want %d", i, got.mkvEnd, want.mkvEnd)
				}
				if got.fileIndex != want.fileIndex {
					t.Errorf("region[%d].fileIndex = %d, want %d", i, got.fileIndex, want.fileIndex)
				}
				if got.srcOffset != want.srcOffset {
					t.Errorf("region[%d].srcOffset = %d, want %d", i, got.srcOffset, want.srcOffset)
				}
				if got.isVideo != want.isVideo {
					t.Errorf("region[%d].isVideo = %v, want %v", i, got.isVideo, want.isVideo)
				}
				if got.audioSubStreamID != want.audioSubStreamID {
					t.Errorf("region[%d].audioSubStreamID = 0x%02X, want 0x%02X", i, got.audioSubStreamID, want.audioSubStreamID)
				}
			}
		})
	}
}

func TestBuildEntries(t *testing.T) {
	tests := []struct {
		name           string
		mkvSize        int64
		regions        []matchedRegion
		wantEntryCount int
		wantEntries    []Entry
		wantDeltaBytes []byte // nil means skip delta content check
	}{
		{
			name:           "all delta",
			mkvSize:        100,
			regions:        nil,
			wantEntryCount: 1,
			wantEntries: []Entry{
				{MkvOffset: 0, Length: 100, Source: 0, SourceOffset: 0},
			},
		},
		{
			name:    "all matched",
			mkvSize: 100,
			regions: []matchedRegion{
				{mkvStart: 0, mkvEnd: 100, fileIndex: 2, srcOffset: 500},
			},
			wantEntryCount: 1,
			wantEntries: []Entry{
				{MkvOffset: 0, Length: 100, Source: 3, SourceOffset: 500},
			},
		},
		{
			name:    "gap match gap",
			mkvSize: 300,
			regions: []matchedRegion{
				{mkvStart: 100, mkvEnd: 200, fileIndex: 0, srcOffset: 1000},
			},
			wantEntryCount: 3,
			wantEntries: []Entry{
				{MkvOffset: 0, Length: 100, Source: 0, SourceOffset: 0},
				{MkvOffset: 100, Length: 100, Source: 1, SourceOffset: 1000},
				{MkvOffset: 200, Length: 100, Source: 0, SourceOffset: 100},
			},
		},
		{
			name:    "match at start",
			mkvSize: 200,
			regions: []matchedRegion{
				{mkvStart: 0, mkvEnd: 100, fileIndex: 1, srcOffset: 0},
			},
			wantEntryCount: 2,
			wantEntries: []Entry{
				{MkvOffset: 0, Length: 100, Source: 2, SourceOffset: 0},
				{MkvOffset: 100, Length: 100, Source: 0, SourceOffset: 0},
			},
		},
		{
			name:    "match at end",
			mkvSize: 200,
			regions: []matchedRegion{
				{mkvStart: 100, mkvEnd: 200, fileIndex: 0, srcOffset: 500},
			},
			wantEntryCount: 2,
			wantEntries: []Entry{
				{MkvOffset: 0, Length: 100, Source: 0, SourceOffset: 0},
				{MkvOffset: 100, Length: 100, Source: 1, SourceOffset: 500},
			},
		},
		{
			name:    "multiple regions",
			mkvSize: 500,
			regions: []matchedRegion{
				{mkvStart: 50, mkvEnd: 150, fileIndex: 0, srcOffset: 0},
				{mkvStart: 250, mkvEnd: 350, fileIndex: 0, srcOffset: 0},
			},
			wantEntryCount: 5,
			wantEntries: []Entry{
				{MkvOffset: 0, Length: 50, Source: 0, SourceOffset: 0},
				{MkvOffset: 50, Length: 100, Source: 1, SourceOffset: 0},
				{MkvOffset: 150, Length: 100, Source: 0, SourceOffset: 50},
				{MkvOffset: 250, Length: 100, Source: 1, SourceOffset: 0},
				{MkvOffset: 350, Length: 150, Source: 0, SourceOffset: 150},
			},
		},
		{
			name:    "source field propagation",
			mkvSize: 300,
			regions: []matchedRegion{
				{mkvStart: 100, mkvEnd: 200, fileIndex: 2, srcOffset: 5000, isVideo: true, audioSubStreamID: 0x80},
			},
			wantEntryCount: 3,
			wantEntries: []Entry{
				{MkvOffset: 0, Length: 100, Source: 0, SourceOffset: 0},
				{MkvOffset: 100, Length: 100, Source: 3, SourceOffset: 5000, IsVideo: true, AudioSubStreamID: 0x80},
				{MkvOffset: 200, Length: 100, Source: 0, SourceOffset: 100},
			},
		},
		{
			name:           "zero length file",
			mkvSize:        0,
			regions:        nil,
			wantEntryCount: 0,
			wantEntries:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mkvData := make([]byte, tt.mkvSize)
			for i := range mkvData {
				mkvData[i] = byte(i % 256)
			}
			m := newTestMatcherWithRegions(tt.mkvSize, mkvData, tt.regions)
			entries, deltaWriter, err := m.buildEntries()
			if err != nil {
				t.Fatalf("buildEntries error: %v", err)
			}
			defer deltaWriter.Close()

			if len(entries) != tt.wantEntryCount {
				t.Fatalf("got %d entries, want %d", len(entries), tt.wantEntryCount)
			}

			for i, got := range entries {
				if i >= len(tt.wantEntries) {
					break
				}
				want := tt.wantEntries[i]
				if got.MkvOffset != want.MkvOffset {
					t.Errorf("entry[%d].MkvOffset = %d, want %d", i, got.MkvOffset, want.MkvOffset)
				}
				if got.Length != want.Length {
					t.Errorf("entry[%d].Length = %d, want %d", i, got.Length, want.Length)
				}
				if got.Source != want.Source {
					t.Errorf("entry[%d].Source = %d, want %d", i, got.Source, want.Source)
				}
				if got.SourceOffset != want.SourceOffset {
					t.Errorf("entry[%d].SourceOffset = %d, want %d", i, got.SourceOffset, want.SourceOffset)
				}
				if got.IsVideo != want.IsVideo {
					t.Errorf("entry[%d].IsVideo = %v, want %v", i, got.IsVideo, want.IsVideo)
				}
				if got.AudioSubStreamID != want.AudioSubStreamID {
					t.Errorf("entry[%d].AudioSubStreamID = 0x%02X, want 0x%02X", i, got.AudioSubStreamID, want.AudioSubStreamID)
				}
			}

			// For "all matched", delta should be empty
			if tt.name == "all matched" && deltaWriter.Size() != 0 {
				t.Errorf("expected empty delta data for all matched, got %d bytes", deltaWriter.Size())
			}
		})
	}
}

func TestBuildEntries_DeltaDataContent(t *testing.T) {
	mkvSize := int64(50)
	mkvData := make([]byte, mkvSize)
	for i := range mkvData {
		mkvData[i] = byte(i % 256)
	}

	regions := []matchedRegion{
		{mkvStart: 20, mkvEnd: 30, fileIndex: 0, srcOffset: 0},
	}
	m := newTestMatcherWithRegions(mkvSize, mkvData, regions)
	_, deltaWriter, err := m.buildEntries()
	if err != nil {
		t.Fatalf("buildEntries error: %v", err)
	}
	defer deltaWriter.Close()

	// Read delta data back from temp file
	f := deltaWriter.File()
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatalf("seek error: %v", err)
	}
	deltaData := make([]byte, deltaWriter.Size())
	if _, err := io.ReadFull(f, deltaData); err != nil {
		t.Fatalf("read delta file: %v", err)
	}

	// Delta should be mkvData[0:20] + mkvData[30:50]
	expectedDelta := make([]byte, 0, 40)
	expectedDelta = append(expectedDelta, mkvData[0:20]...)
	expectedDelta = append(expectedDelta, mkvData[30:50]...)

	if !bytes.Equal(deltaData, expectedDelta) {
		t.Errorf("delta data mismatch: got %d bytes, want %d bytes", len(deltaData), len(expectedDelta))
		for i := 0; i < len(deltaData) && i < len(expectedDelta); i++ {
			if deltaData[i] != expectedDelta[i] {
				t.Errorf("first difference at byte %d: got 0x%02X, want 0x%02X", i, deltaData[i], expectedDelta[i])
				break
			}
		}
	}
}
