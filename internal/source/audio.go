package source

// FindAudioSyncPoints finds all audio sync pattern positions in the data.
// Detects AC3, DTS, TrueHD, and MPEG Audio sync patterns.
// Returns offsets where sync patterns begin.
func FindAudioSyncPoints(data []byte) []int {
	if len(data) < 2 {
		return nil
	}

	var offsets []int

	for i := 0; i <= len(data)-2; i++ {
		// AC3/E-AC3: 0B 77
		if data[i] == 0x0B && data[i+1] == 0x77 {
			offsets = append(offsets, i)
			continue
		}

		// DTS/DTS-HD: 7F FE 80 01
		if i <= len(data)-4 &&
			data[i] == 0x7F && data[i+1] == 0xFE &&
			data[i+2] == 0x80 && data[i+3] == 0x01 {
			offsets = append(offsets, i)
			continue
		}

		// TrueHD: F8 72 6F BA
		if i <= len(data)-4 &&
			data[i] == 0xF8 && data[i+1] == 0x72 &&
			data[i+2] == 0x6F && data[i+3] == 0xBA {
			offsets = append(offsets, i)
			continue
		}

		// MPEG Audio / AAC ADTS: FF Fx (0xFF followed by 0xF0-0xFF)
		// The sync word is 11 bits of 1s, so we check for 0xFF followed by 0xFx.
		// Validate byte 2: bitrate index 1111 (upper nibble 0xF) is reserved/invalid.
		// This eliminates massive false positives from 0xFF adaptation field padding
		// in MPEG-TS, where every consecutive byte pair in a 0xFF run would match.
		if i <= len(data)-3 &&
			data[i] == 0xFF && (data[i+1]&0xF0) == 0xF0 &&
			(data[i+2]&0xF0) != 0xF0 {
			offsets = append(offsets, i)
			continue
		}
	}

	return offsets
}

// FindAudioSyncPointsInRange finds audio sync points within a specific range of data.
// This is useful for processing large files in chunks.
func FindAudioSyncPointsInRange(data []byte, startOffset int) []int {
	if len(data) < 2 {
		return nil
	}

	var offsets []int

	for i := 0; i <= len(data)-2; i++ {
		// AC3/E-AC3: 0B 77
		if data[i] == 0x0B && data[i+1] == 0x77 {
			offsets = append(offsets, startOffset+i)
			continue
		}

		// DTS/DTS-HD: 7F FE 80 01
		if i <= len(data)-4 &&
			data[i] == 0x7F && data[i+1] == 0xFE &&
			data[i+2] == 0x80 && data[i+3] == 0x01 {
			offsets = append(offsets, startOffset+i)
			continue
		}

		// TrueHD: F8 72 6F BA
		if i <= len(data)-4 &&
			data[i] == 0xF8 && data[i+1] == 0x72 &&
			data[i+2] == 0x6F && data[i+3] == 0xBA {
			offsets = append(offsets, startOffset+i)
			continue
		}

		// MPEG Audio / AAC ADTS: FF Fx with valid bitrate index
		if i <= len(data)-3 &&
			data[i] == 0xFF && (data[i+1]&0xF0) == 0xF0 &&
			(data[i+2]&0xF0) != 0xF0 {
			offsets = append(offsets, startOffset+i)
			continue
		}
	}

	return offsets
}

// AC3FrameSize returns the frame size in bytes for an AC3 sync frame given
// the fscod (sample rate code, 2 bits) and frmsizecod (frame size code, 6 bits)
// from byte 4 of the sync frame. Returns 0 if the codes are invalid.
// Based on ATSC A/52 Table 5.18.
func AC3FrameSize(fscod, frmsizecod byte) int {
	if frmsizecod >= 38 || fscod >= 3 {
		return 0
	}
	// Frame sizes in 16-bit words, indexed by [fscod][frmsizecod]
	var frameSizeWords = [3][38]int{
		// 48 kHz
		{64, 64, 80, 80, 96, 96, 112, 112, 128, 128, 160, 160, 192, 192, 224, 224, 256, 256, 320, 320, 384, 384, 448, 448, 512, 512, 640, 640, 768, 768, 896, 896, 1024, 1024, 1152, 1152, 1280, 1280},
		// 44.1 kHz
		{69, 70, 87, 88, 104, 105, 121, 122, 139, 140, 174, 175, 208, 209, 243, 244, 278, 279, 348, 349, 417, 418, 487, 488, 557, 558, 696, 697, 835, 836, 975, 976, 1114, 1115, 1253, 1254, 1393, 1394},
		// 32 kHz
		{96, 96, 120, 120, 144, 144, 168, 168, 192, 192, 240, 240, 288, 288, 336, 336, 384, 384, 480, 480, 576, 576, 672, 672, 768, 768, 960, 960, 1152, 1152, 1344, 1344, 1536, 1536, 1728, 1728, 1920, 1920},
	}
	return frameSizeWords[fscod][frmsizecod] * 2
}

// FindAllSyncPoints finds both video start codes and audio sync patterns.
// Returns combined offsets sorted by position.
func FindAllSyncPoints(data []byte) []int {
	videoOffsets := FindVideoStartCodes(data)
	audioOffsets := FindAudioSyncPoints(data)

	// Combine and sort
	combined := make([]int, 0, len(videoOffsets)+len(audioOffsets))
	combined = append(combined, videoOffsets...)
	combined = append(combined, audioOffsets...)

	// Simple insertion sort since lists are already sorted
	// and we just need to merge them
	result := make([]int, 0, len(combined))
	vi, ai := 0, 0
	for vi < len(videoOffsets) || ai < len(audioOffsets) {
		if vi >= len(videoOffsets) {
			result = append(result, audioOffsets[ai])
			ai++
		} else if ai >= len(audioOffsets) {
			result = append(result, videoOffsets[vi])
			vi++
		} else if videoOffsets[vi] <= audioOffsets[ai] {
			result = append(result, videoOffsets[vi])
			vi++
		} else {
			result = append(result, audioOffsets[ai])
			ai++
		}
	}

	return result
}
