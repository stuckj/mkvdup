package source

import (
	"testing"
)

func TestISOAdapter_DataSlice_Contiguous(t *testing.T) {
	// Simulate an ISO with an M2TS region starting at baseOffset
	const baseOffset = 2048
	isoData := make([]byte, 4096)
	for i := range isoData {
		isoData[i] = byte(i % 256)
	}

	// Create a parser with the M2TS sub-slice
	m2tsData := isoData[baseOffset : baseOffset+1024]
	parser := NewMPEGTSParser(m2tsData)

	adapter := newISOAdapter(parser, isoData, baseOffset)

	// DataSize should return the parser's data size (M2TS region size)
	if ds := adapter.DataSize(); ds != 1024 {
		t.Errorf("DataSize() = %d, want 1024", ds)
	}

	// DataSlice should map parser-relative offsets to ISO-relative data
	// Parser offset 0, size 10 → ISO offset baseOffset, size 10
	s := adapter.DataSlice(0, 10)
	if len(s) != 10 {
		t.Fatalf("DataSlice(0, 10) len = %d, want 10", len(s))
	}
	for i := 0; i < 10; i++ {
		want := isoData[baseOffset+i]
		if s[i] != want {
			t.Errorf("DataSlice(0, 10)[%d] = 0x%02X, want 0x%02X", i, s[i], want)
		}
	}

	// DataSlice at a non-zero offset
	s = adapter.DataSlice(500, 20)
	if len(s) != 20 {
		t.Fatalf("DataSlice(500, 20) len = %d, want 20", len(s))
	}
	for i := 0; i < 20; i++ {
		want := isoData[baseOffset+500+i]
		if s[i] != want {
			t.Errorf("DataSlice(500, 20)[%d] = 0x%02X, want 0x%02X", i, s[i], want)
		}
	}

	// Data() should return the full ISO mmap for contiguous case
	if d := adapter.Data(); d == nil {
		t.Error("Data() returned nil for contiguous adapter")
	}
}

func TestISOAdapter_DataSlice_MultiExtent(t *testing.T) {
	isoData := make([]byte, 4096)
	// Extent 1 at ISO 100-299 filled with 0xAA
	for i := 100; i < 300; i++ {
		isoData[i] = 0xAA
	}
	// Extent 2 at ISO 2048-2247 filled with 0xBB
	for i := 2048; i < 2248; i++ {
		isoData[i] = 0xBB
	}

	extents := []isoPhysicalRange{
		{ISOOffset: 100, Length: 200},
		{ISOOffset: 2048, Length: 200},
	}

	mr := newMultiRegionData(extents, isoData)
	parser := NewMPEGTSParserMultiRegion(mr)
	adapter := newISOAdapterMultiExtent(parser, mr, extents)

	// DataSize should return the virtual total size (400)
	if ds := adapter.DataSize(); ds != 400 {
		t.Errorf("DataSize() = %d, want 400", ds)
	}

	// DataSlice within first extent
	s := adapter.DataSlice(0, 10)
	if len(s) != 10 {
		t.Fatalf("DataSlice(0, 10) len = %d, want 10", len(s))
	}
	for i, b := range s {
		if b != 0xAA {
			t.Errorf("DataSlice(0, 10)[%d] = 0x%02X, want 0xAA", i, b)
		}
	}

	// DataSlice within second extent (logical offset 200+)
	s = adapter.DataSlice(200, 10)
	if len(s) != 10 {
		t.Fatalf("DataSlice(200, 10) len = %d, want 10", len(s))
	}
	for i, b := range s {
		if b != 0xBB {
			t.Errorf("DataSlice(200, 10)[%d] = 0x%02X, want 0xBB", i, b)
		}
	}

	// DataSlice spanning extent boundary
	s = adapter.DataSlice(195, 10)
	if len(s) != 10 {
		t.Fatalf("DataSlice(195, 10) len = %d, want 10", len(s))
	}
	for i := 0; i < 5; i++ {
		if s[i] != 0xAA {
			t.Errorf("cross-extent DataSlice[%d] = 0x%02X, want 0xAA", i, s[i])
		}
	}
	for i := 5; i < 10; i++ {
		if s[i] != 0xBB {
			t.Errorf("cross-extent DataSlice[%d] = 0x%02X, want 0xBB", i, s[i])
		}
	}

	// Data() should return nil for multi-extent adapter
	if d := adapter.Data(); d != nil {
		t.Error("Data() should return nil for multi-extent adapter")
	}
}

func TestISOAdapter_ImplementsESDataProvider(t *testing.T) {
	// Verify both adapter construction paths satisfy esDataProvider at compile time.
	// The contiguous adapter:
	isoData := make([]byte, 4096)
	parser := NewMPEGTSParser(isoData[:1024])
	var _ esDataProvider = newISOAdapter(parser, isoData, 0)

	// The multi-extent adapter:
	extents := []isoPhysicalRange{{ISOOffset: 0, Length: 1024}}
	mr := newMultiRegionData(extents, isoData)
	parserMR := NewMPEGTSParserMultiRegion(mr)
	var _ esDataProvider = newISOAdapterMultiExtent(parserMR, mr, extents)
}
