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

func TestScanPESCodecs_MultipleAudioTypes(t *testing.T) {
	buf := make([]byte, 128)

	// MPEG audio stream
	buf[0] = 0x00
	buf[1] = 0x00
	buf[2] = 0x01
	buf[3] = 0xC0

	// Private stream 1 with AC3
	off := 20
	buf[off+0] = 0x00
	buf[off+1] = 0x00
	buf[off+2] = 0x01
	buf[off+3] = 0xBD
	buf[off+4] = 0x00
	buf[off+5] = 0x20
	buf[off+6] = 0x80
	buf[off+7] = 0x00
	buf[off+8] = 0x00
	buf[off+9] = 0x80 // AC3

	// Private stream 1 with LPCM
	off = 50
	buf[off+0] = 0x00
	buf[off+1] = 0x00
	buf[off+2] = 0x01
	buf[off+3] = 0xBD
	buf[off+4] = 0x00
	buf[off+5] = 0x20
	buf[off+6] = 0x80
	buf[off+7] = 0x00
	buf[off+8] = 0x00
	buf[off+9] = 0xA0 // LPCM

	codecs, err := scanPESCodecs(buf)
	if err != nil {
		t.Fatalf("scanPESCodecs error: %v", err)
	}

	if len(codecs.AudioCodecs) != 3 {
		t.Fatalf("audio codecs count = %d, want 3", len(codecs.AudioCodecs))
	}
	wantAudio := map[CodecType]bool{CodecMPEGAudio: true, CodecAC3Audio: true, CodecLPCMAudio: true}
	for _, ct := range codecs.AudioCodecs {
		if !wantAudio[ct] {
			t.Errorf("unexpected audio codec: %v", CodecTypeName(ct))
		}
	}
}

func TestScanPESCodecs_NoCodecs(t *testing.T) {
	buf := make([]byte, 64)
	_, err := scanPESCodecs(buf)
	if err == nil {
		t.Error("expected error for empty data, got nil")
	}
}

func TestFindDirEntry_Found(t *testing.T) {
	const sectorSize = 2048
	// Build a minimal directory with two entries: "." and "VIDEO_TS"
	dir := make([]byte, sectorSize)

	// First entry: "." (self)
	dir[0] = 34 // record length
	dir[32] = 1 // name length
	dir[33] = 0 // name = 0x00 (current dir)

	// Second entry: "VIDEO_TS"
	off := 34
	dir[off] = 42 // record length
	// Extent location (LE): sector 100
	dir[off+2] = 100
	dir[off+3] = 0
	dir[off+4] = 0
	dir[off+5] = 0
	// Data length (LE): 4096
	dir[off+10] = 0
	dir[off+11] = 16
	dir[off+12] = 0
	dir[off+13] = 0
	dir[off+32] = 8 // name length
	copy(dir[off+33:], "VIDEO_TS")

	extent, dataLen := findDirEntry(dir, "VIDEO_TS", sectorSize)
	if extent != 100 {
		t.Errorf("extent = %d, want 100", extent)
	}
	if dataLen != 4096 {
		t.Errorf("dataLen = %d, want 4096", dataLen)
	}
}

func TestFindDirEntry_WithSemicolon(t *testing.T) {
	const sectorSize = 2048
	dir := make([]byte, sectorSize)

	// Entry with ISO9660 version suffix: "VIDEO_TS;1"
	dir[0] = 44
	dir[2] = 50 // extent = 50
	dir[10] = 0
	dir[11] = 8 // data length = 2048
	dir[32] = 10
	copy(dir[33:], "VIDEO_TS;1")

	extent, dataLen := findDirEntry(dir, "VIDEO_TS", sectorSize)
	if extent != 50 {
		t.Errorf("extent = %d, want 50", extent)
	}
	if dataLen != 2048 {
		t.Errorf("dataLen = %d, want 2048", dataLen)
	}
}

func TestFindDirEntry_NotFound(t *testing.T) {
	const sectorSize = 2048
	dir := make([]byte, sectorSize)

	// Entry: "AUDIO_TS"
	dir[0] = 42
	dir[2] = 50
	dir[10] = 0
	dir[11] = 8
	dir[32] = 8
	copy(dir[33:], "AUDIO_TS")

	extent, dataLen := findDirEntry(dir, "VIDEO_TS", sectorSize)
	if extent != 0 || dataLen != 0 {
		t.Errorf("expected (0, 0), got (%d, %d)", extent, dataLen)
	}
}

func TestFindDirEntry_CaseInsensitive(t *testing.T) {
	const sectorSize = 2048
	dir := make([]byte, sectorSize)

	dir[0] = 42
	dir[2] = 75
	dir[10] = 0
	dir[11] = 8
	dir[32] = 8
	copy(dir[33:], "video_ts")

	extent, _ := findDirEntry(dir, "VIDEO_TS", sectorSize)
	if extent != 75 {
		t.Errorf("extent = %d, want 75", extent)
	}
}

func TestFindMainVOBOffset_NoISO(t *testing.T) {
	// Non-ISO file should return 0 (fallback to scanning from start)
	dir := t.TempDir()
	path := filepath.Join(dir, "test.iso")
	buf := make([]byte, 256)
	if err := os.WriteFile(path, buf, 0644); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	offset := findMainVOBOffset(f)
	if offset != 0 {
		t.Errorf("offset = %d, want 0 for non-ISO file", offset)
	}
}

func TestFindMainVOBOffset_ValidISO(t *testing.T) {
	const sectorSize = 2048

	// Build a minimal ISO9660 with VIDEO_TS directory containing a VTS_01_1.VOB
	// We need:
	// - Sector 16: Primary Volume Descriptor
	// - Sector 20: Root directory
	// - Sector 30: VIDEO_TS directory
	// - Sector 500: VTS_01_1.VOB data (this is what we want to find)

	rootSector := uint32(20)
	videoTSSector := uint32(30)
	vobSector := uint32(500)

	// Allocate enough space for the ISO structure
	iso := make([]byte, int(vobSector+1)*sectorSize)

	// --- Primary Volume Descriptor at sector 16 ---
	pvd := iso[16*sectorSize:]
	pvd[0] = 1 // Type = Primary
	copy(pvd[1:6], "CD001")

	// Root directory record at PVD offset 156
	rootRec := pvd[156:]
	rootRec[0] = 34 // record length
	// Extent location (LE)
	rootRec[2] = byte(rootSector)
	rootRec[3] = byte(rootSector >> 8)
	rootRec[4] = byte(rootSector >> 16)
	rootRec[5] = byte(rootSector >> 24)
	// Data length (LE): 1 sector (2048 = 0x0800)
	rootRec[10] = 0x00
	rootRec[11] = 0x08
	rootRec[32] = 1 // name length
	rootRec[33] = 0 // root

	// --- Root directory at rootSector ---
	rootDir := iso[rootSector*sectorSize:]
	// Self entry "."
	rootDir[0] = 34
	rootDir[2] = byte(rootSector)
	rootDir[32] = 1
	rootDir[33] = 0

	// VIDEO_TS entry
	off := 34
	rootDir[off] = 42
	rootDir[off+2] = byte(videoTSSector)
	rootDir[off+3] = byte(videoTSSector >> 8)
	// Data length (LE): 1 sector (2048 = 0x0800)
	rootDir[off+10] = 0x00
	rootDir[off+11] = 0x08
	rootDir[off+32] = 8
	copy(rootDir[off+33:], "VIDEO_TS")

	// --- VIDEO_TS directory at videoTSSector ---
	vtsDir := iso[videoTSSector*sectorSize:]
	// Self entry
	vtsDir[0] = 34
	vtsDir[2] = byte(videoTSSector)
	vtsDir[32] = 1
	vtsDir[33] = 0

	// VTS_01_0.VOB (navigation - should be skipped)
	off = 34
	navSector := uint32(400)
	vtsDir[off] = 46 // record length
	vtsDir[off+2] = byte(navSector)
	vtsDir[off+3] = byte(navSector >> 8)
	navSize := uint32(1024 * 1024) // 1MB
	vtsDir[off+10] = byte(navSize)
	vtsDir[off+11] = byte(navSize >> 8)
	vtsDir[off+12] = byte(navSize >> 16)
	vtsDir[off+13] = byte(navSize >> 24)
	vtsDir[off+32] = 12 // name length
	copy(vtsDir[off+33:], "VTS_01_0.VOB")

	// VTS_01_1.VOB (content - should be found)
	off += 46
	vtsDir[off] = 46
	vtsDir[off+2] = byte(vobSector)
	vtsDir[off+3] = byte(vobSector >> 8)
	vobSize := uint32(2 * 1024 * 1024) // 2MB
	vtsDir[off+10] = byte(vobSize)
	vtsDir[off+11] = byte(vobSize >> 8)
	vtsDir[off+12] = byte(vobSize >> 16)
	vtsDir[off+13] = byte(vobSize >> 24)
	vtsDir[off+32] = 12
	copy(vtsDir[off+33:], "VTS_01_1.VOB")

	// Write to temp file
	dir := t.TempDir()
	path := filepath.Join(dir, "test.iso")
	if err := os.WriteFile(path, iso, 0644); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	offset := findMainVOBOffset(f)
	want := int64(vobSector) * sectorSize
	if offset != want {
		t.Errorf("offset = %d, want %d", offset, want)
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
