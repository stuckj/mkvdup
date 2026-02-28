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

func TestMPEGTSParser_DTSHDCoreSplit(t *testing.T) {
	data := buildDTSHDCoreM2TSData()
	p := NewMPEGTSParser(data)
	if err := p.Parse(); err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	// Should have 2 audio sub-streams: original DTS-HD combined (0) + DTS core (1)
	if got := p.AudioSubStreamCount(); got != 2 {
		t.Fatalf("AudioSubStreamCount = %d, want 2 (DTS-HD combined + DTS core)", got)
	}

	subs := p.AudioSubStreams()

	// Sub-stream 0 should be the original combined DTS-HD stream (preserved)
	combinedSize := p.AudioSubStreamESSize(subs[0])
	// Sub-stream 1 should be the extracted DTS core
	coreSize := p.AudioSubStreamESSize(subs[1])

	// Combined should be 911 bytes (total payload)
	if combinedSize != 911 {
		t.Errorf("Combined DTS-HD sub-stream size = %d, want 911", combinedSize)
	}

	// DTS core: 2 × 256 = 512 bytes
	if coreSize != 512 {
		t.Errorf("DTS core sub-stream size = %d, want 512 (2 × 256)", coreSize)
	}

	// Verify core codec is DTS (not DTS-HD)
	if p.subStreamCodec[subs[1]] != CodecDTSAudio {
		t.Errorf("DTS core sub-stream codec = %v, want CodecDTSAudio", p.subStreamCodec[subs[1]])
	}

	// Verify combined codec is still DTS-HD
	if p.subStreamCodec[subs[0]] != CodecDTSHDAudio {
		t.Errorf("Combined sub-stream codec = %v, want CodecDTSHDAudio", p.subStreamCodec[subs[0]])
	}

	// Verify core data starts with DTS sync word
	coreData, err := p.ReadAudioSubStreamData(subs[1], 0, 4)
	if err != nil {
		t.Fatalf("ReadAudioSubStreamData(core) error: %v", err)
	}
	if coreData[0] != 0x7F || coreData[1] != 0xFE || coreData[2] != 0x80 || coreData[3] != 0x01 {
		t.Errorf("DTS core data starts with [%02X %02X %02X %02X], want [7F FE 80 01]",
			coreData[0], coreData[1], coreData[2], coreData[3])
	}
}

func TestMPEGTSParser_DTSHDCoreSplit_CorrectFSIZE(t *testing.T) {
	// Test with FSIZE matching core-only size (some encoders may set FSIZE correctly).
	// The detection should still work: ExSS sync marks the core boundary regardless.
	data := buildDTSHDCoreM2TSData_CorrectFSIZE()
	p := NewMPEGTSParser(data)
	if err := p.Parse(); err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if got := p.AudioSubStreamCount(); got != 2 {
		t.Fatalf("AudioSubStreamCount = %d, want 2", got)
	}

	subs := p.AudioSubStreams()
	coreSize := p.AudioSubStreamESSize(subs[1])

	// DTS core: 2 × 256 = 512 bytes (same as inflated FSIZE test)
	if coreSize != 512 {
		t.Errorf("DTS core sub-stream size = %d, want 512 (2 × 256)", coreSize)
	}

	// Verify core data starts with DTS sync word
	coreData, err := p.ReadAudioSubStreamData(subs[1], 0, 4)
	if err != nil {
		t.Fatalf("ReadAudioSubStreamData(core) error: %v", err)
	}
	if coreData[0] != 0x7F || coreData[1] != 0xFE || coreData[2] != 0x80 || coreData[3] != 0x01 {
		t.Errorf("DTS core data starts with [%02X %02X %02X %02X], want [7F FE 80 01]",
			coreData[0], coreData[1], coreData[2], coreData[3])
	}
}

func TestMPEGTSParser_NoSplitForPureDTS(t *testing.T) {
	// A stream with type 0x82 (pure DTS core, not DTS-HD) should NOT be split
	data := buildTestM2TS(t, []testStream{
		{0x1B}, // H.264 video
		{0x82}, // DTS core audio
	})

	p := NewMPEGTSParser(data)
	if err := p.Parse(); err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	// Should have just 1 audio sub-stream (no split for non-DTS-HD)
	if got := p.AudioSubStreamCount(); got != 1 {
		t.Errorf("AudioSubStreamCount = %d, want 1 (no split for pure DTS core)", got)
	}
}

func TestDetectCombinedDTSHDCore(t *testing.T) {
	// Data with both DTS core and DTS-HD ExSS sync words
	combined := make([]byte, 512)
	combined[0] = 0x7F // DTS core sync
	combined[1] = 0xFE
	combined[2] = 0x80
	combined[3] = 0x01
	combined[200] = 0x64 // DTS-HD ExSS sync
	combined[201] = 0x58
	combined[202] = 0x20
	combined[203] = 0x25

	p := &MPEGTSParser{
		data: combined,
		size: int64(len(combined)),
	}
	ranges := []PESPayloadRange{
		{FileOffset: 0, Size: 512, ESOffset: 0},
	}

	if !p.detectCombinedDTSHDCore(ranges) {
		t.Error("detectCombinedDTSHDCore() = false, want true for data with both sync words")
	}

	// Data with only DTS core sync
	coreOnly := make([]byte, 512)
	coreOnly[0] = 0x7F
	coreOnly[1] = 0xFE
	coreOnly[2] = 0x80
	coreOnly[3] = 0x01
	p2 := &MPEGTSParser{data: coreOnly, size: int64(len(coreOnly))}
	if p2.detectCombinedDTSHDCore(ranges) {
		t.Error("detectCombinedDTSHDCore() = true, want false for DTS-core-only data")
	}

	// Data with only ExSS sync
	exssOnly := make([]byte, 512)
	exssOnly[0] = 0x64
	exssOnly[1] = 0x58
	exssOnly[2] = 0x20
	exssOnly[3] = 0x25
	p3 := &MPEGTSParser{data: exssOnly, size: int64(len(exssOnly))}
	if p3.detectCombinedDTSHDCore(ranges) {
		t.Error("detectCombinedDTSHDCore() = true, want false for ExSS-only data")
	}
}

func TestSplitDTSHDCoreRanges(t *testing.T) {
	// Build combined payload: DTSCore(256) + ExSS(128) + DTSCore(256) + ExSS(100)
	var payload []byte
	payload = append(payload, makeDTSCoreFrame(256, 0x11)...)  // 256 bytes core
	payload = append(payload, makeDTSHDExSSUnit(128, 0x22)...) // 128 bytes extension
	payload = append(payload, makeDTSCoreFrame(256, 0x33)...)  // 256 bytes core
	payload = append(payload, makeDTSHDExSSUnit(100, 0x44)...) // 100 bytes extension
	// Total: 740 bytes

	p := &MPEGTSParser{
		data: payload,
		size: int64(len(payload)),
	}

	ranges := []PESPayloadRange{
		{FileOffset: 0, Size: len(payload), ESOffset: 0},
	}

	coreRanges := p.splitDTSHDCoreRanges(ranges)

	var coreTotal int64
	for _, r := range coreRanges {
		coreTotal += int64(r.Size)
	}

	if coreTotal != 512 {
		t.Errorf("DTS core total size = %d, want 512 (2 × 256)", coreTotal)
	}

	// Verify ES offsets are sequential
	for i := 1; i < len(coreRanges); i++ {
		prev := coreRanges[i-1]
		cur := coreRanges[i]
		if cur.ESOffset != prev.ESOffset+int64(prev.Size) {
			t.Errorf("core range[%d] ESOffset = %d, want %d", i, cur.ESOffset, prev.ESOffset+int64(prev.Size))
		}
	}
}

func TestSplitDTSHDCoreRanges_CrossRange(t *testing.T) {
	// Build combined payload: DTSCore(256) + ExSS(128) + DTSCore(256) + ExSS(100)
	var payload []byte
	payload = append(payload, makeDTSCoreFrame(256, 0x11)...)  // 256 bytes core
	payload = append(payload, makeDTSHDExSSUnit(128, 0x22)...) // 128 bytes extension
	payload = append(payload, makeDTSCoreFrame(256, 0x33)...)  // 256 bytes core
	payload = append(payload, makeDTSHDExSSUnit(100, 0x44)...) // 100 bytes extension
	// Total: 740 bytes

	p := &MPEGTSParser{
		data: payload,
		size: int64(len(payload)),
	}

	// Split into ranges that force DTS core header to straddle boundary.
	// Second core frame starts at offset 384. Put range boundary at 386
	// so the sync word 7F FE 80 01 and frame size are split across ranges.
	ranges := []PESPayloadRange{
		{FileOffset: 0, Size: 386, ESOffset: 0},     // ends mid-header of second DTS core
		{FileOffset: 386, Size: 354, ESOffset: 386}, // rest
	}

	coreRanges := p.splitDTSHDCoreRanges(ranges)

	var coreTotal int64
	for _, r := range coreRanges {
		coreTotal += int64(r.Size)
	}

	if coreTotal != 512 {
		t.Errorf("DTS core total size = %d, want 512 (2 × 256)", coreTotal)
	}

	// Verify ES offsets are sequential
	for i := 1; i < len(coreRanges); i++ {
		prev := coreRanges[i-1]
		cur := coreRanges[i]
		if cur.ESOffset != prev.ESOffset+int64(prev.Size) {
			t.Errorf("core range[%d] ESOffset = %d, want %d", i, cur.ESOffset, prev.ESOffset+int64(prev.Size))
		}
	}
}

func TestSplitDTSHDCoreRanges_SyncNearRangeEnd(t *testing.T) {
	// Regression test: ensure DTS core sync words starting 5-6 bytes before
	// a range boundary are detected. DTSCoreFrameSize() needs 7 bytes, so
	// the lookback window must cover at least 6 bytes from the end.
	var payload []byte
	payload = append(payload, makeDTSHDExSSUnit(128, 0x11)...) // 128 bytes extension
	payload = append(payload, makeDTSCoreFrame(256, 0x22)...)  // 256 bytes core at offset 128
	payload = append(payload, makeDTSHDExSSUnit(100, 0x33)...) // 100 bytes extension
	// Total: 484 bytes

	p := &MPEGTSParser{
		data: payload,
		size: int64(len(payload)),
	}

	// Place range boundary 5 bytes after the DTS core sync word starts.
	// Core frame starts at offset 128. Boundary at 133 means the sync word
	// (bytes 128-131) plus one header byte are in range[0], but the remaining
	// 2 header bytes needed for frame size are in range[1].
	ranges := []PESPayloadRange{
		{FileOffset: 0, Size: 133, ESOffset: 0},     // has 0x7F at [128], 5 bytes of header
		{FileOffset: 133, Size: 351, ESOffset: 133}, // rest of core + extension
	}

	coreRanges := p.splitDTSHDCoreRanges(ranges)

	var coreTotal int64
	for _, r := range coreRanges {
		coreTotal += int64(r.Size)
	}

	if coreTotal != 256 {
		t.Errorf("DTS core total size = %d, want 256", coreTotal)
	}

	// Verify ES offsets are sequential
	for i := 1; i < len(coreRanges); i++ {
		prev := coreRanges[i-1]
		cur := coreRanges[i]
		if cur.ESOffset != prev.ESOffset+int64(prev.Size) {
			t.Errorf("core range[%d] ESOffset = %d, want %d", i, cur.ESOffset, prev.ESOffset+int64(prev.Size))
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
