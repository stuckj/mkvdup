package bitshift

import (
	"testing"
)

func TestApply_AllShifts(t *testing.T) {
	// Known source data
	src := []byte{0xA5, 0x3C, 0xF0, 0x0F, 0xFF, 0x00, 0x81, 0x42, 0xBD}

	for shift := uint8(1); shift <= 7; shift++ {
		dst := make([]byte, len(src)-1)
		Apply(src, shift, dst)

		// Verify each output byte manually
		rshift := 8 - shift
		for j := range dst {
			expected := (src[j] << shift) | (src[j+1] >> rshift)
			if dst[j] != expected {
				t.Errorf("shift=%d, byte %d: got %02x, want %02x", shift, j, dst[j], expected)
			}
		}
	}
}

func TestVerify_MatchesApply(t *testing.T) {
	src := []byte{0xDE, 0xAD, 0xBE, 0xEF, 0xCA, 0xFE, 0xBA, 0xBE, 0x00}

	for shift := uint8(1); shift <= 7; shift++ {
		// Generate MKV data using Apply
		mkv := make([]byte, len(src)-1)
		Apply(src, shift, mkv)

		// Verify should match
		if !Verify(src, shift, mkv) {
			t.Errorf("shift=%d: Verify returned false for data produced by Apply", shift)
		}

		// Verify with wrong shift should not match
		wrongShift := (shift % 7) + 1
		if wrongShift != shift && Verify(src, wrongShift, mkv) {
			t.Errorf("shift=%d: Verify returned true with wrong shift %d", shift, wrongShift)
		}
	}
}

func TestVerify_DetectsCorruption(t *testing.T) {
	src := []byte{0xFF, 0x00, 0xFF, 0x00, 0xFF, 0x00}
	mkv := make([]byte, len(src)-1)
	Apply(src, 3, mkv)

	// Corrupt one byte
	mkv[2] ^= 0x01

	if Verify(src, 3, mkv) {
		t.Error("Verify should detect corrupted data")
	}
}

func TestApply_SingleByte(t *testing.T) {
	src := []byte{0xAB, 0xCD}
	dst := make([]byte, 1)
	Apply(src, 4, dst)

	expected := byte((0xAB<<4)&0xFF) | byte(0xCD>>4)
	if dst[0] != expected {
		t.Errorf("got %02x, want %02x", dst[0], expected)
	}
}

func TestVerify_EmptySlice(t *testing.T) {
	src := []byte{0xFF}
	mkv := []byte{}

	// Verify with empty MKV should return true (vacuously)
	if !Verify(src, 1, mkv) {
		t.Error("Verify with empty mkv should return true")
	}
}
