package source

import (
	"os"
	"path/filepath"
	"testing"
)

// buildTestIFO creates a minimal VTS_xx_0.IFO binary blob.
// videoCompression: 0=MPEG-1, 1=MPEG-2.
// audioEntries: each entry is (codingMode, channels-1) where codingMode is
// 0=AC3, 2=MPEG-1, 3=MPEG-2ext, 4=LPCM, 6=DTS.
func buildTestIFO(videoCompression byte, audioEntries [][2]byte) []byte {
	data := make([]byte, 0x244) // Minimum size for VTS_MAT with 8 audio entries
	copy(data[0:12], "DVDVIDEO-VTS")

	// Video attributes at 0x200 (bits 15-14 = compression).
	data[0x200] = videoCompression << 6

	// Audio stream count at 0x202 (big-endian).
	numAudio := len(audioEntries)
	data[0x202] = byte(numAudio >> 8)
	data[0x203] = byte(numAudio)

	// Audio attributes at 0x204 (8 bytes each).
	for i, entry := range audioEntries {
		off := 0x204 + i*8
		data[off] = entry[0] << 5   // coding mode in bits 7-5
		data[off+1] = entry[1] & 07 // channels-1 in bits 2-0
		data[off+2] = 'e'           // language code
		data[off+3] = 'n'
	}

	return data
}

func TestParseDVDIFOCodecs_MPEG2WithAC3(t *testing.T) {
	data := buildTestIFO(1, [][2]byte{{0, 1}}) // MPEG-2, AC3 stereo
	codecs, err := parseDVDIFOCodecs(data)
	if err != nil {
		t.Fatal(err)
	}

	if len(codecs.VideoCodecs) != 1 || codecs.VideoCodecs[0] != CodecMPEG2Video {
		t.Errorf("video = %v, want [MPEG-2]", codecs.VideoCodecs)
	}
	if len(codecs.AudioCodecs) != 1 || codecs.AudioCodecs[0] != CodecAC3Audio {
		t.Errorf("audio = %v, want [AC3]", codecs.AudioCodecs)
	}
}

func TestParseDVDIFOCodecs_MPEG1WithDTS(t *testing.T) {
	data := buildTestIFO(0, [][2]byte{{6, 5}}) // MPEG-1, DTS 5.1
	codecs, err := parseDVDIFOCodecs(data)
	if err != nil {
		t.Fatal(err)
	}

	if len(codecs.VideoCodecs) != 1 || codecs.VideoCodecs[0] != CodecMPEG1Video {
		t.Errorf("video = %v, want [MPEG-1]", codecs.VideoCodecs)
	}
	if len(codecs.AudioCodecs) != 1 || codecs.AudioCodecs[0] != CodecDTSAudio {
		t.Errorf("audio = %v, want [DTS]", codecs.AudioCodecs)
	}
}

func TestParseDVDIFOCodecs_MultipleAudio(t *testing.T) {
	data := buildTestIFO(1, [][2]byte{
		{0, 5}, // AC3 5.1
		{4, 1}, // LPCM stereo
		{6, 5}, // DTS 5.1
		{2, 1}, // MPEG-1 stereo
	})
	codecs, err := parseDVDIFOCodecs(data)
	if err != nil {
		t.Fatal(err)
	}

	if len(codecs.AudioCodecs) != 4 {
		t.Fatalf("audio count = %d, want 4", len(codecs.AudioCodecs))
	}
	want := map[CodecType]bool{
		CodecAC3Audio:  true,
		CodecLPCMAudio: true,
		CodecDTSAudio:  true,
		CodecMPEGAudio: true,
	}
	for _, ct := range codecs.AudioCodecs {
		if !want[ct] {
			t.Errorf("unexpected audio codec: %v", CodecTypeName(ct))
		}
	}
}

func TestParseDVDIFOCodecs_DuplicateCodecs(t *testing.T) {
	// Two AC3 streams should produce only one AC3 entry.
	data := buildTestIFO(1, [][2]byte{
		{0, 1}, // AC3 stereo
		{0, 5}, // AC3 5.1
	})
	codecs, err := parseDVDIFOCodecs(data)
	if err != nil {
		t.Fatal(err)
	}

	if len(codecs.AudioCodecs) != 1 || codecs.AudioCodecs[0] != CodecAC3Audio {
		t.Errorf("audio = %v, want [AC3] (deduplicated)", codecs.AudioCodecs)
	}
}

func TestParseDVDIFOCodecs_InvalidMagic(t *testing.T) {
	data := make([]byte, 0x244)
	copy(data[0:12], "NOT-DVD-DATA")
	_, err := parseDVDIFOCodecs(data)
	if err == nil {
		t.Error("expected error for invalid magic")
	}
}

func TestParseDVDIFOCodecs_TooShort(t *testing.T) {
	data := []byte("DVDVIDEO-VTS")
	_, err := parseDVDIFOCodecs(data)
	if err == nil {
		t.Error("expected error for short data")
	}
}

// buildTestDVDISO creates a minimal ISO9660 filesystem with VIDEO_TS containing
// the given IFO files and optionally VOB files with PES data.
//
// Layout:
//
//	Sector 16: PVD
//	Sector 17: Terminator
//	Sector 20: Root directory
//	Sector 21: VIDEO_TS directory
//	Sector 30+: IFO file data
func buildTestDVDISO(ifoEntries []testIFOEntry) []byte {
	const sector = 2048

	// Calculate sectors needed for IFO data
	ifoStartSector := 30
	currentSector := ifoStartSector
	type ifoPlacement struct {
		name   string
		sector int
		data   []byte
	}
	var placements []ifoPlacement
	for _, e := range ifoEntries {
		sectors := (len(e.data) + sector - 1) / sector
		placements = append(placements, ifoPlacement{e.name, currentSector, e.data})
		currentSector += sectors
	}

	totalSectors := currentSector + 1
	iso := make([]byte, totalSectors*sector)

	// PVD at sector 16
	pvd := iso[16*sector:]
	pvd[0] = 1
	copy(pvd[1:6], "CD001")
	pvd[6] = 1
	writeISO9660DirRecord(pvd[156:], 20, sector)

	// Terminator at sector 17
	iso[17*sector] = 255
	copy(iso[17*sector+1:17*sector+6], "CD001")

	// Root directory at sector 20
	rootDir := iso[20*sector:]
	off := 0
	off += writeISO9660DirEntry(rootDir[off:], "\x00", 20, sector, true)
	off += writeISO9660DirEntry(rootDir[off:], "\x01", 20, sector, true)
	off += writeISO9660DirEntry(rootDir[off:], "VIDEO_TS", 21, sector, true)

	// VIDEO_TS directory at sector 21
	vtsDir := iso[21*sector:]
	off = 0
	off += writeISO9660DirEntry(vtsDir[off:], "\x00", 21, sector, true)
	off += writeISO9660DirEntry(vtsDir[off:], "\x01", 20, sector, true)
	for _, p := range placements {
		off += writeISO9660DirEntry(vtsDir[off:], p.name, p.sector, len(p.data), false)
	}

	// Write IFO data
	for _, p := range placements {
		copy(iso[p.sector*sector:], p.data)
	}

	return iso
}

type testIFOEntry struct {
	name string
	data []byte
}

func TestDetectDVDCodecsFromFile_IFOBased(t *testing.T) {
	ifoData := buildTestIFO(1, [][2]byte{{0, 1}, {0, 5}}) // MPEG-2, 2x AC3
	iso := buildTestDVDISO([]testIFOEntry{
		{"VTS_01_0.IFO", ifoData},
	})

	dir := t.TempDir()
	path := filepath.Join(dir, "test.iso")
	if err := os.WriteFile(path, iso, 0644); err != nil {
		t.Fatal(err)
	}

	codecs, err := detectDVDCodecsFromFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(codecs.VideoCodecs) != 1 || codecs.VideoCodecs[0] != CodecMPEG2Video {
		t.Errorf("video = %v, want [MPEG-2]", codecs.VideoCodecs)
	}
	if len(codecs.AudioCodecs) != 1 || codecs.AudioCodecs[0] != CodecAC3Audio {
		t.Errorf("audio = %v, want [AC3]", codecs.AudioCodecs)
	}
}

func TestDetectDVDCodecsFromFile_IFOMultiVTS(t *testing.T) {
	ifo1 := buildTestIFO(1, [][2]byte{{0, 1}}) // AC3
	ifo2 := buildTestIFO(1, [][2]byte{{6, 5}}) // DTS
	iso := buildTestDVDISO([]testIFOEntry{
		{"VTS_01_0.IFO", ifo1},
		{"VTS_02_0.IFO", ifo2},
	})

	dir := t.TempDir()
	path := filepath.Join(dir, "test.iso")
	if err := os.WriteFile(path, iso, 0644); err != nil {
		t.Fatal(err)
	}

	codecs, err := detectDVDCodecsFromFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(codecs.AudioCodecs) != 2 {
		t.Fatalf("audio count = %d, want 2", len(codecs.AudioCodecs))
	}
	hasAC3, hasDTS := false, false
	for _, ct := range codecs.AudioCodecs {
		if ct == CodecAC3Audio {
			hasAC3 = true
		}
		if ct == CodecDTSAudio {
			hasDTS = true
		}
	}
	if !hasAC3 || !hasDTS {
		t.Errorf("expected AC3 and DTS, got %v", codecs.AudioCodecs)
	}
}

func TestDetectDVDCodecsFromFile_FallbackToPES(t *testing.T) {
	// Non-ISO file without IFO structure should fall back to PES scanning.
	buf := make([]byte, 256)
	// Video PES
	buf[0] = 0x00
	buf[1] = 0x00
	buf[2] = 0x01
	buf[3] = 0xE0
	// AC3 audio PES
	buf[20] = 0x00
	buf[21] = 0x00
	buf[22] = 0x01
	buf[23] = 0xBD
	buf[28] = 0x00 // PES header data length
	buf[29] = 0x80 // AC3 sub-stream ID

	dir := t.TempDir()
	path := filepath.Join(dir, "test.iso")
	if err := os.WriteFile(path, buf, 0644); err != nil {
		t.Fatal(err)
	}

	codecs, err := detectDVDCodecsFromFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(codecs.VideoCodecs) != 1 || codecs.VideoCodecs[0] != CodecMPEG2Video {
		t.Errorf("video = %v, want [MPEG-2]", codecs.VideoCodecs)
	}
	if len(codecs.AudioCodecs) != 1 || codecs.AudioCodecs[0] != CodecAC3Audio {
		t.Errorf("audio = %v, want [AC3]", codecs.AudioCodecs)
	}
}

func TestFindIFOsInISO(t *testing.T) {
	ifoData := buildTestIFO(1, [][2]byte{{0, 1}})
	iso := buildTestDVDISO([]testIFOEntry{
		{"VTS_01_0.IFO", ifoData},
		{"VTS_02_0.IFO", ifoData},
	})

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

	ifos := findIFOsInISO(f)
	if len(ifos) != 2 {
		t.Fatalf("got %d IFOs, want 2", len(ifos))
	}
	if ifos[0].Name != "VTS_01_0.IFO" {
		t.Errorf("IFO[0] name = %q, want VTS_01_0.IFO", ifos[0].Name)
	}
	if ifos[1].Name != "VTS_02_0.IFO" {
		t.Errorf("IFO[1] name = %q, want VTS_02_0.IFO", ifos[1].Name)
	}
}

func TestFindIFOsInISO_SkipsNonIFO(t *testing.T) {
	ifoData := buildTestIFO(1, [][2]byte{{0, 1}})
	// Include a VOB file alongside the IFO — should be skipped.
	vobData := make([]byte, 2048)
	iso := buildTestDVDISO([]testIFOEntry{
		{"VTS_01_0.IFO", ifoData},
		{"VTS_01_1.VOB", vobData},
	})

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

	ifos := findIFOsInISO(f)
	if len(ifos) != 1 {
		t.Fatalf("got %d IFOs, want 1 (should skip VOB)", len(ifos))
	}
}
