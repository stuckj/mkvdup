package source

import "testing"

func TestReadByteAt_MultiRegion(t *testing.T) {
	// Create multi-region data from two non-contiguous extents
	isoData := make([]byte, 500)
	for i := 100; i < 200; i++ {
		isoData[i] = byte(i)
	}
	for i := 300; i < 400; i++ {
		isoData[i] = byte(i)
	}
	mr := newMultiRegionData([]isoPhysicalRange{
		{ISOOffset: 100, Length: 100},
		{ISOOffset: 300, Length: 100},
	}, isoData)

	// readByteAt with mr should read from the virtual contiguous view
	if b := readByteAt(nil, mr, 0); b != byte(100) {
		t.Errorf("readByteAt(0) = 0x%02X, want 0x%02X", b, byte(100))
	}
	// Logical offset 100 maps to second region (isoData[300])
	if b := readByteAt(nil, mr, 100); b != byte(44) {
		t.Errorf("readByteAt(100) = 0x%02X, want 0x%02X", b, byte(44))
	}
}

func TestReadSliceAt_MultiRegion(t *testing.T) {
	isoData := make([]byte, 500)
	for i := 100; i < 200; i++ {
		isoData[i] = 0xAA
	}
	for i := 300; i < 400; i++ {
		isoData[i] = 0xBB
	}
	mr := newMultiRegionData([]isoPhysicalRange{
		{ISOOffset: 100, Length: 100},
		{ISOOffset: 300, Length: 100},
	}, isoData)

	// Within one region
	s := readSliceAt(nil, mr, 0, 5)
	for i, b := range s {
		if b != 0xAA {
			t.Errorf("readSliceAt within region [%d] = 0x%02X, want 0xAA", i, b)
		}
	}

	// Spanning region boundary (logical 98-102)
	s = readSliceAt(nil, mr, 98, 104)
	if s[0] != 0xAA || s[1] != 0xAA {
		t.Errorf("cross-boundary: first 2 bytes should be 0xAA")
	}
	if s[2] != 0xBB || s[3] != 0xBB {
		t.Errorf("cross-boundary: last 2 bytes should be 0xBB, got 0x%02X 0x%02X", s[2], s[3])
	}
}

func TestReadByteWithHint_MultiRegion(t *testing.T) {
	// Two PES ranges that happen to live in different ISO extents
	isoData := make([]byte, 500)
	// Extent 1 at ISO 100-199: fill with identifiable bytes
	for i := 100; i < 200; i++ {
		isoData[i] = byte(i)
	}
	// Extent 2 at ISO 300-399: fill with identifiable bytes
	for i := 300; i < 400; i++ {
		isoData[i] = byte(i)
	}
	mr := newMultiRegionData([]isoPhysicalRange{
		{ISOOffset: 100, Length: 100},
		{ISOOffset: 300, Length: 100},
	}, isoData)

	// PES ranges referencing the virtual contiguous view:
	// Range 0: logical offset 10, size 20, ES 0-19
	// Range 1: logical offset 110, size 20, ES 20-39 (in second extent)
	ranges := []PESPayloadRange{
		{FileOffset: 10, Size: 20, ESOffset: 0},
		{FileOffset: 110, Size: 20, ESOffset: 20},
	}

	// Read from range 0 (logical offset 10 → ISO 110)
	b, hint, ok := readByteWithHint(nil, mr, mr.Len(), ranges, 0, 0)
	if !ok {
		t.Fatal("readByteWithHint(es=0) failed")
	}
	if b != byte(110) || hint != 0 {
		t.Errorf("es=0: got byte=0x%02X hint=%d, want 0x%02X 0", b, hint, byte(110))
	}

	// Read from range 1 (logical offset 110 → ISO 310, since second extent starts at logical 100)
	b, hint, ok = readByteWithHint(nil, mr, mr.Len(), ranges, 20, 1)
	if !ok {
		t.Fatal("readByteWithHint(es=20) failed")
	}
	// logical 110 = second region offset 10 → ISO 310
	if b != byte(310%256) || hint != 1 {
		t.Errorf("es=20: got byte=0x%02X hint=%d, want 0x%02X 1", b, hint, byte(310%256))
	}

	// Cross forward from range 0 hint into range 1
	b, hint, ok = readByteWithHint(nil, mr, mr.Len(), ranges, 21, 0)
	if !ok {
		t.Fatal("readByteWithHint(es=21, hint=0) failed")
	}
	if hint != 1 {
		t.Errorf("forward cross: got hint=%d, want 1", hint)
	}
	// ES 21 → range 1 offset 1 → logical 111 → second region offset 11 → ISO 311
	if b != byte(311%256) {
		t.Errorf("forward cross: got byte=0x%02X, want 0x%02X", b, byte(311%256))
	}
}

func TestReadFromRanges_MultiRegion(t *testing.T) {
	isoData := make([]byte, 500)
	for i := 100; i < 200; i++ {
		isoData[i] = 0xAA
	}
	for i := 300; i < 400; i++ {
		isoData[i] = 0xBB
	}
	mr := newMultiRegionData([]isoPhysicalRange{
		{ISOOffset: 100, Length: 100},
		{ISOOffset: 300, Length: 100},
	}, isoData)

	// Two PES ranges: one in each extent
	ranges := []PESPayloadRange{
		{FileOffset: 80, Size: 20, ESOffset: 0},   // logical 80-99 (end of first extent)
		{FileOffset: 100, Size: 20, ESOffset: 20}, // logical 100-119 (start of second extent)
	}

	// Read within first range (zero-copy path)
	data, err := readFromRanges(nil, mr, mr.Len(), ranges, 0, 5)
	if err != nil {
		t.Fatalf("readFromRanges single range: %v", err)
	}
	for i, b := range data {
		if b != 0xAA {
			t.Errorf("single-range byte %d = 0x%02X, want 0xAA", i, b)
		}
	}

	// Read spanning both ranges (copy path)
	data, err = readFromRanges(nil, mr, mr.Len(), ranges, 15, 10)
	if err != nil {
		t.Fatalf("readFromRanges cross-range: %v", err)
	}
	// First 5 bytes from range 0 (0xAA), last 5 from range 1 (0xBB)
	for i := 0; i < 5; i++ {
		if data[i] != 0xAA {
			t.Errorf("cross-range byte %d = 0x%02X, want 0xAA", i, data[i])
		}
	}
	for i := 5; i < 10; i++ {
		if data[i] != 0xBB {
			t.Errorf("cross-range byte %d = 0x%02X, want 0xBB", i, data[i])
		}
	}
}

func TestReadByteWithHint_BackwardBoundaryCrossing(t *testing.T) {
	// Create two adjacent PES payload ranges
	data := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, // padding (offsets 0-4)
		0xAA, 0xBB, 0xCC, // range 0 payload at file offsets 5-7
		0x00, 0x00, // padding (offsets 8-9)
		0xDD, 0xEE, 0xFF, // range 1 payload at file offsets 10-12
	}
	dataSize := int64(len(data))

	ranges := []PESPayloadRange{
		{FileOffset: 5, Size: 3, ESOffset: 0},  // ES 0-2 → file 5-7
		{FileOffset: 10, Size: 3, ESOffset: 3}, // ES 3-5 → file 10-12
	}

	// Read forward through range 1 (ES offset 4, in range 1)
	b, hint, ok := readByteWithHint(data, nil, dataSize, ranges, 4, 1)
	if !ok || b != 0xEE || hint != 1 {
		t.Errorf("Forward read: got byte=0x%02X hint=%d ok=%v, want 0xEE 1 true", b, hint, ok)
	}

	// Now read backward into range 0 (ES offset 2, should use hint-1)
	b, hint, ok = readByteWithHint(data, nil, dataSize, ranges, 2, 1)
	if !ok || b != 0xCC || hint != 0 {
		t.Errorf("Backward read: got byte=0x%02X hint=%d ok=%v, want 0xCC 0 true", b, hint, ok)
	}

	// Continue backward within range 0
	b, hint, ok = readByteWithHint(data, nil, dataSize, ranges, 1, hint)
	if !ok || b != 0xBB || hint != 0 {
		t.Errorf("Continue backward: got byte=0x%02X hint=%d ok=%v, want 0xBB 0 true", b, hint, ok)
	}
}

func TestReadByteWithHint_ForwardBoundaryCrossing(t *testing.T) {
	data := []byte{
		0xAA, 0xBB, 0xCC, // range 0 at file offsets 0-2
		0x00, 0x00, // padding
		0xDD, 0xEE, 0xFF, // range 1 at file offsets 5-7
	}
	dataSize := int64(len(data))

	ranges := []PESPayloadRange{
		{FileOffset: 0, Size: 3, ESOffset: 0}, // ES 0-2 → file 0-2
		{FileOffset: 5, Size: 3, ESOffset: 3}, // ES 3-5 → file 5-7
	}

	// Read at end of range 0
	b, hint, ok := readByteWithHint(data, nil, dataSize, ranges, 2, 0)
	if !ok || b != 0xCC || hint != 0 {
		t.Errorf("End of range 0: got byte=0x%02X hint=%d ok=%v, want 0xCC 0 true", b, hint, ok)
	}

	// Read forward into range 1 (should use hint+1)
	b, hint, ok = readByteWithHint(data, nil, dataSize, ranges, 3, 0)
	if !ok || b != 0xDD || hint != 1 {
		t.Errorf("Forward cross: got byte=0x%02X hint=%d ok=%v, want 0xDD 1 true", b, hint, ok)
	}
}
