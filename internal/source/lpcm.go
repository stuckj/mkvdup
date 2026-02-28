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

// LPCMQuantizationBits returns the bit depth for a quantization code.
func LPCMQuantizationBits(q byte) int {
	switch q {
	case 0:
		return 16
	case 1:
		return 20
	case 2:
		return 24
	default:
		return 16 // fallback
	}
}

// LPCMSampleRate returns the sample rate for a sample rate code.
func LPCMSampleRate(code byte) int {
	switch code {
	case 0:
		return 48000
	case 1:
		return 96000
	default:
		return 48000 // fallback
	}
}

// LPCMChannelCount returns the channel count from the channels field (channels + 1).
func LPCMChannelCount(code byte) int {
	return int(code) + 1
}

// LPCMSampleGroupSize returns the number of bytes per sample group for the
// given bit depth and channel count. A sample group is the smallest unit of
// data that can be independently transformed.
//
// For 16-bit: 2 bytes per sample × channels
// For 20-bit: (2 bytes upper + 1 byte lower per 2 channels) × channels, rounded
// For 24-bit: (2 bytes upper + 1 byte lower) × channels = 3 × channels
func LPCMSampleGroupSize(bitDepth, channels int) int {
	switch bitDepth {
	case 16:
		return 2 * channels
	case 20:
		// DVD 20-bit: N channels × 2 bytes (upper 16 bits) + ceil(N/2) bytes (lower 4 bits packed in pairs)
		return 2*channels + (channels+1)/2
	case 24:
		// DVD 24-bit: N channels × 2 bytes (upper 16 bits) + N bytes (lower 8 bits)
		return 3 * channels
	default:
		return 2 * channels
	}
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

// TransformLPCM20BE unpacks DVD 20-bit grouped big-endian samples into
// interleaved little-endian 24-bit samples (3 bytes per sample, zero-padded
// lower 4 bits). This matches what video extraction tools produce for A_PCM/INT/LIT.
//
// DVD 20-bit packing (per sample group of N channels):
//
//	First: N × 2 bytes of upper 16 bits (big-endian)
//	Then: ceil(N/2) bytes of lower 4-bit nibbles packed in pairs
//
// Output: N × 3 bytes per sample, little-endian 24-bit with lower 4 bits zero-padded.
func TransformLPCM20BE(data []byte, channels int) []byte {
	if channels <= 0 {
		return nil
	}
	groupIn := LPCMSampleGroupSize(20, channels)
	if groupIn == 0 || len(data) < groupIn {
		return nil
	}

	// Output: 3 bytes per sample per channel
	numGroups := len(data) / groupIn
	out := make([]byte, numGroups*channels*3)

	for g := range numGroups {
		inOff := g * groupIn
		outOff := g * channels * 3
		upperBytes := data[inOff : inOff+channels*2]
		lowerBytes := data[inOff+channels*2 : inOff+groupIn]

		for ch := range channels {
			hi := upperBytes[ch*2]
			lo := upperBytes[ch*2+1]
			// Lower nibble: packed 2 per byte, upper nibble = even channel, lower nibble = odd channel
			var lower byte
			if ch/2 < len(lowerBytes) {
				if ch%2 == 0 {
					lower = lowerBytes[ch/2] & 0xF0 // upper nibble
				} else {
					lower = (lowerBytes[ch/2] & 0x0F) << 4 // lower nibble, shift to upper
				}
			}
			// Little-endian 24-bit: [low_byte, mid_byte, high_byte]
			out[outOff+ch*3] = lower // lowest byte (4 data bits + 4 zero bits)
			out[outOff+ch*3+1] = lo  // middle byte
			out[outOff+ch*3+2] = hi  // highest byte
		}
	}
	return out
}

// TransformLPCM24BE unpacks DVD 24-bit grouped big-endian samples into
// interleaved little-endian 24-bit samples (3 bytes per sample).
//
// DVD 24-bit packing (per sample group of N channels):
//
//	First: N × 2 bytes of upper 16 bits (big-endian)
//	Then: N bytes of lower 8 bits
//
// Output: N × 3 bytes per sample, little-endian.
func TransformLPCM24BE(data []byte, channels int) []byte {
	if channels <= 0 {
		return nil
	}
	groupIn := LPCMSampleGroupSize(24, channels)
	if groupIn == 0 || len(data) < groupIn {
		return nil
	}

	numGroups := len(data) / groupIn
	out := make([]byte, numGroups*channels*3)

	for g := range numGroups {
		inOff := g * groupIn
		outOff := g * channels * 3
		upperBytes := data[inOff : inOff+channels*2]
		lowerBytes := data[inOff+channels*2 : inOff+groupIn]

		for ch := range channels {
			hi := upperBytes[ch*2]
			lo := upperBytes[ch*2+1]
			low := lowerBytes[ch]
			// Little-endian 24-bit: [low_byte, mid_byte, high_byte]
			out[outOff+ch*3] = low
			out[outOff+ch*3+1] = lo
			out[outOff+ch*3+2] = hi
		}
	}
	return out
}

// InverseTransformLPCM16 converts little-endian 16-bit PCM back to big-endian.
// Byte swap is its own inverse, so this is identical to TransformLPCM16BE.
func InverseTransformLPCM16(data []byte) {
	TransformLPCM16BE(data)
}

// InverseTransformLPCM20 converts interleaved little-endian 24-bit samples
// (from a 20-bit source) back into DVD 20-bit grouped big-endian format.
func InverseTransformLPCM20(data []byte, channels int) []byte {
	if channels <= 0 {
		return nil
	}
	// Input: 3 bytes per sample × channels per group
	groupOut := 3 * channels
	if groupOut == 0 || len(data) < groupOut {
		return nil
	}

	numGroups := len(data) / groupOut
	dvdGroupSize := LPCMSampleGroupSize(20, channels)
	out := make([]byte, numGroups*dvdGroupSize)

	for g := range numGroups {
		inOff := g * groupOut
		outOff := g * dvdGroupSize
		upperOff := outOff
		lowerOff := outOff + channels*2

		for ch := range channels {
			low := data[inOff+ch*3]  // lowest byte
			lo := data[inOff+ch*3+1] // middle byte
			hi := data[inOff+ch*3+2] // highest byte

			// Upper 16 bits: big-endian
			out[upperOff+ch*2] = hi
			out[upperOff+ch*2+1] = lo

			// Lower nibbles: pack 2 per byte
			if ch/2 < dvdGroupSize-channels*2 {
				if ch%2 == 0 {
					out[lowerOff+ch/2] |= low & 0xF0 // upper nibble
				} else {
					out[lowerOff+ch/2] |= (low >> 4) & 0x0F // lower nibble
				}
			}
		}
	}
	return out
}

// InverseTransformLPCM24 converts interleaved little-endian 24-bit samples
// back into DVD 24-bit grouped big-endian format.
func InverseTransformLPCM24(data []byte, channels int) []byte {
	if channels <= 0 {
		return nil
	}
	groupOut := 3 * channels
	if groupOut == 0 || len(data) < groupOut {
		return nil
	}

	numGroups := len(data) / groupOut
	dvdGroupSize := LPCMSampleGroupSize(24, channels)
	out := make([]byte, numGroups*dvdGroupSize)

	for g := range numGroups {
		inOff := g * groupOut
		outOff := g * dvdGroupSize
		upperOff := outOff
		lowerOff := outOff + channels*2

		for ch := range channels {
			low := data[inOff+ch*3]
			lo := data[inOff+ch*3+1]
			hi := data[inOff+ch*3+2]

			out[upperOff+ch*2] = hi
			out[upperOff+ch*2+1] = lo
			out[lowerOff+ch] = low
		}
	}
	return out
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
