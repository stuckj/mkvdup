package mkv

import (
	"bytes"
	"os"
	"testing"
)

func TestReadVINT(t *testing.T) {
	tests := []struct {
		name       string
		data       []byte
		keepMarker bool
		wantValue  uint64
		wantLen    int
		wantErr    bool
	}{
		{
			name:       "1-byte VINT (0x81 = 1)",
			data:       []byte{0x81},
			keepMarker: false,
			wantValue:  1,
			wantLen:    1,
		},
		{
			name:       "1-byte VINT with marker",
			data:       []byte{0x81},
			keepMarker: true,
			wantValue:  0x81,
			wantLen:    1,
		},
		{
			name:       "2-byte VINT (0x4000 = 0)",
			data:       []byte{0x40, 0x00},
			keepMarker: false,
			wantValue:  0,
			wantLen:    2,
		},
		{
			name:       "2-byte VINT (0x4001 = 1)",
			data:       []byte{0x40, 0x01},
			keepMarker: false,
			wantValue:  1,
			wantLen:    2,
		},
		{
			name:       "3-byte VINT",
			data:       []byte{0x20, 0x00, 0x01},
			keepMarker: false,
			wantValue:  1,
			wantLen:    3,
		},
		{
			name:       "4-byte VINT",
			data:       []byte{0x10, 0x00, 0x00, 0x01},
			keepMarker: false,
			wantValue:  1,
			wantLen:    4,
		},
		{
			name:       "EBML header ID",
			data:       []byte{0x1A, 0x45, 0xDF, 0xA3},
			keepMarker: true,
			wantValue:  0x1A45DFA3,
			wantLen:    4,
		},
		{
			name:       "Segment ID",
			data:       []byte{0x18, 0x53, 0x80, 0x67},
			keepMarker: true,
			wantValue:  0x18538067,
			wantLen:    4,
		},
		{
			name:       "empty data",
			data:       []byte{},
			keepMarker: false,
			wantErr:    true,
		},
		{
			name:       "invalid VINT (0x00)",
			data:       []byte{0x00},
			keepMarker: false,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bytes.NewReader(tt.data)
			value, length, err := readVINT(r, tt.keepMarker)

			if tt.wantErr {
				if err == nil {
					t.Errorf("readVINT() expected error, got none")
				}
				return
			}

			if err != nil {
				t.Errorf("readVINT() error = %v", err)
				return
			}

			if value != tt.wantValue {
				t.Errorf("readVINT() value = 0x%X, want 0x%X", value, tt.wantValue)
			}
			if length != tt.wantLen {
				t.Errorf("readVINT() length = %d, want %d", length, tt.wantLen)
			}
		})
	}
}

func TestReadElementHeader(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		wantID   uint64
		wantSize int64
		wantErr  bool
	}{
		{
			name:     "EBML header element",
			data:     []byte{0x1A, 0x45, 0xDF, 0xA3, 0x9F}, // ID + size 31 (0x9F = 0x80 | 0x1F)
			wantID:   0x1A45DFA3,
			wantSize: 31,
		},
		{
			name:     "simple element",
			data:     []byte{0x42, 0x86, 0x81, 0x01}, // EBML version, size 1, value 1
			wantID:   0x4286,
			wantSize: 1,
		},
		{
			name:     "Cluster element",
			data:     []byte{0x1F, 0x43, 0xB6, 0x75, 0x01, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00},
			wantID:   0x1F43B675,
			wantSize: 0x10000, // 65536
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bytes.NewReader(tt.data)
			elem, err := ReadElementHeader(r, 0)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ReadElementHeader() expected error, got none")
				}
				return
			}

			if err != nil {
				t.Errorf("ReadElementHeader() error = %v", err)
				return
			}

			if elem.ID != tt.wantID {
				t.Errorf("ReadElementHeader() ID = 0x%X, want 0x%X", elem.ID, tt.wantID)
			}
			if elem.Size != tt.wantSize {
				t.Errorf("ReadElementHeader() Size = %d, want %d", elem.Size, tt.wantSize)
			}
		})
	}
}

func TestReadUint(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		size      int64
		wantValue uint64
		wantErr   bool
	}{
		{
			name:      "1-byte uint",
			data:      []byte{0x42},
			size:      1,
			wantValue: 0x42,
		},
		{
			name:      "2-byte uint",
			data:      []byte{0x12, 0x34},
			size:      2,
			wantValue: 0x1234,
		},
		{
			name:      "4-byte uint",
			data:      []byte{0x12, 0x34, 0x56, 0x78},
			size:      4,
			wantValue: 0x12345678,
		},
		{
			name:      "8-byte uint",
			data:      []byte{0x12, 0x34, 0x56, 0x78, 0x9A, 0xBC, 0xDE, 0xF0},
			size:      8,
			wantValue: 0x123456789ABCDEF0,
		},
		{
			name:      "zero size",
			data:      []byte{},
			size:      0,
			wantValue: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bytes.NewReader(tt.data)
			value, err := ReadUint(r, tt.size)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ReadUint() expected error, got none")
				}
				return
			}

			if err != nil {
				t.Errorf("ReadUint() error = %v", err)
				return
			}

			if value != tt.wantValue {
				t.Errorf("ReadUint() = 0x%X, want 0x%X", value, tt.wantValue)
			}
		})
	}
}

func TestReadString(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		size      int64
		wantValue string
	}{
		{
			name:      "simple string",
			data:      []byte("hello"),
			size:      5,
			wantValue: "hello",
		},
		{
			name:      "string with null padding",
			data:      []byte{'h', 'i', 0, 0, 0},
			size:      5,
			wantValue: "hi",
		},
		{
			name:      "empty string",
			data:      []byte{},
			size:      0,
			wantValue: "",
		},
		{
			name:      "codec ID",
			data:      []byte("V_MPEG4/ISO/AVC"),
			size:      15,
			wantValue: "V_MPEG4/ISO/AVC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bytes.NewReader(tt.data)
			value, err := ReadString(r, tt.size)
			if err != nil {
				t.Errorf("ReadString() error = %v", err)
				return
			}
			if value != tt.wantValue {
				t.Errorf("ReadString() = %q, want %q", value, tt.wantValue)
			}
		})
	}
}

func TestParseSimpleBlockHeader(t *testing.T) {
	tests := []struct {
		name       string
		data       []byte
		wantTrack  uint64
		wantTS     int16
		wantFlags  byte
		wantHeader int
		wantErr    bool
	}{
		{
			name:       "basic SimpleBlock",
			data:       []byte{0x81, 0x00, 0x00, 0x80}, // Track 1, TS 0, keyframe
			wantTrack:  1,
			wantTS:     0,
			wantFlags:  0x80,
			wantHeader: 4,
		},
		{
			name:       "track 2 with timestamp",
			data:       []byte{0x82, 0x00, 0x10, 0x00}, // Track 2, TS 16, no keyframe
			wantTrack:  2,
			wantTS:     16,
			wantFlags:  0x00,
			wantHeader: 4,
		},
		{
			name:       "negative timestamp",
			data:       []byte{0x81, 0xFF, 0xF0, 0x80}, // Track 1, TS -16, keyframe
			wantTrack:  1,
			wantTS:     -16,
			wantFlags:  0x80,
			wantHeader: 4,
		},
		{
			name:    "too short",
			data:    []byte{0x81, 0x00},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header, err := ParseSimpleBlockHeader(tt.data)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseSimpleBlockHeader() expected error, got none")
				}
				return
			}

			if err != nil {
				t.Errorf("ParseSimpleBlockHeader() error = %v", err)
				return
			}

			if header.TrackNumber != tt.wantTrack {
				t.Errorf("TrackNumber = %d, want %d", header.TrackNumber, tt.wantTrack)
			}
			if header.Timestamp != tt.wantTS {
				t.Errorf("Timestamp = %d, want %d", header.Timestamp, tt.wantTS)
			}
			if header.Flags != tt.wantFlags {
				t.Errorf("Flags = 0x%X, want 0x%X", header.Flags, tt.wantFlags)
			}
			if header.HeaderSize != tt.wantHeader {
				t.Errorf("HeaderSize = %d, want %d", header.HeaderSize, tt.wantHeader)
			}
		})
	}
}

func TestSimpleBlockHeader_IsKeyframe(t *testing.T) {
	tests := []struct {
		name  string
		flags byte
		want  bool
	}{
		{"keyframe", 0x80, true},
		{"not keyframe", 0x00, false},
		{"keyframe with other flags", 0x86, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := SimpleBlockHeader{Flags: tt.flags}
			if got := h.IsKeyframe(); got != tt.want {
				t.Errorf("IsKeyframe() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSimpleBlockHeader_LacingType(t *testing.T) {
	tests := []struct {
		name  string
		flags byte
		want  byte
	}{
		{"no lacing", 0x80, LacingNone},
		{"Xiph lacing", 0x82, LacingXiph},
		{"fixed lacing", 0x84, LacingFixed},
		{"EBML lacing", 0x86, LacingEBML},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := SimpleBlockHeader{Flags: tt.flags}
			if got := h.LacingType(); got != tt.want {
				t.Errorf("LacingType() = 0x%X, want 0x%X", got, tt.want)
			}
		})
	}
}

func TestIsUnknownSize(t *testing.T) {
	tests := []struct {
		value  uint64
		length int
		want   bool
	}{
		{0x7F, 1, true},
		{0x3FFF, 2, true},
		{0x1FFFFF, 3, true},
		{0x0FFFFFFF, 4, true},
		{100, 1, false},
		{1000, 2, false},
	}

	for _, tt := range tests {
		got := isUnknownSize(tt.value, tt.length)
		if got != tt.want {
			t.Errorf("isUnknownSize(%d, %d) = %v, want %v", tt.value, tt.length, got, tt.want)
		}
	}
}

func TestReadVINT_LongerValues(t *testing.T) {
	tests := []struct {
		name       string
		data       []byte
		keepMarker bool
		wantValue  uint64
		wantLen    int
	}{
		{
			name:       "5-byte VINT",
			data:       []byte{0x08, 0x00, 0x00, 0x00, 0x01},
			keepMarker: false,
			wantValue:  1,
			wantLen:    5,
		},
		{
			name:       "6-byte VINT",
			data:       []byte{0x04, 0x00, 0x00, 0x00, 0x00, 0x01},
			keepMarker: false,
			wantValue:  1,
			wantLen:    6,
		},
		{
			name:       "7-byte VINT",
			data:       []byte{0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
			keepMarker: false,
			wantValue:  1,
			wantLen:    7,
		},
		{
			name:       "8-byte VINT",
			data:       []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
			keepMarker: false,
			wantValue:  1,
			wantLen:    8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bytes.NewReader(tt.data)
			value, length, err := readVINT(r, tt.keepMarker)
			if err != nil {
				t.Errorf("readVINT() error = %v", err)
				return
			}
			if value != tt.wantValue {
				t.Errorf("readVINT() value = 0x%X, want 0x%X", value, tt.wantValue)
			}
			if length != tt.wantLen {
				t.Errorf("readVINT() length = %d, want %d", length, tt.wantLen)
			}
		})
	}
}

func TestReadVINT_TruncatedData(t *testing.T) {
	// 2-byte VINT indicator but only 1 byte of data
	data := []byte{0x40}
	r := bytes.NewReader(data)
	_, _, err := readVINT(r, false)
	if err == nil {
		t.Error("readVINT() expected error for truncated data")
	}
}

func TestReadBinary(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		size      int64
		wantValue []byte
	}{
		{
			name:      "simple binary",
			data:      []byte{0x01, 0x02, 0x03, 0x04},
			size:      4,
			wantValue: []byte{0x01, 0x02, 0x03, 0x04},
		},
		{
			name:      "empty binary",
			data:      []byte{},
			size:      0,
			wantValue: []byte{},
		},
		{
			name:      "single byte",
			data:      []byte{0xFF},
			size:      1,
			wantValue: []byte{0xFF},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bytes.NewReader(tt.data)
			value, err := ReadBinary(r, tt.size)
			if err != nil {
				t.Errorf("ReadBinary() error = %v", err)
				return
			}
			if !bytes.Equal(value, tt.wantValue) {
				t.Errorf("ReadBinary() = %v, want %v", value, tt.wantValue)
			}
		})
	}
}

func TestReadBinary_TruncatedData(t *testing.T) {
	data := []byte{0x01, 0x02}
	r := bytes.NewReader(data)
	_, err := ReadBinary(r, 10) // Request more than available
	if err == nil {
		t.Error("ReadBinary() expected error for truncated data")
	}
}

func TestReadUint_TruncatedData(t *testing.T) {
	data := []byte{0x01}
	r := bytes.NewReader(data)
	_, err := ReadUint(r, 4) // Request 4 bytes but only 1 available
	if err == nil {
		t.Error("ReadUint() expected error for truncated data")
	}
}

func TestReadString_TruncatedData(t *testing.T) {
	data := []byte{0x41, 0x42}
	r := bytes.NewReader(data)
	_, err := ReadString(r, 10) // Request more than available
	if err == nil {
		t.Error("ReadString() expected error for truncated data")
	}
}

func TestReadInt(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		size      int64
		wantValue int64
	}{
		{
			name:      "positive 1-byte",
			data:      []byte{0x42},
			size:      1,
			wantValue: 0x42,
		},
		{
			name:      "negative 1-byte",
			data:      []byte{0xFF},
			size:      1,
			wantValue: -1,
		},
		{
			name:      "positive 2-byte",
			data:      []byte{0x00, 0x7F},
			size:      2,
			wantValue: 127,
		},
		{
			name:      "negative 2-byte",
			data:      []byte{0xFF, 0x80},
			size:      2,
			wantValue: -128,
		},
		{
			name:      "zero size",
			data:      []byte{},
			size:      0,
			wantValue: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bytes.NewReader(tt.data)
			value, err := ReadInt(r, tt.size)
			if err != nil {
				t.Errorf("ReadInt() error = %v", err)
				return
			}
			if value != tt.wantValue {
				t.Errorf("ReadInt() = %d, want %d", value, tt.wantValue)
			}
		})
	}
}

func TestParseSimpleBlockHeader_LargeTrackNumber(t *testing.T) {
	// Track number > 127 requires 2-byte VINT
	data := []byte{0x40, 0x80, 0x00, 0x00, 0x80} // Track 128, TS 0, keyframe
	header, err := ParseSimpleBlockHeader(data)
	if err != nil {
		t.Fatalf("ParseSimpleBlockHeader() error = %v", err)
	}
	if header.TrackNumber != 128 {
		t.Errorf("TrackNumber = %d, want 128", header.TrackNumber)
	}
	if header.HeaderSize != 5 { // 2-byte track VINT + 2-byte TS + 1-byte flags
		t.Errorf("HeaderSize = %d, want 5", header.HeaderSize)
	}
}

func TestParseSimpleBlockHeader_EmptyData(t *testing.T) {
	_, err := ParseSimpleBlockHeader([]byte{})
	if err == nil {
		t.Error("ParseSimpleBlockHeader() expected error for empty data")
	}
}

func TestParseVINTFromBytes(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		wantValue uint64
		wantLen   int
	}{
		{
			name:      "1-byte VINT",
			data:      []byte{0x81},
			wantValue: 1,
			wantLen:   1,
		},
		{
			name:      "2-byte VINT",
			data:      []byte{0x40, 0x02},
			wantValue: 2,
			wantLen:   2,
		},
		{
			name:      "empty data",
			data:      []byte{},
			wantValue: 0,
			wantLen:   0,
		},
		{
			name:      "invalid VINT (0x00)",
			data:      []byte{0x00},
			wantValue: 0,
			wantLen:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, length := parseVINTFromBytes(tt.data)
			if value != tt.wantValue {
				t.Errorf("parseVINTFromBytes() value = 0x%X, want 0x%X", value, tt.wantValue)
			}
			if length != tt.wantLen {
				t.Errorf("parseVINTFromBytes() length = %d, want %d", length, tt.wantLen)
			}
		})
	}
}

func TestReadElementHeader_TruncatedData(t *testing.T) {
	// VINT indicates 4-byte ID but not enough data
	data := []byte{0x1A, 0x45}
	r := bytes.NewReader(data)
	_, err := ReadElementHeader(r, 0)
	if err == nil {
		t.Error("ReadElementHeader() expected error for truncated data")
	}
}

func TestReadElementHeader_UnknownSize(t *testing.T) {
	// Create element with unknown size (all 1s in the size bytes)
	// Cluster element with 1-byte unknown size (0xFF = 0x7F after removing marker)
	data := []byte{0x1F, 0x43, 0xB6, 0x75, 0xFF} // Cluster ID + unknown 1-byte size
	r := bytes.NewReader(data)
	elem, err := ReadElementHeader(r, 0)
	if err != nil {
		t.Errorf("ReadElementHeader() error = %v", err)
		return
	}
	if elem.Size >= 0 {
		t.Errorf("ReadElementHeader() Size = %d, expected negative for unknown size", elem.Size)
	}
}

func TestElementIDs(t *testing.T) {
	// Verify that all defined element IDs have correct values
	tests := []struct {
		name     string
		id       uint64
		expected uint64
	}{
		{"EBML Header", IDEBMLHeader, 0x1A45DFA3},
		{"Segment", IDSegment, 0x18538067},
		{"Cluster", IDCluster, 0x1F43B675},
		{"Tracks", IDTracks, 0x1654AE6B},
		{"SimpleBlock", IDSimpleBlock, 0xA3},
		{"Block", IDBlock, 0xA1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.id != tt.expected {
				t.Errorf("%s ID = 0x%X, want 0x%X", tt.name, tt.id, tt.expected)
			}
		})
	}
}

func TestLacingConstants(t *testing.T) {
	// Verify lacing constants are correct
	if LacingNone != 0x00 {
		t.Errorf("LacingNone = 0x%02X, want 0x00", LacingNone)
	}
	if LacingXiph != 0x02 {
		t.Errorf("LacingXiph = 0x%02X, want 0x02", LacingXiph)
	}
	if LacingFixed != 0x04 {
		t.Errorf("LacingFixed = 0x%02X, want 0x04", LacingFixed)
	}
	if LacingEBML != 0x06 {
		t.Errorf("LacingEBML = 0x%02X, want 0x06", LacingEBML)
	}
}

func TestTrackTypeConstants(t *testing.T) {
	// Verify track type constants
	if TrackTypeVideo != 1 {
		t.Errorf("TrackTypeVideo = %d, want 1", TrackTypeVideo)
	}
	if TrackTypeAudio != 2 {
		t.Errorf("TrackTypeAudio = %d, want 2", TrackTypeAudio)
	}
}

func TestNewParser_FileNotFound(t *testing.T) {
	_, err := NewParser("/nonexistent/path/file.mkv")
	if err == nil {
		t.Error("NewParser() expected error for nonexistent file")
	}
}

func TestParser_EmptyFile(t *testing.T) {
	// Create an empty file
	tmpDir := t.TempDir()
	emptyFile := tmpDir + "/empty.mkv"
	f, err := os.Create(emptyFile)
	if err != nil {
		t.Fatalf("Failed to create empty file: %v", err)
	}
	f.Close()

	parser, err := NewParser(emptyFile)
	if err != nil {
		// Empty file might fail to open, that's ok
		return
	}
	defer parser.Close()

	// Parsing should fail
	err = parser.Parse(nil)
	if err == nil {
		t.Error("Parse() expected error for empty file")
	}
}

func TestParser_Close(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := tmpDir + "/test.mkv"

	// Create a minimal file with EBML header
	f, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	// Write minimal EBML header
	f.Write([]byte{0x1A, 0x45, 0xDF, 0xA3, 0x81, 0x00}) // EBML header, size 0
	f.Close()

	parser, err := NewParser(testFile)
	if err != nil {
		t.Fatalf("NewParser() error = %v", err)
	}

	// Close should not error
	if err := parser.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Close again should be safe (idempotent)
	if err := parser.Close(); err != nil {
		t.Errorf("Second Close() error = %v", err)
	}
}

func TestParser_Accessors(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := tmpDir + "/test.mkv"

	// Create a minimal MKV-like file
	f, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	// Write EBML header + Segment
	f.Write([]byte{
		0x1A, 0x45, 0xDF, 0xA3, 0x81, 0x00, // EBML header, size 0
		0x18, 0x53, 0x80, 0x67, 0x81, 0x00, // Segment, size 0
	})
	f.Close()

	parser, err := NewParser(testFile)
	if err != nil {
		t.Fatalf("NewParser() error = %v", err)
	}
	defer parser.Close()

	// Test Size()
	if parser.Size() != 12 {
		t.Errorf("Size() = %d, want 12", parser.Size())
	}

	// Test Packets() on empty parser (before Parse)
	if len(parser.Packets()) != 0 {
		t.Errorf("Packets() len = %d, want 0", len(parser.Packets()))
	}

	// Test Tracks() on empty parser
	if len(parser.Tracks()) != 0 {
		t.Errorf("Tracks() len = %d, want 0", len(parser.Tracks()))
	}

	// Test PacketCount()
	if parser.PacketCount() != 0 {
		t.Errorf("PacketCount() = %d, want 0", parser.PacketCount())
	}

	// Test Data()
	if parser.Data() == nil {
		t.Error("Data() returned nil")
	}
}

func TestIsTopLevelElement(t *testing.T) {
	tests := []struct {
		id   uint64
		want bool
	}{
		{IDSeekHead, true},
		{IDInfo, true},
		{IDTracks, true},
		{IDChapters, true},
		{IDCluster, true},
		{IDCues, true},
		{IDTags, true},
		{IDSimpleBlock, false}, // Not top-level
		{IDBlock, false},       // Not top-level
		{0x1234, false},        // Unknown ID
	}

	for _, tt := range tests {
		got := isTopLevelElement(tt.id)
		if got != tt.want {
			t.Errorf("isTopLevelElement(0x%X) = %v, want %v", tt.id, got, tt.want)
		}
	}
}
