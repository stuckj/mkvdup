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

		// Subtitle
		{"S_HDMV/PGS", CodecPGSSubtitle},

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

func TestIsSubtitleCodec(t *testing.T) {
	if !IsSubtitleCodec(CodecPGSSubtitle) {
		t.Error("IsSubtitleCodec(CodecPGSSubtitle) = false, want true")
	}
	nonSubtitle := []CodecType{CodecUnknown, CodecH264Video, CodecAC3Audio, CodecFLACAudio}
	for _, c := range nonSubtitle {
		if IsSubtitleCodec(c) {
			t.Errorf("IsSubtitleCodec(%v) = true, want false", c)
		}
	}
}

func TestCheckCodecCompatibility_SubtitleMatch(t *testing.T) {
	tracks := []mkv.Track{
		{Number: 1, Type: mkv.TrackTypeSubtitle, CodecID: "S_HDMV/PGS"},
	}
	sc := &SourceCodecs{
		SubtitleCodecs: []CodecType{CodecPGSSubtitle},
	}
	mismatches := CheckCodecCompatibility(tracks, sc)
	if len(mismatches) != 0 {
		t.Errorf("PGS subtitle should match, got %d mismatches", len(mismatches))
	}
}

func TestCheckCodecCompatibility_SubtitleMismatch(t *testing.T) {
	tracks := []mkv.Track{
		{Number: 1, Type: mkv.TrackTypeSubtitle, CodecID: "S_HDMV/PGS"},
	}
	// Source has no subtitle codecs but SubtitleCodecs is empty → should skip (no mismatch)
	sc := &SourceCodecs{}
	mismatches := CheckCodecCompatibility(tracks, sc)
	if len(mismatches) != 0 {
		t.Errorf("empty source subtitle codecs should produce no mismatches, got %d", len(mismatches))
	}
}
