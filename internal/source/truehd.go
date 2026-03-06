package source

// ParseTrueHDAULength extracts the access unit length in bytes from
// the first 2 bytes of a TrueHD AU header. The lower 12 bits encode
// the length in 16-bit words; multiply by 2 for bytes.
func ParseTrueHDAULength(header []byte) int {
	if len(header) < 2 {
		return 0
	}
	return (int(header[0])<<8 | int(header[1])) & 0x0FFF * 2
}
