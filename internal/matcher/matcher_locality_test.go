package matcher

import (
	"testing"

	"github.com/stuckj/mkvdup/internal/mkv"
	"github.com/stuckj/mkvdup/internal/source"
)

// mockESReader implements source.ESReader for testing locality recovery.
type mockESReader struct {
	data []byte
}

func (r *mockESReader) ReadESData(esOffset int64, size int, _ bool) ([]byte, error) {
	if esOffset < 0 || esOffset >= int64(len(r.data)) {
		return nil, nil
	}
	end := int(esOffset) + size
	if end > len(r.data) {
		end = len(r.data)
	}
	return r.data[esOffset:end], nil
}

func (r *mockESReader) ESOffsetToFileOffset(esOffset int64, _ bool) (int64, int) {
	return esOffset, len(r.data) - int(esOffset)
}

func (r *mockESReader) TotalESSize(_ bool) int64 {
	return int64(len(r.data))
}

func (r *mockESReader) AudioSubStreams() []byte { return nil }

func (r *mockESReader) AudioSubStreamESSize(_ byte) int64 { return 0 }

func (r *mockESReader) ReadAudioSubStreamData(_ byte, _ int64, _ int) ([]byte, error) {
	return nil, nil
}

func TestTryLocalityMatch_ExactMatch(t *testing.T) {
	// Source ES data: two consecutive NALs of 128 bytes each
	srcData := make([]byte, 300)
	for i := range srcData {
		srcData[i] = byte((i * 7) & 0xFF)
	}

	// MKV data: same as source (exact match scenario)
	// NAL 1 at MKV offset 0, NAL 2 at MKV offset 132 (128 + 4-byte AVCC prefix)
	// Source: NAL 1 at ES offset 0, NAL 2 at ES offset 131 (128 + 3-byte start code)
	nal1Size := 128
	nal2Size := 128
	mkvNAL2Offset := int64(nal1Size + 4) // AVCC: 4-byte length prefix
	srcNAL2Offset := int64(nal1Size + 3) // Annex B: 3-byte start code (00 00 01)

	// MKV packet data containing both NALs
	mkvData := make([]byte, 300)
	copy(mkvData, srcData[:nal1Size])                                                   // NAL 1 (same bytes)
	copy(mkvData[mkvNAL2Offset:], srcData[srcNAL2Offset:srcNAL2Offset+int64(nal2Size)]) // NAL 2

	idx := &source.Index{
		WindowSize:      64,
		HashToLocations: map[uint64][]source.Location{},
		SourceDir:       "/test",
		SourceType:      source.TypeBluray,
		Files:           []source.File{{RelativePath: "test.m2ts", Size: int64(len(srcData))}},
		UsesESOffsets:   true,
		ESReaders:       []source.ESReader{&mockESReader{data: srcData}},
	}

	m, err := NewMatcher(idx)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	m.mkvData = mkvData
	m.mkvSize = int64(len(mkvData))

	pkt := mkv.Packet{
		Offset:   0,
		Size:     int64(len(mkvData)),
		TrackNum: 1,
	}

	// Locality from NAL 1 match: source ended at nal1Size, MKV ended at nal1Size
	loc := packetLocality{
		valid:   true,
		fileIdx: 0,
		srcEnd:  int64(nal1Size),
		mkvEnd:  int64(nal1Size),
	}

	// Try to match NAL 2 via locality
	region := m.tryLocalityMatch(pkt, int(mkvNAL2Offset), mkvData[mkvNAL2Offset:], loc, nal2Size)
	if region == nil {
		t.Fatal("expected locality match but got nil")
	}
	if region.srcOffset != srcNAL2Offset {
		t.Errorf("srcOffset = %d, want %d", region.srcOffset, srcNAL2Offset)
	}
	matchLen := region.mkvEnd - region.mkvStart
	if matchLen != int64(nal2Size) {
		t.Errorf("match length = %d, want %d", matchLen, nal2Size)
	}
}

func TestTryLocalityMatch_NoLocality(t *testing.T) {
	srcData := make([]byte, 200)
	mkvData := make([]byte, 200)

	idx := &source.Index{
		WindowSize:      64,
		HashToLocations: map[uint64][]source.Location{},
		SourceDir:       "/test",
		SourceType:      source.TypeBluray,
		Files:           []source.File{{RelativePath: "test.m2ts", Size: int64(len(srcData))}},
		UsesESOffsets:   true,
		ESReaders:       []source.ESReader{&mockESReader{data: srcData}},
	}

	m, err := NewMatcher(idx)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	m.mkvData = mkvData
	m.mkvSize = int64(len(mkvData))

	pkt := mkv.Packet{Offset: 0, Size: int64(len(mkvData)), TrackNum: 1}

	// No locality info — should return nil
	region := m.tryLocalityMatch(pkt, 0, mkvData, packetLocality{}, 128)
	if region != nil {
		t.Error("expected nil with no locality, got match")
	}
}

func TestTryLocalityMatch_NALTooSmall(t *testing.T) {
	srcData := make([]byte, 200)
	mkvData := make([]byte, 200)

	idx := &source.Index{
		WindowSize:      64,
		HashToLocations: map[uint64][]source.Location{},
		SourceDir:       "/test",
		SourceType:      source.TypeBluray,
		Files:           []source.File{{RelativePath: "test.m2ts", Size: int64(len(srcData))}},
		UsesESOffsets:   true,
		ESReaders:       []source.ESReader{&mockESReader{data: srcData}},
	}

	m, err := NewMatcher(idx)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	m.mkvData = mkvData
	m.mkvSize = int64(len(mkvData))

	pkt := mkv.Packet{Offset: 0, Size: int64(len(mkvData)), TrackNum: 1}

	loc := packetLocality{valid: true, fileIdx: 0, srcEnd: 10, mkvEnd: 10}

	// NAL smaller than localityVerifyLen (64) — should return nil
	region := m.tryLocalityMatch(pkt, 20, mkvData[20:], loc, 32)
	if region != nil {
		t.Error("expected nil for small NAL, got match")
	}
}

func TestTryLocalityMatch_GapTooLarge(t *testing.T) {
	srcData := make([]byte, 200)
	mkvData := make([]byte, 10000)
	copy(mkvData, srcData)

	idx := &source.Index{
		WindowSize:      64,
		HashToLocations: map[uint64][]source.Location{},
		SourceDir:       "/test",
		SourceType:      source.TypeBluray,
		Files:           []source.File{{RelativePath: "test.m2ts", Size: int64(len(srcData))}},
		UsesESOffsets:   true,
		ESReaders:       []source.ESReader{&mockESReader{data: srcData}},
	}

	m, err := NewMatcher(idx)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	m.mkvData = mkvData
	m.mkvSize = int64(len(mkvData))

	pkt := mkv.Packet{Offset: 0, Size: int64(len(mkvData)), TrackNum: 1}

	// Last match at offset 0, current NAL at offset 5000 — gap >> nalSize*2
	loc := packetLocality{valid: true, fileIdx: 0, srcEnd: 100, mkvEnd: 100}
	region := m.tryLocalityMatch(pkt, 5000, mkvData[5000:], loc, 128)
	if region != nil {
		t.Error("expected nil for large gap, got match")
	}
}

func TestTryLocalityMatch_ByteMismatch(t *testing.T) {
	srcData := make([]byte, 200)
	for i := range srcData {
		srcData[i] = byte(i)
	}
	// MKV data deliberately different from source at the predicted position
	mkvData := make([]byte, 200)
	for i := range mkvData {
		mkvData[i] = byte(i + 100) // Different from source
	}

	idx := &source.Index{
		WindowSize:      64,
		HashToLocations: map[uint64][]source.Location{},
		SourceDir:       "/test",
		SourceType:      source.TypeBluray,
		Files:           []source.File{{RelativePath: "test.m2ts", Size: int64(len(srcData))}},
		UsesESOffsets:   true,
		ESReaders:       []source.ESReader{&mockESReader{data: srcData}},
	}

	m, err := NewMatcher(idx)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	m.mkvData = mkvData
	m.mkvSize = int64(len(mkvData))

	pkt := mkv.Packet{Offset: 0, Size: int64(len(mkvData)), TrackNum: 1}

	loc := packetLocality{valid: true, fileIdx: 0, srcEnd: 64, mkvEnd: 64}
	region := m.tryLocalityMatch(pkt, 68, mkvData[68:], loc, 128)
	if region != nil {
		t.Error("expected nil for byte mismatch, got match")
	}
}
