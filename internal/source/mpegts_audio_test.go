package source

import (
	"testing"
)

func TestMPEGTSParser_TrueHDAC3Split(t *testing.T) {
	data := buildTrueHDAC3M2TSData()
	p := NewMPEGTSParser(data)
	if err := p.Parse(); err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	// Should have 2 audio sub-streams after splitting: TrueHD (original) and AC3 (new)
	if got := p.AudioSubStreamCount(); got != 2 {
		t.Fatalf("AudioSubStreamCount = %d, want 2 (TrueHD + AC3 after split)", got)
	}

	subs := p.AudioSubStreams()

	// Sub-stream 0 should be TrueHD-only (original, now filtered)
	truehdSize := p.AudioSubStreamESSize(subs[0])
	// Sub-stream 1 should be AC3-only (newly created)
	ac3Size := p.AudioSubStreamESSize(subs[1])

	// AC3: 2 frames × 192 bytes = 384 bytes
	if ac3Size != 384 {
		t.Errorf("AC3 sub-stream size = %d, want 384 (2 × 192)", ac3Size)
	}

	// TrueHD: 300 + 227 = 527 bytes
	if truehdSize != 527 {
		t.Errorf("TrueHD sub-stream size = %d, want 527 (300 + 227)", truehdSize)
	}

	// Total should equal original audio payload size (911 bytes)
	totalAudio := truehdSize + ac3Size
	if totalAudio != 911 {
		t.Errorf("Total audio ES = %d, want 911", totalAudio)
	}

	// Verify AC3 data starts with sync word
	ac3Data, err := p.ReadAudioSubStreamData(subs[1], 0, 2)
	if err != nil {
		t.Fatalf("ReadAudioSubStreamData(AC3) error: %v", err)
	}
	if ac3Data[0] != 0x0B || ac3Data[1] != 0x77 {
		t.Errorf("AC3 data starts with [%02X %02X], want [0B 77]", ac3Data[0], ac3Data[1])
	}

	// Verify TrueHD data starts with major sync
	truehdData, err := p.ReadAudioSubStreamData(subs[0], 0, 4)
	if err != nil {
		t.Fatalf("ReadAudioSubStreamData(TrueHD) error: %v", err)
	}
	if truehdData[0] != 0xF8 || truehdData[1] != 0x72 || truehdData[2] != 0x6F || truehdData[3] != 0xBA {
		t.Errorf("TrueHD data starts with [%02X %02X %02X %02X], want [F8 72 6F BA]",
			truehdData[0], truehdData[1], truehdData[2], truehdData[3])
	}
}

func TestMPEGTSParser_NoSplitForNonTrueHD(t *testing.T) {
	// Regular AC3 stream (type 0x81) should NOT be split
	data := buildBasicM2TSData()
	p := NewMPEGTSParser(data)
	if err := p.Parse(); err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	// Should still have just 1 audio sub-stream
	if got := p.AudioSubStreamCount(); got != 1 {
		t.Errorf("AudioSubStreamCount = %d, want 1 (no split for non-TrueHD)", got)
	}
}

func TestDetectCombinedTrueHDAC3(t *testing.T) {
	// Create test data buffer that contains both AC3 and TrueHD sync words
	combined := make([]byte, 512)
	// AC3 sync at offset 0
	combined[0] = 0x0B
	combined[1] = 0x77
	// TrueHD sync at offset 200
	combined[200] = 0xF8
	combined[201] = 0x72
	combined[202] = 0x6F
	combined[203] = 0xBA

	// Build a minimal parser with this data
	p := &MPEGTSParser{
		data: combined,
		size: int64(len(combined)),
	}

	ranges := []PESPayloadRange{
		{FileOffset: 0, Size: 512, ESOffset: 0},
	}

	if !p.detectCombinedTrueHDAC3(ranges) {
		t.Error("detectCombinedTrueHDAC3() = false, want true for data with both sync words")
	}

	// Data with only AC3 sync
	ac3Only := make([]byte, 512)
	ac3Only[0] = 0x0B
	ac3Only[1] = 0x77
	p2 := &MPEGTSParser{
		data: ac3Only,
		size: int64(len(ac3Only)),
	}
	if p2.detectCombinedTrueHDAC3(ranges) {
		t.Error("detectCombinedTrueHDAC3() = true, want false for AC3-only data")
	}

	// Data with only TrueHD sync
	truehdOnly := make([]byte, 512)
	truehdOnly[0] = 0xF8
	truehdOnly[1] = 0x72
	truehdOnly[2] = 0x6F
	truehdOnly[3] = 0xBA
	p3 := &MPEGTSParser{
		data: truehdOnly,
		size: int64(len(truehdOnly)),
	}
	if p3.detectCombinedTrueHDAC3(ranges) {
		t.Error("detectCombinedTrueHDAC3() = true, want false for TrueHD-only data")
	}
}

func TestSplitCombinedAudioRanges(t *testing.T) {
	// Build combined payload: AC3(192) + TrueHD(100) + AC3(192) + TrueHD(50)
	var payload []byte
	payload = append(payload, makeAC3Frame(0x11)...)        // 192 bytes
	payload = append(payload, makeTrueHDUnit(100, 0x22)...) // 100 bytes
	payload = append(payload, makeAC3Frame(0x33)...)        // 192 bytes
	payload = append(payload, makeTrueHDUnit(50, 0x44)...)  // 50 bytes
	// Total: 534 bytes

	p := &MPEGTSParser{
		data: payload,
		size: int64(len(payload)),
	}

	ranges := []PESPayloadRange{
		{FileOffset: 0, Size: len(payload), ESOffset: 0},
	}

	ac3Ranges, truehdRanges := p.splitCombinedAudioRanges(ranges)

	// Calculate totals
	var ac3Total, truehdTotal int64
	for _, r := range ac3Ranges {
		ac3Total += int64(r.Size)
	}
	for _, r := range truehdRanges {
		truehdTotal += int64(r.Size)
	}

	if ac3Total != 384 {
		t.Errorf("AC3 total size = %d, want 384 (2 × 192)", ac3Total)
	}
	if truehdTotal != 150 {
		t.Errorf("TrueHD total size = %d, want 150 (100 + 50)", truehdTotal)
	}
	if ac3Total+truehdTotal != int64(len(payload)) {
		t.Errorf("AC3(%d) + TrueHD(%d) = %d, want %d (total payload)",
			ac3Total, truehdTotal, ac3Total+truehdTotal, len(payload))
	}

	// Verify ES offsets are sequential
	for i := 1; i < len(ac3Ranges); i++ {
		prev := ac3Ranges[i-1]
		cur := ac3Ranges[i]
		if cur.ESOffset != prev.ESOffset+int64(prev.Size) {
			t.Errorf("AC3 range[%d] ESOffset = %d, want %d", i, cur.ESOffset, prev.ESOffset+int64(prev.Size))
		}
	}
	for i := 1; i < len(truehdRanges); i++ {
		prev := truehdRanges[i-1]
		cur := truehdRanges[i]
		if cur.ESOffset != prev.ESOffset+int64(prev.Size) {
			t.Errorf("TrueHD range[%d] ESOffset = %d, want %d", i, cur.ESOffset, prev.ESOffset+int64(prev.Size))
		}
	}
}

func TestSplitCombinedAudioRanges_CrossRange(t *testing.T) {
	// Build combined payload: AC3(192) + TrueHD(100) + AC3(192) + TrueHD(50)
	var payload []byte
	payload = append(payload, makeAC3Frame(0x11)...)        // 192 bytes
	payload = append(payload, makeTrueHDUnit(100, 0x22)...) // 100 bytes
	payload = append(payload, makeAC3Frame(0x33)...)        // 192 bytes
	payload = append(payload, makeTrueHDUnit(50, 0x44)...)  // 50 bytes
	// Total: 534 bytes

	p := &MPEGTSParser{
		data: payload,
		size: int64(len(payload)),
	}

	// Split into small ranges that force AC3 headers to straddle boundaries.
	// Use 100-byte ranges so the second AC3 frame at offset 292 has its header
	// split: bytes 0-7 are in range[2] (200-300), sync word at byte 92 in that range,
	// but if we use 90-byte ranges, AC3 at 292 will have its sync at offset 292
	// which is range[3] (270-360) at pos 22 — that fits. So let's use a boundary
	// that splits the AC3 header. AC3 frame 2 starts at offset 292.
	// Using 294-byte first range puts the AC3 sync 0B 77 inside range[0],
	// but let's use a size that splits the 5-byte header.
	// Offset 292 = sync word. If range boundary is at 293, the 0B is in range[0], 77 in range[1].
	ranges := []PESPayloadRange{
		{FileOffset: 0, Size: 293, ESOffset: 0},     // ends mid AC3 header (has 0B at [292])
		{FileOffset: 293, Size: 241, ESOffset: 293}, // rest (has 77 at [0])
	}

	ac3Ranges, truehdRanges := p.splitCombinedAudioRanges(ranges)

	var ac3Total, truehdTotal int64
	for _, r := range ac3Ranges {
		ac3Total += int64(r.Size)
	}
	for _, r := range truehdRanges {
		truehdTotal += int64(r.Size)
	}

	if ac3Total != 384 {
		t.Errorf("AC3 total size = %d, want 384 (2 × 192)", ac3Total)
	}
	if truehdTotal != 150 {
		t.Errorf("TrueHD total size = %d, want 150 (100 + 50)", truehdTotal)
	}
	if ac3Total+truehdTotal != int64(len(payload)) {
		t.Errorf("AC3(%d) + TrueHD(%d) = %d, want %d",
			ac3Total, truehdTotal, ac3Total+truehdTotal, len(payload))
	}

	// Verify ES offsets are sequential
	for i := 1; i < len(ac3Ranges); i++ {
		prev := ac3Ranges[i-1]
		cur := ac3Ranges[i]
		if cur.ESOffset != prev.ESOffset+int64(prev.Size) {
			t.Errorf("AC3 range[%d] ESOffset = %d, want %d", i, cur.ESOffset, prev.ESOffset+int64(prev.Size))
		}
	}
	for i := 1; i < len(truehdRanges); i++ {
		prev := truehdRanges[i-1]
		cur := truehdRanges[i]
		if cur.ESOffset != prev.ESOffset+int64(prev.Size) {
			t.Errorf("TrueHD range[%d] ESOffset = %d, want %d", i, cur.ESOffset, prev.ESOffset+int64(prev.Size))
		}
	}
}

func TestMPEGTSParser_SubtitleSubStreams(t *testing.T) {
	// Build M2TS with video + 2 audio + 1 PGS subtitle
	data := buildTestM2TS(t, []testStream{
		{0x1B}, // H.264 video
		{0x81}, // AC3 audio → sub-stream 0
		{0x83}, // TrueHD audio → sub-stream 1
		{0x90}, // PGS subtitle → sub-stream 2
	})

	parser := NewMPEGTSParser(data)
	if err := parser.ParseWithProgress(nil); err != nil {
		t.Fatalf("ParseWithProgress: %v", err)
	}

	// AudioSubStreams should include all 3 non-video sub-streams
	allSubs := parser.AudioSubStreams()
	if len(allSubs) != 3 {
		t.Fatalf("AudioSubStreams() = %d, want 3", len(allSubs))
	}

	// SubtitleSubStreams should return only the PGS sub-stream
	subtitleSubs := parser.SubtitleSubStreams()
	if len(subtitleSubs) != 1 {
		t.Fatalf("SubtitleSubStreams() = %d, want 1", len(subtitleSubs))
	}
	if subtitleSubs[0] != 2 {
		t.Errorf("SubtitleSubStreams()[0] = %d, want 2", subtitleSubs[0])
	}

	// Verify the subtitle sub-stream codec type
	if parser.subStreamCodec[subtitleSubs[0]] != CodecPGSSubtitle {
		t.Errorf("subStreamCodec[%d] = %v, want CodecPGSSubtitle", subtitleSubs[0], parser.subStreamCodec[subtitleSubs[0]])
	}
}
