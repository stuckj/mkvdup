package source

import (
	"testing"
)

func TestMPEGTSParser_BasicParsing(t *testing.T) {
	data := buildBasicM2TSData()
	p := NewMPEGTSParser(data)
	if err := p.Parse(); err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if p.VideoPID() != 0x1011 {
		t.Errorf("VideoPID = 0x%04X, want 0x1011", p.VideoPID())
	}
	if p.VideoCodec() != CodecH264Video {
		t.Errorf("VideoCodec = %v, want CodecH264Video", p.VideoCodec())
	}
	if got := len(p.AudioPIDs()); got != 1 {
		t.Fatalf("AudioPIDs count = %d, want 1", got)
	}
	if p.AudioPIDs()[0] != 0x1101 {
		t.Errorf("AudioPIDs[0] = 0x%04X, want 0x1101", p.AudioPIDs()[0])
	}
	if got := len(p.AudioSubStreams()); got != 1 {
		t.Fatalf("AudioSubStreams count = %d, want 1", got)
	}

	// Video: 175 (PUSI) + 184 (cont) + 175 (PUSI) = 534
	if got := p.TotalESSize(true); got != 534 {
		t.Errorf("TotalESSize(video) = %d, want 534", got)
	}
	if got := p.AudioSubStreamESSize(0); got != 175 {
		t.Errorf("AudioSubStreamESSize(0) = %d, want 175", got)
	}

	// Verify video ranges
	vr := p.FilteredVideoRanges()
	if len(vr) != 3 {
		t.Fatalf("video ranges count = %d, want 3", len(vr))
	}
	expectedRanges := []PESPayloadRange{
		{FileOffset: 401, Size: 175, ESOffset: 0},
		{FileOffset: 584, Size: 184, ESOffset: 175},
		{FileOffset: 977, Size: 175, ESOffset: 359},
	}
	for i, exp := range expectedRanges {
		if vr[i] != exp {
			t.Errorf("videoRanges[%d] = %+v, want %+v", i, vr[i], exp)
		}
	}

	// Verify audio ranges
	ar := p.FilteredAudioRanges(0)
	if len(ar) != 1 || ar[0] != (PESPayloadRange{FileOffset: 785, Size: 175, ESOffset: 0}) {
		t.Errorf("audioRanges[0] = %+v, want {785, 175, 0}", ar[0])
	}

	// H.264 should not filter user_data — filtered == raw
	if &p.filteredVideoRanges[0] != &p.videoRanges[0] {
		t.Error("H.264 filteredVideoRanges should be same slice as videoRanges")
	}
}

func TestMPEGTSParser_AdaptFieldPayload(t *testing.T) {
	const (
		pmtPID   = uint16(0x0100)
		videoPID = uint16(0x1011)
	)

	var data []byte
	data = append(data, makeM2TSPacket(0, true, 0x01, 0, 0, makePATPayload(pmtPID))...)
	data = append(data, makeM2TSPacket(pmtPID, true, 0x01, 0, 0,
		makePMTPayload(videoPID, 0x1B, nil, nil))...)

	// Video PUSI with adaptation field (10 bytes)
	// TS payload = 192 - 8 - 1 - 10 = 173 bytes; ES = 173 - 9 = 164 bytes
	data = append(data, makeM2TSPacket(videoPID, true, 0x03, 10, 1,
		makePESStart(0xE0, 0, seqBytes(0, 164)))...)

	// Continuation with adaptation field (5 bytes)
	// TS payload = 192 - 8 - 1 - 5 = 178 bytes
	data = append(data, makeM2TSPacket(videoPID, false, 0x03, 5, 2,
		seqBytes(164, 178))...)

	p := NewMPEGTSParser(data)
	if err := p.Parse(); err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if got := p.TotalESSize(true); got != 342 {
		t.Errorf("TotalESSize = %d, want 342", got)
	}

	// Verify reading across the boundary
	esData, err := p.ReadESData(162, 4, true)
	if err != nil {
		t.Fatalf("ReadESData() error: %v", err)
	}
	expected := []byte{162, 163, 164, 165}
	for i, b := range esData {
		if b != expected[i] {
			t.Errorf("byte[%d] = %d, want %d", i, b, expected[i])
		}
	}
}

func TestMPEGTSParser_AdaptFieldOnly(t *testing.T) {
	const (
		pmtPID   = uint16(0x0100)
		videoPID = uint16(0x1011)
	)

	var data []byte
	data = append(data, makeM2TSPacket(0, true, 0x01, 0, 0, makePATPayload(pmtPID))...)
	data = append(data, makeM2TSPacket(pmtPID, true, 0x01, 0, 0,
		makePMTPayload(videoPID, 0x1B, nil, nil))...)

	// Video PUSI
	data = append(data, makeM2TSPacket(videoPID, true, 0x01, 0, 1,
		makePESStart(0xE0, 0, seqBytes(0, 175)))...)

	// Adaptation-only packet (AFC=0x02) — should be skipped
	data = append(data, makeM2TSPacket(videoPID, false, 0x02, 183, 2, nil)...)

	// Video continuation
	data = append(data, makeM2TSPacket(videoPID, false, 0x01, 0, 3, seqBytes(175, 184))...)

	p := NewMPEGTSParser(data)
	if err := p.Parse(); err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	// Only 2 ranges (adapt-only packet skipped)
	if got := len(p.FilteredVideoRanges()); got != 2 {
		t.Errorf("video ranges count = %d, want 2", got)
	}
	if got := p.TotalESSize(true); got != 359 {
		t.Errorf("TotalESSize = %d, want 359", got)
	}
}

func TestMPEGTSParser_PESHeaderSpanning(t *testing.T) {
	const (
		pmtPID   = uint16(0x0100)
		videoPID = uint16(0x1011)
	)

	var data []byte
	data = append(data, makeM2TSPacket(0, true, 0x01, 0, 0, makePATPayload(pmtPID))...)
	data = append(data, makeM2TSPacket(pmtPID, true, 0x01, 0, 0,
		makePMTPayload(videoPID, 0x1B, nil, nil))...)

	// PUSI packet with PES header that spans beyond the packet.
	// PES header = 9 + 180 = 189 bytes, but TS payload = 184 bytes.
	// Spill: 189 - 184 = 5 bytes into next packet.
	pesPayload := makePESStart(0xE0, 180, nil)
	data = append(data, makeM2TSPacket(videoPID, true, 0x01, 0, 1, pesPayload[:184])...)

	// Continuation: first 5 bytes are remaining PES header, then 179 bytes ES data
	contPayload := make([]byte, 184)
	for i := 5; i < 184; i++ {
		contPayload[i] = byte((i - 5) & 0xFF)
	}
	data = append(data, makeM2TSPacket(videoPID, false, 0x01, 0, 2, contPayload)...)

	p := NewMPEGTSParser(data)
	if err := p.Parse(); err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if got := p.TotalESSize(true); got != 179 {
		t.Errorf("TotalESSize = %d, want 179", got)
	}

	// Verify ES data
	esData, err := p.ReadESData(0, 5, true)
	if err != nil {
		t.Fatalf("ReadESData() error: %v", err)
	}
	for i := 0; i < 5; i++ {
		if esData[i] != byte(i) {
			t.Errorf("ES byte[%d] = %d, want %d", i, esData[i], i)
		}
	}
}

func TestMPEGTSParser_MultipleAudioStreams(t *testing.T) {
	const (
		pmtPID    = uint16(0x0100)
		videoPID  = uint16(0x1011)
		audioPID1 = uint16(0x1101)
		audioPID2 = uint16(0x1102)
	)

	var data []byte
	data = append(data, makeM2TSPacket(0, true, 0x01, 0, 0, makePATPayload(pmtPID))...)
	data = append(data, makeM2TSPacket(pmtPID, true, 0x01, 0, 0,
		makePMTPayload(videoPID, 0x1B,
			[]uint16{audioPID1, audioPID2},
			[]byte{0x81, 0x82}))...) // AC3, DTS

	// Video PUSI
	data = append(data, makeM2TSPacket(videoPID, true, 0x01, 0, 1,
		makePESStart(0xE0, 0, seqBytes(0, 175)))...)

	// Audio 1 PUSI
	data = append(data, makeM2TSPacket(audioPID1, true, 0x01, 0, 0,
		makePESStart(0xFD, 0, seqBytes(0xA0, 175)))...)

	// Audio 2 PUSI
	data = append(data, makeM2TSPacket(audioPID2, true, 0x01, 0, 0,
		makePESStart(0xFD, 0, seqBytes(0xC0, 175)))...)

	p := NewMPEGTSParser(data)
	if err := p.Parse(); err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if got := p.AudioSubStreamCount(); got != 2 {
		t.Fatalf("AudioSubStreamCount = %d, want 2", got)
	}

	subs := p.AudioSubStreams()
	if subs[0] != 0 || subs[1] != 1 {
		t.Errorf("AudioSubStreams = %v, want [0, 1]", subs)
	}

	if got := p.AudioSubStreamESSize(0); got != 175 {
		t.Errorf("audio sub-stream 0 size = %d, want 175", got)
	}
	if got := p.AudioSubStreamESSize(1); got != 175 {
		t.Errorf("audio sub-stream 1 size = %d, want 175", got)
	}

	// Verify audio data is from the correct stream
	d1, err := p.ReadAudioSubStreamData(0, 0, 3)
	if err != nil {
		t.Fatalf("ReadAudioSubStreamData(0) error: %v", err)
	}
	if d1[0] != 0xA0 || d1[1] != 0xA1 || d1[2] != 0xA2 {
		t.Errorf("audio 0 data = %v, want [0xA0, 0xA1, 0xA2]", d1[:3])
	}

	d2, err := p.ReadAudioSubStreamData(1, 0, 3)
	if err != nil {
		t.Fatalf("ReadAudioSubStreamData(1) error: %v", err)
	}
	if d2[0] != 0xC0 || d2[1] != 0xC1 || d2[2] != 0xC2 {
		t.Errorf("audio 1 data = %v, want [0xC0, 0xC1, 0xC2]", d2[:3])
	}
}

func TestMPEGTSParser_StandardTS188(t *testing.T) {
	const (
		pmtPID   = uint16(0x0100)
		videoPID = uint16(0x1011)
	)

	var data []byte
	data = append(data, makeTSPacket(0, true, 0x01, 0, 0, makePATPayload(pmtPID))...)
	data = append(data, makeTSPacket(pmtPID, true, 0x01, 0, 0,
		makePMTPayload(videoPID, 0x1B, nil, nil))...)
	data = append(data, makeTSPacket(videoPID, true, 0x01, 0, 1,
		makePESStart(0xE0, 0, seqBytes(0, 175)))...)
	data = append(data, makeTSPacket(videoPID, false, 0x01, 0, 2, seqBytes(175, 184))...)

	p := NewMPEGTSParser(data)
	if err := p.Parse(); err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if p.packetSize != 188 {
		t.Errorf("packetSize = %d, want 188", p.packetSize)
	}
	if p.tsOffset != 0 {
		t.Errorf("tsOffset = %d, want 0", p.tsOffset)
	}
	if got := p.TotalESSize(true); got != 359 {
		t.Errorf("TotalESSize = %d, want 359", got)
	}

	// Verify file offsets differ from M2TS (no 4-byte timestamp prefix)
	vr := p.FilteredVideoRanges()
	if len(vr) != 2 {
		t.Fatalf("video ranges count = %d, want 2", len(vr))
	}
	// Packet 2 at pos=376: ES at 376+4+9=389
	if vr[0].FileOffset != 389 {
		t.Errorf("vr[0].FileOffset = %d, want 389", vr[0].FileOffset)
	}
	// Packet 3 at pos=564: ES at 564+4=568
	if vr[1].FileOffset != 568 {
		t.Errorf("vr[1].FileOffset = %d, want 568", vr[1].FileOffset)
	}
}

func TestMPEGTSParser_NoStreams(t *testing.T) {
	const pmtPID = uint16(0x0100)

	var data []byte
	data = append(data, makeM2TSPacket(0, true, 0x01, 0, 0, makePATPayload(pmtPID))...)
	// PMT with no video and no audio streams
	data = append(data, makeM2TSPacket(pmtPID, true, 0x01, 0, 0,
		makePMTPayload(0, 0, nil, nil))...)
	data = append(data, makeM2TSPacket(0, false, 0x01, 0, 1, nil)...)
	data = append(data, makeM2TSPacket(0, false, 0x01, 0, 2, nil)...)

	p := NewMPEGTSParser(data)
	err := p.Parse()
	if err == nil {
		t.Fatal("expected error for no streams")
	}
}

func TestMPEGTSParser_ProgressCallback(t *testing.T) {
	data := buildBasicM2TSData()
	p := NewMPEGTSParser(data)

	var finalProcessed, finalTotal int64
	called := false
	err := p.ParseWithProgress(func(processed, total int64) {
		called = true
		finalProcessed = processed
		finalTotal = total
	})
	if err != nil {
		t.Fatalf("ParseWithProgress() error: %v", err)
	}

	if !called {
		t.Error("progress callback was never called")
	}
	if finalTotal != int64(len(data)) {
		t.Errorf("final total = %d, want %d", finalTotal, len(data))
	}
	if finalProcessed != finalTotal {
		t.Errorf("final processed = %d, want %d (should equal total at completion)", finalProcessed, finalTotal)
	}
}

// --- Tests for mergeAdjacentRanges ---

func TestMergeAdjacentRanges(t *testing.T) {
	tests := []struct {
		name   string
		input  []PESPayloadRange
		expect []PESPayloadRange
	}{
		{
			name:   "empty",
			input:  nil,
			expect: nil,
		},
		{
			name:   "single",
			input:  []PESPayloadRange{{FileOffset: 100, Size: 50, ESOffset: 0}},
			expect: []PESPayloadRange{{FileOffset: 100, Size: 50, ESOffset: 0}},
		},
		{
			name: "two contiguous",
			input: []PESPayloadRange{
				{FileOffset: 100, Size: 50, ESOffset: 0},
				{FileOffset: 150, Size: 30, ESOffset: 50},
			},
			expect: []PESPayloadRange{
				{FileOffset: 100, Size: 80, ESOffset: 0},
			},
		},
		{
			name: "two non-contiguous file offset",
			input: []PESPayloadRange{
				{FileOffset: 100, Size: 50, ESOffset: 0},
				{FileOffset: 200, Size: 30, ESOffset: 50},
			},
			expect: []PESPayloadRange{
				{FileOffset: 100, Size: 50, ESOffset: 0},
				{FileOffset: 200, Size: 30, ESOffset: 50},
			},
		},
		{
			name: "two non-contiguous ES offset",
			input: []PESPayloadRange{
				{FileOffset: 100, Size: 50, ESOffset: 0},
				{FileOffset: 150, Size: 30, ESOffset: 100},
			},
			expect: []PESPayloadRange{
				{FileOffset: 100, Size: 50, ESOffset: 0},
				{FileOffset: 150, Size: 30, ESOffset: 100},
			},
		},
		{
			name: "three ranges merge first two",
			input: []PESPayloadRange{
				{FileOffset: 100, Size: 50, ESOffset: 0},
				{FileOffset: 150, Size: 30, ESOffset: 50},
				{FileOffset: 300, Size: 20, ESOffset: 80},
			},
			expect: []PESPayloadRange{
				{FileOffset: 100, Size: 80, ESOffset: 0},
				{FileOffset: 300, Size: 20, ESOffset: 80},
			},
		},
		{
			name: "all three merge",
			input: []PESPayloadRange{
				{FileOffset: 100, Size: 50, ESOffset: 0},
				{FileOffset: 150, Size: 30, ESOffset: 50},
				{FileOffset: 180, Size: 20, ESOffset: 80},
			},
			expect: []PESPayloadRange{
				{FileOffset: 100, Size: 100, ESOffset: 0},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeAdjacentRanges(tt.input)
			if len(got) != len(tt.expect) {
				t.Fatalf("mergeAdjacentRanges() returned %d ranges, want %d", len(got), len(tt.expect))
			}
			for i, r := range got {
				if r != tt.expect[i] {
					t.Errorf("range[%d] = %+v, want %+v", i, r, tt.expect[i])
				}
			}
		})
	}
}

func TestMPEGTSParser_MultiPacketPMT(t *testing.T) {
	// Build M2TS data where the PMT spans two TS packets.
	// With 1 video + 8 audio + 4 PGS streams = 13 streams × 5 bytes = 65 bytes of stream
	// descriptors, plus section header overhead. This exceeds a single TS payload when
	// ES info descriptors are included, forcing multi-packet reassembly.

	const packetSize = 192
	const tsPayloadStart = 4

	// We'll build: 1 video (0x1B) + 8 audio (0x81) + 4 PGS (0x90) = 13 streams
	// PMT section: table_id(1) + section_length(2) + TSID(2) + version(1) + section_num(1) +
	//   last_section(1) + PCR_PID(2) + prog_info_len(2) = 12 bytes header
	//   + 13 * 5 = 65 bytes streams = 77 bytes + 4 CRC = 81 bytes total section
	// That fits in one packet, so let's add ES info descriptors to make it larger.
	// Instead, we manually build a PMT that spans two packets by using ES info descriptors.

	// Actually, let's just create enough streams: 1 video + 8 audio + 17 PGS = 26 streams
	// Section: 12 header + 26*5 streams + 4 CRC = 146 bytes
	// TS payload with pointer field: 184 - 1 = 183 bytes available
	// Section data (146 bytes) fits in one packet. We need ES info descriptors.
	// Let's add 6-byte ES info descriptors per audio stream to push it over.

	numAudio := 8
	numPGS := 17
	numStreams := 1 + numAudio + numPGS // 26

	// Build PMT section manually with ES info descriptors on audio streams
	// to push the section past 184 bytes
	pmtSection := make([]byte, 0, 300)
	pmtSection = append(pmtSection, 0x02) // table_id
	// Placeholder for section_length (fill later)
	pmtSection = append(pmtSection, 0xB0, 0x00)
	pmtSection = append(pmtSection, 0x00, 0x01) // TSID
	pmtSection = append(pmtSection, 0xC1)       // version 0, current
	pmtSection = append(pmtSection, 0x00, 0x00) // section/last section
	pmtSection = append(pmtSection, 0xE1, 0x01) // PCR PID = 0x101
	pmtSection = append(pmtSection, 0xF0, 0x00) // prog_info_len = 0

	// Video stream (PID 0x101)
	pmtSection = append(pmtSection, 0x1B)       // H.264
	pmtSection = append(pmtSection, 0xE1, 0x01) // PID 0x101
	pmtSection = append(pmtSection, 0xF0, 0x00) // ES info len = 0

	// Audio streams (PIDs 0x1100-0x1107) with 6-byte descriptors each
	for i := 0; i < numAudio; i++ {
		pid := 0x1100 + i
		pmtSection = append(pmtSection, 0x81)                           // AC3
		pmtSection = append(pmtSection, 0xE0|byte(pid>>8), byte(pid))   // PID
		pmtSection = append(pmtSection, 0xF0, 0x06)                     // ES info len = 6
		pmtSection = append(pmtSection, 0x05, 0x04, 'A', 'C', '-', '3') // registration descriptor
	}

	// PGS subtitle streams (PIDs 0x1200-0x1210)
	for i := 0; i < numPGS; i++ {
		pid := 0x1200 + i
		pmtSection = append(pmtSection, 0x90)                         // PGS
		pmtSection = append(pmtSection, 0xE0|byte(pid>>8), byte(pid)) // PID
		pmtSection = append(pmtSection, 0xF0, 0x00)                   // ES info len = 0
	}

	// CRC placeholder
	pmtSection = append(pmtSection, 0x00, 0x00, 0x00, 0x00)

	// Fill section_length (total bytes after section_length field, including CRC)
	sectionLen := len(pmtSection) - 3
	pmtSection[1] = 0xB0 | byte(sectionLen>>8)
	pmtSection[2] = byte(sectionLen)

	// Verify it's too large for one TS packet payload
	// TS payload = 188 - 4 (TS header) - 1 (pointer field) = 183 bytes
	if len(pmtSection) <= 183 {
		t.Fatalf("PMT section %d bytes fits in one packet; test needs larger PMT", len(pmtSection))
	}

	// Build M2TS packets
	// We need: PAT + PMT pkt1 (PUSI) + PMT pkt2 (continuation) + padding packets
	numPackets := 8 // PAT + 2 PMT + 5 padding for sync detection
	data := make([]byte, packetSize*numPackets)

	// Initialize all as null packets
	for i := 0; i < numPackets; i++ {
		off := i*packetSize + tsPayloadStart
		data[off] = 0x47
		data[off+1] = 0x1F
		data[off+2] = 0xFF
		data[off+3] = 0x10
	}

	// Packet 0: PAT
	pat := data[tsPayloadStart:]
	pat[0] = 0x47
	pat[1] = 0x40 // PUSI, PID 0
	pat[2] = 0x00
	pat[3] = 0x10 // payload only
	pat[4] = 0x00 // pointer field
	pat[5] = 0x00 // table_id = PAT
	pat[6] = 0xB0
	pat[7] = 0x0D // section_length = 13
	pat[8] = 0x00
	pat[9] = 0x01
	pat[10] = 0xC1
	pat[11] = 0x00
	pat[12] = 0x00
	pat[13] = 0x00
	pat[14] = 0x01
	pat[15] = 0xE1 // PMT PID = 0x100
	pat[16] = 0x00

	// Packet 1: PMT first packet (PUSI)
	pmt1 := data[packetSize+tsPayloadStart:]
	pmt1[0] = 0x47
	pmt1[1] = 0x41 // PUSI, PID 0x100
	pmt1[2] = 0x00
	pmt1[3] = 0x10                  // payload only, CC=0
	pmt1[4] = 0x00                  // pointer field
	firstPayloadSize := 188 - 4 - 1 // 183 bytes
	copy(pmt1[5:], pmtSection[:firstPayloadSize])

	// Packet 2: PMT continuation
	pmt2 := data[2*packetSize+tsPayloadStart:]
	pmt2[0] = 0x47
	pmt2[1] = 0x01 // no PUSI, PID 0x100
	pmt2[2] = 0x00
	pmt2[3] = 0x11 // payload only, CC=1
	remaining := pmtSection[firstPayloadSize:]
	copy(pmt2[4:], remaining)

	// Now add PES data packets for at least the first audio and PGS PIDs
	// so the parser has some data to work with (not strictly needed for PMT test,
	// but ensures the full parse succeeds)

	parser := NewMPEGTSParser(data)
	if err := parser.ParseWithProgress(nil); err != nil {
		t.Fatalf("ParseWithProgress: %v", err)
	}

	// Verify video was detected (PID 0x101 as set in the PMT section above)
	if got := parser.VideoPID(); got != 0x0101 {
		t.Fatalf("VideoPID() = 0x%x, want 0x0101", got)
	}

	// Verify all audio + subtitle sub-streams were detected
	allSubs := parser.AudioSubStreams()
	wantTotal := numAudio + numPGS
	if len(allSubs) != wantTotal {
		t.Fatalf("AudioSubStreams() = %v (len %d), want %d sub-streams (%d audio + %d PGS)",
			allSubs, len(allSubs), wantTotal, numAudio, numPGS)
	}

	// Verify subtitle sub-streams
	subtitleSubs := parser.SubtitleSubStreams()
	if len(subtitleSubs) != numPGS {
		t.Fatalf("SubtitleSubStreams() = %v (len %d), want %d PGS sub-streams",
			subtitleSubs, len(subtitleSubs), numPGS)
	}

	// Verify all subtitle sub-streams have PGS codec
	for _, id := range subtitleSubs {
		if parser.subStreamCodec[id] != CodecPGSSubtitle {
			t.Errorf("subStreamCodec[%d] = %v, want CodecPGSSubtitle", id, parser.subStreamCodec[id])
		}
	}

	_ = numStreams
}
