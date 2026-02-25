package source

import "testing"

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
