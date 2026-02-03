// Package mkv provides functionality for parsing MKV (Matroska) files.
package mkv

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// EBML Element IDs (Matroska specification)
const (
	// EBML Header elements
	IDEBMLHeader        = 0x1A45DFA3
	IDEBMLVersion       = 0x4286
	IDEBMLReadVersion   = 0x42F7
	IDEBMLMaxIDLength   = 0x42F2
	IDEBMLMaxSizeLength = 0x42F3
	IDDocType           = 0x4282
	IDDocTypeVersion    = 0x4287
	IDDocTypeReadVer    = 0x4285

	// Segment and top-level elements
	IDSegment  = 0x18538067
	IDSeekHead = 0x114D9B74
	IDInfo     = 0x1549A966
	IDTracks   = 0x1654AE6B
	IDChapters = 0x1043A770
	IDCluster  = 0x1F43B675
	IDCues     = 0x1C53BB6B
	IDTags     = 0x1254C367

	// Cluster elements
	IDTimestamp   = 0xE7
	IDSimpleBlock = 0xA3
	IDBlockGroup  = 0xA0
	IDBlock       = 0xA1

	// Track elements
	IDTrackEntry    = 0xAE
	IDTrackNum      = 0xD7
	IDTrackUID      = 0x73C5
	IDTrackType     = 0x83
	IDCodecID       = 0x86
	IDCodecPrivate  = 0x63A2
)

// Track types
const (
	TrackTypeVideo    = 1
	TrackTypeAudio    = 2
	TrackTypeComplex  = 3
	TrackTypeLogo     = 0x10
	TrackTypeSubtitle = 0x11
	TrackTypeButtons  = 0x12
	TrackTypeControl  = 0x20
)

// ErrInvalidEBML is returned when EBML parsing fails.
var ErrInvalidEBML = errors.New("invalid EBML data")

// Element represents a parsed EBML element.
type Element struct {
	ID         uint64 // Element ID (variable length)
	Size       int64  // Element size (-1 for unknown size)
	DataOffset int64  // Offset of element data in file
	HeaderSize int    // Size of ID + Size encoding
}

// ReadElementHeader reads an EBML element header (ID and size) from the reader.
// Returns the element info and any error encountered.
func ReadElementHeader(r io.Reader, offset int64) (Element, error) {
	elem := Element{DataOffset: offset}

	// Read element ID (variable length, 1-4 bytes)
	id, idLen, err := readVINT(r, true)
	if err != nil {
		return elem, fmt.Errorf("read element ID: %w", err)
	}
	elem.ID = id
	elem.HeaderSize = idLen

	// Read element size (variable length, 1-8 bytes)
	size, sizeLen, err := readVINT(r, false)
	if err != nil {
		return elem, fmt.Errorf("read element size: %w", err)
	}
	elem.HeaderSize += sizeLen

	// Handle unknown size (all 1 bits after VINT marker)
	if isUnknownSize(size, sizeLen) {
		elem.Size = -1
	} else {
		elem.Size = int64(size)
	}

	elem.DataOffset = offset + int64(elem.HeaderSize)

	return elem, nil
}

// readVINT reads a variable-length integer (VINT) used in EBML.
// If keepMarker is true, the VINT marker bit is preserved (used for IDs).
// Returns the value, number of bytes read, and any error.
func readVINT(r io.Reader, keepMarker bool) (uint64, int, error) {
	// Read first byte to determine length
	var firstByte [1]byte
	if _, err := io.ReadFull(r, firstByte[:]); err != nil {
		return 0, 0, err
	}

	b := firstByte[0]
	if b == 0 {
		return 0, 0, ErrInvalidEBML
	}

	// Determine length from leading zeros
	var length int
	var mask byte
	switch {
	case b&0x80 != 0:
		length = 1
		mask = 0x7F
	case b&0x40 != 0:
		length = 2
		mask = 0x3F
	case b&0x20 != 0:
		length = 3
		mask = 0x1F
	case b&0x10 != 0:
		length = 4
		mask = 0x0F
	case b&0x08 != 0:
		length = 5
		mask = 0x07
	case b&0x04 != 0:
		length = 6
		mask = 0x03
	case b&0x02 != 0:
		length = 7
		mask = 0x01
	case b&0x01 != 0:
		length = 8
		mask = 0x00
	default:
		return 0, 0, ErrInvalidEBML
	}

	// Build the value
	var value uint64
	if keepMarker {
		value = uint64(b)
	} else {
		value = uint64(b & mask)
	}

	// Read remaining bytes
	if length > 1 {
		remaining := make([]byte, length-1)
		if _, err := io.ReadFull(r, remaining); err != nil {
			return 0, 0, err
		}
		for _, rb := range remaining {
			value = (value << 8) | uint64(rb)
		}
	}

	return value, length, nil
}

// isUnknownSize checks if a VINT value represents "unknown size".
// Unknown size is represented by all data bits being 1.
func isUnknownSize(value uint64, length int) bool {
	// Unknown size values: 0x7F (1 byte), 0x3FFF (2 bytes), etc.
	maxValues := []uint64{
		0x7F,
		0x3FFF,
		0x1FFFFF,
		0x0FFFFFFF,
		0x07FFFFFFFF,
		0x03FFFFFFFFFF,
		0x01FFFFFFFFFFFF,
		0x00FFFFFFFFFFFFFF,
	}
	if length < 1 || length > 8 {
		return false
	}
	return value == maxValues[length-1]
}

// ReadUint reads an unsigned integer element value.
func ReadUint(r io.Reader, size int64) (uint64, error) {
	if size < 0 || size > 8 {
		return 0, fmt.Errorf("invalid uint size: %d", size)
	}
	if size == 0 {
		return 0, nil
	}

	buf := make([]byte, size)
	if _, err := io.ReadFull(r, buf); err != nil {
		return 0, err
	}

	var value uint64
	for _, b := range buf {
		value = (value << 8) | uint64(b)
	}
	return value, nil
}

// ReadInt reads a signed integer element value.
func ReadInt(r io.Reader, size int64) (int64, error) {
	u, err := ReadUint(r, size)
	if err != nil {
		return 0, err
	}

	// Sign extend if necessary
	if size > 0 && u>>(uint(size)*8-1) != 0 {
		// Negative number - extend sign
		mask := ^uint64(0) << (uint(size) * 8)
		return int64(u | mask), nil
	}
	return int64(u), nil
}

// ReadString reads a string element value.
func ReadString(r io.Reader, size int64) (string, error) {
	if size < 0 {
		return "", fmt.Errorf("invalid string size: %d", size)
	}
	if size == 0 {
		return "", nil
	}

	buf := make([]byte, size)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}

	// Trim null bytes
	for i := len(buf) - 1; i >= 0 && buf[i] == 0; i-- {
		buf = buf[:i]
	}

	return string(buf), nil
}

// ReadBinary reads binary data of the specified size.
func ReadBinary(r io.Reader, size int64) ([]byte, error) {
	if size < 0 {
		return nil, fmt.Errorf("invalid binary size: %d", size)
	}
	if size == 0 {
		return nil, nil
	}

	buf := make([]byte, size)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// SimpleBlockHeader contains the decoded header of a SimpleBlock.
type SimpleBlockHeader struct {
	TrackNumber uint64
	Timestamp   int16 // Relative to cluster timestamp
	Flags       byte  // Keyframe, invisible, lacing, discardable
	HeaderSize  int   // Total header size in bytes
}

// Block flags
const (
	FlagKeyframe    = 0x80
	FlagInvisible   = 0x08
	FlagLacing      = 0x06 // Mask for lacing type
	FlagDiscardable = 0x01
)

// Lacing types
const (
	LacingNone  = 0x00
	LacingXiph  = 0x02
	LacingFixed = 0x04
	LacingEBML  = 0x06
)

// ParseSimpleBlockHeader parses the header of a SimpleBlock element.
// The data should start at the beginning of the SimpleBlock element data (after ID and size).
func ParseSimpleBlockHeader(data []byte) (SimpleBlockHeader, error) {
	if len(data) < 4 {
		return SimpleBlockHeader{}, fmt.Errorf("SimpleBlock too short: %d bytes", len(data))
	}

	var header SimpleBlockHeader
	offset := 0

	// Track number (VINT without marker)
	trackNum, trackLen := parseVINTFromBytes(data[offset:])
	header.TrackNumber = trackNum
	offset += trackLen

	if offset+3 > len(data) {
		return SimpleBlockHeader{}, fmt.Errorf("SimpleBlock header truncated")
	}

	// Timestamp (2 bytes, signed, big-endian)
	header.Timestamp = int16(binary.BigEndian.Uint16(data[offset:]))
	offset += 2

	// Flags (1 byte)
	header.Flags = data[offset]
	offset++

	header.HeaderSize = offset
	return header, nil
}

// parseVINTFromBytes parses a VINT from a byte slice (without marker preservation).
func parseVINTFromBytes(data []byte) (uint64, int) {
	if len(data) == 0 {
		return 0, 0
	}

	b := data[0]
	if b == 0 {
		return 0, 0
	}

	var length int
	var mask byte
	switch {
	case b&0x80 != 0:
		length = 1
		mask = 0x7F
	case b&0x40 != 0:
		length = 2
		mask = 0x3F
	case b&0x20 != 0:
		length = 3
		mask = 0x1F
	case b&0x10 != 0:
		length = 4
		mask = 0x0F
	default:
		return 0, 0
	}

	if len(data) < length {
		return 0, 0
	}

	var value uint64 = uint64(b & mask)
	for i := 1; i < length; i++ {
		value = (value << 8) | uint64(data[i])
	}

	return value, length
}

// IsKeyframe returns true if the SimpleBlock/Block is a keyframe.
func (h SimpleBlockHeader) IsKeyframe() bool {
	return h.Flags&FlagKeyframe != 0
}

// LacingType returns the lacing type used in the block.
func (h SimpleBlockHeader) LacingType() byte {
	return h.Flags & FlagLacing
}
