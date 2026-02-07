package source

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectDVDCodecsFromFile_MPEG2WithAC3(t *testing.T) {
	// Build a minimal buffer with PES start codes for MPEG-2 video + AC3 audio
	buf := make([]byte, 256)

	// Video PES: 00 00 01 E0 (video stream 0)
	buf[0] = 0x00
	buf[1] = 0x00
	buf[2] = 0x01
	buf[3] = 0xE0

	// Private stream 1 with AC3 sub-stream: 00 00 01 BD
	// PES header: bytes 4-5 = PES length, 6-7 = flags, 8 = header data len
	off := 20
	buf[off+0] = 0x00
	buf[off+1] = 0x00
	buf[off+2] = 0x01
	buf[off+3] = 0xBD // Private stream 1
	buf[off+4] = 0x00 // PES packet length high
	buf[off+5] = 0x20 // PES packet length low
	buf[off+6] = 0x80 // Marker bits
	buf[off+7] = 0x00 // No PTS/DTS
	buf[off+8] = 0x00 // PES header data length = 0
	buf[off+9] = 0x80 // Sub-stream ID = AC3 (0x80)

	// Write to temp file
	dir := t.TempDir()
	path := filepath.Join(dir, "test.iso")
	if err := os.WriteFile(path, buf, 0644); err != nil {
		t.Fatal(err)
	}

	codecs, err := detectDVDCodecsFromFile(path)
	if err != nil {
		t.Fatalf("detectDVDCodecsFromFile error: %v", err)
	}

	if len(codecs.VideoCodecs) != 1 || codecs.VideoCodecs[0] != CodecMPEG2Video {
		t.Errorf("video codecs = %v, want [MPEG-2]", codecs.VideoCodecs)
	}
	if len(codecs.AudioCodecs) != 1 || codecs.AudioCodecs[0] != CodecAC3Audio {
		t.Errorf("audio codecs = %v, want [AC3]", codecs.AudioCodecs)
	}
}

func TestDetectDVDCodecsFromFile_MPEGAudio(t *testing.T) {
	buf := make([]byte, 64)

	// MPEG audio stream: 00 00 01 C0
	buf[0] = 0x00
	buf[1] = 0x00
	buf[2] = 0x01
	buf[3] = 0xC0

	dir := t.TempDir()
	path := filepath.Join(dir, "test.iso")
	if err := os.WriteFile(path, buf, 0644); err != nil {
		t.Fatal(err)
	}

	codecs, err := detectDVDCodecsFromFile(path)
	if err != nil {
		t.Fatalf("detectDVDCodecsFromFile error: %v", err)
	}

	if len(codecs.AudioCodecs) != 1 || codecs.AudioCodecs[0] != CodecMPEGAudio {
		t.Errorf("audio codecs = %v, want [MPEG Audio]", codecs.AudioCodecs)
	}
}

func TestDetectDVDCodecsFromFile_DTS(t *testing.T) {
	buf := make([]byte, 64)

	// Private stream 1 with DTS sub-stream
	buf[0] = 0x00
	buf[1] = 0x00
	buf[2] = 0x01
	buf[3] = 0xBD
	buf[4] = 0x00
	buf[5] = 0x20
	buf[6] = 0x80
	buf[7] = 0x00
	buf[8] = 0x00 // PES header data length = 0
	buf[9] = 0x88 // Sub-stream ID = DTS (0x88)

	dir := t.TempDir()
	path := filepath.Join(dir, "test.iso")
	if err := os.WriteFile(path, buf, 0644); err != nil {
		t.Fatal(err)
	}

	codecs, err := detectDVDCodecsFromFile(path)
	if err != nil {
		t.Fatalf("detectDVDCodecsFromFile error: %v", err)
	}

	if len(codecs.AudioCodecs) != 1 || codecs.AudioCodecs[0] != CodecDTSAudio {
		t.Errorf("audio codecs = %v, want [DTS]", codecs.AudioCodecs)
	}
}

func TestDetectDVDCodecsFromFile_Empty(t *testing.T) {
	// A file with no PES headers should return an error
	buf := make([]byte, 256)

	dir := t.TempDir()
	path := filepath.Join(dir, "test.iso")
	if err := os.WriteFile(path, buf, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := detectDVDCodecsFromFile(path)
	if err == nil {
		t.Error("expected error for empty codec detection, got nil")
	}
}

func TestDetectDVDCodecsFromFile_BoundaryStartCode(t *testing.T) {
	// Test that a start code at the very end of the buffer is detected
	// (verifies the i+3 < len(buf) fix)
	buf := make([]byte, 8)
	// Place video PES start code at last valid position (index 4)
	buf[4] = 0x00
	buf[5] = 0x00
	buf[6] = 0x01
	buf[7] = 0xE0

	dir := t.TempDir()
	path := filepath.Join(dir, "test.iso")
	if err := os.WriteFile(path, buf, 0644); err != nil {
		t.Fatal(err)
	}

	codecs, err := detectDVDCodecsFromFile(path)
	if err != nil {
		t.Fatalf("detectDVDCodecsFromFile error: %v", err)
	}

	if len(codecs.VideoCodecs) != 1 || codecs.VideoCodecs[0] != CodecMPEG2Video {
		t.Errorf("video codecs = %v, want [MPEG-2]", codecs.VideoCodecs)
	}
}
