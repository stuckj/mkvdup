package source

// FindVideoStartCodes finds all video start code positions (00 00 01 XX pattern) in the data.
// These are potential sync points where video frames or other structures begin.
func FindVideoStartCodes(data []byte) []int {
	if len(data) < 4 {
		return nil
	}

	var offsets []int

	// Scan for 00 00 01 XX pattern (any video start code)
	for i := 0; i <= len(data)-4; i++ {
		if data[i] == 0x00 && data[i+1] == 0x00 && data[i+2] == 0x01 {
			offsets = append(offsets, i)
		}
	}

	return offsets
}

// FindVideoStartCodesInRange finds video start codes within a specific range.
func FindVideoStartCodesInRange(data []byte, startOffset int) []int {
	if len(data) < 4 {
		return nil
	}

	var offsets []int

	for i := 0; i <= len(data)-4; i++ {
		if data[i] == 0x00 && data[i+1] == 0x00 && data[i+2] == 0x01 {
			offsets = append(offsets, startOffset+i)
		}
	}

	return offsets
}
