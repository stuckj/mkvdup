package matcher

import (
	"testing"

	"github.com/stuckj/mkvdup/internal/mkv"
	"github.com/stuckj/mkvdup/internal/source"
)

// bytesRawReader implements source.RawReader backed by an in-memory byte slice.
type bytesRawReader struct {
	data []byte
}

func (r *bytesRawReader) ReadAt(buf []byte, offset int64) (int, error) {
	if offset >= int64(len(r.data)) {
		return 0, nil
	}
	n := copy(buf, r.data[offset:])
	return n, nil
}

func (r *bytesRawReader) Slice(offset int64, size int) []byte {
	if offset < 0 || offset >= int64(len(r.data)) {
		return nil
	}
	end := offset + int64(size)
	if end > int64(len(r.data)) {
		end = int64(len(r.data))
	}
	return r.data[offset:end]
}

func (r *bytesRawReader) Len() int    { return len(r.data) }
func (r *bytesRawReader) Close() error { return nil }

// TestTryVerifyAndExpand_RejectsLPCMForNonPCMTrack verifies that
// tryVerifyAndExpand returns nil when a source location has an LPCM
// sub-stream ID (0xA0-0xA7) but the MKV track is not PCM audio.
// This is a regression test for a bug where coincidental byte-level
// matches between non-PCM MKV data (e.g. AC3) and LPCM source data
// produced entries flagged as LPCM, causing the byte-swap transform
// to corrupt the output during FUSE reconstruction.
func TestTryVerifyAndExpand_RejectsLPCMForNonPCMTrack(t *testing.T) {
	const windowSize = 64
	const trackNum = 1

	// Build identical data for MKV and source so verification passes.
	testData := make([]byte, 256)
	for i := range testData {
		testData[i] = byte(i)
	}

	// Source index with a raw reader (non-ES path) containing the same data.
	idx := source.NewIndex("/test/src", source.TypeDVD, windowSize)
	idx.RawReaders = []source.RawReader{&bytesRawReader{data: testData}}
	idx.Files = []source.File{{RelativePath: "test.vob", Size: int64(len(testData))}}

	// Matcher with mkvData identical to source so the verify step succeeds.
	m := &Matcher{
		sourceIndex: idx,
		mkvData:     testData,
		mkvSize:     int64(len(testData)),
		windowSize:  windowSize,
		isPCMTrack:  make(map[int]bool),
		isAVCTrack:  make(map[int]bool),
		trackCodecs: make(map[int]trackCodecInfo),
		trackTypes:  make(map[int]int),
	}

	// An LPCM source location (sub-stream 0xA0 is in the LPCM range).
	loc := source.Location{
		FileIndex:        0,
		Offset:           0,
		IsVideo:          false,
		AudioSubStreamID: 0xA0,
	}

	pkt := mkv.Packet{
		Offset:   0,
		Size:     int64(len(testData)),
		TrackNum: trackNum,
	}

	t.Run("non-PCM track rejects LPCM match", func(t *testing.T) {
		// Track is NOT PCM (e.g. AC3). The LPCM match should be rejected.
		m.isPCMTrack[trackNum] = false

		region := m.tryVerifyAndExpand(pkt, loc, 0, false)
		if region != nil {
			t.Errorf("expected nil (LPCM match rejected for non-PCM track), got region [%d, %d)",
				region.mkvStart, region.mkvEnd)
		}
	})

	t.Run("PCM track accepts LPCM match", func(t *testing.T) {
		// Track IS PCM. The LPCM match should be accepted.
		m.isPCMTrack[trackNum] = true

		region := m.tryVerifyAndExpand(pkt, loc, 0, false)
		if region == nil {
			t.Fatal("expected non-nil region for LPCM match on PCM track, got nil")
		}
		if !region.isLPCM {
			t.Error("expected region.isLPCM to be true")
		}
		if region.audioSubStreamID != 0xA0 {
			t.Errorf("audioSubStreamID = 0x%02X, want 0xA0", region.audioSubStreamID)
		}
	})

	t.Run("non-LPCM sub-stream on non-PCM track is accepted", func(t *testing.T) {
		// AC3 sub-stream (0x80) on a non-PCM track should NOT be rejected
		// by the LPCM guard.
		m.isPCMTrack[trackNum] = false

		ac3Loc := source.Location{
			FileIndex:        0,
			Offset:           0,
			IsVideo:          false,
			AudioSubStreamID: 0x80,
		}

		region := m.tryVerifyAndExpand(pkt, ac3Loc, 0, false)
		if region == nil {
			t.Fatal("expected non-nil region for non-LPCM match on non-PCM track, got nil")
		}
		if region.isLPCM {
			t.Error("expected region.isLPCM to be false for AC3 sub-stream")
		}
	})
}
