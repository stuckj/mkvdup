package source

// DVD LPCM audio frame format (after 4-byte PS private stream header):
//
//	Byte 0: emphasis(1) | mute(1) | reserved(1) | frame_number(5)
//	Byte 1: quant_word_length(2) | sampling_freq(2) | reserved(1) | num_channels(3)
//	Byte 2: dynamic_range_control
//	Bytes 3+: PCM sample data (big-endian, grouped by bit depth)
//
// DVD stores big-endian samples with per-frame headers, while MKV stores
// A_PCM/INT/LIT (raw little-endian PCM, no framing). The transforms in this
// file convert between these two representations.

// LPCMHeaderSize is the size of the LPCM frame header after the 4-byte PS header.
const LPCMHeaderSize = 3

// LPCMTotalHeaderSize is the total header size to strip (4-byte PS + 3-byte LPCM).
const LPCMTotalHeaderSize = 7

// lpcmSyncInterval is the interval in bytes between LPCM sync points.
// PCM has no natural sync patterns, so we use a fixed interval.
const lpcmSyncInterval = 2048

// LPCMFrameHeader represents a parsed DVD LPCM frame header.
type LPCMFrameHeader struct {
	Emphasis     bool
	Mute         bool
	FrameNumber  byte // 5 bits
	Quantization byte // 2 bits: 0=16-bit, 1=20-bit, 2=24-bit
	SampleRate   byte // 2 bits: 0=48kHz, 1=96kHz
	Channels     byte // 3 bits: number of channels minus 1
}

// ParseLPCMFrameHeader parses a 3-byte DVD LPCM frame header.
func ParseLPCMFrameHeader(data []byte) LPCMFrameHeader {
	if len(data) < LPCMHeaderSize {
		return LPCMFrameHeader{}
	}
	return LPCMFrameHeader{
		Emphasis:     data[0]&0x80 != 0,
		Mute:         data[0]&0x40 != 0,
		FrameNumber:  data[0] & 0x1F,
		Quantization: (data[1] >> 6) & 0x03,
		SampleRate:   (data[1] >> 4) & 0x03,
		Channels:     data[1] & 0x07,
	}
}

// IsLPCM16Bit returns true if the quantization code indicates 16-bit LPCM.
// Only 16-bit LPCM is supported for matching and FUSE reconstruction.
// 20-bit (code 1) and 24-bit (code 2) use grouped big-endian packing that
// changes data size during transform, making in-place FUSE reconstruction
// infeasible without significant complexity.
func IsLPCM16Bit(quantization byte) bool {
	return quantization == 0
}

// TransformLPCM16BE performs an in-place byte swap for 16-bit big-endian PCM
// samples, converting to little-endian. Each pair of bytes [HI][LO] becomes
// [LO][HI]. If len(data) is odd, the last byte is left unchanged.
func TransformLPCM16BE(data []byte) {
	n := len(data) &^ 1 // round down to even
	for i := 0; i < n; i += 2 {
		data[i], data[i+1] = data[i+1], data[i]
	}
}

// InverseTransformLPCM16 converts little-endian 16-bit PCM back to big-endian.
// Byte swap is its own inverse, so this is identical to TransformLPCM16BE.
func InverseTransformLPCM16(data []byte) {
	TransformLPCM16BE(data)
}

// FindLPCMSyncPoints returns fixed-interval sync points for LPCM data.
// PCM has no natural sync patterns, so we generate sync points every
// lpcmSyncInterval bytes, aligned to the sample group boundary.
func FindLPCMSyncPoints(data []byte) []int {
	if len(data) == 0 {
		return nil
	}

	var offsets []int
	for off := 0; off < len(data); off += lpcmSyncInterval {
		offsets = append(offsets, off)
	}
	return offsets
}

// IsLPCMSubStreamID returns true if the sub-stream ID is in the LPCM range (0xA0-0xA7).
func IsLPCMSubStreamID(subStreamID byte) bool {
	return subStreamID >= 0xA0 && subStreamID <= 0xA7
}
