package source

// FindVideoStartCodes finds all video start code positions in the data.
// Video codecs (MPEG-2, H.264, HEVC, VC-1) use 00 00 01 as a start code prefix.
// Returns offsets where start codes begin.
func FindVideoStartCodes(data []byte) []int {
	if len(data) < 3 {
		return nil
	}

	var offsets []int

	// Scan for 00 00 01 pattern
	for i := 0; i <= len(data)-3; i++ {
		if data[i] == 0x00 && data[i+1] == 0x00 && data[i+2] == 0x01 {
			offsets = append(offsets, i)
		}
	}

	return offsets
}

// FindVideoStartCodesInRange finds video start codes within a specific range of data.
// This is useful for processing large files in chunks.
func FindVideoStartCodesInRange(data []byte, startOffset int) []int {
	if len(data) < 3 {
		return nil
	}

	var offsets []int

	for i := 0; i <= len(data)-3; i++ {
		if data[i] == 0x00 && data[i+1] == 0x00 && data[i+2] == 0x01 {
			offsets = append(offsets, startOffset+i)
		}
	}

	return offsets
}
