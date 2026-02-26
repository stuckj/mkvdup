package source

import (
	"bytes"
	"testing"
)

func TestMultiRegionData_BasicAccess(t *testing.T) {
	// Simulate 3 extents with gaps between them in a larger ISO
	isoData := make([]byte, 1000)
	// Extent 1: bytes 100-199 contain 0xAA
	for i := 100; i < 200; i++ {
		isoData[i] = 0xAA
	}
	// Extent 2: bytes 400-499 contain 0xBB
	for i := 400; i < 500; i++ {
		isoData[i] = 0xBB
	}
	// Extent 3: bytes 700-799 contain 0xCC
	for i := 700; i < 800; i++ {
		isoData[i] = 0xCC
	}

	extents := []isoPhysicalRange{
		{ISOOffset: 100, Length: 100},
		{ISOOffset: 400, Length: 100},
		{ISOOffset: 700, Length: 100},
	}

	mr := newMultiRegionData(extents, isoData)

	// Test total size
	if mr.Len() != 300 {
		t.Errorf("Len() = %d, want 300", mr.Len())
	}

	// Test ByteAt within each region
	if b := mr.ByteAt(0); b != 0xAA {
		t.Errorf("ByteAt(0) = 0x%02X, want 0xAA", b)
	}
	if b := mr.ByteAt(99); b != 0xAA {
		t.Errorf("ByteAt(99) = 0x%02X, want 0xAA", b)
	}
	if b := mr.ByteAt(100); b != 0xBB {
		t.Errorf("ByteAt(100) = 0x%02X, want 0xBB", b)
	}
	if b := mr.ByteAt(200); b != 0xCC {
		t.Errorf("ByteAt(200) = 0x%02X, want 0xCC", b)
	}

	// Test Slice within one region (zero-copy)
	s := mr.Slice(50, 60)
	if len(s) != 10 {
		t.Fatalf("Slice(50,60) len = %d, want 10", len(s))
	}
	for i, b := range s {
		if b != 0xAA {
			t.Errorf("Slice(50,60)[%d] = 0x%02X, want 0xAA", i, b)
		}
	}

	// Test Slice spanning region boundary
	s = mr.Slice(95, 105)
	if len(s) != 10 {
		t.Fatalf("Slice(95,105) len = %d, want 10", len(s))
	}
	// First 5 bytes from region 1 (0xAA), next 5 from region 2 (0xBB)
	for i := 0; i < 5; i++ {
		if s[i] != 0xAA {
			t.Errorf("cross-boundary slice[%d] = 0x%02X, want 0xAA", i, s[i])
		}
	}
	for i := 5; i < 10; i++ {
		if s[i] != 0xBB {
			t.Errorf("cross-boundary slice[%d] = 0x%02X, want 0xBB", i, s[i])
		}
	}
}

func TestMultiRegionData_ParserIntegration(t *testing.T) {
	// Build a small M2TS-like data split across 2 extents
	// Each M2TS packet is 192 bytes
	const pktSize = 192
	const nPackets = 10 // 5 packets per extent

	// Create M2TS data with identifiable packets
	m2tsData := make([]byte, pktSize*nPackets)
	for i := 0; i < nPackets; i++ {
		pkt := m2tsData[i*pktSize : (i+1)*pktSize]
		// 4-byte timestamp
		pkt[0] = 0x00
		pkt[1] = 0x00
		pkt[2] = 0x00
		pkt[3] = byte(i)
		// TS sync byte
		pkt[4] = 0x47
		// Fill rest with pattern
		for j := 5; j < pktSize; j++ {
			pkt[j] = byte(i)
		}
	}

	// Place the two halves at different locations in a fake ISO
	isoData := make([]byte, 4096)
	copy(isoData[100:100+5*pktSize], m2tsData[:5*pktSize])
	copy(isoData[2048:2048+5*pktSize], m2tsData[5*pktSize:])

	extents := []isoPhysicalRange{
		{ISOOffset: 100, Length: int64(5 * pktSize)},
		{ISOOffset: 2048, Length: int64(5 * pktSize)},
	}

	mr := newMultiRegionData(extents, isoData)

	// Verify the multi-region view matches the original contiguous data
	for i := int64(0); i < int64(len(m2tsData)); i++ {
		got := mr.ByteAt(i)
		want := m2tsData[i]
		if got != want {
			t.Errorf("ByteAt(%d) = 0x%02X, want 0x%02X", i, got, want)
			break
		}
	}

	// Verify Slice at boundary returns correct data
	boundaryOff := int64(5*pktSize - 10)
	s := mr.Slice(boundaryOff, boundaryOff+20)
	expected := m2tsData[boundaryOff : boundaryOff+20]
	if !bytes.Equal(s, expected) {
		t.Errorf("Slice across boundary mismatch at offset %d", boundaryOff)
	}
}

func TestMultiRegionData_NegativeOffsets(t *testing.T) {
	isoData := make([]byte, 200)
	for i := range isoData {
		isoData[i] = 0xAA
	}
	mr := newMultiRegionData([]isoPhysicalRange{
		{ISOOffset: 0, Length: 200},
	}, isoData)

	// ByteAt with negative offset should return 0, not panic
	if b := mr.ByteAt(-1); b != 0 {
		t.Errorf("ByteAt(-1) = 0x%02X, want 0", b)
	}
	if b := mr.ByteAt(-100); b != 0 {
		t.Errorf("ByteAt(-100) = 0x%02X, want 0", b)
	}

	// ByteAt beyond totalSize should return 0
	if b := mr.ByteAt(200); b != 0 {
		t.Errorf("ByteAt(200) = 0x%02X, want 0", b)
	}

	// Slice with negative off should return nil, not panic
	if s := mr.Slice(-1, 10); s != nil {
		t.Errorf("Slice(-1, 10) = %v, want nil", s)
	}

	// Slice with negative end should return nil
	if s := mr.Slice(0, -1); s != nil {
		t.Errorf("Slice(0, -1) = %v, want nil", s)
	}

	// Slice with both negative should return nil
	if s := mr.Slice(-10, -5); s != nil {
		t.Errorf("Slice(-10, -5) = %v, want nil", s)
	}
}
