package matcher

import (
	"testing"

	"github.com/cespare/xxhash/v2"
	"github.com/stuckj/mkvdup/internal/mkv"
	"github.com/stuckj/mkvdup/internal/source"
)

// TestPhase2ShortCircuit_DTSFrameSize verifies that when Phase 1 finds a match
// covering an entire DTS-sized frame (~512 bytes), Phase 2 is NOT triggered.
// This is the core diagnostic for the DTS thrashing fix.
//
// The source and MKV data diverge immediately after the frame boundary so that
// expansion stops at nalSize, ensuring the nalSize short-circuit (not the 4KB
// threshold) is what actually skips Phase 2.
func TestPhase2ShortCircuit_DTSFrameSize(t *testing.T) {
	const windowSize = 64
	const dtsFrameSize = 512 // Typical DTS core frame

	// Create source data: one "correct" file and 300 "wrong" files.
	// All files share the same first 64 bytes (same hash window)
	// but only the correct file matches beyond the window.
	correctData := make([]byte, 65536)
	for i := range correctData {
		correctData[i] = byte(i % 251) // Prime modulus for variety
	}

	// Compute the hash of the first windowSize bytes
	hash := xxhash.Sum64(correctData[:windowSize])

	// Create 300 "wrong" source files — same first 64 bytes, different after that
	numWrongFiles := 300
	wrongData := make([]byte, 65536)
	copy(wrongData, correctData[:windowSize]) // Same window
	for i := windowSize; i < len(wrongData); i++ {
		wrongData[i] = byte((i * 7) % 256) // Different pattern after window
	}

	// Build source index with 301 files (1 correct + 300 wrong)
	files := make([]source.File, numWrongFiles+1)
	rawReaders := make([]source.RawReader, numWrongFiles+1)
	locations := make([]source.Location, numWrongFiles+1)

	// File 0 = correct file
	files[0] = source.File{RelativePath: "correct.m2ts", Size: int64(len(correctData))}
	rawReaders[0] = &sliceReader{data: correctData}
	locations[0] = source.Location{FileIndex: 0, Offset: 0}

	// Files 1..300 = wrong files (same hash, different data)
	for i := 1; i <= numWrongFiles; i++ {
		files[i] = source.File{RelativePath: "wrong.m2ts", Size: int64(len(wrongData))}
		rawReaders[i] = &sliceReader{data: wrongData}
		locations[i] = source.Location{FileIndex: uint16(i), Offset: 0}
	}

	idx := &source.Index{
		WindowSize:      windowSize,
		HashToLocations: map[uint64][]source.Location{hash: locations},
		SourceDir:       "/test",
		SourceType:      source.TypeBluray,
		Files:           files,
		RawReaders:      rawReaders,
		UsesESOffsets:   false,
	}
	// Sort for locality search
	idx.SortLocationsByOffset()

	m, err := NewMatcher(idx)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	// Set up MKV data that matches the correct file for the first dtsFrameSize
	// bytes, then diverges. This ensures expansion stops at exactly dtsFrameSize
	// so only the nalSize short-circuit can skip Phase 2 (not the 4KB threshold).
	mkvData := make([]byte, 65536)
	copy(mkvData[:dtsFrameSize], correctData[:dtsFrameSize])
	for i := dtsFrameSize; i < len(mkvData); i++ {
		mkvData[i] = byte((i * 13) % 256) // Different from all source files
	}
	m.mkvData = mkvData
	m.mkvSize = int64(len(mkvData))
	m.trackTypes = map[int]int{1: mkv.TrackTypeAudio}
	m.trackCodecs = map[int]trackCodecInfo{1: {trackType: mkv.TrackTypeAudio}}
	m.trackHints = map[uint64]*trackLocalityHint{1: {}}

	// Initialize coverage bitmap
	numChunks := (m.mkvSize + coverageChunkSize - 1) / coverageChunkSize
	m.coveredChunks = make([]uint64, (numChunks+63)/64)

	// Set up locality hint pointing to the correct file
	hint := m.trackHints[1]
	hint.fileIndex.Store(0) // Correct file
	hint.offset.Store(0)
	hint.valid.Store(true)

	// Create packet
	pkt := mkv.Packet{
		Offset:   0,
		Size:     int64(len(mkvData)),
		TrackNum: 1,
	}

	// Call tryMatchFromOffsetParallel with DTS-like nalSize (nalSizeExact=true)
	matched := m.tryMatchFromOffsetParallel(pkt, 0, mkvData, false, hint, dtsFrameSize, true)
	if !matched {
		t.Fatal("expected match but got none")
	}

	// KEY DIAGNOSTIC: Phase 2 should NOT have been triggered.
	// The match is exactly dtsFrameSize (512) bytes — below the 4KB threshold —
	// so only the nalSize short-circuit could have prevented Phase 2.
	phase2Fallbacks := m.diagPhase2Fallbacks.Load()
	phase2Locations := m.diagPhase2Locations.Load()

	t.Logf("Phase 2 fallbacks: %d (want 0)", phase2Fallbacks)
	t.Logf("Phase 2 locations checked: %d (want 0)", phase2Locations)

	if phase2Fallbacks != 0 {
		t.Errorf("Phase 2 was triggered %d times; nalSize-aware short-circuit failed", phase2Fallbacks)
	}
	if phase2Locations != 0 {
		t.Errorf("Phase 2 checked %d locations; should be 0 with short-circuit", phase2Locations)
	}
}

// TestPhase2ShortCircuit_LargeFrame verifies that the existing 4KB threshold
// still works — a large match should skip Phase 2 via the original condition.
func TestPhase2ShortCircuit_LargeFrame(t *testing.T) {
	const windowSize = 64
	const largeNALSize = 8192

	correctData := make([]byte, 65536)
	for i := range correctData {
		correctData[i] = byte(i % 251)
	}
	hash := xxhash.Sum64(correctData[:windowSize])

	numWrongFiles := 50
	wrongData := make([]byte, 65536)
	copy(wrongData, correctData[:windowSize])
	for i := windowSize; i < len(wrongData); i++ {
		wrongData[i] = byte((i * 7) % 256)
	}

	files := make([]source.File, numWrongFiles+1)
	rawReaders := make([]source.RawReader, numWrongFiles+1)
	locations := make([]source.Location, numWrongFiles+1)

	files[0] = source.File{RelativePath: "correct.m2ts", Size: int64(len(correctData))}
	rawReaders[0] = &sliceReader{data: correctData}
	locations[0] = source.Location{FileIndex: 0, Offset: 0}

	for i := 1; i <= numWrongFiles; i++ {
		files[i] = source.File{RelativePath: "wrong.m2ts", Size: int64(len(wrongData))}
		rawReaders[i] = &sliceReader{data: wrongData}
		locations[i] = source.Location{FileIndex: uint16(i), Offset: 0}
	}

	idx := &source.Index{
		WindowSize:      windowSize,
		HashToLocations: map[uint64][]source.Location{hash: locations},
		SourceDir:       "/test",
		SourceType:      source.TypeBluray,
		Files:           files,
		RawReaders:      rawReaders,
		UsesESOffsets:   false,
	}
	idx.SortLocationsByOffset()

	m, err := NewMatcher(idx)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	mkvData := make([]byte, 65536)
	copy(mkvData, correctData)
	m.mkvData = mkvData
	m.mkvSize = int64(len(mkvData))
	m.trackTypes = map[int]int{1: mkv.TrackTypeAudio}
	m.trackCodecs = map[int]trackCodecInfo{1: {trackType: mkv.TrackTypeAudio}}
	m.trackHints = map[uint64]*trackLocalityHint{1: {}}

	numChunks := (m.mkvSize + coverageChunkSize - 1) / coverageChunkSize
	m.coveredChunks = make([]uint64, (numChunks+63)/64)

	hint := m.trackHints[1]
	hint.fileIndex.Store(0)
	hint.offset.Store(0)
	hint.valid.Store(true)

	pkt := mkv.Packet{Offset: 0, Size: int64(len(mkvData)), TrackNum: 1}

	matched := m.tryMatchFromOffsetParallel(pkt, 0, mkvData, false, hint, largeNALSize, true)
	if !matched {
		t.Fatal("expected match but got none")
	}

	// With a large match (>4KB), the original localityGoodMatchThreshold kicks in
	phase2Fallbacks := m.diagPhase2Fallbacks.Load()
	t.Logf("Phase 2 fallbacks: %d (want 0)", phase2Fallbacks)
	if phase2Fallbacks != 0 {
		t.Errorf("Phase 2 was triggered %d times for large frame", phase2Fallbacks)
	}
}

// TestPhase2ShortCircuit_LPCMNotShortCircuited verifies that LPCM tracks
// (nalSize=8 < windowSize=64) do NOT get the nalSize short-circuit,
// since their tiny sync intervals make hash collisions more likely.
// The MKV data diverges after the window size (64 bytes) so the match stays
// below the 4KB threshold, ensuring only the nalSize guard could skip Phase 2.
// Since nalSize < windowSize, the guard should NOT fire and Phase 2 SHOULD run.
func TestPhase2ShortCircuit_LPCMNotShortCircuited(t *testing.T) {
	const windowSize = 64
	const lpcmNALSize = 8 // LPCM sync intervals are 8 bytes

	correctData := make([]byte, 65536)
	for i := range correctData {
		correctData[i] = byte(i % 251)
	}
	hash := xxhash.Sum64(correctData[:windowSize])

	numWrongFiles := 10
	wrongData := make([]byte, 65536)
	copy(wrongData, correctData[:windowSize])
	for i := windowSize; i < len(wrongData); i++ {
		wrongData[i] = byte((i * 7) % 256)
	}

	files := make([]source.File, numWrongFiles+1)
	rawReaders := make([]source.RawReader, numWrongFiles+1)
	locations := make([]source.Location, numWrongFiles+1)

	files[0] = source.File{RelativePath: "correct.m2ts", Size: int64(len(correctData))}
	rawReaders[0] = &sliceReader{data: correctData}
	locations[0] = source.Location{FileIndex: 0, Offset: 0}

	for i := 1; i <= numWrongFiles; i++ {
		files[i] = source.File{RelativePath: "wrong.m2ts", Size: int64(len(wrongData))}
		rawReaders[i] = &sliceReader{data: wrongData}
		locations[i] = source.Location{FileIndex: uint16(i), Offset: 0}
	}

	idx := &source.Index{
		WindowSize:      windowSize,
		HashToLocations: map[uint64][]source.Location{hash: locations},
		SourceDir:       "/test",
		SourceType:      source.TypeBluray,
		Files:           files,
		RawReaders:      rawReaders,
		UsesESOffsets:   false,
	}
	idx.SortLocationsByOffset()

	m, err := NewMatcher(idx)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	// MKV data matches correct source for only windowSize bytes, then diverges.
	// This keeps the match at exactly 64 bytes — well below the 4KB threshold —
	// so only the nalSize guard could prevent Phase 2 from running.
	mkvData := make([]byte, 65536)
	copy(mkvData[:windowSize], correctData[:windowSize])
	for i := windowSize; i < len(mkvData); i++ {
		mkvData[i] = byte((i * 13) % 256) // Different from all source files
	}
	m.mkvData = mkvData
	m.mkvSize = int64(len(mkvData))
	m.trackTypes = map[int]int{1: mkv.TrackTypeAudio}
	m.trackCodecs = map[int]trackCodecInfo{1: {trackType: mkv.TrackTypeAudio}}
	m.trackHints = map[uint64]*trackLocalityHint{1: {}}

	numChunks := (m.mkvSize + coverageChunkSize - 1) / coverageChunkSize
	m.coveredChunks = make([]uint64, (numChunks+63)/64)

	hint := m.trackHints[1]
	hint.fileIndex.Store(0)
	hint.offset.Store(0)
	hint.valid.Store(true)

	pkt := mkv.Packet{Offset: 0, Size: int64(len(mkvData)), TrackNum: 1}

	matched := m.tryMatchFromOffsetParallel(pkt, 0, mkvData, false, hint, lpcmNALSize, true)
	if !matched {
		t.Fatal("expected match but got none")
	}

	// LPCM nalSize=8 < windowSize=64, so the nalSize short-circuit must NOT apply.
	// The match is only 64 bytes (below 4KB threshold), so neither short-circuit
	// fires. Phase 2 SHOULD run.
	phase2Fallbacks := m.diagPhase2Fallbacks.Load()
	t.Logf("Phase 2 fallbacks: %d (want 1 — nalSize guard prevented short-circuit)", phase2Fallbacks)
	if phase2Fallbacks != 1 {
		t.Errorf("Phase 2 fallbacks = %d, want 1 (LPCM nalSize < windowSize should not short-circuit)", phase2Fallbacks)
	}
}

// TestPhase2Fallback_NoHint verifies that Phase 2 IS triggered when there's
// no locality hint (first packet for a track), and confirms the diagnostic
// counters track it correctly.
func TestPhase2Fallback_NoHint(t *testing.T) {
	const windowSize = 64
	const dtsFrameSize = 512

	correctData := make([]byte, 65536)
	for i := range correctData {
		correctData[i] = byte(i % 251)
	}
	hash := xxhash.Sum64(correctData[:windowSize])

	numWrongFiles := 10
	wrongData := make([]byte, 65536)
	copy(wrongData, correctData[:windowSize])
	for i := windowSize; i < len(wrongData); i++ {
		wrongData[i] = byte((i * 7) % 256)
	}

	files := make([]source.File, numWrongFiles+1)
	rawReaders := make([]source.RawReader, numWrongFiles+1)
	locations := make([]source.Location, numWrongFiles+1)

	files[0] = source.File{RelativePath: "correct.m2ts", Size: int64(len(correctData))}
	rawReaders[0] = &sliceReader{data: correctData}
	locations[0] = source.Location{FileIndex: 0, Offset: 0}

	for i := 1; i <= numWrongFiles; i++ {
		files[i] = source.File{RelativePath: "wrong.m2ts", Size: int64(len(wrongData))}
		rawReaders[i] = &sliceReader{data: wrongData}
		locations[i] = source.Location{FileIndex: uint16(i), Offset: 0}
	}

	idx := &source.Index{
		WindowSize:      windowSize,
		HashToLocations: map[uint64][]source.Location{hash: locations},
		SourceDir:       "/test",
		SourceType:      source.TypeBluray,
		Files:           files,
		RawReaders:      rawReaders,
		UsesESOffsets:   false,
	}
	idx.SortLocationsByOffset()

	m, err := NewMatcher(idx)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	// MKV data matches correct source for dtsFrameSize bytes, then diverges.
	mkvData := make([]byte, 65536)
	copy(mkvData[:dtsFrameSize], correctData[:dtsFrameSize])
	for i := dtsFrameSize; i < len(mkvData); i++ {
		mkvData[i] = byte((i * 13) % 256)
	}
	m.mkvData = mkvData
	m.mkvSize = int64(len(mkvData))
	m.trackTypes = map[int]int{1: mkv.TrackTypeAudio}
	m.trackCodecs = map[int]trackCodecInfo{1: {trackType: mkv.TrackTypeAudio}}
	m.trackHints = map[uint64]*trackLocalityHint{1: {}}

	numChunks := (m.mkvSize + coverageChunkSize - 1) / coverageChunkSize
	m.coveredChunks = make([]uint64, (numChunks+63)/64)

	// No hint set — hint.valid is false (zero value)
	pkt := mkv.Packet{Offset: 0, Size: int64(len(mkvData)), TrackNum: 1}

	// With no valid hint, Phase 1 is skipped entirely, so Phase 2 must run
	matched := m.tryMatchFromOffsetParallel(pkt, 0, mkvData, false, m.trackHints[1], dtsFrameSize, true)
	if !matched {
		t.Fatal("expected match but got none")
	}

	phase2Fallbacks := m.diagPhase2Fallbacks.Load()
	t.Logf("Phase 2 fallbacks: %d (want 1 — no hint available)", phase2Fallbacks)
	if phase2Fallbacks != 1 {
		t.Errorf("Phase 2 fallbacks = %d, want 1 (no hint means Phase 2 needed)", phase2Fallbacks)
	}
}

// TestNALSizeComputation_AudioSyncPoints verifies that computeNALSize correctly
// computes frame size as distance-to-next-sync-point for audio tracks.
func TestNALSizeComputation_AudioSyncPoints(t *testing.T) {
	// Build a data buffer with DTS sync words at known offsets
	data := make([]byte, 4096)

	// Place DTS sync words (7F FE 80 01) at offsets 0, 512, 1024
	dtsSyncWord := []byte{0x7F, 0xFE, 0x80, 0x01}
	copy(data[0:], dtsSyncWord)
	copy(data[512:], dtsSyncWord)
	copy(data[1024:], dtsSyncWord)

	syncPoints := source.FindAudioSyncPoints(data)

	// Verify sync points were found at expected offsets
	if len(syncPoints) < 3 {
		t.Fatalf("expected at least 3 sync points, got %d: %v", len(syncPoints), syncPoints)
	}
	if syncPoints[0] != 0 || syncPoints[1] != 512 || syncPoints[2] != 1024 {
		t.Fatalf("unexpected sync points: %v (want [0, 512, 1024, ...])", syncPoints)
	}

	// Test nalSize computation using the production helper
	for i, syncOff := range syncPoints {
		nalSize, exact := computeNALSize(syncPoints, i, syncOff, len(data), false, 0)

		switch i {
		case 0:
			if nalSize != 512 || !exact {
				t.Errorf("syncPoint[0] nalSize = %d, exact = %v, want 512, true", nalSize, exact)
			}
		case 1:
			if nalSize != 512 || !exact {
				t.Errorf("syncPoint[1] nalSize = %d, exact = %v, want 512, true", nalSize, exact)
			}
		case 2:
			// Last sync point: nalSize = remaining data, not exact
			expected := len(data) - 1024
			if nalSize != expected || exact {
				t.Errorf("syncPoint[2] nalSize = %d, exact = %v, want %d, false", nalSize, exact, expected)
			}
		}
	}

	t.Logf("DTS sync points found: %v", syncPoints[:3])
	t.Logf("nalSize values: 512, 512, %d", len(data)-1024)
}

// TestNALSizeExact_PreventsShortCircuit verifies that when nalSizeExact is
// false (last sync point in truncated buffer), the nalSize short-circuit
// does NOT apply, even if bestMatchLen >= nalSize.
func TestNALSizeExact_PreventsShortCircuit(t *testing.T) {
	const windowSize = 64
	const nalSize = 512

	correctData := make([]byte, 65536)
	for i := range correctData {
		correctData[i] = byte(i % 251)
	}
	hash := xxhash.Sum64(correctData[:windowSize])

	// Only one source file — no wrong files. With nalSizeExact=false,
	// the match of 512 bytes shouldn't trigger the nalSize short-circuit,
	// so Phase 2 should run (even though Phase 1 found a match).
	files := []source.File{{RelativePath: "correct.m2ts", Size: int64(len(correctData))}}
	rawReaders := []source.RawReader{&sliceReader{data: correctData}}
	locations := []source.Location{{FileIndex: 0, Offset: 0}}

	idx := &source.Index{
		WindowSize:      windowSize,
		HashToLocations: map[uint64][]source.Location{hash: locations},
		SourceDir:       "/test",
		SourceType:      source.TypeBluray,
		Files:           files,
		RawReaders:      rawReaders,
		UsesESOffsets:   false,
	}
	idx.SortLocationsByOffset()

	m, err := NewMatcher(idx)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	// MKV matches for nalSize bytes then diverges
	mkvData := make([]byte, 65536)
	copy(mkvData[:nalSize], correctData[:nalSize])
	for i := nalSize; i < len(mkvData); i++ {
		mkvData[i] = byte((i * 13) % 256)
	}
	m.mkvData = mkvData
	m.mkvSize = int64(len(mkvData))
	m.trackTypes = map[int]int{1: mkv.TrackTypeAudio}
	m.trackCodecs = map[int]trackCodecInfo{1: {trackType: mkv.TrackTypeAudio}}
	m.trackHints = map[uint64]*trackLocalityHint{1: {}}

	numChunks := (m.mkvSize + coverageChunkSize - 1) / coverageChunkSize
	m.coveredChunks = make([]uint64, (numChunks+63)/64)

	hint := m.trackHints[1]
	hint.fileIndex.Store(0)
	hint.offset.Store(0)
	hint.valid.Store(true)

	pkt := mkv.Packet{Offset: 0, Size: int64(len(mkvData)), TrackNum: 1}

	// nalSizeExact=false: match is 512 bytes (== nalSize) but nalSize is
	// not from a real sync point boundary, so the short-circuit must NOT fire.
	// With only 1 location, Phase 2 has nothing new to try, but the gate
	// decision (phase2Skipped) should be false.
	m.tryMatchFromOffsetParallel(pkt, 0, mkvData, false, hint, nalSize, false)

	phase1Skips := m.diagPhase1Skips.Load()
	t.Logf("Phase 1 skips: %d (want 0 — nalSizeExact=false should prevent skip)", phase1Skips)
	if phase1Skips != 0 {
		t.Errorf("Phase 1 skips = %d, want 0 (nalSizeExact=false should not short-circuit)", phase1Skips)
	}
}
