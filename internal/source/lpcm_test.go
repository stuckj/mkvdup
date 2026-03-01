package source

import (
	"bytes"
	"testing"
)

func TestParseLPCMFrameHeader(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected LPCMFrameHeader
	}{
		{
			name: "16-bit stereo 48kHz",
			data: []byte{0x00, 0x01, 0x80}, // no emphasis, no mute, frame 0, quant=0(16), sr=0(48k), ch=1(stereo)
			expected: LPCMFrameHeader{
				Quantization: 0, SampleRate: 0, Channels: 1,
			},
		},
		{
			name: "20-bit 6ch 96kHz with emphasis",
			data: []byte{0x85, 0x55, 0x00}, // emphasis, no mute, frame 5, quant=1(20), sr=1(96k), ch=5(6ch)
			expected: LPCMFrameHeader{
				Emphasis: true, FrameNumber: 5,
				Quantization: 1, SampleRate: 1, Channels: 5,
			},
		},
		{
			name: "24-bit mono 48kHz muted",
			data: []byte{0x40, 0x80, 0xFF}, // no emphasis, mute, frame 0, quant=2(24), sr=0(48k), ch=0(mono)
			expected: LPCMFrameHeader{
				Mute:         true,
				Quantization: 2, SampleRate: 0, Channels: 0,
			},
		},
		{
			name:     "too short",
			data:     []byte{0x00, 0x01},
			expected: LPCMFrameHeader{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseLPCMFrameHeader(tt.data)
			if got != tt.expected {
				t.Errorf("ParseLPCMFrameHeader(%x) = %+v, want %+v", tt.data, got, tt.expected)
			}
		})
	}
}

func TestIsLPCM16Bit(t *testing.T) {
	if !IsLPCM16Bit(0) {
		t.Error("quant 0 should be 16-bit")
	}
	if IsLPCM16Bit(1) {
		t.Error("quant 1 (20-bit) should not be 16-bit")
	}
	if IsLPCM16Bit(2) {
		t.Error("quant 2 (24-bit) should not be 16-bit")
	}
	if IsLPCM16Bit(3) {
		t.Error("quant 3 (reserved) should not be 16-bit")
	}
}

func TestTransformLPCM16BE(t *testing.T) {
	t.Run("basic swap", func(t *testing.T) {
		data := []byte{0x01, 0x02, 0x03, 0x04}
		TransformLPCM16BE(data)
		expected := []byte{0x02, 0x01, 0x04, 0x03}
		if !bytes.Equal(data, expected) {
			t.Errorf("got %x, want %x", data, expected)
		}
	})

	t.Run("empty", func(t *testing.T) {
		data := []byte{}
		TransformLPCM16BE(data) // should not panic
	})

	t.Run("odd length", func(t *testing.T) {
		data := []byte{0x01, 0x02, 0x03}
		TransformLPCM16BE(data)
		// Only first 2 bytes swapped, third unchanged
		expected := []byte{0x02, 0x01, 0x03}
		if !bytes.Equal(data, expected) {
			t.Errorf("got %x, want %x", data, expected)
		}
	})

	t.Run("single byte", func(t *testing.T) {
		data := []byte{0xAB}
		TransformLPCM16BE(data) // should not panic
		if data[0] != 0xAB {
			t.Error("single byte should be unchanged")
		}
	})
}

func TestInverseTransformRoundTrip16(t *testing.T) {
	original := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	data := make([]byte, len(original))
	copy(data, original)

	TransformLPCM16BE(data)
	InverseTransformLPCM16(data)

	if !bytes.Equal(data, original) {
		t.Errorf("round trip failed: got %x, want %x", data, original)
	}
}

func TestFindLPCMIndexSyncPoints(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		if FindLPCMIndexSyncPoints(nil) != nil {
			t.Error("nil data should return nil")
		}
	})

	t.Run("small data", func(t *testing.T) {
		data := make([]byte, 100)
		points := FindLPCMIndexSyncPoints(data)
		if len(points) != 1 || points[0] != 0 {
			t.Errorf("expected [0], got %v", points)
		}
	})

	t.Run("fixed interval", func(t *testing.T) {
		data := make([]byte, 5000)
		points := FindLPCMIndexSyncPoints(data)
		// Expect points at 0, 2048, 4096
		expected := []int{0, 2048, 4096}
		if len(points) != len(expected) {
			t.Fatalf("expected %d sync points, got %d: %v", len(expected), len(points), points)
		}
		for i, p := range points {
			if p != expected[i] {
				t.Errorf("point[%d] = %d, want %d", i, p, expected[i])
			}
		}
	})

	t.Run("exactly at interval", func(t *testing.T) {
		data := make([]byte, 2048)
		points := FindLPCMIndexSyncPoints(data)
		if len(points) != 1 || points[0] != 0 {
			t.Errorf("expected [0], got %v", points)
		}
	})

	t.Run("one past interval", func(t *testing.T) {
		data := make([]byte, 2049)
		points := FindLPCMIndexSyncPoints(data)
		expected := []int{0, 2048}
		if len(points) != len(expected) {
			t.Fatalf("expected %d sync points, got %d", len(expected), len(points))
		}
	})

	t.Run("match sync points dense", func(t *testing.T) {
		data := make([]byte, 25)
		points := FindLPCMMatchSyncPoints(data)
		// Expect points at 0, 8, 16, 24
		expected := []int{0, 8, 16, 24}
		if len(points) != len(expected) {
			t.Fatalf("expected %d sync points, got %d: %v", len(expected), len(points), points)
		}
		for i, p := range points {
			if p != expected[i] {
				t.Errorf("point[%d] = %d, want %d", i, p, expected[i])
			}
		}
	})
}

func TestIsLPCMSubStreamID(t *testing.T) {
	// LPCM range: 0xA0-0xA7
	for id := byte(0xA0); id <= 0xA7; id++ {
		if !IsLPCMSubStreamID(id) {
			t.Errorf("0x%02X should be LPCM", id)
		}
	}

	// Not LPCM
	nonLPCM := []byte{0x80, 0x87, 0x88, 0x8F, 0x9F, 0xA8, 0xFF}
	for _, id := range nonLPCM {
		if IsLPCMSubStreamID(id) {
			t.Errorf("0x%02X should not be LPCM", id)
		}
	}
}

func TestByteSwapAlignment(t *testing.T) {
	// Verify that reading at arbitrary offsets into byte-swapped data
	// produces consistent results. This is important for the FUSE reader
	// which may read partial ranges.
	original := make([]byte, 20)
	for i := range original {
		original[i] = byte(i)
	}

	// Full transform
	full := make([]byte, len(original))
	copy(full, original)
	TransformLPCM16BE(full)

	// Verify each pair independently
	for i := 0; i+1 < len(original); i += 2 {
		pair := []byte{original[i], original[i+1]}
		TransformLPCM16BE(pair)
		if pair[0] != full[i] || pair[1] != full[i+1] {
			t.Errorf("offset %d: pair transform %x != full transform %x", i, pair, full[i:i+2])
		}
	}
}

func TestLPCMAlignedSubRead(t *testing.T) {
	// Verify the aligned-read-then-trim approach for reading byte-swapped
	// data at arbitrary offsets. This is the algorithm used by both
	// ReadAudioSubStreamData and the FUSE reader for odd-offset reads.
	//
	// Source (big-endian): [H0, L0, H1, L1, H2, L2, H3, L3, ...]
	// MKV (little-endian): [L0, H0, L1, H1, L2, H2, L3, H3, ...]
	//
	// Reading at odd offset N requires reading from N-1, swapping, then
	// trimming the first byte.

	srcData := make([]byte, 20)
	for i := range srcData {
		srcData[i] = byte(i) // [0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19]
	}

	// Compute the full MKV-format (byte-swapped) data as ground truth
	mkvData := make([]byte, len(srcData))
	copy(mkvData, srcData)
	TransformLPCM16BE(mkvData)
	// mkvData = [1,0,3,2,5,4,7,6,9,8,11,10,13,12,15,14,17,16,19,18]

	// Simulate aligned reads at various offsets and sizes
	tests := []struct {
		name   string
		offset int // offset within the ES (entry)
		size   int // bytes to read
	}{
		{"even offset, even size", 0, 6},
		{"even offset, odd size", 2, 5},
		{"odd offset, even size", 1, 4},
		{"odd offset, odd size", 3, 5},
		{"odd offset, size 1", 5, 1},
		{"even offset, size 1", 4, 1},
		{"odd offset at end", 17, 3},
		{"last byte", 19, 1},
		{"full range even", 0, 20},
		{"odd offset, size 2", 1, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the aligned read algorithm
			alignedOffset := tt.offset
			trimFront := 0
			if tt.offset%2 == 1 {
				alignedOffset = tt.offset - 1
				trimFront = 1
			}
			alignedSize := tt.size + trimFront
			// Extend to complete trailing pair if possible
			if alignedSize%2 == 1 && alignedOffset+alignedSize < len(srcData) {
				alignedSize++
			}
			if alignedOffset+alignedSize > len(srcData) {
				alignedSize = len(srcData) - alignedOffset
			}

			// Read aligned source data
			buf := make([]byte, alignedSize)
			copy(buf, srcData[alignedOffset:alignedOffset+alignedSize])

			// Byte-swap the aligned buffer
			TransformLPCM16BE(buf)

			// Trim to get the requested range
			result := buf[trimFront:]
			if len(result) > tt.size {
				result = result[:tt.size]
			}

			// Compare against ground truth
			expected := mkvData[tt.offset : tt.offset+len(result)]
			for i := range result {
				if result[i] != expected[i] {
					t.Errorf("byte %d: got 0x%02X, want 0x%02X (offset=%d, size=%d)",
						i, result[i], expected[i], tt.offset, tt.size)
				}
			}
		})
	}
}
