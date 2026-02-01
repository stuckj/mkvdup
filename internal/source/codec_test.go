package source

import (
	"testing"

	"github.com/stuckj/mkvdup/internal/mkv"
)

func TestMKVCodecToType(t *testing.T) {
	tests := []struct {
		codecID string
		want    CodecType
	}{
		// Video
		{"V_MPEG1", CodecMPEG1Video},
		{"V_MPEG2", CodecMPEG2Video},
		{"V_MPEG4/ISO/AVC", CodecH264Video},
		{"V_MPEGH/ISO/HEVC", CodecH265Video},
		{"V_MS/VFW/FOURCC", CodecUnknown},

		// Audio
		{"A_AC3", CodecAC3Audio},
		{"A_EAC3", CodecEAC3Audio},
		{"A_DTS", CodecDTSAudio},
		{"A_DTS/EXPRESS", CodecDTSHDAudio},
		{"A_DTS/LOSSLESS", CodecDTSHDAudio},
		{"A_TRUEHD", CodecTrueHDAudio},
		{"A_PCM/INT/LIT", CodecLPCMAudio},
		{"A_PCM/INT/BIG", CodecLPCMAudio},
		{"A_PCM/FLOAT/IEEE", CodecLPCMAudio},
		{"A_MPEG/L2", CodecMPEGAudio},
		{"A_MPEG/L3", CodecMPEGAudio},
		{"A_AAC", CodecAACaudio},
		{"A_AAC/MPEG2/LC", CodecAACaudio},
		{"A_FLAC", CodecFLACAudio},
		{"A_OPUS", CodecOpusAudio},

		// Unknown
		{"S_TEXT/UTF8", CodecUnknown},
		{"", CodecUnknown},
	}

	for _, tt := range tests {
		got := MKVCodecToType(tt.codecID)
		if got != tt.want {
			t.Errorf("MKVCodecToType(%q) = %v, want %v", tt.codecID, got, tt.want)
		}
	}
}

func TestCodecTypeName(t *testing.T) {
	tests := []struct {
		ct   CodecType
		want string
	}{
		{CodecMPEG1Video, "MPEG-1"},
		{CodecMPEG2Video, "MPEG-2"},
		{CodecH264Video, "H.264"},
		{CodecH265Video, "H.265"},
		{CodecVC1Video, "VC-1"},
		{CodecAC3Audio, "AC3"},
		{CodecEAC3Audio, "E-AC3"},
		{CodecDTSAudio, "DTS"},
		{CodecDTSHDAudio, "DTS-HD"},
		{CodecTrueHDAudio, "TrueHD"},
		{CodecLPCMAudio, "LPCM"},
		{CodecMPEGAudio, "MPEG Audio"},
		{CodecAACaudio, "AAC"},
		{CodecFLACAudio, "FLAC"},
		{CodecOpusAudio, "Opus"},
		{CodecUnknown, "Unknown"},
	}

	for _, tt := range tests {
		got := CodecTypeName(tt.ct)
		if got != tt.want {
			t.Errorf("CodecTypeName(%v) = %q, want %q", tt.ct, got, tt.want)
		}
	}
}

func TestCheckCodecCompatibility_Match(t *testing.T) {
	tracks := []mkv.Track{
		{Number: 1, Type: mkv.TrackTypeVideo, CodecID: "V_MPEG2"},
		{Number: 2, Type: mkv.TrackTypeAudio, CodecID: "A_AC3"},
	}
	sc := &SourceCodecs{
		VideoCodecs: []CodecType{CodecMPEG2Video},
		AudioCodecs: []CodecType{CodecAC3Audio},
	}

	mismatches := CheckCodecCompatibility(tracks, sc)
	if len(mismatches) != 0 {
		t.Errorf("expected no mismatches, got %d: %+v", len(mismatches), mismatches)
	}
}

func TestCheckCodecCompatibility_VideoMismatch(t *testing.T) {
	tracks := []mkv.Track{
		{Number: 1, Type: mkv.TrackTypeVideo, CodecID: "V_MPEG4/ISO/AVC"},
		{Number: 2, Type: mkv.TrackTypeAudio, CodecID: "A_AC3"},
	}
	sc := &SourceCodecs{
		VideoCodecs: []CodecType{CodecMPEG2Video},
		AudioCodecs: []CodecType{CodecAC3Audio},
	}

	mismatches := CheckCodecCompatibility(tracks, sc)
	if len(mismatches) != 1 {
		t.Fatalf("expected 1 mismatch, got %d", len(mismatches))
	}
	if mismatches[0].TrackType != "video" {
		t.Errorf("mismatch track type = %q, want %q", mismatches[0].TrackType, "video")
	}
	if mismatches[0].MKVCodecType != CodecH264Video {
		t.Errorf("mismatch MKV codec = %v, want %v", mismatches[0].MKVCodecType, CodecH264Video)
	}
}

func TestCheckCodecCompatibility_AudioMismatch(t *testing.T) {
	tracks := []mkv.Track{
		{Number: 1, Type: mkv.TrackTypeVideo, CodecID: "V_MPEG2"},
		{Number: 2, Type: mkv.TrackTypeAudio, CodecID: "A_FLAC"},
	}
	sc := &SourceCodecs{
		VideoCodecs: []CodecType{CodecMPEG2Video},
		AudioCodecs: []CodecType{CodecAC3Audio, CodecDTSAudio},
	}

	mismatches := CheckCodecCompatibility(tracks, sc)
	if len(mismatches) != 1 {
		t.Fatalf("expected 1 mismatch, got %d", len(mismatches))
	}
	if mismatches[0].TrackType != "audio" {
		t.Errorf("mismatch track type = %q, want %q", mismatches[0].TrackType, "audio")
	}
	if mismatches[0].MKVCodecType != CodecFLACAudio {
		t.Errorf("mismatch MKV codec = %v, want %v", mismatches[0].MKVCodecType, CodecFLACAudio)
	}
}

func TestCheckCodecCompatibility_FamilyMatch(t *testing.T) {
	// DTS in MKV should match DTS-HD in source (same family)
	tracks := []mkv.Track{
		{Number: 1, Type: mkv.TrackTypeAudio, CodecID: "A_DTS"},
	}
	sc := &SourceCodecs{
		AudioCodecs: []CodecType{CodecDTSHDAudio},
	}

	mismatches := CheckCodecCompatibility(tracks, sc)
	if len(mismatches) != 0 {
		t.Errorf("DTS should match DTS-HD family, got %d mismatches", len(mismatches))
	}

	// AC3 in MKV should match E-AC3 in source (same family)
	tracks = []mkv.Track{
		{Number: 1, Type: mkv.TrackTypeAudio, CodecID: "A_AC3"},
	}
	sc = &SourceCodecs{
		AudioCodecs: []CodecType{CodecEAC3Audio},
	}

	mismatches = CheckCodecCompatibility(tracks, sc)
	if len(mismatches) != 0 {
		t.Errorf("AC3 should match E-AC3 family, got %d mismatches", len(mismatches))
	}

	// MPEG-1 in MKV should match MPEG-2 in source (same family)
	tracks = []mkv.Track{
		{Number: 1, Type: mkv.TrackTypeVideo, CodecID: "V_MPEG1"},
	}
	sc = &SourceCodecs{
		VideoCodecs: []CodecType{CodecMPEG2Video},
	}

	mismatches = CheckCodecCompatibility(tracks, sc)
	if len(mismatches) != 0 {
		t.Errorf("MPEG-1 should match MPEG-2 family, got %d mismatches", len(mismatches))
	}
}

func TestCheckCodecCompatibility_UnknownSkipped(t *testing.T) {
	// Unknown codecs should not produce mismatches
	tracks := []mkv.Track{
		{Number: 1, Type: mkv.TrackTypeVideo, CodecID: "V_MS/VFW/FOURCC"},
		{Number: 2, Type: mkv.TrackTypeAudio, CodecID: "A_SOMETHING_NEW"},
	}
	sc := &SourceCodecs{
		VideoCodecs: []CodecType{CodecMPEG2Video},
		AudioCodecs: []CodecType{CodecAC3Audio},
	}

	mismatches := CheckCodecCompatibility(tracks, sc)
	if len(mismatches) != 0 {
		t.Errorf("unknown codecs should be skipped, got %d mismatches", len(mismatches))
	}
}

func TestCheckCodecCompatibility_SubtitleSkipped(t *testing.T) {
	// Subtitle tracks should not be checked
	tracks := []mkv.Track{
		{Number: 1, Type: mkv.TrackTypeSubtitle, CodecID: "S_TEXT/UTF8"},
	}
	sc := &SourceCodecs{
		VideoCodecs: []CodecType{CodecMPEG2Video},
	}

	mismatches := CheckCodecCompatibility(tracks, sc)
	if len(mismatches) != 0 {
		t.Errorf("subtitle tracks should be skipped, got %d mismatches", len(mismatches))
	}
}

func TestCheckCodecCompatibility_EmptySource(t *testing.T) {
	// If source has no detected codecs, no mismatches should be reported
	tracks := []mkv.Track{
		{Number: 1, Type: mkv.TrackTypeVideo, CodecID: "V_MPEG4/ISO/AVC"},
		{Number: 2, Type: mkv.TrackTypeAudio, CodecID: "A_AC3"},
	}
	sc := &SourceCodecs{}

	mismatches := CheckCodecCompatibility(tracks, sc)
	if len(mismatches) != 0 {
		t.Errorf("empty source codecs should produce no mismatches, got %d", len(mismatches))
	}
}

func TestCheckCodecCompatibility_MultipleAudioOneMatches(t *testing.T) {
	// MKV has two audio tracks; one matches source, one doesn't
	// The non-matching one should still be reported
	tracks := []mkv.Track{
		{Number: 1, Type: mkv.TrackTypeAudio, CodecID: "A_AC3"},
		{Number: 2, Type: mkv.TrackTypeAudio, CodecID: "A_FLAC"},
	}
	sc := &SourceCodecs{
		AudioCodecs: []CodecType{CodecAC3Audio},
	}

	mismatches := CheckCodecCompatibility(tracks, sc)
	if len(mismatches) != 1 {
		t.Fatalf("expected 1 mismatch for FLAC track, got %d", len(mismatches))
	}
	if mismatches[0].MKVCodecType != CodecFLACAudio {
		t.Errorf("expected FLAC mismatch, got %v", mismatches[0].MKVCodecType)
	}
}

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
		{0xFF, CodecUnknown},
	}

	for _, tt := range tests {
		got := tsStreamTypeToCodecType(tt.streamType)
		if got != tt.want {
			t.Errorf("tsStreamTypeToCodecType(0x%02X) = %v, want %v", tt.streamType, got, tt.want)
		}
	}
}

func TestIsVideoCodec(t *testing.T) {
	videoCodecs := []CodecType{CodecMPEG1Video, CodecMPEG2Video, CodecH264Video, CodecH265Video, CodecVC1Video}
	audioCodecs := []CodecType{CodecAC3Audio, CodecDTSAudio, CodecFLACAudio}

	for _, c := range videoCodecs {
		if !IsVideoCodec(c) {
			t.Errorf("IsVideoCodec(%v) = false, want true", c)
		}
	}
	for _, c := range audioCodecs {
		if IsVideoCodec(c) {
			t.Errorf("IsVideoCodec(%v) = true, want false", c)
		}
	}
}

func TestIsAudioCodec(t *testing.T) {
	audioCodecs := []CodecType{CodecAC3Audio, CodecEAC3Audio, CodecDTSAudio, CodecDTSHDAudio,
		CodecTrueHDAudio, CodecLPCMAudio, CodecMPEGAudio, CodecAACaudio, CodecFLACAudio, CodecOpusAudio}
	videoCodecs := []CodecType{CodecMPEG1Video, CodecMPEG2Video, CodecH264Video}

	for _, c := range audioCodecs {
		if !IsAudioCodec(c) {
			t.Errorf("IsAudioCodec(%v) = false, want true", c)
		}
	}
	for _, c := range videoCodecs {
		if IsAudioCodec(c) {
			t.Errorf("IsAudioCodec(%v) = true, want false", c)
		}
	}
}

// testStream describes a stream for building test M2TS data.
type testStream struct {
	streamType byte
}

// buildTestM2TS creates synthetic M2TS data with a PAT and PMT containing
// the specified streams. Uses 192-byte M2TS packets.
func buildTestM2TS(t *testing.T, streams []testStream) []byte {
	t.Helper()

	const packetSize = 192
	const tsPayloadStart = 4 // M2TS timestamp prefix

	// Need at least 4 packets for detectTSPacketSize to verify sync pattern.
	// Packet 1: PAT, Packet 2: PMT, Packets 3+: null padding
	numPackets := 6
	data := make([]byte, packetSize*numPackets)

	// Add sync bytes to all packets (null/padding packets)
	for i := 0; i < numPackets; i++ {
		data[i*packetSize+tsPayloadStart] = 0x47
		// Set PID to 0x1FFF (null packet) with no payload
		data[i*packetSize+tsPayloadStart+1] = 0x1F
		data[i*packetSize+tsPayloadStart+2] = 0xFF
		data[i*packetSize+tsPayloadStart+3] = 0x10
	}

	// Packet 1: PAT
	patOffset := 0
	// 4-byte M2TS timestamp (zeros)
	ts := data[patOffset+tsPayloadStart:]
	ts[0] = 0x47                   // Sync byte
	ts[1] = 0x40                   // Payload unit start + PID high = 0
	ts[2] = 0x00                   // PID low = 0 (PAT)
	ts[3] = 0x10                   // No adaptation, payload only
	ts[4] = 0x00                   // Pointer field = 0
	ts[5] = 0x00                   // Table ID = 0 (PAT)
	ts[6] = 0xB0                   // Section syntax + length high
	ts[7] = 0x0D                   // Section length = 13 (5 header + 4 program + 4 CRC)
	ts[8] = 0x00                   // Transport stream ID high
	ts[9] = 0x01                   // Transport stream ID low
	ts[10] = 0xC1                  // Version 0, current
	ts[11] = 0x00                  // Section number
	ts[12] = 0x00                  // Last section number
	ts[13] = 0x00                  // Program number high = 0
	ts[14] = 0x01                  // Program number low = 1
	ts[15] = 0xE0 | byte(0x100>>8) // PMT PID high (0x100)
	ts[16] = byte(0x100 & 0xFF)    // PMT PID low
	// CRC (just zeros for test — we don't validate CRC)

	// Packet 2: PMT (PID 0x100)
	pmtOffset := packetSize
	ts = data[pmtOffset+tsPayloadStart:]
	ts[0] = 0x47 // Sync byte
	ts[1] = 0x41 // Payload unit start + PID high = 1
	ts[2] = 0x00 // PID low = 0x00 → PID = 0x100
	ts[3] = 0x10 // No adaptation, payload only
	ts[4] = 0x00 // Pointer field = 0
	ts[5] = 0x02 // Table ID = 2 (PMT)

	// Build PMT section
	// Section: 5 bytes header + 4 bytes (PCR PID + prog info len) + 5*len(streams) + 4 CRC
	sectionLen := 5 + 4 + 5*len(streams) + 4
	ts[6] = 0xB0 | byte(sectionLen>>8) // Section syntax + length high
	ts[7] = byte(sectionLen)           // Section length low
	ts[8] = 0x00                       // Program number high
	ts[9] = 0x01                       // Program number low
	ts[10] = 0xC1                      // Version 0, current
	ts[11] = 0x00                      // Section number
	ts[12] = 0x00                      // Last section number
	ts[13] = 0xE0 | 0x01               // PCR PID high (0x101)
	ts[14] = 0x01                      // PCR PID low
	ts[15] = 0xF0                      // Program info length high
	ts[16] = 0x00                      // Program info length low = 0

	// Stream descriptors
	offset := 17
	for i, s := range streams {
		pid := 0x101 + i
		ts[offset] = s.streamType          // stream_type
		ts[offset+1] = 0xE0 | byte(pid>>8) // elementary PID high
		ts[offset+2] = byte(pid & 0xFF)    // elementary PID low
		ts[offset+3] = 0xF0                // ES info length high
		ts[offset+4] = 0x00                // ES info length low = 0
		offset += 5
	}

	return data
}
