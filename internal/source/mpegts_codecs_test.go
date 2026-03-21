package source

import (
	"testing"
)

func TestDetectBlurayCodecs(t *testing.T) {
	// Build a synthetic M2TS with PAT + PMT
	data := buildTestM2TS(t, []testStream{
		{streamType: 0x1B}, // H.264 video
		{streamType: 0x81}, // AC3 audio
		{streamType: 0x86}, // DTS-HD audio
	})

	codecs, err := parseTSCodecs(data)
	if err != nil {
		t.Fatalf("parseTSCodecs error: %v", err)
	}

	if len(codecs.VideoCodecs) != 1 || codecs.VideoCodecs[0] != CodecH264Video {
		t.Errorf("video codecs = %v, want [H.264]", codecs.VideoCodecs)
	}

	if len(codecs.AudioCodecs) != 2 {
		t.Fatalf("audio codecs len = %d, want 2", len(codecs.AudioCodecs))
	}
	if codecs.AudioCodecs[0] != CodecAC3Audio {
		t.Errorf("audio codec 0 = %v, want AC3", codecs.AudioCodecs[0])
	}
	if codecs.AudioCodecs[1] != CodecDTSHDAudio {
		t.Errorf("audio codec 1 = %v, want DTS-HD", codecs.AudioCodecs[1])
	}
}

func TestDetectBlurayCodecs_MPEG2(t *testing.T) {
	data := buildTestM2TS(t, []testStream{
		{streamType: 0x02}, // MPEG-2 video
		{streamType: 0x83}, // TrueHD audio
	})

	codecs, err := parseTSCodecs(data)
	if err != nil {
		t.Fatalf("parseTSCodecs error: %v", err)
	}

	if len(codecs.VideoCodecs) != 1 || codecs.VideoCodecs[0] != CodecMPEG2Video {
		t.Errorf("video codecs = %v, want [MPEG-2]", codecs.VideoCodecs)
	}

	if len(codecs.AudioCodecs) != 1 || codecs.AudioCodecs[0] != CodecTrueHDAudio {
		t.Errorf("audio codecs = %v, want [TrueHD]", codecs.AudioCodecs)
	}
}

func TestParseTSCodecs_MultiPacketPMT(t *testing.T) {
	// Build M2TS data where the PMT spans two TS packets.
	// This verifies that parseTSCodecs (the lightweight codec scanner) correctly
	// reassembles multi-packet PMTs, which is common on Blu-rays with many streams.
	// Issue #155: AC3 audio streams were missed because they appeared past the
	// first packet boundary in the PMT.

	const packetSize = 192
	const tsPayloadStart = 4

	// 1 video (H.264) + 3 audio (DTS-HD, AC3, DTS) + 17 PGS subtitles = 21 streams
	// With 6-byte ES info descriptors on audio streams, the PMT exceeds one TS packet.

	// Build PMT section manually
	pmtSection := make([]byte, 0, 300)
	pmtSection = append(pmtSection, 0x02)       // table_id
	pmtSection = append(pmtSection, 0xB0, 0x00) // section_length placeholder
	pmtSection = append(pmtSection, 0x00, 0x01) // TSID
	pmtSection = append(pmtSection, 0xC1)       // version 0, current
	pmtSection = append(pmtSection, 0x00, 0x00) // section/last section
	pmtSection = append(pmtSection, 0xE1, 0x01) // PCR PID = 0x101
	pmtSection = append(pmtSection, 0xF0, 0x00) // prog_info_len = 0

	// Video stream (H.264, PID 0x101)
	pmtSection = append(pmtSection, 0x1B)       // H.264
	pmtSection = append(pmtSection, 0xE1, 0x01) // PID 0x101
	pmtSection = append(pmtSection, 0xF0, 0x00) // ES info len = 0

	// Audio streams with 16-byte ES info descriptors to push PMT past one packet.
	// DTS-HD (PID 0x1100)
	pmtSection = append(pmtSection, 0x86)       // DTS-HD
	pmtSection = append(pmtSection, 0xF1, 0x00) // PID 0x1100
	pmtSection = append(pmtSection, 0xF0, 0x10) // ES info len = 16
	pmtSection = append(pmtSection, 0x05, 0x04, 'D', 'T', 'S', 'H',
		0x0A, 0x04, 'e', 'n', 'g', 0x00, // ISO 639 language descriptor
		0x7F, 0x02, 0x86, 0x0F) // extension descriptor

	// AC3 (PID 0x1101) — the stream previously missed due to single-packet PMT parsing
	pmtSection = append(pmtSection, 0x81)       // AC3
	pmtSection = append(pmtSection, 0xF1, 0x01) // PID 0x1101
	pmtSection = append(pmtSection, 0xF0, 0x10) // ES info len = 16
	pmtSection = append(pmtSection, 0x05, 0x04, 'A', 'C', '-', '3',
		0x0A, 0x04, 'e', 'n', 'g', 0x00,
		0x7F, 0x02, 0x81, 0x0F)

	// DTS (PID 0x1102)
	pmtSection = append(pmtSection, 0x82)       // DTS
	pmtSection = append(pmtSection, 0xF1, 0x02) // PID 0x1102
	pmtSection = append(pmtSection, 0xF0, 0x10) // ES info len = 16
	pmtSection = append(pmtSection, 0x05, 0x04, 'D', 'T', 'S', ' ',
		0x0A, 0x04, 'e', 'n', 'g', 0x00,
		0x7F, 0x02, 0x82, 0x0F)

	// 20 PGS subtitle streams (PIDs 0x1200-0x1213)
	for i := 0; i < 20; i++ {
		pid := 0x1200 + i
		pmtSection = append(pmtSection, 0x90)                         // PGS
		pmtSection = append(pmtSection, 0xE0|byte(pid>>8), byte(pid)) // PID
		pmtSection = append(pmtSection, 0xF0, 0x00)                   // ES info len = 0
	}

	// CRC placeholder
	pmtSection = append(pmtSection, 0x00, 0x00, 0x00, 0x00)

	// Fill section_length
	sectionLen := len(pmtSection) - 3
	pmtSection[1] = 0xB0 | byte(sectionLen>>8)
	pmtSection[2] = byte(sectionLen)

	// Verify it exceeds one TS packet payload (183 bytes)
	if len(pmtSection) <= 183 {
		t.Fatalf("PMT section %d bytes fits in one packet; test needs larger PMT", len(pmtSection))
	}
	t.Logf("PMT section size: %d bytes (spans 2 packets)", len(pmtSection))

	// Build M2TS packets: PAT + PMT pkt1 (PUSI) + PMT pkt2 (continuation) + padding
	numPackets := 8
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

	// Test parseTSCodecs with the multi-packet PMT
	codecs, err := parseTSCodecs(data)
	if err != nil {
		t.Fatalf("parseTSCodecs error: %v", err)
	}

	// Verify video
	if len(codecs.VideoCodecs) != 1 || codecs.VideoCodecs[0] != CodecH264Video {
		t.Errorf("video codecs = %v, want [H.264]", codecs.VideoCodecs)
	}

	// Verify all 3 audio codecs detected (DTS-HD, AC3, DTS)
	if len(codecs.AudioCodecs) != 3 {
		t.Fatalf("audio codecs = %v (len %d), want 3 (DTS-HD, AC3, DTS)", codecs.AudioCodecs, len(codecs.AudioCodecs))
	}
	wantAudio := []CodecType{CodecDTSHDAudio, CodecAC3Audio, CodecDTSAudio}
	for i, want := range wantAudio {
		if codecs.AudioCodecs[i] != want {
			t.Errorf("audio codec %d = %v, want %v", i, codecs.AudioCodecs[i], want)
		}
	}

	// Verify PGS subtitles detected
	if len(codecs.SubtitleCodecs) != 1 || codecs.SubtitleCodecs[0] != CodecPGSSubtitle {
		t.Errorf("subtitle codecs = %v, want [PGS]", codecs.SubtitleCodecs)
	}
}

func TestDetectTSPacketSize(t *testing.T) {
	// Test 192-byte M2TS packets
	data := make([]byte, 192*5)
	for i := 0; i < 5; i++ {
		data[i*192+4] = 0x47 // Sync byte after 4-byte timestamp
	}
	size, offset := detectTSPacketSize(data)
	if size != 192 {
		t.Errorf("M2TS packet size = %d, want 192", size)
	}
	if offset != 0 {
		t.Errorf("M2TS offset = %d, want 0", offset)
	}

	// Test 188-byte standard TS packets
	data = make([]byte, 188*5)
	for i := 0; i < 5; i++ {
		data[i*188] = 0x47
	}
	size, offset = detectTSPacketSize(data)
	if size != 188 {
		t.Errorf("standard TS packet size = %d, want 188", size)
	}
	if offset != 0 {
		t.Errorf("standard TS offset = %d, want 0", offset)
	}
}

func TestTSStreamTypeToCodecType(t *testing.T) {
	tests := []struct {
		streamType byte
		want       CodecType
	}{
		{0x01, CodecMPEG1Video},
		{0x02, CodecMPEG2Video},
		{0x1B, CodecH264Video},
		{0x24, CodecH265Video},
		{0xEA, CodecVC1Video},
		{0x03, CodecMPEGAudio},
		{0x04, CodecMPEGAudio},
		{0x0F, CodecAACaudio},
		{0x80, CodecLPCMAudio},
		{0x81, CodecAC3Audio},
		{0x82, CodecDTSAudio},
		{0x83, CodecTrueHDAudio},
		{0x84, CodecEAC3Audio},
		{0x85, CodecDTSHDAudio},
		{0x86, CodecDTSHDAudio},
		{0x90, CodecPGSSubtitle},
		{0xFF, CodecUnknown},
	}

	for _, tt := range tests {
		got := tsStreamTypeToCodecType(tt.streamType)
		if got != tt.want {
			t.Errorf("tsStreamTypeToCodecType(0x%02X) = %v, want %v", tt.streamType, got, tt.want)
		}
	}
}
