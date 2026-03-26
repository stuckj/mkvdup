package source

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// buildTestCLPI creates a minimal CLPI binary with the given streams.
// Each stream is specified as a stream_coding_type byte.
func buildTestCLPI(streams []byte) []byte {
	// CLPI header: magic(4) + version(4) + SeqInfoOff(4) + ProgInfoOff(4) + ...
	// We put ProgramInfo immediately after the header at offset 40.
	const headerSize = 40
	const progInfoOff = headerSize

	// Build ProgramInfo section:
	// length(4) + reserved(1) + numSeqs(1)
	// Sequence: SPN(4) + PMT_PID(2) + numStreams(1) + numGroups(1)
	// Per stream: PID(2) + ciLen(1) + streamType(1) + padding(ciLen-1)
	ciLen := byte(5) // coding info length per stream (type + 4 bytes of attributes)
	seqHeaderSize := 8
	streamSize := 3 + int(ciLen)
	piContentSize := 2 + seqHeaderSize + len(streams)*streamSize // reserved+numSeqs + seqHeader + streams
	piTotalSize := 4 + piContentSize                             // length field + content

	data := make([]byte, progInfoOff+piTotalSize)

	// Header
	copy(data[0:4], "HDMV")
	copy(data[4:8], "0200")
	// SequenceInfo offset (unused, set to 0)
	binary.BigEndian.PutUint32(data[8:12], 0)
	// ProgramInfo offset
	binary.BigEndian.PutUint32(data[12:16], progInfoOff)

	// ProgramInfo
	pi := data[progInfoOff:]
	binary.BigEndian.PutUint32(pi[0:4], uint32(piContentSize))
	pi[4] = 0 // reserved
	pi[5] = 1 // 1 program sequence

	// Sequence header at offset 6
	off := 6
	binary.BigEndian.PutUint32(pi[off:off+4], 0)        // SPN
	binary.BigEndian.PutUint16(pi[off+4:off+6], 0x0100) // PMT PID
	pi[off+6] = byte(len(streams))                      // num streams
	pi[off+7] = 0                                       // num groups
	off += 8

	// Streams
	for i, st := range streams {
		binary.BigEndian.PutUint16(pi[off:off+2], uint16(0x1011+i)) // PID
		pi[off+2] = ciLen                                           // coding info length
		pi[off+3] = st                                              // stream_coding_type
		off += 3 + int(ciLen)
	}

	return data
}

func TestParseBlurayClipInfoCodecs_H264WithAC3(t *testing.T) {
	data := buildTestCLPI([]byte{0x1B, 0x81}) // H.264, AC3
	codecs, err := parseBlurayClipInfoCodecs(data)
	if err != nil {
		t.Fatal(err)
	}

	if len(codecs.VideoCodecs) != 1 || codecs.VideoCodecs[0] != CodecH264Video {
		t.Errorf("video = %v, want [H.264]", codecs.VideoCodecs)
	}
	if len(codecs.AudioCodecs) != 1 || codecs.AudioCodecs[0] != CodecAC3Audio {
		t.Errorf("audio = %v, want [AC3]", codecs.AudioCodecs)
	}
}

func TestParseBlurayClipInfoCodecs_H265WithTrueHD(t *testing.T) {
	data := buildTestCLPI([]byte{0x24, 0x83}) // H.265, TrueHD
	codecs, err := parseBlurayClipInfoCodecs(data)
	if err != nil {
		t.Fatal(err)
	}

	if len(codecs.VideoCodecs) != 1 || codecs.VideoCodecs[0] != CodecH265Video {
		t.Errorf("video = %v, want [H.265]", codecs.VideoCodecs)
	}
	if len(codecs.AudioCodecs) != 1 || codecs.AudioCodecs[0] != CodecTrueHDAudio {
		t.Errorf("audio = %v, want [TrueHD]", codecs.AudioCodecs)
	}
}

func TestParseBlurayClipInfoCodecs_MultipleStreams(t *testing.T) {
	data := buildTestCLPI([]byte{
		0x1B, // H.264
		0x86, // DTS-HD MA
		0x81, // AC3
		0x90, // PGS
	})
	codecs, err := parseBlurayClipInfoCodecs(data)
	if err != nil {
		t.Fatal(err)
	}

	if len(codecs.VideoCodecs) != 1 {
		t.Errorf("video count = %d, want 1", len(codecs.VideoCodecs))
	}
	if len(codecs.AudioCodecs) != 2 {
		t.Errorf("audio count = %d, want 2 (DTS-HD, AC3)", len(codecs.AudioCodecs))
	}
	if len(codecs.SubtitleCodecs) != 1 {
		t.Errorf("subtitle count = %d, want 1", len(codecs.SubtitleCodecs))
	}
}

func TestParseBlurayClipInfoCodecs_InvalidMagic(t *testing.T) {
	data := make([]byte, 64)
	copy(data[0:4], "NOPE")
	_, err := parseBlurayClipInfoCodecs(data)
	if err == nil {
		t.Error("expected error for invalid magic")
	}
}

func TestParseBlurayClipInfoCodecs_TooShort(t *testing.T) {
	_, err := parseBlurayClipInfoCodecs([]byte("HDMV"))
	if err == nil {
		t.Error("expected error for short data")
	}
}

// buildTestBlurayISOWithCLPI creates a minimal ISO9660 with both BDMV/STREAM/
// and BDMV/CLIPINF/ directories. The M2TS data and CLPI data are provided.
//
// Layout:
//
//	Sector 16: PVD
//	Sector 17: Terminator
//	Sector 20: Root directory
//	Sector 21: BDMV directory
//	Sector 22: STREAM directory
//	Sector 23: CLIPINF directory
//	Sector 24+: M2TS data
//	After M2TS: CLPI data
func buildTestBlurayISOWithCLPI(m2tsData, clpiData []byte) []byte {
	const sector = 2048

	m2tsStartSector := 24
	m2tsSectors := (len(m2tsData) + sector - 1) / sector
	clpiStartSector := m2tsStartSector + m2tsSectors
	clpiSectors := (len(clpiData) + sector - 1) / sector
	totalSectors := clpiStartSector + clpiSectors + 1

	iso := make([]byte, totalSectors*sector)

	// PVD
	pvd := iso[16*sector:]
	pvd[0] = 1
	copy(pvd[1:6], "CD001")
	pvd[6] = 1
	writeISO9660DirRecord(pvd[156:], 20, sector)

	// Terminator
	iso[17*sector] = 255
	copy(iso[17*sector+1:17*sector+6], "CD001")

	// Root directory
	rootDir := iso[20*sector:]
	off := 0
	off += writeISO9660DirEntry(rootDir[off:], "\x00", 20, sector, true)
	off += writeISO9660DirEntry(rootDir[off:], "\x01", 20, sector, true)
	off += writeISO9660DirEntry(rootDir[off:], "BDMV", 21, 2*sector, true)

	// BDMV directory
	bdmvDir := iso[21*sector:]
	off = 0
	off += writeISO9660DirEntry(bdmvDir[off:], "\x00", 21, 2*sector, true)
	off += writeISO9660DirEntry(bdmvDir[off:], "\x01", 20, sector, true)
	off += writeISO9660DirEntry(bdmvDir[off:], "STREAM", 22, sector, true)
	off += writeISO9660DirEntry(bdmvDir[off:], "CLIPINF", 23, sector, true)

	// STREAM directory
	streamDir := iso[22*sector:]
	off = 0
	off += writeISO9660DirEntry(streamDir[off:], "\x00", 22, sector, true)
	off += writeISO9660DirEntry(streamDir[off:], "\x01", 21, 2*sector, true)
	off += writeISO9660DirEntry(streamDir[off:], "00000.M2TS", m2tsStartSector, len(m2tsData), false)

	// CLIPINF directory
	clipinfDir := iso[23*sector:]
	off = 0
	off += writeISO9660DirEntry(clipinfDir[off:], "\x00", 23, sector, true)
	off += writeISO9660DirEntry(clipinfDir[off:], "\x01", 21, 2*sector, true)
	off += writeISO9660DirEntry(clipinfDir[off:], "00000.CLPI", clpiStartSector, len(clpiData), false)

	// Write data
	copy(iso[m2tsStartSector*sector:], m2tsData)
	copy(iso[clpiStartSector*sector:], clpiData)

	return iso
}

func TestDetectBlurayCodecsFromISO_CLPIBased(t *testing.T) {
	clpiData := buildTestCLPI([]byte{0x1B, 0x86, 0x81}) // H.264, DTS-HD MA, AC3
	m2tsData := buildBasicM2TSData()
	isoData := buildTestBlurayISOWithCLPI(m2tsData, clpiData)

	dir := t.TempDir()
	path := filepath.Join(dir, "test.iso")
	if err := os.WriteFile(path, isoData, 0644); err != nil {
		t.Fatal(err)
	}

	codecs, err := detectBlurayCodecsFromFile(path)
	if err != nil {
		t.Fatal(err)
	}

	// Should detect codecs from CLPI (not PMT).
	if len(codecs.VideoCodecs) != 1 || codecs.VideoCodecs[0] != CodecH264Video {
		t.Errorf("video = %v, want [H.264]", codecs.VideoCodecs)
	}
	// CLPI has both DTS-HD MA and AC3.
	if len(codecs.AudioCodecs) != 2 {
		t.Fatalf("audio count = %d, want 2", len(codecs.AudioCodecs))
	}
	hasDTSHD, hasAC3 := false, false
	for _, ct := range codecs.AudioCodecs {
		if ct == CodecDTSHDAudio {
			hasDTSHD = true
		}
		if ct == CodecAC3Audio {
			hasAC3 = true
		}
	}
	if !hasDTSHD || !hasAC3 {
		t.Errorf("expected DTS-HD and AC3, got %v", codecs.AudioCodecs)
	}
}

func TestDetectBlurayCodecsFromISO_FallbackToPMT(t *testing.T) {
	// ISO with M2TS but no CLIPINF directory should fall back to PMT scanning.
	m2tsData := buildBasicM2TSData()
	isoData := buildTestBlurayISO(m2tsData)

	dir := t.TempDir()
	path := filepath.Join(dir, "test.iso")
	if err := os.WriteFile(path, isoData, 0644); err != nil {
		t.Fatal(err)
	}

	codecs, err := detectBlurayCodecsFromFile(path)
	if err != nil {
		t.Fatal(err)
	}

	// buildBasicM2TSData has H.264 video and AC3 audio in PMT.
	if len(codecs.VideoCodecs) != 1 || codecs.VideoCodecs[0] != CodecH264Video {
		t.Errorf("video = %v, want [H.264]", codecs.VideoCodecs)
	}
	if len(codecs.AudioCodecs) != 1 || codecs.AudioCodecs[0] != CodecAC3Audio {
		t.Errorf("audio = %v, want [AC3]", codecs.AudioCodecs)
	}
}

func TestDetectBlurayCodecsFromCLPIDir(t *testing.T) {
	dir := t.TempDir()
	clipinfDir := filepath.Join(dir, "BDMV", "CLIPINF")
	if err := os.MkdirAll(clipinfDir, 0755); err != nil {
		t.Fatal(err)
	}

	clpiData := buildTestCLPI([]byte{0x1B, 0x83, 0x90}) // H.264, TrueHD, PGS
	if err := os.WriteFile(filepath.Join(clipinfDir, "00000.clpi"), clpiData, 0644); err != nil {
		t.Fatal(err)
	}

	codecs, err := detectBlurayCodecsFromCLPIDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(codecs.VideoCodecs) != 1 || codecs.VideoCodecs[0] != CodecH264Video {
		t.Errorf("video = %v, want [H.264]", codecs.VideoCodecs)
	}
	if len(codecs.AudioCodecs) != 1 || codecs.AudioCodecs[0] != CodecTrueHDAudio {
		t.Errorf("audio = %v, want [TrueHD]", codecs.AudioCodecs)
	}
	if len(codecs.SubtitleCodecs) != 1 || codecs.SubtitleCodecs[0] != CodecPGSSubtitle {
		t.Errorf("subtitles = %v, want [PGS]", codecs.SubtitleCodecs)
	}
}

func TestDetectBlurayCodecsFromCLPIDir_NoCLPIDir(t *testing.T) {
	dir := t.TempDir()
	_, err := detectBlurayCodecsFromCLPIDir(dir)
	if err == nil {
		t.Error("expected error when CLIPINF dir doesn't exist")
	}
}
