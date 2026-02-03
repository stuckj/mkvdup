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
