package source

import (
	"os"
	"path/filepath"
	"testing"
)

// buildTestBlurayISO creates a minimal ISO9660 filesystem containing a
// BDMV/STREAM/ directory with a single M2TS file. The M2TS data is provided
// by the caller. Returns the raw ISO bytes.
//
// ISO9660 layout:
//
//	Sector  0-15: System area (zeros)
//	Sector 16:    Primary Volume Descriptor (root dir at sector 20)
//	Sector 17:    Volume Descriptor Set Terminator
//	Sector 18:    (padding)
//	Sector 19:    (padding)
//	Sector 20:    Root directory (contains "." , ".." , "BDMV")
//	Sector 21:    BDMV directory (contains "." , ".." , "STREAM")
//	Sector 22:    STREAM directory (contains "." , ".." , "00000.M2TS")
//	Sector 23+:   M2TS file data (padded to sector boundary)
func buildTestBlurayISO(m2tsData []byte) []byte {
	const sector = 2048

	// M2TS file starts at sector 23, padded to sector boundary
	m2tsStartSector := 23
	m2tsSectors := (len(m2tsData) + sector - 1) / sector
	totalSectors := m2tsStartSector + m2tsSectors

	iso := make([]byte, totalSectors*sector)

	// --- Primary Volume Descriptor (sector 16) ---
	pvd := iso[16*sector:]
	pvd[0] = 1                                   // Type: Primary
	copy(pvd[1:6], []byte("CD001"))              // Standard Identifier
	pvd[6] = 1                                   // Version
	writeISO9660DirRecord(pvd[156:], 20, sector) // Root directory record

	// --- Volume Descriptor Set Terminator (sector 17) ---
	iso[17*sector] = 255 // Type: Terminator
	copy(iso[17*sector+1:17*sector+6], []byte("CD001"))

	// --- Root directory (sector 20) ---
	rootDir := iso[20*sector:]
	off := 0
	off += writeISO9660DirEntry(rootDir[off:], "\x00", 20, sector, true) // "."
	off += writeISO9660DirEntry(rootDir[off:], "\x01", 20, sector, true) // ".."
	off += writeISO9660DirEntry(rootDir[off:], "BDMV", 21, sector, true)

	// --- BDMV directory (sector 21) ---
	bdmvDir := iso[21*sector:]
	off = 0
	off += writeISO9660DirEntry(bdmvDir[off:], "\x00", 21, sector, true) // "."
	off += writeISO9660DirEntry(bdmvDir[off:], "\x01", 20, sector, true) // ".."
	off += writeISO9660DirEntry(bdmvDir[off:], "STREAM", 22, sector, true)

	// --- STREAM directory (sector 22) ---
	streamDir := iso[22*sector:]
	off = 0
	off += writeISO9660DirEntry(streamDir[off:], "\x00", 22, sector, true)                            // "."
	off += writeISO9660DirEntry(streamDir[off:], "\x01", 21, sector, true)                            // ".."
	off += writeISO9660DirEntry(streamDir[off:], "00000.M2TS", m2tsStartSector, len(m2tsData), false) // M2TS file

	// --- M2TS file data (sector 23+) ---
	copy(iso[m2tsStartSector*sector:], m2tsData)

	return iso
}

// writeISO9660DirRecord writes a 34-byte directory record into buf.
// Used for the root directory record in the PVD.
func writeISO9660DirRecord(buf []byte, extentLBA int, dataLen int) {
	buf[0] = 34 // Record length
	// Extent location (little-endian then big-endian)
	buf[2] = byte(extentLBA)
	buf[3] = byte(extentLBA >> 8)
	buf[4] = byte(extentLBA >> 16)
	buf[5] = byte(extentLBA >> 24)
	buf[6] = byte(extentLBA >> 24)
	buf[7] = byte(extentLBA >> 16)
	buf[8] = byte(extentLBA >> 8)
	buf[9] = byte(extentLBA)
	// Data length (little-endian then big-endian)
	buf[10] = byte(dataLen)
	buf[11] = byte(dataLen >> 8)
	buf[12] = byte(dataLen >> 16)
	buf[13] = byte(dataLen >> 24)
	buf[14] = byte(dataLen >> 24)
	buf[15] = byte(dataLen >> 16)
	buf[16] = byte(dataLen >> 8)
	buf[17] = byte(dataLen)
	// Flags: directory
	buf[25] = 0x02
	// File identifier length
	buf[32] = 1
	buf[33] = 0x00 // "." (root)
}

// writeISO9660DirEntry writes a directory entry and returns its total length.
func writeISO9660DirEntry(buf []byte, name string, extentLBA int, dataLen int, isDir bool) int {
	nameLen := len(name)
	recLen := 33 + nameLen
	if recLen%2 != 0 {
		recLen++ // Pad to even
	}

	buf[0] = byte(recLen) // Record length
	// Extent location (little-endian then big-endian)
	buf[2] = byte(extentLBA)
	buf[3] = byte(extentLBA >> 8)
	buf[4] = byte(extentLBA >> 16)
	buf[5] = byte(extentLBA >> 24)
	buf[6] = byte(extentLBA >> 24)
	buf[7] = byte(extentLBA >> 16)
	buf[8] = byte(extentLBA >> 8)
	buf[9] = byte(extentLBA)
	// Data length (little-endian then big-endian)
	buf[10] = byte(dataLen)
	buf[11] = byte(dataLen >> 8)
	buf[12] = byte(dataLen >> 16)
	buf[13] = byte(dataLen >> 24)
	buf[14] = byte(dataLen >> 24)
	buf[15] = byte(dataLen >> 16)
	buf[16] = byte(dataLen >> 8)
	buf[17] = byte(dataLen)
	// Flags
	if isDir {
		buf[25] = 0x02
	}
	// File identifier
	buf[32] = byte(nameLen)
	copy(buf[33:], name)

	return recLen
}

func TestFindBlurayM2TSInISO(t *testing.T) {
	m2tsData := buildBasicM2TSData()
	isoData := buildTestBlurayISO(m2tsData)

	// Write to temp file
	dir := t.TempDir()
	isoPath := filepath.Join(dir, "test.iso")
	if err := os.WriteFile(isoPath, isoData, 0644); err != nil {
		t.Fatal(err)
	}

	extents, err := findBlurayM2TSInISO(isoPath)
	if err != nil {
		t.Fatal(err)
	}

	if len(extents) != 1 {
		t.Fatalf("expected 1 M2TS file, got %d", len(extents))
	}

	e := extents[0]
	if e.Name != "00000.M2TS" {
		t.Errorf("expected name 00000.M2TS, got %q", e.Name)
	}
	if e.Offset != 23*2048 {
		t.Errorf("expected offset %d, got %d", 23*2048, e.Offset)
	}
	if e.Size != int64(len(m2tsData)) {
		t.Errorf("expected size %d, got %d", len(m2tsData), e.Size)
	}
}

func TestDetectType_BlurayISO(t *testing.T) {
	m2tsData := buildBasicM2TSData()
	isoData := buildTestBlurayISO(m2tsData)

	dir := t.TempDir()
	isoPath := filepath.Join(dir, "movie.iso")
	if err := os.WriteFile(isoPath, isoData, 0644); err != nil {
		t.Fatal(err)
	}

	typ, err := DetectType(dir)
	if err != nil {
		t.Fatal(err)
	}
	if typ != TypeBluray {
		t.Errorf("expected TypeBluray, got %v", typ)
	}
}

func TestEnumerateMediaFiles_BlurayISO(t *testing.T) {
	m2tsData := buildBasicM2TSData()
	isoData := buildTestBlurayISO(m2tsData)

	dir := t.TempDir()
	isoPath := filepath.Join(dir, "movie.iso")
	if err := os.WriteFile(isoPath, isoData, 0644); err != nil {
		t.Fatal(err)
	}

	files, err := EnumerateMediaFiles(dir, TypeBluray)
	if err != nil {
		t.Fatal(err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0] != "movie.iso" {
		t.Errorf("expected movie.iso, got %q", files[0])
	}
}

func TestEnumerateMediaFiles_BlurayExtractedPreferred(t *testing.T) {
	// When both extracted M2TS and ISOs exist, prefer extracted
	dir := t.TempDir()

	// Create extracted BDMV structure
	streamDir := filepath.Join(dir, "BDMV", "STREAM")
	if err := os.MkdirAll(streamDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(streamDir, "00000.m2ts"), []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}

	// Also create an ISO
	isoData := buildTestBlurayISO(buildBasicM2TSData())
	if err := os.WriteFile(filepath.Join(dir, "movie.iso"), isoData, 0644); err != nil {
		t.Fatal(err)
	}

	files, err := EnumerateMediaFiles(dir, TypeBluray)
	if err != nil {
		t.Fatal(err)
	}

	// Should return extracted M2TS, not the ISO
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d: %v", len(files), files)
	}
	if filepath.Base(files[0]) != "00000.m2ts" {
		t.Errorf("expected extracted M2TS, got %q", files[0])
	}
}

func TestISOAdapterAdjustsOffsets(t *testing.T) {
	m2tsData := buildBasicM2TSData()
	parser := NewMPEGTSParser(m2tsData)
	if err := parser.Parse(); err != nil {
		t.Fatal(err)
	}

	baseOffset := int64(23 * 2048) // Simulated ISO offset
	fullISO := make([]byte, int(baseOffset)+len(m2tsData))
	copy(fullISO[baseOffset:], m2tsData)

	adapter := newISOAdapter(parser, fullISO, baseOffset)

	// FilteredVideoRanges should return parser's ranges directly (zero-copy)
	parserRanges := parser.FilteredVideoRanges()
	adapterRanges := adapter.FilteredVideoRanges()

	if len(adapterRanges) != len(parserRanges) {
		t.Fatalf("range count mismatch: parser=%d, adapter=%d", len(parserRanges), len(adapterRanges))
	}

	for i, pr := range parserRanges {
		ar := adapterRanges[i]
		// Ranges should be identical (parser-relative, zero-copy)
		if ar.FileOffset != pr.FileOffset {
			t.Errorf("range %d: expected parser-relative FileOffset %d, got %d", i, pr.FileOffset, ar.FileOffset)
		}
		if ar.Size != pr.Size {
			t.Errorf("range %d: Size mismatch: %d vs %d", i, ar.Size, pr.Size)
		}
		if ar.ESOffset != pr.ESOffset {
			t.Errorf("range %d: ESOffset mismatch: %d vs %d", i, ar.ESOffset, pr.ESOffset)
		}
	}

	// FileOffsetConverter should add baseOffset
	conv := adapter.FileOffsetConverter()
	for i, pr := range parserRanges {
		isoOff := conv(pr.FileOffset)
		expected := pr.FileOffset + baseOffset
		if isoOff != expected {
			t.Errorf("range %d: FileOffsetConverter(%d) = %d, want %d", i, pr.FileOffset, isoOff, expected)
		}
	}

	// Verify Data() returns the full ISO data
	if len(adapter.Data()) != len(fullISO) {
		t.Errorf("Data() length: expected %d, got %d", len(fullISO), len(adapter.Data()))
	}

	// Verify DataSlice works with parser-relative offsets
	for i, pr := range parserRanges {
		data := adapter.DataSlice(pr.FileOffset, pr.Size)
		m2tsSlice := m2tsData[pr.FileOffset : pr.FileOffset+int64(pr.Size)]
		for j := range data {
			if data[j] != m2tsSlice[j] {
				t.Errorf("range %d byte %d: DataSlice data %02x != M2TS data %02x", i, j, data[j], m2tsSlice[j])
				break
			}
		}
	}

	// Verify ES reads still work (sub-slice-relative internally)
	data, err := adapter.ReadESData(0, 10, true)
	if err != nil {
		t.Fatalf("ReadESData: %v", err)
	}
	if len(data) != 10 {
		t.Errorf("ReadESData: expected 10 bytes, got %d", len(data))
	}
}

func TestIndexBlurayISO(t *testing.T) {
	m2tsData := buildBasicM2TSData()
	isoData := buildTestBlurayISO(m2tsData)

	dir := t.TempDir()
	isoPath := filepath.Join(dir, "movie.iso")
	if err := os.WriteFile(isoPath, isoData, 0644); err != nil {
		t.Fatal(err)
	}

	indexer, err := NewIndexer(dir, DefaultWindowSize)
	if err != nil {
		t.Fatal(err)
	}

	if indexer.SourceType() != TypeBluray {
		t.Fatalf("expected TypeBluray, got %v", indexer.SourceType())
	}

	if err := indexer.Build(nil); err != nil {
		t.Fatal(err)
	}

	index := indexer.Index()
	defer index.Close()

	// Should have source file entries
	if len(index.Files) == 0 {
		t.Fatal("expected at least one source file entry")
	}

	// All entries should reference the ISO
	for i, f := range index.Files {
		if f.RelativePath != "movie.iso" {
			t.Errorf("file %d: expected RelativePath movie.iso, got %q", i, f.RelativePath)
		}
		if f.Size != int64(len(isoData)) {
			t.Errorf("file %d: expected Size %d, got %d", i, len(isoData), f.Size)
		}
	}

	// Note: buildBasicM2TSData() uses sequential bytes without NAL start codes,
	// so no video sync points are found. The important thing is the indexing
	// completes and produces the right structure.

	// Should use ES offsets
	if !index.UsesESOffsets {
		t.Error("expected UsesESOffsets=true")
	}

	// Should have ESReaders
	if len(index.ESReaders) != len(index.Files) {
		t.Errorf("ESReaders count (%d) != Files count (%d)", len(index.ESReaders), len(index.Files))
	}
}

func TestDetectBlurayCodecsFromISO(t *testing.T) {
	m2tsData := buildBasicM2TSData()
	isoData := buildTestBlurayISO(m2tsData)

	dir := t.TempDir()
	isoPath := filepath.Join(dir, "movie.iso")
	if err := os.WriteFile(isoPath, isoData, 0644); err != nil {
		t.Fatal(err)
	}

	codecs, err := detectBlurayCodecsFromFile(isoPath)
	if err != nil {
		t.Fatal(err)
	}

	// buildBasicM2TSData uses H.264 video (0x1B) and AC3 audio (0x81)
	if len(codecs.VideoCodecs) != 1 || codecs.VideoCodecs[0] != CodecH264Video {
		t.Errorf("expected H.264 video codec, got %v", codecs.VideoCodecs)
	}
	if len(codecs.AudioCodecs) != 1 || codecs.AudioCodecs[0] != CodecAC3Audio {
		t.Errorf("expected AC3 audio codec, got %v", codecs.AudioCodecs)
	}
}
