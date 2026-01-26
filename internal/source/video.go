package source

import "bytes"

// FindVideoStartCodes finds all video start code positions (00 00 01 XX pattern) in the data.
// These are potential sync points where video frames or other structures begin.
// Optimized to use bytes.IndexByte for fast scanning (uses SIMD on x86).
func FindVideoStartCodes(data []byte) []int {
	if len(data) < 4 {
		return nil
	}

	// Pre-allocate with estimated capacity (roughly 1 start code per 2KB of video data)
	offsets := make([]int, 0, len(data)/2048+1)

	// Use bytes.IndexByte to quickly find the 0x01 byte (third byte of start code)
	// This is faster than checking every byte since IndexByte uses SIMD
	i := 2 // Start at position 2 since we need at least 00 00 before 01
	for i < len(data)-1 {
		// Find next 0x01 byte
		idx := bytes.IndexByte(data[i:], 0x01)
		if idx < 0 {
			break
		}
		pos := i + idx

		// Check if preceded by 00 00
		if pos >= 2 && data[pos-1] == 0x00 && data[pos-2] == 0x00 {
			offsets = append(offsets, pos-2)
		}

		// Move past this position
		i = pos + 1
	}

	return offsets
}

// FindVideoStartCodesInRange finds video start codes within a specific range.
// Optimized version using bytes.IndexByte for fast scanning.
func FindVideoStartCodesInRange(data []byte, startOffset int) []int {
	if len(data) < 4 {
		return nil
	}

	// Pre-allocate with estimated capacity
	offsets := make([]int, 0, len(data)/2048+1)

	i := 2
	for i < len(data)-1 {
		idx := bytes.IndexByte(data[i:], 0x01)
		if idx < 0 {
			break
		}
		pos := i + idx

		if pos >= 2 && data[pos-1] == 0x00 && data[pos-2] == 0x00 {
			offsets = append(offsets, startOffset+pos-2)
		}

		i = pos + 1
	}

	return offsets
}
