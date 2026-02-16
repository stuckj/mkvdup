package main

import (
	"testing"

	"github.com/stuckj/mkvdup/internal/mkv"
)

func TestProbeResult(t *testing.T) {
	// Test that ProbeResult struct has expected fields and behavior
	result := ProbeResult{
		SourcePath:   "/path/to/source",
		MatchCount:   85,
		TotalSamples: 100,
		MatchPercent: 85.0,
	}

	if result.SourcePath != "/path/to/source" {
		t.Errorf("SourcePath = %q, want %q", result.SourcePath, "/path/to/source")
	}
	if result.MatchCount != 85 {
		t.Errorf("MatchCount = %d, want 85", result.MatchCount)
	}
	if result.TotalSamples != 100 {
		t.Errorf("TotalSamples = %d, want 100", result.TotalSamples)
	}
	if result.MatchPercent != 85.0 {
		t.Errorf("MatchPercent = %f, want 85.0", result.MatchPercent)
	}
}

func TestProbeResult_ZeroValues(t *testing.T) {
	// Test zero value behavior
	var result ProbeResult

	if result.SourcePath != "" {
		t.Errorf("Zero SourcePath = %q, want empty", result.SourcePath)
	}
	if result.MatchCount != 0 {
		t.Errorf("Zero MatchCount = %d, want 0", result.MatchCount)
	}
	if result.TotalSamples != 0 {
		t.Errorf("Zero TotalSamples = %d, want 0", result.TotalSamples)
	}
	if result.MatchPercent != 0.0 {
		t.Errorf("Zero MatchPercent = %f, want 0.0", result.MatchPercent)
	}
}

func TestDeltadiagFindPacket(t *testing.T) {
	packets := []mkv.Packet{
		{Offset: 0, Size: 100},
		{Offset: 100, Size: 200},
		{Offset: 300, Size: 50},
		{Offset: 500, Size: 150},
	}

	tests := []struct {
		name   string
		offset int64
		want   int
	}{
		{"start of first packet", 0, 0},
		{"middle of first packet", 50, 0},
		{"last byte of first packet", 99, 0},
		{"start of second packet", 100, 1},
		{"middle of second packet", 200, 1},
		{"start of third packet", 300, 2},
		{"start of fourth packet", 500, 3},
		{"last byte of fourth packet", 649, 3},
		{"gap between packets", 350, -1},
		{"after all packets", 700, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deltadiagFindPacket(packets, tt.offset)
			if got != tt.want {
				t.Errorf("deltadiagFindPacket(offset=%d) = %d, want %d", tt.offset, got, tt.want)
			}
		})
	}

	// Edge case: empty packets
	if got := deltadiagFindPacket(nil, 0); got != -1 {
		t.Errorf("deltadiagFindPacket(nil, 0) = %d, want -1", got)
	}

	// Edge case: single packet
	single := []mkv.Packet{{Offset: 10, Size: 5}}
	if got := deltadiagFindPacket(single, 12); got != 0 {
		t.Errorf("deltadiagFindPacket(single, 12) = %d, want 0", got)
	}
	if got := deltadiagFindPacket(single, 9); got != -1 {
		t.Errorf("deltadiagFindPacket(single, 9) = %d, want -1", got)
	}
	if got := deltadiagFindPacket(single, 15); got != -1 {
		t.Errorf("deltadiagFindPacket(single, 15) = %d, want -1", got)
	}
}

func TestDeltadiagClassifyAVCC(t *testing.T) {
	// Build a synthetic AVCC packet with NAL length size = 4
	// NAL 1: SPS (type 7), 10 bytes
	// NAL 2: non-IDR slice (type 1), 5000 bytes (large)
	// NAL 3: SEI (type 6), 20 bytes
	nalLenSize := 4

	var pktData []byte

	// NAL 1: SPS (type 7), body = 10 bytes
	nalLen1 := uint32(10)
	pktData = append(pktData, byte(nalLen1>>24), byte(nalLen1>>16), byte(nalLen1>>8), byte(nalLen1))
	pktData = append(pktData, 0x67) // NAL type 7 (SPS)
	for i := 1; i < 10; i++ {
		pktData = append(pktData, 0xAA)
	}

	// NAL 2: non-IDR slice (type 1), body = 5000 bytes
	nalLen2 := uint32(5000)
	pktData = append(pktData, byte(nalLen2>>24), byte(nalLen2>>16), byte(nalLen2>>8), byte(nalLen2))
	pktData = append(pktData, 0x41) // NAL type 1 (non-IDR slice)
	for i := 1; i < 5000; i++ {
		pktData = append(pktData, 0xBB)
	}

	// NAL 3: SEI (type 6), body = 20 bytes
	nalLen3 := uint32(20)
	pktData = append(pktData, byte(nalLen3>>24), byte(nalLen3>>16), byte(nalLen3>>8), byte(nalLen3))
	pktData = append(pktData, 0x06) // NAL type 6 (SEI)
	for i := 1; i < 20; i++ {
		pktData = append(pktData, 0xCC)
	}

	pkt := mkv.Packet{
		Offset: 0,
		Size:   int64(len(pktData)),
	}

	// Delta covers entire packet
	var byNAL [32]deltaClass
	var sliceSmall, sliceLarge deltaClass

	deltadiagClassifyAVCC(pktData, pkt, nalLenSize, 0, int64(len(pktData)),
		&byNAL, &sliceSmall, &sliceLarge)

	// NAL type 7 (SPS): 4 (length prefix) + 10 (body) = 14 bytes overlap
	if byNAL[7].count != 1 {
		t.Errorf("SPS count = %d, want 1", byNAL[7].count)
	}
	if byNAL[7].bytes != 14 {
		t.Errorf("SPS bytes = %d, want 14", byNAL[7].bytes)
	}

	// NAL type 1 (non-IDR slice): 4 + 5000 = 5004 bytes
	if byNAL[1].count != 1 {
		t.Errorf("non-IDR slice count = %d, want 1", byNAL[1].count)
	}
	if byNAL[1].bytes != 5004 {
		t.Errorf("non-IDR slice bytes = %d, want 5004", byNAL[1].bytes)
	}

	// NAL type 6 (SEI): 4 + 20 = 24 bytes
	if byNAL[6].count != 1 {
		t.Errorf("SEI count = %d, want 1", byNAL[6].count)
	}
	if byNAL[6].bytes != 24 {
		t.Errorf("SEI bytes = %d, want 24", byNAL[6].bytes)
	}

	// Slice classification: 5000 bytes >= 4096, so it's large
	if sliceLarge.count != 1 {
		t.Errorf("sliceLarge count = %d, want 1", sliceLarge.count)
	}
	if sliceSmall.count != 0 {
		t.Errorf("sliceSmall count = %d, want 0", sliceSmall.count)
	}
}

func TestDeltadiagClassifyAVCC_PartialOverlap(t *testing.T) {
	// Test that only the overlapping portion of a NAL is counted
	nalLenSize := 4

	var pktData []byte

	// Single NAL: type 1, 100 bytes body
	nalLen := uint32(100)
	pktData = append(pktData, byte(nalLen>>24), byte(nalLen>>16), byte(nalLen>>8), byte(nalLen))
	pktData = append(pktData, 0x41) // NAL type 1
	for i := 1; i < 100; i++ {
		pktData = append(pktData, 0xDD)
	}

	pkt := mkv.Packet{Offset: 0, Size: int64(len(pktData))}

	var byNAL [32]deltaClass
	var sliceSmall, sliceLarge deltaClass

	// Delta only covers bytes 10-50 (40 bytes within the NAL)
	deltadiagClassifyAVCC(pktData, pkt, nalLenSize, 10, 50,
		&byNAL, &sliceSmall, &sliceLarge)

	if byNAL[1].bytes != 40 {
		t.Errorf("partial overlap bytes = %d, want 40", byNAL[1].bytes)
	}
	if byNAL[1].count != 1 {
		t.Errorf("partial overlap count = %d, want 1", byNAL[1].count)
	}
}
