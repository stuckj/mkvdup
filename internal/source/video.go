package source

import (
	"bytes"
	"encoding/binary"
)

// FindVideoStartCodes finds all video start code positions (00 00 01 XX pattern) in the data.
// Returns the position of the first 00 in each start code.
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
// Returns the position of the first 00 in each start code, offset by startOffset.
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

// FindVideoNALStarts finds NAL unit start positions in Annex B formatted data.
// Returns positions of NAL header bytes (the byte AFTER 00 00 01).
// This is used for hashing: NAL header + NAL data are identical in both
// Annex B (source) and AVCC (MKV) formats, enabling cross-format matching.
func FindVideoNALStarts(data []byte) []int {
	if len(data) < 4 {
		return nil
	}

	offsets := make([]int, 0, len(data)/2048+1)

	i := 2
	for i < len(data)-1 {
		idx := bytes.IndexByte(data[i:], 0x01)
		if idx < 0 {
			break
		}
		pos := i + idx

		// Check if preceded by 00 00 — start code is at pos-2
		// NAL header byte is at pos+1
		if pos >= 2 && data[pos-1] == 0x00 && data[pos-2] == 0x00 {
			nalStart := pos + 1
			if nalStart < len(data) {
				offsets = append(offsets, nalStart)
			}
		}

		i = pos + 1
	}

	return offsets
}

// FindVideoNALStartsInRange finds NAL unit start positions in a specific range.
// Returns positions offset by startOffset for use during chunked file processing.
func FindVideoNALStartsInRange(data []byte, startOffset int) []int {
	if len(data) < 4 {
		return nil
	}

	offsets := make([]int, 0, len(data)/2048+1)

	i := 2
	for i < len(data)-1 {
		idx := bytes.IndexByte(data[i:], 0x01)
		if idx < 0 {
			break
		}
		pos := i + idx

		if pos >= 2 && data[pos-1] == 0x00 && data[pos-2] == 0x00 {
			nalStart := pos + 1
			if nalStart < len(data) {
				offsets = append(offsets, startOffset+nalStart)
			}
		}

		i = pos + 1
	}

	return offsets
}

// FindAVCCNALStarts finds NAL unit start positions in AVCC/HVCC formatted data.
// In AVCC format, each NAL unit is prefixed with a length field (nalLengthSize bytes,
// big-endian). Returns positions of NAL header bytes (the byte after each length prefix).
// nalLengthSize is typically 4 for H.264 AVCC and H.265 HVCC.
func FindAVCCNALStarts(data []byte, nalLengthSize int) []int {
	if nalLengthSize < 1 || nalLengthSize > 4 {
		return nil
	}
	if len(data) < nalLengthSize+1 {
		return nil
	}

	offsets := make([]int, 0, len(data)/2048+1)

	pos := 0
	for pos+nalLengthSize < len(data) {
		// Read NAL unit length
		var nalLen uint32
		switch nalLengthSize {
		case 4:
			nalLen = binary.BigEndian.Uint32(data[pos:])
		case 3:
			nalLen = uint32(data[pos])<<16 | uint32(data[pos+1])<<8 | uint32(data[pos+2])
		case 2:
			nalLen = uint32(binary.BigEndian.Uint16(data[pos:]))
		case 1:
			nalLen = uint32(data[pos])
		}

		nalStart := pos + nalLengthSize
		if nalLen == 0 || nalStart >= len(data) {
			break
		}

		offsets = append(offsets, nalStart)

		// Move to next NAL unit
		next := nalStart + int(nalLen)
		if next <= pos {
			break // Overflow protection
		}
		pos = next
	}

	return offsets
}
