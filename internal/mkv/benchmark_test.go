package mkv

import (
	"bytes"
	"math/rand/v2"
	"os"
	"testing"
)

// encodeVINT encodes a value as an EBML VINT (variable-length integer).
// Returns the encoded bytes.
func encodeVINT(value uint64) []byte {
	switch {
	case value <= 0x7E: // 1 byte (0x7F reserved for unknown size)
		return []byte{byte(0x80 | value)}
	case value <= 0x3FFE: // 2 bytes
		return []byte{byte(0x40 | (value >> 8)), byte(value & 0xFF)}
	case value <= 0x1FFFFE: // 3 bytes
		return []byte{byte(0x20 | (value >> 16)), byte((value >> 8) & 0xFF), byte(value & 0xFF)}
	case value <= 0x0FFFFFFE: // 4 bytes
		return []byte{
			byte(0x10 | (value >> 24)),
			byte((value >> 16) & 0xFF),
			byte((value >> 8) & 0xFF),
			byte(value & 0xFF),
		}
	default: // 8 bytes for larger values
		return []byte{
			0x01,
			byte((value >> 48) & 0xFF),
			byte((value >> 40) & 0xFF),
			byte((value >> 32) & 0xFF),
			byte((value >> 24) & 0xFF),
			byte((value >> 16) & 0xFF),
			byte((value >> 8) & 0xFF),
			byte(value & 0xFF),
		}
	}
}

// encodeElementID encodes an EBML element ID as bytes.
func encodeElementID(id uint64) []byte {
	switch {
	case id <= 0xFF:
		return []byte{byte(id)}
	case id <= 0xFFFF:
		return []byte{byte(id >> 8), byte(id & 0xFF)}
	case id <= 0xFFFFFF:
		return []byte{byte(id >> 16), byte((id >> 8) & 0xFF), byte(id & 0xFF)}
	default:
		return []byte{byte(id >> 24), byte((id >> 16) & 0xFF), byte((id >> 8) & 0xFF), byte(id & 0xFF)}
	}
}

// createSyntheticMKV creates a synthetic MKV file with valid EBML structure.
// Parameters:
//   - numClusters: number of clusters to create
//   - blocksPerCluster: number of SimpleBlocks per cluster
//   - blockDataSize: size of data in each SimpleBlock
//
// Returns the MKV data and the expected number of packets.
func createSyntheticMKV(numClusters, blocksPerCluster, blockDataSize int) ([]byte, int) {
	var buf bytes.Buffer
	rng := rand.New(rand.NewPCG(42, 0))

	// EBML Header
	ebmlHeaderData := []byte{
		0x42, 0x86, 0x81, 0x01, // EBMLVersion = 1
		0x42, 0xF7, 0x81, 0x01, // EBMLReadVersion = 1
		0x42, 0xF2, 0x81, 0x04, // EBMLMaxIDLength = 4
		0x42, 0xF3, 0x81, 0x08, // EBMLMaxSizeLength = 8
		0x42, 0x82, 0x88, 'm', 'a', 't', 'r', 'o', 's', 'k', 'a', // DocType = "matroska"
		0x42, 0x87, 0x81, 0x04, // DocTypeVersion = 4
		0x42, 0x85, 0x81, 0x02, // DocTypeReadVersion = 2
	}
	buf.Write(encodeElementID(IDEBMLHeader))
	buf.Write(encodeVINT(uint64(len(ebmlHeaderData))))
	buf.Write(ebmlHeaderData)

	// Build segment content first to know its size
	var segmentBuf bytes.Buffer

	// Tracks element with one video track
	trackEntryData := []byte{
		0xD7, 0x81, 0x01, // TrackNumber = 1
		0x73, 0xC5, 0x88, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, // TrackUID
		0x83, 0x81, 0x01, // TrackType = 1 (video)
	}
	codecIDBytes := []byte("V_MPEG4/ISO/AVC")
	trackEntryData = append(trackEntryData, 0x86)                              // CodecID element ID
	trackEntryData = append(trackEntryData, encodeVINT(uint64(len(codecIDBytes)))...) // size
	trackEntryData = append(trackEntryData, codecIDBytes...)

	trackEntry := append(encodeElementID(IDTrackEntry), encodeVINT(uint64(len(trackEntryData)))...)
	trackEntry = append(trackEntry, trackEntryData...)

	tracksData := trackEntry
	segmentBuf.Write(encodeElementID(IDTracks))
	segmentBuf.Write(encodeVINT(uint64(len(tracksData))))
	segmentBuf.Write(tracksData)

	// Create clusters
	for c := range numClusters {
		var clusterBuf bytes.Buffer

		// Cluster timestamp (element ID 0xE7)
		clusterBuf.WriteByte(0xE7)
		clusterBuf.Write(encodeVINT(2)) // size = 2 bytes
		clusterBuf.WriteByte(byte((c * 1000) >> 8))
		clusterBuf.WriteByte(byte((c * 1000) & 0xFF))

		// Create SimpleBlocks
		for b := range blocksPerCluster {
			// SimpleBlock data: VINT track number + 2-byte timestamp + 1-byte flags + packet data
			var blockBuf bytes.Buffer
			blockBuf.WriteByte(0x81) // Track 1 (1-byte VINT)
			// Relative timestamp (within cluster)
			relTS := b * 10
			blockBuf.WriteByte(byte(relTS >> 8))
			blockBuf.WriteByte(byte(relTS & 0xFF))
			// Flags: keyframe if first block
			if b == 0 {
				blockBuf.WriteByte(0x80) // Keyframe
			} else {
				blockBuf.WriteByte(0x00) // Not keyframe
			}
			// Random packet data
			packetData := make([]byte, blockDataSize)
			for i := range packetData {
				packetData[i] = byte(rng.IntN(256))
			}
			blockBuf.Write(packetData)

			// Write SimpleBlock element
			clusterBuf.WriteByte(0xA3) // SimpleBlock ID
			clusterBuf.Write(encodeVINT(uint64(blockBuf.Len())))
			clusterBuf.Write(blockBuf.Bytes())
		}

		// Write cluster to segment
		segmentBuf.Write(encodeElementID(IDCluster))
		segmentBuf.Write(encodeVINT(uint64(clusterBuf.Len())))
		segmentBuf.Write(clusterBuf.Bytes())
	}

	// Write Segment with known size
	buf.Write(encodeElementID(IDSegment))
	buf.Write(encodeVINT(uint64(segmentBuf.Len())))
	buf.Write(segmentBuf.Bytes())

	expectedPackets := numClusters * blocksPerCluster
	return buf.Bytes(), expectedPackets
}

// BenchmarkReadVINT benchmarks VINT decoding, which is a hot path in MKV parsing.
func BenchmarkReadVINT(b *testing.B) {
	// Create test data with various VINT lengths
	testCases := []struct {
		name string
		data []byte
	}{
		{"1-byte", []byte{0x81}},                             // 1
		{"2-byte", []byte{0x40, 0x80}},                       // 128
		{"4-byte", []byte{0x10, 0x00, 0x10, 0x00}},           // 4096
		{"8-byte", []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x10, 0x00}}, // Large value
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				r := bytes.NewReader(tc.data)
				_, _, _ = readVINT(r, false)
			}
		})
	}
}

// BenchmarkReadElementHeader benchmarks EBML element header reading.
func BenchmarkReadElementHeader(b *testing.B) {
	// Create various element headers
	testCases := []struct {
		name string
		data []byte
	}{
		{"SimpleBlock", []byte{0xA3, 0x82, 0x10, 0x00}},                         // ID 0xA3, size 4096
		{"Cluster", []byte{0x1F, 0x43, 0xB6, 0x75, 0x41, 0x00, 0x00}},            // 4-byte ID, 3-byte size
		{"Segment", []byte{0x18, 0x53, 0x80, 0x67, 0x01, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}}, // Unknown size
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				r := bytes.NewReader(tc.data)
				_, _ = ReadElementHeader(r, 0)
			}
		})
	}
}

// BenchmarkParseSimpleBlockHeader benchmarks SimpleBlock header parsing.
func BenchmarkParseSimpleBlockHeader(b *testing.B) {
	// SimpleBlock: track 1, timestamp 0, keyframe, with some data
	data := []byte{0x81, 0x00, 0x00, 0x80, 0x00, 0x00, 0x00, 0x00}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = ParseSimpleBlockHeader(data)
	}
}

// BenchmarkParser_Parse benchmarks full MKV parsing with synthetic data.
func BenchmarkParser_Parse(b *testing.B) {
	// Create a ~10MB synthetic MKV file
	// 100 clusters × 50 blocks × 2KB = ~10MB
	mkvData, expectedPackets := createSyntheticMKV(100, 50, 2048)

	// Write to temp file
	tmpDir := b.TempDir()
	mkvPath := tmpDir + "/benchmark.mkv"
	if err := os.WriteFile(mkvPath, mkvData, 0644); err != nil {
		b.Fatalf("Failed to write test MKV: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	b.SetBytes(int64(len(mkvData)))

	for b.Loop() {
		parser, err := NewParser(mkvPath)
		if err != nil {
			b.Fatalf("NewParser failed: %v", err)
		}

		if err := parser.Parse(nil); err != nil {
			b.Fatalf("Parse failed: %v", err)
		}

		if parser.PacketCount() != expectedPackets {
			b.Fatalf("Expected %d packets, got %d", expectedPackets, parser.PacketCount())
		}

		parser.Close()
	}
}

// BenchmarkParser_Parse_Small benchmarks MKV parsing with smaller data for faster CI runs.
func BenchmarkParser_Parse_Small(b *testing.B) {
	// ~1MB: 50 clusters × 10 blocks × 2KB = ~1MB
	mkvData, expectedPackets := createSyntheticMKV(50, 10, 2048)

	tmpDir := b.TempDir()
	mkvPath := tmpDir + "/benchmark_small.mkv"
	if err := os.WriteFile(mkvPath, mkvData, 0644); err != nil {
		b.Fatalf("Failed to write test MKV: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	b.SetBytes(int64(len(mkvData)))

	for b.Loop() {
		parser, err := NewParser(mkvPath)
		if err != nil {
			b.Fatalf("NewParser failed: %v", err)
		}

		if err := parser.Parse(nil); err != nil {
			b.Fatalf("Parse failed: %v", err)
		}

		if parser.PacketCount() != expectedPackets {
			b.Fatalf("Expected %d packets, got %d", expectedPackets, parser.PacketCount())
		}

		parser.Close()
	}
}
