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

func TestLPCMQuantizationBits(t *testing.T) {
	if LPCMQuantizationBits(0) != 16 {
		t.Error("quant 0 should be 16-bit")
	}
	if LPCMQuantizationBits(1) != 20 {
		t.Error("quant 1 should be 20-bit")
	}
	if LPCMQuantizationBits(2) != 24 {
		t.Error("quant 2 should be 24-bit")
	}
	if LPCMQuantizationBits(3) != 16 {
		t.Error("quant 3 should fallback to 16-bit")
	}
}

func TestLPCMSampleRate(t *testing.T) {
	if LPCMSampleRate(0) != 48000 {
		t.Error("code 0 should be 48kHz")
	}
	if LPCMSampleRate(1) != 96000 {
		t.Error("code 1 should be 96kHz")
	}
}

func TestLPCMChannelCount(t *testing.T) {
	if LPCMChannelCount(0) != 1 {
		t.Error("code 0 should be 1 channel")
	}
	if LPCMChannelCount(1) != 2 {
		t.Error("code 1 should be 2 channels")
	}
	if LPCMChannelCount(7) != 8 {
		t.Error("code 7 should be 8 channels")
	}
}

func TestLPCMSampleGroupSize(t *testing.T) {
	// 16-bit: 2 bytes per sample per channel
	if LPCMSampleGroupSize(16, 1) != 2 {
		t.Error("16-bit mono should be 2")
	}
	if LPCMSampleGroupSize(16, 2) != 4 {
		t.Error("16-bit stereo should be 4")
	}

	// 20-bit: 2*ch + ceil(ch/2) bytes
	if LPCMSampleGroupSize(20, 1) != 3 { // 2 + 1
		t.Errorf("20-bit mono should be 3, got %d", LPCMSampleGroupSize(20, 1))
	}
	if LPCMSampleGroupSize(20, 2) != 5 { // 4 + 1
		t.Errorf("20-bit stereo should be 5, got %d", LPCMSampleGroupSize(20, 2))
	}
	if LPCMSampleGroupSize(20, 6) != 15 { // 12 + 3
		t.Errorf("20-bit 6ch should be 15, got %d", LPCMSampleGroupSize(20, 6))
	}

	// 24-bit: 3 bytes per sample per channel
	if LPCMSampleGroupSize(24, 1) != 3 {
		t.Error("24-bit mono should be 3")
	}
	if LPCMSampleGroupSize(24, 2) != 6 {
		t.Error("24-bit stereo should be 6")
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

func TestTransformLPCM20BE(t *testing.T) {
	t.Run("stereo", func(t *testing.T) {
		// 2 channels, group = 5 bytes: 4 bytes upper + 1 byte lower nibbles
		// Channel 0 upper: 0x12 0x34 → 16-bit upper = 0x1234
		// Channel 1 upper: 0x56 0x78 → 16-bit upper = 0x5678
		// Lower nibbles: 0xAB → ch0 = 0xA0, ch1 = 0xB0
		data := []byte{0x12, 0x34, 0x56, 0x78, 0xAB}
		result := TransformLPCM20BE(data, 2)
		// Output: 2 × 3 bytes = 6 bytes
		// Ch0: LE 24-bit: [0xA0, 0x34, 0x12]
		// Ch1: LE 24-bit: [0xB0, 0x78, 0x56]
		expected := []byte{0xA0, 0x34, 0x12, 0xB0, 0x78, 0x56}
		if !bytes.Equal(result, expected) {
			t.Errorf("got %x, want %x", result, expected)
		}
	})

	t.Run("nil on bad channels", func(t *testing.T) {
		if TransformLPCM20BE([]byte{1, 2, 3}, 0) != nil {
			t.Error("should return nil for 0 channels")
		}
	})

	t.Run("nil on short data", func(t *testing.T) {
		if TransformLPCM20BE([]byte{1, 2}, 2) != nil {
			t.Error("should return nil for data shorter than one group")
		}
	})
}

func TestTransformLPCM24BE(t *testing.T) {
	t.Run("stereo", func(t *testing.T) {
		// 2 channels, group = 6 bytes: 4 bytes upper + 2 bytes lower
		// Channel 0: upper = 0x1234, lower = 0x56
		// Channel 1: upper = 0x789A, lower = 0xBC
		data := []byte{0x12, 0x34, 0x78, 0x9A, 0x56, 0xBC}
		result := TransformLPCM24BE(data, 2)
		// Output: 2 × 3 bytes = 6 bytes
		// Ch0: LE 24-bit: [0x56, 0x34, 0x12]
		// Ch1: LE 24-bit: [0xBC, 0x9A, 0x78]
		expected := []byte{0x56, 0x34, 0x12, 0xBC, 0x9A, 0x78}
		if !bytes.Equal(result, expected) {
			t.Errorf("got %x, want %x", result, expected)
		}
	})

	t.Run("nil on bad channels", func(t *testing.T) {
		if TransformLPCM24BE([]byte{1, 2, 3}, 0) != nil {
			t.Error("should return nil for 0 channels")
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

func TestInverseTransformRoundTrip20(t *testing.T) {
	for _, channels := range []int{1, 2, 4, 6} {
		groupSize := LPCMSampleGroupSize(20, channels)
		// Create 3 groups of test data.
		// The lower nibble bytes are packed: even channels use upper nibble,
		// odd channels use lower nibble. Only 4 bits per channel are preserved,
		// so we need test data where the unused bits are zero.
		original := make([]byte, groupSize*3)
		for i := range original {
			original[i] = byte(i * 17)
		}
		// Zero out the unused nibble bits in the lower-nibble bytes
		// so the round trip is lossless.
		lowerStart := channels * 2
		lowerCount := (channels + 1) / 2
		for g := range 3 {
			for b := range lowerCount {
				idx := g*groupSize + lowerStart + b
				// For even channels: only upper nibble matters
				// For odd channels: only lower nibble matters
				// If this byte holds 2 channels (even+odd), both nibbles matter → keep as-is
				// If this byte holds only 1 channel (even, last byte for odd channel count),
				// only upper nibble matters → zero lower nibble
				if channels%2 == 1 && b == lowerCount-1 {
					original[idx] &= 0xF0
				}
			}
		}

		transformed := TransformLPCM20BE(original, channels)
		if transformed == nil {
			t.Fatalf("TransformLPCM20BE returned nil for %d channels", channels)
		}
		inverse := InverseTransformLPCM20(transformed, channels)
		if inverse == nil {
			t.Fatalf("InverseTransformLPCM20 returned nil for %d channels", channels)
		}

		if !bytes.Equal(inverse, original) {
			t.Errorf("round trip failed for %d channels:\n  original:  %x\n  inverse:   %x", channels, original, inverse)
		}
	}
}

func TestInverseTransformRoundTrip24(t *testing.T) {
	for _, channels := range []int{1, 2, 4, 6} {
		groupSize := LPCMSampleGroupSize(24, channels)
		original := make([]byte, groupSize*3)
		for i := range original {
			original[i] = byte(i * 31)
		}

		transformed := TransformLPCM24BE(original, channels)
		if transformed == nil {
			t.Fatalf("TransformLPCM24BE returned nil for %d channels", channels)
		}
		inverse := InverseTransformLPCM24(transformed, channels)
		if inverse == nil {
			t.Fatalf("InverseTransformLPCM24 returned nil for %d channels", channels)
		}

		if !bytes.Equal(inverse, original) {
			t.Errorf("round trip failed for %d channels:\n  original:  %x\n  inverse:   %x", channels, original, inverse)
		}
	}
}

func TestFindLPCMSyncPoints(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		if FindLPCMSyncPoints(nil) != nil {
			t.Error("nil data should return nil")
		}
	})

	t.Run("small data", func(t *testing.T) {
		data := make([]byte, 100)
		points := FindLPCMSyncPoints(data)
		if len(points) != 1 || points[0] != 0 {
			t.Errorf("expected [0], got %v", points)
		}
	})

	t.Run("fixed interval", func(t *testing.T) {
		data := make([]byte, 5000)
		points := FindLPCMSyncPoints(data)
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
		points := FindLPCMSyncPoints(data)
		if len(points) != 1 || points[0] != 0 {
			t.Errorf("expected [0], got %v", points)
		}
	})

	t.Run("one past interval", func(t *testing.T) {
		data := make([]byte, 2049)
		points := FindLPCMSyncPoints(data)
		expected := []int{0, 2048}
		if len(points) != len(expected) {
			t.Fatalf("expected %d sync points, got %d", len(expected), len(points))
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
