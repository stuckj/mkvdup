package dedup

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stuckj/mkvdup/internal/matcher"
	"github.com/stuckj/mkvdup/internal/source"
)

// TestLPCM_RoundTrip_ByteSwap verifies end-to-end LPCM byte-swap reconstruction.
// It creates a synthetic source file with big-endian 16-bit PCM samples, writes
// a dedup file with LPCM entries and range maps, then verifies ReadAt returns
// correctly byte-swapped (little-endian) data at various offsets and sizes.
func TestLPCM_RoundTrip_ByteSwap(t *testing.T) {
	dir := t.TempDir()

	// Create synthetic big-endian PCM source data.
	// Pattern: sequential 16-bit samples in big-endian order [HI, LO].
	// Sample 0: [0x00, 0x01], Sample 1: [0x00, 0x02], etc.
	const numSamples = 500
	const sourceDataSize = numSamples * 2
	srcData := make([]byte, sourceDataSize)
	for i := 0; i < numSamples; i++ {
		srcData[2*i] = byte(i >> 8)     // HI
		srcData[2*i+1] = byte(i & 0xFF) // LO
	}

	// Expected MKV data: little-endian [LO, HI] pairs.
	expectedMKV := make([]byte, sourceDataSize)
	for i := 0; i < numSamples; i++ {
		expectedMKV[2*i] = byte(i & 0xFF) // LO
		expectedMKV[2*i+1] = byte(i >> 8) // HI
	}

	// Write the source data to a file (simulating a DVD ISO with raw PCM data).
	// Add a 100-byte header before the PCM data to simulate file structure.
	const fileHeaderSize = 100
	sourceFileName := "source.bin"
	sourceFilePath := filepath.Join(dir, sourceFileName)
	sourceFileData := make([]byte, fileHeaderSize+sourceDataSize)
	copy(sourceFileData[fileHeaderSize:], srcData)
	if err := os.WriteFile(sourceFilePath, sourceFileData, 0644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	// Build range map: one contiguous range mapping ES offset 0 to raw file offset fileHeaderSize.
	rangeMaps := []RangeMapData{
		{
			FileIndex: 0,
			AudioStreams: []AudioRangeData{
				{
					SubStreamID: 0xA0,
					Ranges: []source.PESPayloadRange{
						{
							FileOffset: int64(fileHeaderSize),
							Size:       sourceDataSize,
							ESOffset:   0,
						},
					},
				},
			},
		},
	}

	// Create dedup entries: matched LPCM region covering all source data.
	// Include a small delta region at the start (simulating container overhead).
	const deltaSize = 16
	deltaData := bytes.Repeat([]byte{0xEE}, deltaSize)
	originalSize := int64(deltaSize + sourceDataSize)

	entries := []matcher.Entry{
		{MkvOffset: 0, Length: int64(deltaSize), Source: 0, SourceOffset: 0},
		{MkvOffset: int64(deltaSize), Length: int64(sourceDataSize), Source: 1,
			SourceOffset: 0, AudioSubStreamID: 0xA0, IsLPCM: true},
	}

	// Write dedup file with range maps.
	dedupPath := filepath.Join(dir, "test.mkvdup")
	w, err := NewWriter(dedupPath)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	w.SetHeader(originalSize, 0x1234, source.TypeDVD)
	w.SetSourceFiles([]source.File{
		{RelativePath: sourceFileName, Size: int64(len(sourceFileData)), Checksum: 0x5678},
	})
	w.SetRangeMaps(rangeMaps)
	if err := w.SetMatchResult(&matcher.Result{
		Entries:        entries,
		DeltaData:      deltaData,
		MatchedBytes:   int64(sourceDataSize),
		UnmatchedBytes: int64(deltaSize),
		TotalPackets:   2,
	}, nil); err != nil {
		t.Fatalf("SetMatchResult: %v", err)
	}
	if err := w.Write(); err != nil {
		t.Fatalf("Write: %v", err)
	}
	w.Close()

	// Open reader and load source files.
	reader, err := NewReader(dedupPath, dir)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer reader.Close()

	if err := reader.LoadSourceFiles(); err != nil {
		t.Fatalf("LoadSourceFiles: %v", err)
	}

	// Verify entry flags survived serialization.
	e, ok := reader.GetEntry(1)
	if !ok {
		t.Fatal("GetEntry(1) returned false")
	}
	if !e.IsLPCM {
		t.Error("entry 1 IsLPCM should be true")
	}
	if e.AudioSubStreamID != 0xA0 {
		t.Errorf("entry 1 AudioSubStreamID = 0x%02X, want 0xA0", e.AudioSubStreamID)
	}

	// Test ReadAt at various offsets and sizes to exercise all byte-swap alignment cases.
	tests := []struct {
		name   string
		offset int64 // offset in MKV
		size   int
	}{
		{"full LPCM region", int64(deltaSize), sourceDataSize},
		{"even offset, even size", int64(deltaSize), 100},
		{"even offset, odd size", int64(deltaSize), 99},
		{"odd offset, even size", int64(deltaSize) + 1, 100},
		{"odd offset, odd size", int64(deltaSize) + 1, 99},
		{"single byte at even offset", int64(deltaSize), 1},
		{"single byte at odd offset", int64(deltaSize) + 1, 1},
		{"last 2 bytes", int64(deltaSize) + sourceDataSize - 2, 2},
		{"last byte", int64(deltaSize) + sourceDataSize - 1, 1},
		{"chunk boundary simulation", int64(deltaSize) + 465, 1},
		{"cross entry boundary (delta + LPCM)", 0, deltaSize + 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, tt.size)
			n, err := reader.ReadAt(buf, tt.offset)
			if err != nil {
				t.Fatalf("ReadAt(%d, %d): err=%v", tt.offset, tt.size, err)
			}
			if n != tt.size {
				t.Fatalf("ReadAt(%d, %d): n=%d, want %d", tt.offset, tt.size, n, tt.size)
			}

			// Build expected output: delta bytes then byte-swapped PCM.
			for i := 0; i < n; i++ {
				mkvOffset := tt.offset + int64(i)
				var expected byte
				if mkvOffset < int64(deltaSize) {
					expected = deltaData[mkvOffset]
				} else {
					pcmIdx := int(mkvOffset) - deltaSize
					expected = expectedMKV[pcmIdx]
				}
				if buf[i] != expected {
					t.Errorf("byte %d (MKV offset %d): got 0x%02X, want 0x%02X",
						i, mkvOffset, buf[i], expected)
					if i > 5 {
						t.Fatalf("(stopping after 5+ mismatches)")
					}
				}
			}
		})
	}
}

// TestLPCM_RoundTrip_MultiPES verifies LPCM byte-swap across PES boundaries.
// This simulates the real DVD case where the ES data is spread across multiple
// PES payloads with gaps (headers) between them in the source file.
func TestLPCM_RoundTrip_MultiPES(t *testing.T) {
	dir := t.TempDir()

	// Simulate 3 PES payloads of 200 bytes each, separated by 10-byte headers.
	// Source file layout:
	//   [0..9]     PES header 1 (10 bytes)
	//   [10..209]  PCM payload 1 (200 bytes, big-endian)
	//   [210..219] PES header 2 (10 bytes)
	//   [220..419] PCM payload 2 (200 bytes, big-endian)
	//   [420..429] PES header 3 (10 bytes)
	//   [430..629] PCM payload 3 (200 bytes, big-endian)
	const pesHeaderSize = 10
	const pesPayloadSize = 200
	const numPES = 3
	const totalESSize = numPES * pesPayloadSize // 600 bytes of contiguous ES data

	// Build source file with alternating headers and payloads.
	sourceFileSize := numPES * (pesHeaderSize + pesPayloadSize)
	sourceFileData := make([]byte, sourceFileSize)
	for p := 0; p < numPES; p++ {
		headerStart := p * (pesHeaderSize + pesPayloadSize)
		payloadStart := headerStart + pesHeaderSize
		// Fill header with 0xFF (garbage, should never be read)
		for i := 0; i < pesHeaderSize; i++ {
			sourceFileData[headerStart+i] = 0xFF
		}
		// Fill payload with big-endian 16-bit samples continuing from previous PES
		for s := 0; s < pesPayloadSize/2; s++ {
			sampleIdx := p*(pesPayloadSize/2) + s
			sourceFileData[payloadStart+2*s] = byte(sampleIdx >> 8)     // HI
			sourceFileData[payloadStart+2*s+1] = byte(sampleIdx & 0xFF) // LO
		}
	}

	// Expected MKV data: contiguous little-endian samples.
	expectedMKV := make([]byte, totalESSize)
	for i := 0; i < totalESSize/2; i++ {
		expectedMKV[2*i] = byte(i & 0xFF) // LO
		expectedMKV[2*i+1] = byte(i >> 8) // HI
	}

	sourceFileName := "source.bin"
	if err := os.WriteFile(filepath.Join(dir, sourceFileName), sourceFileData, 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	// Build range map with 3 PES payload ranges.
	pesRanges := make([]source.PESPayloadRange, numPES)
	for p := 0; p < numPES; p++ {
		pesRanges[p] = source.PESPayloadRange{
			FileOffset: int64(p*(pesHeaderSize+pesPayloadSize) + pesHeaderSize),
			Size:       pesPayloadSize,
			ESOffset:   int64(p * pesPayloadSize),
		}
	}

	rangeMaps := []RangeMapData{
		{
			FileIndex: 0,
			AudioStreams: []AudioRangeData{
				{SubStreamID: 0xA0, Ranges: pesRanges},
			},
		},
	}

	// Single LPCM entry covering all 600 bytes of ES data.
	entries := []matcher.Entry{
		{MkvOffset: 0, Length: int64(totalESSize), Source: 1,
			SourceOffset: 0, AudioSubStreamID: 0xA0, IsLPCM: true},
	}

	dedupPath := filepath.Join(dir, "test.mkvdup")
	w, err := NewWriter(dedupPath)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	w.SetHeader(int64(totalESSize), 0xABCD, source.TypeDVD)
	w.SetSourceFiles([]source.File{
		{RelativePath: sourceFileName, Size: int64(sourceFileSize), Checksum: 0x9999},
	})
	w.SetRangeMaps(rangeMaps)
	if err := w.SetMatchResult(&matcher.Result{
		Entries:      entries,
		MatchedBytes: int64(totalESSize),
		TotalPackets: 1,
	}, nil); err != nil {
		t.Fatalf("SetMatchResult: %v", err)
	}
	if err := w.Write(); err != nil {
		t.Fatalf("Write: %v", err)
	}
	w.Close()

	reader, err := NewReader(dedupPath, dir)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer reader.Close()
	if err := reader.LoadSourceFiles(); err != nil {
		t.Fatalf("LoadSourceFiles: %v", err)
	}

	// Read the entire region and verify byte-for-byte.
	buf := make([]byte, totalESSize)
	n, err := reader.ReadAt(buf, 0)
	if err != nil {
		t.Fatalf("ReadAt(0, %d): n=%d, err=%v", totalESSize, n, err)
	}
	if n != totalESSize {
		t.Fatalf("ReadAt: n=%d, want %d", n, totalESSize)
	}
	for i := 0; i < totalESSize; i++ {
		if buf[i] != expectedMKV[i] {
			t.Errorf("byte %d: got 0x%02X, want 0x%02X", i, buf[i], expectedMKV[i])
			if i > 5 {
				t.Fatalf("(stopping after 5+ mismatches)")
			}
		}
	}

	// Test reads that span PES boundaries (ES offsets 198-202 cross PES 1→2 boundary).
	t.Run("cross PES boundary", func(t *testing.T) {
		crossBuf := make([]byte, 8)
		n, err := reader.ReadAt(crossBuf, 196)
		if err != nil {
			t.Fatalf("ReadAt(196, 8): err=%v", err)
		}
		if n != 8 {
			t.Fatalf("ReadAt: n=%d, want 8", n)
		}
		for i := 0; i < 8; i++ {
			if crossBuf[i] != expectedMKV[196+i] {
				t.Errorf("byte %d (offset %d): got 0x%02X, want 0x%02X",
					i, 196+i, crossBuf[i], expectedMKV[196+i])
			}
		}
	})

	// Test odd-size read ending at PES boundary.
	t.Run("odd read at PES boundary", func(t *testing.T) {
		oddBuf := make([]byte, 5)
		n, err := reader.ReadAt(oddBuf, 197)
		if err != nil {
			t.Fatalf("ReadAt(197, 5): err=%v", err)
		}
		if n != 5 {
			t.Fatalf("ReadAt: n=%d, want 5", n)
		}
		for i := 0; i < 5; i++ {
			if oddBuf[i] != expectedMKV[197+i] {
				t.Errorf("byte %d (offset %d): got 0x%02X, want 0x%02X",
					i, 197+i, oddBuf[i], expectedMKV[197+i])
			}
		}
	})
}
