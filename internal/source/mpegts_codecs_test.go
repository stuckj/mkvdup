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
