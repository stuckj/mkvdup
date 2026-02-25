package source

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// buildTestUDFBlurayISO creates a minimal UDF (Type 1 physical partition)
// filesystem containing a BDMV/STREAM/ directory with a single M2TS file.
// Returns the raw ISO bytes. This uses no ISO9660 bridge — sector 16
// contains VRS markers instead of a PVD.
//
// UDF layout (2048-byte sectors):
//
//	Sector  0-15:  System area (zeros)
//	Sector 16:     BEA01 (Volume Recognition Sequence)
//	Sector 17:     NSR03 (UDF 2.x marker)
//	Sector 18:     TEA01 (VRS terminator)
//	Sector 32-34:  Volume Descriptor Sequence (Partition Desc, LVD, Terminator)
//	Sector 256:    Anchor Volume Descriptor Pointer (→ VDS at sector 32)
//	Sector 257:    File Set Descriptor (→ root at sector 258)
//	Sector 258:    Root directory File Entry (→ dir data at sector 259)
//	Sector 259:    Root directory FIDs (contains BDMV)
//	Sector 260:    BDMV File Entry (→ dir data at sector 261)
//	Sector 261:    BDMV directory FIDs (contains STREAM)
//	Sector 262:    STREAM File Entry (→ dir data at sector 263)
//	Sector 263:    STREAM directory FIDs (contains 00000.M2TS)
//	Sector 264:    00000.M2TS File Entry (→ data at sector 265+)
//	Sector 265+:   M2TS file data
//
// Partition starts at sector 257 (all logical blocks are relative to this).
func buildTestUDFBlurayISO(m2tsData []byte) []byte {
	const sector = 2048
	const partStart = 257 // partition start sector

	m2tsStartSector := 265
	m2tsSectors := (len(m2tsData) + sector - 1) / sector
	totalSectors := m2tsStartSector + m2tsSectors
	if totalSectors < 266 {
		totalSectors = 266
	}

	iso := make([]byte, totalSectors*sector)

	// --- Volume Recognition Sequence (sectors 16-18) ---
	writeVRSDescriptor(iso[16*sector:], "BEA01")
	writeVRSDescriptor(iso[17*sector:], "NSR03")
	writeVRSDescriptor(iso[18*sector:], "TEA01")

	// --- VDS: Partition Descriptor (sector 32) ---
	writeUDFPartitionDesc(iso[32*sector:], 32, 0, uint32(partStart))

	// --- VDS: Logical Volume Descriptor (sector 33) ---
	// FSD at logical block 0 (= physical sector partStart + 0 = 257), partition ref 0
	writeUDFLogicalVolumeDesc(iso[33*sector:], 33, uint32(sector), 0, 0)

	// --- VDS: Terminating Descriptor (sector 34) ---
	writeUDFTag(iso[34*sector:], 8, 34)

	// --- AVDP at sector 256 ---
	writeUDFAVDP(iso[256*sector:], 32, 3*sector) // VDS at sector 32, length 3 sectors

	// --- File Set Descriptor (sector 257 = partition LB 0) ---
	// Root directory ICB at LB 1 (= sector 258), partition ref 0
	writeUDFFSD(iso[257*sector:], 0, 1, 0)

	// --- Root directory ---
	// File Entry at sector 258 (LB 1), dir data at LB 2 (sector 259), size = 1 sector
	writeUDFFileEntry(iso[258*sector:], 1, 4, uint64(sector), 0, 2, uint32(sector))
	// FIDs: parent + BDMV (pointing to LB 3 = sector 260)
	writeRootDirFIDs(iso[259*sector:], 2, 3, 0)

	// --- BDMV directory ---
	// File Entry at sector 260 (LB 3), dir data at LB 4 (sector 261), size = 1 sector
	writeUDFFileEntry(iso[260*sector:], 3, 4, uint64(sector), 0, 4, uint32(sector))
	// FIDs: parent + STREAM (pointing to LB 5 = sector 262)
	writeBDMVDirFIDs(iso[261*sector:], 4, 5, 0)

	// --- STREAM directory ---
	// File Entry at sector 262 (LB 5), dir data at LB 6 (sector 263), size = 1 sector
	writeUDFFileEntry(iso[262*sector:], 5, 4, uint64(sector), 0, 6, uint32(sector))
	// FIDs: parent + 00000.M2TS (pointing to LB 7 = sector 264)
	writeSTREAMDirFIDs(iso[263*sector:], 6, 7, 0)

	// --- 00000.M2TS File Entry at sector 264 (LB 7) ---
	m2tsLB := uint32(m2tsStartSector - partStart) // logical block within partition
	writeUDFFileEntry(iso[264*sector:], 7, 5, uint64(len(m2tsData)), 0, m2tsLB, uint32(len(m2tsData)))

	// --- M2TS data at sector 265+ ---
	copy(iso[m2tsStartSector*sector:], m2tsData)

	return iso
}

// --- UDF structure writers for test ISO construction ---

// writeVRSDescriptor writes a Volume Recognition Sequence descriptor.
func writeVRSDescriptor(buf []byte, ident string) {
	buf[0] = 0 // Structure Type
	copy(buf[1:6], []byte(ident))
	buf[6] = 1 // Version
}

// writeUDFTag writes a 16-byte UDF descriptor tag.
func writeUDFTag(buf []byte, tagID uint16, tagLocation uint32) {
	binary.LittleEndian.PutUint16(buf[0:2], tagID)
	binary.LittleEndian.PutUint16(buf[2:4], 2)  // Descriptor Version
	binary.LittleEndian.PutUint16(buf[6:8], 0)  // Descriptor CRC Length
	binary.LittleEndian.PutUint16(buf[8:10], 0) // Descriptor CRC
	binary.LittleEndian.PutUint32(buf[12:16], tagLocation)
	// Compute tag checksum (sum of bytes 0-3 + 5-15, skipping byte 4)
	var sum byte
	for i := 0; i < 16; i++ {
		if i == 4 {
			continue
		}
		sum += buf[i]
	}
	buf[4] = sum
}

// writeUDFAVDP writes an Anchor Volume Descriptor Pointer.
func writeUDFAVDP(buf []byte, vdsLocation uint32, vdsLength int) {
	writeUDFTag(buf, udfTagAVDP, 256)
	// Main VDS extent at offset 16
	binary.LittleEndian.PutUint32(buf[16:20], uint32(vdsLength))
	binary.LittleEndian.PutUint32(buf[20:24], vdsLocation)
	// Recompute tag checksum
	recomputeTagChecksum(buf)
}

// writeUDFPartitionDesc writes a Partition Descriptor.
func writeUDFPartitionDesc(buf []byte, tagLoc uint32, partNum uint16, startSector uint32) {
	writeUDFTag(buf, udfTagPartitionDesc, tagLoc)
	binary.LittleEndian.PutUint16(buf[22:24], partNum)
	binary.LittleEndian.PutUint32(buf[188:192], startSector)
	recomputeTagChecksum(buf)
}

// writeUDFLogicalVolumeDesc writes a Logical Volume Descriptor with a
// Type 1 physical partition map.
func writeUDFLogicalVolumeDesc(buf []byte, tagLoc uint32, blockSize uint32, fsdLB uint32, fsdPartRef uint16) {
	writeUDFTag(buf, udfTagLogicalVolume, tagLoc)
	binary.LittleEndian.PutUint32(buf[212:216], blockSize)

	// FSD location at offset 248 (long_ad: 4B length + 4B location + 2B partRef + 6B impl)
	binary.LittleEndian.PutUint32(buf[248:252], blockSize) // length = 1 block
	binary.LittleEndian.PutUint32(buf[252:256], fsdLB)
	binary.LittleEndian.PutUint16(buf[256:258], fsdPartRef)

	// Partition map table at offset 440
	// Type 1 partition map: 6 bytes
	mapTableLen := uint32(6)
	numMaps := uint32(1)
	binary.LittleEndian.PutUint32(buf[264:268], mapTableLen)
	binary.LittleEndian.PutUint32(buf[268:272], numMaps)

	// Type 1 map: type=1, length=6, volSeqNum=1, partNum=0
	buf[440] = 1                                   // Type
	buf[441] = 6                                   // Length
	binary.LittleEndian.PutUint16(buf[442:444], 1) // Volume Sequence Number
	binary.LittleEndian.PutUint16(buf[444:446], 0) // Partition Number

	recomputeTagChecksum(buf)
}

// writeUDFFSD writes a File Set Descriptor.
func writeUDFFSD(buf []byte, tagLoc uint32, rootICBLB uint32, rootICBPartRef uint16) {
	writeUDFTag(buf, udfTagFileSetDesc, tagLoc)
	// Root directory ICB at offset 400 (long_ad)
	binary.LittleEndian.PutUint32(buf[400:404], 2048)      // length
	binary.LittleEndian.PutUint32(buf[404:408], rootICBLB) // location
	binary.LittleEndian.PutUint16(buf[408:410], rootICBPartRef)
	recomputeTagChecksum(buf)
}

// writeUDFFileEntry writes a File Entry (tag 261).
// fileType: 4=directory, 5=regular file
// allocType: 0=short_ad, 1=long_ad
// allocLB/allocLen: single extent location and length
func writeUDFFileEntry(buf []byte, tagLoc uint32, fileType byte, infoLen uint64, allocType byte, allocLB uint32, allocLen uint32) {
	writeUDFTag(buf, udfTagFileEntry, tagLoc)

	// ICB Tag at offset 16 (20 bytes)
	// File type at offset 27 (= 16 + 11)
	buf[27] = fileType
	// ICB flags (alloc type) at offset 34-35 (= 16 + 18-19)
	binary.LittleEndian.PutUint16(buf[34:36], uint16(allocType))

	// Information Length at offset 56 (8 bytes)
	binary.LittleEndian.PutUint64(buf[56:64], infoLen)

	// For tag 261 (ECMA-167 14.9): L_EA at 168, L_AD at 172
	// Allocation descriptors start at offset 176 + L_EA
	if allocType == 0 {
		// short_ad: 8 bytes
		binary.LittleEndian.PutUint32(buf[168:172], 0) // L_EA (extended attributes length)
		binary.LittleEndian.PutUint32(buf[172:176], 8) // L_AD (allocation descriptors length)
		// short_ad at offset 176
		binary.LittleEndian.PutUint32(buf[176:180], allocLen)
		binary.LittleEndian.PutUint32(buf[180:184], allocLB)
	}

	recomputeTagChecksum(buf)
}

// writeRootDirFIDs writes root directory FIDs: parent entry + "BDMV" directory.
func writeRootDirFIDs(buf []byte, tagLoc uint32, bdmvLB uint32, bdmvPartRef uint16) {
	off := 0
	// Parent entry (characteristics 0x0A = parent + directory)
	off += writeUDFFID(buf[off:], tagLoc, 0x0A, udfLongAD{Length: 2048, Location: 0}, "")
	// BDMV directory entry
	off += writeUDFFID(buf[off:], tagLoc, 0x02, udfLongAD{Length: 2048, Location: bdmvLB, PartRef: bdmvPartRef}, "BDMV")
	_ = off
}

// writeBDMVDirFIDs writes BDMV directory FIDs: parent entry + "STREAM" directory.
func writeBDMVDirFIDs(buf []byte, tagLoc uint32, streamLB uint32, streamPartRef uint16) {
	off := 0
	off += writeUDFFID(buf[off:], tagLoc, 0x0A, udfLongAD{Length: 2048, Location: 0}, "")
	off += writeUDFFID(buf[off:], tagLoc, 0x02, udfLongAD{Length: 2048, Location: streamLB, PartRef: streamPartRef}, "STREAM")
	_ = off
}

// writeSTREAMDirFIDs writes STREAM directory FIDs: parent + "00000.M2TS" file.
func writeSTREAMDirFIDs(buf []byte, tagLoc uint32, m2tsLB uint32, m2tsPartRef uint16) {
	off := 0
	off += writeUDFFID(buf[off:], tagLoc, 0x0A, udfLongAD{Length: 2048, Location: 0}, "")
	off += writeUDFFID(buf[off:], tagLoc, 0x00, udfLongAD{Length: 2048, Location: m2tsLB, PartRef: m2tsPartRef}, "00000.M2TS")
	_ = off
}

// writeUDFFID writes a File Identifier Descriptor and returns the total padded length.
func writeUDFFID(buf []byte, tagLoc uint32, characteristics byte, icb udfLongAD, name string) int {
	writeUDFTag(buf, udfTagFID, tagLoc)

	buf[18] = characteristics

	// Encode name as UDF d-string (compression ID 8 = 8-bit)
	var nameBytes []byte
	if name != "" {
		nameBytes = make([]byte, 1+len(name))
		nameBytes[0] = 8 // compression ID
		copy(nameBytes[1:], name)
	}
	buf[19] = byte(len(nameBytes))

	// ICB at offset 20 (16-byte long_ad)
	binary.LittleEndian.PutUint32(buf[20:24], icb.Length)
	binary.LittleEndian.PutUint32(buf[24:28], icb.Location)
	binary.LittleEndian.PutUint16(buf[28:30], icb.PartRef)

	// L_IU (implementation use length) at offset 36
	binary.LittleEndian.PutUint16(buf[36:38], 0)

	// Name starts at offset 38 + L_IU = 38
	copy(buf[38:], nameBytes)

	recomputeTagChecksum(buf)

	// Total length padded to 4-byte boundary
	fidLen := 38 + len(nameBytes)
	fidLen = (fidLen + 3) &^ 3
	return fidLen
}

// recomputeTagChecksum updates the tag checksum byte (offset 4) for a
// UDF descriptor tag.
func recomputeTagChecksum(buf []byte) {
	if len(buf) < 16 {
		return
	}
	var sum byte
	for i := 0; i < 16; i++ {
		if i == 4 {
			continue
		}
		sum += buf[i]
	}
	buf[4] = sum
}

// --- Tests ---

func TestIsUDFImage(t *testing.T) {
	m2tsData := buildBasicM2TSData()
	isoData := buildTestUDFBlurayISO(m2tsData)

	dir := t.TempDir()
	isoPath := filepath.Join(dir, "test.iso")
	if err := os.WriteFile(isoPath, isoData, 0644); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(isoPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if !isUDFImage(f) {
		t.Error("expected isUDFImage to return true for UDF ISO")
	}
}

func TestIsUDFImage_NotUDF(t *testing.T) {
	// A regular ISO9660 image should not be detected as UDF
	m2tsData := buildBasicM2TSData()
	isoData := buildTestBlurayISO(m2tsData)

	dir := t.TempDir()
	isoPath := filepath.Join(dir, "test.iso")
	if err := os.WriteFile(isoPath, isoData, 0644); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(isoPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if isUDFImage(f) {
		t.Error("expected isUDFImage to return false for ISO9660 ISO")
	}
}

func TestFindBlurayM2TSInUDF(t *testing.T) {
	m2tsData := buildBasicM2TSData()
	isoData := buildTestUDFBlurayISO(m2tsData)

	dir := t.TempDir()
	isoPath := filepath.Join(dir, "test.iso")
	if err := os.WriteFile(isoPath, isoData, 0644); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(isoPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	extents, err := findBlurayM2TSInUDF(f)
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
	if e.Offset != 265*2048 {
		t.Errorf("expected offset %d, got %d", 265*2048, e.Offset)
	}
	if e.Size != int64(len(m2tsData)) {
		t.Errorf("expected size %d, got %d", len(m2tsData), e.Size)
	}
}

func TestDetectType_UDFBlurayISO(t *testing.T) {
	m2tsData := buildBasicM2TSData()
	isoData := buildTestUDFBlurayISO(m2tsData)

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

func TestEnumerateMediaFiles_UDFBlurayISO(t *testing.T) {
	m2tsData := buildBasicM2TSData()
	isoData := buildTestUDFBlurayISO(m2tsData)

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

func TestISOAdapterAdjustsOffsets_UDF(t *testing.T) {
	m2tsData := buildBasicM2TSData()
	parser := NewMPEGTSParser(m2tsData)
	if err := parser.Parse(); err != nil {
		t.Fatal(err)
	}

	baseOffset := int64(265 * 2048) // UDF ISO offset
	fullISO := make([]byte, int(baseOffset)+len(m2tsData))
	copy(fullISO[baseOffset:], m2tsData)

	adapter := newISOAdapter(parser, fullISO, baseOffset)

	parserRanges := parser.FilteredVideoRanges()
	adapterRanges := adapter.FilteredVideoRanges()

	if len(adapterRanges) != len(parserRanges) {
		t.Fatalf("range count mismatch: parser=%d, adapter=%d", len(parserRanges), len(adapterRanges))
	}

	for i, pr := range parserRanges {
		ar := adapterRanges[i]
		expectedOffset := pr.FileOffset + baseOffset
		if ar.FileOffset != expectedOffset {
			t.Errorf("range %d: expected FileOffset %d, got %d", i, expectedOffset, ar.FileOffset)
		}
	}
}

func TestIndexBlurayUDFISO(t *testing.T) {
	m2tsData := buildBasicM2TSData()
	isoData := buildTestUDFBlurayISO(m2tsData)

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

	if len(index.Files) == 0 {
		t.Fatal("expected at least one source file entry")
	}

	for i, f := range index.Files {
		if f.RelativePath != "movie.iso" {
			t.Errorf("file %d: expected RelativePath movie.iso, got %q", i, f.RelativePath)
		}
	}

	if !index.UsesESOffsets {
		t.Error("expected UsesESOffsets=true")
	}
}

func TestDetectBlurayCodecsFromUDFISO(t *testing.T) {
	m2tsData := buildBasicM2TSData()
	isoData := buildTestUDFBlurayISO(m2tsData)

	dir := t.TempDir()
	isoPath := filepath.Join(dir, "movie.iso")
	if err := os.WriteFile(isoPath, isoData, 0644); err != nil {
		t.Fatal(err)
	}

	codecs, err := detectBlurayCodecsFromFile(isoPath)
	if err != nil {
		t.Fatal(err)
	}

	if len(codecs.VideoCodecs) != 1 || codecs.VideoCodecs[0] != CodecH264Video {
		t.Errorf("expected H.264 video codec, got %v", codecs.VideoCodecs)
	}
	if len(codecs.AudioCodecs) != 1 || codecs.AudioCodecs[0] != CodecAC3Audio {
		t.Errorf("expected AC3 audio codec, got %v", codecs.AudioCodecs)
	}
}
