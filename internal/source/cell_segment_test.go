package source

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// buildTestNAVPack writes a minimal NAV pack (PCI + DSI) into data at the given offset.
// sectorLBN is the sector address to set in the PCI nv_pck_lbn field.
// Returns the number of bytes written.
func buildTestNAVPack(data []byte, offset int, sectorLBN uint32) int {
	pos := offset

	// Pack header: 00 00 01 BA + MPEG-2 format (14 bytes)
	data[pos] = 0x00
	data[pos+1] = 0x00
	data[pos+2] = 0x01
	data[pos+3] = 0xBA
	data[pos+4] = 0x44 // MPEG-2 marker
	data[pos+13] = 0x00
	pos += 14

	// PCI packet: 00 00 01 BF + length 980
	data[pos] = 0x00
	data[pos+1] = 0x00
	data[pos+2] = 0x01
	data[pos+3] = 0xBF
	binary.BigEndian.PutUint16(data[pos+4:pos+6], 980)
	// nv_pck_lbn at payload offset 0 (bytes 6-9)
	binary.BigEndian.PutUint32(data[pos+6:pos+10], sectorLBN)
	pos += 6 + 980

	// DSI packet: 00 00 01 BF + length 1018
	data[pos] = 0x00
	data[pos+1] = 0x00
	data[pos+2] = 0x01
	data[pos+3] = 0xBF
	binary.BigEndian.PutUint16(data[pos+4:pos+6], 1018)
	pos += 6 + 1018

	return pos - offset
}

// buildTestVideoPES writes a minimal video PES packet into data at the given offset.
// Returns the number of bytes written.
func buildTestVideoPES(data []byte, offset int, payloadSize int) int {
	pos := offset

	// PES header: 00 00 01 E0
	data[pos] = 0x00
	data[pos+1] = 0x00
	data[pos+2] = 0x01
	data[pos+3] = 0xE0
	pesLength := payloadSize + 3 // flags(2) + header_data_len(1)
	binary.BigEndian.PutUint16(data[pos+4:pos+6], uint16(pesLength))
	data[pos+6] = 0x80 // MPEG-2 PES marker
	data[pos+7] = 0x00 // no PTS/DTS
	data[pos+8] = 0x00 // header_data_length = 0
	pos += 9

	// Fill payload with identifiable pattern
	for i := range payloadSize {
		data[pos+i] = byte((offset + i) & 0xFF)
	}
	pos += payloadSize

	return pos - offset
}

func TestParseDVDIFOCells(t *testing.T) {
	// Build a minimal VTS IFO with a C_ADT containing 3 cells across 2 VOBs
	ifoData := make([]byte, 4*isoSectorSize) // 4 sectors

	// VTS_MAT header
	copy(ifoData[0:12], "DVDVIDEO-VTS")
	// vts_vobs at offset 0x84: sector 100 (relative to IFO start)
	binary.BigEndian.PutUint32(ifoData[0x84:0x88], 100)
	// vts_c_adt at offset 0xE0: sector 2 (relative to IFO start)
	binary.BigEndian.PutUint32(ifoData[0xE0:0xE4], 2)

	// C_ADT at sector 2 of the IFO data
	cadt := ifoData[2*isoSectorSize:]
	// nr_of_vobs = 2
	binary.BigEndian.PutUint16(cadt[0:2], 2)
	// last_byte = 8 + 3*12 - 1 = 43
	binary.BigEndian.PutUint32(cadt[4:8], 43)
	// Entry 0: VOB 1, Cell 1, sectors 0-999
	binary.BigEndian.PutUint16(cadt[8:10], 1)    // vob_id
	cadt[10] = 1                                 // cell_id
	binary.BigEndian.PutUint32(cadt[12:16], 0)   // start_sector
	binary.BigEndian.PutUint32(cadt[16:20], 999) // last_sector
	// Entry 1: VOB 2, Cell 1, sectors 1000-1999
	binary.BigEndian.PutUint16(cadt[20:22], 2)
	cadt[22] = 1
	binary.BigEndian.PutUint32(cadt[24:28], 1000)
	binary.BigEndian.PutUint32(cadt[28:32], 1999)
	// Entry 2: VOB 1, Cell 2, sectors 2000-2999
	binary.BigEndian.PutUint16(cadt[32:34], 1)
	cadt[34] = 2
	binary.BigEndian.PutUint32(cadt[36:40], 2000)
	binary.BigEndian.PutUint32(cadt[40:44], 2999)

	info, err := parseDVDIFOCells(ifoData, 500)
	if err != nil {
		t.Fatalf("parseDVDIFOCells() error = %v", err)
	}

	// VTS VOBs start at IFO sector (500) + vts_vobs (100) = 600
	if info.VTSVobsSector != 600 {
		t.Errorf("VTSVobsSector = %d, want 600", info.VTSVobsSector)
	}

	if len(info.Cells) != 3 {
		t.Fatalf("len(Cells) = %d, want 3", len(info.Cells))
	}

	// Verify cell entries
	expected := []cellAddrEntry{
		{VOBId: 1, CellId: 1, StartSector: 0, LastSector: 999},
		{VOBId: 2, CellId: 1, StartSector: 1000, LastSector: 1999},
		{VOBId: 1, CellId: 2, StartSector: 2000, LastSector: 2999},
	}
	for i, exp := range expected {
		got := info.Cells[i]
		if got.VOBId != exp.VOBId || got.CellId != exp.CellId ||
			got.StartSector != exp.StartSector || got.LastSector != exp.LastSector {
			t.Errorf("Cells[%d] = %+v, want %+v", i, got, exp)
		}
	}
}

func TestParseDVDIFOCells_InvalidMagic(t *testing.T) {
	ifoData := make([]byte, 0xE4)
	copy(ifoData[0:12], "NOT_A_VTS!!!")
	_, err := parseDVDIFOCells(ifoData, 0)
	if err == nil {
		t.Error("expected error for invalid magic")
	}
}

func TestBuildCellSegments_NoVOBUs(t *testing.T) {
	p := &MPEGPSParser{}
	p.buildCellSegments()
	if p.CellSegmentCount() != 0 {
		t.Errorf("CellSegmentCount() = %d, want 0", p.CellSegmentCount())
	}
}

func TestRebaseRanges(t *testing.T) {
	ranges := []PESPayloadRange{
		{FileOffset: 1000, Size: 200, ESOffset: 5000},
		{FileOffset: 2000, Size: 300, ESOffset: 5200},
		{FileOffset: 3000, Size: 100, ESOffset: 5500},
	}

	rebased := rebaseRanges(ranges)
	if len(rebased) != 3 {
		t.Fatalf("rebaseRanges() returned %d ranges, want 3", len(rebased))
	}

	if rebased[0].ESOffset != 0 {
		t.Errorf("rebased[0].ESOffset = %d, want 0", rebased[0].ESOffset)
	}
	if rebased[1].ESOffset != 200 {
		t.Errorf("rebased[1].ESOffset = %d, want 200", rebased[1].ESOffset)
	}
	if rebased[2].ESOffset != 500 {
		t.Errorf("rebased[2].ESOffset = %d, want 500", rebased[2].ESOffset)
	}

	if rebased[0].FileOffset != 1000 || rebased[0].Size != 200 {
		t.Errorf("rebased[0] = {%d, %d}, want {1000, 200}", rebased[0].FileOffset, rebased[0].Size)
	}
}

func TestRebaseRanges_Empty(t *testing.T) {
	if rebased := rebaseRanges(nil); rebased != nil {
		t.Errorf("rebaseRanges(nil) = %v, want nil", rebased)
	}
}

func TestCellSegmentVideoRanges(t *testing.T) {
	p := &MPEGPSParser{
		videoRanges: []PESPayloadRange{
			{FileOffset: 100, Size: 50, ESOffset: 0},
			{FileOffset: 200, Size: 50, ESOffset: 50},
			{FileOffset: 300, Size: 50, ESOffset: 100},
			{FileOffset: 400, Size: 50, ESOffset: 150},
		},
		vobus: []vobuInfo{
			{sectorLBN: 10, videoRangeStart: 0, audioRangeStart: 0},
			{sectorLBN: 20, videoRangeStart: 2, audioRangeStart: 0},
		},
		cellSegments: []cellSegment{
			{vobuStart: 0, vobuEnd: 1},
			{vobuStart: 1, vobuEnd: 2},
		},
	}

	seg0 := p.CellSegmentVideoRanges(0)
	if len(seg0) != 2 {
		t.Fatalf("segment 0: %d ranges, want 2", len(seg0))
	}
	if seg0[0].FileOffset != 100 || seg0[0].ESOffset != 0 {
		t.Errorf("seg0[0] = {FileOffset:%d, ESOffset:%d}, want {100, 0}", seg0[0].FileOffset, seg0[0].ESOffset)
	}
	if seg0[1].FileOffset != 200 || seg0[1].ESOffset != 50 {
		t.Errorf("seg0[1] = {FileOffset:%d, ESOffset:%d}, want {200, 50}", seg0[1].FileOffset, seg0[1].ESOffset)
	}

	seg1 := p.CellSegmentVideoRanges(1)
	if len(seg1) != 2 {
		t.Fatalf("segment 1: %d ranges, want 2", len(seg1))
	}
	if seg1[0].FileOffset != 300 || seg1[0].ESOffset != 0 {
		t.Errorf("seg1[0] = {FileOffset:%d, ESOffset:%d}, want {300, 0}", seg1[0].FileOffset, seg1[0].ESOffset)
	}
}

func TestCellSegmentVideoRanges_InvalidIndex(t *testing.T) {
	p := &MPEGPSParser{}
	if ranges := p.CellSegmentVideoRanges(-1); ranges != nil {
		t.Error("expected nil for negative index")
	}
	if ranges := p.CellSegmentVideoRanges(0); ranges != nil {
		t.Error("expected nil for index beyond segments")
	}
}

func TestCellSegmentAdapter_VideoES(t *testing.T) {
	data := make([]byte, 2000)
	for i := range 200 {
		data[100+i] = 0xAA
	}
	for i := range 200 {
		data[500+i] = 0xBB
	}

	p := &MPEGPSParser{
		data: data,
		size: int64(len(data)),
		videoRanges: []PESPayloadRange{
			{FileOffset: 100, Size: 200, ESOffset: 0},
			{FileOffset: 500, Size: 200, ESOffset: 200},
		},
		vobus: []vobuInfo{
			{sectorLBN: 10, videoRangeStart: 0, audioRangeStart: 0},
			{sectorLBN: 20, videoRangeStart: 1, audioRangeStart: 0},
		},
		cellSegments: []cellSegment{
			{vobuStart: 0, vobuEnd: 1},
			{vobuStart: 1, vobuEnd: 2},
		},
		lpcmSubStreams: map[byte]bool{},
	}

	adapter := newCellSegmentAdapter(p, 1)

	if esSize := adapter.TotalESSize(true); esSize != 200 {
		t.Errorf("TotalESSize(true) = %d, want 200", esSize)
	}
	if esSize := adapter.TotalESSize(false); esSize != 0 {
		t.Errorf("TotalESSize(false) = %d, want 0", esSize)
	}

	esData, err := adapter.ReadESData(0, 10, true)
	if err != nil {
		t.Fatalf("ReadESData() error = %v", err)
	}
	for i, b := range esData {
		if b != 0xBB {
			t.Errorf("ReadESData byte %d = 0x%02X, want 0xBB", i, b)
			break
		}
	}

	b, hint, ok := adapter.ReadESByteWithHint(0, true, -1)
	if !ok || b != 0xBB {
		t.Errorf("ReadESByteWithHint(0) = {0x%02X, %d, %v}, want {0xBB, _, true}", b, hint, ok)
	}

	fileOff, remaining := adapter.ESOffsetToFileOffset(10, true)
	if fileOff != 510 || remaining != 190 {
		t.Errorf("ESOffsetToFileOffset(10, true) = {%d, %d}, want {510, 190}", fileOff, remaining)
	}

	fileOff, _ = adapter.ESOffsetToFileOffset(0, false)
	if fileOff != -1 {
		t.Errorf("ESOffsetToFileOffset(0, false) = %d, want -1", fileOff)
	}
}

func TestCellSegmentAdapter_Parser(t *testing.T) {
	p := &MPEGPSParser{
		data:           make([]byte, 100),
		size:           100,
		lpcmSubStreams: map[byte]bool{},
		cellSegments:   []cellSegment{{vobuStart: 0, vobuEnd: 0}},
		vobus:          []vobuInfo{{videoRangeStart: 0, audioRangeStart: 0}},
	}
	adapter := newCellSegmentAdapter(p, 0)
	if adapter.Parser() != p {
		t.Error("Parser() should return the underlying parser")
	}
}

func TestDetectDVDCodecs_WithCellSegmentAdapter(t *testing.T) {
	data := make([]byte, 1000)
	data[100] = 0x80 // AC3 sub-stream header

	p := &MPEGPSParser{
		data: data,
		size: int64(len(data)),
		videoRanges: []PESPayloadRange{
			{FileOffset: 0, Size: 50, ESOffset: 0},
		},
		filteredVideoRanges: []PESPayloadRange{
			{FileOffset: 0, Size: 50, ESOffset: 0},
		},
		filterUserData:  true,
		audioSubStreams: []byte{0x80},
		filteredAudioBySubStream: map[byte][]PESPayloadRange{
			0x80: {{FileOffset: 104, Size: 96, ESOffset: 0}},
		},
		lpcmSubStreams: map[byte]bool{},
		cellSegments:   []cellSegment{{vobuStart: 0, vobuEnd: 1}},
		vobus:          []vobuInfo{{videoRangeStart: 0, audioRangeStart: 0}},
	}
	adapter := newCellSegmentAdapter(p, 0)

	index := &Index{
		SourceType: TypeDVD,
		ESReaders:  []ESReader{adapter},
	}

	codecs, err := DetectSourceCodecs(index)
	if err != nil {
		t.Fatalf("DetectSourceCodecs() error = %v", err)
	}
	if !containsCodec(codecs.VideoCodecs, CodecMPEG2Video) {
		t.Error("expected MPEG-2 video codec")
	}
	if !containsCodec(codecs.AudioCodecs, CodecAC3Audio) {
		t.Error("expected AC3 audio codec")
	}
}

func TestParseWithProgress_DetectsNAVPacks(t *testing.T) {
	const bufSize = 8192
	data := make([]byte, bufSize)
	pos := 0
	pos += buildTestNAVPack(data, pos, 100) // sector 100
	pos += buildTestVideoPES(data, pos, 100)
	pos += buildTestNAVPack(data, pos, 200) // sector 200
	pos += buildTestVideoPES(data, pos, 100)

	p := NewMPEGPSParser(data[:pos])
	if err := p.Parse(); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(p.vobus) != 2 {
		t.Errorf("detected %d VOBUs, want 2", len(p.vobus))
	}
	if p.vobus[0].sectorLBN != 100 {
		t.Errorf("vobus[0].sectorLBN = %d, want 100", p.vobus[0].sectorLBN)
	}
	if p.vobus[1].sectorLBN != 200 {
		t.Errorf("vobus[1].sectorLBN = %d, want 200", p.vobus[1].sectorLBN)
	}

	// No IFO in this data → 0 segments
	if p.CellSegmentCount() != 0 {
		t.Errorf("CellSegmentCount() = %d, want 0 (no IFO available)", p.CellSegmentCount())
	}
}

func TestIndexMPEGPSFile_NonInterleaved(t *testing.T) {
	const bufSize = 16384
	data := make([]byte, bufSize)
	pos := 0
	pos += buildTestNAVPack(data, pos, 0)
	pos += buildTestVideoPES(data, pos, 200)
	pos += buildTestNAVPack(data, pos, 0)
	pos += buildTestVideoPES(data, pos, 200)

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.vob")
	if err := os.WriteFile(tmpFile, data[:pos], 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	idx := &Indexer{
		windowSize: DefaultWindowSize,
		index:      NewIndex(tmpDir, TypeDVD, DefaultWindowSize),
	}
	idx.index.UsesESOffsets = true

	n, _, err := idx.indexMPEGPSFile(0, tmpFile, int64(pos), nil)
	if err != nil {
		t.Fatalf("indexMPEGPSFile() error = %v", err)
	}

	if n != 1 {
		t.Errorf("indexMPEGPSFile() returned %d entries, want 1", n)
	}
	if len(idx.index.ESReaders) != 1 {
		t.Fatalf("len(ESReaders) = %d, want 1", len(idx.index.ESReaders))
	}
	if _, ok := idx.index.ESReaders[0].(*MPEGPSParser); !ok {
		t.Errorf("ESReaders[0] is %T, want *MPEGPSParser", idx.index.ESReaders[0])
	}

	idx.index.Close()
}
