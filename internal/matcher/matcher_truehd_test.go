package matcher

import (
	"testing"

	"github.com/stuckj/mkvdup/internal/source"
)

// mockAudioESReader implements source.ESReader backed by in-memory data
// keyed by audio sub-stream ID. Only ReadAudioSubStreamData is functional;
// all other methods return zero values.
type mockAudioESReader struct {
	audioData map[byte][]byte
}

func (r *mockAudioESReader) ReadESData(int64, int, bool) ([]byte, error)   { return nil, nil }
func (r *mockAudioESReader) ESOffsetToFileOffset(int64, bool) (int64, int) { return 0, 0 }
func (r *mockAudioESReader) TotalESSize(bool) int64                        { return 0 }
func (r *mockAudioESReader) AudioSubStreams() []byte                       { return nil }
func (r *mockAudioESReader) AudioSubStreamESSize(byte) int64               { return 0 }

func (r *mockAudioESReader) ReadAudioSubStreamData(subStreamID byte, esOffset int64, size int) ([]byte, error) {
	data := r.audioData[subStreamID]
	if esOffset >= int64(len(data)) {
		return nil, nil
	}
	end := esOffset + int64(size)
	if end > int64(len(data)) {
		end = int64(len(data))
	}
	return data[esOffset:end], nil
}

func TestFillTrueHDGapSegments(t *testing.T) {
	const subStreamID = 0x02
	const windowSize = 64

	t.Run("basic matching run", func(t *testing.T) {
		// Source and MKV contain identical data.
		srcData := make([]byte, 64)
		for i := range srcData {
			srcData[i] = byte(i + 1)
		}
		mkvData := make([]byte, 64)
		copy(mkvData, srcData)

		idx := source.NewIndex("/test", source.TypeBluray, windowSize)
		idx.ESReaders = []source.ESReader{&mockAudioESReader{
			audioData: map[byte][]byte{subStreamID: srcData},
		}}

		m := &Matcher{
			sourceIndex: idx,
			mkvData:     mkvData,
			mkvSize:     int64(len(mkvData)),
		}

		segments := []mkvSegment{{0, int64(len(mkvData))}}
		regions := m.fillTrueHDGapSegments(segments, 0, int64(len(srcData)), 0, subStreamID)

		if len(regions) != 1 {
			t.Fatalf("expected 1 region, got %d", len(regions))
		}
		r := regions[0]
		if r.mkvStart != 0 || r.mkvEnd != 64 {
			t.Errorf("region = [%d, %d), want [0, 64)", r.mkvStart, r.mkvEnd)
		}
		if r.srcOffset != 0 {
			t.Errorf("srcOffset = %d, want 0", r.srcOffset)
		}
	})

	t.Run("extra MKV bytes skipped", func(t *testing.T) {
		// Source: 32 bytes of sequential data.
		// MKV: same 32 bytes but with 2 extra bytes inserted at offset 16.
		srcData := make([]byte, 32)
		for i := range srcData {
			srcData[i] = byte(i + 1)
		}
		mkvData := make([]byte, 34)
		copy(mkvData[:16], srcData[:16])
		mkvData[16] = 0xFF // extra byte
		mkvData[17] = 0xFE // extra byte
		copy(mkvData[18:], srcData[16:])

		idx := source.NewIndex("/test", source.TypeBluray, windowSize)
		idx.ESReaders = []source.ESReader{&mockAudioESReader{
			audioData: map[byte][]byte{subStreamID: srcData},
		}}

		m := &Matcher{
			sourceIndex: idx,
			mkvData:     mkvData,
			mkvSize:     int64(len(mkvData)),
		}

		segments := []mkvSegment{{0, int64(len(mkvData))}}
		regions := m.fillTrueHDGapSegments(segments, 0, int64(len(srcData)), 0, subStreamID)

		if len(regions) != 2 {
			t.Fatalf("expected 2 regions (split by extra bytes), got %d", len(regions))
		}
		// First region: MKV [0, 16) → src [0, 16)
		if regions[0].mkvStart != 0 || regions[0].mkvEnd != 16 {
			t.Errorf("region[0] = [%d, %d), want [0, 16)", regions[0].mkvStart, regions[0].mkvEnd)
		}
		if regions[0].srcOffset != 0 {
			t.Errorf("region[0].srcOffset = %d, want 0", regions[0].srcOffset)
		}
		// Second region: MKV [18, 34) → src [16, 32)
		if regions[1].mkvStart != 18 || regions[1].mkvEnd != 34 {
			t.Errorf("region[1] = [%d, %d), want [18, 34)", regions[1].mkvStart, regions[1].mkvEnd)
		}
		if regions[1].srcOffset != 16 {
			t.Errorf("region[1].srcOffset = %d, want 16", regions[1].srcOffset)
		}
	})

	t.Run("runs shorter than minRunLen discarded", func(t *testing.T) {
		// Source: 15 bytes (just under the 16-byte minimum).
		// MKV: identical 15 bytes.
		srcData := make([]byte, 15)
		for i := range srcData {
			srcData[i] = byte(i + 1)
		}
		mkvData := make([]byte, 15)
		copy(mkvData, srcData)

		idx := source.NewIndex("/test", source.TypeBluray, windowSize)
		idx.ESReaders = []source.ESReader{&mockAudioESReader{
			audioData: map[byte][]byte{subStreamID: srcData},
		}}

		m := &Matcher{
			sourceIndex: idx,
			mkvData:     mkvData,
			mkvSize:     int64(len(mkvData)),
		}

		segments := []mkvSegment{{0, int64(len(mkvData))}}
		regions := m.fillTrueHDGapSegments(segments, 0, int64(len(srcData)), 0, subStreamID)

		if len(regions) != 0 {
			t.Errorf("expected 0 regions (run too short), got %d", len(regions))
		}
	})

	t.Run("segment boundary flushing", func(t *testing.T) {
		// Source: 48 bytes of sequential data.
		// MKV: same data split across two segments with a gap between them.
		srcData := make([]byte, 48)
		for i := range srcData {
			srcData[i] = byte(i + 1)
		}
		// MKV layout: [0..24) = first segment, [32..56) = second segment, gap at [24..32).
		mkvData := make([]byte, 56)
		copy(mkvData[0:24], srcData[0:24])
		copy(mkvData[32:56], srcData[24:48])

		idx := source.NewIndex("/test", source.TypeBluray, windowSize)
		idx.ESReaders = []source.ESReader{&mockAudioESReader{
			audioData: map[byte][]byte{subStreamID: srcData},
		}}

		m := &Matcher{
			sourceIndex: idx,
			mkvData:     mkvData,
			mkvSize:     int64(len(mkvData)),
		}

		segments := []mkvSegment{{0, 24}, {32, 56}}
		regions := m.fillTrueHDGapSegments(segments, 0, int64(len(srcData)), 0, subStreamID)

		if len(regions) != 2 {
			t.Fatalf("expected 2 regions (one per segment), got %d", len(regions))
		}
		// First segment
		if regions[0].mkvStart != 0 || regions[0].mkvEnd != 24 {
			t.Errorf("region[0] = [%d, %d), want [0, 24)", regions[0].mkvStart, regions[0].mkvEnd)
		}
		// Second segment
		if regions[1].mkvStart != 32 || regions[1].mkvEnd != 56 {
			t.Errorf("region[1] = [%d, %d), want [32, 56)", regions[1].mkvStart, regions[1].mkvEnd)
		}
		if regions[1].srcOffset != 24 {
			t.Errorf("region[1].srcOffset = %d, want 24", regions[1].srcOffset)
		}
	})

	t.Run("fileIndex and subStreamID propagated", func(t *testing.T) {
		srcData := make([]byte, 32)
		for i := range srcData {
			srcData[i] = byte(i + 1)
		}
		mkvData := make([]byte, 32)
		copy(mkvData, srcData)

		idx := source.NewIndex("/test", source.TypeBluray, windowSize)
		idx.ESReaders = []source.ESReader{
			nil, // fileIndex 0 unused
			&mockAudioESReader{audioData: map[byte][]byte{0x05: srcData}},
		}

		m := &Matcher{
			sourceIndex: idx,
			mkvData:     mkvData,
			mkvSize:     int64(len(mkvData)),
		}

		segments := []mkvSegment{{0, 32}}
		regions := m.fillTrueHDGapSegments(segments, 0, 32, 1, 0x05)

		if len(regions) != 1 {
			t.Fatalf("expected 1 region, got %d", len(regions))
		}
		if regions[0].fileIndex != 1 {
			t.Errorf("fileIndex = %d, want 1", regions[0].fileIndex)
		}
		if regions[0].audioSubStreamID != 0x05 {
			t.Errorf("audioSubStreamID = 0x%02X, want 0x05", regions[0].audioSubStreamID)
		}
	})
}
