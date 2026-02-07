package source

// FindPGSSyncPoints returns byte offsets of PGS segment boundaries in data.
// PGS segments have a 3-byte header: [type (1 byte)] [size (2 bytes BE)].
// Each segment start is a sync point. Valid segment types are:
// 0x14 (PDS), 0x15 (ODS), 0x16 (PCS), 0x17 (WDS), 0x80 (END).
func FindPGSSyncPoints(data []byte) []int {
	var offsets []int
	off := 0
	for off+3 <= len(data) {
		segType := data[off]
		if !isValidPGSSegmentType(segType) {
			break
		}
		offsets = append(offsets, off)
		segSize := int(data[off+1])<<8 | int(data[off+2])
		off += 3 + segSize
	}
	return offsets
}

func isValidPGSSegmentType(t byte) bool {
	switch t {
	case 0x14, 0x15, 0x16, 0x17, 0x80:
		return true
	}
	return false
}
