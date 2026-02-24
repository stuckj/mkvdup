package source

import (
	"testing"
)

func TestDetectDVDCodecs(t *testing.T) {
	// Create a minimal index with a mock MPEG-PS parser
	index := &Index{
		SourceType: TypeDVD,
	}

	// Create a parser with known video and audio data
	// We need to simulate that the parser has been indexed
	parser := &MPEGPSParser{}

	// Set up video ranges so TotalESSize(true) > 0
	parser.videoRanges = []PESPayloadRange{
		{FileOffset: 0, Size: 1000, ESOffset: 0},
	}

	// Set up audio sub-streams
	parser.audioSubStreams = []byte{0x80, 0x88} // AC3 + DTS

	index.ESReaders = []ESReader{parser}

	codecs, err := detectDVDCodecs(index)
	if err != nil {
		t.Fatalf("detectDVDCodecs error: %v", err)
	}

	if len(codecs.VideoCodecs) != 1 || codecs.VideoCodecs[0] != CodecMPEG2Video {
		t.Errorf("video codecs = %v, want [MPEG-2]", codecs.VideoCodecs)
	}

	if len(codecs.AudioCodecs) != 2 {
		t.Fatalf("audio codecs len = %d, want 2", len(codecs.AudioCodecs))
	}
	if codecs.AudioCodecs[0] != CodecAC3Audio {
		t.Errorf("audio codec 0 = %v, want AC3", codecs.AudioCodecs[0])
	}
	if codecs.AudioCodecs[1] != CodecDTSAudio {
		t.Errorf("audio codec 1 = %v, want DTS", codecs.AudioCodecs[1])
	}
}

func TestDetectDVDCodecs_WithMPEGAudio(t *testing.T) {
	index := &Index{
		SourceType: TypeDVD,
	}

	parser := &MPEGPSParser{}
	parser.videoRanges = []PESPayloadRange{
		{FileOffset: 0, Size: 1000, ESOffset: 0},
	}
	// Add an MPEG audio packet (stream ID 0xC0)
	parser.packets = []PESPacket{
		{StreamID: 0xC0, IsAudio: true},
	}

	index.ESReaders = []ESReader{parser}

	codecs, err := detectDVDCodecs(index)
	if err != nil {
		t.Fatalf("detectDVDCodecs error: %v", err)
	}

	if len(codecs.AudioCodecs) != 1 || codecs.AudioCodecs[0] != CodecMPEGAudio {
		t.Errorf("audio codecs = %v, want [MPEG Audio]", codecs.AudioCodecs)
	}
}
