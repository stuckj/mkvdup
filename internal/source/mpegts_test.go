package source

import (
	"testing"
)

// --- Helper functions for building synthetic TS/M2TS data ---

func makeM2TSPacket(pid uint16, pusi bool, afc byte, adaptLen int, cc byte, payload []byte) []byte {
	pkt := make([]byte, 192)
	pkt[4] = 0x47
	pkt[5] = byte(pid>>8) & 0x1F
	if pusi {
		pkt[5] |= 0x40
	}
	pkt[6] = byte(pid & 0xFF)
	pkt[7] = (afc << 4) | (cc & 0x0F)
	off := 8
	if afc == 0x02 || afc == 0x03 {
		pkt[off] = byte(adaptLen)
		off += 1 + adaptLen
	}
	if payload != nil && off < 192 {
		copy(pkt[off:], payload)
	}
	return pkt
}

func makeTSPacket(pid uint16, pusi bool, afc byte, adaptLen int, cc byte, payload []byte) []byte {
	pkt := make([]byte, 188)
	pkt[0] = 0x47
	pkt[1] = byte(pid>>8) & 0x1F
	if pusi {
		pkt[1] |= 0x40
	}
	pkt[2] = byte(pid & 0xFF)
	pkt[3] = (afc << 4) | (cc & 0x0F)
	off := 4
	if afc == 0x02 || afc == 0x03 {
		pkt[off] = byte(adaptLen)
		off += 1 + adaptLen
	}
	if payload != nil && off < 188 {
		copy(pkt[off:], payload)
	}
	return pkt
}

func makePATPayload(pmtPID uint16) []byte {
	p := make([]byte, 184)
	p[0] = 0x00 // pointer_field
	p[1] = 0x00 // table_id = PAT
	sectionLen := 13
	p[2] = 0xB0 | byte((sectionLen>>8)&0x0F)
	p[3] = byte(sectionLen & 0xFF)
	p[4] = 0x00 // TSID
	p[5] = 0x01
	p[6] = 0xC1 // reserved, version=0, current_next=1
	p[7] = 0x00
	p[8] = 0x00
	p[9] = 0x00 // program_number = 1
	p[10] = 0x01
	p[11] = 0xE0 | byte((pmtPID>>8)&0x1F)
	p[12] = byte(pmtPID & 0xFF)
	return p
}

func makePMTPayload(videoPID uint16, videoType byte, audioPIDs []uint16, audioTypes []byte) []byte {
	p := make([]byte, 184)
	p[0] = 0x00 // pointer_field
	p[1] = 0x02 // table_id = PMT
	numStreams := len(audioPIDs)
	if videoPID != 0 {
		numStreams++
	}
	sectionLen := 9 + numStreams*5 + 4
	p[2] = 0xB0 | byte((sectionLen>>8)&0x0F)
	p[3] = byte(sectionLen & 0xFF)
	p[4] = 0x00
	p[5] = 0x01
	p[6] = 0xC1
	p[7] = 0x00
	p[8] = 0x00
	pcrPID := videoPID
	if pcrPID == 0 && len(audioPIDs) > 0 {
		pcrPID = audioPIDs[0]
	}
	p[9] = 0xE0 | byte((pcrPID>>8)&0x1F)
	p[10] = byte(pcrPID & 0xFF)
	p[11] = 0xF0
	p[12] = 0x00
	off := 13
	if videoPID != 0 {
		p[off] = videoType
		p[off+1] = 0xE0 | byte((videoPID>>8)&0x1F)
		p[off+2] = byte(videoPID & 0xFF)
		p[off+3] = 0xF0
		p[off+4] = 0x00
		off += 5
	}
	for i, aPID := range audioPIDs {
		p[off] = audioTypes[i]
		p[off+1] = 0xE0 | byte((aPID>>8)&0x1F)
		p[off+2] = byte(aPID & 0xFF)
		p[off+3] = 0xF0
		p[off+4] = 0x00
		off += 5
	}
	return p
}

func makePESStart(streamID byte, headerDataLen int, esData []byte) []byte {
	hdrSize := 9 + headerDataLen
	out := make([]byte, hdrSize+len(esData))
	out[0] = 0x00
	out[1] = 0x00
	out[2] = 0x01
	out[3] = streamID
	out[6] = 0x80
	out[8] = byte(headerDataLen)
	copy(out[hdrSize:], esData)
	return out
}

func seqBytes(start, size int) []byte {
	b := make([]byte, size)
	for i := range b {
		b[i] = byte((start + i) & 0xFF)
	}
	return b
}

// buildBasicM2TSData creates synthetic M2TS data:
//
//	Pkt 0 (pos=0):   PAT → PMT PID 0x0100
//	Pkt 1 (pos=192): PMT → H.264 video 0x1011, AC3 audio 0x1101
//	Pkt 2 (pos=384): Video PUSI  - ES[0:175]   file offset 401
//	Pkt 3 (pos=576): Video cont  - ES[175:359] file offset 584
//	Pkt 4 (pos=768): Audio PUSI  - ES[0:175]   file offset 785
//	Pkt 5 (pos=960): Video PUSI  - ES[359:534] file offset 977
func buildBasicM2TSData() []byte {
	const (
		pmtPID   = uint16(0x0100)
		videoPID = uint16(0x1011)
		audioPID = uint16(0x1101)
	)
	var data []byte
	data = append(data, makeM2TSPacket(0, true, 0x01, 0, 0, makePATPayload(pmtPID))...)
	data = append(data, makeM2TSPacket(pmtPID, true, 0x01, 0, 0,
		makePMTPayload(videoPID, 0x1B, []uint16{audioPID}, []byte{0x81}))...)
	data = append(data, makeM2TSPacket(videoPID, true, 0x01, 0, 1,
		makePESStart(0xE0, 0, seqBytes(0, 175)))...)
	data = append(data, makeM2TSPacket(videoPID, false, 0x01, 0, 2, seqBytes(175, 184))...)
	data = append(data, makeM2TSPacket(audioPID, true, 0x01, 0, 0,
		makePESStart(0xFD, 0, seqBytes(0x80, 175)))...)
	// 359 & 0xFF = 103
	data = append(data, makeM2TSPacket(videoPID, true, 0x01, 0, 3,
		makePESStart(0xE0, 0, seqBytes(103, 175)))...)
	return data
}

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

func TestMPEGTSParser_ReadESData(t *testing.T) {
	data := buildBasicM2TSData()
	p := NewMPEGTSParser(data)
	if err := p.Parse(); err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	tests := []struct {
		name     string
		offset   int64
		size     int
		expected []byte
	}{
		{"start", 0, 4, []byte{0, 1, 2, 3}},
		{"cross first boundary", 173, 4, []byte{173, 174, 175, 176}},
		{"in continuation", 200, 3, []byte{200, 201, 202}},
		{"cross second boundary", 357, 4, []byte{101, 102, 103, 104}},
		{"near end", 530, 4, []byte{18, 19, 20, 21}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := p.ReadESData(tt.offset, tt.size, true)
			if err != nil {
				t.Fatalf("ReadESData(%d, %d) error: %v", tt.offset, tt.size, err)
			}
			for i, b := range got {
				if b != tt.expected[i] {
					t.Errorf("byte[%d] = 0x%02X, want 0x%02X", i, b, tt.expected[i])
				}
			}
		})
	}

	// Audio read
	audioData, err := p.ReadAudioSubStreamData(0, 0, 4)
	if err != nil {
		t.Fatalf("ReadAudioSubStreamData() error: %v", err)
	}
	for i := 0; i < 4; i++ {
		exp := byte((0x80 + i) & 0xFF)
		if audioData[i] != exp {
			t.Errorf("audio byte[%d] = 0x%02X, want 0x%02X", i, audioData[i], exp)
		}
	}

	// ReadESData with isVideo=false should error
	if _, err := p.ReadESData(0, 1, false); err == nil {
		t.Error("expected error for ReadESData with isVideo=false")
	}
}

func TestMPEGTSParser_RawRanges(t *testing.T) {
	data := buildBasicM2TSData()
	p := NewMPEGTSParser(data)
	if err := p.Parse(); err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	// Single range
	ranges, err := p.RawRangesForESRegion(10, 50, true)
	if err != nil {
		t.Fatalf("RawRangesForESRegion() error: %v", err)
	}
	if len(ranges) != 1 {
		t.Fatalf("got %d ranges, want 1", len(ranges))
	}
	if ranges[0].FileOffset != 411 || ranges[0].Size != 50 {
		t.Errorf("range = {%d, %d}, want {411, 50}", ranges[0].FileOffset, ranges[0].Size)
	}

	// Cross boundary
	ranges, err = p.RawRangesForESRegion(170, 20, true)
	if err != nil {
		t.Fatalf("RawRangesForESRegion() error: %v", err)
	}
	if len(ranges) != 2 {
		t.Fatalf("got %d ranges, want 2", len(ranges))
	}
	// 5 bytes in range 0 (ES 170-174), 15 bytes in range 1 (ES 175-189)
	if ranges[0].FileOffset != 571 || ranges[0].Size != 5 {
		t.Errorf("range[0] = {%d, %d}, want {571, 5}", ranges[0].FileOffset, ranges[0].Size)
	}
	if ranges[1].FileOffset != 584 || ranges[1].Size != 15 {
		t.Errorf("range[1] = {%d, %d}, want {584, 15}", ranges[1].FileOffset, ranges[1].Size)
	}

	// Audio sub-stream range
	audioRanges, err := p.RawRangesForAudioSubStream(0, 10, 50)
	if err != nil {
		t.Fatalf("RawRangesForAudioSubStream() error: %v", err)
	}
	if len(audioRanges) != 1 {
		t.Fatalf("got %d audio ranges, want 1", len(audioRanges))
	}
	if audioRanges[0].FileOffset != 795 || audioRanges[0].Size != 50 {
		t.Errorf("audio range = {%d, %d}, want {795, 50}", audioRanges[0].FileOffset, audioRanges[0].Size)
	}

	// Error: isVideo=false
	if _, err := p.RawRangesForESRegion(0, 1, false); err == nil {
		t.Error("expected error for RawRangesForESRegion with isVideo=false")
	}

	// Error: unknown audio sub-stream
	if _, err := p.RawRangesForAudioSubStream(99, 0, 1); err == nil {
		t.Error("expected error for unknown audio sub-stream")
	}
}

func TestMPEGTSParser_HintedReading(t *testing.T) {
	data := buildBasicM2TSData()
	p := NewMPEGTSParser(data)
	if err := p.Parse(); err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	// Video: hint valid for range 0
	b, hint, ok := p.ReadESByteWithHint(100, true, 0)
	if !ok {
		t.Fatal("ReadESByteWithHint failed")
	}
	if hint != 0 {
		t.Errorf("hint = %d, want 0", hint)
	}
	if b != 100 {
		t.Errorf("byte = %d, want 100", b)
	}

	// Video: cross boundary from range 0 to range 1
	b, hint, ok = p.ReadESByteWithHint(175, true, 0)
	if !ok {
		t.Fatal("ReadESByteWithHint at boundary failed")
	}
	if hint != 1 {
		t.Errorf("hint = %d, want 1", hint)
	}
	if b != 175 {
		t.Errorf("byte = %d, want 175", b)
	}

	// Video: fallback to binary search (hint=-1)
	b, hint, ok = p.ReadESByteWithHint(400, true, -1)
	if !ok {
		t.Fatal("ReadESByteWithHint with binary search failed")
	}
	if hint != 2 {
		t.Errorf("hint = %d, want 2", hint)
	}
	// 400 & 0xFF = 144
	if b != 144 {
		t.Errorf("byte = %d, want 144", b)
	}

	// Video: out of bounds
	_, _, ok = p.ReadESByteWithHint(600, true, 0)
	if ok {
		t.Error("expected failure for out-of-bounds ES offset")
	}

	// Video: isVideo=false returns failure
	_, _, ok = p.ReadESByteWithHint(0, false, 0)
	if ok {
		t.Error("expected failure for isVideo=false")
	}

	// Audio hint reading
	b, hint, ok = p.ReadAudioByteWithHint(0, 10, 0)
	if !ok {
		t.Fatal("ReadAudioByteWithHint failed")
	}
	if hint != 0 {
		t.Errorf("audio hint = %d, want 0", hint)
	}
	// Audio ES byte 10 = (0x80 + 10) & 0xFF = 0x8A
	if b != 0x8A {
		t.Errorf("audio byte = 0x%02X, want 0x8A", b)
	}
}

func TestMPEGTSParser_ESOffsetToFileOffset(t *testing.T) {
	data := buildBasicM2TSData()
	p := NewMPEGTSParser(data)
	if err := p.Parse(); err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	// In first range
	fo, rem := p.ESOffsetToFileOffset(10, true)
	if fo != 411 || rem != 165 {
		t.Errorf("ESOffsetToFileOffset(10) = (%d, %d), want (411, 165)", fo, rem)
	}

	// Start of second range
	fo, rem = p.ESOffsetToFileOffset(175, true)
	if fo != 584 || rem != 184 {
		t.Errorf("ESOffsetToFileOffset(175) = (%d, %d), want (584, 184)", fo, rem)
	}

	// isVideo=false
	fo, _ = p.ESOffsetToFileOffset(0, false)
	if fo != -1 {
		t.Errorf("ESOffsetToFileOffset(false) = %d, want -1", fo)
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

// --- Tests for TrueHD+AC3 splitting ---

// makeAC3Frame creates a synthetic AC3 frame with sync word 0B 77 at the start.
// fscod=0 (48kHz), frmsizecod=4 → frame size = 192 bytes.
func makeAC3Frame(fillByte byte) []byte {
	frame := make([]byte, 192)
	frame[0] = 0x0B
	frame[1] = 0x77
	frame[2] = 0xAA         // CRC1
	frame[3] = 0xBB         // CRC1
	frame[4] = (0 << 6) | 4 // fscod=0, frmsizecod=4
	for i := 5; i < 192; i++ {
		frame[i] = fillByte
	}
	return frame
}

// makeTrueHDUnit creates a synthetic TrueHD access unit.
// Starts with the TrueHD major sync word: F8 72 6F BA.
func makeTrueHDUnit(size int, fillByte byte) []byte {
	unit := make([]byte, size)
	if size >= 4 {
		unit[0] = 0xF8
		unit[1] = 0x72
		unit[2] = 0x6F
		unit[3] = 0xBA
	}
	for i := 4; i < size; i++ {
		unit[i] = fillByte
	}
	return unit
}

// buildTrueHDAC3M2TSData creates M2TS data with a combined TrueHD+AC3 stream.
// The PES payload contains: [AC3 frame][TrueHD unit][AC3 frame][TrueHD unit]
//
// Payload sizes are chosen so the total exactly fills M2TS packets:
// First PUSI packet carries 175 bytes ES (184 - 9 PES header).
// Continuations carry 184 bytes each. Total = 175 + 4×184 = 911 bytes.
// AC3: 2 × 192 = 384 bytes. TrueHD: 300 + 227 = 527 bytes.
func buildTrueHDAC3M2TSData() []byte {
	const (
		pmtPID   = uint16(0x0100)
		videoPID = uint16(0x1011)
		audioPID = uint16(0x1101)
	)

	// Build combined TrueHD+AC3 payload (911 bytes = 175 + 4×184)
	var audioPayload []byte
	audioPayload = append(audioPayload, makeAC3Frame(0x11)...)        // 192 bytes AC3
	audioPayload = append(audioPayload, makeTrueHDUnit(300, 0x22)...) // 300 bytes TrueHD
	audioPayload = append(audioPayload, makeAC3Frame(0x33)...)        // 192 bytes AC3
	audioPayload = append(audioPayload, makeTrueHDUnit(227, 0x44)...) // 227 bytes TrueHD
	// Total: 911 bytes

	var data []byte
	data = append(data, makeM2TSPacket(0, true, 0x01, 0, 0, makePATPayload(pmtPID))...)
	data = append(data, makeM2TSPacket(pmtPID, true, 0x01, 0, 0,
		makePMTPayload(videoPID, 0x1B,
			[]uint16{audioPID},
			[]byte{0x83}))...) // 0x83 = TrueHD

	// Video PUSI
	data = append(data, makeM2TSPacket(videoPID, true, 0x01, 0, 1,
		makePESStart(0xE0, 0, seqBytes(0, 175)))...)

	// Audio PUSI - PES header + start of audioPayload
	pesHdr := makePESStart(0xFD, 0, nil) // 9-byte PES header
	firstChunkSize := 184 - len(pesHdr)  // 175 bytes
	firstPayload := make([]byte, 184)
	copy(firstPayload, pesHdr)
	copy(firstPayload[len(pesHdr):], audioPayload[:firstChunkSize])
	data = append(data, makeM2TSPacket(audioPID, true, 0x01, 0, 0, firstPayload)...)

	// Audio continuation packets for remaining data
	remaining := audioPayload[firstChunkSize:]
	cc := byte(1)
	for len(remaining) > 0 {
		chunkSize := 184
		if chunkSize > len(remaining) {
			chunkSize = len(remaining)
		}
		chunk := make([]byte, 184)
		copy(chunk, remaining[:chunkSize])
		data = append(data, makeM2TSPacket(audioPID, false, 0x01, 0, cc, chunk)...)
		remaining = remaining[chunkSize:]
		cc++
	}

	return data
}

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
