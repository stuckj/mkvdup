package mkv

import (
	"bytes"
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
