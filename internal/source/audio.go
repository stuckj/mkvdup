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
		// The sync word is 11 bits of 1s, so we check for 0xFF followed by 0xFx
		if data[i] == 0xFF && (data[i+1]&0xF0) == 0xF0 {
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

		// MPEG Audio / AAC ADTS: FF Fx
		if data[i] == 0xFF && (data[i+1]&0xF0) == 0xF0 {
			offsets = append(offsets, startOffset+i)
			continue
		}
	}

	return offsets
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
